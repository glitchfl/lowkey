package app

import (
	"slices"
	"strings"
	"time"

	"github.com/glitchfl/lowkey/internal/platform"
	"github.com/glitchfl/lowkey/internal/record"
	"github.com/glitchfl/lowkey/internal/settings"
	"github.com/glitchfl/lowkey/internal/static"
)

type dotKind int

const (
	dotDown dotKind = iota
	dotMatchFound
	dotCheck // the few seconds after placing - so you can see where it landed
)

const (
	placeCountdown = 5
	dotCheckFor    = 3 * time.Second
)

type agentDot struct {
	kind  dotKind
	marks []dotMark
	// seq moves on every show and hide so a time limit that fires late can't take down newer dots
	seq  int
	stop func() bool
}

type dotMark struct {
	agent string // the display name - it labels the dot and picks its colors
	spot  settings.Spot
}

func (lv *live) hasDot() bool { return lv.env.Dot.Show != nil }

// matchFound puts the dots up for a match lowkey saw being found - connecting mid agent select is too late for them
func (lv *live) matchFound() {
	cfg := lv.cfg.Settings()
	if !cfg.AgentDot {
		return
	}
	var marks []dotMark
	for _, d := range cfg.AgentDots {
		if d.At.Placed {
			marks = append(marks, dotMark{agent: lv.agentLabel(d.Agent), spot: d.At})
		}
	}
	if len(marks) > 0 {
		lv.showDots(dotMatchFound, "match found", marks, time.Duration(cfg.AgentDotSeconds)*time.Second)
	}
}

// agentLabel writes a hand-typed name the way the game does - one lowkey doesn't know yet stays as typed
func (lv *live) agentLabel(name string) string {
	for _, known := range lv.data.AgentNames() {
		if strings.EqualFold(known, name) {
			return known
		}
	}
	return name
}

// showDots is the only way dots go up - never once a match loads so they can't be on screen in a round
func (lv *live) showDots(kind dotKind, why string, marks []dotMark, limit time.Duration) {
	if !lv.hasDot() || lv.phase == phaseMatchLoad || lv.phase == phaseInRound {
		return
	}
	lv.hideDot("replaced")
	if err := lv.env.Dot.Show(platformMarks(marks)); err != nil {
		lv.log.printf("agent dot: %v", err)
		return
	}
	lv.dot.seq++
	seq := lv.dot.seq
	lv.dot.kind, lv.dot.marks = kind, marks
	lv.dot.stop = lv.env.Clock.AfterFunc(limit, func() {
		lv.loop.post(func() {
			if seq == lv.dot.seq {
				lv.hideDot("time limit")
			}
		})
	})
	lv.recordDots(true, why, marks)
}

func (lv *live) hideDot(why string) {
	if lv.dot.kind == dotDown {
		return
	}
	lv.dot.stop()
	lv.dot.seq++
	marks := lv.dot.marks
	lv.dot.kind, lv.dot.marks, lv.dot.stop = dotDown, nil, nil
	lv.env.Dot.Hide()
	lv.recordDots(false, why, marks)
}

// dropDots takes some agents' dots down and leaves the rest up on the same time limit
func (lv *live) dropDots(agents []string, why string) {
	var keep, gone []dotMark
	for _, m := range lv.dot.marks {
		if slices.ContainsFunc(agents, func(a string) bool { return strings.EqualFold(a, m.agent) }) {
			gone = append(gone, m)
		} else {
			keep = append(keep, m)
		}
	}
	switch {
	case len(gone) == 0:
		return
	case len(keep) == 0:
		lv.hideDot(why)
		return
	}
	if err := lv.env.Dot.Show(platformMarks(keep)); err != nil {
		lv.log.printf("agent dot: %v", err)
		lv.hideDot("couldn't redraw")
		return
	}
	lv.dot.marks = keep
	lv.recordDots(false, why, gone)
}

func platformMarks(marks []dotMark) []platform.Mark {
	out := make([]platform.Mark, len(marks))
	for i, m := range marks {
		fill, ring := static.AgentColors(m.agent)
		out[i] = platform.Mark{X: m.spot.X, Y: m.spot.Y, Label: m.agent, Fill: fill, Ring: ring}
	}
	return out
}

func (lv *live) recordDots(shown bool, why string, marks []dotMark) {
	if lv.rec == nil {
		return
	}
	dots := make([]record.DotMark, len(marks))
	for i, m := range marks {
		dots[i] = record.DotMark{Agent: m.agent, X: m.spot.X, Y: m.spot.Y}
	}
	lv.rec.Dots(shown, why, dots)
}

// pickedAgent reacts to a roster - your own hover or lock means the dots did their job - a teammate
// locking one of your agents leaves the rest up as your backups
func (lv *live) pickedAgent(lob *lobby) {
	if lv.dot.kind != dotMatchFound {
		return
	}
	if me := lob.member(lv.self); me != nil && me.agent != "" {
		lv.hideDot("picked an agent")
		return
	}
	var taken []string
	for _, m := range lob.members {
		if m.puuid == lv.self || m.team != lob.ownTeam || m.agent == "" || m.hover {
			continue
		}
		if name, ok := lv.data.AgentName(m.agent); ok {
			taken = append(taken, name)
		}
	}
	lv.dropDots(taken, "taken by a teammate")
}

func (lv *live) checkDotSetting() {
	if lv.dot.kind == dotMatchFound && !lv.cfg.Settings().AgentDot {
		lv.hideDot("turned off")
	}
}

// placeDot counts down so there's time to switch to Valorant - wherever the cursor rests at zero is the slot's spot
func (lv *live) placeDot(slot int) {
	dots := lv.cfg.Settings().AgentDots
	if lv.tab.placing > 0 || !lv.hasDot() || slot >= len(dots) {
		return
	}
	lv.tab.placing, lv.tab.placingSlot = placeCountdown, slot
	lv.tickPlacing(dots[slot].Agent)
}

func (lv *live) tickPlacing(agent string) {
	lv.env.Clock.AfterFunc(time.Second, func() {
		lv.loop.post(func() {
			if lv.tab.placing--; lv.tab.placing > 0 {
				lv.tickPlacing(agent)
				return
			}
			x, y, ok := lv.env.Dot.Cursor()
			if !ok {
				lv.tab.notice = "can't read where the cursor is - try placing again"
				return
			}
			slot := lv.tab.placingSlot
			if dots := lv.cfg.Settings().AgentDots; slot >= len(dots) || !strings.EqualFold(dots[slot].Agent, agent) {
				lv.tab.notice = "that slot changed while placing - nothing saved"
				return
			}
			spot := settings.Spot{X: x, Y: y, Placed: true}
			// placing means the dot is wanted - leaving it off would look like placing didn't work
			lv.tab.change(func(s *settings.Settings) {
				if slot < len(s.AgentDots) {
					s.AgentDots[slot].At = spot
				}
				s.AgentDot = true
			})
			lv.showDots(dotCheck, "placed", []dotMark{{agent: lv.agentLabel(agent), spot: spot}}, dotCheckFor)
		})
	})
}
