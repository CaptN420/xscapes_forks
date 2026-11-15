package main

import (
	"math"
	"strings"
	"time"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/envx"
	"github.com/donlucasx/xscapes/internal/event"
	"github.com/donlucasx/xscapes/internal/notify"
	"github.com/donlucasx/xscapes/internal/reduce"
	"github.com/donlucasx/xscapes/internal/scape"
	"github.com/donlucasx/xscapes/internal/scenes"
	"github.com/donlucasx/xscapes/internal/spend"
	"github.com/donlucasx/xscapes/internal/term"
)

// frames paints the scape one frame at a time.
//
// It exists because there are now two places that need a frame -- the pane
// that owns its whole window, and the band below a hosted agent -- and a
// scene assembled twice is a scene that drifts. Everything about what the
// scape looks like lives here; the callers only decide where to put it.
type frames struct {
	// spend sums the tokens off the agent's transcript, polled every two
	// seconds from state(); the counter in the top-right corner.
	spend   *spend.Tally
	spendAt time.Time

	c        *canvas.Canvas
	sh       *scape.Shore
	cat      *companion.Cat
	lay      layout
	ccw, chh int
	mirror   bool
	profile  term.Profile
	seed     int64

	// Set once a session is found. Until then the demo cycle runs.
	bus     *event.Bus
	red     *reduce.Reducer
	player  *notify.Player
	knocker notify.Knocker

	ctxUsed, tod float64
	// level pins the demo at one activity level when >= 0 (-live -level):
	// the instrument for looking at the scape at full stretch with no
	// agent in the loop (his A/B on the wind, 2026-09-18). -1 is the cycle.
	level float64
	start time.Time

	// nextCompanionCheck paces refreshCompanion; see it for why this is not
	// read every frame.
	nextCompanionCheck time.Time

	// lastFrame is when the previous frame was painted, and it exists only to
	// give Approach a real dt. It cannot be derived from the frame rate: -fps
	// is a flag, the two launchers disagree about it already (12 in inside.go,
	// 20 in main.go), and a frame that arrives late after a resize or a
	// compaction would otherwise advance the walk by the wrong amount. Zero on
	// the first frame, which yields dt = 0 and no movement.
	lastFrame time.Time

	// The scape. sh is always built -- the shore is the default and the
	// demo, the site and the mockups draw it -- and vista is set while the
	// preference names it (refreshScape). ascii is kept so a switch can
	// build the vista the same way the shore was.
	vista      *scenes.Vista
	harbour    *scenes.Harbour
	harbourCat *companion.Cat
	// knocked, when set, hears every knock the frame rings (tests: which
	// sound would play, without playing it).
	knocked        func(notify.Kind)
	scapeName      string
	ascii          bool
	nextScapeCheck time.Time
	// rainView is the current rain clip index (-1 means no rain). When the
	// scape switches to ScapeRain, refreshScape cycles through the three
	// views; when it switches away, this resets to -1 so the shore path
	// is taken again.
	rainView int
	// evlog is XSCAPES_EVENTLOG, the file every applied event is appended
	// to (eventlog.go); evDropped/evBad are the bus counts last written.
	evlog            string
	evDropped, evBad int64
	// erases reports the agent's erase-displays the host confined, for the
	// same log line as the bus counts; nil when nothing hosts an agent.
	erases   func() int64
	evErased int64

	// Tamagotchi: optional virtual creature overlay.
	// life is the companion's own living (internal/life). It is the default
	// companion; tamagotchi below is the earlier, separate creature and is now
	// opt-in legacy, so one animal is on screen unless someone asks for both.
	life       bool
	tamagotchi bool
}

// companionPoll is how often a running scape re-reads the saved companion.
// Half a second is under the threshold where a switch feels like it did not
// work, and it is 24x less file reading than doing it every frame.
const companionPoll = 500 * time.Millisecond

