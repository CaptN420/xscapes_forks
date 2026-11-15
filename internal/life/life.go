// Package life is the companion's own life. It is a pure state machine: it
// reads a snapshot of the world, advances on a delta in seconds, and returns
// what the animal is doing and where. It knows nothing about canvases, agents,
// events, sockets, files or terminals.
//
// The philosophy, from the Phase 10 brief, is enforced here rather than in a
// document: the companion lives whether or not anybody touches it. Nothing in
// this package can make it sick, hungry in a way that matters, or sad that the
// user went away. Ignoring it for a day is a valid way to keep it.
//
// Two channels, and they never cross -- the same rule the scenes obey:
//
//	POSITION  how far the companion has stepped: a count of the agent's work.
//	POSE      what the companion is doing with its own time: its own life.
//
// A screenshot still reads both, which is the whole reason the pace in pace.go
// is a count and not a drift. The life layer therefore moves the animal at most
// one cell either side of home, inside the strip the layout already reserves.
package life

import (
	"math"
	"strings"
)

// Stage is how old the companion is, and it is named for what you see rather
// than for a schedule: a hatchling, a youngster, a grown crab, an old one.
type Stage int

const (
	Baby Stage = iota
	Child
	Teen
	Adult
	StageCount
)

func (s Stage) String() string {
	switch s {
	case Baby:
		return "hatchling"
	case Child:
		return "youngster"
	case Teen:
		return "grown"
	case Adult:
		return "old friend"
	}
	return "unknown"
}

// StageSecs is how much LIVED TIME each stage takes, in seconds in front of the
// user. Engagement shortens it; nothing lengthens it.
//
// These are the numbers from the earlier creature (12 / 35 / 80 minutes) kept
// deliberately: a stage that arrives in a couple of minutes is a toy, and the
// brief asks for progress that is significant. What changed is the gate: time
// alone is enough now.
var StageSecs = [StageCount]float64{12 * 60, 35 * 60, 80 * 60, 0}

// Pose is what the companion is doing. It is deliberately a small vocabulary
// that maps onto poses the crab already has; the life layer chooses WHEN, never
// how it looks.
type Pose int

const (
	Idle           Pose = iota // standing, breathing
	Resting                    // settled on the sand
	Grooming                   // working its claws over its shell
	Watching                   // looking out at the water
	Digging                    // poking about in the sand
	Dozing                     // eyes shut, lower to the ground
	MoonGazing                 // looking up
	Excited                    // hopping about
	Walking                    // a step is in flight
	Wary                       // something broke; ears back
	Away                       // out on an adventure, not drawn
	ReturningPhase             // on the way back, carrying something
)

func (p Pose) String() string {
	switch p {
	case Idle:
		return "idle"
	case Resting:
		return "resting"
	case Grooming:
		return "grooming"
	case Watching:
		return "watching the water"
	case Digging:
		return "digging"
	case Dozing:
		return "dozing"
	case MoonGazing:
		return "looking up at the moon"
	case Excited:
		return "excited"
	case Walking:
		return "walking"
	case Wary:
		return "wary"
	case Away:
		return "away exploring"
	case ReturningPhase:
		return "coming home"
	}
	return "idle"
}

// World is the read-only snapshot the brain is allowed to know. Everything in
// it is already normalised by the caller; there is no event vocabulary here and
// no agent identifiers, so the companion cannot grow a dependency on any one
// adapter.
type World struct {
	Working     bool
	Level       float64 // 0..1
	Kittens     int
	ContextUsed float64 // 0..1
	Worried     bool    // something is broken
	Asking      bool    // the agent is blocked on the user
	Night       bool
}

// Intent is what the frame should draw.
type Intent struct {
	Pose     Pose
	DX       int    // cells from home, -1..1 by construction
	Speech   string // "" when quiet
	Carrying string // the name of a find, "" when empty-handed
	// Reaction is a short visual event in flight -- fire, a dance, a catch --
	// with its phase 0..1 and a seeded variant so two helpings of the same
	// food do not look identical. Empty means nothing to draw.
	Reaction        string
	ReactionPhase   float64
	ReactionVariant int
}

