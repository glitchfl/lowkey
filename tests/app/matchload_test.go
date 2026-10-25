package app_test

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const (
	enemy1   = "e1000000-0000-4000-8000-000000000001"
	enemy2   = "e2000000-0000-4000-8000-000000000002"
	enemy3   = "e3000000-0000-4000-8000-000000000003"
	enemy4   = "e4000000-0000-4000-8000-000000000004"
	enemy5   = "e5000000-0000-4000-8000-000000000005"
	coreGame = "c9000000-0000-4000-8000-000000000001"
)

// matchLobby is agentSelectLobby plus the enemy team - one of them in streamer mode
func matchLobby(w *fakeriot.World) {
	agentSelectLobby(w)
	e1 := w.Player(enemy1)
	e1.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 16, RR: 10, Wins: 6, WinsWithPlacements: 6, Games: 10})
	e1.Updates = []fakeriot.Update{{MatchID: "y1", Earned: 12}}
	e2 := w.Player(enemy2)
	e2.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 14, RR: 70, Wins: 5, WinsWithPlacements: 5, Games: 10})
	e2.Updates = []fakeriot.Update{{MatchID: "y2", Earned: -6}}
	e3 := w.Player(enemy3)
	e3.Incognito = true
	e3.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 27, RR: 999, Wins: 99, Games: 100})
	e4 := w.Player(enemy4)
	e4.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 17, Wins: 7, WinsWithPlacements: 7, Games: 10})
	e4.Updates = []fakeriot.Update{{MatchID: "y4", Earned: 0}}
	w.Player(enemy5)
	w.CoreGame = &fakeriot.LiveGame{
		ID: coreGame, Queue: "competitive", Map: fakeriot.MapAscent, Mode: fakeriot.ModeBomb, Ranked: true,
		Players: []fakeriot.LivePlayer{
			{PUUID: enemy1, Team: "Red", Agent: fakeriot.Raze},
			{PUUID: ally1, Team: "Blue", Agent: fakeriot.Jett},
			{PUUID: w.Self, Team: "Blue", Agent: fakeriot.Sova},
			{PUUID: enemy2, Team: "Red", Agent: fakeriot.Breach},
			{PUUID: ally2, Team: "Blue", Agent: fakeriot.Killjoy},
			{PUUID: enemy3, Team: "Red", Agent: fakeriot.Viper},
			{PUUID: ally3, Team: "Blue", Agent: fakeriot.Omen},
			{PUUID: enemy4, Team: "Red", Agent: fakeriot.Cypher},
			{PUUID: ally4, Team: "Blue", Agent: fakeriot.Sage},
			{PUUID: enemy5, Team: "Red", Agent: fakeriot.Skye},
		},
	}
}

func (r *rig) enterMatchLoad() {
	r.t.Helper()
	r.fr.Update(func(w *fakeriot.World) { w.Loop, w.Pregame = fakeriot.LoopIngame, nil })
	r.fr.EmitCoreGame(coreGame, false)
}

// fromAgentSelect walks through agent select into match load and waits for the full table
func fromAgentSelect(t *testing.T, setup ...func(w *fakeriot.World)) *rig {
	r := newRig(t, append([]func(w *fakeriot.World){matchLobby}, setup...)...)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.screen("AGENT SELECT", "Dia1 50", "Plat1 30", "SESSION")
	return r
}

