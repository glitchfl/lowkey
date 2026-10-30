package app_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/app"
	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const hiddenStatsOff = "hidden_player_stats = false\n"

// hiddenSamples gives the hidden ally and enemy a record - their match details name them like Riot's used to
func hiddenSamples(w *fakeriot.World) {
	playedMatches(w, ally3, "a3a3a3a3", 2, fakeriot.Omen, line5{score: 4000, rounds: 20, kills: 30, deaths: 10, head: 5, body: 15, damage: 3000, firstBloods: 4})
	playedMatches(w, enemy3, "e3e3e3e3", 1, fakeriot.Viper, line5{score: 3000, rounds: 20, kills: 10, deaths: 10, head: 1, body: 9, damage: 2000})
}

func TestHiddenPlayersShowTheirStatsButNeverTheirName(t *testing.T) {
	r := fromAgentSelect(t, lobbySamples, lobbySkins, hiddenSamples)
	frame := r.settled("AGENT SELECT", "AllyOne#A1", "3.00")
	if got := cellIn(t, frame, "PLAYER", 4); got != "hidden" {
		t.Errorf("agent select names the hidden ally %q\n%s", got, frame)
	}

	r.enterMatchLoad()
	frame = r.settled("MATCH LOADING", "EnemyTwo#E2", "VANDAL", "US - THEM")
	rows := []struct {
		line                                                    int
		name, agent, rank, kd, hs, acs, adr, fb, n, mains, skin string
	}{
		{5, "hidden", "Omen", "Rad 900", "3.00", "25%", "200", "150", "20%", "2", "Omen", "Prime"},
		{11, "hidden", "Viper", "Rad 999", "1.00", "10%", "150", "100", "0%", "1", "Vipr", "Elderflame"},
	}
	for _, w := range rows {
		got := [...]string{cellIn(t, frame, "PLAYER", w.line), cellIn(t, frame, "AGENT", w.line), cellIn(t, frame, "RANK", w.line),
			cellIn(t, frame, "K/D", w.line), cellIn(t, frame, "HS%", w.line), cellIn(t, frame, "ACS", w.line), cellIn(t, frame, "ADR", w.line),
			cellIn(t, frame, "FB%", w.line), cellIn(t, frame, "N", w.line), cellIn(t, frame, "MAINS", w.line), cellIn(t, frame, "VANDAL", w.line)}
		want := [...]string{w.name, w.agent, w.rank, w.kd, w.hs, w.acs, w.adr, w.fb, w.n, w.mains, w.skin}
		if got != want {
			t.Errorf("line %d: got %v, want %v\n%s", w.line, got, want, frame)
		}
	}
	for _, line := range []int{7, 14} {
		if got := cellIn(t, frame, "PLAYER", line); got != "AVERAGE" {
			t.Errorf("the hidden players should count in the averages, line %d: %q\n%s", line, got, frame)
		}
	}
}

// recordings swap real names for fakes on their way to the screen - a hidden player gets neither
func TestLoadingHiddenPlayersStayHiddenAndSoDoesTheirRecording(t *testing.T) {
	r := newRig(t, matchLobby, lobbySamples, hiddenSamples)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/" + ally3, Exact: true, Hold: true})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- app.Run(ctx, r.env, []string{"--record"}) }()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.settledWith(1, "AGENT SELECT", "AllyOne#A1", "3.00")
	if got := cellIn(t, frame, "PLAYER", 4); got != "hidden" {
		t.Errorf("the hidden ally shows %q while their rank loads\n%s", got, frame)
	}
	r.fr.ReleaseHeld()
	r.settled("AGENT SELECT", "Rad 900")
	cancel()
	<-done

	raw, err := os.ReadFile(filepath.Join(recordingDir(t, r.dir), "screens.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if screens := string(raw); !regexp.MustCompile(`hidden\s+Omen\s+Rad 900`).MatchString(screens) {
		t.Errorf("screens.txt should keep the name hidden next to the stats:\n%s", screens)
	}
	// this run isn't r.live() so the rig's own check doesn't cover it
	for _, v := range r.fr.NameViolations(r.con.timedFrames()) {
		t.Error(v)
	}
}

func TestHiddenPlayersNeverGetAPartyMarkerOrMetBefore(t *testing.T) {
	const metHidden = "cccc0000-0000-4000-8000-000000000009"
	r := fromAgentSelect(t, hiddenSamples, likelyParties, func(w *fakeriot.World) {
		w.PartyMembers = []string{w.Self, ally1, ally3}
		versus(w, metHidden, 3*time.Hour, ally3, true, true)
	})
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "US - THEM", "Plat2 10", "Rad 999")
	if requestsExact(r, "/match-details/v1/matches/"+metHidden) == 0 {
		t.Fatal("your match with the hidden ally should have been in your sample")
	}
	if got := cellIn(t, frame, "MET", 5); got != "" {
		t.Errorf("met-before ties the hidden ally to an earlier game: %q\n%s", got, frame)
	}
	if got := marker(t, frame, 5); got != "" {
		t.Errorf("the hidden ally in your party has a marker: %q\n%s", got, frame)
	}
	if m11, m13 := marker(t, frame, 11), marker(t, frame, 13); m11 != "" || m13 != "" {
		t.Errorf("sharing matches ties the hidden enemy to enemy five: %q and %q\n%s", m11, m13, frame)
	}
}

