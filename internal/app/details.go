package app

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/stats"
)

const pickMarker = "▸"

// pressLobby picks a player with the arrows and opens their details with enter - like the settings tab
// a press never asks Riot for anything
func (lv *live) pressLobby(k key) {
	lob, _ := lv.shownLobby()
	if !lv.canPick() || lob == nil || len(lob.members) == 0 {
		return
	}
	n := len(lob.members)
	i := slices.IndexFunc(lob.members, func(m *member) bool { return m.puuid == lv.pick })
	if lv.details && i >= 0 {
		switch k {
		case keyEsc, keyEnter:
			lv.details = false
		case keyUp:
			lv.scroll = max(0, lv.scroll-1)
		case keyDown:
			lv.scroll++
		case keyLeft, keyRight:
			lv.pick, lv.scroll = lob.members[(i+direction(k)+n)%n].puuid, 0
		}
		return
	}
	lv.details = false
	switch k {
	case keyDown:
		lv.pick = lob.members[(i+1)%n].puuid
	case keyUp:
		if i < 0 {
			i = 0
		}
		lv.pick = lob.members[(i-1+n)%n].puuid
	case keyEnter:
		lv.details, lv.scroll = i >= 0, 0
	case keyEsc:
		lv.pick = ""
	}
}

// shownLobby is the lobby the lobby tab draws and its title - nil while the tab is only status lines
func (lv *live) shownLobby() (*lobby, string) {
	switch {
	case lv.phase == phaseIdle || lv.phase == phaseWaiting:
		return nil, ""
	case lv.phase == phaseMenus:
		return lv.last, "LAST MATCH"
	case lv.untracked != "" || lv.midMatch:
		return nil, ""
	case lv.lobby != nil && len(lv.lobby.members) == 0 && lv.unreachable && lv.last != nil:
		return lv.last, "LAST MATCH"
	}
	return lv.lobby, phaseTitles[lv.phase]
}

func (lv *live) canPick() bool {
	return lv.keyboard && lv.cfg.Settings().PlayerDetails
}

func (lv *live) picked(lob *lobby) *member {
	if lob == nil || lv.pick == "" {
		return nil
	}
	return lob.member(lv.pick)
}

func (lv *live) lobbyTab(s *render.Screen, lob *lobby, title string) {
	if m := lv.picked(lob); m != nil && lv.details {
		lv.detailsScreen(s, lob, m, title)
		return
	}
	if lob != nil {
		lv.lobbyScreen(s, lob, title)
	}
	s.Session = lv.sess.line()
}

func (lv *live) tabHint() string {
	lob, _ := lv.shownLobby()
	switch {
	case !lv.canPick() || lob == nil || len(lob.members) == 0:
		return ""
	case lv.details && lv.picked(lob) != nil:
		return " · esc back · ←→ other players · ↑↓ scroll"
	case lv.picked(lob) != nil:
		return " · enter for details"
	}
	return " · ↑↓ pick a player"
}

var detailColumns = struct {
	who, mode, matches, guns, acts []render.Column
}{
	who: hideEmpty(render.Columns([]string{"party", "name", "agent"}),
		[]render.Column{{Key: "team", Header: "TEAM"}, {Key: "level", Header: "LEVEL", Align: render.Right}},
		render.Columns([]string{"rank", "peak", "last_act", "rr_trend", "met"})),
	mode: []render.Column{{Key: "mode", Header: "THIS ACT"}, {Key: "games", Header: "GAMES", Align: render.Right},
		{Key: "wl", Header: "W-L", Align: render.Right}, {Key: "wr", Header: "WR", Align: render.Right}},
	matches: hideEmpty([]render.Column{{Key: "when", Header: "WHEN", Align: render.Right}, {Key: "map", Header: "MAP"},
		{Key: "agent", Header: "AGENT"}, {Key: "result", Header: "RESULT"}, {Key: "kda", Header: "K/D/A", Align: render.Right},
		{Key: "kd", Header: "K/D", Align: render.Right}, {Key: "hs", Header: "HS%", Align: render.Right},
		{Key: "acs", Header: "ACS", Align: render.Right}, {Key: "adr", Header: "ADR", Align: render.Right},
		{Key: "fb", Header: "FB", Align: render.Right}, {Key: "rr", Header: "RR", Align: render.Right}}),
	guns: []render.Column{{Key: "gun", Header: "GUN"}, {Key: "skin", Header: "SKIN"}, {Key: "buddy", Header: "BUDDY", HideEmpty: true}},
	acts: []render.Column{{Key: "act", Header: "ACT"}, {Key: "rank", Header: "RANK"}, {Key: "peak", Header: "PEAK"},
		{Key: "wl", Header: "W-L", Align: render.Right}, {Key: "wr", Header: "WR", Align: render.Right}},
}

