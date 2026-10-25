package app

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/stats"
)

// session is your competitive W/L and net RR since the reset hour, rebuilt from Riot's history each time
type session struct {
	self    string
	since   time.Time
	updates []stats.Update
	done    bool
	err     error
	matches map[string]*stats.Match

	pending  int
	finished func()
}

const updatesPage = 20

func newSession(self string, now time.Time, resetHour int) *session {
	return &session{self: self, since: stats.ResetBoundary(now, resetHour), matches: map[string]*stats.Match{}}
}

// refreshSession rebuilds the session line - done runs once every request it needed is answered
func (e *engine) refreshSession(s *session, done func()) {
	s.pending = 1
	s.finished = done
	e.fetchUpdatesPage(s, 0)
}

func (s *session) settle() {
	s.pending--
	if s.pending == 0 && s.finished != nil {
		s.finished()
	}
}

// fetchUpdatesPage pages back until the updates reach past the reset boundary
func (e *engine) fetchUpdatesPage(s *session, start int) {
	q := url.Values{"queue": {"competitive"}, "startIndex": {strconv.Itoa(start)}, "endIndex": {strconv.Itoa(start + updatesPage)}}
	e.submit(&job{prio: prioTrend, svc: remote.PD, path: "/mmr/v1/players/" + s.self + "/competitiveupdates", query: q},
		func(body []byte, err error) {
			var page []stats.Update
			if err == nil {
				page, err = stats.DecodeUpdates(body)
			}
			if err != nil {
				s.done, s.err = true, err
				s.settle()
				return
			}
			s.updates = append(s.updates, page...)
			for i, u := range page {
				if u.Start.Before(s.since) || !riot.IsUUID(u.MatchID) {
					continue
				}
				id := u.MatchID
				s.pending++
				e.wantMatch(id, s.self, start+i, func(m *stats.Match, err error) {
					if err == nil {
						s.matches[id] = m
					}
					s.settle()
				})
			}
			if len(page) == updatesPage && !page[len(page)-1].Start.Before(s.since) && start < 5*updatesPage {
				s.pending++
				e.fetchUpdatesPage(s, start+updatesPage)
			} else {
				s.done = true
			}
			s.settle()
		})
}

func (s *session) line() string {
	switch {
	case s == nil || !s.done:
		return ""
	case s.err != nil:
		return fmt.Sprintf("SESSION · unavailable (%v)", s.err)
	}
	r := stats.SessionOf(s.self, s.updates, s.since, s.matches)
	return fmt.Sprintf("SESSION · %dW %dL %s RR since %s", r.Wins, r.Losses, signed(r.RR), s.since.Format("15:04"))
}

func signed(n int) string {
	if n < 0 {
		return strconv.Itoa(n)
	}
	return "+" + strconv.Itoa(n)
}
