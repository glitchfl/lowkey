package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// Riot keeps sending lifecycle events about a match after it ends - one about half a minute later
// and the real end of a match you left early - none of them may start a match load

func TestALateEventAboutTheLastMatchStartsNoMatchLoad(t *testing.T) {
	r := newRig(t, matchLobby)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopIngame })
	r.settled("MATCH LOADING", "Plat2 10")
	r.clock.Advance(40 * time.Second)
	r.screen("IN MATCH")

	// the end of the match comes while your presence still says in-game
	r.fr.EmitCoreGame(coreGame, false)
	r.fr.Update(func(w *fakeriot.World) { w.CoreGame = nil })
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("LAST MATCH · Ascent · Competitive", "waiting for the next match")

	r.fr.EmitCoreGame(coreGame, false)
	settle()
	r.screen("LAST MATCH · Ascent · Competitive", "waiting for the next match")
	if n := requestsExact(r, "/core-game/v1/matches/"+coreGame); n != 1 {
		t.Errorf("the finished match was fetched %d times, want only at its match load", n)
	}
}

const endedMatch = "c9000000-0000-4000-8000-0000000000ee"

func endedMatchServed(w *fakeriot.World) {
	w.Raw["glz /core-game/v1/matches/"+endedMatch] = fakeriot.RawResponse{Status: 200, Body: []byte(
		`{"MatchID":"` + endedMatch + `","State":"CLOSED","MapID":"/Game/Maps/Jam/Jam","Players":[],"MatchmakingData":{"QueueID":"abilitydraftarena"}}`)}
}

func TestAnEventAboutAnEndedMatchLeavesAgentSelectAlone(t *testing.T) {
	r := fromAgentSelect(t, endedMatchServed)
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopPregame })
	r.fr.EmitCoreGame(endedMatch, false)
	eventually(t, "the ended match checked", func() bool { return requestsExact(r, "/core-game/v1/matches/"+endedMatch) == 1 })
	r.settled("AGENT SELECT", "Dia1 50", "Plat1 30")
	r.clock.Advance(time.Minute)
	settle()
	r.screen("AGENT SELECT")

	r.enterMatchLoad()
	r.settled("MATCH LOADING", "Plat2 10", "US - THEM")
}

func TestAnEventAboutAMatchYouLeftIsDropped(t *testing.T) {
	const left = "c9000000-0000-4000-8000-0000000000ff"
	r := newRig(t, matchLobby, func(w *fakeriot.World) {
		w.Raw["glz /core-game/v1/matches/"+left] = fakeriot.RawResponse{Status: 200, Body: []byte(
			`{"MatchID":"` + left + `","State":"IN_PROGRESS","MapID":"/Game/Maps/Jam/Jam","Players":[{"Subject":"` + enemy1 +
				`","TeamID":"Blue","CharacterID":"` + fakeriot.Raze + `","PlayerIdentity":{"Incognito":false}}],"MatchmakingData":{"QueueID":"competitive"}}`)}
	})
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.fr.EmitCoreGame(left, false)
	eventually(t, "the match you left checked", func() bool { return requestsExact(r, "/core-game/v1/matches/"+left) == 1 })
	r.clock.Advance(time.Minute)
	settle()
	frame := r.screen("waiting for the next match")
	if strings.Contains(frame, "MATCH LOADING") || strings.Contains(frame, "IN MATCH") {
		t.Fatalf("a match you left started a match load:\n%s", frame)
	}
	if n := requestsExact(r, "/mmr/v1/players/"+enemy1); n != 0 {
		t.Errorf("a player from the match you left was looked up %d times", n)
	}

	r.enterMatchLoad()
	r.settled("MATCH LOADING", "Plat2 10", "US - THEM")
}

func TestAnEventAboutAnUnknownEndedMatchIsDroppedInTheMenus(t *testing.T) {
	r := newRig(t, matchLobby, endedMatchServed)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.fr.EmitCoreGame(endedMatch, false)
	eventually(t, "the ended match checked", func() bool { return requestsExact(r, "/core-game/v1/matches/"+endedMatch) == 1 })
	r.clock.Advance(time.Minute)
	settle()
	frame := r.screen("waiting for the next match")
	if strings.Contains(frame, "MATCH LOADING") || strings.Contains(frame, "IN MATCH") {
		t.Fatalf("an ended match started a match load:\n%s", frame)
	}

	r.fr.EmitCoreGame(endedMatch, false)
	settle()
	if n := requestsExact(r, "/core-game/v1/matches/"+endedMatch); n != 1 {
		t.Errorf("the ended match was fetched %d times, want once", n)
	}
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "Plat2 10", "US - THEM")
}
