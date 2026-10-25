package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

const (
	ally1   = "a1000000-0000-4000-8000-000000000001"
	ally2   = "a2000000-0000-4000-8000-000000000002"
	ally3   = "a3000000-0000-4000-8000-000000000003"
	ally4   = "a4000000-0000-4000-8000-000000000004"
	pregame = "9e000000-0000-4000-8000-000000000001"
)

// agentSelectLobby: you, two ranked allies, one in streamer mode and one unranked
func agentSelectLobby(w *fakeriot.World) {
	ranked(w)
	todaysGames(w)
	a1 := w.Player(ally1)
	a1.Name, a1.Tag = "AllyOne", "A1"
	a1.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 18, RR: 50, Wins: 10, WinsWithPlacements: 10, Games: 20})
	a1.Updates = []fakeriot.Update{{MatchID: "x1", Earned: 10}, {MatchID: "x2", Earned: -5}}
	a2 := w.Player(ally2)
	a2.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 15, RR: 30, Wins: 6, WinsWithPlacements: 6, Games: 10})
	a2.Updates = []fakeriot.Update{{MatchID: "x3", Earned: -10}}
	a3 := w.Player(ally3)
	a3.Incognito = true
	a3.SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 27, RR: 900, Wins: 90, Games: 100})
	w.Player(ally4)
	w.Queue = "competitive"
	w.Pregame = &fakeriot.LiveGame{
		ID: pregame, Queue: "competitive", Map: fakeriot.MapAscent, Mode: fakeriot.ModeBomb, Ranked: true,
		Players: []fakeriot.LivePlayer{
			{PUUID: ally1, Team: "Blue", Agent: fakeriot.Jett, State: "selected"},
			{PUUID: w.Self, Team: "Blue"},
			{PUUID: ally2, Team: "Blue"},
			{PUUID: ally3, Team: "Blue", Agent: fakeriot.Omen, State: "locked"},
			{PUUID: ally4, Team: "Blue", Agent: fakeriot.Sage, State: "locked"},
			{PUUID: enemy, Team: "Red", Agent: fakeriot.Reyna, State: "locked"},
		},
	}
}

// enterAgentSelect is the lifecycle event that starts agent select
func (r *rig) enterAgentSelect() {
	r.t.Helper()
	r.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopPregame })
	r.fr.EmitPregame(pregame, false)
}

func requestsFor(r *rig, prefix string) int {
	n := 0
	for _, req := range r.fr.RemoteRequests() {
		if strings.HasPrefix(req.Path, prefix) {
			n++
		}
	}
	return n
}

func TestAgentSelectShowsYourTeam(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.screen("AGENT SELECT · Ascent · Competitive", "SESSION · 2W 1L +26 RR", "Dia1 50", "Plat1 30")

	rows := []struct {
		agent, rank, peak, last, wr, wl, trend string
	}{
		{"", "Dia3 12", "Imm2", "Dia2", "57%", "12-9", "+14"},
		{"Jett", "Dia1 50", "Dia1", "-", "50%", "10-10", "+5"},
		{"", "Plat1 30", "Plat1", "-", "60%", "6-4", "-10"},
		{"Omen", "", "", "", "", "", ""},
		{"Sage", "-", "-", "-", "-", "-", "-"},
	}
	for i, want := range rows {
		got := struct{ agent, rank, peak, last, wr, wl, trend string }{
			cellIn(t, frame, "AGENT", i+1), cellIn(t, frame, "RANK", i+1), cellIn(t, frame, "PEAK", i+1),
			cellIn(t, frame, "LAST", i+1), cellIn(t, frame, "WR", i+1), cellIn(t, frame, "W-L", i+1),
			cellIn(t, frame, "TREND", i+1),
		}
		if got != want {
			t.Errorf("row %d: got %+v, want %+v\n%s", i+1, got, want, frame)
		}
	}
	if got := cellIn(t, frame, "PLAYER", 4); got != "hidden" {
		t.Errorf("the streamer-mode ally shows %q, want hidden", got)
	}
	avg := []struct{ col, want string }{{"PLAYER", "AVERAGE 4/5"}, {"RANK", "Dia1"}, {"WR", "56%"}, {"TREND", "+3"}}
	for _, a := range avg {
		if got := cellIn(t, frame, a.col, 6); got != a.want {
			t.Errorf("average %s: got %q, want %q\n%s", a.col, got, a.want, frame)
		}
	}
	lines := strings.Split(strings.TrimRight(frame, "\n\f"), "\n")
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, "SESSION") {
		t.Errorf("the session line should come last, got %q", last)
	}
}

