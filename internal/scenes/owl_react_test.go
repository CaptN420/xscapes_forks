package scenes

import (
	"strings"
	"testing"

	"github.com/donlucasx/xscapes/internal/canvas"
)

// The owl's reactions, drawn in the scene's own language. These tests hold the
// cheap promises: every reaction shows, none of them leaves its own patch of
// screen, nothing panics at any phase or any terminal size, and an unknown
// reaction is silence rather than a crash.

var reactionKinds = []string{"fire", "dance", "catch", "dizzy", "celebrate"}

func reactionCanvas() *canvas.Canvas {
	return canvas.New(40, 20, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
}

func TestEveryReactionDrawsSomething(t *testing.T) {
	for _, kind := range reactionKinds {
		ink := 0
		for _, phase := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
			c := reactionCanvas()
			DrawOwlReaction(c, 10, 6, phase, kind, 42)
			ink += strings.Count(c.RenderPlain(), "*") +
				strings.Count(c.RenderPlain(), "·") +
				strings.Count(c.RenderPlain(), "^") +
				strings.Count(c.RenderPlain(), "x") +
				strings.Count(c.RenderPlain(), "~")
		}
		if ink == 0 {
			t.Errorf("%s draws nothing at any phase", kind)
		}
	}
}

// A reaction is bounded: the bird is 12x7 at (x,y) and its event happens around
// it, never over the writing band or the corner of the screen.
func TestAReactionStaysAroundTheOwl(t *testing.T) {
	const x, y = 12, 6
	for _, kind := range reactionKinds {
		for _, phase := range []float64{0.05, 0.25, 0.5, 0.75, 0.99} {
			c := reactionCanvas()
			DrawOwlReaction(c, x, y, phase, kind, 7)
			for row, line := range strings.Split(c.RenderPlain(), "\n") {
				for col, r := range []rune(line) {
					if r == ' ' {
						continue
					}
					if col < x-2 || col > x+13 || row < y-3 || row > y+7 {
						t.Fatalf("%s at phase %.2f drew (%d,%d), outside the owl's neighbourhood:\n%s",
							kind, phase, col, row, c.RenderPlain())
					}
				}
			}
		}
	}
}

// Out of range is silence: a phase outside 0..1 is not a frame to draw.
func TestNothingDrawsOutsideThePhase(t *testing.T) {
	for _, phase := range []float64{-0.5, 1.5} {
		c := reactionCanvas()
		DrawOwlReaction(c, 10, 6, phase, "fire", 1)
		if strings.TrimSpace(c.RenderPlain()) != "" {
			t.Errorf("phase %.1f drew something", phase)
		}
	}
	c := reactionCanvas()
	DrawOwlReaction(c, 10, 6, 0.5, "not-a-reaction", 1)
	if strings.TrimSpace(c.RenderPlain()) != "" {
		t.Error("an unknown reaction drew something")
	}
}

// The bird itself moves for two of them, and not the others.
func TestOnlyTheJoyfulReactionsMoveTheBird(t *testing.T) {
	if dx, _ := OwlHopOffset("dance", 0.1); dx == 0 {
		t.Error("a dance should sway the bird")
	}
	if _, dy := OwlHopOffset("celebrate", 0.3); dy == 0 {
		t.Error("a celebration should hop")
	}
	for _, kind := range []string{"fire", "catch", "dizzy", ""} {
		if dx, dy := OwlHopOffset(kind, 0.3); dx != 0 || dy != 0 {
			t.Errorf("%s moved the bird by (%d,%d)", kind, dx, dy)
		}
	}
}

// Any terminal, any phase.
func TestReactionsSurviveATinyTerminal(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {4, 2}, {12, 7}, {80, 24}} {
		for _, kind := range reactionKinds {
			for _, phase := range []float64{0, 0.5, 1} {
				c := canvas.New(sz[0], sz[1], canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
				DrawOwlReaction(c, 0, 0, phase, kind, 3) // must not panic
			}
		}
	}
}
