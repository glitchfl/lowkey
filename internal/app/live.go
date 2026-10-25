package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glitchfl/lowkey/internal/platform"
	"github.com/glitchfl/lowkey/internal/record"
	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/static"
)

type phase int

const (
	phaseIdle phase = iota
	phaseWaiting
	phaseMenus
	phaseAgentSelect
	phaseMatchLoad
	phaseInRound
)

// live is lowkey's normal mode - everything in it runs on the loop goroutine
type live struct {
	ctx  context.Context
	env  Env
	loop *loop
	data *static.Data
	rec  *record.Recorder

	lock    riot.Lockfile
	conn    *connection
	gen     int
	self    string
	retries int

	phase    phase
	midMatch bool
	notReady bool
	recent   []string

	frame string
}

type connection struct {
	local  *riot.Local
	events *riot.Events
	cancel context.CancelFunc
}

const recentEvents = 32

func runLive(ctx context.Context, env Env, opts options, setup platform.Setup) int {
	lv := &live{ctx: ctx, env: env, loop: newLoop(), data: static.Embedded()}
	if opts.record {
		rec, err := startRecording(env, lv.data)
		if err != nil {
			fmt.Fprintf(env.Console.Out, "can't start a recording: %v\n", err)
			return 1
		}
		defer rec.Close()
		lv.rec = rec
	}
	if err := watchLockfile(ctx, env.Lockfile, func() { lv.loop.post(lv.checkLockfile) }); err != nil {
		fmt.Fprintf(env.Console.Out, "can't watch for the Riot Client: %v\n", err)
		return 1
	}
	lv.loop.post(lv.checkLockfile)
	lv.loop.run(ctx, func() bool { return false }, lv.redraw)
	lv.disconnect()
	return 0
}

func (lv *live) checkLockfile() {
	raw, err := os.ReadFile(lv.env.Lockfile)
	if err != nil {
		lv.disconnect()
		lv.lock = riot.Lockfile{}
		lv.setPhase(phaseIdle)
		return
	}
	lf, err := riot.ParseLockfile(raw)
	if err != nil {
		// a half-written lockfile - the next notification brings the rest
		return
	}
	if lf == lv.lock && lv.conn != nil {
		return
	}
	lv.disconnect()
	lv.lock = lf
	lv.connect(lf, raw)
}

func (lv *live) disconnect() {
	if lv.conn == nil {
		return
	}
	lv.conn.cancel()
	if lv.conn.events != nil {
		lv.conn.events.Close()
	}
	lv.conn.local.Close()
	lv.conn = nil
	lv.gen++
}

func (lv *live) connect(lf riot.Lockfile, raw []byte) {
	lv.gen++
	gen := lv.gen
	ctx, cancel := context.WithCancel(lv.ctx)
	local := riot.NewLocal(lf, lv.env.LocalRoots)
	if lv.rec != nil {
		lv.rec.Lockfile(string(raw), lf.Password)
		local.Observe = lv.rec.Local
		local.Rejected = lv.rec.Chain
	}
	lv.conn = &connection{local: local, cancel: cancel}
	go func() {
		ent, err := local.Entitlements(ctx)
		if err != nil {
			lv.loop.post(func() { lv.connectFailed(gen, err) })
			return
		}
		events, err := local.Events(ctx, riot.EventMessaging, riot.EventPresences)
		if err != nil {
			lv.loop.post(func() { lv.connectFailed(gen, err) })
			return
		}
		list, err := local.Presences(ctx)
		if err != nil {
			events.Close()
			lv.loop.post(func() { lv.connectFailed(gen, err) })
			return
		}
		lv.loop.post(func() { lv.connected(gen, ent, events, list) })
		for {
			ev, err := events.Next(ctx)
			if err != nil {
				lv.loop.post(func() { lv.lost(gen) })
				return
			}
			lv.loop.post(func() { lv.event(gen, ev) })
		}
	}()
}

// connectFailed retries with backoff and re-reads the lockfile each time in case the port changed
func (lv *live) connectFailed(gen int, err error) {
	if gen != lv.gen {
		return
	}
	lv.notReady = errors.Is(err, riot.ErrTokenNotReady)
	lv.disconnect()
	lv.retries++
	wait := time.Duration(min(lv.retries, 15)) * 2 * time.Second
	if lv.notReady {
		wait = 2 * time.Second
	}
	retry := lv.gen
	lv.env.Clock.AfterFunc(wait, func() {
		lv.loop.post(func() {
			if retry == lv.gen && lv.conn == nil {
				lv.checkLockfile()
			}
		})
	})
}

