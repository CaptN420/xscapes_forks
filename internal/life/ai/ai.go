// Package ai is the companion's optional voice: a worker that can ask a model
// for a short story or an observation, and that is guaranteed never to be on the
// critical path.
//
// Three rules shape all of it:
//
//	NOTHING BLOCKS. Requests leave on a buffered queue; answers arrive on a
//	channel the frame polls. A request that hangs is abandoned at its deadline.
//
//	TEMPLATES ALWAYS ANSWER. Every kind of request has a deterministic local
//	answer, so the companion is exactly as talkative with no model installed.
//
//	OUTPUT IS TREATED AS TEXT, NOT AS TRUST. Model output is stripped of
//	control characters, collapsed to one paragraph and capped, because it is
//	going into a terminal that also runs a coding agent.
package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Kind is what is being asked for. It shapes the prompt and the template.
type Kind string

const (
	KindStory       Kind = "story"
	KindObservation Kind = "observation"
	KindChat        Kind = "chat"
)

// Request is one question. Facts are short, already-sanitised lines; the worker
// never reads the project itself, so nothing can leak by accident.
type Request struct {
	Kind     Kind
	Subject  string
	Facts    []string
	MaxWords int
	// Animal and Place name the creature and the world it is in, so a voice
	// belongs to the scene it lives in: the owl does not talk about a tideline
	// and the crab does not fly over a meadow. Empty means the shore.
	Animal string
	Place  string
}

// Response is one answer, and where it came from.
type Response struct {
	Text   string
	Source string // "template", "model", "cache"
	Err    error
}

// Provider is the only thing that knows how to reach a model. Anything that
// implements it can back the worker -- a sub-agent, a local model, a test.
type Provider interface {
	Generate(ctx context.Context, req Request) (string, error)
}

// ErrNoProvider is what a worker without a model reports; callers fall back to
// the template.
var ErrNoProvider = errors.New("ai: no provider configured")

// Template answers every request locally and deterministically. It is the
// fallback, and for a companion it is genuinely enough: a few sentences built
// from the facts of the moment.
type Template struct{}

// Generate implements Provider.
func (Template) Generate(_ context.Context, req Request) (string, error) {
	switch req.Kind {
	case KindStory:
		return templateStory(req), nil
	case KindObservation:
		return templateObservation(req), nil
	default:
		if strings.EqualFold(req.Place, "vista") {
			return "Up on the fence post, watching the meadow.", nil
		}
		return "Down by the waterline, keeping an eye on the swells.", nil
	}
}

// templateStory is the companion's own voice, not a degraded model. Facts are
// context for a model, not slots to paste into a sentence, so the template uses
// only the subject and picks one of a few shapes deterministically -- from the
// shapes of its own world.
func templateStory(req Request) string {
	what := strings.TrimSpace(req.Subject)
	if what == "" {
		what = "something small and interesting"
	}
	shapes := storyShapes(req.Place)
	shapes = []string{
		shapes[0] + what + ".",
		shapes[1] + what + ".",
		shapes[2],
	}
	// "Long walk. X, of all things." needs the subject in the middle.
	shapes[2] = shapes[2] + capitalize(what) + ", of all things."
	sum := 0
	for _, r := range what {
		sum += int(r)
	}
	return shapes[sum%len(shapes)]
}

// storyShapes are the three sentence frames of a place. The third is a stem that
// the subject completes.
func storyShapes(place string) []string {
	switch strings.ToLower(strings.TrimSpace(place)) {
	case "vista", "mountain", "meadow":
		return []string{
			"Flew a low circle over the meadow. Came back with ",
			"Perched on the fence post a while, then found ",
			"A long glide, and then: ",
		}
	default: // the shore
		return []string{
			"Went out along the tideline. Came back with ",
			"Nothing much out there except ",
			"Sat on a rock for a while, then found ",
		}
	}
}

func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}

