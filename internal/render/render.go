// Package render lays lowkey's screens out as plain or ANSI-colored text
package render

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type Align int

const (
	Left Align = iota
	Right
)

type Column struct {
	Key    string
	Header string
	Align  Align
	// HideEmpty drops the column while no row has a value - skins only arrive at match load
	HideEmpty bool
}

var catalog = []Column{
	{Key: "party", Header: ""},
	{Key: "name", Header: "PLAYER"},
	{Key: "agent", Header: "AGENT"},
	{Key: "rank", Header: "RANK"},
	{Key: "peak", Header: "PEAK"},
	{Key: "last_act", Header: "LAST"},
	{Key: "wr", Header: "WR", Align: Right},
	{Key: "wl", Header: "W-L", Align: Right},
	{Key: "kd", Header: "K/D", Align: Right},
	{Key: "hs", Header: "HS%", Align: Right},
	{Key: "acs", Header: "ACS", Align: Right},
	{Key: "adr", Header: "ADR", Align: Right},
	{Key: "fb", Header: "FB%", Align: Right},
	{Key: "n", Header: "N", Align: Right},
	{Key: "mains", Header: "MAINS"},
	{Key: "rr_trend", Header: "TREND", Align: Right},
	{Key: "met", Header: "MET"},
	{Key: "skin", Header: "SKIN", HideEmpty: true},
}

// Keys lists every column in the default order
func Keys() []string {
	keys := make([]string, len(catalog))
	for i, c := range catalog {
		keys[i] = c.Key
	}
	return keys
}

func Lookup(key string) (Column, bool) {
	for _, c := range catalog {
		if c.Key == key {
			return c, true
		}
	}
	return Column{}, false
}

// Columns turns keys into columns and skips unknown ones
func Columns(keys []string) []Column {
	var cols []Column
	for _, k := range keys {
		if c, ok := Lookup(k); ok {
			cols = append(cols, c)
		}
	}
	return cols
}

type Cell struct {
	Text  string
	Color string // RRGGBB - only rank tiers and skin editions carry one
}

type Row map[string]Cell

type Section struct {
	Title string
	Rows  []Row
}

// Table is a block with its own columns - the player details stack several under the header
type Table struct {
	Columns  []Column
	Rows     []Row
	Sections []Section // titled groups after Rows - they share its column widths
	Note     string    // plain lines under the rows
}

type Screen struct {
	Status   []string
	Header   string
	Sections []Section
	Tables   []Table
	Session  string
	// Height lets everything under the header scroll by Scroll lines once it doesn't fit - zero never scrolls
	Height int
	Scroll int
}

// wide is where the console gives a letter two cells - Korean Japanese and Chinese names and fullwidth forms
var wide = [][2]rune{
	{0x1100, 0x115f}, {0x2e80, 0x303e}, {0x3041, 0x33ff}, {0x3400, 0x4dbf}, {0x4e00, 0x9fff}, {0xa000, 0xa4cf},
	{0xa960, 0xa97f}, {0xac00, 0xd7a3}, {0xf900, 0xfaff}, {0xfe10, 0xfe19}, {0xfe30, 0xfe6f}, {0xff00, 0xff60},
	{0xffe0, 0xffe6}, {0x1f300, 0x1f64f}, {0x1f900, 0x1f9ff}, {0x20000, 0x3fffd},
}

// cells is how many columns the console draws a letter in - combining marks sit on the letter before them
func cells(r rune) int {
	if unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) {
		return 0
	}
	if _, found := slices.BinarySearchFunc(wide, r, func(span [2]rune, r rune) int {
		switch {
		case r < span[0]:
			return 1
		case r > span[1]:
			return -1
		}
		return 0
	}); found {
		return 2
	}
	return 1
}

func width(s string) int {
	n := 0
	for _, r := range s {
		n += cells(r)
	}
	return n
}

// Cut shortens s to at most n cells and ends it with an ellipsis when anything went
func Cut(s string, n int) string {
	if width(s) <= n {
		return s
	}
	w := 0
	for i, r := range s {
		if w+cells(r) > n-1 {
			return s[:i] + "…"
		}
		w += cells(r)
	}
	return s
}

// Lines lays the screen out for the console width - columns that don't fit drop from the end
func (s Screen) Lines(cols []Column, maxWidth int, ansi bool) []string {
	lines, _ := s.Layout(cols, maxWidth, ansi)
	return lines
}

