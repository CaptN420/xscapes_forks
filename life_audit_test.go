package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/life"
	"github.com/donlucasx/xscapes/internal/reduce"
)

// The life layer as the PROGRAM uses it: the switch, the file, the pose the crab
// actually wears, and the frames a terminal receives.

func resetLife(t *testing.T) {
	t.Helper()
	lifeBrain, lifeLast, lifeInFrame = nil, time.Time{}, true
	// The voice's memory lives in package variables too; a test that leaves a
	// story behind rewrites the next one's world.
	lifeStory, lifeObservation, lifeAISubject, lifeSeenFinds = "", "", "", 0
	lifeProject, lifeAIMode = false, "off"
	lifeStoryAt, lifeObsAt = time.Time{}, time.Time{}
	closeLifeAI()
	t.Cleanup(func() {
		lifeBrain, lifeLast, lifeInFrame = nil, time.Time{}, true
		lifeStory, lifeObservation, lifeAISubject, lifeSeenFinds = "", "", "", 0
		lifeProject, lifeAIMode = false, "off"
		closeLifeAI()
	})
}

// The companion is on by default and off on request, and the earlier creature is
// the other way round now.
func TestTheCompanionSwitches(t *testing.T) {
	for _, v := range []string{"off", "0", "false", "no"} {
		t.Setenv("XSCAPES_LIFE", v)
		if !isOff(v) {
			t.Fatalf("%q should read as off", v)
		}
	}
	// The frame's own gate, with the environment read at start-up.
	if !isOn("on") || isOn("off") {
		t.Error("the yes/no reader is wrong")
	}
	if isOn("") {
		t.Error("the legacy creature is on by default again: two animals would ship")
	}
}

// A life is kept where every other setting is kept, and it survives a process.
func TestALifeSurvivesAProcess(t *testing.T) {
	resetLife(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	initLife()
	if lifeBrain == nil {
		t.Fatal("no companion was started")
	}
	lifeBrain.Do(life.CmdExplore)
	for i := 0; i < 400 && lifeBrain.Intent().Pose == life.Away; i++ {
		lifeBrain.Tick(1, life.World{})
	}
	lifeBrain.Do(life.CmdPet)
	wantFinds := len(lifeBrain.Finds())
	wantSeed := lifeBrain.Seed()
	wantFriends := lifeBrain.Attributes().Friendship
	saveLife()

	path, err := lifeStateFile()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the life is not on disk: %v", err)
	}

	// A new process.
	lifeBrain, lifeLast = nil, time.Time{}
	initLife()
	if len(lifeBrain.Finds()) != wantFinds {
		t.Errorf("its finds did not survive: %v", lifeBrain.Finds())
	}
	if lifeBrain.Seed() != wantSeed {
		t.Error("the animal changed identity across a restart")
	}
	if lifeBrain.Attributes().Friendship != wantFriends {
		t.Errorf("its friendship did not survive: %.1f -> %.1f",
			wantFriends, lifeBrain.Attributes().Friendship)
	}
}

