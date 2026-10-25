package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
)

type doctor struct {
	env    Env
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

func runDoctor(ctx context.Context, env Env, opts options) int {
	d := &doctor{env: env}
	fmt.Fprintf(env.Console.Out, "lowkey %s doctor\n", env.Version)
	d.check(ctx)
	if d.failed {
		fmt.Fprintln(env.Console.Out, "some checks failed")
		return 1
	}
	fmt.Fprintln(env.Console.Out, "all checks passed")
	return 0
}

func (d *doctor) check(ctx context.Context) {
	env := d.env
	lf, err := riot.ReadLockfile(env.Lockfile)
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
	})
	client.SetTarget(region, shard, "")

	version, source := detectVersion(ctx, client, priv)
	if version == "" {
		d.report(false, "client version", "no valid release-NN.NN-shipping-N-N version from presence or valorant-api.com")
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

func (d *doctor) session(ctx context.Context, local *riot.Local) (region, shard string, ok bool) {
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
		exp, _ := riot.Expiry(ent.AccessToken)
		return remote.Tokens{Access: ent.AccessToken, Entitlement: ent.Token, Expires: exp}, nil
	}
}

// detectVersion never trusts external-sessions - its version is a manifest ID
func detectVersion(ctx context.Context, client *remote.Client, priv riot.Private) (version, source string) {
	if riot.ValidVersion(priv.PartyClientVersion) {
		return priv.PartyClientVersion, "presence"
	}
	body, err := client.Get(ctx, remote.Static, "/v1/version", nil)
	if err != nil {
		return "", ""
	}
	var v struct {
		Data struct {
			RiotClientVersion string `json:"riotClientVersion"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &v) == nil && riot.ValidVersion(v.Data.RiotClientVersion) {
		return v.Data.RiotClientVersion, "valorant-api.com"
	}
	return "", ""
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
