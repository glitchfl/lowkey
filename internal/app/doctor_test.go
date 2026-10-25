package app_test

import (
	"os"
	"strings"
	"testing"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

func lockfilePassword(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(data), ":")
	if len(parts) != 5 {
		t.Fatalf("odd lockfile %q", data)
	}
	return parts[3]
}

var doctorItems = []string{"lockfile", "local API", "entitlements", "phase", "session", "client version", "rank data", "static data"}

func TestDoctorPassesEveryCheckAgainstAHealthyRiotClient(t *testing.T) {
	r := newRig(t)
	out, code := r.run("doctor")
	if code != 0 {
		t.Fatalf("exit code %d, want 0:\n%s", code, out)
	}
	for _, item := range doctorItems {
		if l := line(t, out, item); !strings.HasPrefix(l, "PASS") {
			t.Errorf("want a pass line for %s, got %q", item, l)
		}
	}
}

func TestDoctorNamesTheProblemAndFailsWhenSomethingIsWrong(t *testing.T) {
	cases := []struct {
		name    string
		break_  func(r *rig)
		item    string
		problem string
	}{
		{"no lockfile", func(r *rig) { r.fr.StopClient() }, "lockfile", "not found"},
		{"wrong password", func(r *rig) {
			if err := r.fr.SetLockfilePassword("not-the-password"); err != nil {
				t.Fatal(err)
			}
		}, "local API", "rejected the lockfile password"},
		{"rank request fails", func(r *rig) {
			r.fr.AddFault(fakeriot.Fault{Service: "pd", Path: "/mmr/v1/players/", Status: 500})
		}, "rank data", "HTTP 500"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			c.break_(r)
			out, code := r.run("doctor")
			if code == 0 {
				t.Fatalf("exit code 0, want non-zero:\n%s", out)
			}
			l := line(t, out, c.item)
			if !strings.HasPrefix(l, "FAIL") || !strings.Contains(l, c.problem) {
				t.Errorf("want a fail line for %s naming %q, got %q", c.item, c.problem, l)
			}
		})
	}
}

func TestDoctorNeverPrintsTokensOrThePassword(t *testing.T) {
	r := newRig(t)
	out, _ := r.run("doctor")
	pw := lockfilePassword(t, r.fr.Lockfile())
	secrets := []string{pw}
	for _, req := range r.fr.Requests() {
		secrets = append(secrets, req.Header.Get("Authorization"), req.Header.Get("X-Riot-Entitlements-JWT"))
		if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			secrets = append(secrets, strings.TrimPrefix(auth, "Bearer "))
		}
	}
	for _, s := range secrets {
		if s != "" && strings.Contains(out, s) {
			t.Errorf("doctor printed a secret %.12q...", s)
		}
	}
	if strings.Contains(out, "eyJ") {
		t.Errorf("doctor printed something JWT-shaped:\n%s", out)
	}
}

// the standard base64 of the spec's platform JSON - worked out by hand
const wantPlatform = "eyJwbGF0Zm9ybVR5cGUiOiJQQyIsInBsYXRmb3JtT1MiOiJXaW5kb3dzIiwicGxhdGZvcm1PU1ZlcnNpb24iOiIxMC4wLjE5MDQyLjEuMjU2LjY0Yml0IiwicGxhdGZvcm1DaGlwc2V0IjoiVW5rbm93biJ9"

func jwtShaped(s string) bool { return len(strings.Split(s, ".")) == 3 && strings.HasPrefix(s, "eyJ") }

