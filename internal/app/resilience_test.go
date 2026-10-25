package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const rateLimited = `{"httpStatus":429,"errorCode":"RATE_LIMITED","message":"slow down"}`

func inAgentSelect(t *testing.T, setup ...func(w *fakeriot.World)) *rig {
	r := newRig(t, append([]func(w *fakeriot.World){agentSelectLobby}, setup...)...)
	r.live()
	r.settled("waiting for the next match", "SESSION")
	r.subscribed()
	return r
}

func TestRateLimitsBackOffFiveTenTwentyThenGiveUp(t *testing.T) {
	cases := []struct {
		name   string
		header map[string]string
		waits  []time.Duration
	}{
		{"without Retry-After", nil, []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}},
		{"with Retry-After", map[string]string{"Retry-After": "7"}, []time.Duration{7 * time.Second, 7 * time.Second, 7 * time.Second}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := inAgentSelect(t)
			path := "/mmr/v1/players/" + ally2
			r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: path, Exact: true, Status: 429, Body: rateLimited, Header: c.header})
			r.enterAgentSelect()
			eventually(t, "the first 429", func() bool { return r.fr.Answered(path, 429) == 1 })
			r.screen("Riot is rate-limiting")
			for i, wait := range c.waits {
				r.clock.Advance(wait - 100*time.Millisecond)
				settle()
				if n := r.fr.Answered(path, 429); n != i+1 {
					t.Fatalf("retry %d came %v early (%d tries)", i+1, 100*time.Millisecond, n)
				}
				r.clock.Advance(100 * time.Millisecond)
				eventually(t, fmt.Sprintf("retry %d", i+1), func() bool { return r.fr.Answered(path, 429) == i+2 })
			}
			r.clock.Advance(5 * time.Minute)
			frame := r.settled("Riot is rate-limiting")
			if n := r.fr.Answered(path, 429); n != 4 {
				t.Errorf("tried %d times, want 4 then give up", n)
			}
			if got := cellIn(t, frame, "RANK", 3); got != "-" {
				t.Errorf("the rate-limited ally's rank: got %q, want -", got)
			}
			if got := cellIn(t, frame, "RANK", 2); got != "Dia1 50" {
				t.Errorf("everyone else should still fill in, ally one's rank: got %q", got)
			}
		})
	}
}

func TestTokensAreRefreshedBeforeTheyExpire(t *testing.T) {
	r := inAgentSelect(t, func(w *fakeriot.World) { w.TokenLifetime = 10 * time.Minute })
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("waiting for the next match")

	r.clock.Advance(9 * time.Minute)
	r.fr.Update(func(w *fakeriot.World) { w.Pregame.ID = "9e000000-0000-4000-8000-000000000002" })
	r.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopPregame })
	r.fr.EmitPregame("9e000000-0000-4000-8000-000000000002", false)
	frame := r.settled("AGENT SELECT", "Dia1 50")
	for _, req := range r.fr.RemoteRequests() {
		if req.Status == 400 || req.Status == 401 {
			t.Errorf("%s was answered %d - the token should have been refreshed first", req.Path, req.Status)
		}
	}
	if strings.Contains(frame, "can't") {
		t.Errorf("an error is visible:\n%s", frame)
	}
}

