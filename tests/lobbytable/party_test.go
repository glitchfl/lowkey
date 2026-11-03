package lobbytable_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glitchfl/lowkey/internal/lobbytable"
	"github.com/glitchfl/lowkey/internal/stats"
)

// together is a past match where the listed players were on one team - an empty party ID means none was read
func together(id int, team, partyID string, puuids ...string) *stats.Match {
	m := &stats.Match{ID: fmt.Sprintf("m%d", id), Players: map[string]*stats.MatchPlayer{}}
	for _, p := range puuids {
		m.Players[p] = &stats.MatchPlayer{PUUID: p, Team: team, PartyID: partyID}
	}
	return m
}

// partyLobby is a match load with you and four allies on Blue and five enemies on Red - enemy three is hidden
// with stats and every player's sample is the matches from history they played in
func partyLobby(t *testing.T, exact bool, history ...*stats.Match) lobbytable.Table {
	t.Helper()
	var members []lobbytable.Member
	add := func(puuid, team string) {
		p := player(puuid, puuid[:4]+"#1")
		var mine []*stats.Match
		for _, m := range history {
			if m.Players[puuid] != nil {
				mine = append(mine, m)
			}
		}
		p.Matches = ready(mine)
		m := member(puuid, team, p)
		if puuid == enemy3 {
			m = hidden(m)
		}
		members = append(members, m)
	}
	for _, p := range []string{self, ally1, ally2, ally3, ally4} {
		add(p, "Blue")
	}
	for _, p := range []string{enemy1, enemy2, enemy3, enemy4, enemy5} {
		add(p, "Red")
	}
	in := input(t, lobbytable.Lobby{MatchLoad: true, Members: members})
	in.Settings.ExactEnemyParties = exact
	return lobbytable.Build(in)
}

func marks(t *testing.T, tb lobbytable.Table, puuids ...string) []string {
	t.Helper()
	var out []string
	for _, p := range puuids {
		out = append(out, cell(t, tb, p, "party"))
	}
	return out
}

func TestYourOwnPartyIsMarkedExactly(t *testing.T) {
	in := input(t, lobbytable.Lobby{Party: map[string]bool{self: true, ally1: true}, Members: []lobbytable.Member{
		member(self, "Blue", player(self, "You#1")), member(ally1, "Blue", player(ally1, "A#1")), member(ally2, "Blue", player(ally2, "B#1")),
	}})
	tb := lobbytable.Build(in)
	if got := strings.Join(marks(t, tb, self, ally1, ally2), ","); got != "●,●," {
		t.Errorf("got %q", got)
	}
	in.Lobby.Party = map[string]bool{self: true}
	if got := cell(t, lobbytable.Build(in), self, "party"); got != "" {
		t.Errorf("queueing alone is no party, got %q", got)
	}
}

func TestLikelyPartiesShareAGroupSymbol(t *testing.T) {
	opponents := func(id int) *stats.Match {
		m := together(id, "Blue", "", ally4)
		m.Players[ally1] = &stats.MatchPlayer{PUUID: ally1, Team: "Red"}
		return m
	}
	tb := partyLobby(t, false,
		together(1, "Blue", "", ally1, ally2), together(2, "Red", "", ally1, ally2),
		together(3, "Red", "", enemy1, enemy2), together(4, "Blue", "", enemy1, enemy2),
		together(5, "Red", "", enemy2, enemy4), together(6, "Red", "", enemy2, enemy4),
		together(7, "Red", "", enemy1, enemy5),
		together(8, "Red", "", enemy3, enemy5), together(9, "Blue", "", enemy3, enemy5),
		opponents(10), opponents(11),
	)
	m := marks(t, tb, self, ally1, ally2, ally4, enemy1, enemy2, enemy3, enemy4, enemy5)
	if m[1] == "" || m[1] == "●" || m[2] != m[1] {
		t.Errorf("allies one and two should share a likely-party symbol, got %q and %q", m[1], m[2])
	}
	if m[0] != "" || m[3] != "" {
		t.Errorf("you (solo) and ally four (only ever an opponent) shouldn't be marked, got %q and %q", m[0], m[3])
	}
	if m[4] == "" || m[4] == m[1] || m[5] != m[4] || m[7] != m[4] {
		t.Errorf("enemies one, two and four form one group through enemy two, got %q %q %q", m[4], m[5], m[7])
	}
	if m[6] != "" || m[8] != "" {
		t.Errorf("the hidden enemy is never marked and never links enemy five to anyone, got %q and %q", m[6], m[8])
	}
}

