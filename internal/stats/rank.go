package stats

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Acts struct {
	Current string
	Last    string
}

// DecodeContent finds the active act - episodes are flagged active too and names repeat
func DecodeContent(body []byte) (Acts, error) {
	var v struct {
		Seasons []struct {
			ID        string    `json:"ID"`
			Type      string    `json:"Type"`
			StartTime time.Time `json:"StartTime"`
			IsActive  bool      `json:"IsActive"`
		} `json:"Seasons"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Acts{}, fmt.Errorf("content isn't JSON: %w", err)
	}
	acts := v.Seasons[:0]
	for _, s := range v.Seasons {
		if strings.EqualFold(s.Type, "act") {
			acts = append(acts, s)
		}
	}
	sort.Slice(acts, func(i, j int) bool { return acts[i].StartTime.Before(acts[j].StartTime) })
	for i, s := range acts {
		if !s.IsActive {
			continue
		}
		out := Acts{Current: strings.ToLower(s.ID)}
		if i > 0 {
			out.Last = strings.ToLower(acts[i-1].ID)
		}
		return out, nil
	}
	return Acts{}, fmt.Errorf("content lists no active act")
}

type Season struct {
	Tier               int            `json:"CompetitiveTier"`
	RR                 int            `json:"RankedRating"`
	Wins               int            `json:"NumberOfWins"`
	WinsWithPlacements int            `json:"NumberOfWinsWithPlacements"`
	Games              int            `json:"NumberOfGames"`
	WinsByTier         map[string]int `json:"WinsByTier"`
}

// MMR holds seasons by queue and then act, act IDs lowercased
type MMR map[string]map[string]Season

func DecodeMMR(body []byte) (MMR, error) {
	var v struct {
		QueueSkills map[string]struct {
			SeasonalInfoBySeasonID map[string]Season `json:"SeasonalInfoBySeasonID"`
		} `json:"QueueSkills"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("rank data isn't JSON: %w", err)
	}
	m := MMR{}
	for queue, skills := range v.QueueSkills {
		m[queue] = map[string]Season{}
		for act, s := range skills.SeasonalInfoBySeasonID {
			m[queue][strings.ToLower(act)] = s
		}
	}
	return m, nil
}

// Remap moves an act's tier onto today's tier set
type Remap func(act string, tier int) int

type Rank struct {
	Tier int // competitive this act - below 3 is unranked
	RR   int
	Peak int
	Last int
	// the chosen queue this act
	Wins      int
	Losses    int
	HasRecord bool
}

func RankOf(m MMR, acts Acts, queue string, remap Remap) Rank {
	var r Rank
	comp := m["competitive"]
	if cur, ok := comp[acts.Current]; ok {
		r.Tier, r.RR = cur.Tier, cur.RR
	}
	if last, ok := comp[acts.Last]; ok && last.Tier >= 3 {
		r.Last = remap(acts.Last, last.Tier)
	}
	if r.Tier >= 3 {
		r.Peak = r.Tier
	}
	for act, s := range comp {
		for key := range s.WinsByTier {
			tier, err := strconv.Atoi(key)
			if err != nil || tier < 3 {
				continue
			}
			if t := remap(act, tier); t > r.Peak {
				r.Peak = t
			}
		}
	}
	if s, ok := m[queue][acts.Current]; ok && s.Games > 0 {
		wins := s.WinsWithPlacements
		if wins == 0 {
			wins = s.Wins
		}
		r.Wins, r.Losses, r.HasRecord = wins, s.Games-wins, true
	}
	return r
}

// WinRate is a whole percentage
func (r Rank) WinRate() int {
	games := r.Wins + r.Losses
	if games == 0 {
		return 0
	}
	return int(float64(r.Wins)/float64(games)*100 + 0.5)
}
