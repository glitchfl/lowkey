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
	Start   time.Time
	Players map[string]*MatchPlayer
	Won     map[string]bool
}

type MatchPlayer struct {
	PUUID       string
	Team        string
	Agent       string
	Score       int
	Rounds      int
	Kills       int
	Deaths      int
	Head        int
	Body        int
	Leg         int
	Damage      int
	FirstBloods int
}

type kill struct {
	Killer string `json:"killer"`
	// community docs call it roundTime - the spec and older captures say timeSinceRoundStartMillis
	RoundTime  *int64 `json:"roundTime"`
	SinceStart *int64 `json:"timeSinceRoundStartMillis"`
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

func DecodeMatch(body []byte) (*Match, error) {
	var v struct {
		MatchInfo struct {
			MatchID         string `json:"matchId"`
			QueueID         string `json:"queueId"`
			GameStartMillis int64  `json:"gameStartMillis"`
		} `json:"matchInfo"`
		Players []struct {
			Subject     string `json:"subject"`
			TeamID      string `json:"teamId"`
			CharacterID string `json:"characterId"`
			Stats       *struct {
				Score        int `json:"score"`
				RoundsPlayed int `json:"roundsPlayed"`
				Kills        int `json:"kills"`
				Deaths       int `json:"deaths"`
			} `json:"stats"`
		} `json:"players"`
		Teams []struct {
			TeamID string `json:"teamId"`
			Won    bool   `json:"won"`
		} `json:"teams"`
		RoundResults []struct {
			PlayerStats []struct {
				Subject string `json:"subject"`
				Kills   []kill `json:"kills"`
				Damage  []struct {
					Damage    int `json:"damage"`
					Headshots int `json:"headshots"`
					Bodyshots int `json:"bodyshots"`
					Legshots  int `json:"legshots"`
				} `json:"damage"`
			} `json:"playerStats"`
		} `json:"roundResults"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("match details aren't JSON: %w", err)
	}
	m := &Match{
		ID:      strings.ToLower(v.MatchInfo.MatchID),
		Queue:   v.MatchInfo.QueueID,
		Start:   time.UnixMilli(v.MatchInfo.GameStartMillis),
		Players: map[string]*MatchPlayer{},
		Won:     map[string]bool{},
	}
	for _, p := range v.Players {
		mp := &MatchPlayer{
			PUUID: strings.ToLower(p.Subject), Team: p.TeamID, Agent: strings.ToLower(p.CharacterID),
		}
		if p.Stats != nil {
			mp.Score, mp.Rounds, mp.Kills, mp.Deaths = p.Stats.Score, p.Stats.RoundsPlayed, p.Stats.Kills, p.Stats.Deaths
		}
		m.Players[mp.PUUID] = mp
	}
	for _, t := range v.Teams {
		m.Won[t.TeamID] = t.Won
	}
	for _, round := range v.RoundResults {
		first, firstAt := "", int64(math.MaxInt64)
		for _, ps := range round.PlayerStats {
			subject := strings.ToLower(ps.Subject)
			if mp := m.Players[subject]; mp != nil {
				for _, d := range ps.Damage {
					mp.Damage += d.Damage
					mp.Head += d.Headshots
					mp.Body += d.Bodyshots
					mp.Leg += d.Legshots
				}
			}
			for _, k := range ps.Kills {
				at, ok := k.at()
				if !ok || at >= firstAt {
					continue
				}
				first, firstAt = subject, at
				if k.Killer != "" {
					first = strings.ToLower(k.Killer)
				}
			}
		}
		if mp := m.Players[first]; mp != nil {
			mp.FirstBloods++
		}
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
	Kills, Deaths, Score, Rounds        int
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
		s.Kills += p.Kills
		s.Deaths += p.Deaths
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