// Attributes are the positive counters. NONE of them can go down. Appetite is
// the one that rises on its own, and it is an appetite: it makes the companion
// interested in food, never unwell, and feeding it is a pleasure rather than a
// duty.
type Attributes struct {
	Happiness   float64 `json:"happiness"`
	Friendship  float64 `json:"friendship"`
	Curiosity   float64 `json:"curiosity"`
	Exploration float64 `json:"exploration"`
	Appetite    float64 `json:"appetite"`
	Discoveries int     `json:"discoveries"`
	Memories    int     `json:"memories"`
	Toys        int     `json:"toys"`
}

// Result is what an interaction did, for the command line and the tests.
type Result struct {
	OK      bool
	Message string
	Speech  string
}

// rng is a small xorshift so the brain is deterministic from a seed and does
// not touch the global generator (which would make two scapes in one process
// share a stream and drift apart from their saves).
type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return r.s
}

func (r *rng) f() float64 { return float64(r.next()>>11) / float64(1<<53) }

func (r *rng) between(a, b float64) float64 { return a + (b-a)*r.f() }

func (r *rng) intn(n int) int {
	if n <= 1 {
		return 0
	}
	return int(r.next() % uint64(n))
}

// brain constants. Named so the tests can hold them to account.
const (
	// Bout: what it does with its own time.
	boutMin = 5.0
	boutMax = 16.0
	gapMin  = 7.0
	gapMax  = 26.0

	// Engagement: a soft measure of how recently it was touched. It only ever
	// multiplies growth, and only upwards.
	engagePerTouch = 0.25
	engageHalfLife = 240.0 // seconds
	engageBonus    = 0.5   // at engagement 1.0 growth runs at 1.5x

	// Appetite fills over an hour; feeding takes a big bite out of it. The
	// amount by food is in food.go.
	appetitePerSec = 100.0 / 3600.0

	// Excursions.
	excMin          = 40.0
	excMax          = 150.0
	excSelfMin      = 6 * 60.0
	excSelfMax      = 14 * 60.0
	returnCarrySecs = 6.0

	speechSecs = 4.0
	stepSecs   = 0.30
)

// Brain is the companion's life. Create it with New and advance it with Tick.
type Brain struct {
	rng   rng
	seed  int64
	stage Stage
	// age is lived time in seconds; stageSecs is the part of it spent in the
	// current stage. Both are advanced by the same call, so they cannot drift.
	age, stageSecs float64
	attrs          Attributes

	// finds are the discoveries, oldest first; toys the gifts given.
	finds []string
	toys  []string
	// memories are the short lines it keeps. Bounded: a life is not a log.
	memories []string

	// The current bout.
	pose     Pose
	boutLeft float64
	gapLeft  float64

	// Where it is, in cells from home, and where it is heading.
	dx, dxTarget int
	stepLeft     float64
	moving       bool

	// Speech, held so it can be read.
	speech     string
	lastSaid   string
	speechLeft float64

	// Excursion.
	away      bool
	awayLeft  float64
	carrying  string
	carryLeft float64
	nextSelf  float64

	// engagement decays toward zero without interaction.
	engagement float64

	// A reaction in flight; see food.go.
	react        string
	reactLeft    float64
	reactTotal   float64
	reactVariant int

	// What it has eaten, and how much of each. Two helpings are a taste, not a
	// statistic: the CLI shows them, nothing in the engine branches on them.
	foodsEaten int
	foodSeen   map[string]int
	// quietLeft is the clock for an unprompted observation.
	quietLeft float64

	intent Intent
}

