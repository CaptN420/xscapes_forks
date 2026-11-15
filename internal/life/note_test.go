package life

import (
	"strings"
	"testing"
)

// A story is said in the scene's budget but kept whole. Cutting the sentence
// that is drawn is right; cutting the sentence that is remembered is not.

func TestTheWholeStoryIsKeptAndAShortLineIsDrawn(t *testing.T) {
	b := New(9)
	long := strings.Repeat("a long story about the tideline ", 8) // ~250 chars
	b.Note(long)

	if got := b.LastSaid(); len(got) < 150 {
		t.Errorf("the story was cut down to %d chars: %q", len(got), got)
	}
	if got := b.Intent().Speech; len([]rune(got)) > 62 {
		t.Errorf("the drawn line is %d runes, over the scene's budget: %q", len([]rune(got)), got)
	}
	if len(b.Memories()) == 0 {
		t.Error("a story that long should be remembered")
	}
	// And the memory is the whole sentence, not the drawn fragment.
	best := ""
	for _, m := range b.Memories() {
		if len(m) > len(best) {
			best = m
		}
	}
	if len(best) < 150 {
		t.Errorf("the memory kept only %d chars: %q", len(best), best)
	}
}

// A note that is already short is said and kept unchanged.
func TestAShortNoteIsUntouched(t *testing.T) {
	b := New(3)
	b.Note("Came back with a pearl.")
	if b.LastSaid() != "Came back with a pearl." {
		t.Errorf("the note changed: %q", b.LastSaid())
	}
	if b.Intent().Speech != "Came back with a pearl." {
		t.Errorf("the drawn line is %q", b.Intent().Speech)
	}
}

// An empty note is silence, not an empty speech bubble.
func TestAnEmptyNoteSaysNothing(t *testing.T) {
	b := New(3)
	b.Note("   ")
	if b.LastSaid() != "" || b.Intent().Speech != "" {
		t.Error("an empty note produced speech")
	}
}
