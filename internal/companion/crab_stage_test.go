package companion

import (
	"strings"
	"testing"

	"github.com/donlucasx/xscapes/internal/canvas"
)

// The animal grows without new art: the same pose, resampled. These tests hold
// the two things that matter -- it gets bigger in order, and the shipped crab is
// untouched when nothing has grown.

func stageCanvas() *canvas.Canvas {
	return canvas.New(24, 12, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
}

func drawAt(t *testing.T, scale float64, st State, tm float64) string {
	t.Helper()
	c := stageCanvas()
	cat := NewCrab()
	cat.SetScale(scale)
	cat.Draw(c.Near(), 2, 2, tm, st)
	return c.RenderPlain()
}

// The shipped crab, byte for byte, when no growth is set. This is the guard that
// keeps every existing frame, fingerprint and balloon test honest.
func TestTheUnscaledCrabIsUntouched(t *testing.T) {
	for _, st := range []State{Resting, Working, NeedsYou, Worried, Done} {
		for _, tm := range []float64{0, 0.4, 1.1, 2.2, 3.3, 4.4} {
			plain := drawAt(t, 0, st, tm) // zero means "no scale set"
			one := drawAt(t, 1.0, st, tm) // explicitly full size
			if plain != one {
				t.Errorf("state %d at t=%.1f: setting the shipped size changed the frame", st, tm)
			}
		}
	}
}

// Every stage draws, nothing panics, and the animal grows in order.
func TestTheAnimalGrowsInOrder(t *testing.T) {
	var widths []int
	for stage := 0; stage < 4; stage++ {
		s := ScaleForStage(stage)
		plain := drawAt(t, s, Resting, 1.0)
		w, h := inkBox(plain)
		if w == 0 || h == 0 {
			t.Fatalf("stage %d draws nothing:\n%s", stage, plain)
		}
		widths = append(widths, w)
	}
	for i := 1; i < len(widths); i++ {
		if widths[i] <= widths[i-1] {
			t.Errorf("stage %d is %d wide, stage %d is %d: growth is not visible",
				i, widths[i], i-1, widths[i-1])
		}
	}
	if widths[len(widths)-1] != 12 {
		t.Errorf("the adult is %d cells wide, want the shipped 12", widths[len(widths)-1])
	}
}

// inkBox is the bounding box of anything drawn on the layer.
func inkBox(plain string) (w, h int) {
	x0, y0, x1, y1 := 1<<30, 1<<30, -1, -1
	for y, row := range strings.Split(plain, "\n") {
		for x, r := range []rune(row) {
			if r == ' ' {
				continue
			}
			if x < x0 {
				x0 = x
			}
			if x > x1 {
				x1 = x
			}
			if y < y0 {
				y0 = y
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	if x1 < 0 {
		return 0, 0
	}
	return x1 - x0 + 1, y1 - y0 + 1
}

// The animal stays inside its box at every stage and pose: the layout reserves
// 12x7 and the sand, the litter and the balloon are all laid out against that.
func TestTheYoungStayInsideTheBox(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		for _, st := range []State{Resting, Working, NeedsYou, Worried, Done} {
			c := stageCanvas()
			cat := NewCrab()
			cat.SetScale(ScaleForStage(stage))
			cat.Draw(c.Near(), 2, 2, 1.2, st)
			plain := c.RenderPlain()
			for y, row := range strings.Split(plain, "\n") {
				for x, r := range []rune(row) {
					if r == ' ' {
						continue
					}
					if x < 2 || x >= 2+12 || y < 2 || y >= 2+7 {
						t.Fatalf("stage %d, state %d drew at (%d,%d), outside its 12x7 box:\n%s",
							stage, st, x, y, plain)
					}
				}
			}
		}
	}
}

// The feet stay on the bottom of the box, so a small animal stands on the same
// line as a grown one -- otherwise a hatchling floats.
func TestTheFeetStayOnTheSand(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		plain := drawAt(t, ScaleForStage(stage), Resting, 1.0)
		_, h := inkBox(plain)
		if h == 0 {
			t.Fatalf("stage %d draws nothing", stage)
		}
		rows := strings.Split(plain, "\n")
		bottom := 0
		for y, row := range rows {
			if strings.TrimSpace(row) != "" {
				bottom = y
			}
		}
		if bottom != 2+7-1 {
			t.Errorf("stage %d stands on row %d, want %d (the box's last row)",
				stage, bottom, 2+7-1)
		}
	}
}

// The face column is where the agent's balloon points, at every size: the
// reported head must sit between the two drawn eyes.
func TestTheBalloonStillFindsTheFace(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		c := stageCanvas()
		cat := NewCrab()
		cat.SetScale(ScaleForStage(stage))
		cat.Draw(c.Near(), 2, 2, 1.0, Resting) // resting draws the eyes as '-'

		eyeRow := 2 + crabEyeRow
		if cat.scaled() {
			_, _, _, dy := cat.grownBox()
			eyeRow = 2 + dy + cat.grownEyeRow()
		}
		line := []rune(strings.Split(c.RenderPlain(), "\n")[eyeRow])
		var eyes []int
		for x, r := range line {
			if r == '-' {
				eyes = append(eyes, x)
			}
		}
		if len(eyes) < 2 {
			t.Fatalf("stage %d: %d eyes drawn on row %d:\n%s", stage, len(eyes), eyeRow,
				c.RenderPlain())
		}
		head := 2 + cat.HeadCol()
		if head < eyes[0] || head > eyes[len(eyes)-1] {
			t.Errorf("stage %d: the balloon points at column %d, the eyes are %v",
				stage, head, eyes)
		}
	}
}

// A tiny canvas must not panic at any stage, and the eye arithmetic must stay
// inside the sprite at every scale.
func TestTheGrownCrabAtAnySize(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		s := ScaleForStage(stage)
		for _, sz := range [][2]int{{1, 1}, {6, 3}, {12, 7}, {80, 24}} {
			c := canvas.New(sz[0], sz[1], canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
			cat := NewCrab()
			cat.SetScale(s)
			cat.Draw(c.Near(), 0, 0, 0.7, Working)
		}
		w, h, _, _ := NewCrab().grownBox()
		if s < 1 {
			if w < 3 || h < 2 {
				t.Errorf("scale %.2f produced a %dx%d box", s, w, h)
			}
		}
	}
}

// A printer, so a person can look at the four ages instead of trusting a number.
// Run with -run TestShowTheFourAges -v to see the animal grow.
func TestShowTheFourAges(t *testing.T) {
	names := []string{"hatchling", "youngster", "grown", "old friend"}
	for stage := 0; stage < 4; stage++ {
		t.Logf("%s (%.2f):\n%s", names[stage], ScaleForStage(stage),
			drawAt(t, ScaleForStage(stage), Resting, 1.0))
	}
}
