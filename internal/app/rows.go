package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

var sampleKeys = []string{"name", "kd", "hs", "acs", "adr", "fb", "mains"}

// playerRow shows what's known - cells still loading stay blank, cells with no value show "-"
func playerRow(p *player, mode stats.Mode, acts *stats.Acts, data *static.Data) render.Row {
	row := render.Row{}
	switch {
	case p.mmrDone && p.mmrErr == nil && acts != nil:
		rankCells(row, stats.RankOf(p.mmr, *acts, mode.Queue, data.CurrentTier), data, mode)
	case p.mmrDone:
		for _, k := range []string{"rank", "peak", "last_act", "wr", "wl"} {
			row[k] = dash
		}
	}
	if p.updatesDone {
		if trend, ok := stats.Trend(p.updates, 5); ok && p.updatesErr == nil {
			row["rr_trend"] = render.Cell{Text: signed(trend)}
		} else {
			row["rr_trend"] = dash
		}
	}
	if p.historyDone {
		if p.historyErr != nil {
			for _, k := range append(sampleKeys, "n") {
				row[k] = dash
			}
		} else {
			sampleCells(row, stats.SampleOf(p.puuid, p.loaded()), mode, data)
		}
	}
	return row
}

func sampleCells(row render.Row, s stats.Sample, mode stats.Mode, data *static.Data) {
	row["n"] = render.Cell{Text: strconv.Itoa(s.N)}
	if s.N == 0 {
		for _, k := range sampleKeys {
			row[k] = dash
		}
		return
	}
	row["name"] = render.Cell{Text: cut(s.Name, 16)}
	row["kd"] = render.Cell{Text: fmt.Sprintf("%.2f", s.KD())}
	row["hs"] = percentCell(s.HS())
	if mode.Rounds {
		row["acs"] = numberCell(s.ACS())
		row["adr"] = numberCell(s.ADR())
		row["fb"] = percentCell(s.FB())
	} else {
		row["acs"], row["adr"], row["fb"] = dash, dash, dash
	}
	var mains []string
	for _, agent := range s.Mains {
		if name, ok := data.AgentName(agent); ok {
			mains = append(mains, static.AgentAbbr(name))
		} else {
			mains = append(mains, "?")
		}
	}
	row["mains"] = render.Cell{Text: strings.Join(mains, " ")}
}

func percentCell(v int, ok bool) render.Cell {
	if !ok {
		return dash
	}
	return render.Cell{Text: strconv.Itoa(v) + "%"}
}

func numberCell(v int, ok bool) render.Cell {
	if !ok {
		return dash
	}
	return render.Cell{Text: strconv.Itoa(v)}
}

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
