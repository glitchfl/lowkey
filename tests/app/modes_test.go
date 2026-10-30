package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// modeMatches gives a player n identical matches in a queue
func modeMatches(w *fakeriot.World, puuid, queue, mode, prefix string, n int, s line5) {
	for i := 0; i < n; i++ {
		m := &fakeriot.Match{
			ID: fmt.Sprintf("%s-0000-4000-8000-%012d", prefix, i), Queue: queue, Map: fakeriot.MapBind, Mode: mode,
			Start: testStart.Add(-time.Duration(2+i) * time.Hour),
			Players: []fakeriot.MatchPlayer{
				{PUUID: puuid, Team: puuid, Agent: fakeriot.Reyna, Score: s.score, Rounds: s.rounds, Kills: s.kills, Deaths: s.deaths},
			},
		}
		addRounds(m, puuid, s)
		w.AddMatch(m)
	}
}

const dmMatch = "d9000000-0000-4000-8000-000000000001"

func dmPlayer(i int) string { return fmt.Sprintf("d%07d-0000-4000-8000-000000000000", i) }

// deathmatch - you and 13 others each on a team of one - player 13 is in streamer mode
func deathmatch(w *fakeriot.World) {
	me := w.Player(w.Self)
	me.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 20, RR: 12})
	me.SetRank("deathmatch", fakeriot.ActCurrent, fakeriot.SeasonInfo{Wins: 3, WinsWithPlacements: 3, Games: 9})
	modeMatches(w, w.Self, "deathmatch", fakeriot.ModeDeathmatch, "dddd0000", 2, line5{score: 1500, rounds: 1, kills: 30, deaths: 20, head: 20, body: 30, damage: 4000})
	w.Queue = "deathmatch"
	w.CoreGame = &fakeriot.LiveGame{ID: dmMatch, Queue: "deathmatch", Map: fakeriot.MapBind, Mode: fakeriot.ModeDeathmatch}
	w.CoreGame.Players = append(w.CoreGame.Players, fakeriot.LivePlayer{PUUID: w.Self, Team: w.Self, Agent: fakeriot.Jett})
	for i := 1; i <= 13; i++ {
		p := w.Player(dmPlayer(i))
		p.Name, p.Tag = fmt.Sprintf("dm%d", i), "DM"
		if i <= 2 {
			p.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 14 + i*2})
			modeMatches(w, p.PUUID, "deathmatch", fakeriot.ModeDeathmatch, fmt.Sprintf("dd%06d", i), 1,
				line5{score: 1000, rounds: 1, kills: 20 * i, deaths: 20, head: 10, body: 10, damage: 2000})
		}
		p.Incognito = i == 13
		w.CoreGame.Players = append(w.CoreGame.Players, fakeriot.LivePlayer{PUUID: p.PUUID, Team: p.PUUID, Agent: fakeriot.Phoenix})
	}
}

func TestDeathmatchIsOneListWithALobbyAverage(t *testing.T) {
	r := newRig(t, deathmatch)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopIngame })
	r.fr.EmitCoreGame(dmMatch, false)
	frame := r.settled("MATCH LOADING · Bind · Deathmatch", "dm1#DM", "dm2#DM", "AVERAGE")

	for _, word := range []string{"ALLIES", "ENEMIES", "US - THEM"} {
		if strings.Contains(frame, word) {
			t.Errorf("deathmatch shows %q:\n%s", word, frame)
		}
	}
	// header line then 14 players and the lobby average
	if got := cellIn(t, frame, "PLAYER", 15); !strings.HasPrefix(got, "AVERAGE") {
		t.Errorf("want the lobby average after all 14 players, got %q\n%s", got, frame)
	}
	for row := 1; row <= 15; row++ {
		for _, col := range []string{"WR", "ACS", "ADR", "FB%"} {
			if got := cellIn(t, frame, col, row); got != "-" && got != "" {
				t.Errorf("line %d %s: got %q, want - in deathmatch\n%s", row, col, got, frame)
			}
		}
	}
	// you: 60 kills / 40 deaths and 40 of 100 hits over two matches
	if kd, hs := cellIn(t, frame, "K/D", 1), cellIn(t, frame, "HS%", 1); kd != "1.50" || hs != "40%" {
		t.Errorf("your K/D and HS%%: got %s %s, want 1.50 40%%", kd, hs)
	}
	if got := cellIn(t, frame, "PLAYER", 14); got != "hidden" {
		t.Errorf("the streamer-mode player shows %q", got)
	}
}

