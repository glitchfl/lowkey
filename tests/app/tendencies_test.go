package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// spots on the snapshot's callouts so each one is plainly in its place
var (
	ascentA   = fakeriot.Spot{X: 6130, Y: -8210}
	ascentB   = fakeriot.Spot{X: -2344, Y: -7549}
	ascentMid = fakeriot.Spot{X: 1089, Y: -7363}
	havenC    = fakeriot.Spot{X: -2378, Y: -9011}
)

// allyOneTendencies gives ally one two Ascent matches and a Haven one on Jett - on Ascent they held B in 3 of
// the 4 defense rounds placed in time - their team planted A twice and B once - both opening duels on attack
// were in Mid - across all three they got 2 Operator kills 3 Judge kills and a Vandal kill and started 3 rounds
// with an Operator
func allyOneTendencies(w *fakeriot.World) {
	add := func(id, mapURL string, ago time.Duration, rounds ...fakeriot.Round) {
		w.AddMatch(&fakeriot.Match{
			ID: id, Queue: "competitive", Map: mapURL, Mode: fakeriot.ModeBomb, Start: testStart.Add(-ago),
			Players: []fakeriot.MatchPlayer{
				{PUUID: ally1, Team: "Blue", Agent: fakeriot.Jett, Score: 3000, Rounds: len(rounds), Kills: 5, Deaths: 3},
				{PUUID: enemy, Team: "Red", Agent: fakeriot.Omen, Score: 1000, Rounds: len(rounds), Kills: 3, Deaths: 5},
			},
			Teams:  []fakeriot.TeamResult{{ID: "Blue", Won: true, RoundsWon: 13}, {ID: "Red", RoundsWon: 5}},
			Rounds: rounds,
		})
	}
	opRound := map[string]string{ally1: fakeriot.Operator}
	add("7e7e7e7e-0000-4000-8000-000000000001", fakeriot.MapAscent, 2*time.Hour,
		fakeriot.Round{Attackers: "Red", Spots: map[string]fakeriot.Spot{ally1: ascentB, enemy: ascentMid},
			Kills: []fakeriot.Kill{{Killer: enemy, Victim: ally1, At: 5000}}},
		fakeriot.Round{Attackers: "Red", Spots: map[string]fakeriot.Spot{ally1: ascentB, enemy: ascentB}, Loadouts: opRound,
			Kills: []fakeriot.Kill{{Killer: ally1, Victim: enemy, At: 20000, Weapon: fakeriot.Operator}}},
		fakeriot.Round{Attackers: "Red", Spots: map[string]fakeriot.Spot{ally1: ascentA, enemy: ascentA}, Loadouts: opRound,
			Kills: []fakeriot.Kill{{Killer: ally1, Victim: enemy, At: 10000, Weapon: fakeriot.Operator}}},
		// nobody died early enough to say where they set up
		fakeriot.Round{Attackers: "Red", Spots: map[string]fakeriot.Spot{ally1: ascentA, enemy: ascentA},
			Kills: []fakeriot.Kill{{Killer: ally1, Victim: enemy, At: 30000}}},
		fakeriot.Round{Attackers: "Blue", Plant: "A", PlantAt: 40000, Spots: map[string]fakeriot.Spot{ally1: ascentMid, enemy: ascentMid},
			Kills: []fakeriot.Kill{{Killer: ally1, Victim: enemy, At: 8000, Weapon: fakeriot.Judge}}},
		fakeriot.Round{Attackers: "Blue", Plant: "B", PlantAt: 50000, Spots: map[string]fakeriot.Spot{ally1: ascentMid, enemy: ascentB},
			Kills: []fakeriot.Kill{{Killer: enemy, Victim: ally1, At: 6000}}},
	)
	add("7e7e7e7e-0000-4000-8000-000000000002", fakeriot.MapAscent, 26*time.Hour,
		fakeriot.Round{Attackers: "Red", Spots: map[string]fakeriot.Spot{ally1: ascentB, enemy: ascentB}, Loadouts: opRound,
			Kills: []fakeriot.Kill{{Killer: ally1, Victim: enemy, At: 12000, Weapon: fakeriot.Judge}}},
		fakeriot.Round{Attackers: "Blue", Plant: "A", PlantAt: 45000},
	)
	add("7e7e7e7e-0000-4000-8000-000000000003", fakeriot.MapHaven, 50*time.Hour,
		fakeriot.Round{Attackers: "Red", Spots: map[string]fakeriot.Spot{ally1: havenC, enemy: havenC},
			Kills: []fakeriot.Kill{{Killer: ally1, Victim: enemy, At: 5000, Weapon: fakeriot.Judge}}},
	)
}

