package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/glitchfl/lowkey/internal/remote"
)

type priority int

const (
	prioRoster priority = iota
	prioParty
	prioRank
	prioTrend
	prioHistory
	prioDetails
)

type job struct {
	prio   priority
	player string // match details rotate between players
	seq    int    // newest match first within a player
	svc    remote.Service
	path   string
	query  url.Values
	// done runs on the worker goroutine - it never runs for work that was cancelled
	done func(body []byte, err error)

	order int
	tries int
}

// backoff is how long to wait after each 429 before giving up on the item
var backoff = []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}

// scheduler is the only path to Riot's servers: at most two requests in flight, in priority order
type scheduler struct {
	client *remote.Client
	clock  Clock
	log    *errlog
	limit  int
	// rateLimited hears when a 429 pauses everything and when an item is given up
	rateLimited func(retryIn time.Duration, gaveUp bool)

	mu        sync.Mutex
	queue     []*job
	inflight  int
	order     int
	perPlayer map[string]int
	firstSeen map[string]int
	gen       int
	ctx       context.Context
	cancel    context.CancelFunc
	paused    bool
	batching  int
}

func newScheduler(client *remote.Client, clock Clock, log *errlog) *scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &scheduler{
		client: client, clock: clock, log: log, limit: 2,
		perPlayer: map[string]int{}, firstSeen: map[string]int{},
		ctx: ctx, cancel: cancel,
	}
}

func (s *scheduler) submit(j *job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.order++
	j.order = s.order
	if _, ok := s.firstSeen[j.player]; !ok {
		s.firstSeen[j.player] = s.order
	}
	s.queue = append(s.queue, j)
	s.dispatch()
}

// cancelAll drops queued work and aborts what's in flight - nothing cancelled reports back
func (s *scheduler) cancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.gen++
	s.queue = nil
	s.perPlayer = map[string]int{}
	s.firstSeen = map[string]int{}
}

func (s *scheduler) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel()
	s.gen++
	s.queue = nil
}

// hold and release bracket a batch so a whole roster's lookups queue up before anything is picked
func (s *scheduler) hold() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batching++
}

func (s *scheduler) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batching--
	s.dispatch()
}

func (s *scheduler) dispatch() {
	for s.inflight < s.limit && len(s.queue) > 0 && !s.paused && s.batching == 0 {
		j := s.pick()
		s.inflight++
		go s.run(j, s.ctx, s.gen)
	}
}

func (s *scheduler) pick() *job {
	best := 0
	for i, j := range s.queue[1:] {
		if s.before(j, s.queue[best]) {
			best = i + 1
		}
	}
	j := s.queue[best]
	s.queue = append(s.queue[:best], s.queue[best+1:]...)
	if j.prio == prioDetails {
		s.perPlayer[j.player]++
	}
	return j
}

func (s *scheduler) before(a, b *job) bool {
	if a.prio != b.prio {
		return a.prio < b.prio
	}
	if a.prio == prioDetails && a.player != b.player {
		if s.perPlayer[a.player] != s.perPlayer[b.player] {
			return s.perPlayer[a.player] < s.perPlayer[b.player]
		}
		return s.firstSeen[a.player] < s.firstSeen[b.player]
	}
	if a.prio == prioDetails && a.seq != b.seq {
		return a.seq < b.seq
	}
	return a.order < b.order
}

func (s *scheduler) run(j *job, ctx context.Context, gen int) {
	body, err := s.client.Get(ctx, j.svc, j.path, j.query)

	s.mu.Lock()
	s.inflight--
	if gen != s.gen {
		s.dispatch()
		s.mu.Unlock()
		return
	}
	var se *remote.StatusError
	if errors.As(err, &se) && se.Status == http.StatusTooManyRequests && j.tries < len(backoff) {
		wait := backoff[j.tries]
		if se.RetryAfter > 0 {
			wait = se.RetryAfter
		}
		j.tries++
		if j.prio == prioDetails {
			s.perPlayer[j.player]--
		}
		s.queue = append(s.queue, j)
		s.pause(wait)
		s.mu.Unlock()
		if s.rateLimited != nil {
			s.rateLimited(wait, false)
		}
		return
	}
	s.dispatch()
	s.mu.Unlock()
	if errors.As(err, &se) && se.Status == http.StatusTooManyRequests && s.rateLimited != nil {
		s.rateLimited(0, true)
	}
	var refused *remote.RefusedError
	if errors.As(err, &refused) {
		s.log.printf("%s %s: %v", j.svc, j.path, err)
	}
	j.done(body, err)
}

// pause stops every dispatch - Riot's limits apply to the account, not one endpoint
func (s *scheduler) pause(d time.Duration) {
	if s.paused {
		return
	}
	s.paused = true
	s.clock.AfterFunc(d, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.paused = false
		s.dispatch()
	})
}
