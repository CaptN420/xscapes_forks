// Package main includes Tamagotchi integration for xscapes.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/event"
	"github.com/donlucasx/xscapes/internal/reduce"
	"github.com/donlucasx/xscapes/internal/tamagotchi"
	"github.com/donlucasx/xscapes/internal/term"
)

// tamagotchiState holds the virtual creature state.
var tamagotchiState *companion.TamagotchiState

// tamagotchiRenderer handles rendering the creature.
var tamagotchiRenderer *companion.TamagotchiRenderer

// tamLastUpdate is when the creature's clock last advanced. The creature's
// needs are a function of REAL SECONDS, not of frames: the old code decayed
// them by a fixed amount on every frame, so the same pet aged a third faster at
// 30 fps than at 20 and nobody could say what its metabolism was.
var tamLastUpdate time.Time

// tamNagAt is when the creature last changed what it is saying, so a hungry one
// holds a sentence instead of a new one every frame.
var tamNagAt time.Time

// TamDecayPerSecond is that metabolism, stated once. It is the old per-frame
// constant at the shipped loop's 20 fps, so the creature keeps the appetite it
// had and stops caring how fast the terminal redraws.
const TamDecayPerSecond = 1.0 / 30.0 / 60.0 * 24.0 * 20.0

// initTamagotchi initializes the Tamagotchi creature, loading the saved one if
// there is one.
//
// Until now nothing outside `xscapes tamagotchi` called this, so the creature
// was nil in a live scape and nothing it owns -- the animations, the gauges, the
// growth -- was ever drawn. It is one life: the stage it reached is the stage it
// wakes up in.
func initTamagotchi() {
	st := companion.NewTamagotchiState()
	if loadTamagotchi(&st) {
		// A held animation from the previous run would freeze it mid-chew.
		st.AnimLeft = 0
		switch st.Animation {
		case companion.ClipEat, companion.ClipPlay, companion.ClipPet:
			st.Animation = companion.ClipIdle
		}
	}
	tamagotchiState = &st
	tamagotchiRenderer = companion.NewTamagotchiRenderer(tamagotchiState)
	tamLastUpdate = time.Time{}
	tamNagAt = time.Time{}
}

// tamagotchiLive reports whether the creature is up.
func tamagotchiLive() bool { return tamagotchiState != nil && tamagotchiRenderer != nil }

