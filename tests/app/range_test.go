package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// you can queue from the Range - your presence says in-game there but lowkey treats it as the menus so
// the match you queue for is picked up straight from it without passing the menus

const rangeMatch = "c9000000-0000-4000-8000-0000000000a0"

// rangeServed answers for the Range's core-game match the way Riot does - you alone and no matchmaking
func rangeServed(w *fakeriot.World) {
	w.Raw["glz /core-game/v1/matches/"+rangeMatch] = fakeriot.RawResponse{Status: 200, Body: []byte(
		`{"MatchID":"` + rangeMatch + `","State":"IN_PROGRESS","ProvisioningFlow":"ShootingRange","IsReconnectable":false,` +
			`"MapID":"/Game/Maps/PovegliaV2/RangeV2","ModeID":"/Game/GameModes/ShootingRange/ShootingRangeGameMode.ShootingRangeGameMode_C",` +
			`"Players":[{"Subject":"` + w.Self + `","TeamID":"Blue","CharacterID":"` + fakeriot.Phoenix + `","PlayerIdentity":{"Incognito":true}}],` +
			`"MatchmakingData":null}`)}
}

func inTheRange(w *fakeriot.World) { w.Loop, w.Flow = fakeriot.LoopIngame, fakeriot.FlowRange }

// enterTheRange loads the Range like Riot does - its core-game events come seconds before your presence
func (r *rig) enterTheRange() {
	r.t.Helper()
	r.leftoverInMenus(rangeMatch)
	r.presence(inTheRange)
	settle()
	r.screen("waiting for the next match")
}

// noRangeLookups checks the Range was only fetched once to tell it apart from a match - call it
// before the next match looks anyone up
func noRangeLookups(t *testing.T, r *rig) {
	t.Helper()
	rangeFetchedOnce(t, r)
	if n := requestsExact(r, "/mmr/v1/players/"+fakeriot.DefaultSelf); n != 0 {
		t.Errorf("your rank was looked up %d times for the Range", n)
	}
	if n := requestsFor(r, "/match-history/"); n != 0 {
		t.Errorf("match history was fetched %d times for the Range", n)
	}
}

func rangeFetchedOnce(t *testing.T, r *rig) {
	t.Helper()
	if n := requestsExact(r, "/core-game/v1/matches/"+rangeMatch); n != 1 {
		t.Errorf("the Range's match was fetched %d times, want once", n)
	}
}

func TestTheRangeCountsAsTheMenus(t *testing.T) {
	r := newRig(t, matchLobby, rangeServed)
	r.live()
	r.screen("waiting for the next match", "SESSION")
	r.subscribed()
	r.enterTheRange()

	// Riot keeps sending events about the Range while you're in it
	r.fr.EmitCoreGame(rangeMatch, false)
	r.clock.Advance(time.Minute)
	settle()
	frame := r.screen("waiting for the next match", "SESSION")
	if strings.Contains(frame, "The Range") || strings.Contains(frame, "IN MATCH") {
		t.Fatalf("the Range shows as a match:\n%s", frame)
	}

	r.presence(func(w *fakeriot.World) { w.Loop, w.Flow = fakeriot.LoopMenus, "" })
	r.fr.EmitCoreGame(rangeMatch, false)
	settle()
	if frame := r.screen("waiting for the next match"); strings.Contains(frame, "LAST MATCH") {
		t.Fatalf("the Range became the last match:\n%s", frame)
	}
	noRangeLookups(t, r)
}

func TestAMatchQueuedFromTheRangeIsTracked(t *testing.T) {
	r := newRig(t, matchLobby, rangeServed)
	r.live()
	r.screen("waiting for the next match", "SESSION")
	r.subscribed()
	r.enterTheRange()

	// the order Riot sent on 2026-10-30 - the queue starts and pops while your presence still says Range
	// and agent select's event comes before your presence says pregame - the Range ends after that
	r.presence(func(w *fakeriot.World) { w.Queue = "competitive" })
	r.fr.EmitCoreGame(rangeMatch, false)
	settle()
	noRangeLookups(t, r)
	r.fr.EmitPregame(pregame, false)
	r.screen("AGENT SELECT")
	r.presence(func(w *fakeriot.World) { w.Loop, w.Flow = fakeriot.LoopPregame, "Invalid" })
	r.settled("AGENT SELECT · Ascent · Competitive", "Dia1 50", "Plat1 30")
	r.presence(func(w *fakeriot.World) { w.Flow = "Matchmaking" })
	r.fr.EmitCoreGame(rangeMatch, false)
	settle()
	r.screen("AGENT SELECT · Ascent · Competitive")

	r.enterMatchLoad()
	r.presence(func(w *fakeriot.World) {})
	r.settled("MATCH LOADING · Ascent · Competitive", "Plat2 10", "US - THEM")
	r.clock.Advance(40 * time.Second)
	r.screen("IN MATCH · Ascent · Competitive")

	r.fr.Update(func(w *fakeriot.World) { w.CoreGame = nil })
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("LAST MATCH · Ascent · Competitive", "waiting for the next match")
	rangeFetchedOnce(t, r)
}

func TestOpeningLowkeyInTheRangeWaitsForTheNextMatch(t *testing.T) {
	r := newRig(t, matchLobby, rangeServed, inTheRange)
	r.live()
	frame := r.screen("waiting for the next match", "SESSION")
	if strings.Contains(frame, "match in progress") {
		t.Fatalf("the Range shows as a match in progress:\n%s", frame)
	}
	r.subscribed()

	// lowkey never saw the Range load so its next event is checked once
	r.leftoverInMenus(rangeMatch)
	r.fr.EmitCoreGame(rangeMatch, false)
	settle()
	r.screen("waiting for the next match")
	noRangeLookups(t, r)
	r.enterAgentSelect()
	r.settled("AGENT SELECT · Ascent · Competitive", "Dia1 50", "Plat1 30")
}

func TestDoctorNamesTheRange(t *testing.T) {
	r := newRig(t, inTheRange)
	out, _ := r.run("doctor")
	if l := line(t, out, "phase"); !strings.HasSuffix(l, "In the Range (nested presence)") {
		t.Errorf("phase line %q, want In the Range", l)
	}
}
