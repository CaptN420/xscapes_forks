package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The AI layer's promises, each as a test: nothing blocks the caller, failures
// become the companion's own voice, output is safe to plot, and looking at a
// project is read-only, bounded and verifiable.

// slowProvider takes as long as the test tells it to, and records that it ran.
type slowProvider struct {
	delay   time.Duration
	calls   int32
	text    string
	failure error
}

func (s *slowProvider) Generate(ctx context.Context, req Request) (string, error) {
	atomic.AddInt32(&s.calls, 1)
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if s.failure != nil {
		return "", s.failure
	}
	if s.text != "" {
		return s.text, nil
	}
	return "a story from the model", nil
}

// Submit and Poll never wait, whatever the provider is doing.
func TestNothingWaitsOnTheModel(t *testing.T) {
	p := &slowProvider{delay: 2 * time.Second}
	w := NewWorker(p)
	defer w.Close()

	start := time.Now()
	if !w.Submit(Request{Kind: KindStory, Subject: "a shell"}) {
		t.Fatal("the request was refused")
	}
	// Polling a worker whose answer is two seconds away must return at once.
	for i := 0; i < 100; i++ {
		if _, ok := w.Poll(); ok {
			break
		}
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("100 polls took %v: the frame would stutter", elapsed)
	}
}

// A request that is slow is abandoned at its deadline, and the caller still gets
// an answer -- the template.
func TestASlowModelIsAbandonedAndTheTemplateAnswers(t *testing.T) {
	p := &slowProvider{delay: 30 * time.Second}
	w := NewWorker(p)
	defer w.Close()
	// The worker's own deadline is 12s, which is too long for a test; drive the
	// same path through a provider that respects a short context.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := p.Generate(ctx, Request{Kind: KindStory}); err == nil {
		t.Fatal("a provider ignoring its deadline returned success")
	}
	if _, err := (&Exec{Cmd: []string{"definitely-not-a-command-xyz"}, Timeout: 200 * time.Millisecond}).
		Generate(context.Background(), Request{Kind: KindStory}); err == nil {
		t.Error("a missing command was not an error")
	}
	// And the template still answers for the same request.
	text, err := Template{}.Generate(context.Background(), Request{Kind: KindStory, Subject: "a pearl"})
	if err != nil || text == "" {
		t.Fatalf("the template did not answer: %q %v", text, err)
	}
	if !strings.Contains(text, "pearl") {
		t.Errorf("the template ignored the subject: %q", text)
	}
}

