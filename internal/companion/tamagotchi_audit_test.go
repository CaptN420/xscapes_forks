package companion

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/canvas"
)

// ---------------------------------------------------------------------------
// THE AUDIT. One file that asks the creature every question a user can ask it,
// so a regression has somewhere to fail. The companion's own behaviour (growth)
// is audited in tamagotchi_evolution_test.go; this file is the rest of it.
// ---------------------------------------------------------------------------

func sane(t *testing.T, st *TamagotchiState, what string) {
	t.Helper()
	for name, v := range map[string]float64{
		"hunger": st.Hunger, "happiness": st.Happiness, "energy": st.Energy,
		"health": st.Health, "cleanliness": st.Cleanliness, "affection": st.Affection,
		"care": st.Care(),
	} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("%s: %s is %v", what, name, v)
		}
		if v < 0 || v > 100 && name != "care" {
			t.Errorf("%s: %s left its range: %.1f", what, name, v)
		}
	}
	if st.Care() < 0 || st.Care() > 1 {
		t.Errorf("%s: care out of range: %.2f", what, st.Care())
	}
}

// EVERY INTERACTION, on the happy path and on the path where it is refused.
func TestEveryInteraction(t *testing.T) {
	type act struct {
		name string
		do   func(*TamagotchiState) InteractionResult
		// ready puts the creature in the state where this interaction works.
		ready func(*TamagotchiState)
		// block puts it in the state where it is refused.
		block func(*TamagotchiState)
		check func(*TamagotchiState) error
	}
	acts := []act{
		{
			name:  "feed",
			do:    func(s *TamagotchiState) InteractionResult { return s.Feed() },
			ready: func(s *TamagotchiState) { s.Hunger = 40 },
			block: func(s *TamagotchiState) { s.Hunger = 99 },
			check: func(s *TamagotchiState) error {
				if s.Hunger <= 40 {
					return errf("feeding did not fill it: %.0f%%", s.Hunger)
				}
				if s.Animation != ClipEat {
					return errf("feeding played %q", s.Animation)
				}
				return nil
			},
		},
		{
			name:  "play",
			do:    func(s *TamagotchiState) InteractionResult { return s.Play() },
			ready: func(s *TamagotchiState) { s.Energy = 80; s.Happiness = 40 },
			block: func(s *TamagotchiState) { s.Energy = 10 },
			check: func(s *TamagotchiState) error {
				if s.Happiness <= 40 {
					return errf("playing did not cheer it: %.0f%%", s.Happiness)
				}
				if s.Animation != ClipPlay {
					return errf("playing played %q", s.Animation)
				}
				return nil
			},
		},
		{
			name:  "pet",
			do:    func(s *TamagotchiState) InteractionResult { return s.Pet() },
			ready: func(s *TamagotchiState) { s.Affection = 30; s.Sleeping = false },
			block: func(s *TamagotchiState) { s.Sleeping = true },
			check: func(s *TamagotchiState) error {
				if s.Affection <= 30 {
					return errf("petting did not warm it: %.0f%%", s.Affection)
				}
				if s.Animation != ClipPet {
					return errf("petting played %q", s.Animation)
				}
				return nil
			},
		},
		{
			name:  "clean",
			do:    func(s *TamagotchiState) InteractionResult { return s.Clean() },
			ready: func(s *TamagotchiState) { s.Cleanliness = 40 },
			block: func(s *TamagotchiState) { s.Cleanliness = 95 },
			check: func(s *TamagotchiState) error {
				if s.Cleanliness <= 40 {
					return errf("cleaning did not clean: %.0f%%", s.Cleanliness)
				}
				return nil
			},
		},
		{
			name:  "sleep",
			do:    func(s *TamagotchiState) InteractionResult { return s.Sleep() },
			ready: func(s *TamagotchiState) { s.Energy = 40 },
			block: func(s *TamagotchiState) { s.Energy = 95 },
			check: func(s *TamagotchiState) error {
				if !s.Sleeping {
					return errf("sleeping did not put it to sleep")
				}
				return nil
			},
		},
	}

	for _, a := range acts {
		// the happy path
		v := NewTamagotchiState()
		st := &v
		a.ready(st)
		before := st.stats()
		beforeCared := st.CaredFor
		res := a.do(st)
		if !res.Success {
			t.Errorf("%s was refused when it should have worked: %s", a.name, res.Message)
		}
		if res.Message == "" {
			t.Errorf("%s said nothing", a.name)
		}
		if err := a.check(st); err != nil {
			t.Errorf("%s: %v", a.name, err)
		}
		if st.CaredFor != beforeCared+1 {
			t.Errorf("%s did not credit one moment of care (%d -> %d)",
				a.name, beforeCared, st.CaredFor)
		}
		if a.name == "sleep" {
			if !st.Sleeping || st.Animation != ClipSleep {
				t.Errorf("sleep left the creature awake (anim=%q)", st.Animation)
			}
		} else if st.AnimLeft <= 0 {
			t.Errorf("%s played no animation", a.name)
		}
		sane(t, st, a.name)
		_ = before

		// the refused path: the creature is left exactly as it was
		v2 := NewTamagotchiState()
		st2 := &v2
		a.block(st2)
		was := st2.stats()
		cared := st2.CaredFor
		res2 := a.do(st2)
		if res2.Success {
			t.Errorf("%s was allowed when it should have been refused", a.name)
		}
		if st2.stats() != was {
			t.Errorf("%s changed the stats even though it was refused: %v -> %v",
				a.name, was, st2.stats())
		}
		if st2.CaredFor != cared {
			t.Errorf("%s credited care even though it was refused", a.name)
		}
	}

	// wake, which is not a care action
	v := NewTamagotchiState()
	st := &v
	st.Sleeping = true
	st.Animation = ClipSleep
	if res := st.WakeUp(); res.Message == "" {
		t.Error("waking said nothing")
	}
	if st.Sleeping {
		t.Error("waking did not wake it")
	}
	// talk, with a message and without one
	if res := st.Talk("hello there"); res.Speech == "" {
		t.Error("talking got no answer")
	}
	if res := st.Talk(""); res.Speech == "" {
		t.Error("talking to it with no words got no answer")
	}
}

