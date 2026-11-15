package companion

import (
	"fmt"
	"math"
	"strings"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/term"
)

// TamagotchiRenderer draws the creature. It is the same shape as the shipped
// companions' draw path -- assemble a bitmap, breathe it, clear a rim around it,
// plot the quadrant glyphs, then the face on top (cat.go, crab.go) -- with one
// addition: the pose comes from a TamAnimator, so a change of animation is a
// blend rather than a cut, and the idle fidgets happen on their own.
type TamagotchiRenderer struct {
	state *TamagotchiState
	anim  *TamAnimator

	lastT   float64
	started bool

	// cache holds what is a pure function of a small number of integers: the
	// quadrant rows of a blend between two poses, and the face art. It is
	// per-renderer and unlocked on purpose -- a renderer is drawn from one
	// goroutine -- and it exists so that a 12fps scene does not re-cut 28
	// bitmap rows into glyphs sixty times a second or dissolve the same two
	// silhouettes again on every frame of a transition.
	cache map[string][]string
	// bodyBitmapCache holds one silhouette per pose.
	bodyBitmapCache map[string]*Bitmap
}

// tamCacheMax bounds both caches. A creature has a few hundred distinct poses
// over a long session (a squash, a lift and six effect weights), and past that
// the picture is repeating itself: the old entries are dropped wholesale rather
// than tracked, which is cheaper than an LRU and cannot leak.
const tamCacheMax = 600

func (r *TamagotchiRenderer) trimCache() {
	if len(r.cache) >= tamCacheMax {
		r.cache = map[string][]string{}
	}
	if len(r.bodyBitmapCache) >= tamCacheMax {
		r.bodyBitmapCache = map[string]*Bitmap{}
	}
}

// NewTamagotchiRenderer creates a renderer for one creature.
func NewTamagotchiRenderer(state *TamagotchiState) *TamagotchiRenderer {
	seed := int64(1)
	if state != nil {
		seed = int64(state.FedCount*7+state.PlayedCount*13+state.PettedCount*29) + 1
	}
	return &TamagotchiRenderer{
		state:           state,
		anim:            NewTamAnimator(seed),
		cache:           map[string][]string{},
		bodyBitmapCache: map[string]*Bitmap{},
	}
}

// Anim exposes the animator, for a caller that wants to trigger a reaction (a
// hop on a keypress) without going through the mood.
func (r *TamagotchiRenderer) Anim() *TamAnimator { return r.anim }

// Draw renders the creature with its top-left at (x, y). t is the scene clock in
// seconds; the animator advances by the DIFFERENCE between calls, so drawing the
// same instant twice costs nothing and draws the same frame.
func (r *TamagotchiRenderer) Draw(l *canvas.Layer, x, y int, t float64) {
	if r.state == nil {
		return
	}
	// Which animation is wanted. The state is authoritative -- it, and not the
	// renderer, knows whether the creature is asleep or being fed -- and a
	// request for the clip already playing is a no-op inside the animator.
	r.anim.SetBase(tamClipForState(r.state))
	r.anim.FidgetEvery = TempoOf(r.stage()).Fidget

	dt := 0.0
	if r.started {
		dt = t - r.lastT
	}
	r.lastT, r.started = t, true
	r.anim.Advance(dt)

	out, in, w := r.anim.Frames()
	r.drawPose(l, x, y, out, in, w, t)
}

// tamClipForState maps the creature's state onto an animation. Anything the
// state writes to Animation is a clip name already; the rest is the mood
// fallback, so a creature nobody has touched still animates.
func tamClipForState(st *TamagotchiState) string {
	if st == nil {
		return ClipIdle
	}
	if st.Sleeping {
		return ClipSleep
	}
	switch st.Animation {
	case ClipEat, ClipPlay, ClipPet, ClipJoy, ClipSad, ClipSick, ClipSleep:
		return st.Animation
	case "sleepy":
		return ClipSleep
	case "angry", "worried":
		return ClipSad
	case "unwell":
		return ClipSick
	case "happy", "excited":
		return ClipJoy
	}
	switch st.Mood {
	case Sick, Ill:
		return ClipSick
	case Tired:
		return ClipSleep
	case Hungry, Lonely, Melancholy, Furious, Bored:
		return ClipSad
	}
	return ClipIdle
}

