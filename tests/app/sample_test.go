package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const enemy = "e0000000-0000-4000-8000-000000000001"

type played struct {
	id                         string
	queue                      string
	ago                        time.Duration
	agent                      string
	score, rounds, kills, dies int
	head, body, leg, damage    int
	firstBloods, lateKills     int
	name, tag                  string
}

// match builds a match where you make firstBloods opening kills and lateKills kills after an enemy opened
func match(w *fakeriot.World, p played) *fakeriot.Match {
	m := &fakeriot.Match{
		ID: p.id, Queue: p.queue, Map: fakeriot.MapAscent, Mode: fakeriot.ModeBomb, Start: testStart.Add(-p.ago),
		Players: []fakeriot.MatchPlayer{
			{PUUID: w.Self, Name: p.name, Tag: p.tag, Team: "Blue", Agent: p.agent, Score: p.score, Rounds: p.rounds, Kills: p.kills, Deaths: p.dies},
			{PUUID: enemy, Team: "Red", Agent: fakeriot.Omen, Score: 3000, Rounds: p.rounds, Kills: 15, Deaths: 15},
		},
		Teams: []fakeriot.TeamResult{{ID: "Blue", Won: true, RoundsWon: 13}, {ID: "Red", RoundsWon: 7}},
	}
	for i := 0; i < p.rounds; i++ {
		var r fakeriot.Round
		switch {
		case i < p.firstBloods:
			r.Kills = []fakeriot.Kill{{Killer: w.Self, Victim: enemy, At: 5000}, {Killer: enemy, Victim: w.Self, At: 20000}}
		case i < p.firstBloods+p.lateKills:
			r.Kills = []fakeriot.Kill{{Killer: enemy, Victim: w.Self, At: 8000}, {Killer: w.Self, Victim: enemy, At: 15000}}
		}
		if i == 0 {
			r.Damage = []fakeriot.Hit{
				{From: w.Self, To: enemy, Damage: p.damage, Head: p.head, Body: p.body, Leg: p.leg},
				// damage you took never counts toward your HS% or ADR
				{From: enemy, To: w.Self, Damage: 900, Head: 40, Body: 1, Leg: 1},
			}
		}
		m.Rounds = append(m.Rounds, r)
	}
	w.AddMatch(m)
	return m
}

func threeCompMatches(w *fakeriot.World) {
	match(w, played{id: "a0000000-0000-4000-8000-00000000000a", queue: "competitive", ago: time.Hour, agent: fakeriot.Jett,
		score: 4800, rounds: 20, kills: 20, dies: 15, head: 10, body: 30, leg: 5, damage: 2900, firstBloods: 4, lateKills: 3,
		name: "Kestrel", tag: "EUW"})
	match(w, played{id: "b0000000-0000-4000-8000-00000000000b", queue: "competitive", ago: 2 * time.Hour, agent: fakeriot.Raze,
		score: 3000, rounds: 15, kills: 10, dies: 12, head: 4, body: 20, leg: 4, damage: 1700, firstBloods: 1, lateKills: 2,
		name: "OldName", tag: "OLD"})
	match(w, played{id: "c0000000-0000-4000-8000-00000000000c", queue: "competitive", ago: 3 * time.Hour, agent: fakeriot.Jett,
		score: 5200, rounds: 25, kills: 22, dies: 18, head: 12, body: 40, leg: 8, damage: 3400, firstBloods: 4,
		name: "OldName", tag: "OLD"})
	match(w, played{id: "d0000000-0000-4000-8000-00000000000d", queue: "unrated", ago: 30 * time.Minute, agent: fakeriot.Sage,
		score: 9999, rounds: 13, kills: 40, dies: 1, head: 50, body: 1, leg: 0, damage: 5000, firstBloods: 13,
		name: "Kestrel", tag: "EUW"})
}

