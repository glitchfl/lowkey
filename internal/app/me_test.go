package app_test

import (
	"strings"
	"testing"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// ranked is your account across acts and queues - the expected values are worked out by hand
func ranked(w *fakeriot.World) {
	me := w.Player(w.Self)
	me.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{
		Tier: 20, RR: 12, Wins: 10, WinsWithPlacements: 12, Games: 21,
		WinsByTier: map[string]int{"0": 1, "1": 2, "18": 3, "19": 5, "20": 2},
	})
	me.SetRank("competitive", fakeriot.ActLast, fakeriot.SeasonInfo{
		Tier: 19, RR: 40, Wins: 20, WinsWithPlacements: 20, Games: 38,
		WinsByTier: map[string]int{"17": 4, "19": 3},
	})
	// episode 4 - 22 was immortal 2 back then and is 25 today
	me.SetRank("competitive", fakeriot.ActEp4, fakeriot.SeasonInfo{
		Tier: 22, Wins: 30, WinsWithPlacements: 30, Games: 50,
		WinsByTier: map[string]int{"21": 2, "22": 1},
	})
	me.SetRank("unrated", fakeriot.ActCurrent, fakeriot.SeasonInfo{Wins: 7, WinsWithPlacements: 7, Games: 10})
}

// cell reads a column of the first row under the header line - cells are separated by 2+ spaces
func cell(t *testing.T, out, column string) string {
	t.Helper()
	return cellIn(t, out, column, 1)
}

// cellIn reads the column from the nth line below the header - positions are runes so markers line up
func cellIn(t *testing.T, out, column string, nth int) string {
	t.Helper()
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		// titles and the session line use "·" - only the column header line doesn't
		hs := headerIndex([]rune(l), []rune(column))
		if hs < 0 || strings.Contains(l, "·") || i+nth >= len(lines) {
			continue
		}
		he := hs + len([]rune(column))
		for _, seg := range segments([]rune(lines[i+nth])) {
			if seg.start < he && seg.end > hs {
				return seg.text
			}
		}
		return ""
	}
	t.Fatalf("no %s column in:\n%s", column, out)
	return ""
}

func headerIndex(header, column []rune) int {
	for i := 0; i+len(column) <= len(header); i++ {
		if string(header[i:i+len(column)]) != string(column) {
			continue
		}
		before := i == 0 || header[i-1] == ' '
		after := i+len(column) == len(header) || header[i+len(column)] == ' '
		if before && after {
			return i
		}
	}
	return -1
}

type segment struct {
	start, end int
	text       string
}

func segments(row []rune) []segment {
	var out []segment
	i := 0
	for i < len(row) {
		for i < len(row) && row[i] == ' ' {
			i++
		}
		if i >= len(row) {
			break
		}
		start := i
		for i < len(row) && !(row[i] == ' ' && (i+1 >= len(row) || row[i+1] == ' ')) {
			i++
		}
		out = append(out, segment{start, i, string(row[start:i])})
	}
	return out
}

func TestMeShowsYourRankColumns(t *testing.T) {
	r := newRig(t, ranked)
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	want := map[string]string{"RANK": "Dia3 12", "PEAK": "Imm2", "LAST": "Dia2", "WR": "57%", "W-L": "12-9"}
	for col, v := range want {
		if got := cell(t, out, col); got != v {
			t.Errorf("%s: got %q, want %q\n%s", col, got, v, out)
		}
	}
}

func TestMeFollowsTheChosenModeForWinRateButNotRank(t *testing.T) {
	r := newRig(t, ranked)
	out, code := r.run("me", "--mode", "unrated")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	want := map[string]string{"RANK": "Dia3 12", "WR": "70%", "W-L": "7-3"}
	for col, v := range want {
		if got := cell(t, out, col); got != v {
			t.Errorf("%s: got %q, want %q\n%s", col, got, v, out)
		}
	}
	if !strings.Contains(out, "Unrated") {
		t.Errorf("the header should name the mode:\n%s", out)
	}
}

