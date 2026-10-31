package stats

import "strings"

type Mode struct {
	Key   string // the name lowkey.toml uses
	Queue string
	Name  string
	// deathmatch and TDM have no rounds worth averaging - ACS ADR and FB% show "-" there
	Rounds  bool
	Half    int // rounds before the teams swap sides
	Teams   bool
	WinRate bool
}

var Modes = []Mode{
	{Key: "competitive", Queue: "competitive", Name: "Competitive", Rounds: true, Half: 12, Teams: true, WinRate: true},
	{Key: "premier", Queue: "premier", Name: "Premier", Rounds: true, Half: 12, Teams: true, WinRate: true},
	{Key: "unrated", Queue: "unrated", Name: "Unrated", Rounds: true, Half: 12, Teams: true, WinRate: true},
	{Key: "swiftplay", Queue: "swiftplay", Name: "Swiftplay", Rounds: true, Half: 4, Teams: true, WinRate: true},
	{Key: "deathmatch", Queue: "deathmatch", Name: "Deathmatch"},
	{Key: "team_deathmatch", Queue: "hurm", Name: "TDM", Teams: true, WinRate: true},
}

// Swapped is whether the next round is played on the other side from the start - overtime goes back
// to the starting side and then swaps every round
func (m Mode) Swapped(played int) bool {
	if played < 2*m.Half {
		return played >= m.Half
	}
	return (played-2*m.Half)%2 == 1
}

func ModeByKey(key string) (Mode, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, m := range Modes {
		if m.Key == key || m.Queue == key {
			return m, true
		}
	}
	return Mode{}, false
}

func ModeByQueue(queue string) (Mode, bool) {
	for _, m := range Modes {
		if m.Queue == queue {
			return m, true
		}
	}
	return Mode{}, false
}

func ModeKeys() []string {
	keys := make([]string, len(Modes))
	for i, m := range Modes {
		keys[i] = m.Key
	}
	return keys
}
