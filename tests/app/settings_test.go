package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func (r *rig) writeSettings(content string) {
	r.t.Helper()
	path := filepath.Join(r.dir, "lowkey.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
	// filesystems with coarse timestamps would hide a quick second edit
	future := time.Now().Add(time.Duration(len(content)) * time.Second)
	os.Chtimes(path, future, future)
}

func (r *rig) settingsFile() string {
	r.t.Helper()
	data, err := os.ReadFile(filepath.Join(r.dir, "lowkey.toml"))
	if err != nil {
		r.t.Fatal(err)
	}
	return string(data)
}

func TestTheFirstRunWritesEveryDefaultWithAComment(t *testing.T) {
	r := newRig(t)
	r.live()
	r.screen("waiting for the next match")
	file := r.settingsFile()
	defaults := []string{
		`columns = ["party", "name", "agent", "rank", "peak", "last_act", "wr", "wl", "kd", "hs", "acs", "adr", "fb", "n", "mains", "rr_trend", "met", "skin"]`,
		`matches = 13`,
		`modes = ["competitive", "premier", "unrated", "swiftplay", "deathmatch", "team_deathmatch"]`,
		`session_reset_hour = 6`,
		`cutoff_seconds = 40`,
		`enemy_lookups = true`,
		`hidden_player_stats = true`,
		`region = "auto"`,
		`weapon = "vandal"`,
	}
	lines := strings.Split(file, "\n")
	for _, d := range defaults {
		i := -1
		for j, l := range lines {
			if l == d {
				i = j
			}
		}
		if i < 1 || !strings.HasPrefix(lines[i-1], "# ") {
			t.Errorf("want %q with a comment above it in:\n%s", d, file)
			continue
		}
		comment := lines[i-1]
		if strings.HasSuffix(comment, ".") || comment != strings.ToLower(comment) {
			t.Errorf("comment %q should be lowercase without a trailing period", comment)
		}
	}

	edited := strings.Replace(file, "matches = 13", "matches = 7", 1)
	r.writeSettings(edited)
	r.restart()
	r.screen("waiting for the next match")
	if got := r.settingsFile(); got != edited {
		t.Errorf("a later run rewrote the user's edits:\n%s", got)
	}
}

func TestColumnsComeFromTheSettingsInTheirOrder(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.writeSettings(`columns = ["rank", "agent", "rr_trend"]` + "\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "Dia1 50")
	header := ""
	for _, l := range strings.Split(frame, "\n") {
		if strings.Contains(l, "RANK") && !strings.Contains(l, "·") {
			header = strings.Join(strings.Fields(l), " ")
		}
	}
	if header != "RANK AGENT TREND" {
		t.Errorf("header %q, want RANK AGENT TREND\n%s", header, frame)
	}
}

func TestEditsApplyAtTheNextRedraw(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50", "MAINS")
	r.writeSettings(`columns = ["name", "agent", "rank"]` + "\n")
	frame := r.screen("AGENT SELECT", "Dia1 50")
	eventually(t, "the edited columns", func() bool {
		frame = r.con.last()
		return !strings.Contains(frame, "MAINS") && strings.Contains(frame, "RANK")
	})
}

func TestInvalidSettingsKeepTheLastGoodOnesAndSayWhy(t *testing.T) {
	cases := []struct {
		name, content, warning string
	}{
		{"broken toml", "columns = [\"name\",\n", "lowkey.toml"},
		{"unknown column", `columns = ["name", "kda"]` + "\n", `unknown column "kda"`},
		{"unknown setting", `matchs = 5` + "\n", `unknown setting "matchs"`},
		{"bad hour", `session_reset_hour = 25` + "\n", "session_reset_hour"},
		{"unknown weapon", `weapon = "vandl"` + "\n", `unknown weapon "vandl"`},
		{"unknown weapon in a list", `weapon = ["vandal", "vandl"]` + "\n", `unknown weapon "vandl"`},
		{"a gun listed twice", `weapon = ["vandal", "Vandal"]` + "\n", `weapon "Vandal" is listed twice`},
		{"no guns", `weapon = []` + "\n", "weapon is an empty list"},
		{"a gun that isn't a name", `weapon = ["vandal", 3]` + "\n", "weapon takes a gun's name or a list of them"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, agentSelectLobby)
			r.writeSettings(`columns = ["agent", "rank"]` + "\n")
			r.live()
			r.screen("waiting for the next match")
			r.subscribed()
			r.writeSettings(c.content)
			r.enterAgentSelect()
			frame := r.settled("AGENT SELECT", "Dia1 50", c.warning)
			if !strings.Contains(frame, "using the last good settings") {
				t.Errorf("the warning should say the last good settings stay:\n%s", frame)
			}
			if strings.Contains(frame, "PLAYER") {
				t.Errorf("the last good columns (agent and rank) should still apply:\n%s", frame)
			}
		})
	}
}

