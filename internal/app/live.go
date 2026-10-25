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
	"github.com/glitchfl/lowkey/internal/remote"
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

const (
	defaultMatches = 13
	recentEvents   = 32
)

// live is lowkey's normal mode - everything in it runs on the loop goroutine
type live struct {
	ctx  context.Context
	env  Env
	loop *loop
	data *static.Data
	rec  *record.Recorder
	log  *errlog
	eng  *engine

	lock    riot.Lockfile
	conn    *connection
	gen     int
	self    string
	retries int

	guard    *remote.Guard
	client   *remote.Client
	presence riot.Private
	region   string
	shard    string
	version  string
	logRead  bool

	phase       phase
	midMatch    bool
	notReady    bool
	untracked   string
	problem     string
	recent      []string
	rateLimit   rateState
	unreachable bool

	lobby      *lobby
	last       *lobby
	history    []pastLobby
	everHidden map[string]bool
	sess       *session
	rosterAt   time.Time
	rosterStop func() bool
	cutoffStop func() bool

	frame string
}

type rateState int

const (
	rateFine rateState = iota
	rateBackingOff
	rateGaveUp
)

type connection struct {
	local  *riot.Local
	events *riot.Events
	cancel context.CancelFunc
}

func runLive(ctx context.Context, env Env, opts options, setup platform.Setup) int {
	lv := &live{ctx: ctx, env: env, loop: newLoop(), data: static.Embedded(), log: newErrlog(env.Dir, env.Clock.Now),
		everHidden: map[string]bool{}}
	lv.eng = newEngine(nil, lv.loop.post)
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
		lv.toIdle()
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
	if lv.eng.sched != nil {
		lv.eng.cancel()
		lv.eng.sched.close()
		lv.eng.sched = nil
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
	lv.log.secret(lf.Password)
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
	lv.guard = remote.NewGuard()
	lv.guard.SetSelf(lv.self)
	lv.client = remote.New(remote.Config{
		Endpoints: lv.env.endpoints(),
		Transport: lv.env.Servers.Transport,
		UserAgent: "lowkey/" + lv.env.Version,
		Now:       lv.env.Clock.Now,
		Tokens:    tokenSource(lv.conn.local),
		Guard:     lv.guard,
		Observe:   remoteObserver(lv.rec),
	})
	lv.eng.sched = newScheduler(lv.client, lv.env.Clock, lv.log)
	lv.eng.sched.rateLimited = func(paused, gaveUp bool) {
		lv.loop.post(func() {
			switch {
			case gaveUp:
				lv.rateLimit = rateGaveUp
			case lv.rateLimit == rateGaveUp:
			case paused:
				lv.rateLimit = rateBackingOff
			default:
				lv.rateLimit = rateFine
			}
		})
	}
	lv.eng.sched.outcome = func(err error) {
		var ue *remote.UnreachableError
		unreachable := errors.As(err, &ue)
		lv.loop.post(func() { lv.unreachable = unreachable })
	}
	lv.region, lv.shard, lv.version = "", "", ""
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
		lv.cancelLobby()
		lv.midMatch, lv.untracked = false, ""
		lv.region, lv.logRead = "", false
		lv.setPhase(phaseWaiting)
		return
	}
	lv.presence = priv
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

func (lv *live) toIdle() {
	lv.lobby, lv.midMatch, lv.untracked = nil, false, ""
	lv.setPhase(phaseIdle)
}

func (lv *live) toMenus() {
	from := lv.phase
	switch from {
	case phaseAgentSelect:
		lv.remember(lv.lobby)
		lv.cancelLobby()
	case phaseMatchLoad, phaseInRound:
		lv.matchEnded()
	}
	lv.midMatch, lv.untracked, lv.problem = false, "", ""
	lv.setPhase(phaseMenus)
	if lv.sess == nil && (from == phaseIdle || from == phaseWaiting) {
		lv.refreshSessionInMenus()
	}
}

func (lv *live) onPregame(matchID string) {
	switch lv.phase {
	case phaseMenus, phaseWaiting, phaseIdle:
		lv.startAgentSelect(matchID)
	case phaseAgentSelect:
		switch {
		case lv.lobby == nil:
		case lv.lobby.matchID == "" && matchID != "":
			lv.lobby.matchID = matchID
			lv.fetchRoster(lv.lobby)
		default:
			lv.refreshRoster()
		}
	}
}

func (lv *live) onCoreGame(matchID string) {
	switch {
	case lv.phase == phaseMenus || lv.phase == phaseAgentSelect || lv.phase == phaseWaiting:
		lv.startMatchLoad(matchID)
	case lv.phase == phaseMatchLoad && lv.lobby != nil && lv.lobby.matchID == "" && matchID != "":
		lv.lobby.matchID = matchID
		lv.fetchRoster(lv.lobby)
	}
}

func (lv *live) setPhase(p phase) {
	lv.phase = p
}

// ensureTarget makes sure lowkey knows the region, shard and client version before it asks Riot anything
func (lv *live) ensureTarget(then func()) {
	if lv.region == "" {
		gen, local := lv.gen, lv.conn.local
		go func() {
			sessions, err := local.Sessions(lv.ctx)
			lv.loop.post(func() {
				if gen != lv.gen {
					return
				}
				if args, ok := riot.ValorantArgs(sessions); err == nil && ok {
					lv.region = riot.RegionFromArgs(args)
				}
				shard, known := riot.Shard(lv.region)
				if !known {
					lv.region = ""
					lv.problem = "region not detected - is Valorant running?"
					return
				}
				lv.shard = shard
				lv.ensureTarget(then)
			})
		}()
		return
	}
	if riot.ValidVersion(lv.presence.PartyClientVersion) {
		lv.version = lv.presence.PartyClientVersion
	} else if !lv.logRead {
		// the log only changes when Valorant restarts, so it's read once per game session
		lv.logRead = true
		if v, ok := riot.VersionFromLog(lv.env.GameLog); ok {
			lv.version = v
		}
	}
	if lv.version != "" {
		lv.client.SetTarget(lv.region, lv.shard, lv.version)
		then()
		return
	}
	lv.client.SetTarget(lv.region, lv.shard, "")
	lv.eng.submit(&job{prio: prioRoster, svc: remote.Static, path: "/v1/version"}, func(body []byte, err error) {
		if v, ok := staticVersion(body); err == nil && ok {
			lv.version = v
			lv.client.SetTarget(lv.region, lv.shard, v)
			then()
			return
		}
		lv.problem = "no valid client version from presence, ShooterGame.log or valorant-api.com"
	})
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
		if lv.last != nil {
			lv.lobbyScreen(&s, lv.last, "LAST MATCH")
		}
		s.Session = lv.sess.line()
	case lv.untracked != "":
		s.Status = []string{"not tracking " + lv.untracked + " - waiting for the next match"}
	case lv.midMatch:
		s.Status = []string{"match in progress - lowkey waits for it to end"}
	case lv.lobby != nil && len(lv.lobby.members) == 0 && lv.unreachable && lv.last != nil:
		lv.lobbyScreen(&s, lv.last, "LAST MATCH")
		s.Session = lv.sess.line()
	case lv.lobby != nil:
		title := map[phase]string{phaseAgentSelect: "AGENT SELECT", phaseMatchLoad: "MATCH LOADING", phaseInRound: "IN MATCH"}[lv.phase]
		lv.lobbyScreen(&s, lv.lobby, title)
		s.Session = lv.sess.line()
	case lv.phase == phaseAgentSelect:
		s.Header = "AGENT SELECT"
	case lv.phase == phaseMatchLoad:
		s.Header = "MATCH LOADING"
	default:
		s.Header = "IN MATCH"
	}
	switch {
	case lv.unreachable:
		s.Status = append(s.Status, "can't reach Riot's servers - showing the last table")
	case lv.problem != "":
		s.Status = append(s.Status, lv.problem)
	}
	switch lv.rateLimit {
	case rateBackingOff:
		s.Status = append(s.Status, "Riot is rate-limiting - backing off")
	case rateGaveUp:
		s.Status = append(s.Status, "Riot is rate-limiting - some stats are missing")
	}
	return s
}

// redraw writes the whole screen as one write, in place on a terminal
func (lv *live) redraw() {
	lines := lv.screen().Lines(render.Columns(render.Keys()), lv.env.Console.Width(), lv.env.Console.Terminal)
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
