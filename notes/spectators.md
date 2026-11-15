# The spectators — event-driven reactions

`internal/banter` (the engine), `banter_draw.go` (the drawing), `banter_ai.go`
(the optional rewrite). Branch `feature/companion-beta`.

## The one rule

They watch. They do not help. The package imports `time`, `strings`, `strconv`
and `internal/event` and nothing else, and a test parses its own imports to keep
it that way — no `os`, no `os/exec`, no network, no file writing. There is no
code path in which a creature could modify, run or repair anything, because there
is no code path that can reach a file.

## What the events actually are

The brief listed eighteen event names. These are the ones this program receives,
and the rest do not exist here — so they are not faked:

| brief | here |
|---|---|
| `tool_call_started` | `tool_start` |
| `tool_call_succeeded` | `tool_end` (no error) |
| `tool_call_failed` | `error` |
| `test_failed` / `test_passed` | `test_fail` / `test_pass` |
| `file_modified` | `tool_end` with `op=write` or `op=edit` |
| `task_started` / `task_completed` | `prompt` / `done` |
| `agent_spawned` / `agent_completed` | `sub_start` / `sub_end` |
| `tool_call_timeout` | **not an event**: a `tool_start` with no end is noticed by the clock after 40 s |
| `build_failed`, `build_succeeded` | not distinguishable; a failing shell command is `error` and is described as that |
| `repeated_error` | derived: the same op+tool+target failing again within 8 min |
| `unexpected_output` | not available; never claimed |

Metadata used: `Kind`, `Op`, `Tool`, `Target`, `ID`, `Detail`, `Text`, `Agent`.
`Text` is only ever read for one narrow purpose (does the error say "no such
file"), and nothing from it is sent anywhere.

## The ladder

```
1st failure of op+tool+target   one line, one cloud
2nd within 8 min                the friend walks in; 4 lines between them
3rd and beyond                  longer, with a cloud over the pair
success on the same key         both celebrate, ladder cleared
8 min of quiet                  the episode is forgotten
```

Anti-spam, all constants in `banter.go`:

- the same event delivered twice within 900 ms is one event;
- a show never overlaps another;
- 9 s between unrelated shows, 3 s between rungs of the same escalation;
- 5 shows a minute, and a blocked rung is not consumed (the ladder is only
  climbed when the rung is heard);
- one timeout remark per tool call, ever.

## The two creatures

Existing art only. Shore: the crab and the cat (slate coat, the sprite that
ships). Vista: the owl and an owlet (the litter sprite). Personalities: the crab
is blunt, the cat is unimpressed; the owl is dramatic and the owlet is far too
earnest. Each world has its own bank for every category, and a test fails if the
two worlds are the same script.

## Optional AI

`XSCAPES_BANTER_AI=exec` lets a model reword ONE sentence of a show, before it
has been read (the first two seconds), and nothing else: no new lines, no new
creatures, no category change, refused if it is empty or too long to draw. One
ask per (category, place) per session, cached by the shared worker, and a refusal
costs nothing because the written line is already on screen. The facts sent are
the situation and the existing line — no paths, no code, no output.

## Switch

`XSCAPES_BANTER=off` — no engine, no worker, no drawing; a test holds the frame
to that promise. `banterOn` is read once at start-up, like the other switches.
