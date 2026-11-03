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
	"github.com/glitchfl/lowkey/internal/platform"
)

// nobody in the test lobbies plays Raze so a teammate's pick never takes this dot down
const dotOn = "[agent_select]\nagent_dot = true\nagent_dots = [{agent = \"Raze\", at = [800, 450]}]\n"

func dotsOn(dots string) string {
	return "[agent_select]\nagent_dot = true\nagent_dots = [" + dots + "]\n"
}

type dotCall struct {
	marks []platform.Mark // nil is a hide
	at    time.Time
}

// fakeDot stands in for the windows - it keeps every show and hide with the fake clock's time
type fakeDot struct {
	t     *testing.T
	clock *fakeriot.Clock

	mu       sync.Mutex
	calls    []dotCall
	up       []platform.Mark
	cursorOK bool
}

const cursorX, cursorY = 1234, 567

func (r *rig) withDot() *fakeDot {
	d := &fakeDot{t: r.t, clock: r.clock, cursorOK: true}
	r.env.Dot = app.Dot{Show: d.show, Hide: d.hide, Cursor: d.cursor}
	return d
}

func (d *fakeDot) show(marks []platform.Mark) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(marks) == 0 {
		d.t.Error("showed no dots - that's a hide")
	}
	d.up = slices.Clone(marks)
	d.calls = append(d.calls, dotCall{marks: d.up, at: d.clock.Now()})
	return nil
}

func (d *fakeDot) hide() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.up == nil {
		d.t.Error("hid dots that weren't up")
	}
	d.up = nil
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
	return d.up != nil
}

func (d *fakeDot) marksUp() []platform.Mark {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.up)
}

func (d *fakeDot) labelsUp() []string {
	var labels []string
	for _, m := range d.marksUp() {
		labels = append(labels, m.Label)
	}
	return labels
}

func (d *fakeDot) shows() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for i, c := range d.calls {
		// a redraw with fewer dots doesn't count as going up again
		if c.marks != nil && (i == 0 || d.calls[i-1].marks == nil) {
			n++
		}
	}
	return n
}

// upFor is how long the last dots stayed up on the fake clock - redraws with fewer dots included
func (d *fakeDot) upFor() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.calls)
	if n < 2 || d.calls[n-1].marks != nil {
		d.t.Fatalf("no dots that went up and came down in %v", d.calls)
	}
	start := n - 2
	for start > 0 && d.calls[start-1].marks != nil {
		start--
	}
	return d.calls[n-1].at.Sub(d.calls[start].at)
}

func (d *fakeDot) wantUpAt(x, y int) {
	d.t.Helper()
	eventually(d.t, fmt.Sprintf("a dot at %d, %d", x, y), func() bool {
		return slices.ContainsFunc(d.marksUp(), func(m platform.Mark) bool { return m.X == x && m.Y == y })
	})
}

func (d *fakeDot) wantLabels(labels ...string) {
	d.t.Helper()
	eventually(d.t, fmt.Sprintf("dots for %v", labels), func() bool { return slices.Equal(d.labelsUp(), labels) })
}

func (d *fakeDot) wantDown(why string) {
	d.t.Helper()
	eventually(d.t, "the dots to go "+why, func() bool { return !d.isUp() })
}

func (d *fakeDot) wantNone() {
	d.t.Helper()
	settle()
	if n := d.shows(); n != 0 {
		d.t.Errorf("the dots went up %d times, want never", n)
	}
}

// dotLobby starts lowkey in the menus with the given settings and dots it can draw
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

// openSlots goes from the main list to the agent dots list - it's the last row so up wraps to it - it
// returns the main list on that row since the top of the list doesn't leave room for the dot rows
func (r *rig) openSlots() string {
	r.t.Helper()
	r.press(upKey)
	frame := r.screen("> agent dots", "(enter)")
	r.press(enterKey)
	r.screen("agent dots - up to 3", "> 1  ")
	return frame
}

