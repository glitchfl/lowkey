package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

func TestFullFlow(t *testing.T) {
	fr := startRiot(t, fakeriot.LobbyWorld(time.Now(), false))
	lk := startLowkey(t, fr, fastSettings)
	lk.screen("waiting for the next match", "SESSION")
	subscribed(t, fr)

	fr.StartAgentSelect(fakeriot.LobbyPregame)
	lk.screen("AGENT SELECT · Ascent · Defenders · Competitive", "Dia3 12", "AVERAGE")
	fr.Hover(fakeriot.LobbyAllies[0], fakeriot.Breach, true)
	lk.screen("AGENT SELECT", "Breach")

	fr.LoadMatch(fakeriot.LobbyCoreGame)
	frame := lk.screen("MATCH LOADING · Ascent · Defenders · Competitive", "ALLIES", "ENEMIES", "Player5#AA", "US - THEM", "Imm3 20", "Rad 60")
	if strings.Count(frame, "hidden") < 2 {
		t.Errorf("both streamer-mode players should show as hidden:\n%s", frame)
	}
	lk.screen("IN MATCH")
	cut := len(fr.RemoteRequests())
	time.Sleep(3 * time.Second)
	if n := len(fr.RemoteRequests()); n != cut {
		t.Errorf("%d requests after the cutoff:\n%s", n-cut, describe(fr))
	}

	fr.EndMatch()
	lk.screen("LAST MATCH · Ascent · Defenders · Competitive", "waiting for the next match")
}

func TestHiddenPlayerStatsSwitchedOff(t *testing.T) {
	fr := startRiot(t, fakeriot.LobbyWorld(time.Now(), false))
	lk := startLowkey(t, fr, fastSettings+"hidden_player_stats = false\n")
	lk.screen("waiting for the next match", "SESSION")
	subscribed(t, fr)
	fr.StartAgentSelect(fakeriot.LobbyPregame)
	lk.screen("AGENT SELECT", "Dia3 12", "AVERAGE")
	fr.LoadMatch(fakeriot.LobbyCoreGame)
	frame := lk.screen("MATCH LOADING", "ENEMIES", "Player5#AA", "US - THEM")
	if strings.Contains(frame, "Imm3 20") || strings.Contains(frame, "Rad 60") {
		t.Errorf("a hidden player's rank showed with the setting off:\n%s", frame)
	}
	lk.screen("IN MATCH")
	for _, req := range fr.HiddenLookups() {
		t.Errorf("asked about a hidden player with the setting off: %s", req)
	}
}

func TestDodge(t *testing.T) {
	fr := startRiot(t, fakeriot.LobbyWorld(time.Now(), false))
	lk := startLowkey(t, fr, fastSettings)
	lk.screen("waiting for the next match", "SESSION")
	subscribed(t, fr)
	fr.StartAgentSelect(fakeriot.LobbyPregame)
	lk.screen("AGENT SELECT")
	fr.Dodge()
	frame := lk.screen("waiting for the next match")
	if strings.Contains(frame, "AGENT SELECT") {
		t.Errorf("agent select lingered after the dodge:\n%s", frame)
	}
}

func TestTheCutoffCancelsPendingLookups(t *testing.T) {
	fr := startRiot(t, fakeriot.LobbyWorld(time.Now(), false))
	fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/" + fakeriot.LobbyEnemies[0], Hold: true})
	fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/" + fakeriot.LobbyEnemies[1], Hold: true})
	lk := startLowkey(t, fr, fastSettings)
	lk.screen("waiting for the next match", "SESSION")
	subscribed(t, fr)
	fr.LoadMatch(fakeriot.LobbyCoreGame)
	lk.screen("MATCH LOADING")
	lk.screen("IN MATCH")
	deadline := time.Now().Add(10 * time.Second)
	for len(fr.Held()) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("held lookups weren't cancelled at the cutoff")
		}
		time.Sleep(20 * time.Millisecond)
	}
	before := len(fr.RemoteRequests())
	fr.ReleaseHeld()
	time.Sleep(3 * time.Second)
	if n := len(fr.RemoteRequests()); n != before {
		t.Errorf("%d requests after the cutoff", n-before)
	}
}

func TestDeathmatch(t *testing.T) {
	fr := startRiot(t, fakeriot.DeathmatchWorld(time.Now()))
	lk := startLowkey(t, fr, fastSettings)
	lk.screen("waiting for the next match")
	subscribed(t, fr)
	fr.LoadMatch(fakeriot.DMCoreGame)
	frame := lk.screen("MATCH LOADING · Bind · Deathmatch", "AVERAGE", "hidden")
	for _, word := range []string{"ALLIES", "ENEMIES", "US - THEM"} {
		if strings.Contains(frame, word) {
			t.Errorf("deathmatch shows %q:\n%s", word, frame)
		}
	}
}

func TestRateLimiting(t *testing.T) {
	fr := startRiot(t, fakeriot.LobbyWorld(time.Now(), false))
	path := "/mmr/v1/players/" + fakeriot.LobbyAllies[0]
	fr.AddFault(fakeriot.Fault{Service: "pd", Path: path, Exact: true, Status: 429, Header: map[string]string{"Retry-After": "1"},
		Body: `{"httpStatus":429,"errorCode":"RATE_LIMITED"}`})
	lk := startLowkey(t, fr, fastSettings)
	lk.screen("waiting for the next match", "SESSION")
	subscribed(t, fr)
	fr.StartAgentSelect(fakeriot.LobbyPregame)
	lk.screen("Riot is rate-limiting")
	lk.screen("Riot is rate-limiting - some stats are missing")
	if n := fr.Answered(path, 429); n != 4 {
		t.Errorf("tried %d times, want 4", n)
	}
}

func TestRiotClientRestart(t *testing.T) {
	fr := startRiot(t, fakeriot.LobbyWorld(time.Now(), false))
	lk := startLowkey(t, fr, fastSettings)
	lk.screen("waiting for the next match")
	subscribed(t, fr)
	fr.StopClient()
	lk.screen("waiting for the Riot Client")
	if err := fr.StartClient(); err != nil {
		t.Fatal(err)
	}
	lk.screen("waiting for the next match")
	subscribed(t, fr)
	fr.StartAgentSelect(fakeriot.LobbyPregame)
	lk.screen("AGENT SELECT · Ascent · Defenders · Competitive", "Dia3 12")
}
