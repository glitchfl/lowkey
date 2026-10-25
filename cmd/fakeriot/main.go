package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

func main() {
	dir := flag.String("dir", "", "folder for the lockfile, CA and env file (default: a temporary folder)")
	name := flag.String("scenario", "menus", "scenario to play - "+scenarioNames())
	flag.Parse()
	sc, ok := fakeriot.FindScenario(*name)
	if !ok {
		fail(fmt.Errorf("no scenario %q - pick one of %s", *name, scenarioNames()))
	}

	if *dir != "" {
		if err := os.MkdirAll(*dir, 0o755); err != nil {
			fail(err)
		}
	}
	srv, err := fakeriot.Start(fakeriot.Options{Dir: *dir, World: sc.World(time.Now())})
	if err != nil {
		fail(err)
	}
	defer srv.Close()

	folder := filepath.Dir(srv.Lockfile())
	ca := filepath.Join(folder, "fakeriot-ca.pem")
	if err := os.WriteFile(ca, srv.CAPEM(), 0o644); err != nil {
		fail(err)
	}
	urls := srv.URLs()
	vars := []string{
		"LOWKEY_LOCKFILE=" + srv.Lockfile(),
		"LOWKEY_LOCAL_CA=" + ca,
		"LOWKEY_PD_URL=" + urls.PD,
		"LOWKEY_GLZ_URL=" + urls.GLZ,
		"LOWKEY_SHARED_URL=" + urls.Shared,
		"LOWKEY_STATIC_URL=" + urls.Static,
	}
	// written last and renamed into place so a script never reads half of it
	envFile := filepath.Join(folder, "fakeriot.env")
	if err := os.WriteFile(envFile+".tmp", []byte(strings.Join(vars, "\n")+"\n"), 0o644); err != nil {
		fail(err)
	}
	if err := os.Rename(envFile+".tmp", envFile); err != nil {
		fail(err)
	}
	fmt.Println(strings.Join(vars, "\n"))
	fmt.Println("# request log: " + strings.TrimSuffix(urls.Static, "/static") + "/fakeriot/requests")
	fmt.Println("# env file: " + envFile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go srv.Play(ctx, sc, func(format string, args ...any) { fmt.Printf("# "+format+"\n", args...) })
	<-ctx.Done()
}

func scenarioNames() string {
	var names []string
	for _, sc := range fakeriot.Scenarios() {
		names = append(names, sc.Name)
	}
	return strings.Join(names, " ")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fakeriot:", err)
	os.Exit(1)
}
