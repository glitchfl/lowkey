package app_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/app"
	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const (
	agentDotRow = weaponRow + 1 + iota
	dotTimeRow
	placeDotRow
)

const dotOn = "agent_dot = true\nagent_dot_at = [800, 450]\n"

type dotCall struct {
	shown bool
	x, y  int
	at    time.Time
}

// fakeDot stands in for the window - it keeps every show and hide with the fake clock's time
type fakeDot struct {
	t     *testing.T
	clock *fakeriot.Clock

	mu       sync.Mutex
	calls    []dotCall
	up       bool
	cursorOK bool
}

const cursorX, cursorY = 1234, 567

func (r *rig) withDot() *fakeDot {
	d := &fakeDot{t: r.t, clock: r.clock, cursorOK: true}
	r.env.Dot = app.Dot{Show: d.show, Hide: d.hide, Cursor: d.cursor}
	return d
}

func (d *fakeDot) show(x, y int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.up {
		d.t.Errorf("a second dot at %d, %d while one was still up", x, y)
	}
	d.up = true
	d.calls = append(d.calls, dotCall{shown: true, x: x, y: y, at: d.clock.Now()})
	return nil
}

func (d *fakeDot) hide() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.up {
		d.t.Error("hid a dot that wasn't up")
	}
	d.up = false
	d.calls = append(d.calls, dotCall{at: d.clock.Now()})
}

func (d *fakeDot) cursor() (int, int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return cursorX, cursorY, d.cursorOK
}

func (d *fakeDot) isUp() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.up
}

func (d *fakeDot) shows() []dotCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []dotCall
	for _, c := range d.calls {
		if c.shown {
			out = append(out, c)
		}
	}
	return out
}

// upFor is how long the last dot stayed up on the fake clock
func (d *fakeDot) upFor() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.calls)
	if n < 2 || d.calls[n-1].shown || !d.calls[n-2].shown {
		d.t.Fatalf("no dot that went up and came down in %v", d.calls)
	}
	return d.calls[n-1].at.Sub(d.calls[n-2].at)
}

func (d *fakeDot) wantUpAt(x, y int) {
	d.t.Helper()
	eventually(d.t, fmt.Sprintf("the dot at %d, %d", x, y), d.isUp)
	shows := d.shows()
	if last := shows[len(shows)-1]; last.x != x || last.y != y {
		d.t.Errorf("dot at %d, %d - want %d, %d", last.x, last.y, x, y)
	}
}

func (d *fakeDot) wantDown(why string) {
	d.t.Helper()
	eventually(d.t, "the dot to go "+why, func() bool { return !d.isUp() })
}

func (d *fakeDot) wantNone() {
	d.t.Helper()
	settle()
	if shows := d.shows(); len(shows) != 0 {
		d.t.Errorf("the dot went up %d times, want never", len(shows))
	}
}

// dotLobby starts lowkey in the menus with the given settings and a dot it can draw
func dotLobby(t *testing.T, settings string, setup ...func(w *fakeriot.World)) (*rig, *fakeDot) {
	t.Helper()
	r := newRig(t, append([]func(w *fakeriot.World){agentSelectLobby}, setup...)...)
	d := r.withDot()
	r.writeSettings(settings)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	return r, d
}

func dotInAgentSelect(t *testing.T, settings string, setup ...func(w *fakeriot.World)) (*rig, *fakeDot) {
	t.Helper()
	r, d := dotLobby(t, settings, setup...)
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	return r, d
}

func (r *rig) self() string {
	var self string
	r.fr.Update(func(w *fakeriot.World) { self = w.Self })
	return self
}

func TestTheAgentDotShowsWhenAMatchIsFoundUntilItsTimeLimit(t *testing.T) {
	r, d := dotInAgentSelect(t, dotOn+"agent_dot_seconds = 4\n")
	d.wantUpAt(800, 450)
	r.clock.Advance(3 * time.Second)
	settle()
	if !d.isUp() {
		t.Fatal("the dot went before its 4 s were up")
	}
	r.clock.Advance(time.Second)
	d.wantDown("at its time limit")
	if got := d.upFor(); got != 4*time.Second {
		t.Errorf("the dot stayed up %v, want 4s", got)
	}
}

