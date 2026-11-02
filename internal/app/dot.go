package app

import (
	"time"

	"github.com/glitchfl/lowkey/internal/settings"
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
	kind dotKind
	spot settings.Spot
	// seq moves on every show and hide so a time limit that fires late can't take down a newer dot
	seq  int
	stop func() bool
}

func (lv *live) hasDot() bool { return lv.env.Dot.Show != nil }

// matchFound puts the dot up for a match lowkey saw being found - connecting mid agent select is too late for it
func (lv *live) matchFound() {
	cfg := lv.cfg.Settings()
	if cfg.AgentDot && cfg.AgentDotAt.Placed {
		lv.showDot(dotMatchFound, "match found", cfg.AgentDotAt, time.Duration(cfg.AgentDotSeconds)*time.Second)
	}
}

// showDot is the only way the dot goes up - never once a match loads so it can't be on screen in a round
func (lv *live) showDot(kind dotKind, why string, spot settings.Spot, limit time.Duration) {
	if !lv.hasDot() || lv.phase == phaseMatchLoad || lv.phase == phaseInRound {
		return
	}
	lv.hideDot("replaced")
	if err := lv.env.Dot.Show(spot.X, spot.Y); err != nil {
		lv.log.printf("agent dot: %v", err)
		return
	}
	lv.dot.seq++
	seq := lv.dot.seq
	lv.dot.kind, lv.dot.spot = kind, spot
	lv.dot.stop = lv.env.Clock.AfterFunc(limit, func() {
		lv.loop.post(func() {
			if seq == lv.dot.seq {
				lv.hideDot("time limit")
			}
		})
	})
	lv.recordDot(true, why)
}

func (lv *live) hideDot(why string) {
	if lv.dot.kind == dotDown {
		return
	}
	lv.dot.stop()
	lv.dot.seq++
	lv.dot.kind, lv.dot.stop = dotDown, nil
	lv.env.Dot.Hide()
	lv.recordDot(false, why)
}

func (lv *live) recordDot(shown bool, why string) {
	if lv.rec != nil {
		lv.rec.Dot(shown, why, lv.dot.spot.X, lv.dot.spot.Y)
	}
}

// pickedAgent takes the dot down once your own row has an agent - hovered or locked - it's done its job
func (lv *live) pickedAgent(lob *lobby) {
	if me := lob.member(lv.self); lv.dot.kind == dotMatchFound && me != nil && me.agent != "" {
		lv.hideDot("picked an agent")
	}
}

func (lv *live) checkDotSetting() {
	if lv.dot.kind == dotMatchFound && !lv.cfg.Settings().AgentDot {
		lv.hideDot("turned off")
	}
}

// placeDot counts down so there's time to switch to Valorant - wherever the cursor rests at zero is the spot
func (lv *live) placeDot() {
	if lv.tab.placing > 0 || !lv.hasDot() {
		return
	}
	lv.tab.placing = placeCountdown
	lv.tickPlacing()
}

func (lv *live) tickPlacing() {
	lv.env.Clock.AfterFunc(time.Second, func() {
		lv.loop.post(func() {
			if lv.tab.placing--; lv.tab.placing > 0 {
				lv.tickPlacing()
				return
			}
			x, y, ok := lv.env.Dot.Cursor()
			if !ok {
				lv.tab.notice = "can't read where the cursor is - try place dot again"
				return
			}
			spot := settings.Spot{X: x, Y: y, Placed: true}
			lv.tab.change(func(s *settings.Settings) { s.AgentDotAt = spot })
			lv.showDot(dotCheck, "placed", spot, dotCheckFor)
		})
	})
}
