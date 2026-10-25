package app

import (
	"fmt"
	"math"
	"time"

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
	ownTeam string
	members []*member
}

type member struct {
	puuid  string
	agent  string
	team   string
	hidden bool
	p      *player
}

const (
	rosterEvery    = 3 * time.Second
	defaultCutoff  = 40 * time.Second
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
	lv.setPhase(phaseAgentSelect)
	if q, ok := lv.presenceQueue(); ok && !tracked(q) {
		lv.untrackedMode(q)
		return
	}
	lob := &lobby{kind: lobbyAgentSelect, matchID: matchID}
	lv.lobby = lob
	lv.guard.SetWindow(remote.AgentSelect)
	lv.ensureTarget(func() {
		if lob != lv.lobby {
			return
		}
		if lob.matchID == "" {
			lv.findMatchID(lob, "/pregame/v1/players/")
			return
		}
		lv.fetchRoster(lob)
	})
}

// startMatchLoad keeps everything agent select already fetched, in flight or not
func (lv *live) startMatchLoad(matchID string) {
	prev := lv.lobby
	lv.stopRosterTimer()
	lv.setPhase(phaseMatchLoad)
	if lv.untracked != "" {
		return
	}
	if q, ok := lv.presenceQueue(); ok && !tracked(q) && (prev == nil || !prev.known) {
		lv.untrackedMode(q)
		return
	}
	lob := &lobby{kind: lobbyMatchLoad, matchID: matchID}
	if prev != nil {
		lob.mode, lob.known, lob.mapURL, lob.ownTeam = prev.mode, prev.known, prev.mapURL, prev.ownTeam
		lob.members = prev.members
	}
	lv.lobby = lob
	lv.guard.SetWindow(remote.MatchLoad)
	lv.cutoffStop = lv.env.Clock.AfterFunc(defaultCutoff, func() {
		lv.loop.post(func() { lv.onCutoff(lob) })
	})
	lv.ensureTarget(func() {
		if lob != lv.lobby {
			return
		}
		if lob.matchID == "" {
			lv.findMatchID(lob, "/core-game/v1/players/")
			return
		}
		lv.fetchRoster(lob)
	})
}

// onCutoff is the deadline after match load - rounds are about to start, so everything stops
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
	sess := newSession(lv.self, lv.env.Clock.Now(), defaultResetHour)
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
		lv.applyRoster(lob, roster)
	})
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
	lv.eng.batch(func() { lv.applyRosterBatch(lob, roster) })
}

