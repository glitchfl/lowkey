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

const weaponComment = "# the gun the skin column shows - vandal phantom operator sheriff melee or any other by its name\n" +
	"# a list like [\"vandal\", \"phantom\"] gives each gun a skin column\n"

func TestChangeKeepsEverythingElseInTheFile(t *testing.T) {
	more := func(s *settings.Settings) { s.Matches++ }
	cases := []struct {
		name, before string
		edit         func(*settings.Settings)
		after        string
	}{
		{"comments and order", "# mine\n\nweapon = \"phantom\"\n\n# also mine\nmatches = 7\n", more,
			"# mine\n\nweapon = \"phantom\"\n\n# also mine\nmatches = 8\n"},
		{"a comment after the value", "matches = 7 # my favorite\n", more, "matches = 8 # my favorite\n"},
		{"tight spacing", "matches=7\n", more, "matches=8\n"},
		{"wide spacing", "  matches   =   7\n", more, "  matches   =   8\n"},
		{"windows line endings", "weapon = \"ares\"\r\nmatches = 7\r\nenemy_lookups = false\r\n", more,
			"weapon = \"ares\"\r\nmatches = 8\r\nenemy_lookups = false\r\n"},
		{"a byte order mark", "\xef\xbb\xbfmatches = 7\n", more, "\xef\xbb\xbfmatches = 8\n"},
		{"a quoted key", "\"matches\" = 7\n", more, "\"matches\" = 8\n"},
		{"a literal quoted key", "'matches' = 7\n", more, "'matches' = 8\n"},
		{"a commented-out copy of the key", "# matches = 3\nmatches = 7\n", more, "# matches = 3\nmatches = 8\n"},
		{"tricky values before it", "region = \"auto\" # na ] [ # = \nweapon = 'phantom'\nmodes = [\"unrated\", # [ \" \n  \"premier\",\n]\nmatches = 7\n", more,
			"region = \"auto\" # na ] [ # = \nweapon = 'phantom'\nmodes = [\"unrated\", # [ \" \n  \"premier\",\n]\nmatches = 8\n"},
		{"a string with an escaped quote before it", "# x\nweapon = \"vandal\"\nregion = \"\\u0061uto\"\nmatches = 7\n", more,
			"# x\nweapon = \"vandal\"\nregion = \"\\u0061uto\"\nmatches = 8\n"},
		{"a multi-line array", "columns = [\n  \"name\", # who\n  \"rank\",\n]\nmatches = 7\n",
			func(s *settings.Settings) { s.Columns = []string{"rank", "name"} }, "columns = [\"rank\", \"name\"]\nmatches = 7\n"},
		{"a multi-line string value", "weapon = \"\"\"vandal\"\"\"\nmatches = 7\n",
			func(s *settings.Settings) { s.Weapons = []string{"phantom"} }, "weapon = \"phantom\"\nmatches = 7\n"},
		{"a literal multi-line string before it", "weapon = '''vandal'''\nmatches = 7\n", more, "weapon = '''vandal'''\nmatches = 8\n"},
		{"a key the file doesn't have", "matches = 7\n", func(s *settings.Settings) { s.Weapons = []string{"phantom"} },
			"matches = 7\n\n" + weaponComment + "weapon = \"phantom\"\n"},
		{"no newline at the end", "matches = 7", func(s *settings.Settings) { s.Weapons = []string{"phantom"} },
			"matches = 7\n\n" + weaponComment + "weapon = \"phantom\"\n"},
		{"a key at the very end without a newline", "matches = 7", more, "matches = 8"},
		{"one gun becomes a list", "weapon = \"vandal\" # mine\nmatches = 7\n",
			func(s *settings.Settings) { s.Weapons = append(s.Weapons, "phantom") }, "weapon = [\"vandal\", \"phantom\"] # mine\nmatches = 7\n"},
		{"a list of one goes back to a name", "weapon = [\n  \"vandal\",\n  \"phantom\",\n]\nmatches = 7\n",
			func(s *settings.Settings) { s.Weapons = s.Weapons[1:] }, "weapon = \"phantom\"\nmatches = 7\n"},
		{"two keys at once", "matches = 7\nweapon = \"ares\"\n",
			func(s *settings.Settings) { s.Matches, s.Weapons = 8, []string{"odin"} }, "matches = 8\nweapon = \"odin\"\n"},
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
	f, _ := openWith(t, "matches = 7\n")
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
	f, path := openWith(t, "# mine\nmatches = 7\n")
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
	f, path := openWith(t, "matches = 7\n")
	if err := os.WriteFile(path, []byte("matches = 7\nsession_reset_hour = 18\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(path, future, future)
	if _, err := f.Change(func(s *settings.Settings) { s.Matches = 8 }); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, path), "matches = 8\nsession_reset_hour = 18\n"; got != want {
		t.Errorf("\n--- want\n%q\n--- got\n%q", want, got)
	}
	if got := f.Settings().SessionResetHour; got != 18 {
		t.Errorf("reset hour: got %d, want 18", got)
	}
}

func TestChangeWorksFromTheFreshestValue(t *testing.T) {
	f, path := openWith(t, "matches = 7\n")
	if err := os.WriteFile(path, []byte("matches = 20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(path, future, future)
	if _, err := f.Change(func(s *settings.Settings) { s.Matches++ }); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "matches = 21\n" {
		t.Errorf("got %q", got)
	}
}

func TestChangeReplacesABrokenFileAndKeepsACopy(t *testing.T) {
	f, path := openWith(t, "matches = 7\n")
	broken := "matches = 7\ncolumns = [\n"
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
	f, path := openWith(t, "weapon = \"phantom\"\n")
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
	before := "# mine\ncolumns = [\"name\"]\n"
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
	f, _ := openWith(t, "columns = [\"name\", \"rank\"]\n")
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
	f, path := openWith(t, "matches = 99\n")
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
	f, path := openWith(t, "matches = 7\n")
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
		s.Cutoff = 5 * time.Second
		s.EnemyLookups = false
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
		s.Cutoff != 5*time.Second || s.EnemyLookups || s.Region != "kr" || strings.Join(s.Weapons, ",") != "ghost,vandal" {
		t.Errorf("got %+v", s)
	}
}
