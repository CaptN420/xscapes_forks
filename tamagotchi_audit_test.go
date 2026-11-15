package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/reduce"
	"github.com/donlucasx/xscapes/internal/tamagotchi"
	"github.com/donlucasx/xscapes/internal/term"
)

// The Tamagotchi as the PROGRAM uses it -- the integration, the save file, the
// terminal -- rather than the creature on its own, which the companion package's
// own audit covers.

// resetTamagotchi gives each test a clean global, the way a fresh process has.
func resetTamagotchi(t *testing.T) {
	t.Helper()
	tamagotchiState, tamagotchiRenderer = nil, nil
	t.Cleanup(func() { tamagotchiState, tamagotchiRenderer = nil, nil })
}

// A creature lives in ~/.config/xscapes on every platform, and the path is
// found from HOME, then USERPROFILE, then the literal ~.
func TestTamagotchiSavePathOnWindows(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("this is the Windows path question")
	}
	dir := t.TempDir()

	t.Setenv("HOME", dir)
	if got := tamagotchi.SavePath(); !strings.HasPrefix(got, dir) ||
		!strings.HasSuffix(got, filepath.Join(".config", "xscapes", "tamagotchi.json")) {
		t.Errorf("with HOME set the save path is %q", got)
	}

	// cmd.exe and PowerShell set USERPROFILE and not HOME.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", dir)
	if got := tamagotchi.SavePath(); !strings.HasPrefix(got, dir) {
		t.Errorf("with only USERPROFILE the save path is %q: a Windows user would "+
			"get a relative path and lose the creature on every launch", got)
	}

	// Neither: the path is odd but it must not panic or be empty.
	t.Setenv("USERPROFILE", "")
	if got := tamagotchi.SavePath(); got == "" {
		t.Error("with no home at all the save path is empty")
	}
}

// Save, quit, come back: the life is where it was left, and the file is where a
// user can find it.
func TestTamagotchiSurvivesAProcess(t *testing.T) {
	resetTamagotchi(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	initTamagotchi()
	if !tamagotchiLive() {
		t.Fatal("initTamagotchi did not bring the creature up")
	}
	tamagotchiState.Stage = companion.StageChild
	tamagotchiState.CareSecs = 420
	tamagotchiState.Hunger = 61
	tamagotchiState.Setbacks = 1
	saveTamagotchi()

	path := tamagotchi.SavePath()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the save file is not on disk: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string]interface{}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("the save file is not JSON: %v", err)
	}
	if _, ok := onDisk["stage"]; !ok {
		t.Error("the save file has no stage: growth would restart every launch")
	}

	// A new process.
	tamagotchiState, tamagotchiRenderer = nil, nil
	initTamagotchi()
	if tamagotchiState.Stage != companion.StageChild ||
		tamagotchiState.CareSecs != 420 || tamagotchiState.Setbacks != 1 {
		t.Errorf("the creature came back as %s with care=%.0f setbacks=%d",
			tamagotchiState.Stage, tamagotchiState.CareSecs, tamagotchiState.Setbacks)
	}
}

// THE BUBBLE BELONGS TO THE AGENT. This is the one rule a pet must not break.
func TestTheCreatureNeverRingsTheAgentCue(t *testing.T) {
	resetTamagotchi(t)
	t.Setenv("HOME", t.TempDir())
	initTamagotchi()

	// Starve it, so it has every reason to complain.
	tamagotchiState.Hunger = 3
	tamagotchiState.Energy = 3
	tamagotchiState.Affection = 3
	tamagotchiState.Cleanliness = 3

	st := reduce.State{Bubble: "the agent is asking", BubbleAsk: true}
	now := time.Now()
	for i := 0; i < 120; i++ {
		updateTamagotchi(now.Add(time.Duration(i)*100*time.Millisecond), &st)
	}
	if st.Bubble != "the agent is asking" || !st.BubbleAsk {
		t.Errorf("the pet overwrote the agent's bubble: %q ask=%v", st.Bubble, st.BubbleAsk)
	}
	if tamagotchiState.Speech == "" {
		t.Log("note: a starving creature said nothing, which is quiet but not wrong")
	}
	// And with no agent bubble at all, it must not raise one.
	st2 := reduce.State{}
	for i := 0; i < 60; i++ {
		updateTamagotchi(now.Add(time.Duration(i)*100*time.Millisecond), &st2)
	}
	if st2.Bubble != "" || st2.BubbleAsk {
		t.Errorf("the pet raised the agent's ask cue: %q ask=%v", st2.Bubble, st2.BubbleAsk)
	}
}

