# The companion beta — architecture and integration points

Branch: `feature/companion-beta`. Baseline commit before any of this:
`974b169 Baseline before the companion beta`.

## What was inspected, and what it decided

- `internal/scape` — scenes read only an `Activity` (working, level, context,
  tokens, time of day, todos). A scene never learns an agent's vocabulary.
- `frames.go` — one composition path: `reduce.State` → `Activity` → `f.compose`.
  Both the live loop and the tests go through it, which is why every claim below
  can be tested without a terminal.
- `internal/companion` — the crab and the cat, `Draw(layer, x, y, t, State)`, five
  faces, a 12x7 box whose size the layout depends on.
- `internal/scenes` — the study painters; the vista's owl is
  `DrawOwlPose`/`DrawOwlMoving` with the SAME five faces, in the same 12x7 box.
- `internal/life` — the Phase 10 companion engine: pure, deterministic, no I/O in
  the brain, no punishment by construction.
- `internal/tamagotchi` — the earlier creature's save format, read once by a
  migration.

## Module structure

```
internal/life/            the engine (Layer A). No canvas, no reduce, no net.
  life.go                 Brain, World, Intent, poses, stages, bouts, excursions
  food.go                 the larder, the five reactions, the seeded variants
  words.go                finds, toys, the quiet observations
  store.go                life.json v2, atomic write, migration from the old pet

internal/scenes/owl_react.go   what a reaction looks like (Layer A's art)
internal/companion/crab_grow.go  the crab's stage sizes

life_integration.go       the bridge: switch, file, World snapshot, pose mapping,
                          the speech line, the finds on the sand
life_cmd.go               `xscapes life ...`
life_owl_test.go          the owl in the composited frame
```

Nothing in `internal/reduce`, `internal/scape`, `internal/event`, the host or the
Windows layer was changed for this feature.

## Integration points (all of them)

| Where | What it does | Cost when the switch is off |
|---|---|---|
| `frames.go composeFrame` | `updateLife(now, st)` once per frame | one `if` |
| `frames.go newFrames` | `life:` and `tamagotchi:` from the environment | two reads at start-up |
| `live.go drawScene` | pose + wander + coat + scale + speech + finds | one `if` |
| `vista_live.go drawVistaWith` | the owl's pose, hop, reaction, speech | one `if` |
| `live.go` / `inside.go` | `initLife()` + `saveLife()` on exit | two calls |
| `cmd.go` | `xscapes life` | one case |

## The two channels (do not merge them)

```
POSITION  how far the animal has stepped in with the agent's work (pace.go)
POSE      what it does with its own time (the life brain)
```

A screenshot reads both. The wander is clamped to `lay.PaceSpan`, the strip the
layout already reserves, because the sand and the litter are laid out against it.

## Switch

```
XSCAPES_LIFE=off             no companion (rendering identical to the baseline)
XSCAPES_LIFE_INTERACT=off    it lives, nothing may touch it
XSCAPES_TAMAGOTCHI=on        the earlier creature as well (legacy)
```

## Beta 0.1 — what it does

- One animal per scape: the crab on the shore, the owl on the vista. Same brain.
- Idle life: bouts (groom, dig, watch, rest, doze, moon-gaze, be wary) on a
  7-26 s clock; observations every 10-25 quiet minutes; rare self-initiated
  excursions that return with a find.
- Interactions: feed (five foods), pet, play, walk, explore, toy, call — each
  answers within a frame, each is a positive experience.
- Food reactions: berry → dance, beetle → catch it midair, spicy → a breath of
  ascii fire, mystery mushroom → dizzy rings, treat → sparks and a hop. Seeded
  variants so two helpings differ while a replay does not.
- Adventures: 40-150 s away, always come back with something, always positive.
- Growth: time only (12 / 35 / 80 min); engagement multiplies up to 1.5x; nothing
  can reduce, skip or punish.