// detailsScreen is everything lowkey already has on one player - it never asks for more so it reads the same
// mid-round as at match load - a hidden player gets all of it except their name and what ties them to others
func (lv *live) detailsScreen(s *render.Screen, lob *lobby, m *member, title string) {
	s.Header = lv.lobbyTitle(lob, title)
	who := lv.memberRow(m, lob.mode)
	if m.p != nil && m.p.name != "" && !m.hidden {
		who["name"] = render.Cell{Text: lv.shownName(m.p)}
	}
	if mark, ok := lv.markers(lob)[m.puuid]; ok {
		who["party"] = render.Cell{Text: mark}
	}
	if lob.mode.Teams && lob.kind == lobbyMatchLoad {
		who["team"] = render.Cell{Text: "enemy"}
		if m.team == lob.ownTeam {
			who["team"] = render.Cell{Text: "ally"}
		}
	}
	if m.level > 0 {
		who["level"] = render.Cell{Text: strconv.Itoa(m.level)}
	}
	if e, ok := lv.metIndex(lob.matchID)[m.puuid]; ok && !m.hidden && m.p != nil {
		who["met"] = render.Cell{Text: e.text(lv.env.Clock.Now())}
	}
	s.Tables = []render.Table{{Columns: detailColumns.who, Rows: []render.Row{who}}}
	p := m.p
	if p == nil {
		s.Tables[0].Note = "not looked up - switched off in the settings"
		return
	}
	s.Tables = append(s.Tables, lv.modesTable(p), lv.agentsTable(p, lob.mode))
	if t, ok := lv.tendencies(lob, p); ok {
		s.Tables = append(s.Tables, t)
	}
	s.Tables = append(s.Tables, lv.matchesTable(p, lob.mode), lv.gunsTable(lob, m), lv.actsTable(p))
}

// tendencies reads only the matches already fetched - site scouting needs a mode with sites to read
func (lv *live) tendencies(lob *lobby, p *player) (render.Table, bool) {
	cfg := lv.cfg.Settings()
	sites, guns := cfg.SiteScouting && lob.mode.Rounds, cfg.WeaponProfiling
	if !sites && !guns || !p.historyDone || p.historyErr != nil {
		return render.Table{}, false
	}
	matches := p.loaded()
	lines := []string{"TENDENCIES"}
	if sites {
		name, ok := lv.data.MapName(lob.mapURL)
		if !ok {
			name = "?"
		}
		s := stats.SitesOf(p.puuid, lob.mapURL, matches, lv.data.Callouts(lob.mapURL))
		if s.Matches == 0 {
			lines = append(lines, "no matches on "+name+" in sample")
		} else {
			lines[0] = fmt.Sprintf("TENDENCIES · %s (%s in sample)", strings.ToUpper(name), count(s.Matches, "match", "matches"))
			lines = append(lines,
				"DEFENSE:  "+shares(s.Defense, "round", "rounds"),
				"ATTACK:   "+shares(s.Plants, "plant", "plants"),
				"OPENING:  "+shares(s.Openings, "opening duel", "opening duels"))
		}
	}
	if guns {
		lines = append(lines, leanings(stats.LeaningsOf(p.puuid, matches, lv.data.Duelist))...)
	}
	return render.Table{Note: strings.Join(lines, "\n")}, true
}

func shares(t stats.Tally, one, many string) string {
	if t.N == 0 {
		return "no " + many + " to go by"
	}
	parts := make([]string, len(t.Shares))
	for i, s := range t.Shares {
		parts[i] = fmt.Sprintf("%s %d%%", s.Place, s.Percent)
	}
	return fmt.Sprintf("%s (%s)", strings.Join(parts, " · "), count(t.N, one, many))
}