// the Windows session measured 12.8 s from the match found to the first click
func TestTheAgentDotTimeLimitIsFifteenSecondsByDefault(t *testing.T) {
	r, d := dotInAgentSelect(t, dotOn)
	d.wantUpAt(800, 450)
	r.clock.Advance(13 * time.Second)
	settle()
	if !d.isUp() {
		t.Fatal("the dot went before agent select could be clicked")
	}
	r.clock.Advance(2 * time.Second)
	d.wantDown("at its time limit")
	if got := d.upFor(); got != 15*time.Second {
		t.Errorf("the dot stayed up %v, want 15s", got)
	}
}

func TestTheAgentDotGoesOnceYouPickAnAgent(t *testing.T) {
	for _, locked := range []bool{false, true} {
		t.Run(map[bool]string{false: "hover", true: "lock"}[locked], func(t *testing.T) {
			r, d := dotInAgentSelect(t, dotOn)
			d.wantUpAt(800, 450)
			r.fr.Hover(r.self(), fakeriot.Sova, locked)
			settle()
			if !d.isUp() {
				t.Fatal("the dot went before lowkey saw your pick")
			}
			r.clock.Advance(3 * time.Second)
			d.wantDown("after your pick")
			if got := d.upFor(); got > 3*time.Second {
				t.Errorf("the dot stayed up %v after the pick, want at most the 3 s roster refresh", got)
			}
		})
	}
}

// the stats lookups fill both request slots right after a match is found - your pick's roster fetch
// has to jump them or the dot would stay up through the whole burst
func TestAPickDuringTheStatsLookupsStillTakesTheAgentDotDown(t *testing.T) {
	r, d := dotLobby(t, dotOn, func(w *fakeriot.World) {
		line := line5{score: 3000, rounds: 20, kills: 15, deaths: 15, head: 3, body: 12, damage: 2500}
		playedMatches(w, ally1, "b1b1b1b1", 8, fakeriot.Jett, line)
		playedMatches(w, ally2, "b2b2b2b2", 8, fakeriot.Killjoy, line)
	})
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/match-details/", Hold: true})
	r.enterAgentSelect()
	held := waitHeld(t, r.fr, 2)
	d.wantUpAt(800, 450)
	r.fr.Hover(r.self(), fakeriot.Sova, true)
	settle()
	r.clock.Advance(3 * time.Second)
	settle()
	if !d.isUp() {
		t.Fatal("the dot went before any request slot was free for the roster")
	}
	mark := len(r.fr.RemoteRequests())
	r.fr.Release(held[0])
	d.wantDown("after your pick")
	// other top-priority work like the content service may go first - a stats lookup may not
	for _, req := range r.fr.RemoteRequests()[mark:] {
		if strings.HasPrefix(req.Path, "/pregame/v1/matches/") {
			break
		}
		if strings.HasPrefix(req.Path, "/match-details/") {
			t.Errorf("%s went out ahead of the roster with your pick", req.Path)
		}
	}
	r.fr.RemoveFaults()
	r.fr.ReleaseHeld()
}

func TestATeammatesPickLeavesTheAgentDotUp(t *testing.T) {
	r, d := dotInAgentSelect(t, dotOn)
	d.wantUpAt(800, 450)
	r.fr.Hover(ally2, fakeriot.Killjoy, true)
	settle()
	r.clock.Advance(3 * time.Second)
	r.screen("Killjoy")
	if !d.isUp() {
		t.Error("a teammate's lock took your dot down")
	}
}

func TestADodgeTakesTheAgentDotDownAndTheNextMatchBringsItBack(t *testing.T) {
	r, d := dotInAgentSelect(t, dotOn)
	d.wantUpAt(800, 450)
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("waiting for the next match")
	if d.isUp() {
		t.Fatal("the dot is still up in the menus after a dodge")
	}
	r.enterAgentSelect()
	r.screen("AGENT SELECT")
	d.wantUpAt(800, 450)
	if n := len(d.shows()); n != 2 {
		t.Errorf("the dot went up %d times over two agent selects", n)
	}
}

