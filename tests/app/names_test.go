package app_test

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
	"github.com/glitchfl/lowkey/internal/remote"
)

func nameLookups(r *rig) [][]string {
	var out [][]string
	for _, req := range r.fr.RemoteRequests() {
		if req.Path == remote.NamesPath {
			out = append(out, req.Named)
		}
	}
	return out
}

func sameNames(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func TestEveryVisiblePlayerGetsTheirNameFromOneLookupPerRoster(t *testing.T) {
	r := fromAgentSelect(t, func(w *fakeriot.World) {
		w.Player(ally2).Name, w.Player(ally2).Tag = "AllyTwo", "A2"
		w.Player(ally4).Name, w.Player(ally4).Tag = "AllyFour", "A4"
		w.Player(enemy1).Name, w.Player(enemy1).Tag = "EnemyOne", "E1"
		w.Player(enemy2).Name, w.Player(enemy2).Tag = "EnemyTwo", "E2"
	})
	// nobody here has a single match - a name doesn't depend on match history
	frame := r.settled("AGENT SELECT", "Kestrel#EUW", "AllyOne#A1", "AllyTwo#A2", "AllyFour#A4")
	if got := cellIn(t, frame, "PLAYER", 4); got != "hidden" {
		t.Errorf("the hidden ally shows %q\n%s", got, frame)
	}
	r.fr.Update(func(w *fakeriot.World) { w.Pregame.Players[1].Agent = fakeriot.Sova })
	r.fr.EmitPregame(pregame, false)
	r.clock.Advance(3 * time.Second)
	r.settled("AGENT SELECT", "Sova")

	r.enterMatchLoad()
	frame = r.settled("MATCH LOADING", "AllyOne#A1", "EnemyOne#E1", "EnemyTwo#E2", "US - THEM")
	for _, line := range []int{5, 11} {
		if got := cellIn(t, frame, "PLAYER", line); got != "hidden" {
			t.Errorf("line %d shows %q\n%s", line, got, frame)
		}
	}
	got := nameLookups(r)
	if len(got) != 2 {
		t.Fatalf("got name lookups %v, want one for your team and one for the enemies", got)
	}
	if want := []string{fakeriot.DefaultSelf, ally1, ally2, ally4}; !sameNames(got[0], want) {
		t.Errorf("agent select asked about %v, want %v", got[0], want)
	}
	if want := []string{enemy1, enemy2, enemy4, enemy5}; !sameNames(got[1], want) {
		t.Errorf("match load asked about %v, want %v", got[1], want)
	}
}

func TestNameLookupsNeverIncludeHiddenPlayersWhateverTheSetting(t *testing.T) {
	for _, c := range []struct{ name, settings, stats string }{
		{"stats on", "", "Rad 900"},
		{"stats off", hiddenStatsOff, "Dia1 50"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, matchLobby, hiddenSamples)
			r.writeSettings(c.settings)
			r.live()
			r.screen("waiting for the next match")
			r.subscribed()
			r.enterAgentSelect()
			r.settled("AGENT SELECT", c.stats, "AllyOne#A1")
			r.enterMatchLoad()
			r.settled("MATCH LOADING", "US - THEM")
			for _, list := range nameLookups(r) {
				if slices.Contains(list, ally3) || slices.Contains(list, enemy3) {
					t.Errorf("a name lookup asked about a hidden player: %v", list)
				}
			}
		})
	}
}

func TestEnemyLookupsOffKeepsEnemiesOutOfTheNameLookup(t *testing.T) {
	r := newRig(t, matchLobby, func(w *fakeriot.World) { w.Player(enemy1).Name, w.Player(enemy1).Tag = "EnemyOne", "E1" })
	r.writeSettings("enemy_lookups = false\n")
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterMatchLoad()
	frame := r.settled("MATCH LOADING", "AllyOne#A1", "ENEMIES", "Raze")
	if strings.Contains(frame, "EnemyOne") {
		t.Errorf("an enemy was named with enemy lookups off\n%s", frame)
	}
	for _, list := range nameLookups(r) {
		for _, e := range []string{enemy1, enemy2, enemy3, enemy4, enemy5} {
			if slices.Contains(list, e) {
				t.Errorf("a name lookup asked about enemy %s: %v", e, list)
			}
		}
	}
}

