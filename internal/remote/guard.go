package remote

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Window is when lowkey may talk to Riot's servers or static data at all
type Window int

const (
	Closed Window = iota
	AgentSelect
	MatchLoad // until the cutoff
	SessionRefresh
	OneShot // doctor and the own-stats printout
)

type RefusedError struct {
	Reason string
}

func (e *RefusedError) Error() string { return "refused by the policy guard: " + e.Reason }

// Guard sits in front of every remote request and refuses anything the policy forbids
type Guard struct {
	mu     sync.Mutex
	window Window
	self   string
	hidden map[string]bool
	// nameHidden is everyone in streamer mode - with hidden stats on they aren't in hidden but still can't be named
	nameHidden map[string]bool
}

func NewGuard() *Guard {
	return &Guard{hidden: map[string]bool{}, nameHidden: map[string]bool{}}
}

func (g *Guard) SetWindow(w Window) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.window = w
}

func (g *Guard) Window() Window {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.window
}

func (g *Guard) SetSelf(puuid string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.self = strings.ToLower(puuid)
	delete(g.hidden, g.self)
	delete(g.nameHidden, g.self)
}

// Hide refuses anything about these players until Show - your own PUUID is never treated as hidden
func (g *Guard) Hide(puuids ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, p := range puuids {
		p = strings.ToLower(p)
		if p != "" && p != g.self {
			g.hidden[p] = true
		}
	}
}

func (g *Guard) Show(puuid string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.hidden, strings.ToLower(puuid))
}

// KeepNameHidden follows a player's streamer mode from roster to roster - your own name is never hidden
func (g *Guard) KeepNameHidden(puuid string, keep bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	puuid = strings.ToLower(puuid)
	if keep && puuid != g.self {
		g.nameHidden[puuid] = true
	} else {
		delete(g.nameHidden, puuid)
	}
}

// CheckNames judges a name lookup - Riot answers for anyone it's asked about so a hidden player can't be on the list
func (g *Guard) CheckNames(puuids []string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.window != AgentSelect && g.window != MatchLoad && g.window != OneShot {
		return &RefusedError{Reason: "name lookups only happen in a lobby or a one-shot command"}
	}
	for _, p := range puuids {
		p = strings.ToLower(p)
		if g.hidden[p] || g.nameHidden[p] {
			return &RefusedError{Reason: "the name lookup includes a hidden player"}
		}
	}
	return nil
}

// Check judges the endpoint path - the full URL only matters for spotting a hidden player anywhere in it
func (g *Guard) Check(method string, svc Service, endpoint string, u *url.URL) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	path := strings.ToLower(endpoint)
	full := strings.ToLower(u.EscapedPath() + "?" + u.RawQuery)
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case method != http.MethodGet:
		return &RefusedError{Reason: method + " is a write"}
	case strings.Contains(path, "name-service"):
		return &RefusedError{Reason: "the name service only takes checked name lookups"}
	case strings.Contains(path, "/pregame/") && (hasPart(parts, "select") || hasPart(parts, "lock")):
		return &RefusedError{Reason: "agent select actions are never performed"}
	case strings.Contains(path, "/parties/") && !g.ownPartyRead(parts):
		return &RefusedError{Reason: "party calls only read your own party"}
	case g.window == Closed:
		return &RefusedError{Reason: "outside every allowed window"}
	}
	for p := range g.hidden {
		if strings.Contains(full, p) {
			return &RefusedError{Reason: fmt.Sprintf("the request references a hidden player (%s)", svc)}
		}
	}
	return nil
}

func (g *Guard) ownPartyRead(parts []string) bool {
	if len(parts) != 4 || parts[0] != "parties" || parts[1] != "v1" {
		return false
	}
	switch parts[2] {
	case "players":
		return parts[3] == g.self
	case "parties":
		return true
	}
	return false
}

func hasPart(parts []string, want string) bool {
	for _, p := range parts {
		if p == want {
			return true
		}
	}
	return false
}
