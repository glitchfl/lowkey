package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

type result int

const (
	win result = iota
	loss
	draw
)

// compGame adds one of your competitive matches with its RR change
func compGame(w *fakeriot.World, id string, start time.Time, rr int, res result) {
	teams := []fakeriot.TeamResult{{ID: "Blue", Won: res == win}, {ID: "Red", Won: res == loss}}
	w.AddMatch(&fakeriot.Match{
		ID: id, Queue: "competitive", Map: fakeriot.MapBind, Mode: fakeriot.ModeBomb, Start: start,
		Players: []fakeriot.MatchPlayer{
			{PUUID: w.Self, Team: "Blue", Agent: fakeriot.Jett, Score: 4000, Rounds: 20, Kills: 15, Deaths: 15},
			{PUUID: enemy, Team: "Red", Agent: fakeriot.Omen, Score: 4000, Rounds: 20, Kills: 15, Deaths: 15},
		},
		Teams: teams,
	})
	me := w.Player(w.Self)
	me.Updates = append(me.Updates, fakeriot.Update{MatchID: id, Start: start, Earned: rr})
}

func at(day, hour, minute int) time.Time {
	return time.Date(2026, 10, day, hour, minute, 0, 0, testZone)
}

func todaysGames(w *fakeriot.World) {
	compGame(w, "10000000-0000-4000-8000-000000000001", at(25, 19, 0), 18, win)
	compGame(w, "10000000-0000-4000-8000-000000000002", at(25, 18, 0), -15, loss)
	compGame(w, "10000000-0000-4000-8000-000000000003", at(25, 17, 0), 3, draw)
	compGame(w, "10000000-0000-4000-8000-000000000004", at(25, 7, 0), 20, win)
	compGame(w, "10000000-0000-4000-8000-000000000005", at(25, 5, 30), -12, loss)
	compGame(w, "10000000-0000-4000-8000-000000000006", at(24, 21, 0), 25, win)
}

func sessionLine(t *testing.T, out string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "SESSION") {
			return l
		}
	}
	t.Fatalf("no session line in:\n%s", out)
	return ""
}

func TestRRTrendIsTheNetOfYourLastFiveCompetitiveGames(t *testing.T) {
	r := newRig(t, ranked, todaysGames)
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	// 18 - 15 + 3 + 20 - 12
	if got := cell(t, out, "TREND"); got != "+14" {
		t.Errorf("trend: got %q, want +14\n%s", got, out)
	}
}

func TestSessionCountsCompetitiveGamesSinceTheResetHour(t *testing.T) {
	r := newRig(t, ranked, todaysGames)
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	// since 06:00 - two wins and one loss - the draw adds RR but no result
	if l := sessionLine(t, out); !strings.Contains(l, "2W 1L +26 RR") || !strings.Contains(l, "since 06:00") {
		t.Errorf("got %q", l)
	}
}

func TestSessionResetsAtSixInTheMorningLocalTime(t *testing.T) {
	games := func(w *fakeriot.World) {
		compGame(w, "20000000-0000-4000-8000-000000000001", at(26, 5, 30), 10, win)
		compGame(w, "20000000-0000-4000-8000-000000000002", at(25, 23, 0), -8, loss)
		compGame(w, "20000000-0000-4000-8000-000000000003", at(25, 5, 0), 30, win)
	}
	cases := []struct {
		now  time.Time
		want string
	}{
		{at(26, 5, 59), "1W 1L +2 RR"},
		{at(26, 6, 1), "0W 0L +0 RR"},
	}
	for _, c := range cases {
		t.Run(c.now.Format("15:04"), func(t *testing.T) {
			r := newRigAt(t, c.now, fakeriot.Options{}, ranked, games)
			out, code := r.run("me")
			if code != 0 {
				t.Fatalf("me failed:\n%s", out)
			}
			if l := sessionLine(t, out); !strings.Contains(l, c.want) {
				t.Errorf("got %q, want %q", l, c.want)
			}
		})
	}
}

func TestEachMatchIsFetchedOnceEvenWhenTheSessionAndSampleShareIt(t *testing.T) {
	r := newRig(t, ranked, todaysGames)
	if out, code := r.run("me"); code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	seen := map[string]int{}
	updates := 0
	for _, req := range r.fr.RemoteRequests() {
		if strings.HasPrefix(req.Path, "/match-details/") {
			seen[req.Path]++
		}
		if strings.HasSuffix(req.Path, "/competitiveupdates") {
			updates++
		}
	}
	for path, n := range seen {
		if n > 1 {
			t.Errorf("%s fetched %d times", path, n)
		}
	}
	if updates != 1 {
		t.Errorf("competitive updates fetched %d times, want once for both the trend and the session", updates)
	}
}

func TestTheLiveSessionLineStartsOverAtTheResetHourAcrossARestart(t *testing.T) {
	r := newRigAt(t, at(26, 5, 50), fakeriot.Options{}, ranked, func(w *fakeriot.World) {
		compGame(w, "20000000-0000-4000-8000-000000000001", at(26, 5, 30), 10, win)
		compGame(w, "20000000-0000-4000-8000-000000000002", at(25, 23, 0), -8, loss)
	})
	r.live()
	r.screen("SESSION · 1W 1L +2 RR since 06:00")

	// after 6am a fresh start has to count from this morning and not yesterday
	r.clock.Advance(20 * time.Minute)
	r.restart()
	r.screen("SESSION · 0W 0L +0 RR since 06:00")
}
