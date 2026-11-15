# Audit — 2026-11-14

Scope: the tree at `d491816` plus the uncommitted work in progress (the `rain`
scape and three new owl motions). Machine: Windows 11 (build 26300), amd64,
Go 1.27.4. Everything marked **verified** was run here and produced the stated
output; everything marked **not verified** could not be run on this machine.

## Result

`go build ./...`, `go vet ./...` and `go test ./...` pass on Windows, all 16
packages. Before the audit, `go build ./...` and `go vet ./...` failed on
Windows and `go test ./...` failed one test (see findings 1 and 2).

## Findings and what was done

| # | finding | severity | fix |
|---|---|---|---|
| 1 | Eight probe programs in `notes/` use Unix-only syscalls with no build tag, so `go build ./...` / `go vet ./...` failed on Windows | high (breaks CI and contributors on Windows) | tagged `//go:build darwin`; build and vet now clean for windows, linux, darwin |
| 2 | `TestTheStudyVistaIsUnchanged` fails on amd64: its golden hash was recorded elsewhere. Fails identically at the repo's first commit and with `GOAMD64=v3`, so it is not a regression | medium | golden per `GOARCH`; arm64 keeps the original, amd64 recorded here, others skip. Cause (suspected arm64 fused multiply-add) **not isolated** |
| 3 | No Windows installer. `install.sh` told Windows users it was unsupported, and the site said "Not Windows yet", though the ConPTY port exists and works | high | `site/install.ps1` (arch detection, SHA-256 check, user PATH, in-place upgrade, `go install` fallback); `install.sh` points Git-Bash users at it; `start.bat` for clones |
| 4 | Releases built for 4 targets only; no Windows assets; `shasum` only (absent on most Linux/Windows) | high | `site/release.sh` and `.github/workflows/release.yml` build 6 targets, stamp the version, write `checksums.txt` with `sha256sum` or `shasum` |
| 5 | Installer never verified what it downloaded | medium (supply chain) | both installers verify against `checksums.txt`; mismatch installs nothing; tested (below) |
| 6 | No `xscapes version` | low | added (`version.go`, ldflags-stamped, falls back to build info) |
| 7 | `uninstall <agent>` with no config file died with a raw OS error | low | prints "nothing to remove" and exits 0 |
| 8 | `install hermes` hint and help text said `~/.hermes`, wrong on Windows; help listed three scapes, there are four | low | corrected |
| 9 | No CI | high | `.github/workflows/ci.yml`: build + vet + test on ubuntu/macos/windows, cross-build of all six targets, script parse/lint |
| 10 | Line-ending drift (Go files CRLF in the Windows working copy, `.sh` must be LF) | medium | `.gitattributes` |
| 11 | `xsc_pr/` (381 MB upstream clone), `.probehome/`, `dist/` not ignored | low | `.gitignore` |

## Verified on this machine

- Cross-compile (CGO off) for windows/amd64, windows/arm64, linux/amd64,
  linux/arm64, darwin/amd64, darwin/arm64: all build. freebsd/amd64 does not
  (no pty implementation) — documented as unsupported.
- Every CLI command in the README ran and returned 0 with sane output: `life`
  (feed ×5 foods, pet, play, walk, explore, toy, call, finds, toys, memory,
  foods, tell, story, ai, ai --check), `tamagotchi` (all 12 subcommands),
  `companion`, `scape` (all four), `notify`, `emit` (6 kinds, invalid kind
  rejected with exit 2), `version`, one-frame render.
- Agent installers, real round trip against throwaway configs: Claude
  (existing user hook preserved, install idempotent, uninstall restores
  exactly), Kimi (TOML), Hermes (YAML via `$HERMES_HOME`), all with backups.
- `xscapes inside cmd /c …` on ConPTY: runs the child, paints the scape,
  returns the child's exit code. The `hook` adapter writes the session pointer
  and spool correctly.
- `install.sh` (run under Git Bash with a stubbed `uname`): good checksum
  installs; tampered checksum is refused with nothing installed; missing
  `checksums.txt` installs with a warning.
- `install.ps1` under Windows PowerShell 5.1: good install, in-place upgrade
  (old binary moved aside), tampered checksum refused with nothing installed,
  missing `checksums.txt` warns. The write to the real user PATH was **not**
  exercised (it would change this account); the test used `XSCAPES_NO_PATH=1`.
- `start.bat`: builds, `version`, menu, launches the scape.

## Not verified

- Runtime on macOS and Linux (only cross-compiled here; the suite there is run
  by CI, which has not run yet).
- A real Claude Code / Kimi / Hermes session inside the scape on Windows. The
  hooks install and the adapter writes events, but I did not drive a live agent.
- Event → scene reaction end to end on Windows. I tried to confirm the scape
  applied hook events via `XSCAPES_EVENTLOG` and the log was never created
  (`ListenSince` did not run in that harness either). Not established whether
  that is the harness (stdout was a pipe, not a console) or a bug. **Run it
  once in a real Windows Terminal before relying on it.**
- `shellcheck` on `install.sh` and the CI workflows (no network runner here).
- The `Notification` hook payload I sent did not produce a `needs_input`
  event; the payload shape I used may simply not be what Claude Code sends.

## Feature status (against CLAUDE.md and the README)

| feature | status |
|---|---|
| Event protocol, reducer, bus (socket on Unix, spool on Windows) | implemented, tested |
| Shore, vista (owl, owlets), harbour scapes | implemented |
| `rain` scape (three clips cycling) | **uncommitted work in progress**; listed by `xscapes scape`, builds, passes tests; not documented in the README |
| Companion (crab, cat), life, food, adventures, spectators/banter, optional AI voice | implemented, tested |
| Legacy tamagotchi | implemented, opt-in |
| Adapters: Claude Code, Kimi, Hermes, generic process watcher (`inside`) | implemented |
| Context moon, todo stars, token count, kittens/owlets for subagents | implemented |
| Three sound cues (ask, done, worried) | implemented; Windows via winmm |
| tmux launcher (`claude -beside`), no-tmux inside host | implemented |
| Keypress-to-focus-agent-pane on a notification | **not built** (CLAUDE.md milestone 1, item 6) |
| Weather (rain/clouds/fog, real-weather sync) | deferred by decision, not built |
| Aquarium and rainy-window scapes | drawn as site clips only, not scapes |
| Codex CLI, Gemini CLI, OpenCode adapters | not built ("coming soon" on the site) |
| FreeBSD / other Unix | not supported |
