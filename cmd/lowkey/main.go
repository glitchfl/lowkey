package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/glitchfl/lowkey/internal/app"
	"github.com/glitchfl/lowkey/internal/platform"
)

var version = "dev"

func main() {
	// the spec caps lowkey at 100 MB - a soft limit makes the GC work before it gets close
	debug.SetMemoryLimit(64 << 20)

	env := app.Env{
		Lockfile: override("LOWKEY_LOCKFILE", platform.LockfilePath()),
		Servers:  servers(),
		Clock:    app.RealClock(),
		Console:  app.Console{Out: os.Stdout, Width: platform.ConsoleWidth, Terminal: platform.EnableANSI()},
		Dir:      override("LOWKEY_DIR", platform.ExeDir()),
		GameLog:  override("LOWKEY_GAME_LOG", platform.GameLogPath()),
		Version:  version,
	}
	if path := os.Getenv("LOWKEY_LOCAL_CA"); path != "" {
		pool, err := loadRoots(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "LOWKEY_LOCAL_CA: %v\n", err)
			os.Exit(2)
		}
		env.LocalRoots = pool
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := app.Run(ctx, env, os.Args[1:])
	stop()
	os.Exit(code)
}

// the LOWKEY_* overrides point the real exe at the fakeriot program
func override(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func servers() app.Servers {
	s := app.ProductionServers()
	s.PD = override("LOWKEY_PD_URL", s.PD)
	s.GLZ = override("LOWKEY_GLZ_URL", s.GLZ)
	s.Shared = override("LOWKEY_SHARED_URL", s.Shared)
	s.Static = override("LOWKEY_STATIC_URL", s.Static)
	return s
}

func loadRoots(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s holds no certificate", path)
	}
	return pool, nil
}
