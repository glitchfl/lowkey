package stats

import "time"

// ResetBoundary is the most recent reset hour in the clock's own time zone
func ResetBoundary(now time.Time, hour int) time.Time {
	b := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if now.Before(b) {
		b = b.AddDate(0, 0, -1)
	}
	return b
}

type Session struct {
	Wins   int
	Losses int
	RR     int
}

// SessionOf counts competitive updates since the boundary - results come from match details and draws count for neither
func SessionOf(self string, updates []Update, since time.Time, matches map[string]*Match) Session {
	var s Session
	for _, u := range updates {
		if u.Start.Before(since) {
			continue
		}
		s.RR += u.Earned
		m := matches[u.MatchID]
		if m == nil || m.Players[self] == nil {
			continue
		}
		switch Result(m, m.Players[self].Team) {
		case "W":
			s.Wins++
		case "L":
			s.Losses++
		}
	}
	return s
}

// Result is the match from one team's side - W or L or D when nobody won
func Result(m *Match, team string) string {
	if m.Won[team] {
		return "W"
	}
	for t, won := range m.Won {
		if t != team && won {
			return "L"
		}
	}
	return "D"
}

func Trend(updates []Update, n int) (int, bool) {
	if len(updates) == 0 {
		return 0, false
	}
	if len(updates) > n {
		updates = updates[:n]
	}
	sum := 0
	for _, u := range updates {
		sum += u.Earned
	}
	return sum, true
}