func errf(f string, a ...interface{}) error { return fmt.Errorf(f, a...) }

// stats is the six needs as one comparable value.
func (t *TamagotchiState) stats() [6]float64 {
	return [6]float64{t.Hunger, t.Happiness, t.Energy, t.Health, t.Cleanliness, t.Affection}
}

// THE STATISTICS: decay runs one way, never out of bounds, and never in a step
// bigger than asked for.
func TestTheStatisticsHoldTheirBounds(t *testing.T) {
	v := NewTamagotchiState()
	st := &v

	// An hour of decay at the shipped rate.
	for i := 0; i < 3600; i++ {
		st.Decay(TamAuditDecayPerSecond)
		sane(t, st, "decay")
	}

	// Starving for a day must not wrap, go negative or go NaN.
	for i := 0; i < 24*60; i++ {
		st.Decay(1.0 / 60)
	}
	if st.Hunger < 0 || st.Energy < 0 || st.Health < 0 {
		t.Errorf("a day of neglect went negative: h=%.1f e=%.1f hp=%.1f",
			st.Hunger, st.Energy, st.Health)
	}
	sane(t, st, "a day of neglect")

	// Overfeeding past the ceiling stops at it.
	for i := 0; i < 20; i++ {
		st.Hunger = 94
		st.Feed()
	}
	if st.Hunger > 100 {
		t.Errorf("hunger went past 100: %.1f", st.Hunger)
	}

	// The mood follows the numbers rather than drifting on its own.
	st.Hunger, st.Happiness, st.Energy = 5, 5, 5
	st.Health, st.Cleanliness, st.Affection = 5, 5, 5
	st.UpdateMood()
	if st.Mood == Happy || st.Mood == Excited {
		t.Errorf("a starving creature reports %s", st.Mood)
	}
}

