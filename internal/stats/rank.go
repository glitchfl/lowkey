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
	// All is every act newest first
	All []Act
}

type Act struct {
	ID string
	// Label is short like V26 A6 or E4 A3 - act names repeat in every episode
	Label string
	Start time.Time
}

type contentSeason struct {
	ID        string    `json:"ID"`
	Name      string    `json:"Name"`
	Type      string    `json:"Type"`
	StartTime time.Time `json:"StartTime"`
	EndTime   time.Time `json:"EndTime"`
	IsActive  bool      `json:"IsActive"`
}

// DecodeContent finds the active act - episodes are flagged active too and names repeat
func DecodeContent(body []byte) (Acts, error) {
	var v struct {
		Seasons []contentSeason `json:"Seasons"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Acts{}, fmt.Errorf("content isn't JSON: %w", err)
	}
	var acts, episodes []contentSeason
	for _, s := range v.Seasons {
		switch {
		case strings.EqualFold(s.Type, "act"):
			acts = append(acts, s)
		case strings.EqualFold(s.Type, "episode"):
			episodes = append(episodes, s)
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
		for j := len(acts) - 1; j >= 0; j-- {
			a := acts[j]
			out.All = append(out.All, Act{ID: strings.ToLower(a.ID), Label: actLabel(a, episodes), Start: a.StartTime})
		}
		return out, nil
	}
	return Acts{}, fmt.Errorf("content lists no active act")
}

// actLabel shortens EPISODE 4 + ACT III to E4 A3 and V26 + ACT VI to V26 A6 - an act outside every
// episode goes by its year
func actLabel(act contentSeason, episodes []contentSeason) string {
	name := strings.ToUpper(strings.TrimSpace(act.Name))
	if n, ok := roman(strings.TrimSpace(strings.TrimPrefix(name, "ACT"))); ok {
		name = "A" + strconv.Itoa(n)
	}
	episode := strconv.Itoa(act.StartTime.Year())
	for _, e := range episodes {
		if !act.StartTime.Before(e.StartTime) && act.StartTime.Before(e.EndTime) {
			episode = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(e.Name)), " ", "")
			episode = strings.Replace(episode, "EPISODE", "E", 1)
			break
		}
	}
	return episode + " " + name
}

func roman(s string) (int, bool) {
	values := map[rune]int{'I': 1, 'V': 5, 'X': 10}
	total, prev := 0, 0
	for i := len(s) - 1; i >= 0; i-- {
		v, ok := values[rune(s[i])]
		if !ok {
			return 0, false
		}
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	return total, total > 0
}

type Season struct {
	Tier               int            `json:"CompetitiveTier"`
	RR                 int            `json:"RankedRating"`
	Wins               int            `json:"NumberOfWins"`
	WinsWithPlacements int            `json:"NumberOfWinsWithPlacements"`
	Games              int            `json:"NumberOfGames"`
	WinsByTier         map[string]int `json:"WinsByTier"`
}

// MMR holds seasons by queue and then act - act IDs lowercased
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
		r.Wins, r.Losses = record(s)
		r.HasRecord = true
	}
	return r
}

// WinRate is a whole percentage
func (r Rank) WinRate() int {
	return int(r.WinRatio()*100 + 0.5)
}

func (r Rank) WinRatio() float64 {
	games := r.Wins + r.Losses
	if games == 0 {
		return 0
	}
	return float64(r.Wins) / float64(games)
}

type Update struct {
	MatchID string
	Start   time.Time
	Earned  int
}

func DecodeUpdates(body []byte) ([]Update, error) {
	var v struct {
		Matches []struct {
			MatchID            string `json:"MatchID"`
			MatchStartTime     int64  `json:"MatchStartTime"`
			RankedRatingEarned int    `json:"RankedRatingEarned"`
		} `json:"Matches"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("competitive updates aren't JSON: %w", err)
	}
	out := make([]Update, 0, len(v.Matches))
	for _, m := range v.Matches {
		out = append(out, Update{MatchID: strings.ToLower(m.MatchID), Start: time.UnixMilli(m.MatchStartTime), Earned: m.RankedRatingEarned})
	}
	return out, nil
}