// drawPose paints one frame: the body, the face, the overlays. out and in are
// the poses either side of a transition (equal when there is none) and w is how
// far it has run.
func (r *TamagotchiRenderer) drawPose(l *canvas.Layer, x, y int, out, in TamPose, w float64, t float64) {
	p := lerpTamPose(out, in, w) // the continuous half: movement, colour, overlays
	stage := r.stage()
	// The same clip table plays four different animals: a baby's joy is a bigger
	// jump than a child's, and an adult's is smaller than either.
	p = tempoPose(p, TempoOf(stage))

	q := r.spriteRows(out, in, w)
	look := LookOf(stage)
	dy := look.Drop() // a short body hangs from the box's floor, not its top
	fb := look.Face() // the face scales with the head it is on

	plotRim(l, q, x, y+dy)
	(&Sprite{Rows: q, Body: stageCoat(p.Tint, stage)}).Draw(l, x, y+dy)
	r.drawFace(l, x+fb.DX, y+dy+fb.DY, p, out, in, w, fb)
	r.drawOverlays(l, x, y+dy, p, t, fb)
}

// stage is the creature's life stage, and the child is the shape the art was
// drawn at: a renderer attached to no state draws the reference animal.
func (r *TamagotchiRenderer) stage() Stage {
	if r.state == nil {
		return StageChild
	}
	if r.state.Stage < StageBaby || r.state.Stage >= StageCount {
		return StageChild
	}
	return r.state.Stage
}

// tempoPose scales the motion a pose asks for by its stage's tempo. Gaze is not
// scaled: where the creature is LOOKING is not how excited it is.
func tempoPose(p TamPose, tp StageTempo) TamPose {
	if tp.Motion == 1 || tp.Motion <= 0 {
		return p
	}
	p.Lift *= tp.Motion
	p.Sway *= tp.Motion
	p.Lean *= tp.Motion
	p.Squash *= tp.Motion
	return p
}

// stageCoat is the companion's coat with the stage's light or shade blended in:
// a baby is paler than the adult it becomes, an adult deeper than the child it
// was. The hue is still the companion's choice -- the stage only decides how
// much of it has grown in.
func stageCoat(c term.RGB, s Stage) term.RGB {
	ink := InkOf(s)
	if ink.Blend <= 0 {
		return c
	}
	return term.Lerp(c, term.RGB{
		R: clamp255(ink.To[0]), G: clamp255(ink.To[1]), B: clamp255(ink.To[2]),
	}, ink.Blend)
}