func TestEveryRiotServerRequestCarriesRiotsHeadersAndAnHonestUserAgent(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) { w.PresenceVersion = "" })
	r.env.Version = "v9.9.9"
	if out, code := r.run("doctor"); code != 0 {
		t.Fatalf("doctor failed:\n%s", out)
	}
	riotCalls, staticCalls := 0, 0
	for _, req := range r.fr.RemoteRequests() {
		if ua := req.Header.Get("User-Agent"); ua != "lowkey/v9.9.9" {
			t.Errorf("%s %s: User-Agent %q", req.Service, req.Path, ua)
		}
		if req.Service == "static" {
			staticCalls++
			if req.Header.Get("Authorization") != "" || req.Header.Get("X-Riot-Entitlements-JWT") != "" {
				t.Errorf("static data request %s carried Riot tokens", req.Path)
			}
			continue
		}
		riotCalls++
		if auth := req.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") || !jwtShaped(strings.TrimPrefix(auth, "Bearer ")) {
			t.Errorf("%s: Authorization isn't a bearer JWT", req.Path)
		}
		if !jwtShaped(req.Header.Get("X-Riot-Entitlements-JWT")) {
			t.Errorf("%s: X-Riot-Entitlements-JWT isn't a JWT", req.Path)
		}
		if got := req.Header.Get("X-Riot-ClientPlatform"); got != wantPlatform {
			t.Errorf("%s: X-Riot-ClientPlatform %q", req.Path, got)
		}
		if got := req.Header.Get("X-Riot-ClientVersion"); got != fakeriot.DefaultClientVersion {
			t.Errorf("%s: X-Riot-ClientVersion %q", req.Path, got)
		}
	}
	if riotCalls == 0 || staticCalls == 0 {
		t.Fatalf("want Riot server and static data requests, got %d and %d", riotCalls, staticCalls)
	}
}

func TestClientVersionComesFromPresenceThenValorantAPI(t *testing.T) {
	const older = "release-13.07-shipping-11-5500000"
	cases := []struct {
		name, presence, static string
		want                   string
	}{
		{"from presence", fakeriot.DefaultClientVersion, older, fakeriot.DefaultClientVersion + " (from presence)"},
		{"missing from presence", "", older, older + " (from valorant-api.com)"},
		{"manifest ID in presence", "13.8.0.5715960", older, older + " (from valorant-api.com)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, func(w *fakeriot.World) { w.PresenceVersion, w.StaticVersion = c.presence, c.static })
			out, code := r.run("doctor")
			if code != 0 {
				t.Fatalf("doctor failed:\n%s", out)
			}
			if l := line(t, out, "client version"); !strings.HasSuffix(l, c.want) {
				t.Errorf("got %q, want it to end in %q", l, c.want)
			}
		})
	}
	t.Run("nothing valid", func(t *testing.T) {
		r := newRig(t, func(w *fakeriot.World) { w.PresenceVersion, w.StaticVersion = "", "13.8.0.5715960" })
		out, code := r.run("doctor")
		if l := line(t, out, "client version"); code == 0 || !strings.HasPrefix(l, "FAIL") {
			t.Errorf("want a failed client version check, got %q (exit %d)", l, code)
		}
	})
}

func TestRegionAndShardComeFromTheLaunchArguments(t *testing.T) {
	shards := map[string]string{"na": "na", "latam": "na", "br": "na", "eu": "eu", "ap": "ap", "kr": "kr", "pbe": "pbe"}
	for region, shard := range shards {
		t.Run(region, func(t *testing.T) {
			r := newRig(t, func(w *fakeriot.World) { w.Region = region })
			out, code := r.run("doctor")
			if code != 0 {
				t.Fatalf("doctor failed:\n%s", out)
			}
			if l := line(t, out, "session"); !strings.HasSuffix(l, "region "+region+" · shard "+shard) {
				t.Errorf("got %q", l)
			}
			for _, req := range r.fr.RemoteRequests() {
				if req.Service == "pd" && req.Shard != shard {
					t.Errorf("rank data went to shard %q, want %q", req.Shard, shard)
				}
			}
		})
	}
}

func TestDoctorReportsTheRegionAsNotDetectedWithoutAValorantSession(t *testing.T) {
	r := newRig(t, func(w *fakeriot.World) { w.Valorant = false })
	out, code := r.run("doctor")
	if l := line(t, out, "session"); code == 0 || !strings.HasPrefix(l, "FAIL") || !strings.Contains(l, "region not detected") {
		t.Errorf("got %q (exit %d)", l, code)
	}
}

func TestPresenceIsReadInNestedFlatAndHybridShapes(t *testing.T) {
	shapes := map[string]fakeriot.Shape{"nested": fakeriot.Nested, "flat": fakeriot.Flat, "hybrid": fakeriot.Hybrid}
	phases := map[string]string{fakeriot.LoopMenus: "Menus", fakeriot.LoopPregame: "Agent select", fakeriot.LoopIngame: "In match"}
	for name, shape := range shapes {
		for loop, phase := range phases {
			t.Run(name+" "+loop, func(t *testing.T) {
				r := newRig(t, func(w *fakeriot.World) { w.Shape, w.Loop = shape, loop })
				out, code := r.run("doctor")
				if code != 0 {
					t.Fatalf("doctor failed:\n%s", out)
				}
				if l := line(t, out, "phase"); !strings.HasSuffix(l, phase+" ("+name+" presence)") {
					t.Errorf("got %q", l)
				}
				if l := line(t, out, "client version"); !strings.HasSuffix(l, "(from presence)") {
					t.Errorf("partyClientVersion wasn't read from the %s shape: %q", name, l)
				}
			})
		}
	}
}

