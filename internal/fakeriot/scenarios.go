package fakeriot

import (
	"context"
	"fmt"
	"sort"
	"time"
)

const (
	LobbyPregame  = "9e000000-0000-4000-8000-0000000000aa"
	LobbyCoreGame = "c9000000-0000-4000-8000-0000000000aa"
	DMCoreGame    = "d9000000-0000-4000-8000-0000000000aa"
)

// lobby players - index 3 on each side plays in streamer mode
var (
	LobbyAllies  = []string{"a1000000-0000-4000-8000-0000000000aa", "a2000000-0000-4000-8000-0000000000aa", "a3000000-0000-4000-8000-0000000000aa", "a4000000-0000-4000-8000-0000000000aa"}
	LobbyEnemies = []string{"e1000000-0000-4000-8000-0000000000aa", "e2000000-0000-4000-8000-0000000000aa", "e3000000-0000-4000-8000-0000000000aa", "e4000000-0000-4000-8000-0000000000aa", "e5000000-0000-4000-8000-0000000000aa"}
)

type Step struct {
	After time.Duration
	Note  string
	Do    func(s *Server)
}

type Scenario struct {
	Name  string
	About string
	World func(now time.Time) *World
	Steps []Step
}

// LobbyWorld is a competitive lobby with ranks, recent matches and games today - heavy makes match
// details as big as real ones for memory checks
func LobbyWorld(now time.Time, heavy bool) *World {
	w := NewWorld(now)
	w.Queue = "competitive"
	me := w.Player(w.Self)
	me.SetRank("competitive", ActCurrent, SeasonInfo{Tier: 20, RR: 12, Wins: 12, WinsWithPlacements: 12, Games: 21, WinsByTier: map[string]int{"20": 2}})
	tiers := []int{18, 15, 26, 9, 16, 14, 27, 17, 0}
	agents := []string{Jett, Killjoy, Omen, Sage, Raze, Breach, Viper, Cypher, Skye}
	everyone := append(append([]string{}, LobbyAllies...), LobbyEnemies...)
	for i, puuid := range everyone {
		p := w.Player(puuid)
		p.Name, p.Tag = fmt.Sprintf("Player%d", i+1), "AA"
		p.Incognito = puuid == LobbyAllies[2] || puuid == LobbyEnemies[2]
		if tiers[i] > 0 {
			p.SetRank("competitive", ActCurrent, SeasonInfo{Tier: tiers[i], RR: 10 * i, Wins: 5 + i, WinsWithPlacements: 5 + i, Games: 12 + i})
		}
		p.Updates = []Update{{MatchID: "u1", Earned: 10 - 3*i}, {MatchID: "u2", Earned: 5}}
	}
	n := 0
	for _, puuid := range append([]string{w.Self}, everyone...) {
		for j := 0; j < 13; j++ {
			n++
			w.AddMatch(sampleMatch(fmt.Sprintf("5a5a0000-0000-4000-8000-%012d", n), puuid, now.Add(-time.Duration(20+j)*time.Hour), heavy))
		}
	}
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("70d40000-0000-4000-8000-%012d", i)
		m := sampleMatch(id, w.Self, now.Add(-time.Duration(i+1)*time.Hour), heavy)
		m.Teams = []TeamResult{{ID: "Blue", Won: i != 1}, {ID: "Red", Won: i == 1}}
		w.AddMatch(m)
		me.Updates = append(me.Updates, Update{MatchID: id, Start: m.Start, Earned: []int{18, -14, 21}[i]})
	}
	w.Pregame = &LiveGame{ID: LobbyPregame, Queue: "competitive", Map: MapAscent, Mode: ModeBomb, Ranked: true}
	w.CoreGame = &LiveGame{ID: LobbyCoreGame, Queue: "competitive", Map: MapAscent, Mode: ModeBomb, Ranked: true}
	for i, puuid := range append([]string{w.Self}, LobbyAllies...) {
		w.Pregame.Players = append(w.Pregame.Players, LivePlayer{PUUID: puuid, Team: "Blue"})
		w.CoreGame.Players = append(w.CoreGame.Players, LivePlayer{PUUID: puuid, Team: "Blue", Agent: agents[i%len(agents)]})
	}
	for i, puuid := range LobbyEnemies {
		w.CoreGame.Players = append(w.CoreGame.Players, LivePlayer{PUUID: puuid, Team: "Red", Agent: agents[(i+4)%len(agents)]})
	}
	return w
}

