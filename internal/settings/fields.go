package settings

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

// Field is one key of lowkey.toml and its row on the settings tab - the commented file and the tab's help
// come from the same words - its section is the toml table it sits under and its heading on the tab
type Field struct {
	Section   string
	Key       string
	Comment   []string
	Label     string
	Help      string // the tab's help when the comment's first line doesn't fit there
	NextLobby bool   // lookups only pick it up at the next lobby
	DotOnly   bool   // the tab only has it in a build that can draw the agent dot
	Kind      Kind
}

// Fields are in file order and a section's keys stay together - the tab shows them in the same order
var Fields = []Field{
	{Section: "table", Key: "columns", Label: "columns",
		Comment: []string{"columns left to right - drop or reorder freely", strings.Join(render.Keys(), " ")},
		Kind: List{Field: func(s *Settings) *[]string { return &s.Columns }, All: render.Keys, Noun: "column",
			Shape: "columns takes a list of column names", Empty: "columns is empty", Once: true,
			Title: "columns - top is leftmost in the table", Movable: true, KeepOne: "at least one column has to stay on"}},
	{Section: "table", Key: "weapon", Label: "skin weapons",
		Comment: []string{"the gun the skin column shows - vandal phantom operator sheriff melee or any other by its name",
			`a list like ["vandal", "phantom"] gives each gun a skin column`},
		Kind: List{Field: func(s *Settings) *[]string { return &s.Weapons }, All: static.Weapons, Noun: "weapon",
			Shape: "weapon takes a gun's name or a list of them", Empty: "weapon is an empty list", Once: true, Fold: true,
			OneName: true, Names: true, Title: "skin weapons - top is leftmost in the table", Movable: true,
			KeepOne: "at least one gun has to stay on - turn the skin column off under columns"}},
	{Section: "table", Key: "matches", Label: "matches", NextLobby: true,
		Comment: []string{"recent matches per player behind k/d hs% acs adr fb% and mains - 1 to 25"},
		Kind:    Number{Field: func(s *Settings) *int { return &s.Matches }, Min: MinMatches, Max: MaxMatches}},
	{Section: "table", Key: "modes", Label: "modes", NextLobby: true,
		Comment: []string{"modes to track - competitive premier unrated swiftplay deathmatch team_deathmatch"},
		Kind: List{Field: func(s *Settings) *[]string { return &s.Modes }, All: stats.ModeKeys, Noun: "mode",
			Shape: "modes takes a list of mode names", Title: "modes to track",
			Known: func(name string) bool { _, ok := stats.ModeByKey(name); return ok }}},
	{Section: "players", Key: "enemy_lookups", Label: "enemy lookups", NextLobby: true,
		Comment: []string{"look up the enemy team at match load - off shows enemies by agent only"},
		Kind:    OnOff(func(s *Settings) *bool { return &s.EnemyLookups })},
	{Section: "players", Key: "hidden_player_stats", Label: "hidden player stats", NextLobby: true,
		Comment: []string{"show the stats of hidden players - never their names - off shows them by agent only"},
		Kind:    OnOff(func(s *Settings) *bool { return &s.HiddenPlayerStats })},
	{Section: "players", Key: "exact_enemy_parties", Label: "enemy parties",
		Comment: []string{"mark enemies who queued together in a recent match with a party letter - off shows likely parties only"},
		Kind:    OnOff(func(s *Settings) *bool { return &s.ExactEnemyParties })},
	{Section: "players", Key: "player_details", Label: "player details",
		Comment: []string{"pick a player on the lobby tab with the up and down arrows and press enter for their details"},
		Kind:    OnOff(func(s *Settings) *bool { return &s.PlayerDetails })},
	{Section: "players", Key: "site_scouting", Label: "site scouting",
		Comment: []string{"show in player details where a player tends to play the current map - from their recent matches on it"},
		Kind:    OnOff(func(s *Settings) *bool { return &s.SiteScouting })},
	{Section: "players", Key: "weapon_profiling", Label: "weapon profiling",
		Comment: []string{"say in player details when a player leans hard on the operator or on shotguns"},
		Kind:    OnOff(func(s *Settings) *bool { return &s.WeaponProfiling })},
	{Section: "match", Key: "server_pod", Label: "server pod",
		Comment: []string{"show the game server's city in the header at match load and during the match"},
		Kind:    OnOff(func(s *Settings) *bool { return &s.ServerPod })},
	{Section: "match", Key: "voice_check", Label: "voice check",
		Comment: []string{"show which teammates are in team voice during agent select"},
		Kind:    OnOff(func(s *Settings) *bool { return &s.VoiceCheck })},
	{Section: "general", Key: "session_reset_hour", Label: "session starts at", NextLobby: true,
		Comment: []string{"hour of the day in local time when the session line starts over"},
		Kind: Number{Field: func(s *Settings) *int { return &s.SessionResetHour }, Min: 0, Max: 23,
			Refuse: "isn't an hour from 0 to 23", Format: "%02d:00", Wrap: true}},
	{Section: "general", Key: "cutoff_seconds", Label: "cutoff", NextLobby: true,
		Comment: []string{"seconds after the match loads when every lookup stops - rounds start soon after"},
		// the tab lands on multiples of 5 so a hand-set 37 goes to 40 or 35 - the limits can be off that grid
		Kind: Number{Field: func(s *Settings) *int { return &s.CutoffSeconds }, Min: MinCutoffSeconds, Max: MaxCutoffSeconds,
			Refuse: fmt.Sprintf("isn't between %d and %d", MinCutoffSeconds, MaxCutoffSeconds), Format: "%ds", Grid: 5}},
	{Section: "general", Key: "region", Label: "region", NextLobby: true,
		Comment: []string{"auto reads the region from the game - na latam br eu ap kr or pbe overrides it"},
		Kind:    Choice{Field: func(s *Settings) *string { return &s.Region }, Options: riot.Regions}},
	// the section can't be called agent_dot - a flat file's agent_dot key would decode into its table
	{Section: "agent_select", Key: "agent_dot", Label: "agent dot", NextLobby: true, DotOnly: true,
		Comment: []string{"draw a small dot on your agent for a few seconds after a match is found - needs windowed or windowed fullscreen",
			"it never picks or locks anything - place dot on the settings tab sets where it goes"},
		Kind: OnOff(func(s *Settings) *bool { return &s.AgentDot })},
	{Section: "agent_select", Key: "agent_dot_seconds", Label: "dot time", NextLobby: true, DotOnly: true,
		Comment: []string{"the most seconds the agent dot stays up - it goes as soon as you hover or lock - 1 to 20"},
		Kind:    Number{Field: func(s *Settings) *int { return &s.AgentDotSeconds }, Min: MinDotSeconds, Max: MaxDotSeconds, Format: "%ds"}},
	{Section: "agent_select", Key: "agent_dots", Label: "agent dots", DotOnly: true,
		Comment: []string{"up to 3 agents and where each one's icon sits in screen pixels - set them up under agent dots on the settings tab",
			`like [{agent = "jett", at = [960, 540]}, {agent = "raze", at = []}] - [] is not placed yet`},
		Help: "enter - up to 3 agents whose dots show together when a match is found",
		Kind: Custom{Write: tomlDots, Read: readDots, Show: showDots,
			Clone: func(s *Settings) { s.AgentDots = slices.Clone(s.AgentDots) }}},
}

