package lobbytable_test

import (
	"slices"
	"testing"

	"github.com/glitchfl/lowkey/internal/fakeriot"
	"github.com/glitchfl/lowkey/internal/lobbytable"
	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/settings"
	"github.com/glitchfl/lowkey/internal/stats"
)

func has(t *testing.T, tb lobbytable.Table, puuid, key string) bool {
	t.Helper()
	row, ok := tb.Row(puuid)
	if !ok {
		t.Fatalf("no row for %s", puuid)
	}
	_, ok = row[key]
	return ok
}

func columnKeys(cols []render.Column) []string {
	var out []string
	for _, c := range cols {
		out = append(out, c.Key)
	}
	return out
}

func TestAHiddenPlayerShowsTheirStatsButNeverTheirNameOrATie(t *testing.T) {
	secret := hidden(member(ally1, "Blue", player(ally1, "Secret#1")))
	secret.Met = "enemy L 2d"
	met := member(ally2, "Blue", player(ally2, "Seen#1"))
	met.Met = "ally W 3h"
	tb := lobbytable.Build(input(t, lobbytable.Lobby{
		Members: []lobbytable.Member{member(self, "Blue", player(self, "You#1")), secret, met},
		// your party is you and the hidden ally - a party of one shows no marker
		Party: map[string]bool{self: true, ally1: true},
	}))
	if got := cell(t, tb, ally1, "name"); got != "hidden" {
		t.Errorf("a name fetched before they went hidden should stay off, got %q", got)
	}
	if got := cell(t, tb, ally1, "rank"); got != "Dia1 50" {
		t.Errorf("hidden player stats are on so their rank shows, got %q", got)
	}
	if got := cell(t, tb, ally1, "party") + cell(t, tb, self, "party"); got != "" {
		t.Errorf("a hidden player never gets or gives a party marker, got %q", got)
	}
	if got := cell(t, tb, ally1, "met"); got != "" {
		t.Errorf("a hidden player never gets met-before, got %q", got)
	}
	if got := cell(t, tb, ally2, "met"); got != "ally W 3h" {
		t.Errorf("a visible player keeps their met-before, got %q", got)
	}
}

func TestAPlayerLowkeyDidntLookUpShowsOnlyTheirAgent(t *testing.T) {
	off := hidden(member(ally1, "Blue", nil))
	off.Met = "ally W 3h"
	in := input(t, lobbytable.Lobby{
		MatchLoad: true,
		Members:   []lobbytable.Member{member(self, "Blue", player(self, "You#1")), off},
		Skins:     riot.Loadouts{self: {fakeriot.Vandal: {Skin: fakeriot.ReaverVandal}}, ally1: {fakeriot.Vandal: {Skin: fakeriot.PrimeVandal}}},
	})
	tb := lobbytable.Build(in)
	if got := cell(t, tb, ally1, "agent"); got != "Sova" {
		t.Errorf("agent: got %q", got)
	}
	for _, key := range []string{"rank", "rr_trend", "kd", "met", "skin:vandal"} {
		if has(t, tb, ally1, key) {
			t.Errorf("%s shouldn't show for a player lowkey didn't look up, got %q", key, cell(t, tb, ally1, key))
		}
	}
	if got := cell(t, tb, ally1, "name"); got != "hidden" {
		t.Errorf("name: got %q", got)
	}
	if got := cell(t, tb, self, "skin:vandal"); got != "Reaver" {
		t.Errorf("your own skin should show, got %q", got)
	}
}

func TestARecordingAliasRenamesEveryoneButHiddenPlayers(t *testing.T) {
	in := input(t, lobbytable.Lobby{Members: []lobbytable.Member{
		member(ally1, "Blue", player(ally1, "Real#1")),
		hidden(member(ally2, "Blue", player(ally2, "Secret#1"))),
	}})
	in.Alias = func(string) string { return "Fake#1" }
	tb := lobbytable.Build(in)
	if got := cell(t, tb, ally1, "name"); got != "Fake#1" {
		t.Errorf("got %q", got)
	}
	if got := cell(t, tb, ally2, "name"); got != "hidden" {
		t.Errorf("got %q", got)
	}
}