// countdown steps the place countdown down to zero on the fake clock
func (r *rig) countdown() {
	r.t.Helper()
	r.screen("point at your agent - 5")
	for left := 4; left >= 1; left-- {
		r.clock.Advance(time.Second)
		r.screen(fmt.Sprintf("point at your agent - %d", left))
	}
	r.clock.Advance(time.Second)
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
	d.wantDown("at their time limit")
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
	d.wantDown("at their time limit")
	if got := d.upFor(); got != 15*time.Second {
		t.Errorf("the dot stayed up %v, want 15s", got)
	}
}

func TestUpToThreeDotsShowTogetherNamedInTheirAgentsColors(t *testing.T) {
	_, d := dotInAgentSelect(t, dotsOn(`{agent = "jett", at = [700, 450]}, {agent = "Fade", at = [760, 450]}, {agent = "Raze", at = [820, 450]}`))
	d.wantLabels("Jett", "Fade", "Raze")
	want := []platform.Mark{
		// two colors - the second rings the dot
		{X: 700, Y: 450, Label: "Jett", Fill: 0x00E5FF, Ring: 0xF4F7FA},
		// too dark for agent select's background without a light ring
		{X: 760, Y: 450, Label: "Fade", Fill: 0x3A3F44, Ring: 0xF2F2F2},
		{X: 820, Y: 450, Label: "Raze", Fill: 0xFF8A00, Ring: 0x101010},
	}
	if got := d.marksUp(); !slices.Equal(got, want) {
		t.Errorf("dots:\n got %+v\nwant %+v", got, want)
	}
}

func TestYourPickTakesEveryDotDown(t *testing.T) {
	for _, locked := range []bool{false, true} {
		t.Run(map[bool]string{false: "hover", true: "lock"}[locked], func(t *testing.T) {
			r, d := dotInAgentSelect(t, dotsOn(`{agent = "Raze", at = [800, 450]}, {agent = "Fade", at = [860, 450]}`))
			d.wantLabels("Raze", "Fade")
			r.fr.Hover(r.self(), fakeriot.Sova, locked)
			settle()
			if !d.isUp() {
				t.Fatal("the dots went before lowkey saw your pick")
			}
			r.clock.Advance(3 * time.Second)
			d.wantDown("after your pick")
			if got := d.upFor(); got > 3*time.Second {
				t.Errorf("the dots stayed up %v after the pick, want at most the 3 s roster refresh", got)
			}
		})
	}
}

func TestATeammatesLockTakesDownOnlyThatAgentsDot(t *testing.T) {
	// ally one only hovers Jett - a hover can still change so it leaves Jett's dot up
	r, d := dotInAgentSelect(t, dotsOn(`{agent = "Killjoy", at = [700, 450]}, {agent = "Jett", at = [760, 450]}, {agent = "Raze", at = [820, 450]}`))
	d.wantLabels("Killjoy", "Jett", "Raze")
	r.fr.Hover(ally2, fakeriot.Killjoy, true)
	settle()
	r.clock.Advance(3 * time.Second)
	d.wantLabels("Jett", "Raze")
	r.fr.Hover(ally1, fakeriot.Jett, true)
	settle()
	r.clock.Advance(3 * time.Second)
	d.wantLabels("Raze")
	if n := d.shows(); n != 1 {
		t.Errorf("the dots went up %d times - a teammate's lock should only redraw the rest", n)
	}
	r.fr.Hover(r.self(), fakeriot.Sova, true)
	settle()
	r.clock.Advance(3 * time.Second)
	d.wantDown("after your pick")
}

func TestADotForAnAgentATeammateAlreadyLockedGoesWithTheFirstRoster(t *testing.T) {
	// the streamer-mode ally and the unranked one locked Omen and Sage before agent select reached lowkey
	_, d := dotInAgentSelect(t, dotsOn(`{agent = "Sage", at = [700, 450]}, {agent = "Raze", at = [760, 450]}`))
	d.wantLabels("Raze")
}