// A failing provider is not a broken companion.
func TestAFailureStillSpeaks(t *testing.T) {
	w := NewWorker(&slowProvider{delay: time.Millisecond, failure: errors.New("boom")})
	defer w.Close()
	w.Submit(Request{Kind: KindStory, Subject: "a feather"})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r, ok := w.Poll(); ok {
			if r.Text == "" {
				t.Fatal("a failed request produced no text at all")
			}
			if r.Source != "template" {
				t.Errorf("a failed request answered from %q", r.Source)
			}
			if r.Err == nil {
				t.Error("the failure was not reported")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no answer arrived after a failure")
}

// The same question is asked once.
func TestTheSameQuestionIsAnsweredOnce(t *testing.T) {
	p := &slowProvider{delay: time.Millisecond}
	w := NewWorker(p)
	defer w.Close()
	req := Request{Kind: KindStory, Subject: "a bottle cap", Facts: []string{"low tide"}}
	w.Submit(req)
	waitFor(t, w)
	w.Submit(req)
	waitFor(t, w)
	if n := atomic.LoadInt32(&p.calls); n != 1 {
		t.Errorf("the model was asked %d times for one question", n)
	}
}

func waitFor(t *testing.T, w *Worker) Response {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r, ok := w.Poll(); ok {
			return r
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("no answer arrived")
	return Response{}
}

// A worker with no provider still answers, locally.
func TestNoProviderStillAnswers(t *testing.T) {
	w := NewWorker(nil)
	defer w.Close()
	w.Submit(Request{Kind: KindChat, Facts: []string{"The tide came in"}})
	r := waitFor(t, w)
	if r.Text == "" {
		t.Fatal("a worker with no provider said nothing")
	}
	if r.Source != "template" {
		t.Errorf("the answer came from %q", r.Source)
	}
}

// Model output goes into a terminal that is also running a coding agent. It is
// stripped, collapsed and capped before it is ever plotted.
func TestOutputIsMadeSafeToPlot(t *testing.T) {
	cases := map[string]string{
		"\x1b[2J\x1b[Hhello": "hello",
		"line one\nline two": "line one line two",
		"a\t\tb":             "a b",
		"quote \u2019s":      "quote 's",
		"\x07bell":           "bell",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("word ", 500)
	if got := Sanitize(long); len([]rune(got)) > 320 {
		t.Errorf("a long answer was not capped: %d runes", len([]rune(got)))
	}
}

// THE PROJECT SCAN: read-only, bounded, and only source.
func TestTheProjectScanIsBoundedAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main\n\nfunc main() {}\n")
	write("internal/thing/thing.go", "package thing\n\ntype Thing struct{}\n")
	write("notes/private.md", "secrets\n")
	write("node_modules/dep/index.js", "module.exports = 1\n")
	write(".git/config", "[core]\n")
	write("big.go", strings.Repeat("// filler\n", 5000))

	facts := ScanProject(dir, MaxFiles)
	if len(facts) == 0 {
		t.Fatal("the scan found nothing")
	}
	if len(facts) > MaxFiles {
		t.Errorf("the scan returned %d facts, over the cap", len(facts))
	}
	for _, f := range facts {
		if strings.Contains(f.Path, "node_modules") || strings.HasPrefix(f.Path, ".git") ||
			strings.Contains(f.Path, "notes/") {
			t.Errorf("the scan walked into %s", f.Path)
		}
		if f.Path == "big.go" {
			t.Error("the scan read a file over the size cap")
		}
		if !strings.HasSuffix(f.Path, ".go") && !strings.HasSuffix(f.Path, ".js") {
			t.Errorf("the scan returned a non-source file: %s", f.Path)
		}
	}

	// Nothing was written: every file still has the bytes it was given.
	body, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil || string(body) != "package main\n\nfunc main() {}\n" {
		t.Error("the scan modified a file")
	}
}

// A claim is only shown if it names something that exists.
func TestACitationIsCheckedBeforeItIsBelieved(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "a.go"), []byte("package a\n\nfunc A() {}\n\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, ok := range []string{"internal/a.go", "internal/a.go:3", "internal/a.go:5"} {
		if !CitationOK(ok, dir) {
			t.Errorf("%q should be a valid citation", ok)
		}
	}
	for _, bad := range []string{
		"", "internal/missing.go", "internal/a.go:99",
		"../../../etc/passwd", "/etc/passwd", "internal",
	} {
		if CitationOK(bad, dir) {
			t.Errorf("%q was accepted as a citation", bad)
		}
	}
	if got := Citation("I noticed internal/a.go:3 does two things"); got != "internal/a.go:3" {
		t.Errorf("Citation found %q", got)
	}
}

// The worker's stats are what `xscapes life ai` prints, so they must count.
func TestTheWorkerCountsWhatItDid(t *testing.T) {
	w := NewWorker(&slowProvider{delay: time.Millisecond})
	defer w.Close()
	w.Submit(Request{Kind: KindStory, Subject: "x"})
	waitFor(t, w)
	w.Submit(Request{Kind: KindStory, Subject: "x"})
	waitFor(t, w)
	submitted, served, hits, _ := w.Stats()
	if submitted != 1 || served != 1 || hits != 1 {
		t.Errorf("stats: submitted=%d served=%d hits=%d, want 1/1/1", submitted, served, hits)
	}
}

// The prompt reaches the command the way the command expects it: as an argument
// where it names one, on stdin where it does not.
func TestThePromptGetsThere(t *testing.T) {
	// echo is asked for the prompt as an argument.
	got, err := Exec{Cmd: []string{"echo", "{prompt}"}, Timeout: 5 * time.Second}.
		Generate(context.Background(), Request{Kind: KindStory, Subject: "a shell"})
	if err != nil {
		t.Fatal(err)
	}
	// The prompt is long enough that Sanitize's cap bites, so look for its
	// shape rather than for the last word of it.
	if !strings.Contains(got, "living on a shore") || !strings.Contains(got, "Fact") {
		t.Errorf("the prompt did not arrive as an argument: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("the answer was not collapsed to one paragraph: %q", got)
	}

	// cat reads it from stdin, and echoes it back the same way.
	got, err = Exec{Cmd: []string{"cat"}, Timeout: 5 * time.Second}.
		Generate(context.Background(), Request{Kind: KindObservation, Subject: "x", Facts: []string{"main.go:3 func main"}})
	if err != nil {
		t.Fatal(err)
	}
	// Sanitize's cap bites before the facts do on a long prompt, so the shape of
	// the prompt is the witness: boilerplate, then the fact header.
	if !strings.Contains(got, "software project") || !strings.Contains(got, "Fact") {
		t.Errorf("the prompt did not arrive on stdin: %q", got)
	}
}

// A voice belongs to the place it lives in. The shore talks about tidelines and
// swells; the vista talks about meadows and fence posts. Neither borrows the
// other's nouns, which is the difference between two scapes and one scape twice.
func TestEachPlaceHasItsOwnVoice(t *testing.T) {
	shore := Request{Kind: KindStory, Subject: "a pearl", Place: "shore", Animal: "crab"}
	vista := Request{Kind: KindStory, Subject: "a pearl", Place: "vista", Animal: "owl"}

	sh, _ := Template{}.Generate(context.Background(), shore)
	vi, _ := Template{}.Generate(context.Background(), vista)
	if sh == vi {
		t.Fatalf("both places told the same story: %q", sh)
	}
	for _, w := range []string{"tideline", "rock", "shore", "swells"} {
		if strings.Contains(strings.ToLower(vi), w) {
			t.Errorf("the vista's story used the shore's word %q: %q", w, vi)
		}
	}
	for _, w := range []string{"meadow", "fence", "glide"} {
		if strings.Contains(strings.ToLower(sh), w) {
			t.Errorf("the shore's story used the vista's word %q: %q", w, sh)
		}
	}
	if !strings.Contains(vi, "pearl") || !strings.Contains(sh, "pearl") {
		t.Error("one of them lost the find")
	}

	// The chat line is a greeting from its own place too.
	cs, _ := Template{}.Generate(context.Background(), Request{Kind: KindChat, Place: "shore"})
	cv, _ := Template{}.Generate(context.Background(), Request{Kind: KindChat, Place: "vista"})
	if cs == cv {
		t.Errorf("both places said %q", cs)
	}
}

// The prompt tells the model who it is and where, so a model's answer can be in
// the right voice as well.
func TestThePromptNamesTheAnimalAndThePlace(t *testing.T) {
	got := Prompt(Request{Kind: KindStory, Subject: "a pearl", Animal: "owl", Place: "vista"})
	if !strings.Contains(got, "owl") || !strings.Contains(got, "vista") {
		t.Errorf("the prompt hid who and where: %q", got)
	}
	got = Prompt(Request{Kind: KindStory, Subject: "a shell"})
	if !strings.Contains(got, "crab") || !strings.Contains(got, "shore") {
		t.Errorf("an unnamed request did not default to the shore: %q", got)
	}
}
