package app

import (
	"fmt"
	"time"

	"github.com/glitchfl/lowkey/internal/lobbytable"
	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/stats"
)

type lobbyKind int

const (
	lobbyAgentSelect lobbyKind = iota
	lobbyMatchLoad
)

type lobby struct {
	kind    lobbyKind
	matchID string
	mode    stats.Mode
	known   bool // mode and map came from the roster
	mapURL  string
	pod     string
	ownTeam string
	members []*member
	party   *party
	voice   map[string]string // what the local client says about each player's voice - asked again with every agent select roster
	asking  bool              // a voice question is out
	started time.Time
	rounds  int // rounds played - what picks the side in the header
	// skins come once per match load and only while the skin column is on
	skins         riot.Loadouts
	loadoutsAsked bool
}

type member struct {
	puuid string
	agent string
	hover bool // the agent is picked in agent select but not locked in
	team  string
	vis   lobbytable.Visibility
	level int     // zero when hidden by the player
	p     *player // nil while vis.Stats is off
}

const (
	rosterEvery    = 3 * time.Second
	sessionRefresh = 15 * time.Second
)

func (l *lobby) member(puuid string) *member {
	for _, m := range l.members {
		if m.puuid == puuid {
			return m
		}
	}
	return nil
}

func (lv *live) startAgentSelect(matchID string) {
	from := lv.phase
	lv.setPhase(phaseAgentSelect)
	lv.rateLimit, lv.problem = rateFine, ""
	if q, ok := lv.presenceQueue(); ok && !lv.tracked(q) {
		lv.untrackedMode(q)
		return
	}
	lob := &lobby{kind: lobbyAgentSelect, matchID: matchID, party: &party{}, started: lv.env.Clock.Now()}
	lv.lobby = lob
	if from == phaseMenus {
		lv.matchFound()
	}
	lv.guard.SetWindow(remote.AgentSelect)
	lv.ensureTarget(func() {
		if lob != lv.lobby {
			return
		}
		lv.fetchParty(lob.party)
		if lob.matchID == "" {
			lv.findMatchID(lob, "/pregame/v1/players/")
			return
		}
		lv.fetchRoster(lob)
	})
}

// startMatchLoad keeps everything agent select already fetched - in flight or not
func (lv *live) startMatchLoad(matchID string) {
	prev := lv.lobby
	lv.stopRosterTimer()
	lv.setPhase(phaseMatchLoad)
	if lv.untracked != "" {
		return
	}
	if q, ok := lv.presenceQueue(); ok && !lv.tracked(q) && (prev == nil || !prev.known) {
		lv.untrackedMode(q)
		return
	}
	lob := &lobby{kind: lobbyMatchLoad, matchID: matchID, started: lv.env.Clock.Now()}
	if prev == nil {
		lv.rateLimit, lv.problem = rateFine, ""
	}
	if prev != nil {
		lob.mode, lob.known, lob.mapURL, lob.pod, lob.ownTeam = prev.mode, prev.known, prev.mapURL, prev.pod, prev.ownTeam
		lob.members, lob.party = prev.members, prev.party
		// agent select is over so whatever was still hovered got locked in
		for _, m := range lob.members {
			m.hover = false
		}
	}
	lv.lobby = lob
	lv.guard.SetWindow(remote.MatchLoad)
	lv.cutoffStop = lv.env.Clock.AfterFunc(lv.cfg.Settings().Cutoff, func() {
		lv.loop.post(func() { lv.onCutoff(lob) })
	})
	lv.ensureTarget(func() {
		if lob != lv.lobby {
			return
		}
		if lob.party == nil {
			lob.party = &party{}
			lv.fetchParty(lob.party)
		}
		if lob.matchID == "" {
			lv.findMatchID(lob, "/core-game/v1/players/")
			return
		}
		lv.fetchRoster(lob)
	})
}

// onCutoff is the deadline after match load - rounds are about to start so everything stops
func (lv *live) onCutoff(lob *lobby) {
	if lob != lv.lobby || lv.phase != phaseMatchLoad {
		return
	}
	lv.cutoffStop = nil
	lv.eng.cancel()
	lv.guard.SetWindow(remote.Closed)
	lv.setPhase(phaseInRound)
}

func (lv *live) stopCutoff() {
	if lv.cutoffStop != nil {
		lv.cutoffStop()
		lv.cutoffStop = nil
	}
}

