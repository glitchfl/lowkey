package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/glitchfl/lowkey/internal/record"
	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/static"
)

// link is one connection to the Riot Client plus a client for Riot's servers
type link struct {
	local       *riot.Local
	client      *remote.Client
	guard       *remote.Guard
	self        string
	presence    riot.Private
	hasPresence bool
	gameLog     string
	region      string
	shard       string
	version     string
}

func connect(ctx context.Context, env Env, rec *record.Recorder) (*link, error) {
	raw, err := os.ReadFile(env.Lockfile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no lockfile at %s - is the Riot Client running?", env.Lockfile)
	}
	if err != nil {
		return nil, err
	}
	lf, err := riot.ParseLockfile(raw)
	if err != nil {
		return nil, err
	}
	local := riot.NewLocal(lf, env.LocalRoots)
	if rec != nil {
		rec.Lockfile(string(raw), lf.Password)
		local.Observe = rec.Local
	}
	ent, err := local.Entitlements(ctx)
	if err != nil {
		local.Close()
		return nil, fmt.Errorf("riot client entitlements: %w", err)
	}
	l := &link{local: local, self: strings.ToLower(ent.Subject), gameLog: env.GameLog}
	if list, err := local.Presences(ctx); err == nil {
		l.presence, l.hasPresence = riot.OwnValorant(list, ent.Subject)
	}
	if sessions, err := local.Sessions(ctx); err == nil {
		if args, ok := riot.ValorantArgs(sessions); ok {
			l.region = riot.RegionFromArgs(args)
		}
	}
	if l.region == "" {
		local.Close()
		return nil, errors.New("no Valorant session - region not detected")
	}
	shard, ok := riot.Shard(l.region)
	if !ok {
		local.Close()
		return nil, fmt.Errorf("region %q has no known shard", l.region)
	}
	l.shard = shard
	l.guard = remote.NewGuard()
	l.guard.SetSelf(ent.Subject)
	l.client = remote.New(remote.Config{
		Endpoints: env.endpoints(),
		Transport: env.Servers.Transport,
		UserAgent: "lowkey/" + env.Version,
		Now:       env.Clock.Now,
		Tokens:    tokenSource(local),
		Guard:     l.guard,
		Observe:   remoteObserver(rec),
	})
	l.client.SetTarget(l.region, l.shard, "")
	return l, nil
}

// ensureVersion has to run inside an allowed window - its fallback asks valorant-api.com
func (l *link) ensureVersion(ctx context.Context) error {
	if l.version != "" {
		return nil
	}
	v, _ := detectVersion(ctx, l.client, l.presence, l.gameLog)
	if v == "" {
		return errors.New("no valid client version from presence, ShooterGame.log or valorant-api.com")
	}
	l.version = v
	l.client.SetTarget(l.region, l.shard, v)
	return nil
}

func (l *link) close() { l.local.Close() }

func remoteObserver(rec *record.Recorder) func(remote.Service, string, string, int, []byte) {
	if rec == nil {
		return nil
	}
	return func(svc remote.Service, path, query string, status int, body []byte) {
		rec.Remote(svc.String(), path, query, status, body)
	}
}

func startRecording(env Env, data *static.Data) (*record.Recorder, error) {
	return record.Start(env.Dir, env.Clock.Now, data.Public)
}
