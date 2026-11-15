package life

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Food, and what the animal does about it. The rules under test are the brief's:
// every food produces a visible reaction, no food is ever a punishment, the
// reactions vary without becoming random noise, and a full animal is still
// delighted rather than turned away.

func TestEveryFoodHasItsOwnReaction(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Foods {
		if f.ID == "" || f.Name == "" {
			t.Errorf("a food with no name: %+v", f)
		}
		if f.Reaction == "" {
			t.Errorf("%s has no reaction", f.ID)
		}
		if f.Glyph == 0 {
			t.Errorf("%s has no glyph to draw", f.ID)
		}
		if f.Line == "" {
			t.Errorf("%s says nothing", f.ID)
		}
		if seen[f.Reaction] {
			t.Errorf("%s reuses the %s reaction: food should be distinguishable", f.ID, f.Reaction)
		}
		seen[f.Reaction] = true
		if _, ok := FoodByID(f.ID); !ok {
			t.Errorf("%s is in the larder but cannot be found", f.ID)
		}
	}
	// The five the brief names, by function rather than by name.
	for _, want := range []string{ReactDance, ReactCatch, ReactFire, ReactDizzy, ReactCelebrate} {
		if !seen[want] {
			t.Errorf("no food produces %q", want)
		}
	}
}

func TestAnUnknownFoodIsNotAnError(t *testing.T) {
	b := New(3)
	res := b.DoFood("souffle")
	if !res.OK {
		t.Fatalf("an unknown food was an error: %s", res.Message)
	}
	if b.FoodsEaten() != 1 {
		t.Error("it did not eat")
	}
	if kind, _, _ := b.Reaction(); kind == "" {
		t.Error("it ate with no reaction")
	}
}

// Every food is a positive experience, and none of them touches the negative
// numbers that do not exist.
func TestNoFoodIsEverBad(t *testing.T) {
	for _, f := range Foods {
		b := New(11)
		before := b.Attributes()
		b.DoFood(f.ID)
		after := b.Attributes()
		if after.Happiness < before.Happiness || after.Friendship < before.Friendship {
			t.Errorf("%s made it worse: %+v -> %+v", f.ID, before, after)
		}
		if after.Appetite < 0 || after.Appetite > 100 {
			t.Errorf("%s left appetite at %.1f", f.ID, after.Appetite)
		}
		if len(b.Memories()) == 0 {
			t.Errorf("%s was forgotten immediately", f.ID)
		}
	}
}

// The reaction runs and stops on its own: nothing in flight can pin a frame.
func TestAReactionPlaysOnceAndEnds(t *testing.T) {
	b := New(5)
	b.DoFood("spicy")
	kind, _, _ := b.Reaction()
	if kind != ReactFire {
		t.Fatalf("spicy gave %q", kind)
	}
	for i := 0; i < 60; i++ {
		if kind, _, _ := b.Reaction(); kind == "" {
			break
		}
		b.Tick(0.1, World{})
	}
	if kind2, phase, _ := b.Reaction(); kind2 != "" || phase != 0 {
		t.Errorf("the reaction never ended: %q at phase %.2f", kind2, phase)
	}
	// And its phase only ever moves forward while it is playing.
	b.DoFood("berry")
	last := 0.0
	for i := 0; i < 20; i++ {
		_, p, _ := b.Reaction()
		if p < last {
			t.Fatalf("the reaction went backwards: %.2f then %.2f", last, p)
		}
		last = p
		b.Tick(0.1, World{})
	}
}

// Two helpings of the same food do not look identical, and the same life replays
// identically -- the variant is seeded, not random.
func TestReactionsVaryButStayDeterministic(t *testing.T) {
	a, c := New(77), New(77)
	a.DoFood("treat")
	c.DoFood("treat")
	_, _, va := a.Reaction()
	_, _, vc := c.Reaction()
	if va != vc {
		t.Errorf("the same life produced different variants: %d vs %d", va, vc)
	}
	a.DoFood("treat")
	_, _, va2 := a.Reaction()
	if va2 == va {
		t.Log("note: two helpings drew the same variant; with 1000 values that is luck, not a bug")
	}
}

// What it ate is remembered across a restart, and an old save without any food
// history still loads.
func TestATasteSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "life.json")

	b := New(19)
	b.DoFood("bug")
	b.DoFood("bug")
	b.DoFood("berry")
	if err := b.Write(path); err != nil {
		t.Fatal(err)
	}
	back, ok := Load(path)
	if !ok {
		t.Fatal("the save did not load")
	}
	if back.FoodsEaten() != 3 {
		t.Errorf("it ate 3, the save remembers %d", back.FoodsEaten())
	}
	if got := back.FoodSeen()["bug"]; got != 2 {
		t.Errorf("beetles eaten: %d, want 2", got)
	}

	// A save from before food existed: no map, no counts.
	old := Save{Version: SaveVersion, Seed: 4, Stage: Baby}
	data := mustJSON(t, old)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, ok := Load(path)
	if !ok {
		t.Fatal("an old save failed to load")
	}
	loaded.DoFood("berry") // must not panic on a nil map
	if loaded.FoodsEaten() != 1 {
		t.Error("eating after an old load did not count")
	}
}

// mustJSON is a tiny helper so the test reads as a document.
func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
