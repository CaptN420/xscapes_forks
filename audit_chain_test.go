package main

import (
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/event"
	"github.com/donlucasx/xscapes/internal/reduce"
)

// The whole chain, on the real transport: an emitter process writes what an
// adapter would write, the bus takes it in, the reducer folds it, the spectators
// hear it, and the frame the loop draws carries the sentence.
//
// This is the test the unit tests cannot be: they call feedBanter directly. Here
// the only thing that is simulated is the clock.
func TestRealEventsReachTheSpectators(t *testing.T) {
	resetLife(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XSCAPES_SCAPE", "vista")
	t.Setenv("XSCAPES_BANTER", "")

	const sess = "audit-chain-1"
	bus, err := event.Listen(sess)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	f := newFrames(80, 24, 7, false, true, 0.4, 0.62)
	f.follow(bus, reduce.New(sess))

	now := time.Unix(1700000000, 0)
	emit := func(e event.Event) {
		e.Session = sess
		e.Src = "audit"
		if _, err := event.Emit(e); err != nil {
			t.Fatal(err)
		}
	}
	emit(event.Event{Kind: event.Prompt, Text: "add rate limiting"})
	emit(event.Event{Kind: event.ToolStart, Op: event.OpShell, Tool: "Bash", Target: "curl", ID: "t1"})
	emit(event.Event{Kind: event.Error, Op: event.OpShell, Tool: "Bash", Target: "curl", ID: "t1", Detail: "exit 1"})

	// The spool is read on a timer, so the frame has to come after the bus has
	// had its chance to hand the events over.
	deadline := time.Now().Add(2 * time.Second)
	var plain string
	for time.Now().Before(deadline) {
		plain = f.composeFrame(now.Add(300 * time.Millisecond)).RenderPlain()
		if strings.Contains(plain, "saw that") || strings.Contains(plain, "beautifully") ||
			strings.Contains(plain, "opinions") || strings.Contains(plain, "Excuse") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(plain, "saw that") && !strings.Contains(plain, "beautifully") &&
		!strings.Contains(plain, "opinions") && !strings.Contains(plain, "Excuse") {
		t.Fatalf("the event reached no one; frame:\n%s", plain)
	}

	// And the reducer saw the same failure: the companion is worried, the tail
	// carries the operation, the sea is working.
	st := f.red.State(now)
	if st.Pose != companion.Worried {
		t.Errorf("the reducer did not mark the failure: pose %v", st.Pose)
	}
	if !st.Act.Working {
		t.Error("the sea was not held up by the failure")
	}
}
