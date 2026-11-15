package main

import (
	"math"
	"time"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/envx"
	"github.com/donlucasx/xscapes/internal/life"
	"github.com/donlucasx/xscapes/internal/reduce"
	"github.com/donlucasx/xscapes/internal/term"
)

// The bridge between the companion's own life (internal/life) and this program:
// where its file lives, when it is ticked, how its pose becomes a crab, and what
// it says. Everything that touches the world is here; the brain stays pure.
//
// The animal is the SAME crab the scene has always drawn. There is exactly one
// companion on screen: the life layer chooses which of its five poses it holds,
// how far it has wandered inside the reserved strip, and what it says. The
// earlier second creature is opt-in legacy (XSCAPES_TAMAGOTCHI=on).

var (
	lifeBrain *life.Brain
	lifeLast  time.Time
)

// lifeEnabled is the switch, read once so a frame never touches the environment.
var lifeEnabled = !isOff(envx.Lookup("LIFE"))

// lifeInteract says whether the CLI may touch the animal. It defaults on: the
// interactions are the point, and none of them can hurt.
var lifeInteract = !isOff(envx.Lookup("LIFE_INTERACT"))

// lifeStateFile is where a life is kept, beside everything else this program
// remembers.
func lifeStateFile() (string, error) {
	home, err := envx.HomeDir()
	if err != nil {
		return "", err
	}
	return home + "/.config/xscapes/life.json", nil
}

// legacyStateFile is the earlier creature's file, read once by the migration.
func legacyStateFile() (string, error) {
	home, err := envx.HomeDir()
	if err != nil {
		return "", err
	}
	return home + "/.config/xscapes/tamagotchi.json", nil
}

// initLife brings the animal up: from its save if there is one, from the old
// creature's save if there is one of those instead, and otherwise new.
func initLife() {
	if !lifeEnabled || lifeBrain != nil {
		return
	}
	path, err := lifeStateFile()
	if err != nil {
		lifeEnabled = false
		return
	}
	if b, ok := life.Load(path); ok {
		lifeBrain = b
		return
	}
	// First run with this system: carry an old animal's life across rather than
	// starting a stranger beside it.
	if old, err := legacyStateFile(); err == nil {
		if b, ok := life.Migrate(old); ok {
			lifeBrain = b
			_ = b.Write(path)
			return
		}
	}
	lifeBrain = life.New(seedFromClock())
}

// seedFromClock gives each new animal its own character without needing a
// random source on the frame path.
func seedFromClock() int64 {
	n := time.Now().UnixNano()
	if n == 0 {
		return 1
	}
	return n
}

// saveLife writes the animal down. Called when the scape closes and after each
// command, so a pet is never lost to a closed window.
func saveLife() {
	if lifeBrain == nil {
		return
	}
	path, err := lifeStateFile()
	if err != nil {
		return
	}
	_ = lifeBrain.Write(path)
}

// lifeInFrame is set by the composer for the frame it is drawing. Without it the
// switch is read twice -- once by newFrames, once by lifeLive -- and the two can
// disagree (a test that changes the environment after start-up sees exactly
// that). The frame's own flag wins.
var lifeInFrame = true

// lifeLive says whether this frame should tick and draw the animal.
func lifeLive() bool { return lifeEnabled && lifeBrain != nil && lifeInFrame }

// lifeWorld is the only thing the animal is allowed to know about the outside:
// a handful of normalised numbers, no events and no agent.
func lifeWorld(st reduce.State) life.World {
	tod := st.Act.TimeOfDay
	return life.World{
		Working:     st.Act.Working,
		Level:       st.Act.Level,
		Kittens:     st.Kittens,
		ContextUsed: st.Act.ContextUsed,
		Worried:     st.Pose == companion.Worried,
		Asking:      st.Bubble != "" && st.BubbleAsk,
		Night:       tod < 0.24 || tod > 0.82,
	}
}

// updateLife advances the animal. The clock is the frame's own, clamped, so a
// suspended laptop wakes to an animal that has dozed rather than aged a week.
func updateLife(now time.Time, st reduce.State) {
	if !lifeLive() {
		return
	}
	dt := 0.0
	if !lifeLast.IsZero() {
		dt = now.Sub(lifeLast).Seconds()
	}
	lifeLast = now
	if dt < 0 {
		dt = 0
	}
	if dt > 0.5 {
		dt = 0.5
	}
	lifeBrain.Tick(dt, lifeWorld(st))
	// The voice: drain whatever arrived, and ask for something new when an
	// adventure has just ended. Both halves are non-blocking.
	updateLifeAI(now)
	// Its lines live in the sand's own budget: a speech now and then, never a
	// stream. The brain holds each for a few seconds already.
}

// lifePose maps the animal's own pose onto the five faces the crab has. The
// vocabulary is the crab's, not a new one: nothing here invents art.
//
//	Excited  -> NeedsYou   claws up, alert: the same silhouette he chose for
//	                       "look at me", which reads equally as delight.
//	Dozing   -> Done       the settled, lowered-legs pose.
//	Grooming,
//	Digging  -> Working    the busy claws.
//	Warily.. -> Worried    claws back, ears down.
func lifePose(p life.Pose) (companion.State, bool) {
	switch p {
	case life.Excited, life.ReturningPhase:
		return companion.NeedsYou, false
	case life.Dozing:
		return companion.Done, false
	case life.Grooming, life.Digging:
		return companion.Working, false
	case life.Walking:
		return companion.Resting, true
	case life.Wary:
		return companion.Worried, false
	case life.Watching, life.MoonGazing, life.Resting:
		return companion.Resting, false
	}
	return companion.Resting, false
}

