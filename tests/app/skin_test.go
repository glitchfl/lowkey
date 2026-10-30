package app_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const (
	loadouts    = "/core-game/v1/matches/" + coreGame + "/loadouts"
	skinColumns = `columns = ["name", "agent", "rank", "skin"]` + "\n"
	newSkin     = "5c1a0000-0000-4000-8000-000000000001"
	ghostSkin   = "5c1a0000-0000-4000-8000-000000000002"
	newPhantom  = "5c1a0000-0000-4000-8000-000000000003"
	ultra       = "411e4a55-4e59-7757-41f0-86a53f101bb5"
)

// wear equips a skin in the core-game loadouts - every gun left alone stays standard
func wear(w *fakeriot.World, puuid, gun, skin string) {
	for i := range w.CoreGame.Players {
		if p := &w.CoreGame.Players[i]; p.PUUID == puuid {
			if p.Skins == nil {
				p.Skins = map[string]string{}
			}
			p.Skins[gun] = skin
		}
	}
}

// lobbySkins dresses the match lobby - the streamer-mode ally and enemy wear skins that must never show
func lobbySkins(w *fakeriot.World) {
	wear(w, w.Self, fakeriot.Vandal, fakeriot.ReaverVandal)
	wear(w, w.Self, fakeriot.Phantom, fakeriot.IonPhantom)
	wear(w, ally1, fakeriot.Vandal, fakeriot.ElderflameVandal)
	wear(w, ally3, fakeriot.Vandal, fakeriot.PrimeVandal)
	wear(w, enemy1, fakeriot.Vandal, fakeriot.SakuraVandal)
	wear(w, enemy3, fakeriot.Vandal, fakeriot.ElderflameVandal)
}

// skinColumn reads a match load's skins - you and the four allies then the five enemies
func skinColumn(t *testing.T, frame, header string) string {
	t.Helper()
	var cells []string
	for _, row := range []int{2, 3, 4, 5, 6, 9, 10, 11, 12, 13} {
		cells = append(cells, cellIn(t, frame, header, row))
	}
	return strings.Join(cells, ",")
}

func TestTheSkinColumnShowsEveryVisiblePlayersSkinForTheChosenGun(t *testing.T) {
	r := newRig(t, matchLobby, lobbySkins)
	r.writeSettings(skinColumns)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "Dia1 50")
	if strings.Contains(frame, "VANDAL") {
		t.Errorf("the skin column shows before there are any skins:\n%s", frame)
	}

	r.enterMatchLoad()
	frame = r.settled("MATCH LOADING · Ascent", "Reaver", "Sakura", "US - THEM")
	if got, want := skinColumn(t, frame, "VANDAL"), "Reaver,Elderflame,Standard,,Standard,Sakura,Standard,,Standard,Standard"; got != want {
		t.Errorf("vandals: got %s, want %s\n%s", got, want, frame)
	}

	// another gun is a settings edit away and needs no new request
	r.writeSettings(skinColumns + `weapon = "Phantom"` + "\n")
	frame = r.screen("PHANTOM", "Ion")
	if got, want := skinColumn(t, frame, "PHANTOM"), "Ion,Standard,Standard,,Standard,Standard,Standard,,Standard,Standard"; got != want {
		t.Errorf("phantoms: got %s, want %s\n%s", got, want, frame)
	}
	if n := requestsExact(r, loadouts); n != 1 {
		t.Errorf("loadouts fetched %d times, want once", n)
	}
}

// headerLine is the table's column header - the one line naming the column without a "·"
func headerLine(t *testing.T, frame, column string) string {
	t.Helper()
	for _, l := range strings.Split(frame, "\n") {
		if strings.Contains(l, column) && !strings.Contains(l, "·") {
			return strings.Join(strings.Fields(l), " ")
		}
	}
	t.Fatalf("no %s column in:\n%s", column, frame)
	return ""
}

func TestEachGunInTheListGetsASkinColumnInTheListsOrder(t *testing.T) {
	r := newRig(t, matchLobby, lobbySkins)
	r.writeSettings(skinColumns + `weapon = ["Phantom", "vandal"]` + "\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING · Ascent", "Reaver", "Ion", "Sakura", "US - THEM")
	if got, want := headerLine(t, frame, "PHANTOM"), "PLAYER AGENT RANK PHANTOM VANDAL"; got != want {
		t.Errorf("header %q, want %q\n%s", got, want, frame)
	}
	if got, want := skinColumn(t, frame, "PHANTOM"), "Ion,Standard,Standard,,Standard,Standard,Standard,,Standard,Standard"; got != want {
		t.Errorf("phantoms: got %s, want %s\n%s", got, want, frame)
	}
	if got, want := skinColumn(t, frame, "VANDAL"), "Reaver,Elderflame,Standard,,Standard,Sakura,Standard,,Standard,Standard"; got != want {
		t.Errorf("vandals: got %s, want %s\n%s", got, want, frame)
	}

	// dropping a gun only drops its column
	r.writeSettings(skinColumns + `weapon = "phantom"` + "\n")
	frame = r.screen("PHANTOM", "Ion")
	eventually(t, "the vandal column to go", func() bool {
		frame = r.con.last()
		return !strings.Contains(frame, "VANDAL") && strings.Contains(frame, "PHANTOM")
	})
	if n := requestsExact(r, loadouts); n != 1 {
		t.Errorf("loadouts fetched %d times, want once whatever the guns", n)
	}
}

