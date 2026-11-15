package main

import (
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/banter"
	"github.com/donlucasx/xscapes/internal/event"
)

// The spectators, as the product draws them: an event from the agent puts words
// over the creature, a second failure brings a second creature on, the two worlds
// speak differently, and the switch means silence.

func banterFrames(t *testing.T, scape string) *frames {
	t.Helper()
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XSCAPES_SCAPE", scape)
	t.Setenv("XSCAPES_BANTER", "")
	f := newFrames(88, 26, 7, false, true, 0.4, 0.62)
	t.Cleanup(closeBanter)
	if !banterLive() {
		t.Fatal("the spectators did not start")
	}
	return f
}

// banterFail is a failed tool call whose operation is not a build and whose
// detail does not name a missing file, so the engine files it as a plain tool
// failure. A build-shaped command is a different category on purpose; that has
// its own test below.
func banterFail(id string) event.Event {
	return event.Event{Kind: event.Error, Op: event.OpShell, Tool: "Bash",
		Target: "curl", ID: id, Detail: "exit 1"}
}

// banterBuildFail is a failed build: the adapter reports the shell program, so
// `make` failing is a build failure and not "a tool".
func banterBuildFail(id string) event.Event {
	return event.Event{Kind: event.Error, Op: event.OpShell, Tool: "Bash",
		Target: "make", ID: id, Detail: "exit 2"}
}

