package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

func TestStatusLineFollowsTheRiotClientAndValorantStartingUp(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) { w.RiotClient, w.Valorant = false, false })
	r.live()
	r.screen("waiting for the Riot Client")

	if err := r.fr.StartClient(); err != nil {
		t.Fatal(err)
	}
	r.screen("waiting for Valorant")
	r.subscribed()

	r.presence(func(w *fakeriot.World) { w.Valorant, w.Loop = true, fakeriot.LoopMenus })
	r.screen("waiting for the next match")
}

func TestLowkeyReconnectsWhenTheRiotClientRestarts(t *testing.T) {
	r := newRig(t)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()

	r.fr.StopClient()
	r.screen("waiting for the Riot Client")
	if err := r.fr.StartClient(); err != nil {
		t.Fatal(err)
	}
	r.screen("waiting for the next match")
	r.subscribed()

	sockets := 0
	for _, req := range r.fr.Requests() {
		if req.Service == "local" && req.Path == "/" {
			sockets++
		}
	}
	if sockets != 2 {
		t.Errorf("lowkey opened %d WebSocket connections, want one per Riot Client run", sockets)
	}
}

func TestOpenedMidMatchWaitsForTheMenusWithoutRequests(t *testing.T) {
	r := newRig(t, ranked, func(w *fakeriot.World) { w.Loop = fakeriot.LoopIngame })
	r.live()
	r.screen("match in progress")
	r.subscribed()
	r.clock.Advance(10 * time.Minute)
	settle()
	if reqs := r.fr.RemoteRequests(); len(reqs) != 0 {
		t.Fatalf("lowkey made %d Riot server requests mid-match: %+v", len(reqs), reqs[0])
	}

	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("waiting for the next match")
}

func TestEveryPresenceShapeDrivesPhases(t *testing.T) {
	shapes := map[string]fakeriot.Shape{"nested": fakeriot.Nested, "flat": fakeriot.Flat, "hybrid": fakeriot.Hybrid}
	for name, shape := range shapes {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, func(w *fakeriot.World) { w.Shape, w.Loop = shape, fakeriot.LoopIngame })
			r.live()
			r.screen("match in progress")
			r.subscribed()
			r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
			r.screen("waiting for the next match")
			r.presence(func(w *fakeriot.World) { w.Valorant = false })
			r.screen("waiting for Valorant")
		})
	}
}

func TestWaitingIsQuietAndTheLockfileIsNoticedWithoutPolling(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) { w.RiotClient, w.Valorant = false, false })
	r.live()
	r.screen("waiting for the Riot Client")
	settle()
	if n := r.clock.Pending(); n != 0 {
		t.Errorf("lowkey keeps %d timers while waiting for the Riot Client", n)
	}

	// the clock stays put - only a filesystem notification can tell lowkey about the lockfile
	if err := r.fr.StartClient(); err != nil {
		t.Fatal(err)
	}
	r.screen("waiting for Valorant")
	r.clock.Advance(30 * time.Minute)
	settle()
	if reqs := r.fr.RemoteRequests(); len(reqs) != 0 {
		t.Errorf("lowkey made %d Riot server requests while waiting", len(reqs))
	}
}

func TestEachPhaseChangeRedrawsTheScreenInPlace(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) { w.Valorant = false })
	r.env.Console.Terminal = true
	r.live()
	r.screen("waiting for Valorant")
	r.subscribed()
	r.presence(func(w *fakeriot.World) { w.Valorant = true })
	r.screen("waiting for the next match")

	frames := r.con.framesCopy()
	if len(frames) < 2 {
		t.Fatalf("got %d frames, want one per phase", len(frames))
	}
	for _, f := range frames {
		if !strings.HasPrefix(f, "\x1b[H") || !strings.HasSuffix(f, "\x1b[J") {
			t.Errorf("frame isn't drawn in place: %q", f)
		}
	}
}
