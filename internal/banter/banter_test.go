package banter

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/donlucasx/xscapes/internal/event"
)

// The acceptance criteria of the feature, as tests: a real event makes a real
// creature say something; a second failure of the same thing brings a second
// creature on; the same event twice is one reaction; a burst is not a chorus; a
// recovery is celebrated; an old escalation is forgotten; and none of it needs a
// model, a file, or a command.

var t0 = time.Unix(1700000000, 0)

func fail(id, tool, target string) event.Event {
	return event.Event{Kind: event.Error, Op: event.OpShell, Tool: tool, Target: target,
		ID: id, Detail: "exit 1"}
}

// --- the first reaction ----------------------------------------------------

func TestTheFirstFailureSaysSomething(t *testing.T) {
	e := New(Shore, 7)
	now := t0
	e.Observe(fail("1", "Bash", "make"), now)
	snap := e.Current(now)
	if len(snap.Lines) == 0 {
		t.Fatal("the first failure produced no line")
	}
	if snap.Lines[0].Who != Primary {
		t.Error("the second creature spoke first")
	}
	if snap.Lines[0].Text == "" || len(snap.Lines[0].Text) > 46 {
		t.Errorf("the line is empty or too long to draw: %q", snap.Lines[0].Text)
	}
	// The cloud rises a beat after the words, which is what makes it read as a
	// reaction rather than as a graphic.
	if snap := e.Current(now.Add(600 * time.Millisecond)); !snap.PropUp || snap.Prop != Smoke {
		t.Error("a failure should leave a small cloud")
	}
}

// Every category has words in both worlds, and they differ.
func TestBothWorldsHaveWordsForEveryCategory(t *testing.T) {
	cats := []Category{CatToolFail, CatTestFail, CatTimeout, CatMissing,
		CatBuild, CatBuildPass, CatBored,
		CatRecovered, CatCelebrate, CatSubagent, CatWrite, CatIdle}
	for _, p := range []Place{Shore, Vista} {
		for _, c := range cats {
			if len(primary[p][c]) == 0 {
				t.Errorf("%s has nothing to say about %s", p, c)
			}
			if len(second[p][c]) == 0 {
				t.Errorf("%s's second creature has nothing to say about %s", p, c)
			}
			for _, line := range primary[p][c] {
				if len([]rune(line)) > 46 {
					t.Errorf("%s: %q is too long to draw", p, line)
				}
			}
		}
	}
	// The two worlds must not be the same script with a different name.
	if primary[Shore][CatTimeout][0] == primary[Vista][CatTimeout][0] &&
		primary[Shore][CatToolFail][0] == primary[Vista][CatToolFail][0] {
		t.Error("the crab and the owl have the same voice")
	}
}

// --- escalation ------------------------------------------------------------

func TestTheSecondFailureBringsTheOtherOne(t *testing.T) {
	e := New(Vista, 3)
	now := t0
	e.Observe(fail("1", "Bash", "make"), now)
	first := e.Current(now)
	if first.SecondOn {
		t.Fatal("the second creature arrived for a first failure")
	}

	// The first show has to finish before the next can be heard.
	now = now.Add(6 * time.Second)
	e.Observe(fail("2", "Bash", "make"), now)
	// It walks in a beat later, not on the same frame as the first word.
	if e.Current(now).SecondOn {
		t.Log("note: the second creature arrived on the same frame")
	}
	second := e.Current(now.Add(1200 * time.Millisecond))
	if !second.SecondOn {
		t.Fatal("the second failure did not bring the second creature on")
	}
	if len(second.Lines) == 0 || second.Lines[0].Who != Primary {
		t.Fatal("the conversation did not start with the first creature")
	}
	// And it becomes a conversation: both speak.
	seen := map[Who]bool{}
	for at := 0; at < 20; at += 1 {
		for _, l := range e.Current(now.Add(time.Duration(at) * time.Second)).Lines {
			seen[l.Who] = true
		}
	}
	if !seen[Primary] || !seen[Second] {
		t.Error("only one of them spoke")
	}
}

func TestTheThirdIsMoreDramatic(t *testing.T) {
	e := New(Shore, 5)
	now := t0
	e.Observe(fail("1", "Bash", "make"), now) // one line
	lens := []int{0}
	for i := 2; i <= 3; i++ {
		now = now.Add(14 * time.Second)
		e.Observe(fail(strconv.Itoa(i), "Bash", "make"), now)
		n := 0
		for at := 0; at < 30; at++ {
			n += len(e.Current(now.Add(time.Duration(at) * time.Second)).Lines)
		}
		lens = append(lens, n)
	}
	if !(lens[2] > lens[1]) {
		t.Errorf("the third failure was not bigger than the second: %d vs %d lines", lens[2], lens[1])
	}
}