// New starts a life. A zero or negative seed is replaced with one, so two
// creatures made in the same millisecond are still different animals.
func New(seed int64) *Brain {
	if seed == 0 {
		seed = 1
	}
	b := &Brain{
		seed:     seed,
		rng:      rng{s: uint64(seed) | 1},
		stage:    Baby,
		pose:     Idle,
		gapLeft:  4,
		nextSelf: 3 * 60,
	}
	b.foodSeen = make(map[string]int, len(Foods))
	b.attrs.Happiness = 55
	b.attrs.Friendship = 10
	// It starts a little hungry, so the first meal is a welcome rather than a
	// refusal. This is the only place appetite is ever seeded.
	b.attrs.Appetite = 35
	return b
}

// Seed is the animal's identity for anything that needs to reproduce it.
func (b *Brain) Seed() int64 { return b.seed }

// Attributes returns a copy: the brain's numbers are not the caller's to edit.
func (b *Brain) Attributes() Attributes { return b.attrs }

// Finds returns the discoveries, oldest first.
func (b *Brain) Finds() []string { return b.finds }

// Toys returns the gifts it has been given.
func (b *Brain) Toys() []string { return b.toys }

// Memories returns the short lines it has kept, newest last.
func (b *Brain) Memories() []string { return b.memories }

// Intent is what to draw this frame.
func (b *Brain) Intent() Intent { return b.intent }

// Stage reports the age and how far into it the animal is, as a fraction of
// the lived time the stage needs. It is the number the status line quotes.
func (b *Brain) Stage() (Stage, float64) {
	need := StageSecs[b.stage]
	if need <= 0 {
		return b.stage, 1
	}
	p := b.stageSecs / need
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	return b.stage, p
}

// WhatNext says, in words, what the next stage is waiting for. There is only
// ever one answer now, because time is the only gate: engagement is a bonus,
// not a requirement, and the line says so.
func (b *Brain) WhatNext() string {
	if b.stage >= Adult {
		return "fully grown"
	}
	need := StageSecs[b.stage] - b.stageSecs
	if need <= 0 {
		return "growing up"
	}
	mins := int(need/60) + 1
	return itoa(mins) + " min more time together"
}

// Interaction verbs.
const (
	CmdFeed    = "feed"
	CmdPet     = "pet"
	CmdPlay    = "play"
	CmdWalk    = "walk"
	CmdExplore = "explore"
	CmdToy     = "toy"
	CmdCall    = "call"
)

// Do performs an interaction. Every verb answers, and none of them can fail in
// a way that punishes: the worst outcome is "not just now", which costs nothing
// and is forgotten immediately.
func (b *Brain) Do(cmd string) Result {
	// Whatever the verb did, the frame is drawn from the intent, so it must be
	// current before this returns -- including the "not just now" answers.
	defer b.publish()
	switch cmd {
	case CmdFeed:
		return b.eat("")

	case CmdPet:
		b.attrs.Happiness = bump(b.attrs.Happiness, 5)
		b.attrs.Friendship = bump(b.attrs.Friendship, 4)
		b.touch()
		b.setPose(Resting, 4)
		b.say("It leans into your hand")
		return Result{OK: true, Message: "It leans into your hand and stays there.", Speech: b.speech}

	case CmdPlay:
		b.attrs.Happiness = bump(b.attrs.Happiness, 7)
		b.attrs.Friendship = bump(b.attrs.Friendship, 3)
		b.attrs.Curiosity = bump(b.attrs.Curiosity, 2)
		b.touch()
		b.setPose(Excited, 5)
		b.say("A game! A game!")
		return Result{OK: true, Message: "It hops about, thoroughly delighted.", Speech: b.speech}

	case CmdWalk:
		b.attrs.Exploration = bump(b.attrs.Exploration, 4)
		b.attrs.Curiosity = bump(b.attrs.Curiosity, 3)
		b.touch()
		b.dxTarget = -1
		b.setPose(Walking, 3)
		b.say("Going for a stroll")
		return Result{OK: true, Message: "It sets off along the tideline and comes back.", Speech: b.speech}

	case CmdExplore:
		return b.sendOut(true)

	case CmdToy:
		name := toyNames[b.rng.intn(len(toyNames))]
		b.toys = append(b.toys, name)
		b.attrs.Toys = len(b.toys)
		b.attrs.Happiness = bump(b.attrs.Happiness, 6)
		b.attrs.Curiosity = bump(b.attrs.Curiosity, 4)
		b.touch()
		b.setPose(Excited, 4)
		b.say("For me? " + name)
		return Result{OK: true, Message: "It turns the " + name + " over and over.", Speech: b.speech}

	case CmdCall:
		// Calling it home is always possible; it is an invitation, not an order.
		if b.away {
			b.away = false
			b.awayLeft = 0
			b.pose = Excited
			b.boutLeft = 3
			b.say("Coming!")
			return Result{OK: true, Message: "It turns for home.", Speech: b.speech}
		}
		b.dxTarget = 0
		b.setPose(Walking, 2)
		b.say("I'm here")
		return Result{OK: true, Message: "It settles back at its spot.", Speech: b.speech}
	}
	return Result{OK: false, Message: "unknown interaction"}
}

