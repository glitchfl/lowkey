package settings_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/settings"
)

func openWith(t *testing.T, content string) (*settings.File, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, settings.FileName)
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return settings.Open(dir), path
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const serverPodComment = "# show the game server's city in the header at match load and during the match\n"

const weaponComment = "# the gun the skin column shows - vandal phantom operator sheriff melee or any other by its name\n" +
	"# a list like [\"vandal\", \"phantom\"] gives each gun a skin column\n"

func TestChangeKeepsEverythingElseInTheFile(t *testing.T) {
	more := func(s *settings.Settings) { s.Matches++ }
	cases := []struct {
		name, before string
		edit         func(*settings.Settings)
		after        string
	}{
		{"comments and order", "# mine\n[table]\n\nweapon = \"phantom\"\n\n# also mine\nmatches = 7\n", more,
			"# mine\n[table]\n\nweapon = \"phantom\"\n\n# also mine\nmatches = 8\n"},
		{"a comment after the value", "[table]\nmatches = 7 # my favorite\n", more, "[table]\nmatches = 8 # my favorite\n"},
		{"tight spacing", "[table]\nmatches=7\n", more, "[table]\nmatches=8\n"},
		{"wide spacing", "  [ table ]  # mine\n  matches   =   7\n", more, "  [ table ]  # mine\n  matches   =   8\n"},
		{"windows line endings", "[table]\r\nweapon = \"ares\"\r\nmatches = 7\r\n[players]\r\nenemy_lookups = false\r\n", more,
			"[table]\r\nweapon = \"ares\"\r\nmatches = 8\r\n[players]\r\nenemy_lookups = false\r\n"},
		{"a byte order mark", "\xef\xbb\xbf[table]\nmatches = 7\n", more, "\xef\xbb\xbf[table]\nmatches = 8\n"},
		{"a quoted key", "[table]\n\"matches\" = 7\n", more, "[table]\n\"matches\" = 8\n"},
		{"a literal quoted key", "[table]\n'matches' = 7\n", more, "[table]\n'matches' = 8\n"},
		{"a commented-out copy of the key", "[table]\n# matches = 3\nmatches = 7\n", more, "[table]\n# matches = 3\nmatches = 8\n"},
		{"tricky values before it", "[general]\nregion = \"auto\" # na ] [ # = \n[table]\nweapon = 'phantom'\nmodes = [\"unrated\", # [ \" \n  \"premier\",\n]\nmatches = 7\n", more,
			"[general]\nregion = \"auto\" # na ] [ # = \n[table]\nweapon = 'phantom'\nmodes = [\"unrated\", # [ \" \n  \"premier\",\n]\nmatches = 8\n"},
		{"a string with an escaped quote before it", "# x\n[general]\nregion = \"\\u0061uto\"\n[table]\nweapon = \"vandal\"\nmatches = 7\n", more,
			"# x\n[general]\nregion = \"\\u0061uto\"\n[table]\nweapon = \"vandal\"\nmatches = 8\n"},
		{"a multi-line array", "[table]\ncolumns = [\n  \"name\", # who\n  \"rank\",\n]\nmatches = 7\n",
			func(s *settings.Settings) { s.Columns = []string{"rank", "name"} }, "[table]\ncolumns = [\"rank\", \"name\"]\nmatches = 7\n"},
		{"a multi-line string value", "[table]\nweapon = \"\"\"vandal\"\"\"\nmatches = 7\n",
			func(s *settings.Settings) { s.Weapons = []string{"phantom"} }, "[table]\nweapon = \"phantom\"\nmatches = 7\n"},
		{"a literal multi-line string before it", "[table]\nweapon = '''vandal'''\nmatches = 7\n", more, "[table]\nweapon = '''vandal'''\nmatches = 8\n"},
		{"a key the file doesn't have", "[table]\nmatches = 7\n", func(s *settings.Settings) { s.Weapons = []string{"phantom"} },
			"[table]\nmatches = 7\n\n" + weaponComment + "weapon = \"phantom\"\n"},
		{"no newline at the end", "[table]\nmatches = 7", func(s *settings.Settings) { s.Weapons = []string{"phantom"} },
			"[table]\nmatches = 7\n\n" + weaponComment + "weapon = \"phantom\"\n"},
		{"a key at the very end without a newline", "[table]\nmatches = 7", more, "[table]\nmatches = 8"},
		{"a new key goes at the end of its own section", "[table]\nmatches = 7 # mine\n\n# about the rest\n[general]\nregion = \"ap\"\n",
			func(s *settings.Settings) { s.Weapons = []string{"phantom"} },
			"[table]\nmatches = 7 # mine\n\n" + weaponComment + "weapon = \"phantom\"\n\n# about the rest\n[general]\nregion = \"ap\"\n"},
		{"a new key in an empty section", "[match]\n\n[table]\nmatches = 7\n", func(s *settings.Settings) { s.ServerPod = true },
			"[match]\n\n" + serverPodComment + "server_pod = true\n\n[table]\nmatches = 7\n"},
		{"a section the file doesn't have", "[table]\nmatches = 7\n", func(s *settings.Settings) { s.ServerPod = true },
			"[table]\nmatches = 7\n\n[match]\n\n" + serverPodComment + "server_pod = true\n"},
		{"one gun becomes a list", "[table]\nweapon = \"vandal\" # mine\nmatches = 7\n",
			func(s *settings.Settings) { s.Weapons = append(s.Weapons, "phantom") }, "[table]\nweapon = [\"vandal\", \"phantom\"] # mine\nmatches = 7\n"},
		{"a list of one goes back to a name", "[table]\nweapon = [\n  \"vandal\",\n  \"phantom\",\n]\nmatches = 7\n",
			func(s *settings.Settings) { s.Weapons = s.Weapons[1:] }, "[table]\nweapon = \"phantom\"\nmatches = 7\n"},
		{"two keys at once", "[table]\nmatches = 7\nweapon = \"ares\"\n",
			func(s *settings.Settings) { s.Matches, s.Weapons = 8, []string{"odin"} }, "[table]\nmatches = 8\nweapon = \"odin\"\n"},
		{"two keys in two sections", "[table]\nmatches = 7\n\n[general]\nregion = \"ap\"\n",
			func(s *settings.Settings) { s.Matches, s.Region = 8, "eu" }, "[table]\nmatches = 8\n\n[general]\nregion = \"eu\"\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, path := openWith(t, c.before)
			if f.Invalid() {
				t.Fatalf("the starting file should be fine: %s", f.Warning())
			}
			backup, err := f.Change(c.edit)
			if err != nil || backup != "" {
				t.Fatalf("backup %q err %v", backup, err)
			}
			if got := read(t, path); got != c.after {
				t.Errorf("\n--- want\n%q\n--- got\n%q", c.after, got)
			}
		})
	}
}

