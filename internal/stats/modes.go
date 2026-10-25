package stats

import "strings"

type Mode struct {
	Key   string // the name lowkey.toml uses
	Queue string
	Name  string
	// deathmatch and TDM have no rounds worth averaging - ACS ADR and FB% show "-" there
	Rounds  bool
	Teams   bool
	WinRate bool
}

var Modes = []Mode{
	{Key: "competitive", Queue: "competitive", Name: "Competitive", Rounds: true, Teams: true, WinRate: true},
	{Key: "premier", Queue: "premier", Name: "Premier", Rounds: true, Teams: true, WinRate: true},
	{Key: "unrated", Queue: "unrated", Name: "Unrated", Rounds: true, Teams: true, WinRate: true},
	{Key: "swiftplay", Queue: "swiftplay", Name: "Swiftplay", Rounds: true, Teams: true, WinRate: true},
	{Key: "deathmatch", Queue: "deathmatch", Name: "Deathmatch"},
	{Key: "team_deathmatch", Queue: "hurm", Name: "TDM", Teams: true, WinRate: true},
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