// Layout is Lines plus how far it really scrolled - never past the last line
func (s Screen) Layout(cols []Column, maxWidth int, ansi bool) ([]string, int) {
	var head, body []string
	head = append(head, s.Status...)
	if s.Header != "" {
		head = append(head, s.Header)
	}
	if len(s.Sections) > 0 {
		body = append(body, table(cols, s.Sections, maxWidth, ansi)...)
	}
	for _, t := range s.Tables {
		body = append(body, "")
		sections := t.Sections
		if len(t.Rows) > 0 {
			sections = append([]Section{{Rows: t.Rows}}, sections...)
		}
		if slices.ContainsFunc(sections, func(sec Section) bool { return len(sec.Rows) > 0 }) {
			body = append(body, table(t.Columns, sections, maxWidth, ansi)...)
		}
		if t.Note != "" {
			body = append(body, strings.Split(t.Note, "\n")...)
		}
	}
	if s.Session != "" {
		body = append(body, s.Session)
	}
	scroll := 0
	// the last row goes to saying what's off screen
	if room := s.Height - len(head) - 1; s.Height > 0 && len(body) > room+1 && room > 0 {
		scroll = max(0, min(s.Scroll, len(body)-room))
		var more []string
		if scroll > 0 {
			more = append(more, fmt.Sprintf("↑ %d above", scroll))
		}
		if below := len(body) - room - scroll; below > 0 {
			more = append(more, fmt.Sprintf("↓ %d below", below))
		}
		body = append(body[scroll:scroll+room:scroll+room], strings.Join(more, " · "))
	}
	lines := append(head, body...)
	for i, l := range lines {
		lines[i] = clip(strings.TrimRight(l, " "), maxWidth)
	}
	return lines, scroll
}

func table(cols []Column, sections []Section, maxWidth int, ansi bool) []string {
	widths := map[string]int{}
	var shown []Column
	for _, c := range cols {
		w, filled := width(c.Header), false
		for _, sec := range sections {
			for _, row := range sec.Rows {
				n := width(row[c.Key].Text)
				w, filled = max(w, n), filled || n > 0
			}
		}
		if w == 0 || c.HideEmpty && !filled {
			continue
		}
		widths[c.Key] = w
		shown = append(shown, c)
	}

	total := func(gap int) int {
		t := gap * (len(shown) - 1)
		for _, c := range shown {
			t += widths[c.Key]
		}
		return t
	}
	gap, hidden := 3, 0
	for gap > 1 && total(gap) > maxWidth {
		gap--
	}
	for len(shown) > 1 && total(gap) > maxWidth {
		shown = shown[:len(shown)-1]
		hidden++
	}

	var lines []string
	header := make([]string, len(shown))
	for i, c := range shown {
		header[i] = pad(c.Header, widths[c.Key], c.Align)
	}
	lines = append(lines, strings.Join(header, strings.Repeat(" ", gap)))
	for _, sec := range sections {
		if sec.Title != "" {
			lines = append(lines, sec.Title)
		}
		for _, row := range sec.Rows {
			parts := make([]string, len(shown))
			for i, c := range shown {
				cell := row[c.Key]
				text := pad(cell.Text, widths[c.Key], c.Align)
				if ansi && cell.Color != "" && cell.Text != "" {
					text = colorize(text, cell.Text, cell.Color)
				}
				parts[i] = text
			}
			lines = append(lines, strings.Join(parts, strings.Repeat(" ", gap)))
		}
	}
	if hidden > 0 {
		lines = append(lines, fmt.Sprintf("+%d hidden - widen the window", hidden))
	}
	return lines
}

func pad(s string, w int, a Align) string {
	n := w - width(s)
	if n <= 0 {
		return s
	}
	if a == Right {
		return strings.Repeat(" ", n) + s
	}
	return s + strings.Repeat(" ", n)
}

// colorize wraps only the text inside its padding so the colors never bleed into the gaps
func colorize(padded, text, rgb string) string {
	if len(rgb) < 6 {
		return padded
	}
	r, err1 := strconv.ParseUint(rgb[0:2], 16, 8)
	g, err2 := strconv.ParseUint(rgb[2:4], 16, 8)
	b, err3 := strconv.ParseUint(rgb[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return padded
	}
	i := strings.Index(padded, text)
	return padded[:i] + fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", r, g, b, text) + padded[i+len(text):]
}

// clip cuts plain lines to the width - table rows never need it since columns already fit
func clip(s string, w int) string {
	if w <= 0 || strings.Contains(s, "\x1b[") {
		return s
	}
	return Cut(s, w)
}
