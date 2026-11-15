package companion

import (
	"math"
	"strings"
	"testing"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/term"
)

// ---------------------------------------------------------------------------
// THE ART.
// ---------------------------------------------------------------------------

// A ragged bitmap renders as a subtle shear rather than as an error, so the
// shapes are checked before anything can draw them.
func TestTamagotchiArtIsWellFormed(t *testing.T) {
	for i, rows := range tamUpperArt {
		if len(rows) != tamUpperRows {
			t.Errorf("upper art %d: %d rows, want %d", i, len(rows), tamUpperRows)
		}
		for y, row := range rows {
			if n := len([]rune(row)); n != tamBodyCols {
				t.Errorf("upper art %d row %d: %d runes, want %d", i, y, n, tamBodyCols)
			}
		}
	}
	if len(tamLowerArt) != tamBodyRows-tamUpperRows {
		t.Errorf("lower art: %d rows, want %d", len(tamLowerArt), tamBodyRows-tamUpperRows)
	}
	for y, row := range tamLowerArt {
		if n := len([]rune(row)); n != tamBodyCols {
			t.Errorf("lower art row %d: %d runes, want %d", y, n, tamBodyCols)
		}
	}
	// Every pose must be a DIFFERENT animal, or the animations are invisible.
	// The old tamagotchi bitmaps were five copies of one blob, which is why
	// nothing it did could be seen.
	for i := 1; i < len(tamUpperArt); i++ {
		if strings.Join(tamUpperArt[i], "\n") == strings.Join(tamUpperArt[0], "\n") {
			t.Errorf("upper art %d is identical to the stand pose", i)
		}
	}
	// An eye is 4 source px by 8: two cells wide, two tall. A mouth is 4x4.
	for i, art := range tamEyeArt {
		checkArt(t, "eye", i, art, 4, 8)
	}
	for i, art := range tamMouthArt {
		checkArt(t, "mouth", i, art, 4, 4)
	}
	checkArt(t, "heart", 0, tamHeartArt, 6, 6)
	checkArt(t, "sparkle", 0, tamSparkleArt, 6, 6)
	checkArt(t, "tear", 0, tamTearArt, 4, 4)
	checkArt(t, "crumb", 0, tamCrumbArt, 4, 4)
	// And the eyes must differ from each other: nine copies of one dot would
	// pass the checks above and carry no expression at all.
	for i := 1; i < len(tamEyeArt); i++ {
		if strings.Join(tamEyeArt[i], "") == strings.Join(tamEyeArt[0], "") &&
			tamEyeInk(i) == tamEyeInk(0) {
			t.Errorf("eye %d is identical to eye 0, in shape and in ink", i)
		}
	}
}

