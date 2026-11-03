package lobbytable

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

// the formats below are shared with player details so a stat reads the same in both

var Dash = render.Cell{Text: "-"}

var sampleKeys = []string{"kd", "hs", "acs", "adr", "fb", "mains"}

func TierCell(data *static.Data, tier int, suffix string) render.Cell {
	label := data.TierLabel(tier)
	if label == "" {
		return Dash
	}
	return render.Cell{Text: label + suffix, Color: data.TierColor(tier)}
}

func rankCells(row render.Row, r stats.Rank, data *static.Data, mode stats.Mode) {
	row["rank"] = TierCell(data, r.Tier, fmt.Sprintf(" %d", r.RR))
	row["peak"] = TierCell(data, r.Peak, "")
	row["last_act"] = TierCell(data, r.Last, "")
	if !mode.WinRate || !r.HasRecord {
		row["wr"], row["wl"] = Dash, Dash
		return
	}
	row["wr"] = render.Cell{Text: fmt.Sprintf("%d%%", r.WinRate())}
	row["wl"] = render.Cell{Text: fmt.Sprintf("%d-%d", r.Wins, r.Losses)}
}

func SampleCells(row render.Row, s stats.Sample, mode stats.Mode, data *static.Data) {
	row["n"] = render.Cell{Text: strconv.Itoa(s.N)}
	if s.N == 0 {
		for _, k := range sampleKeys {
			row[k] = Dash
		}
		return
	}
	row["kd"] = render.Cell{Text: fmt.Sprintf("%.2f", s.KD())}
	row["hs"] = PercentCell(s.HS())
	if mode.Rounds {
		row["acs"] = NumberCell(s.ACS())
		row["adr"] = NumberCell(s.ADR())
		row["fb"] = PercentCell(s.FB())
	} else {
		row["acs"], row["adr"], row["fb"] = Dash, Dash, Dash
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

func PercentCell(v int, ok bool) render.Cell {
	if !ok {
		return Dash
	}
	return render.Cell{Text: strconv.Itoa(v) + "%"}
}

func NumberCell(v int, ok bool) render.Cell {
	if !ok {
		return Dash
	}
	return render.Cell{Text: strconv.Itoa(v)}
}

// SkinCell is in its edition's color - "?" is a skin newer than the snapshot until a refresh names it
func SkinCell(data *static.Data, id string) render.Cell {
	s, ok := data.Skin(id)
	if !ok {
		return render.Cell{Text: "?"}
	}
	return render.Cell{Text: Cut(s.Name, 16), Color: data.EditionColor(s.Edition)}
}

func Signed(n int) string {
	if n < 0 {
		return strconv.Itoa(n)
	}
	return "+" + strconv.Itoa(n)
}

func Cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
