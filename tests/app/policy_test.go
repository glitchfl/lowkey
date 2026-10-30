package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

func countSince(r *rig, from int) []string {
	var out []string
	for _, req := range r.fr.RemoteRequests()[from:] {
		out = append(out, req.Service+" "+req.Path)
	}
	return out
}

// sessionOnly is everything the session refresh window may ask for
func sessionOnly(t *testing.T, reqs []string, what string) {
	t.Helper()
	for _, req := range reqs {
		ok := strings.HasSuffix(req, "/mmr/v1/players/"+fakeriot.DefaultSelf+"/competitiveupdates") ||
			strings.HasPrefix(req, "pd /match-details/") || req == "static /v1/version"
		if !ok {
			t.Errorf("%s: %s isn't part of a session refresh", what, req)
		}
	}
}

func TestLowkeyIsSilentOutsideItsWindows(t *testing.T) {
	r := newRig(t, matchLobby, func(w *fakeriot.World) { w.RiotClient, w.Valorant = false, false })
	r.live()
	r.screen("waiting for the Riot Client")
	r.clock.Advance(time.Hour)
	settle()
	if reqs := countSince(r, 0); len(reqs) != 0 {
		t.Errorf("idle: %v", reqs)
	}

	if err := r.fr.StartClient(); err != nil {
		t.Fatal(err)
	}
	r.screen("waiting for Valorant")
	r.subscribed()
	r.clock.Advance(time.Hour)
	settle()
	if reqs := countSince(r, 0); len(reqs) != 0 {
		t.Errorf("waiting for Valorant: %v", reqs)
	}

	r.presence(func(w *fakeriot.World) { w.Valorant, w.Loop = true, fakeriot.LoopMenus })
	r.settled("waiting for the next match", "SESSION")
	sessionOnly(t, countSince(r, 0), "menus at startup")
	quiet := len(r.fr.RemoteRequests())
	r.clock.Advance(time.Hour)
	settle()
	if reqs := countSince(r, quiet); len(reqs) != 0 {
		t.Errorf("menus after the startup refresh: %v", reqs)
	}

	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "Plat2 10")
	r.clock.Advance(40 * time.Second)
	r.screen("IN MATCH")
	inRound := len(r.fr.RemoteRequests())
	r.clock.Advance(30 * time.Minute)
	settle()
	if reqs := countSince(r, inRound); len(reqs) != 0 {
		t.Errorf("in round: %v", reqs)
	}

	r.presence(func(w *fakeriot.World) { w.Loop, w.CoreGame = fakeriot.LoopMenus, nil })
	r.screen("LAST MATCH")
	menus := len(r.fr.RemoteRequests())
	r.clock.Advance(15 * time.Second)
	r.settled("LAST MATCH")
	sessionOnly(t, countSince(r, menus), "post-match refresh")
	after := len(r.fr.RemoteRequests())
	r.clock.Advance(2 * time.Hour)
	settle()
	if reqs := countSince(r, after); len(reqs) != 0 {
		t.Errorf("menus after the post-match refresh: %v", reqs)
	}
}

func errorLog(t *testing.T, r *rig) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.dir, "lowkey.log"))
	if err != nil {
		t.Fatalf("no error log: %v", err)
	}
	return string(data)
}

func TestTheGuardRefusesRequestsAboutHiddenPlayersWhateverTheData(t *testing.T) {
	// a visible ally's history claims a match whose ID is a hidden player's PUUID
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) {
		w.AddMatch(&fakeriot.Match{
			ID: ally3, Queue: "competitive", Map: fakeriot.MapAscent, Mode: fakeriot.ModeBomb, Start: testStart.Add(-time.Hour),
			Players: []fakeriot.MatchPlayer{{PUUID: ally1, Team: "Blue", Agent: fakeriot.Jett, Score: 1, Rounds: 1}},
		})
	})
	r.writeSettings(hiddenStatsOff)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	if log := errorLog(t, r); !strings.Contains(log, "refused") || !strings.Contains(log, "hidden player") {
		t.Errorf("the refusal wasn't logged:\n%s", log)
	}
}

func TestIDsFromRiotNeverBecomeForbiddenPaths(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) {
		w.Raw["glz /pregame/v1/players/"+w.Self] = fakeriot.RawResponse{Status: 200,
			Body: []byte(`{"Subject":"` + w.Self + `","MatchID":"` + pregame + `/select/` + fakeriot.Jett + `","Version":1}`)}
		w.Raw["pd /match-history/v1/history/"+ally1+"?endIndex=13&queue=competitive&startIndex=0"] = fakeriot.RawResponse{Status: 200,
			Body: []byte(`{"Subject":"` + ally1 + `","History":[{"MatchID":"../../../name-service/v2/players","GameStartTime":1,"QueueID":"competitive"}]}`)}
	})
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	// presence moves first so lowkey asks Riot for the match ID
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopPregame })
	r.screen("AGENT SELECT")
	r.clock.Advance(10 * time.Second)
	settle()
	for _, req := range r.fr.RemoteRequests() {
		if strings.Contains(req.Path, "/select/") || strings.Contains(req.Path, "name-service") || strings.Contains(req.Path, "..") {
			t.Errorf("an ID from a response became the path %s", req.Path)
		}
	}
}

func TestOneShotCommandsStillMakeTheirRequests(t *testing.T) {
	r := newRig(t, ranked)
	if out, code := r.run("me"); code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	if len(r.fr.RemoteRequests()) == 0 {
		t.Error("me made no requests")
	}
	r2 := newRig(t)
	if out, code := r2.run("doctor"); code != 0 {
		t.Fatalf("doctor failed:\n%s", out)
	}
	if requestsExact(r2, "/mmr/v1/players/"+fakeriot.DefaultSelf) != 1 {
		t.Error("doctor's rank data request didn't go out")
	}
}