// NextLobby swaps in ten new players with their own recent matches - for runs of many lobbies in a row
func NextLobby(w *World, round int, heavy bool) (pregame, coreGame string) {
	pregame = fmt.Sprintf("9e%06d-0000-4000-8000-0000000000bb", round)
	coreGame = fmt.Sprintf("c9%06d-0000-4000-8000-0000000000bb", round)
	w.Pregame = &LiveGame{ID: pregame, Queue: "competitive", Map: MapAscent, Mode: ModeBomb, Ranked: true,
		Players: []LivePlayer{{PUUID: w.Self, Team: "Blue"}}}
	w.CoreGame = &LiveGame{ID: coreGame, Queue: "competitive", Map: MapAscent, Mode: ModeBomb, Ranked: true,
		Players: []LivePlayer{{PUUID: w.Self, Team: "Blue", Agent: Jett}}}
	for i := 0; i < 9; i++ {
		puuid := fmt.Sprintf("%02x%06d-0000-4000-8000-0000000000bb", i+1, round)
		p := w.Player(puuid)
		p.Name, p.Tag = fmt.Sprintf("R%dP%d", round, i), "BB"
		p.SetRank("competitive", ActCurrent, SeasonInfo{Tier: 10 + i, RR: 5 * i, Wins: 6, WinsWithPlacements: 6, Games: 11})
		team := "Red"
		if i < 4 {
			team = "Blue"
			w.Pregame.Players = append(w.Pregame.Players, LivePlayer{PUUID: puuid, Team: team})
		}
		w.CoreGame.Players = append(w.CoreGame.Players, LivePlayer{PUUID: puuid, Team: team, Agent: Sova})
		for j := 0; j < 13; j++ {
			id := fmt.Sprintf("%02x%06d-%04d-4000-8000-0000000000bb", i+1, round, j)
			w.AddMatch(sampleMatch(id, puuid, w.Seasons[0].Start.Add(time.Duration(j)*time.Hour), heavy))
		}
	}
	return pregame, coreGame
}

// sampleMatch is one 24-round match for a player against a fixed bot team
func sampleMatch(id, puuid string, start time.Time, heavy bool) *Match {
	m := &Match{ID: id, Queue: "competitive", Map: MapHaven, Mode: ModeBomb, Start: start,
		Teams: []TeamResult{{ID: "Blue", Won: true, RoundsWon: 13}, {ID: "Red", RoundsWon: 11}}}
	m.Players = append(m.Players, MatchPlayer{PUUID: puuid, Team: "Blue", Agent: Jett, Score: 5000, Rounds: 24, Kills: 20, Deaths: 15})
	var bots []string
	for b := 0; b < 9; b++ {
		bot := fmt.Sprintf("b0b00000-0000-4000-8000-%012d", b)
		bots = append(bots, bot)
		team := "Red"
		if b < 4 {
			team = "Blue"
		}
		m.Players = append(m.Players, MatchPlayer{PUUID: bot, Team: team, Agent: Sova, Score: 4000, Rounds: 24, Kills: 15, Deaths: 15})
	}
	for r := 0; r < 24; r++ {
		round := Round{Damage: []Hit{{From: puuid, To: bots[5], Damage: 150, Head: 1, Body: 3}}}
		if r%3 == 0 {
			round.Kills = append(round.Kills, Kill{Killer: puuid, Victim: bots[5], At: 4000})
		}
		if heavy {
			for k := 0; k < 8; k++ {
				round.Kills = append(round.Kills, Kill{Killer: bots[k%4], Victim: bots[4+k%5], At: 10000 + k*3000})
				round.Damage = append(round.Damage, Hit{From: bots[k%4], To: bots[4+k%5], Damage: 140, Head: 1, Body: 2, Leg: 1})
			}
		}
		m.Rounds = append(m.Rounds, round)
	}
	m.Heavy = heavy
	return m
}

