package app

import (
	"fmt"
	"time"

	"github.com/glitchfl/lowkey/internal/stats"
)

// pastLobby is a lobby that ended this session - kept in memory only, so a restart forgets it
type pastLobby struct {
	matchID string
	at      time.Time
	players []pastPlayer
}

type pastPlayer struct {
	puuid string
	ally  bool
}

type encounter struct {
	ally   bool
	result string // W, L, D or empty when lowkey never saw how it ended
	at     time.Time
}

// remember stores a finished or dodged lobby - streamer-mode players are left out for good
func (lv *live) remember(lob *lobby) {
	if lob == nil || !lob.known {
		return
	}
	past := pastLobby{matchID: lob.matchID, at: lob.started}
	for _, m := range lob.members {
		if m.hidden {
			lv.everHidden[m.puuid] = true
			continue
		}
		if m.puuid != lv.self {
			past.players = append(past.players, pastPlayer{puuid: m.puuid, ally: m.team == lob.ownTeam})
		}
	}
	lv.history = append(lv.history, past)
}

// metIndex keeps each player's most recent encounter from matches lowkey already has and earlier lobbies -
// the match on screen never counts as meeting its own players
func (lv *live) metIndex(shown string) map[string]encounter {
	idx := map[string]encounter{}
	add := func(puuid string, e encounter) {
		if puuid == lv.self || lv.everHidden[puuid] {
			return
		}
		if old, ok := idx[puuid]; !ok || e.at.After(old.at) {
			idx[puuid] = e
		}
	}
	for _, m := range lv.eng.matches {
		me := m.Players[lv.self]
		if me == nil || m.ID == shown {
			continue
		}
		res := resultFor(m, me.Team)
		for puuid, p := range m.Players {
			add(puuid, encounter{ally: p.Team == me.Team, result: res, at: m.Start})
		}
	}
	for _, past := range lv.history {
		if past.matchID == shown {
			continue
		}
		res := ""
		if m, ok := lv.eng.matches[past.matchID]; ok && m.Players[lv.self] != nil {
			res = resultFor(m, m.Players[lv.self].Team)
		}
		for _, p := range past.players {
			add(p.puuid, encounter{ally: p.ally, result: res, at: past.at})
		}
	}
	return idx
}

func resultFor(m *stats.Match, team string) string {
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

func (e encounter) text(now time.Time) string {
	rel := "enemy"
	if e.ally {
		rel = "ally"
	}
	if e.result != "" {
		rel += " " + e.result
	}
	return rel + " " + ago(now.Sub(e.at))
}

func ago(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", max(0, int(d.Minutes())))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