// refreshCompanion swaps the animal when the saved preference changes, so
// `xscapes companion cat` takes effect in a scape that is already running.
//
// It did not, and the CLI and the README both promised it did (2026-09-08, he
// switched to the cat and the crab stayed on the beach). companionPref() was
// read exactly once, inside newFrames, which runs at startup; even resize()
// recomputed the layout from the companion width it already had.
//
// The layout is recomputed here exactly as newFrames and resize do it. Today
// that is a no-op: both animals report the same Size(), because Size() returns
// the BOX and not the ink, and the cat's ink uses only nine of its twelve
// columns. It is done anyway because compose() takes the companion's width and
// places the companion's margin, the litter's room and the moon's column from
// it -- a companion with a different box would otherwise be drawn into the old
// one's hole, and nothing in the swap would say so.
func (f *frames) refreshCompanion(now time.Time) {
	if now.Before(f.nextCompanionCheck) {
		return
	}
	f.nextCompanionCheck = now.Add(companionPoll)
	name := companionPref()
	if name == f.cat.Name() {
		return
	}
	c := companion.New(name)
	c.FaceLeft(f.mirror)
	f.cat = c
	f.ccw, f.chh = c.Size()
	f.lay = compose(f.c.W, f.ccw, f.mirror)
	f.sh.MoonX = f.lay.MoonX
}

// isOn reads the ways a person writes "yes" to an environment switch.
func isOn(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "1", "true", "yes":
		return true
	}
	return false
}

// isOff reads the ways a person writes "no" to an environment switch.
func isOff(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "off", "0", "false", "no", "none":
		return true
	}
	return false
}

