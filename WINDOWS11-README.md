# xscapes on Windows 11

This folder contains the Windows x64 build and the complete project source.
Windows 11 is supported; the minimum console API level is Windows 10 version
1809 (build 17763), which provides ConPTY.

## Start without installing Go

1. Extract the ZIP to a normal folder, for example `C:\Tools\xscapes`.
2. Open that folder and double-click `start.bat`.
3. Choose an item from the menu. **Windows Terminal** is recommended for the
   best colours, mouse input, and Unicode rendering.

The ZIP includes `xscapes.exe` for Windows x64. Go is not needed just to run
that executable. When Go 1.27+ is available, `start.bat` attempts to rebuild
from source. If Go is missing or the build fails, it falls back to the included
Windows executable.

## Start from PowerShell

Open PowerShell in the extracted folder:

```powershell
.\xscapes.exe version
.\xscapes.exe -live
```

To connect an installed coding agent, install its hooks once, then launch the
agent inside xscapes:

```powershell
# Choose the agent you actually use:
.\xscapes.exe install claude --apply
.\xscapes.exe claude

# Or use Kimi Code CLI:
.\xscapes.exe install kimi --apply
.\xscapes.exe kimi

# Or use Hermes Agent:
.\xscapes.exe install hermes --apply
.\xscapes.exe hermes
```

`install` backs up the agent configuration before changing it. To preview
changes without applying them, omit `--apply`. Hermes must have been run once
so its configuration exists; its Windows desktop profile is detected under
`%LOCALAPPDATA%\hermes\profiles`.

To run a different command in the scene, use:

```powershell
.\xscapes.exe inside pwsh
.\xscapes.exe inside cmd.exe
```

Windows `.cmd` and `.bat` command shims (including common npm-installed CLI
launchers) are started through Windows PowerShell, so they can run inside the
ConPTY session with their arguments preserved.

## Source builds

Install Go 1.27 or later, open a new PowerShell window, and run:

```powershell
go version
go build -o xscapes.exe .
go test ./...
```

The project has no third-party Go module dependencies. The Windows console
host uses native ConPTY and Win32 APIs.

## What differs from Linux internally

The renderer, scene logic, event protocol, and agent adapters are shared with
the Linux/macOS source. Windows uses native equivalents for the operating
system boundary:

- ConPTY replaces the Unix pseudoterminal device.
- Console mode APIs replace `termios` raw mode.
- Console-size polling replaces `SIGWINCH`/`ioctl` resize notification.
- The JSON-lines event spool is used in place of the Unix datagram-socket fast
  path on Windows; an event can therefore take up to about 250 ms to appear.
- Windows `winmm` plays the notification sounds.

These are platform-specific implementations of the same user-facing workflow,
not a separate or reduced scene renderer. The measured implementation details
are in `notes/windows-conpty.md`; installation and troubleshooting are in
`docs/INSTALL.md`.