// saveTamagotchi writes the creature to the project's state directory: the same
// ~/.config/xscapes everything else persists in. Written on the way out, not
// every frame -- the file is a keepsake, not a log.
func saveTamagotchi() {
	if tamagotchiState == nil {
		return
	}
	data, err := json.MarshalIndent(tamagotchiState, "", "  ")
	if err != nil {
		return
	}
	path := tamagotchi.SavePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// loadTamagotchi reads the saved creature into st, and reports whether there was
// one. A save from before stages existed arrives as a baby with no base
// temperament; basePersonality() fills that in on the first evolution.
func loadTamagotchi(st *companion.TamagotchiState) bool {
	data, err := os.ReadFile(tamagotchi.SavePath())
	if err != nil {
		return false
	}
	return json.Unmarshal(data, st) == nil
}

// updateTamagotchi updates the creature's state based on time and events.
func updateTamagotchi(now time.Time, st *reduce.State) {
	if tamagotchiState == nil {
		return
	}

	// Real seconds since the last frame, clamped: a resize, or a laptop that
	// slept, must not drain the creature in one step.
	dt := 0.0
	if !tamLastUpdate.IsZero() {
		dt = now.Sub(tamLastUpdate).Seconds()
	}
	tamLastUpdate = now
	if dt < 0 {
		dt = 0
	}
	if dt > 1 {
		dt = 1
	}

	// Decay the needs and age the creature, both on that clock.
	tamagotchiState.Decay(dt * TamDecayPerSecond)
	tamagotchiState.AgeProgresses(dt * TamDecayPerSecond)

	// Release a held animation once it has run. This, and not a goroutine
	// sleeping for five minutes, is what lets the mood take the face back.
	tamagotchiState.Tick(dt)

	// Speak up when something is actually wrong -- in the creature's own voice,
	// under the creature, and NEVER through st.Bubble. The bubble is the agent's
	// channel (needs_input, error, done, nothing else); a peckish pet that raised
	// it would ring the ask knock, the one rare cue this project has, and take
	// the scape's whole encoding rule down with it.
	switch {
	case tamagotchiState.SpeechSecs > 0:
		// still talking
	case tamagotchiState.Sleeping || tamagotchiState.AnimLeft > 0 ||
		tamagotchiState.Animation != companion.ClipIdle:
		tamagotchiState.Speech = ""
	case tamagotchiState.Hunger < 30 || tamagotchiState.Energy < 20 ||
		tamagotchiState.Cleanliness < 30 || tamagotchiState.Affection < 20:
		if tamagotchiState.Speech == "" || now.Sub(tamNagAt) > 12*time.Second {
			tamNagAt = now
			tamagotchiState.Speech = tamagotchiState.GetSpeech()
			tamagotchiState.SpeechSecs = 10
		}
	}

	// Sync state to reduce.State if needed
	if st.Tamagotchi == nil {
		st.Tamagotchi = tamagotchiState
	}
}

// handleTamagotchiEvent processes Tamagotchi-specific events.
func handleTamagotchiEvent(e event.Event) bool {
	if tamagotchiState == nil {
		return false
	}

	switch e.Kind {
	case event.TamagotchiFeed:
		fmt.Printf("Xscape: %s\n", tamagotchiState.Feed().Speech)
		return true
	case event.TamagotchiPlay:
		fmt.Printf("Xscape: %s\n", tamagotchiState.Play().Speech)
		return true
	case event.TamagotchiSleep:
		fmt.Printf("Xscape: %s\n", tamagotchiState.Sleep().Speech)
		return true
	case event.TamagotchiClean:
		fmt.Printf("Xscape: %s\n", tamagotchiState.Clean().Speech)
		return true
	case event.TamagotchiSick:
		tamagotchiState.Mood = companion.Sick
		tamagotchiState.Animation = companion.ClipSick
		tamagotchiState.Expression = "unwell"
		return true
	case event.TamagotchiPet:
		fmt.Printf("Xscape: %s\n", tamagotchiState.Pet().Speech)
		return true
	case event.TamagotchiTalk:
		if e.Text != "" {
			fmt.Printf("Xscape: %s\n", tamagotchiState.Talk(e.Text).Speech)
		}
		return true
	}
	return false
}

// Where the creature's box sits on the canvas. It is placed with room ABOVE it,
// because the overlays -- hearts, sparkles, the Zs of sleep -- are drawn up
// there, and a creature whose feelings are cut off by the top of the frame
// reads as a bug rather than as a feeling.
const (
	tamOverlayX = 2
	tamOverlayY = 4
)

// drawTamagotchi renders the virtual creature overlay.
func drawTamagotchi(c *canvas.Canvas, st reduce.State, t float64) {
	if tamagotchiRenderer == nil || st.Tamagotchi == nil {
		return
	}

	x, y := tamOverlayX, tamOverlayY
	l := c.Near()
	tamagotchiRenderer.Draw(l, x, y, t)

	// What it is saying, on the line under its feet: the creature's own voice in
	// the creature's own place, on its own dark ground so a sentence over the sky
	// is still a sentence.
	if s := []rune(tamagotchiState.Speech); len(s) > 0 {
		ink := term.RGB{R: 196, G: 198, B: 214}
		ground := term.RGB{R: 14, G: 16, B: 24}
		room := c.W - (x + 2)
		if room > len(s) {
			room = len(s)
		}
		for i := 0; i < room; i++ {
			l.PlotOn(x+2+i, y+8, s[i], ink, ground, 0.9)
		}
	}

	// The gauges, under the creature: a label, then ten cells of bar.
	bars := []struct {
		label rune
		value float64
		good  term.RGB
	}{
		{'H', st.Tamagotchi.Hunger, term.RGB{R: 50, G: 200, B: 50}},
		{'E', st.Tamagotchi.Energy, term.RGB{R: 50, G: 150, B: 200}},
		{'S', st.Tamagotchi.Health, term.RGB{R: 100, G: 200, B: 100}},
		{'P', st.Tamagotchi.Cleanliness, term.RGB{R: 100, G: 150, B: 200}},
		{'A', st.Tamagotchi.Affection, term.RGB{R: 255, G: 150, B: 200}},
	}
	warn := term.RGB{R: 200, G: 150, B: 50}
	bad := term.RGB{R: 200, G: 50, B: 50}
	dim := term.RGB{R: 100, G: 100, B: 100}
	for i, b := range bars {
		row := y + 10 + i
		l.Plot(x, row, '-', dim, 1)
		l.Plot(x+1, row, b.label, term.RGB{R: 150, G: 150, B: 150}, 1)
		l.Plot(x+2, row, ':', dim, 1)
		col := b.good
		if b.value < 50 {
			col = warn
		}
		if b.value < 30 {
			col = bad
		}
		for k := 0; k < 10; k++ {
			ch := ' '
			if float64(k) < b.value {
				ch = '='
			}
			l.Plot(x+3+k, row, ch, col, 1)
		}
	}

	// Mood, as one letter in the mood's own colour.
	moodRow := y + 10 + len(bars)
	moodColor := term.RGB{R: 100, G: 200, B: 100}
	switch st.Tamagotchi.Mood {
	case companion.Happy:
		moodColor = term.RGB{R: 100, G: 255, B: 100}
	case companion.Hungry:
		moodColor = term.RGB{R: 255, G: 150, B: 50}
	case companion.Tired:
		moodColor = term.RGB{R: 150, G: 150, B: 200}
	case companion.Sick, companion.Ill:
		moodColor = term.RGB{R: 200, G: 100, B: 100}
	case companion.Excited:
		moodColor = term.RGB{R: 255, G: 200, B: 50}
	case companion.Lonely:
		moodColor = term.RGB{R: 150, G: 100, B: 200}
	}
	l.Plot(x, moodRow, '-', dim, 1)
	l.Plot(x+1, moodRow, 'M', term.RGB{R: 150, G: 150, B: 150}, 1)
	l.Plot(x+2, moodRow, ':', dim, 1)
	l.Plot(x+3, moodRow, rune(st.Tamagotchi.Mood.String()[0]), moodColor, 1)

	// The key hints, along the bottom.
	instY := c.H - 3
	for i, ch := range "xscapes tamagotchi feed | play | pet | sleep" {
		col := term.RGB{R: 80, G: 80, B: 80}
		l.Plot(2+i, instY, ch, col, 1)
	}
}

// getTamagotchiStatus returns a text status report.
func getTamagotchiStatus() string {
	if tamagotchiState == nil {
		return "No Tamagotchi initialized"
	}
	return tamagotchiState.GetStatusReport()
}