// matchEnded keeps the final table on screen and refreshes the session line a little later
func (lv *live) matchEnded() {
	if lv.lobby != nil && lv.lobby.kind == lobbyMatchLoad && lv.lobby.known {
		lv.last = lv.lobby
		// the final score counts the winning round - there's no next round to show the side of
		lv.last.rounds = max(lv.last.rounds-1, 0)
		lv.remember(lv.lobby)
	}
	lv.cancelLobby()
	lv.stopCutoff()
	lv.eng.forgetRound()
	lv.env.Clock.AfterFunc(sessionRefresh, func() {
		lv.loop.post(lv.refreshSessionInMenus)
	})
}

// refreshSessionInMenus opens the session refresh window just long enough for its requests
func (lv *live) refreshSessionInMenus() {
	if lv.phase != phaseMenus || lv.conn == nil || lv.eng.sched == nil {
		return
	}
	lv.guard.SetWindow(remote.SessionRefresh)
	sess := newSession(lv.self, lv.env.Clock.Now(), lv.cfg.Settings().SessionResetHour)
	lv.ensureTarget(func() {
		lv.eng.refreshSession(sess, func() {
			if lv.phase == phaseMenus {
				lv.guard.SetWindow(remote.Closed)
			}
			lv.sess = sess
		})
	})
}

// findMatchID asks Riot which match you're in when presence moved before the lifecycle event came
func (lv *live) findMatchID(lob *lobby, path string) {
	lv.eng.submit(&job{prio: prioRoster, svc: remote.GLZ, path: path + lv.self}, func(body []byte, err error) {
		if lob != lv.lobby || lob.matchID != "" {
			return
		}
		if err == nil {
			lob.matchID, err = riot.DecodeMatchID(body)
		}
		if err != nil {
			lv.problem = fmt.Sprintf("can't find your match: %v", err)
			return
		}
		if lv.ended(lob.matchID) {
			// Riot still points at the match that just ended - the next lifecycle event names the new one
			lob.matchID = ""
			return
		}
		lv.fetchRoster(lob)
	})
}

func (lv *live) fetchRoster(lob *lobby) {
	lv.rosterAt = lv.env.Clock.Now()
	path := "/pregame/v1/matches/"
	if lob.kind == lobbyMatchLoad {
		path = "/core-game/v1/matches/"
	}
	lv.eng.submit(&job{prio: prioRoster, svc: remote.GLZ, path: path + lob.matchID}, func(body []byte, err error) {
		if lob != lv.lobby {
			return
		}
		var roster riot.Roster
		if err == nil {
			if lob.kind == lobbyMatchLoad {
				roster, err = riot.DecodeCoreGame(body)
			} else {
				roster, err = riot.DecodePregame(body)
			}
		}
		if err != nil {
			lv.problem = fmt.Sprintf("can't load the roster: %v", err)
			return
		}
		// a match that ended or one you're not in is a leftover - Riot keeps sending events about both -
		// and the Range is no match at all
		if roster.Over || roster.Range || lob.kind == lobbyMatchLoad && !roster.Has(lv.self) {
			lv.matchOver(lob)
			return
		}
		lv.applyRoster(lob, roster)
	})
}

// matchOver drops a match load for a match that already ended or the Range - an event about it
// started the load - and lets your presence say where you really are
func (lv *live) matchOver(lob *lobby) {
	lv.finish(lob.matchID)
	lv.cancelLobby()
	lv.stopCutoff()
	lv.setPhase(phaseMenus)
	lv.applyPresence(lv.presence, true, false)
}

// refreshRoster re-fetches the roster at most every 3 s however many hover and lock events arrive
func (lv *live) refreshRoster() {
	lob := lv.lobby
	if lob == nil || lob.matchID == "" || lv.rosterStop != nil {
		return
	}
	wait := lv.rosterAt.Add(rosterEvery).Sub(lv.env.Clock.Now())
	if wait <= 0 {
		lv.fetchRoster(lob)
		return
	}
	lv.rosterStop = lv.env.Clock.AfterFunc(wait, func() {
		lv.loop.post(func() {
			lv.rosterStop = nil
			if lob == lv.lobby && lv.phase == phaseAgentSelect {
				lv.fetchRoster(lob)
			}
		})
	})
}

func (lv *live) stopRosterTimer() {
	if lv.rosterStop != nil {
		lv.rosterStop()
		lv.rosterStop = nil
	}
}