// DoFood feeds a chosen food. An unknown name is not an error: it eats something
// from the larder instead, because a companion that scolds you for a typo is a
// companion that has misunderstood its job.
func (b *Brain) DoFood(id string) Result {
	// The frame is drawn from the intent, so it must be current before this
	// returns -- the same rule (and the same bug) as Do.
	defer b.publish()
	return b.eat(id)
}

// sendOut starts an excursion. A user-asked one always goes; a self-chosen one
// only when the animal is not needed and nothing is broken.
func (b *Brain) sendOut(asked bool) Result {
	if b.away {
		return Result{OK: false, Message: "It is already out exploring."}
	}
	b.away = true
	b.awayLeft = b.rng.between(excMin, excMax)
	b.pose = Away
	b.touch()
	if asked {
		b.say("I'll bring you something")
		return Result{OK: true, Message: "It scuttles off, and will bring something back.", Speech: b.speech}
	}
	b.say("Off to look around")
	return Result{OK: true, Message: "It wanders off on its own.", Speech: b.speech}
}

// touch credits engagement and advances the friendship clock.
func (b *Brain) touch() {
	b.engagement = bump(b.engagement, engagePerTouch)
}

func (b *Brain) setPose(p Pose, secs float64) {
	b.pose = p
	b.boutLeft = secs
}

func (b *Brain) say(s string) {
	b.lastSaid = s
	b.speech = s
	b.speechLeft = speechSecs
	// A life is not a log. Keep the last handful, and only the ones that were
	// worth saying (the caller decides by using remember).
}

func (b *Brain) remember(s string) {
	b.memories = append(b.memories, s)
	if len(b.memories) > 24 {
		b.memories = b.memories[len(b.memories)-24:]
	}
	b.attrs.Memories = len(b.memories)
}

