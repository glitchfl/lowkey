package app

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/settings"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

const (
	tabBarLobby    = "[LOBBY]  SETTINGS          tab to switch"
	tabBarSettings = " LOBBY  [SETTINGS]         tab to switch"
	nextLobbyNote  = "applies from the next lobby"
)

// settingsTab edits lowkey.toml straight away - a press never asks Riot for anything so holding an arrow
// can't fire a burst of requests - lookups pick the new values up at the next lobby
type settingsTab struct {
	cfg    *settings.File
	rows   []settingRow
	row    int
	list   *optionList // the open sub-list - nil on the main list
	slots  *dotSlots   // the open agent dots list - nil on the main list
	notice string      // what the last press couldn't do or what it did to a broken file
	agents func() []string
	// place starts a slot's countdown and placing is the seconds it has left
	place       func(slot int)
	placing     int
	placingSlot int
}

// dotSlots is the list behind agent dots - a row per agent with a dot plus a free row while there's room
type dotSlots struct{ row int }

const placeHelp = "enter - then within 5 s rest the cursor on that agent in the game - lowkey saves where it is and turns the dots on"

type settingRow struct {
	key, label string
	nextLobby  bool
	value      func(settings.Settings) string
	step       func(s *settings.Settings, dir int)
	flips      bool // space and enter work too - stepping anything else by accident would be saved at once
	open       func(settings.Settings) *optionList
	act        func()
	help       string // shown instead of the file's comment - for a row whose enter does more than change its key
}

var settingRows = []settingRow{
	{key: "columns", label: "columns", open: columnsList,
		value: func(s settings.Settings) string { return fmt.Sprintf("%d of %d", len(s.Columns), len(render.Keys())) }},
	{key: "matches", label: "matches", nextLobby: true,
		value: func(s settings.Settings) string { return strconv.Itoa(s.Matches) },
		step: func(s *settings.Settings, dir int) {
			s.Matches = max(settings.MinMatches, min(s.Matches+dir, settings.MaxMatches))
		}},
	{key: "modes", label: "modes", nextLobby: true, open: modesList,
		value: func(s settings.Settings) string { return fmt.Sprintf("%d of %d", len(s.Modes), len(stats.ModeKeys())) }},
	{key: "session_reset_hour", label: "session starts at", nextLobby: true,
		value: func(s settings.Settings) string { return fmt.Sprintf("%02d:00", s.SessionResetHour) },
		step:  func(s *settings.Settings, dir int) { s.SessionResetHour = (s.SessionResetHour + dir + 24) % 24 }},
	{key: "cutoff_seconds", label: "cutoff", nextLobby: true,
		value: func(s settings.Settings) string { return fmt.Sprintf("%ds", int(s.Cutoff/time.Second)) },
		step:  stepCutoff},
	{key: "enemy_lookups", label: "enemy lookups", nextLobby: true, flips: true,
		value: func(s settings.Settings) string { return onOff(s.EnemyLookups) },
		step:  func(s *settings.Settings, _ int) { s.EnemyLookups = !s.EnemyLookups }},
	{key: "hidden_player_stats", label: "hidden player stats", nextLobby: true, flips: true,
		value: func(s settings.Settings) string { return onOff(s.HiddenPlayerStats) },
		step:  func(s *settings.Settings, _ int) { s.HiddenPlayerStats = !s.HiddenPlayerStats }},
	{key: "region", label: "region", nextLobby: true,
		value: func(s settings.Settings) string {
			if s.Region == "" {
				return "auto"
			}
			return s.Region
		},
		step: func(s *settings.Settings, dir int) {
			s.Region = cycle(append([]string{""}, riot.Regions()...), s.Region, dir)
		}},
	{key: "weapon", label: "skin weapons", open: weaponsList,
		value: func(s settings.Settings) string { return strings.Join(s.Weapons, ", ") }},
}

