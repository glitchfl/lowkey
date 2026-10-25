package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/glitchfl/lowkey/internal/record"
	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/render"
	"github.com/glitchfl/lowkey/internal/static"
	"github.com/glitchfl/lowkey/internal/stats"
)

var meColumns = []string{"name", "rank", "peak", "last_act", "wr", "wl"}

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
	l, err := connect(ctx, env, rec)
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

	body, err := l.client.Get(ctx, remote.Shared, "/content-service/v3/content", nil)
	if err != nil {
		fmt.Fprintf(out, "lowkey me: content: %v\n", err)
		return 1
	}
	acts, err := stats.DecodeContent(body)
	if err != nil {
		fmt.Fprintf(out, "lowkey me: %v\n", err)
		return 1
	}
	body, err = l.client.Get(ctx, remote.PD, "/mmr/v1/players/"+l.self, nil)
	if err != nil {
		fmt.Fprintf(out, "lowkey me: rank data: %v\n", err)
		return 1
	}
	mmr, err := stats.DecodeMMR(body)
	if err != nil {
		fmt.Fprintf(out, "lowkey me: %v\n", err)
		return 1
	}

	row := render.Row{}
	rankCells(row, stats.RankOf(mmr, acts, mode.Queue, data.CurrentTier), data, mode)
	screen := render.Screen{
		Header:   "YOU · " + mode.Name,
		Sections: []render.Section{{Rows: []render.Row{row}}},
	}
	for _, line := range screen.Lines(render.Columns(meColumns), env.Console.Width(), env.Console.Terminal) {
		fmt.Fprintln(out, line)
	}
	return 0
}
