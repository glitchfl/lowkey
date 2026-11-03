package stats_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

// three swiftplay matches from the 2026-10-27 discovery session - already scrubbed
const (
	havenMatch  = "09-match1-swift-match-details.json"
	iceboxMatch = "12-match2-swift-match-details.json"
	sunsetMatch = "14-match3-swift-match-details.json"
	self        = "5e1f5e1f-5e1f-4e1f-8e1f-5e1f5e1f5e1f"
)

const (
	mapHaven  = "/Game/Maps/Triad/Triad"
	mapIcebox = "/Game/Maps/Port/Port"
	mapSunset = "/Game/Maps/Juliett/Juliett"
)

func sessionMatch(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "sessions", "2026-10-27-discovery", "extras", "raw", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func sessionMatches(t *testing.T) []*stats.Match {
	t.Helper()
	var out []*stats.Match
	for _, name := range []string{havenMatch, iceboxMatch, sunsetMatch} {
		m, err := stats.DecodeMatch(sessionMatch(t, name))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func fake(n int) string { return fmt.Sprintf("fa4e0000-0000-4000-8000-%012d", n) }

func TestEveryPlantIsPlacedOnTheSiteItWentDownOn(t *testing.T) {
	data := static.Embedded()
	plants := 0
	for _, name := range []string{havenMatch, iceboxMatch, sunsetMatch} {
		var v struct {
			MatchInfo struct {
				MapID string `json:"mapId"`
			} `json:"matchInfo"`
			RoundResults []struct {
				PlantSite     string      `json:"plantSite"`
				PlantLocation stats.Point `json:"plantLocation"`
			} `json:"roundResults"`
		}
		if err := json.Unmarshal(sessionMatch(t, name), &v); err != nil {
			t.Fatal(err)
		}
		callouts := data.Callouts(v.MatchInfo.MapID)
		for i, r := range v.RoundResults {
			if r.PlantSite == "" {
				continue
			}
			plants++
			if got, ok := stats.Place(callouts, r.PlantLocation); !ok || got != r.PlantSite {
				t.Errorf("%s round %d: a plant on %s was placed on %q", name, i, r.PlantSite, got)
			}
		}
	}
	if plants != 17 {
		t.Errorf("checked %d plants, the three matches have 17", plants)
	}
}

func TestEveryCompetitiveMapHasItsSites(t *testing.T) {
	data := static.Embedded()
	maps := map[string][]string{
		"/Game/Maps/Ascent/Ascent": {"A", "B", "Mid"}, "/Game/Maps/Bonsai/Bonsai": {"A", "B", "Mid"},
		"/Game/Maps/Canyon/Canyon": {"A", "B"}, "/Game/Maps/Duality/Duality": {"A", "B"},
		"/Game/Maps/Foxtrot/Foxtrot": {"A", "B", "Mid"}, "/Game/Maps/Infinity/Infinity": {"A", "B", "Mid"},
		"/Game/Maps/Jam/Jam": {"A", "B", "C"}, "/Game/Maps/Juliett/Juliett": {"A", "B", "Mid"},
		"/Game/Maps/Pitt/Pitt": {"A", "B", "Mid"}, "/Game/Maps/Plummet/Plummet": {"A", "B", "Mid"},
		"/Game/Maps/Port/Port": {"A", "B", "Mid"}, "/Game/Maps/Rook/Rook": {"A", "B", "Mid"},
		"/Game/Maps/Triad/Triad": {"A", "B", "C", "Mid"},
	}
	for url, want := range maps {
		var got []string
		for _, c := range data.Callouts(url) {
			if _, ok := stats.Place([]stats.Callout{c}, stats.Point{X: c.X, Y: c.Y}); ok && !slices.Contains(got, c.Region) {
				got = append(got, c.Region)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s has callouts in %v, want %v", url, got, want)
		}
	}
}

func TestASpawnIsNoPlace(t *testing.T) {
	callouts := []stats.Callout{{Region: "A", X: 0, Y: 0}, {Region: "Defender Side", X: 1000, Y: 0}}
	if _, ok := stats.Place(callouts, stats.Point{X: 900, Y: 0}); ok {
		t.Error("a spot next to a spawn shouldn't count as a site")
	}
	if _, ok := stats.Place(nil, stats.Point{}); ok {
		t.Error("a map without callouts has no places")
	}
}

func shares(t stats.Tally) string {
	s := fmt.Sprintf("%d:", t.N)
	for _, sh := range t.Shares {
		s += fmt.Sprintf(" %s %d", sh.Place, sh.Percent)
	}
	return s
}

// the expected counts come from a separate script over the raw JSON - not from this code
func TestSitesOnlyCountMatchesOnTheMap(t *testing.T) {
	matches := sessionMatches(t)
	data := static.Embedded()
	cases := []struct {
		name, mapURL, who             string
		matches                       int
		defense, plants, openingDuels string
	}{
		{"you on Haven", mapHaven, self, 1, "4: A 50 C 50", "4: A 50 B 25 C 25", "3: Mid 100"},
		{"you on Icebox", mapIcebox, self, 1, "4: A 75 B 25", "2: A 50 B 50", "3: B 67 Mid 33"},
		{"you on Sunset", mapSunset, self, 1, "4: B 75 A 25", "4: B 75 A 25", "4: A 75 B 25"},
		{"a sage who held C all game", mapHaven, fake(257), 1, "5: C 100", "2: A 50 C 50", "0:"},
		{"three ways on defense", mapHaven, fake(252), 1, "5: B 40 C 40 A 20", "2: A 50 C 50", "1: C 100"},
		{"a map you didn't play", "/Game/Maps/Ascent/Ascent", self, 0, "0:", "0:", "0:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := stats.SitesOf(c.who, c.mapURL, matches, data.Callouts(c.mapURL))
			if s.Matches != c.matches {
				t.Errorf("counted %d matches, want %d", s.Matches, c.matches)
			}
			if got := shares(s.Defense); got != c.defense {
				t.Errorf("defense %q, want %q", got, c.defense)
			}
			if got := shares(s.Plants); got != c.plants {
				t.Errorf("plants %q, want %q", got, c.plants)
			}
			if got := shares(s.Openings); got != c.openingDuels {
				t.Errorf("opening duels %q, want %q", got, c.openingDuels)
			}
		})
	}
}

func TestSharesAlwaysAddUpTo100(t *testing.T) {
	matches := sessionMatches(t)
	data := static.Embedded()
	for _, m := range matches {
		for puuid := range m.Players {
			s := stats.SitesOf(puuid, m.Map, matches, data.Callouts(m.Map))
			for _, tally := range []stats.Tally{s.Defense, s.Plants, s.Openings} {
				sum := 0
				for _, sh := range tally.Shares {
					sum += sh.Percent
				}
				if tally.N > 0 && sum != 100 {
					t.Errorf("%s on %s: %s adds up to %d", puuid, m.Map, shares(tally), sum)
				}
			}
		}
	}
	thirds := &stats.Match{Map: mapHaven, Players: map[string]*stats.MatchPlayer{"p": {Team: "Blue"}}}
	for _, place := range []stats.Point{{X: 0}, {X: 100}, {X: 200}} {
		thirds.Rounds = append(thirds.Rounds, stats.Round{Attackers: "Red", Spots: map[string]stats.Point{"p": place}})
	}
	callouts := []stats.Callout{{Region: "A", X: 0}, {Region: "B", X: 100}, {Region: "C", X: 200}}
	if got := shares(stats.SitesOf("p", mapHaven, []*stats.Match{thirds}, callouts).Defense); got != "3: A 34 B 33 C 33" {
		t.Errorf("three even thirds came out %q", got)
	}
}

func TestOperatorLeaningsOnRealMatches(t *testing.T) {
	matches := sessionMatches(t)
	data := static.Embedded()
	cases := []struct {
		name, who string
		want      *stats.OperatorLean
	}{
		{"a jett whose one rifle kill was an operator", fake(317), &stats.OperatorLean{Percent: 100, Rounds: 2}},
		{"a jett with one operator round", fake(345), &stats.OperatorLean{Percent: 100, Rounds: 1}},
		{"a clove with an operator isn't on a duelist", fake(253), nil},
		{"a reyna who never touched one", self, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := stats.LeaningsOf(c.who, matches, data.Duelist)
			switch {
			case c.want == nil && l.Operator != nil:
				t.Errorf("got an operator leaning %+v", *l.Operator)
			case c.want != nil && (l.Operator == nil || *l.Operator != *c.want):
				t.Errorf("got %+v, want %+v", l.Operator, *c.want)
			}
			if l.Shotguns != nil {
				t.Errorf("nobody in these matches got a shotgun kill, got %+v", *l.Shotguns)
			}
		})
	}
	if l := stats.LeaningsOf(self, matches, data.Duelist); l.Matches != 3 {
		t.Errorf("leanings counted %d matches, want all 3", l.Matches)
	}
}

const (
	vandal   = "9c82e19d-4575-0200-1a81-3eacf00cf872"
	operator = "a03b24d3-4319-996d-0f8c-94bbfba1dfc7"
	judge    = "ec845bf4-4f79-ddda-a3da-0db3774b2794"
	bucky    = "910be174-449b-c412-ab22-d0873436b21b"
	classic  = "29a0cfab-485b-f5d5-779a-b59f85e204a8"
	jett     = "add6443a-41bd-e414-f6ad-e58d267f4e95"
	sova     = "320b2a48-4d9b-a075-30f1-1f93a9b638fa"
)

func played(agent string, kills map[string]int, loadouts map[string]int) *stats.Match {
	return &stats.Match{Players: map[string]*stats.MatchPlayer{"p": {Agent: agent, KillsWith: kills, Loadouts: loadouts}}}
}

func TestOperatorThresholds(t *testing.T) {
	cases := []struct {
		name    string
		matches []*stats.Match
		want    *stats.OperatorLean
	}{
		{"exactly a quarter isn't more than one", []*stats.Match{played(jett, map[string]int{operator: 5, vandal: 15}, nil)}, nil},
		{"just over a quarter", []*stats.Match{played(jett, map[string]int{operator: 26, vandal: 74}, nil)}, &stats.OperatorLean{Percent: 26}},
		{"pistol kills don't count against it", []*stats.Match{played(jett, map[string]int{operator: 3, vandal: 6, classic: 30}, nil)}, &stats.OperatorLean{Percent: 33}},
		{"three rounds with one", []*stats.Match{played(jett, nil, map[string]int{operator: 3})}, &stats.OperatorLean{Rounds: 3}},
		{"two rounds with one", []*stats.Match{played(jett, nil, map[string]int{operator: 2})}, nil},
		{"rounds add up over matches", []*stats.Match{
			played(jett, nil, map[string]int{operator: 2}), played(jett, nil, map[string]int{operator: 1}),
		}, &stats.OperatorLean{Rounds: 3}},
		{"not on a duelist", []*stats.Match{played(sova, map[string]int{operator: 20}, map[string]int{operator: 9})}, nil},
		{"only the duelist matches count", []*stats.Match{
			played(sova, map[string]int{operator: 20}, map[string]int{operator: 9}), played(jett, map[string]int{operator: 1, vandal: 9}, nil),
		}, nil},
	}
	duelist := func(agent string) bool { return agent == jett }
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := stats.LeaningsOf("p", c.matches, duelist)
			switch {
			case c.want == nil && l.Operator != nil:
				t.Errorf("got an operator leaning %+v", *l.Operator)
			case c.want != nil && (l.Operator == nil || *l.Operator != *c.want):
				t.Errorf("got %+v, want %+v", l.Operator, *c.want)
			}
		})
	}
}

