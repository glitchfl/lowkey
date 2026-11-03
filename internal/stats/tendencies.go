package stats

import (
	"math"
	"slices"
	"sort"
)

// Callout is a named spot on a map - Region is the part of the map it's in like A or Mid or a team's spawn
type Callout struct {
	Region string  `json:"region"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

var places = []string{"A", "B", "C", "Mid"}

// Place names the part of the map a point is in by its nearest callout - false near a spawn or on a map
// lowkey has no callouts for
func Place(callouts []Callout, p Point) (string, bool) {
	region, best := "", math.Inf(1)
	for _, c := range callouts {
		if d := math.Hypot(c.X-p.X, c.Y-p.Y); d < best {
			region, best = c.Region, d
		}
	}
	return region, slices.Contains(places, region)
}

type Share struct {
	Place   string
	Percent int
}

// Tally is how often each place came up out of N
type Tally struct {
	N      int
	Shares []Share
}

// tally rounds by largest remainder so the shares add up to 100 - a place that rounds to 0 drops out
func tally(counts map[string]int) Tally {
	var t Tally
	for _, place := range places {
		t.N += counts[place]
	}
	if t.N == 0 {
		return t
	}
	type part struct {
		place              string
		percent, remainder int
	}
	var order []part
	left := 100
	for _, place := range places {
		if n := counts[place]; n > 0 {
			p := part{place: place, percent: n * 100 / t.N, remainder: n * 100 % t.N}
			left -= p.percent
			order = append(order, p)
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].remainder > order[j].remainder })
	for i := 0; i < left; i++ {
		order[i%len(order)].percent++
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].percent != order[j].percent {
			return order[i].percent > order[j].percent
		}
		return slices.Index(places, order[i].place) < slices.Index(places, order[j].place)
	})
	for _, p := range order {
		if p.percent > 0 {
			t.Shares = append(t.Shares, Share{Place: p.place, Percent: p.percent})
		}
	}
	return t
}

// Sites is where a player tends to play one map
type Sites struct {
	Matches int
	// Defense is where they stood early in their defense rounds
	Defense Tally
	// Plants is where their team planted in their attack rounds
	Plants Tally
	// Openings is where they stood for the round's first kill in their attack rounds - theirs or on them
	Openings Tally
}

func SitesOf(puuid, mapURL string, matches []*Match, callouts []Callout) Sites {
	var s Sites
	defense, plants, openings := map[string]int{}, map[string]int{}, map[string]int{}
	for _, m := range matches {
		p := m.Players[puuid]
		if p == nil || m.Map != mapURL {
			continue
		}
		s.Matches++
		for _, r := range m.Rounds {
			switch r.Attackers {
			case "":
			case p.Team:
				if r.Plant != "" {
					plants[r.Plant]++
				}
				if at, ok := r.Opening.spotOf(puuid); ok {
					if place, ok := Place(callouts, at); ok {
						openings[place]++
					}
				}
			default:
				if at, ok := r.Spots[puuid]; ok {
					if place, ok := Place(callouts, at); ok {
						defense[place]++
					}
				}
			}
		}
	}
	s.Defense, s.Plants, s.Openings = tally(defense), tally(plants), tally(openings)
	return s
}

func (o *Opening) spotOf(puuid string) (Point, bool) {
	switch {
	case o == nil:
		return Point{}, false
	case o.Killer == puuid:
		return o.KillerAt, true
	case o.Victim == puuid:
		return o.VictimAt, true
	}
	return Point{}, false
}

const (
	operator = "a03b24d3-4319-996d-0f8c-94bbfba1dfc7"
	judge    = "ec845bf4-4f79-ddda-a3da-0db3774b2794"
	bucky    = "910be174-449b-c412-ab22-d0873436b21b"
)

var riflesAndSnipers = []string{
	"ae3de142-4d85-2547-dd26-4e90bed35cf7", // bulldog
	"4ade7faa-4cf1-8376-95ef-39884480959b", // guardian
	"ee8e8d15-496b-07ac-e5f6-8fae5d4c7b1a", // phantom
	"9c82e19d-4575-0200-1a81-3eacf00cf872", // vandal
	"8db0a1bf-4a50-832a-4566-faaaa6d250ca", // warden
	"c4883e50-4494-202c-3ec3-6b8a9284f00b", // marshal
	"5f0aaf7a-4289-3998-d5ff-eb9a5cf7ef5c", // outlaw
	operator,
}

// Leanings are the guns a player leans on hard enough to say so - nil when they don't
type Leanings struct {
	Matches  int
	Operator *OperatorLean
	Shotguns *ShotgunLean
}

// OperatorLean only counts their matches on a duelist
type OperatorLean struct {
	Percent int // of their rifle and sniper kills
	Rounds  int // they started holding one
}

type ShotgunLean struct {
	Guns    []string // the ones they got kills with - most first
	Kills   int
	Percent int // of all their kills
}

func LeaningsOf(puuid string, matches []*Match, duelist func(agent string) bool) Leanings {
	var l Leanings
	var opKills, longKills, opRounds, kills int
	shotguns := map[string]int{}
	for _, m := range matches {
		p := m.Players[puuid]
		if p == nil {
			continue
		}
		l.Matches++
		for id, n := range p.KillsWith {
			kills += n
			switch id {
			case judge:
				shotguns["Judge"] += n
			case bucky:
				shotguns["Bucky"] += n
			}
		}
		if !duelist(p.Agent) {
			continue
		}
		opKills += p.KillsWith[operator]
		opRounds += p.Loadouts[operator]
		for _, id := range riflesAndSnipers {
			longKills += p.KillsWith[id]
		}
	}
	if opKills*100 > 25*longKills || opRounds >= 3 {
		l.Operator = &OperatorLean{Percent: percent(opKills, longKills), Rounds: opRounds}
	}
	if n := shotguns["Judge"] + shotguns["Bucky"]; n*100 > 20*kills || n >= 15 {
		l.Shotguns = &ShotgunLean{Kills: n, Percent: percent(n, kills)}
		for _, gun := range []string{"Judge", "Bucky"} {
			if shotguns[gun] > 0 {
				l.Shotguns.Guns = append(l.Shotguns.Guns, gun)
			}
		}
		sort.SliceStable(l.Shotguns.Guns, func(i, j int) bool { return shotguns[l.Shotguns.Guns[i]] > shotguns[l.Shotguns.Guns[j]] })
	}
	return l
}

func percent(n, of int) int {
	if of == 0 {
		return 0
	}
	return int(math.Round(float64(n) * 100 / float64(of)))
}
