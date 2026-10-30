package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const (
	newAgent   = "12345678-1234-4123-8123-123456789abc"
	ghostAgent = "87654321-4321-4321-8321-cba987654321"
	newMap     = "/Game/Maps/Orbit/Orbit"
)

func staticRequests(r *rig, path string) int {
	n := 0
	for _, req := range r.fr.RemoteRequests() {
		if req.Service == "static" && req.Path == path {
			n++
		}
	}
	return n
}

func TestAnUnknownAgentShowsAPlaceholderUntilOneRefresh(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) {
		w.StaticAgents[newAgent] = "Nova"
		w.Pregame.Players[0].Agent, w.Pregame.Players[0].State = newAgent, "locked"
	})
	r.fr.AddFault(fakeriot.Fault{Service: "static", Path: "/v1/agents", Hold: true})
	r.live()
	r.settled("waiting for the next match", "SESSION")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.screen("AGENT SELECT", "Dia1 50")
	if got := cellIn(t, frame, "AGENT", 2); got != "?" {
		t.Errorf("unknown agent before the refresh: got %q, want ?", got)
	}
	waitHeld(t, r.fr, 1)
	r.fr.ReleaseHeld()
	frame = r.settled("AGENT SELECT", "Nova")
	if got := cellIn(t, frame, "AGENT", 2); got != "Nova" {
		t.Errorf("after the refresh: got %q, want Nova", got)
	}

	// more redraws and roster refreshes never ask again
	r.fr.EmitPregame(pregame, false)
	r.clock.Advance(3 * time.Second)
	r.settled("AGENT SELECT", "Nova")
	if n := staticRequests(r, "/v1/agents"); n != 1 {
		t.Errorf("asked valorant-api.com for agents %d times, want once", n)
	}
	if n := staticRequests(r, "/v1/maps"); n != 0 {
		t.Errorf("asked for maps %d times although every map was known", n)
	}
}

func TestAnIDValorantAPIDoesntKnowEitherIsOnlyAskedAboutOnce(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) {
		w.Pregame.Players[0].Agent, w.Pregame.Players[0].State = ghostAgent, "locked"
		w.Pregame.Map = newMap
		w.StaticMaps[newMap] = "Orbit"
	})
	r.live()
	r.settled("waiting for the next match", "SESSION")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT · Orbit", "Dia1 50")
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("waiting for the next match")
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT · Orbit", "Dia1 50")
	if got := cellIn(t, frame, "AGENT", 2); got != "?" {
		t.Errorf("an agent nobody knows: got %q, want ?", got)
	}
	if n := staticRequests(r, "/v1/agents"); n != 1 {
		t.Errorf("asked about the unknown agent %d times, want once per run", n)
	}
	if n := staticRequests(r, "/v1/maps"); n != 1 {
		t.Errorf("asked about the new map %d times, want once", n)
	}
}

func TestStaticRefreshesOnlyHappenInsideAWindow(t *testing.T) {
	r := fromAgentSelect(t, func(w *fakeriot.World) {
		w.StaticAgents[newAgent] = "Nova"
		w.CoreGame.Players[0].Agent = newAgent
	})
	r.fr.AddFault(fakeriot.Fault{Service: "static", Path: "/v1/agents", Hold: true})
	r.enterMatchLoad()
	r.screen("MATCH LOADING", "?")
	waitHeld(t, r.fr, 1)
	r.clock.Advance(40 * time.Second)
	r.screen("IN MATCH")
	eventually(t, "the refresh to be cancelled at the cutoff", func() bool { return len(r.fr.Held()) == 0 })
	r.fr.ReleaseHeld()
	r.presence(func(w *fakeriot.World) { w.Loop, w.CoreGame = fakeriot.LoopMenus, nil })
	r.clock.Advance(20 * time.Second)
	frame := r.settled("LAST MATCH")
	if !strings.Contains(frame, "?") {
		t.Errorf("the unknown agent should stay a placeholder in the menus:\n%s", frame)
	}
	if n := staticRequests(r, "/v1/agents"); n != 1 {
		t.Errorf("asked for agents %d times - only the cancelled one inside match load was allowed", n)
	}
}

func TestARefreshCutShortByADodgeGoesAgainInTheNextLobby(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) {
		w.StaticAgents[newAgent] = "Nova"
		w.Pregame.Players[0].Agent, w.Pregame.Players[0].State = newAgent, "locked"
	})
	r.fr.AddFault(fakeriot.Fault{Service: "static", Path: "/v1/agents", Hold: true, Times: 1})
	r.live()
	r.settled("waiting for the next match", "SESSION")
	r.subscribed()
	r.enterAgentSelect()
	waitHeld(t, r.fr, 1)
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("waiting for the next match")
	eventually(t, "the dodge to cancel the refresh", func() bool { return len(r.fr.Held()) == 0 })

	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "Nova")
	if got := cellIn(t, frame, "AGENT", 2); got != "Nova" {
		t.Errorf("the next lobby after a cancelled refresh: got %q, want Nova", got)
	}
	if n := staticRequests(r, "/v1/agents"); n != 2 {
		t.Errorf("asked for agents %d times, want the cancelled one and one more", n)
	}
}

func TestKnownIDsNeverTouchStaticData(t *testing.T) {
	r := fromAgentSelect(t, lobbySamples)
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "EnemyTwo#E2", "US - THEM")
	for _, req := range r.fr.RemoteRequests() {
		if req.Service == "static" {
			t.Errorf("a static data request with nothing unknown: %s", req.Path)
		}
	}
}

func TestAFailingStaticRefreshIsNotRetriedInALoop(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) {
		w.Pregame.Players[0].Agent, w.Pregame.Players[0].State = newAgent, "locked"
	})
	r.fr.AddFault(fakeriot.Fault{Service: "static", Path: "/v1/agents", Status: 500, Body: `{"status":500}`})
	r.live()
	r.settled("waiting for the next match", "SESSION")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "Dia1 50")
	if got := cellIn(t, frame, "AGENT", 2); got != "?" {
		t.Errorf("unknown agent after a failed refresh: got %q, want ?", got)
	}
	if n := staticRequests(r, "/v1/agents"); n != 1 {
		t.Errorf("asked valorant-api.com %d times after it failed, want once", n)
	}
}

func TestValorantAPIRateLimitsDontStopRiotLookups(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) { w.Pregame.Players[0].Agent = newAgent })
	r.fr.AddFault(fakeriot.Fault{Service: "static", Path: "/v1/agents", Status: 429, Body: `{"status":429}`})
	r.live()
	r.settled("waiting for the next match", "SESSION")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "Dia1 50", "Plat1 30")
	if strings.Contains(frame, "rate-limiting") {
		t.Errorf("a 429 from valorant-api.com isn't Riot rate-limiting:\n%s", frame)
	}
	if n := staticRequests(r, "/v1/agents"); n != 1 {
		t.Errorf("asked valorant-api.com %d times after a 429, want once", n)
	}
	if r.clock.Pending() != 0 {
		t.Errorf("a 429 from valorant-api.com left %d timers - Riot lookups must not pause for it", r.clock.Pending())
	}
}