func TestLoadingCellsStayBlankAndFailedOnesShowADash(t *testing.T) {
	gone := &lobbytable.Player{PUUID: ally2, Name: failed[string](), MMR: failed[stats.MMR](), Updates: failed[[]stats.Update](),
		Matches: failed[[]*stats.Match]()}
	tb := lobbytable.Build(input(t, lobbytable.Lobby{Members: []lobbytable.Member{
		member(ally1, "Blue", &lobbytable.Player{PUUID: ally1}),
		member(ally2, "Blue", gone),
	}}))
	for _, key := range []string{"name", "rank", "wr", "rr_trend", "kd", "n"} {
		if has(t, tb, ally1, key) {
			t.Errorf("%s is still loading so it should stay blank, got %q", key, cell(t, tb, ally1, key))
		}
		if got := cell(t, tb, ally2, key); got != "-" {
			t.Errorf("%s failed so it should show a dash, got %q", key, got)
		}
	}
}

func TestRanksWaitForTheActs(t *testing.T) {
	in := input(t, lobbytable.Lobby{Members: []lobbytable.Member{member(ally1, "Blue", player(ally1, "A#1"))}})
	in.Acts = nil
	if got := cell(t, lobbytable.Build(in), ally1, "rank"); got != "-" {
		t.Errorf("got %q", got)
	}
}

func TestALongRiotIDIsCutToTwentyCharacters(t *testing.T) {
	tb := lobbytable.Build(input(t, lobbytable.Lobby{Members: []lobbytable.Member{
		member(ally1, "Blue", player(ally1, "abcdefghijklmnopqrstuvw#EUW")),
		member(ally2, "Blue", player(ally2, "abcdefghijklmnop#EUW")),
	}}))
	if got := cell(t, tb, ally1, "name"); got != "abcdefghijklmnopqrs…" {
		t.Errorf("got %q", got)
	}
	if got := cell(t, tb, ally2, "name"); got != "abcdefghijklmnop#EUW" {
		t.Errorf("20 characters fit whole, got %q", got)
	}
}

func TestMatchLoadAveragesEachTeamAndShowsUsMinusThem(t *testing.T) {
	enemy := func(puuid string) lobbytable.Member {
		p := player(puuid, "E#1")
		p.MMR, p.Updates = ready(ranked(16, 10)), ready(trend(-5))
		return member(puuid, "Red", p)
	}
	ascendant := player(ally1, "A#1")
	ascendant.MMR = ready(ranked(21, 0))
	tb := lobbytable.Build(input(t, lobbytable.Lobby{MatchLoad: true, Members: []lobbytable.Member{
		member(self, "Blue", player(self, "You#1")), member(ally1, "Blue", ascendant), enemy(enemy1), enemy(enemy2),
	}}))
	if len(tb.Sections) != 2 || tb.Sections[0].Title != "ALLIES" || tb.Sections[1].Title != "ENEMIES" {
		t.Fatalf("want ALLIES and ENEMIES, got %+v", tb.Sections)
	}
	// Dia1 and Asc1 are tiers 18 and 21 - 19.5 rounds up
	if got, want := tb.Sections[0].Average["rank"].Text, data.TierLabel(20); got != want {
		t.Errorf("allies average rank: got %q, want %q", got, want)
	}
	if got := tb.Sections[0].Average["name"].Text; got != "AVERAGE" {
		t.Errorf("everyone has a rank so no coverage count, got %q", got)
	}
	if got := tb.Diff["rank"].Text; got != "▲4" {
		t.Errorf("US - THEM rank: got %q", got)
	}
	if got := tb.Diff["rr_trend"].Text; got != "▲15" {
		t.Errorf("US - THEM trend: got %q", got)
	}
	sections := tb.Render()
	if len(sections) != 3 || sections[2].Rows[0]["name"].Text != "US - THEM" {
		t.Errorf("want the US - THEM row in its own section last, got %+v", sections)
	}
	if n := len(sections[0].Rows); n != 3 {
		t.Errorf("allies render as 2 players and their average, got %d rows", n)
	}
}

