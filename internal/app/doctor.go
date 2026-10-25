package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/glitchfl/lowkey/internal/platform"
	"github.com/glitchfl/lowkey/internal/record"
	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/settings"
	"github.com/glitchfl/lowkey/internal/static"
)

type doctor struct {
	env    Env
	setup  platform.Setup
	rec    *record.Recorder
	cfg    *settings.File
	failed bool
}

func (d *doctor) report(ok bool, item, detail string) {
	status := "PASS"
	if !ok {
		status = "FAIL"
		d.failed = true
	}
	fmt.Fprintf(d.env.Console.Out, "%s  %-15s %s\n", status, item, detail)
}

func (d *doctor) skip(needs string, items ...string) {
	for _, item := range items {
		d.report(false, item, "not checked - needs "+needs)
	}
}

func runDoctor(ctx context.Context, env Env, opts options, setup platform.Setup) int {
	d := &doctor{env: env, setup: setup, cfg: settings.Open(env.Dir)}
	if opts.record {
		rec, err := startRecording(env, static.Embedded())
		if err != nil {
			fmt.Fprintf(env.Console.Out, "can't start a recording: %v\n", err)
			return 1
		}
		defer rec.Close()
		d.rec = rec
	}
	fmt.Fprintf(env.Console.Out, "lowkey %s doctor\n", env.Version)
	d.check(ctx)
	d.offline()
	if d.rec != nil {
		fmt.Fprintf(env.Console.Out, "recording saved in %s\n", d.rec.Dir)
	}
	if d.failed {
		fmt.Fprintln(env.Console.Out, "some checks failed")
		return 1
	}
	fmt.Fprintln(env.Console.Out, "all checks passed")
	return 0
}

func (d *doctor) check(ctx context.Context) {
	env := d.env
	raw, err := os.ReadFile(env.Lockfile)
	var lf riot.Lockfile
	if err == nil {
		lf, err = riot.ParseLockfile(raw)
	}
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			d.report(false, "lockfile", fmt.Sprintf("not found at %s - is the Riot Client running?", env.Lockfile))
		} else {
			d.report(false, "lockfile", fmt.Sprintf("%s: %v", env.Lockfile, err))
		}
		d.skip("the lockfile", "local API", "entitlements", "phase", "session", "client version", "rank data")
		return
	}
	d.report(true, "lockfile", fmt.Sprintf("%s (port %d)", env.Lockfile, lf.Port))

	local := riot.NewLocal(lf, env.LocalRoots)
	defer local.Close()
	if d.rec != nil {
		d.rec.Lockfile(string(raw), lf.Password)
		local.Observe = d.rec.Local
		local.Rejected = d.rec.Chain
	}
	base := fmt.Sprintf("https://127.0.0.1:%d", lf.Port)
	ent, err := local.Entitlements(ctx)
	var se *riot.StatusError
	switch {
	case errors.Is(err, riot.ErrTokenNotReady):
		d.report(true, "local API", base+" answers with the lockfile password")
		d.report(false, "entitlements", "the Riot Client hasn't got a token yet - is it logged in?")
		d.skip("entitlements", "phase", "session", "client version", "rank data")
		return
	case errors.As(err, &se) && (se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden):
		d.report(false, "local API", fmt.Sprintf("%s rejected the lockfile password (HTTP %d)", base, se.Status))
		d.skip("the local API", "entitlements", "phase", "session", "client version", "rank data")
		return
	case errors.As(err, &se):
		d.report(true, "local API", base+" answers with the lockfile password")
		d.report(false, "entitlements", fmt.Sprintf("entitlements answered HTTP %d", se.Status))
		d.skip("entitlements", "phase", "session", "client version", "rank data")
		return
	case err != nil:
		d.report(false, "local API", fmt.Sprintf("can't reach %s: %v", base, err))
		d.skip("the local API", "entitlements", "phase", "session", "client version", "rank data")
		return
	}
	d.report(true, "local API", base+" answers with the lockfile password")
	expiry := "no expiry in the token"
	if exp, ok := riot.Expiry(ent.AccessToken); ok {
		expiry = fmt.Sprintf("expires in %dm", int(exp.Sub(env.Clock.Now()).Round(time.Minute).Minutes()))
	}
	d.report(true, "entitlements", "tokens obtained - "+expiry)

	presences, err := local.Presences(ctx)
	priv, havePresence := riot.OwnValorant(presences, ent.Subject)
	switch {
	case err != nil:
		d.report(false, "phase", fmt.Sprintf("presences: %v", err))
	case !havePresence:
		d.report(false, "phase", "no Valorant presence - is Valorant running?")
	default:
		d.report(true, "phase", fmt.Sprintf("%s (%s presence)", phaseFromLoop(priv.SessionLoopState), priv.Shape))
	}

	region, shard, ok := d.session(ctx, local)
	if !ok {
		d.skip("a region", "client version", "rank data")
		return
	}

	guard := remote.NewGuard()
	guard.SetSelf(ent.Subject)
	guard.SetWindow(remote.OneShot)
	client := remote.New(remote.Config{
		Endpoints: env.endpoints(),
		Transport: env.Servers.Transport,
		UserAgent: "lowkey/" + env.Version,
		Now:       env.Clock.Now,
		Tokens:    tokenSource(local),
		Guard:     guard,
		Observe:   remoteObserver(d.rec),
	})
	client.SetTarget(region, shard, "")

	version, source := detectVersion(ctx, client, priv, env.GameLog)
	if version == "" {
		d.report(false, "client version", "no valid release-NN.NN-shipping-N-N version from presence, ShooterGame.log or valorant-api.com")
		d.skip("a client version", "rank data")
		return
	}
	d.report(true, "client version", fmt.Sprintf("%s (from %s)", version, source))
	client.SetTarget(region, shard, version)

	if _, err := client.Get(ctx, remote.PD, "/mmr/v1/players/"+ent.Subject, nil); err != nil {
		d.report(false, "rank data", fmt.Sprintf("your rank data request to pd (shard %s) failed: %v", shard, err))
		return
	}
	d.report(true, "rank data", fmt.Sprintf("your rank data came back from pd (shard %s)", shard))
}