func TestAFailedNameLookupLeavesDashesAndTheStats(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: remote.NamesPath, Status: 500, Body: `{"httpStatus":500}`})
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	frame := r.settled("AGENT SELECT", "Dia1 50", "Plat1 30")
	for line, want := range map[int]string{1: "-", 2: "-", 3: "-", 4: "hidden", 5: "-"} {
		if got := cellIn(t, frame, "PLAYER", line); got != want {
			t.Errorf("line %d: got %q, want %q\n%s", line, got, want, frame)
		}
	}
	r.fr.EmitPregame(pregame, false)
	r.clock.Advance(3 * time.Second)
	eventually(t, "the next roster", func() bool { return requestsFor(r, "/pregame/v1/matches/") == 2 })
	r.settled("AGENT SELECT")
	if got := nameLookups(r); len(got) != 1 {
		t.Errorf("a failed name lookup was asked again on the next roster: %v", got)
	}
}

func TestARateLimitedNameLookupSendsItsListAgain(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: remote.NamesPath, Status: 429, Body: rateLimited, Times: 1})
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	eventually(t, "the 429", func() bool { return r.fr.Answered(remote.NamesPath, 429) == 1 })
	eventually(t, "the backoff timer", func() bool { return r.clock.Pending() >= 1 })
	r.clock.Advance(5 * time.Second)
	r.settled("AGENT SELECT", "AllyOne#A1")
	got := nameLookups(r)
	if len(got) != 2 || !sameNames(got[0], got[1]) || len(got[1]) == 0 {
		t.Errorf("the retry should repeat the same list: %v", got)
	}
}

func TestARejectedTokenResendsTheNameLookup(t *testing.T) {
	r := newRig(t, ranked)
	r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: remote.NamesPath, Status: 401, Body: `{"httpStatus":401,"errorCode":"BAD_CLAIMS"}`, Times: 1})
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	if got := cell(t, out, "PLAYER"); got != "Kestrel#EUW" {
		t.Errorf("got %q after the token refresh\n%s", got, out)
	}
	if got := nameLookups(r); len(got) != 2 || !slices.Equal(got[1], []string{fakeriot.DefaultSelf}) {
		t.Errorf("the second try should send the list again: %v", got)
	}
}

func TestAFullRiotIDFitsUpTo20Characters(t *testing.T) {
	r := newRig(t, ranked, func(w *fakeriot.World) { w.Player(w.Self).Name, w.Player(w.Self).Tag = "FifteenCharName", "1234" })
	out, code := r.run("me")
	if code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	if got := cell(t, out, "PLAYER"); got != "FifteenCharName#1234" {
		t.Errorf("got %q\n%s", got, out)
	}
}