// lifeOffset is how far the animal has wandered from home, in cells, clamped to
// the strip the layout already reserved for pacing. The clamp is the whole
// safety of this feature: the sand and the litter are laid out against that
// strip, so a wander that left it would write through the scene's text.
func lifeOffset(mirror bool, span int) int {
	if !lifeLive() {
		return 0
	}
	dx := lifeBrain.Intent().DX
	if mirror {
		// Mirrored: inward is negative, and the strip runs to -span.
		if dx > 0 {
			dx = 0
		}
		if dx < -span {
			dx = -span
		}
		return dx
	}
	if dx < 0 {
		dx = 0
	}
	if dx > span {
		dx = span
	}
	return dx
}

// lifeCoat tints the companion by age: a hatchling is pale, an old friend is
// the deep colour it has always been. The base is the companion's OWN coat —
// the cat stays white, the crab stays coral — so the age ramp never repaints
// a picked companion into someone else's colour. The size comes from the same
// stage table in the companion package (ScaleForStage), so coat and size
// cannot disagree.
func lifeCoat(st life.Stage) term.RGB {
	f := 0.74
	switch st {
	case life.Child:
		f = 0.84
	case life.Teen:
		f = 0.93
	case life.Adult:
		f = 1.0
	}
	base := companion.New(companionPref()).Coat()
	return term.RGB{
		R: uint8(math.Min(255, float64(base.R)*f)),
		G: uint8(math.Min(255, float64(base.G)*f)),
		B: uint8(math.Min(255, float64(base.B)*f)),
	}
}

// drawLifeSpeech puts what the animal is saying on the line under its feet, on
// its own dark ground, the way the earlier creature spoke -- and never in the
// agent's balloon.
func drawLifeSpeech(c *canvas.Canvas, st reduce.State, cat *companion.Cat, catX, top int, t float64) {
	if !lifeLive() {
		return
	}
	in := lifeBrain.Intent()
	line := in.Speech
	if line == "" {
		return
	}
	if len(line) > 40 {
		line = line[:40]
	}
	_, h := cat.Size()
	y := top + h
	if y < 0 || y >= c.H {
		return
	}
	x := catX + cat.HeadCol() - len(line)/2
	if x < 1 {
		x = 1
	}
	if x+len(line) > c.W-1 {
		x = c.W - 1 - len(line)
	}
	if x < 1 {
		return
	}
	col := companion.EyeInk()
	(&companion.Sprite{Rows: []string{line}, Body: col, Opaque: true}).Draw(c.Near(), x, y)
}

// drawLifeSpeechNear puts the animal's line under its own box, or above it when
// the band is in the way. Used where the box is not a Cat's (the vista's owl).
func drawLifeSpeechNear(c *canvas.Canvas, boxX, boxY, bandTop int) {
	if !lifeLive() {
		return
	}
	line := lifeBrain.Intent().Speech
	if line == "" {
		return
	}
	if len(line) > 40 {
		line = line[:40]
	}
	y := boxY + 7
	if bandTop > 0 && y >= bandTop {
		y = boxY - 1
	}
	if y < 0 || y >= c.H {
		return
	}
	x := boxX + 6 - len(line)/2
	if x < 1 {
		x = 1
	}
	if x+len(line) > c.W-1 {
		x = c.W - 1 - len(line)
	}
	if x < 1 {
		return
	}
	(&companion.Sprite{Rows: []string{line}, Body: companion.EyeInk(), Opaque: true}).Draw(c.Near(), x, y)
}

// drawLifeFinds puts the newest discoveries on the sand beside the animal, in
// the strip the layout already reserves for it, so a find is visible without a
// word being read. The finds are the scene's own language: one glyph, the same
// kind of speck the litter already leaves.
func drawLifeFinds(c *canvas.Canvas, cat *companion.Cat, catX, top, span int) {
	if !lifeLive() || span < 1 {
		return
	}
	finds := lifeBrain.Finds()
	if len(finds) == 0 {
		return
	}
	n := len(finds)
	if n > 3 {
		n = 3 // the newest few; a collection belongs in `life finds`
	}
	if n > span {
		n = span
	}
	row := top + 1
	if row < 0 || row >= c.H {
		return
	}
	ink := term.RGB{R: 196, G: 176, B: 132} // bone: dimmer than the coat, brighter than sand
	for i := 0; i < n; i++ {
		f := finds[len(finds)-1-i]
		g, ok := life.FindGlyphs[f]
		if !ok {
			continue
		}
		x := catX - 1 - i
		if x < 1 || x >= c.W-1 {
			continue
		}
		(&companion.Sprite{Rows: []string{string(g)}, Body: ink}).Draw(c.Near(), x, row)
	}
}

// statusString is what `xscapes life` prints for a person: where it is in its
// life, what it is doing, and what it has found.
func lifeStatusString() string {
	if lifeBrain == nil {
		return "no companion"
	}
	st, pct := lifeBrain.Stage()
	a := lifeBrain.Attributes()
	s := st.String() + ", " + lifeNum(int(pct*100)) + "% to " + nextStageName(st)
	if st == life.Adult {
		s = st.String() + ", fully grown"
	}
	p := lifeBrain.Intent().Pose.String()
	return s + " | " + p + " | friends " + lifeNum(int(a.Friendship)) +
		"% | curious " + lifeNum(int(a.Curiosity)) +
		"% | found " + lifeNum(a.Discoveries) +
		" | appetite " + lifeNum(int(a.Appetite)) + "%"
}

// lifeNum is itoa without pulling strconv into a package whose test files
// already use that name for something else.
func lifeNum(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func nextStageName(s life.Stage) string {
	if s+1 >= life.StageCount {
		return "nothing"
	}
	return (s + 1).String()
}
