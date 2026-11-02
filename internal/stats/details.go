package stats

import (
	"sort"
	"strconv"
)

// ActRank is one competitive act from a player's rank data - tiers on today's tier set
type ActRank struct {
	Label        string
	Tier         int
	Peak         int
	Wins, Losses int
}

// ActHistory lists every competitive act a player played or ended ranked in - newest first
func ActHistory(m MMR, acts Acts, remap Remap) []ActRank {
	comp := m["competitive"]
	var out []ActRank
	for _, act := range acts.All {
		s, ok := comp[act.ID]
		if !ok || s.Games == 0 && s.Tier < 3 {
			continue
		}
		r := ActRank{Label: act.Label}
		if s.Tier >= 3 {
			r.Tier = remap(act.ID, s.Tier)
			r.Peak = r.Tier
		}
		for key := range s.WinsByTier {
			if tier, err := strconv.Atoi(key); err == nil && tier >= 3 {
				r.Peak = max(r.Peak, remap(act.ID, tier))
			}
		}
		r.Wins, r.Losses = record(s)
		out = append(out, r)
	}
	return out
}

func record(s Season) (wins, losses int) {
	wins = s.WinsWithPlacements
	if wins == 0 {
		wins = s.Wins
	}
	return wins, s.Games - wins
}

type QueueRecord struct {
	Mode         Mode
	Games        int
	Wins, Losses int
}

func ThisAct(m MMR, acts Acts) []QueueRecord {
	var out []QueueRecord
	for _, mode := range Modes {
		s, ok := m[mode.Queue][acts.Current]
		if !ok || s.Games == 0 {
			continue
		}
		r := QueueRecord{Mode: mode, Games: s.Games}
		r.Wins, r.Losses = record(s)
		out = append(out, r)
	}
	return out
}

type AgentSample struct {
	Agent string
	Sample
}

// AgentsOf splits a sample by agent - most played first and ties to the one played most recently
func AgentsOf(puuid string, matches []*Match) []AgentSample {
	byAgent := map[string][]*Match{}
	var order []string
	for _, m := range matches {
		p := m.Players[puuid]
		if p == nil || p.Agent == "" {
			continue
		}
		if _, ok := byAgent[p.Agent]; !ok {
			order = append(order, p.Agent)
		}
		byAgent[p.Agent] = append(byAgent[p.Agent], m)
	}
	out := make([]AgentSample, 0, len(order))
	for _, agent := range order {
		out = append(out, AgentSample{Agent: agent, Sample: SampleOf(puuid, byAgent[agent])})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].N > out[j].N })
	return out
}