func TestMatchesIsClampedToOneThroughTwentyFive(t *testing.T) {
	for _, c := range []struct {
		value, want, warning string
	}{{"0", "1", "matches 0 is below 1 - using 1"}, {"99", "25", "matches 99 is above 25 - using 25"}} {
		t.Run(c.value, func(t *testing.T) {
			r := newRig(t, agentSelectLobby)
			r.writeSettings("matches = " + c.value + "\n")
			r.live()
			r.screen("waiting for the next match", c.warning)
			r.subscribed()
			r.enterAgentSelect()
			r.settled("AGENT SELECT", "Dia1 50")
			for _, req := range r.fr.RemoteRequests() {
				if strings.HasPrefix(req.Path, "/match-history/") && req.Query.Get("endIndex") != c.want {
					t.Errorf("history asked for %s matches, want %s", req.Query.Get("endIndex"), c.want)
				}
			}
		})
	}
}

func TestEnemyLookupsCanBeSwitchedOff(t *testing.T) {
	r := newRig(t, matchLobby)
	r.writeSettings("enemy_lookups = false\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "ENEMIES", "Raze")
	for _, e := range []string{enemy1, enemy2, enemy3, enemy4, enemy5} {
		for _, req := range r.fr.RemoteRequests() {
			if strings.Contains(req.Path+"?"+req.Query.Encode(), e) {
				t.Errorf("an enemy was looked up: %s", req.Path)
			}
		}
	}
	if got := cellIn(t, frame, "RANK", 9); got != "" {
		t.Errorf("enemies show their agent only, got rank %q", got)
	}
	if got := cellIn(t, frame, "AGENT", 9); got != "Raze" {
		t.Errorf("enemy agent: got %q", got)
	}
	if got := cellIn(t, frame, "RANK", 3); got != "Dia1 50" {
		t.Errorf("allies are unaffected, ally one's rank: got %q", got)
	}
}

func TestModesCutoffAndResetHourComeFromTheSettings(t *testing.T) {
	t.Run("modes", func(t *testing.T) {
		r := newRig(t, agentSelectLobby)
		r.writeSettings(`modes = ["unrated"]` + "\n")
		r.live()
		r.screen("waiting for the next match")
		r.subscribed()
		r.enterAgentSelect()
		r.screen("not tracking competitive")
	})
	t.Run("cutoff", func(t *testing.T) {
		r := newRig(t, matchLobby)
		r.writeSettings("cutoff_seconds = 10\n")
		r.live()
		r.screen("waiting for the next match")
		r.subscribed()
		r.enterMatchLoad()
		r.settled("MATCH LOADING", "Plat2 10")
		r.clock.Advance(10 * time.Second)
		r.screen("IN MATCH")
	})
	t.Run("reset hour", func(t *testing.T) {
		r := newRig(t, ranked, todaysGames)
		r.writeSettings("session_reset_hour = 18\n")
		r.live()
		// since 18:00 only the 19:00 win and the 18:00 loss count
		r.screen("SESSION · 1W 1L +3 RR since 18:00")
	})
}

func TestARegionInTheSettingsOverridesTheLaunchArguments(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.writeSettings(`region = "ap"` + "\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	for _, req := range r.fr.RemoteRequests() {
		if req.Service != "static" && req.Shard != "ap" {
			t.Errorf("%s went to shard %q, want ap", req.Path, req.Shard)
		}
		if req.Service == "glz" && req.Region != "ap" {
			t.Errorf("%s went to region %q, want ap", req.Path, req.Region)
		}
	}
}

func TestDoctorReportsTheSettingsFile(t *testing.T) {
	r := newRig(t)
	out, _ := r.run("doctor")
	if l := line(t, out, "settings"); !strings.HasPrefix(l, "PASS") || !strings.Contains(l, "defaults") {
		t.Errorf("missing file: got %q", l)
	}
	r2 := newRig(t)
	r2.writeSettings(`columns = ["kda"]` + "\n")
	out, code := r2.run("doctor")
	if l := line(t, out, "settings"); code == 0 || !strings.HasPrefix(l, "FAIL") || !strings.Contains(l, `unknown column "kda"`) {
		t.Errorf("broken file: got %q (exit %d)", l, code)
	}
	r3 := newRig(t)
	r3.writeSettings("matches = 9\n")
	out, _ = r3.run("doctor")
	if l := line(t, out, "settings"); !strings.HasPrefix(l, "PASS") || !strings.Contains(l, "lowkey.toml") {
		t.Errorf("good file: got %q", l)
	}
}

func TestMeUsesTheSettings(t *testing.T) {
	r := newRig(t, ranked, threeCompMatches)
	r.writeSettings("matches = 2\nsession_reset_hour = 18\n")
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	if got := cell(t, out, "N"); got != "2" {
		t.Errorf("n: got %q, want 2", got)
	}
	if !strings.Contains(out, "since 18:00") {
		t.Errorf("the session should start at 18:00:\n%s", out)
	}
}
