package app_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glitchfl/lowkey/internal/app"
	"github.com/glitchfl/lowkey/internal/fakeriot"
)

var testZone = time.FixedZone("UTC+2", 2*60*60)

// 20:00 local on 2026-10-25, well past the 6am session reset
var testStart = time.Date(2026, 10, 25, 20, 0, 0, 0, testZone)

type console struct {
	mu       sync.Mutex
	frames   []string
	width    int
	terminal bool
}

func (c *console) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = append(c.frames, string(p))
	return len(p), nil
}

func (c *console) Width() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.width
}

func (c *console) setWidth(w int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.width = w
}

func (c *console) output() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.frames, "")
}

type rig struct {
	t     *testing.T
	fr    *fakeriot.Server
	clock *fakeriot.Clock
	con   *console
	dir   string
	env   app.Env
	stop  func()
}

func newRig(t *testing.T, setup ...func(w *fakeriot.World)) *rig {
	t.Helper()
	return newRigWith(t, fakeriot.Options{}, setup...)
}

func newRigWith(t *testing.T, opts fakeriot.Options, setup ...func(w *fakeriot.World)) *rig {
	t.Helper()
	return newRigAt(t, testStart, opts, setup...)
}

func newRigAt(t *testing.T, start time.Time, opts fakeriot.Options, setup ...func(w *fakeriot.World)) *rig {
	t.Helper()
	clock := fakeriot.NewClock(start)
	world := fakeriot.NewWorld(clock.Now())
	for _, f := range setup {
		f(world)
	}
	opts.Clock, opts.World = clock, world
	fr, err := fakeriot.Start(opts)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, fr: fr, clock: clock, con: &console{width: 120}, dir: t.TempDir()}
	t.Cleanup(func() {
		r.checkPolicy()
		fr.Close()
	})
	urls := fr.URLs()
	r.env = app.Env{
		Lockfile:   fr.Lockfile(),
		LocalRoots: fr.Roots(),
		Servers:    app.Servers{PD: urls.PD, GLZ: urls.GLZ, Shared: urls.Shared, Static: urls.Static},
		Clock:      clock,
		Console:    app.Console{Out: r.con, Width: r.con.Width},
		Dir:        r.dir,
	}
	return r
}

// run executes a one-shot command and returns everything it printed
func (r *rig) run(args ...string) (string, int) {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	code := app.Run(ctx, r.env, args)
	return r.con.output(), code
}

// line finds the output line for a doctor item
func line(t *testing.T, out, item string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && (f[0] == "PASS" || f[0] == "FAIL") && strings.HasPrefix(strings.TrimSpace(l[4:]), item) {
			return l
		}
	}
	t.Fatalf("no doctor line for %q in:\n%s", item, out)
	return ""
}

// eventually polls a condition the way a person watching the screen would
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitHeld waits until n requests are parked on a Hold fault and returns their paths
func waitHeld(t *testing.T, fr *fakeriot.Server, n int) []string {
	t.Helper()
	var paths []string
	eventually(t, fmt.Sprintf("%d held requests", n), func() bool {
		held := fr.Held()
		if len(held) != n {
			return false
		}
		paths = paths[:0]
		for _, h := range held {
			paths = append(paths, h.Path)
		}
		return true
	})
	sort.Strings(paths)
	return paths
}

// start runs a command in the background so the test can step fakeriot meanwhile
func (r *rig) start(args ...string) <-chan int {
	done := make(chan int, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		done <- app.Run(ctx, r.env, args)
	}()
	return done
}

func (c *console) framesCopy() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.frames...)
}

func (c *console) last() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.frames) == 0 {
		return ""
	}
	return c.frames[len(c.frames)-1]
}

// live starts lowkey's normal mode and stops it when the test ends
func (r *rig) live() {
	r.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- app.Run(ctx, r.env, nil) }()
	var once sync.Once
	r.stop = func() {
		once.Do(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				r.t.Error("lowkey didn't stop when its window closed")
			}
		})
	}
	r.t.Cleanup(r.stop)
}

// restart closes lowkey's window and opens it again against the same Riot Client
func (r *rig) restart() *rig {
	r.t.Helper()
	r.stop()
	r.live()
	return r
}

// screen waits until the latest frame shows every piece of text and returns that frame
func (r *rig) screen(texts ...string) string {
	r.t.Helper()
	var frame string
	eventually(r.t, fmt.Sprintf("a screen showing %q", texts), func() bool {
		frame = r.con.last()
		for _, s := range texts {
			if !strings.Contains(frame, s) {
				return false
			}
		}
		return true
	})
	return frame
}

// subscribed waits for lowkey's WebSocket subscriptions so emitted events reach it
func (r *rig) subscribed() {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.fr.WaitSubscribed(ctx, fakeriot.EventMessaging, fakeriot.EventPresences); err != nil {
		r.t.Fatalf("lowkey never subscribed to Riot Client events: %v", err)
	}
}

// presence changes your presence and pushes it like the chat service does
func (r *rig) presence(f func(w *fakeriot.World)) {
	r.t.Helper()
	r.fr.Update(f)
	r.fr.EmitPresence()
}

// settle gives lowkey a moment of real time to act on whatever just happened
func settle() { time.Sleep(50 * time.Millisecond) }

// settled waits for the texts, then for Riot to go quiet and the screen to stop changing
func (r *rig) settled(texts ...string) string {
	r.t.Helper()
	r.screen(texts...)
	var frame string
	eventually(r.t, "the screen to settle", func() bool {
		if r.fr.InFlight() != 0 {
			return false
		}
		frame = r.con.last()
		time.Sleep(30 * time.Millisecond)
		return r.fr.InFlight() == 0 && r.con.last() == frame
	})
	return frame
}

// checkPolicy runs after every test: whatever happened, Riot must never have seen a forbidden request
func (r *rig) checkPolicy() {
	t := r.t
	hidden := r.fr.Hidden()
	self := r.fr.Self()
	for _, req := range r.fr.RemoteRequests() {
		where := req.Path + "?" + req.Query.Encode()
		low := strings.ToLower(where)
		for _, h := range hidden {
			if strings.Contains(low, strings.ToLower(h)) {
				t.Errorf("policy: a request referenced the hidden player %s: %s", h, where)
			}
		}
		parts := strings.Split(strings.Trim(req.Path, "/"), "/")
		switch {
		case req.Method != "GET":
			t.Errorf("policy: a %s request was sent: %s", req.Method, where)
		case strings.Contains(low, "name-service"):
			t.Errorf("policy: the name service was called: %s", where)
		case strings.Contains(low, "/pregame/") && (strings.Contains(low, "/select/") || strings.Contains(low, "/lock/")):
			t.Errorf("policy: an agent select action was sent: %s", where)
		case len(parts) >= 4 && parts[0] == "parties" && parts[2] == "players" && parts[3] != self:
			t.Errorf("policy: party data was asked for someone else: %s", where)
		}
	}
}
