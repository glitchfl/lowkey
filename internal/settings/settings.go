// Package settings reads lowkey.toml - the last good settings stay when an edit breaks the file
package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/stats"
)

const FileName = "lowkey.toml"

type Settings struct {
	Columns          []string
	Matches          int
	Modes            []string
	SessionResetHour int
	Cutoff           time.Duration
	EnemyLookups     bool
	Region           string // empty means detect it
}

func Default() Settings {
	return Settings{
		Columns:          render.Keys(),
		Matches:          13,
		Modes:            stats.ModeKeys(),
		SessionResetHour: 6,
		Cutoff:           40 * time.Second,
		EnemyLookups:     true,
	}
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

const defaultFile = `# lowkey settings - edits apply at the next redraw without a restart

# columns left to right - drop or reorder freely
# party name agent rank peak last_act wr wl kd hs acs adr fb n mains rr_trend met
columns = ["party", "name", "agent", "rank", "peak", "last_act", "wr", "wl", "kd", "hs", "acs", "adr", "fb", "n", "mains", "rr_trend", "met"]

# recent matches per player behind k/d hs% acs adr fb% and mains - 1 to 25
matches = 13

# modes to track - competitive premier unrated swiftplay deathmatch team_deathmatch
modes = ["competitive", "premier", "unrated", "swiftplay", "deathmatch", "team_deathmatch"]

# hour of the day in local time when the session line starts over
session_reset_hour = 6

# seconds after the match loads when every lookup stops - rounds start soon after
cutoff_seconds = 40

# look up the enemy team at match load - false shows enemies by agent only
enemy_lookups = true

# auto reads the region from the game - na latam br eu ap kr or pbe overrides it
region = "auto"
`

var regions = []string{"na", "latam", "br", "eu", "ap", "kr", "pbe"}

type file struct {
	Columns          *[]string `toml:"columns"`
	Matches          *int      `toml:"matches"`
	Modes            *[]string `toml:"modes"`
	SessionResetHour *int      `toml:"session_reset_hour"`
	CutoffSeconds    *int      `toml:"cutoff_seconds"`
	EnemyLookups     *bool     `toml:"enemy_lookups"`
	Region           *string   `toml:"region"`
}

// Parse fills missing keys with defaults - a clamped value comes back with a note, anything invalid as an error
func Parse(data []byte) (Settings, string, error) {
	var f file
	md, err := toml.Decode(string(data), &f)
	var pe toml.ParseError
	if errors.As(err, &pe) {
		return Settings{}, "", fmt.Errorf("line %d: %s", pe.Position.Line, pe.Message)
	}
	if err != nil {
		return Settings{}, "", fmt.Errorf("can't read it: %v", firstLine(err))
	}
	if extra := md.Undecoded(); len(extra) > 0 {
		return Settings{}, "", fmt.Errorf("unknown setting %q", extra[0].String())
	}
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
		case m < 1:
			s.Matches, note = 1, fmt.Sprintf("matches %d is below 1 - using 1", m)
		case m > 25:
			s.Matches, note = 25, fmt.Sprintf("matches %d is above 25 - using 25", m)
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
		if c := *f.CutoffSeconds; c < 1 || c > 600 {
			return Settings{}, "", fmt.Errorf("cutoff_seconds %d isn't between 1 and 600", c)
		}
		s.Cutoff = time.Duration(*f.CutoffSeconds) * time.Second
	}
	if f.EnemyLookups != nil {
		s.EnemyLookups = *f.EnemyLookups
	}
	if f.Region != nil {
		r := strings.ToLower(strings.TrimSpace(*f.Region))
		switch {
		case r == "auto":
		case contains(regions, r):
			s.Region = r
		default:
			return Settings{}, "", fmt.Errorf("region %q isn't auto or one of %s", *f.Region, strings.Join(regions, " "))
		}
	}
	return s, note, nil
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
}

func Open(dir string) *File {
	f := &File{path: filepath.Join(dir, FileName), current: Default()}
	f.Refresh()
	return f
}

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
	_, err = fh.WriteString(defaultFile)
	if cerr := fh.Close(); err == nil {
		err = cerr
	}
	f.Refresh()
	return err
}

// Refresh re-reads the file when its modification time changed and reports whether anything did
func (f *File) Refresh() bool {
	fi, err := os.Stat(f.path)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		changed := f.exists
		f.exists, f.warning, f.invalid, f.mtime = false, "", false, time.Time{}
		return changed
	}
	if f.exists && fi.ModTime().Equal(f.mtime) {
		return false
	}
	f.exists, f.mtime = true, fi.ModTime()
	data, err := os.ReadFile(f.path)
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

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
