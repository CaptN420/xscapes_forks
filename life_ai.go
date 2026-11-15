package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/donlucasx/xscapes/internal/envx"
	"github.com/donlucasx/xscapes/internal/life/ai"
)

// The companion's optional voice: a worker that turns an adventure into a short
// story, and (if allowed) one bounded observation about the project.
//
// Three modes, from the environment:
//
//	XSCAPES_LIFE_AI=off        nothing is asked for, nothing is said
//	XSCAPES_LIFE_AI=template   local templates (the default): no model, no
//	                           network, no cost, and it still tells the story
//	XSCAPES_LIFE_AI=exec       run XSCAPES_LIFE_AI_CMD (default `hermes -z`) per
//	                           request; output is sanitised and cached
//
// XSCAPES_LIFE_PROJECT=on additionally lets it look at the project, read-only,
// and shows an observation only when the file and line it cites really exist.

var (
	lifeAI        *ai.Worker
	lifeAIMode    = "off"
	lifeProject   bool
	lifeAISubject string

	lifeStory       string
	lifeStoryAt     time.Time
	lifeObservation string
	lifeObsAt       time.Time

	lifeSeenFinds  int
	lifeAnimal     = "crab"
	lifePlace      = "shore"
	lifeNextObsAt  time.Time
	lifeProjectDir string
)

// sceneVoice says who is speaking and where it lives, from the running scape.
// A voice belongs to the scene it lives in: the owl does not talk about a
// tideline, and the crab does not glide over a meadow.
func sceneVoice() (animal, place string) {
	switch strings.ToLower(strings.TrimSpace(envx.Lookup("SCAPE"))) {
	case "vista", "mountain", "meadow":
		return "owl", "vista"
	default:
		return "crab", "shore"
	}
}

// initLifeAI starts the worker if the companion is on. It never fails: an absent
// or broken model simply means templates.
func initLifeAI() {
	if !lifeLive() || lifeBrain == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(envx.Lookup("LIFE_AI")))
	if mode == "" {
		mode = "template"
	}
	lifeSeenFinds = len(lifeBrain.Finds())
	lifeAnimal, lifePlace = sceneVoice()

	switch mode {
	case "off":
		lifeAIMode = "off"
		return
	case "exec":
		cmd := strings.TrimSpace(envx.Lookup("LIFE_AI_CMD"))
		if cmd == "" {
			// hermes -z takes the prompt as an argument, not on stdin.
			cmd = "hermes -z {prompt}"
		}
		lifeAI = ai.NewWorker(ai.Exec{Cmd: strings.Fields(cmd), Timeout: 20 * time.Second})
		lifeAIMode = "exec"
	default:
		lifeAI = ai.NewWorker(nil) // templates only, and they are not a downgrade
		lifeAIMode = "template"
	}

	if isOn(envx.Lookup("LIFE_PROJECT")) {
		if wd, err := os.Getwd(); err == nil {
			lifeProjectDir = wd
			lifeProject = true
			lifeNextObsAt = time.Now().Add(2 * time.Minute)
		}
	}
}

// closeLifeAI stops the goroutine. Called wherever the life is saved.
func closeLifeAI() {
	if lifeAI != nil {
		lifeAI.Close()
		lifeAI = nil
	}
}

