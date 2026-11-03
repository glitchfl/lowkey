// Package lobbytable builds the lobby table - every player's row with its overlays and the team averages
// and US - THEM - and decides what lowkey may show about each lobby player
package lobbytable

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/settings"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

// TrendGames is how many competitive games TREND sums - other players' lookups fetch only that many
const TrendGames = 5

type State int

const (
	Loading State = iota
	Failed
	Ready
)

// Fact is one download about a player - Value holds what arrived so far whatever the state
type Fact[T any] struct {
	State State
	Value T
}

type Player struct {
	PUUID   string
	Name    Fact[string] // ready with no name shows a dash
	MMR     Fact[stats.MMR]
	Updates Fact[[]stats.Update]
	Matches Fact[[]*stats.Match] // the sample so far - newest first
}

type Member struct {
	PUUID      string
	Agent      string
	Hover      bool // picked in agent select but not locked in
	Team       string
	Level      int // zero when the player hides it
	Visibility Visibility
	Player     *Player // nil while Visibility.Stats is off
	Met        string  // empty when they were never met
}

type Lobby struct {
	MatchLoad bool
	Mode      stats.Mode
	Self      string
	OwnTeam   string
	Members   []Member
	Party     map[string]bool // your own party - nil until it arrives
	Skins     riot.Loadouts
	Voice     map[string]string // what the local client says about each player's voice
}

type Input struct {
	Lobby    Lobby
	Acts     *stats.Acts
	Data     *static.Data
	Settings settings.Settings
	Picking  bool                // the arrows pick players - so the pick column is there
	Picked   string              // by PUUID
	Alias    func(string) string // swaps Riot IDs for a recording's fakes
}

// Row is one player's cells - they carry the who row's team and level too and the lobby's columns leave those out
type Row struct {
	PUUID string
	Cells render.Row
}

type Section struct {
	Title   string
	Players []Row
	Average render.Row
}

type Table struct {
	Columns  []render.Column
	Sections []Section
	Diff     render.Row // US - THEM - nil until both teams are known
}

const (
	pickMarker = "▸"
	// a full Riot ID can reach 22 - 20 is what still fits every stat column in 120 columns
	nameWidth = 20
)

func Build(in Input) Table {
	t := Table{Columns: Columns(in.Settings, in.Picking)}
	lob := in.Lobby
	if len(lob.Members) == 0 {
		return t
	}
	b := builder{in: in, marks: markers(lob, in.Settings.ExactEnemyParties), skins: skinCells(lob.Skins, in.Settings.Weapons, in.Data)}
	if !lob.MatchLoad && in.Settings.VoiceCheck {
		b.voice = map[string]string{}
		for _, m := range lob.Members {
			b.voice[m.PUUID] = cmp.Or(lob.Voice[m.PUUID], "-")
		}
	}
	// deathmatch has no teams - everyone is one list with a lobby average and nothing to diff
	if !lob.MatchLoad || !lob.Mode.Teams {
		sec, _ := b.section("", lob.Members)
		t.Sections = []Section{sec}
		return t
	}
	var allies, enemies []Member
	for _, m := range lob.Members {
		if m.Team == lob.OwnTeam {
			allies = append(allies, m)
		} else {
			enemies = append(enemies, m)
		}
	}
	us, usAvg := b.section("ALLIES", allies)
	t.Sections = []Section{us}
	// until the core-game roster arrives there are only the allies carried over from agent select
	if len(enemies) == 0 {
		return t
	}
	them, themAvg := b.section("ENEMIES", enemies)
	t.Sections = append(t.Sections, them)
	t.Diff = diffRow(usAvg, themAvg)
	return t
}

// You is lowkey me's one row - only the columns that mean something outside a lobby
func You(p Player, mode stats.Mode, acts *stats.Acts, data *static.Data, cfg settings.Settings) Table {
	var keys []string
	for _, c := range cfg.Columns {
		if !lobbyOnly[c] {
			keys = append(keys, c)
		}
	}
	row := Row{PUUID: p.PUUID, Cells: playerRow(p, mode, acts, data)}
	return Table{Columns: render.Columns(keys), Sections: []Section{{Players: []Row{row}}}}
}

var lobbyOnly = map[string]bool{"party": true, "agent": true, "met": true, "skin": true}

// Columns follow the settings - the skin column is one column per gun headed by its name - the pick
// marker's column stays empty and so hidden until someone presses an arrow
func Columns(cfg settings.Settings, picking bool) []render.Column {
	var cols []render.Column
	if picking {
		cols = append(cols, render.Column{Key: "pick"})
	}
	for _, c := range render.Columns(cfg.Columns) {
		if c.Key != "skin" {
			cols = append(cols, c)
			continue
		}
		for _, gun := range cfg.Weapons {
			c.Key, c.Header = skinKey(gun), strings.ToUpper(gun)
			cols = append(cols, c)
		}
	}
	// the voice column has no place in the columns setting - outside agent select it has no cells and so hides
	if cfg.VoiceCheck {
		at := slices.IndexFunc(cols, func(c render.Column) bool { return c.Key == "agent" })
		if at < 0 {
			at = len(cols) - 1
		}
		cols = slices.Insert(cols, at+1, render.ColumnVoice)
	}
	return cols
}