// Tick advances the life by dt seconds (clamped by the caller). It is the only
// function that changes state on its own, and it is allocation-free when
// nothing happens.
func (b *Brain) Tick(dt float64, w World) {
	if !(dt > 0) {
		b.publish()
		return
	}
	if dt > 1.0 {
		dt = 1.0 // the caller clamps too; this is the second door
	}

	// Lived time, and growth. Engagement is a multiplier, never a gate.
	b.age += dt
	b.stageSecs += dt * (1 + engageBonus*b.engagement)
	if b.stage < Adult && b.stageSecs >= StageSecs[b.stage] {
		b.stageSecs -= StageSecs[b.stage]
		b.stage++
		b.growMoment()
	}
	if b.engagement > 0 {
		b.engagement *= pow05(dt / engageHalfLife)
		if b.engagement < 0.001 {
			b.engagement = 0
		}
	}

	// Appetite rises; it is never a need. Nothing else decays, ever.
	b.attrs.Appetite = bump(b.attrs.Appetite, appetitePerSec*dt)

	// Speech runs out on its own clock.
	if b.speechLeft > 0 {
		b.speechLeft -= dt
		if b.speechLeft <= 0 {
			b.speech = ""
		}
	}

	if b.reactLeft > 0 {
		b.reactLeft -= dt
		if b.reactLeft <= 0 {
			b.react, b.reactTotal = "", 0
		}
	}

	if b.away {
		b.awayLeft -= dt
		if b.awayLeft <= 0 {
			b.away = false
			b.arrive()
		}
		b.publish()
		return
	}

	if b.carryLeft > 0 {
		b.carryLeft -= dt
		if b.carryLeft <= 0 {
			b.carrying = ""
		}
	}

	// Wandering: one cell, slowly, and only inside the reserved strip.
	if b.dx != b.dxTarget {
		b.stepLeft -= dt
		if b.stepLeft <= 0 {
			if b.dx < b.dxTarget {
				b.dx++
			} else {
				b.dx--
			}
			b.stepLeft = stepSecs
			b.moving = true
		}
	} else {
		b.moving = false
	}

	// The bout clock.
	if b.boutLeft > 0 {
		b.boutLeft -= dt
		if b.boutLeft <= 0 {
			b.pose = Idle
			b.gapLeft = b.rng.between(gapMin, gapMax)
		}
	} else {
		b.gapLeft -= dt
		if b.gapLeft <= 0 {
			b.chooseBout(w)
		}
	}

	// A broken thing is the one thing that interrupts it, and it passes on its
	// own: the companion is allowed to notice, never to worry the user.
	if w.Worried && b.pose != Wary && b.boutLeft < 1.5 {
		b.pose = Wary
		b.boutLeft = 4
	}

	// Left alone for a long while, it notices something and says so. This is the
	// whole of "no notifications demanding attention": a line waits in the
	// scene for whenever the user next looks, and says nothing about them.
	b.quietLeft -= dt
	if b.quietLeft <= 0 {
		b.quietLeft = b.rng.between(600, 1500)
		if b.engagement <= 0 && b.speech == "" && !b.away {
			line := QuietLines[b.rng.intn(len(QuietLines))]
			b.say(line)
			b.remember(line)
		}
	}

	// Its own excursions, rarely, and only when there is nothing to watch.
	b.nextSelf -= dt
	if b.nextSelf <= 0 {
		b.nextSelf = b.rng.between(excSelfMin, excSelfMax)
		if !w.Working && !w.Worried && w.Level < 0.15 && b.attrs.Appetite < 70 {
			b.sendOut(false)
		}
	}

	b.publish()
}

// chooseBout picks the next thing it does with its time from the world it is in.
func (b *Brain) chooseBout(w World) {
	// The world suggests; the animal decides. A busy agent gets a companion
	// that watches; a broken one gets a companion that sits closer; the night
	// gets one that sleeps. Nothing here is required for the animal to be well.
	switch {
	case w.Worried:
		b.pose, b.boutLeft = Wary, b.rng.between(4, 8)
	case w.Asking:
		b.pose, b.boutLeft = Watching, b.rng.between(6, 10)
	case w.Night:
		b.pose, b.boutLeft = Dozing, b.rng.between(12, 26)
	case w.ContextUsed > 0.85:
		b.pose, b.boutLeft = MoonGazing, b.rng.between(6, 12)
	case w.Working && w.Level > 0.5:
		b.pose, b.boutLeft = Watching, b.rng.between(6, 12)
	default:
		switch b.rng.intn(5) {
		case 0:
			b.pose, b.boutLeft = Grooming, b.rng.between(boutMin, boutMax)
		case 1:
			b.pose, b.boutLeft = Digging, b.rng.between(boutMin, boutMax)
		case 2:
			b.pose, b.boutLeft = Watching, b.rng.between(boutMin, boutMax)
		case 3:
			b.pose, b.boutLeft = Resting, b.rng.between(boutMin, boutMax)
		default:
			b.pose, b.boutLeft = Idle, b.rng.between(boutMin, boutMax)
			b.dxTarget = b.rng.intn(3) - 1
		}
	}
	if b.pose != Idle && b.pose != Wary {
		b.dxTarget = 0
	}
}