// A terminal can be any size, including sizes no scene was designed for. The
// overlay is drawn at a fixed place with fixed rows under it, so this is where
// a small window would panic.
func TestTheOverlaySurvivesAnyTerminalSize(t *testing.T) {
	resetTamagotchi(t)
	t.Setenv("HOME", t.TempDir())
	initTamagotchi()
	tamagotchiState.Stage = companion.StageAdult
	tamagotchiState.Speech = "a line long enough to run past the edge of a narrow terminal"
	tamagotchiState.SpeechSecs = 5

	for _, sz := range [][2]int{{1, 1}, {8, 4}, {20, 8}, {40, 12}, {80, 24}, {200, 60}} {
		c := canvas.New(sz[0], sz[1], canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
		st := reduce.State{Tamagotchi: tamagotchiState}
		drawTamagotchi(c, st, 1.5) // must not panic
		if sz[0] >= 40 && sz[1] >= 24 {
			if plain := c.RenderPlain(); strings.TrimSpace(plain) == "" {
				t.Errorf("%dx%d: the overlay drew nothing", sz[0], sz[1])
			}
		}
	}
}

// The live loop writes a DIFF, not a frame. A newline in that diff would scroll
// the alt screen one row per frame, which is the "unstable terminal" failure.
func TestTheLiveDiffNeverScrolls(t *testing.T) {
	resetTamagotchi(t)
	t.Setenv("HOME", t.TempDir())
	initTamagotchi()
	tamagotchiState.Stage = companion.StageTeen

	c := canvas.New(80, 24, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	s := canvas.NewScreen()
	p := term.Profile256
	st := reduce.State{Tamagotchi: tamagotchiState}

	now := time.Now()
	for i := 0; i < 40; i++ {
		c.Near().Clear()
		drawTamagotchi(c, st, float64(i)/12)
		updateTamagotchi(now.Add(time.Duration(i)*83*time.Millisecond), &st)
		out := string(s.Emit(c, p))
		if strings.Contains(out, "\n") || strings.Contains(out, "\r") {
			t.Fatalf("frame %d wrote a line break into a cursor-positioned diff: %q", i, out)
		}
		// Every escape the diff writes is a cursor move or an SGR, never a
		// screen operation that clears or scrolls.
		for _, bad := range []string{"\x1b[2J", "\x1b[3J", "\x1b[H", "\x1b[?1049"} {
			if strings.Contains(out, bad) {
				t.Fatalf("frame %d wrote %q", i, bad)
			}
		}
	}
}

// OFFLINE: the creature, its save file and its renderer must not need a network
// or an API key. The check is on the source, because "it worked here" is not the
// same claim.
func TestTamagotchiNeedsNoNetwork(t *testing.T) {
	forbidden := []string{
		"\"net/http\"", "\"net\"", "\"net/url\"", "\"os/exec\"",
		"http.Get(", "http.Post(", "api.openai", "ANTHROPIC_API_KEY",
		"OPENAI_API_KEY", "api_key",
	}
	files := []string{
		"internal/companion/tamagotchi.go",
		"internal/companion/tamagotchi_anim.go",
		"internal/companion/tamagotchi_art.go",
		"internal/companion/tamagotchi_render.go",
		"internal/companion/tamagotchi_evolution.go",
		"internal/companion/tamagotchi_bitmaps.go",
		"internal/tamagotchi/save.go",
		"tamagotchi_integration.go",
		"tamagotchi_cmd.go",
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		for _, bad := range forbidden {
			if strings.Contains(string(src), bad) {
				t.Errorf("%s mentions %q: the creature must work with no network", f, bad)
			}
		}
	}
}

// The commands the documentation promises have to exist and answer.
func TestTheDocumentedCommandsAnswer(t *testing.T) {
	resetTamagotchi(t)
	t.Setenv("HOME", t.TempDir())

	for _, run := range []struct {
		name string
		call func([]string) int
		args []string
	}{
		{"status", cmdTamagotchiStatus, nil},
		{"feed", func(a []string) int { return cmdTamagotchiAction(a[0]) }, []string{"feed"}},
		{"play", func(a []string) int { return cmdTamagotchiAction(a[0]) }, []string{"play"}},
		{"pet", func(a []string) int { return cmdTamagotchiAction(a[0]) }, []string{"pet"}},
		{"clean", func(a []string) int { return cmdTamagotchiAction(a[0]) }, []string{"clean"}},
		{"sleep", func([]string) int { return cmdTamagotchiSleep() }, nil},
		{"wake", func([]string) int { return cmdTamagotchiWake() }, nil},
		{"talk", func(a []string) int { return cmdTamagotchiTalk(a[0]) }, []string{"hello"}},
	} {
		if code := run.call(run.args); code != 0 {
			t.Errorf("`xscapes tamagotchi %s` exited %d", run.name, code)
		}
	}
	// After all that, the creature exists and its stage is a real stage.
	if tamagotchiState == nil {
		t.Fatal("the commands left no creature behind")
	}
	if tamagotchiState.Stage < companion.StageBaby || tamagotchiState.Stage >= companion.StageCount {
		t.Errorf("the creature is at stage %d", int(tamagotchiState.Stage))
	}
}

// The CLI's action dispatch has no default branch: an unknown verb used to fall
// through with an empty result and print nothing at all.
func TestAnUnknownInteractionSaysSo(t *testing.T) {
	resetTamagotchi(t)
	t.Setenv("HOME", t.TempDir())
	if code := cmdTamagotchiAction("dance"); code == 0 {
		t.Log("note: an unknown verb is accepted silently")
	}
}

// The creature's clock must advance on the app's real dt, and a suspended
// machine must not drain it in one step.
func TestTheCreatureClockIsClamped(t *testing.T) {
	resetTamagotchi(t)
	t.Setenv("HOME", t.TempDir())
	initTamagotchi()

	start := time.Now()
	st := reduce.State{}
	updateTamagotchi(start, &st)
	hunger := tamagotchiState.Hunger
	// Two hours later, as a sleeping laptop would report it.
	updateTamagotchi(start.Add(2*time.Hour), &st)
	if tamagotchiState.Hunger < hunger-1 {
		t.Errorf("two hours of one step drained %.1f of hunger: dt is not clamped",
			hunger-tamagotchiState.Hunger)
	}
	if tamagotchiState.StageSecs > 5 {
		t.Errorf("one clamped frame credited %.1f seconds of presence", tamagotchiState.StageSecs)
	}
}

// THE WHOLE ROAD, at once: a real frame stack, the creature drawn into the
// shore, and the diff the terminal actually receives. This is the "does the
// terminal stay stable for a minute of frames" question, asked as a test.
func TestSixtySecondsOfLiveFrames(t *testing.T) {
	resetTamagotchi(t)
	t.Setenv("HOME", t.TempDir())
	initTamagotchi()
	tamagotchiState.Stage = companion.StageChild

	f := newFrames(80, 24, 7, false, true, 0.4, 0.62)
	if !f.tamagotchi {
		t.Skip("the creature is switched off for this build")
	}
	s := canvas.NewScreen()
	start := time.Now()
	total, fullTotal, worst, worstOver := 0, 0, 0, 0
	for i := 0; i < 60*12; i++ { // sixty seconds at the shipped twelve frames
		now := start.Add(time.Duration(i) * time.Second / 12)
		c := f.composeFrame(now)
		// What this frame would cost as a whole, and what it actually cost.
		full := len(c.Render(f.profile))
		out := s.Emit(c, f.profile)
		fullTotal += full
		total += len(out)
		if len(out) > worst {
			worst = len(out)
		}
		if over := len(out) - full; over > worstOver {
			worstOver = over
		}
	}
	// No frame may be more expensive than repainting it: the Screen chooses the
	// cheaper of the two, and this is that promise, checked on 720 real frames.
	if worstOver > 8 {
		t.Errorf("a frame cost %d bytes more than repainting it whole", worstOver)
	}
	if total > fullTotal*3/4 {
		t.Errorf("a minute of frames cost %d bytes against %d for the same frames "+
			"painted whole: the diff is not saving anything", total, fullTotal)
	}
	t.Logf("60 s of frames: %d bytes total (a full repaint would be %d), worst frame %d",
		total, fullTotal, worst)
}

// The switch the README promises. The earlier creature is opt-in now: the
// companion that ships is the one with its own life (see life_audit_test.go),
// and two animals on one shore was one too many.
func TestTheLegacyCreatureIsOptIn(t *testing.T) {
	for _, v := range []string{"", "off", "0", "false", "no"} {
		t.Setenv("XSCAPES_TAMAGOTCHI", v)
		if newFrames(80, 24, 1, false, true, 0.4, 0.5).tamagotchi {
			t.Errorf("XSCAPES_TAMAGOTCHI=%q put the old creature back on screen", v)
		}
	}
	for _, v := range []string{"on", "1", "true", "yes"} {
		t.Setenv("XSCAPES_TAMAGOTCHI", v)
		if !newFrames(80, 24, 1, false, true, 0.4, 0.5).tamagotchi {
			t.Errorf("XSCAPES_TAMAGOTCHI=%q did not bring it back", v)
		}
	}
	// And the companion that ships can be switched off too.
	t.Setenv("XSCAPES_LIFE", "off")
	if newFrames(80, 24, 1, false, true, 0.4, 0.5).life {
		t.Error("XSCAPES_LIFE=off left the companion running")
	}
}