func (t Table) Render() []render.Section {
	var out []render.Section
	for _, s := range t.Sections {
		sec := render.Section{Title: s.Title}
		for _, r := range s.Players {
			sec.Rows = append(sec.Rows, r.Cells)
		}
		if s.Average != nil {
			sec.Rows = append(sec.Rows, s.Average)
		}
		out = append(out, sec)
	}
	if t.Diff != nil {
		out = append(out, render.Section{Rows: []render.Row{t.Diff}})
	}
	return out
}

func (t Table) Row(puuid string) (render.Row, bool) {
	for _, s := range t.Sections {
		for _, r := range s.Players {
			if r.PUUID == puuid {
				return r.Cells, true
			}
		}
	}
	return nil, false
}

type builder struct {
	in    Input
	marks map[string]string
	skins map[string]render.Row
	voice map[string]string
}

func (b builder) section(title string, members []Member) (Section, averages) {
	lob := b.in.Lobby
	sec := Section{Title: title}
	for _, m := range members {
		row := b.memberRow(m)
		if mark, ok := b.marks[m.PUUID]; ok {
			row["party"] = render.Cell{Text: mark}
		}
		if v, ok := b.voice[m.PUUID]; ok {
			row["voice"] = render.Cell{Text: v}
		}
		// meeting someone ties them to a game you saw them in - so a hidden player never gets it even with stats
		if m.Met != "" && !m.Visibility.Hidden && m.Player != nil {
			row["met"] = render.Cell{Text: m.Met}
		}
		// like every other stat a skin stays off players lowkey doesn't look up
		if m.Player != nil {
			maps.Copy(row, b.skins[m.PUUID])
		}
		if m.PUUID == b.in.Picked {
			row["pick"] = render.Cell{Text: pickMarker}
		}
		if lob.Mode.Teams && lob.MatchLoad {
			row["team"] = render.Cell{Text: "enemy"}
			if m.Team == lob.OwnTeam {
				row["team"] = render.Cell{Text: "ally"}
			}
		}
		if m.Level > 0 {
			row["level"] = render.Cell{Text: strconv.Itoa(m.Level)}
		}
		sec.Players = append(sec.Players, Row{PUUID: m.PUUID, Cells: row})
	}
	avg := teamAverages(members, lob.Mode, b.in.Acts, b.in.Data)
	sec.Average = avg.row(b.in.Data)
	return sec, avg
}

func (b builder) memberRow(m Member) render.Row {
	row := render.Row{}
	if p := m.Player; p != nil {
		row = playerRow(*p, b.in.Lobby.Mode, b.in.Acts, b.in.Data)
		if b.in.Alias != nil && p.Name.Value != "" {
			row["name"] = render.Cell{Text: Cut(b.in.Alias(p.Name.Value), nameWidth)}
		}
	}
	row["agent"] = agentCell(m, b.in.Data)
	// a name fetched before they went hidden stays off too - Riot only allows their stats while it does
	if m.Visibility.Hidden {
		row["name"] = render.Cell{Text: "hidden"}
	}
	return row
}

// agentCell brackets an agent that's only hovered - a locked-in agent stands bare
func agentCell(m Member, data *static.Data) render.Cell {
	if m.Agent == "" {
		return render.Cell{}
	}
	name, ok := data.AgentName(m.Agent)
	if !ok {
		name = "?"
	}
	if m.Hover {
		name = "(" + name + ")"
	}
	return render.Cell{Text: name}
}

// playerRow shows what's known - cells still loading stay blank and cells with no value show "-"
func playerRow(p Player, mode stats.Mode, acts *stats.Acts, data *static.Data) render.Row {
	row := render.Row{}
	if p.Name.State != Loading {
		row["name"] = Dash
		if p.Name.Value != "" {
			row["name"] = render.Cell{Text: Cut(p.Name.Value, nameWidth)}
		}
	}
	switch {
	case p.MMR.State == Ready && acts != nil:
		rankCells(row, stats.RankOf(p.MMR.Value, *acts, mode.Queue, data.CurrentTier), data, mode)
	case p.MMR.State != Loading:
		for _, k := range []string{"rank", "peak", "last_act", "wr", "wl"} {
			row[k] = Dash
		}
	}
	switch p.Updates.State {
	case Ready:
		row["rr_trend"] = Dash
		if trend, ok := stats.Trend(p.Updates.Value, TrendGames); ok {
			row["rr_trend"] = render.Cell{Text: Signed(trend)}
		}
	case Failed:
		row["rr_trend"] = Dash
	}
	switch p.Matches.State {
	case Ready:
		SampleCells(row, stats.SampleOf(p.PUUID, p.Matches.Value), mode, data)
	case Failed:
		for _, k := range append(sampleKeys, "n") {
			row[k] = Dash
		}
	}
	return row
}

// skinKey is the column a gun's skins go in
func skinKey(gun string) string { return "skin:" + gun }

// skinCells show everyone's skin for each gun in the settings in its edition's color
func skinCells(skins riot.Loadouts, weapons []string, data *static.Data) map[string]render.Row {
	cells := map[string]render.Row{}
	for _, gun := range weapons {
		weapon, _, ok := data.Weapon(gun)
		if !ok {
			continue
		}
		for puuid, guns := range skins {
			g, ok := guns[weapon]
			if !ok {
				continue
			}
			if cells[puuid] == nil {
				cells[puuid] = render.Row{}
			}
			cells[puuid][skinKey(gun)] = SkinCell(data, g.Skin)
		}
	}
	return cells
}
