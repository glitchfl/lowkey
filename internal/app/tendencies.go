package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/stats"
)

type view int

const (
	viewLobby view = iota
	viewTendencies
	viewSettings
)

func (lv *live) canTendencies() bool {
	return lv.keyboard && lv.cfg.Settings().TendenciesTab
}

func (lv *live) nextView() view {
	switch lv.view {
	case viewLobby:
		if lv.canTendencies() {
			return viewTendencies
		}
		return viewSettings
	case viewTendencies:
		return viewSettings
	}
	return viewLobby
}

// switchView starts the tendencies tab at its top - the lobby tab keeps where its details were scrolled to
func (lv *live) switchView(v view) {
	if lv.view == viewTendencies || v == viewTendencies {
		lv.scroll = 0
	}
	lv.view = v
}

func (lv *live) tabBar() string {
	names := []string{"LOBBY"}
	views := []view{viewLobby}
	if lv.canTendencies() {
		names, views = append(names, "TENDENCIES"), append(views, viewTendencies)
	}
	names, views = append(names, "SETTINGS"), append(views, viewSettings)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = " " + name + " "
		if views[i] == lv.view {
			parts[i] = "[" + name + "]"
		}
	}
	return strings.Join(parts, " ") + strings.Repeat(" ", 9) + "tab to switch"
}

func (lv *live) pressTendencies(k key) {
	switch k {
	case keyUp:
		lv.scroll = max(0, lv.scroll-1)
	case keyDown:
		lv.scroll++
	case keyEsc:
		lv.switchView(viewLobby)
	}
}

// tendenciesTab puts what player details say about site scouting and weapon profiling side by side for the
// whole lobby - from the matches already fetched so it never asks Riot for anything
func (lv *live) tendenciesTab(s *render.Screen, lob *lobby, title string) {
	if lob == nil {
		return
	}
	s.Header = lv.lobbyTitle(lob, title)
	cfg := lv.cfg.Settings()
	sites, guns := cfg.SiteScouting && lob.mode.Rounds, cfg.WeaponProfiling
	switch {
	case !cfg.SiteScouting && !guns:
		s.Tables = []render.Table{{Note: "turn on site scouting or weapon profiling on the settings tab"}}
		return
	case !sites && !guns:
		s.Tables = []render.Table{{Note: lob.mode.Name + " has no sites to scout - and weapon profiling is off"}}
		return
	case len(lob.members) == 0:
		return
	}
	var allies, enemies []render.Row
	for _, m := range lob.members {
		row := lv.tendencyRow(lob, m, sites, guns)
		if lob.kind == lobbyMatchLoad && lob.mode.Teams && m.team != lob.ownTeam {
			enemies = append(enemies, row)
		} else {
			allies = append(allies, row)
		}
	}
	t := render.Table{Columns: tendencyColumns(sites, guns), Sections: []render.Section{{Rows: allies}}}
	if lob.kind == lobbyMatchLoad && lob.mode.Teams {
		t.Sections = []render.Section{{Title: "ALLIES", Rows: allies}}
		if len(enemies) > 0 {
			t.Sections = append(t.Sections, render.Section{Title: "ENEMIES", Rows: enemies})
		}
	}
	if sites {
		name, ok := lv.data.MapName(lob.mapURL)
		if !ok {
			name = "?"
		}
		t.Note = fmt.Sprintf("\nON MAP is how many of their matches were on %s - brackets count the rounds plants or opening duels behind a split", name)
	}
	s.Tables = []render.Table{t}
}

func tendencyColumns(sites, guns bool) []render.Column {
	cols := render.Columns([]string{"name", "agent"})
	if sites {
		cols = append(cols, render.Column{Key: "on_map", Header: "ON MAP", Align: render.Right},
			render.Column{Key: "defense", Header: "DEFENSE"}, render.Column{Key: "attack", Header: "ATTACK"},
			render.Column{Key: "opening", Header: "OPENING"})
	}
	if guns {
		cols = append(cols, render.Columns([]string{"n"})...)
		cols = append(cols, render.Column{Key: "leans", Header: "LEANS ON"})
	}
	return cols
}

// tendencyRow stays blank while their matches load like the lobby table does - a player lowkey doesn't
// look up stays blank for good
func (lv *live) tendencyRow(lob *lobby, m *member, sites, guns bool) render.Row {
	row := render.Row{"name": lv.nameCell(m), "agent": lv.agentCell(m)}
	p := m.p
	switch {
	case p == nil || !p.historyDone:
		return row
	case p.historyErr != nil:
		for _, k := range []string{"on_map", "defense", "attack", "opening", "n", "leans"} {
			row[k] = dash
		}
		return row
	}
	matches := p.loaded()
	if sites {
		st := stats.SitesOf(p.puuid, lob.mapURL, matches, lv.data.Callouts(lob.mapURL))
		row["on_map"] = render.Cell{Text: strconv.Itoa(st.Matches)}
		row["defense"], row["attack"], row["opening"] = splitCell(st.Defense), splitCell(st.Plants), splitCell(st.Openings)
	}
	if guns {
		l := stats.LeaningsOf(p.puuid, matches, lv.data.Duelist)
		row["n"] = render.Cell{Text: strconv.Itoa(l.Matches)}
		row["leans"] = leansCell(l)
	}
	return row
}

func splitCell(t stats.Tally) render.Cell {
	if t.N == 0 {
		return dash
	}
	parts := make([]string, len(t.Shares))
	for i, s := range t.Shares {
		parts[i] = fmt.Sprintf("%s %d%%", s.Place, s.Percent)
	}
	return render.Cell{Text: fmt.Sprintf("%s (%d)", strings.Join(parts, " "), t.N)}
}

func leansCell(l stats.Leanings) render.Cell {
	var guns []string
	if l.Operator != nil {
		guns = append(guns, "Operator")
	}
	if l.Shotguns != nil {
		guns = append(guns, l.Shotguns.Guns...)
	}
	if len(guns) == 0 {
		return dash
	}
	return render.Cell{Text: strings.Join(guns, " · ")}
}