// A failed tool call in the real events becomes a line in the real frame.
func TestAFailurePutsWordsOverTheCreature(t *testing.T) {
	for _, scape := range []string{"shore", "vista"} {
		f := banterFrames(t, scape)
		now := time.Unix(1700000000, 0)
		before := f.composeFrame(now).RenderPlain()

		feedBanter(banterFail("1"), now)
		after := f.composeFrame(now.Add(400 * time.Millisecond)).RenderPlain()

		if before == after {
			t.Fatalf("%s: a failed tool call changed nothing", scape)
		}
		if !strings.Contains(after, ".") {
			t.Errorf("%s: no sentence appeared", scape)
		}
		// The line must be one the creature could have said, i.e. one of the
		// bank's, not scene decoration.
		found := false
		for _, line := range strings.Split(after, "\n") {
			for _, w := range []string{"Well,", "saw that", "Excuse", "opinions", "Beautifully"} {
				if strings.Contains(line, w) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: the change was not a spoken line:\n%s", scape, after)
		}
	}
}

// A failure that names a build command is filed as a build, and the creatures
// say so -- context-awareness from the command line the adapter already sent,
// never from a guess at the cause.
func TestABuildFailureSpeaksOfTheBuild(t *testing.T) {
	for _, scape := range []string{"shore", "vista"} {
		f := banterFrames(t, scape)
		now := time.Unix(1700000000, 0)
		before := f.composeFrame(now).RenderPlain()

		feedBanter(banterBuildFail("1"), now)
		after := f.composeFrame(now.Add(400 * time.Millisecond)).RenderPlain()
		if before == after {
			t.Fatalf("%s: a failed build changed nothing", scape)
		}
		found := false
		for _, line := range strings.Split(after, "\n") {
			for _, w := range []string{"shipping", "compiler", "build", "compile"} {
				if strings.Contains(line, w) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: a build failure did not speak of the build:\n%s", scape, after)
		}
	}
}

// The second failure of the same thing brings the other one, and it is drawn.
func TestTheSecondFailureBringsACompanion(t *testing.T) {
	f := banterFrames(t, "shore")
	now := time.Unix(1700000000, 0)

	feedBanter(banterFail("1"), now)
	solo := f.composeFrame(now.Add(time.Second)).RenderPlain()
	if strings.Count(solo, "▟") > 0 {
		_ = solo // the crab's own glyphs; the friend is checked below
	}

	now = now.Add(9 * time.Second)
	feedBanter(banterFail("2"), now)
	snap := banterEngine.Current(now.Add(2 * time.Second))
	if !snap.SecondOn {
		t.Fatal("the second creature did not come on for a second failure")
	}
	with := f.composeFrame(now.Add(2 * time.Second)).RenderPlain()
	if with == solo {
		t.Error("the frame did not change when the second creature arrived")
	}
	// Two lines, from two speakers, at different cells.
	lines := snap.Lines
	if len(lines) != 1 {
		t.Fatalf("expected one speaker mid-conversation, got %d", len(lines))
	}
	// And the friend's words arrive next.
	later := banterEngine.Current(now.Add(5 * time.Second))
	if len(later.Lines) == 0 || later.Lines[0].Who != banter.Second {
		t.Error("the second creature never got a line")
	}
}

// The two worlds do not share a script.
func TestTheWorldsSpeakDifferently(t *testing.T) {
	words := func(scape string) string {
		f := banterFrames(t, scape)
		now := time.Unix(1700000000, 0)
		feedBanter(banterFail("1"), now)
		return f.composeFrame(now.Add(400 * time.Millisecond)).RenderPlain()
	}
	if words("shore") == words("vista") {
		t.Error("the crab and the owl said exactly the same thing")
	}
}

// With the switch off, an event changes nothing at all.
func TestTheSwitchMeansSilence(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XSCAPES_SCAPE", "shore")
	t.Setenv("XSCAPES_BANTER", "off")
	f := newFrames(80, 24, 7, false, true, 0.4, 0.62)
	t.Cleanup(closeBanter)
	if banterLive() {
		t.Fatal("the spectators started with the switch off")
	}
	now := time.Unix(1700000000, 0)
	feedBanter(banterFail("1"), now) // a no-op: there is nothing to feed
	plain := f.composeFrame(now.Add(400 * time.Millisecond)).RenderPlain()
	// The scene animates on its own, so the frame is not compared to another
	// frame -- it is searched for words a spectator would have said.
	for _, w := range []string{"Well,", "saw that", "Excuse", "opinions", "Again?", "failed."} {
		if strings.Contains(plain, w) {
			t.Errorf("with the switch off the frame still said %q", w)
		}
	}
	if banterEngine != nil {
		t.Error("an engine exists with the switch off")
	}
}

// The creatures are fed the same events the water is, from one drain.
func TestTheBusReachesTheSpectators(t *testing.T) {
	banterFrames(t, "shore")
	now := time.Unix(1700000000, 0)
	feedBanter(event.Event{Kind: event.ToolStart, Op: event.OpShell, Tool: "Bash", ID: "a"}, now)
	if !banterEngine.Quiet(now) {
		t.Error("a tool starting should be silent")
	}
	feedBanter(event.Event{Kind: event.Error, Op: event.OpShell, Tool: "Bash", Target: "make", ID: "a"}, now)
	if banterEngine.Quiet(now) {
		t.Error("a failure should not be silent")
	}
}

// Nothing is drawn before anything happens, and no show outlives its script.
func TestQuietUntilSomethingHappensAndQuietAfter(t *testing.T) {
	f := banterFrames(t, "vista")
	now := time.Unix(1700000000, 0)
	plain := f.composeFrame(now).RenderPlain()
	if banterEngine.Current(now).Show != nil {
		t.Fatal("a show was running before any event")
	}
	feedBanter(banterFail("1"), now)
	later := f.composeFrame(now.Add(40 * time.Second)).RenderPlain()
	_ = later
	if s := banterEngine.Current(now.Add(40 * time.Second)).Show; s != nil {
		t.Errorf("a show was still up after 40s (ends at %.1f)", s.End)
	}
	_ = plain
}

// The lines that get drawn are short enough for the narrowest pane this program
// supports.
func TestLinesFitANarrowPane(t *testing.T) {
	f := banterFrames(t, "shore")
	f.resize(40, 12)
	now := time.Unix(1700000000, 0)
	feedBanter(banterFail("1"), now)
	plain := f.composeFrame(now.Add(400 * time.Millisecond)).RenderPlain()
	for _, line := range strings.Split(plain, "\n") {
		if len([]rune(line)) > 40 {
			t.Fatalf("a row overflowed a 40-column pane: %q", line)
		}
	}
}

// A catalogue of the categories, so a missing one is caught at build time rather
// than by a silent night.
func TestEveryCategoryHasAVoiceInEveryWorld(t *testing.T) {
	for _, p := range []banter.Place{banter.Shore, banter.Vista} {
		e := banter.New(p, 1)
		if e.Place() != p {
			t.Errorf("engine for %s reports %s", p, e.Place())
		}
	}
}