func TestANewSkinOnAnyGunInTheListIsNamedByARefresh(t *testing.T) {
	r := newRig(t, matchLobby, lobbySkins, func(w *fakeriot.World) {
		w.StaticSkins[newPhantom] = fakeriot.StaticSkin{Name: "Nova Phantom", Edition: ultra}
		wear(w, ally1, fakeriot.Phantom, newPhantom)
	})
	r.writeSettings(skinColumns + `weapon = ["vandal", "phantom"]` + "\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "Reaver", "Nova", "US - THEM")
	if got := cellIn(t, frame, "PHANTOM", 3); got != "Nova" {
		t.Errorf("a phantom newer than the snapshot: got %q, want Nova\n%s", got, frame)
	}
	if got := cellIn(t, frame, "VANDAL", 3); got != "Elderflame" {
		t.Errorf("the same player's vandal: got %q, want Elderflame\n%s", got, frame)
	}
	if n := staticRequests(r, "/v1/weapons/skins/"+newPhantom); n != 1 {
		t.Errorf("asked valorant-api.com about the new phantom %d times, want once", n)
	}
}

func TestLoadoutsAreOnlyFetchedForTheSkinColumn(t *testing.T) {
	r := newRig(t, matchLobby, lobbySkins)
	r.writeSettings(`columns = ["name", "agent", "rank"]` + "\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "Plat2 10", "US - THEM")
	if n := requestsExact(r, loadouts); n != 0 {
		t.Errorf("loadouts fetched %d times with the skin column off", n)
	}
}

func TestEnemiesShownByAgentOnlyShowNoSkins(t *testing.T) {
	r := newRig(t, matchLobby, lobbySkins)
	r.writeSettings(skinColumns + "enemy_lookups = false\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "Reaver", "US - THEM")
	if got, want := skinColumn(t, frame, "VANDAL"), "Reaver,Elderflame,Standard,,Standard,,,,,"; got != want {
		t.Errorf("vandals: got %s, want %s\n%s", got, want, frame)
	}
}

func TestSkinsAreColoredByTheirEdition(t *testing.T) {
	r := newRig(t, matchLobby, lobbySkins, func(w *fakeriot.World) {
		w.StaticSkins[newSkin] = fakeriot.StaticSkin{Name: "Nova Vandal", Edition: ultra}
		wear(w, ally2, fakeriot.Vandal, newSkin)
	})
	r.env.Console.Terminal = true
	r.writeSettings(skinColumns)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "Reaver", "Nova", "US - THEM")
	// premium is d1548d and ultra fad663 - a skin a refresh named gets its edition's color too
	for _, want := range []string{"\x1b[38;2;209;84;141mReaver\x1b[0m", "\x1b[38;2;250;214;99mElderflame\x1b[0m", "\x1b[38;2;250;214;99mNova\x1b[0m"} {
		if !strings.Contains(frame, want) {
			t.Errorf("want %q in:\n%q", want, frame)
		}
	}
	if strings.Contains(frame, "mStandard") {
		t.Errorf("a standard skin has a color:\n%q", frame)
	}
}

func TestASkinNewerThanTheSnapshotIsNamedByOneRefresh(t *testing.T) {
	r := newRig(t, matchLobby, lobbySkins, func(w *fakeriot.World) {
		w.StaticSkins[newSkin] = fakeriot.StaticSkin{Name: "Nova Vandal", Edition: ultra}
		wear(w, ally1, fakeriot.Vandal, newSkin)
		wear(w, enemy3, fakeriot.Vandal, ghostSkin)
	})
	r.fr.AddFault(fakeriot.Fault{Service: "static", Path: "/v1/weapons/skins/", Hold: true})
	r.writeSettings(skinColumns)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	waitHeld(t, r.fr, 1)
	frame := r.screen("MATCH LOADING", "Reaver")
	if got := cellIn(t, frame, "VANDAL", 3); got != "?" {
		t.Errorf("a skin the snapshot doesn't know: got %q, want ?\n%s", got, frame)
	}
	r.fr.ReleaseHeld()
	frame = r.settled("MATCH LOADING", "Nova", "US - THEM")
	if got := cellIn(t, frame, "VANDAL", 3); got != "Nova" {
		t.Errorf("after the refresh: got %q, want Nova\n%s", got, frame)
	}
	if n := staticRequests(r, "/v1/weapons/skins/"+newSkin); n != 1 {
		t.Errorf("asked valorant-api.com about the new skin %d times, want once", n)
	}
	if n := staticRequests(r, "/v1/weapons/skins/"+ghostSkin); n != 0 {
		t.Errorf("asked about a streamer-mode player's skin %d times", n)
	}
}

// Riot has moved the player out of the loadout and next to it - lowkey reads either
func TestSkinsReadThePlayerBesideTheLoadoutToo(t *testing.T) {
	const socket = "bcef87d6-209b-46c6-8b19-fbe40bd95abc"
	r := newRig(t, matchLobby, func(w *fakeriot.World) {
		w.Raw["glz "+loadouts] = fakeriot.RawResponse{Status: 200, Body: []byte(fmt.Sprintf(
			`{"Loadouts":[{"Subject":%q,"CharacterID":%q,"Loadout":{"Items":{%q:{"ID":%[3]q,"Sockets":{%[4]q:{"ID":%[4]q,"Item":{"ID":%[5]q,"TypeID":%[4]q}}}}}}}]}`,
			strings.ToUpper(w.Self), fakeriot.Sova, fakeriot.Vandal, socket, strings.ToUpper(fakeriot.ReaverVandal)))}
	})
	r.writeSettings(skinColumns)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "Reaver", "US - THEM")
	if got := cellIn(t, frame, "VANDAL", 2); got != "Reaver" {
		t.Errorf("your vandal: got %q, want Reaver\n%s", got, frame)
	}
}
