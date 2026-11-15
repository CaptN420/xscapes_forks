// Package companion -- the creature's growth.
//
// A Tamagotchi that never changes is a screen saver. This file is what makes it
// a creature with a life: four stages, each one earned by the way it was kept
// rather than by the passage of the clock.
//
// WHAT DECIDES A STAGE. Five things, and every one of them has to be there,
// because a pet that grows on time alone grows while it is ignored, and a pet
// that grows on food alone grows by button-mashing:
//
//   - time spent with the player  StageSecs, the seconds this stage has been
//     lived in the presence of a running scape
//   - the quality of the care     CareRatio, the mean of the six needs over
//     that time, sampled every frame
//   - the interactions done       CaredFor, one per feed/play/pet/clean/sleep
//   - its statistics              Health and Happiness at the moment it turns
//   - its history                 Setbacks, and which KIND of care it has had:
//     a creature that has never played does not
//     become a teenager, however well fed
//
// NOT TOO FAST. The three thresholds are 12, 35 and 80 minutes of presence, of
// which 5, 15 and 35 must be spent actually cared for (all six needs above
// CareFloor). That is two hours and change to adulthood for a creature that is
// looked after, and much longer for one that is not, because the cared-for time
// is what the gate is made of. The old code evolved at 24 and 72 hours of AGE,
// and Age runs at sixteen times real time in this project (TamDecayPerSecond) --
// so the first evolution fired after a minute and a half, mid-session, and the
// second before anyone had looked twice.
package companion

import (
	"fmt"
	"math"
	"time"
)

// Stage is how grown the creature is. The order is the order of its life, so
// comparisons work and a stage can never be skipped.
type Stage int

const (
	StageBaby Stage = iota
	StageChild
	StageTeen
	StageAdult
	StageCount
)

func (s Stage) String() string {
	switch s {
	case StageBaby:
		return "baby"
	case StageChild:
		return "child"
	case StageTeen:
		return "teen"
	case StageAdult:
		return "adult"
	}
	return "unknown"
}

// Next is the stage after this one, and false at adulthood.
func (s Stage) Next() (Stage, bool) {
	if s < StageBaby || s >= StageAdult {
		return s, false
	}
	return s + 1, true
}

// CareFloor is the mean of the six needs that counts as "cared for" for the
// purpose of the clock. Below it the creature is surviving, not being kept: the
// stage clock stops and the cared-for clock never starts.
const CareFloor = 0.45

// NeglectFloor is where a stretch of neglect begins to cost. A creature below
// this for a minute is not having a bad afternoon, it is being left.
const NeglectFloor = 0.22

// stageReq is what a stage demands before the next one is allowed. Age is
// deliberately absent: the creature's lifespan is not the requirement, the
// player's attention is.
type stageReq struct {
	Secs     float64 // seconds lived in this stage, presence
	CareSecs float64 // of which this many with every need up
	Cared    int     // interactions credited
	Ratio    float64 // mean care over the stage
	Health   float64 // at the moment it turns
	Happy    float64
	Fed      int // history: the kinds of care, not just the count
	Played   int
}

// The requirements, in one place, because the whole system is a judgement call
// and a judgement call that is spread over four functions cannot be argued
// with.
var stageReqs = [StageCount]stageReq{
	StageBaby: { // ~12 min of presence, 5 of them well kept
		Secs: 12 * 60, CareSecs: 5 * 60, Cared: 3, Ratio: 0.50,
		Health: 55, Happy: 45,
	},
	StageChild: { // ~35 min, 15 well kept, and it must have played
		Secs: 35 * 60, CareSecs: 15 * 60, Cared: 10, Ratio: 0.55,
		Health: 60, Happy: 50,
		Fed: 2, Played: 2,
	},
	StageTeen: { // ~80 min, 35 well kept, and it must have been petted
		Secs: 80 * 60, CareSecs: 35 * 60, Cared: 25, Ratio: 0.60,
		Health: 65, Happy: 55,
		Fed: 3, Played: 3,
	},
}

