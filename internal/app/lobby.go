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

const rosterEvery = 3 * time.Second

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
	lv.lobby = &lobby{kind: lobbyAgentSelect, matchID: matchID}
	lv.guard.SetWindow(remote.AgentSelect)
	lob := lv.lobby
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
	if !lob.known {
		queue, ok := roster.Queue, roster.HasQueue
		if !ok {
			queue, ok = lv.presenceQueue()
		}
		mode, tracked := stats.ModeByQueue(queue)
		if !tracked {
			lv.untrackedMode(queue)
			return
		}
		lob.mode, lob.mapURL, lob.known = mode, roster.Map, true
		lv.eng.fetchActs()
		lv.sess = newSession(lv.self, lv.env.Clock.Now(), defaultResetHour)
		lv.eng.refreshSession(lv.sess)
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
				want := wantRank | wantTrend
				if m.puuid == lv.self {
					// your trend comes from the session's competitive updates
					want = wantRank
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
		if rp.PUUID != lv.self && (lob.kind == lobbyMatchLoad || rp.Team == lob.ownTeam) {
			add(rp)
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

// cancelLobby drops the lobby and every outstanding lookup - a dodge, the cutoff or an untracked mode
func (lv *live) cancelLobby() {
	lv.eng.cancel()
	lv.stopRosterTimer()
	lv.lobby = nil
	lv.guard.SetWindow(remote.Closed)
}

func (lv *live) presenceQueue() (string, bool) {
	return lv.presence.QueueID, lv.presence.HasQueueID
}

func tracked(queue string) bool {
	_, ok := stats.ModeByQueue(queue)
	return ok
}

func (lv *live) lobbyScreen(s *render.Screen) {
	lob := lv.lobby
	title := map[lobbyKind]string{lobbyAgentSelect: "AGENT SELECT", lobbyMatchLoad: "MATCH LOADING"}[lob.kind]
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
	var rows []render.Row
	for _, m := range lob.members {
		rows = append(rows, lv.memberRow(m, lob.mode))
	}
	rows = append(rows, lv.averageRow(lob.members, lob.mode))
	s.Sections = []render.Section{{Rows: rows}}
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

// averageRow is the mean over visible players that have a value - ranks use the nearest tier
func (lv *live) averageRow(members []*member, mode stats.Mode) render.Row {
	var tiers, peaks, lasts, wins, trends []float64
	covered := 0
	for _, m := range members {
		if m.hidden || !m.p.mmrDone || m.p.mmrErr != nil || lv.eng.acts == nil {
			continue
		}
		covered++
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
	label := "AVERAGE"
	if covered < len(members) {
		label += fmt.Sprintf(" %d/%d", covered, len(members))
	}
	row := render.Row{"name": {Text: label}}
	if v, ok := mean(tiers); ok {
		row["rank"] = tierCell(lv.data, int(math.Round(v)), "")
	}
	if v, ok := mean(peaks); ok {
		row["peak"] = tierCell(lv.data, int(math.Round(v)), "")
	}
	if v, ok := mean(lasts); ok {
		row["last_act"] = tierCell(lv.data, int(math.Round(v)), "")
	}
	if v, ok := mean(wins); ok {
		row["wr"] = render.Cell{Text: fmt.Sprintf("%d%%", int(math.Round(v)))}
	}
	if v, ok := mean(trends); ok {
		row["rr_trend"] = render.Cell{Text: signed(int(math.Round(v)))}
	}
	return row
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
