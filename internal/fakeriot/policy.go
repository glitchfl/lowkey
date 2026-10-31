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

const namesPath = "/name-service/v2/players"

// Violations lists every request Riot must never see from lowkey whatever it was doing
func (s *Server) Violations() []string {
	self := s.Self()
	var out []string
	for _, req := range s.RemoteRequests() {
		where := strings.ToLower(req.Path + "?" + req.Query.Encode())
		parts := strings.Split(strings.Trim(req.Path, "/"), "/")
		switch {
		case req.Method == "PUT" && req.Path == namesPath:
			for _, puuid := range req.NamedHidden {
				out = append(out, "the name service was asked about the hidden player "+puuid)
			}
		case req.Method != "GET":
			out = append(out, fmt.Sprintf("a %s request was sent: %s", req.Method, where))
		case strings.Contains(where, "name-service"):
			out = append(out, "the name service was called outside a name lookup: "+where)
		case strings.Contains(where, "/pregame/") && (strings.Contains(where, "/select/") || strings.Contains(where, "/lock/")):
			out = append(out, "an agent select action was sent: "+where)
		case len(parts) >= 4 && parts[0] == "parties" && parts[2] == "players" && parts[3] != self:
			out = append(out, "party data was asked for someone else: "+where)
		}
	}
	return out
}

// HiddenLookups lists the requests that named a hidden player - with hidden_player_stats off there must be none
func (s *Server) HiddenLookups() []string {
	hidden := s.Hidden()
	var out []string
	for _, req := range s.RemoteRequests() {
		where := strings.ToLower(req.Path + "?" + req.Query.Encode())
		for _, h := range hidden {
			if strings.Contains(where, strings.ToLower(h)) {
				out = append(out, where)
			}
		}
	}
	return out
}

// NameViolations lists every screen that showed a hidden player's name - whatever the settings that must never happen
func (s *Server) NameViolations(frames []Frame) []string {
	s.mu.Lock()
	var names []string
	for puuid, p := range s.world.Players {
		if p.Incognito && puuid != s.world.Self && p.Name != "" {
			names = append(names, p.Name)
		}
	}
	s.mu.Unlock()
	var out []string
	for _, f := range frames {
		for _, name := range names {
			if strings.Contains(f.Text, name) {
				out = append(out, fmt.Sprintf("the hidden player %s was named on screen:\n%s", name, f.Text))
			}
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

// grace covers a request already in flight when the screen changed or sent just before its screen was drawn
const grace = 100 * time.Millisecond

// QuietViolations lists requests sent while the screen said lowkey was idle or waiting or mid-round or untracked -
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
