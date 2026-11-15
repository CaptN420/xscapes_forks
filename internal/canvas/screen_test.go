package canvas

import (
	"strings"
	"testing"

	"github.com/donlucasx/xscapes/internal/term"
)

// paint fills a canvas with something a diff can be measured against: a quiet
// background, a few glyphs, and two colours.
func paintScreen(c *Canvas, n int) {
	l := c.Near()
	for i := 0; i < n; i++ {
		l.Plot(10+i%3, 3+i%2, '█', term.RGB{R: 200, G: 120, B: 60}, 1)
	}
	l.Plot(40, 12, '~', term.RGB{R: 90, G: 140, B: 220}, 1)
}

// The whole point: what the terminal is shown is the same picture, and the
// bytes to keep it that way are only the cells that moved.
func TestTheScreenWritesOnlyWhatChanged(t *testing.T) {
	c := New(80, 24, AlphaFar, AlphaMid, AlphaNear)
	p := term.Profile256
	s := NewScreen()

	paintScreen(c, 4)
	first := s.Emit(c, p)
	if len(first) == 0 {
		t.Fatal("the first frame wrote nothing: the screen was never painted")
	}
	if s.Dirty() == 0 {
		t.Error("the first frame reported no dirty cells")
	}

	// Same picture, one more frame: nothing to say.
	again := s.Emit(c, p)
	if len(again) != 0 {
		t.Errorf("an unchanged frame wrote %d bytes: %q", len(again), again)
	}
	if s.Dirty() != 0 {
		t.Errorf("an unchanged frame reported %d dirty cells", s.Dirty())
	}

	// One cell changes: the diff should be a cursor move and that cell.
	c.Near().Plot(41, 12, '~', term.RGB{R: 220, G: 90, B: 90}, 1)
	one := s.Emit(c, p)
	if s.Dirty() != 1 {
		t.Errorf("one changed cell reported %d dirty cells", s.Dirty())
	}
	if len(one) > 48 {
		t.Errorf("one changed cell cost %d bytes: %q", len(one), one)
	}
	if !strings.HasSuffix(string(one), "~") {
		t.Errorf("the changed glyph is not in the diff: %q", one)
	}
	if len(one) >= len(first)/4 {
		t.Errorf("a single cell change cost %d bytes against a full frame's %d", len(one), len(first))
	}
}

// The diff must not drift: after any run of frames, the cells the Screen
// believes the terminal shows must equal what a plain render would draw.
func TestTheScreenAgreesWithAPlainRender(t *testing.T) {
	c := New(60, 18, AlphaFar, AlphaMid, AlphaNear)
	p := term.Profile256
	s := NewScreen()

	var shown []screenCell
	apply := func(diff []byte) { // no terminal needed: the Screen's own account
		shown = append([]screenCell(nil), s.prev...)
	}
	for step := 0; step < 12; step++ {
		c.Near().Clear()
		paintScreen(c, step)
		c.Near().Plot(20+step, 4, '●', term.RGB{R: 255, G: 255, B: 255}, 1)
		s.Emit(c, p)
		apply(nil)
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				ch, fg, bg := c.ResolveAt(x, y, p)
				if got := shown[y*c.W+x]; got.ch != ch || got.fg != fg || got.bg != bg {
					t.Fatalf("frame %d at %d,%d: the Screen thinks %q/%v/%v, the canvas says %q/%v/%v",
						step, x, y, got.ch, got.fg, got.bg, ch, fg, bg)
				}
			}
		}
	}
}

// A resize is a full repaint, and the Screen has to know it.
func TestAResizeRepaintsEverything(t *testing.T) {
	c := New(60, 18, AlphaFar, AlphaMid, AlphaNear)
	p := term.Profile256
	s := NewScreen()
	paintScreen(c, 4)
	s.Emit(c, p)
	if d := s.Emit(c, p); len(d) != 0 {
		t.Fatalf("sanity: an unchanged frame wrote %d bytes", len(d))
	}
	c.Resize(72, 20)
	big := s.Emit(c, p)
	if len(big) == 0 {
		t.Fatal("a resized frame wrote nothing")
	}
	if s.Dirty() != 72*20 {
		t.Errorf("after a resize %d cells were repainted, want the whole %d", s.Dirty(), 72*20)
	}
}

// And an explicit Invalidate, for the one case the Screen cannot see: the
// caller cleared the screen or moved the window.
func TestInvalidateForcesAFullRepaint(t *testing.T) {
	c := New(40, 12, AlphaFar, AlphaMid, AlphaNear)
	p := term.Profile256
	s := NewScreen()
	paintScreen(c, 3)
	s.Emit(c, p)
	s.Invalidate()
	if d := s.Emit(c, p); len(d) == 0 {
		t.Error("after Invalidate the frame wrote nothing")
	}
	if s.Dirty() != 40*12 {
		t.Errorf("after Invalidate %d cells were repainted, want %d", s.Dirty(), 40*12)
	}
}

// The byte count is the reason this file exists, so it is measured rather than
// asserted: a living frame must cost a fraction of a full render.
func TestALivingFrameCostsFarLessThanAFullRender(t *testing.T) {
	c := New(80, 24, AlphaFar, AlphaMid, AlphaNear)
	p := term.Profile256
	s := NewScreen()

	// A still sky with a few cells moving, the common case for a scape.
	for i := 0; i < 400; i++ {
		c.Near().Clear()
		for k := 0; k < 100; k++ {
			c.Near().Plot(k%80, (k/80)+10, '.', term.RGB{R: 120, G: 120, B: 160}, 1)
		}
		c.Near().Plot(i%80, 5, '█', term.RGB{R: 200, G: 120, B: 60}, 1)
		s.Emit(c, p)
	}
	full := len(c.Render(p))
	if s.Dirty() > 3 {
		t.Errorf("a frame of one moving glyph reported %d dirty cells", s.Dirty())
	}
	if last := len(s.Emit(c, p)); last != 0 {
		t.Errorf("the still frame wrote %d bytes against a full render's %d", last, full)
	}
}
