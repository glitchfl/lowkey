package fakeriot

import (
	"fmt"
	"strings"
	"time"
)

// Frame is one screen lowkey drew and when it was written
type Frame struct {
	At   time.Time
	Text string
}

// Violations lists every request Riot must never see from lowkey whatever it was doing
func (s *Server) Violations() []string {
	hidden := s.Hidden()
	self := s.Self()
	var out []string
	for _, req := range s.RemoteRequests() {
		where := strings.ToLower(req.Path + "?" + req.Query.Encode())
		for _, h := range hidden {
			if strings.Contains(where, strings.ToLower(h)) {
				out = append(out, fmt.Sprintf("a request referenced the hidden player %s: %s", h, where))
			}
		}
		parts := strings.Split(strings.Trim(req.Path, "/"), "/")
		switch {
		case req.Method != "GET":
			out = append(out, fmt.Sprintf("a %s request was sent: %s", req.Method, where))
		case strings.Contains(where, "name-service"):
			out = append(out, "the name service was called: "+where)
		case strings.Contains(where, "/pregame/") && (strings.Contains(where, "/select/") || strings.Contains(where, "/lock/")):
			out = append(out, "an agent select action was sent: "+where)
		case len(parts) >= 4 && parts[0] == "parties" && parts[2] == "players" && parts[3] != self:
			out = append(out, "party data was asked for someone else: "+where)
		}
	}
	return out
}

type screenState int

const (
	inLobby screenState = iota
	inMenus
	silent
)

func stateOf(text string) (screenState, string) {
	for _, quiet := range []string{"waiting for the Riot Client", "waiting for Valorant", "match in progress", "IN MATCH ·", "not tracking"} {
		if strings.Contains(text, quiet) {
			return silent, quiet
		}
	}
	if strings.Contains(text, "waiting for the next match") {
		return inMenus, "waiting for the next match"
	}
	return inLobby, ""
}

// grace covers a request already in flight when the screen changed, or sent just before its screen was drawn
const grace = 100 * time.Millisecond

// QuietViolations lists requests sent while the screen said lowkey was idle, waiting, mid-round or untracked -
// in the menus only the session refresh may ask anything
func (s *Server) QuietViolations(frames []Frame) []string {
	self := s.Self()
	allows := func(state screenState, req Request) bool {
		switch state {
		case inLobby:
			return true
		case inMenus:
			return req.Service == "pd" && (req.Path == "/mmr/v1/players/"+self+"/competitiveupdates" || strings.HasPrefix(req.Path, "/match-details/")) ||
				req.Service == "static" && req.Path == "/v1/version"
		}
		return false
	}
	var out []string
	for _, req := range s.RemoteRequests() {
		prev := -1
		for i, f := range frames {
			if !f.At.After(req.Start) {
				prev = i
			}
		}
		if prev < 0 {
			continue
		}
		state, label := stateOf(frames[prev].Text)
		if allows(state, req) {
			continue
		}
		if next := prev + 1; next < len(frames) && frames[next].At.Sub(req.Start) <= grace {
			if nextState, _ := stateOf(frames[next].Text); allows(nextState, req) {
				continue
			}
		}
		// a request in flight when the screen changed is fine only if the screen before allowed it
		since := prev
		for since > 0 {
			if st, _ := stateOf(frames[since-1].Text); st != state {
				break
			}
			since--
		}
		if since > 0 && req.Start.Sub(frames[since].At) <= grace {
			if before, _ := stateOf(frames[since-1].Text); allows(before, req) {
				continue
			}
		}
		if label == "" {
			label = "a lobby"
		}
		out = append(out, fmt.Sprintf("%s %s went out while the screen said %q", req.Service, req.Path, label))
	}
	return out
}
