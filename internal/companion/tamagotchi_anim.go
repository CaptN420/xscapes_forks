package companion

import (
	"math"
	"math/rand"

	"github.com/donlucasx/xscapes/internal/term"
)

// ---------------------------------------------------------------------------
// THE CREATURE'S ANIMATION LAYER.
//
// Same shape as the rest of the companions in this package and for the same
// reason: a companion is a PROCEDURAL pose, recomputed from the clock every
// frame, never a stored frame counter. cat.go says it in one line -- "everything
// that moves is computed here; nothing is stored between frames" -- and the cat
// has breathed that way since it was written.
//
// What is new here is that a Tamagotchi needs more than one animation at a time,
// and a transition between them that does not pop. Two things make that work:
//
//  1. A POSE IS CONTINUOUS. Lift, sway, lean, squash, gaze, blush and the six
//     effect weights are floats, so two animations can be blended by
//     interpolating their poses. The blob hops; it does not cut to a hop.
//  2. THE DISCRETE PARTS SWAP AT THE MIDDLE of a transition, when the outgoing
//     animation's continuous envelope has already dragged its body toward the
//     incoming one. The arms of BodyArms appear while the blob is already
//     rising, which reads as raising its arms.
//
// The clips are data (keyframes), not code, so a new animation is a table entry
// and the tests can assert on the timeline rather than on pixels.
// ---------------------------------------------------------------------------

// Clip names. TamagotchiState.Animation carries one of these, and the animator
// turns a change in it into a transition.
const (
	ClipIdle  = "idle"
	ClipJoy   = "joy"
	ClipSad   = "sad"
	ClipSleep = "sleep"
	ClipEat   = "eat"
	ClipPlay  = "play"
	ClipPet   = "pet"
	ClipSick  = "sick"

	// The fidgets. Short, one-shot, and only ever played out of idle: the
	// small business of an animal that has nothing to do. They start and end
	// on the idle pose so an interruption is invisible.
	ClipBlink   = "blink"
	ClipLook    = "look"
	ClipTwitch  = "twitch"
	ClipSigh    = "sigh"
	ClipHop     = "hop"
	ClipStretch = "stretch"
)

// Body art indices. The upper half of the sprite is what a pose swaps; the
// lower half (the feet) is shared, exactly as the crab splits its shell.
const (
	BodyStand = iota
	BodyArms
	BodyCurl
	BodyLean
	BodyPuff
	BodyCount
)

// Eye and mouth indices.
const (
	EyeOpen = iota
	EyeHappy
	EyeSparkle
	EyeLove
	EyeSad
	EyeWide
	EyeShut
	EyeSleepy
	EyeSick
	EyeCount
)

const (
	MouthFlat = iota
	MouthSmile
	MouthOpen
	MouthChew
	MouthTiny
	MouthFrown
	MouthWide
	MouthCount
)

// TamPose is one frame of the creature: continuous where it can be, indexed
// where it cannot. The ints are the parts a bitmap cannot blend (which body,
// which eye, which mouth); everything else is a float precisely so that two
// clips can be lerped into one another.
type TamPose struct {
	Body int

	// Body motion, in SOURCE pixels (the sprite is authored 24x28 and drawn at
	// half width, quarter height, so 2 px is one character cell).
	Lift   float64 // up, for hops
	Sway   float64 // sideways, for the idle wobble
	Lean   float64 // upper half only, for leaning into a hand or away
	Squash float64 // upper half compressed downward, 0..4, for crouch/sleep

	// The face.
	Eyes  int
	Mouth int
	Gaze  float64 // eye offset, source px

	// Overlays. Each is 0..1; they are weights, not positions, because the
	// position of a heart is a function of its weight and the clock.
	Blush   float64
	Sparkle float64
	Hearts  float64
	Zzz     float64
	Crumbs  float64
	Tear    float64

	// Tint is the coat the pose asks for. Interpolated like everything else,
	// so a sad blue-grey becomes a happy green THROUGH the colour rather than
	// by cutting to it.
	Tint term.RGB
}

// TamKey is one keyframe: a pose at a position in the clip, 0..1.
type TamKey struct {
	At   float64
	Pose TamPose
}