// Unrelated work starts its own ladder.
func TestUnrelatedFailuresAreSeparate(t *testing.T) {
	e := New(Shore, 11)
	now := t0
	e.Observe(fail("1", "Bash", "make"), now)
	now = now.Add(14 * time.Second)
	e.Observe(fail("2", "Edit", "main.go"), now)
	snap := e.Current(now)
	if snap.SecondOn {
		t.Error("a different operation counted as the same problem")
	}
}

// --- anti-spam -------------------------------------------------------------

func TestTheSameEventTwiceIsOneReaction(t *testing.T) {
	e := New(Shore, 3)
	now := t0
	ev := fail("1", "Bash", "make")
	e.Observe(ev, now)
	e.Observe(ev, now.Add(200*time.Millisecond)) // a second listener
	if n := len(e.shows); n != 1 {
		t.Errorf("one event became %d shows", n)
	}
}

func TestABurstIsNotAChorus(t *testing.T) {
	e := New(Shore, 3)
	now := t0
	for i := 0; i < 6; i++ {
		e.Observe(fail(strconv.Itoa(i), "Bash", "make"), now.Add(time.Duration(i)*300*time.Millisecond))
	}
	if n := len(e.shows); n != 1 {
		t.Errorf("six failures in two seconds made %d shows", n)
	}
}

func TestTheMinuteHasABudget(t *testing.T) {
	e := New(Shore, 3)
	now := t0
	// Twenty failures inside one minute, each with room to speak.
	for i := 0; i < 20; i++ {
		e.Observe(fail(strconv.Itoa(i), "Bash", "target"+strconv.Itoa(i)), now)
		now = now.Add(2 * time.Second)
	}
	if n := len(e.shows); n > Budget {
		t.Errorf("%d shows in %v, budget is %d per minute", n, 40*time.Second, Budget)
	}
	if len(e.shows) == 0 {
		t.Error("the budget silenced everything")
	}
}

// --- recovery --------------------------------------------------------------

func TestAFixIsCelebrated(t *testing.T) {
	e := New(Shore, 3)
	now := t0
	e.Observe(fail("1", "Bash", "go test"), now)
	now = now.Add(12 * time.Second)
	e.Observe(event.Event{Kind: event.ToolEnd, Op: event.OpShell, Tool: "Bash", Target: "go test", ID: "2"}, now)
	snap := e.Current(now)
	if len(snap.Lines) == 0 {
		t.Fatal("the recovery said nothing")
	}
	if snap.Prop != Sparkle {
		t.Error("a comeback should sparkle")
	}
	// And the ledger is clean: the next failure is a first failure again.
	now = now.Add(12 * time.Second)
	e.Observe(fail("3", "Bash", "go test"), now)
	if snap := e.Current(now); snap.SecondOn {
		t.Error("the ladder did not reset after the fix")
	}
}

func TestAPassingTestAfterAFailingOneIsCelebrated(t *testing.T) {
	e := New(Vista, 3)
	now := t0
	e.Observe(event.Event{Kind: event.TestFail, Detail: "2 failed"}, now)
	now = now.Add(12 * time.Second)
	e.Observe(event.Event{Kind: event.TestPass, Detail: "ok"}, now)
	snap := e.Current(now)
	if len(snap.Lines) == 0 {
		t.Fatal("a passing test after a failing one said nothing")
	}
	if e.LastCategory() != CatCelebrate {
		t.Errorf("it reacted as %q", e.LastCategory())
	}
}

// --- forgetting ------------------------------------------------------------

func TestAnOldEscalationIsForgotten(t *testing.T) {
	e := New(Shore, 3)
	now := t0
	e.Observe(fail("1", "Bash", "make"), now)
	now = now.Add(TTL + time.Minute)
	e.Observe(fail("2", "Bash", "make"), now)
	if snap := e.Current(now); snap.SecondOn {
		t.Error("an escalation from another age was continued")
	}
}

// --- what it says, and what it refuses to say ------------------------------

func TestATimeoutIsNotInvented(t *testing.T) {
	e := New(Shore, 3)
	now := t0
	// A tool that starts and never ends is noticed by the clock, once. A
	// duplicate tool_start for the same call must not restart its patience.
	e.Observe(event.Event{Kind: event.ToolStart, Op: event.OpShell, Tool: "Bash", ID: "x"}, now)
	e.Observe(event.Event{Kind: event.ToolStart, Op: event.OpShell, Tool: "Bash", ID: "x"}, now.Add(Stuck/2))
	if snap := e.Current(now.Add(Stuck / 2)); len(snap.Lines) != 0 {
		t.Error("it complained before the tool was slow")
	}
	snap := e.Current(now.Add(Stuck + time.Second))
	if len(snap.Lines) == 0 || e.LastCategory() != CatTimeout {
		t.Error("a stuck tool was not noticed")
	}
	// Twice is nagging; one tool call says it once.
	e.Tick(now.Add(Stuck + 30*time.Second))
	for _, s := range e.shows {
		if s.cat == CatTimeout {
			count := 0
			for _, s2 := range e.shows {
				if s2.cat == CatTimeout {
					count++
				}
			}
			if count > 1 {
				t.Error("the same slow tool was mentioned more than once")
			}
		}
	}
}