// StageReq returns the requirement for a stage, for the status line and for
// tests.
func StageReq(s Stage) stageReq {
	if s < StageBaby || s >= StageAdult {
		return stageReq{}
	}
	return stageReqs[s]
}

// setbackPenalty is what one stretch of real neglect adds to the cared-for
// clock. It is a delay, not a block: a creature that was left can still grow,
// it just has to be kept longer than one that was not.
const setbackPenalty = 90.0 // seconds of extra care per setback

// Care is the mean of the six needs, 0..1. The single number the growth gates
// are made of.
func (t *TamagotchiState) Care() float64 {
	sum := t.Hunger + t.Happiness + t.Energy + t.Health + t.Cleanliness + t.Affection
	return math.Max(0, math.Min(1, sum/600))
}

// Grow advances the growth of the creature by dt real seconds. Called once per
// frame from the tick, with the same dt the needs decay by, so a creature left
// in a window that is not being drawn does not grow.
//
// It is safe to call at any stage: an adult just stops.
func (t *TamagotchiState) Grow(dt float64) {
	if dt <= 0 {
		return
	}
	if t.Stage < StageBaby || t.Stage >= StageAdult {
		return // an adult has nowhere left to go
	}
	if t.StageBornAt.IsZero() {
		t.StageBornAt = time.Now()
	}

	care := t.Care()
	t.StageSecs += dt
	t.CareSum += care * dt
	t.CareSpan += dt
	if care >= CareFloor {
		t.CareSecs += dt
	}
	if care < NeglectFloor {
		t.NeglectSecs += dt
		if t.NeglectSecs > 60 && !t.neglectOpen {
			t.neglectOpen = true
			t.Setbacks++
		}
	} else if care > CareFloor && t.NeglectSecs > 0 {
		t.NeglectSecs, t.neglectOpen = 0, false
	}

	if t.readyToEvolve() {
		t.evolve()
	}
}

// readyToEvolve is the whole judgement, in one place, in the order a person
// would ask the questions.
func (t *TamagotchiState) readyToEvolve() bool {
	req := stageReqs[t.Stage]
	// The cared-for clock is what neglect lengthens.
	needCare := req.CareSecs + float64(t.Setbacks)*setbackPenalty
	if t.StageSecs < req.Secs || t.CareSecs < needCare {
		return false
	}
	if t.CaredFor < req.Cared {
		return false
	}
	if t.CareRatio() < req.Ratio {
		return false
	}
	if t.Health < req.Health || t.Happiness < req.Happy {
		return false
	}
	// History, not a count: it must have had each KIND of care. A creature fed
	// and never played is not a teenager, it is a feeding machine.
	if t.FedCount < req.Fed || t.PlayedCount < req.Played {
		return false
	}
	return true
}

// CareRatio is the mean care over the stage so far.
func (t *TamagotchiState) CareRatio() float64 {
	if t.CareSpan <= 0 {
		return 0
	}
	return t.CareSum / t.CareSpan
}

// StageProgress is how far into the current stage the creature is, 0..1, as the
// WORST of the four clocks. It is what a progress bar should show: the thing
// that is holding it back, not an average that hides it.
func (t *TamagotchiState) StageProgress() float64 {
	if t.Stage >= StageAdult {
		return 1
	}
	req := stageReqs[t.Stage]
	part := func(v, want float64) float64 {
		if want <= 0 {
			return 1
		}
		return math.Max(0, math.Min(1, v/want))
	}
	needCare := req.CareSecs + float64(t.Setbacks)*setbackPenalty
	return math.Min(
		part(t.StageSecs, req.Secs),
		math.Min(
			part(t.CareSecs, needCare),
			math.Min(part(float64(t.CaredFor), float64(req.Cared)),
				math.Min(part(t.CareRatio(), req.Ratio),
					math.Min(part(t.Health, req.Health), part(t.Happiness, req.Happy)))),
		),
	)
}

