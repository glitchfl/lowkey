package lobbytable

import (
	"fmt"
	"math"

	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

// averages are the shown values of a team's average row - the diff subtracts what's on screen
type averages struct {
	covered, size    int
	rank, peak, last int
	wr, trend        int
	hasWR, hasTrend  bool
	kd               float64
	hasKD            bool
	hs, acs, adr, fb int
	hasHS, hasRounds bool
}

func teamAverages(members []Member, mode stats.Mode, acts *stats.Acts, data *static.Data) averages {
	var tiers, peaks, lasts, wins, trends []float64
	var kds, hss, acss, adrs, fbs []float64
	a := averages{size: len(members)}
	for _, m := range members {
		p := m.Player
		if p == nil {
			continue
		}
		if p.Matches.State == Ready {
			if s := stats.SampleOf(p.PUUID, p.Matches.Value); s.N > 0 {
				kds = append(kds, s.KD())
				if v, ok := s.HSExact(); ok {
					hss = append(hss, v)
				}
				if mode.Rounds {
					acs, _ := s.ACSExact()
					adr, _ := s.ADRExact()
					fb, _ := s.FBExact()
					acss, adrs, fbs = append(acss, acs), append(adrs, adr), append(fbs, fb)
				}
			}
		}
		if p.Updates.State == Ready {
			if t, ok := stats.Trend(p.Updates.Value, TrendGames); ok {
				trends = append(trends, float64(t))
			}
		}
		if p.MMR.State != Ready || acts == nil {
			continue
		}
		a.covered++
		r := stats.RankOf(p.MMR.Value, *acts, mode.Queue, data.CurrentTier)
		if r.Tier >= 3 {
			tiers = append(tiers, float64(r.Tier))
		}
		if r.Peak >= 3 {
			peaks = append(peaks, float64(r.Peak))
		}
		if r.Last >= 3 {
			lasts = append(lasts, float64(r.Last))
		}
		if mode.WinRate && r.HasRecord {
			wins = append(wins, r.WinRatio()*100)
		}
	}
	a.rank, _ = roundedMean(tiers)
	a.peak, _ = roundedMean(peaks)
	a.last, _ = roundedMean(lasts)
	a.wr, a.hasWR = roundedMean(wins)
	a.trend, a.hasTrend = roundedMean(trends)
	if v, ok := mean(kds); ok {
		a.kd, a.hasKD = math.Round(v*100)/100, true
	}
	a.hs, a.hasHS = roundedMean(hss)
	a.acs, a.hasRounds = roundedMean(acss)
	a.adr, _ = roundedMean(adrs)
	a.fb, _ = roundedMean(fbs)
	return a
}

func (a averages) row(data *static.Data) render.Row {
	label := "AVERAGE"
	if a.covered < a.size {
		label += fmt.Sprintf(" %d/%d", a.covered, a.size)
	}
	row := render.Row{"name": {Text: label}}
	if a.rank > 0 {
		row["rank"] = TierCell(data, a.rank, "")
	}
	if a.peak > 0 {
		row["peak"] = TierCell(data, a.peak, "")
	}
	if a.last > 0 {
		row["last_act"] = TierCell(data, a.last, "")
	}
	if a.hasWR {
		row["wr"] = render.Cell{Text: fmt.Sprintf("%d%%", a.wr)}
	}
	if a.hasTrend {
		row["rr_trend"] = render.Cell{Text: Signed(a.trend)}
	}
	if a.hasKD {
		row["kd"] = render.Cell{Text: fmt.Sprintf("%.2f", a.kd)}
	}
	if a.hasHS {
		row["hs"] = render.Cell{Text: fmt.Sprintf("%d%%", a.hs)}
	}
	if a.hasRounds {
		row["acs"] = render.Cell{Text: fmt.Sprint(a.acs)}
		row["adr"] = render.Cell{Text: fmt.Sprint(a.adr)}
		row["fb"] = render.Cell{Text: fmt.Sprintf("%d%%", a.fb)}
	}
	return row
}

// diffRow is us minus them - ranks in tiers - for every stat both teams have
func diffRow(us, them averages) render.Row {
	row := render.Row{"name": {Text: "US - THEM"}}
	diff := func(key string, a, b int, ok bool) {
		if ok {
			row[key] = render.Cell{Text: arrow(a - b)}
		}
	}
	diff("rank", us.rank, them.rank, us.rank > 0 && them.rank > 0)
	diff("peak", us.peak, them.peak, us.peak > 0 && them.peak > 0)
	diff("last_act", us.last, them.last, us.last > 0 && them.last > 0)
	diff("wr", us.wr, them.wr, us.hasWR && them.hasWR)
	diff("rr_trend", us.trend, them.trend, us.hasTrend && them.hasTrend)
	diff("hs", us.hs, them.hs, us.hasHS && them.hasHS)
	diff("acs", us.acs, them.acs, us.hasRounds && them.hasRounds)
	diff("adr", us.adr, them.adr, us.hasRounds && them.hasRounds)
	diff("fb", us.fb, them.fb, us.hasRounds && them.hasRounds)
	if us.hasKD && them.hasKD {
		row["kd"] = render.Cell{Text: arrowKD(us.kd - them.kd)}
	}
	return row
}

func arrowKD(d float64) string {
	d = math.Round(d*100) / 100
	switch {
	case d > 0:
		return fmt.Sprintf("▲%.2f", d)
	case d < 0:
		return fmt.Sprintf("▼%.2f", -d)
	}
	return "0"
}

func arrow(d int) string {
	switch {
	case d > 0:
		return fmt.Sprintf("▲%d", d)
	case d < 0:
		return fmt.Sprintf("▼%d", -d)
	}
	return "0"
}

func mean(vs []float64) (float64, bool) {
	if len(vs) == 0 {
		return 0, false
	}
	sum := 0.0
	for _, v := range vs {
		sum += v
	}
	return sum / float64(len(vs)), true
}

func roundedMean(vs []float64) (int, bool) {
	v, ok := mean(vs)
	return int(math.Round(v)), ok
}
