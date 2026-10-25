// Package static is lowkey's built-in knowledge of agents, maps and rank tiers
package static

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

//go:generate go run ./gen snapshot.json

//go:embed snapshot.json
var snapshotJSON []byte

type Tier struct {
	Tier  int    `json:"tier"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type snapshot struct {
	Generated      time.Time         `json:"generated"`
	ClientVersion  string            `json:"clientVersion"`
	Agents         map[string]string `json:"agents"`
	Maps           map[string]string `json:"maps"`
	TierSets       map[string][]Tier `json:"tierSets"`
	CurrentTierSet string            `json:"currentTierSet"`
	Acts           map[string]string `json:"acts"`
	IDs            []string          `json:"ids"`
}

// Data is safe for concurrent use - a refresh can add agents while a redraw reads them
type Data struct {
	mu   sync.RWMutex
	s    snapshot
	ids  map[string]bool
	tier map[int]Tier
}

func Embedded() *Data {
	var s snapshot
	if err := json.Unmarshal(snapshotJSON, &s); err != nil {
		panic("static: broken snapshot.json: " + err.Error())
	}
	d := &Data{s: s, ids: map[string]bool{}, tier: map[int]Tier{}}
	for _, id := range s.IDs {
		d.ids[strings.ToLower(id)] = true
	}
	for id := range s.Agents {
		d.ids[strings.ToLower(id)] = true
	}
	for id, set := range s.Acts {
		d.ids[strings.ToLower(id)] = true
		d.ids[strings.ToLower(set)] = true
	}
	for _, t := range s.TierSets[s.CurrentTierSet] {
		d.tier[t.Tier] = t
	}
	return d
}

func (d *Data) Generated() time.Time { return d.s.Generated }

// Public reports whether a UUID is static game content rather than something tied to a person or match
func (d *Data) Public(uuid string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.ids[strings.ToLower(uuid)]
}

func (d *Data) AgentName(id string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	name, ok := d.s.Agents[strings.ToLower(id)]
	return name, ok
}

func (d *Data) MapName(mapURL string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	name, ok := d.s.Maps[mapURL]
	return name, ok
}

// AddAgents and AddMaps merge a refresh from valorant-api.com - known entries are kept
func (d *Data) AddAgents(agents map[string]string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, name := range agents {
		d.s.Agents[strings.ToLower(id)] = name
		d.ids[strings.ToLower(id)] = true
	}
}

func (d *Data) AddMaps(maps map[string]string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for url, name := range maps {
		d.s.Maps[url] = name
	}
}

// CurrentTier maps an act's tier onto today's tier set
func (d *Data) CurrentTier(act string, tier int) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	set, ok := d.s.Acts[strings.ToLower(act)]
	if !ok || set == d.s.CurrentTierSet {
		return tier
	}
	// episode 1-4 sets topped out at radiant 24 - ascendant pushed immortal and radiant up by 3
	top := 0
	for _, t := range d.s.TierSets[set] {
		if t.Tier > top {
			top = t.Tier
		}
	}
	if top == 24 && tier > 20 {
		return tier + 3
	}
	return tier
}

// TierLabel is the compact rank name like Dia3 - unranked tiers have none
func (d *Data) TierLabel(tier int) string {
	t, ok := d.tier[tier]
	if !ok || tier < 3 {
		return ""
	}
	return compactTier(t.Name)
}

// TierColor is the tier's RRGGBB color from the current set
func (d *Data) TierColor(tier int) string {
	t, ok := d.tier[tier]
	if !ok || tier < 3 || len(t.Color) < 6 {
		return ""
	}
	return t.Color[:6]
}

// MaxTier is radiant in the current set
func (d *Data) MaxTier() int {
	top := 0
	for t := range d.tier {
		if t > top {
			top = t
		}
	}
	return top
}

var divisions = map[string]string{
	"IRON": "Iron", "BRONZE": "Brnz", "SILVER": "Silv", "GOLD": "Gold", "PLATINUM": "Plat",
	"DIAMOND": "Dia", "ASCENDANT": "Asc", "IMMORTAL": "Imm", "RADIANT": "Rad",
}

func compactTier(name string) string {
	f := strings.Fields(name)
	if len(f) == 0 {
		return ""
	}
	short, ok := divisions[strings.ToUpper(f[0])]
	if !ok {
		short = abbreviate(f[0])
	}
	if len(f) > 1 {
		short += f[1]
	}
	return short
}

var agentAbbr = map[string]string{
	"Astra": "Astr", "Breach": "Brch", "Brimstone": "Brim", "Chamber": "Cham", "Clove": "Clov",
	"Cypher": "Cyph", "Deadlock": "Dlck", "Gekko": "Gekk", "Harbor": "Harb", "KAY/O": "KAYO",
	"Killjoy": "KJ", "Phoenix": "Phnx", "Reyna": "Reyn", "Viper": "Vipr", "Waylay": "Wayl",
}

// AgentAbbr is the 4-letter form used in the mains column
func AgentAbbr(name string) string {
	if a, ok := agentAbbr[name]; ok {
		return a
	}
	return abbreviate(name)
}

func abbreviate(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r == '/' || r == ' ' {
			continue
		}
		if b.Len() >= 4 {
			break
		}
		if i == 0 {
			b.WriteString(strings.ToUpper(string(r)))
		} else {
			b.WriteString(strings.ToLower(string(r)))
		}
	}
	return b.String()
}
