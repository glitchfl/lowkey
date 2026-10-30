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
