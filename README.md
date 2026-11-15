<p align="center">
  <img src="assets/readme/cover.gif" alt="xscapes: a thinking screen for terminal agents" width="100%">
</p>

<p align="center">
  <b>Living ASCII scenes that react as your agent works.</b><br>
  Your agent runs at the top of the window, the scene underneath shows what it is doing, and the companion knocks when it needs you.
</p>

<p align="center"><a href="https://donlucasx.github.io/xscapes">site</a> · <a href="https://donlucasx.github.io/xscapes/manual.html">manual</a> · <a href="https://github.com/donlucasx/hermes-xscapes">hermes plugin</a></p>

## Install

macOS and Linux:

```sh
curl -fsSL https://donlucasx.github.io/xscapes/install.sh | sh
```

Windows (PowerShell, no administrator rights):

```powershell
irm https://donlucasx.github.io/xscapes/install.ps1 | iex
```

Both check the download's SHA-256 against the release before installing.
Full guide, options, uninstall and troubleshooting: [docs/INSTALL.md](docs/INSTALL.md).
What was audited and what is not built yet: [docs/AUDIT.md](docs/AUDIT.md).

Then run your agent inside the scene:

```sh
xscapes install claude --apply && xscapes claude    # Claude Code
xscapes install kimi --apply   && xscapes kimi      # Kimi Code CLI
hermes plugins install donlucasx/hermes-xscapes --enable && xscapes hermes    # Hermes Agent
xscapes inside <any command>                        # anything else
```