// dotRows only exist in a build that can draw the agent dot
func (t *settingsTab) dotRows() []settingRow {
	return []settingRow{
		{key: "agent_dot", label: "agent dot", nextLobby: true, flips: true,
			value: func(s settings.Settings) string { return onOff(s.AgentDot) },
			step:  func(s *settings.Settings, _ int) { s.AgentDot = !s.AgentDot }},
		{key: "agent_dot_seconds", label: "dot time", nextLobby: true,
			value: func(s settings.Settings) string { return fmt.Sprintf("%ds", s.AgentDotSeconds) },
			step: func(s *settings.Settings, dir int) {
				s.AgentDotSeconds = max(settings.MinDotSeconds, min(s.AgentDotSeconds+dir, settings.MaxDotSeconds))
			}},
		{key: "agent_dots", label: "agent dots", act: func() { t.slots = &dotSlots{} },
			help: "enter - up to 3 agents whose dots show together when a match is found",
			value: func(s settings.Settings) string {
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
			}},
	}
}

func slotCount(dots []settings.DotSlot) int {
	return min(len(dots)+1, settings.MaxDots)
}

func (t *settingsTab) pressSlots(k key) {
	dots := t.cfg.Settings().AgentDots
	n := slotCount(dots)
	sl := t.slots
	sl.row = min(sl.row, n-1)
	switch k {
	case keyUp:
		sl.row = (sl.row - 1 + n) % n
	case keyDown:
		sl.row = (sl.row + 1) % n
	case keyLeft, keyRight:
		if t.placing > 0 {
			t.notice = "wait for placing to finish"
			return
		}
		row, current := sl.row, ""
		if row < len(dots) {
			current = dots[row].Agent
		}
		next := cycleFold(append([]string{""}, t.agents()...), current, direction(k))
		t.change(func(s *settings.Settings) {
			switch {
			case row < len(s.AgentDots) && next == "":
				s.AgentDots = slices.Delete(s.AgentDots, row, row+1)
			case row < len(s.AgentDots):
				// another agent's icon sits somewhere else so the old spot means nothing for it
				s.AgentDots[row] = settings.DotSlot{Agent: next}
			case next != "" && len(s.AgentDots) < settings.MaxDots:
				s.AgentDots = append(s.AgentDots, settings.DotSlot{Agent: next})
			}
		})
	case keySpace, keyEnter:
		if sl.row >= len(dots) {
			t.notice = "pick an agent with left/right first"
			return
		}
		t.place(sl.row)
	case keyEsc:
		t.slots = nil
	}
}

func (t *settingsTab) slotRows(dots []settings.DotSlot) []string {
	n := slotCount(dots)
	out := make([]string, n)
	for i := range n {
		mark, name, value := " ", "none", ""
		if i == t.slots.row {
			mark = ">"
		}
		if i < len(dots) {
			name = dots[i].Agent
			switch {
			case t.placing > 0 && t.placingSlot == i:
				value = fmt.Sprintf("point at your agent - %d", t.placing)
			case dots[i].At.Placed:
				value = fmt.Sprintf("%d, %d", dots[i].At.X, dots[i].At.Y)
			default:
				value = "not placed"
			}
		}
		out[i] = strings.TrimRight(fmt.Sprintf("%s %d  %-10s  %s", mark, i+1, name, value), " ")
	}
	return out
}

// cycleFold is cycle for names a hand edit may have typed in another case - an unknown one starts from none
func cycleFold(values []string, current string, dir int) string {
	i := max(0, slices.IndexFunc(values, func(v string) bool { return strings.EqualFold(v, current) }))
	return values[(i+dir+len(values))%len(values)]
}