func (lv *live) applyRoster(lob *lobby, roster riot.Roster) {
	lv.eng.batch(func() {
		lv.applyRosterBatch(lob, roster)
		lv.refreshStatic()
	})
}

func (lv *live) applyRosterBatch(lob *lobby, roster riot.Roster) {
	if !lob.known {
		queue, ok := roster.Queue, roster.HasQueue
		if !ok || queue == "" && lob.kind == lobbyMatchLoad {
			queue, ok = lv.presenceQueue()
		}
		mode, known := stats.ModeByQueue(queue)
		if !known || !lv.tracked(queue) {
			lv.untrackedMode(queue)
			return
		}
		lob.mode, lob.mapURL, lob.known = mode, roster.Map, true
		lv.eng.fetchActs()
		if lob.kind == lobbyAgentSelect {
			sess := newSession(lv.self, lv.env.Clock.Now(), lv.cfg.Settings().SessionResetHour)
			lv.eng.refreshSession(sess, nil)
			lv.sess = sess
		}
	}
	if roster.GamePodID != "" {
		lob.pod = roster.GamePodID
	}
	if lob.kind == lobbyAgentSelect {
		lv.fetchVoice(lob)
	}
	cfg := lv.cfg.Settings()
	for _, rp := range roster.Players {
		if rp.PUUID == lv.self {
			lob.ownTeam = rp.Team
		}
	}
	rules := lobbytable.Rules{Self: lv.self, OwnTeam: lob.ownTeam, MatchLoad: lob.kind == lobbyMatchLoad,
		HiddenPlayerStats: cfg.HiddenPlayerStats, EnemyLookups: cfg.EnemyLookups}
	// every roster decides again - or a setting switched back on would keep refusing last lobby's players
	for _, rp := range roster.Players {
		v := rules.Decide(rp)
		if v.Stats {
			lv.guard.Show(rp.PUUID)
		} else {
			lv.guard.Hide(rp.PUUID)
		}
		lv.guard.KeepNameHidden(rp.PUUID, v.Hidden)
	}
	var members []*member
	add := func(rp riot.RosterPlayer) {
		v := rules.Decide(rp)
		m := lob.member(rp.PUUID)
		if m == nil {
			m = &member{puuid: rp.PUUID, vis: v}
			if v.Stats {
				m.p = newPlayer(m.puuid, lob.mode.Queue, cfg.Matches)
				want := wantRank | wantTrend | wantSample
				if m.puuid == lv.self {
					// your trend comes from the session's competitive updates
					want = wantRank | wantSample
				}
				lv.eng.lookUp(m.p, want)
			}
		}
		m.agent, m.hover, m.team, m.level = rp.Agent, rp.Hover, rp.Team, rp.Level
		m.vis = m.vis.Next(v)
		members = append(members, m)
	}
	for _, rp := range roster.Players {
		if rp.PUUID == lv.self {
			add(rp)
		}
	}
	for _, rp := range roster.Players {
		if rp.PUUID != lv.self && rp.Team == lob.ownTeam {
			add(rp)
		}
	}
	if lob.kind == lobbyMatchLoad {
		for _, rp := range roster.Players {
			if rp.Team != lob.ownTeam {
				add(rp)
			}
		}
	}
	lob.members = members
	if lob.kind == lobbyAgentSelect {
		lv.pickedAgent(lob)
	}
	var named []*player
	for _, m := range members {
		if m.p != nil && !m.vis.Hidden {
			named = append(named, m.p)
		}
	}
	lv.eng.lookUpNames(named)
	// the details list every gun - so while they can be opened the skins come even with the skin column off
	if lob.kind == lobbyMatchLoad && !lob.loadoutsAsked && (cfg.Shows("skin") || lv.canPick()) {
		lv.fetchLoadouts(lob)
	}
}

// fetchLoadouts asks once for the whole match's skins and buddies - one request whatever the lobby's size
func (lv *live) fetchLoadouts(lob *lobby) {
	lob.loadoutsAsked = true
	lv.eng.submit(&job{prio: prioLoadouts, svc: remote.GLZ, path: "/core-game/v1/matches/" + lob.matchID + "/loadouts"}, func(body []byte, err error) {
		// a failed request is in the log already and leaves the skin column hidden
		if err != nil {
			return
		}
		if lob.skins, err = riot.DecodeLoadouts(body); err != nil {
			lv.log.printf("loadouts: %v", err)
		}
	})
}