func TestTDMHasTeamsButNoRoundStats(t *testing.T) {
	r := fromAgentSelect(t, lobbySamples, func(w *fakeriot.World) {
		w.Queue, w.Pregame.Queue, w.CoreGame.Queue = "hurm", "hurm", "hurm"
		w.Pregame.Mode, w.CoreGame.Mode = fakeriot.ModeTDM, fakeriot.ModeTDM
		for _, m := range w.Matches {
			m.Queue = "hurm"
		}
		w.Player(ally1).SetRank("hurm", fakeriot.ActCurrent, fakeriot.SeasonInfo{Wins: 3, WinsWithPlacements: 3, Games: 4})
	})
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING · Ascent · TDM", "ALLIES", "ENEMIES", "US - THEM", "AllyOne#A1")
	if got := cellIn(t, frame, "WR", 3); got != "75%" {
		t.Errorf("ally one's TDM win rate: got %q, want 75%%", got)
	}
	if got := cellIn(t, frame, "K/D", 3); got != "2.00" {
		t.Errorf("ally one's TDM K/D: got %q", got)
	}
	for row := 1; row <= 15; row++ {
		for _, col := range []string{"ACS", "ADR", "FB%"} {
			if got := cellIn(t, frame, col, row); got != "-" && got != "" {
				t.Errorf("line %d %s: got %q, want - in TDM\n%s", row, col, got, frame)
			}
		}
	}
	for _, req := range r.fr.RemoteRequests() {
		if strings.HasPrefix(req.Path, "/match-history/") && req.Query.Get("queue") != "hurm" {
			t.Errorf("TDM history asked for queue %q", req.Query.Get("queue"))
		}
	}
}

func TestPremierAndSwiftplayFollowTheirOwnQueues(t *testing.T) {
	for _, q := range []struct{ queue, name, prefix string }{{"premier", "Premier", "a1a10001"}, {"swiftplay", "Swiftplay", "a1a10002"}} {
		t.Run(q.name, func(t *testing.T) {
			r := newRig(t, agentSelectLobby, func(w *fakeriot.World) {
				w.Queue, w.Pregame.Queue = q.queue, q.queue
				w.Player(ally1).SetRank(q.queue, fakeriot.ActCurrent, fakeriot.SeasonInfo{Wins: 1, WinsWithPlacements: 1, Games: 4})
				modeMatches(w, ally1, q.queue, fakeriot.ModeBomb, q.prefix, 2, line5{score: 3000, rounds: 15, kills: 12, deaths: 8})
			})
			r.live()
			r.screen("waiting for the next match")
			r.subscribed()
			r.enterAgentSelect()
			frame := r.settled("AGENT SELECT · Ascent · Defenders · "+q.name, "AllyOne#A1")
			if got := cellIn(t, frame, "WR", 2); got != "25%" {
				t.Errorf("ally one's %s win rate: got %q, want 25%%", q.name, got)
			}
			if got := cellIn(t, frame, "W-L", 2); got != "1-3" {
				t.Errorf("ally one's %s W-L: got %q", q.name, got)
			}
			if n, acs := cellIn(t, frame, "N", 2), cellIn(t, frame, "ACS", 2); n != "2" || acs != "200" {
				t.Errorf("ally one's %s sample: n %q ACS %q, want 2 and 200", q.name, n, acs)
			}
			for _, req := range r.fr.RemoteRequests() {
				if strings.HasPrefix(req.Path, "/match-history/") && req.Query.Get("queue") != q.queue {
					t.Errorf("history asked for queue %q", req.Query.Get("queue"))
				}
			}
		})
	}
}

func TestMePrintoutUsesTheSameModeRules(t *testing.T) {
	r := newRig(t, deathmatch)
	out, code := r.run("me", "--mode", "deathmatch")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	for _, col := range []string{"WR", "W-L", "ACS", "ADR", "FB%"} {
		if got := cell(t, out, col); got != "-" {
			t.Errorf("%s: got %q, want - in deathmatch\n%s", col, got, out)
		}
	}
	if got := cell(t, out, "K/D"); got != "1.50" {
		t.Errorf("K/D: got %q", got)
	}

	r = newRig(t, deathmatch, func(w *fakeriot.World) {
		for _, m := range w.Matches {
			m.Queue = "hurm"
		}
		w.Player(w.Self).SetRank("hurm", fakeriot.ActCurrent, fakeriot.SeasonInfo{Wins: 2, WinsWithPlacements: 2, Games: 5})
	})
	out, code = r.run("me", "--mode", "team_deathmatch")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	if got := cell(t, out, "WR"); got != "40%" {
		t.Errorf("TDM WR: got %q, want 40%%", got)
	}
	for _, col := range []string{"ACS", "ADR", "FB%"} {
		if got := cell(t, out, col); got != "-" {
			t.Errorf("%s: got %q, want - in TDM", col, got)
		}
	}
}
