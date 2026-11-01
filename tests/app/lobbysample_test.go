package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

type line5 struct{ score, rounds, kills, deaths, head, body, damage, firstBloods int }

// played gives a player n identical matches so the sample ratios are easy to work out by hand
func playedMatches(w *fakeriot.World, puuid, prefix string, n int, agent string, s line5) {
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%s-0000-4000-8000-%012d", prefix, i)
		m := &fakeriot.Match{
			ID: id, Queue: "competitive", Map: fakeriot.MapHaven, Mode: fakeriot.ModeBomb,
			Start: testStart.Add(-time.Duration(30+i) * time.Hour),
			Players: []fakeriot.MatchPlayer{
				{PUUID: puuid, Team: "Blue", Agent: agent, Score: s.score, Rounds: s.rounds, Kills: s.kills, Deaths: s.deaths},
				{PUUID: enemy, Team: "Red", Agent: fakeriot.Omen, Score: 1000, Rounds: s.rounds, Kills: 1, Deaths: 1},
			},
			Teams: []fakeriot.TeamResult{{ID: "Blue", Won: true}, {ID: "Red"}},
		}
		addRounds(m, puuid, s)
		w.AddMatch(m)
	}
}

// addRounds spreads first bloods over the first rounds and puts all damage in round one
func addRounds(m *fakeriot.Match, puuid string, s line5) {
	for r := 0; r < s.rounds; r++ {
		var round fakeriot.Round
		if r < s.firstBloods {
			round.Kills = []fakeriot.Kill{{Killer: puuid, Victim: enemy, At: 3000}}
		}
		if r == 0 && s.head+s.body > 0 {
			round.Damage = []fakeriot.Hit{{From: puuid, To: enemy, Damage: s.damage, Head: s.head, Body: s.body}}
		}
		m.Rounds = append(m.Rounds, round)
	}
}

// lobbySamples - KD/HS/ACS/ADR/FB of 2.00/30/200/150/10 for ally one and 0.50/20/150/100/5 for ally two
// (one match shared with ally one) - 2.50/50/250/200/20 and 0.50/10/100/80/0 for two enemies
func lobbySamples(w *fakeriot.World) {
	w.Player(ally2).Name, w.Player(ally2).Tag = "AllyTwo", "A2"
	w.Player(enemy1).Name, w.Player(enemy1).Tag = "EnemyOne", "E1"
	w.Player(enemy2).Name, w.Player(enemy2).Tag = "EnemyTwo", "E2"
	one := line5{score: 4000, rounds: 20, kills: 20, deaths: 10, head: 6, body: 14, damage: 3000, firstBloods: 2}
	two := line5{score: 3000, rounds: 20, kills: 10, deaths: 20, head: 2, body: 8, damage: 2000, firstBloods: 1}
	playedMatches(w, ally1, "a1a1a1a1", 4, fakeriot.Jett, one)
	playedMatches(w, ally2, "a2a2a2a2", 2, fakeriot.Killjoy, two)
	shared := &fakeriot.Match{
		ID: "5a5a5a5a-0000-4000-8000-000000000001", Queue: "competitive", Map: fakeriot.MapLotus, Mode: fakeriot.ModeBomb,
		Start: testStart.Add(-40 * time.Hour),
		Players: []fakeriot.MatchPlayer{
			{PUUID: ally1, Team: "Blue", Agent: fakeriot.Jett, Score: one.score, Rounds: one.rounds, Kills: one.kills, Deaths: one.deaths},
			{PUUID: ally2, Team: "Blue", Agent: fakeriot.Killjoy, Score: two.score, Rounds: two.rounds, Kills: two.kills, Deaths: two.deaths},
			{PUUID: enemy, Team: "Red", Agent: fakeriot.Omen, Score: 1000, Rounds: 20, Kills: 1, Deaths: 1},
		},
		Teams: []fakeriot.TeamResult{{ID: "Blue", Won: true}, {ID: "Red"}},
	}
	addRounds(shared, ally1, one)
	// ally one opens rounds 0 and 1 - ally two's first blood is round 2
	shared.Rounds[2].Kills = []fakeriot.Kill{{Killer: ally2, Victim: enemy, At: 3000}}
	shared.Rounds[0].Damage = append(shared.Rounds[0].Damage, fakeriot.Hit{From: ally2, To: enemy, Damage: two.damage, Head: two.head, Body: two.body})
	w.AddMatch(shared)
	playedMatches(w, enemy1, "e1e1e1e1", 2, fakeriot.Raze, line5{score: 5000, rounds: 20, kills: 25, deaths: 10, head: 10, body: 10, damage: 4000, firstBloods: 4})
	playedMatches(w, enemy2, "e2e2e2e2", 1, fakeriot.Breach, line5{score: 2000, rounds: 20, kills: 8, deaths: 16, head: 1, body: 9, damage: 1600})
}