// Kind is how a key is written to lowkey.toml - read back from it - and shown on the settings tab
type Kind interface {
	write(s Settings) string
	read(s *Settings, key string, v any) (note string, err error)
	show(s Settings) string
}

// OnOff is true or false - space and enter flip it on the tab as well as the arrows
type OnOff func(*Settings) *bool

func (k OnOff) write(s Settings) string { return strconv.FormatBool(*k(&s)) }

func (k OnOff) read(s *Settings, key string, v any) (string, error) {
	b, ok := v.(bool)
	if !ok {
		return "", fmt.Errorf("%s isn't true or false", key)
	}
	*k(s) = b
	return "", nil
}

func (k OnOff) show(s Settings) string {
	if *k(&s) {
		return "on"
	}
	return "off"
}

// Number is a whole number from Min to Max - out of range is clamped with a note unless Refuse says why it
// breaks the file
type Number struct {
	Field    func(*Settings) *int
	Min, Max int
	Refuse   string // follows the key and value in the warning
	Format   string // the tab's value - %d when empty
	Wrap     bool   // the tab goes past Max round to Min
	Grid     int    // the tab steps onto multiples of it - 1 when zero
}

func (k Number) write(s Settings) string { return strconv.Itoa(*k.Field(&s)) }

func (k Number) read(s *Settings, key string, v any) (string, error) {
	n, ok := v.(int64)
	switch {
	case !ok:
		return "", fmt.Errorf("%s isn't a whole number", key)
	case n >= int64(k.Min) && n <= int64(k.Max):
		*k.Field(s) = int(n)
		return "", nil
	case k.Refuse != "":
		return "", fmt.Errorf("%s %d %s", key, n, k.Refuse)
	case n < int64(k.Min):
		*k.Field(s) = k.Min
		return fmt.Sprintf("%s %d is below %d - using %d", key, n, k.Min, k.Min), nil
	}
	*k.Field(s) = k.Max
	return fmt.Sprintf("%s %d is above %d - using %d", key, n, k.Max, k.Max), nil
}

