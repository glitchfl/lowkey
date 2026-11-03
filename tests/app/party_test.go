package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// marker is the party column under the column header
func marker(t *testing.T, frame string, nth int) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	for i, l := range lines {
		if headerIndex([]rune(l), []rune("PLAYER")) >= 0 && !strings.Contains(l, "·") && i+nth < len(lines) {
			row := []rune(lines[i+nth])
			if len(row) == 0 || row[0] == ' ' {
				return ""
			}
			pIdx := headerIndex([]rune(l), []rune("PLAYER"))
			if pIdx == 0 {
				return ""
			}
			if len(row) < pIdx {
				pIdx = len(row)
			}
			return strings.TrimSpace(string(row[:pIdx]))
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