func TestTheAgentDotIsNeverUpOnceTheMatchLoads(t *testing.T) {
	r := newRig(t, matchLobby, func(w *fakeriot.World) {
		for i := range w.Pregame.Players {
			if w.Pregame.Players[i].PUUID == w.Self {
				w.Pregame.Players[i].Agent, w.Pregame.Players[i].State = "", ""
			}
		}
	})
	d := r.withDot()
	r.writeSettings(dotOn + "agent_dot_seconds = 20\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.screen("AGENT SELECT")
	d.wantUpAt(800, 450)

	r.enterMatchLoad()
	r.screen("MATCH LOADING")
	if d.isUp() {
		t.Fatal("the dot is up at match load")
	}
	r.settled("MATCH LOADING", "US - THEM")
	r.clock.Advance(time.Minute)
	r.screen("IN MATCH")
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("LAST MATCH")
	if n := len(d.shows()); n != 1 {
		t.Errorf("the dot went up %d times in one match", n)
	}
	if d.isUp() {
		t.Error("the dot is up after the match")
	}
}

func TestNoAgentDotUnlessItsOnAndPlaced(t *testing.T) {
	for name, file := range map[string]string{
		"off":        "agent_dot = false\nagent_dot_at = [800, 450]\n",
		"not placed": "agent_dot = true\n",
		"bad spot":   "agent_dot = true\nagent_dot_at = [800]\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, d := dotInAgentSelect(t, file)
			d.wantNone()
		})
	}
}

func TestNoAgentDotInModesLowkeyDoesntTrack(t *testing.T) {
	cases := map[string]struct {
		file, queue, status string
	}{
		"switched off in modes": {dotOn + `modes = ["unrated"]` + "\n", "competitive", "not tracking competitive"},
		"spike rush":            {dotOn, "spikerush", "not tracking spikerush"},
		"custom game":           {dotOn, "", "not tracking custom games"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r, d := dotLobby(t, c.file, func(w *fakeriot.World) { w.Queue, w.Pregame.Queue = c.queue, c.queue })
			r.enterAgentSelect()
			r.screen(c.status)
			d.wantNone()
		})
	}
}

func TestNoAgentDotWhenLowkeyOpensMidAgentSelect(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) { w.Loop = fakeriot.LoopPregame })
	d := r.withDot()
	r.writeSettings(dotOn)
	r.live()
	r.settled("AGENT SELECT", "Dia1 50")
	d.wantNone()
}

func TestTurningTheAgentDotOffTakesItDown(t *testing.T) {
	r, d := dotInAgentSelect(t, dotOn)
	d.wantUpAt(800, 450)
	r.writeSettings("agent_dot = false\nagent_dot_at = [800, 450]\n")
	d.wantDown("when it's turned off")
}

func TestTheAgentDotGoesWhenLowkeyCloses(t *testing.T) {
	r, d := dotInAgentSelect(t, dotOn)
	d.wantUpAt(800, 450)
	r.stop()
	if d.isUp() {
		t.Error("the dot outlived lowkey's window")
	}
}

func TestTheAgentDotMakesNoRequests(t *testing.T) {
	requests := func(file string) []string {
		r, _ := dotInAgentSelect(t, file)
		r.fr.Hover(r.self(), fakeriot.Sova, true)
		settle()
		r.clock.Advance(15 * time.Second)
		r.settled("AGENT SELECT", "Sova")
		var paths []string
		for _, req := range r.fr.RemoteRequests() {
			paths = append(paths, req.Path)
		}
		// lookups run side by side so only the set of requests is stable
		slices.Sort(paths)
		return paths
	}
	on, off := requests(dotOn), requests("agent_dot = false\n")
	if strings.Join(on, "\n") != strings.Join(off, "\n") {
		t.Errorf("agent select with the dot made %d requests, without it %d:\n%v\n%v", len(on), len(off), on, off)
	}
}

func TestPlacingTheDotSavesWhereTheCursorRestsAfterFiveSecondsAndTurnsItOn(t *testing.T) {
	r := newRig(t)
	d := r.withDot()
	frame := r.settingsTab()
	if !regexp.MustCompile(`agent dot +off`).MatchString(frame) {
		t.Fatalf("want the dot off before placing:\n%s", frame)
	}
	r.press(upKey)
	r.screen("> place dot", "not placed  (enter)", "rest the cursor on your agent")
	before := len(r.fr.RemoteRequests())
	r.press(enterKey)
	r.screen("point at your agent - 5")
	for left := 4; left >= 1; left-- {
		r.clock.Advance(time.Second)
		r.screen(fmt.Sprintf("point at your agent - %d", left))
	}
	r.clock.Advance(time.Second)
	r.saved(fmt.Sprintf("agent_dot_at = [%d, %d]", cursorX, cursorY))
	r.saved("agent_dot = true")
	r.screen(fmt.Sprintf("%d, %d", cursorX, cursorY))
	eventually(t, "the agent dot row to say on", func() bool { return regexp.MustCompile(`agent dot +on`).MatchString(r.con.last()) })
	d.wantUpAt(cursorX, cursorY)
	r.clock.Advance(3 * time.Second)
	d.wantDown("after its 3 s check")
	if after := len(r.fr.RemoteRequests()); after != before {
		t.Errorf("placing the dot made %d requests", after-before)
	}
}

