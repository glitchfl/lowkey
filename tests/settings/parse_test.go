package settings_test

import (
	"os"
	"slices"
	"testing"

	"github.com/glitchfl/lowkey/internal/settings"
)

func TestWeaponTakesOneGunOrAList(t *testing.T) {
	cases := []struct {
		name, file string
		want       []string
	}{
		{"left out", "", []string{"vandal"}},
		{"one gun", `weapon = "Phantom"`, []string{"phantom"}},
		{"a list in its order", `weapon = ["Sheriff", " vandal ", "operator"]`, []string{"sheriff", "vandal", "operator"}},
		{"a list of one", `weapon = ["melee"]`, []string{"melee"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _, err := settings.Parse([]byte("[table]\n" + c.file))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(s.Weapons, c.want) {
				t.Errorf("got %v, want %v", s.Weapons, c.want)
			}
		})
	}
}

func TestAgentDotsKeepWhatTheyCanUse(t *testing.T) {
	raze := settings.DotSlot{Agent: "Raze", At: settings.Spot{X: 800, Y: 450, Placed: true}}
	cases := []struct {
		name, file string
		want       []settings.DotSlot
		noted      bool
	}{
		{"left out", "", nil, false},
		{"empty", "agent_dots = []", nil, false},
		{"one placed", `agent_dots = [{agent = "Raze", at = [800, 450]}]`, []settings.DotSlot{raze}, false},
		{"not placed yet", `agent_dots = [{agent = "Raze", at = []}, {agent = "Jett"}]`,
			[]settings.DotSlot{{Agent: "Raze"}, {Agent: "Jett"}}, false},
		{"on a screen left of the main one", `agent_dots = [{agent = "Raze", at = [-1200, 300]}]`,
			[]settings.DotSlot{{Agent: "Raze", At: settings.Spot{X: -1200, Y: 300, Placed: true}}}, false},
		{"a bad spot unplaces only its own dot", `agent_dots = [{agent = "Raze", at = [800, 450]}, {agent = "Jett", at = [1.5, 2]}]`,
			[]settings.DotSlot{raze, {Agent: "Jett"}}, true},
		{"one number", `agent_dots = [{agent = "Raze", at = [800]}]`, []settings.DotSlot{{Agent: "Raze"}}, true},
		{"no agent", `agent_dots = [{at = [800, 450]}, {agent = "Raze", at = [800, 450]}]`, []settings.DotSlot{raze}, true},
		{"an unknown key", `agent_dots = [{agent = "Raze", at = [800, 450], color = "red"}]`, []settings.DotSlot{raze}, true},
		{"more than three", `agent_dots = [{agent = "Raze", at = [800, 450]}, {agent = "Jett"}, {agent = "Sova"}, {agent = "Omen"}]`,
			[]settings.DotSlot{raze, {Agent: "Jett"}, {Agent: "Sova"}}, true},
		{"not a list", `agent_dots = "raze"`, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, note, err := settings.Parse([]byte("[agent_select]\n" + c.file))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(s.AgentDots, c.want) {
				t.Errorf("got %+v, want %+v", s.AgentDots, c.want)
			}
			if (note != "") != c.noted {
				t.Errorf("note %q - want one: %v", note, c.noted)
			}
		})
	}
}

func TestAgentDotsWriteBackTheWayTheyRead(t *testing.T) {
	f := settings.Open(t.TempDir())
	want := []settings.DotSlot{{Agent: "KAY/O", At: settings.Spot{X: 800, Y: 450, Placed: true}}, {Agent: "Jett"}}
	if _, err := f.Change(func(s *settings.Settings) { s.AgentDots = want }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Path())
	if err != nil {
		t.Fatal(err)
	}
	s, note, err := settings.Parse(data)
	if err != nil || note != "" {
		t.Fatalf("the written file reads back with %v %q:\n%s", err, note, data)
	}
	if !slices.Equal(s.AgentDots, want) {
		t.Errorf("after a round trip got %+v, want %+v", s.AgentDots, want)
	}
}

func TestTheAgentDotTimeLimitStaysBetweenOneAndTwentySeconds(t *testing.T) {
	cases := []struct {
		name, file string
		want       int
		noted      bool
	}{
		{"left out", "", 15, false},
		{"in range", "agent_dot_seconds = 5", 5, false},
		{"too short", "agent_dot_seconds = 0", 1, true},
		{"too long", "agent_dot_seconds = 60", 20, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, note, err := settings.Parse([]byte("[agent_select]\n" + c.file))
			if err != nil {
				t.Fatal(err)
			}
			if s.AgentDotSeconds != c.want {
				t.Errorf("got %d, want %d", s.AgentDotSeconds, c.want)
			}
			if (note != "") != c.noted {
				t.Errorf("note %q - want one: %v", note, c.noted)
			}
		})
	}
}

func TestTheAgentDotIsOffByDefault(t *testing.T) {
	if settings.Default().AgentDot {
		t.Error("the agent dot is on by default")
	}
}
