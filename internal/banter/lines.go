package banter

import (
	"strings"

	"github.com/donlucasx/xscapes/internal/event"
)

// The words. Two creatures, two temperaments, and the same four categories in
// each world.
//
// Everything here is short on purpose: a line is drawn over a creature in a
// terminal beside a coding agent, so it has to be read in a glance and must
// never be mistaken for the agent talking. No line advises a fix -- that is the
// one thing the spectators are not allowed to do.

// primary is the sentence the animal says when it has the stage to itself.
var primary = map[Place]map[Category][]string{
	Shore: {
		CatToolFail: {
			"Well, that went beautifully.",
			"Excuse me?",
			"I think the machine just sneezed.",
			"That wasn't supposed to happen, was it.",
		},
		CatTestFail: {
			"Tests. Plural. Failing.",
			"I'd like to speak to the manager.",
			"The little red words are back.",
			"Nobody touch anything.",
		},
		CatTimeout: {
			"Still waiting...",
			"Did it go get groceries?",
			"Hello? Anybody home?",
			"I could have grown a tree by now.",
		},
		CatMissing: {
			"It's looking for something that isn't there.",
			"Have you checked under the sofa?",
			"That file is imaginary.",
			"Searching for a ghost, this one.",
		},
		CatRecovered: {
			"IT LIVES!",
			"I never doubted you.",
			"Okay. I doubted you a little.",
			"Put that in the history books.",
		},
		CatCelebrate: {
			"WAIT. IT WORKS?",
			"Nobody touch anything.",
			"Screenshot it before it changes its mind.",
			"I knew the plan all along.",
		},
		CatSubagent: {
			"Another one? Who invited them.",
			"Ah. Reinforcements.",
			"Now there are two of them doing... something.",
		},
		CatWrite: {
			"Another file, boldly changed.",
			"Gone. Rewritten. Same as yesterday.",
			"That's going to compile beautifully. Probably.",
		},
		CatBuild: {
			"So... we're not shipping that.",
			"The compiler has concerns.",
			"I would like to speak to the manager.",
			"That's a creative interpretation of success.",
		},
		CatBuildPass: {
			"It compiled. On the first try.",
			"Somebody check the time. This is historic.",
			"No errors. Suspicious, but fine.",
		},
		CatBored: {
			"I have named three of these pebbles.",
			"Wake me when the shell returns a feeling.",
			"I'm going to count the waves. Out loud.",
			"At this point I'm part of the build.",
		},
		CatIdle: {
			"Another day of organised electricity.",
			"We did a thing.",
			"Time for a victory snack.",
			"I'll be over here. Being decorative.",
		},
	},
	Vista: {
		CatToolFail: {
			"I saw that. We all saw that.",
			"The tool call has opinions.",
			"Beautifully done. Truly.",
			"That was not in the plan.",
		},
		CatTestFail: {
			"The tests have concerns.",
			"Red is a choice, I suppose.",
			"I would like to speak to the compiler.",
			"A creative interpretation of success.",
		},
		CatTimeout: {
			"Still waiting.",
			"I have watched a whole sunset.",
			"Is it thinking? Is it stuck? Who knows.",
			"The clock and I are becoming friends.",
		},
		CatMissing: {
			"Searching for something that isn't there.",
			"Nowhere. I checked from up here.",
			"A ghost file. How atmospheric.",
			"Nothing there. Beautifully empty.",
		},
		CatRecovered: {
			"IT LIVES!",
			"I never doubted you.",
			"Dramatic pause: over.",
			"The history books are ready.",
		},
		CatCelebrate: {
			"IT WORKS. STOP EVERYTHING.",
			"Nobody move. It might be shy.",
			"A green run. Frame it.",
			"I predicted this. Ask anyone.",
		},
		CatSubagent: {
			"Someone new on the meadow.",
			"Reinforcements. Marvellous.",
			"They brought their own little tasks.",
		},
		CatWrite: {
			"Another file, changed with confidence.",
			"Words, rearranged. Bold.",
			"I'll be honest: I only read the top line.",
		},
		CatBuild: {
			"So we're not shipping that.",
			"The compiler has opinions.",
			"I would like a word with the build.",
			"Bold of it to fail so completely.",
		},
		CatBuildPass: {
			"It built. Nobody speak.",
			"A clean compile. Frame it.",
			"I felt the ground steady.",
		},
		CatBored: {
			"I have watched the entire sky.",
			"The wind has run out of things to say.",
			"I'm asleep. Professionally.",
			"Wake me when it finishes or dies.",
		},
		CatIdle: {
			"A fine evening for organised electricity.",
			"We did a thing.",
			"The meadow approves.",
			"I shall sit here, professionally.",
		},
	},
}

