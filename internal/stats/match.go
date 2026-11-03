package stats

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Match keeps the few numbers lowkey needs from match details - the rest is dropped while decoding
type Match struct {
	ID      string
	Queue   string
	Map     string // the map URL like the rosters send
	Start   time.Time
	Players map[string]*MatchPlayer
	Won     map[string]bool
	// RoundsWon is by team - deathmatch teams are single players
	RoundsWon map[string]int
	Rounds    []Round
}

type MatchPlayer struct {
	PUUID       string
	Team        string
	PartyID     string
	Agent       string
	Score       int
	Rounds      int
	Kills       int
	Deaths      int
	Assists     int
	Head        int
	Body        int
	Leg         int
	Damage      int
	FirstBloods int
	// KillsWith counts kills by the weapon ID that finished them - other kills go by their damage type
	KillsWith map[string]int
	// Loadouts counts the rounds they started holding each weapon - by its ID
	Loadouts map[string]int
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Round is who attacked and where players stood - Attackers is empty when the result doesn't say
type Round struct {
	Attackers string
	Plant     string // the site the spike went down on - empty without a plant
	// Spots is each player's last known position in the first EarlyRound of it
	Spots   map[string]Point
	Opening *Opening // the round's first kill - nil when nobody died
}

type Opening struct {
	Killer, Victim     string
	KillerAt, VictimAt Point
}

// EarlyRound is how long into a round a position still shows where a player set up
const EarlyRound = 25 * time.Second

type spot struct {
	Subject  string `json:"subject"`
	Location Point  `json:"location"`
}

type kill struct {
	Killer string `json:"killer"`
	Victim string `json:"victim"`
	// community docs call it roundTime - the spec and older captures say timeSinceRoundStartMillis
	RoundTime  *int64 `json:"roundTime"`
	SinceStart *int64 `json:"timeSinceRoundStartMillis"`

	VictimLocation  Point  `json:"victimLocation"`
	PlayerLocations []spot `json:"playerLocations"`
	FinishingDamage struct {
		DamageType string `json:"damageType"`
		DamageItem string `json:"damageItem"`
	} `json:"finishingDamage"`
}

func (k kill) at() (int64, bool) {
	switch {
	case k.RoundTime != nil:
		return *k.RoundTime, true
	case k.SinceStart != nil:
		return *k.SinceStart, true
	}
	return 0, false
}

// finisher is the weapon's ID for a gun kill and the damage type for anything else - an ability or the spike
func (k kill) finisher() string {
	if k.FinishingDamage.DamageType == "Weapon" && k.FinishingDamage.DamageItem != "" {
		return strings.ToLower(k.FinishingDamage.DamageItem)
	}
	return strings.ToLower(k.FinishingDamage.DamageType)
}

// opening falls back to where the victim fell when the killer's own spot is missing
func (k kill) opening() *Opening {
	o := &Opening{Killer: strings.ToLower(k.Killer), Victim: strings.ToLower(k.Victim), VictimAt: k.VictimLocation}
	o.KillerAt = o.VictimAt
	for _, s := range k.PlayerLocations {
		if strings.EqualFold(s.Subject, k.Killer) {
			o.KillerAt = s.Location
		}
	}
	return o
}

// attackers reads the attacking team off the round's winner and the side it won on
func attackers(winner, role string, teams []string) string {
	switch role {
	case "Attacker":
		return winner
	case "Defender":
		if len(teams) == 2 {
			for _, t := range teams {
				if t != winner {
					return t
				}
			}
		}
	}
	return ""
}

func DecodeMatch(body []byte) (*Match, error) {
	var v struct {
		MatchInfo struct {
			MatchID         string `json:"matchId"`
			QueueID         string `json:"queueId"`
			MapID           string `json:"mapId"`
			GameStartMillis int64  `json:"gameStartMillis"`
		} `json:"matchInfo"`
		Players []struct {
			Subject     string `json:"subject"`
			TeamID      string `json:"teamId"`
			PartyID     string `json:"partyId"`
			CharacterID string `json:"characterId"`
			Stats       *struct {
				Score        int `json:"score"`
				RoundsPlayed int `json:"roundsPlayed"`
				Kills        int `json:"kills"`
				Deaths       int `json:"deaths"`
				Assists      int `json:"assists"`
			} `json:"stats"`
		} `json:"players"`
		Teams []struct {
			TeamID    string `json:"teamId"`
			Won       bool   `json:"won"`
			RoundsWon int    `json:"roundsWon"`
		} `json:"teams"`
		RoundResults []struct {
			WinningTeam          string `json:"winningTeam"`
			WinningTeamRole      string `json:"winningTeamRole"`
			PlantSite            string `json:"plantSite"`
			PlantRoundTime       int64  `json:"plantRoundTime"`
			PlantPlayerLocations []spot `json:"plantPlayerLocations"`
			PlayerStats          []struct {
				Subject string `json:"subject"`
				Kills   []kill `json:"kills"`
				Damage  []struct {
					Damage    int `json:"damage"`
					Headshots int `json:"headshots"`
					Bodyshots int `json:"bodyshots"`
					Legshots  int `json:"legshots"`
				} `json:"damage"`
				Economy *struct {
					Weapon string `json:"weapon"`
				} `json:"economy"`
			} `json:"playerStats"`
		} `json:"roundResults"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("match details aren't JSON: %w", err)
	}
	m := &Match{
		ID:        strings.ToLower(v.MatchInfo.MatchID),
		Queue:     v.MatchInfo.QueueID,
		Map:       v.MatchInfo.MapID,
		Start:     time.UnixMilli(v.MatchInfo.GameStartMillis),
		Players:   map[string]*MatchPlayer{},
		Won:       map[string]bool{},
		RoundsWon: map[string]int{},
	}
	for _, p := range v.Players {
		mp := &MatchPlayer{
			PUUID: strings.ToLower(p.Subject), Team: p.TeamID, PartyID: p.PartyID, Agent: strings.ToLower(p.CharacterID),
			KillsWith: map[string]int{}, Loadouts: map[string]int{},
		}
		if p.Stats != nil {
			mp.Score, mp.Rounds, mp.Kills, mp.Deaths = p.Stats.Score, p.Stats.RoundsPlayed, p.Stats.Kills, p.Stats.Deaths
			mp.Assists = p.Stats.Assists
		}
		m.Players[mp.PUUID] = mp
	}
	var teams []string
	for _, t := range v.Teams {
		m.Won[t.TeamID] = t.Won
		m.RoundsWon[t.TeamID] = t.RoundsWon
		teams = append(teams, t.TeamID)
	}
	early := EarlyRound.Milliseconds()
	for _, round := range v.RoundResults {
		r := Round{Attackers: attackers(round.WinningTeam, round.WinningTeamRole, teams), Plant: round.PlantSite, Spots: map[string]Point{}}
		seen := map[string]int64{}
		saw := func(subject string, at int64, p Point) {
			subject = strings.ToLower(subject)
			if last, ok := seen[subject]; subject != "" && at <= early && (!ok || at >= last) {
				seen[subject], r.Spots[subject] = at, p
			}
		}
		if round.PlantSite != "" {
			for _, s := range round.PlantPlayerLocations {
				saw(s.Subject, round.PlantRoundTime, s.Location)
			}
		}
		var first *kill
		firstAt := int64(math.MaxInt64)
		for _, ps := range round.PlayerStats {
			subject := strings.ToLower(ps.Subject)
			mp := m.Players[subject]
			if mp != nil {
				for _, d := range ps.Damage {
					mp.Damage += d.Damage
					mp.Head += d.Headshots
					mp.Body += d.Bodyshots
					mp.Leg += d.Legshots
				}
				if ps.Economy != nil && ps.Economy.Weapon != "" {
					mp.Loadouts[strings.ToLower(ps.Economy.Weapon)]++
				}
			}
			for i, k := range ps.Kills {
				if k.Killer == "" {
					ps.Kills[i].Killer, k.Killer = subject, subject
				}
				if killer := m.Players[strings.ToLower(k.Killer)]; killer != nil {
					killer.KillsWith[k.finisher()]++
				}
				at, ok := k.at()
				if !ok {
					continue
				}
				for _, s := range k.PlayerLocations {
					saw(s.Subject, at, s.Location)
				}
				saw(k.Victim, at, k.VictimLocation)
				if at < firstAt {
					first, firstAt = &ps.Kills[i], at
				}
			}
		}
		if first != nil {
			r.Opening = first.opening()
			if mp := m.Players[r.Opening.Killer]; mp != nil {
				mp.FirstBloods++
			}
		}
		m.Rounds = append(m.Rounds, r)
	}
	return m, nil
}

// DecodeNames maps PUUIDs to their name#tag - Riot's match details stopped carrying these
func DecodeNames(body []byte) (map[string]string, error) {
	var v []struct {
		Subject  string `json:"Subject"`
		GameName string `json:"GameName"`
		TagLine  string `json:"TagLine"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("the name lookup isn't JSON: %w", err)
	}
	names := make(map[string]string, len(v))
	for _, p := range v {
		if p.GameName != "" {
			names[strings.ToLower(p.Subject)] = p.GameName + "#" + p.TagLine
		}
	}
	return names, nil
}

// Sample sums a player's matches - newest first
type Sample struct {
	N                                   int
	Wins, Losses                        int
	Kills, Deaths, Assists, Score       int
	Rounds                              int
	Head, Body, Leg, Damage, FirstBlood int
	Mains                               []string
}

func SampleOf(puuid string, matches []*Match) Sample {
	var s Sample
	count := map[string]int{}
	lastSeen := map[string]int{}
	for i, m := range matches {
		p := m.Players[puuid]
		if p == nil {
			continue
		}
		s.N++
		switch Result(m, p.Team) {
		case "W":
			s.Wins++
		case "L":
			s.Losses++
		}
		s.Kills += p.Kills
		s.Deaths += p.Deaths
		s.Assists += p.Assists
		s.Score += p.Score
		s.Rounds += p.Rounds
		s.Head += p.Head
		s.Body += p.Body
		s.Leg += p.Leg
		s.Damage += p.Damage
		s.FirstBlood += p.FirstBloods
		if p.Agent != "" {
			if _, ok := count[p.Agent]; !ok {
				lastSeen[p.Agent] = i
			}
			count[p.Agent]++
		}
	}
	for agent := range count {
		s.Mains = append(s.Mains, agent)
	}
	// ties go to the agent played most recently
	sort.Slice(s.Mains, func(i, j int) bool {
		a, b := s.Mains[i], s.Mains[j]
		if count[a] != count[b] {
			return count[a] > count[b]
		}
		return lastSeen[a] < lastSeen[b]
	})
	if len(s.Mains) > 2 {
		s.Mains = s.Mains[:2]
	}
	return s
}

func (s Sample) KD() float64 {
	return float64(s.Kills) / math.Max(1, float64(s.Deaths))
}

// KDA counts assists with kills - the way the game's own scoreboard reads them
func (s Sample) KDA() float64 {
	return float64(s.Kills+s.Assists) / math.Max(1, float64(s.Deaths))
}

func ratio(num, den int, scale float64) (float64, bool) {
	if den == 0 {
		return 0, false
	}
	return float64(num) / float64(den) * scale, true
}

func whole(v float64, ok bool) (int, bool) { return int(math.Round(v)), ok }

// the exact values feed averages - the whole numbers are what a row shows
func (s Sample) HSExact() (float64, bool)  { return ratio(s.Head, s.Head+s.Body+s.Leg, 100) }
func (s Sample) ACSExact() (float64, bool) { return ratio(s.Score, s.Rounds, 1) }
func (s Sample) ADRExact() (float64, bool) { return ratio(s.Damage, s.Rounds, 1) }
func (s Sample) FBExact() (float64, bool)  { return ratio(s.FirstBlood, s.Rounds, 100) }

func (s Sample) HS() (int, bool)  { return whole(s.HSExact()) }
func (s Sample) ACS() (int, bool) { return whole(s.ACSExact()) }
func (s Sample) ADR() (int, bool) { return whole(s.ADRExact()) }
func (s Sample) FB() (int, bool)  { return whole(s.FBExact()) }