// NextEvolution reports what it is growing into and what is still missing, in
// the player's words. Empty when there is nothing left to become.
func (t *TamagotchiState) NextEvolution() (Stage, string) {
	next, ok := t.Stage.Next()
	if !ok {
		return t.Stage, "fully grown"
	}
	req := stageReqs[t.Stage]
	switch {
	case t.StageSecs < req.Secs:
		return next, fmt.Sprintf("needs %s more time together",
			shortDur(time.Duration((req.Secs-t.StageSecs)*float64(time.Second))))
	case t.CareSecs < req.CareSecs+float64(t.Setbacks)*setbackPenalty:
		return next, "needs to be cared for"
	case t.CaredFor < req.Cared:
		return next, fmt.Sprintf("needs %d more moments with you", req.Cared-t.CaredFor)
	case t.CareRatio() < req.Ratio:
		return next, "needs steadier care"
	case t.Health < req.Health:
		return next, "is not well enough"
	case t.Happiness < req.Happy:
		return next, "is not happy enough"
	case t.FedCount < req.Fed:
		return next, "has not been fed enough"
	case t.PlayedCount < req.Played:
		return next, "has not played enough"
	}
	return next, "ready"
}

// shortDur says a duration the way a person does: minutes up to an hour, then
// hours and minutes.
func shortDur(d time.Duration) string {
	if d < time.Minute {
		return "a moment"
	}
	m := int(d.Minutes())
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	h := m / 60
	if r := m % 60; r > 0 {
		return fmt.Sprintf("%dh %dmin", h, r)
	}
	return fmt.Sprintf("%dh", h)
}

// evolve turns the creature into the next stage: a new body, a different
// temperament, a moment it will remember, and something to say about it.
func (t *TamagotchiState) evolve() {
	next, ok := t.Stage.Next()
	if !ok {
		return
	}
	old := t.Stage
	t.Stage = next
	t.EvolutionPhase = int(next) // the field the renderer and the save already had
	t.IsEvolving = true
	t.EvolvedAt = time.Now()
	t.StageBornAt = time.Now()

	// The new stage starts with a clean clock, at whatever health it arrived
	// with. Carrying the old counters over would make the next stage begin
	// nearly complete, which is how a four-stage life becomes a two-stage one.
	t.StageSecs, t.CareSecs, t.CareSum, t.CareSpan = 0, 0, 0, 0
	t.CaredFor, t.Setbacks, t.NeglectSecs, t.neglectOpen = 0, 0, 0, false

	t.applyStagePersonality(old)

	// Growing up feels good, and it takes something out of you.
	t.Happiness = clamp100(t.Happiness + 12)
	t.Affection = clamp100(t.Affection + 8)
	t.Energy = clamp100(t.Energy - 10)

	t.AddMemory(fmt.Sprintf("grew from a %s into a %s", old, t.Stage), false)
	t.Speech = evolveLine(t.Stage)
	t.SpeechSecs = 7
	t.PlayAnim(ClipJoy, 4.5) // the visible moment: a hop, sparkles, a happy face
	t.UpdateMood()
}

// Σ stageArchetype is the temperament each stage leans towards. Growth is a
// partial blend, not an assignment: the creature keeps the personality it was
// born with and is STEERED by what it became, so two adults of the same species
// are not the same animal.
var stageArchetype = [StageCount]Personality{
	StageBaby: {Reactivity: 0.85, MoodStability: 0.30, Sociality: 0.85, Curiosity: 0.90,
		Attachment: 0.55, Whimsy: 0.85, Nostalgia: 0.15},
	StageChild: {Reactivity: 0.80, MoodStability: 0.45, Sociality: 0.80, Curiosity: 0.80,
		Attachment: 0.65, Whimsy: 0.80, Nostalgia: 0.30},
	StageTeen: {Reactivity: 0.85, MoodStability: 0.50, Sociality: 0.90, Curiosity: 0.60,
		Attachment: 0.70, Whimsy: 0.55, Nostalgia: 0.50},
	StageAdult: {Reactivity: 0.50, MoodStability: 0.85, Sociality: 0.70, Curiosity: 0.50,
		Attachment: 0.90, Whimsy: 0.40, Nostalgia: 0.85},
}