// second is what the cat or the owlet says when it is its turn.
var second = map[Place]map[Category][]string{
	Shore: {
		CatToolFail:  {"Again?", "How many times now.", "So we're debugging emotionally."},
		CatTestFail:  {"How many failed?", "Red suits you.", "This is a lifestyle now."},
		CatTimeout:   {"It's been a while.", "I've had three naps.", "Is the machine asleep?"},
		CatMissing:   {"It's not there.", "I looked. It's not there.", "Nothing. Not even dust."},
		CatRecovered: {"It came back.", "There it is.", "Fine. I'm impressed."},
		CatCelebrate: {"It works.", "Don't breathe on it.", "I saw nothing."},
		CatSubagent:  {"Who's this?", "You invited someone?", "Great. More feet."},
		CatWrite:     {"You changed it again.", "Bold.", "I liked the old one."},
		CatBuild:     {"Same error, different day.", "The compiler said no.", "Ask it nicely next time."},
		CatBuildPass: {"It built.", "Don't breathe on it.", "I saw nothing."},
		CatBored:     {"Is it alive?", "I've had four naps.", "Still here. Sadly."},
		CatIdle:      {"We did a thing.", "Snack?", "I'm going back to sleep."},
	},
	Vista: {
		CatToolFail:  {"Again?", "That's the same error.", "Details are not important, are they."},
		CatTestFail:  {"How many?", "Twice is a pattern.", "The tests are suspicious of us now."},
		CatTimeout:   {"Still going.", "I counted clouds.", "Is it alive?"},
		CatMissing:   {"It isn't there.", "Nothing. I checked twice.", "An empty path. Poetic."},
		CatRecovered: {"It worked.", "There. Done.", "I said nothing."},
		CatCelebrate: {"Green.", "Don't touch it.", "Marvellous. Terrifying."},
		CatSubagent:  {"Someone arrived.", "Did you order this?", "More owls. More opinions."},
		CatWrite:     {"Changed again?", "Bold edits.", "I only saw the summary."},
		CatBuild:     {"The compiler said no.", "Not even a warning first.", "It had plans of its own."},
		CatBuildPass: {"It compiled.", "Don't touch it.", "Marvellous. Terrifying."},
		CatBored:     {"Still going.", "I completed the sky.", "It's been a season."},
		CatIdle:      {"A thing was done.", "Snack?", "I shall supervise the evening."},
	},
}

// scripts are the conversations: alternating lines, primary first, drawn in
// order until the requested length is reached (a longer escalation loops back to
// the top of the script and lands on its stronger half).
var scripts = map[Place]map[Category][][]string{
	Shore: {
		CatToolFail: {
			{"The tool call failed.", "Again?", "No, this is a completely different failure.",
				"It has the same error message.", "Details are not important."},
			{"It failed.", "How many times?", "Twice.", "So we're debugging emotionally now.",
				"I don't get paid enough for this.", "Neither do I. I live in a shell."},
		},
		CatBuild: {
			{"The build failed.", "It did?", "It did.", "The compiler has spoken.", "Rude.", "Honest, though."},
		},
		CatTestFail: {
			{"The tests are red.", "All of them?", "Most of them.", "That's a theme.",
				"A theme is not a plan.", "It is a plan with feelings."},
			{"Tests failed.", "How many?", "Enough.", "Enough is not a number.",
				"It's a lifestyle.", "It's Tuesday."},
		},
		CatTimeout: {
			{"Still waiting...", "How long has it been?", "Long enough to grow a tree.",
				"Did you water it?", "I was being metaphorical.", "The tree wasn't."},
			{"It's been running a while.", "Is it stuck?", "It's thinking.", "For ten minutes?",
				"It's thinking slowly.", "It's asleep."},
		},
		CatMissing: {
			{"It's looking for a file.", "Is it there?", "No.", "So it's looking for a ghost.",
				"Ghosts are cheap.", "Cheaper than the file."},
			{"Nothing at that path.", "Nothing at all?", "Not even a directory.",
				"Have you tried asking nicely?", "It's a path, not a person.",
				"Everything is a person if you're polite."},
		},
		CatRecovered: {
			{"It worked.", "It worked?", "It worked.", "I never doubted it.",
				"You said it was doomed.", "I said it was doomed BEAUTIFULLY."},
		},
		CatCelebrate: {
			{"The tests passed.", "All of them?", "All of them.", "Nose to nose, then.",
				"Don't touch anything.", "I'm not touching anything."},
		},
		CatSubagent: {
			{"Someone else arrived.", "Who?", "A sub-agent.", "Doing what?",
				"The same thing. In parallel.", "Inefficiently.", "Professionally."},
		},
		CatWrite: {
			{"It wrote a file.", "Which one?", "The important one.", "They're all important.",
				"That's the problem.", "That's the job."},
		},
		CatIdle: {
			{"Another turn done.", "Did we win?", "We did a thing.", "A thing is a thing.",
				"I'll take it.", "Snack?", "Snack."},
		},
	},
	Vista: {
		CatToolFail: {
			{"The tool call failed.", "Again?", "No, this is a completely different failure.",
				"It has the same error message.", "Details are not important."},
			{"It failed.", "How many times?", "Twice.", "So we're debugging emotionally now.",
				"I don't get paid enough for this.", "Neither do I. I live in a terminal."},
		},
		CatBuild: {
			{"The build failed.", "What happened?", "The compiler said no.", "Did you ask nicely?",
				"I sent it a strongly worded JSON object.", "Understandable."},
			{"The build failed.", "It failed.", "It failed again.", "How many times can it fail?",
				"We're about to find out.", "I'll get comfortable."},
		},
		CatTestFail: {
			{"The tests failed.", "Twice now.", "The tests are getting suspicious.",
				"Suspicious of us?", "Suspicious of everything.", "That's how tests start."},
		},
		CatTimeout: {
			{"It's still running.", "How long?", "I watched the sun move.",
				"Did it get anywhere?", "It got further.", "Further where?", "Away."},
		},
		CatMissing: {
			{"It's calling a path that isn't there.", "Empty?", "Completely.",
				"Not even an error message worth reading?", "It's a very honest emptiness."},
		},
		CatRecovered: {
			{"It worked.", "It worked?", "It worked.", "Say nothing.", "Saying nothing.",
				"Good. Nothing is exactly what I'd say."},
		},
		CatCelebrate: {
			{"Everything passed.", "Everything?", "Everything.", "Nobody move.",
				"I'm not moving.", "Perfect. Stay like that."},
		},
		CatSubagent: {
			{"Someone arrived.", "Who?", "A sub-agent.", "Do they know what they're doing?",
				"They know what they were told.", "That's not the same thing.", "It never is."},
		},
		CatWrite: {
			{"It's writing.", "What?", "Something ambitious.", "Ambitious how?",
				"Confidently.", "That's how it starts."},
		},
		CatIdle: {
			{"The turn is over.", "Did we win?", "We did a thing.", "I'll allow it.",
				"Snack?", "Snack."},
		},
	},
}