func TestPlacingKeepsCountingWhenANewLobbyTakesTheScreen(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	d := r.withDot()
	r.settingsTab()
	r.subscribed()
	r.press(upKey, enterKey)
	r.screen("point at your agent - 5")
	r.enterAgentSelect()
	r.settled("[LOBBY]", "AGENT SELECT", "Dia1 50")
	for range 5 {
		r.clock.Advance(time.Second)
		settle()
	}
	r.saved(fmt.Sprintf("agent_dot_at = [%d, %d]", cursorX, cursorY))
	d.wantUpAt(cursorX, cursorY)
}

func TestPlacingDuringAMatchSavesButDrawsNothing(t *testing.T) {
	r := newRig(t, matchLobby)
	d := r.withDot()
	r.liveWithKeyboard()
	r.subscribed()
	r.enterAgentSelect()
	r.screen("AGENT SELECT")
	r.enterMatchLoad()
	r.settled("[LOBBY]", "MATCH LOADING", "US - THEM")
	r.press(tabKey, upKey, enterKey)
	r.screen("point at your agent - 5")
	for left := 4; left >= 1; left-- {
		r.clock.Advance(time.Second)
		r.screen(fmt.Sprintf("point at your agent - %d", left))
	}
	r.clock.Advance(time.Second)
	r.saved(fmt.Sprintf("agent_dot_at = [%d, %d]", cursorX, cursorY))
	d.wantNone()
}

func TestAnUnreadableCursorSaysSoAndSavesNothing(t *testing.T) {
	r := newRig(t)
	d := r.withDot()
	d.cursorOK = false
	r.settingsTab()
	r.press(upKey, enterKey)
	r.screen("point at your agent - 5")
	for left := 4; left >= 0; left-- {
		r.clock.Advance(time.Second)
		if left > 0 {
			r.screen(fmt.Sprintf("point at your agent - %d", left))
		}
	}
	r.screen("can't read where the cursor is", "not placed")
	if file := r.settingsFile(); !strings.Contains(file, "agent_dot_at = []") {
		t.Errorf("an unread cursor changed the spot:\n%s", file)
	}
	d.wantNone()
}

func TestTheAgentDotRowsChangeTheFile(t *testing.T) {
	r := newRig(t)
	r.withDot()
	r.settingsTab()
	r.onRow(agentDotRow)
	r.screen("> agent dot", "needs windowed or windowed fullscreen")
	r.press(enterKey)
	r.saved("agent_dot = true")
	r.press(downKey)
	r.screen("> dot time", "< 15s >")
	for range 12 {
		r.press(rightKey)
	}
	r.saved("agent_dot_seconds = 20")
	for range 25 {
		r.press(leftKey)
	}
	r.saved("agent_dot_seconds = 1")
}

func TestWithoutADotThereAreNoDotRows(t *testing.T) {
	r := newRig(t)
	frame := r.settingsTab()
	for _, row := range []string{"agent dot", "dot time", "place dot"} {
		if strings.Contains(frame, row) {
			t.Errorf("a build that can't draw the dot shows %q:\n%s", row, frame)
		}
	}
}

func TestARecordingSaysWhenTheAgentDotWentUpAndDown(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	d := r.withDot()
	r.writeSettings(dotOn + "agent_dot_seconds = 4\n")
	r.live("--record")
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	d.wantUpAt(800, 450)
	r.clock.Advance(4 * time.Second)
	d.wantDown("at its time limit")
	r.stop()

	entries, _ := readRecording(t, recordingDir(t, r.dir))
	type dotEntry struct {
		Shown bool   `json:"shown"`
		Why   string `json:"why"`
		X     int    `json:"x"`
		Y     int    `json:"y"`
	}
	var got []dotEntry
	for _, e := range entries {
		if e.Kind != "dot" {
			continue
		}
		var de dotEntry
		if err := json.Unmarshal(e.Body, &de); err != nil {
			t.Fatal(err)
		}
		got = append(got, de)
	}
	want := []dotEntry{{true, "match found", 800, 450}, {false, "time limit", 800, 450}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("dot entries: got %v, want %v", got, want)
	}
}
