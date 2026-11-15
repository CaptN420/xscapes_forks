package companion_test

import (
	"testing"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/term"
)

// TestCatBGColor verifies that setting the canvas background before drawing
// the cat produces the expected result.
func TestCatBGColor(t *testing.T) {
	cat := companion.NewCat()

	l := canvas.New(14, 9, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	bg := term.RGB{R: 30, G: 45, B: 60}

	// Set the background before drawing the cat.
	w, h := cat.Size()
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			l.SetBG(1+dx, 1+dy, bg)
		}
	}

	cat.Draw(l.Near(), 1, 1, 3.0, companion.Working)

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			got := l.BGAt(1+dx, 1+dy)
			if got != bg {
				t.Errorf("cell (%d,%d) = %v, want %v", dx, dy, got, bg)
			}
		}
	}
}

// TestCatNoBGColor verifies that without setting a custom background, the
// scene background is preserved.
func TestCatNoBGColor(t *testing.T) {
	cat := companion.NewCat()

	l := canvas.New(14, 9, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			l.SetBG(x, y, term.RGB{R: 10, G: 20, B: 30})
		}
	}

	cat.Draw(l.Near(), 1, 1, 3.0, companion.Working)

	// Cells behind the cat should keep the scene background (not overwritten).
	w, h := cat.Size()
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			got := l.BGAt(1+dx, 1+dy)
			want := term.RGB{R: 10, G: 20, B: 30}
			if got != want {
				t.Errorf("cell (%d,%d) = %v, want %v", dx, dy, got, want)
			}
		}
	}
}