func (lv *live) untrackedMode(queue string) {
	switch m, ok := stats.ModeByQueue(queue); {
	case ok:
		queue = m.Key
	case queue == "":
		queue = "custom games"
	}
	lv.untracked = queue
	lv.hideDot("untracked mode")
	lv.cancelLobby()
}

// cancelLobby drops the lobby and every outstanding lookup - a dodge or an untracked mode or the match ending
func (lv *live) cancelLobby() {
	if lv.eng.sched != nil {
		lv.eng.cancel()
	}
	lv.stopRosterTimer()
	lv.lobby = nil
	if lv.guard != nil {
		lv.guard.SetWindow(remote.Closed)
	}
}

func (lv *live) presenceQueue() (string, bool) {
	return lv.presence.QueueID, lv.presence.HasQueueID
}

func (lv *live) tracked(queue string) bool {
	return lv.cfg.Settings().Tracks(queue)
}

// sides is the half each team starts on - Riot's red team always attacks first
var sides = map[string]string{"Red": "Attackers", "Blue": "Defenders"}

var otherSide = map[string]string{"Attackers": "Defenders", "Defenders": "Attackers"}

func (lv *live) lobbyTitle(lob *lobby, title string) string {
	if lob.known {
		mapName, ok := lv.data.MapName(lob.mapURL)
		if !ok {
			mapName = "?"
		}
		title += " · " + mapName
		// only the round modes have sides - deathmatch and TDM show none
		if side, ok := sides[lob.ownTeam]; ok && lob.mode.Rounds {
			if lob.mode.Swapped(lob.rounds) {
				side = otherSide[side]
			}
			title += " · " + side
		}
		title += " · " + lob.mode.Name
	}
	if lv.cfg.Settings().ServerPod && (lv.phase == phaseMatchLoad || lv.phase == phaseInRound) {
		if pod := riot.PodName(lob.pod); pod != "" {
			title += " · POD: " + pod
		}
	}
	return title
}

func (lv *live) lobbyScreen(s *render.Screen, lob *lobby, title string) {
	s.Header = lv.lobbyTitle(lob, title)
	s.Sections = lobbytable.Build(lv.tableInput(lob)).Render()
}

// tableInput is everything the lobby table reads - your own trend comes from the session here so it's
// there before any row is built
func (lv *live) tableInput(lob *lobby) lobbytable.Input {
	in := lobbytable.Input{
		Lobby: lobbytable.Lobby{MatchLoad: lob.kind == lobbyMatchLoad, Mode: lob.mode, Self: lv.self, OwnTeam: lob.ownTeam,
			Skins: lob.skins, Voice: lob.voice},
		Acts: lv.eng.acts, Data: lv.data, Settings: lv.cfg.Settings(), Picking: lv.canPick(), Picked: lv.pick, Alias: lv.alias,
	}
	if lob.party != nil {
		in.Lobby.Party = lob.party.members
	}
	met := lv.metIndex(lob.matchID)
	now := lv.env.Clock.Now()
	for _, m := range lob.members {
		tm := lobbytable.Member{PUUID: m.puuid, Agent: m.agent, Hover: m.hover, Team: m.team, Level: m.level, Visibility: m.vis}
		if m.p != nil {
			p := lv.facts(m)
			tm.Player = &p
		}
		if e, ok := met[m.puuid]; ok {
			tm.Met = e.text(now)
		}
		in.Lobby.Members = append(in.Lobby.Members, tm)
	}
	return in
}

// facts are a member's downloads for the lobby table and player details
func (lv *live) facts(m *member) lobbytable.Player {
	if m.puuid == lv.self {
		return m.p.facts(lv.sess)
	}
	return m.p.facts(nil)
}

func (lv *live) fetchVoice(lob *lobby) {
	if lob.asking || lv.conn == nil || lv.conn.local == nil || !lv.cfg.Settings().VoiceCheck {
		return
	}
	lob.asking = true
	gen, local := lv.gen, lv.conn.local
	go func() {
		status, err := local.Voice(lv.ctx)
		lv.loop.post(func() {
			lob.asking = false
			if gen != lv.gen || lob != lv.lobby {
				return
			}
			if err != nil {
				status = nil
			}
			lob.voice = status
		})
	}()
}