// the guard checks again when a lookup finally goes out - someone can go hidden while it waits its turn
func TestANameLookupThatWaitedIsRefusedIfSomeoneWentHiddenThenAsksWithoutThem(t *testing.T) {
	r := newRig(t, agentSelectLobby)
	r.live()
	r.screen("waiting for the next match")
	r.subscribed()
	// your party and the content take both request slots so the lookup queues behind them
	r.fr.AddFault(fakeriot.Fault{Service: "glz", Path: "/parties/", Hold: true})
	r.fr.AddFault(fakeriot.Fault{Service: "shared", Path: "/content-service/", Hold: true})
	r.enterAgentSelect()
	waitHeld(t, r.fr, 2)

	r.fr.Update(func(w *fakeriot.World) { w.Player(ally1).Incognito = true })
	r.fr.EmitPregame(pregame, false)
	r.clock.Advance(3 * time.Second)
	settle()
	r.fr.Release("/parties/v1/players/" + fakeriot.DefaultSelf)
	eventually(t, "the roster that shows them hidden", func() bool { return requestsFor(r, "/pregame/v1/matches/") == 2 })
	settle()
	r.fr.RemoveFaults()
	r.fr.ReleaseHeld()
	eventually(t, "the refusal", func() bool {
		log, _ := os.ReadFile(filepath.Join(r.dir, "lowkey.log"))
		return strings.Contains(string(log), "the name lookup includes a hidden player")
	})

	r.fr.EmitPregame(pregame, false)
	r.clock.Advance(3 * time.Second)
	frame := r.settled("AGENT SELECT", "Kestrel#EUW", "Dia1 50")
	if got := cellIn(t, frame, "PLAYER", 2); got != "hidden" {
		t.Errorf("the ally who went hidden shows %q\n%s", got, frame)
	}
	got := nameLookups(r)
	if len(got) != 1 {
		t.Fatalf("got name lookups %v, want only the one without them", got)
	}
	if want := []string{fakeriot.DefaultSelf, ally2, ally4}; !sameNames(got[0], want) {
		t.Errorf("asked about %v, want %v", got[0], want)
	}
}

func TestTheGuardOnlyAllowsCheckedNameLookupsAboutVisiblePlayers(t *testing.T) {
	g := remote.NewGuard()
	self, player := fakeriot.DefaultSelf, []string{ally1}
	g.SetSelf(self)
	refused := func(what string, list []string) {
		t.Helper()
		if g.CheckNames(list) == nil {
			t.Errorf("%s should be refused", what)
		}
	}
	allowed := func(what string, list []string) {
		t.Helper()
		if err := g.CheckNames(list); err != nil {
			t.Errorf("%s: %v", what, err)
		}
	}
	refused("with every window closed", player)
	g.SetWindow(remote.SessionRefresh)
	refused("during a session refresh", player)
	for _, w := range []remote.Window{remote.AgentSelect, remote.MatchLoad, remote.OneShot} {
		g.SetWindow(w)
		allowed("in a lobby or one-shot", player)
	}
	g.KeepNameHidden(ally1, true)
	refused("a hidden player", []string{self, strings.ToUpper(ally1)})
	g.KeepNameHidden(ally1, false)
	allowed("after they left streamer mode", player)
	g.Hide(ally1)
	refused("a player who's off limits", player)
	g.Show(ally1)
	g.KeepNameHidden(self, true)
	allowed("your own name in streamer mode", []string{self})

	u, err := url.Parse("https://pd.eu.a.pvp.net" + remote.NamesPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "PUT", "POST"} {
		if g.Check(method, remote.PD, remote.NamesPath, u) == nil {
			t.Errorf("a plain %s to the name service got past the guard", method)
		}
	}
}

func TestRecordingsKeepNameLookupsAnonymous(t *testing.T) {
	r := newRig(t, ranked, func(w *fakeriot.World) {
		w.Raw["pd "+remote.NamesPath] = fakeriot.RawResponse{Status: 200, Body: []byte(
			`[{"DisplayName":"Kestrel#EUW","Subject":"` + w.Self + `","GameName":"Kestrel","TagLine":"EUW"}]`)}
	})
	if out, code := r.run("me", "--record"); code != 0 {
		t.Fatalf("me failed:\n%s", out)
	}
	entries, raw := readRecording(t, recordingDir(t, r.dir))
	for _, real := range []string{"Kestrel", "EUW", fakeriot.DefaultSelf} {
		if strings.Contains(raw, real) {
			t.Errorf("the recording holds the real identity %q", real)
		}
	}
	seen := false
	for _, e := range entries {
		if e.Path == remote.NamesPath {
			seen = true
			if e.Method != "PUT" {
				t.Errorf("the name lookup was recorded as a %s", e.Method)
			}
		}
	}
	if !seen {
		t.Error("the name lookup wasn't recorded")
	}
}
