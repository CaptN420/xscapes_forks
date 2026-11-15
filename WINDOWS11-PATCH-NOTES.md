# Windows 11 port — correction notes

This package keeps the same renderer, scene logic, event model, and agent
adapters as the Linux/macOS project. The OS boundary is implemented with the
native Windows console APIs so the user-facing workflow stays the same.

## Corrections in this package

- `.cmd` and `.bat` agent launchers are now supported inside ConPTY. Windows
  `CreateProcessW` cannot execute a batch shim directly, so the host invokes
  the built-in Windows PowerShell and passes the shim path and each argument
  as a quoted literal. This covers common npm-installed CLI wrappers.
- Windows event-session ownership now uses an exclusive Win32 `LockFileEx`
  byte-range lock held by the open file handle. The kernel releases it when a
  process exits, including an unexpected exit; a live session is no longer
  mistaken for a stale lock just because it has run for a while.
- Added Windows-only integration tests for batch-shim argument preservation
  and session-lock exclusivity/release.
- Updated `WINDOWS11-README.md` and `notes/windows-conpty.md` with the behavior.

## Run on Windows 11

1. Extract the ZIP.
2. Double-click `start.bat`, or open PowerShell in this folder and run
   `./xscapes.exe version`.
3. Use Windows Terminal for the best VT color, Unicode, and input support.

The included executable is Windows x64. No Go installation is needed to run
it. To build from source, install Go 1.27 or later and follow
`WINDOWS11-README.md`.

## Validation performed for this package

- All packages and test sources cross-compiled for `windows/amd64`.
- The Windows executable was rebuilt from the corrected source and verified
  as a 64-bit Windows PE console application.
- `go test ./internal/event ./internal/notify` passed on the Linux build host.
- The new Windows ConPTY and `LockFileEx` integration tests are included and
  compile for Windows, but they were not executed on a native Windows 11 host
  during this packaging run.

The repository's `go.mod` remains unchanged at Go 1.27.0. The bundled bootstrap
executable was built with the locally available Go 1.23.2 compiler to make a
runnable binary available here; `start.bat` will rebuild from source when Go
1.27+ is installed and available.
