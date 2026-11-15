package main

import (
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/reduce"
)

// The vista's owl, as the product draws it: its own life, its own genies of
// smoke and fire, and the agent's cue untouched.

func vistaFrames(t *testing.T) *frames {
	t.Helper()
	t.Setenv("XSCAPES_SCAPE", "vista")
	f := newFrames(90, 30, 7, false, true, 0.4, 0.62)
	if f.vista == nil {
		t.Fatal("the vista did not come up")
	}
	return f
}

// A fed owl does something you can see, and the something is around the bird.
func TestTheOwlReactsToAFeed(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	f := vistaFrames(t)
	now := time.Unix(1700000000, 0)

	before := f.composeFrame(now).RenderPlain()
	if res := lifeBrain.DoFood("spicy"); !res.OK {
		t.Fatalf("feeding failed: %s", res.Message)
	}

	// Sample the whole reaction rather than one instant: the flame inhales,
	// blooms and gutters, and a single frame is a bad witness.
	marks := func(s string) int {
		return countRunes(s, '^') + countRunes(s, '·') + countRunes(s, '`') + countRunes(s, '*')
	}
	base := marks(before)
	best, changed := 0, false
	for i := 1; i <= 12; i++ {
		plain := f.composeFrame(now.Add(time.Duration(i) * 150 * time.Millisecond)).RenderPlain()
		if plain != before {
			changed = true
		}
		if d := marks(plain) - base; d > best {
			best = d
		}
	}
	if !changed {
		t.Fatal("a spicy snack changed nothing on screen over its whole reaction")
	}
	if best <= 0 {
		t.Error("no fire appeared: the reaction never added a mark")
	}
}

// The life layer's own pose reaches the owl, not just the crab on the shore.
func TestTheOwlHasItsOwnPosture(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	f := vistaFrames(t)
	now := time.Unix(1700000000, 0)

	// A night world with a quiet agent puts it to sleep; a broken one wakes it
	// wary. Both frames are composed with the same clock, so the difference can
	// only come from the bird.
	f.tod = 0.95 // deep night
	night := f.composeFrame(now).RenderPlain()
	f.tod = 0.5 // midday
	day := f.composeFrame(now).RenderPlain()
	if night == day {
		t.Error("the owl looks the same at midnight and at noon: the life layer is not reaching it")
	}
}

// The agent's balloon is still the agent's, and the owl never speaks over it.
func TestTheOwlKeepsOutOfTheAgentsCue(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	_ = vistaFrames(t) // the same composition, to prove the owl is drawing

	st := reduce.State{Bubble: "allow Bash?", BubbleAsk: true}
	for i := 0; i < 40; i++ {
		updateLife(time.Unix(1700000000, int64(i)), st)
	}
	if st.Bubble != "allow Bash?" || !st.BubbleAsk {
		t.Errorf("the owl overwrote the agent's bubble: %q", st.Bubble)
	}
}

func countRunes(s string, r rune) int { return strings.Count(s, string(r)) }