// offline covers what doesn't need the Riot Client
func (d *doctor) offline() {
	generated := static.Embedded().Generated()
	days := int(d.env.Clock.Now().Sub(generated).Hours() / 24)
	d.report(true, "static data", fmt.Sprintf("snapshot from %s (%d days old)", generated.Format("2006-01-02"), days))

	if !platform.Windows {
		d.report(true, "priority", "not Windows - nothing to lower")
		d.report(true, "efficiency", "not Windows - no EcoQoS")
	} else {
		d.windows()
	}
	ansi := "ANSI off - plain output"
	if d.env.Console.Terminal {
		ansi = "ANSI on"
	}
	d.report(true, "console", fmt.Sprintf("width %d · %s", d.env.Console.Width(), ansi))

	switch {
	case !d.cfg.Exists():
		d.report(true, "settings", fmt.Sprintf("no %s yet - defaults in use (the first normal run creates it)", settings.FileName))
	case d.cfg.Invalid():
		d.report(false, "settings", d.cfg.Warning())
	case d.cfg.Warning() != "":
		d.report(true, "settings", d.cfg.Warning())
	default:
		d.report(true, "settings", d.cfg.Path()+" ok")
	}
}

func (d *doctor) windows() {
	switch class, err := platform.Priority(); {
	case err != nil:
		d.report(false, "priority", fmt.Sprintf("can't read the priority class: %v", err))
	case class != "below normal":
		d.report(false, "priority", fmt.Sprintf("%s, want below normal (setting it said: %v)", class, d.setup.PriorityErr))
	default:
		d.report(true, "priority", "below normal")
	}
	switch on, err := platform.EfficiencyMode(); {
	case errors.Is(err, platform.ErrCantReadBack) && d.setup.EcoErr == nil:
		d.report(true, "efficiency", "on - EcoQoS was accepted at startup (this Windows can't read it back)")
	case err != nil || d.setup.EcoErr != nil:
		d.report(false, "efficiency", fmt.Sprintf("EcoQoS didn't take: %v", errors.Join(err, d.setup.EcoErr)))
	case !on:
		d.report(false, "efficiency", "off")
	default:
		d.report(true, "efficiency", "on (EcoQoS)")
	}
}

func (d *doctor) session(ctx context.Context, local *riot.Local) (region, shard string, ok bool) {
	if r := d.cfg.Settings().Region; r != "" {
		shard, _ := riot.Shard(r)
		d.report(true, "session", fmt.Sprintf("region %s from %s · shard %s", r, settings.FileName, shard))
		return r, shard, true
	}
	sessions, err := local.Sessions(ctx)
	if err != nil {
		d.report(false, "session", fmt.Sprintf("external sessions: %v", err))
		return "", "", false
	}
	args, running := riot.ValorantArgs(sessions)
	region = riot.RegionFromArgs(args)
	switch {
	case !running:
		d.report(false, "session", "no Valorant session - region not detected")
		return "", "", false
	case region == "":
		d.report(false, "session", "Valorant has no -ares-deployment argument - region not detected")
		return "", "", false
	}
	shard, known := riot.Shard(region)
	if !known {
		d.report(false, "session", fmt.Sprintf("region %q has no known shard", region))
		return "", "", false
	}
	d.report(true, "session", fmt.Sprintf("region %s · shard %s", region, shard))
	return region, shard, true
}

func tokenSource(local *riot.Local) func(ctx context.Context) (remote.Tokens, error) {
	return func(ctx context.Context) (remote.Tokens, error) {
		ent, err := local.Entitlements(ctx)
		if err != nil {
			return remote.Tokens{}, err
		}
		// the pair is only as good as whichever token runs out first
		exp, _ := riot.Expiry(ent.AccessToken)
		if e, ok := riot.Expiry(ent.Token); ok && (exp.IsZero() || e.Before(exp)) {
			exp = e
		}
		return remote.Tokens{Access: ent.AccessToken, Entitlement: ent.Token, Expires: exp}, nil
	}
}

// detectVersion never trusts external-sessions - its version is a manifest ID
func detectVersion(ctx context.Context, client *remote.Client, priv riot.Private, gameLog string) (version, source string) {
	if riot.ValidVersion(priv.PartyClientVersion) {
		return priv.PartyClientVersion, "presence"
	}
	if v, ok := riot.VersionFromLog(gameLog); ok {
		return v, "ShooterGame.log"
	}
	body, err := client.Get(ctx, remote.Static, "/v1/version", nil)
	if err != nil {
		return "", ""
	}
	if v, ok := staticVersion(body); ok {
		return v, "valorant-api.com"
	}
	return "", ""
}

func staticVersion(body []byte) (string, bool) {
	var v struct {
		Data struct {
			RiotClientVersion string `json:"riotClientVersion"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &v) != nil || !riot.ValidVersion(v.Data.RiotClientVersion) {
		return "", false
	}
	return v.Data.RiotClientVersion, true
}

func phaseFromLoop(state string) string {
	switch strings.ToUpper(state) {
	case "MENUS":
		return "Menus"
	case "PREGAME":
		return "Agent select"
	case "INGAME":
		return "In match"
	}
	return fmt.Sprintf("unknown session loop state %q", state)
}