// show is a one-line reaction.
func (e *Engine) show(cat Category, ev event.Event) *Show {
	line := Line{Who: Primary, Text: e.pick(cat, Primary), At: 0, Hold: 4.5}
	s := &Show{Lines: []Line{line}, End: 5.5}
	if cat == CatToolFail || cat == CatTestFail || cat == CatBuild {
		s.Prop, s.PropAt = Smoke, 0.4
	}
	if cat == CatCelebrate || cat == CatRecovered || cat == CatBuildPass {
		s.Prop, s.PropAt = Sparkle, 0.2
	}
	if cat == CatBored {
		// A creature with a tiny sign, because a stall is now furniture.
		s.Prop, s.PropAt = Sign, 0.3
	}
	return s
}

// conversation brings the second creature on and gives them both a turn.
func (e *Engine) conversation(cat Category, ev event.Event, want int) *Show {
	script := e.script(cat)
	s := &Show{Second: true, SecondAt: 0.7}
	at := 0.9
	for i := 0; i < want && len(script) > 0; i++ {
		who := Primary
		text := ""
		if i%2 == 0 {
			text = script[(i/2)%len(script)]
			if i/2 >= len(script) {
				text = script[len(script)-1]
			}
			// Every other primary turn comes from the solo bank, so a long
			// escalation does not repeat the same script sentence word for word.
			if i >= len(script) {
				text = e.pick(cat, Primary)
			}
		} else {
			who = Second
			text = e.pick(cat, Second)
		}
		hold := 2.4
		if len(text) > 34 {
			hold = 3.0
		}
		s.Lines = append(s.Lines, Line{Who: who, Text: text, At: at, Hold: hold})
		at += hold + 0.35
	}
	s.End = at + 2.2
	if want >= 6 {
		s.Prop, s.PropAt = Smoke, 0.5
	}
	return s
}

func (e *Engine) script(cat Category) []string {
	bank := scripts[e.place][cat]
	if len(bank) == 0 {
		return nil
	}
	return bank[e.roll(len(bank))]
}

// pick cycles the variants of a category, seeded, so the same failure twice does
// not produce the same sentence twice in a row.
func (e *Engine) pick(cat Category, who Who) string {
	bank := primary[e.place][cat]
	key := string(cat) + "/p"
	if who == Second {
		bank = second[e.place][cat]
		key = string(cat) + "/s"
	}
	if len(bank) == 0 {
		return "..."
	}
	i := e.lastLine[key]
	v := bank[i%len(bank)]
	e.lastLine[key] = i + 1
	return v
}

// catOf is the category a show was built from, for the caller that wants to know
// what just happened; the engine records it when it starts one.
func (e *Engine) LastCategory() Category { return e.lastCat }

// LastRung is how many times the failure behind the current show had been heard
// when it started: 1 for the first, 2 for the conversation, 3 and up for the
// louder one. Zero when the show was not an escalation.
func (e *Engine) LastRung() int { return e.lastRung }

// FitLine trims a line to a width a scene can draw, on a word boundary.
func FitLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)[:max]
	for i := len(r) - 1; i > max/2; i-- {
		if r[i] == ' ' {
			return string(r[:i])
		}
	}
	return string(r)
}
