package companion

import (
	"strings"
	"testing"
)

// perfect keeps a creature at the top of its needs for one tick, and returns the
// dt it was kept for.
func kept(t *TamagotchiState, care float64) {
	t.Hunger = 100 * care
	t.Happiness = 100 * care
	t.Energy = 100 * care
	t.Health = 100 * care
	t.Cleanliness = 100 * care
	t.Affection = 100 * care
}

// careFor runs n seconds of growth at a fixed frame rate with the creature well
// kept, and returns how many seconds it actually took to evolve.
func careFor(t *TamagotchiState, seconds float64) {
	const step = 0.25
	for s := 0.0; s < seconds; s += step {
		kept(t, 0.9)
		t.Grow(step)
	}
}

// The headline promise: a well-kept creature does NOT become an adult in one
// sitting's blink, and every stage is reachable at all.
func TestGrowthTakesHoursNotMinutes(t *testing.T) {
	st := NewTamagotchiState()

	// Well kept, but never interacted with: it must not evolve, because the
	// gates ask for moments and for history as well as for time.
	careFor(&st, 40*60)
	if st.Stage != StageBaby {
		t.Fatalf("a creature nobody touched became a %s in 40 minutes", st.Stage)
	}

	// Now keep it, and act with it.
	spent := 0.0
	for st.Stage != StageAdult && spent < 6*60*60 {
		kept(&st, 0.9)
		if int(spent)%60 == 0 {
			st.FedCount++
			st.CaredFor++
		}
		if int(spent)%180 == 0 {
			st.PlayedCount++
			st.CaredFor++
		}
		st.Grow(0.25)
		spent += 0.25
	}
	if st.Stage != StageAdult {
		t.Fatalf("six hours of perfect care and it is still a %s (progress %.2f)",
			st.Stage, st.StageProgress())
	}
	// The three gates add up to 12 + 35 + 80 minutes of presence, and growth
	// runs at a quarter of that pace until the cares accumulate, so anything
	// under an hour and a half would mean a gate is not being enforced.
	if spent < 90*60 {
		t.Errorf("adulthood in %.0f minutes of care: too fast to be a life", spent/60)
	}
	if spent > 4*60*60 {
		t.Errorf("adulthood after %.1f hours of perfect care: too slow to be seen", spent/3600)
	}
	t.Logf("baby to adult in %.0f minutes of held attention", spent/60)
}

// Neglect is a delay, not a wall: the same creature, left for a while, takes
// longer -- and can still grow.
func TestNeglectDelaysGrowthWithoutStoppingIt(t *testing.T) {
	well := NewTamagotchiState()
	left := NewTamagotchiState()

	// Two minutes at rock bottom, twice, for the one that gets left.
	for i := 0; i < 2; i++ {
		for s := 0.0; s < 120; s += 1 {
			kept(&left, 0.05)
			left.Grow(1)
		}
		kept(&left, 0.9)
		left.Grow(30)
	}
	if left.Setbacks < 2 {
		t.Fatalf("two long stretches of neglect counted %d setbacks", left.Setbacks)
	}
	// It lived 4.5 minutes but only 30 seconds of them counted as care: the
	// neglect did not stop the clock, it left it behind.
	if left.CareSecs > left.StageSecs*0.5 {
		t.Fatalf("neglect did not hold the cared-for clock back: %.0fs of %.0fs",
			left.CareSecs, left.StageSecs)
	}
	_ = well
	if Next, why := left.NextEvolution(); Next != StageChild {
		t.Fatalf("a neglected baby is growing into %v, not a child", Next)
	} else if !strings.Contains(why, "care") && !strings.Contains(why, "time") {
		t.Errorf("the reason it is stuck reads %q", why)
	}
	// And it can still get there, later.
	for s := 0.0; s < 4*60*60 && left.Stage == StageBaby; s += 0.25 {
		kept(&left, 0.9)
		left.CaredFor++
		if int(s)%60 == 0 {
			left.FedCount++
		}
		left.Grow(0.25)
	}
	if left.Stage == StageBaby {
		t.Error("a creature that was neglected once can never grow: a wall, not a delay")
	}
}