// auditLayer is a canvas large enough for any stage, so a clip can be drawn
// through the real renderer rather than guessed at.
func auditLayer() *canvas.Layer {
	c := canvas.New(80, 24, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	return c.Near()
}

// TamAuditDecayPerSecond mirrors the shipped metabolism (package main's
// TamDecayPerSecond) so the audit runs on the rate users actually get.
const TamAuditDecayPerSecond = 1.0 / 30.0 / 60.0 * 24.0 * 20.0

// THE ANIMATIONS: every clip, at every stage, draws something and nothing
// panics; a held animation releases; sleep survives a mood pass.
func TestEveryClipAtEveryStage(t *testing.T) {
	clips := []string{
		ClipIdle, ClipJoy, ClipSad, ClipSleep, ClipEat,
		ClipPlay, ClipPet, ClipSick, ClipBlink, ClipLook,
		ClipTwitch, ClipSigh, ClipHop, ClipStretch,
	}
	for s := StageBaby; s < StageCount; s++ {
		for _, clip := range clips {
			v := NewTamagotchiState()
			v.Stage = s
			st := &v
			st.Animation = clip
			st.AnimLeft = 0
			st.Sleeping = clip == ClipSleep
			r := NewTamagotchiRenderer(st)
			ink := 0
			for i := 0; i < 24; i++ {
				for _, row := range strings.Split(r.GetASCIIArt(), "\n") {
					ink += strings.Count(row, "█") + strings.Count(row, "▀") +
						strings.Count(row, "▄") + strings.Count(row, "▌") + strings.Count(row, "▐")
				}
				r.Draw(auditLayer(), 0, 0, float64(i)/12)
			}
			if ink == 0 {
				t.Errorf("%s at %s draws no ink", clip, s)
			}
		}
	}
}

// A held animation has to let go, and sleep has to outrank the mood pass.
func TestHeldAnimationsReleaseAndSleepHolds(t *testing.T) {
	v := NewTamagotchiState()
	st := &v
	st.Hunger, st.Happiness, st.Energy, st.Health, st.Cleanliness, st.Affection = 60, 60, 60, 60, 60, 60
	st.Feed()
	if st.AnimLeft <= 0 || st.Animation != ClipEat {
		t.Fatalf("feeding did not hold the eat clip (anim=%q left=%.2f)", st.Animation, st.AnimLeft)
	}
	for i := 0; i < 300; i++ { // 5 seconds at 60 fps
		st.Tick(1.0 / 60)
	}
	if st.Animation == ClipEat {
		t.Errorf("the eat clip never released (left=%.2f)", st.AnimLeft)
	}
	if st.Animation != tamClipForState(st) {
		t.Errorf("after the hold the animation is %q but the state wants %q",
			st.Animation, tamClipForState(st))
	}

	// Asleep stays asleep through every mood pass, and the renderer draws sleep.
	st.Sleep()
	for i := 0; i < 30; i++ {
		st.UpdateMood()
	}
	if !st.Sleeping {
		t.Error("a mood pass woke the creature up")
	}
	if got := tamClipForState(st); got != ClipSleep {
		t.Errorf("a sleeping creature renders %q", got)
	}
}

// THE PASSAGE OF TIME: the creature tolerates a pause, a huge step, and a
// clock that goes backwards, because a laptop lid closes.
func TestTimeCanBehaveBadly(t *testing.T) {
	v := NewTamagotchiState()
	st := &v
	sane(t, st, "new")

	for _, dt := range []float64{0, -1, -1e9, 1e-9, 0.016, 1.0, 60.0, 1e6} {
		st.Tick(dt)
		st.Grow(dt)
		st.Decay(dt * TamAuditDecayPerSecond)
		st.AgeProgresses(dt * TamAuditDecayPerSecond)
		sane(t, st, "odd dt")
	}
	// A single enormous step must not skip a stage: one life at a time.
	if st.Stage > StageChild {
		t.Errorf("one huge tick carried it to %s: stages must be walked", st.Stage)
	}

	// Age only ever moves forward for sane input.
	was := st.Age
	st.AgeProgresses(1)
	if st.Age <= was {
		t.Error("age did not advance")
	}
}

// SAVE AND LOAD: a round trip keeps the life; a broken file does not eat the
// creature; a file from before stages existed still loads.
func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tamagotchi.json")
	write := func(v interface{}) {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(st *TamagotchiState) bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		return json.Unmarshal(data, st) == nil
	}

	// A life in progress.
	v := NewTamagotchiState()
	v.Stage = StageTeen
	v.CareSecs = 900
	v.StageSecs = 2400
	v.CaredFor = 12
	v.Setbacks = 1
	v.Hunger = 33.5
	v.Speech = "not persisted"
	v.PlayAnim(ClipPet, 3)
	write(&v)

	// A new creature over the top, the way initTamagotchi does it.
	w := NewTamagotchiState()
	if !read(&w) {
		t.Fatal("a save this program wrote could not be read back")
	}
	if w.Stage != StageTeen || w.CareSecs != 900 || w.StageSecs != 2400 ||
		w.CaredFor != 12 || w.Setbacks != 1 || w.Hunger != 33.5 {
		t.Errorf("the round trip lost the life: stage=%s care=%.0f secs=%.0f cared=%d setbacks=%d hunger=%.1f",
			w.Stage, w.CareSecs, w.StageSecs, w.CaredFor, w.Setbacks, w.Hunger)
	}
	if w.Speech != "" {
		t.Errorf("transient speech was persisted: %q", w.Speech)
	}
	// The base temperament must survive, or every relaunch resets its character.
	if w.Base != v.Base {
		t.Error("the birth temperament was not kept across the save")
	}

	// A corrupt file: no load, and no panic.
	if err := os.WriteFile(path, []byte("{not json at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := NewTamagotchiState()
	before := fresh.stats()
	if read(&fresh) {
		t.Error("a corrupt file reported a successful load")
	}
	if fresh.stats() != before {
		t.Error("a corrupt load modified the creature")
	}

	// A file from before stages existed: it must arrive as a baby with a base
	// temperament it can grow from, not with an impossible stage.
	old := map[string]interface{}{
		"hunger": 50.0, "happiness": 50.0, "energy": 50.0,
		"health": 50.0, "cleanliness": 50.0, "affection": 50.0,
		"evolution_phase": 2, "is_evolving": true,
		"personality": map[string]float64{"curiosity": 0.9, "whimsy": 0.8},
	}
	write(old)
	ancient := NewTamagotchiState()
	if !read(&ancient) {
		t.Fatal("an old save could not be read")
	}
	if ancient.Stage != StageBaby {
		t.Errorf("an old save loaded as a %s (phase %d)", ancient.Stage, ancient.EvolutionPhase)
	}
	if b := ancient.basePersonality(); b == (Personality{}) {
		t.Error("an old save has no birth temperament to grow from")
	}
}

// A creature with no save, a creature with a save, and a creature whose state
// is zero: all three must be usable.
func TestDegenerateStates(t *testing.T) {
	var zero TamagotchiState
	zero.Tick(1.0 / 60)
	zero.Grow(1.0 / 60)
	zero.UpdateMood()
	if s := zero.GetStatusReport(); s == "" {
		t.Error("a zero creature has no status")
	}
	r := NewTamagotchiRenderer(&zero)
	if art := r.GetASCIIArt(); art == "" {
		t.Error("a zero creature draws nothing")
	}
	// A nil state must not panic the renderer or the status.
	nilr := NewTamagotchiRenderer(nil)
	_ = nilr.GetASCIIArt()
	_ = nilr.GetStatusReport()
}

// THE STATUS LINE is what the user reads: it must name the stage and what is
// missing, in the player's words.
func TestTheStatusLineSaysWhereItIs(t *testing.T) {
	v := NewTamagotchiState()
	st := &v
	s := st.GetStatusReport()
	if !strings.Contains(s, "baby") {
		t.Errorf("the status does not name the stage: %q", s)
	}
	if !strings.Contains(s, "%") || !strings.Contains(s, "child") {
		t.Errorf("the status does not say how far the next stage is: %q", s)
	}
	st.Stage = StageAdult
	if s := st.GetStatusReport(); !strings.Contains(s, "fully grown") {
		t.Errorf("an adult's status reads %q", s)
	}
	// And every stage names itself, in the save and on screen.
	for st := StageBaby; st < StageCount; st++ {
		if st.String() == "" || st.String() == "unknown" {
			t.Errorf("stage %d has no name", int(st))
		}
	}
}

// The renderer's own report is the verbose one; it must stay usable.
func TestTheRendererReportStillWorks(t *testing.T) {
	v := NewTamagotchiState()
	v.Stage = StageChild
	st := &v
	r := NewTamagotchiRenderer(st)
	rep := r.GetStatusReport()
	for _, want := range []string{"Mood", "Hunger", "Age", "Interactions"} {
		if !strings.Contains(rep, want) {
			t.Errorf("the verbose report lost %q:\n%s", want, rep)
		}
	}
	if !strings.Contains(rep, "Evolution Phase") && st.EvolutionPhase != 0 {
		t.Error("a creature mid-life is in no phase")
	}
}

// The clock the audit uses must match the one the app uses, or these numbers
// are fiction.
func TestTheAuditClockMatchesTheApp(t *testing.T) {
	// Age runs at sixteen times real time in this project: 1/30/60*24*20.
	if math.Abs(TamAuditDecayPerSecond-0.2666666666) > 1e-6 {
		t.Errorf("the audit's metabolism is %.4f/s", TamAuditDecayPerSecond)
	}
	if d := time.Duration(TamAuditDecayPerSecond * float64(time.Hour)); d < 15*time.Minute {
		t.Errorf("one real second ages the creature %v", d)
	}
}