func TestChangeUpdatesTheSettingsStraightAway(t *testing.T) {
	f, _ := openWith(t, "[table]\nmatches = 7\n")
	if _, err := f.Change(func(s *settings.Settings) { s.Matches = 8 }); err != nil {
		t.Fatal(err)
	}
	if got := f.Settings().Matches; got != 8 {
		t.Errorf("matches: got %d, want 8", got)
	}
	if f.Refresh() {
		t.Error("its own save shouldn't look like a new edit")
	}
}

func TestChangeThatChangesNothingWritesNothing(t *testing.T) {
	f, path := openWith(t, "# mine\n[table]\nmatches = 7\n")
	long := time.Now().Add(-time.Hour)
	os.Chtimes(path, long, long)
	f.Refresh()
	if _, err := f.Change(func(s *settings.Settings) { s.Matches = 7 }); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(long) {
		t.Error("the file was written")
	}
}

func TestChangeDoesNotLoseAHandEditItHasNotSeenYet(t *testing.T) {
	f, path := openWith(t, "[table]\nmatches = 7\n")
	if err := os.WriteFile(path, []byte("[table]\nmatches = 7\n[general]\nsession_reset_hour = 18\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(path, future, future)
	if _, err := f.Change(func(s *settings.Settings) { s.Matches = 8 }); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, path), "[table]\nmatches = 8\n[general]\nsession_reset_hour = 18\n"; got != want {
		t.Errorf("\n--- want\n%q\n--- got\n%q", want, got)
	}
	if got := f.Settings().SessionResetHour; got != 18 {
		t.Errorf("reset hour: got %d, want 18", got)
	}
}

func TestChangeWorksFromTheFreshestValue(t *testing.T) {
	f, path := openWith(t, "[table]\nmatches = 7\n")
	if err := os.WriteFile(path, []byte("[table]\nmatches = 20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(path, future, future)
	if _, err := f.Change(func(s *settings.Settings) { s.Matches++ }); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "[table]\nmatches = 21\n" {
		t.Errorf("got %q", got)
	}
}

func TestChangeReplacesABrokenFileAndKeepsACopy(t *testing.T) {
	f, path := openWith(t, "[table]\nmatches = 7\n")
	broken := "[table]\nmatches = 7\ncolumns = [\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(path, future, future)
	f.Refresh()
	if !f.Invalid() {
		t.Fatal("the file should count as broken")
	}
	backup, err := f.Change(func(s *settings.Settings) { s.Weapons = []string{"phantom"} })
	if err != nil {
		t.Fatal(err)
	}
	if backup != "lowkey.toml.bak" {
		t.Errorf("backup: got %q", backup)
	}
	if got := read(t, path+".bak"); got != broken {
		t.Errorf("the copy should be word for word: %q", got)
	}
	s, _, err := settings.Parse([]byte(read(t, path)))
	if err != nil {
		t.Fatalf("the new file should parse: %v", err)
	}
	if s.Matches != 7 || strings.Join(s.Weapons, ",") != "phantom" {
		t.Errorf("it should hold the last good settings plus the change: %+v", s)
	}
	if f.Invalid() || f.Warning() != "" {
		t.Errorf("the file is fine now: %q", f.Warning())
	}
}

func TestChangeWithNoFileWritesAWholeOne(t *testing.T) {
	f, path := openWith(t, "")
	backup, err := f.Change(func(s *settings.Settings) { s.Matches = 8 })
	if err != nil || backup != "" {
		t.Fatalf("backup %q err %v", backup, err)
	}
	got := read(t, path)
	for _, want := range []string{"matches = 8\n", `region = "auto"`, "# recent matches per player"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if !f.Exists() {
		t.Error("the file exists now")
	}
}

func TestChangeWithAnEmptyFileHoldsOnToTheLastGoodSettings(t *testing.T) {
	f, path := openWith(t, "[table]\nweapon = \"phantom\"\n")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	backup, err := f.Change(func(s *settings.Settings) { s.Matches = 8 })
	if err != nil || backup != "" {
		t.Fatalf("backup %q err %v", backup, err)
	}
	got := read(t, path)
	if !strings.Contains(got, `weapon = "phantom"`) || !strings.Contains(got, "matches = 8\n") {
		t.Errorf("an empty file shouldn't reset the rest to defaults:\n%s", got)
	}
}

func TestChangeRefusesAnInvalidEditAndWritesNothing(t *testing.T) {
	before := "# mine\n[table]\ncolumns = [\"name\"]\n"
	f, path := openWith(t, before)
	if _, err := f.Change(func(s *settings.Settings) { s.Columns = nil }); err == nil {
		t.Error("an empty column list should be refused")
	}
	if got := read(t, path); got != before {
		t.Errorf("nothing should change: %q", got)
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("nothing should be backed up")
	}
	if got := f.Settings().Columns; len(got) != 1 || got[0] != "name" {
		t.Errorf("columns: got %v", got)
	}
}

func TestChangeDoesNotTouchTheSlicesCallersHold(t *testing.T) {
	f, _ := openWith(t, "[table]\ncolumns = [\"name\", \"rank\"]\n")
	held := f.Settings().Columns
	if _, err := f.Change(func(s *settings.Settings) { s.Columns[0], s.Columns[1] = s.Columns[1], s.Columns[0] }); err != nil {
		t.Fatal(err)
	}
	if held[0] != "name" {
		t.Errorf("the edit reached a slice someone else holds: %v", held)
	}
	if got := f.Settings().Columns; got[0] != "rank" {
		t.Errorf("columns: got %v", got)
	}
}

func TestChangeKeepsAClampNoteUntilThatValueChanges(t *testing.T) {
	f, path := openWith(t, "[table]\nmatches = 99\n")
	if !strings.Contains(f.Warning(), "matches 99 is above 25") {
		t.Fatalf("warning: %q", f.Warning())
	}
	if _, err := f.Change(func(s *settings.Settings) { s.Weapons = []string{"phantom"} }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, path), "matches = 99") || !strings.Contains(f.Warning(), "matches 99 is above 25") {
		t.Errorf("the note stays while the file still says 99: %q", f.Warning())
	}
	if _, err := f.Change(func(s *settings.Settings) { s.Matches = 24 }); err != nil {
		t.Fatal(err)
	}
	if f.Warning() != "" {
		t.Errorf("the note should go: %q", f.Warning())
	}
}

func TestChangeLeavesNoTemporaryFilesBehind(t *testing.T) {
	f, path := openWith(t, "[table]\nmatches = 7\n")
	if _, err := f.Change(func(s *settings.Settings) { s.Matches = 8 }); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("want only lowkey.toml, got %v", entries)
	}
}

func TestAWholeNewFileParsesToWhatItWasWrittenFrom(t *testing.T) {
	f, path := openWith(t, "")
	if _, err := f.Change(func(s *settings.Settings) {
		s.Columns = []string{"rank", "name"}
		s.Matches = 3
		s.Modes = nil
		s.SessionResetHour = 0
		s.CutoffSeconds = 5
		s.EnemyLookups = false
		s.HiddenPlayerStats = false
		s.ExactEnemyParties, s.ServerPod, s.VoiceCheck, s.PlayerDetails = true, true, true, true
		s.SiteScouting, s.WeaponProfiling = true, true
		s.Region = "kr"
		s.Weapons = []string{"ghost", "vandal"}
	}); err != nil {
		t.Fatal(err)
	}
	s, note, err := settings.Parse([]byte(read(t, path)))
	if err != nil || note != "" {
		t.Fatalf("note %q err %v", note, err)
	}
	if strings.Join(s.Columns, ",") != "rank,name" || s.Matches != 3 || len(s.Modes) != 0 || s.SessionResetHour != 0 ||
		s.CutoffSeconds != 5 || s.EnemyLookups || s.HiddenPlayerStats ||
		!s.ExactEnemyParties || !s.ServerPod || !s.VoiceCheck || !s.PlayerDetails || !s.SiteScouting || !s.WeaponProfiling ||
		s.Region != "kr" || strings.Join(s.Weapons, ",") != "ghost,vandal" {
		t.Errorf("got %+v", s)
	}
}
