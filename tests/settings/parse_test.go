package settings_test

import (
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
			s, _, err := settings.Parse([]byte(c.file))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(s.Weapons, c.want) {
				t.Errorf("got %v, want %v", s.Weapons, c.want)
			}
		})
	}
}

func TestTheAgentDotSpotTakesTwoWholeNumbers(t *testing.T) {
	cases := []struct {
		name, file string
		want       settings.Spot
		noted      bool
	}{
		{"left out", "", settings.Spot{}, false},
		{"empty", "agent_dot_at = []", settings.Spot{}, false},
		{"placed", "agent_dot_at = [1920, 1080]", settings.Spot{X: 1920, Y: 1080, Placed: true}, false},
		{"on a screen left of the main one", "agent_dot_at = [-1200, 300]", settings.Spot{X: -1200, Y: 300, Placed: true}, false},
		{"one number", "agent_dot_at = [800]", settings.Spot{}, true},
		{"three numbers", "agent_dot_at = [1, 2, 3]", settings.Spot{}, true},
		{"not whole", "agent_dot_at = [1.5, 2]", settings.Spot{}, true},
		{"words", `agent_dot_at = "center"`, settings.Spot{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, note, err := settings.Parse([]byte(c.file))
			if err != nil {
				t.Fatal(err)
			}
			if s.AgentDotAt != c.want {
				t.Errorf("got %+v, want %+v", s.AgentDotAt, c.want)
			}
			if (note != "") != c.noted {
				t.Errorf("note %q - want one: %v", note, c.noted)
			}
		})
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
			s, note, err := settings.Parse([]byte(c.file))
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