func TestATrendWithoutARankStillCountsInTheTeamsTrend(t *testing.T) {
	unranked := player(self, "You#1")
	unranked.MMR, unranked.Updates = failed[stats.MMR](), ready(trend(30))
	tb := lobbytable.Build(input(t, lobbytable.Lobby{Members: []lobbytable.Member{
		member(self, "Blue", unranked), member(ally1, "Blue", player(ally1, "A#1")),
	}}))
	avg := tb.Sections[0].Average
	if got := avg["rr_trend"].Text; got != "+20" {
		t.Errorf("trend average: got %q", got)
	}
	if got := avg["name"].Text; got != "AVERAGE 1/2" {
		t.Errorf("label: got %q", got)
	}
}

func TestUsMinusThemWaitsForTheEnemies(t *testing.T) {
	tb := lobbytable.Build(input(t, lobbytable.Lobby{MatchLoad: true, Members: []lobbytable.Member{
		member(self, "Blue", player(self, "You#1")), member(ally1, "Blue", player(ally1, "A#1")),
	}}))
	if len(tb.Sections) != 1 || tb.Sections[0].Title != "ALLIES" || tb.Diff != nil {
		t.Errorf("want only the allies until the enemies arrive, got %+v", tb)
	}
}

func TestDeathmatchIsOneListWithALobbyAverage(t *testing.T) {
	tb := lobbytable.Build(input(t, lobbytable.Lobby{MatchLoad: true, Mode: mode(t, "deathmatch"), Members: []lobbytable.Member{
		member(self, self, player(self, "You#1")), member(enemy1, enemy1, player(enemy1, "E#1")),
	}}))
	if len(tb.Sections) != 1 || tb.Sections[0].Title != "" || tb.Diff != nil {
		t.Fatalf("want one untitled list and no diff, got %+v", tb)
	}
	if tb.Sections[0].Average == nil {
		t.Error("want a lobby average")
	}
}

func TestModesWithoutRoundsShowDashesForRoundStats(t *testing.T) {
	match := &stats.Match{ID: "m1", Players: map[string]*stats.MatchPlayer{
		ally1: {PUUID: ally1, Team: "Blue", Agent: fakeriot.Jett, Kills: 30, Deaths: 15, Score: 4000, Rounds: 1, Head: 5, Body: 5},
	}}
	p := player(ally1, "A#1")
	p.Matches = ready([]*stats.Match{match})
	tb := lobbytable.Build(input(t, lobbytable.Lobby{MatchLoad: true, Mode: mode(t, "team_deathmatch"), Members: []lobbytable.Member{
		member(ally1, "Blue", p),
	}}))
	if got := cell(t, tb, ally1, "kd"); got != "2.00" {
		t.Errorf("kd: got %q", got)
	}
	for _, key := range []string{"acs", "adr", "fb"} {
		if got := cell(t, tb, ally1, key); got != "-" {
			t.Errorf("%s: got %q", key, got)
		}
		if _, ok := tb.Sections[0].Average[key]; ok {
			t.Errorf("the average shouldn't have %s either", key)
		}
	}
}

