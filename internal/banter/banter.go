// Package banter is the comedy layer: two small creatures watching a coding
// session and commenting on it.
//
// THE RULE THAT SHAPES EVERYTHING: they are spectators. Nothing here opens a
// file, runs a command, or suggests a fix. The package imports `event` and
// `time` and nothing else, and a test holds that line.
//
// WHAT IT IS FED is exactly what the reducer already receives -- tool_start,
// tool_end, error, test_pass, test_fail, sub_start, sub_end, done -- plus a
// clock, so a tool that never returns can be noticed without inventing an event
// that no adapter emits.
//
// WHAT IT DECIDES is a Show: who speaks, when, for how long, whether the second
// creature walks in, and what prop appears. The renderer draws a Show; the engine
// never touches a canvas.
package banter

import (
	"strings"
	"time"

	"github.com/donlucasx/xscapes/internal/event"
)

// Place is which world the creatures live in, so the lines fit the scenery.
type Place string

const (
	Shore Place = "shore"
	Vista Place = "vista"
)

// Who is a speaker.
type Who int

const (
	Primary Who = iota // the crab on the shore, the owl on the vista
	Second             // the cat, the owlet
)

// Line is one spoken line, timed from the start of its Show.
type Line struct {
	Who  Who
	Text string
	At   float64 // seconds after the show starts
	Hold float64 // seconds it stays up
}

// Prop is a small visual joke staged with a show.
type Prop string

const (
	NoProp  Prop = ""
	Smoke   Prop = "smoke"   // a little cloud of "that did not go well"
	Sign    Prop = "sign"    // a tiny board with a mark on it
	Sparkle Prop = "sparkle" // the celebration
)

// Show is one staged reaction.
type Show struct {
	cat      Category // what produced it; not drawn, only remembered
	Start    time.Time
	Lines    []Line
	Second   bool    // the second creature comes on
	SecondAt float64 // seconds after Start
	Prop     Prop
	PropAt   float64
	End      float64 // seconds after Start
}

// Over says whether the show has finished.
func (s *Show) Over(now time.Time) bool {
	if s == nil {
		return true
	}
	return now.Sub(s.Start).Seconds() > s.End
}

// SecondOn says whether the second creature should be drawn now.
func (s *Show) SecondOn(now time.Time) bool {
	if s == nil || !s.Second {
		return false
	}
	return now.Sub(s.Start).Seconds() >= s.SecondAt
}

// Speaking returns the lines that are up at this instant, in order.
func (s *Show) Speaking(now time.Time) []Line {
	if s == nil {
		return nil
	}
	t := now.Sub(s.Start).Seconds()
	var out []Line
	for _, l := range s.Lines {
		if t >= l.At && t <= l.At+l.Hold {
			out = append(out, l)
		}
	}
	return out
}

// PropUp says whether the prop is drawn now.
func (s *Show) PropUp(now time.Time) bool {
	if s == nil || s.Prop == NoProp {
		return false
	}
	t := now.Sub(s.Start).Seconds()
	return t >= s.PropAt && t <= s.End
}

// Snapshot is everything the renderer needs for one frame.
type Snapshot struct {
	Show     *Show
	Lines    []Line
	SecondOn bool
	PropUp   bool
	Prop     Prop
}

// Category is what kind of thing happened, which selects the words.
type Category string

const (
	CatToolFail  Category = "tool_fail"
	CatTestFail  Category = "test_fail"
	CatTimeout   Category = "timeout"
	CatMissing   Category = "missing"
	CatBuild     Category = "build_fail"
	CatBuildPass Category = "build_pass"
	CatBored     Category = "bored"
	CatRecovered Category = "recovered"
	CatCelebrate Category = "celebrate"
	CatSubagent  Category = "subagent"
	CatWrite     Category = "write"
	CatIdle      Category = "idle"
)

// Tunables. They are constants because a companion should feel the same in the
// morning as it did last night, and because a test can then assert them.
const (
	// TTL is how long an unfinished escalation is remembered.
	TTL = 8 * time.Minute
	// Cool is the least time between two shows, whatever caused them.
	Cool = 9 * time.Second
	// KeyCool is the least time between two shows for the same operation.
	KeyCool = 20 * time.Second
	// Budget is how many shows may start in one minute.
	Budget = 5
	// Stuck is how long a tool may run before the creatures comment on it.
	Stuck = 40 * time.Second
	// Bored is how long before the comment becomes a complaint. The second
	// remark on one refusal to finish; the brief asks for creatures that get
	// silly rather than louder, which is why it is a different category and
	// not a longer timeout line.
	Bored = 2 * time.Minute
	// DedupeWindow drops the same event delivered twice by two listeners.
	DedupeWindow = 900 * time.Millisecond
)