func (k Number) show(s Settings) string {
	return fmt.Sprintf(cmp.Or(k.Format, "%d"), *k.Field(&s))
}

func (k Number) step(s *Settings, dir int) {
	v := k.Field(s)
	if k.Wrap {
		span := k.Max - k.Min + 1
		*v = k.Min + ((*v-k.Min+dir)%span+span)%span
		return
	}
	g, n := max(k.Grid, 1), *v
	if dir > 0 {
		n = (n/g + 1) * g
	} else {
		n = (n - 1) / g * g
	}
	*v = max(k.Min, min(n, k.Max))
}

// List is names from a known set - the tab opens it as a sub-list where switching a name off keeps its place
type List struct {
	Field func(*Settings) *[]string
	All   func() []string
	Known func(name string) bool // what a hand edit may name - every one of All when nil
	Noun  string                 // one item in a warning
	Shape string                 // the warning for a value that isn't a list of names
	Empty string                 // the warning for an empty list - empty is fine when this is
	Once  bool                   // a name listed twice breaks the file
	Fold  bool                   // names are trimmed and lowercased
	// OneName writes one item as a plain name and reads a plain name as one item - so a file that only ever
	// had one still reads the way it always did
	OneName bool
	Title   string
	Movable bool   // left and right reorder it on the tab
	KeepOne string // the tab's notice when the last name would go - empty lets it go
	Names   bool   // the tab's value is the names instead of how many are on
}

func (k List) write(s Settings) string {
	items := *k.Field(&s)
	if k.OneName && len(items) == 1 {
		return strconv.Quote(items[0])
	}
	return tomlList(items)
}

func (k List) read(s *Settings, key string, v any) (string, error) {
	items, ok := v.([]any)
	if !ok {
		if _, one := v.(string); !one || !k.OneName {
			return "", errors.New(k.Shape)
		}
		items = []any{v}
	}
	if len(items) == 0 && k.Empty != "" {
		return "", errors.New(k.Empty)
	}
	known := k.Known
	if known == nil {
		all := k.All()
		known = func(name string) bool { return slices.Contains(all, name) }
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		name, ok := item.(string)
		if !ok {
			return "", errors.New(k.Shape)
		}
		n := name
		if k.Fold {
			n = strings.ToLower(strings.TrimSpace(name))
		}
		if !known(n) {
			return "", fmt.Errorf("unknown %s %q", k.Noun, name)
		}
		if k.Once && slices.Contains(out, n) {
			return "", fmt.Errorf("%s %q is listed twice", k.Noun, name)
		}
		out = append(out, n)
	}
	*k.Field(s) = out
	return "", nil
}