// TamClip is one animation.
type TamClip struct {
	Name string
	Dur  float64 // seconds for one pass
	Loop bool
	Keys []TamKey
}

// poseAt reads the clip at a phase. Looping clips wrap; one-shot clips clamp
// at their last key, so a finished animation holds its final frame instead of
// flicking back to the first.
func (c *TamClip) poseAt(phase float64) TamPose {
	if len(c.Keys) == 0 {
		return TamPose{}
	}
	if c.Loop {
		phase -= math.Floor(phase)
	}
	if phase < 0 {
		phase = 0
	}
	if phase >= 1 {
		return c.Keys[len(c.Keys)-1].Pose
	}
	for i := 1; i < len(c.Keys); i++ {
		b := c.Keys[i]
		if phase > b.At {
			continue
		}
		a := c.Keys[i-1]
		span := b.At - a.At
		u := 0.0
		if span > 0 {
			u = (phase - a.At) / span
		}
		return lerpTamPose(a.Pose, b.Pose, smoothstep(u))
	}
	return c.Keys[len(c.Keys)-1].Pose
}

// smoothstep is the ease. Every transition and every keyframe segment goes
// through it, so nothing in this file starts or stops abruptly -- which is the
// whole of "smooth transitions" at this resolution.
func smoothstep(u float64) float64 {
	if u <= 0 {
		return 0
	}
	if u >= 1 {
		return 1
	}
	return u * u * (3 - 2*u)
}

// lerpTamPose blends two poses. Floats interpolate; the four discrete choices
// step at the halfway point, by which time the continuous half of the blend has
// already carried the body most of the way across.
func lerpTamPose(a, b TamPose, w float64) TamPose {
	if w <= 0 {
		return a
	}
	if w >= 1 {
		return b
	}
	pick := func(x, y int) int {
		if w < 0.5 {
			return x
		}
		return y
	}
	f := func(x, y float64) float64 { return x + (y-x)*w }
	return TamPose{
		Body:    pick(a.Body, b.Body),
		Lift:    f(a.Lift, b.Lift),
		Sway:    f(a.Sway, b.Sway),
		Lean:    f(a.Lean, b.Lean),
		Squash:  f(a.Squash, b.Squash),
		Eyes:    pick(a.Eyes, b.Eyes),
		Mouth:   pick(a.Mouth, b.Mouth),
		Gaze:    f(a.Gaze, b.Gaze),
		Blush:   f(a.Blush, b.Blush),
		Sparkle: f(a.Sparkle, b.Sparkle),
		Hearts:  f(a.Hearts, b.Hearts),
		Zzz:     f(a.Zzz, b.Zzz),
		Crumbs:  f(a.Crumbs, b.Crumbs),
		Tear:    f(a.Tear, b.Tear),
		Tint:    term.Lerp(a.Tint, b.Tint, w),
	}
}

// ---------------------------------------------------------------------------
// THE COATS. One colour per feeling, and they are all near-neutrals except the
// joyful green, so term.GlyphBoost (see colors.go) leaves the unhappy ones and
// lifts the happy one -- which is the right way round for a dark scene.
// ---------------------------------------------------------------------------

var (
	TamCoatCalm    = term.RGB{R: 150, G: 200, B: 150}
	TamCoatHappy   = term.RGB{R: 170, G: 226, B: 158}
	TamCoatSad     = term.RGB{R: 150, G: 180, B: 200}
	TamCoatTired   = term.RGB{R: 150, G: 158, B: 190}
	TamCoatSick    = term.RGB{R: 186, G: 162, B: 162}
	TamCoatExcited = term.RGB{R: 214, G: 196, B: 140}
)

// pose builds a pose with the coat filled in, so the tables below stay short.
func tamPose(body, eyes, mouth int, coat term.RGB) TamPose {
	return TamPose{Body: body, Eyes: eyes, Mouth: mouth, Tint: coat}
}

// base is the idle stand: the pose every fidget starts and ends on, so an
// interruption cannot be seen.
func tamBasePose() TamPose {
	return tamPose(BodyStand, EyeOpen, MouthFlat, TamCoatCalm)
}

