// Package settings reads lowkey.toml - the last good settings stay when an edit breaks the file
package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

const FileName = "lowkey.toml"

const (
	MinMatches       = 1
	MaxMatches       = 25
	MinCutoffSeconds = 1
	MaxCutoffSeconds = 600
)

type Settings struct {
	Columns           []string
	Matches           int
	Modes             []string
	SessionResetHour  int
	Cutoff            time.Duration
	EnemyLookups      bool
	HiddenPlayerStats bool
	ExactEnemyParties bool
	ServerPod         bool
	VoiceCheck        bool
	PlayerDetails     bool
	Region            string   // empty means detect it
	Weapons           []string // the guns the skin column shows left to right - lowercase
}

func Default() Settings {
	return Settings{
		Columns:           render.Keys(),
		Matches:           13,
		Modes:             stats.ModeKeys(),
		SessionResetHour:  6,
		Cutoff:            40 * time.Second,
		EnemyLookups:      true,
		HiddenPlayerStats: true,
		Weapons:           []string{"vandal"},
	}
}

// Shows reports whether a column is switched on
func (s Settings) Shows(column string) bool {
	return slices.Contains(s.Columns, column)
}

// Tracks reports whether a queue's mode is switched on
func (s Settings) Tracks(queue string) bool {
	m, ok := stats.ModeByQueue(queue)
	if !ok {
		return false
	}
	for _, k := range s.Modes {
		if k == m.Key {
			return true
		}
	}
	return false
}

// field is one key of lowkey.toml - the commented file and the settings tab's help come from the same words -
// its section is the toml table it sits under and its heading on the settings tab
type field struct {
	section string
	key     string
	comment []string
	value   func(Settings) string
}

// fields are in file order and a section's keys stay together
var fields = []field{
	{"table", "columns", []string{"columns left to right - drop or reorder freely", "party name agent rank peak last_act wr wl kd hs acs adr fb n mains rr_trend met skin"},
		func(s Settings) string { return tomlList(s.Columns) }},
	{"table", "weapon", []string{"the gun the skin column shows - vandal phantom operator sheriff melee or any other by its name",
		`a list like ["vandal", "phantom"] gives each gun a skin column`},
		// one gun stays a plain name so a file that only ever had one still reads the way it always did
		func(s Settings) string {
			if len(s.Weapons) == 1 {
				return strconv.Quote(s.Weapons[0])
			}
			return tomlList(s.Weapons)
		}},
	{"table", "matches", []string{"recent matches per player behind k/d hs% acs adr fb% and mains - 1 to 25"},
		func(s Settings) string { return strconv.Itoa(s.Matches) }},
	{"table", "modes", []string{"modes to track - competitive premier unrated swiftplay deathmatch team_deathmatch"},
		func(s Settings) string { return tomlList(s.Modes) }},
	{"players", "enemy_lookups", []string{"look up the enemy team at match load - off shows enemies by agent only"},
		func(s Settings) string { return strconv.FormatBool(s.EnemyLookups) }},
	{"players", "hidden_player_stats", []string{"show the stats of hidden players - never their names - off shows them by agent only"},
		func(s Settings) string { return strconv.FormatBool(s.HiddenPlayerStats) }},
	{"players", "exact_enemy_parties", []string{"mark enemies who queued together in a recent match with a party letter - off shows likely parties only"},
		func(s Settings) string { return strconv.FormatBool(s.ExactEnemyParties) }},
	{"players", "player_details", []string{"pick a player on the lobby tab with the up and down arrows and press enter for their details"},
		func(s Settings) string { return strconv.FormatBool(s.PlayerDetails) }},
	{"match", "server_pod", []string{"show the game server's city in the header at match load and during the match"},
		func(s Settings) string { return strconv.FormatBool(s.ServerPod) }},
	{"match", "voice_check", []string{"show which teammates are in team voice during agent select"},
		func(s Settings) string { return strconv.FormatBool(s.VoiceCheck) }},
	{"general", "session_reset_hour", []string{"hour of the day in local time when the session line starts over"},
		func(s Settings) string { return strconv.Itoa(s.SessionResetHour) }},
	{"general", "cutoff_seconds", []string{"seconds after the match loads when every lookup stops - rounds start soon after"},
		func(s Settings) string { return strconv.Itoa(int(s.Cutoff / time.Second)) }},
	{"general", "region", []string{"auto reads the region from the game - na latam br eu ap kr or pbe overrides it"},
		func(s Settings) string {
			if s.Region == "" {
				return `"auto"`
			}
			return strconv.Quote(s.Region)
		}},
}

