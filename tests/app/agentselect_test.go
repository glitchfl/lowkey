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

// agentSelectLobby - you plus two ranked allies and one in streamer mode and one unranked
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

func requestsExact(r *rig, path string) int {
	n := 0
	for _, req := range r.fr.RemoteRequests() {
		if req.Path == path {
			n++
		}
	}
	return n
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
	frame := r.settled("AGENT SELECT · Ascent · Defenders · Competitive", "SESSION · 2W 1L +26 RR", "Dia1 50", "Plat1 30")

	rows := []struct {
		agent, rank, peak, last, wr, wl, trend string
	}{
		{"", "Dia3 12", "Imm2", "Dia2", "57%", "12-9", "+14"},
		{"(Jett)", "Dia1 50", "Dia1", "-", "50%", "10-10", "+5"},
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
	if requestsExact(r, "/mmr/v1/players/"+fakeriot.DefaultSelf) == 0 {
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
	if got := cellIn(t, frame, "AGENT", 1); got != "(Sova)" {
		t.Errorf("your hovered agent: got %q, want (Sova)", got)
	}
	if n := requestsFor(r, "/pregame/v1/matches/"); n != 2 {
		t.Errorf("roster fetched %d times, want 2", n)
	}
}

func TestHoveredAgentsShowInBracketsUntilLockedIn(t *testing.T) {
	r := fromAgentSelect(t)
	frame := r.settled("AGENT SELECT", "(Jett)")
	// ally one only hovers Jett - the streamer-mode ally and the unranked one locked theirs in
	for row, want := range map[int]string{2: "(Jett)", 4: "Omen", 5: "Sage"} {
		if got := cellIn(t, frame, "AGENT", row); got != want {
			t.Errorf("row %d agent: got %q, want %q\n%s", row, got, want, frame)
		}
	}

	r.fr.Hover(ally1, fakeriot.Jett, true)
	settle()
	r.clock.Advance(3 * time.Second)
	eventually(t, "the lock to drop the brackets", func() bool { return cellIn(t, r.con.last(), "AGENT", 2) == "Jett" })

	// a hover still on screen when agent select ends was locked in - the allies carried into match load
	// show bare even before the core-game roster lands
	r.fr.Hover(ally2, fakeriot.Killjoy, false)
	settle()
	r.clock.Advance(3 * time.Second)
	r.screen("(Killjoy)")
	r.fr.AddFault(fakeriot.Fault{Service: "glz", Path: "/core-game/v1/matches/" + coreGame, Exact: true, Hold: true})
	r.enterMatchLoad()
	frame = r.screen("MATCH LOADING", "Killjoy")
	if strings.Contains(frame, "(") {
		t.Errorf("match load shows a hover:\n%s", frame)
	}
	r.fr.ReleaseHeld()
	frame = r.settled("MATCH LOADING", "US - THEM")
	if strings.Contains(frame, "(") {
		t.Errorf("match load shows a hover:\n%s", frame)
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
	r.screen("waiting for the next match", "SESSION")
	r.subscribed()
	before := len(r.fr.RemoteRequests())
	r.enterAgentSelect()
	r.screen("not tracking spikerush")
	r.clock.Advance(10 * time.Second)
	settle()
	if reqs := r.fr.RemoteRequests()[before:]; len(reqs) != 0 {
		t.Errorf("made %d requests in an untracked mode, first %s", len(reqs), reqs[0].Path)
	}
}

func TestADodgeDropsCachedRankDataToo(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	r.presence(func(w *fakeriot.World) { w.Loop = fakeriot.LoopMenus })
	r.screen("waiting for the next match")

	// a dodge costs RR so the next lobby must not reuse the old rank data
	r.fr.Update(func(w *fakeriot.World) {
		w.Player(w.Self).SetRank("competitive", fakeriot.ActCurrent, fakeriot.SeasonInfo{Tier: 20, RR: 2, Wins: 12, WinsWithPlacements: 12, Games: 21})
	})
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "Dia1 50")
	if got := cellIn(t, frame, "RANK", 1); got != "Dia3 2" {
		t.Errorf("your rank after the dodge: got %q, want the fresh Dia3 2", got)
	}
	if n := requestsExact(r, "/match-history/v1/history/"+ally1); n != 2 {
		t.Errorf("ally one's history fetched %d times, want once per lobby", n)
	}
}