// updateLifeAI is called once per frame: it drains anything that arrived and, at
// a gentle cadence, asks for something new. Nothing here blocks.
func updateLifeAI(now time.Time) {
	if lifeAI == nil {
		return
	}
	// Drain first: an answer that arrived while the last frame was drawing.
	for {
		res, ok := lifeAI.Poll()
		if !ok {
			break
		}
		if res.Text == "" {
			continue
		}
		if lifeAISubject == "observation" {
			// A claim about the project is shown only if it can be checked.
			if cite := ai.Citation(res.Text); lifeProject && ai.CitationOK(cite, lifeProjectDir) {
				lifeObservation = res.Text
				lifeObsAt = now
				lifeBrain.Note(res.Text)
			}
			continue
		}
		lifeStory = res.Text
		lifeStoryAt = now
		lifeBrain.Note(res.Text)
	}

	// An adventure that just ended asks for its story.
	if finds := lifeBrain.Finds(); len(finds) > lifeSeenFinds {
		lifeSeenFinds = len(finds)
		lifeAISubject = "story"
		facts := []string{"the " + lifePlace + " at " + time.Now().Format("15:04"),
			"the walk took a little while",
			"the animal is " + lifeBrain.Intent().Pose.String()}
		lifeAI.Submit(ai.Request{
			Kind:     ai.KindStory,
			Subject:  finds[len(finds)-1],
			Facts:    facts,
			MaxWords: 24,
			Animal:   lifeAnimal,
			Place:    lifePlace,
		})
	}

	// One look at the project, rarely, and only when asked for.
	if lifeProject && now.After(lifeNextObsAt) {
		lifeNextObsAt = now.Add(12 * time.Minute)
		if facts := ai.ScanProject(lifeProjectDir, ai.MaxFiles); len(facts) > 0 {
			lines := make([]string, 0, len(facts))
			for _, f := range facts {
				lines = append(lines, f.String())
			}
			// The scan is deterministic, so rotate which slice is asked about
			// rather than sending the whole list every time.
			if len(lines) > 4 {
				start := int(now.Unix()/60) % len(lines)
				lines = append(lines[start:], lines[:start]...)
				lines = lines[:4]
			}
			lifeAISubject = "observation"
			lifeAI.Submit(ai.Request{
				Kind:     ai.KindObservation,
				Subject:  "project",
				Facts:    lines,
				MaxWords: 40,
				Animal:   lifeAnimal,
				Place:    lifePlace,
			})
		}
	}
}

// lifeAIStatus is what `xscapes life ai` prints.
func lifeAIStatus() string {
	if lifeAI == nil {
		return "voice: off"
	}
	submitted, served, hits, lastErr := lifeAI.Stats()
	s := "voice: " + lifeAIMode
	s += " | asked " + lifeNum(int(submitted)) + " | answered " + lifeNum(int(served)) +
		" | reused " + lifeNum(int(hits))
	if lifeProject {
		s += " | project: on (" + lifeProjectDir + ")"
	} else {
		s += " | project: off"
	}
	if lastErr != nil {
		s += " | last error: " + lastErr.Error()
	}
	return s
}

// lifeAICheck is `xscapes life ai --check`: ONE real round trip, synchronously,
// because this is the command whose whole purpose is to prove the pipe works.
// It reports the latency and where the answer came from, since "the model
// answered in 9.3s" and "the template answered at once" are very different facts.
func lifeAICheck() int {
	if lifeAI == nil {
		fmt.Println("voice: off -- nothing to check (XSCAPES_LIFE_AI=off)")
		return 0
	}
	if lifeAIMode == "template" {
		fmt.Println("mode: template -- no model is configured, so this checks the")
		fmt.Println("prompt, the worker and the template, and nothing else.")
	}
	start := time.Now()
	lifeAI.Submit(ai.Request{
		Kind:     ai.KindChat,
		Subject:  "a check",
		Facts:    []string{"one short line about where you are"},
		MaxWords: 12,
		Animal:   lifeAnimal,
		Place:    lifePlace,
	})
	deadline := start.Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if res, ok := lifeAI.Poll(); ok {
			took := time.Since(start).Round(100 * time.Millisecond)
			fmt.Printf("answered in %s, from %s, %d chars, one line\n", took, res.Source, len(res.Text))
			fmt.Println(res.Text)
			if res.Err != nil && lifeAIMode == "exec" {
				// A template answering is the design, not a failure; only a
				// model that was asked and did not answer is worth an exit code.
				fmt.Printf("error: %v\n", res.Err)
				return 1
			}
			return 0
		}
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("no answer within 30s")
	return 1
}

// lifeLatest says what it has said lately, for the CLI.
func lifeLatest() (string, time.Time) {
	if lifeObservation != "" && lifeObsAt.After(lifeStoryAt) {
		return lifeObservation, lifeObsAt
	}
	return lifeStory, lifeStoryAt
}