// stagePull is how far from its birth temperament each stage sits: nothing at
// birth, most of the way at adulthood. It is measured from BASE, not from the
// previous stage, so growing up is a single continuous settling and a trait
// can never be dragged past the target it was heading for.
var stagePull = [StageCount]float64{
	StageBaby:  0,
	StageChild: 0.35,
	StageTeen:  0.55,
	StageAdult: 0.72,
}

// applyStagePersonality steers the temperament towards the new stage's.
func (t *TamagotchiState) applyStagePersonality(from Stage) {
	a := stageArchetype[t.Stage]
	base := t.basePersonality()
	pull := stagePull[t.Stage]
	lerp := func(v, want float64) float64 {
		return math.Max(0.05, math.Min(0.95, v+(want-v)*pull))
	}
	p := &t.Personality
	p.Reactivity = lerp(base.Reactivity, a.Reactivity)
	p.MoodStability = lerp(base.MoodStability, a.MoodStability)
	p.Sociality = lerp(base.Sociality, a.Sociality)
	p.Curiosity = lerp(base.Curiosity, a.Curiosity)
	p.Attachment = lerp(base.Attachment, a.Attachment)
	p.Whimsy = lerp(base.Whimsy, a.Whimsy)
	p.Nostalgia = lerp(base.Nostalgia, a.Nostalgia)
	// The legacy mirrors the rest of the code reads.
	t.Curiosity = p.Curiosity
	t.Friendliness = p.Sociality
	t.EnergyLevel = p.Reactivity
	_ = from
}

// basePersonality is the temperament at birth, defaulting to the current one for
// a creature saved before Base existed.
func (t *TamagotchiState) basePersonality() Personality {
	b := t.Base
	if b == (Personality{}) {
		b = t.Personality
	}
	return b
}

// evolveLine is what the creature says as it changes. Short, and different at
// every stage, because the moment is the point.
func evolveLine(s Stage) string {
	switch s {
	case StageChild:
		return "! everything is bigger!"
	case StageTeen:
		return "I can see further from up here."
	case StageAdult:
		return "thank you for staying."
	}
	return ""
}

// stageVoice is the register each stage speaks in, used when the creature has
// nothing more specific to say. A baby says one word, an adult says a sentence.
func stageVoice(s Stage) []string {
	switch s {
	case StageBaby:
		return []string{"gaa!", "mm?", "aah", "ba-ba"}
	case StageChild:
		return []string{"again!", "look, look!", "I did it!", "hmm hmm"}
	case StageTeen:
		return []string{"...what.", "I'm fine.", "do you ever sleep?", "obviously."}
	case StageAdult:
		return []string{"I like it when you're here.", "the light is nice today.",
			"you work too much.", "I'll keep watch."}
	}
	return nil
}

// ---------------------------------------------------------------------------
// HOW A STAGE LOOKS, AND HOW IT MOVES.
// ---------------------------------------------------------------------------

// StageLook is the silhouette of a stage, in SOURCE pixels of the rendered
// body, and how the whole animal sits in its box.
//
// The creature is authored at 24x28 and the stage resamples it: a baby is 20x24
// and sits a cell lower (smaller, rounder, still on the same ground), a child is
// the art itself, a teen is narrower and taller, an adult is broader and taller
// still. The face is offset by the same proportion, so the eyes stay on the
// head, and -- because the eyes are a fixed two cells while the head is not --
// a baby ends up with visibly bigger eyes than an adult. That is the whole trick
// of reading age at twelve columns.
type StageLook struct {
	W, H int // source pixels of the body
}

func LookOf(s Stage) StageLook {
	switch s {
	case StageBaby:
		return StageLook{W: 19, H: 24}
	case StageChild:
		return StageLook{W: 24, H: 28}
	case StageTeen:
		return StageLook{W: 21, H: 30}
	case StageAdult:
		return StageLook{W: 28, H: 30}
	}
	return StageLook{W: 24, H: 28}
}