func (lv *live) connected(gen int, ent riot.Entitlements, events *riot.Events, list []riot.Presence) {
	if gen != lv.gen {
		events.Close()
		return
	}
	lv.conn.events = events
	lv.retries, lv.notReady = 0, false
	lv.self = strings.ToLower(ent.Subject)
	priv, present := riot.OwnValorant(list, ent.Subject)
	lv.applyPresence(priv, present, true)
}

// lost is the WebSocket closing - the lockfile says whether the Riot Client quit or moved to a new port
func (lv *live) lost(gen int) {
	if gen != lv.gen {
		return
	}
	lv.disconnect()
	lv.checkLockfile()
}

func (lv *live) event(gen int, ev riot.Event) {
	if gen != lv.gen {
		return
	}
	if lv.rec != nil {
		lv.rec.Event(ev.Name, ev.Raw)
	}
	switch ev.Name {
	case riot.EventPresences:
		if priv, present, mentioned := riot.OwnPresence(ev, lv.self); mentioned {
			lv.applyPresence(priv, present, false)
		}
	case riot.EventMessaging:
		lc, ok := riot.ParseLifecycle(ev)
		if !ok || lv.duplicate(ev.DedupeKey()) {
			return
		}
		if lc.Pregame {
			lv.onPregame(lc.MatchID)
		} else {
			lv.onCoreGame(lc.MatchID)
		}
	}
}

func (lv *live) duplicate(key string) bool {
	for _, k := range lv.recent {
		if k == key {
			return true
		}
	}
	lv.recent = append(lv.recent, key)
	if len(lv.recent) > recentEvents {
		lv.recent = lv.recent[1:]
	}
	return false
}

// applyPresence turns your Valorant presence into a phase - INGAME right after connecting means lowkey opened mid-match
func (lv *live) applyPresence(priv riot.Private, present, startup bool) {
	if !present {
		lv.midMatch = false
		lv.setPhase(phaseWaiting)
		return
	}
	switch strings.ToUpper(priv.SessionLoopState) {
	case "MENUS":
		lv.toMenus()
	case "PREGAME":
		if startup || lv.phase == phaseMenus || lv.phase == phaseWaiting {
			lv.onPregame("")
		}
	case "INGAME":
		switch {
		case startup || lv.phase == phaseWaiting || lv.phase == phaseIdle:
			lv.midMatch = true
			lv.setPhase(phaseInRound)
		case lv.phase == phaseMenus || lv.phase == phaseAgentSelect:
			lv.onCoreGame("")
		}
	default:
		if lv.phase == phaseIdle || lv.phase == phaseWaiting {
			lv.setPhase(phaseMenus)
		}
	}
}

func (lv *live) toMenus() {
	lv.midMatch = false
	lv.setPhase(phaseMenus)
}

func (lv *live) onPregame(matchID string) {
	if lv.phase == phaseMenus || lv.phase == phaseWaiting || lv.phase == phaseIdle {
		lv.setPhase(phaseAgentSelect)
	}
}

func (lv *live) onCoreGame(matchID string) {
	if lv.phase == phaseMenus || lv.phase == phaseAgentSelect || lv.phase == phaseWaiting {
		lv.setPhase(phaseMatchLoad)
	}
}

func (lv *live) setPhase(p phase) {
	lv.phase = p
}

func (lv *live) screen() render.Screen {
	var s render.Screen
	switch {
	case lv.phase == phaseIdle && lv.notReady:
		s.Status = []string{"waiting for the Riot Client to log in"}
	case lv.phase == phaseIdle:
		s.Status = []string{"waiting for the Riot Client"}
	case lv.phase == phaseWaiting:
		s.Status = []string{"waiting for Valorant"}
	case lv.phase == phaseMenus:
		s.Status = []string{"waiting for the next match"}
	case lv.phase == phaseAgentSelect:
		s.Header = "AGENT SELECT"
	case lv.phase == phaseMatchLoad:
		s.Header = "MATCH LOADING"
	case lv.midMatch:
		s.Status = []string{"match in progress - lowkey waits for it to end"}
	default:
		s.Header = "IN MATCH"
	}
	return s
}

// redraw writes the whole screen as one write, in place on a terminal
func (lv *live) redraw() {
	lines := lv.screen().Lines(nil, lv.env.Console.Width(), lv.env.Console.Terminal)
	var frame string
	if lv.env.Console.Terminal {
		var b strings.Builder
		b.WriteString("\x1b[H")
		for _, l := range lines {
			b.WriteString(l)
			b.WriteString("\x1b[K\n")
		}
		b.WriteString("\x1b[J")
		frame = b.String()
	} else {
		frame = strings.Join(lines, "\n") + "\n\f\n"
	}
	if frame == lv.frame {
		return
	}
	lv.frame = frame
	lv.env.Console.Out.Write([]byte(frame))
}