func TestMeShowsYourSampleStats(t *testing.T) {
	r := newRig(t, ranked, threeCompMatches)
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	// 52 kills / 45 deaths - 26 of 133 hits - 13000 score and 8000 damage over 60 rounds - 9 first bloods
	want := map[string]string{
		"PLAYER": "Kestrel#EUW", "K/D": "1.16", "HS%": "20%", "ACS": "217", "ADR": "133", "FB%": "15%",
		"N": "3", "MAINS": "Jett Raze",
	}
	for col, v := range want {
		if got := cell(t, out, col); got != v {
			t.Errorf("%s: got %q, want %q\n%s", col, got, v, out)
		}
	}
}

func TestMeSampleFollowsTheChosenModeAndCapsAtThirteen(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) {
		for i := 0; i < 15; i++ {
			match(w, played{id: fmt.Sprintf("f0000000-0000-4000-8000-%012d", i), queue: "swiftplay", ago: time.Duration(i+1) * time.Hour,
				agent: fakeriot.Sova, score: 1000, rounds: 5, kills: 5, dies: 5, head: 1, body: 1, leg: 0, damage: 500})
		}
		threeCompMatches(w)
	})
	out, code := r.run("me", "--mode", "swiftplay")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	if got := cell(t, out, "N"); got != "13" {
		t.Errorf("n: got %q, want 13", got)
	}
	if got := cell(t, out, "MAINS"); got != "Sova" {
		t.Errorf("mains: got %q, want Sova", got)
	}
	for _, req := range r.fr.RemoteRequests() {
		if strings.HasPrefix(req.Path, "/match-history/") && req.Query.Get("queue") != "swiftplay" {
			t.Errorf("history asked for queue %q", req.Query.Get("queue"))
		}
	}
}

func TestMeFetchesHistoryThenDetailsNewestFirstOnceEach(t *testing.T) {
	r := newRig(t, ranked, threeCompMatches)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/match-details/", Hold: true})
	done := r.start("me")
	const a, b, c = "/match-details/v1/matches/a0000000-0000-4000-8000-00000000000a",
		"/match-details/v1/matches/b0000000-0000-4000-8000-00000000000b",
		"/match-details/v1/matches/c0000000-0000-4000-8000-00000000000c"
	if got := waitHeld(t, r.fr, 2); got[0] != a || got[1] != b {
		t.Fatalf("first two match details were %v, want the two newest", got)
	}
	r.fr.Release(a)
	if got := waitHeld(t, r.fr, 2); got[0] != b || got[1] != c {
		t.Fatalf("after the newest came back, in flight were %v, want the next newest", got)
	}
	r.fr.ReleaseHeld()
	if code := <-done; code != 0 {
		t.Fatalf("me failed:\n%s", r.con.output())
	}

	historyAt, firstDetailAt := -1, -1
	seen := map[string]int{}
	for i, req := range r.fr.RemoteRequests() {
		switch {
		case strings.HasPrefix(req.Path, "/match-history/"):
			historyAt = i
		case strings.HasPrefix(req.Path, "/match-details/"):
			if firstDetailAt < 0 {
				firstDetailAt = i
			}
			seen[req.Path]++
		case strings.Contains(req.Path, "name-service"):
			t.Errorf("called the name service: %s", req.Path)
		}
	}
	if historyAt < 0 || firstDetailAt < historyAt {
		t.Errorf("match history must come before match details")
	}
	for path, n := range seen {
		if n > 1 {
			t.Errorf("%s fetched %d times", path, n)
		}
	}
}

func TestPlayersWithoutMatchesInTheModeShowDashes(t *testing.T) {
	r := newRig(t, ranked)
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	for _, col := range []string{"PLAYER", "K/D", "HS%", "ACS", "ADR", "FB%", "MAINS"} {
		if got := cell(t, out, col); got != "-" {
			t.Errorf("%s: got %q, want -\n%s", col, got, out)
		}
	}
	if got := cell(t, out, "N"); got != "0" {
		t.Errorf("n: got %q, want 0", got)
	}
}
