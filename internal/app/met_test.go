package app_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// versus adds one of your competitive matches with one other player on your team or theirs
func versus(w *fakeriot.World, id string, ago time.Duration, other string, sameTeam, youWon bool) {
	team := "Red"
	if sameTeam {
		team = "Blue"
	}
	w.AddMatch(&fakeriot.Match{
		ID: id, Queue: "competitive", Map: fakeriot.MapLotus, Mode: fakeriot.ModeBomb, Start: testStart.Add(-ago),
		Players: []fakeriot.MatchPlayer{
			{PUUID: w.Self, Team: "Blue", Agent: fakeriot.Jett, Score: 3000, Rounds: 20, Kills: 15, Deaths: 15},
			{PUUID: other, Team: team, Agent: fakeriot.Omen, Score: 3000, Rounds: 20, Kills: 15, Deaths: 15},
		},
		Teams: []fakeriot.TeamResult{{ID: "Blue", Won: youWon}, {ID: "Red", Won: !youWon}},
	})
}

func TestPlayersFromYourMatchesAreMarkedAsMetBefore(t *testing.T) {
	r := fromAgentSelect(t, func(w *fakeriot.World) {
		versus(w, "cccc0000-0000-4000-8000-000000000001", 3*time.Hour, enemy1, false, true)
		versus(w, "cccc0000-0000-4000-8000-000000000002", 50*time.Hour, ally1, true, false)
		versus(w, "cccc0000-0000-4000-8000-000000000003", 40*time.Minute, enemy2, true, true)
	})
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "US - THEM", "Plat2 10")
	want := map[int]string{
		3:  "ally L 2d",
		4:  "",
		9:  "enemy W 3h",
		10: "ally W 40m",
		12: "",
	}
	for line, w := range want {
		if got := cellIn(t, frame, "MET", line); got != w {
			t.Errorf("line %d: got %q, want %q\n%s", line, got, w, frame)
		}
	}
	if got := cellIn(t, frame, "MET", 2); got != "" {
		t.Errorf("you never meet yourself, got %q", got)
	}
}

// playLobby runs one full match from agent select to the menus
func playLobby(t *testing.T, r *rig) {
	t.Helper()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "Dia1 50")
	r.enterMatchLoad()
	r.settled("MATCH LOADING", "Plat2 10")
	r.clock.Advance(40 * time.Second)
	r.screen("IN MATCH")
	r.clock.Advance(35 * time.Minute)
	r.presence(func(w *fakeriot.World) { w.Loop, w.CoreGame = fakeriot.LoopMenus, nil })
	r.screen("LAST MATCH")
	r.clock.Advance(15 * time.Second)
	r.settled("LAST MATCH")
}

const unratedGame = "c9000000-0000-4000-8000-000000000002"

// secondLobby is an unrated match where ally two and the formerly hidden ally three are now enemies
func secondLobby(w *fakeriot.World) {
	w.Player(ally3).Incognito = false
	w.Queue = "unrated"
	w.CoreGame = &fakeriot.LiveGame{
		ID: unratedGame, Queue: "unrated", Map: fakeriot.MapHaven, Mode: fakeriot.ModeBomb,
		Players: []fakeriot.LivePlayer{
			{PUUID: w.Self, Team: "Blue", Agent: fakeriot.Sova},
			{PUUID: ally2, Team: "Red", Agent: fakeriot.Killjoy},
			{PUUID: ally3, Team: "Red", Agent: fakeriot.Omen},
			{PUUID: enemy5, Team: "Red", Agent: fakeriot.Skye},
		},
	}
}

func TestPlayersFromEarlierLobbiesThisSessionAreMarked(t *testing.T) {
	r := newRig(t, matchLobby, func(w *fakeriot.World) {
		compGame(w, coreGame, at(25, 20, 0), 17, win)
		me := w.Player(w.Self)
		me.Updates = append([]fakeriot.Update{me.Updates[len(me.Updates)-1]}, me.Updates[:len(me.Updates)-1]...)
	})
	r.live()
	r.settled("waiting for the next match", "SESSION")
	r.subscribed()
	playLobby(t, r)

	r.clock.Advance(5 * time.Minute)
	r.fr.Update(secondLobby)
	r.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopIngame })
	r.fr.EmitCoreGame(unratedGame, false)
	frame := r.settled("MATCH LOADING · Haven · Unrated")
	// the first match loaded 40m55s ago and you won it - recency rounds down
	if got := cellIn(t, frame, "MET", 5); got != "ally W 40m" {
		t.Errorf("ally two from the first lobby: got %q, want ally W 40m\n%s", got, frame)
	}
	if got := cellIn(t, frame, "MET", 6); got != "" {
		t.Errorf("ally three was hidden in the first lobby and must never be indexed, got %q", got)
	}
	if got := cellIn(t, frame, "MET", 7); got != "enemy W 40m" {
		t.Errorf("enemy five from the first lobby: got %q\n%s", got, frame)
	}
}

func TestMetBeforeLivesInMemoryOnly(t *testing.T) {
	r := newRig(t, matchLobby)
	r.live()
	r.settled("waiting for the next match", "SESSION")
	r.subscribed()
	playLobby(t, r)
	if files, _ := os.ReadDir(r.dir); len(files) != 0 {
		t.Errorf("lowkey wrote %d files during a clean session", len(files))
	}
	requests := len(r.fr.RemoteRequests())

	r2 := r.restart()
	r2.settled("waiting for the next match", "SESSION")
	r2.subscribed()
	r2.fr.Update(secondLobby)
	r2.fr.Update(func(w *fakeriot.World) { w.Loop = fakeriot.LoopIngame })
	r2.fr.EmitCoreGame(unratedGame, false)
	frame := r2.settled("MATCH LOADING · Haven · Unrated")
	if strings.Contains(frame, "ally ") || strings.Contains(frame, "enemy ") {
		t.Errorf("a restarted lowkey remembered the earlier lobby:\n%s", frame)
	}
	for _, req := range r2.fr.RemoteRequests()[requests:] {
		if strings.HasPrefix(req.Path, "/match-details/v1/matches/"+coreGame) {
			t.Errorf("met-before asked for a match: %s", req.Path)
		}
	}
}
