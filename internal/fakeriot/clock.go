package fakeriot

import (
	"sort"
	"sync"
	"time"
)

// timers only fire inside Advance
type Clock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*clockTimer
	seq    int
}

type clockTimer struct {
	at      time.Time
	seq     int
	f       func()
	stopped bool
}

func NewClock(start time.Time) *Clock {
	return &Clock{now: start}
}

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *Clock) AfterFunc(d time.Duration, f func()) func() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	t := &clockTimer{at: c.now.Add(d), seq: c.seq, f: f}
	c.timers = append(c.timers, t)
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if t.stopped {
			return false
		}
		t.stopped = true
		return true
	}
}

func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	end := c.now.Add(d)
	c.mu.Unlock()
	for {
		c.mu.Lock()
		t := c.nextDue(end)
		if t == nil {
			c.now = end
			c.mu.Unlock()
			return
		}
		t.stopped = true
		if t.at.After(c.now) {
			c.now = t.at
		}
		c.mu.Unlock()
		t.f()
	}
}

func (c *Clock) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.timers {
		if !t.stopped {
			n++
		}
	}
	return n
}

func (c *Clock) nextDue(end time.Time) *clockTimer {
	live := c.timers[:0]
	for _, t := range c.timers {
		if !t.stopped {
			live = append(live, t)
		}
	}
	c.timers = live
	sort.Slice(live, func(i, j int) bool {
		if !live[i].at.Equal(live[j].at) {
			return live[i].at.Before(live[j].at)
		}
		return live[i].seq < live[j].seq
	})
	if len(live) == 0 || live[0].at.After(end) {
		return nil
	}
	return live[0]
}