// Engine is the whole state of the comedy. It is deterministic given a seed, so
// a session can be replayed.
type Engine struct {
	place Place
	rng   uint64
	seed  uint64

	shows   []*Show
	episode map[string]*episode
	stuck   map[string]time.Time
	bored   map[string]time.Time
	flight  map[string]time.Time
	recent  []time.Time // show start times, for the minute budget
	seen    map[string]time.Time
	lastAny time.Time

	testFailed bool
	lastCat    Category
	lastRung   int            // how many times this failure has been heard
	lastLine   map[string]int // category -> next variant index, so it does not repeat
}

type episode struct {
	cat   Category
	count int
	last  time.Time
	key   string
}

// New makes an engine for a place.
func New(place Place, seed int64) *Engine {
	if place != Shore && place != Vista {
		place = Shore
	}
	return &Engine{
		place:    place,
		seed:     uint64(seed) | 1,
		rng:      uint64(seed) | 1,
		episode:  make(map[string]*episode),
		stuck:    make(map[string]time.Time),
		bored:    make(map[string]time.Time),
		flight:   make(map[string]time.Time),
		seen:     make(map[string]time.Time),
		lastLine: make(map[string]int),
	}
}

// Place reports the world this engine speaks for.
func (e *Engine) Place() Place { return e.place }

// Observe folds one real event in. It never returns an error and never panics on
// missing metadata: an event with no tool, no target and no detail is simply a
// thinner fact.
func (e *Engine) Observe(ev event.Event, now time.Time) {
	e.expire(now)

	// The same event can arrive twice (two listeners, a socket and a file);
	// one fingerprint within the window is one thing that happened.
	fp := string(ev.Kind) + "|" + ev.ID + "|" + string(ev.Op) + "|" + ev.Tool + "|" + ev.Target + "|" + ev.Detail
	if at, ok := e.seen[fp]; ok && now.Sub(at) < DedupeWindow {
		return
	}
	e.seen[fp] = now

	// The main thread is the one that watches; a subagent's errors are its own
	// business (the reducer already made that ruling for the worried face).
	main := ev.Agent == ""

	switch ev.Kind {
	case event.SessionStart, event.SessionEnd:
		e.episode = make(map[string]*episode)
		e.flight = make(map[string]time.Time)
		e.stuck = make(map[string]time.Time)
		e.bored = make(map[string]time.Time)
		e.testFailed = false

	case event.Prompt:
		e.testFailed = false

	case event.ToolStart:
		// A tool_start for an id already in flight is the same call being
		// re-announced (two hooks, a replay). Restarting its clock would hide a
		// genuine stall behind a duplicate event.
		if ev.ID != "" {
			if _, seen := e.flight[ev.ID]; !seen {
				e.flight[ev.ID] = now
			}
		}

	case event.ToolEnd:
		delete(e.flight, ev.ID)
		delete(e.stuck, ev.ID)
		delete(e.bored, ev.ID)
		if !main {
			return
		}
		// A tool that succeeds where the same tool just failed is a comeback.
		k := keyOf(ev)
		if ep, ok := e.episode[k]; ok && ep.count > 0 {
			delete(e.episode, k)
			e.startStampedRung(CatRecovered, e.show(CatRecovered, ev), now, ep.count)
			return
		}
		// A build that finishes clean is its own small event; the command line
		// is the evidence, so the creatures are not guessing at a cause.
		if looksBuild(ev) {
			e.kindly(CatBuildPass, ev, now)
			return
		}
		if ev.Op == event.OpWrite || ev.Op == event.OpEdit {
			// Not every save deserves a remark; one in four does.
			if e.roll(4) == 1 {
				e.maybe(CatWrite, ev, now)
			}
		}

	case event.Error, event.TestFail:
		delete(e.flight, ev.ID)
		delete(e.stuck, ev.ID)
		if !main {
			return
		}
		cat := CatToolFail
		if ev.Kind == event.TestFail {
			cat = CatTestFail
			e.testFailed = true
		} else if looksBuild(ev) {
			// The command said what it was doing; nothing is inferred.
			cat = CatBuild
		} else if looksMissing(ev) {
			cat = CatMissing
		}
		k := keyOf(ev)
		ep, ok := e.episode[k]
		if !ok || now.Sub(ep.last) > TTL {
			ep = &episode{cat: cat, key: k}
			e.episode[k] = ep
		}

		// The ladder is only climbed when the rung is HEARD. A failure that
		// arrives too soon to speak is not forgotten -- it stays at the same
		// count, so the next one is still the second and still brings the
		// second creature on. Counting silently would burn the escalation on
		// a burst of errors the user never saw.
		if !e.allowed(now, ep.count >= 1) {
			return
		}
		ep.count++
		ep.last = now

		// The escalation ladder, and the only place a second creature appears.
		switch {
		case ep.count == 1:
			e.startStampedRung(cat, e.show(cat, ev), now, ep.count)
		case ep.count == 2:
			e.startStampedRung(cat, e.conversation(cat, ev, 4), now, ep.count)
		default:
			e.startStampedRung(cat, e.conversation(cat, ev, 6), now, ep.count)
		}

	case event.TestPass:
		if e.testFailed {
			e.testFailed = false
			e.kindly(CatCelebrate, ev, now)
		}

	case event.SubStart:
		if e.roll(2) == 1 {
			e.maybe(CatSubagent, ev, now)
		}

	case event.Done:
		// The end of a turn is the one moment both are watching the same thing.
		if e.roll(3) == 1 {
			e.maybe(CatIdle, ev, now)
		}
	}
}