func newFrames(w, h int, seed int64, ascii, mirror bool, ctxUsed, tod float64) *frames {
	c := canvas.New(w, h, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	sh := scape.NewShore(seed, ascii)
	cat := companion.New(companionPref())
	cat.FaceLeft(mirror)
	ccw, chh := cat.Size()
	lay := compose(w, ccw, mirror)
	sh.MoonX = lay.MoonX
	f := &frames{
		c: c, sh: sh, cat: cat, lay: lay, ccw: ccw, chh: chh,
		mirror: mirror, profile: term.DetectProfile(), seed: seed,
		player: notify.New(), ctxUsed: ctxUsed, tod: tod, level: -1, start: time.Now(),
		ascii: ascii, scapeName: ScapeShore,
		evlog: logPath(envx.Lookup("EVENTLOG"), "events.jsonl"),
		// The old creature is now opt-in: two companions was one too many.
		tamagotchi: isOn(envx.Lookup("TAMAGOTCHI")),
		life:       !isOff(envx.Lookup("LIFE")),
		rainView:   -1,
	}
	f.refreshScape(time.Time{})
	// refreshScape returns early when nothing changed, which it always is at
	// start-up, so the spectators are started here as well as on a switch.
	initBanter(f)
	return f
}

// refreshScape re-reads the scape preference on the companion's clock, so
// `xscapes scape vista` reaches a running scape without a restart -- the
// same defect the companion switch had until 2026-09-08, not repeated.
func (f *frames) refreshScape(now time.Time) {
	if !now.IsZero() && now.Before(f.nextScapeCheck) {
		return
	}
	f.nextScapeCheck = now.Add(companionPoll)
	name := scapePref()
	if name == f.scapeName && (name == ScapeShore || f.vista != nil || f.harbour != nil) {
		return
	}
	f.scapeName = name
	f.vista, f.harbour = nil, nil
	switch name {
	case ScapeVista:
		f.vista = scenes.NewVista(f.seed, f.ascii)
		f.rainView = -1
	case ScapeHarbour:
		// The harbour's companion is the cat, whatever the shore's is: it
		// sits on the sill and walks up one rung.
		f.harbour = scenes.NewHarbour(f.seed, f.ascii)
		f.harbourCat = companion.New(companion.NameCat)
		f.harbourCat.FaceLeft(f.mirror)
		f.harbourCat.MaxRung = 1
		f.rainView = -1
	case ScapeRain:
		// Cycle through the three rain views each time we switch in.
		f.rainView = (f.rainView + 1) % 3
	default:
		// Switching away from rain resets the clip index so the shore path
		// is taken again on the next frame.
		f.rainView = -1
	}
	// The spectators are whoever belongs to the world that is now showing, and
	// the escalation ladder starts fresh with the scenery.
	initBanter(f)
}

// resize refits the scene, and says whether the window got smaller -- the only
// case that needs the terminal cleared, since growing paints strictly more
// cells than the frame before it.
func (f *frames) resize(w, h int) (shrank bool) {
	shrank = w < f.c.W || h < f.c.H
	f.c.Resize(w, h)
	f.lay = compose(w, f.ccw, f.mirror)
	f.sh.MoonX = f.lay.MoonX
	return shrank
}

func (f *frames) size() (w, h int) { return f.c.W, f.c.H }

// follow attaches the scene to a session's event bus.
func (f *frames) follow(bus *event.Bus, red *reduce.Reducer) { f.bus, f.red = bus, red }

func (f *frames) following() bool { return f.red != nil }

// state drains everything that has arrived since the last frame and returns
// what the scene should show now. Non-blocking: a frame must never wait on
// the bus.
func (f *frames) state(now time.Time, t float64) reduce.State {
	if f.red == nil {
		// The sky is the world: time of day is the wall clock whether or not
		// there is a session to follow. Without this the demo ran at tod 0 --
		// midnight, which is deliberately monochrome -- so every launch showed
		// a black-and-white scene for the second or two before the agent
		// announced itself, and then jumped to colour.
		tod := f.tod
		if tod == 0 {
			tod = timeOfDay(now)
		}
		if f.level >= 0 {
			return pinnedState(f.level, f.ctxUsed, tod)
		}
		return demoState(t, f.ctxUsed, tod)
	}
	// A reducer with no bus is fed by the launcher itself: the generic
	// adapter (internal/watch) applies what it synthesised before each
	// frame, so there is nothing here to drain.
	for draining := f.bus != nil; draining; {
		select {
		case e := <-f.bus.C:
			// The spectators watch the same events the water does.
			feedBanter(e, now)
			// A command typed in another terminal reaches the companion here,
			// so the animal reacts in this frame rather than in the file the
			// next process reads.
			if handleLifeEvent(e) {
				break
			}
			// Handle Tamagotchi events first
			if handleTamagotchiEvent(e) {
				break
			}
			f.red.Apply(e, now)
			if f.evlog != "" {
				appendEventLog(f.evlog, e, now)
			}
		default:
			draining = false
		}
	}
	if f.evlog != "" && f.bus != nil {
		dropped, bad := f.bus.Stats()
		var erased int64
		if f.erases != nil {
			erased = f.erases()
		}
		if dropped != f.evDropped || bad != f.evBad || erased != f.evErased {
			f.evDropped, f.evBad, f.evErased = dropped, bad, erased
			appendEventLogStats(f.evlog, dropped, bad, erased, now)
		}
	}
	st := f.red.State(now)

	// Update Tamagotchi state if enabled (legacy, opt-in)
	if f.tamagotchi && tamagotchiState != nil {
		updateTamagotchi(now, &st)
	}

	// The spend, summed off the transcript every two seconds (his ask of
	// 2026-09-16, drawTokens). An adapter that sends a figure of its own
	// wins; the Claude adapter sends none, its payload has no such total.
	if now.Sub(f.spendAt) >= 2*time.Second {
		if f.spend == nil {
			f.spend = spend.New()
		}
		f.spend.Poll(st.Transcript)
		f.spendAt = now
	}
	if st.Act.Tokens == 0 && f.spend != nil {
		st.Act.Tokens = f.spend.Total()
	}
	// The moon, from the transcript, when no status line reported it. Kimi's
	// wire records the context in use and the window (internal/spend/kimi.go);
	// Claude Code's transcript does not, and its status line does.
	if !st.ContextKnown && f.spend != nil {
		if used, window, ok := f.spend.Context(); ok {
			frac := float64(used) / float64(window)
			st.Act.ContextUsed = math.Max(0, math.Min(1, frac))
			st.Act.Window = window
		}
	}
	// Re-render the sand to the columns this layout actually leaves it, so a
	// narrow pane loses whole pieces of a line rather than getting a path
	// chopped mid-word. The vista's band runs the whole width.
	budget := f.lay.SandTo - f.lay.SandFrom
	if f.vista != nil || f.harbour != nil {
		budget = f.c.W - 4
	}
	st.Tail = st.FitTail(now, budget)
	// The sky is the world: time of day is the wall clock, never anything the
	// agent did.
	st.Act.TimeOfDay = timeOfDay(now)
	if f.tod != 0 {
		st.Act.TimeOfDay = f.tod
	}
	if f.ctxUsed != 0 {
		st.Act.ContextUsed = f.ctxUsed
	}
	return st
}

// frame paints one frame and returns it as ANSI, rows joined by newlines.
//
// It also rings the audible half of the nudge, keyed off the bubble rather
// than the pose and edge-detected, so a sixty-second permission nag knocks
// once rather than once a minute. Only when following a real session: the
// demo cycles every state on a timer, and a scape nobody attached to has
// nothing to announce.
func (f *frames) frame(now time.Time) string {
	return f.composeFrame(now).Render(f.profile)
}

// composeFrame is everything frame does except turn the canvas into bytes, so
// a caller that writes a DIFF (the live loop; see canvas.Screen) does not have
// to render the frame twice to find out what changed.
func (f *frames) composeFrame(now time.Time) *canvas.Canvas {
	f.refreshCompanion(now)
	t := now.Sub(f.start).Seconds()
	st := f.state(now, t)
	// The spectators' clock, and their one per-frame duty: noticing a tool that
	// has been running too long, whether or not anything is being drawn.
	banterNow = now
	if banterLive() {
		banterEngine.Tick(now)
		banterWatch(now)
	}
	// The animal lives whether or not a session is being followed, and before
	// anything is drawn -- including the demo and `-print` paths, which have no
	// reducer at all. One tick per frame, in one place.
	if f.life {
		updateLife(now, st)
	}
	if f.red != nil {
		// The pose is consulted for one thing only: a finish while the
		// companion is still worried is the third knock (his ruling of
		// 2026-09-15). The ask and the edge detection stay keyed off the bubble.
		if k, ring := f.knocker.Knock(st.Bubble, st.BubbleAsk, st.Pose == companion.Worried); ring {
			f.player.Play(k)
			if f.knocked != nil {
				f.knocked(k)
			}
		}
	}

	// The approach. His idea, 2026-09-10: when the agent needs him, the
	// companion should "walk up closer to the screen and get bigger" before it
	// prompts -- so it is driven by the POSE, eased over real seconds, and it
	// retreats through the same call the moment the ask is answered.
	//
	// It is NOT a delay in front of the notification. The knock above has
	// already rung and the balloon goes up this same frame; the walk happens
	// underneath it. His asks stay open a median 130 s (28 closed ask windows
	// over 30 recordings), so there is no shortage of time to cross the sand.
	//
	// Only NeedsYou brings it forward. Done is a knock he can answer whenever
	// he likes, and Worried persists for a median 795 s -- a companion parked
	// at full size for thirteen minutes is not an approach, it is a new
	// default size.
	dt := 0.0
	if !f.lastFrame.IsZero() {
		dt = now.Sub(f.lastFrame).Seconds()
	}
	f.lastFrame = now
	f.cat.Approach(dt, st.Pose == companion.NeedsYou)
	if f.harbourCat != nil {
		f.harbourCat.Approach(dt, st.Pose == companion.NeedsYou)
	}

	f.refreshScape(now)
	f.compose(st, t)
	return f.c
}

// compose paints one state at one time into f.c through the scape in use:
// the vista or the shore, each through its own composer. The live frame
// and the tests share it, so a test composes exactly what the loop draws.
func (f *frames) compose(st reduce.State, t float64) {
	// The frame's own switch decides whether this drawing carries the animal;
	// see lifeLive.
	lifeInFrame = f.life
	if f.rainView >= 0 {
		// The rain is a scripted clip: one of three pre-rendered views that
		// cycle each time the scape switches to ScapeRain. It replaces the
		// shore, vista and harbour entirely.
		scenes.PaintRainFrame(f.c, f.rainView, t, f.seed)
		return
	}
	if f.harbour != nil {
		f.harbour.CatX = f.lay.CatX
		f.harbour.Kittens, f.harbour.Exits = st.Kittens, st.KittenExits
		f.harbour.Update(f.c, t, st.Act)
		drawHarbour(f.c, f.harbour, f.harbourCat, f.lay, st, t)
		return
	}
	if f.vista != nil {
		f.vista.OwlX = f.lay.CatX
		f.vista.Update(f.c, t, st.Act)
		drawVista(f.c, f.vista, f.lay, st, t)
		if banterLive() {
			owlX, owlY, _, bandTop := f.vista.Layout()
			drawBanterVista(f.c, owlX, owlY, bandTop)
		}
		return
	}
	f.sh.Update(f.c, t, st.Act)
	top := f.c.H - 2 - f.chh
	drawScene(f.c, f.sh, f.cat, f.lay, st, t, f.seed, top)
	if banterLive() {
		// The same cell drawScene put the crab in, so a line lands over the
		// creature that is saying it rather than over where it used to stand.
		dx, _ := pace(st, f.c.W)
		catX := f.lay.CatX + int(math.Round(paceOffsetOf(dx, f.lay))) + lifeOffset(f.lay.Mirror, f.lay.PaceSpan)
		drawBanterShore(f.c, catX, top, 0, f.lay.Mirror)
	}

	// Draw Tamagotchi overlay if enabled
	if f.tamagotchi && tamagotchiState != nil {
		drawTamagotchi(f.c, st, t)
	}
}
