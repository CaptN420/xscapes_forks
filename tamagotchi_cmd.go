package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/donlucasx/xscapes/internal/companion"
)

// cmdTamagotchi handles the "xscapes tamagotchi" command.
// It provides an interactive interface for interacting with the Tamagotchi creature.
func cmdTamagotchi(args []string) int {
	if len(args) == 0 {
		return cmdTamagotchiStatus(nil)
	}

	switch args[0] {
	case "status", "check":
		return cmdTamagotchiStatus(nil)
	case "personality", "traits":
		return cmdTamagotchiPersonality()
	case "habits", "patterns":
		return cmdTamagotchiHabits()
	case "memory", "memories":
		return cmdTamagotchiMemory()
	case "idle", "absence":
		return cmdTamagotchiIdleTime()
	case "feed", "eat":
		return cmdTamagotchiAction("feed")
	case "play", "game":
		return cmdTamagotchiAction("play")
	case "pet", "stroke":
		return cmdTamagotchiAction("pet")
	case "clean", "bath":
		return cmdTamagotchiAction("clean")
	case "sleep", "nap":
		return cmdTamagotchiSleep()
	case "wake", "awake":
		return cmdTamagotchiWake()
	case "talk":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "xscapes tamagotchi talk: need a message")
			return 1
		}
		msg := strings.Join(args[1:], " ")
		return cmdTamagotchiTalk(msg)
	case "interact":
		return cmdTamagotchiInteractive()
	default:
		fmt.Fprintf(os.Stderr, "xscapes tamagotchi: unknown command %q\n", args[0])
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi              Show status")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi status       Show detailed status")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi feed         Feed the creature")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi play         Play with the creature")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi pet          Pet the creature")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi clean        Clean the creature")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi sleep        Put creature to sleep")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi wake         Wake up the creature")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi talk <msg>   Talk to the creature")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi interact     Interactive mode")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi personality  Its temperament, and how it has grown")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi habits       What it has learned about you")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi memory       The moments it keeps")
		fmt.Fprintln(os.Stderr, "  xscapes tamagotchi idle         How long since it was last touched")
		return 2
	}
}

// cmdTamagotchiStatus shows the current status of the Tamagotchi.
func cmdTamagotchiStatus(args []string) int {
	if tamagotchiState == nil {
		// Initialize a new creature if none exists
		initTamagotchi()
	}

	renderer := companion.NewTamagotchiRenderer(tamagotchiState)
	// The state's own report leads: it is the one that says what stage of life
	// the creature is in and what is holding the next one back.
	fmt.Println(tamagotchiState.GetStatusReport())
	fmt.Println()
	fmt.Println(renderer.GetASCIIArt())
	// An interaction is growth: keep it, or the care is forgotten the moment
	// this process exits.
	saveTamagotchi()
	return 0
}

// cmdTamagotchiAction performs a single action on the Tamagotchi.
func cmdTamagotchiAction(action string) int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	var result companion.InteractionResult
	switch action {
	case "feed":
		result = tamagotchiState.Feed()
	case "play":
		result = tamagotchiState.Play()
	case "pet":
		result = tamagotchiState.Pet()
	case "clean":
		result = tamagotchiState.Clean()
	default:
		fmt.Fprintf(os.Stderr, "xscapes: unknown interaction %q\n", action)
		fmt.Fprintln(os.Stderr, "  try: feed, play, pet, clean, sleep, wake, talk <message>, status")
		return 2
	}

	renderer := companion.NewTamagotchiRenderer(tamagotchiState)
	fmt.Println(result.Message)
	if result.Success {
		fmt.Printf("Animation: %s | Expression: %s\n", result.Animation, result.Expression)
	} else {
		fmt.Printf("(Creature refused: %s)\n", result.Speech)
	}

	// Show updated ASCII art
	fmt.Println("\n" + renderer.GetASCIIArt())
	// An interaction is growth: keep it, or the care is forgotten the moment
	// this process exits.
	saveTamagotchi()
	return 0
}

// cmdTamagotchiSleep puts the creature to sleep.
func cmdTamagotchiSleep() int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	result := tamagotchiState.Sleep()
	renderer := companion.NewTamagotchiRenderer(tamagotchiState)
	fmt.Println(result.Message)
	fmt.Println("\n" + renderer.GetASCIIArt())
	// An interaction is growth: keep it, or the care is forgotten the moment
	// this process exits.
	saveTamagotchi()
	return 0
}

// cmdTamagotchiWake wakes the creature up.
func cmdTamagotchiWake() int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	result := tamagotchiState.WakeUp()
	renderer := companion.NewTamagotchiRenderer(tamagotchiState)
	fmt.Println(result.Message)
	fmt.Println("\n" + renderer.GetASCIIArt())
	// An interaction is growth: keep it, or the care is forgotten the moment
	// this process exits.
	saveTamagotchi()
	return 0
}

// cmdTamagotchiTalk sends a message to the creature.
func cmdTamagotchiTalk(message string) int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	result := tamagotchiState.Talk(message)
	renderer := companion.NewTamagotchiRenderer(tamagotchiState)
	fmt.Printf("You: %s\n", message)
	fmt.Printf("Xscape: %s\n", result.Speech)
	fmt.Println("\n" + renderer.GetASCIIArt())
	// An interaction is growth: keep it, or the care is forgotten the moment
	// this process exits.
	saveTamagotchi()
	return 0
}

