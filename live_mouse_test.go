package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Clicking the companion: the one thing on the strip that answers a mouse.

func TestSGRMouseReportsDecode(t *testing.T) {
	cases := []struct {
		in     string
		col    int
		row    int
		button int
		press  bool
	}{
		{"\x1b[<0;10;5M", 10, 5, 0, true},   // left press
		{"\x1b[<0;10;5m", 10, 5, 0, false},  // left release
		{"\x1b[<2;80;24M", 80, 24, 2, true}, // right press, far corner
		{"\x1b[<1;3;4M", 3, 4, 1, true},     // middle press
	}
	for _, c := range cases {
		got, n, need, ok := parseMouse([]byte(c.in))
		if !ok || need {
			t.Errorf("%q: ok=%v need=%v", c.in, ok, need)
			continue
		}
		if n != len(c.in) {
			t.Errorf("%q: consumed %d of %d", c.in, n, len(c.in))
		}
		if got.col != c.col || got.row != c.row || got.button != c.button || got.press != c.press {
			t.Errorf("%q: got %+v", c.in, got)
		}
	}
}

func TestAReportCutInHalfIsHeldNotSwallowed(t *testing.T) {
	in := &liveInput{clicks: make(chan mouseClick, 4), quit: make(chan struct{}, 1)}
	// The read boundary falls mid-report, which is the common case on a
	// 256-byte read of a fast terminal.
	tail := scanMouse([]byte("\x1b[<2;12;7"), in)
	if len(tail) == 0 {
		t.Fatal("the half report was swallowed instead of held")
	}
	scanMouse(append(tail, 'M'), in)
	select {
	case c := <-in.clicks:
		if c.col != 12 || c.row != 7 || c.button != 2 || !c.press {
			t.Errorf("decoded %+v", c)
		}
	default:
		t.Fatal("the completed report produced no click")
	}
}

func TestABareCtrlCStillQuits(t *testing.T) {
	// In raw mode Ctrl-C is a byte, not a signal, so the reader is what keeps
	// Ctrl-C meaning quit.
	in := &liveInput{clicks: make(chan mouseClick, 4), quit: make(chan struct{}, 1)}
	scanMouse([]byte{0x03}, in)
	select {
	case <-in.quit:
	default:
		t.Fatal("a bare Ctrl-C did not quit")
	}
}

// A left click on the animal pets it: the same answer `xscapes life pet` gives.
func TestAClickOnTheAnimalPetsIt(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	setLifeHit(10, 5, 12, 7)

	if !clickLife(mouseClick{col: 12, row: 7, button: 0, press: true}) {
		t.Fatal("a left click on the animal was not taken")
	}
	if s := lifeBrain.Intent().Speech; !strings.Contains(s, "leans into your hand") {
		t.Errorf("the animal did not answer the click: %q", s)
	}
}

// A right click feeds it.
func TestAClickOnTheAnimalFeedsIt(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	setLifeHit(10, 5, 12, 7)
	if lifeBrain.FoodsEaten() != 0 {
		t.Fatalf("the animal ate before the click: %d", lifeBrain.FoodsEaten())
	}

	if !clickLife(mouseClick{col: 11, row: 6, button: 2, press: true}) {
		t.Fatal("a right click on the animal was not taken")
	}
	if lifeBrain.FoodsEaten() != 1 {
		t.Errorf("the right click did not feed the animal: %d", lifeBrain.FoodsEaten())
	}
}

// The hit test is the whole feature: a click in the water does nothing, so the
// strip stays a picture you can watch without disturbing it.
func TestAClickOffTheAnimalIsIgnored(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	setLifeHit(10, 5, 12, 7)

	if clickLife(mouseClick{col: 40, row: 20, button: 0, press: true}) {
		t.Error("a click in empty water was taken")
	}
	if lifeBrain.FoodsEaten() != 0 {
		t.Error("a stray click fed the animal")
	}
	if s := lifeBrain.Intent().Speech; s != "" {
		t.Errorf("a stray click made it speak: %q", s)
	}
	// The box is columns 10..21 (x=10, w=12), so one past the right edge is
	// column 23 in the terminal's 1-based coordinates.
	if clickLife(mouseClick{col: 23, row: 7, button: 0, press: true}) {
		t.Error("a click one cell past the box was taken")
	}
}

// Before any frame is drawn there is nothing to click.
func TestAClickWithNothingDrawnIsIgnored(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	lifeHit = hitBox{}

	if clickLife(mouseClick{col: 11, row: 6, button: 0, press: true}) {
		t.Error("a click landed with nothing on screen")
	}
}

// The whole path the live loop takes: reader -> channel -> drain -> animal.
func TestTheLoopDrainsAClickIntoAFeed(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	setLifeHit(10, 5, 12, 7)

	in := &liveInput{clicks: make(chan mouseClick, 4), quit: make(chan struct{}, 1)}
	// Exactly what the terminal sends for a right click on the animal.
	scanMouse([]byte("\x1b[<2;11;6M"), in)
	drainInput(in, func() { t.Error("the drain shut the scape down") })

	if lifeBrain.FoodsEaten() != 1 {
		t.Errorf("the loop did not feed the animal: %d", lifeBrain.FoodsEaten())
	}
}

// A composed frame records a box, and a click inside it is an interaction --
// the renderer and the input path agreeing on where the animal is drawn.
func TestAComposedFramePutsTheAnimalWhereAClickCanFindIt(t *testing.T) {
	resetLife(t)
	home, err := os.MkdirTemp("", "xs-hit")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("XSCAPES_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("XSCAPES_LIFE", "")
	initLife()

	f := newFrames(100, 30, 7, false, true, 0.4, 0.62)
	f.composeFrame(time.Unix(1700000000, 0))

	if !lifeHit.ok || lifeHit.w == 0 || lifeHit.h == 0 {
		t.Fatalf("the renderer recorded no clickable box: %+v", lifeHit)
	}
	if lifeHit.x < 0 || lifeHit.y < 0 || lifeHit.x+lifeHit.w > f.c.W || lifeHit.y+lifeHit.h > f.c.H {
		t.Errorf("the box %+v is outside the %dx%d frame", lifeHit, f.c.W, f.c.H)
	}

	cx, cy := lifeHit.x+lifeHit.w/2, lifeHit.y+lifeHit.h/2
	in := &liveInput{clicks: make(chan mouseClick, 4), quit: make(chan struct{}, 1)}
	scanMouse([]byte("\x1b[<2;"+lifeNum(cx+1)+";"+lifeNum(cy+1)+"M"), in)
	drainInput(in, func() { t.Error("the drain shut the scape down") })
	if lifeBrain.FoodsEaten() != 1 {
		t.Errorf("a click on the recorded box did not feed the animal: %d", lifeBrain.FoodsEaten())
	}
}

// A release is not a second interaction: press feeds, release is ignored.
func TestAReleaseIsNotAnInteraction(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	setLifeHit(10, 5, 12, 7)

	in := &liveInput{clicks: make(chan mouseClick, 4), quit: make(chan struct{}, 1)}
	scanMouse([]byte("\x1b[<2;11;6M\x1b[<2;11;6m"), in)
	drainInput(in, func() {})

	if lifeBrain.FoodsEaten() != 1 {
		t.Errorf("press+release fed it %d times, want 1", lifeBrain.FoodsEaten())
	}
}
