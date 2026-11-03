package lobbytable_test

import (
	"testing"

	"github.com/glitchfl/lowkey/internal/lobbytable"
	"github.com/glitchfl/lowkey/internal/riot"
)

func TestEachRosterPlayersVisibility(t *testing.T) {
	rules := func(matchLoad, hiddenStats, enemyLookups bool) lobbytable.Rules {
		return lobbytable.Rules{Self: self, OwnTeam: "Blue", MatchLoad: matchLoad, HiddenPlayerStats: hiddenStats, EnemyLookups: enemyLookups}
	}
	ally := riot.RosterPlayer{PUUID: ally1, Team: "Blue"}
	hiddenAlly := riot.RosterPlayer{PUUID: ally2, Team: "Blue", Incognito: true}
	enemy := riot.RosterPlayer{PUUID: enemy1, Team: "Red"}
	hiddenEnemy := riot.RosterPlayer{PUUID: enemy3, Team: "Red", Incognito: true}
	you := riot.RosterPlayer{PUUID: self, Team: "Blue", Incognito: true}
	for _, c := range []struct {
		name  string
		rules lobbytable.Rules
		p     riot.RosterPlayer
		want  lobbytable.Visibility
	}{
		{"a visible ally is looked up", rules(false, true, true), ally, lobbytable.Visibility{Stats: true}},
		{"a hidden ally is looked up but stays hidden", rules(false, true, true), hiddenAlly, lobbytable.Visibility{Stats: true, Hidden: true}},
		{"hidden player stats off leaves them out", rules(false, false, true), hiddenAlly, lobbytable.Visibility{Hidden: true}},
		{"you are never hidden from yourself", rules(false, false, false), you, lobbytable.Visibility{Stats: true}},
		{"enemy lookups off leaves enemies out at match load", rules(true, true, false), enemy, lobbytable.Visibility{}},
		{"enemy lookups off leaves allies alone", rules(true, true, false), ally, lobbytable.Visibility{Stats: true}},
		{"agent select has no enemies to leave out", rules(false, true, false), enemy, lobbytable.Visibility{Stats: true}},
		{"a hidden enemy needs both settings", rules(true, true, false), hiddenEnemy, lobbytable.Visibility{Hidden: true}},
		{"a hidden enemy with both settings on is looked up", rules(true, true, true), hiddenEnemy, lobbytable.Visibility{Stats: true, Hidden: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.rules.Decide(c.p); got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestStreamerModeSeenOnceStaysForTheLobby(t *testing.T) {
	v := lobbytable.Visibility{Stats: true}
	v = v.Next(lobbytable.Visibility{Stats: true, Hidden: true})
	v = v.Next(lobbytable.Visibility{Stats: true})
	if !v.Hidden {
		t.Errorf("a player who went hidden between rosters should stay hidden, got %+v", v)
	}
}

func TestLookupsStayAsFirstDecidedUntilTheNextLobby(t *testing.T) {
	if v := (lobbytable.Visibility{}).Next(lobbytable.Visibility{Stats: true}); v.Stats {
		t.Errorf("a setting switched on mid-lobby shouldn't start lookups until the next lobby, got %+v", v)
	}
	if v := (lobbytable.Visibility{Stats: true}).Next(lobbytable.Visibility{}); !v.Stats {
		t.Errorf("stats already looked up stay on screen for the lobby, got %+v", v)
	}
}
