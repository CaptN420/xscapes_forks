package main

import (
	"strings"
	"time"

	"github.com/donlucasx/xscapes/internal/banter"
	"github.com/donlucasx/xscapes/internal/envx"
	"github.com/donlucasx/xscapes/internal/life/ai"
)

// The optional rewrite: a model may reword ONE sentence of a show before it is
// read. Nothing else about the comedy is its business.
//
//	XSCAPES_BANTER_AI=off     (default) the written lines, always
//	XSCAPES_BANTER_AI=exec    ask XSCAPES_BANTER_AI_CMD for a line per show
//
// The engine stays deterministic and complete without it: a refused, failed or
// slow answer changes nothing at all, because the line it would have replaced is
// already good.

var (
	banterAI      *ai.Worker
	banterAIOn    bool
	banterLast    *banter.Show
	banterAISent  map[string]bool
	banterAIAsked int
	banterAILast  time.Time
)

func initBanterAI() {
	banterAISent = map[string]bool{}
	banterAIAsked = 0
	banterAILast = time.Time{}
	mode := strings.ToLower(strings.TrimSpace(envx.Lookup("BANTER_AI")))
	if mode != "exec" {
		banterAI, banterAIOn = nil, false
		return
	}
	cmd := strings.TrimSpace(envx.Lookup("BANTER_AI_CMD"))
	if cmd == "" {
		cmd = "hermes -z {prompt}"
	}
	banterAI = ai.NewWorker(ai.Exec{Cmd: strings.Fields(cmd), Timeout: 20 * time.Second})
	banterAIOn = true
}

func closeBanterAI() {
	if banterAI != nil {
		banterAI.Close()
		banterAI = nil
	}
	banterAIOn, banterLast = false, nil
}

// banterWatch hands a fresh show to the model, and applies a rewrite if one comes
// back while the sentence is still unread.
func banterWatch(now time.Time) {
	if !banterAIOn || banterAI == nil || !banterLive() {
		return
	}
	// Drain first.
	for {
		res, ok := banterAI.Poll()
		if !ok {
			break
		}
		if res.Text != "" {
			banterEngine.Restyle(res.Text, now)
		}
	}
	show := banterEngine.Now(now)
	if show == nil || show == banterLast {
		return
	}
	banterLast = show
	if len(show.Lines) == 0 {
		return
	}
	cat := banterEngine.LastCategory()
	rung := banterEngine.LastRung()
	// Bounded, not banned: a handful of asks per session, spaced out, and each
	// distinct moment asked about once. A model call per error is exactly what
	// this must not become; a session where the first failure is the only one
	// ever reworded is also worse than the written line on every third or so.
	key := string(cat) + "|" + string(banterEngine.Place()) + "|" + lifeNum(rung)
	if banterAISent[key] || banterAIAsked >= aiCap {
		return
	}
	if !banterAILast.IsZero() && now.Sub(banterAILast) < aiSpacing {
		return
	}
	banterAISent[key] = true
	banterAIAsked++
	banterAILast = now
	banterAI.Submit(ai.Request{
		Kind:     ai.KindChat,
		Subject:  "one funny line",
		MaxWords: 14,
		Animal:   helpAnimal(),
		Place:    string(banterEngine.Place()),
		Facts:    []string{describeMoment(banterEngine.Place(), cat, rung), describeShow(show)},
	})
}

const (
	// aiCap is how many rewrites one session may ask for in total.
	aiCap = 8
	// aiSpacing keeps two asks from landing in the same minute.
	aiSpacing = 75 * time.Second
)

// describeMoment is the whole of what the model is told about the event: a
// category, how many times it has been heard, and the world it happened in.
// No target, no path, no command, no output -- there is nothing secret in those
// fields today, and a spectator does not need them to be funny.
func describeMoment(place banter.Place, cat banter.Category, rung int) string {
	world := "a crab on a beach at the edge of a terminal"
	if place == banter.Vista {
		world = "an owl on a meadow above a terminal"
	}
	what := map[banter.Category]string{
		banter.CatToolFail:  "a tool call failed",
		banter.CatTestFail:  "the tests failed",
		banter.CatBuild:     "the build failed",
		banter.CatBuildPass: "the build succeeded",
		banter.CatTimeout:   "a command has been running for a long time",
		banter.CatBored:     "a command has been running for so long that waiting has become a lifestyle",
		banter.CatMissing:   "a command is looking for a file that is not there",
		banter.CatRecovered: "something that failed twice just worked",
		banter.CatCelebrate: "the tests passed",
		banter.CatSubagent:  "another agent just joined the session",
		banter.CatWrite:     "a file was edited",
		banter.CatIdle:      "a turn just ended and nothing is happening",
	}[cat]
	if what == "" {
		what = "something happened in a coding session"
	}
	line := "Setting: " + world + ". Situation: " + what + "."
	if rung == 1 {
		line += " This is the first time."
	} else if rung > 1 {
		line += " This keeps happening; the creatures have commented before."
	}
	return line
}

// describeShow says what the moment looks like on stage, in words that carry no
// code, no path and no output.
func describeShow(s *banter.Show) string {
	who := "the creature"
	switch {
	case len(s.Lines) > 1 && s.Second:
		who = "two creatures on a calm evening"
	case s.Second:
		who = "a creature and a friend who just arrived"
	}
	return "Stage: " + who + ". Existing line: \"" + s.Lines[0].Text +
		"\". Write a different, funnier line with the same meaning and length."
}

func helpAnimal() string {
	if banterEngine == nil {
		return "crab"
	}
	if banterEngine.Place() == banter.Vista {
		return "owl"
	}
	return "crab"
}
