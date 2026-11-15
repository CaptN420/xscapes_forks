# Installing xscapes

One binary, no runtime dependencies. Pick your platform.

| platform | installer | binary asset |
|---|---|---|
| macOS (Apple silicon, Intel) | `install.sh` | `xscapes-darwin-arm64`, `xscapes-darwin-amd64` |
| Linux (amd64, arm64), WSL | `install.sh` | `xscapes-linux-amd64`, `xscapes-linux-arm64` |
| Windows 10 1809+ / 11 (amd64, arm64) | `install.ps1` or `start.bat` | `xscapes-windows-amd64.exe`, `xscapes-windows-arm64.exe` |
| anything else with Go 1.27+ | `go install` | built from source |

FreeBSD and other Unixes are not supported: the host needs a pty file per OS
(`internal/host/pty_*.go`) and only darwin, linux and windows have one.

## macOS and Linux

```sh
curl -fsSL https://donlucasx.github.io/xscapes/install.sh | sh
```

The script downloads the binary for your OS and chip into `~/.local/bin`,
**verifies its SHA-256 against the release's `checksums.txt`** (a mismatch
installs nothing), adds `~/.local/bin` to your shell's rc file if it is not on
PATH yet, and prints next steps that work in the current window.

Options (environment variables): `XSCAPES_VERSION=v0.4.2` pins a release,
`XSCAPES_INSTALL_DIR` changes the directory, `XSCAPES_SKIP_VERIFY=1` skips the
checksum, `XSCAPES_DOWNLOAD_BASE` points at a mirror.

## Windows

From PowerShell (no administrator rights needed):

```powershell
irm https://donlucasx.github.io/xscapes/install.ps1 | iex
```

It installs `xscapes.exe` to `%LOCALAPPDATA%\Programs\xscapes`, verifies the
SHA-256, and adds that directory to your **user** PATH. Open a new terminal to
use the bare name. The same environment variables apply, plus
`XSCAPES_NO_PATH=1` to leave PATH alone. Running it again upgrades in place
(a running copy is renamed aside, not overwritten).

Use Windows Terminal or any console with virtual-terminal support.

### From a clone: `start.bat`

Double-click `start.bat` (or run it from a terminal). The Windows package
includes `xscapes.exe`, so Go is not required just to run it. If Go 1.27+ is
available, the launcher tries to rebuild from source; if Go is missing or the
build fails (for example, because an older toolchain is installed), it uses
the bundled executable. It then offers a menu: Hermes, Claude Code, Kimi, a
plain shell inside the scape, the scape alone, hook installation for all three
agents, and a local web server for the manual
(`http://127.0.0.1:8000`). `start.bat hermes`, `start.bat claude`,
`start.bat site`, `start.bat install` skip the menu.

### From source (any OS)

```sh
git clone https://github.com/donlucasx/xscapes && cd xscapes
go build -o xscapes .          # xscapes.exe on Windows
go test ./...                  # the whole suite
```

## Connect an agent

`install` without `--apply` prints the plan and changes nothing. With
`--apply` it backs up the agent's config to `~/.config/xscapes/backups/` first.

```sh
xscapes install claude --apply  && xscapes claude    # Claude Code
xscapes install kimi   --apply  && xscapes kimi      # Kimi Code CLI
xscapes install hermes --apply  && xscapes hermes    # Hermes Agent
xscapes inside <any command>                         # no hooks needed
```

Config files it edits: `~/.claude/settings.json`; `$KIMI_CODE_HOME` or
`~/.kimi-code/config.toml`; Hermes' `config.yaml` (`$HERMES_HOME`, else on
Windows the desktop app's profile under `%LOCALAPPDATA%\hermes\profiles\<name>`,
else `~/.hermes`). The agent must have been run once so its config exists.
Hermes asks consent per hook the first time; `hermes hooks doctor` answers once.

## Verify

```sh
xscapes version        # xscapes v0.4.2 (windows/amd64, go1.27.4)
xscapes                # renders one frame
xscapes notify         # plays the three cues (sound check)
xscapes emit done      # sends an event (a running scape reacts)
```

## Uninstall

```sh
xscapes uninstall claude --apply     # also: kimi, hermes. Restores your settings.
rm ~/.local/bin/xscapes ~/.config/xscapes          # macOS / Linux
```

Windows: `xscapes uninstall <agent> --apply` for each agent, then delete
`%LOCALAPPDATA%\Programs\xscapes`, remove that entry from your user PATH
(System Properties → Environment Variables) and delete `%USERPROFILE%\.config\xscapes`
if you also want to drop the companion's saved life.

## Troubleshooting

| symptom | cause / fix |
|---|---|
| `command not found: xscapes` right after install | PATH changes reach **new** terminals; use the full path the installer printed |
| `checksum mismatch` | the download was corrupted or altered; nothing was installed. Retry; do not bypass unless you know why |
| Windows: `Windows build N is too old` | the host needs ConPTY (Windows 10 1809 / build 17763) |
| `install hermes` says run `hermes setup` first | Hermes has not written its config yet |
| hooks installed but the scene stays still | restart the agent so it re-reads its config; Hermes: `hermes hooks doctor` |
| Windows: first event takes up to 250 ms | no AF_UNIX there, events go through the spool file (by design) |
| scrambled colours | set `XSCAPES_COLOR=truecolor` only on a terminal that supports it; default is the 256-colour cube everywhere |