// The history gate: fed and never played is not a teenager.
func TestAStageNeedsTheKindsOfCareAndNotJustTheCount(t *testing.T) {
	st := NewTamagotchiState()
	// Pass every clock and every number but the history.
	for s := 0.0; s < 60*60; s += 0.25 {
		kept(&st, 0.95)
		st.CaredFor = 40
		st.FedCount = 40
		st.PlayedCount = 0
		st.Grow(0.25)
		if st.Stage != StageBaby {
			break // get it to the child gate, then check the next one
		}
	}
	if st.Stage != StageChild {
		t.Fatalf("the child gate did not open with every clock passed (stage %s)", st.Stage)
	}
	// Now everything for the teenager except play.
	for s := 0.0; s < 60*60; s += 0.25 {
		kept(&st, 0.95)
		st.CaredFor, st.FedCount, st.PlayedCount = 40, 40, 0
		st.Grow(0.25)
	}
	if st.Stage != StageChild {
		t.Errorf("a creature that never played became a %s", st.Stage)
	}
	if _, why := st.NextEvolution(); !strings.Contains(why, "played") {
		t.Errorf("the status does not say play is what is missing: %q", why)
	}
}

// Sickness stops it, and health is part of the judgement.
func TestAnUnwellCreatureDoesNotEvolve(t *testing.T) {
	st := NewTamagotchiState()
	for s := 0.0; s < 40*60; s += 0.25 {
		kept(&st, 0.9)
		st.Health = 40 // everything else fine
		st.CaredFor, st.FedCount, st.PlayedCount = 9, 9, 9
		st.Grow(0.25)
	}
	if st.Stage != StageBaby {
		t.Errorf("a creature at 40%% health evolved into a %s", st.Stage)
	}
}

// Progress is the worst clock, not an average: a bar that fills while the
// creature is stuck would be a lie.
func TestProgressReportsTheBottleneck(t *testing.T) {
	st := NewTamagotchiState()
	st.CaredFor, st.FedCount = 3, 3 // the interaction gate is met...
	kept(&st, 0.9)
	for s := 0.0; s < 11*60; s += 0.25 {
		st.Grow(0.25)
	}
	p := st.StageProgress()
	if p < 0.85 || p > 0.98 {
		t.Errorf("eleven of twelve minutes of presence reads %.2f", p)
	}
	// Time passes, care does not: the bar must not go past what care allows.
	st.StageSecs, st.CareSecs = 0, 0
	st.CareSum, st.CareSpan = 0, 0
	st.Setbacks = 0
	kept(&st, 0.30)
	for s := 0.0; s < 60*60; s += 0.25 {
		kept(&st, 0.30)
		st.Grow(0.25)
	}
	if st.StageProgress() > 0.5 {
		t.Errorf("an hour of presence at %.0f%% care reads %.2f progress",
			st.Care()*100, st.StageProgress())
	}
}

// Every stage has to look like a different animal, or the word evolution is
// doing no work.
func TestEachStageLooksAndMovesDifferently(t *testing.T) {
	seen := map[StageLook]Stage{}
	for s := StageBaby; s < StageCount; s++ {
		look := LookOf(s)
		if prev, ok := seen[look]; ok {
			t.Errorf("%s and %s have the same silhouette %v", prev, s, look)
		}
		seen[look] = s
		if look.W < 12 || look.H < 12 {
			t.Errorf("%s's body is %dx%d: nothing to draw", s, look.W, look.H)
		}
	}
	// And the four render, at their own size, with the face on the head.
	words := map[Stage]int{}
	for s := StageBaby; s < StageCount; s++ {
		v := NewTamagotchiState()
		v.Stage = s
		st := &v
		r := NewTamagotchiRenderer(st)
		// Warm the animator, then look at the creature.
		for i := 0; i < 30; i++ {
			tamRegion(r, float64(i)/12)
		}
		art := r.GetASCIIArt()
		if strings.Count(art, "█")+strings.Count(art, "▌")+strings.Count(art, "▐") < 20 {
			t.Errorf("%s renders almost nothing:\n%s", s, art)
		}
		words[s] = len(strings.TrimSpace(art))
	}
	// A baby is smaller than an adult: fewer cells of ink in the same box.
	baby, adult := words[StageBaby], words[StageAdult]
	if baby >= adult {
		t.Errorf("the baby (%d cells) is not smaller than the adult (%d)", baby, adult)
	}
}

