package app_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glitchfl/lowkey/internal/app"
	"github.com/glitchfl/lowkey/internal/fakeriot"
)

type recEntry struct {
	Method  string          `json:"method"`
	Kind    string          `json:"kind"`
	Service string          `json:"service"`
	Path    string          `json:"path"`
	Status  int             `json:"status"`
	Body    json.RawMessage `json:"body"`
}

// the recognizable fake every recording uses for your own PUUID
const selfFake = "5e1f5e1f-5e1f-4e1f-8e1f-5e1f5e1f5e1f"

var uuidRE = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func recordingDir(t *testing.T, parent string) string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(parent, "lowkey-record-*"))
	if len(matches) != 1 {
		t.Fatalf("want one recording folder in %s, found %v", parent, matches)
	}
	return matches[0]
}

func readRecording(t *testing.T, dir string) ([]recEntry, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "record.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []recEntry
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e recEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("bad recording line %q: %v", sc.Text(), err)
		}
		entries = append(entries, e)
	}
	return entries, string(raw)
}

func normalize(service, path string) string {
	return service + " " + uuidRE.ReplaceAllString(path, "<id>")
}

func TestRecordingHoldsEveryResponseDoctorSaw(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) { w.PresenceVersion = "" })
	if out, code := r.run("doctor", "--record"); code != 0 {
		t.Fatalf("doctor failed:\n%s", out)
	}
	entries, _ := readRecording(t, recordingDir(t, r.dir))

	var want, got []string
	for _, req := range r.fr.Requests() {
		want = append(want, normalize(req.Service, req.Path))
	}
	sawLockfile := false
	for _, e := range entries {
		switch e.Kind {
		case "lockfile":
			sawLockfile = true
		case "local":
			got = append(got, normalize("local", e.Path))
		case "remote":
			got = append(got, normalize(e.Service, e.Path))
		}
		if e.Kind != "lockfile" && e.Status == 0 {
			t.Errorf("entry %s %s has no status", e.Kind, e.Path)
		}
	}
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Errorf("recording doesn't match what fakeriot answered\nwant:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
	if !sawLockfile {
		t.Error("the lockfile isn't in the recording")
	}
}