func TestRejectedTokensAreRefreshedOnceAndRetried(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"400 BAD_CLAIMS", 400, `{"httpStatus":400,"errorCode":"BAD_CLAIMS","message":"Failure validating/decoding RSO Access Token"}`},
		{"401", 401, `{"httpStatus":401,"errorCode":"BAD_CLAIMS","message":"unauthorized"}`},
		{"403", 403, `{"httpStatus":403,"errorCode":"FORBIDDEN","message":"forbidden"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := inAgentSelect(t)
			path := "/mmr/v1/players/" + ally1
			r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: path, Exact: true, Status: c.status, Body: c.body, Times: 1})
			before := 0
			for _, req := range r.fr.Requests() {
				if req.Path == "/entitlements/v1/token" {
					before++
				}
			}
			r.enterAgentSelect()
			r.settled("AGENT SELECT", "Dia1 50")
			if n := r.fr.Answered(path, c.status); n != 1 {
				t.Errorf("rejected %d times", n)
			}
			if n := r.fr.Answered(path, 200); n != 1 {
				t.Errorf("the retry was answered 200 %d times, want once", n)
			}
			after := 0
			for _, req := range r.fr.Requests() {
				if req.Path == "/entitlements/v1/token" {
					after++
				}
			}
			if after-before != 1 {
				t.Errorf("tokens refreshed %d times, want once", after-before)
			}
		})
	}
}

func TestLowkeyWaitsForTheTokenToBeReady(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) { w.TokenNotReady = 3 })
	r.live()
	r.screen("waiting for the Riot Client to log in")
	for i := 0; i < 3; i++ {
		r.clock.Advance(2 * time.Second)
		settle()
	}
	r.screen("waiting for the next match")
}

func TestARiotClientOnANewPortIsPickedUp(t *testing.T) {
	r := inAgentSelect(t)
	if err := r.fr.Restart(); err != nil {
		t.Fatal(err)
	}
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
}

func TestUnreachableServersKeepTheLastTable(t *testing.T) {
	r := fromAgentSelect(t)
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "Plat2 10", "US - THEM")
	r.clock.Advance(40 * time.Second)
	r.presence(func(w *fakeriot.World) { w.Loop, w.CoreGame = fakeriot.LoopMenus, nil })
	r.screen("LAST MATCH", "Plat2 10")
	r.clock.Advance(15 * time.Second)
	r.settled("LAST MATCH")

	for _, svc := range []string{"pd", "glz", "shared"} {
		r.fr.AddFault(fakeriot.Fault{Service: svc, Drop: true})
	}
	const next = "9e000000-0000-4000-8000-000000000003"
	r.fr.Update(func(w *fakeriot.World) {
		w.Pregame = &fakeriot.LiveGame{ID: next, Queue: "competitive", Map: fakeriot.MapBind, Mode: fakeriot.ModeBomb,
			Players: []fakeriot.LivePlayer{{PUUID: w.Self, Team: "Blue"}}}
	})
	r.enterAgentSelectWith(next)
	frame := r.screen("can't reach Riot's servers", "Plat2 10")
	if !strings.Contains(frame, "LAST MATCH") {
		t.Errorf("the last table should stay while Riot is unreachable:\n%s", frame)
	}
}

func (r *rig) enterAgentSelectWith(id string) {
	r.t.Helper()
	r.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopPregame })
	r.fr.EmitPregame(id, false)
}

func TestOnePlayersFailureLeavesEveryoneElseComplete(t *testing.T) {
	r := inAgentSelect(t, lobbySamples)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/match-history/v1/history/" + ally2, Status: 500, Body: `{"httpStatus":500}`})
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/" + ally2, Exact: true, Status: 503, Body: `{"httpStatus":503}`})
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "AllyOne#A1", "Kestrel#EUW")
	if got := cellIn(t, frame, "RANK", 3); got != "-" {
		t.Errorf("the failed ally's rank: got %q", got)
	}
	want := map[string]string{"PLAYER": "AllyOne#A1", "RANK": "Dia1 50", "K/D": "2.00", "N": "5", "TREND": "+5"}
	for col, v := range want {
		if got := cellIn(t, frame, col, 2); got != v {
			t.Errorf("ally one's %s: got %q, want %q\n%s", col, got, v, frame)
		}
	}

	log := errorLog(t, r)
	if !strings.Contains(log, "500") || !strings.Contains(log, "503") {
		t.Errorf("the failures aren't in the error log:\n%s", log)
	}
	for _, req := range r.fr.Requests() {
		for _, secret := range []string{req.Header.Get("Authorization"), req.Header.Get("X-Riot-Entitlements-JWT")} {
			if secret != "" && strings.Contains(log, strings.TrimPrefix(strings.TrimPrefix(secret, "Bearer "), "Basic ")) {
				t.Errorf("the error log holds a token")
			}
		}
	}
	if pw := lockfilePassword(t, r.fr.Lockfile()); strings.Contains(log, pw) || strings.Contains(log, "eyJ") {
		t.Errorf("the error log holds a secret:\n%s", log)
	}
}