func TestShotgunThresholds(t *testing.T) {
	cases := []struct {
		name  string
		kills map[string]int
		want  *stats.ShotgunLean
	}{
		{"exactly a fifth isn't more than one", map[string]int{judge: 10, vandal: 40}, nil},
		{"just over a fifth", map[string]int{judge: 21, vandal: 79}, &stats.ShotgunLean{Guns: []string{"Judge"}, Kills: 21, Percent: 21}},
		{"fifteen kills however many else", map[string]int{bucky: 15, vandal: 200}, &stats.ShotgunLean{Guns: []string{"Bucky"}, Kills: 15, Percent: 7}},
		{"fourteen kills isn't enough", map[string]int{bucky: 14, vandal: 200}, nil},
		{"both shotguns most first", map[string]int{bucky: 9, judge: 12, vandal: 20, "ability": 4}, &stats.ShotgunLean{Guns: []string{"Judge", "Bucky"}, Kills: 21, Percent: 47}},
		{"no kills at all", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := stats.LeaningsOf("p", []*stats.Match{played(sova, c.kills, nil)}, func(string) bool { return false })
			switch {
			case c.want == nil && l.Shotguns != nil:
				t.Errorf("got a shotgun leaning %+v", *l.Shotguns)
			case c.want != nil && (l.Shotguns == nil || !slices.Equal(l.Shotguns.Guns, c.want.Guns) ||
				l.Shotguns.Kills != c.want.Kills || l.Shotguns.Percent != c.want.Percent):
				t.Errorf("got %+v, want %+v", l.Shotguns, *c.want)
			}
		})
	}
}