func TestASubagentsErrorIsItsOwnBusiness(t *testing.T) {
	e := New(Shore, 3)
	ev := fail("1", "Bash", "grep")
	ev.Agent = "agent-7"
	e.Observe(ev, t0)
	if n := len(e.shows); n != 0 {
		t.Errorf("a subagent's error produced %d shows", n)
	}
}

func TestMissingFileIsRecognisedFromTheText(t *testing.T) {
	e := New(Shore, 3)
	ev := fail("1", "Read", "config.yaml")
	ev.Detail = "no such file or directory"
	e.Observe(ev, t0)
	if e.LastCategory() != CatMissing {
		t.Errorf("a missing file was classified %q", e.LastCategory())
	}
}

// --- variety and determinism ----------------------------------------------

func TestItDoesNotRepeatItselfImmediately(t *testing.T) {
	e := New(Shore, 3)
	now := t0
	first := ""
	for i := 0; i < 6; i++ {
		e.Observe(event.Event{Kind: event.Error, Op: event.OpShell, Tool: "Bash",
			Target: "build" + strconv.Itoa(i), Detail: "exit 1"}, now)
		line := e.Current(now).Lines
		if len(line) > 0 {
			if line[0].Text == first {
				t.Errorf("said %q twice in a row", first)
			}
			first = line[0].Text
		}
		now = now.Add(14 * time.Second)
	}
}

func TestTheSameLifeReplaysIdentically(t *testing.T) {
	run := func() string {
		e := New(Vista, 42)
		now := t0
		var out []string
		for i := 0; i < 8; i++ {
			e.Observe(fail(strconv.Itoa(i), "Bash", "make"), now)
			for _, l := range e.Current(now).Lines {
				out = append(out, l.Text)
			}
			now = now.Add(13 * time.Second)
		}
		return strings.Join(out, "|")
	}
	if run() != run() {
		t.Error("the same seed and the same events produced different words")
	}
}

// A show always ends: nothing can pin the screen.
func TestEveryShowEnds(t *testing.T) {
	e := New(Shore, 9)
	now := t0
	for i := 0; i < 4; i++ {
		e.Observe(fail(strconv.Itoa(i), "Bash", "make"), now)
		now = now.Add(14 * time.Second)
	}
	for _, s := range e.shows {
		if s.End <= 0 || s.End > 60 {
			t.Errorf("a show runs for %.1fs", s.End)
		}
	}
	last := e.shows[len(e.shows)-1]
	if !last.Over(last.Start.Add(time.Duration(last.End+1) * time.Second)) {
		t.Error("the last show did not end")
	}
}

// --- the line it must not cross -------------------------------------------

// They are spectators. The package may not reach the filesystem, run a command,
// or import anything that could.
func TestTheCreaturesCannotTouchTheCode(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	allowed := map[string]bool{
		"time": true, "strings": true, "strconv": true,
		"github.com/donlucasx/xscapes/internal/event": true,
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if !allowed[path] {
				t.Errorf("%s imports %q: the spectators must stay spectators", name, path)
			}
		}
	}
	// And no shelling out, however it is spelled.
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, bad := range []string{"exec.Command", "os.Open", "os.WriteFile", "net/http"} {
			if strings.Contains(string(body), bad) {
				t.Errorf("%s contains %q", p, bad)
			}
		}
		return nil
	})
}

// The optional AI may reword a sentence that has not been read yet, and nothing
// else: no new lines, no new creatures, no change of category.
func TestARestyleIsOneSentenceAndOnlyEarly(t *testing.T) {
	e := New(Shore, 3)
	now := t0
	e.Observe(fail("1", "Bash", "make"), now)
	if !e.Restyle("The compiler has concerns.", now) {
		t.Fatal("an early restyle was refused")
	}
	if got := e.Now(now).Lines[0].Text; got != "The compiler has concerns." {
		t.Errorf("the line is %q", got)
	}
	// Too late: the sentence has been read.
	e2 := New(Shore, 3)
	e2.Observe(fail("1", "Bash", "make"), now)
	if e2.Restyle("late", now.Add(3*time.Second)) {
		t.Error("a restyle mid-sentence was allowed")
	}
	// Nonsense is refused: empty, or too long to draw.
	e3 := New(Shore, 3)
	e3.Observe(fail("1", "Bash", "make"), now)
	if e3.Restyle("", now) {
		t.Error("an empty restyle was allowed")
	}
	if e3.Restyle(strings.Repeat("x", 60), now) {
		t.Error("a restyle too long to draw was allowed")
	}
	// And it cannot invent a show where there is none.
	e4 := New(Shore, 3)
	if e4.Restyle("hello", now) {
		t.Error("a restyle with no show on stage was allowed")
	}
}