func (k List) show(s Settings) string {
	items := *k.Field(&s)
	if k.Names {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%d of %d", len(items), len(k.All()))
}

// Choice is auto or one of Options - auto is the empty value
type Choice struct {
	Field   func(*Settings) *string
	Options func() []string
}

func (k Choice) write(s Settings) string {
	return strconv.Quote(cmp.Or(*k.Field(&s), "auto"))
}

func (k Choice) read(s *Settings, key string, v any) (string, error) {
	name, ok := v.(string)
	r := strings.ToLower(strings.TrimSpace(name))
	if !ok || r != "auto" && !slices.Contains(k.Options(), r) {
		return "", fmt.Errorf("%s %q isn't auto or one of %s", key, fmt.Sprint(v), strings.Join(k.Options(), " "))
	}
	*k.Field(s) = ""
	if r != "auto" {
		*k.Field(s) = r
	}
	return "", nil
}

func (k Choice) show(s Settings) string { return cmp.Or(*k.Field(&s), "auto") }

func (k Choice) step(s *Settings, dir int) {
	values := append([]string{""}, k.Options()...)
	v := k.Field(s)
	i := slices.Index(values, *v)
	*v = values[(i+dir+len(values))%len(values)]
}

// Custom is a key no kind fits - it brings its own code and the tab its own screen
type Custom struct {
	Write func(Settings) string
	Read  func(s *Settings, v any) (note string)
	Show  func(Settings) string
	Clone func(*Settings) // copies what an edit could change in place
}

func (k Custom) write(s Settings) string { return k.Write(s) }

func (k Custom) read(s *Settings, _ string, v any) (string, error) { return k.Read(s, v), nil }

func (k Custom) show(s Settings) string { return k.Show(s) }

func (f Field) Show(s Settings) string { return f.Kind.show(s) }

// Steps says whether left and right change it on the tab - a list or a custom key opens instead
func (f Field) Steps() bool {
	switch f.Kind.(type) {
	case OnOff, Number, Choice:
		return true
	}
	return false
}

func (f Field) Step(s *Settings, dir int) {
	switch k := f.Kind.(type) {
	case OnOff:
		*k(s) = !*k(s)
	case Number:
		k.step(s, dir)
	case Choice:
		k.step(s, dir)
	}
}

// Flips says whether space and enter change it too - only an on/off key can't be changed by accident that way
func (f Field) Flips() bool {
	_, ok := f.Kind.(OnOff)
	return ok
}

func (f Field) TabHelp() string { return cmp.Or(f.Help, f.Comment[0]) }

func (f Field) block(value string) string {
	var b strings.Builder
	for _, line := range f.Comment {
		b.WriteString("# " + line + "\n")
	}
	b.WriteString(f.Key + " = " + value + "\n")
	return b.String()
}

func lookup(key string) (Field, bool) {
	i := slices.IndexFunc(Fields, func(f Field) bool { return f.Key == key })
	if i < 0 {
		return Field{}, false
	}
	return Fields[i], true
}

func isSection(name string) bool {
	return slices.ContainsFunc(Fields, func(f Field) bool { return f.Section == name })
}

// clone copies the lists so an edit can't reach into settings someone else holds
func (s Settings) clone() Settings {
	for _, f := range Fields {
		switch k := f.Kind.(type) {
		case List:
			p := k.Field(&s)
			*p = slices.Clone(*p)
		case Custom:
			k.Clone(&s)
		}
	}
	return s
}

func tomlList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = strconv.Quote(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func tomlDots(s Settings) string {
	parts := make([]string, len(s.AgentDots))
	for i, d := range s.AgentDots {
		at := "[]"
		if d.At.Placed {
			at = fmt.Sprintf("[%d, %d]", d.At.X, d.At.Y)
		}
		parts[i] = fmt.Sprintf("{agent = %s, at = %s}", strconv.Quote(d.Agent), at)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func readDots(s *Settings, v any) string {
	dots, note := parseDots(v)
	s.AgentDots = dots
	return note
}

func showDots(s Settings) string {
	if len(s.AgentDots) == 0 {
		return "none"
	}
	names := make([]string, len(s.AgentDots))
	for i, d := range s.AgentDots {
		names[i] = d.Agent
		if !d.At.Placed {
			names[i] += " (not placed)"
		}
	}
	return strings.Join(names, ", ")
}