func clamp255(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// ---------------------------------------------------------------------------
// THE BODY.
// ---------------------------------------------------------------------------

// spriteRows assembles what is drawn: the upper half for the pose's body,
// squashed and leaning, on the shared lower half, lifted and swayed.
//
// During a transition the two silhouettes are CROSS-FADED, not switched. A swap
// at the halfway point would be a one-frame cut in the middle of a movement,
// which is exactly what a smooth transition is not; the fade is a dithered
// dissolve (see blendBitmaps), which at this resolution is what a dissolve can
// be. The result is cached per (out pose, in pose, fade bucket), so the work is
// done once per visible step and not once per frame.
func (r *TamagotchiRenderer) spriteRows(out, in TamPose, w float64) []string {
	e := smoothstep(w)
	key := "q" + itoa(int(r.stage())) + tamPoseKey(out) + "|" + tamPoseKey(in) + "|" + itoa(int(math.Round(e*12)))
	if rows, ok := r.cache[key]; ok {
		return rows
	}
	bm := r.bodyBitmap(out)
	if e < 1 {
		bm = blendBitmaps(bm, r.bodyBitmap(in), e, tamBlendSeed)
	}
	rows := bm.ToQuadrant()
	r.trimCache()
	r.cache[key] = rows
	return rows
}

// bodyBitmap is one pose's silhouette: the upper half squashed and leaned, on
// the shared lower half, the whole thing lifted and swayed. Cached per pose.
func (r *TamagotchiRenderer) bodyBitmap(p TamPose) *Bitmap {
	key := "s" + itoa(int(r.stage())) + tamPoseKey(p)
	if bm, ok := r.bodyBitmapCache[key]; ok {
		return bm
	}
	body := p.Body
	if body < 0 || body >= BodyCount {
		body = BodyStand
	}
	upper := squashUpper(tamUpperArt[body], int(math.Round(p.Squash)))
	upper = shiftRows(upper, int(math.Round(p.Lean)))
	rows := append(append([]string{}, upper...), tamLowerArt...)

	// The stage's own proportions. Resampling is nearest-neighbour on purpose:
	// the art is a blob of solid ink, and a smooth filter would turn its edge
	// into a smear of half-blocks that reads as a different animal entirely.
	if look := LookOf(r.stage()); look.W != len([]rune(rows[0])) || look.H != len(rows) {
		rows = resizeRows(rows, look.W, look.H)
	}

	src := ParseBitmap(rows)
	f := src.Blank()
	dx := int(math.Round(p.Sway))
	dy := -int(math.Round(p.Lift))
	for sy := 0; sy < f.H; sy++ {
		for sx := 0; sx < f.W; sx++ {
			if src.at(sx+dx, sy+dy) {
				f.Set(sx, sy)
			}
		}
	}
	r.trimCache()
	r.bodyBitmapCache[key] = f
	return f
}

// tamPoseKey is the cache key for a pose: everything that changes the picture,
// at the resolution the picture has (half a source pixel).
func tamPoseKey(p TamPose) string {
	return fmt.Sprintf("%d,%d,%d,%.0f,%.0f,%.0f,%.0f",
		p.Body, p.Eyes, p.Mouth, p.Lift*2, p.Sway*2, p.Lean*2, p.Squash*2)
}

// resizeRows resamples a 1-bit bitmap to another size, nearest neighbour. The
// stage system is built on it: the creature is authored once, at 24x28, and a
// baby is that art at 19x24 while an adult is the same art at 28x30.
func resizeRows(rows []string, w, h int) []string {
	if w < 2 || h < 2 || len(rows) == 0 {
		return rows
	}
	srcH, srcW := len(rows), len([]rune(rows[0]))
	out := make([]string, h)
	for y := 0; y < h; y++ {
		row := []rune(rows[y*srcH/h])
		b := make([]rune, w)
		for x := 0; x < w; x++ {
			b[x] = row[x*srcW/w]
		}
		out[y] = string(b)
	}
	return out
}

// tamBlendSeed makes the dissolve the same one every time: a given pixel flips
// at a given moment on every machine, so a test can assert on it.
const tamBlendSeed = 0x5ca9

// blendBitmaps cross-fades two silhouettes. Pixels the two agree on are ink;
// pixels they differ on are decided by a deterministic hash against w, so the
// incoming shape creeps over the outgoing one across the transition instead of
// replacing it in one frame.
func blendBitmaps(a, b *Bitmap, w float64, seed int64) *Bitmap {
	if w <= 0 {
		return a
	}
	if w >= 1 {
		return b
	}
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := b.Blank()
	for y := 0; y < out.H; y++ {
		for x := 0; x < out.W; x++ {
			switch ao, bo := a.at(x, y), b.at(x, y); {
			case ao == bo:
				if ao {
					out.Set(x, y)
				}
			case bo:
				if HashF(x, y, seed) < w {
					out.Set(x, y)
				}
			case ao:
				if HashF(x, y, seed) >= w {
					out.Set(x, y)
				}
			}
		}
	}
	return out
}

// squashUpper CROUCHES the upper half: the top s rows are emptied and the rest
// is resampled into the rows that are left, so the head comes down toward the
// feet without the picture changing height. This is how sleep, the sigh and the
// eat-lean read as the animal sinking rather than as the animal being cut off.
func squashUpper(rows []string, s int) []string {
	n := len(rows)
	if s <= 0 || n < 4 {
		return rows
	}
	if s > n-2 {
		s = n - 2
	}
	keep := n - s
	blank := strings.Repeat(".", tamBodyCols)
	out := make([]string, n)
	for i := 0; i < s; i++ {
		out[i] = blank
	}
	for i := 0; i < keep; i++ {
		out[s+i] = rows[i*n/keep]
	}
	return out
}

// shiftRows moves every row sideways, in source pixels. Two pixels is one
// character cell, and anything shifted past an edge is dropped rather than
// wrapped.
func shiftRows(rows []string, dx int) []string {
	if dx == 0 {
		return rows
	}
	out := make([]string, len(rows))
	for i, row := range rows {
		rr := []rune(row)
		n := make([]rune, len(rr))
		for j := range n {
			n[j] = '.'
		}
		for j, ch := range rr {
			if k := j + dx; k >= 0 && k < len(n) {
				n[k] = ch
			}
		}
		out[i] = string(n)
	}
	return out
}

// ---------------------------------------------------------------------------
// THE FACE.
// ---------------------------------------------------------------------------

// drawFace puts the eyes and the mouth on the head.
//
// Every cell goes through PlotOn with the coat as its ground, INCLUDING the
// cells whose glyph is a space: canvas.Plot replaces a cell, so an unpainted
// hole would show the sea through the middle of an eye. That is the crab's
// "two blue holes" lesson of 2026-09-04, and it is what makes a ring drawable
// at all at this size.
//
// The eyes and the mouth are a bitmap, not a character, so a change of
// expression is cross-faded like the body and not swapped: the pupil of a
// startled wide eye creeps into the bead of a calm one.
func (r *TamagotchiRenderer) drawFace(l *canvas.Layer, x, y int, p, out, in TamPose, w float64, fb FaceBox) {
	coat := p.Tint
	e := smoothstep(w)
	// Lean moves the head, so the face rides with it; sway and lift move the
	// whole animal.
	dx := int(math.Round((p.Sway + p.Lean) / 2))
	dy := -int(math.Round(p.Lift / 2))
	gazeSrc := int(math.Round(p.Gaze))
	gaze := int(math.Round(p.Gaze / 2)) // cells

	eye := r.faceRows("e", out.Eyes, in.Eyes, e, gazeSrc)
	eyeInk := term.Lerp(tamEyeInk(out.Eyes), tamEyeInk(in.Eyes), e)
	drawFaceArt(l, x+fb.EyeA+dx+gaze, y+fb.EyeRow+dy, eye, eyeInk, coat)
	drawFaceArt(l, x+fb.EyeB+dx+gaze, y+fb.EyeRow+dy, eye, eyeInk, coat)
	// A shine, in the eye's upper left, wherever the eye is open enough to
	// have one. Without it a 2x2-cell eye is a dark slab across the face, which
	// is what the first build of this creature looked like: the difference
	// between an eye and a visor is one bright pixel.
	if sh := tamEyeShineFor(out.Eyes)*(1-e) + tamEyeShineFor(in.Eyes)*e; sh > 0.02 {
		shine := term.Lerp(tamEyeInk(out.Eyes), tamShineInk, 0.6+0.4*sh)
		for _, cx := range []int{fb.EyeA, fb.EyeB} {
			l.Plot(x+cx+dx+gaze, y+fb.EyeRow+dy, '▀', shine, sh)
		}
	}

	ink := term.Lerp(tamMouthInkOf(out.Mouth), tamMouthInkOf(in.Mouth), e)
	drawFaceArt(l, x+fb.MouthCol+dx, y+fb.MouthRow+dy, r.faceRows("m", out.Mouth, in.Mouth, e, 0), ink, coat)

	// The blush is the light shade the rest of the scene uses for texture:
	// narrow (see width.go), on the cheeks beside the mouth.
	if p.Blush > 0.12 {
		a := math.Min(1, p.Blush)
		col := term.Lerp(coat, tamBlushInk, 0.75)
		for _, cx := range []int{fb.CheekA, fb.CheekB} {
			l.Plot(x+cx+dx, y+fb.MouthRow+dy, '░', col, a)
		}
	}
}

// tamMouthInkOf is the ink for a mouth: the inside of an open one is dark, and
// the rest are the muted rose the cat's nose is drawn in.
func tamMouthInkOf(mouth int) term.RGB {
	switch mouth {
	case MouthOpen, MouthWide, MouthChew:
		return tamMouthOpenInk
	}
	return tamMouthInk
}

// faceRows caches the quadrant form of one face bitmap, or of a dissolve
// between two of them, at one gaze offset.
func (r *TamagotchiRenderer) faceRows(kind string, outIdx, inIdx int, e float64, shift int) []string {
	var art [][]string
	limit := EyeCount
	if kind == "e" {
		art = tamEyeArt[:]
	} else {
		art, limit = tamMouthArt[:], MouthCount
	}
	if outIdx < 0 || outIdx >= limit {
		outIdx = 0
	}
	if inIdx < 0 || inIdx >= limit {
		inIdx = outIdx
	}
	key := fmt.Sprintf("f%s%d/%d/%d/%d", kind, outIdx, inIdx, shift, int(math.Round(e*12)))
	if rows, ok := r.cache[key]; ok {
		return rows
	}
	bm := ParseBitmap(shiftRows(art[outIdx], shift))
	if e < 1 && inIdx != outIdx {
		bm = blendBitmaps(bm, ParseBitmap(shiftRows(art[inIdx], shift)), e, tamBlendSeed)
	}
	rows := bm.ToQuadrant()
	r.trimCache()
	r.cache[key] = rows
	return rows
}

// drawFaceArt plots a face bitmap with its own ground, spaces included.
func drawFaceArt(l *canvas.Layer, x, y int, rows []string, ink, coat term.RGB) {
	for dy, row := range rows {
		for dx, ch := range []rune(row) {
			col := ink
			if ch == ' ' {
				col = coat
			}
			l.PlotOn(x+dx, y+dy, ch, col, coat, 1)
		}
	}
}

// ---------------------------------------------------------------------------
// THE OVERLAYS. Each one is a WEIGHT, not a position: where a heart is at a
// given moment is a function of its weight and the clock, so an overlay of a
// blended pose rises smoothly out of one animation into the next.
// ---------------------------------------------------------------------------

func (r *TamagotchiRenderer) drawOverlays(l *canvas.Layer, x, y int, p TamPose, t float64, fb FaceBox) {
	if p.Sparkle > 0.05 {
		a := math.Min(1, p.Sparkle)
		for i, off := range [][2]int{{-1, 1}, {12, 3}, {0, 6}} {
			ph := t*2.4 + float64(i)*2.1
			sx := x + off[0] + int(math.Round(math.Sin(ph)*1.2))
			sy := y + off[1] + int(math.Round(math.Cos(ph*0.8)))
			r.overlay(l, tamSparkleArt, sx, sy, tamSparkleInk, a*(0.6+0.4*math.Abs(math.Sin(ph))))
		}
	}
	if p.Hearts > 0.05 {
		a := math.Min(1, p.Hearts)
		for i := 0; i < 2; i++ {
			ph := t*1.5 + float64(i)*1.7
			rise := int(ph) % 4
			sx := x + 7 + i*3 + int(math.Round(math.Sin(ph*2)))
			sy := y - 2 - rise + i
			r.overlay(l, tamHeartArt, sx, sy, tamHeartInk, a*(1-0.18*float64(rise)))
		}
	}
	if p.Zzz > 0.05 {
		a := math.Min(1, p.Zzz)
		for _, z := range []struct {
			dx, dy int
			w      float64
		}{{10, -1, 0.5}, {12, -2, 0.8}, {14, -3, 1.0}} {
			if a+0.35 < z.w {
				continue
			}
			l.Plot(x+z.dx, y+z.dy, 'z', tamZzzInk, a*0.9)
		}
	}
	if p.Crumbs > 0.05 {
		a := math.Min(1, p.Crumbs)
		drop := int(t*3) % 3
		r.overlay(l, tamCrumbArt, x+fb.MouthCol+2, y+fb.MouthRow+1+drop, tamCrumbInk, a)
	}
	if p.Tear > 0.05 {
		a := math.Min(1, p.Tear)
		drop := int(t*1.2) % 3
		r.overlay(l, tamTearArt, x+fb.EyeA+1, y+fb.EyeRow+2+drop, tamTearInk, a)
	}
}

// overlay plots one small bitmap at a cell offset with a weight.
func (r *TamagotchiRenderer) overlay(l *canvas.Layer, art []string, x, y int, ink term.RGB, a float64) {
	if a <= 0 {
		return
	}
	if a > 1 {
		a = 1
	}
	key := "o" + strings.Join(art, "/")
	rows, ok := r.cache[key]
	if !ok {
		rows = ParseBitmap(art).ToQuadrant()
		if len(r.cache) < 512 {
			r.cache[key] = rows
		}
	}
	for dy, row := range rows {
		for dx, ch := range []rune(row) {
			if ch == ' ' {
				continue
			}
			l.Plot(x+dx, y+dy, ch, ink, a)
		}
	}
}

// ---------------------------------------------------------------------------
// THE CLI VIEW. The same pose, through the same code, so `xscapes tamagotchi`
// shows the picture the scene shows and not a second, hand-kept version of it.
// ---------------------------------------------------------------------------

// GetASCIIArt renders the current pose into a small canvas and reads it back as
// plain glyphs.
func (r *TamagotchiRenderer) GetASCIIArt() string {
	if r.state == nil {
		return ""
	}
	const aw, ah = 18, 11
	c := canvas.New(aw, ah, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	r.anim.SetBase(tamClipForState(r.state))
	r.anim.FidgetEvery = TempoOf(r.stage()).Fidget
	out, in, bw := r.anim.Frames()
	r.drawPose(c.Near(), 3, 1, out, in, bw, r.anim.Clock())

	var lines []string
	for y := 0; y < ah; y++ {
		var b strings.Builder
		for x := 0; x < aw; x++ {
			ch, _, _ := c.ResolveAt(x, y, term.ProfileTrueColor)
			b.WriteRune(ch)
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	// Drop the blank rows so the art sits tight in a terminal.
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return fmt.Sprintf("%s\n%s (%s)", strings.Join(lines, "\n"), r.state.Mood.String(), r.anim.Base())
}

// GetStatusReport returns a text report of the creature's status.
func (r *TamagotchiRenderer) GetStatusReport() string {
	if r.state == nil {
		return "No creature initialized"
	}

	report := fmt.Sprintf("Tamagotchi Status:\n")
	report += fmt.Sprintf("  Mood: %s\n", r.state.Mood.String())
	report += fmt.Sprintf("  Animation: %s\n", r.anim.Base())
	report += fmt.Sprintf("  Hunger: %.0f%%\n", r.state.Hunger)
	report += fmt.Sprintf("  Energy: %.0f%%\n", r.state.Energy)
	report += fmt.Sprintf("  Happiness: %.0f%%\n", r.state.Happiness)
	report += fmt.Sprintf("  Cleanliness: %.0f%%\n", r.state.Cleanliness)
	report += fmt.Sprintf("  Health: %.0f%%\n", r.state.Health)
	report += fmt.Sprintf("  Affection: %.0f%%\n", r.state.Affection)
	report += fmt.Sprintf("  Age: %v\n", r.state.Age)
	report += fmt.Sprintf("  Interactions: Fed=%d, Played=%d, Slept=%d, Petted=%d, Cleaned=%d\n",
		r.state.FedCount, r.state.PlayedCount, r.state.SleptCount,
		r.state.PettedCount, r.state.CleanedCount)

	if r.state.IsEvolving {
		report += fmt.Sprintf("  Evolution Phase: %d\n", r.state.EvolutionPhase)
	}

	return report
}
