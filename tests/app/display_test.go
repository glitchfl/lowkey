package app_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

var update = flag.Bool("update", false, "rewrite the golden screens from the current output")

func plain(frame string) string { return strings.TrimSuffix(frame, "\n\f\n") }

// golden compares a frame with testdata/golden/<name>.txt
func golden(t *testing.T, name, frame string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".txt")
	got := plain(frame) + "\n"
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden screen %s - run go test -update once and review it: %v", path, err)
	}
	if got != string(want) {
		t.Errorf("screen %s changed:\n--- want\n%s--- got\n%s", name, want, got)
	}
}

var sgr = regexp.MustCompile(`\x1b\[([0-9;]*)([A-Za-z])`)

func TestOnlyRankCellsAreColored(t *testing.T) {
	r := newRig(t, matchLobby, lobbySamples)
	r.env.Console.Terminal = true
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "EnemyTwo#E2", "US - THEM")

	colored := regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m([^\x1b]*)\x1b\[0m`)
	tier := regexp.MustCompile(`^(Iron|Brnz|Silv|Gold|Plat|Dia|Asc|Imm)[1-3]( \d+)?$|^Rad( \d+)?$`)
	found := 0
	for _, m := range colored.FindAllStringSubmatch(frame, -1) {
		found++
		if !tier.MatchString(m[4]) {
			t.Errorf("%q is colored but isn't a rank", m[4])
		}
	}
	// diamond 3 is b489c4 in today's tier set
	if !strings.Contains(frame, "\x1b[38;2;180;137;196mDia3 12\x1b[0m") {
		t.Errorf("your Dia3 12 isn't in diamond's color:\n%q", frame)
	}
	if found < 10 {
		t.Errorf("only %d colored cells - every rank, peak and last-act cell should be", found)
	}
	for _, m := range sgr.FindAllStringSubmatch(frame, -1) {
		switch {
		case m[2] == "H" && m[1] == "", m[2] == "K" && m[1] == "", m[2] == "J" && m[1] == "", m[2] == "m":
		default:
			t.Errorf("unexpected escape code %q", m[0])
		}
	}
}

func TestRedirectedOutputHasNoEscapeCodes(t *testing.T) {
	r := newRig(t, matchLobby, lobbySamples)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "EnemyTwo#E2", "US - THEM")
	for _, f := range r.con.framesCopy() {
		if strings.Contains(f, "\x1b") {
			t.Fatalf("a frame written to a non-terminal holds escape codes: %q", f)
		}
	}
	out, _ := newRig(t, ranked).run("me")
	if strings.Contains(out, "\x1b") {
		t.Errorf("me wrote escape codes to a non-terminal: %q", out)
	}
}

func TestNarrowWindowsDropTrailingColumns(t *testing.T) {
	order := []string{"PLAYER", "AGENT", "RANK", "PEAK", "LAST", "WR", "W-L", "K/D", "HS%", "ACS", "ADR", "FB%", "N", "MAINS", "TREND", "MET"}
	for _, width := range []int{80, 60} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			r := newRig(t, matchLobby, lobbySamples)
			r.con.setWidth(width)
			r.live()
			r.screen("waiting for the next match")
			r.subscribed()
			r.enterMatchLoad()
			frame := r.settled("MATCH LOADING", "EnemyTwo", "hidden - widen the window")
			var header []string
			hidden := ""
			for _, l := range strings.Split(plain(frame), "\n") {
				if n := utf8.RuneCountInString(l); n > width {
					t.Errorf("a %d-character line in a %d-wide window: %q", n, width, l)
				}
				if strings.Contains(l, "RANK") && !strings.Contains(l, "·") {
					header = strings.Fields(l)
				}
				if strings.HasSuffix(l, "hidden - widen the window") {
					hidden = l
				}
			}
			for i, h := range header {
				if h != order[i] {
					t.Fatalf("shown columns %v aren't the start of the settings order", header)
				}
			}
			if want := fmt.Sprintf("+%d hidden - widen the window", len(order)-len(header)); hidden != want {
				t.Errorf("note %q, want %q", hidden, want)
			}
		})
	}
}

// widest fills every column with its widest realistic value
func widest(w *fakeriot.World) {
	matchLobby(w)
	w.PartyMembers = []string{w.Self, ally1}
	p := w.Player(ally1)
	p.Name, p.Tag = "LongestNameEver", "12345"
	p.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 27, RR: 1000, Wins: 100, WinsWithPlacements: 100, Games: 200,
		WinsByTier: map[string]int{"27": 3}})
	p.SetRank("competitive", fakeriot.ActLast, fakeriot.SeasonInfo{Tier: 8})
	p.Updates = []fakeriot.Update{{MatchID: "w1", Earned: 30}, {MatchID: "w2", Earned: 30}, {MatchID: "w3", Earned: 30}, {MatchID: "w4", Earned: 30}, {MatchID: "w5", Earned: 30}}
	for i := 0; i < 13; i++ {
		agent := fakeriot.Brimstone
		if i%2 == 1 {
			agent = fakeriot.Astra
		}
		m := &fakeriot.Match{ID: fmt.Sprintf("ffff0000-0000-4000-8000-%012d", i), Queue: "competitive", Map: fakeriot.MapAscent, Mode: fakeriot.ModeBomb,
			Start: testStart.Add(-time.Duration(20+i) * time.Hour),
			Players: []fakeriot.MatchPlayer{
				{PUUID: ally1, Team: "Blue", Agent: agent, Score: 9990, Rounds: 10, Kills: 100, Deaths: 10},
				{PUUID: w.Self, Team: "Red", Agent: fakeriot.Jett, Score: 1000, Rounds: 10, Kills: 5, Deaths: 5},
			},
			Teams: []fakeriot.TeamResult{{ID: "Blue", Won: true}, {ID: "Red"}},
		}
		addRounds(m, ally1, line5{score: 9990, rounds: 10, kills: 100, deaths: 10, head: 10, damage: 9990, firstBloods: 10})
		w.AddMatch(m)
	}
	for i := range w.CoreGame.Players {
		if w.CoreGame.Players[i].PUUID == ally1 {
			w.CoreGame.Players[i].Agent = fakeriot.Brimstone
		}
	}
}

func TestTheFullDefaultColumnSetFitsIn120Columns(t *testing.T) {
	r := newRig(t, widest)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "LongestNameEve", "Rad 1000", "US - THEM")
	if strings.Contains(frame, "hidden - widen the window") {
		t.Errorf("columns were dropped at 120:\n%s", frame)
	}
	for _, want := range []string{"Brimstone", "Rad 1000", "100-100", "10.00", "100%", "999", "Brim Astr", "+150", "enemy L 20h", "●"} {
		if !strings.Contains(frame, want) {
			t.Errorf("the widest row should show %q:\n%s", want, frame)
		}
	}
	for _, l := range strings.Split(plain(frame), "\n") {
		if n := utf8.RuneCountInString(l); n > 120 {
			t.Errorf("%d-character line: %q", n, l)
		}
	}
	golden(t, "widest", frame)
}

func TestGoldenScreens(t *testing.T) {
	t.Run("agent select", func(t *testing.T) {
		r := newRig(t, agentSelectLobby, lobbySamples)
		r.live()
		r.settled("waiting for the next match", "SESSION")
		r.subscribed()
		r.enterAgentSelect()
		golden(t, "agent-select", r.settled("AGENT SELECT", "AllyOne#A1", "AllyTwo#A2"))
	})
	t.Run("match load and last match", func(t *testing.T) {
		r := fromAgentSelect(t, lobbySamples)
		r.enterMatchLoad()
		golden(t, "match-load", r.settled("MATCH LOADING", "EnemyTwo#E2", "US - THEM"))
		r.clock.Advance(40 * time.Second)
		r.screen("IN MATCH")
		r.clock.Advance(35 * time.Minute)
		r.presence(func(w *fakeriot.World) { w.Loop, w.CoreGame = fakeriot.LoopMenus, nil })
		r.screen("LAST MATCH")
		r.clock.Advance(15 * time.Second)
		golden(t, "last-match", r.settled("LAST MATCH", "SESSION"))
	})
	t.Run("deathmatch", func(t *testing.T) {
		r := newRig(t, deathmatch)
		r.live()
		r.settled("waiting for the next match", "SESSION")
		r.subscribed()
		r.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopIngame })
		r.fr.EmitCoreGame(dmMatch, false)
		golden(t, "deathmatch", r.settled("MATCH LOADING", "dm1#DM", "dm2#DM"))
	})
	t.Run("hidden players", func(t *testing.T) {
		r := newRig(t, matchLobby, func(w *fakeriot.World) {
			for _, p := range []string{ally1, enemy1, enemy2} {
				w.Player(p).Incognito = true
			}
		})
		r.live()
		r.settled("waiting for the next match", "SESSION")
		r.subscribed()
		r.enterMatchLoad()
		golden(t, "hidden-players", r.settled("MATCH LOADING", "Plat3 0"))
	})
	t.Run("partial counts", func(t *testing.T) {
		r := fromAgentSelect(t, lobbySamples)
		r.settled("AGENT SELECT", "AllyOne#A1", "AllyTwo#A2")
		r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/match-details/v1/matches/e1e1e1e1-0000-4000-8000-000000000001", Hold: true})
		r.enterMatchLoad()
		r.settledWith(1, "MATCH LOADING", "EnemyOne#E1", "EnemyTwo#E2")
		r.clock.Advance(40 * time.Second)
		golden(t, "partial-counts", r.settled("IN MATCH"))
	})
	t.Run("overflow", func(t *testing.T) {
		r := newRig(t, matchLobby, lobbySamples)
		r.con.setWidth(72)
		r.live()
		r.settled("waiting for the next match", "SESSION")
		r.subscribed()
		r.enterMatchLoad()
		golden(t, "overflow", r.settled("MATCH LOADING", "EnemyTwo", "hidden - widen the window"))
	})
}

func TestGoldenStatusLines(t *testing.T) {
	statuses := []struct {
		name  string
		setup func(w *fakeriot.World)
		act   func(r *rig)
		want  string
	}{
		{"waiting-for-riot-client", func(w *fakeriot.World) { w.RiotClient = false }, nil, "waiting for the Riot Client"},
		{"waiting-for-valorant", func(w *fakeriot.World) { w.Valorant = false }, nil, "waiting for Valorant"},
		{"waiting-for-next-match", ranked, nil, "SESSION"},
		{"mid-match", func(w *fakeriot.World) { w.Loop = fakeriot.LoopIngame }, nil, "match in progress"},
		{"untracked", func(w *fakeriot.World) { agentSelectLobby(w); w.Queue, w.Pregame.Queue = "spikerush", "spikerush" },
			func(r *rig) { r.enterAgentSelect() }, "not tracking spikerush"},
		{"rate-limited", agentSelectLobby, func(r *rig) {
			path := "/mmr/v1/players/" + ally2
			r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: path, Exact: true, Status: 429, Body: rateLimited})
			r.enterAgentSelect()
			// walk through every backoff so the frozen screen is the same each run
			eventually(r.t, "the first 429", func() bool { return r.fr.Answered(path, 429) == 1 })
			for i, wait := range []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second} {
				eventually(r.t, "the backoff timer", func() bool { return r.clock.Pending() == 1 })
				r.clock.Advance(wait)
				eventually(r.t, "a retry", func() bool { return r.fr.Answered(path, 429) == i+2 })
			}
		}, "some stats are missing"},
		{"settings-error", ranked, func(r *rig) { r.writeSettings(`columns = ["kda"]` + "\n") }, `unknown column "kda"`},
		{"unreachable", agentSelectLobby, func(r *rig) {
			for _, svc := range []string{"pd", "glz", "shared"} {
				r.fr.AddFault(fakeriot.Fault{Service: svc, Drop: true})
			}
			r.enterAgentSelect()
		}, "can't reach Riot's servers"},
	}
	for _, s := range statuses {
		t.Run(s.name, func(t *testing.T) {
			r := newRig(t, s.setup)
			r.live()
			if s.act != nil {
				r.settled("waiting for the next match", "SESSION")
				r.subscribed()
				s.act(r)
			}
			golden(t, "status-"+s.name, r.settled(s.want))
		})
	}
}

func TestNewDataRedrawsAtMostAboutFourTimesASecond(t *testing.T) {
	r := fromAgentSelect(t, lobbySamples)
	r.settled("AGENT SELECT", "AllyOne#A1", "AllyTwo#A2")
	// slow answers spread the enemies' lookups over a few hundred milliseconds
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Delay: 40 * time.Millisecond})
	from := len(r.con.writeTimes())
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "EnemyTwo#E2", "US - THEM")
	times := r.con.writeTimes()[from:]
	elapsed := times[len(times)-1].Sub(times[0])
	// the phase change and the finished table draw at once - everything between waits its turn
	allowed := int(elapsed/(250*time.Millisecond)) + 3
	if len(times) > allowed {
		t.Errorf("%d frames in %v of match load, want at most %d", len(times), elapsed.Round(time.Millisecond), allowed)
	}
}