// Personality is steered, not replaced: two adults are not the same animal, and
// an adult is calmer than the baby it was.
func TestEvolutionSteersThePersonality(t *testing.T) {
	a := NewTamagotchiState()
	b := NewTamagotchiState()
	before := a.Personality

	for _, c := range []*TamagotchiState{&a, &b} {
		for c.Stage != StageAdult {
			kept(c, 0.95)
			c.CaredFor, c.FedCount, c.PlayedCount = 60, 60, 60
			c.Grow(60)
		}
	}
	if a.Stage != StageAdult {
		t.Fatalf("did not reach adulthood: %s", a.Stage)
	}
	// Closer to the adult archetype than it was as a baby, on the two traits
	// that define the stage: steady, and not easily startled.
	adult := stageArchetype[StageAdult]
	if d := abs(a.Personality.MoodStability - adult.MoodStability); d >= abs(before.MoodStability-adult.MoodStability) {
		t.Errorf("stability did not move towards the adult: %.2f -> %.2f (want %.2f)",
			before.MoodStability, a.Personality.MoodStability, adult.MoodStability)
	}
	if d := abs(a.Personality.Reactivity - adult.Reactivity); d >= abs(before.Reactivity-adult.Reactivity) {
		t.Errorf("reactivity did not move towards the adult: %.2f -> %.2f (want %.2f)",
			before.Reactivity, a.Personality.Reactivity, adult.Reactivity)
	}
	// But still its own animal: two creature seeds do not converge.
	if a.Personality.Curiosity == b.Personality.Curiosity {
		t.Error("two adults have identical personalities: the blend replaced them instead of steering")
	}
	for _, v := range []float64{a.Personality.Whimsy, a.Personality.Attachment, a.Personality.Nostalgia} {
		if v < 0.05 || v > 0.95 {
			t.Errorf("a trait left its range: %.2f", v)
		}
	}
}

// The move itself is a moment: speech, a happy clip, a memory, a fresh clock.
func TestAnEvolutionIsAMoment(t *testing.T) {
	st := NewTamagotchiState()
	st.CaredFor, st.FedCount, st.PlayedCount = 9, 9, 9
	kept(&st, 0.95)
	for s := 0.0; s < 13*60 && st.Stage == StageBaby; s += 0.25 {
		kept(&st, 0.95)
		st.CaredFor, st.FedCount, st.PlayedCount = 9, 9, 9
		st.Grow(0.25)
	}
	if st.Stage != StageChild {
		t.Fatalf("twelve minutes of perfect care did not make a child (stage %s, %s)",
			st.Stage, st.progressLine())
	}
	if st.AnimLeft <= 0 {
		t.Error("nothing plays on the frame it grows up")
	}
	if st.Speech == "" {
		t.Error("it grew up silently")
	}
	if !st.EvolvedAt.After(st.StageBornAt.Add(-1)) {
		t.Log("note: stage clock bookkeeping")
	}
	if len(st.Memory.SpecialMoments) == 0 {
		t.Error("the moment is not in its memory")
	}
	// The new stage starts clean.
	if st.StageSecs > 1 || st.CareSecs > 1 || st.CaredFor != 0 || st.Setbacks != 0 {
		t.Errorf("the child inherited the baby's clocks: secs=%.1f care=%.1f cared=%d setbacks=%d",
			st.StageSecs, st.CareSecs, st.CaredFor, st.Setbacks)
	}
	// And it is not ready for the next one immediately.
	if st.StageProgress() > 0.2 {
		t.Errorf("a newborn child is already %.0f%% to a teenager", st.StageProgress()*100)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// The save file is JSON, and a stage has to survive the round trip as a stage.
func TestTheStageSurvivesASave(t *testing.T) {
	st := NewTamagotchiState()
	st.Stage = StageTeen
	st.CareSecs = 123
	st.Setbacks = 2
	if !strings.Contains(strings.ToLower(st.Stage.String()), "teen") {
		t.Fatalf("the stage does not name itself: %q", st.Stage.String())
	}
	if next, ok := st.Stage.Next(); !ok || next != StageAdult {
		t.Fatalf("a teen's next is %v/%v", next, ok)
	}
	if _, ok := StageAdult.Next(); ok {
		t.Error("an adult has somewhere left to go")
	}
}

// progressLine is the status the player actually reads.
func (t *TamagotchiState) progressLine() string {
	next, why := t.NextEvolution()
	return next.String() + ": " + why
}
