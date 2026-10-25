package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/glitchfl/lowkey/internal/record"
	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/settings"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

// these columns only mean something in a lobby
var lobbyOnly = map[string]bool{"party": true, "agent": true, "met": true}

func runMe(ctx context.Context, env Env, opts options) int {
	out := env.Console.Out
	key := opts.mode
	if key == "" {
		key = "competitive"
	}
	mode, ok := stats.ModeByKey(key)
	if !ok {
		fmt.Fprintf(out, "unknown mode %q - pick one of %s\n", key, strings.Join(stats.ModeKeys(), ", "))
		return 2
	}
	data := static.Embedded()
	cfg := settings.Open(env.Dir).Settings()
	var rec *record.Recorder
	if opts.record {
		var err error
		if rec, err = startRecording(env, data); err != nil {
			fmt.Fprintf(out, "can't start a recording: %v\n", err)
			return 1
		}
		defer func() {
			rec.Close()
			fmt.Fprintf(out, "recording saved in %s\n", rec.Dir)
		}()
	}
	l, err := connect(ctx, env, rec, cfg.Region)
	if err != nil {
		fmt.Fprintf(out, "lowkey me: %v\n", err)
		return 1
	}
	defer l.close()
	l.guard.SetWindow(remote.OneShot)
	if err := l.ensureVersion(ctx); err != nil {
		fmt.Fprintf(out, "lowkey me: %v\n", err)
		return 1
	}

	lp := newLoop()
	sched := newScheduler(l.client, env.Clock, newErrlog(env.Dir, env.Clock.Now))
	defer sched.close()
	eng := newEngine(sched, lp.post)
	eng.fetchActs()
	me := newPlayer(l.self, mode.Queue, cfg.Matches)
	eng.lookUp(me, wantRank|wantSample)
	sess := newSession(l.self, env.Clock.Now(), cfg.SessionResetHour)
	eng.refreshSession(sess, nil)
	if err := lp.run(ctx, func() bool { return eng.busy == 0 }, nil); err != nil {
		fmt.Fprintf(out, "lowkey me: %v\n", err)
		return 1
	}
	for what, err := range map[string]error{"content": eng.actsErr, "rank data": me.mmrErr, "match history": me.historyErr} {
		if err != nil {
			fmt.Fprintf(out, "lowkey me: %s: %v\n", what, err)
			return 1
		}
	}

	me.useSession(sess)
	row := playerRow(me, mode, eng.acts, data)
	screen := render.Screen{
		Header:   "YOU · " + mode.Name,
		Sections: []render.Section{{Rows: []render.Row{row}}},
		Session:  sess.line(),
	}
	var columns []string
	for _, c := range cfg.Columns {
		if !lobbyOnly[c] {
			columns = append(columns, c)
		}
	}
	for _, line := range screen.Lines(render.Columns(columns), env.Console.Width(), env.Console.Terminal) {
		fmt.Fprintln(out, line)
	}
	return 0
}