func TestEveryVisiblePlayerGetsSampleStats(t *testing.T) {
	r := fromAgentSelect(t, lobbySamples)
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "AllyOne#A1", "EnemyTwo#E2", "US - THEM")

	type sample struct{ name, kd, hs, acs, adr, fb, n, mains string }
	rows := map[int]sample{
		// todaysGames - six of 15/15 with 4000 score over 20 rounds and no round data
		2:  {"Kestrel#EUW", "1.00", "-", "200", "0", "0%", "6", "Jett"},
		3:  {"AllyOne#A1", "2.00", "30%", "200", "150", "10%", "5", "Jett"},
		4:  {"AllyTwo#A2", "0.50", "20%", "150", "100", "5%", "3", "KJ"},
		6:  {"-", "-", "-", "-", "-", "-", "0", "-"},
		9:  {"EnemyOne#E1", "2.50", "50%", "250", "200", "20%", "2", "Raze"},
		10: {"EnemyTwo#E2", "0.50", "10%", "100", "80", "0%", "1", "Brch"},
		13: {"-", "-", "-", "-", "-", "-", "0", "-"},
	}
	for line, want := range rows {
		got := sample{
			cellIn(t, frame, "PLAYER", line), cellIn(t, frame, "K/D", line), cellIn(t, frame, "HS%", line),
			cellIn(t, frame, "ACS", line), cellIn(t, frame, "ADR", line), cellIn(t, frame, "FB%", line),
			cellIn(t, frame, "N", line), cellIn(t, frame, "MAINS", line),
		}
		if got != want {
			t.Errorf("line %d: got %+v, want %+v\n%s", line, got, want, frame)
		}
	}
	if got := cellIn(t, frame, "AGENT", 6); got != "Sage" {
		t.Errorf("a player with no matches is identified by agent, got %q", got)
	}

	// allies average 1.17/25/183/83/5 over three players - enemies 1.50/30/175/140/10 over two
	averages := []struct {
		line                 int
		kd, hs, acs, adr, fb string
	}{
		{7, "1.17", "25%", "183", "83", "5%"},
		{14, "1.50", "30%", "175", "140", "10%"},
		{15, "▼0.33", "▼5", "▲8", "▼57", "▼5"},
	}
	for _, a := range averages {
		got := [5]string{cellIn(t, frame, "K/D", a.line), cellIn(t, frame, "HS%", a.line), cellIn(t, frame, "ACS", a.line),
			cellIn(t, frame, "ADR", a.line), cellIn(t, frame, "FB%", a.line)}
		want := [5]string{a.kd, a.hs, a.acs, a.adr, a.fb}
		if got != want {
			t.Errorf("line %d: got %v, want %v\n%s", a.line, got, want, frame)
		}
	}
}

func TestLobbyRequestsStayWithinTheBudget(t *testing.T) {
	r := fromAgentSelect(t, lobbySamples)
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "AllyOne#A1", "EnemyTwo#E2", "US - THEM")

	if max := r.fr.MaxInFlight(); max > 2 {
		t.Errorf("%d requests were in flight at once", max)
	}
	perPath := map[string]int{}
	details := 0
	for _, req := range r.fr.RemoteRequests() {
		perPath[req.Path]++
		if strings.HasPrefix(req.Path, "/match-details/") {
			details++
		}
		if strings.HasPrefix(req.Path, "/match-history/") && req.Query.Get("queue") != "competitive" {
			t.Errorf("history asked for queue %q", req.Query.Get("queue"))
		}
	}
	for path, n := range perPath {
		if n > 1 && !strings.HasSuffix(path, "/competitiveupdates") && !strings.HasPrefix(path, "/pregame/") && !strings.HasPrefix(path, "/name-service/") {
			t.Errorf("%s fetched %d times", path, n)
		}
	}
	// you 6 - ally one 4 plus the shared match - ally two 2 - enemies 2 and 1
	if details != 16 {
		t.Errorf("fetched %d match details, want 16 with the premade's shared match counted once", details)
	}
	// two rosters - your party - content - two session refreshes - the match's loadouts - two name lookups - then
	// rank and trend and history per player
	players := 10
	budget := 2 + 2 + 1 + 2 + 1 + 2 + players*3 + details
	if n := len(r.fr.RemoteRequests()); n > budget {
		t.Errorf("%d requests for the lobby, over the budget of %d", n, budget)
	}
}

