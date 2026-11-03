package settings_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/glitchfl/lowkey/internal/settings"
)

func TestAFileFromBeforeSectionsIsMovedWhenItOpens(t *testing.T) {
	flat := "# my own note\nmatches = 7\nregion = \"ap\"\nweapon = [\"phantom\", \"vandal\"]\n" +
		"hidden_player_stats = false\nserver_pod = true\ncutoff_seconds = 20\ncolumns = [\"name\", \"rank\"]\n"
	f, path := openWith(t, flat)
	moved, err := f.Moved()
	if err != nil || moved != "lowkey.toml moved into sections - the old one is now lowkey.toml.bak" {
		t.Errorf("moved %q err %v", moved, err)
	}
	if got := read(t, path+".bak"); got != flat {
		t.Errorf("the old file should be kept word for word: %q", got)
	}
	file := read(t, path)
	s, _, err := settings.Parse([]byte(file))
	if err != nil {
		t.Fatalf("the moved file should parse: %v\n%s", err, file)
	}
	want := settings.Default()
	want.Matches, want.Region, want.Weapons = 7, "ap", []string{"phantom", "vandal"}
	want.HiddenPlayerStats, want.ServerPod, want.CutoffSeconds, want.Columns = false, true, 20, []string{"name", "rank"}
	if !sameSettings(s, want) || !sameSettings(f.Settings(), want) {
		t.Errorf("every value should come across\n got %+v\nwant %+v", s, want)
	}
	if f.Invalid() || f.Warning() != "" {
		t.Errorf("the moved file is fine: %q", f.Warning())
	}
	for _, header := range []string{"[table]", "[players]", "[match]", "[general]"} {
		if !strings.Contains(file, "\n"+header+"\n") {
			t.Errorf("missing %s in:\n%s", header, file)
		}
	}
}

func TestAFlatFileFromAnAgentDotBuildKeepsItsDotsWhenItsMoved(t *testing.T) {
	flat := "matches = 7\nagent_dot = true\nagent_dot_seconds = 9\n" +
		`agent_dots = [{agent = "Raze", at = [800, 450]}, {agent = "Jett", at = []}]` + "\n"
	f, path := openWith(t, flat)
	if moved, err := f.Moved(); moved == "" || err != nil {
		t.Fatalf("moved %q err %v", moved, err)
	}
	file := read(t, path)
	s, _, err := settings.Parse([]byte(file))
	if err != nil {
		t.Fatalf("the moved file should parse: %v\n%s", err, file)
	}
	want := settings.Default()
	want.Matches, want.AgentDot, want.AgentDotSeconds = 7, true, 9
	want.AgentDots = []settings.DotSlot{{Agent: "Raze", At: settings.Spot{X: 800, Y: 450, Placed: true}}, {Agent: "Jett"}}
	if !sameSettings(s, want) {
		t.Errorf("every dot should come across\n got %+v\nwant %+v", s, want)
	}
	if !strings.Contains(file, "\n[agent_select]\n") {
		t.Errorf("missing [agent_select] in:\n%s", file)
	}
}

func TestOnlyAFileFromBeforeSectionsIsMoved(t *testing.T) {
	cases := []struct{ name, content, warning string }{
		{"one with sections", "[table]\nmatches = 7\n", ""},
		{"an empty one", "", ""},
		{"one that doesn't parse", "matches = 7\ncolumns = [\n", "lowkey.toml: line"},
		{"one with a bad value", "matches = 7\ncolumns = [\"kda\"]\n", `unknown column "kda"`},
		{"one with a key lowkey doesn't have", "matches = 7\nmmr = true\n", `unknown setting "mmr"`},
		{"a stray key next to sections", "matches = 7\n[players]\nenemy_lookups = false\n", "matches goes under [table]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, path := openWith(t, c.content)
			if moved, err := f.Moved(); moved != "" || err != nil {
				t.Errorf("moved %q err %v", moved, err)
			}
			if c.content != "" && read(t, path) != c.content {
				t.Errorf("the file should stay as it was: %q", read(t, path))
			}
			if _, err := os.Stat(path + ".bak"); err == nil {
				t.Error("nothing should be backed up")
			}
			if !strings.Contains(f.Warning(), c.warning) || (c.warning == "") != (f.Warning() == "") {
				t.Errorf("warning: got %q, want %q", f.Warning(), c.warning)
			}
		})
	}
}

func TestAKeyOutsideItsSectionSaysWhereItGoes(t *testing.T) {
	cases := []struct{ name, content, err string }{
		{"at the top", "server_pod = true\n", "server_pod goes under [match]"},
		{"under another section", "[players]\nserver_pod = true\n", "server_pod goes under [match] - not [players]"},
		{"after a section ends", "[match]\nserver_pod = true\n\n[general]\nmatches = 7\n", "matches goes under [table] - not [general]"},
		{"a key no section has", "[players]\nfoo = 1\n", `unknown setting "players.foo"`},
		{"a section lowkey doesn't have", "[colors]\nfoo = 1\n", `unknown setting "colors"`},
		{"an agent dot key at the top", "agent_dot = true\n", "agent_dot goes under [agent_select]"},
		{"agent dots under another section", "[match]\nagent_dots = [{agent = \"Raze\", at = [800, 450]}]\n",
			"agent_dots goes under [agent_select] - not [match]"},
		{"a key agent select doesn't have", "[agent_select]\nagent = \"Raze\"\n", `unknown setting "agent_select.agent"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := settings.Parse([]byte(c.content)); err == nil || err.Error() != c.err {
				t.Errorf("got %v, want %q", err, c.err)
			}
		})
	}
}

func sameSettings(a, b settings.Settings) bool {
	return slices.Equal(a.Columns, b.Columns) && slices.Equal(a.Modes, b.Modes) && slices.Equal(a.Weapons, b.Weapons) &&
		a.Matches == b.Matches && a.SessionResetHour == b.SessionResetHour && a.CutoffSeconds == b.CutoffSeconds &&
		a.EnemyLookups == b.EnemyLookups && a.HiddenPlayerStats == b.HiddenPlayerStats &&
		a.ExactEnemyParties == b.ExactEnemyParties && a.ServerPod == b.ServerPod && a.VoiceCheck == b.VoiceCheck &&
		a.PlayerDetails == b.PlayerDetails && a.SiteScouting == b.SiteScouting && a.WeaponProfiling == b.WeaponProfiling &&
		a.Region == b.Region && a.AgentDot == b.AgentDot && a.AgentDotSeconds == b.AgentDotSeconds &&
		slices.Equal(a.AgentDots, b.AgentDots)
}