var (
	siteRows = []string{
		"TENDENCIES · ASCENT (2 matches in sample)",
		"DEFENSE: B 75% · A 25% (4 rounds)",
		"ATTACK: A 67% · B 33% (3 plants)",
		"OPENING: Mid 100% (2 opening duels)",
	}
	gunRows = []string{
		"LEANS ON (3 matches in sample):",
		"• Operator on duelists (67% of rifle and sniper kills · 3 rounds with one)",
		"• Judge (3 kills · 50% of kills)",
	}
)

// allyOneDetails opens ally one's details at match load with the given extras switched on
func allyOneDetails(t *testing.T, extras string, setup ...func(w *fakeriot.World)) (*rig, string) {
	r := newRig(t, append([]func(w *fakeriot.World){matchLobby, lobbySkins}, setup...)...)
	r.writeSettings(detailsOn + extras)
	r.keyboard()
	r.live()
	r.screen("[LOBBY]", "waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "AllyOne#A1")
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "US - THEM", "Elderflame")
	r.con.setHeight(80)
	r.press(downKey, downKey, enterKey)
	return r, r.settled("esc back", "AllyOne#A1", "THIS ACT")
}

func TestSiteScoutingShowsWhereAPlayerTendsToPlayTheMap(t *testing.T) {
	r, frame := allyOneDetails(t, "site_scouting = true\n", allyOneTendencies)
	frame = r.settled("esc back", "TENDENCIES")
	wantRows(t, frame, siteRows...)
	if strings.Contains(frame, "LEANS ON") {
		t.Errorf("weapon profiling is still off:\n%s", frame)
	}
	if strings.Index(frame, "TENDENCIES") > strings.Index(frame, "WHEN") {
		t.Errorf("the tendencies go above the match list:\n%s", frame)
	}
}

func TestWeaponProfilingSaysWhichGunsAPlayerLeansOn(t *testing.T) {
	r, _ := allyOneDetails(t, "weapon_profiling = true\n", allyOneTendencies)
	frame := r.settled("esc back", "LEANS ON")
	wantRows(t, frame, append([]string{"TENDENCIES"}, gunRows...)...)
	if strings.Contains(frame, "DEFENSE") || strings.Contains(frame, "ASCENT") {
		t.Errorf("site scouting is still off:\n%s", frame)
	}
}

func TestBothExtrasShareOneBlock(t *testing.T) {
	r, _ := allyOneDetails(t, "site_scouting = true\nweapon_profiling = true\n", allyOneTendencies)
	frame := r.settled("esc back", "TENDENCIES", "LEANS ON")
	wantRows(t, frame, append(siteRows, gunRows...)...)
	if n := strings.Count(frame, "TENDENCIES"); n != 1 {
		t.Errorf("one heading for both, got %d:\n%s", n, frame)
	}
}

func TestTendenciesStayOutUntilSwitchedOnAndNeverAskRiot(t *testing.T) {
	r, frame := allyOneDetails(t, "", allyOneTendencies)
	if strings.Contains(frame, "TENDENCIES") {
		t.Errorf("both extras are off by default:\n%s", frame)
	}
	before := len(r.fr.RemoteRequests())
	r.writeSettings(detailsOn + "site_scouting = true\nweapon_profiling = true\n")
	frame = r.settled("esc back", "TENDENCIES", "LEANS ON")
	wantRows(t, frame, siteRows...)
	r.writeSettings(detailsOn)
	eventually(t, "the block to go once both are off", func() bool {
		return !strings.Contains(r.con.last(), "TENDENCIES") && strings.Contains(r.con.last(), "THIS ACT")
	})
	settle()
	if after := len(r.fr.RemoteRequests()); after != before {
		t.Errorf("switching the extras on and off sent %d requests, want none", after-before)
	}
}

func TestTendenciesSayWhenThereIsNothingToGoBy(t *testing.T) {
	r, _ := allyOneDetails(t, "site_scouting = true\nweapon_profiling = true\n", allyOneRecord)
	frame := r.settled("esc back", "TENDENCIES", "LEANS ON")
	wantRows(t, frame,
		"TENDENCIES",
		"no matches on Ascent in sample",
		"LEANS ON: no gun stands out (2 matches in sample)",
	)
}

func TestHiddenPlayersGetTheirTendenciesWithoutTheirName(t *testing.T) {
	r := inMatchLoad(t, hiddenSamples)
	r.writeSettings(detailsOn + "site_scouting = true\nweapon_profiling = true\n")
	r.press(downKey, downKey, downKey, downKey, enterKey)
	frame := r.settled("esc back", "TENDENCIES", "LEANS ON")
	wantRows(t, frame,
		"hidden Omen ally 120 Rad 900 Rad - -",
		"no matches on Ascent in sample",
		"LEANS ON: no gun stands out (2 matches in sample)",
	)
	if strings.Contains(frame, "SecretAlly") {
		t.Errorf("the hidden ally's name showed up:\n%s", frame)
	}
}
