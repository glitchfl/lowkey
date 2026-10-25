package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// marker is the party column - the first rune of the nth line under the column header
func marker(t *testing.T, frame string, nth int) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	for i, l := range lines {
		if headerIndex([]rune(l), []rune("PLAYER")) >= 0 && !strings.Contains(l, "·") && i+nth < len(lines) {
			row := []rune(lines[i+nth])
			if len(row) == 0 || row[0] == ' ' {
				return ""
			}
			if headerIndex([]rune(l), []rune("PLAYER")) == 0 {
				return ""
			}
			return string(row[0])
		}
	}
	t.Fatalf("no table in:\n%s", frame)
	return ""
}

// together adds a match where every listed player was on the same team
func together(w *fakeriot.World, id string, ago time.Duration, team string, puuids ...string) {
	m := &fakeriot.Match{ID: id, Queue: "competitive", Map: fakeriot.MapAscent, Mode: fakeriot.ModeBomb, Start: testStart.Add(-ago),
		Teams: []fakeriot.TeamResult{{ID: team, Won: true}}}
	for _, p := range puuids {
		m.Players = append(m.Players, fakeriot.MatchPlayer{PUUID: p, Team: team, Agent: fakeriot.Sova, Score: 3000, Rounds: 20, Kills: 15, Deaths: 15})
	}
	w.AddMatch(m)
}

func matchID(n int) string { return fmt.Sprintf("bbbb0000-0000-4000-8000-%012d", n) }

func TestYourOwnPartyIsMarkedExactly(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) { w.PartyMembers = []string{w.Self, ally1, ally3} })
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "Dia1 50")
	want := []string{"●", "●", "", "", ""}
	for i, w := range want {
		if got := marker(t, frame, i+1); got != w {
			t.Errorf("row %d marker: got %q, want %q\n%s", i+1, got, w, frame)
		}
	}
	var party []string
	for _, req := range r.fr.RemoteRequests() {
		if strings.HasPrefix(req.Path, "/parties/") {
			party = append(party, req.Path)
		}
	}
	want2 := []string{"/parties/v1/players/" + fakeriot.DefaultSelf, "/parties/v1/parties/9a870000-0000-4000-8000-000000000001"}
	if strings.Join(party, " ") != strings.Join(want2, " ") {
		t.Errorf("party requests: got %v, want %v", party, want2)
	}
}

// likelyParties - allies one and two queued together twice and so did enemies one and two and enemies two and four
// enemy five shared one match with enemy one and two with the hidden enemy three
func likelyParties(w *fakeriot.World) {
	together(w, matchID(1), 30*time.Hour, "Blue", ally1, ally2)
	together(w, matchID(2), 31*time.Hour, "Red", ally1, ally2)
	together(w, matchID(3), 32*time.Hour, "Red", enemy1, enemy2)
	together(w, matchID(4), 33*time.Hour, "Blue", enemy1, enemy2)
	together(w, matchID(5), 34*time.Hour, "Red", enemy2, enemy4)
	together(w, matchID(6), 35*time.Hour, "Red", enemy2, enemy4)
	together(w, matchID(7), 36*time.Hour, "Red", enemy1, enemy5)
	together(w, matchID(8), 37*time.Hour, "Red", enemy3, enemy5)
	together(w, matchID(9), 38*time.Hour, "Blue", enemy3, enemy5)
	// on opposite teams twice - opponents aren't a party
	w.AddMatch(&fakeriot.Match{ID: matchID(10), Queue: "competitive", Map: fakeriot.MapAscent, Mode: fakeriot.ModeBomb, Start: testStart.Add(-39 * time.Hour),
		Players: []fakeriot.MatchPlayer{{PUUID: ally4, Team: "Blue", Agent: fakeriot.Sage, Rounds: 20}, {PUUID: ally1, Team: "Red", Agent: fakeriot.Jett, Rounds: 20}}})
	w.AddMatch(&fakeriot.Match{ID: matchID(11), Queue: "competitive", Map: fakeriot.MapAscent, Mode: fakeriot.ModeBomb, Start: testStart.Add(-40 * time.Hour),
		Players: []fakeriot.MatchPlayer{{PUUID: ally4, Team: "Blue", Agent: fakeriot.Sage, Rounds: 20}, {PUUID: ally1, Team: "Red", Agent: fakeriot.Jett, Rounds: 20}}})
}

func TestLikelyPartiesShareAGroupSymbol(t *testing.T) {
	r := fromAgentSelect(t, likelyParties)
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "US - THEM", "Plat2 10")
	m := func(line int) string { return marker(t, frame, line) }

	allyGroup := m(3)
	if allyGroup == "" || allyGroup == "●" || m(4) != allyGroup {
		t.Errorf("allies one and two should share a likely-party symbol, got %q and %q\n%s", m(3), m(4), frame)
	}
	if m(2) != "" || m(6) != "" {
		t.Errorf("you (solo) and ally four (only ever an opponent) shouldn't be marked, got %q and %q", m(2), m(6))
	}
	enemyGroup := m(9)
	if enemyGroup == "" || enemyGroup == allyGroup || m(10) != enemyGroup || m(12) != enemyGroup {
		t.Errorf("enemies one, two and four form one group through enemy two, got %q %q %q\n%s", m(9), m(10), m(12), frame)
	}
	if m(11) != "" || m(13) != "" {
		t.Errorf("the hidden enemy and enemy five shouldn't be marked, got %q and %q\n%s", m(11), m(13), frame)
	}
}

func TestDeathmatchHasNoLikelyParties(t *testing.T) {
	r := newRig(t, deathmatch, func(w *fakeriot.World) {
		for i, id := range []string{matchID(20), matchID(21)} {
			m := &fakeriot.Match{ID: id, Queue: "deathmatch", Map: fakeriot.MapBind, Mode: fakeriot.ModeDeathmatch, Start: testStart.Add(-time.Duration(i+1) * time.Hour)}
			for _, p := range []string{dmPlayer(1), dmPlayer(2)} {
				m.Players = append(m.Players, fakeriot.MatchPlayer{PUUID: p, Team: "Blue", Agent: fakeriot.Reyna, Rounds: 1, Kills: 10, Deaths: 10})
			}
			w.AddMatch(m)
		}
	})
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopIngame })
	r.fr.EmitCoreGame(dmMatch, false)
	frame := r.settled("MATCH LOADING", "dm1#DM", "dm2#DM")
	for line := 1; line <= 14; line++ {
		if got := marker(t, frame, line); got != "" {
			t.Errorf("line %d has a party marker %q in deathmatch\n%s", line, got, frame)
		}
	}
}