func leanings(l stats.Leanings) []string {
	sample := count(l.Matches, "match", "matches") + " in sample"
	if l.Operator == nil && l.Shotguns == nil {
		return []string{"LEANS ON: no gun stands out (" + sample + ")"}
	}
	lines := []string{"LEANS ON (" + sample + "):"}
	if o := l.Operator; o != nil {
		lines = append(lines, fmt.Sprintf("  • Operator on duelists (%d%% of rifle and sniper kills · %s with one)",
			o.Percent, count(o.Rounds, "round", "rounds")))
	}
	if g := l.Shotguns; g != nil {
		lines = append(lines, fmt.Sprintf("  • %s (%s · %d%% of kills)", strings.Join(g.Guns, " and "), count(g.Kills, "kill", "kills"), g.Percent))
	}
	return lines
}

func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// hideEmpty drops a details column nobody has a value in - a team in agent select or a hidden level
func hideEmpty(cols ...[]render.Column) []render.Column {
	out := slices.Concat(cols...)
	for i := range out {
		out[i].HideEmpty = true
	}
	return out
}

func (lv *live) modesTable(p *player) render.Table {
	t := render.Table{Columns: detailColumns.mode}
	switch {
	case !p.mmrDone || lv.eng.acts == nil:
		return t
	case p.mmrErr != nil:
		t.Note = "no rank data"
		return t
	}
	for _, r := range stats.ThisAct(p.mmr, *lv.eng.acts) {
		row := render.Row{"mode": {Text: r.Mode.Name}, "games": {Text: strconv.Itoa(r.Games)}, "wl": dash, "wr": dash}
		if r.Mode.WinRate {
			row["wl"], row["wr"] = wlCells(r.Wins, r.Losses)
		}
		t.Rows = append(t.Rows, row)
	}
	if len(t.Rows) == 0 {
		t.Note = "no games this act"
	}
	return t
}

func (lv *live) agentsTable(p *player, mode stats.Mode) render.Table {
	t := render.Table{Columns: slices.Concat(
		[]render.Column{{Key: "agent", Header: strings.ToUpper(mode.Name)}},
		render.Columns([]string{"n", "wl", "kd"}),
		[]render.Column{{Key: "kda", Header: "KDA", Align: render.Right}},
		render.Columns([]string{"hs", "acs", "adr", "fb"}),
	)}
	switch {
	case !p.historyDone:
		return t
	case p.historyErr != nil:
		t.Note = "no match history"
		return t
	}
	matches := p.loaded()
	all := stats.SampleOf(p.puuid, matches)
	if all.N == 0 {
		if len(p.history) == 0 {
			t.Note = "no recent " + mode.Name + " matches"
		}
		return t
	}
	row := lv.sampleRow(all, mode)
	row["agent"] = render.Cell{Text: "all"}
	t.Rows = append(t.Rows, row)
	for _, a := range stats.AgentsOf(p.puuid, matches) {
		row := lv.sampleRow(a.Sample, mode)
		row["agent"] = lv.agentName(a.Agent)
		t.Rows = append(t.Rows, row)
	}
	return t
}

func (lv *live) sampleRow(s stats.Sample, mode stats.Mode) render.Row {
	row := render.Row{}
	sampleCells(row, s, mode, lv.data)
	row["kda"] = render.Cell{Text: fmt.Sprintf("%.2f", s.KDA())}
	row["wl"] = dash
	if mode.WinRate {
		row["wl"] = render.Cell{Text: fmt.Sprintf("%d-%d", s.Wins, s.Losses)}
	}
	return row
}

func (lv *live) agentName(id string) render.Cell {
	if name, ok := lv.data.AgentName(id); ok {
		return render.Cell{Text: name}
	}
	return render.Cell{Text: "?"}
}

