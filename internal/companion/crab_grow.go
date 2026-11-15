package companion

import "math"

// Growing up, in the one currency a 1-bit sprite has: how many cells it takes up.
//
// The crab is drawn procedurally -- its pose rows are composed per frame and
// parsed into a bitmap -- so a stage cannot be a second set of hand-drawn art
// without authoring five more crabs (one per pose). Instead the composed bitmap
// is resampled to the size the stage has earned, in the bitmap's own domain,
// before it becomes cells.
//
// Everything the layout depends on is untouched by this: Size() still reports the
// shipped 12x7 box forever, because compose() derives the companion's margin,
// the litter's room, SandTo and the moon's column from it. A smaller animal is
// drawn INSIDE that box, standing on the same bottom edge (so its feet stay on
// the sand) and centred on the same head column (so the agent's balloon still
// points at its face).

// stageScale is how big the animal is drawn, by stage: a hatchling is a little
// over half the adult, an old friend is the crab itself.
var stageScale = [4]float64{0.55, 0.72, 0.88, 1.0}

// ScaleForStage is the drawing scale for a stage index, clamped to the array. It
// exists so a caller outside this package (the CLI's own drawing) does not have
// to know the table.
func ScaleForStage(stage int) float64 {
	if stage < 0 {
		stage = 0
	}
	if stage >= len(stageScale) {
		stage = len(stageScale) - 1
	}
	return stageScale[stage]
}

// SetScale says how large to draw the animal. Anything at or below zero, and
// anything at or above one, is the shipped crab; the growth path is only ever a
// shrinking of the same sprite.
func (c *Cat) SetScale(s float64) {
	if s <= 0 || s > 1 {
		s = 1
	}
	c.scale = s
}

// Scale is the drawing scale in force.
func (c *Cat) Scale() float64 {
	if c.scale <= 0 || c.scale > 1 {
		return 1
	}
	return c.scale
}

// scaled says whether a resize is in force at all. Kept as one expression so the
// byte-identical path is one comparison, not a scatter of them.
func (c *Cat) scaled() bool { return c.Scale() < 0.9995 }

// grownBox returns the sprite's size in cells at the current scale, and where
// that smaller box sits inside the shipped one: bottom-aligned, and centred on
// the head column.
func (c *Cat) grownBox() (w, h, dx, dy int) {
	bw, bh := c.baseBox()
	head := baseCrabHead()
	s := c.Scale()
	w = int(math.Round(float64(bw) * s))
	h = int(math.Round(float64(bh) * s))
	if w < 3 {
		w = 3
	}
	if h < 2 {
		h = 2
	}
	if w > bw {
		w = bw
	}
	if h > bh {
		h = bh
	}
	// On the same feet, on the same face column.
	dx = head - w/2
	dy = bh - h
	if dx < 0 {
		dx = 0
	}
	if dx+w > bw {
		dx = bw - w
	}
	return w, h, dx, dy
}

// baseBox is the shipped sprite's size in cells. It never changes: Size() is
// the layout's contract, and growth is drawn INSIDE it.
func (c *Cat) baseBox() (w, h int) {
	if c.body != nil {
		return c.body.W / 2, c.body.H / 4
	}
	return 12, 7
}

// baseCrabHead is the crab's face column in the shipped box, which is also the
// column a balloon points at. The scaled box is centred on it.
func baseCrabHead() int { return (crabEyeCells[0] + crabEyeCells[1] + 1) / 2 }

// grownEyeX is an eye's column inside the scaled sprite.
func (c *Cat) grownEyeX(i int) int {
	w, _, _, _ := c.grownBox()
	bw, _ := c.baseBox()
	e := int(math.Round(float64(crabEyeCells[i]) * float64(w) / float64(bw)))
	if e < 0 {
		e = 0
	}
	if e > w-1 {
		e = w - 1
	}
	return e
}

// grownEyeRow is the eye row inside the scaled sprite: the same relative height
// on the face, not the same distance from the top.
func (c *Cat) grownEyeRow() int {
	_, h, _, _ := c.grownBox()
	_, bh := c.baseBox()
	r := int(math.Round(float64(crabEyeRow) * float64(h) / float64(bh)))
	if r < 0 {
		r = 0
	}
	if r > h-1 {
		r = h - 1
	}
	return r
}

// resizeInk resamples a 1-bit bitmap. A target pixel is ink when enough of the
// source region it covers was ink: the threshold is low on purpose, because a
// claw one pixel wide is still a claw, and dropping it turns the animal into a
// pebble. Measured on the crab's own art at every stage; see
// crab_stage_test.go, which asserts the silhouette keeps its mass and its gaps.
const resizeInkPct = 32

func resizeInk(b *Bitmap, w, h int) *Bitmap {
	if b == nil || w < 1 || h < 1 {
		return b
	}
	out := &Bitmap{W: w, H: h, On: make([]bool, w*h)}
	for ty := 0; ty < h; ty++ {
		y0 := ty * b.H / h
		y1 := (ty + 1) * b.H / h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for tx := 0; tx < w; tx++ {
			x0 := tx * b.W / w
			x1 := (tx + 1) * b.W / w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			ink, total := 0, 0
			for sy := y0; sy < y1 && sy < b.H; sy++ {
				for sx := x0; sx < x1 && sx < b.W; sx++ {
					total++
					if b.at(sx, sy) {
						ink++
					}
				}
			}
			if total > 0 && ink*100 >= resizeInkPct*total {
				out.On[ty*w+tx] = true
			}
		}
	}
	return out
}
