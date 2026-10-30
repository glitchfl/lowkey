package app_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/app"
	"github.com/glitchfl/lowkey/internal/fakeriot"
	"github.com/glitchfl/lowkey/internal/static"
)

// what the console sends for each key - each string is one read
const (
	tabKey   = "\t"
	upKey    = "\x1b[A"
	downKey  = "\x1b[B"
	rightKey = "\x1b[C"
	leftKey  = "\x1b[D"
	spaceKey = " "
	enterKey = "\r"
	escKey   = "\x1b"
)

const (
	columnsRow = iota
	matchesRow
	modesRow
	sessionRow
	cutoffRow
	enemyRow
	hiddenStatsRow
	regionRow
	weaponRow
)

// keyboard plugs a keyboard in so the tabs show up - the frames stay plain so the usual helpers read them
func (r *rig) keyboard() {
	r.env.Console.Keyboard = func() (io.Reader, func(), bool) {
		pr, pw := io.Pipe()
		r.keys.Store(pw)
		return pr, func() {
			pw.Close()
			r.restored.Add(1)
		}, true
	}
}

func (r *rig) press(keys ...string) {
	r.t.Helper()
	for _, k := range keys {
		if _, err := r.keys.Load().Write([]byte(k)); err != nil {
			r.t.Fatal(err)
		}
	}
}

// onRow moves the highlight down from the top of the main list
func (r *rig) onRow(row int) {
	r.t.Helper()
	for range row {
		r.press(downKey)
	}
}

func (r *rig) liveWithKeyboard() {
	r.t.Helper()
	r.keyboard()
	r.live()
	r.screen("[LOBBY]", "waiting for the next match")
}

func (r *rig) settingsTab() string {
	r.t.Helper()
	r.liveWithKeyboard()
	r.press(tabKey)
	return r.screen("[SETTINGS]", "> columns")
}

func (r *rig) saved(want string) {
	r.t.Helper()
	eventually(r.t, "lowkey.toml to say "+want, func() bool { return strings.Contains(r.settingsFile(), want) })
}

func TestWithoutAKeyboardThereAreNoTabs(t *testing.T) {
	r := newRig(t)
	r.live()
	if frame := r.screen("waiting for the next match"); strings.Contains(frame, "SETTINGS") {
		t.Errorf("a window nobody can type in shouldn't offer a settings tab:\n%s", frame)
	}
}

func TestAConsoleThatRefusesTheKeyboardKeepsTheLobbyAlone(t *testing.T) {
	r := newRig(t)
	r.env.Console.Keyboard = func() (io.Reader, func(), bool) { return nil, nil, false }
	r.live()
	if frame := r.screen("waiting for the next match"); strings.Contains(frame, "SETTINGS") {
		t.Errorf("no keyboard means no tab bar:\n%s", frame)
	}
}

func TestTabFlipsBetweenTheLobbyAndTheSettings(t *testing.T) {
	r := newRig(t, agentSelectLobby, lobbySamples)
	r.keyboard()
	r.live()
	r.settled("[LOBBY]", "waiting for the next match", "SESSION")
	r.subscribed()
	r.enterAgentSelect()
	golden(t, "tab-bar-lobby", r.settled("[LOBBY]", "AGENT SELECT", "AllyOne#A1", "AllyTwo#A2"))

	r.press(tabKey)
	frame := r.settled("[SETTINGS]")
	golden(t, "settings-tab", frame)
	if strings.Contains(frame, "AGENT SELECT") {
		t.Errorf("the settings tab should replace the table:\n%s", frame)
	}

	r.press(tabKey)
	r.screen("[LOBBY]", "AGENT SELECT", "AllyOne#A1")
	r.press("\x1b[Z")
	r.screen("[SETTINGS]")
	r.press(escKey)
	r.screen("[LOBBY]", "AGENT SELECT", "AllyOne#A1")
}

func TestKeysOnTheLobbyDoNothing(t *testing.T) {
	r := newRig(t)
	r.liveWithKeyboard()
	before := r.settingsFile()
	r.press(upKey, downKey, leftKey, rightKey, spaceKey, enterKey, escKey, "x")
	settle()
	r.screen("[LOBBY]", "waiting for the next match")
	if got := r.settingsFile(); got != before {
		t.Errorf("a key on the lobby changed the file:\n%s", got)
	}
}