func TestDeathmatchHasNoLikelyParties(t *testing.T) {
	var members []lobbytable.Member
	history := []*stats.Match{together(1, "Blue", "", enemy1, enemy2), together(2, "Blue", "", enemy1, enemy2)}
	for _, puuid := range []string{self, enemy1, enemy2} {
		p := player(puuid, "P#1")
		p.Matches = ready(history)
		members = append(members, member(puuid, puuid, p))
	}
	tb := lobbytable.Build(input(t, lobbytable.Lobby{MatchLoad: true, Mode: mode(t, "deathmatch"), Members: members}))
	if got := strings.Join(marks(t, tb, self, enemy1, enemy2), ""); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestExactEnemyParties(t *testing.T) {
	allies := []*stats.Match{together(1, "Blue", "", ally1, ally2), together(2, "Red", "", ally1, ally2)}

	t.Run("a shared party ID gives a letter and shared teams still give a shape", func(t *testing.T) {
		tb := partyLobby(t, true, append(allies,
			together(3, "Red", "party-enemy-a", enemy1, enemy2),
			together(4, "Blue", "", enemy4, enemy5), together(5, "Blue", "", enemy4, enemy5))...)
		m := marks(t, tb, ally1, ally2, enemy1, enemy2, enemy3, enemy4, enemy5)
		if m[0] == "" || m[0] == "●" || m[1] != m[0] {
			t.Errorf("allies one and two should share a likely-party symbol, got %q and %q", m[0], m[1])
		}
		if m[2] != "[A]" || m[3] != "[A]" {
			t.Errorf("enemies one and two should be party [A], got %q and %q", m[2], m[3])
		}
		if m[4] != "" {
			t.Errorf("the hidden enemy should have no marker, got %q", m[4])
		}
		if m[5] == "" || m[5] != m[6] || m[5] == m[0] || strings.HasPrefix(m[5], "[") {
			t.Errorf("enemies four and five should share their own likely-party shape, got %q and %q (allies %q)", m[5], m[6], m[0])
		}
	})

	t.Run("off brings back the shapes alone", func(t *testing.T) {
		tb := partyLobby(t, false, together(3, "Red", "party-enemy-a", enemy1, enemy2),
			together(4, "Blue", "", enemy4, enemy5), together(5, "Blue", "", enemy4, enemy5))
		m := marks(t, tb, enemy1, enemy2, enemy4, enemy5)
		if strings.HasPrefix(m[0], "[") || strings.HasPrefix(m[1], "[") {
			t.Errorf("no letters when it's off, got %q and %q", m[0], m[1])
		}
		if m[2] == "" || m[2] != m[3] {
			t.Errorf("enemies four and five should share a likely symbol, got %q and %q", m[2], m[3])
		}
	})

	t.Run("two parties get different letters", func(t *testing.T) {
		tb := partyLobby(t, true, together(10, "Red", "party-alpha", enemy1, enemy2), together(11, "Red", "party-beta", enemy4, enemy5))
		if got := strings.Join(marks(t, tb, enemy1, enemy2, enemy4, enemy5), ","); got != "[A],[A],[B],[B]" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("two guessed parties get different shapes", func(t *testing.T) {
		tb := partyLobby(t, true,
			together(10, "Red", "", enemy1, enemy2), together(11, "Red", "", enemy1, enemy2),
			together(12, "Red", "", enemy4, enemy5), together(13, "Red", "", enemy4, enemy5))
		m := marks(t, tb, enemy1, enemy2, enemy4, enemy5)
		if m[0] == "" || m[0] != m[1] || m[2] == "" || m[2] != m[3] || m[0] == m[2] {
			t.Errorf("want one shape for enemies one and two and another for four and five, got %v", m)
		}
	})

	t.Run("a hidden player never gets a letter or gives one", func(t *testing.T) {
		tb := partyLobby(t, true, together(10, "Red", "party-hidden", enemy1, enemy3))
		if got := strings.Join(marks(t, tb, enemy1, enemy3), ""); got != "" {
			t.Errorf("got %q", got)
		}
	})
}
