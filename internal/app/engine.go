package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/riot"
	"github.com/glitchfl/lowkey/internal/stats"
)

// engine looks players up through the scheduler and keeps what came back - it lives on the loop goroutine
type engine struct {
	sched   *scheduler
	post    func(func())
	changed func()

	acts    *stats.Acts
	actsErr error

	matches   map[string]*stats.Match
	waiters   map[string][]func(*stats.Match, error)
	ranks     map[string]stats.MMR
	histories map[string][]string
	busy      int
	gen       int
}

// player is everything known about one visible player so far
type player struct {
	puuid string
	queue string
	n     int

	mmr     stats.MMR
	mmrDone bool
	mmrErr  error

	updates     []stats.Update
	updatesDone bool
	updatesErr  error

	history     []string
	historyDone bool
	historyErr  error

	sample    map[string]*stats.Match
	detailErr int
}

type parts int

const (
	wantRank parts = 1 << iota
	wantTrend
	wantSample
)

func newEngine(sched *scheduler, post func(func())) *engine {
	return &engine{
		sched:     sched,
		post:      post,
		changed:   func() {},
		matches:   map[string]*stats.Match{},
		waiters:   map[string][]func(*stats.Match, error){},
		ranks:     map[string]stats.MMR{},
		histories: map[string][]string{},
	}
}

func newPlayer(puuid, queue string, n int) *player {
	return &player{puuid: strings.ToLower(puuid), queue: queue, n: n, sample: map[string]*stats.Match{}}
}

// submit hands a job's result back on the loop - unless the work was cancelled meanwhile
func (e *engine) submit(j *job, handle func(body []byte, err error)) {
	e.busy++
	gen := e.gen
	j.done = func(body []byte, err error) {
		e.post(func() {
			if gen != e.gen {
				return
			}
			e.busy--
			handle(body, err)
			e.changed()
		})
	}
	e.sched.submit(j)
}

func (e *engine) batch(f func()) {
	e.sched.hold()
	defer e.sched.release()
	f()
}

// cancel drops every outstanding lookup - a dodge or the cutoff
func (e *engine) cancel() {
	e.sched.cancelAll()
	e.gen++
	e.busy = 0
	e.waiters = map[string][]func(*stats.Match, error){}
}

// forgetRound clears what's only good until the menus
func (e *engine) forgetRound() {
	e.ranks = map[string]stats.MMR{}
	e.histories = map[string][]string{}
}

func (e *engine) fetchActs() {
	if e.acts != nil {
		return
	}
	e.submit(&job{prio: prioRoster, svc: remote.Shared, path: "/content-service/v3/content"}, func(body []byte, err error) {
		if err == nil {
			var acts stats.Acts
			if acts, err = stats.DecodeContent(body); err == nil {
				e.acts = &acts
			}
		}
		e.actsErr = err
	})
}

func (e *engine) lookUp(p *player, want parts) {
	if want&wantRank != 0 {
		e.lookUpRank(p)
	}
	if want&wantTrend != 0 {
		e.lookUpTrend(p, 5)
	}
	if want&wantSample != 0 {
		e.lookUpSample(p)
	}
}

func (e *engine) lookUpRank(p *player) {
	if mmr, ok := e.ranks[p.puuid]; ok {
		p.mmr, p.mmrDone = mmr, true
		return
	}
	e.submit(&job{prio: prioRank, svc: remote.PD, path: "/mmr/v1/players/" + p.puuid}, func(body []byte, err error) {
		p.mmrDone = true
		if err == nil {
			p.mmr, err = stats.DecodeMMR(body)
		}
		if err == nil {
			e.ranks[p.puuid] = p.mmr
		}
		p.mmrErr = err
	})
}

func (e *engine) lookUpTrend(p *player, count int) {
	q := url.Values{"queue": {"competitive"}, "startIndex": {"0"}, "endIndex": {strconv.Itoa(count)}}
	e.submit(&job{prio: prioTrend, svc: remote.PD, path: "/mmr/v1/players/" + p.puuid + "/competitiveupdates", query: q},
		func(body []byte, err error) {
			p.updatesDone = true
			if err == nil {
				p.updates, err = stats.DecodeUpdates(body)
			}
			p.updatesErr = err
		})
}

func (e *engine) lookUpSample(p *player) {
	key := p.puuid + "|" + p.queue
	if ids, ok := e.histories[key]; ok {
		e.gotHistory(p, ids)
		return
	}
	q := url.Values{"queue": {p.queue}, "startIndex": {"0"}, "endIndex": {strconv.Itoa(p.n)}}
	e.submit(&job{prio: prioHistory, svc: remote.PD, path: "/match-history/v1/history/" + p.puuid, query: q},
		func(body []byte, err error) {
			var ids []string
			if err == nil {
				ids, err = decodeHistory(body)
			}
			if err != nil {
				p.historyDone, p.historyErr = true, err
				return
			}
			e.histories[key] = ids
			e.gotHistory(p, ids)
		})
}

func (e *engine) gotHistory(p *player, ids []string) {
	if len(ids) > p.n {
		ids = ids[:p.n]
	}
	p.history, p.historyDone = ids, true
	for seq, id := range ids {
		e.wantMatch(id, p.puuid, seq, func(m *stats.Match, err error) {
			if err != nil {
				p.detailErr++
				return
			}
			p.sample[id] = m
		})
	}
}

// wantMatch fetches each match once no matter how many players or the session line want it
func (e *engine) wantMatch(id, owner string, seq int, got func(*stats.Match, error)) {
	if m, ok := e.matches[id]; ok {
		got(m, nil)
		return
	}
	if ws, ok := e.waiters[id]; ok {
		e.waiters[id] = append(ws, got)
		return
	}
	e.waiters[id] = []func(*stats.Match, error){got}
	e.submit(&job{prio: prioDetails, player: owner, seq: seq, svc: remote.PD, path: "/match-details/v1/matches/" + id},
		func(body []byte, err error) {
			ws := e.waiters[id]
			delete(e.waiters, id)
			var m *stats.Match
			if err == nil {
				m, err = stats.DecodeMatch(body)
			}
			if err == nil {
				e.matches[id] = m
			}
			for _, w := range ws {
				w(m, err)
			}
		})
}

// loaded is the sample so far - newest first
func (p *player) loaded() []*stats.Match {
	var out []*stats.Match
	for _, id := range p.history {
		if m, ok := p.sample[id]; ok {
			out = append(out, m)
		}
	}
	return out
}

func decodeHistory(body []byte) ([]string, error) {
	var v struct {
		History []struct {
			MatchID string `json:"MatchID"`
		} `json:"History"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("match history isn't JSON: %w", err)
	}
	ids := make([]string, 0, len(v.History))
	for _, h := range v.History {
		if riot.IsUUID(h.MatchID) {
			ids = append(ids, strings.ToLower(h.MatchID))
		}
	}
	return ids, nil
}
