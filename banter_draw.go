package main

import (
	"strings"
	"time"

	"github.com/donlucasx/xscapes/internal/banter"
	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/envx"
	"github.com/donlucasx/xscapes/internal/event"
	"github.com/donlucasx/xscapes/internal/scenes"
	"github.com/donlucasx/xscapes/internal/term"
)

// The two spectators, wired to the scene.
//
// They are fed the same events the sea is fed, from the same drain, so a failed
// tool is one fact that both the water and the creatures hear. They are drawn
// with the primitives the scene already has: the cat sprite that shipped, the
// owlet that shipped, and a line of text where a creature's mouth is.
//
// Switch: XSCAPES_BANTER=off. Nothing here runs when it is off.

var (
	banterEngine *banter.Engine
	banterOn     bool
	banterNow    time.Time
	banterFriend *companion.Cat // the cat on the shore: a second creature, not a new species
	banterSeen   int            // how many lines the last frame drew, for the tests
)

// banterPlaceOf says which world the scene is showing.
func banterPlaceOf(f *frames) banter.Place {
	if f.vista != nil {
		return banter.Vista
	}
	return banter.Shore
}

// initBanter starts the spectators for a frame.
func initBanter(f *frames) {
	if strings.EqualFold(strings.TrimSpace(envx.Lookup("BANTER")), "off") {
		banterOn, banterEngine = false, nil
		return
	}
	banterOn = true
	banterEngine = banter.New(banterPlaceOf(f), f.seed)
	initBanterAI()
	if f.vista == nil {
		// The friend is the cat that already ships, in a coat of its own so the
		// two are told apart in one glance -- a new species would be a new art
		// style, and the brief forbids that.
		c := companion.NewCat()
		c.SetCoat(companion.Coats["slate"])
		banterFriend = c
	}
}

func closeBanter() {
	closeBanterAI()
	banterEngine, banterFriend = nil, nil
}

func banterLive() bool { return banterOn && banterEngine != nil }

// feedBanter hands one real event to the spectators.
func feedBanter(e event.Event, now time.Time) {
	if banterLive() {
		banterEngine.Observe(e, now)
	}
}

// drawBanterShore draws whatever the spectators are doing beside the crab.
func drawBanterShore(c *canvas.Canvas, catX, top, bandTop int, mirror bool) {
	if !banterLive() {
		return
	}
	now := banterNow
	snap := banterEngine.Current(now)
	if snap.Show == nil {
		return
	}
	// The friend stands on the far side of the crab from the frame edge, which
	// is the side the sand is written from, so it never covers the tail.
	friendX := catX - 14
	if !mirror {
		friendX = catX + 14
	}
	if friendX < 1 {
		friendX = 1
	}

	if snap.SecondOn && banterFriend != nil {
		banterFriend.Draw(c.Near(), friendX, top, now.Sub(snap.Show.Start).Seconds(), companion.Resting)
	}
	// Prop first, words second: a line of dialogue must never be interrupted by
	// the joke that goes with it.
	drawBanterProp(c, snap, catX, top, friendX)
	drawBanterLines(c, snap, catX, top, bandTop, friendX, 7)
}

// drawBanterVista draws the same thing on the mountain, with the owlet as the
// friend: it is the sprite the litter already uses.
func drawBanterVista(c *canvas.Canvas, owlX, owlY, bandTop int) {
	if !banterLive() {
		return
	}
	now := banterNow
	snap := banterEngine.Current(now)
	banterSeen = len(snap.Lines)
	if snap.Show == nil {
		return
	}
	friendX := owlX + 8
	if friendX+6 > c.W-1 {
		friendX = owlX - 7
	}
	if snap.SecondOn {
		scenes.DrawOwlet(c, scenes.OwlCoat, friendX, owlY+4, true)
	}
	drawBanterProp(c, snap, owlX, owlY, friendX)
	drawBanterLines(c, snap, owlX, owlY, bandTop, friendX, 7)
}

// drawBanterLines puts each speaker's line above its own head, or below its feet
// when the writing band is in the way. Two lines on screen at most, because two
// creatures talking at once over a coding agent is already a lot.
func drawBanterLines(c *canvas.Canvas, snap banter.Snapshot, primaryX, primaryY, bandTop, friendX, boxH int) {
	ink := companion.EyeInk()
	// When both are talking, the second speaker sits a row higher: two lines on
	// one row would overwrite each other and read as one long sentence.
	stacked := len(snap.Lines) > 1
	for i, l := range snap.Lines {
		text := banter.FitLine(l.Text, 38)
		if text == "" {
			continue
		}
		x, y := primaryX+6-len(text)/2, primaryY-1
		if l.Who == banter.Second {
			x, y = friendX+3-len(text)/2, primaryY-1
			if stacked && i > 0 {
				y = primaryY - 2
			}
		}
		if y < 0 || (bandTop > 0 && y >= bandTop) {
			y = primaryY + boxH
		}
		if y < 0 || y >= c.H {
			continue
		}
		if x < 1 {
			x = 1
		}
		if x+len(text) > c.W-1 {
			x = c.W - 1 - len(text)
		}
		if x < 1 {
			continue
		}
		(&companion.Sprite{Rows: []string{text}, Body: ink, Opaque: true}).Draw(c.Near(), x, y)
	}
}

// puff draws one small decoration at a cell, through the same sprite path the
// speech uses, so a prop is in the scene's language rather than a second one.
func puff(c *canvas.Canvas, x, y int, text string, ink term.RGB) {
	if y < 0 || y >= c.H || x < 0 || x+len(text) > c.W {
		return
	}
	(&companion.Sprite{Rows: []string{text}, Body: ink, Opaque: true}).Draw(c.Near(), x, y)
}

// drawBanterProp stages the little joke: a cloud of regret, or a sparkle.
func drawBanterProp(c *canvas.Canvas, snap banter.Snapshot, primaryX, primaryY, friendX int) {
	if !snap.PropUp {
		return
	}
	switch snap.Prop {
	case banter.Smoke:
		// Above the line of dialogue, never through it.
		puff(c, primaryX+4, primaryY-3, "  .  ", term.RGB{R: 150, G: 150, B: 158})
		puff(c, primaryX+3, primaryY-2, " ( ~ ) ", term.RGB{R: 128, G: 128, B: 138})
	case banter.Sparkle:
		puff(c, primaryX+2, primaryY-3, " *  * ", term.RGB{R: 240, G: 220, B: 140})
		puff(c, primaryX+6, primaryY-4, "  *  ", term.RGB{R: 250, G: 240, B: 190})
	case banter.Sign:
		puff(c, friendX+2, primaryY+3, "[ ! ]", term.RGB{R: 220, G: 180, B: 120})
	}
}