// stepCutoff lands on multiples of 5 so a hand-set 37 goes to 40 or 35 - the limits can be off that grid
func stepCutoff(s *settings.Settings, dir int) {
	c := int(s.Cutoff / time.Second)
	if dir > 0 {
		c = (c/5 + 1) * 5
	} else {
		c = (c - 1) / 5 * 5
	}
	c = max(settings.MinCutoffSeconds, min(c, settings.MaxCutoffSeconds))
	s.Cutoff = time.Duration(c) * time.Second
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func direction(k key) int {
	if k == keyLeft {
		return -1
	}
	return 1
}

func cycle(values []string, current string, dir int) string {
	i := slices.Index(values, current)
	return values[(i+dir+len(values))%len(values)]
}

// press handles a key on the tab and reports whether it leaves for the lobby
func (t *settingsTab) press(k key) bool {
	t.notice = ""
	if t.list != nil {
		t.pressList(k)
		return false
	}
	if t.slots != nil {
		t.pressSlots(k)
		return false
	}
	row := t.rows[t.row]
	switch k {
	case keyUp:
		t.row = (t.row - 1 + len(t.rows)) % len(t.rows)
	case keyDown:
		t.row = (t.row + 1) % len(t.rows)
	case keyLeft, keyRight:
		if row.step != nil {
			t.change(func(s *settings.Settings) { row.step(s, direction(k)) })
		}
	case keySpace, keyEnter:
		switch {
		case row.open != nil:
			t.list = row.open(t.cfg.Settings())
		case row.act != nil:
			row.act()
		case row.flips:
			t.change(func(s *settings.Settings) { row.step(s, 1) })
		}
	case keyEsc:
		return true
	}
	return false
}

func (t *settingsTab) pressList(k key) {
	l := t.list
	cfg := t.cfg.Settings()
	l.sync(*l.field(&cfg))
	switch k {
	case keyUp:
		l.row = (l.row - 1 + len(l.items)) % len(l.items)
	case keyDown:
		l.row = (l.row + 1) % len(l.items)
	case keyLeft, keyRight:
		if to := l.row + direction(k); l.movable && to >= 0 && to < len(l.items) {
			l.items[l.row], l.items[to] = l.items[to], l.items[l.row]
			l.row = to
			t.change(func(s *settings.Settings) { *l.field(s) = l.ordered(*l.field(s)) })
		}
	case keySpace:
		item := l.items[l.row]
		if on := *l.field(&cfg); slices.Contains(on, item) && len(on) <= l.keepOn {
			t.notice = l.keepOnNotice
			return
		}
		t.change(func(s *settings.Settings) {
			on := l.field(s)
			if i := slices.Index(*on, item); i >= 0 {
				*on = slices.Delete(*on, i, i+1)
			} else {
				*on = l.ordered(append(*on, item))
			}
		})
	case keyEnter, keyEsc:
		t.list = nil
	}
}

func (t *settingsTab) change(edit func(*settings.Settings)) {
	backup, err := t.cfg.Change(edit)
	switch {
	case err != nil:
		t.notice = fmt.Sprintf("can't save %s: %v", settings.FileName, err)
	case backup != "":
		t.notice = fmt.Sprintf("%s was broken - the old one is now %s", settings.FileName, backup)
	}
}

// lines draws the tab in the rows it gets - a list too long for the window scrolls with the highlight
func (t *settingsTab) lines(rows int) []string {
	cfg := t.cfg.Settings()
	var lines []string
	if w := t.cfg.Warning(); w != "" {
		lines = append(lines, w)
	}
	if t.notice != "" {
		lines = append(lines, t.notice)
	}
	lines = append(lines, "")
	var body, foot []string
	var row int
	switch {
	case t.list != nil:
		l := t.list
		l.sync(*l.field(&cfg))
		lines = append(lines, l.title)
		body, row, foot = l.rows(*l.field(&cfg)), l.row, []string{l.hint}
	case t.slots != nil:
		t.slots.row = min(t.slots.row, slotCount(cfg.AgentDots)-1)
		lines = append(lines, "agent dots - up to 3 - shown together when a match is found")
		body, row, foot = t.slotRows(cfg.AgentDots), t.slots.row, []string{placeHelp, "left/right agent · enter place · esc back"}
	default:
		body, row = t.mainRows(cfg), t.row
		r := t.rows[t.row]
		note, help := "", r.help
		if r.nextLobby {
			note = nextLobbyNote
		}
		if help == "" {
			help = settings.Help(r.key)
		}
		foot = []string{help, note, "up/down move · left/right change · enter open · saved at once"}
	}
	from, to := window(len(body), row, rows-len(lines)-1-len(foot))
	lines = append(lines, body[from:to]...)
	lines = append(lines, "")
	return append(lines, foot...)
}

func (t *settingsTab) mainRows(cfg settings.Settings) []string {
	width := 0
	for _, r := range t.rows {
		width = max(width, len(r.label))
	}
	out := make([]string, len(t.rows))
	for i, r := range t.rows {
		value := r.value(cfg)
		mark := " "
		if i == t.row {
			mark = ">"
			if r.open != nil || r.act != nil {
				value += "  (enter)"
			} else {
				value = "< " + value + " >"
			}
		}
		out[i] = fmt.Sprintf("%s %-*s  %s", mark, width, r.label, value)
	}
	return out
}

func window(total, row, room int) (from, to int) {
	room = max(room, 1)
	if total <= room {
		return 0, total
	}
	from = max(0, min(row-room/2, total-room))
	return from, from + room
}

// optionList is the sub-list behind columns or modes - a row stays where the player left it when it's
// switched off even though the file only knows the ones that are on - so switching it back is undo
type optionList struct {
	title, hint  string
	all          []string
	items        []string
	row          int
	movable      bool
	keepOn       int
	keepOnNotice string
	field        func(*settings.Settings) *[]string
}

func columnsList(cfg settings.Settings) *optionList {
	return newOptionList(optionList{
		title: "columns - top is leftmost in the table", hint: "space on/off · left/right move · esc back",
		all: render.Keys(), movable: true, keepOn: 1, keepOnNotice: "at least one column has to stay on",
		field: func(s *settings.Settings) *[]string { return &s.Columns },
	}, cfg)
}

func modesList(cfg settings.Settings) *optionList {
	return newOptionList(optionList{
		title: "modes to track - " + nextLobbyNote, hint: "space on/off · esc back",
		all:   stats.ModeKeys(),
		field: func(s *settings.Settings) *[]string { return &s.Modes },
	}, cfg)
}

func weaponsList(cfg settings.Settings) *optionList {
	return newOptionList(optionList{
		title: "skin weapons - top is leftmost in the table", hint: "space on/off · left/right move · esc back",
		all: static.Weapons(), movable: true, keepOn: 1, keepOnNotice: "at least one gun has to stay on - turn the skin column off under columns",
		field: func(s *settings.Settings) *[]string { return &s.Weapons },
	}, cfg)
}

func newOptionList(l optionList, cfg settings.Settings) *optionList {
	l.items = arranged(*l.field(&cfg), l.all)
	return &l
}

func arranged(on, all []string) []string {
	items := slices.Clone(on)
	for _, item := range all {
		if !slices.Contains(items, item) {
			items = append(items, item)
		}
	}
	return items
}

func (l *optionList) ordered(on []string) []string {
	out := make([]string, 0, len(on))
	for _, item := range l.items {
		if slices.Contains(on, item) {
			out = append(out, item)
		}
	}
	return out
}

// sync follows a hand edit made while the list is open
func (l *optionList) sync(on []string) {
	if slices.Equal(l.ordered(on), on) {
		return
	}
	current := l.items[l.row]
	l.items = arranged(on, l.all)
	l.row = max(0, slices.Index(l.items, current))
}

func (l *optionList) rows(on []string) []string {
	out := make([]string, len(l.items))
	for i, item := range l.items {
		mark, box := " ", "[ ]"
		if i == l.row {
			mark = ">"
		}
		if slices.Contains(on, item) {
			box = "[x]"
		}
		out[i] = mark + " " + box + " " + item
	}
	return out
}
