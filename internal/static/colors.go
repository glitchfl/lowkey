package static

import (
	"sort"
	"strings"
)

// agentColors are 0xRRGGBB picks for each agent's look - a second color rings the dot and outlines the name
var agentColors = map[string][]uint32{
	"astra":     {0x8A4FFF},           // cosmic purple
	"breach":    {0xFFA000},           // amber orange
	"brimstone": {0xCC5500},           // burnt orange
	"chamber":   {0x1F2F6B, 0xD4AF37}, // french navy and gold
	"clove":     {0xE8A5D8},           // thistle pink
	"cypher":    {0xC3B091, 0xFFFFFF}, // desert khaki and white
	"deadlock":  {0x1E3A5F},           // arctic navy
	"fade":      {0x3A3F44},           // shadow charcoal
	"gekko":     {0x9BE22F},           // lime green
	"harbor":    {0x30D5C8},           // turquoise
	"hush":      {0xFF77D9, 0xC9B6F2}, // luminescent pink and pearlescent violet
	"iso":       {0xB38CE6},           // lilac violet
	"jett":      {0x00E5FF, 0xF4F7FA}, // cyan and cloud white
	"kay/o":     {0x1FB8D6},           // cybernetic cyan
	"killjoy":   {0xFFE600},           // canary yellow
	"miks":      {0x2A2D33, 0x00E0F0}, // shark charcoal and frequency cyan
	"neon":      {0x1F6BFF},           // electric blue
	"omen":      {0x3B2F7A},           // shadow indigo
	"phoenix":   {0xFF5A1F},           // flame orange
	"raze":      {0xFF8A00},           // tangerine orange
	"reyna":     {0xD1179B},           // empress magenta
	"sage":      {0x00A88A},           // jade teal
	"skye":      {0x2E8B3E},           // forest green
	"sova":      {0x234A7A},           // arctic navy blue
	"tejo":      {0x4A5560, 0x12A4A0}, // shark slate and atoll teal
	"veto":      {0x2B2B2E, 0x4169E1}, // stealth charcoal and royal blue
	"viper":     {0x39FF14},           // toxic green
	"vyse":      {0xAAA9AD},           // metallic silver
	"waylay":    {0x7851A9, 0x5FD8D8}, // royal purple and prismatic robin's egg blue
	"yoru":      {0x191970},           // midnight blue
}

const (
	lightRing = 0xF2F2F2
	darkRing  = 0x101010
	newAgent  = 0xF2F2F2
)

// AgentColors gives an agent's fill and ring - an agent the table doesn't know yet shows white
func AgentColors(name string) (fill, ring uint32) {
	c, ok := agentColors[strings.ToLower(name)]
	if !ok {
		return newAgent, darkRing
	}
	if len(c) > 1 {
		return c[0], c[1]
	}
	return c[0], contrastRing(c[0])
}

// contrastRing keeps a dark agent visible on agent select's dark background
func contrastRing(c uint32) uint32 {
	r, g, b := c>>16&0xFF, c>>8&0xFF, c&0xFF
	if (2126*r+7152*g+722*b)/10000 < 90 {
		return lightRing
	}
	return darkRing
}

// AgentNames lists every agent by its display name, sorted
func (d *Data) AgentNames() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	names := make([]string, 0, len(d.s.Agents))
	for _, name := range d.s.Agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
