package app

import (
	"crypto/x509"
	"io"
	"net/http"
	"time"

	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
)

// Env is the single seam - production builds it from the real machine and tests build it around fakeriot
type Env struct {
	Lockfile string
	// nil trusts Riot's own root - tests pass fakeriot's
	LocalRoots *x509.CertPool
	Servers    Servers
	Clock      Clock
	Console    Console
	// Dir is the folder next to the exe that holds lowkey.toml plus the error log and recordings
	Dir     string
	GameLog string
	Version string
}

// Servers are base URL templates with {region} and {shard} placeholders
type Servers struct {
	PD        string
	GLZ       string
	Shared    string
	Static    string
	Transport http.RoundTripper
}

type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) (stop func() bool)
}

type Console struct {
	Out   io.Writer
	Width func() int
	// Terminal allows ANSI codes - redirected output gets plain text
	Terminal bool
}

func ProductionServers() Servers {
	e := remote.ProductionEndpoints()
	return Servers{PD: e.PD, GLZ: e.GLZ, Shared: e.Shared, Static: e.Static}
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) func() bool {
	return time.AfterFunc(d, f).Stop
}

func RealClock() Clock { return realClock{} }

func (e Env) withDefaults() Env {
	if e.LocalRoots == nil {
		e.LocalRoots = riot.RiotRoots()
	}
	if e.Servers.PD == "" {
		t := e.Servers.Transport
		e.Servers = ProductionServers()
		e.Servers.Transport = t
	}
	if e.Clock == nil {
		e.Clock = RealClock()
	}
	if e.Console.Out == nil {
		e.Console.Out = io.Discard
	}
	if e.Console.Width == nil {
		e.Console.Width = func() int { return 120 }
	}
	if e.Version == "" {
		e.Version = "dev"
	}
	return e
}

func (e Env) endpoints() remote.Endpoints {
	return remote.Endpoints{PD: e.Servers.PD, GLZ: e.Servers.GLZ, Shared: e.Servers.Shared, Static: e.Servers.Static}
}