func TestEveryRowChangesTheFileAtOnce(t *testing.T) {
	cases := []struct {
		name  string
		row   int
		keys  []string
		frame string
		file  string
	}{
		{"matches up", matchesRow, []string{rightKey}, "< 14 >", "matches = 14\n"},
		{"matches down", matchesRow, []string{leftKey}, "< 12 >", "matches = 12\n"},
		{"session up", sessionRow, []string{rightKey}, "< 07:00 >", "session_reset_hour = 7\n"},
		{"cutoff up", cutoffRow, []string{rightKey}, "< 45s >", "cutoff_seconds = 45\n"},
		{"cutoff down", cutoffRow, []string{leftKey}, "< 35s >", "cutoff_seconds = 35\n"},
		{"enemies by arrow", enemyRow, []string{rightKey}, "< off >", "enemy_lookups = false\n"},
		{"enemies by space", enemyRow, []string{spaceKey}, "< off >", "enemy_lookups = false\n"},
		{"enemies by enter", enemyRow, []string{enterKey}, "< off >", "enemy_lookups = false\n"},
		{"hidden stats by arrow", hiddenStatsRow, []string{rightKey}, "< off >", "hidden_player_stats = false\n"},
		{"hidden stats by space", hiddenStatsRow, []string{spaceKey}, "< off >", "hidden_player_stats = false\n"},
		{"hidden stats by enter", hiddenStatsRow, []string{enterKey}, "< off >", "hidden_player_stats = false\n"},
		{"region forward", regionRow, []string{rightKey}, "< ap >", `region = "ap"` + "\n"},
		{"region backward", regionRow, []string{leftKey}, "< pbe >", `region = "pbe"` + "\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			r.settingsTab()
			r.onRow(c.row)
			r.press(c.keys...)
			r.screen(c.frame)
			if file := r.settingsFile(); !strings.Contains(file, c.file) {
				t.Errorf("the file should hold %q as soon as the screen shows it:\n%s", c.file, file)
			}
		})
	}
}