// DeathmatchWorld is fourteen players with no teams - one in streamer mode
func DeathmatchWorld(now time.Time) *World {
	w := NewWorld(now)
	w.Queue = "deathmatch"
	w.CoreGame = &LiveGame{ID: DMCoreGame, Queue: "deathmatch", Map: MapBind, Mode: ModeDeathmatch}
	w.CoreGame.Players = append(w.CoreGame.Players, LivePlayer{PUUID: w.Self, Team: w.Self, Agent: Jett})
	for i := 1; i <= 13; i++ {
		puuid := fmt.Sprintf("d%07d-0000-4000-8000-0000000000aa", i)
		p := w.Player(puuid)
		p.Name, p.Tag, p.Incognito = fmt.Sprintf("dm%d", i), "DM", i == 13
		p.SetRank("competitive", ActCurrent, SeasonInfo{Tier: 3 + i})
		w.CoreGame.Players = append(w.CoreGame.Players, LivePlayer{PUUID: puuid, Team: puuid, Agent: Phoenix})
	}
	return w
}

// StartAgentSelect is presence moving to PREGAME and the lifecycle event that comes with it
func (s *Server) StartAgentSelect(matchID string) {
	s.Update(func(w *World) { w.Loop = LoopPregame })
	s.EmitPregame(matchID, false)
	s.EmitPresence()
}

// Hover changes a player's agent in agent select
func (s *Server) Hover(puuid, agent string, locked bool) {
	var id string
	s.Update(func(w *World) {
		if w.Pregame == nil {
			return
		}
		id = w.Pregame.ID
		for i := range w.Pregame.Players {
			if w.Pregame.Players[i].PUUID == puuid {
				w.Pregame.Players[i].Agent = agent
				w.Pregame.Players[i].State = map[bool]string{false: "selected", true: "locked"}[locked]
			}
		}
	})
	if id != "" {
		s.EmitPregame(id, false)
	}
}

func (s *Server) Dodge() {
	s.Update(func(w *World) { w.Loop = LoopMenus })
	s.EmitPresence()
}

func (s *Server) LoadMatch(matchID string) {
	s.Update(func(w *World) { w.Loop = LoopIngame })
	s.EmitCoreGame(matchID, false)
	s.EmitPresence()
}

func (s *Server) EndMatch() {
	s.Update(func(w *World) { w.Loop = LoopMenus })
	s.EmitPresence()
}

func Scenarios() []Scenario {
	lobby := func(now time.Time) *World { return LobbyWorld(now, false) }
	out := []Scenario{
		{Name: "menus", About: "a Riot Client sitting in the menus", World: lobby},
		{Name: "full-flow", About: "menus, agent select with hovers, match load, back to the menus", World: lobby, Steps: []Step{
			{After: 3 * time.Second, Note: "agent select", Do: func(s *Server) { s.StartAgentSelect(LobbyPregame) }},
			{After: 2 * time.Second, Note: "hover", Do: func(s *Server) { s.Hover(LobbyAllies[0], Jett, false) }},
			{After: 2 * time.Second, Note: "lock", Do: func(s *Server) { s.Hover(LobbyAllies[0], Jett, true) }},
			{After: 3 * time.Second, Note: "match load", Do: func(s *Server) { s.LoadMatch(LobbyCoreGame) }},
			{After: 45 * time.Second, Note: "match over", Do: func(s *Server) { s.EndMatch() }},
		}},
		{Name: "dodge", About: "agent select that gets dodged", World: lobby, Steps: []Step{
			{After: 3 * time.Second, Note: "agent select", Do: func(s *Server) { s.StartAgentSelect(LobbyPregame) }},
			{After: 4 * time.Second, Note: "dodge", Do: func(s *Server) { s.Dodge() }},
		}},
		{Name: "deathmatch", About: "a deathmatch that skips agent select", World: DeathmatchWorld, Steps: []Step{
			{After: 3 * time.Second, Note: "match load", Do: func(s *Server) { s.LoadMatch(DMCoreGame) }},
			{After: 45 * time.Second, Note: "match over", Do: func(s *Server) { s.EndMatch() }},
		}},
		{Name: "restart", About: "the Riot Client restarting on a new port", World: lobby, Steps: []Step{
			{After: 5 * time.Second, Note: "restart", Do: func(s *Server) { s.Restart() }},
		}},
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func FindScenario(name string) (Scenario, bool) {
	for _, sc := range Scenarios() {
		if sc.Name == name {
			return sc, true
		}
	}
	return Scenario{}, false
}

// Play runs the steps in real time - for the fakeriot program, tests drive steps themselves
func (s *Server) Play(ctx context.Context, sc Scenario, logf func(format string, args ...any)) error {
	for _, step := range sc.Steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(step.After):
		}
		logf("%s: %s", sc.Name, step.Note)
		step.Do(s)
	}
	return nil
}