func TestHiddenPlayerStatsCanBeSwitchedOff(t *testing.T) {
	r := newRig(t, matchLobby, lobbySamples, lobbySkins, hiddenSamples)
	r.writeSettings(hiddenStatsOff)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "AllyOne#A1")
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "EnemyTwo#E2", "VANDAL", "US - THEM")
	for _, req := range r.fr.HiddenLookups() {
		t.Errorf("asked about a hidden player with the setting off: %s", req)
	}
	for _, line := range []int{5, 11} {
		for _, col := range []string{"RANK", "K/D", "N", "MAINS", "VANDAL"} {
			if got := cellIn(t, frame, col, line); got != "" {
				t.Errorf("line %d %s should be blank with the setting off, got %q\n%s", line, col, got, frame)
			}
		}
		if got := cellIn(t, frame, "PLAYER", line); got != "hidden" {
			t.Errorf("line %d: got %q, want hidden\n%s", line, got, frame)
		}
	}
	if got := cellIn(t, frame, "AGENT", 11); got != "Viper" {
		t.Errorf("the hidden enemy's agent: got %q", got)
	}
	for _, line := range []int{7, 14} {
		if got := cellIn(t, frame, "PLAYER", line); got != "AVERAGE 4/5" {
			t.Errorf("line %d: got %q, want AVERAGE 4/5", line, got)
		}
	}
}

func TestEnemyLookupsOffAlsoKeepsHiddenEnemiesToTheirAgent(t *testing.T) {
	r := newRig(t, matchLobby, hiddenSamples)
	r.writeSettings("enemy_lookups = false\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "Rad 900", "ENEMIES", "Viper")
	if n := requestsFor(r, "/mmr/v1/players/"+enemy3); n != 0 {
		t.Errorf("looked up the hidden enemy %d times with enemy lookups off", n)
	}
	if got := cellIn(t, frame, "RANK", 11); got != "" {
		t.Errorf("the hidden enemy shows rank %q\n%s", got, frame)
	}
}

// the guard remembers who it refused - switching a setting back on has to free them for the next lobby
func TestSwitchingTheSettingsBackOnTakesEffectInTheNextLobby(t *testing.T) {
	for _, c := range []struct{ name, off, on, player, want string }{
		{"hidden player stats", hiddenStatsOff, "hidden_player_stats = true\n", ally3, "Rad 900"},
		{"enemy lookups", "enemy_lookups = false\n", "enemy_lookups = true\n", enemy1, "Plat2 10"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, matchLobby)
			r.writeSettings(c.off)
			r.live()
			r.screen("waiting for the next match")
			r.subscribed()
			r.enterMatchLoad()
			r.settled("MATCH LOADING", "ENEMIES", "Dia3 12")
			if n := requestsFor(r, "/mmr/v1/players/"+c.player); n != 0 {
				t.Fatalf("asked about %s %d times with the setting off", c.player, n)
			}
			r.clock.Advance(40 * time.Second)
			r.screen("IN MATCH")
			r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
			r.screen("LAST MATCH")
			r.clock.Advance(15 * time.Second)
			r.settled("LAST MATCH")
			r.writeSettings(c.on)
			settle()
			const next = "c9000000-0000-4000-8000-000000000077"
			r.fr.Update(func(w *fakeriot.World) { w.CoreGame.ID, w.Loop = next, fakeriot.LoopIngame })
			r.fr.EmitCoreGame(next, false)
			r.settled("MATCH LOADING", "ENEMIES", c.want)
			if _, err := os.Stat(filepath.Join(r.dir, "lowkey.log")); err == nil {
				if log := errorLog(t, r); strings.Contains(log, "refused") {
					t.Errorf("the guard kept refusing after the setting came back on:\n%s", log)
				}
			}
		})
	}
}

func TestAPlayerGoingHiddenBetweenRostersLosesTheirName(t *testing.T) {
	r := fromAgentSelect(t, lobbySamples)
	r.settled("AGENT SELECT", "AllyOne#A1")
	r.fr.Update(func(w *fakeriot.World) { w.Player(ally1).Incognito = true })
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "EnemyTwo#E2", "US - THEM")
	if got := cellIn(t, frame, "PLAYER", 3); got != "hidden" {
		t.Errorf("ally one went hidden and shows %q\n%s", got, frame)
	}
	// the rig would rightly flag agent select's frames - the name was fine to show back then
	r.fr.Update(func(w *fakeriot.World) { w.Player(ally1).Incognito = false })
}