func checkArt(t *testing.T, what string, i int, art []string, w, h int) {
	t.Helper()
	if len(art) != h {
		t.Errorf("%s %d: %d rows, want %d", what, i, len(art), h)
	}
	for y, row := range art {
		if n := len([]rune(row)); n != w {
			t.Errorf("%s %d row %d: %d runes, want %d", what, i, y, n, w)
		}
		for _, r := range row {
			if r != '#' && r != '.' {
				t.Errorf("%s %d row %d: unexpected rune %q", what, i, y, r)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// THE CLIPS.
// ---------------------------------------------------------------------------

// Each animation must be visibly its own thing at the same phase -- otherwise
// the creature is expressing one mood with five names.
func TestEveryClipShowsSomething(t *testing.T) {
	idle := TamClipByName(ClipIdle).poseAt(0.1)
	for _, name := range []string{ClipJoy, ClipSad, ClipSleep, ClipEat, ClipPlay, ClipPet, ClipSick} {
		c := TamClipByName(name)
		if c.Name != name {
			t.Fatalf("clip %q not found, got %q", name, c.Name)
		}
		distinct := false
		for k := 0; k < 8; k++ {
			p := c.poseAt(float64(k) / 8)
			if tamDistance(p, idle) > 0.4 {
				distinct = true
				break
			}
		}
		if !distinct {
			t.Errorf("clip %q never differs from idle", name)
		}
	}
}

// tamDistance is a rough difference between two poses, weighted so that a
// change of body or face counts for as much as a large movement. It is used
// only by the tests, as a proxy for "would a person see this".
func tamDistance(a, b TamPose) float64 {
	d := math.Abs(a.Lift-b.Lift)/4 + math.Abs(a.Squash-b.Squash)/4 +
		math.Abs(a.Lean-b.Lean)/4 + math.Abs(a.Sway-b.Sway)/4 +
		math.Abs(a.Gaze-b.Gaze)/4 + math.Abs(a.Blush-b.Blush) +
		math.Abs(a.Sparkle-b.Sparkle) + math.Abs(a.Hearts-b.Hearts) +
		math.Abs(a.Zzz-b.Zzz) + math.Abs(a.Crumbs-b.Crumbs) + math.Abs(a.Tear-b.Tear)
	if a.Body != b.Body {
		d += 0.6
	}
	if a.Eyes != b.Eyes {
		d += 0.6
	}
	if a.Mouth != b.Mouth {
		d += 0.4
	}
	return d
}

// Clips loop seamlessly, or they snap: a looping clip whose last key is not its
// first drops the creature back to the start of the animation in one frame.
func TestLoopingClipsEndWhereTheyBegin(t *testing.T) {
	for i := range TamClips {
		c := &TamClips[i]
		if !c.Loop {
			continue
		}
		first := c.poseAt(0)
		last := c.poseAt(0.9999)
		if d := tamDistance(first, last); d > 0.35 {
			t.Errorf("clip %q: loop seam distance %.2f", c.Name, d)
		}
	}
}

// ---------------------------------------------------------------------------
// THE ANIMATOR.
// ---------------------------------------------------------------------------

// step runs an animator for n frames at dt and returns the poses it produced.
func step(a *TamAnimator, dt float64, n int) []TamPose {
	out := make([]TamPose, 0, n)
	for i := 0; i < n; i++ {
		a.Advance(dt)
		out = append(out, a.Pose())
	}
	return out
}

func maxStep(poses []TamPose) float64 {
	m := 0.0
	for i := 1; i < len(poses); i++ {
		if d := tamDistance(poses[i-1], poses[i]); d > m {
			m = d
		}
	}
	return m
}

// THE POINT OF THE WHOLE FILE: switching animation must not be a cut.
//
// Measured at the PIXEL, not at the pose, because the parts of a pose that
// cannot be interpolated (which body, which eye) are exactly the parts a
// pose-level test cannot see.
//
// The measure is how the distance from the outgoing frame is COVERED. A cut
// covers all of it in one frame; a fade covers it a slice at a time. Whole-cell
// steps are allowed and expected -- the sprite is a character grid, and a hop
// moves it a cell at a time, exactly as it moves the cat (see cat.go's lift) --
// so the assertion is a share of the total, not a flat cell count.
func TestTransitionsNeverJump(t *testing.T) {
	pairs := [][2]string{
		{ClipIdle, ClipJoy}, {ClipIdle, ClipSleep}, {ClipJoy, ClipSad},
		{ClipIdle, ClipPet}, {ClipEat, ClipIdle}, {ClipPlay, ClipIdle},
		{ClipSleep, ClipJoy}, {ClipIdle, ClipSick}, {ClipSad, ClipPlay},
	}
	for _, pr := range pairs {
		v := NewTamagotchiState()
		st := &v
		r := NewTamagotchiRenderer(st)
		play := func(clip string) { // pin the state on one animation
			st.Sleeping = clip == ClipSleep
			st.Animation = clip
			st.AnimLeft = 1e9
		}
		tm := 0.0
		step := func(n int) []float64 {
			ts := make([]float64, 0, n)
			for i := 0; i < n; i++ {
				tm += 1.0 / 60
				ts = append(ts, tm)
			}
			return ts
		}

		// Settle on the first clip and take its frame as the origin.
		play(pr[0])
		for _, f := range step(150) {
			tamRegion(r, f)
		}
		origin := tamRegion(r, tm)

		// Switch, and watch the frames of a fade go by. The window is exactly
		// one transition, so the hops of the incoming clip do not drown it.
		// (The frames are kept as they are drawn: the renderer's clock only
		// goes forward, which is the point of it.)
		play(pr[1])
		regions := make([][]rune, 0, int(BlendDur*60)+1)
		for _, f := range step(int(BlendDur*60) + 1) {
			regions = append(regions, tamRegion(r, f))
		}
		moved := 0
		prev := origin
		for _, cur := range regions {
			if tamDiff(prev, cur) > 0 {
				moved++
			}
			prev = cur
		}
		total := tamDiff(origin, prev)
		if moved < 5 {
			t.Errorf("%s -> %s: only %d frames of a %d-frame fade changed anything",
				pr[0], pr[1], moved, len(regions))
		}
		// The floor is low on purpose: the smallest real transition here is
		// eat -> idle, which shares a lower half and only unbows the head, and
		// that moves about 18 cells.
		if total < 12 {
			t.Errorf("%s -> %s: the picture barely changed (%d cells)", pr[0], pr[1], total)
		}
		// The first frame of a 0.30 s fade is 1/60 s in, so it may not carry
		// more than a quarter of what the whole fade will end up doing. A cut
		// carries all of it on frame one.
		first := tamDiff(origin, regions[0])
		if share := float64(first) / float64(total); share > 0.25 {
			t.Errorf("%s -> %s: the first frame of the switch already covered %.0f%% of the change",
				pr[0], pr[1], share*100)
		}
	}
}

// The dissolve itself, without a sprite's cell-step confound: two silhouettes
// that differ, faded in 18 steps the way a 0.30 s fade runs at 60 fps. Each
// step may flip a fraction of the differing pixels -- never all of them, which
// is what a swap at the halfway point would do.
func TestTheFadeIsGradual(t *testing.T) {
	// BodyCurl against BodyArms: no antennae against arms out, the largest
	// difference two poses have.
	a := ParseBitmap(append(append([]string{}, tamUpperArt[BodyCurl]...), tamLowerArt...))
	b := ParseBitmap(append(append([]string{}, tamUpperArt[BodyArms]...), tamLowerArt...))
	diffPixels := func(x, y *Bitmap) int {
		n := 0
		for i := range x.On {
			if x.On[i] != y.On[i] {
				n++
			}
		}
		return n
	}
	total := diffPixels(a, b)
	if total < 40 {
		t.Fatalf("the two bodies differ by only %d pixels: nothing to fade", total)
	}
	prev := a
	worst := 0
	for i := 1; i <= 18; i++ {
		cur := blendBitmaps(a, b, float64(i)/18, tamBlendSeed)
		if d := diffPixels(prev, cur); d > worst {
			worst = d
		}
		prev = cur
	}
	// A perfect fade flips total/18 of the pixels per step; the hash dither is
	// uneven, so allow triple -- but not the whole set.
	if worst > total/3 {
		t.Errorf("one step of the fade flipped %d of %d differing pixels", worst, total)
	}
	if worst < total/40 {
		t.Errorf("the fade is not moving: %d of %d pixels per step", worst, total)
	}
	if last := diffPixels(prev, b); last != 0 {
		t.Errorf("the fade did not arrive: %d pixels still differ at the end", last)
	}
}

// tamRegion renders the creature's box and returns its glyphs, so two frames
// can be compared without depending on the scene around them.
func tamRegion(r *TamagotchiRenderer, t float64) []rune {
	c := canvas.New(80, 24, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	r.Draw(c.Near(), 6, 8, t)
	out := make([]rune, 0, 18*14)
	for y := 5; y < 19; y++ {
		for x := 4; x < 22; x++ {
			ch, _, _ := c.ResolveAt(x, y, term.ProfileTrueColor)
			out = append(out, ch)
		}
	}
	return out
}

func tamDiff(a, b []rune) int {
	n := 0
	for i := range a {
		if i < len(b) && a[i] != b[i] {
			n++
		}
	}
	return n
}

// The transition is a fade over a known duration, and it finishes.
func TestTransitionRunsAndSettles(t *testing.T) {
	a := NewTamAnimator(3)
	for i := 0; i < 40; i++ {
		a.Advance(1.0 / 60)
	}
	if a.Transitioning() {
		t.Fatal("idle never settled")
	}
	a.SetBase(ClipJoy)
	if !a.Transitioning() {
		t.Fatal("the switch to joy did not open a transition")
	}
	seen := 0
	for i := 0; i < int(BlendDur*60)+3; i++ {
		a.Advance(1.0 / 60)
		if a.Blend() > 0 && a.Blend() < 1 {
			seen++
		}
	}
	if seen < int(BlendDur*60)-4 {
		t.Errorf("blend passed through only %d frames", seen)
	}
	if a.Transitioning() || a.Clip() != ClipJoy {
		t.Errorf("transition did not settle on joy (clip %s, blend %.2f)", a.Clip(), a.Blend())
	}
}

// A one-shot clip that the STATE is holding keeps its last frame: the state
// owns that hold (TamagotchiState.AnimLeft), and an animator that gave the
// creature back on its own would fight it.
func TestOneShotBaseClipsHoldTheirLastFrame(t *testing.T) {
	for _, name := range []string{ClipEat, ClipPlay, ClipPet} {
		a := NewTamAnimator(11)
		a.SetBase(name)
		for i := 0; i < 40; i++ { // 4 s, longer than any of these clips
			a.Advance(0.1)
		}
		if a.Clip() != name || a.Base() != name {
			t.Errorf("%s: clip %q base %q", name, a.Clip(), a.Base())
		}
		last := TamClipByName(name).poseAt(1)
		if d := tamDistance(a.Pose(), last); d > 0.05 {
			t.Errorf("%s held pose differs from its last key by %.2f", name, d)
		}
		if !a.Transitioning() == false {
			t.Errorf("%s: still transitioning long after it finished", name)
		}
		// Handing the creature back is a transition, not a cut.
		a.SetBase(ClipIdle)
		if a.Clip() != ClipIdle || !a.Transitioning() {
			t.Errorf("%s -> idle: clip %q, transitioning %v", name, a.Clip(), a.Transitioning())
		}
	}
}

// Fidgets happen, they are short, and they never run over a held animation.
func TestIdleFidgetsHappenAndNeverInterrupt(t *testing.T) {
	a := NewTamAnimator(5)
	seen := map[string]int{}
	for i := 0; i < 30*180; i++ { // three minutes at 30 fps
		a.Advance(1.0 / 30)
		if c := a.Clip(); c != ClipIdle {
			seen[c]++
			if c == ClipEat || c == ClipPlay || c == ClipPet {
				t.Fatalf("a fidget played the real animation %q", c)
			}
		}
	}
	if len(seen) < 4 {
		t.Errorf("only %d kinds of fidget in three idle minutes: %v", len(seen), seen)
	}
	for _, f := range TamFidgets {
		if seen[f] == 0 {
			t.Errorf("fidget %q never ran in three idle minutes", f)
		}
	}

	// Held: nothing may interrupt the eat.
	b := NewTamAnimator(5)
	b.SetBase(ClipEat)
	for i := 0; i < 30*3; i++ {
		b.Advance(1.0 / 30)
		if c := b.Clip(); c != ClipEat && c != ClipIdle {
			t.Fatalf("a fidget (%s) interrupted a held animation", c)
		}
	}
}

// ---------------------------------------------------------------------------
// THE CREATURE'S OWN CLOCK.
// ---------------------------------------------------------------------------

// A held animation is not overwritten by the mood, and it ends on time. This is
// the bug that made every animation invisible: Decay ran UpdateMood every frame
// and put the face straight back to idle.
func TestHeldAnimationsSurviveTheMoodPass(t *testing.T) {
	v := NewTamagotchiState()
	st := &v
	st.Hunger, st.Energy, st.Happiness = 80, 80, 80
	if got := st.Feed(); !got.Success {
		t.Fatal("the creature refused to eat")
	}
	if st.Animation != ClipEat {
		t.Fatalf("fed, but the animation is %q", st.Animation)
	}
	for i := 0; i < 20; i++ {
		st.Decay(0.05)
		if st.Animation != ClipEat {
			t.Fatalf("the mood overwrote the eating animation after %d frames", i+1)
		}
	}
	for i := 0; i < 100; i++ {
		st.Tick(0.05)
	}
	if st.AnimLeft != 0 || st.Animation != ClipIdle {
		t.Errorf("the hold did not release: left %.2f, animation %q", st.AnimLeft, st.Animation)
	}
}

// Sleep is a mode, not a frame: it lasts until it is woken.
func TestSleepLastsUntilWoken(t *testing.T) {
	v := NewTamagotchiState()
	st := &v
	st.Energy = 40
	if got := st.Sleep(); !got.Success || !st.Sleeping {
		t.Fatal("the creature did not go to sleep")
	}
	for i := 0; i < 200; i++ {
		st.Decay(0.05)
		st.Tick(0.05)
		if !st.Sleeping || st.Animation != ClipSleep {
			t.Fatalf("sleep was lost after %d frames (animation %q)", i+1, st.Animation)
		}
	}
	if got := st.Feed(); got.Success {
		t.Error("it ate in its sleep")
	}
	st.WakeUp()
	if st.Sleeping || st.Animation != ClipIdle {
		t.Errorf("waking left it asleep=%v animation=%q", st.Sleeping, st.Animation)
	}
}

// Petted, it shows the affection reaction -- and only then.
func TestPetPlaysTheAffectionReaction(t *testing.T) {
	v := NewTamagotchiState()
	st := &v
	if got := st.Pet(); !got.Success {
		t.Fatal("the creature refused to be petted")
	}
	if st.Animation != ClipPet {
		t.Fatalf("petted, but the animation is %q", st.Animation)
	}
	if clip := tamClipForState(st); clip != ClipPet {
		t.Errorf("the renderer would play %q, not the pet", clip)
	}
}

// ---------------------------------------------------------------------------
// DRAWING.
// ---------------------------------------------------------------------------

// drawTamagotchiFrame paints one frame of a pose into a canvas the size the
// scene uses.
func drawTamagotchiFrame(t *testing.T, st *TamagotchiState, t0 float64, r *TamagotchiRenderer) *canvas.Canvas {
	t.Helper()
	c := canvas.New(80, 24, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	r.Draw(c.Near(), 4, 6, t0)
	return c
}

// Nothing the creature draws may be a wide rune: one two-cell glyph shears
// every row after it (see width.go).
func TestEveryGlyphTheCreatureDrawsIsSafe(t *testing.T) {
	v := NewTamagotchiState()
	st := &v
	r := NewTamagotchiRenderer(st)
	clips := []string{ClipIdle, ClipJoy, ClipSad, ClipSleep, ClipEat, ClipPlay, ClipPet, ClipSick}
	for _, name := range clips {
		st.Sleeping = name == ClipSleep
		st.Animation = name
		st.AnimLeft = 10
		for _, tm := range []float64{0.4, 0.9, 1.4, 2.2} {
			c := drawTamagotchiFrame(t, st, tm, r)
			for y := 0; y < c.H; y++ {
				for x := 0; x < c.W; x++ {
					ch, _, _ := c.ResolveAt(x, y, term.Profile256)
					if ch == ' ' {
						continue
					}
					if !isNarrow(ch) && !isBlockSafe(ch) {
						t.Fatalf("clip %s drew %q at %d,%d, which is not a verified narrow rune",
							name, ch, x, y)
					}
				}
			}
		}
	}
}

// The same instant draws the same frame: the animation is a function of the
// clock, so a test, a screenshot and the live loop agree.
func TestTheSameInstantDrawsTheSameFrame(t *testing.T) {
	v := NewTamagotchiState()
	st := &v
	one := NewTamagotchiRenderer(st)
	two := NewTamagotchiRenderer(&v)
	a := drawTamagotchiFrame(t, st, 1.25, one).RenderPlain()
	b := drawTamagotchiFrame(t, st, 1.25, two).RenderPlain()
	if a != b {
		t.Error("two renderers at the same instant disagree")
	}
}

// A sleep animation must actually put the eyes out, and a joy must not: a POSE
// check, so that a future edit to the clips cannot quietly make them the same.
func TestSleepShutsTheEyesAndJoyDoesNot(t *testing.T) {
	sleep := TamClipByName(ClipSleep)
	joy := TamClipByName(ClipJoy)
	sawShut := false
	for k := 0; k < 10; k++ {
		if sleep.poseAt(float64(k)/10).Eyes == EyeShut {
			sawShut = true
		}
		if joy.poseAt(float64(k)/10).Eyes == EyeShut {
			t.Error("joy blinks like sleep")
		}
	}
	if !sawShut {
		t.Error("sleep never shuts the creature's eyes")
	}
}