// TamClips is the animation table. Keys are positions in the clip and poses.
//
// A note on the fidgets: they begin and end on tamBasePose for the reason above,
// and they are short on purpose. The point of an idle animation is that the
// animal looks alive when you glance at it, not that it performs.
var TamClips = []TamClip{
	{ // idle: breathing, and a blink two thirds of the way through
		Name: ClipIdle, Dur: 4.6, Loop: true,
		Keys: []TamKey{
			{0.00, tamBasePose()},
			{0.42, withLift(tamBasePose(), 2)},
			{0.66, tamBasePose()},
			{0.72, withEyes(tamBasePose(), EyeShut)},
			{0.78, tamBasePose()},
			{0.90, withLift(tamBasePose(), 1)},
			{1.00, tamBasePose()},
		},
	},
	{ // joy: arms up, a two-part hop, sparkles, the whole animal lit
		Name: ClipJoy, Dur: 1.0, Loop: true,
		Keys: []TamKey{
			{0.00, joyPose(0, 0.55)},
			{0.22, joyPose(5, 1.00)},
			{0.46, joyPose(0, 0.80)},
			{0.70, joyPose(3, 1.00)},
			{1.00, joyPose(0, 0.55)},
		},
	},
	{ // sad: drawn in, antennae gone, a slow sideways slump and a tear
		Name: ClipSad, Dur: 4.2, Loop: true,
		Keys: []TamKey{
			{0.00, sadPose(0, 0)},
			{0.30, sadPose(1, 0)},
			{0.55, sadPose(0, 0.35)},
			{0.75, sadPose(-1, 1)},
			{1.00, sadPose(0, 0)},
		},
	},
	{ // sleep: crouched, eyes shut, a slow deep breath and drifting Zs
		Name: ClipSleep, Dur: 5.4, Loop: true,
		Keys: []TamKey{
			{0.00, sleepPose(3, 0, 0)},
			{0.35, sleepPose(3, 1.5, 0.55)},
			{0.50, sleepPose(4, 2, 1.00)},
			{0.80, sleepPose(3, 0.5, 0.55)},
			{1.00, sleepPose(3, 0, 0)},
		},
	},
	{ // eat: leaning over the food, chewing, crumbs falling
		Name: ClipEat, Dur: 1.6, Loop: false,
		Keys: []TamKey{
			{0.00, eatPose(MouthOpen, EyeHappy, 1, 0.4)},
			{0.25, eatPose(MouthChew, EyeHappy, 3, 1.0)},
			{0.50, eatPose(MouthOpen, EyeHappy, 2, 0.8)},
			{0.75, eatPose(MouthChew, EyeHappy, 3, 1.0)},
			{1.00, eatPose(MouthSmile, EyeHappy, 0, 0.0)},
		},
	},
	{ // play: two hops, sparkles, arms up -- the excited version of joy
		Name: ClipPlay, Dur: 1.8, Loop: false,
		Keys: []TamKey{
			{0.00, playPose(0, EyeSparkle)},
			{0.18, playPose(6, EyeSparkle)},
			{0.36, playPose(0, EyeWide)},
			{0.54, playPose(5, EyeSparkle)},
			{0.72, playPose(1, EyeWide)},
			{1.00, playPose(0, EyeHappy)},
		},
	},
	{ // the affection reaction: leans into the hand, blushes, hearts rise
		Name: ClipPet, Dur: 1.9, Loop: false,
		Keys: []TamKey{
			{0.00, petPose(0, 0.00, 0.0, EyeOpen)},
			{0.20, petPose(2, 0.55, 0.6, EyeHappy)},
			{0.45, petPose(3, 1.00, 1.0, EyeLove)},
			{0.70, petPose(2, 0.80, 0.7, EyeHappy)},
			{1.00, petPose(0, 0.15, 0.0, EyeHappy)},
		},
	},
	{ // sick: puffed up, squinting, shivering sideways, a sweat bead
		Name: ClipSick, Dur: 3.0, Loop: true,
		Keys: []TamKey{
			{0.00, sickPose(-1, 0.4)},
			{0.25, sickPose(1, 1.0)},
			{0.50, sickPose(-1, 0.6)},
			{0.75, sickPose(1, 1.0)},
			{1.00, sickPose(-1, 0.4)},
		},
	},

	// ---- the fidgets -------------------------------------------------------
	{
		Name: ClipBlink, Dur: 0.24, Loop: false,
		Keys: []TamKey{
			{0.00, tamBasePose()},
			{0.30, withEyes(tamBasePose(), EyeShut)},
			{0.70, withEyes(tamBasePose(), EyeShut)},
			{1.00, tamBasePose()},
		},
	},
	{
		Name: ClipLook, Dur: 1.4, Loop: false,
		Keys: []TamKey{
			{0.00, tamBasePose()},
			{0.25, withGaze(tamBasePose(), -4)},
			{0.50, tamBasePose()},
			{0.75, withGaze(tamBasePose(), 4)},
			{1.00, tamBasePose()},
		},
	},
	{
		Name: ClipTwitch, Dur: 0.5, Loop: false,
		Keys: []TamKey{
			{0.00, tamBasePose()},
			{0.25, withLean(tamBasePose(), 2)},
			{0.50, withLean(tamBasePose(), -2)},
			{0.75, withLean(tamBasePose(), 1)},
			{1.00, tamBasePose()},
		},
	},
	{
		Name: ClipSigh, Dur: 1.6, Loop: false,
		Keys: []TamKey{
			{0.00, tamBasePose()},
			{0.30, withSquash(withEyes(tamBasePose(), EyeSleepy), 3)},
			{0.75, withSquash(tamBasePose(), 1)},
			{1.00, tamBasePose()},
		},
	},
	{
		Name: ClipHop, Dur: 0.7, Loop: false,
		Keys: []TamKey{
			{0.00, tamBasePose()},
			{0.35, withSparkle(withLift(tamBasePose(), 5), 0.5)},
			{0.70, withLift(tamBasePose(), 1)},
			{1.00, tamBasePose()},
		},
	},
	{
		Name: ClipStretch, Dur: 1.8, Loop: false,
		Keys: []TamKey{
			{0.00, tamBasePose()},
			{0.20, withSquash(withEyes(tamBasePose(), EyeShut), 3)},
			{0.55, withLift(withEyes(tamBasePose(), EyeOpen), 3)},
			{0.80, withLift(tamBasePose(), 1)},
			{1.00, tamBasePose()},
		},
	},
}