func TestTheLocalAPIIsOnlyTrustedThroughTheSeam(t *testing.T) {
	r := newRig(t)
	r.env.LocalRoots = nil
	out, code := r.run("doctor")
	if l := line(t, out, "local API"); code == 0 || !strings.HasPrefix(l, "FAIL") || !strings.Contains(l, "isn't signed by Riot's root") {
		t.Errorf("fakeriot's certificate must not pass the production check, got %q (exit %d)", l, code)
	}
	if l := line(t, out, "local API"); !strings.Contains(l, "https://127.0.0.1:") {
		t.Errorf("local API must be 127.0.0.1, got %q", l)
	}
}

func TestASHA1SignedLocalCertificateIsAcceptedOnLoopback(t *testing.T) {
	r := newRigWith(t, fakeriot.Options{SHA1Cert: true})
	r.env.LocalRoots = nil
	if out, code := r.run("doctor"); code != 0 {
		t.Fatalf("doctor failed:\n%s", out)
	}
}

func TestDoctorReportsTheStaticDataSnapshotAgeEvenWithoutARiotClient(t *testing.T) {
	r := newRig(t)
	r.fr.StopClient()
	out, _ := r.run("doctor")
	l := line(t, out, "static data")
	if !strings.HasPrefix(l, "PASS") || !strings.Contains(l, "snapshot from 2026-10-25") || !strings.Contains(l, "0 days old") {
		t.Errorf("got %q", l)
	}
}

func gameLog(t *testing.T, version string) string {
	t.Helper()
	path := t.TempDir() + "/ShooterGame.log"
	content := "[2026.10.25-17.58.01:120][  0]LogInit: Display: Running engine for game: ShooterGame\n" +
		"[2026.10.25-17.58.01:204][  0]LogShooter: Display: CI server version: " + version + "\n" +
		"[2026.10.25-17.58.02:001][  0]LogShooter: Display: Branch: release-13.08\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClientVersionFallsBackToShooterGameLogBeforeValorantAPI(t *testing.T) {
	const fromLog = "release-13.08-shipping-20-5715961"
	cases := []struct {
		name, presence, logged string
		want                   string
	}{
		{"presence wins", fakeriot.DefaultClientVersion, fromLog, fakeriot.DefaultClientVersion + " (from presence)"},
		{"log before valorant-api.com", "", fromLog, fromLog + " (from ShooterGame.log)"},
		{"invalid log line", "", "13.8.0.5715960", fakeriot.DefaultClientVersion + " (from valorant-api.com)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, func(w *fakeriot.World) { w.PresenceVersion = c.presence })
			r.env.GameLog = gameLog(t, c.logged)
			out, code := r.run("doctor")
			if code != 0 {
				t.Fatalf("doctor failed:\n%s", out)
			}
			if l := line(t, out, "client version"); !strings.HasSuffix(l, c.want) {
				t.Errorf("got %q, want it to end in %q", l, c.want)
			}
			for _, req := range r.fr.RemoteRequests() {
				if req.Service == "static" && !strings.HasSuffix(c.want, "(from valorant-api.com)") {
					t.Errorf("asked valorant-api.com although an earlier source had the version")
				}
			}
		})
	}
}

func TestDoctorReportsTheConsole(t *testing.T) {
	r := newRig(t)
	r.con.setWidth(100)
	r.env.Console.Terminal = true
	out, _ := r.run("doctor")
	if l := line(t, out, "console"); !strings.HasPrefix(l, "PASS") || !strings.Contains(l, "width 100") || !strings.Contains(l, "ANSI on") {
		t.Errorf("got %q", l)
	}
}

func TestDoctorReportsPriorityAndEfficiencyMode(t *testing.T) {
	r := newRig(t)
	out, code := r.run("doctor")
	if code != 0 {
		t.Fatalf("doctor failed:\n%s", out)
	}
	for _, item := range []string{"priority", "efficiency"} {
		if l := line(t, out, item); !strings.HasPrefix(l, "PASS") {
			t.Errorf("got %q", l)
		}
	}
}
