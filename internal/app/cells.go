package app

import (
	"fmt"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

var dash = render.Cell{Text: "-"}

func tierCell(data *static.Data, tier int, suffix string) render.Cell {
	label := data.TierLabel(tier)
	if label == "" {
		return dash
	}
	return render.Cell{Text: label + suffix, Color: data.TierColor(tier)}
}

func rankCells(row render.Row, r stats.Rank, data *static.Data, mode stats.Mode) {
	row["rank"] = tierCell(data, r.Tier, fmt.Sprintf(" %d", r.RR))
	row["peak"] = tierCell(data, r.Peak, "")
	row["last_act"] = tierCell(data, r.Last, "")
	if !mode.WinRate || !r.HasRecord {
		row["wr"], row["wl"] = dash, dash
		return
	}
	row["wr"] = render.Cell{Text: fmt.Sprintf("%d%%", r.WinRate())}
	row["wl"] = render.Cell{Text: fmt.Sprintf("%d-%d", r.Wins, r.Losses)}
}