// The pose builders. Small and dull on purpose: a clip table is read far more
// often than it is written.
func withLift(p TamPose, v float64) TamPose   { p.Lift = v; return p }
func withGaze(p TamPose, v float64) TamPose   { p.Gaze = v; return p }
func withLean(p TamPose, v float64) TamPose   { p.Lean = v; return p }
func withSquash(p TamPose, v float64) TamPose { p.Squash = v; return p }
func withSparkle(p TamPose, v float64) TamPose {
	p.Sparkle = v
	return p
}
func withEyes(p TamPose, e int) TamPose { p.Eyes = e; return p }

func joyPose(lift, sparkle float64) TamPose {
	p := tamPose(BodyArms, EyeSparkle, MouthSmile, TamCoatHappy)
	p.Lift, p.Sparkle, p.Blush = lift, sparkle, 0.35
	return p
}

func sadPose(sway, tear float64) TamPose {
	p := tamPose(BodyCurl, EyeSad, MouthFrown, TamCoatSad)
	p.Sway, p.Tear, p.Squash = sway, tear, 2
	return p
}

func sleepPose(squash, lift, zzz float64) TamPose {
	p := tamPose(BodyCurl, EyeShut, MouthTiny, TamCoatTired)
	p.Squash, p.Lift, p.Zzz = squash, lift, zzz
	return p
}

func eatPose(mouth, eyes int, squash, crumbs float64) TamPose {
	p := tamPose(BodyLean, eyes, mouth, TamCoatExcited)
	p.Squash, p.Crumbs = squash, crumbs
	return p
}

func playPose(lift float64, eyes int) TamPose {
	p := tamPose(BodyArms, eyes, MouthWide, TamCoatHappy)
	p.Lift, p.Sparkle, p.Blush = lift, 0.8, 0.3
	return p
}

func petPose(lean, blush, hearts float64, eyes int) TamPose {
	p := tamPose(BodyStand, eyes, MouthSmile, TamCoatHappy)
	p.Lean, p.Blush, p.Hearts = lean, blush, hearts
	return p
}