// Section names the section a key sits under - empty for a key lowkey doesn't have
func Section(key string) string {
	for _, fd := range fields {
		if fd.key == key {
			return fd.section
		}
	}
	return ""
}

func isSection(name string) bool {
	return slices.ContainsFunc(fields, func(fd field) bool { return fd.section == name })
}

func tomlList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = strconv.Quote(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func (fd field) block(value string) string {
	var b strings.Builder
	for _, line := range fd.comment {
		b.WriteString("# " + line + "\n")
	}
	b.WriteString(fd.key + " = " + value + "\n")
	return b.String()
}

func Help(key string) string {
	for _, fd := range fields {
		if fd.key == key {
			return fd.comment[0]
		}
	}
	return ""
}

func (s Settings) contents() []byte {
	var b strings.Builder
	b.WriteString("# lowkey settings - edit them here or on the settings tab - changes apply without a restart\n")
	section := ""
	for _, fd := range fields {
		if fd.section != section {
			section = fd.section
			b.WriteString("\n[" + section + "]\n")
		}
		b.WriteString("\n" + fd.block(fd.value(s)))
	}
	return []byte(b.String())
}

type tableKeys struct {
	Columns *[]string `toml:"columns"`
	Weapon  any       `toml:"weapon"` // one gun's name or a list of them
	Matches *int      `toml:"matches"`
	Modes   *[]string `toml:"modes"`
}

type playersKeys struct {
	EnemyLookups      *bool `toml:"enemy_lookups"`
	HiddenPlayerStats *bool `toml:"hidden_player_stats"`
	ExactEnemyParties *bool `toml:"exact_enemy_parties"`
	PlayerDetails     *bool `toml:"player_details"`
}

type matchKeys struct {
	ServerPod  *bool `toml:"server_pod"`
	VoiceCheck *bool `toml:"voice_check"`
}

type generalKeys struct {
	SessionResetHour *int    `toml:"session_reset_hour"`
	CutoffSeconds    *int    `toml:"cutoff_seconds"`
	Region           *string `toml:"region"`
}

type keys struct {
	tableKeys
	playersKeys
	matchKeys
	generalKeys
}

// file reads the keys at the top too - that's where a file from before sections has them
type file struct {
	keys
	Table   *tableKeys   `toml:"table"`
	Players *playersKeys `toml:"players"`
	Match   *matchKeys   `toml:"match"`
	General *generalKeys `toml:"general"`
}

// Parse fills missing keys with defaults - a clamped value comes back as a note and anything invalid as an error
func Parse(data []byte) (Settings, string, error) {
	s, note, _, err := parse(data, false)
	return s, note, err
}

// parse with flatOK also takes a file from before sections - flat says whether it was one
func parse(data []byte, flatOK bool) (s Settings, note string, flat bool, err error) {
	var f file
	md, err := toml.Decode(string(data), &f)
	var pe toml.ParseError
	if errors.As(err, &pe) {
		return Settings{}, "", false, fmt.Errorf("line %d: %s", pe.Position.Line, pe.Message)
	}
	if err != nil {
		return Settings{}, "", false, fmt.Errorf("can't read it: %v", firstLine(err))
	}
	stray, sections := "", false
	for _, k := range md.Keys() {
		switch {
		case len(k) == 1 && isSection(k[0]):
			sections = true
		case len(k) == 1 && Section(k[0]) != "" && stray == "":
			stray = k[0]
		case len(k) == 2 && isSection(k[0]) && Section(k[1]) != "" && Section(k[1]) != k[0]:
			return Settings{}, "", false, fmt.Errorf("%s goes under [%s] - not [%s]", k[1], Section(k[1]), k[0])
		}
	}
	// a stray key next to sections is a hand edit to fix - only a file with no sections at all gets moved
	if stray != "" && sections {
		return Settings{}, "", false, fmt.Errorf("%s goes under [%s]", stray, Section(stray))
	}
	flat = stray != ""
	if extra := md.Undecoded(); len(extra) > 0 {
		return Settings{}, "", false, fmt.Errorf("unknown setting %q", extra[0].String())
	}
	if f.Table != nil {
		f.tableKeys = *f.Table
	}
	if f.Players != nil {
		f.playersKeys = *f.Players
	}
	if f.Match != nil {
		f.matchKeys = *f.Match
	}
	if f.General != nil {
		f.generalKeys = *f.General
	}
	s, note, err = f.keys.settings()
	// a flat file that couldn't be moved names what kept it from moving first
	if err == nil && flat && !flatOK {
		err = fmt.Errorf("%s goes under [%s]", stray, Section(stray))
	}
	if err != nil {
		return Settings{}, "", false, err
	}
	return s, note, flat, nil
}

func (f keys) settings() (Settings, string, error) {
	s := Default()
	note := ""
	if f.Columns != nil {
		if len(*f.Columns) == 0 {
			return Settings{}, "", errors.New("columns is empty")
		}
		seen := map[string]bool{}
		for _, c := range *f.Columns {
			if _, ok := render.Lookup(c); !ok {
				return Settings{}, "", fmt.Errorf("unknown column %q", c)
			}
			if seen[c] {
				return Settings{}, "", fmt.Errorf("column %q is listed twice", c)
			}
			seen[c] = true
		}
		s.Columns = *f.Columns
	}
	if f.Matches != nil {
		switch m := *f.Matches; {
		case m < MinMatches:
			s.Matches, note = MinMatches, fmt.Sprintf("matches %d is below %d - using %d", m, MinMatches, MinMatches)
		case m > MaxMatches:
			s.Matches, note = MaxMatches, fmt.Sprintf("matches %d is above %d - using %d", m, MaxMatches, MaxMatches)
		default:
			s.Matches = m
		}
	}
	if f.Modes != nil {
		for _, m := range *f.Modes {
			if _, ok := stats.ModeByKey(m); !ok {
				return Settings{}, "", fmt.Errorf("unknown mode %q", m)
			}
		}
		s.Modes = *f.Modes
	}
	if f.SessionResetHour != nil {
		if h := *f.SessionResetHour; h < 0 || h > 23 {
			return Settings{}, "", fmt.Errorf("session_reset_hour %d isn't an hour from 0 to 23", h)
		}
		s.SessionResetHour = *f.SessionResetHour
	}
	if f.CutoffSeconds != nil {
		if c := *f.CutoffSeconds; c < MinCutoffSeconds || c > MaxCutoffSeconds {
			return Settings{}, "", fmt.Errorf("cutoff_seconds %d isn't between %d and %d", c, MinCutoffSeconds, MaxCutoffSeconds)
		}
		s.Cutoff = time.Duration(*f.CutoffSeconds) * time.Second
	}
	if f.EnemyLookups != nil {
		s.EnemyLookups = *f.EnemyLookups
	}
	if f.HiddenPlayerStats != nil {
		s.HiddenPlayerStats = *f.HiddenPlayerStats
	}
	if f.ExactEnemyParties != nil {
		s.ExactEnemyParties = *f.ExactEnemyParties
	}
	if f.ServerPod != nil {
		s.ServerPod = *f.ServerPod
	}
	if f.VoiceCheck != nil {
		s.VoiceCheck = *f.VoiceCheck
	}
	if f.PlayerDetails != nil {
		s.PlayerDetails = *f.PlayerDetails
	}
	if f.Region != nil {
		r := strings.ToLower(strings.TrimSpace(*f.Region))
		if _, known := riot.Shard(r); r != "auto" && !known {
			return Settings{}, "", fmt.Errorf("region %q isn't auto or one of %s", *f.Region, strings.Join(riot.Regions(), " "))
		}
		if r != "auto" {
			s.Region = r
		}
	}
	if f.Weapon != nil {
		weapons, err := parseWeapons(f.Weapon)
		if err != nil {
			return Settings{}, "", err
		}
		s.Weapons = weapons
	}
	return s, note, nil
}

func parseWeapons(v any) ([]string, error) {
	names := []any{v}
	if list, ok := v.([]any); ok {
		names = list
	}
	if len(names) == 0 {
		return nil, errors.New("weapon is an empty list")
	}
	var weapons []string
	for _, n := range names {
		name, ok := n.(string)
		if !ok {
			return nil, errors.New("weapon takes a gun's name or a list of them")
		}
		w := strings.ToLower(strings.TrimSpace(name))
		if !slices.Contains(static.Weapons(), w) {
			return nil, fmt.Errorf("unknown weapon %q", name)
		}
		if slices.Contains(weapons, w) {
			return nil, fmt.Errorf("weapon %q is listed twice", name)
		}
		weapons = append(weapons, w)
	}
	return weapons, nil
}

// File watches lowkey.toml next to the exe and keeps the last good settings
type File struct {
	path string

	mu      sync.Mutex
	current Settings
	mtime   time.Time
	warning string
	invalid bool
	exists  bool
	moved   string
	moveErr error
}

func Open(dir string) *File {
	f := &File{path: filepath.Join(dir, FileName), current: Default()}
	f.moved, f.moveErr = f.moveIntoSections()
	f.Refresh()
	return f
}

// moveIntoSections rewrites a file from before sections with the same values - the old one is kept as a
// backup - a file that doesn't read as one stays as it is and the usual warning says what's wrong
func (f *File) moveIntoSections() (string, error) {
	raw, err := os.ReadFile(f.path)
	if err != nil {
		return "", nil
	}
	s, _, flat, err := parse(raw, true)
	if err != nil || !flat {
		return "", nil
	}
	if err := os.WriteFile(f.path+".bak", raw, 0o644); err != nil {
		return "", fmt.Errorf("can't move %s into sections: %v", FileName, err)
	}
	if err := replace(f.path, s.contents()); err != nil {
		return "", fmt.Errorf("can't move %s into sections: %v", FileName, err)
	}
	return fmt.Sprintf("%s moved into sections - the old one is now %s.bak", FileName, FileName), nil
}

// Moved says what opening the file did to one from before sections - both empty when it had nothing to move
func (f *File) Moved() (string, error) { return f.moved, f.moveErr }

func (f *File) Path() string { return f.path }

// Create writes the commented defaults on first run and never touches an existing file
func (f *File) Create() error {
	fh, err := os.OpenFile(f.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = fh.Write(Default().contents())
	if cerr := fh.Close(); err == nil {
		err = cerr
	}
	f.Refresh()
	return err
}

// Refresh re-reads the file when its modification time changed and reports whether anything did
func (f *File) Refresh() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refresh()
}

func (f *File) refresh() bool {
	fi, err := os.Stat(f.path)
	if err != nil {
		changed := f.exists
		f.exists, f.warning, f.invalid, f.mtime = false, "", false, time.Time{}
		return changed
	}
	if f.exists && fi.ModTime().Equal(f.mtime) {
		return false
	}
	data, err := os.ReadFile(f.path)
	if err == nil && len(strings.TrimSpace(string(data))) == 0 {
		// editors truncate before they write - an empty read is a save in progress so look again next time
		return false
	}
	f.exists, f.mtime = true, fi.ModTime()
	if err == nil {
		var s Settings
		var note string
		if s, note, err = Parse(data); err == nil {
			f.current, f.warning, f.invalid = s, "", false
			if note != "" {
				f.warning = FileName + ": " + note
			}
			return true
		}
	}
	f.warning, f.invalid = fmt.Sprintf("%s: %v - using the last good settings", FileName, err), true
	return true
}

// Invalid means the file couldn't be used at all - a clamped value doesn't count
func (f *File) Invalid() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.invalid
}

func (f *File) Settings() Settings {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current
}

// Warning names what's wrong with the file - empty when it's fine
func (f *File) Warning() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.warning
}

func (f *File) Exists() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.exists
}

func firstLine(err error) string {
	msg := err.Error()
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	return msg
}