func templateObservation(req Request) string {
	if len(req.Facts) == 0 {
		return ""
	}
	if strings.EqualFold(req.Place, "vista") {
		return "From the fence post I could see " + req.Facts[0] + "."
	}
	return "Looked at " + req.Facts[0] + "."
}

// Exec runs an external command for each request -- the shape a sub-agent or a
// local model CLI takes.
//
// The prompt is passed as an ARGUMENT where the command names one with
// {prompt} (`hermes -z {prompt}`, `ollama run llama3 {prompt}`), and on stdin
// where it does not (`cat`, a filter, a script that reads stdin). The first line
// of stdout is the answer. A command that is missing, slow or noisy is an error,
// not a crash.
type Exec struct {
	Cmd     []string
	Timeout time.Duration
}

// Generate implements Provider.
func (e Exec) Generate(ctx context.Context, req Request) (string, error) {
	if len(e.Cmd) == 0 {
		return "", ErrNoProvider
	}
	t := e.Timeout
	if t <= 0 {
		t = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	prompt := Prompt(req)
	args := make([]string, 0, len(e.Cmd)-1)
	passed := false
	for _, a := range e.Cmd[1:] {
		if strings.Contains(a, "{prompt}") {
			a = strings.ReplaceAll(a, "{prompt}", prompt)
			passed = true
		}
		args = append(args, a)
	}
	cmd := exec.CommandContext(ctx, e.Cmd[0], args...)
	if !passed {
		cmd.Stdin = strings.NewReader(prompt)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return Sanitize(string(out)), nil
}

// Prompt turns a request into the text a model sees. Kept here so a provider and
// a test agree on what was asked.
func Prompt(req Request) string {
	var b strings.Builder
	who := strings.TrimSpace(req.Animal)
	if who == "" {
		who = "crab"
	}
	where := strings.TrimSpace(req.Place)
	if where == "" {
		where = "shore"
	}
	switch req.Kind {
	case KindStory:
		b.WriteString("You are a small " + who + " living on a " + where +
			" in a terminal scene beside a programmer's AI agent. ")
		b.WriteString("In one or two short sentences, in first person, tell what you just did. ")
		b.WriteString("No greetings, no moral, no emoji, no markdown. Max ")
		b.WriteString(itoa(req.MaxWords))
		b.WriteString(" words.\n\nFacts:\n")
	case KindObservation:
		b.WriteString("You are a small " + who + " living on a " + where +
			" in a terminal scene. You looked at a software project. ")
		b.WriteString("State ONE concrete observation in at most two sentences, in first person, and cite the ")
		b.WriteString("file and line as path:line. Do not suggest large rewrites. If the facts are too thin, ")
		b.WriteString("say nothing useful and stop.\n\nFacts:\n")
	default:
		b.WriteString("You are a small " + who + ". Answer in one short sentence, no emoji.\n\n")
	}
	for _, f := range req.Facts {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	return b.String()
}

// Sanitize makes model output safe to plot in a terminal: control characters
// and escape sequences gone, whitespace collapsed, quotes normalised, length
// capped. It is the last line of defence for text this program did not write.
func Sanitize(s string) string {
	// Strip ANSI escapes first: a stray "\x1b[2J" would clear the screen.
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			// Skip CSI sequences, and any single-character escape.
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				if j < len(s) {
					j++
				}
			} else if j < len(s) {
				j++
			}
			i = j - 1
			continue
		}
		if c < 0x20 || c == 0x7f {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	out = strings.ReplaceAll(out, "\"", "\"")
	out = strings.ReplaceAll(out, "\u2019", "'")
	out = strings.ReplaceAll(out, "\u201c", "\"")
	out = strings.ReplaceAll(out, "\u201d", "\"")
	const max = 300
	if utf8.RuneCountInString(out) > max {
		r := []rune(out)
		out = string(r[:max]) + "..."
	}
	return strings.TrimSpace(out)
}

// Worker owns the goroutine, the queue and the cache.
//
// It is written so a caller cannot wait: Submit never blocks (it drops the
// request when the queue is full, which at companion pace is a non-event), and
// Poll never blocks.
type Worker struct {
	p     Provider
	queue chan Request
	out   chan Response

	mu    sync.Mutex
	cache map[string]string
	order []string

	submitted, served, hits, failed int64
	lastErr                         error
	closed                          bool
}

// CacheSize is how many answers are kept; a companion asks the same question
// rarely, so a small cache is enough and it is bounded on purpose.
const CacheSize = 32

// NewWorker starts a worker with a provider. A nil provider means templates
// only: the worker still answers, locally and at once.
func NewWorker(p Provider) *Worker {
	w := &Worker{
		p:     p,
		queue: make(chan Request, 4),
		out:   make(chan Response, 8),
		cache: make(map[string]string, CacheSize),
	}
	go w.loop()
	return w
}

// Close ends the goroutine. Safe to call twice.
func (w *Worker) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.mu.Unlock()
	close(w.queue)
}

// Submit offers a request. It returns false when the queue is full or the worker
// is closed -- never an error, because a companion that cannot think of a line
// simply says nothing.
func (w *Worker) Submit(req Request) bool {
	key := cacheKey(req)
	w.mu.Lock()
	if text, ok := w.cache[key]; ok {
		w.hits++
		w.mu.Unlock()
		select {
		case w.out <- Response{Text: text, Source: "cache"}:
		default:
		}
		return true
	}
	if w.closed {
		w.mu.Unlock()
		return false
	}
	w.submitted++
	w.mu.Unlock()

	select {
	case w.queue <- req:
		return true
	default:
		return false // at companion pace this means "one is already in flight"
	}
}

// Poll takes a finished answer if there is one, and never waits.
func (w *Worker) Poll() (Response, bool) {
	select {
	case r := <-w.out:
		return r, true
	default:
		return Response{}, false
	}
}

// Stats reports what the worker has done, for `xscapes life ai`.
func (w *Worker) Stats() (submitted, served, hits int64, lastErr error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.submitted, w.served, w.hits, w.lastErr
}

func (w *Worker) loop() {
	for req := range w.queue {
		text, err := w.answer(req)
		w.mu.Lock()
		w.served++
		if err != nil {
			w.lastErr = err
		}
		if err == nil {
			key := cacheKey(req)
			if len(w.cache) >= CacheSize {
				// Evict the oldest insertion; a map plus a slice is plenty at
				// this size and avoids a dependency.
				if len(w.order) > 0 {
					delete(w.cache, w.order[0])
					w.order = w.order[1:]
				}
			}
			w.cache[key] = text
			w.order = append(w.order, key)
		}
		w.mu.Unlock()

		resp := Response{Text: text, Source: "model", Err: err}
		if err != nil {
			// A failed request still answers: the template is not a degraded
			// mode, it is the companion's own voice.
			resp = Response{Text: templateFor(req), Source: "template", Err: err}
		}
		select {
		case w.out <- resp:
		default:
			// Nobody was listening; the answer is dropped rather than queued
			// forever. The companion will ask again another day.
		}
	}
}

func (w *Worker) answer(req Request) (string, error) {
	if w.p == nil {
		return templateFor(req), ErrNoProvider
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	text, err := w.p.Generate(ctx, req)
	if err != nil {
		return "", err
	}
	text = Sanitize(text)
	if text == "" {
		return "", errors.New("ai: empty answer")
	}
	return text, nil
}

func templateFor(req Request) string {
	t, _ := Template{}.Generate(context.Background(), req)
	return t
}

func cacheKey(req Request) string {
	h := sha256.New()
	h.Write([]byte(req.Kind))
	h.Write([]byte{0})
	h.Write([]byte(req.Subject))
	for _, f := range req.Facts {
		h.Write([]byte{0})
		h.Write([]byte(f))
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

func itoa(n int) string {
	if n <= 0 {
		return "12"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
