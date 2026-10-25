package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

func main() {
	dir := flag.String("dir", "", "folder for the lockfile, CA and env file (default: a temporary folder)")
	flag.Parse()

	if *dir != "" {
		if err := os.MkdirAll(*dir, 0o755); err != nil {
			fail(err)
		}
	}
	srv, err := fakeriot.Start(fakeriot.Options{Dir: *dir})
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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fakeriot:", err)
	os.Exit(1)
}