`install` without `--apply` only prints what it would change, and `uninstall`
puts your settings back. Every command and option is in the
[manual](https://donlucasx.github.io/xscapes/manual.html).

### Windows

Needs Windows 10 1809 or later (the scene runs the agent on a pseudoconsole).
The Windows package includes `xscapes.exe`, so you can run it without Go. From
a clone, double-click `start.bat`: it builds with Go 1.27+ when possible, and
falls back to the bundled executable if Go is missing or the build fails. The
menu offers Hermes, Claude, Kimi, a shell, hook installation, and a local
server for the manual. By hand, no tmux, no install.sh:

```powershell
go build -o xscapes.exe .
.\xscapes.exe install hermes --apply      # writes hooks into your Hermes profile
.\xscapes.exe hermes                      # then run Hermes inside the scape
.\xscapes.exe inside <any command>        # anything else
```

`install hermes` finds the config your agent actually reads: `$HERMES_HOME`,
else the profile the desktop app runs (`%LOCALAPPDATA%\hermes\profiles\<name>`),
else `%LOCALAPPDATA%\hermes`. Hermes asks for consent the first time each hook
fires; `hermes hooks doctor` answers once for all of them. What the port had to
change, and how each piece was verified, is in
[notes/windows-conpty.md](notes/windows-conpty.md).

## What you see

<table>
  <tr>
    <td width="50%"><img src="assets/readme/vista-timelapse.gif" alt="the mountain vista"><br><b>The vista.</b> The wind and the fire are the work. Owlets arrive for subagents.</td>
    <td width="50%"><img src="assets/readme/shore.gif" alt="the shore"><br><b>The shore.</b> The sea rises with the work, and the sand says what it is doing.</td>
  </tr>
  <tr>
    <td><img src="assets/readme/rain.gif" alt="the harbour"><br><b>The harbour.</b> Rain on the window is the work. A thought cloud means it is thinking.</td>
    <td><img src="assets/readme/crab-pings-you.gif" alt="the crab asks"><br><b>It needs you.</b> The companion walks up and asks, with its own sound.</td>
  </tr>
</table>

The moon is your context and sinks as it fills. Finished todos light up as
stars. The sky follows your clock. Switch with `xscapes scape vista|shore|harbour|rain`
and `xscapes companion crab|cat`.

## The companion

The crab on the sand lives its own life. It dozes at night, grooms its shell,
pokes about in the sand, watches the water when the agent is busy, looks up when
your context is nearly full, and goes quiet when something is broken. None of
this needs you: **a companion, not a responsibility.**

**Nothing it does can hurt it.** It cannot die, it cannot get sick, no need
declines while you are away, and it never asks you for anything. Leave it alone
for a day and you will find an animal that has slept, wandered and grown up a
little, and nothing worse.

**It grows.** Four stages, earned by time alone:

| stage | it takes | what changes |
|---|---|---|
| hatchling | ~12 min in front of you | pale coat, small and busy, quick to startle |
| youngster | ~35 min | brighter coat, more curiosity, watches more |
| grown | ~1 h 20 min | deeper colour, calmer, notices the world |
| old friend | — | the crab's own deep red, slow, settled |

Time is the only gate. Interacting with it is a bonus: engagement runs growth up
to half again as fast, so a companion you play with reaches its old age in about
two thirds of the time, and one you ignore still gets there.

It literally gets bigger: the hatchling is drawn at a little over half the adult's
size and grows through three steps into the crab itself. It is the same sprite,
resampled inside the box the layout reserves for it, so the feet stay on the sand
and the agent's balloon still finds its face.

**Its own time** is a separate channel from the agent's, which is the rule the
whole scene obeys:

```
POSITION   how far it has stepped in from home = how much the agent has done
POSE       what it is doing with its own time  = its own life
```

A screenshot still reads both, and the wander is limited to the one strip of
columns the layout already reserves, so it can never write over the sand.

**Food.** `xscapes life feed berry|bug|spicy|mushroom|treat` (or no name for
whatever is at hand). Each one produces its own foolish reaction -- a berry is a
wobble, a beetle is snapped out of the air, a spicy snack is a breath of ascii
fire, a mystery mushroom is a moment of colours, a special treat is an
unrequested celebration. `xscapes life foods` shows the larder and what it has
tasted. There is no bad food, no overfeeding, and nothing to get wrong.

**Interactions, all optional, all offline:**

```sh
xscapes life                  # where it is in its life, and what it is doing
xscapes life feed             # something to eat
xscapes life pet              # a hand on its shell
xscapes life play             # a game
xscapes life walk             # a stroll along the tideline
xscapes life explore          # send it out; it comes back with something
xscapes life toy              # give it a toy
xscapes life call             # back to its spot
xscapes life finds            # everything it has brought home
xscapes life toys             # what it has been given
xscapes life memory           # the moments it keeps
```

Each one answers within a frame, in its pose and in a line under its feet.
`explore` sends it away for a minute or two; it comes back with a find -- a
shell, a stone, a pearl, a bottle cap -- which is drawn on the sand beside it in
the scene's own one-glyph language, and kept in `life finds`. It also wanders off
on its own now and then, when nothing is happening, and comes back with
something. **Adventures never fail, and never cost anything.**

**The spectators.** A failed tool call is not just water: the creature looks up
and says something about it. Fail the same thing twice and the cat walks in on
the shore (an owlet on the vista) and the two of them discuss it -- badly, and
with increasing drama on the third. A fix is celebrated. A tool that runs too
long gets noticed. They never touch your code, never suggest a fix, and never
interrupt: everything is a line over a creature in the scene's own glyphs.

```
XSCAPES_BANTER=off           no commentary at all
XSCAPES_BANTER_AI=exec       let a model reword one line per situation
                             (XSCAPES_BANTER_AI_CMD, default `hermes -z {prompt}`)
```

What they react to is what the scene already receives: `tool_start`, `tool_end`,
`error`, `test_pass`, `test_fail`, `sub_start`, `done`. A subagent's errors are
its own business, a burst of failures is one reaction rather than six, and
nothing is said twice.

**Settings.** It is on by default:

```
XSCAPES_LIFE=off             no companion at all
XSCAPES_LIFE_INTERACT=off    it lives; nothing may touch it
XSCAPES_TAMAGOTCHI=on        the earlier, separate creature as well (legacy)
```

**Its voice (optional, offline by default).** When an adventure ends, it can tell
you about it in a sentence:

```
xscapes life tell            # ask it for a story now
xscapes life story           # the last thing it said
xscapes life ai              # mode, calls, cache, last error
xscapes life ai --check      # one real round trip, and how long it took

XSCAPES_LIFE_AI=template     the default: local templates, no model, no network,
                             no cost, and it still speaks
XSCAPES_LIFE_AI=off          silence
XSCAPES_LIFE_AI=exec         run a command per request; output is sanitised,
                             cached, and a failure falls back to the template
XSCAPES_LIFE_AI_CMD="hermes -z"   what `exec` runs (anything that reads a prompt
                                  on stdin and writes an answer on stdout)
XSCAPES_LIFE_PROJECT=on      let it look at the project: read-only, twelve files,
                             24 KB each, source only, and an observation is shown
                             only if the file and line it cites really exist
```

**The voice is the scene's.** The crab on the shore talks about the tideline;
the owl on the vista talks about the meadow and the fence post. The model is told
who it is and where, so its answers arrive in the right voice too.

Nothing in the voice can stall a frame: requests leave on a buffered queue, the
answer is collected on the next frame, and a slow or broken model is abandoned at
its deadline and replaced by the animal's own words.

The companion's state is one small file, `~/.config/xscapes/life.json`,
written when the scape closes and after each command. If you had the earlier
creature, its life is carried across once -- its stage, its time and every
kindness it was shown -- and the fields that used to punish it are simply
dropped, because this animal has none of them.

**Cost.** The whole life tick is about fifteen nanoseconds and allocates
nothing; a minute of live frames with the companion in the scene costs 873 KB
through the terminal diff against 8.4 MB for full repaints. Measured, and held
in place by tests.

## The earlier creature (legacy, opt-in)

`xscapes tamagotchi` was the first creature: a separate animal that floated in
the sky, with needs, decay and stages of its own. The companion above replaced it
as what ships — one animal on one shore. The whole subsystem is still in the
tree, unchanged, behind `XSCAPES_TAMAGOTCHI=on`, and its old save is carried into
the new companion once.

What it is, for the record: a small animal lives on the shore. It is not a widget: it eats, sleeps, sulks,
plays and grows up, and it is the same animal every time you come back.

**It grows.** Four stages, and every one of them has to be earned:

| stage | it takes | what changes |
|---|---|---|
| baby | ~12 min in front of you, 5 of them well kept, 3 moments of care | small and round, huge eyes, no patience, fidgets constantly |
| child | ~35 min, 15 well kept, 10 moments, has been fed **and** played with | the full silhouette, brighter coat |
| teen | ~1 h 20 min, 35 well kept, 25 moments, fed and played and petted | taller and narrower, nosier, quick to react |
| adult | — | broad and deep-coloured, calmer, steadier, remembers |

Growth is judged on five things at once, and a creature that is only one of them
does not move: **time with you**, **the quality of the care** (the mean of all
six needs over the stage), **the interactions** (one credit each, in this stage
only), **its statistics** (health and happiness at the moment it turns), and
**its history** (which kinds of care it has had, and how often it was left
alone — every stretch of real neglect adds 90 seconds of care it must make up).
A creature kept perfectly but never touched stays a baby, and a creature fed and
never played never becomes a teenager. Growth is slow on purpose: a perfectly
kept creature takes about two hours of attention to reach adulthood, and a
neglected one longer.

The status line always says what is holding the next stage back, in words:

```
baby, 34% to child (needs to be cared for)
```

**Commands** — every one of these works offline, with no account and no API key:

```sh
xscapes tamagotchi                # status: stage, mood, needs, and the picture
xscapes tamagotchi feed           # eat
xscapes tamagotchi play           # play
xscapes tamagotchi pet            # affection
xscapes tamagotchi clean          # a bath
xscapes tamagotchi sleep          # put it to bed
xscapes tamagotchi wake           # wake it up
xscapes tamagotchi talk "hello"   # it answers
xscapes tamagotchi interact       # a prompt: all of the above, by name
xscapes tamagotchi personality    # its temperament, and how the stages moved it
xscapes tamagotchi habits         # what it has learned about you
xscapes tamagotchi memory         # the moments it keeps
xscapes tamagotchi idle           # how long since it was last touched
```

Each interaction plays its own animation — eating chews and drops crumbs,
playing hops with sparkles, petting brings hearts and a blush, a bath is a
sparkling hop — and growing up is a moment of its own: a hop, a line, and a
memory it keeps.

**It lives in the scape too.** In a running shore it sits in the sky, breathing,
blinking, fidgeting, and says what it thinks on the line under its feet. It
never uses the agent's bubble or its knock: those belong to your agent, and a
peckish pet is not a reason to ring the one rare cue the scene has. It is
off by default now; `XSCAPES_TAMAGOTCHI=on` brings it back beside the companion.

**Persistence.** One file, `~/.config/xscapes/tamagotchi.json` (`%HOME%` or
`%USERPROFILE%` on Windows). It is written when the scape closes and after each
command, so the animal you come back to is the one you left. Delete the file to
start a new life. Nothing here talks to a network: the creature, its growth and
its save file are entirely local.

**Looking at the animations.** `xscapes -tamanim creature.html` writes a sheet of
every clip and every transition, at every stage, driven through the real
renderer, and cycles them in a browser.

## The protocol

Each agent's adapter turns its hooks into a small set of events, and any
program that sends them can drive a scene:

```
session_start  prompt  tool_start  tool_end  error  test_pass  test_fail
sub_start  sub_end  needs_input  done  compact  context  todo  session_end
```

```sh
xscapes emit needs_input -text "allow Bash?"
```

## Code

Go, standard library only, one static binary. `go test ./...` runs the tests.
MIT.