// growMoment is the moment a stage changes. It is the only place the animal is
// allowed to make a noise about itself, and it is an invitation, not an alarm.
func (b *Brain) growMoment() {
	b.pose, b.boutLeft = Excited, 6
	b.attrs.Happiness = bump(b.attrs.Happiness, 10)
	b.remember("grew into " + b.stage.String())
	switch b.stage {
	case Child:
		b.say("I'm bigger!")
	case Teen:
		b.say("Look how far I can see")
	case Adult:
		b.say("I think I'll stay here")
	}
}

// arrive finishes an excursion: a find, a line, a moment.
func (b *Brain) arrive() {
	f := finds[b.rng.intn(len(finds))]
	b.finds = append(b.finds, f)
	b.attrs.Discoveries = len(b.finds)
	b.attrs.Exploration = bump(b.attrs.Exploration, 8)
	b.attrs.Curiosity = bump(b.attrs.Curiosity, 5)
	b.attrs.Happiness = bump(b.attrs.Happiness, 4)
	b.carrying = f
	b.carryLeft = returnCarrySecs
	b.pose, b.boutLeft = ReturningPhase, returnCarrySecs
	b.remember("found " + f)
	b.say("Look what I found: " + f)
}

// publish prepares the frame's intent.
func (b *Brain) publish() {
	p := b.pose
	dx := b.dx
	if b.moving && (p == Idle || p == Walking) {
		p = Walking
	}
	if b.away {
		p = Away
		dx = 0
	}
	kind, phase, variant := b.Reaction()
	b.intent = Intent{Pose: p, DX: dx, Speech: b.speech, Carrying: b.carrying,
		Reaction: kind, ReactionPhase: phase, ReactionVariant: variant}
}

// LastSaid is the most recent line it spoke, for the command line.
func (b *Brain) LastSaid() string { return b.lastSaid }

// Note is a line the animal says because something outside told it to -- a story
// about its own adventure, or an observation about the project. It is a normal
// speech line, so it obeys the same budget as everything else, and it is kept as
// a memory because it was worth saying.
func (b *Brain) Note(s string) {
	full := trimTo(s, 200)
	if full == "" {
		return
	}
	// What is drawn obeys the scene's budget; what is kept and what the command
	// line prints is the whole sentence.
	b.say(trimTo(full, 60))
	b.lastSaid = full
	b.remember(full)
	b.publish()
}

// trimTo cuts a line to a length the scene can plot, on a word boundary when
// there is one.
func trimTo(s string, max int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)[:max]
	for i := len(r) - 1; i > max/2; i-- {
		if r[i] == ' ' {
			return string(r[:i])
		}
	}
	return string(r)
}

// Status is the one line a person reads when they ask.
func (b *Brain) Status() string {
	st, _ := b.Stage()
	s := st.String() + ", " + itoa(int(b.stageSecs/(StageSecs[st]+1e-9)*100)) + "% to the next"
	if st == Adult {
		s = st.String() + ", fully grown"
	}
	return s + " | " + b.pose.String() + " | friends " + itoa(int(b.attrs.Friendship)) +
		"% | curious " + itoa(int(b.attrs.Curiosity)) + "% | found " + itoa(len(b.finds))
}

func bump(v, by float64) float64 {
	v += by
	if v > 100 {
		return 100
	}
	if v < 0 {
		return 0
	}
	return v
}

// pow05 is 0.5^x: the engagement half-life arithmetic, and the only exponential
// in the file. One exp per frame is nothing next to the scene's own work.
func pow05(x float64) float64 { return math.Exp(-x * 0.6931471805599453) }

// itoa avoids fmt in anything the frame path can reach.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
