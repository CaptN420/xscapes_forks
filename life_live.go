package main

import (
	"github.com/donlucasx/xscapes/internal/event"
)

// The live channel for interactions: a command typed in another terminal
// reaches a RUNNING scape through the same bus the agent's events use, so the
// animal reacts on screen within a frame and not only in the file the next
// process will read.
//
// There are two writers and that is deliberate. `xscapes life feed` applies the
// interaction to the file itself, so it works with nothing else open; it then
// emits the interaction, and the running scape applies the SAME single
// interaction to its own brain and saves. Each brain adds one helping, and each
// write holds one helping -- the file never shows two, because the two writers
// are not two interactions.
//
// The spool is followed from its end, so a scape that starts later never
// replays an interaction that was already handled (see internal/event/recv.go).

// lifeSession resolves the session a running scape would be listening on: the
// agent's own environment first, then the newest session pointer.
func lifeSession() string {
	if s := event.SessionFromEnv(); s != "" {
		return s
	}
	return event.Current()
}

// emitLifeInteract tells a running scape about one interaction, best effort. A
// failure here never fails the command: the local apply has already happened,
// and a decoration that breaks the thing it decorates is worse than none.
func emitLifeInteract(verb, arg string) {
	sess := lifeSession()
	if sess == "" {
		return
	}
	_, _ = event.Emit(event.Event{
		Kind:    event.LifeInteract,
		Session: sess,
		Src:     "life",
		Target:  verb,
		Text:    arg,
	})
}

// handleLifeEvent applies a live interaction to the running companion. It
// reports whether the event was ours, so the drain does not hand it to the
// reducer, for which it has no meaning.
func handleLifeEvent(e event.Event) bool {
	if e.Kind != event.LifeInteract {
		return false
	}
	if !lifeLive() {
		// Ours, but there is no animal on this scape to touch.
		return true
	}
	if e.Target == "feed" && e.Text != "" {
		lifeBrain.DoFood(e.Text)
	} else {
		lifeBrain.Do(e.Target)
	}
	// Keep it, so the reaction on screen is also what the next process reads.
	saveLife()
	return true
}