// FaceBox is where the face sits on a stage's body, in CELLS relative to the
// box. The face is not a fixed overlay: it scales with the head, so a mouth that
// is on the chin of a child is on the chin of a baby half its size, and a baby's
// eyes sit closer together than an adult's.
//
// The one thing that does NOT scale is the eye itself, which is two cells at
// every age -- so a baby's eyes take up a visibly bigger share of its head. That
// is the whole trick of reading age at twelve columns.
type FaceBox struct {
	DX, DY   int // where the whole face starts, in cells
	EyeA     int // left eye, first cell
	EyeB     int // right eye, first cell
	EyeRow   int
	MouthCol int
	MouthRow int
	CheekA   int
	CheekB   int
}

// Face returns the face's place on this body, scaled from the 24x28 reference.
func (l StageLook) Face() FaceBox {
	sx := float64(l.W) / 24
	sy := float64(l.H) / 28
	scale := func(v float64, f float64) int { return int(math.Round(v * f)) }
	return FaceBox{
		DX: roundHalf((24 - l.W) / 2), DY: (28 - l.H) / 4,
		EyeA: scale(2, sx), EyeB: scale(7, sx), EyeRow: scale(2, sy),
		MouthCol: scale(5, sx), MouthRow: scale(4, sy),
		CheekA: scale(1, sx), CheekB: scale(10, sx),
	}
}

// Drop is how many cells down the body is drawn, so every stage stands on the
// same ground: a short body hangs from the bottom of the box, not its top.
func (l StageLook) Drop() int { return (28 - l.H) / 4 }

func roundHalf(v int) int {
	if v >= 0 {
		return (v + 1) / 2
	}
	return -((-v + 1) / 2)
}

// StageTempo is how a stage moves: how far its feelings carry it, and how often
// it fidgets when nothing is happening.
//
// A baby's joy is enormous and its patience is not; an adult's joy is smaller
// and its stillness is the point. The numbers multiply the pose's motion and
// divide the fidget interval, so the same clip table plays four different
// animals.
type StageTempo struct {
	Motion  float64 // multiplies lift, sway, lean, squash
	Fidget  float64 // multiplies the seconds between fidgets (>1 = calmer)
	BlinkIn float64 // multiplies the idle blink's length, cosmetic
}

func TempoOf(s Stage) StageTempo {
	switch s {
	case StageBaby:
		return StageTempo{Motion: 1.30, Fidget: 0.55, BlinkIn: 0.8}
	case StageChild:
		return StageTempo{Motion: 1.15, Fidget: 0.80, BlinkIn: 0.9}
	case StageTeen:
		return StageTempo{Motion: 1.00, Fidget: 1.00, BlinkIn: 1.0}
	case StageAdult:
		return StageTempo{Motion: 0.85, Fidget: 1.60, BlinkIn: 1.2}
	}
	return StageTempo{Motion: 1, Fidget: 1, BlinkIn: 1}
}

// StageInk is the light or shade a stage's coat is pulled towards: a baby is
// paler than the adult it becomes, and an adult is deeper than the child it was.
// It is a blend, not a colour, so the companion's coat choice still decides the
// hue -- the stage only decides how much of it has grown in.
type StageInk struct {
	To    [3]float64 // target RGB, 0..255
	Blend float64
}

func InkOf(s Stage) StageInk {
	switch s {
	case StageBaby:
		return StageInk{To: [3]float64{236, 240, 228}, Blend: 0.22}
	case StageChild:
		return StageInk{To: [3]float64{128, 128, 128}, Blend: 0.0}
	case StageTeen:
		return StageInk{To: [3]float64{255, 255, 255}, Blend: 0.06}
	case StageAdult:
		return StageInk{To: [3]float64{40, 44, 36}, Blend: 0.16}
	}
	return StageInk{}
}