func TestTheCutoffLeavesHonestPartialSamples(t *testing.T) {
	r := fromAgentSelect(t, lobbySamples)
	older := "/match-details/v1/matches/e1e1e1e1-0000-4000-8000-000000000001"
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: older, Hold: true})
	r.enterMatchLoad()
	r.screen("MATCH LOADING", "EnemyOne#E1", "EnemyTwo#E2")
	waitHeld(t, r.fr, 1)
	r.clock.Advance(40 * time.Second)
	frame := r.screen("IN MATCH")
	if got := cellIn(t, frame, "N", 9); got != "1" {
		t.Errorf("enemy one's n: got %q, want the 1 match that arrived\n%s", got, frame)
	}
	if got := cellIn(t, frame, "K/D", 9); got != "2.50" {
		t.Errorf("enemy one's K/D from the match that arrived: got %q", got)
	}
}

func TestMatchDetailsGoRoundRobinNewestFirst(t *testing.T) {
	// no session matches and no premades so the order is only about the round robin
	r := newRig(t, matchLobby, func(w *fakeriot.World) {
		w.Matches = map[string]*fakeriot.Match{}
		w.Player(w.Self).Updates = nil
		threeCompMatches(w)
		playedMatches(w, ally1, "a1a1a1a1", 4, fakeriot.Jett, line5{score: 4000, rounds: 20, kills: 20, deaths: 10})
		playedMatches(w, ally2, "a2a2a2a2", 2, fakeriot.Killjoy, line5{score: 3000, rounds: 20, kills: 10, deaths: 20})
	})
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/match-history/", Hold: true})
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/match-details/", Hold: true})
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()

	// one release frees one slot - waiting for the next request to park makes the order exact
	release := func(path string, thenHeld int) {
		t.Helper()
		if !r.fr.Release(path) {
			t.Fatalf("%s wasn't held", path)
		}
		eventually(t, fmt.Sprintf("%d held requests", thenHeld), func() bool { return len(r.fr.Held()) == thenHeld })
	}
	waitHeld(t, r.fr, 2)
	// histories finish in roster order - that's the order players join the rotation
	for _, p := range []string{fakeriot.DefaultSelf, ally1, ally2, ally3, ally4} {
		release("/match-history/v1/history/"+p, 2)
	}
	for unreleased := 9; unreleased > 0; unreleased-- {
		release(r.fr.Held()[0].Path, min(2, unreleased-1))
	}
	r.settled("AGENT SELECT", "AllyOne")

	var got []string
	for _, req := range r.fr.RemoteRequests() {
		if id, ok := strings.CutPrefix(req.Path, "/match-details/v1/matches/"); ok {
			got = append(got, id[:2]+id[len(id)-1:])
		}
	}
	want := "a0a a10 a20 b0b a11 a21 c0c a12 a13"
	if strings.Join(got, " ") != want {
		t.Errorf("match details went %v, want %s", got, want)
	}

	lastRank, firstHistory := -1, -1
	for i, req := range r.fr.RemoteRequests() {
		if strings.HasPrefix(req.Path, "/mmr/v1/players/") && !strings.HasSuffix(req.Path, "/competitiveupdates") {
			lastRank = i
		}
		if strings.HasPrefix(req.Path, "/match-history/") && firstHistory < 0 {
			firstHistory = i
		}
	}
	if firstHistory < lastRank {
		t.Errorf("a match history went out before every rank lookup")
	}
}