func TestPeakRank(t *testing.T) {
	cases := []struct {
		name  string
		ranks map[string]fakeriot.SeasonInfo
		want  string
	}{
		{"null wins by tier falls back to the current tier", map[string]fakeriot.SeasonInfo{
			fakeriot.ActCurrent: {Tier: 15, Games: 5},
			fakeriot.ActLast:    {Tier: 12, Games: 5},
		}, "Plat1"},
		{"keys 0 to 2 don't count", map[string]fakeriot.SeasonInfo{
			fakeriot.ActCurrent: {Tier: 0, WinsByTier: map[string]int{"0": 4, "1": 1, "2": 1}},
		}, "-"},
		{"episode 1 radiant is today's radiant", map[string]fakeriot.SeasonInfo{
			fakeriot.ActCurrent: {Tier: 18, WinsByTier: map[string]int{"18": 1}},
			fakeriot.ActEp1:     {Tier: 24, WinsByTier: map[string]int{"23": 3, "24": 1}},
		}, "Rad"},
		{"episode 2 immortal maps to immortal 1", map[string]fakeriot.SeasonInfo{
			fakeriot.ActCurrent: {Tier: 18, WinsByTier: map[string]int{"18": 1}},
			fakeriot.ActEp2:     {Tier: 21, WinsByTier: map[string]int{"21": 3}},
		}, "Imm1"},
		{"episode 5 tiers stay as they are", map[string]fakeriot.SeasonInfo{
			fakeriot.ActCurrent: {Tier: 18, WinsByTier: map[string]int{"18": 1}},
			fakeriot.ActEp5:     {Tier: 22, WinsByTier: map[string]int{"21": 3, "22": 1}},
		}, "Asc2"},
		{"current tier above every win tier", map[string]fakeriot.SeasonInfo{
			fakeriot.ActCurrent: {Tier: 19, WinsByTier: map[string]int{"16": 2, "17": 1}},
		}, "Dia2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, func(w *fakeriot.World) {
				for act, info := range c.ranks {
					w.Player(w.Self).SetRank("competitive", act, info)
				}
			})
			out, code := r.run("me")
			if code != 0 {
				t.Fatalf("me failed:\n%s", out)
			}
			if got := cell(t, out, "PEAK"); got != c.want {
				t.Errorf("got %q, want %q\n%s", got, c.want, out)
			}
		})
	}
}

func TestLastActIsThePreviousActByStartTimeNotTheEpisode(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) {
		me := w.Player(w.Self)
		me.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 14, RR: 3})
		me.SetRank("competitive", fakeriot.ActLast, fakeriot.SeasonInfo{Tier: 16})
		me.SetRank("competitive", fakeriot.ActEarlier, fakeriot.SeasonInfo{Tier: 9})
		me.SetRank("competitive", fakeriot.EpisodeV26, fakeriot.SeasonInfo{Tier: 27})
	})
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	if got := cell(t, out, "RANK"); got != "Gold3 3" {
		t.Errorf("rank: got %q, want the active act's Gold3 3", got)
	}
	if got := cell(t, out, "LAST"); got != "Plat2" {
		t.Errorf("last act: got %q, want Plat2", got)
	}
}

func TestWinRateNeedsGamesInTheQueue(t *testing.T) {
	cases := []struct {
		name  string
		setup func(w *fakeriot.World)
	}{
		{"brand-new account", func(w *fakeriot.World) {}},
		{"no games this act", func(w *fakeriot.World) {
			w.Player(w.Self).SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{})
		}},
		{"games only last act", func(w *fakeriot.World) {
			w.Player(w.Self).SetRank("competitive", fakeriot.ActLast, fakeriot.SeasonInfo{Tier: 12, Wins: 4, Games: 9})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, c.setup)
			out, code := r.run("me")
			if code != 0 {
				t.Fatalf("me failed:\n%s", out)
			}
			for _, col := range []string{"RANK", "WR", "W-L"} {
				if got := cell(t, out, col); got != "-" {
					t.Errorf("%s: got %q, want -\n%s", col, got, out)
				}
			}
		})
	}
}

func TestMeAsksRiotOnlyForWhatYourRowNeeds(t *testing.T) {
	r := newRig(t, ranked)
	if out, code := r.run("me"); code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	var got []string
	for _, req := range r.fr.RemoteRequests() {
		got = append(got, req.Service+" "+req.Path)
	}
	want := map[string]bool{
		"pd /mmr/v1/players/" + fakeriot.DefaultSelf:                         true,
		"pd /match-history/v1/history/" + fakeriot.DefaultSelf:               true,
		"pd /mmr/v1/players/" + fakeriot.DefaultSelf + "/competitiveupdates": true,
		"shared /content-service/v3/content":                                 true,
	}
	if len(got) != len(want) {
		t.Fatalf("got requests %v, want exactly %v", got, want)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected request %s", g)
		}
	}
}