func sickPose(sway, sweat float64) TamPose {
	p := tamPose(BodyPuff, EyeSick, MouthFlat, TamCoatSick)
	p.Sway, p.Tear, p.Squash = sway, sweat, 1
	return p
}

// TamClipByName finds a clip. Unknown names fall back to idle rather than to a
// blank pose: an animation nobody authored should still be a living animal.
func TamClipByName(name string) *TamClip {
	for i := range TamClips {
		if TamClips[i].Name == name {
			return &TamClips[i]
		}
	}
	return TamClipByName(ClipIdle)
}

// TamFidgets is what the creature may do when it has nothing to do. Ordered, so
// the choice is a table lookup and not a second list to keep in step.
var TamFidgets = []string{ClipBlink, ClipLook, ClipTwitch, ClipSigh, ClipHop, ClipStretch}

// ---------------------------------------------------------------------------
// THE ANIMATOR.
// ---------------------------------------------------------------------------

// BlendDur is how long a transition takes. 0.30 s is long enough to read as a
// movement and short enough that a hop answering a tap still feels immediate.
var BlendDur = 0.30

// TamAnimator plays one clip at a time and blends between them.
//
// It is deliberately NOT a state machine over the creature's needs: the state
// decides WHICH clip it wants (base), the animator decides how to get there, and
// the fidgets are the animator's own business because they are not something the
// creature decides to do.
type TamAnimator struct {
	// FidgetEvery multiplies the seconds between idle fidgets: a baby's
	// patience is half an adult's. Zero is treated as one so a zero-valued
	// animator behaves like the one this file shipped with.
	FidgetEvery float64

	base *TamClip
	cur  *TamClip

	// The outgoing clip, kept RUNNING while it fades out. A frozen silhouette
	// under a fade is the one thing that makes a blend look like a cut.
	prev      *TamClip
	prevPhase float64
	prevDur   float64

	phase float64
	clock float64
	blend float64

	rng     *rand.Rand
	idleFor float64
	nextFid float64
}

// NewTamAnimator builds an animator on the idle clip. The seed only drives
// which fidget is picked and when, so a fixed seed gives a reproducible life.
// FidgetScale stretches or shortens the wait between idle fidgets: a baby
// fidgets twice as often as an adult, and a still adult is the point of being
// old. Zero (the default) means one.
func (a *TamAnimator) FidgetScale() float64 { return a.fidgetScale() }

func (a *TamAnimator) fidgetScale() float64 {
	if a.FidgetEvery < 0.05 {
		return 1
	}
	return a.FidgetEvery
}

func NewTamAnimator(seed int64) *TamAnimator {
	a := &TamAnimator{rng: rand.New(rand.NewSource(seed))}
	a.base = TamClipByName(ClipIdle)
	a.cur = a.base
	a.blend = 1
	a.nextFid = 3 + a.rng.Float64()*5
	return a
}

// Clip is the animation actually on screen, which is a fidget while one plays.
func (a *TamAnimator) Clip() string { return a.cur.Name }

// Base is the animation asked for from outside.
func (a *TamAnimator) Base() string { return a.base.Name }

// Blend is how far the current transition has run, 0..1. 1 means settled.
func (a *TamAnimator) Blend() float64 { return a.blend }

// Transitioning reports whether a blend is still in flight.
func (a *TamAnimator) Transitioning() bool { return a.blend < 1 }

// SetBase asks for a clip. A request for the clip already playing is ignored,
// so a state that re-states its mood every frame (UpdateMood does) cannot
// restart the animation sixty times a second.
func (a *TamAnimator) SetBase(name string) {
	def := TamClipByName(name)
	if a.base != nil && def.Name == a.base.Name {
		return
	}
	a.start(def)
	a.base = def
}

// Fidget starts one of the idle animations by name, if the creature is idle.
// Exposed for the tests and for a future keypress; the scheduler below uses it.
func (a *TamAnimator) Fidget(name string) bool {
	if a.base == nil || a.base.Name != ClipIdle || a.Transitioning() {
		return false
	}
	def := TamClipByName(name)
	if def.Name == ClipIdle {
		return false
	}
	a.start(def)
	return true
}