// cmdTamagotchiInteractive starts an interactive session with the Tamagotchi.
func cmdTamagotchiInteractive() int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	renderer := companion.NewTamagotchiRenderer(tamagotchiState)

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║     Tamagotchi Interactive Mode      ║")
	fmt.Println("╠══════════════════════════════════════╣")
	fmt.Println("║ Commands:                            ║")
	fmt.Println("║   feed, eat       - Feed the creature  ║")
	fmt.Println("║   play, game      - Play with it       ║")
	fmt.Println("║   pet, stroke     - Pet the creature   ║")
	fmt.Println("║   clean, bath     - Clean the creature ║")
	fmt.Println("║   sleep, nap      - Put to sleep       ║")
	fmt.Println("║   wake, awake     - Wake up            ║")
	fmt.Println("║   talk <msg>      - Talk to it         ║")
	fmt.Println("║   status          - Show status        ║")
	fmt.Println("║   quit, exit      - Exit               ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	fs := flag.NewFlagSet("interact", flag.ContinueOnError)
	fs.Usage = func() {}
	_ = fs

	// A whole line, not one word: fmt.Scanln stopped at the first space, so
	// `talk hello there` answered "talk" and the rest of the line was eaten by
	// the next prompt. Scanner also behaves on a Windows console, where Scanln
	// leaves the newline behind.
	lines := bufio.NewScanner(os.Stdin)
	defer saveTamagotchi() // leaving is not a reason to forget the day

	for {
		fmt.Printf("\n%s\n", renderer.GetASCIIArt())
		fmt.Print("> ")

		if !lines.Scan() {
			fmt.Println()
			break
		}
		input := lines.Text()

		input = strings.TrimSpace(strings.ToLower(input))
		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}
		cmd := parts[0]

		switch cmd {
		case "quit", "exit":
			fmt.Println("Goodbye! Come back soon!")
			return 0
		case "help":
			fmt.Println("\nAvailable commands:")
			fmt.Println("  feed/eat    - Feed the creature")
			fmt.Println("  play/game   - Play with it")
			fmt.Println("  pet/stroke  - Pet the creature")
			fmt.Println("  clean/bath  - Clean the creature")
			fmt.Println("  sleep/nap   - Put to sleep")
			fmt.Println("  wake/awake  - Wake up")
			fmt.Println("  talk <msg>  - Talk to it")
			fmt.Println("  status      - Show detailed status")
			fmt.Println()
		case "feed", "eat":
			result := tamagotchiState.Feed()
			fmt.Printf("%s\n", result.Message)
		case "play", "game":
			result := tamagotchiState.Play()
			fmt.Printf("%s\n", result.Message)
		case "pet", "stroke":
			result := tamagotchiState.Pet()
			fmt.Printf("%s\n", result.Message)
		case "clean", "bath":
			result := tamagotchiState.Clean()
			fmt.Printf("%s\n", result.Message)
		case "sleep", "nap":
			result := tamagotchiState.Sleep()
			fmt.Printf("%s\n", result.Message)
		case "wake", "awake":
			result := tamagotchiState.WakeUp()
			fmt.Printf("%s\n", result.Message)
		case "talk":
			if len(parts) < 2 {
				fmt.Println("Talk to the creature: talk <message>")
				continue
			}
			msg := strings.Join(parts[1:], " ")
			result := tamagotchiState.Talk(msg)
			fmt.Printf("You: %s\n", msg)
			fmt.Printf("Xscape: %s\n", result.Speech)
		case "status":
			// The state's report first: it is the one that says what stage of
			// life it is in and what the next one is waiting for.
			fmt.Println(tamagotchiState.GetStatusReport())
			fmt.Println()
			fmt.Println(renderer.GetStatusReport())
		default:
			fmt.Printf("Unknown command: %s\n", cmd)
			fmt.Println("Type 'help' for available commands.")
		}
	}

	return 0
}

// cmdTamagotchiPersonality shows the creature's personality traits.
func cmdTamagotchiPersonality() int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	fmt.Println(tamagotchiState.GetPersonalityReport())
	return 0
}

// cmdTamagotchiHabits shows the creature's observed habits.
func cmdTamagotchiHabits() int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	fmt.Println(tamagotchiState.GetHabitReport())
	return 0
}

// cmdTamagotchiMemory shows the creature's memories.
func cmdTamagotchiMemory() int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	fmt.Println(tamagotchiState.GetMemoryReport())
	return 0
}

// cmdTamagotchiIdleTime shows how long the creature has been idle.
func cmdTamagotchiIdleTime() int {
	if tamagotchiState == nil {
		initTamagotchi()
	}

	idle := tamagotchiState.GetIdleTime()
	fmt.Printf("The creature has been idle for %s\n", idle.Round(time.Second))

	if idle > 2*time.Hour {
		fmt.Println("*ears droop* I've been waiting...")
	} else if idle > 1*time.Hour {
		fmt.Println("*tilts head* Are you still there?")
	} else if idle > 30*time.Minute {
		fmt.Println("*stretches* Oh, you're back!")
	}

	return 0
}