func lineIndex(frame, prefix string) int {
	for i, l := range strings.Split(frame, "\n") {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

func TestMatchLoadShowsBothTeamsWithAveragesAndADiff(t *testing.T) {
	r := fromAgentSelect(t)
	allyLookups := requestsExact(r, "/mmr/v1/players/"+ally1)
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING · Ascent · Competitive", "Plat2 10", "Gold3 70", "Plat3 0", "▼4", "▲1")

	if n := requestsExact(r, "/mmr/v1/players/"+ally1); n != allyLookups {
		t.Errorf("ally rank data fetched again at match load (%d times)", n)
	}
	for _, e := range []string{enemy1, enemy2, enemy4, enemy5} {
		if requestsExact(r, "/mmr/v1/players/"+e) != 1 {
			t.Errorf("enemy %s wasn't looked up exactly once", e)
		}
	}
	allies, enemies := lineIndex(frame, "ALLIES"), lineIndex(frame, "ENEMIES")
	if allies < 0 || enemies < allies {
		t.Fatalf("want an ALLIES section before ENEMIES:\n%s", frame)
	}
	// under the header come ALLIES - 5 allies - the average - ENEMIES - then the enemies from the 9th line
	// none of the enemies has matches in this fixture so names show "-"
	enemyRows := []struct{ agent, name, rank string }{
		{"Raze", "-", "Plat2 10"}, {"Breach", "-", "Gold3 70"}, {"Viper", "hidden", ""}, {"Cypher", "-", "Plat3 0"}, {"Skye", "-", "-"},
	}
	for i, want := range enemyRows {
		n := 9 + i
		got := struct{ agent, name, rank string }{cellIn(t, frame, "AGENT", n), cellIn(t, frame, "PLAYER", n), cellIn(t, frame, "RANK", n)}
		if got != want {
			t.Errorf("enemy row %d: got %+v, want %+v\n%s", i+1, got, want, frame)
		}
	}
	checks := []struct {
		row       int
		col, want string
	}{
		{7, "PLAYER", "AVERAGE 4/5"}, {7, "RANK", "Dia1"}, {7, "WR", "56%"}, {7, "TREND", "+3"},
		{14, "PLAYER", "AVERAGE 4/5"}, {14, "RANK", "Plat2"}, {14, "PEAK", "Plat2"}, {14, "WR", "60%"}, {14, "TREND", "+2"},
		{15, "PLAYER", "US - THEM"}, {15, "RANK", "▲2"}, {15, "PEAK", "▲3"}, {15, "WR", "▼4"}, {15, "TREND", "▲1"},
	}
	for _, c := range checks {
		if got := cellIn(t, frame, c.col, c.row); got != c.want {
			t.Errorf("line %d %s: got %q, want %q\n%s", c.row, c.col, got, c.want, frame)
		}
	}
	for _, req := range r.fr.RemoteRequests() {
		if strings.Contains(req.Path, enemy3) {
			t.Errorf("looked up the hidden enemy: %s", req.Path)
		}
	}
}

func TestNothingRevealsTheStartingSide(t *testing.T) {
	for _, team := range []string{"Blue", "Red"} {
		t.Run("you on "+team, func(t *testing.T) {
			r := newRig(t, matchLobby, func(w *fakeriot.World) {
				if team == "Blue" {
					return
				}
				for i := range w.CoreGame.Players {
					p := &w.CoreGame.Players[i]
					p.Team = map[string]string{"Blue": "Red", "Red": "Blue"}[p.Team]
				}
			})
			r.live()
			r.screen("waiting for the next match")
			r.subscribed()
			r.enterMatchLoad()
			frame := r.screen("MATCH LOADING", "Plat2 10", "Dia3 12")
			if a, e := lineIndex(frame, "ALLIES"), lineIndex(frame, "ENEMIES"); a < 0 || e < a {
				t.Errorf("allies must come first whatever side you start on:\n%s", frame)
			}
			low := strings.ToLower(frame)
			for _, word := range []string{"attack", "defend", "atk", "def ", "red", "blue"} {
				if strings.Contains(low, word) {
					t.Errorf("the screen says %q:\n%s", word, frame)
				}
			}
		})
	}
}

func TestTheCutoffStopsEveryLookup(t *testing.T) {
	r := fromAgentSelect(t)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/" + enemy1, Hold: true})
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/" + enemy2, Hold: true})
	r.enterMatchLoad()
	r.screen("MATCH LOADING")
	waitHeld(t, r.fr, 2)

	r.clock.Advance(40 * time.Second)
	r.screen("IN MATCH · Ascent · Competitive")
	eventually(t, "held lookups to be cancelled", func() bool { return len(r.fr.Held()) == 0 })
	before := len(r.fr.RemoteRequests())
	r.fr.ReleaseHeld()
	r.clock.Advance(20 * time.Minute)
	settle()
	if after := len(r.fr.RemoteRequests()); after != before {
		t.Errorf("%d requests after the cutoff", after-before)
	}
}

func TestBackInTheMenusShowsTheLastMatchAndRefreshesTheSession(t *testing.T) {
	r := fromAgentSelect(t)
	r.enterMatchLoad()
	// everything has to land before the cutoff cancels what's left - the session refresh included
	r.settled("MATCH LOADING", "Plat2 10", "US - THEM")
	r.clock.Advance(40 * time.Second)
	r.screen("IN MATCH")

	const played = "10000000-0000-4000-8000-000000000099"
	r.clock.Advance(35 * time.Minute)
	r.fr.Update(func(w *fakeriot.World) {
		w.CoreGame = nil
		compGame(w, played, at(25, 20, 0), 21, win)
		me := w.Player(w.Self)
		me.Updates = append([]fakeriot.Update{me.Updates[len(me.Updates)-1]}, me.Updates[:len(me.Updates)-1]...)
	})
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	frame := r.screen("LAST MATCH · Ascent · Competitive", "waiting for the next match", "Plat2 10")
	if !strings.Contains(frame, "SESSION · 2W 1L +26 RR") {
		t.Errorf("the session line should stay until the refresh:\n%s", frame)
	}

	before := len(r.fr.RemoteRequests())
	r.clock.Advance(14 * time.Second)
	settle()
	if n := len(r.fr.RemoteRequests()); n != before {
		t.Fatalf("the session refreshed %d requests early", n-before)
	}
	r.clock.Advance(time.Second)
	r.screen("SESSION · 3W 1L +47 RR")
	var got []string
	for _, req := range r.fr.RemoteRequests()[before:] {
		got = append(got, req.Path)
	}
	sort.Strings(got)
	want := []string{"/match-details/v1/matches/" + played, "/mmr/v1/players/" + fakeriot.DefaultSelf + "/competitiveupdates"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the refresh asked for %v, want only %v", got, want)
	}
}

func TestAMatchWithoutAgentSelectLooksEveryoneUp(t *testing.T) {
	r := newRig(t, matchLobby)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "Dia3 12", "Dia1 50", "Plat2 10", "Gold3 70")
	for _, p := range []string{fakeriot.DefaultSelf, ally1, ally2, ally4, enemy1, enemy2, enemy4, enemy5} {
		if requestsExact(r, "/mmr/v1/players/"+p) != 1 {
			t.Errorf("%s wasn't looked up once", p)
		}
	}
}

func TestUntrackedModesDoNothingAtMatchLoad(t *testing.T) {
	r := newRig(t, matchLobby, func(w *fakeriot.World) { w.Queue, w.CoreGame.Queue = "spikerush", "spikerush" })
	r.live()
	r.screen("waiting for the next match", "SESSION")
	r.subscribed()
	before := len(r.fr.RemoteRequests())
	r.enterMatchLoad()
	r.screen("not tracking spikerush")
	r.clock.Advance(time.Minute)
	settle()
	if n := len(r.fr.RemoteRequests()); n != before {
		t.Errorf("%d requests at an untracked match load", n-before)
	}
}