- Persistence: `~/.config/xscapes/life.json`, written on exit and after each
  command; loaded on start.

## Beta 0.2 — the AI layer (built, model not yet driven live)

```
internal/life/ai/ai.go        Provider, Template, Exec, Worker, Sanitize
internal/life/ai/project.go   the read-only project glance and citation check
life_ai.go                    the wiring: modes, drain, when to ask
```

Design as built:

- `Provider` is one method, so a sub-agent, a local model or a test all fit:
  `Generate(ctx, Request) (string, error)`.
- `Exec` runs `XSCAPES_LIFE_AI_CMD` (default `hermes -z`) with the prompt on
  stdin, a 12 s deadline and the answer on stdout. A missing binary is an error,
  not a crash -- verified.
- `Worker`: buffered request queue, one goroutine, results on a channel the frame
  polls. `Submit` and `Poll` never block; the queue drops when busy, which at
  companion pace means "one is already in flight".
- A failed or slow request answers with the template, so the companion speaks in
  its own voice rather than going quiet.
- Caching by (kind, subject, facts) hash, 32 entries, LRU-ish eviction.
- `Sanitize` is applied to everything a provider returns: ANSI sequences,
  control characters, newlines and smart quotes are removed before the text can
  reach a terminal that is also running a coding agent.
- The project glance is bounded (12 files, 24 KB each, source extensions, no
  `.git`, no `node_modules`, no `vendor`, no symlink climbing) and read-only.
  An observation is shown only if `path:line` in it resolves to a real file and a
  real line; otherwise it is dropped silently.

Measured on this machine: `hermes -z "<prompt>"` returns in 7.1 s for a trivial
prompt and 9.3 s for a real one, through the whole chain (prompt as an argument,
answer sanitised to one paragraph, story kept, `life story` reads it back after a
restart). So the request deadline is 20 s and the command-line wait is 25 s; a
frame still never waits, it collects the answer when it arrives.

`xscapes life ai --check` does one round trip on demand and prints the latency
and the source: template 0 s, and through `hermes -z` 4.9 s for the shore (crab)
and 8.7 s for the vista (owl), both one sanitised line. A broken command exits 1
and shows the error, having already answered in its own voice.

The voice belongs to the place. `Request` carries `Animal` and `Place`, filled
from the running scape (`sceneVoice`), and both the template and the prompt use
them, so the owl on the vista never mentions a tideline and the crab never glides
over a meadow. Held by tests in `internal/life/ai` and by one in the root package
that asks the vista for a story and fails if it says "sand".

The prompt goes where the command wants it: `{prompt}` in `XSCAPES_LIFE_AI_CMD`
passes it as an argument (`hermes -z {prompt}`, which is the default), and a
command without the placeholder gets it on stdin (`cat`, a filter, a script).

What is still NOT done: no live model call has been made from the worker -- the exec
path is exercised with a missing command (failure) and the template path covers
every test. The first real `hermes -z` round trip is the next thing to try, and
it should be tried with `XSCAPES_LIFE_PROJECT=off` first so nothing about the
project leaves the machine until the pipeline is trusted.


## Compatibility risks seen

1. **The vista fingerprint.** `TestTheStudyVistaIsUnchanged` compares the study
   painter byte for byte and was already failing before this branch (the study
   and the product have drifted). The product path is covered by `life_owl_test`.
2. **Two animals at once.** `XSCAPES_TAMAGOTCHI=on` plus the life layer draws the
   old creature in the sky over the new one. Deliberate: legacy is opt-in.
3. **Windows behaviour.** No new platform code. The life layer touches no socket,
   no PTY, no console API; the only I/O is one JSON file under `~/.config`.
4. **CRLF.** Every source file in this tree uses CRLF; a line-based edit that
   matches on `\n` silently does nothing. Use the editor's patch mode.
5. **No git history existed** in this working copy; the baseline commit added one
   so this branch can be diffed and reverted as a unit.
