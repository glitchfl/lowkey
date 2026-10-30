// gen rebuilds snapshot.json from valorant-api.com - run it through go generate before a release
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type tier struct {
	Tier  int    `json:"tier"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type skin struct {
	Name    string `json:"name"`
	Edition string `json:"edition,omitempty"`
}

type snapshot struct {
	Generated      time.Time         `json:"generated"`
	ClientVersion  string            `json:"clientVersion"`
	Agents         map[string]string `json:"agents"`
	Maps           map[string]string `json:"maps"`
	TierSets       map[string][]tier `json:"tierSets"`
	CurrentTierSet string            `json:"currentTierSet"`
	Acts           map[string]string `json:"acts"`
	Weapons        map[string]string `json:"weapons"`
	Skins          map[string]skin   `json:"skins"`
	Editions       map[string]string `json:"editions"`
	IDs            []string          `json:"ids"`
}

func get(path string, v any) error {
	resp, err := http.Get("https://valorant-api.com" + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", path, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, &struct {
		Data any `json:"data"`
	}{v})
}

func main() {
	out := "snapshot.json"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := run(out); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	s := snapshot{
		Generated: time.Now().UTC().Truncate(time.Second),
		Agents:    map[string]string{},
		Maps:      map[string]string{},
		TierSets:  map[string][]tier{},
		Acts:      map[string]string{},
		Weapons:   map[string]string{},
		Skins:     map[string]skin{},
		Editions:  map[string]string{},
	}
	ids := map[string]bool{}

	var version struct {
		RiotClientVersion string `json:"riotClientVersion"`
	}
	if err := get("/v1/version", &version); err != nil {
		return err
	}
	s.ClientVersion = version.RiotClientVersion

	var agents []struct {
		UUID        string `json:"uuid"`
		DisplayName string `json:"displayName"`
	}
	if err := get("/v1/agents?isPlayableCharacter=true", &agents); err != nil {
		return err
	}
	for _, a := range agents {
		s.Agents[a.UUID] = a.DisplayName
	}

	var maps []struct {
		UUID        string `json:"uuid"`
		DisplayName string `json:"displayName"`
		MapURL      string `json:"mapUrl"`
	}
	if err := get("/v1/maps", &maps); err != nil {
		return err
	}
	for _, m := range maps {
		if m.MapURL != "" {
			s.Maps[m.MapURL] = m.DisplayName
		}
		ids[m.UUID] = true
	}

	var sets []struct {
		UUID  string `json:"uuid"`
		Tiers []struct {
			Tier     int    `json:"tier"`
			TierName string `json:"tierName"`
			Color    string `json:"color"`
		} `json:"tiers"`
	}
	if err := get("/v1/competitivetiers", &sets); err != nil {
		return err
	}
	for _, set := range sets {
		for _, t := range set.Tiers {
			s.TierSets[set.UUID] = append(s.TierSets[set.UUID], tier{Tier: t.Tier, Name: t.TierName, Color: t.Color})
		}
	}

	var seasons []struct {
		UUID string `json:"uuid"`
	}
	if err := get("/v1/seasons", &seasons); err != nil {
		return err
	}
	for _, season := range seasons {
		ids[season.UUID] = true
	}

	var competitive []struct {
		SeasonUUID string    `json:"seasonUuid"`
		TiersUUID  string    `json:"competitiveTiersUuid"`
		StartTime  time.Time `json:"startTime"`
	}
	if err := get("/v1/seasons/competitive", &competitive); err != nil {
		return err
	}
	var latest time.Time
	for _, c := range competitive {
		s.Acts[c.SeasonUUID] = c.TiersUUID
		if c.StartTime.After(latest) {
			latest, s.CurrentTierSet = c.StartTime, c.TiersUUID
		}
	}

	var weapons []struct {
		UUID            string `json:"uuid"`
		DisplayName     string `json:"displayName"`
		DefaultSkinUUID string `json:"defaultSkinUuid"`
		Skins           []struct {
			UUID            string `json:"uuid"`
			DisplayName     string `json:"displayName"`
			ContentTierUUID string `json:"contentTierUuid"`
		} `json:"skins"`
	}
	if err := get("/v1/weapons", &weapons); err != nil {
		return err
	}
	for _, w := range weapons {
		s.Weapons[w.UUID] = w.DisplayName
		for _, sk := range w.Skins {
			s.Skins[sk.UUID] = skin{Name: skinName(sk.DisplayName, w.DisplayName, sk.UUID == w.DefaultSkinUUID), Edition: sk.ContentTierUUID}
		}
	}

	var editions []struct {
		UUID           string `json:"uuid"`
		HighlightColor string `json:"highlightColor"`
	}
	if err := get("/v1/contenttiers", &editions); err != nil {
		return err
	}
	for _, e := range editions {
		s.Editions[e.UUID] = e.HighlightColor
	}

	for id := range ids {
		s.IDs = append(s.IDs, id)
	}
	sort.Strings(s.IDs)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, append(data, '\n'), 0o644)
}

// skinName drops the weapon from the name - every default becomes Standard since the melee's is just "Melee"
// and the random favorites placeholder becomes Random
func skinName(name, weapon string, isDefault bool) string {
	name = strings.TrimSpace(name)
	switch {
	case isDefault:
		return "Standard"
	case name == "Random Favorite Skin":
		return "Random"
	}
	return strings.TrimSuffix(name, " "+weapon)
}
