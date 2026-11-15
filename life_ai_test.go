package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/life"
	"github.com/donlucasx/xscapes/internal/life/ai"
	"github.com/donlucasx/xscapes/internal/reduce"
)

// The voice, as the product uses it: a worker behind the frame, a story that
// becomes a memory, an observation that must cite something real, and a switch
// that means silence.

type testProvider struct {
	delay time.Duration
	text  string
}

func (p *testProvider) Generate(ctx context.Context, req ai.Request) (string, error) {
	select {
	case <-time.After(p.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return p.text, nil
}

// A frame must be drawn at frame pace whatever the model is doing.
func TestTheFrameNeverWaitsForAStory(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	f := newFrames(90, 26, 7, false, true, 0.4, 0.62)

	lifeAI = ai.NewWorker(&testProvider{delay: 2 * time.Second, text: "a story"})
	defer func() { closeLifeAI() }()

	lifeAISubject = "story"
	if !lifeAI.Submit(ai.Request{Kind: ai.KindStory, Subject: "a shell"}) {
		t.Fatal("the request was refused")
	}
	start := time.Now()
	for i := 0; i < 5; i++ {
		f.composeFrame(time.Unix(1700000000, int64(i*16)))
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("five frames took %v while a story was in flight", elapsed)
	}
	if lifeStory != "" {
		t.Error("a story that could not have arrived was already shown")
	}
}

// When it does arrive, it is said, it is remembered, and it is on screen.
func TestAStoryIsSaidAndRemembered(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	f := newFrames(90, 26, 7, false, true, 0.4, 0.62)

	lifeAI = ai.NewWorker(&testProvider{delay: 5 * time.Millisecond, text: "Came back with a pearl."})
	defer func() { closeLifeAI() }()

	lifeAISubject = "story"
	lifeAI.Submit(ai.Request{Kind: ai.KindStory, Subject: "a pearl"})

	// Give the worker a moment, then let a frame pick it up.
	time.Sleep(60 * time.Millisecond)
	plain := f.composeFrame(time.Unix(1700000000, 0)).RenderPlain()

	if !strings.Contains(plain, "pearl") {
		t.Errorf("the story never reached the scene:\n%s", plain)
	}
	if lifeStory == "" {
		t.Error("the story was not kept")
	}
	found := false
	for _, m := range lifeBrain.Memories() {
		if strings.Contains(m, "pearl") {
			found = true
		}
	}
	if !found {
		t.Error("the story was not a memory")
	}
}

// With the voice off nothing is asked, nothing is kept, nothing is drawn.
func TestVoiceOffIsSilence(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	f := newFrames(90, 26, 7, false, true, 0.4, 0.62)
	lifeAI = nil
	lifeAIMode = "off"

	before := f.composeFrame(time.Unix(1700000000, 0)).RenderPlain()
	// A find arrives; with no worker it cannot produce a story.
	lifeBrain.Restore(lifeSaveWithFind(t))
	after := f.composeFrame(time.Unix(1700000000, 1)).RenderPlain()

	if lifeStory != "" {
		t.Error("a story appeared with the voice off")
	}
	if strings.Contains(after, "Came back") {
		t.Errorf("the scene changed under a silent voice:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// An observation about the project is shown only if its citation checks out.
func TestAnObservationMustCiteSomethingReal(t *testing.T) {
	resetLife(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	initLife()
	dir := t.TempDir()
	writeProject(t, dir)
	lifeProject, lifeProjectDir = true, dir
	defer func() { lifeProject = false }()

	// A claim about a file that does not exist is dropped.
	lifeAI = ai.NewWorker(&testProvider{delay: time.Millisecond, text: "I noticed ghost.go:12 is empty."})
	lifeAISubject = "observation"
	lifeAI.Submit(ai.Request{Kind: ai.KindObservation, Subject: "project"})
	time.Sleep(50 * time.Millisecond)
	updateLifeAI(time.Unix(1700000000, 0))
	if lifeObservation != "" {
		t.Errorf("an unverifiable claim was shown: %q", lifeObservation)
	}
	closeLifeAI()

	// A claim that can be checked is shown.
	lifeAI = ai.NewWorker(&testProvider{delay: time.Millisecond, text: "I noticed main.go:3 has func main."})
	lifeAISubject = "observation"
	lifeAI.Submit(ai.Request{Kind: ai.KindObservation, Subject: "project"})
	time.Sleep(50 * time.Millisecond)
	updateLifeAI(time.Unix(1700000000, 1))
	if !strings.Contains(lifeObservation, "main.go:3") {
		t.Errorf("a verifiable claim was dropped: %q", lifeObservation)
	}
}

// The status line is honest about what the worker did.
func TestTheStatusLineSaysWhatItIs(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	initLife()
	if s := lifeAIStatus(); !strings.Contains(s, "off") {
		t.Errorf("with no worker the status reads %q", s)
	}
	lifeAI = ai.NewWorker(nil)
	defer closeLifeAI()
	lifeAIMode = "template"
	if s := lifeAIStatus(); !strings.Contains(s, "template") {
		t.Errorf("a template worker reads %q", s)
	}
}

func lifeSaveWithFind(t *testing.T) life.Save {
	t.Helper()
	return life.Save{Version: 2, Seed: 5, Stage: life.Adult, StageSecs: 4000, Finds: []string{"a bottle cap"}}
}

func writeProject(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"main.go":       "package main\n\nfunc main() {}\n",
		"internal/a.go": "package a\n\ntype A struct{}\n",
	}
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Unused reduce import guard: the frame test above composes real states.
var _ = reduce.State{}

// The running scape decides who speaks. The vista's owl must not tell a story
// about a tideline, and the shore's crab must not glide over a meadow.
func TestTheSceneChoosesTheVoice(t *testing.T) {
	for _, c := range []struct {
		scape, animal, place string
	}{
		{"vista", "owl", "vista"},
		{"", "crab", "shore"},
		{"shore", "crab", "shore"},
		{"VISTA", "owl", "vista"},
	} {
		t.Setenv("XSCAPES_SCAPE", c.scape)
		a, p := sceneVoice()
		if a != c.animal || p != c.place {
			t.Errorf("scape %q gave (%q,%q), want (%q,%q)", c.scape, a, p, c.animal, c.place)
		}
	}
}

// And a story asked for on the vista never mentions the shore's furniture.
func TestAVistaStoryStaysOnTheMountain(t *testing.T) {
	resetLife(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XSCAPES_SCAPE", "vista")
	initLife()
	initLifeAI()
	defer closeLifeAI()

	lifeAISubject = "story"
	lifeAI.Submit(ai.Request{
		Kind: ai.KindStory, Subject: "a shiny beetle",
		Animal: lifeAnimal, Place: lifePlace, MaxWords: 24,
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if res, ok := lifeAI.Poll(); ok {
			low := strings.ToLower(res.Text)
			for _, w := range []string{"tideline", "sand", "swell"} {
				if strings.Contains(low, w) {
					t.Errorf("owl on the vista said %q: %s", w, res.Text)
				}
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no story arrived")
}