func TestStreamerModePlayersAreNeverLookedUp(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) { w.Player(w.Self).Incognito = true })
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.screen("AGENT SELECT", "Dia1 50", "Dia3 12")
	for _, req := range r.fr.RemoteRequests() {
		if strings.Contains(req.Path+"?"+req.Query.Encode(), ally3) {
			t.Errorf("requested a hidden player's data: %s", req.Path)
		}
	}
	if requestsFor(r, "/mmr/v1/players/"+fakeriot.DefaultSelf) == 0 {
		t.Error("streamer mode on yourself must not stop your own lookup")
	}
}

func TestHoversAndLocksUpdateTheAgentsAtMostEveryThreeSeconds(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.screen("AGENT SELECT", "Dia1 50")
	if n := requestsFor(r, "/pregame/v1/matches/"); n != 1 {
		t.Fatalf("roster fetched %d times on entry, want once", n)
	}

	r.fr.Update(func(w *fakeriot.World) {
		w.Pregame.Players[1].Agent, w.Pregame.Players[1].State = fakeriot.Sova, "selected"
		w.Pregame.Players[2].Agent, w.Pregame.Players[2].State = fakeriot.Killjoy, "locked"
	})
	for i := 0; i < 3; i++ {
		r.fr.EmitPregame(pregame, false)
	}
	settle()
	if n := requestsFor(r, "/pregame/v1/matches/"); n != 1 {
		t.Fatalf("roster fetched %d times within 3 s", n)
	}
	r.clock.Advance(3 * time.Second)
	frame := r.screen("Sova", "Killjoy")
	if got := cellIn(t, frame, "AGENT", 1); got != "Sova" {
		t.Errorf("your agent: got %q, want Sova", got)
	}
	if n := requestsFor(r, "/pregame/v1/matches/"); n != 2 {
		t.Errorf("roster fetched %d times, want 2", n)
	}
}

func TestDuplicateLifecycleEventsCountOnce(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopPregame })
	r.fr.EmitPregame(pregame, true)
	r.screen("AGENT SELECT", "Dia1 50")
	r.clock.Advance(10 * time.Second)
	settle()
	if n := requestsFor(r, "/pregame/v1/matches/"); n != 1 {
		t.Errorf("roster fetched %d times for one event sent under both message paths", n)
	}
}

func TestRowsFillInAsDataArrives(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/", Hold: true})
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.screen("AGENT SELECT", "Jett")
	if got := cellIn(t, frame, "RANK", 2); got != "" {
		t.Errorf("rank shown before rank data arrived: %q", got)
	}
	r.fr.ReleaseHeld()
	frame = r.screen("Dia1 50")
	if got := cellIn(t, frame, "RANK", 2); got != "Dia1 50" {
		t.Errorf("rank after data arrived: got %q", got)
	}
}

func TestADodgeDropsBackAndCancelsLookups(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/", Hold: true})
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.screen("AGENT SELECT")
	waitHeld(t, r.fr, 2)

	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus; w.Pregame = nil })
	r.screen("waiting for the next match")
	eventually(t, "the held lookups to be cancelled", func() bool { return len(r.fr.Held()) == 0 })
	before := len(r.fr.RemoteRequests())
	r.fr.ReleaseHeld()
	r.clock.Advance(10 * time.Second)
	settle()
	if after := len(r.fr.RemoteRequests()); after != before {
		t.Errorf("%d requests after the dodge", after-before)
	}
	if strings.Contains(r.con.last(), "AGENT SELECT") {
		t.Errorf("agent select data lingered after the dodge:\n%s", r.con.last())
	}
}

func TestUntrackedModesAreIgnored(t *testing.T) {
	r := newRig(t, agentSelectLobby, func(w *fakeriot.World) { w.Queue, w.Pregame.Queue = "spikerush", "spikerush" })
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.screen("not tracking spikerush")
	r.clock.Advance(10 * time.Second)
	settle()
	if reqs := r.fr.RemoteRequests(); len(reqs) != 0 {
		t.Errorf("made %d requests in an untracked mode, first %s", len(reqs), reqs[0].Path)
	}
}