// Tick is the clock's contribution: a tool that has been running for a long time
// gets noticed, and finished shows fall away. Called once per frame; it costs a
// map walk.
func (e *Engine) Tick(now time.Time) {
	e.expire(now)
	for id, at := range e.flight {
		age := now.Sub(at)
		if age < Stuck {
			continue
		}
		if _, done := e.stuck[id]; !done {
			e.stuck[id] = now
			e.startStamped(CatTimeout, e.show(CatTimeout, event.Event{Kind: event.ToolStart, ID: id}), now)
			continue
		}
		// Past Bored it stops being a wait and starts being a personality
		// problem: one more remark, and then it lives with it.
		if age >= Bored {
			if _, said := e.bored[id]; !said {
				e.bored[id] = now
				e.maybe(CatBored, event.Event{Kind: event.ToolStart, ID: id}, now)
			}
		}
	}
	// Drop finished shows so a long session does not accumulate them.
	live := e.shows[:0]
	for _, s := range e.shows {
		if !s.Over(now) {
			live = append(live, s)
		}
	}
	e.shows = live
}

// Current is what to draw right now. Never nil-safe by accident: an engine with
// nothing to say returns an empty Snapshot.
func (e *Engine) Current(now time.Time) Snapshot {
	e.Tick(now)
	if len(e.shows) == 0 {
		return Snapshot{}
	}
	s := e.shows[len(e.shows)-1]
	if s.Over(now) {
		return Snapshot{}
	}
	return Snapshot{
		Show:     s,
		Lines:    s.Speaking(now),
		SecondOn: s.SecondOn(now),
		PropUp:   s.PropUp(now),
		Prop:     s.Prop,
	}
}

// Now is the show that is on stage, if any. It is how a caller knows whether a
// line it has just been given can still be used.
func (e *Engine) Now(now time.Time) *Show {
	if len(e.shows) == 0 {
		return nil
	}
	s := e.shows[len(e.shows)-1]
	if s.Over(now) {
		return nil
	}
	return s
}

// Restyle replaces the FIRST line of the show on stage, if that line has not
// been spoken yet in any meaningful sense -- the first two seconds. This is the
// whole of the optional AI's power over the comedy: it may reword a sentence
// before it is read, and nothing else. It cannot add a line, bring a creature
// on, or change a category.
func (e *Engine) Restyle(text string, now time.Time) bool {
	s := e.Now(now)
	if s == nil || len(s.Lines) == 0 {
		return false
	}
	if now.Sub(s.Start).Seconds() > 2.0 {
		return false
	}
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > 46 {
		return false
	}
	if s.Lines[0].Who != Primary {
		return false
	}
	s.Lines[0].Text = text
	return true
}

// Quiet says whether anything has happened lately, for a caller that wants to
// avoid drawing over a show.
func (e *Engine) Quiet(now time.Time) bool {
	return len(e.shows) == 0 || e.shows[len(e.shows)-1].Over(now)
}

// --- internals -------------------------------------------------------------

// maybe starts a show if the world allows it.
func (e *Engine) maybe(cat Category, ev event.Event, now time.Time) {
	if e.allowed(now, false) {
		e.startStamped(cat, e.show(cat, ev), now)
	}
}