func TestStepsStopAtTheLimitsAndWrapWhereItMakesSense(t *testing.T) {
	cases := []struct {
		name, setting string
		row           int
		key           string
		frame, file   string
	}{
		{"most matches", "matches = 25\n", matchesRow, rightKey, "< 25 >", "matches = 25"},
		{"fewest matches", "matches = 1\n", matchesRow, leftKey, "< 1 >", "matches = 1"},
		{"hour past midnight", "session_reset_hour = 23\n", sessionRow, rightKey, "< 00:00 >", "session_reset_hour = 0"},
		{"hour before midnight", "session_reset_hour = 0\n", sessionRow, leftKey, "< 23:00 >", "session_reset_hour = 23"},
		{"longest cutoff", "cutoff_seconds = 600\n", cutoffRow, rightKey, "< 600s >", "cutoff_seconds = 600"},
		{"shortest cutoff", "cutoff_seconds = 1\n", cutoffRow, leftKey, "< 1s >", "cutoff_seconds = 1"},
		{"cutoff off the shortest", "cutoff_seconds = 1\n", cutoffRow, rightKey, "< 5s >", "cutoff_seconds = 5"},
		{"cutoff down to the shortest", "cutoff_seconds = 5\n", cutoffRow, leftKey, "< 1s >", "cutoff_seconds = 1"},
		{"hand-set cutoff up", "cutoff_seconds = 37\n", cutoffRow, rightKey, "< 40s >", "cutoff_seconds = 40"},
		{"hand-set cutoff down", "cutoff_seconds = 37\n", cutoffRow, leftKey, "< 35s >", "cutoff_seconds = 35"},
		{"region back to auto", `region = "ap"` + "\n", regionRow, leftKey, "< auto >", `region = "auto"`},
		{"region around the end", `region = "pbe"` + "\n", regionRow, rightKey, "< auto >", `region = "auto"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			r.writeSettings(c.setting)
			r.settingsTab()
			r.onRow(c.row)
			r.press(c.key)
			r.screen(c.frame)
			r.saved(c.file)
		})
	}
}

func TestAChangeOnlyTouchesItsOwnValue(t *testing.T) {
	r := newRig(t)
	r.writeSettings("# my own note\nweapon = \"phantom\"   # the good one\n\nmatches=7\n\n# more notes\ncolumns = [\n  \"name\", # who\n  \"rank\",\n]\n")
	r.settingsTab()
	r.onRow(matchesRow)
	r.press(rightKey)
	r.screen("< 8 >")
	want := "# my own note\nweapon = \"phantom\"   # the good one\n\nmatches=8\n\n# more notes\ncolumns = [\n  \"name\", # who\n  \"rank\",\n]\n"
	if got := r.settingsFile(); got != want {
		t.Errorf("the rest of the file should stay as it was\n--- want\n%s--- got\n%s", want, got)
	}
}

func TestChangingTheFirstRunFileKeepsItsComments(t *testing.T) {
	r := newRig(t)
	r.settingsTab()
	before := r.settingsFile()
	r.onRow(matchesRow)
	r.press(rightKey)
	r.screen("< 14 >")
	if got, want := r.settingsFile(), strings.Replace(before, "matches = 13", "matches = 14", 1); got != want {
		t.Errorf("only the value should change\n--- want\n%s--- got\n%s", want, got)
	}
}

// downTo moves from vandal at the top of the guns list to another gun - the guns that are on come first
func downTo(gun string) []string {
	off := slices.DeleteFunc(slices.Clone(static.Weapons()), func(w string) bool { return w == "vandal" })
	return slices.Repeat([]string{downKey}, slices.Index(off, gun)+1)
}

func TestAKeyTheFileDoesNotHaveIsAddedWithItsComment(t *testing.T) {
	r := newRig(t)
	r.writeSettings("matches = 7")
	r.settingsTab()
	r.onRow(weaponRow)
	r.press(enterKey)
	r.press(downTo("phantom")...)
	r.press(spaceKey)
	r.screen("> [x] phantom")
	want := "matches = 7\n\n# the gun the skin column shows - vandal phantom operator sheriff melee or any other by its name\n" +
		"# a list like [\"vandal\", \"phantom\"] gives each gun a skin column\nweapon = [\"vandal\", \"phantom\"]\n"
	if got := r.settingsFile(); got != want {
		t.Errorf("--- want\n%s--- got\n%s", want, got)
	}
}

func TestAHandEditShowsOnTheTabAndAChangeKeepsIt(t *testing.T) {
	r := newRig(t)
	r.settingsTab()
	r.writeSettings("matches = 9\nsession_reset_hour = 18\n")
	r.screen("  matches              9", "  session starts at    18:00")
	r.onRow(matchesRow)
	r.press(rightKey)
	r.screen("< 10 >")
	if got, want := r.settingsFile(), "matches = 10\nsession_reset_hour = 18\n"; got != want {
		t.Errorf("--- want\n%s--- got\n%s", want, got)
	}
}

func TestTheHelpLineSaysWhatARowDoesAndWhenItApplies(t *testing.T) {
	r := newRig(t)
	r.settingsTab()
	for row, c := range []struct {
		help      string
		nextLobby bool
	}{
		{"columns left to right", false},
		{"recent matches per player", true},
		{"modes to track", true},
		{"when the session line starts over", true},
		{"when every lookup stops", true},
		{"look up the enemy team", true},
		{"show the stats of hidden players", true},
		{"auto reads the region", true},
		{"the gun the skin column shows", false},
	} {
		if row > 0 {
			r.press(downKey)
		}
		frame := r.screen(c.help)
		if got := strings.Contains(frame, "applies from the next lobby"); got != c.nextLobby {
			t.Errorf("row %d: says it waits for the next lobby: %v, want %v\n%s", row, got, c.nextLobby, frame)
		}
	}
}

func TestTheHighlightWrapsAroundTheList(t *testing.T) {
	r := newRig(t)
	r.settingsTab()
	r.press(upKey)
	r.screen("> skin weapon")
	r.press(downKey)
	r.screen("> columns")
}

func TestColumnsSubList(t *testing.T) {
	t.Run("shows them in table order", func(t *testing.T) {
		r := newRig(t)
		r.writeSettings(`columns = ["rank", "name"]` + "\n")
		r.settingsTab()
		r.screen("2 of 18  (enter)")
		r.press(enterKey)
		golden(t, "settings-columns", r.settled("columns - top is leftmost"))
	})
	t.Run("switching one off and on again puts it back", func(t *testing.T) {
		r := newRig(t)
		r.writeSettings(`columns = ["name", "agent", "rank"]` + "\n")
		r.settingsTab()
		r.press(enterKey)
		r.screen("> [x] name")
		r.press(downKey, spaceKey)
		r.screen("> [ ] agent")
		r.saved(`columns = ["name", "rank"]`)
		r.press(spaceKey)
		r.screen("> [x] agent")
		r.saved(`columns = ["name", "agent", "rank"]`)
	})
	t.Run("a column that's off comes on where its row is", func(t *testing.T) {
		r := newRig(t)
		r.writeSettings(`columns = ["name", "rank"]` + "\n")
		r.settingsTab()
		r.press(enterKey, downKey, downKey, spaceKey)
		r.screen("> [x] party")
		r.saved(`columns = ["name", "rank", "party"]`)
		r.press(leftKey)
		r.saved(`columns = ["name", "party", "rank"]`)
	})
	t.Run("moving swaps it with its neighbor", func(t *testing.T) {
		r := newRig(t)
		r.writeSettings(`columns = ["name", "agent", "rank"]` + "\n")
		r.settingsTab()
		r.press(enterKey, rightKey)
		r.screen("  [x] agent\n> [x] name")
		r.saved(`columns = ["agent", "name", "rank"]`)
		r.press(leftKey)
		r.screen("> [x] name\n  [x] agent")
		r.saved(`columns = ["name", "agent", "rank"]`)
	})
	t.Run("a move stops at both ends", func(t *testing.T) {
		r := newRig(t)
		r.writeSettings(`columns = ["name", "agent"]` + "\n")
		r.settingsTab()
		r.press(enterKey, leftKey)
		settle()
		r.screen("> [x] name\n  [x] agent")
		r.saved(`columns = ["name", "agent"]`)
		r.press(upKey, rightKey)
		settle()
		frame := r.screen("columns - top is leftmost")
		if !strings.HasSuffix(strings.Split(frame, "\n")[20], "> [ ] skin") {
			t.Errorf("the last row should stay last:\n%s", frame)
		}
	})
	t.Run("the last one has to stay on", func(t *testing.T) {
		r := newRig(t)
		r.writeSettings(`columns = ["name"]` + "\n")
		r.settingsTab()
		r.press(enterKey, spaceKey)
		r.screen("at least one column has to stay on", "> [x] name")
		r.saved(`columns = ["name"]`)
		r.press(downKey)
		if frame := r.screen("> [ ] party"); strings.Contains(frame, "at least one") {
			t.Errorf("the next key should clear the notice:\n%s", frame)
		}
		r.press(upKey, spaceKey)
		r.screen("at least one column has to stay on")
		r.press(tabKey)
		r.screen("[LOBBY]")
		r.press(tabKey)
		if frame := r.screen("[SETTINGS]"); strings.Contains(frame, "at least one") {
			t.Errorf("flipping tabs should clear the notice too:\n%s", frame)
		}
	})
	t.Run("esc and enter go back to the main list", func(t *testing.T) {
		r := newRig(t)
		r.settingsTab()
		r.press(enterKey)
		r.screen("columns - top is leftmost")
		r.press(escKey)
		r.screen("[SETTINGS]", "> columns")
		r.press(enterKey)
		r.screen("columns - top is leftmost")
		r.press(enterKey)
		r.screen("[SETTINGS]", "> columns")
	})
	t.Run("tab leaves it open for coming back", func(t *testing.T) {
		r := newRig(t)
		r.settingsTab()
		r.press(enterKey, downKey, downKey)
		r.screen("> [x] agent")
		r.press(tabKey)
		r.screen("[LOBBY]")
		r.press(tabKey)
		r.screen("[SETTINGS]", "> [x] agent")
	})
	t.Run("follows a hand edit while it's open", func(t *testing.T) {
		r := newRig(t)
		r.settingsTab()
		r.press(enterKey)
		r.screen("> [x] party")
		r.writeSettings(`columns = ["rank", "name"]` + "\n")
		frame := r.screen("[x] rank\n", "[x] name\n", "[ ] agent\n")
		if strings.Contains(frame, "[x] party") {
			t.Errorf("party isn't on any more:\n%s", frame)
		}
		r.press(rightKey)
		r.saved("columns = ")
	})
	t.Run("changes the table on screen", func(t *testing.T) {
		r := atAgentSelectWithKeyboard(t)
		r.press(tabKey)
		r.screen("[SETTINGS]")
		r.press(enterKey, downKey, downKey, downKey, spaceKey)
		r.screen("> [ ] rank")
		r.press(tabKey)
		frame := r.screen("[LOBBY]", "AGENT SELECT", "PEAK")
		if strings.Contains(frame, "RANK") {
			t.Errorf("the rank column is off:\n%s", frame)
		}
	})
}

func atAgentSelectWithKeyboard(t *testing.T, setup ...func(w *fakeriot.World)) *rig {
	r := newRig(t, append([]func(w *fakeriot.World){matchLobby}, setup...)...)
	r.liveWithKeyboard()
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50", "Plat1 30", "SESSION")
	return r
}

func TestModesSubList(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.settingsTab()
	r.onRow(modesRow)
	r.screen("6 of 6  (enter)")
	r.press(enterKey)
	golden(t, "settings-modes", r.settled("modes to track - applies from the next lobby"))
	r.press(spaceKey)
	r.screen("> [ ] competitive")
	r.saved(`modes = ["premier", "unrated", "swiftplay", "deathmatch", "team_deathmatch"]`)
	r.press(rightKey, leftKey)
	settle()
	r.screen("> [ ] competitive\n  [x] premier")

	r.press(escKey)
	r.screen("5 of 6  (enter)")
	r.subscribed()
	r.enterAgentSelect()
	r.screen("[LOBBY]", "not tracking competitive")
}

func TestWeaponsSubList(t *testing.T) {
	t.Run("adds a gun and moves it", func(t *testing.T) {
		r := newRig(t)
		r.settingsTab()
		r.onRow(weaponRow)
		r.screen("> skin weapons         vandal  (enter)")
		r.press(enterKey)
		r.screen("skin weapons - top is leftmost in the table", "> [x] vandal")
		r.press(downTo("phantom")...)
		r.press(spaceKey)
		r.screen("> [x] phantom")
		r.saved(`weapon = ["vandal", "phantom"]` + "\n")
		for range downTo("phantom") {
			r.press(leftKey)
		}
		r.screen("> [x] phantom\n  [x] vandal")
		r.saved(`weapon = ["phantom", "vandal"]` + "\n")
		r.press(escKey)
		r.screen("> skin weapons         phantom, vandal  (enter)")
	})
	t.Run("a single gun is saved as a name", func(t *testing.T) {
		r := newRig(t)
		r.writeSettings(`weapon = ["vandal", "phantom"]` + "\n")
		r.settingsTab()
		r.onRow(weaponRow)
		r.press(enterKey, spaceKey)
		r.screen("> [ ] vandal")
		r.saved(`weapon = "phantom"` + "\n")
	})
	t.Run("the last one has to stay on", func(t *testing.T) {
		r := newRig(t)
		r.settingsTab()
		r.onRow(weaponRow)
		r.press(enterKey, spaceKey)
		r.screen("at least one gun has to stay on - turn the skin column off under columns", "> [x] vandal")
		r.saved(`weapon = "vandal"` + "\n")
	})
}

func TestABrokenFileIsKeptAsABackupAndReplaced(t *testing.T) {
	r := newRig(t)
	r.writeSettings(`matches = 7` + "\n")
	r.settingsTab()
	r.screen("  matches              7")
	broken := "matches = 7\ncolumns = [\"name\",\n"
	r.writeSettings(broken)
	r.screen("using the last good settings", "  matches              7")

	r.onRow(matchesRow)
	r.press(rightKey)
	frame := r.screen("< 8 >", "lowkey.toml was broken - the old one is now lowkey.toml.bak")
	if strings.Contains(frame, "using the last good settings") {
		t.Errorf("the file is fine now - the warning should go:\n%s", frame)
	}
	backup, err := os.ReadFile(filepath.Join(r.dir, "lowkey.toml.bak"))
	if err != nil || string(backup) != broken {
		t.Errorf("the broken file should be kept word for word: %q %v", backup, err)
	}
	file := r.settingsFile()
	if !strings.Contains(file, "matches = 8\n") || !strings.Contains(file, "# recent matches per player") {
		t.Errorf("the new file should be whole and commented:\n%s", file)
	}
	r.press(downKey)
	if frame := r.screen("> modes"); strings.Contains(frame, "was broken") {
		t.Errorf("the next key should clear the notice:\n%s", frame)
	}
}

func TestADeletedFileIsWrittenAgainWithTheCurrentSettings(t *testing.T) {
	r := newRig(t)
	r.writeSettings(`weapon = "phantom"` + "\n")
	r.settingsTab()
	if err := os.Remove(filepath.Join(r.dir, "lowkey.toml")); err != nil {
		t.Fatal(err)
	}
	r.onRow(matchesRow)
	r.press(rightKey)
	frame := r.screen("< 14 >")
	if strings.Contains(frame, "broken") {
		t.Errorf("a missing file isn't a broken one:\n%s", frame)
	}
	file := r.settingsFile()
	if !strings.Contains(file, "matches = 14\n") || !strings.Contains(file, `weapon = "phantom"`) {
		t.Errorf("the new file should hold every setting in use:\n%s", file)
	}
	if _, err := os.Stat(filepath.Join(r.dir, "lowkey.toml.bak")); err == nil {
		t.Error("there was nothing to back up")
	}
}

func TestAFailedSaveSaysSoAndChangesNothing(t *testing.T) {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("needs a folder this user can't write to")
	}
	r := newRig(t)
	r.settingsTab()
	if err := os.Chmod(r.dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(r.dir, 0o755) })
	r.onRow(matchesRow)
	r.press(rightKey)
	frame := r.screen("can't save lowkey.toml", "< 13 >")
	if strings.Contains(frame, "< 14 >") {
		t.Errorf("the value shouldn't move when it can't be saved:\n%s", frame)
	}
	os.Chmod(r.dir, 0o755)
	r.press(rightKey)
	if frame := r.screen("< 14 >"); strings.Contains(frame, "can't save") {
		t.Errorf("the notice should go once saving works:\n%s", frame)
	}
}

func TestANewLobbyTakesTheScreenBack(t *testing.T) {
	r := newRig(t, matchLobby)
	r.settingsTab()
	r.onRow(cutoffRow)
	r.screen("> cutoff")
	r.subscribed()

	r.enterAgentSelect()
	r.screen("[LOBBY]", "AGENT SELECT")
	r.press(tabKey)
	r.screen("[SETTINGS]", "> cutoff")

	r.enterMatchLoad()
	r.screen("[LOBBY]", "MATCH LOADING")
	r.press(tabKey)
	r.screen("[SETTINGS]", "> cutoff")

	r.clock.Advance(40 * time.Second)
	settle()
	r.screen("[SETTINGS]", "> cutoff")
	r.press(tabKey)
	r.screen("[LOBBY]", "IN MATCH")
}

func TestOnlyANewLobbyMovesTheTab(t *testing.T) {
	r := newRig(t)
	r.settingsTab()
	r.subscribed()
	r.presence(func(w *fakeriot.World) { w.Valorant = false })
	settle()
	r.screen("[SETTINGS]", "> columns")
}

func TestKeysNeverMakeARiotRequest(t *testing.T) {
	everyRow := func(r *rig) {
		r.press(enterKey, downKey, spaceKey, rightKey, leftKey, escKey)
		for range weaponRow {
			r.press(downKey, rightKey, leftKey, rightKey)
		}
		r.press(enterKey, downKey, spaceKey, rightKey, escKey)
	}
	t.Run("in the menus", func(t *testing.T) {
		r := newRig(t, ranked)
		r.settingsTab()
		r.settled("[SETTINGS]")
		before := len(r.fr.RemoteRequests())
		everyRow(r)
		settle()
		if got := len(r.fr.RemoteRequests()); got != before {
			t.Errorf("keys sent %d requests", got-before)
		}
	})
	t.Run("mid-match", func(t *testing.T) {
		r := atAgentSelectWithKeyboard(t, lobbySkins)
		r.enterMatchLoad()
		r.settled("MATCH LOADING", "US - THEM")
		r.clock.Advance(40 * time.Second)
		r.settled("IN MATCH")
		r.press(tabKey)
		r.settled("[SETTINGS]")
		before := len(r.fr.RemoteRequests())
		everyRow(r)
		settle()
		if got := len(r.fr.RemoteRequests()); got != before {
			t.Errorf("keys sent %d requests mid-match", got-before)
		}
	})
}

func TestAddingAGunOnTheTabAddsItsSkinColumnWithoutARequest(t *testing.T) {
	r := atAgentSelectWithKeyboard(t, lobbySkins)
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "VANDAL", "Reaver")
	r.press(tabKey)
	r.screen("[SETTINGS]")
	r.onRow(weaponRow)
	r.press(enterKey)
	r.press(downTo("phantom")...)
	r.press(spaceKey)
	r.screen("> [x] phantom")
	r.press(tabKey)
	r.screen("[LOBBY]", "VANDAL", "Reaver", "PHANTOM", "Ion")
	if n := requestsExact(r, loadouts); n != 1 {
		t.Errorf("loadouts fetched %d times, want once", n)
	}
}

func TestMatchesFromTheTabApplyToTheNextLobby(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.settingsTab()
	r.onRow(matchesRow)
	for range 6 {
		r.press(leftKey)
	}
	r.screen("< 7 >")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("[LOBBY]", "AGENT SELECT", "Dia1 50")
	found := false
	for _, req := range r.fr.RemoteRequests() {
		if strings.HasPrefix(req.Path, "/match-history/") {
			found = true
			if req.Query.Get("endIndex") != "7" {
				t.Errorf("history asked for %s matches, want 7", req.Query.Get("endIndex"))
			}
		}
	}
	if !found {
		t.Error("no history was asked for")
	}
}

func TestSteppingTheRegionBackToAutoGoesBackToTheGamesRegion(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.writeSettings(`region = "ap"` + "\n")
	r.settingsTab()
	r.subscribed()
	r.settled("[SETTINGS]")
	r.onRow(regionRow)
	r.press(leftKey)
	r.screen("< auto >")
	before := len(r.fr.RemoteRequests())
	r.enterAgentSelect()
	r.settled("[LOBBY]", "AGENT SELECT", "Dia1 50")
	requests := r.fr.RemoteRequests()[before:]
	if len(requests) == 0 {
		t.Fatal("the lobby made no requests")
	}
	for _, req := range requests {
		if req.Service != "static" && req.Shard != "eu" {
			t.Errorf("%s went to shard %q, want the game's eu", req.Path, req.Shard)
		}
	}
}

func TestKeysAreReadFromEveryWayTheConsoleSendsThem(t *testing.T) {
	cases := []struct {
		name  string
		keys  []string
		shows string
	}{
		{"application mode arrows", []string{"\x1bOB"}, "> matches"},
		{"arrows with a modifier", []string{"\x1b[1;2B"}, "> matches"},
		{"several keys in one read", []string{downKey + downKey}, "> modes"},
		{"a key cut in half by a read", []string{"\x1b[", "B"}, "> matches"},
		{"an application mode key cut in half", []string{"\x1bO", "B"}, "> matches"},
		{"sequences it doesn't know", []string{"\x1b[3~\x1b[15~x", downKey}, "> matches"},
		{"a sequence that breaks off", []string{"\x1b[\x01", downKey}, "> matches"},
		{"letters from another alphabet", []string{"שלום", downKey}, "> matches"},
		{"an escape with a letter behind it", []string{"\x1bx"}, "[LOBBY]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			r.settingsTab()
			r.press(c.keys...)
			r.screen(c.shows)
		})
	}
}

func TestLongListsScrollInAShortWindow(t *testing.T) {
	lineCount := func(frame string) int { return len(strings.Split(plain(frame), "\n")) }
	t.Run("the main list", func(t *testing.T) {
		r := newRig(t)
		r.con.setHeight(12)
		frame := r.settingsTab()
		if !strings.Contains(frame, "> columns") || strings.Contains(frame, "skin weapon") {
			t.Errorf("the top of the list should show:\n%s", frame)
		}
		r.onRow(weaponRow)
		frame = r.screen("> skin weapon")
		if strings.Contains(frame, "columns ") {
			t.Errorf("the top should have scrolled off:\n%s", frame)
		}
		if !strings.HasPrefix(frame, " LOBBY  [SETTINGS]") {
			t.Errorf("the tab bar stays put:\n%s", frame)
		}
		if n := lineCount(frame); n > 11 {
			t.Errorf("%d lines don't fit a 12-row window with the newline after them:\n%s", n, frame)
		}
	})
	t.Run("the columns list", func(t *testing.T) {
		r := newRig(t)
		r.con.setHeight(12)
		r.settingsTab()
		r.press(enterKey, upKey)
		frame := r.screen("> [x] skin")
		if strings.Contains(frame, "[x] party") {
			t.Errorf("the top should have scrolled off:\n%s", frame)
		}
		if n := lineCount(frame); n > 11 {
			t.Errorf("%d lines don't fit a 12-row window with the newline after them:\n%s", n, frame)
		}
		r.press(downKey)
		r.screen("> [x] party")
	})
	t.Run("a window too short for anything still shows the highlight", func(t *testing.T) {
		r := newRig(t)
		r.con.setHeight(1)
		r.settingsTab()
		r.press(enterKey, upKey)
		r.screen("> [x] skin")
	})
}

func TestATerminalRedrawsTheTabsInPlace(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.env.Console.Terminal = true
	r.settingsTab()
	r.press(tabKey)
	r.screen("[LOBBY]")
	r.press(tabKey)
	r.press(downKey, rightKey)
	frame := r.screen("< 14 >")
	if !strings.HasPrefix(frame, "\x1b[H") || !strings.HasSuffix(frame, "\x1b[J") {
		t.Errorf("a redraw should start at the top and clear what's left below:\n%q", frame)
	}
	for _, m := range regexp.MustCompile(`\x1b\[([0-9;]*)([A-Za-z])`).FindAllStringSubmatch(frame, -1) {
		if m[1] != "" || !strings.Contains("HKJ", m[2]) {
			t.Errorf("unexpected escape code %q", m[0])
		}
	}
}

func TestClosingTheWindowPutsTheConsoleBack(t *testing.T) {
	r := newRig(t)
	r.liveWithKeyboard()
	r.stop()
	if n := r.restored.Load(); n != 1 {
		t.Errorf("the console was restored %d times, want once", n)
	}
}

func TestRecordingsKeepTheLobbyAndNeverTheTabs(t *testing.T) {
	r := newRig(t, matchLobby, lobbySamples)
	r.keyboard()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- app.Run(ctx, r.env, []string{"--record"}) }()
	stop := func() {
		if cancel != nil {
			cancel()
			<-done
			cancel = nil
		}
	}
	defer stop()
	r.screen("[LOBBY]", "waiting for the next match")
	r.subscribed()
	r.press(tabKey)
	r.screen("[SETTINGS]")
	r.settled("[SETTINGS]")
	// the world moves on while the settings are open
	r.presence(func(w *fakeriot.World) { w.Valorant = false })
	eventually(t, "the recording to see it", func() bool {
		raw, err := os.ReadFile(filepath.Join(recordingDir(t, r.dir), "screens.txt"))
		return err == nil && strings.Contains(string(raw), "waiting for Valorant")
	})
	r.press(tabKey)
	r.screen("[LOBBY]", "waiting for Valorant")
	stop()
	raw, err := os.ReadFile(filepath.Join(recordingDir(t, r.dir), "screens.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if s := string(raw); strings.Contains(s, "SETTINGS") || strings.Contains(s, "LOBBY") || strings.Contains(s, "tab to switch") {
		t.Errorf("screens.txt should only hold lobby screens:\n%s", s)
	}
}