func TestRecordingsHoldNoSecrets(t *testing.T) {
	r := newRig(t)
	if out, code := r.run("doctor", "--record"); code != 0 {
		t.Fatalf("doctor failed:\n%s", out)
	}
	_, raw := readRecording(t, recordingDir(t, r.dir))
	secrets := []string{lockfilePassword(t, r.fr.Lockfile()), "8yVg0EMbWbvC9b4yUP5Bjw"}
	for _, req := range r.fr.Requests() {
		if auth := req.Header.Get("Authorization"); auth != "" {
			secrets = append(secrets, auth, strings.TrimPrefix(auth, "Bearer "), strings.TrimPrefix(auth, "Basic "))
		}
		secrets = append(secrets, req.Header.Get("X-Riot-Entitlements-JWT"))
	}
	for _, s := range secrets {
		if s != "" && strings.Contains(raw, s) {
			t.Errorf("recording holds a secret %.12q...", s)
		}
	}
	if regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.`).MatchString(raw) {
		t.Error("recording holds something JWT-shaped")
	}
	if strings.Contains(strings.ToLower(raw), "authorization") {
		t.Error("recording mentions an Authorization header")
	}
}

func TestRecordingsReplaceEveryIdentityConsistently(t *testing.T) {
	r := newRig(t)
	if out, code := r.run("doctor", "--record"); code != 0 {
		t.Fatalf("doctor failed:\n%s", out)
	}
	entries, raw := readRecording(t, recordingDir(t, r.dir))
	for _, real := range []string{fakeriot.DefaultSelf, "f81e0000-0000-4000-8000-000000000001", "Kestrel", "friend"} {
		if strings.Contains(raw, real) {
			t.Errorf("recording still holds the real identity %q", real)
		}
	}
	selfSeen := map[string]bool{}
	for _, e := range entries {
		switch {
		case e.Kind == "local" && e.Path == "/entitlements/v1/token":
			var v struct{ Subject string }
			json.Unmarshal(e.Body, &v)
			if v.Subject != selfFake {
				t.Errorf("entitlements subject became %q, want the self fake", v.Subject)
			}
			selfSeen["entitlements"] = true
		case e.Kind == "remote" && strings.HasPrefix(e.Path, "/mmr/v1/players/"):
			var v struct{ Subject string }
			json.Unmarshal(e.Body, &v)
			if e.Path != "/mmr/v1/players/"+selfFake || v.Subject != selfFake {
				t.Errorf("rank data for you was recorded as %s with subject %q", e.Path, v.Subject)
			}
			selfSeen["rank data"] = true
		case e.Kind == "local" && e.Path == "/chat/v4/presences":
			var v struct {
				Presences []struct {
					Puuid    string `json:"puuid"`
					Product  string `json:"product"`
					GameName string `json:"game_name"`
					GameTag  string `json:"game_tag"`
				}
			}
			json.Unmarshal(e.Body, &v)
			for _, p := range v.Presences {
				if p.Product == "valorant" {
					if p.Puuid != selfFake || p.GameName != "self" || p.GameTag != "self" {
						t.Errorf("your presence was recorded as %s %s#%s", p.Puuid, p.GameName, p.GameTag)
					}
					selfSeen["presence"] = true
				} else if p.Puuid == selfFake || p.GameName == "friend" {
					t.Errorf("a friend's presence wasn't replaced: %+v", p)
				}
			}
		}
	}
	if len(selfSeen) != 3 {
		t.Errorf("found your identity in %v, want entitlements, presence and rank data", selfSeen)
	}
}

func TestARecordingPlaysBackThroughFakeriot(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) { w.Region, w.Shape = "ap", fakeriot.Hybrid })
	first, code := r.run("doctor", "--record")
	if code != 0 {
		t.Fatalf("doctor failed:\n%s", first)
	}
	world, err := fakeriot.LoadRecording(recordingDir(t, r.dir), testStart)
	if err != nil {
		t.Fatal(err)
	}
	replay := newRig(t, func(w *fakeriot.World) { *w = *world })
	second, code := replay.run("doctor")
	if code != 0 {
		t.Fatalf("doctor against the recording failed:\n%s", second)
	}
	for _, item := range []string{"phase", "session", "client version", "rank data"} {
		if a, b := detail(line(t, first, item)), detail(line(t, second, item)); a != b {
			t.Errorf("%s differs after playback:\n  recorded %q\n  replayed %q", item, a, b)
		}
	}
}

func detail(l string) string { return strings.Join(strings.Fields(l)[1:], " ") }

func TestNothingIsWrittenWithoutRecord(t *testing.T) {
	r := newRig(t)
	if out, code := r.run("doctor"); code != 0 {
		t.Fatalf("doctor failed:\n%s", out)
	}
	files, _ := os.ReadDir(r.dir)
	if len(files) != 0 {
		t.Errorf("doctor wrote %d files without --record", len(files))
	}
}

func TestARejectedLocalCertificateIsSavedWithTheRecording(t *testing.T) {
	r := newRig(t)
	r.env.LocalRoots = nil
	if out, code := r.run("doctor", "--record"); code == 0 {
		t.Fatalf("doctor passed with an untrusted certificate:\n%s", out)
	}
	pem, err := os.ReadFile(filepath.Join(recordingDir(t, r.dir), "local-api-chain.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pem), "-----BEGIN CERTIFICATE-----") {
		t.Errorf("local-api-chain.pem holds no certificate:\n%s", pem)
	}
}

func TestLiveRecordingsKeepAnonymizedScreens(t *testing.T) {
	r := newRig(t, matchLobby, lobbySamples)
	r.env.Console.Terminal = true
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- app.Run(ctx, r.env, []string{"--record"}) }()
	stopped := false
	defer func() {
		if !stopped {
			cancel()
			<-done
		}
	}()
	r.screen("waiting for the next match")
	r.subscribed()
	r.enterAgentSelect()
	r.settled("AGENT SELECT", "AllyOne#A1", "AllyTwo#A2")
	cancel()
	<-done
	stopped = true

	raw, err := os.ReadFile(filepath.Join(recordingDir(t, r.dir), "screens.txt"))
	if err != nil {
		t.Fatal(err)
	}
	screens := string(raw)
	if !strings.Contains(screens, "AGENT SELECT · Ascent · Defenders · Competitive") || !strings.Contains(screens, "Dia1 50") {
		t.Errorf("screens.txt doesn't hold the agent select table:\n%s", screens)
	}
	for _, real := range []string{"Kestrel", "AllyOne", "AllyTwo", "EUW"} {
		if strings.Contains(screens, real) {
			t.Errorf("screens.txt shows the real name %q", real)
		}
	}
	if !strings.Contains(screens, "self#self") {
		t.Errorf("your own row should show the self fake:\n%s", screens)
	}
	if strings.Contains(screens, "\x1b") {
		t.Error("screens.txt holds escape codes")
	}
}
