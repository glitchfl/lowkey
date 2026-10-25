// Package e2e runs the real lowkey binary against fakeriot - CI points LOWKEY_EXE at the Windows build
package e2e

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

// shortened settings so a whole match fits in seconds of real time
const fastSettings = "cutoff_seconds = 2\n"

type lowkey struct {
	t      *testing.T
	cmd    *exec.Cmd
	dir    string
	mu     sync.Mutex
	frames []string
	timed  []fakeriot.Frame
	exited chan struct{}
}

func exe(t *testing.T) string {
	t.Helper()
	path := os.Getenv("LOWKEY_EXE")
	if path == "" {
		t.Skip("set LOWKEY_EXE to a built lowkey binary to run the out-of-process suite")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func startRiot(t *testing.T, world *fakeriot.World) *fakeriot.Server {
	t.Helper()
	fr, err := fakeriot.Start(fakeriot.Options{World: world})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		checkPolicy(t, fr)
		fr.Close()
	})
	return fr
}

func startLowkey(t *testing.T, fr *fakeriot.Server, settings string, args ...string) *lowkey {
	t.Helper()
	path := exe(t)
	dir := t.TempDir()
	if settings != "" {
		if err := os.WriteFile(filepath.Join(dir, "lowkey.toml"), []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ca := filepath.Join(dir, "fakeriot-ca.pem")
	if err := os.WriteFile(ca, fr.CAPEM(), 0o644); err != nil {
		t.Fatal(err)
	}
	urls := fr.URLs()
	cmd := exec.Command(path, args...)
	cmd.Env = append(os.Environ(),
		"LOWKEY_LOCKFILE="+fr.Lockfile(), "LOWKEY_LOCAL_CA="+ca, "LOWKEY_DIR="+dir,
		"LOWKEY_PD_URL="+urls.PD, "LOWKEY_GLZ_URL="+urls.GLZ, "LOWKEY_SHARED_URL="+urls.Shared, "LOWKEY_STATIC_URL="+urls.Static,
	)
	l := run(t, cmd, dir)
	t.Cleanup(func() {
		l.stop()
		l.checkQuiet(fr)
	})
	return l
}

func run(t *testing.T, cmd *exec.Cmd, dir string) *lowkey {
	t.Helper()
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	l := &lowkey{t: t, cmd: cmd, dir: dir, exited: make(chan struct{})}
	go func() {
		defer close(l.exited)
		r := bufio.NewReader(out)
		var b strings.Builder
		for {
			// a non-terminal frame ends with a form feed line
			chunk, err := r.ReadString('\f')
			b.WriteString(chunk)
			if strings.HasSuffix(chunk, "\f") {
				frame := strings.TrimSuffix(b.String(), "\f")
				l.mu.Lock()
				l.frames = append(l.frames, frame)
				l.timed = append(l.timed, fakeriot.Frame{At: time.Now(), Text: frame})
				l.mu.Unlock()
				b.Reset()
			}
			if err != nil {
				cmd.Wait()
				return
			}
		}
	}()
	t.Cleanup(l.stop)
	return l
}

func (l *lowkey) stop() {
	select {
	case <-l.exited:
		return
	default:
	}
	l.cmd.Process.Kill()
	<-l.exited
}

func (l *lowkey) last() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.frames) == 0 {
		return ""
	}
	return l.frames[len(l.frames)-1]
}

// screen waits in real time until the latest frame shows every text
func (l *lowkey) screen(texts ...string) string {
	l.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		frame := l.last()
		ok := true
		for _, s := range texts {
			if !strings.Contains(frame, s) {
				ok = false
			}
		}
		if ok {
			return frame
		}
		if time.Now().After(deadline) {
			l.t.Fatalf("lowkey never showed %q - last frame:\n%s", texts, frame)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// quiet waits until fakeriot has nothing in flight for a moment
func quiet(t *testing.T, fr *fakeriot.Server) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	still := 0
	for still < 10 {
		if time.Now().After(deadline) {
			t.Fatal("Riot never went quiet")
		}
		if fr.InFlight() == 0 {
			still++
		} else {
			still = 0
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func subscribed(t *testing.T, fr *fakeriot.Server) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for fr.Subscribers(fakeriot.EventMessaging) == 0 || fr.Subscribers(fakeriot.EventPresences) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("lowkey never subscribed to Riot Client events")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func checkPolicy(t *testing.T, fr *fakeriot.Server) {
	for _, v := range fr.Violations() {
		t.Errorf("policy: %s", v)
	}
}

// checkQuiet holds the real exe to the screen it showed - no requests while it said it was idle or mid-round
func (l *lowkey) checkQuiet(fr *fakeriot.Server) {
	l.mu.Lock()
	frames := append([]fakeriot.Frame(nil), l.timed...)
	l.mu.Unlock()
	for _, v := range fr.QuietViolations(frames) {
		l.t.Errorf("policy: %s", v)
	}
}

func count(fr *fakeriot.Server, prefix string) int {
	n := 0
	for _, req := range fr.RemoteRequests() {
		if strings.HasPrefix(req.Path, prefix) {
			n++
		}
	}
	return n
}

func describe(fr *fakeriot.Server) string {
	var b strings.Builder
	for _, req := range fr.RemoteRequests() {
		fmt.Fprintf(&b, "%s %s %d\n", req.Service, req.Path, req.Status)
	}
	return b.String()
}