// kindly is a celebration: it may break the per-key cooldown, because good news
// that arrives late is still good news, but never the global one.
func (e *Engine) kindly(cat Category, ev event.Event, now time.Time) {
	if now.Sub(e.lastAny) < Cool {
		return
	}
	e.startStamped(cat, e.show(cat, ev), now)
}

// allowed is the anti-spam gate.
//
// An escalation (a second or third failure of the same operation) is allowed to
// speak sooner than an unrelated remark, because it is the same conversation
// continuing -- but never while a show is still playing, and never past the
// minute's budget.
func (e *Engine) allowed(now time.Time, escalate bool) bool {
	// Nothing overlaps: a creature talking over itself is not comedy.
	for _, s := range e.shows {
		if !s.Over(now) {
			return false
		}
	}
	gap := Cool
	if escalate {
		gap = 3 * time.Second
	}
	if now.Sub(e.lastAny) < gap {
		return false
	}
	recent := e.recent[:0]
	for _, t := range e.recent {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	e.recent = recent
	return len(e.recent) < Budget
}

func (e *Engine) startStamped(cat Category, s *Show, now time.Time) {
	e.startStampedRung(cat, s, now, 0)
}

// startStampedRung records the category and, for a failure, which rung of the
// escalation ladder was heard -- the optional voice needs to know whether this
// was the first time or the third.
func (e *Engine) startStampedRung(cat Category, s *Show, now time.Time, rung int) {
	if s == nil {
		return
	}
	s.cat = cat
	e.lastRung = rung
	e.start(s, now)
}

func (e *Engine) start(s *Show, now time.Time) {
	if s == nil {
		return
	}
	s.Start = now
	e.shows = append(e.shows, s)
	e.lastAny = now
	e.recent = append(e.recent, now)
	e.lastCat = s.cat
}

func (e *Engine) expire(now time.Time) {
	for k, ep := range e.episode {
		if now.Sub(ep.last) > TTL {
			delete(e.episode, k)
		}
	}
	for fp, at := range e.seen {
		if now.Sub(at) > 5*time.Minute {
			delete(e.seen, fp)
		}
	}
}

// roll returns n for one call out of d, deterministically.
func (e *Engine) roll(d int) int {
	e.rng ^= e.rng << 13
	e.rng ^= e.rng >> 7
	e.rng ^= e.rng << 17
	return int(e.rng % uint64(d))
}

// keyOf normalises an operation so two different commands are not "the same
// problem" by accident, and two failures of one command are.
func keyOf(ev event.Event) string {
	k := string(ev.Op) + "|" + ev.Tool + "|" + ev.Target
	if ev.Target == "" {
		k = string(ev.Op) + "|" + ev.Tool
	}
	return k
}

// looksBuild is the second cause the engine is allowed to name, and it names it
// from the command line the adapter actually sent -- never from a guess at what
// a failure meant. "go build ./..." is a build; "go vet ./..." is not.
func looksBuild(ev event.Event) bool {
	for _, field := range []string{ev.Target, ev.Text, ev.Detail} {
		if buildCommand(field) {
			return true
		}
	}
	return false
}

func buildCommand(cmd string) bool {
	f := strings.Fields(cmd)
	if len(f) == 0 {
		return false
	}
	// Skip the wrappers a shell command usually opens with.
	i := 0
	for i < len(f) && (f[i] == "sudo" || f[i] == "env" || f[i] == "time" || strings.Contains(f[i], "=")) {
		i++
	}
	if i >= len(f) {
		return false
	}
	word := strings.ToLower(strings.Trim(f[i], `"'`))
	tail := f[i+1:]
	first := func() string {
		if len(tail) > 0 {
			return strings.ToLower(tail[0])
		}
		return ""
	}
	switch word {
	case "make", "cmake", "gradle", "mvn", "msbuild", "tsc", "xcodebuild", "ninja", "bazel":
		return true
	case "go", "cargo", "zig", "swift", "dotnet", "mix":
		return first() == "build" || first() == "compile"
	case "npm", "yarn", "pnpm":
		return (first() == "run" && len(tail) > 1 && strings.ToLower(tail[1]) == "build") || first() == "build"
	}
	return false
}

func looksMissing(ev event.Event) bool {
	return contains(ev.Text, "no such file") || contains(ev.Detail, "no such file") ||
		contains(ev.Text, "not found") || contains(ev.Detail, "not found")
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	n, m := len(s), len(sub)
	for i := 0; i+m <= n; i++ {
		ok := true
		for j := 0; j < m; j++ {
			if lower(s[i+j]) != lower(sub[j]) {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}
