package app_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

type recEntry struct {
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
