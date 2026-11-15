package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/life"
	"github.com/donlucasx/xscapes/internal/life/ai"
)

// xscapes life -- the companion's own life, from the command line.
//
// Every verb here is optional by construction. Nothing in this file can make
// the animal worse off, and none of it runs by itself: the animal lives whether
// or not anyone types a word.
func cmdLife(args []string) int {
	if !lifeEnabled {
		fmt.Fprintln(os.Stderr, "xscapes: the companion is switched off (XSCAPES_LIFE=off)")
		return 1
	}
	initLife()
	// The voice is started too, so `life explore` can ask for the story that
	// belongs to the adventure it just started, and `life story` can show it.
	initLifeAI()
	defer closeLifeAI()
	if lifeBrain == nil {
		fmt.Fprintln(os.Stderr, "xscapes: no companion could be started")
		return 1
	}
	defer saveLife()

	if len(args) == 0 {
		fmt.Println(lifeStatusString())
		fmt.Println()
		fmt.Println(lifeArt())
		return 0
	}

	switch args[0] {
	case "status":
		fmt.Println(lifeStatusString())
		fmt.Println()
		fmt.Println(lifeArt())
		return 0

	case "feed", "pet", "play", "walk", "explore", "toy", "call":
		if !lifeInteract {
			fmt.Fprintln(os.Stderr, "xscapes: interactions are switched off (XSCAPES_LIFE_INTERACT=off)")
			return 1
		}
		// One helping, not two: a named food replaces the default meal rather
		// than adding to it.
		verb := args[0]
		arg := ""
		var res life.Result
		if verb == "feed" && len(args) > 1 {
			arg = args[1]
			res = lifeBrain.DoFood(arg)
		} else {
			res = lifeBrain.Do(verb)
		}
		fmt.Println(res.Message)
		if res.Speech != "" {
			fmt.Println()
			fmt.Println(lifeArt())
		}
		// And tell a running scape, so the animal reacts on screen too, not
		// only in the file the next process will read.
		emitLifeInteract(verb, arg)
		return 0

	case "tell":
		// Ask for a story now, and wait a bounded moment for it. A command
		// line may wait; a frame may not, which is why this is the only place
		// that does.
		subject := "something it found"
		if f := lifeBrain.Finds(); len(f) > 0 {
			subject = f[len(f)-1]
		}
		if lifeAI == nil {
			fmt.Println("Its voice is off (XSCAPES_LIFE_AI=off).")
			return 0
		}
		lifeAISubject = "story"
		if !lifeAI.Submit(ai.Request{
			Kind: ai.KindStory, Subject: subject,
			Facts:    []string{"a quiet moment", "the animal is " + lifeBrain.Intent().Pose.String()},
			MaxWords: 24,
		}) {
			fmt.Println("It is still thinking about the last one.")
			return 0
		}
		// A model round trip through an agent measured 7s on this machine, so
		// a command that is told to wait waits properly.
		deadline := time.Now().Add(25 * time.Second)
		for time.Now().Before(deadline) {
			if res, ok := lifeAI.Poll(); ok {
				lifeBrain.Note(res.Text)
				saveLife()
				fmt.Println(res.Text)
				if res.Err != nil && lifeAIMode == "exec" {
					fmt.Printf("  (the model did not answer, so it spoke for itself: %v)\n", res.Err)
				}
				return 0
			}
			time.Sleep(20 * time.Millisecond)
		}
		fmt.Println("It did not think of anything.")
		return 0

	case "story":
		text, at := lifeLatest()
		if text == "" {
			text = lifeBrain.LastSaid() // what it said before the process ended
		}
		if text == "" {
			fmt.Println("It has not told you a story yet. Send it out: `xscapes life explore`.")
			return 0
		}
		fmt.Println(text)
		if !at.IsZero() {
			fmt.Printf("  (said at %s)\n", at.Format("15:04"))
		}
		return 0

	case "ai":
		if len(args) > 1 && strings.TrimLeft(args[1], "-") == "check" {
			return lifeAICheck()
		}
		fmt.Println(lifeAIStatus())
		return 0

	case "foods":
		seen := lifeBrain.FoodSeen()
		fmt.Printf("It has had %d helping(s):\n", lifeBrain.FoodsEaten())
		for _, f := range life.Foods {
			n := seen[f.ID]
			mark := " "
			if n > 0 {
				mark = "x"
			}
			fmt.Printf("  [%s] %-18s %s  (%d)\n", mark, f.ID, f.Name, n)
		}
		return 0

	case "finds":
		finds := lifeBrain.Finds()
		if len(finds) == 0 {
			fmt.Println("Nothing brought home yet.")
			return 0
		}
		fmt.Printf("%d thing(s) brought home:\n", len(finds))
		for _, f := range finds {
			fmt.Println("  " + f)
		}
		return 0

	case "toys":
		toys := lifeBrain.Toys()
		if len(toys) == 0 {
			fmt.Println("No toys given yet -- it is perfectly happy with the sand.")
			return 0
		}
		for _, t := range toys {
			fmt.Println("  " + t)
		}
		return 0

	case "memory":
		mem := lifeBrain.Memories()
		if len(mem) == 0 {
			fmt.Println("Nothing remembered yet.")
			return 0
		}
		for _, m := range mem {
			fmt.Println("  " + m)
		}
		return 0

	case "help", "-h", "--help":
		lifeUsage(os.Stderr)
		return 0
	}

	fmt.Fprintf(os.Stderr, "xscapes life: unknown command %q\n\n", args[0])
	lifeUsage(os.Stderr)
	return 2
}

func lifeUsage(w *os.File) {
	fmt.Fprint(w, `xscapes life — a companion, not a responsibility

  xscapes life                 where it is in its life, and what it is doing
  xscapes life feed [food]     something to eat (see 'xscapes life foods')
  xscapes life foods           the larder, and what it has tasted
  xscapes life tell            ask it for a story now
  xscapes life story           the last thing it told you
  xscapes life ai              the voice: mode, calls, cache, last error
  xscapes life ai --check      one real round trip, and how long it took
  xscapes life pet             a hand on its shell
  xscapes life play            a game
  xscapes life walk            a stroll along the tideline
  xscapes life explore         send it out; it comes back with something
  xscapes life toy             give it a toy
  xscapes life call            back to its spot
  xscapes life finds           everything it has brought home
  xscapes life toys            what it has been given
  xscapes life memory          the moments it keeps

  Nothing here is required. Ignoring it costs it nothing: it eats, sleeps,
  wanders and explores on its own, and grows up on time alone.

  XSCAPES_LIFE=off             no companion at all
  XSCAPES_LIFE_INTERACT=off    it lives, but nothing may touch it
  XSCAPES_TAMAGOTCHI=on        the earlier, separate creature as well
`)
}

// lifeArt draws the companion as it is right now, so a status is a glance and
// not a paragraph. It goes through the same companion the scape draws — and,
// like the scape, it wears the coat the user picked. Forcing CrabCoat here
// turned the white cat pink on every platform, which is how the desktop
// context bar read "cat" and still drew the crab's coral.
func lifeArt() string {
	c := canvas.New(28, 10, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	cat := companion.New(companionPref())
	if lifeBrain != nil {
		stage, _ := lifeBrain.Stage()
		cat.SetCoat(lifeCoat(stage))
		cat.SetScale(companion.ScaleForStage(int(stage)))
	}
	cat.Draw(c.Near(), 2, 1, 0.3, companion.Resting)
	out := c.RenderPlain()
	return strings.TrimRight(out, "\n")
}