func (lv *live) matchesTable(p *player, mode stats.Mode) render.Table {
	t := render.Table{Columns: detailColumns.matches}
	earned := map[string]int{}
	if p.updatesErr == nil {
		for _, u := range p.updates {
			earned[u.MatchID] = u.Earned
		}
	}
	now := lv.env.Clock.Now()
	for _, m := range p.loaded() {
		mp := m.Players[p.puuid]
		if mp == nil {
			continue
		}
		s := stats.SampleOf(p.puuid, []*stats.Match{m})
		row := render.Row{
			"when":  {Text: ago(now.Sub(m.Start))},
			"agent": lv.agentName(mp.Agent),
			"kda":   {Text: fmt.Sprintf("%d/%d/%d", mp.Kills, mp.Deaths, mp.Assists)},
			"kd":    {Text: fmt.Sprintf("%.2f", s.KD())},
			"hs":    percentCell(s.HS()),
			"acs":   dash, "adr": dash, "fb": dash,
		}
		row["map"] = render.Cell{Text: "?"}
		if name, ok := lv.data.MapName(m.Map); ok {
			row["map"] = render.Cell{Text: name}
		}
		row["result"] = render.Cell{Text: result(m, mp.Team, mode)}
		if mode.Rounds {
			row["acs"], row["adr"] = numberCell(s.ACS()), numberCell(s.ADR())
			row["fb"] = render.Cell{Text: strconv.Itoa(mp.FirstBloods)}
		}
		if rr, ok := earned[m.ID]; ok {
			row["rr"] = render.Cell{Text: signed(rr)}
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// result is W L or D with the round score in the round modes - your rounds first
func result(m *stats.Match, team string, mode stats.Mode) string {
	r := stats.Result(m, team)
	if !mode.Rounds {
		return r
	}
	theirs := 0
	for t, won := range m.RoundsWon {
		if t != team {
			theirs = max(theirs, won)
		}
	}
	return fmt.Sprintf("%s %d-%d", r, m.RoundsWon[team], theirs)
}

// gunOrder is the buy menu's - a gun the list doesn't know yet goes just before the melee
var gunOrder = []string{
	"Classic", "Shorty", "Frenzy", "Ghost", "Bandit", "Sheriff", "Stinger", "Spectre", "Bucky", "Judge",
	"Bulldog", "Guardian", "Phantom", "Vandal", "Warden", "Marshal", "Outlaw", "Operator", "Ares", "Odin", "Melee",
}

func gunRank(name string) int {
	if i := slices.Index(gunOrder, name); i >= 0 {
		return i
	}
	return len(gunOrder) - 1
}

// gunsTable lists every gun with a skin or buddy on it - the rest share one line of standard guns
func (lv *live) gunsTable(lob *lobby, m *member) render.Table {
	t := render.Table{Columns: detailColumns.guns}
	guns := lob.skins[m.puuid]
	if len(guns) == 0 {
		return t
	}
	names := lv.data.Guns()
	ids := make([]string, 0, len(guns))
	for id := range guns {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b string) int {
		if d := gunRank(names[a]) - gunRank(names[b]); d != 0 {
			return d
		}
		return strings.Compare(names[a], names[b])
	})
	var standard []string
	for _, id := range ids {
		g, name := guns[id], names[id]
		if name == "" {
			name = "?"
		}
		skin := lv.skinCell(g.Skin)
		if skin.Text == "Standard" && g.Buddy == "" {
			standard = append(standard, name)
			continue
		}
		row := render.Row{"gun": {Text: name}, "skin": skin}
		if g.Buddy != "" {
			buddy, ok := lv.data.Buddy(g.Buddy)
			if !ok {
				buddy = "?"
			}
			row["buddy"] = render.Cell{Text: render.Cut(buddy, 24)}
		}
		t.Rows = append(t.Rows, row)
	}
	if len(standard) > 0 {
		t.Note = "standard: " + strings.Join(standard, ", ")
	}
	return t
}

func (lv *live) actsTable(p *player) render.Table {
	t := render.Table{Columns: detailColumns.acts}
	if !p.mmrDone || p.mmrErr != nil || lv.eng.acts == nil {
		return t
	}
	for _, a := range stats.ActHistory(p.mmr, *lv.eng.acts, lv.data.CurrentTier) {
		row := render.Row{"act": {Text: a.Label}, "rank": tierCell(lv.data, a.Tier, ""), "peak": tierCell(lv.data, a.Peak, "")}
		row["wl"], row["wr"] = dash, dash
		if a.Wins+a.Losses > 0 {
			row["wl"], row["wr"] = wlCells(a.Wins, a.Losses)
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

func wlCells(wins, losses int) (wl, wr render.Cell) {
	r := stats.Rank{Wins: wins, Losses: losses}
	return render.Cell{Text: fmt.Sprintf("%d-%d", wins, losses)}, render.Cell{Text: fmt.Sprintf("%d%%", r.WinRate())}
}