// start begins a clip and opens a transition from whatever is playing.
func (a *TamAnimator) start(def *TamClip) {
	if a.cur != nil && def.Name == a.cur.Name {
		return
	}
	a.prev = a.cur
	a.prevPhase = a.phase
	if a.cur != nil {
		a.prevDur = a.cur.Dur
	}
	if a.prevDur <= 0 {
		a.prevDur = def.Dur
	}
	a.cur = def
	a.phase = 0
	if BlendDur <= 0 {
		a.blend = 1
	} else {
		a.blend = 0
	}
}

// Advance moves the clock. dt is real seconds; it is clamped because a stalled
// frame (a resize, a sleeping laptop) must not fast-forward the animal through
// half its animation in one step.
func (a *TamAnimator) Advance(dt float64) {
	if dt < 0 {
		dt = 0
	}
	if dt > 0.5 {
		dt = 0.5
	}
	a.clock += dt
	a.idleFor += dt

	if a.blend < 1 {
		if BlendDur <= 0 {
			a.blend = 1
		} else {
			a.blend += dt / BlendDur
			if a.blend > 1 {
				a.blend = 1
			}
		}
	}
	if a.blend >= 1 {
		a.prev = nil
	}

	// The outgoing clip keeps moving under the fade.
	if a.prev != nil && a.prevDur > 0 {
		a.prevPhase += dt / a.prevDur
	}

	if a.cur == nil {
		a.cur = TamClipByName(ClipIdle)
	}
	if a.cur.Dur > 0 {
		a.phase += dt / a.cur.Dur
	}
	if a.cur.Loop {
		if a.phase >= 1 {
			a.phase -= math.Floor(a.phase)
		}
	} else if a.phase >= 1 {
		// A finished one-shot gives the animal back. A one-shot that IS the
		// base (the state holding "eat" for a few seconds) holds its last
		// frame instead: the state owns the hold, not the animator.
		a.phase = 1
		if a.base != nil && a.cur.Name != a.base.Name {
			a.start(a.base)
		}
	}

	a.scheduleFidget(dt)
}

// scheduleFidget fires one of the idle animations every few seconds. Only in
// idle, only when the last transition has settled, and only when no clip is
// held -- an animal being petted must not blink through it.
func (a *TamAnimator) scheduleFidget(dt float64) {
	if a.base == nil || a.base.Name != ClipIdle || a.Transitioning() {
		return
	}
	if a.cur.Name != ClipIdle {
		return
	}
	a.nextFid -= dt
	if a.nextFid > 0 {
		return
	}
	// The short ones are the common case; a hop or a stretch is a treat. The
	// weights are the table order repeated, which keeps the list in one place.
	pool := []string{
		ClipBlink, ClipBlink, ClipBlink, ClipLook, ClipLook,
		ClipTwitch, ClipTwitch, ClipSigh, ClipHop, ClipStretch,
	}
	a.start(TamClipByName(pool[a.rng.Intn(len(pool))]))
	a.nextFid = (4 + a.rng.Float64()*7) * a.fidgetScale()
}

// Pose is the frame to draw: the current clip, blended out of the previous one.
func (a *TamAnimator) Pose() TamPose {
	out, in, w := a.Frames()
	return lerpTamPose(out, in, smoothstep(w))
}

// Frames returns the two poses a transition is between, and how far it has run
// (1 = settled, and both poses are then the same one).
//
// The renderer wants them APART rather than pre-blended, because the parts that
// cannot be interpolated -- which body, which eye, which mouth -- are
// cross-faded in the bitmap instead of being swapped at the halfway point. See
// blendBitmaps.
func (a *TamAnimator) Frames() (out, in TamPose, w float64) {
	if a.cur == nil {
		return tamBasePose(), tamBasePose(), 1
	}
	in = a.cur.poseAt(a.phase)
	if a.prev == nil || a.blend >= 1 {
		return in, in, 1
	}
	return a.prev.poseAt(a.prevPhase), in, a.blend
}

// Clock is how long the animator has been running, for a caller that wants to
// keep its own effects in step.
func (a *TamAnimator) Clock() float64 { return a.clock }
