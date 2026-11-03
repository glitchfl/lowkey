// leanprobe runs lowkey's own weapon-profiling rule over every match a recording holds, so its
// thresholds can be judged against real players. It reads the recording only and sends nothing.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/glitchfl/lowkey/internal/platform"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

const (
	operator = "a03b24d3-4319-996d-0f8c-94bbfba1dfc7"
	judge    = "ec845bf4-4f79-ddda-a3da-0db3774b2794"
	bucky    = "910be174-449b-c412-ab22-d0873436b21b"
)

var long = []string{
	"ae3de142-4d85-2547-dd26-4e90bed35cf7", "4ade7faa-4cf1-8376-95ef-39884480959b",
	"ee8e8d15-496b-07ac-e5f6-8fae5d4c7b1a", "9c82e19d-4575-0200-1a81-3eacf00cf872",
	"8db0a1bf-4a50-832a-4566-faaaa6d250ca", "c4883e50-4494-202c-3ec3-6b8a9284f00b",
	"5f0aaf7a-4289-3998-d5ff-eb9a5cf7ef5c", operator,
}

type row struct {
	Kind   string          `json:"kind"`
	Path   string          `json:"path"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

type tallyRow struct {
	matches, rounds, kills                     int
	duelMatches, duelRounds, opKills, longKill int
	opRounds, shotgun                          int
	lean                                       stats.Leanings
}

func main() {
	platform.Prepare() // below normal priority and EcoQoS, like lowkey itself
	data := static.Embedded()
	matches := map[string]*stats.Match{}
	for _, path := range os.Args[1:] {
		f, err := os.Open(path)
		if err != nil {
			fmt.Println(err)
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
		for sc.Scan() {
			line := sc.Bytes()
			if !strings.Contains(string(line[:min(len(line), 300)]), "/match-details/v1/matches/") {
				continue
			}
			var r row
			if json.Unmarshal(line, &r) != nil || r.Kind != "remote" || r.Status != 200 {
				continue
			}
			m, err := stats.DecodeMatch(r.Body)
			if err != nil || m == nil {
				continue
			}
			matches[m.ID] = m
		}
		f.Close()
	}

	// lobby players are the ones lowkey fetched a sample for - they show up in many matches
	byPlayer := map[string][]*stats.Match{}
	for _, m := range matches {
		for id := range m.Players {
			byPlayer[id] = append(byPlayer[id], m)
		}
	}
	var out []tallyRow
	for id, ms := range byPlayer {
		if len(ms) < 5 {
			continue
		}
		t := tallyRow{lean: stats.LeaningsOf(id, ms, data.Duelist)}
		for _, m := range ms {
			p := m.Players[id]
			t.matches++
			t.rounds += p.Rounds
			for w, n := range p.KillsWith {
				t.kills += n
				if w == judge || w == bucky {
					t.shotgun += n
				}
			}
			if !data.Duelist(p.Agent) {
				continue
			}
			t.duelMatches++
			t.duelRounds += p.Rounds
			t.opKills += p.KillsWith[operator]
			t.opRounds += p.Loadouts[operator]
			for _, w := range long {
				t.longKill += p.KillsWith[w]
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].opRounds > out[j].opRounds })

	pct := func(a, b int) string {
		if b == 0 {
			return "-"
		}
		return fmt.Sprintf("%d%%", a*100/b)
	}
	fmt.Printf("%d matches decoded, %d lobby players with 5+ of them\n\n", len(matches), len(out))
	fmt.Println("N    RND  | DUEL N RND  OP-RND  OP-RND%  OPK/LONG  | SHOTGUN K  %KILLS  | FLAGGED")
	opFlag, sgFlag := 0, 0
	for _, t := range out {
		flags := []string{}
		if t.lean.Operator != nil {
			flags = append(flags, "operator")
			opFlag++
		}
		if t.lean.Shotguns != nil {
			flags = append(flags, "shotguns")
			sgFlag++
		}
		fmt.Printf("%-4d %-4d | %-6d %-4d %-7d %-8s %-9s | %-9d %-7s | %s\n",
			t.matches, t.rounds, t.duelMatches, t.duelRounds, t.opRounds, pct(t.opRounds, t.duelRounds),
			pct(t.opKills, t.longKill), t.shotgun, pct(t.shotgun, t.kills), strings.Join(flags, " "))
	}
	fmt.Printf("\nflagged: operator %d of %d, shotguns %d of %d\n", opFlag, len(out), sgFlag, len(out))
}
