package scenes

import (
	"math"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/term"
)

// The owl's reactions: what it does about a spicy snack, a beetle, a berry.
//
// Everything here is drawn in the same vocabulary as the bird itself -- whole
// cells, quadrant marks, the scene's palette -- and around the owl's 12x7 box,
// never over the writing band or the fire. A reaction is a short event, not a
// second engine: five small functions that take (x, y, t, phase).
//
// The engine names the reaction (internal/life); the scape decides what it looks
// like. That is the same split as the pose vocabulary: the brain never draws.

// Fire colours: the study's ember, and a hotter core.
var (
	emberHot  = term.RGB{R: 255, G: 231, B: 128}
	emberWarm = term.RGB{R: 235, G: 135, B: 60}
	emberDeep = term.RGB{R: 178, G: 62, B: 44}
)

// DrawOwlReaction paints one reaction for an owl whose box is at (x, y).
// phase runs 0..1 over the event; variant is a seeded number so two helpings do
// not flicker identically. An unknown kind draws nothing, which is what lets a
// new food ship before its art does.
func DrawOwlReaction(c *canvas.Canvas, x, y int, phase float64, kind string, variant int) {
	if phase < 0 || phase > 1 {
		return
	}
	switch kind {
	case "fire":
		owlFire(c, x, y, phase, variant)
	case "dance":
		owlSparkle(c, x, y, phase, variant, 7)
	case "catch":
		owlCatch(c, x, y, phase)
	case "dizzy":
		owlDizzy(c, x, y, phase, variant)
	case "celebrate":
		owlSparkle(c, x, y, phase, variant, 12)
		owlHopRing(c, x, y, phase)
	}
}

// owlFire: a breath of flame from the beak, growing then guttering. The beak is
// at column 5-6, row 4 of the box, so the cone starts at its right edge and runs
// outward, away from the face.
func owlFire(c *canvas.Canvas, x, y int, phase float64, variant int) {
	const beakCol, beakRow = 6, 4
	// It inhales first: nothing for the first sixth, then a cone that grows to
	// four cells and shrinks again.
	if phase < 0.16 {
		return
	}
	p := (phase - 0.16) / 0.84
	grow := math.Sin(p * math.Pi)
	reach := 1 + int(math.Round(grow*3.4))
	near := c.Near()
	for i := 1; i <= reach; i++ {
		fx := x + beakCol + i
		// A flicker that is stable per frame-index, so it does not crawl.
		f := math.Sin(float64(i)*2.3 + p*9 + float64(variant%7))
		dy := 0
		if f > 0.55 {
			dy = -1
		} else if f < -0.55 {
			dy = 1
		}
		row := y + beakRow + dy
		var g rune
		var col term.RGB
		switch {
		case i == reach:
			g, col = '*', emberHot
		case i >= reach-1:
			g, col = '^', emberWarm
		default:
			g, col = '·', emberDeep
		}
		if i <= 2 {
			g, col = '^', emberHot
		}
		near.Plot(fx, row, g, col, 1)
		// A wisp above the cone, only while it is tall.
		if grow > 0.6 && i == reach-1 {
			near.Plot(fx, row-1, '`', emberDeep, 1)
		}
	}
}

// owlSparkle: stars fizzing up around the bird, each on its own seeded phase, so
// a dance and a celebration differ in how many and how high, not in style.
func owlSparkle(c *canvas.Canvas, x, y int, phase float64, variant, n int) {
	for i := 0; i < n; i++ {
		// A deterministic scatter: no RNG on the frame path.
		h := uint64(i)*2654435761 + uint64(variant)*40503
		a := float64(h%1000) / 1000
		start := a * 0.5
		if phase < start {
			continue
		}
		p := (phase - start) / (1 - start)
		col := x + 1 + int(a*10)
		rise := p * 4
		row := y + 3 - int(math.Round(rise))
		row -= int(a * 2)
		if row < y-3 {
			continue
		}
		g := '*'
		if p > 0.6 {
			g = '·'
		}
		c.Near().Plot(col, row, g, owlLight, 1)
	}
}

// owlCatch: a beetle drops toward the beak and is snapped out of the air.
func owlCatch(c *canvas.Canvas, x, y int, phase float64) {
	near := c.Near()
	const beakCol, beakRow = 6, 4
	if phase < 0.62 {
		// Falling from above right, on a shallow arc toward the beak.
		p := phase / 0.62
		bx := x + beakCol + 2 - int(math.Round(p*2))
		by := y - 2 + int(math.Round(p*7))
		near.Plot(bx, by, '~', owlBeak, 1)
		// A short trail behind it.
		if by > y-2 {
			near.Plot(bx, by-1, '·', owlBeak, 1)
		}
		return
	}
	if phase < 0.74 {
		// The snap: a bright beat at the beak.
		near.Plot(x+beakCol+1, y+beakRow, '*', owlLight, 1)
	}
}

// owlDizzy: the world tilts. The bird leans a row each way, and two small rings
// orbit where its eyes were -- enough to read as "that mushroom was a mistake,
// and a delightful one".
func owlDizzy(c *canvas.Canvas, x, y int, phase float64, variant int) {
	near := c.Near()
	// Rings: a 3x1 line that slides left and right above the head.
	off := int(math.Round(math.Sin(phase*4*math.Pi) * 3))
	for i := 0; i < 3; i++ {
		near.Plot(x+4+i+off, y-1, '·', owlLight, 1)
	}
	// And two sparkles at the eyes, alternating.
	if math.Mod(phase*6+float64(variant%2), 2) < 1 {
		near.Plot(x+3, y+2, 'x', owlLight, 1)
		near.Plot(x+8, y+2, 'x', owlLight, 1)
	}
}

// owlHopRing is the celebration's ring of sparks, low and outward.
func owlHopRing(c *canvas.Canvas, x, y int, phase float64) {
	if phase < 0.2 || phase > 0.85 {
		return
	}
	p := (phase - 0.2) / 0.65
	r := 2 + int(math.Round(p*2))
	for i := 0; i < 10; i++ {
		a := 2 * math.Pi * float64(i) / 10
		col := x + 5 + int(math.Round(math.Cos(a)*float64(r)*1.1))
		row := y + 3 + int(math.Round(math.Sin(a)*float64(r)*0.5))
		// Never further out than two cells: an owlet stands one cell away.
		if col < x-2 {
			col = x - 2
		}
		if col > x+13 {
			col = x + 13
		}
		if row < y-2 {
			row = y - 2
		}
		if row > y+7 {
			row = y + 7
		}
		g := '*'
		if p > 0.6 {
			g = '·'
		}
		c.Near().Plot(col, row, g, owlLight, 1)
	}
}

// OwlHopOffset is how far the bird itself moves during a reaction: a dance sways
// a cell either way on the beat, a celebration hops up a row. Renderers add it to
// the owl's x and y; everything else about the drawing is unchanged.
func OwlHopOffset(kind string, phase float64) (dx, dy int) {
	switch kind {
	case "dance":
		if math.Mod(phase*8, 1) < 0.5 {
			return -1, 0
		}
		return 1, 0
	case "celebrate":
		if phase > 0.15 && phase < 0.55 {
			return 0, -1
		}
	}
	return 0, 0
}
