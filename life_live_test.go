package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/event"
	"github.com/donlucasx/xscapes/internal/reduce"
)

// A command typed in another terminal reaches a RUNNING scape: the interaction
// rides the same bus the agent's events use, the drain hands it to the
// companion, and the animal reacts -- so the pet is visible and usable while it
// is on screen, not only in the file the next process will read.
//
// This is the test the unit tests cannot be: it goes through the real
// transport (emit -> spool -> bus -> drain), so it fails if the wiring, and not
// just the brain, is wrong.
func TestAnInteractionReachesTheRunningScape(t *testing.T) {
	resetLife(t)
	// A short XSCAPES_HOME keeps the session socket inside sockaddr_un's
	// limit; t.TempDir() embeds the test name and overflows it.
	home, err := os.MkdirTemp("", "xs-live")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("XSCAPES_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("XSCAPES_SCAPE", "shore")
	t.Setenv("XSCAPES_LIFE", "")
	initLife()

	const sess = "life-live-1"
	bus, err := event.Listen(sess)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	f := newFrames(88, 26, 7, false, true, 0.4, 0.62)
	f.follow(bus, reduce.New(sess))
	if !lifeLive() {
		t.Fatal("no companion came up")
	}
	now := time.Unix(1700000000, 0)
	if lifeBrain.FoodsEaten() != 0 {
		t.Fatalf("the animal ate before it was fed: %d", lifeBrain.FoodsEaten())
	}

	// Exactly what `xscapes life feed spicy` emits.
	if _, err := event.Emit(event.Event{
		Kind: event.LifeInteract, Session: sess, Src: "life",
		Target: "feed", Text: "spicy",
	}); err != nil {
		t.Fatal(err)
	}

	// The spool is read on a timer, so compose frames until the animal has
	// eaten or the clock runs out.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && lifeBrain.FoodsEaten() == 0 {
		f.composeFrame(now)
		time.Sleep(20 * time.Millisecond)
	}
	if lifeBrain.FoodsEaten() != 1 {
		t.Fatalf("the interaction never reached the running scape: %d eaten", lifeBrain.FoodsEaten())
	}
	// And it is a reaction, not a silent state change: something to draw.
	if kind, _, _ := lifeBrain.Reaction(); kind == "" {
		t.Error("the feed armed no reaction: nothing would be drawn")
	}
	// The companion also has words for it, which the scene paints.
	if s := lifeBrain.Intent().Speech; s == "" {
		t.Error("the feed left the companion silent")
	}
}

// The command itself tells the running scape: `xscapes life feed spicy` writes
// the interaction to the session's spool, where a running scape reads it. This
// is the half the drain test cannot see -- that the command emits at all, and
// only the interaction, never anything around it.
func TestTheCommandTellsTheRunningScape(t *testing.T) {
	resetLife(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XSCAPES_HOME", filepath.Join(home, ".config", "xscapes"))
	t.Setenv("XSCAPES_SESSION", "life-cmd-live")
	t.Setenv("XSCAPES_LIFE", "")
	t.Setenv("XSCAPES_LIFE_AI", "off")

	if code := cmdLife([]string{"feed", "spicy"}); code != 0 {
		t.Fatalf("life feed returned %d", code)
	}

	found := ""
	_ = filepath.WalkDir(home, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		if strings.Contains(string(b), "life_interact") && strings.Contains(string(b), "spicy") {
			found = p
		}
		return nil
	})
	if found == "" {
		t.Error("the command never told the running scape about the feed")
	}
}

// handleLifeEvent claims the interaction and leaves every other kind alone, so
// the reducer still sees the agent's own events.
func TestOnlyLifeInteractionsAreClaimed(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()

	if handleLifeEvent(event.Event{Kind: event.ToolStart, Target: "feed"}) {
		t.Error("a tool_start was claimed as an interaction")
	}
	if handleLifeEvent(event.Event{Kind: event.Done}) {
		t.Error("a done was claimed as an interaction")
	}
	if !handleLifeEvent(event.Event{Kind: event.LifeInteract, Target: "pet"}) {
		t.Error("an interaction was not claimed")
	}
}
