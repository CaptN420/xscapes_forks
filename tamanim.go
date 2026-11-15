package main

import (
	"fmt"
	"strings"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/term"
)

// tamAnimPage renders every Tamagotchi animation as a real frame sequence and
// cycles it in the browser, for the same reason animPage exists: a still cannot
// show breathing, and a still certainly cannot show a transition.
//
// It is a STUDY, not the product. The creature in the scene is driven by the
// live loop's clock; this drives the same renderer from a fixed step, so what
// is on the page is what the terminal draws, slowed to something a person can
// look at.
func tamAnimPage(fps float64, frames int) string {
	if fps < 1 {
		fps = 12
	}
	if frames < 4 {
		frames = 48
	}

	var b strings.Builder
	b.WriteString(`<style>
.card{display:flex;gap:26px;align-items:flex-start;padding:16px 18px;margin-bottom:14px;
  background:#141419;border:1px solid #222229;border-radius:8px}
.stage{position:relative}.fr{display:none}.fr:first-child{display:block}
</style>`)
	b.WriteString(`<h1>xscapes &mdash; the creature, animated</h1>`)
	fmt.Fprintf(&b, `<p style="font-size:12px;color:#7d7d8a;max-width:640px">`+
		`%d frames per clip at %.0f fps, driven through the real renderer. `+
		`Every clip starts and ends on the idle pose, so a fidget or a reaction `+
		`can interrupt one without a seam.</p>`, frames, fps)

	type item struct {
		clip string
		note string
	}
	items := []item{
		{companion.ClipIdle, "breathing, and a blink two thirds of the way through"},
		{companion.ClipJoy, "arms up, a two-part hop, sparkles; the happy coat"},
		{companion.ClipSad, "drawn in, antennae gone, a slow slump and a tear"},
		{companion.ClipSleep, "crouched, eyes out, a deep breath and drifting Zs"},
		{companion.ClipEat, "leaning over the food, chewing, crumbs falling"},
		{companion.ClipPlay, "two hops, sparkles, wide eyes"},
		{companion.ClipPet, "leans into the hand, blushes, hearts rise"},
		{companion.ClipSick, "puffed up, squinting, shivering"},
		{companion.ClipBlink, "a fidget"},
		{companion.ClipLook, "a fidget: looking around"},
		{companion.ClipTwitch, "a fidget: a shrug"},
		{companion.ClipSigh, "a fidget: a sigh"},
		{companion.ClipHop, "a fidget: a small hop"},
		{companion.ClipStretch, "a fidget: a stretch"},
	}
	for _, it := range items {
		b.WriteString(`<div class="card"><div class="meta">`)
		fmt.Fprintf(&b, `<div class="nm">%s</div><div class="rg">%d frames</div><div class="nt">%s</div>`,
			it.clip, frames, it.note)
		b.WriteString(`</div><div class="stage">`)
		for _, f := range tamFrames(it.clip, fps, frames) {
			fmt.Fprintf(&b, `<div class="fr">%s</div>`, f)
		}
		b.WriteString(`</div></div>`)
	}

	// The transitions, side by side: the same switch from each of the states a
	// creature spends its time in, so a dissolve can be looked at rather than
	// argued about.
	b.WriteString(`<h1 style="margin-top:34px">transitions</h1>`)
	for _, pr := range [][2]string{
		{companion.ClipIdle, companion.ClipJoy},
		{companion.ClipIdle, companion.ClipSleep},
		{companion.ClipSad, companion.ClipPlay},
		{companion.ClipIdle, companion.ClipPet},
		{companion.ClipPlay, companion.ClipIdle},
	} {
		b.WriteString(`<div class="card"><div class="meta">`)
		fmt.Fprintf(&b, `<div class="nm">%s &rarr; %s</div><div class="rg">%.2f s</div>`+
			`<div class="nt">the two poses are cross-faded in the bitmap: the arms and the `+
			`eyes arrive by dissolve, and everything that can be a number -- the lift, the `+
			`squash, the lean, the coat -- is interpolated.</div>`,
			pr[0], pr[1], companion.BlendDur)
		b.WriteString(`</div><div class="stage">`)
		for _, f := range tamTransitionFrames(pr[0], pr[1], 30, 36) {
			fmt.Fprintf(&b, `<div class="fr">%s</div>`, f)
		}
		b.WriteString(`</div></div>`)
	}

	fmt.Fprintf(&b, `<script>
document.querySelectorAll('.stage').forEach(function(st){
  var fr = Array.prototype.slice.call(st.children), i = 0;
  setInterval(function(){
    fr[i].style.display = 'none';
    i = (i + 1) %% fr.length;
    fr[i].style.display = 'block';
  }, %d);
});
</script>`, int(1000/fps))
	return canvas.HTMLPage("xscapes — the creature, animated", b.String())
}

// tamCanvas builds the little stage the creature is studied on: a dark ground
// with a line under it, so the feet have somewhere to stand.
func tamCanvas() *canvas.Canvas {
	const w, h = 22, 14
	c := canvas.New(w, h, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	ground := term.RGB{R: 28, G: 30, B: 40}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c.SetBG(x, y, ground)
		}
	}
	for x := 0; x < w; x++ {
		c.SetBG(x, h-2, term.RGB{R: 42, G: 46, B: 58})
	}
	return c
}

// tamFrames plays one clip from a settled state and returns the frames.
func tamFrames(clip string, fps float64, n int) []string {
	v := companion.NewTamagotchiState()
	st := &v
	r := companion.NewTamagotchiRenderer(st)
	st.Animation = clip
	st.AnimLeft = 0
	st.Sleeping = clip == companion.ClipSleep

	// Warm up: the clip's own first second runs before the first captured
	// frame, so the opening transition is not what the card shows.
	t := 0.0
	for i := 0; i < int(fps*1.2); i++ {
		t += 1 / fps
		c := tamCanvas()
		r.Draw(c.Near(), 4, 4, t)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		t += 1 / fps
		c := tamCanvas()
		r.Draw(c.Near(), 4, 4, t)
		out = append(out, c.HTMLFragment(20))
	}
	return out
}

// tamTransitionFrames captures the switch itself: the last moments of the first
// clip, then the whole of the fade, then the first moments of the second.
func tamTransitionFrames(from, to string, fps float64, n int) []string {
	v := companion.NewTamagotchiState()
	st := &v
	r := companion.NewTamagotchiRenderer(st)
	play := func(clip string) {
		st.Sleeping = clip == companion.ClipSleep
		st.Animation = clip
		st.AnimLeft = 0
	}
	play(from)
	t := 0.0
	for i := 0; i < int(fps*1.5); i++ {
		t += 1 / fps
		c := tamCanvas()
		r.Draw(c.Near(), 4, 4, t)
	}
	play(to)
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		t += 1 / fps
		c := tamCanvas()
		r.Draw(c.Near(), 4, 4, t)
		out = append(out, c.HTMLFragment(20))
	}
	return out
}
