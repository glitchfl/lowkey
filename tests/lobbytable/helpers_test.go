package lobbytable_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/glitchfl/lowkey/internal/fakeriot"
	"github.com/glitchfl/lowkey/internal/lobbytable"
	"github.com/glitchfl/lowkey/internal/settings"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

var data = static.Embedded()

const (
	self   = "5e1f0000-0000-4000-8000-000000000000"
	ally1  = "a1100000-0000-4000-8000-000000000001"
	ally2  = "a1100000-0000-4000-8000-000000000002"
	ally3  = "a1100000-0000-4000-8000-000000000003"
	ally4  = "a1100000-0000-4000-8000-000000000004"
	enemy1 = "e1100000-0000-4000-8000-000000000001"
	enemy2 = "e1100000-0000-4000-8000-000000000002"
	enemy3 = "e1100000-0000-4000-8000-000000000003"
	enemy4 = "e1100000-0000-4000-8000-000000000004"
	enemy5 = "e1100000-0000-4000-8000-000000000005"
)

// acts are made up - an act the static data doesn't know keeps its tiers as they are
var acts = &stats.Acts{Current: "act-now", Last: "act-last"}

func mode(t *testing.T, key string) stats.Mode {
	t.Helper()
	m, ok := stats.ModeByKey(key)
	if !ok {
		t.Fatalf("no mode %q", key)
	}
	return m
}

func ready[T any](v T) lobbytable.Fact[T] {
	return lobbytable.Fact[T]{State: lobbytable.Ready, Value: v}
}

func failed[T any]() lobbytable.Fact[T] {
	return lobbytable.Fact[T]{State: lobbytable.Failed}
}

func ranked(tier, rr int) stats.MMR {
	return stats.MMR{"competitive": {acts.Current: {Tier: tier, RR: rr, Games: 10, Wins: 6, WinsWithPlacements: 6,
		WinsByTier: map[string]int{strconv.Itoa(tier): 6}}}}
}

// trend is one competitive update per game - newest first
func trend(earned ...int) []stats.Update {
	var out []stats.Update
	for i, e := range earned {
		out = append(out, stats.Update{MatchID: fmt.Sprintf("u%d", i), Earned: e})
	}
	return out
}

// player has everything downloaded - a Diamond 1 on 50 RR with a +10 trend and no matches
func player(puuid, name string) *lobbytable.Player {
	return &lobbytable.Player{
		PUUID:   puuid,
		Name:    ready(name),
		MMR:     ready(ranked(18, 50)),
		Updates: ready(trend(10)),
		Matches: ready[[]*stats.Match](nil),
	}
}

func member(puuid, team string, p *lobbytable.Player) lobbytable.Member {
	return lobbytable.Member{PUUID: puuid, Agent: fakeriot.Sova, Team: team, Visibility: lobbytable.Visibility{Stats: p != nil}, Player: p}
}

func hidden(m lobbytable.Member) lobbytable.Member {
	m.Visibility.Hidden = true
	return m
}

func input(t *testing.T, lob lobbytable.Lobby) lobbytable.Input {
	t.Helper()
	if lob.Mode.Key == "" {
		lob.Mode = mode(t, "competitive")
	}
	if lob.Self == "" {
		lob.Self = self
	}
	if lob.OwnTeam == "" {
		lob.OwnTeam = "Blue"
	}
	return lobbytable.Input{Lobby: lob, Acts: acts, Data: data, Settings: settings.Default()}
}

// cell is a player's cell text - "" when the cell isn't there
func cell(t *testing.T, tb lobbytable.Table, puuid, key string) string {
	t.Helper()
	row, ok := tb.Row(puuid)
	if !ok {
		t.Fatalf("no row for %s", puuid)
	}
	return row[key].Text
}