// The earlier creature's file is not abandoned: it becomes this animal once.
func TestAnOldCreatureIsCarriedOver(t *testing.T) {
	resetLife(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	legacy, err := legacyStateFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home+"/.config/xscapes", 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{"stage":2,"stage_secs":300,"happiness":70,"affection":50,"fed_count":4,"pet_count_ignored":0}`
	if err := os.WriteFile(legacy, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	initLife()
	st, _ := lifeBrain.Stage()
	if st != life.Teen {
		t.Errorf("the old animal arrived as %s, want the stage it had reached", st)
	}
	if lifeBrain.Attributes().Happiness != 70 {
		t.Errorf("its happiness was lost: %.0f", lifeBrain.Attributes().Happiness)
	}
	// And the new file exists, so the migration happens once and only once.
	path, _ := lifeStateFile()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the migrated life was not written: %v", err)
	}
}

// The pose the crab wears comes from the animal, and every pose maps onto a real
// companion state -- the crab has five faces and the life layer invents none.
func TestEveryLifePoseBecomesARealCrabFace(t *testing.T) {
	seen := map[companion.State]bool{}
	for p := life.Idle; p <= life.ReturningPhase; p++ {
		st, _ := lifePose(p)
		if st < companion.Resting || st > companion.Done {
			t.Errorf("%s mapped to state %d, which is not a companion state", p, st)
		}
		seen[st] = true
	}
	if len(seen) < 4 {
		t.Errorf("the whole vocabulary collapses onto %d faces", len(seen))
	}
}

// The wander stays inside the strip the layout reserved, on both compositions.
// Outside it, the sand and the litter are laid out against the companion and a
// step would write through the scene's own text.
func TestTheWanderStaysInsideTheReservedStrip(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()

	for _, mirror := range []bool{true, false} {
		for _, span := range []int{1, 3, 6} {
			for i := 0; i < 4000; i++ {
				lifeBrain.Tick(1, life.World{})
				off := lifeOffset(mirror, span)
				if mirror && (off > 0 || off < -span) {
					t.Fatalf("mirrored: offset %d outside [%d,0]", off, -span)
				}
				if !mirror && (off < 0 || off > span) {
					t.Fatalf("left-anchored: offset %d outside [0,%d]", off, span)
				}
			}
		}
	}
}

// The animal never touches the agent's cue, and never raises one.
func TestTheCompanionNeverRingsTheAgentCue(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()

	st := reduce.State{Bubble: "the agent is asking", BubbleAsk: true}
	now := time.Now()
	for i := 0; i < 200; i++ {
		now = now.Add(100 * time.Millisecond)
		updateLife(now, st)
		drawLifeSpeech(spareCanvas(), st, companion.New(companion.DefaultName), 40, 6, 1)
	}
	if st.Bubble != "the agent is asking" || !st.BubbleAsk {
		t.Errorf("the companion overwrote the agent's bubble: %q", st.Bubble)
	}
}

func spareCanvas() *canvas.Canvas {
	return canvas.New(80, 24, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
}

// A terminal can be any size. The overlay is drawn at a fixed place, so this is
// where a small window would panic.
func TestTheCompanionSurvivesAnyTerminalSize(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	cat := companion.New(companion.DefaultName)

	for _, sz := range [][2]int{{1, 1}, {8, 4}, {20, 8}, {40, 12}, {80, 24}, {200, 60}} {
		c := canvas.New(sz[0], sz[1], canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
		st := reduce.State{}
		updateLife(time.Now(), st)
		drawLifeSpeech(c, st, cat, sz[0]/2, 1, 1) // must not panic
	}
}

// Every stage has a coat, and they are all different: the one visible thing age
// does without new art.
func TestAgeShowsInTheCoat(t *testing.T) {
	var last uint8
	for st := life.Baby; st < life.StageCount; st++ {
		c := lifeCoat(st)
		if c.R == 0 {
			t.Errorf("%s has no coat", st)
		}
		if st > life.Baby && c.R <= last {
			t.Errorf("%s is not deeper than the stage before it: %d then %d", st, last, c.R)
		}
		last = c.R
	}
	if lifeCoat(life.Adult) != companion.CrabCoat {
		t.Error("a grown crab is not the crab's own colour")
	}
}

// The status line is what a person reads, and it must never scold.
func TestTheCompanionStatusIsKind(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	s := lifeStatusString()
	if s == "" || strings.Contains(s, "no companion") {
		t.Fatalf("status: %q", s)
	}
	for _, demand := range []string{"need", "must", "starving", "sick", "dying", "hurry"} {
		if strings.Contains(strings.ToLower(s), demand) {
			t.Errorf("the status makes a demand (%q): %s", demand, s)
		}
	}
	for _, want := range []string{"friends", "curious", "found"} {
		if !strings.Contains(s, want) {
			t.Errorf("the status does not report %q: %s", want, s)
		}
	}
}

// A minute of real frames with the companion alive: the whole composition, the
// diff, and no byte spent on a line break that would scroll the terminal.
func TestALivingCompanionCostsTheTerminalNothing(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	lifeBrain.Tick(60, life.World{}) // give it some life first

	t.Setenv("XSCAPES_SCAPE", "shore") // where the companion lives
	f := newFrames(80, 24, 7, false, true, 0.4, 0.62)
	if !f.life || f.sh == nil {
		t.Fatal("the companion is off, or the shore is not the scape")
	}
	s := canvas.NewScreen()
	start := time.Now()
	total, fullTotal, worstOver := 0, 0, 0
	for i := 0; i < 60*12; i++ {
		now := start.Add(time.Duration(i) * time.Second / 12)
		c := f.composeFrame(now)
		full := len(c.Render(f.profile))
		out := s.Emit(c, f.profile)
		fullTotal += full
		total += len(out)
		if over := len(out) - full; over > worstOver {
			worstOver = over
		}
		if strings.Contains(string(out), "\n") || strings.Contains(string(out), "\x1b[2J") {
			t.Fatalf("frame %d would disturb the terminal", i)
		}
	}
	if worstOver > 8 {
		t.Errorf("a frame cost %d bytes more than repainting it", worstOver)
	}
	if total > fullTotal {
		t.Errorf("the diff costs more than full frames: %d vs %d", total, fullTotal)
	}
	t.Logf("60 s with the companion: %d bytes (full frames %d)", total, fullTotal)
}

// It must not eat the frame budget either: the tick runs twelve times a second
// for hours.
func TestTheCompanionTickIsCheap(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	w := life.World{Working: true, Level: 0.5}
	start := time.Now()
	const n = 200000
	for i := 0; i < n; i++ {
		lifeBrain.Tick(1.0/12.0, w)
	}
	per := time.Since(start) / n
	if per > 20*time.Microsecond {
		t.Errorf("a tick takes %v: the budget is 20us", per)
	}
	t.Logf("a tick takes %v", per)
}

// OFFLINE: the life layer needs no network, like the rest of the creature.
func TestTheCompanionNeedsNoNetwork(t *testing.T) {
	forbidden := []string{"net/http", "\"net\"", "os/exec", "http.Get(", "api_key", "API_KEY"}
	files := []string{
		"internal/life/life.go", "internal/life/store.go", "internal/life/words.go",
		"life_integration.go", "life_cmd.go",
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		for _, bad := range forbidden {
			if strings.Contains(string(src), bad) {
				t.Errorf("%s mentions %q", f, bad)
			}
		}
	}
}

// A discovery is visible, not just recorded: its glyph lands on the sand in the
// reserved strip, using the scene's own one-cell language.
func TestAFindIsDrawnOnTheSand(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	lifeBrain.Do(life.CmdExplore)
	for i := 0; i < 400 && lifeBrain.Intent().Pose == life.Away; i++ {
		lifeBrain.Tick(1, life.World{})
	}
	if len(lifeBrain.Finds()) == 0 {
		t.Fatal("no find to draw")
	}
	find := lifeBrain.Finds()[0]
	glyph, ok := life.FindGlyphs[find]
	if !ok {
		t.Fatalf("no glyph for %q", find)
	}

	blank := canvas.New(40, 20, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	cat := companion.New(companion.DefaultName)
	drawLifeFinds(blank, cat, 30, 4, 4)
	plain := blank.RenderPlain()
	if !strings.Contains(plain, string(glyph)) {
		t.Errorf("the find %q (%q) is nowhere on the canvas:\n%s", find, glyph, plain)
	}
	// With no room reserved for it, nothing is drawn at all -- the sand's own
	// clear zone wins over a curiosity.
	tight := canvas.New(40, 20, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	drawLifeFinds(tight, cat, 30, 4, 0)
	if strings.Contains(tight.RenderPlain(), string(glyph)) {
		t.Error("a find was drawn with no strip reserved for it")
	}
}

// Growth reaches the actual frame: a hatchling and an old friend do not compose
// the same picture, and the difference is the animal itself.
func TestTheAnimalInTheSceneGrows(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	// The companion lives on the shore: the vista and the harbour have their own
	// animals (the owl, the cat) and the life layer does not touch them yet.
	t.Setenv("XSCAPES_SCAPE", "shore")
	initLife()

	at := func(stage life.Stage) string {
		lifeBrain.Restore(life.Save{
			Version: 2, Seed: 4242, Stage: stage, StageSecs: 5,
		})
		f := newFrames(80, 24, 7, false, true, 0.4, 0.62)
		if f.sh == nil {
			t.Fatal("the shore did not come up: the test would compare two vistas")
		}
		return f.composeFrame(time.Unix(1700000000, 0)).RenderPlain()
	}

	baby := at(life.Baby)
	adult := at(life.Adult)
	if baby == adult {
		t.Fatal("a hatchling and a grown crab compose identical frames: the stage never reaches the drawing")
	}
	// And the size table is the one the companion package owns, so the two
	// cannot drift apart.
	if companion.ScaleForStage(int(life.Baby)) >= companion.ScaleForStage(int(life.Adult)) {
		t.Error("the stage scales do not increase with age")
	}
}

// The switch is the whole feature: with it off the frame is the old frame, and
// nothing of the animal is drawn -- no pose, no coat, no line under its feet.
func TestSwitchedOffMeansTheOldFrame(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XSCAPES_SCAPE", "shore")
	initLife()
	lifeBrain.DoFood("spicy")
	lifeBrain.Restore(life.Save{Version: 2, Seed: 4242, Stage: life.Adult, StageSecs: 9})

	off := newFrames(80, 24, 7, false, true, 0.4, 0.62)
	off.life = false
	off.composeFrame(time.Unix(1700000000, 0))

	if off.cat.Scale() != 1 {
		t.Errorf("with the companion off the crab is drawn at %.2f", off.cat.Scale())
	}
	if o := lifeOffset(true, 6); o != 0 {
		t.Errorf("with the companion off it wandered %d cells", o)
	}
}