// the stats lookups fill both request slots right after a match is found - your pick's roster fetch
// has to jump them or the dots would stay up through the whole burst
func TestAPickDuringTheStatsLookupsStillTakesTheDotsDown(t *testing.T) {
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

func TestADodgeTakesTheDotsDownAndTheNextMatchBringsThemBack(t *testing.T) {
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
	if n := d.shows(); n != 2 {
		t.Errorf("the dot went up %d times over two agent selects", n)
	}
}

func TestTheDotsAreNeverUpOnceTheMatchLoads(t *testing.T) {
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
	if n := d.shows(); n != 1 {
		t.Errorf("the dot went up %d times in one match", n)
	}
	if d.isUp() {
		t.Error("the dot is up after the match")
	}
}

func TestNoDotsUnlessTheyreOnAndPlaced(t *testing.T) {
	for name, file := range map[string]string{
		"off":        "[agent_select]\nagent_dot = false\nagent_dots = [{agent = \"Raze\", at = [800, 450]}]\n",
		"none set":   "[agent_select]\nagent_dot = true\n",
		"not placed": dotsOn(`{agent = "Raze"}`),
		"bad spot":   dotsOn(`{agent = "Raze", at = [800]}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, d := dotInAgentSelect(t, file)
			d.wantNone()
		})
	}
}

func TestNoDotsInModesLowkeyDoesntTrack(t *testing.T) {
	cases := map[string]struct {
		file, queue, status string
	}{
		"switched off in modes": {dotOn + "[table]\n" + `modes = ["unrated"]` + "\n", "competitive", "not tracking competitive"},
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

func TestNoDotsWhenLowkeyOpensMidAgentSelect(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) { w.Loop = fakeriot.LoopPregame })
	d := r.withDot()
	r.writeSettings(dotOn)
	r.live()
	r.settled("AGENT SELECT", "Dia1 50")
	d.wantNone()
}

func TestTurningTheDotsOffTakesThemDown(t *testing.T) {
	r, d := dotInAgentSelect(t, dotOn)
	d.wantUpAt(800, 450)
	r.writeSettings("[agent_select]\nagent_dot = false\nagent_dots = [{agent = \"Raze\", at = [800, 450]}]\n")
	d.wantDown("when they're turned off")
}

func TestTheDotsGoWhenLowkeyCloses(t *testing.T) {
	r, d := dotInAgentSelect(t, dotOn)
	d.wantUpAt(800, 450)
	r.stop()
	if d.isUp() {
		t.Error("the dot outlived lowkey's window")
	}
}

func TestTheDotsMakeNoRequests(t *testing.T) {
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
	on, off := requests(dotsOn(`{agent = "Raze", at = [800, 450]}, {agent = "Jett", at = [860, 450]}`)), requests("[agent_select]\nagent_dot = false\n")
	if strings.Join(on, "\n") != strings.Join(off, "\n") {
		t.Errorf("agent select with dots made %d requests, without them %d:\n%v\n%v", len(on), len(off), on, off)
	}
}

func TestTheAgentDotsListPicksAgentsWithLeftAndRight(t *testing.T) {
	r := newRig(t)
	r.withDot()
	r.settingsTab()
	r.openSlots()
	r.screen("> 1  none")
	r.press(enterKey)
	r.screen("pick an agent with left/right first")
	r.press(rightKey)
	r.saved(`agent_dots = [{agent = "Astra", at = []}]`)
	r.screen("> 1  Astra", "not placed", "  2  none")
	r.press(rightKey)
	r.saved(`agent_dots = [{agent = "Breach", at = []}]`)
	r.press(leftKey, leftKey)
	r.saved(`agent_dots = []`)
	r.screen("> 1  none")

	r.press(rightKey, downKey, leftKey, downKey, rightKey, rightKey)
	r.saved(`agent_dots = [{agent = "Astra", at = []}, {agent = "Yoru", at = []}, {agent = "Breach", at = []}]`)
	frame := r.screen("> 3  Breach")
	if strings.Contains(frame, "4  ") {
		t.Errorf("a fourth slot with three agents set:\n%s", frame)
	}
	r.press(downKey)
	r.screen("> 1  Astra")
	r.press(escKey)
	r.screen("> agent dots", "Astra (not placed), Yoru (not placed), Breach (not placed)")
}

func TestPlacingASlotSavesWhereTheCursorRestsAndTurnsTheDotsOn(t *testing.T) {
	r := newRig(t)
	d := r.withDot()
	r.settingsTab()
	if frame := r.openSlots(); !regexp.MustCompile(`agent dot +off`).MatchString(frame) {
		t.Fatalf("want the dots off before placing:\n%s", frame)
	}
	r.press(rightKey)
	r.screen("> 1  Astra", "rest the cursor on that agent")
	before := len(r.fr.RemoteRequests())
	r.press(enterKey)
	r.countdown()
	r.saved(fmt.Sprintf(`agent_dots = [{agent = "Astra", at = [%d, %d]}]`, cursorX, cursorY))
	r.saved("agent_dot = true")
	r.screen(fmt.Sprintf("> 1  Astra       %d, %d", cursorX, cursorY))
	d.wantLabels("Astra")
	d.wantUpAt(cursorX, cursorY)
	r.clock.Advance(3 * time.Second)
	d.wantDown("after their 3 s check")
	r.press(escKey)
	eventually(t, "the agent dot row to say on", func() bool { return regexp.MustCompile(`agent dot +on`).MatchString(r.con.last()) })
	if after := len(r.fr.RemoteRequests()); after != before {
		t.Errorf("placing made %d requests", after-before)
	}
}

func TestPlacingTheSecondSlotKeepsTheFirst(t *testing.T) {
	r := newRig(t)
	r.withDot()
	r.writeSettings(dotOn)
	r.settingsTab()
	r.openSlots()
	r.screen("> 1  Raze        800, 450", "  2  none")
	r.press(downKey, rightKey, enterKey)
	r.countdown()
	r.saved(fmt.Sprintf(`agent_dots = [{agent = "Raze", at = [800, 450]}, {agent = "Astra", at = [%d, %d]}]`, cursorX, cursorY))
}

func TestPlacingSavesTheDotsInPlaceUnderAgentSelect(t *testing.T) {
	const kept = "[general]\n# my own note\ncutoff_seconds = 45\n"
	for name, file := range map[string]string{
		"with the section":    kept + "\n[agent_select]\nagent_dots = []\n",
		"without the section": kept,
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			r.withDot()
			r.writeSettings(file)
			r.settingsTab()
			r.openSlots()
			r.press(rightKey, enterKey)
			r.countdown()
			placed := fmt.Sprintf(`agent_dots = [{agent = "Astra", at = [%d, %d]}]`, cursorX, cursorY)
			r.saved(placed)
			got := r.settingsFile()
			if !strings.HasPrefix(got, kept) {
				t.Errorf("the rest of the file should stay as it was:\n%s", got)
			}
			for _, line := range []string{placed, "agent_dot = true"} {
				if s := sectionOf(got, line); s != "agent_select" {
					t.Errorf("want %q under [agent_select], got [%s] in:\n%s", line, s, got)
				}
			}
		})
	}
}

func sectionOf(file, line string) string {
	section := ""
	for _, l := range strings.Split(file, "\n") {
		if strings.HasPrefix(l, "[") {
			section = strings.Trim(l, "[]")
		}
		if l == line {
			return section
		}
	}
	return ""
}

func TestChangingASlotsAgentForgetsItsSpot(t *testing.T) {
	r := newRig(t)
	r.withDot()
	r.writeSettings(dotOn)
	r.settingsTab()
	r.openSlots()
	r.press(rightKey)
	r.saved(`agent_dots = [{agent = "Reyna", at = []}]`)
}

func TestPlacingKeepsCountingWhenANewLobbyTakesTheScreen(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	d := r.withDot()
	r.settingsTab()
	r.subscribed()
	r.openSlots()
	r.press(rightKey, enterKey)
	r.screen("point at your agent - 5")
	r.enterAgentSelect()
	r.settled("[LOBBY]", "AGENT SELECT", "Dia1 50")
	for range 5 {
		r.clock.Advance(time.Second)
		settle()
	}
	r.saved(fmt.Sprintf(`agent_dots = [{agent = "Astra", at = [%d, %d]}]`, cursorX, cursorY))
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
	r.press(tabKey)
	r.screen("[SETTINGS]")
	r.openSlots()
	r.press(rightKey, enterKey)
	r.countdown()
	r.saved(fmt.Sprintf(`agent_dots = [{agent = "Astra", at = [%d, %d]}]`, cursorX, cursorY))
	d.wantNone()
}

func TestAnUnreadableCursorSaysSoAndSavesNothing(t *testing.T) {
	r := newRig(t)
	d := r.withDot()
	d.cursorOK = false
	r.settingsTab()
	r.openSlots()
	r.press(rightKey, enterKey)
	r.countdown()
	r.screen("can't read where the cursor is", "> 1  Astra       not placed")
	if file := r.settingsFile(); !strings.Contains(file, `agent_dots = [{agent = "Astra", at = []}]`) {
		t.Errorf("an unread cursor changed the spot:\n%s", file)
	}
	d.wantNone()
}

func TestTheDotRowsChangeTheFile(t *testing.T) {
	r := newRig(t)
	r.withDot()
	r.settingsTab()
	r.onRow(agentDotRow)
	frame := r.screen("> agent dot", "needs windowed or windowed fullscreen")
	if !strings.Contains(frame, "\nAGENT SELECT\n> agent dot ") {
		t.Errorf("want the dot rows under an AGENT SELECT heading:\n%s", frame)
	}
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
	r.press(downKey)
	r.screen("> agent dots", "none  (enter)", "up to 3 agents whose dots show together")
}

func TestWithoutADotThereAreNoDotRows(t *testing.T) {
	r := newRig(t)
	r.settingsTab()
	r.press(upKey)
	frame := r.screen("> region")
	for _, row := range []string{"AGENT SELECT", "agent dot", "dot time", "agent dots"} {
		if strings.Contains(frame, row) {
			t.Errorf("a build that can't draw dots shows %q:\n%s", row, frame)
		}
	}
}

func TestARecordingSaysWhenEachDotWentUpAndDown(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	d := r.withDot()
	r.writeSettings(dotsOn(`{agent = "Killjoy", at = [700, 450]}, {agent = "Raze", at = [800, 450]}`) + "agent_dot_seconds = 6\n")
	r.live("--record")
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	d.wantLabels("Killjoy", "Raze")
	r.fr.Hover(ally2, fakeriot.Killjoy, true)
	settle()
	r.clock.Advance(3 * time.Second)
	d.wantLabels("Raze")
	r.clock.Advance(3 * time.Second)
	d.wantDown("at their time limit")
	r.stop()

	entries, _ := readRecording(t, recordingDir(t, r.dir))
	type dotMark struct {
		Agent string `json:"agent"`
		X     int    `json:"x"`
		Y     int    `json:"y"`
	}
	type dotEntry struct {
		Shown bool      `json:"shown"`
		Why   string    `json:"why"`
		Dots  []dotMark `json:"dots"`
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
	want := []dotEntry{
		{true, "match found", []dotMark{{"Killjoy", 700, 450}, {"Raze", 800, 450}}},
		{false, "taken by a teammate", []dotMark{{"Killjoy", 700, 450}}},
		{false, "time limit", []dotMark{{"Raze", 800, 450}}},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("dot entries:\n got %v\nwant %v", got, want)
	}
}
