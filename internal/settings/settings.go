// Package settings reads lowkey.toml - the last good settings stay when an edit breaks the file
package settings

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/stats"
)

const FileName = "lowkey.toml"

const (
	MinMatches       = 1
	MaxMatches       = 25
	MinCutoffSeconds = 1
	MaxCutoffSeconds = 600
	MinDotSeconds    = 1
	MaxDotSeconds    = 20
	MaxDots          = 3
)

type Settings struct {
	Columns           []string
	Matches           int
	Modes             []string
	SessionResetHour  int
	CutoffSeconds     int
	EnemyLookups      bool
	HiddenPlayerStats bool
	ExactEnemyParties bool
	ServerPod         bool
	VoiceCheck        bool
	PlayerDetails     bool
	SiteScouting      bool
	WeaponProfiling   bool
	Region            string   // empty means detect it
	Weapons           []string // the guns the skin column shows left to right - lowercase
	AgentDot          bool
	AgentDotSeconds   int
	AgentDots         []DotSlot
}

// DotSlot is one agent dot - At stays unplaced until place on the settings tab saves it
type DotSlot struct {
	Agent string
	At    Spot
}

// Spot is a point on the screen in physical pixels - the zero Spot isn't placed
type Spot struct {
	X, Y   int
	Placed bool
}

func Default() Settings {
	return Settings{
		Columns:           render.Keys(),
		Matches:           13,
		Modes:             stats.ModeKeys(),
		SessionResetHour:  6,
		CutoffSeconds:     40,
		EnemyLookups:      true,
		HiddenPlayerStats: true,
		Weapons:           []string{"vandal"},
		AgentDotSeconds:   15,
	}
}

// Cutoff is how long after the match loads every lookup stops
func (s Settings) Cutoff() time.Duration { return time.Duration(s.CutoffSeconds) * time.Second }

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

func (s Settings) contents() []byte {
	var b strings.Builder
	b.WriteString("# lowkey settings - edit them here or on the settings tab - changes apply without a restart\n")
	section := ""
	for _, f := range Fields {
		if f.Section != section {
			section = f.Section
			b.WriteString("\n[" + section + "]\n")
		}
		b.WriteString("\n" + f.block(f.Kind.write(s)))
	}
	return []byte(b.String())
}

// Parse fills missing keys with defaults - a clamped value comes back as a note and anything invalid as an error
func Parse(data []byte) (Settings, string, error) {
	s, note, _, err := parse(data, false)
	return s, note, err
}

// parse with flatOK also takes a file from before sections - flat says whether it was one
func parse(data []byte, flatOK bool) (s Settings, note string, flat bool, err error) {
	var raw map[string]any
	md, err := toml.Decode(string(data), &raw)
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
		case len(k) == 1 && stray == "":
			if _, ok := lookup(k[0]); ok {
				stray = k[0]
			}
		case len(k) == 2 && isSection(k[0]):
			if f, ok := lookup(k[1]); ok && f.Section != k[0] {
				return Settings{}, "", false, fmt.Errorf("%s goes under [%s] - not [%s]", k[1], f.Section, k[0])
			}
		}
	}
	// a stray key next to sections is a hand edit to fix - only a file with no sections at all gets moved
	if stray != "" && sections {
		return Settings{}, "", false, strayError(stray)
	}
	flat = stray != ""
	// deeper keys are inside a value like an agent dot's inline table - the key holding it decides
	values := map[string]any{}
	for _, k := range md.Keys() {
		_, known := lookup(k[len(k)-1])
		switch {
		case len(k) == 1 && isSection(k[0]):
			table, ok := raw[k[0]].(map[string]any)
			if !ok {
				return Settings{}, "", false, fmt.Errorf("%s isn't a section like [%s]", k[0], k[0])
			}
			maps.Copy(values, table)
		case len(k) == 1 && known:
			values[k[0]] = raw[k[0]]
		case len(k) == 1 || len(k) == 2 && isSection(k[0]) && !known:
			return Settings{}, "", false, fmt.Errorf("unknown setting %q", k.String())
		}
	}
	s = Default()
	for _, f := range Fields {
		v, ok := values[f.Key]
		if !ok {
			continue
		}
		n, err := f.Kind.read(&s, f.Key, v)
		if err != nil {
			return Settings{}, "", false, err
		}
		if n != "" {
			note = n
		}
	}
	// a flat file that couldn't be moved names what kept it from moving first
	if flat && !flatOK {
		return Settings{}, "", false, strayError(stray)
	}
	return s, note, flat, nil
}

func strayError(key string) error {
	f, _ := lookup(key)
	return fmt.Errorf("%s goes under [%s]", key, f.Section)
}

// parseDots keeps whatever it can use - a bad spot only unplaces its own dot
func parseDots(v any) ([]DotSlot, string) {
	var entries []map[string]any
	switch list := v.(type) {
	case []map[string]any:
		entries = list
	case []any:
		for _, item := range list {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, `agent_dots takes entries like {agent = "jett", at = [960, 540]} - no agent dots`
			}
			entries = append(entries, entry)
		}
	default:
		return nil, `agent_dots isn't a list like [{agent = "jett", at = [960, 540]}] - no agent dots`
	}
	var dots []DotSlot
	note := ""
	for _, entry := range entries {
		name, _ := entry["agent"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			note = "an agent_dots entry has no agent - it's left out"
			continue
		}
		d := DotSlot{Agent: name}
		if at, ok := entry["at"]; ok {
			if d.At, ok = parseSpot(at); !ok {
				note = fmt.Sprintf("%s's at isn't two whole numbers like [960, 540] - that dot isn't placed", name)
			}
		}
		for k := range entry {
			if k != "agent" && k != "at" {
				note = fmt.Sprintf("agent_dots has no %q - it's ignored", k)
			}
		}
		dots = append(dots, d)
	}
	if len(dots) > MaxDots {
		note = fmt.Sprintf("agent_dots lists %d agents - using the first %d", len(dots), MaxDots)
		dots = dots[:MaxDots]
	}
	return dots, note
}

// parseSpot takes exactly two whole numbers - an empty list is a spot that was never placed
func parseSpot(v any) (Spot, bool) {
	list, ok := v.([]any)
	if !ok {
		return Spot{}, false
	}
	if len(list) == 0 {
		return Spot{}, true
	}
	if len(list) != 2 {
		return Spot{}, false
	}
	var xy [2]int
	for i, n := range list {
		whole, ok := n.(int64)
		if !ok || whole < math.MinInt32 || whole > math.MaxInt32 {
			return Spot{}, false
		}
		xy[i] = int(whole)
	}
	return Spot{X: xy[0], Y: xy[1], Placed: true}, true
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