func TestColumnsFollowTheSettings(t *testing.T) {
	cfg := settings.Default()
	cfg.Columns = []string{"name", "agent", "skin", "rank"}
	cfg.Weapons = []string{"vandal", "phantom"}
	cfg.VoiceCheck = true
	cols := lobbytable.Columns(cfg, true)
	if got, want := columnKeys(cols), []string{"pick", "name", "agent", "voice", "skin:vandal", "skin:phantom", "rank"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if cols[4].Header != "VANDAL" || cols[5].Header != "PHANTOM" {
		t.Errorf("each skin column is headed by its gun, got %q and %q", cols[4].Header, cols[5].Header)
	}
	cfg.Columns = []string{"name", "rank"}
	if got, want := columnKeys(lobbytable.Columns(cfg, false)), []string{"name", "rank", "voice"}; !slices.Equal(got, want) {
		t.Errorf("without an agent column voice goes last and there's no pick column, got %v", got)
	}
}

func TestLowkeyMeLeavesOutTheLobbyOnlyColumns(t *testing.T) {
	tb := lobbytable.You(*player(self, "You#1"), mode(t, "competitive"), acts, data, settings.Default())
	got := columnKeys(tb.Columns)
	for _, key := range []string{"party", "agent", "met", "skin"} {
		if slices.Contains(got, key) {
			t.Errorf("%s only means something in a lobby, got %v", key, got)
		}
	}
	if !slices.Contains(got, "rank") || cell(t, tb, self, "rank") != "Dia1 50" {
		t.Errorf("want your rank, got %v and %q", got, cell(t, tb, self, "rank"))
	}
	if sections := tb.Render(); len(sections) != 1 || len(sections[0].Rows) != 1 {
		t.Errorf("want just your row with no average, got %+v", sections)
	}
}

func TestVoiceShowsOnlyInAgentSelect(t *testing.T) {
	build := func(matchLoad, check bool) lobbytable.Table {
		in := input(t, lobbytable.Lobby{MatchLoad: matchLoad, Voice: map[string]string{ally1: "on"}, Members: []lobbytable.Member{
			member(self, "Blue", player(self, "You#1")), member(ally1, "Blue", player(ally1, "A#1")),
		}})
		in.Settings.VoiceCheck = check
		return lobbytable.Build(in)
	}
	tb := build(false, true)
	if got := cell(t, tb, ally1, "voice") + "," + cell(t, tb, self, "voice"); got != "on,-" {
		t.Errorf("agent select: got %q", got)
	}
	if tb := build(true, true); has(t, tb, ally1, "voice") {
		t.Error("match load shouldn't have voice cells")
	}
	if tb := build(false, false); has(t, tb, ally1, "voice") {
		t.Error("voice check off shouldn't have voice cells")
	}
}

func TestSkinsShowForEveryGunInTheirEditionsColor(t *testing.T) {
	in := input(t, lobbytable.Lobby{MatchLoad: true, Members: []lobbytable.Member{member(self, "Blue", player(self, "You#1"))},
		Skins: riot.Loadouts{self: {
			fakeriot.Vandal:  {Skin: fakeriot.ReaverVandal},
			fakeriot.Phantom: {Skin: "5c1a0000-0000-4000-8000-000000000003"},
		}}})
	in.Settings.Weapons = []string{"vandal", "phantom"}
	row, _ := lobbytable.Build(in).Row(self)
	reaver, _ := data.Skin(fakeriot.ReaverVandal)
	if c := row["skin:vandal"]; c.Text != "Reaver" || c.Color == "" || c.Color != data.EditionColor(reaver.Edition) {
		t.Errorf("vandal: got %+v", c)
	}
	if got := row["skin:phantom"].Text; got != "?" {
		t.Errorf("a skin newer than the snapshot shows ?, got %q", got)
	}
}

func TestTheWhoRowHasTeamAndLevel(t *testing.T) {
	you := member(self, "Blue", player(self, "You#1"))
	you.Level = 120
	lob := lobbytable.Lobby{MatchLoad: true, Members: []lobbytable.Member{you, member(enemy1, "Red", player(enemy1, "E#1"))}}
	tb := lobbytable.Build(input(t, lob))
	if got := cell(t, tb, self, "team") + "," + cell(t, tb, enemy1, "team"); got != "ally,enemy" {
		t.Errorf("team: got %q", got)
	}
	if got := cell(t, tb, self, "level"); got != "120" {
		t.Errorf("level: got %q", got)
	}
	if has(t, tb, enemy1, "level") {
		t.Error("a hidden account level stays blank")
	}
	lob.MatchLoad = false
	if has(t, lobbytable.Build(input(t, lob)), self, "team") {
		t.Error("agent select has no team cell")
	}
}

func TestThePickedPlayerGetsTheMarker(t *testing.T) {
	in := input(t, lobbytable.Lobby{Members: []lobbytable.Member{
		member(self, "Blue", player(self, "You#1")), member(ally1, "Blue", player(ally1, "A#1")),
	}})
	in.Picking, in.Picked = true, ally1
	tb := lobbytable.Build(in)
	if got := cell(t, tb, ally1, "pick") + "," + cell(t, tb, self, "pick"); got != "▸," {
		t.Errorf("got %q", got)
	}
	if tb.Columns[0].Key != "pick" {
		t.Errorf("the pick column comes first, got %v", columnKeys(tb.Columns))
	}
}