func (lv *live) applyRosterBatch(lob *lobby, roster riot.Roster) {
	if !lob.known {
		queue, ok := roster.Queue, roster.HasQueue
		if !ok || queue == "" && lob.kind == lobbyMatchLoad {
			queue, ok = lv.presenceQueue()
		}
		mode, tracked := stats.ModeByQueue(queue)
		if !tracked {
			lv.untrackedMode(queue)
			return
		}
		lob.mode, lob.mapURL, lob.known = mode, roster.Map, true
		lv.eng.fetchActs()
		if lob.kind == lobbyAgentSelect {
			sess := newSession(lv.self, lv.env.Clock.Now(), defaultResetHour)
			lv.eng.refreshSession(sess, nil)
			lv.sess = sess
		}
	}
	for _, rp := range roster.Players {
		if rp.PUUID == lv.self {
			lob.ownTeam = rp.Team
		}
	}
	var members []*member
	add := func(rp riot.RosterPlayer) {
		m := lob.member(rp.PUUID)
		if m == nil {
			m = &member{puuid: rp.PUUID, hidden: rp.Incognito && rp.PUUID != lv.self}
			if m.hidden {
				lv.guard.Hide(m.puuid)
			} else {
				m.p = newPlayer(m.puuid, lob.mode.Queue, defaultMatches)
				want := wantRank | wantTrend | wantSample
				if m.puuid == lv.self {
					// your trend comes from the session's competitive updates
					want = wantRank | wantSample
				}
				lv.eng.lookUp(m.p, want)
			}
		}
		m.agent, m.team = rp.Agent, rp.Team
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
}

func (lv *live) untrackedMode(queue string) {
	if queue == "" {
		queue = "custom games"
	}
	lv.untracked = queue
	lv.cancelLobby()
}

// cancelLobby drops the lobby and every outstanding lookup - a dodge, an untracked mode or the match ending
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

func tracked(queue string) bool {
	_, ok := stats.ModeByQueue(queue)
	return ok
}

func (lv *live) lobbyScreen(s *render.Screen, lob *lobby, title string) {
	if lob.known {
		mapName, ok := lv.data.MapName(lob.mapURL)
		if !ok {
			mapName = "?"
		}
		title += " · " + mapName + " · " + lob.mode.Name
	}
	s.Header = title
	if len(lob.members) == 0 {
		return
	}
	// deathmatch has no teams - everyone is one list with a lobby average and nothing to diff
	if lob.kind == lobbyAgentSelect || !lob.mode.Teams {
		s.Sections = []render.Section{lv.teamSection("", lob.members, lob.mode)}
		return
	}
	var allies, enemies []*member
	for _, m := range lob.members {
		if m.team == lob.ownTeam {
			allies = append(allies, m)
		} else {
			enemies = append(enemies, m)
		}
	}
	us, them := lv.teamAverages(allies, lob.mode), lv.teamAverages(enemies, lob.mode)
	s.Sections = []render.Section{
		lv.teamSection("ALLIES", allies, lob.mode),
		lv.teamSection("ENEMIES", enemies, lob.mode),
		{Rows: []render.Row{diffRow(us, them)}},
	}
}

func (lv *live) teamSection(title string, members []*member, mode stats.Mode) render.Section {
	sec := render.Section{Title: title}
	for _, m := range members {
		sec.Rows = append(sec.Rows, lv.memberRow(m, mode))
	}
	sec.Rows = append(sec.Rows, lv.teamAverages(members, mode).row(lv))
	return sec
}

func (lv *live) memberRow(m *member, mode stats.Mode) render.Row {
	if m.hidden {
		return render.Row{"name": {Text: "hidden"}, "agent": lv.agentCell(m.agent)}
	}
	if m.puuid == lv.self && lv.sess != nil && lv.sess.done {
		m.p.updates, m.p.updatesDone, m.p.updatesErr = lv.sess.updates, true, lv.sess.err
	}
	row := playerRow(m.p, mode, lv.eng.acts, lv.data)
	row["agent"] = lv.agentCell(m.agent)
	return row
}

func (lv *live) agentCell(id string) render.Cell {
	if id == "" {
		return render.Cell{}
	}
	if name, ok := lv.data.AgentName(id); ok {
		return render.Cell{Text: name}
	}
	return render.Cell{Text: "?"}
}

// averages are the shown values of a team's average row - the diff subtracts what's on screen
type averages struct {
	covered, size    int
	rank, peak, last int
	wr, trend        int
	hasWR, hasTrend  bool
	kd               float64
	hasKD            bool
	hs, acs, adr, fb int
	hasHS, hasRounds bool
}

func (lv *live) teamAverages(members []*member, mode stats.Mode) averages {
	var tiers, peaks, lasts, wins, trends []float64
	var kds, hss, acss, adrs, fbs []float64
	a := averages{size: len(members)}
	for _, m := range members {
		if m.hidden {
			continue
		}
		if m.p.historyDone && m.p.historyErr == nil {
			if s := stats.SampleOf(m.p.puuid, m.p.loaded()); s.N > 0 {
				kds = append(kds, s.KD())
				if v, ok := s.HSExact(); ok {
					hss = append(hss, v)
				}
				if mode.Rounds {
					acs, _ := s.ACSExact()
					adr, _ := s.ADRExact()
					fb, _ := s.FBExact()
					acss, adrs, fbs = append(acss, acs), append(adrs, adr), append(fbs, fb)
				}
			}
		}
		if !m.p.mmrDone || m.p.mmrErr != nil || lv.eng.acts == nil {
			continue
		}
		a.covered++
		r := stats.RankOf(m.p.mmr, *lv.eng.acts, mode.Queue, lv.data.CurrentTier)
		if r.Tier >= 3 {
			tiers = append(tiers, float64(r.Tier))
		}
		if r.Peak >= 3 {
			peaks = append(peaks, float64(r.Peak))
		}
		if r.Last >= 3 {
			lasts = append(lasts, float64(r.Last))
		}
		if mode.WinRate && r.HasRecord {
			wins = append(wins, r.WinRatio()*100)
		}
		if m.p.updatesDone && m.p.updatesErr == nil {
			if t, ok := stats.Trend(m.p.updates, 5); ok {
				trends = append(trends, float64(t))
			}
		}
	}
	a.rank, _ = roundedMean(tiers)
	a.peak, _ = roundedMean(peaks)
	a.last, _ = roundedMean(lasts)
	a.wr, a.hasWR = roundedMean(wins)
	a.trend, a.hasTrend = roundedMean(trends)
	if v, ok := mean(kds); ok {
		a.kd, a.hasKD = math.Round(v*100)/100, true
	}
	a.hs, a.hasHS = roundedMean(hss)
	a.acs, a.hasRounds = roundedMean(acss)
	a.adr, _ = roundedMean(adrs)
	a.fb, _ = roundedMean(fbs)
	return a
}

func (a averages) row(lv *live) render.Row {
	label := "AVERAGE"
	if a.covered < a.size {
		label += fmt.Sprintf(" %d/%d", a.covered, a.size)
	}
	row := render.Row{"name": {Text: label}}
	if a.rank > 0 {
		row["rank"] = tierCell(lv.data, a.rank, "")
	}
	if a.peak > 0 {
		row["peak"] = tierCell(lv.data, a.peak, "")
	}
	if a.last > 0 {
		row["last_act"] = tierCell(lv.data, a.last, "")
	}
	if a.hasWR {
		row["wr"] = render.Cell{Text: fmt.Sprintf("%d%%", a.wr)}
	}
	if a.hasTrend {
		row["rr_trend"] = render.Cell{Text: signed(a.trend)}
	}
	if a.hasKD {
		row["kd"] = render.Cell{Text: fmt.Sprintf("%.2f", a.kd)}
	}
	if a.hasHS {
		row["hs"] = render.Cell{Text: fmt.Sprintf("%d%%", a.hs)}
	}
	if a.hasRounds {
		row["acs"] = render.Cell{Text: fmt.Sprint(a.acs)}
		row["adr"] = render.Cell{Text: fmt.Sprint(a.adr)}
		row["fb"] = render.Cell{Text: fmt.Sprintf("%d%%", a.fb)}
	}
	return row
}

// diffRow is us minus them - ranks in tiers - for every stat both teams have
func diffRow(us, them averages) render.Row {
	row := render.Row{"name": {Text: "US - THEM"}}
	diff := func(key string, a, b int, ok bool) {
		if ok {
			row[key] = render.Cell{Text: arrow(a - b)}
		}
	}
	diff("rank", us.rank, them.rank, us.rank > 0 && them.rank > 0)
	diff("peak", us.peak, them.peak, us.peak > 0 && them.peak > 0)
	diff("last_act", us.last, them.last, us.last > 0 && them.last > 0)
	diff("wr", us.wr, them.wr, us.hasWR && them.hasWR)
	diff("rr_trend", us.trend, them.trend, us.hasTrend && them.hasTrend)
	diff("hs", us.hs, them.hs, us.hasHS && them.hasHS)
	diff("acs", us.acs, them.acs, us.hasRounds && them.hasRounds)
	diff("adr", us.adr, them.adr, us.hasRounds && them.hasRounds)
	diff("fb", us.fb, them.fb, us.hasRounds && them.hasRounds)
	if us.hasKD && them.hasKD {
		row["kd"] = render.Cell{Text: arrowKD(us.kd - them.kd)}
	}
	return row
}

func arrowKD(d float64) string {
	d = math.Round(d*100) / 100
	switch {
	case d > 0:
		return fmt.Sprintf("▲%.2f", d)
	case d < 0:
		return fmt.Sprintf("▼%.2f", -d)
	}
	return "0"
}

func arrow(d int) string {
	switch {
	case d > 0:
		return fmt.Sprintf("▲%d", d)
	case d < 0:
		return fmt.Sprintf("▼%d", -d)
	}
	return "0"
}

func mean(vs []float64) (float64, bool) {
	if len(vs) == 0 {
		return 0, false
	}
	sum := 0.0
	for _, v := range vs {
		sum += v
	}
	return sum / float64(len(vs)), true
}

func roundedMean(vs []float64) (int, bool) {
	v, ok := mean(vs)
	return int(math.Round(v)), ok
}
