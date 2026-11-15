#!/bin/sh
# install.sh puts xscapes on this machine with one command, on a Mac or a
# Linux box, with or without Go:
#
#     curl -fsSL https://donlucasx.github.io/xscapes/install.sh | sh
#
# It downloads the release binary for this OS and chip into ~/.local/bin,
# the directory Claude Code's own installer uses, so it is often on PATH
# already. When it is not, the script adds one line to the shell's rc file
# for every new terminal and prints the next commands with a path that
# works in THIS window, because a child process cannot change its parent
# shell's PATH. Where no binary is published for the platform it builds one
# with `go install`, and where there is no Go either it says so and stops.
#
# Why it exists: `go install ...@latest` puts the binary in ~/go/bin, which
# is on no Mac's PATH by default, and prints nothing about it. The first
# outside tester (2026-09-17) followed the page to the letter and got
# `zsh: command not found: xscapes`.
#
# Overrides, for the test and for anyone who wants them:
#   XSCAPES_INSTALL_DIR    where the binary goes (default ~/.local/bin)
#   XSCAPES_DOWNLOAD_BASE  where the assets are (default the latest GitHub release)
set -eu

repo=donlucasx/xscapes
if [ -n "${XSCAPES_VERSION:-}" ]; then
  default_base=https://github.com/$repo/releases/download/$XSCAPES_VERSION
else
  default_base=https://github.com/$repo/releases/latest/download
fi
base=${XSCAPES_DOWNLOAD_BASE:-$default_base}
dir=${XSCAPES_INSTALL_DIR:-$HOME/.local/bin}

say() { printf '%s\n' "$*"; }
fail() { printf 'xscapes: %s\n' "$*" >&2; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
#   XSCAPES_VERSION        a release tag to install instead of the latest (v0.4.2)
#   XSCAPES_SKIP_VERIFY=1  install without checking the download's SHA-256
#
# macOS and Linux. Native Windows has its own installer, install.ps1 (the
# host runs the agent on a ConPTY there); inside WSL this is a Linux box and
# this script is the right one. Said here rather than as a 404 on a binary
# name nobody would recognise.
case "$os" in
  darwin|linux) ;;
  mingw*|msys*|cygwin*)
    echo "xscapes: this is a Windows shell. Use the Windows installer, from PowerShell:" >&2
    echo "  irm https://donlucasx.github.io/xscapes/install.ps1 | iex" >&2
    exit 1 ;;
  *) echo "xscapes: runs on macOS, Linux and Windows; $os has no build." >&2; exit 1 ;;
esac
case $arch in
  arm64 | aarch64) arch=arm64 ;;
  x86_64 | amd64) arch=amd64 ;;
  *) fail "no build for the $arch processor (builds: amd64, arm64)" ;;
esac
asset=xscapes-$os-$arch

mkdir -p "$dir"
tmp=$(mktemp "$dir/.xscapes.XXXXXX")
trap 'rm -f "$tmp" "$tmp.err" "$tmp.sums"' EXIT

# curl's own error (a 404, no network) is kept for the final message: on the
# fallback path it is expected and would only be noise.
if curl -fsSL "$base/$asset" -o "$tmp" 2>"$tmp.err"; then
  # The release carries checksums.txt (site/release.sh writes it). A mismatch
  # is fatal; a release without the file is installed with a warning, because
  # the older releases never had one.
  if [ "${XSCAPES_SKIP_VERIFY:-}" != 1 ]; then
    if curl -fsSL "$base/checksums.txt" -o "$tmp.sums" 2>/dev/null; then
      want=$(awk -v a="$asset" '$2 == a || $2 == "*" a { print $1 }' "$tmp.sums")
      if command -v sha256sum >/dev/null 2>&1; then
        got=$(sha256sum "$tmp" | awk '{print $1}')
      elif command -v shasum >/dev/null 2>&1; then
        got=$(shasum -a 256 "$tmp" | awk '{print $1}')
      else
        got= ; say "xscapes: no sha256sum or shasum here; the download was not verified."
      fi
      if [ -n "$got" ]; then
        [ -n "$want" ] || fail "checksums.txt has no entry for $asset"
        [ "$got" = "$want" ] || fail "checksum mismatch for $asset (expected $want, got $got); nothing was installed"
      fi
    else
      say "xscapes: this release has no checksums.txt; the download was not verified."
    fi
  fi
  chmod +x "$tmp"
  # A rename, so a running xscapes keeps its own inode and is never
  # overwritten in place (macOS kills a process whose binary changes under it).
  mv -f "$tmp" "$dir/xscapes"
  how=$asset
elif command -v go >/dev/null 2>&1; then
  say "no release binary for $os/$arch; building one with go install"
  GOBIN=$dir go install "github.com/$repo@latest"
  how="go install"
else
  fail "no release binary for $os/$arch ($(cat "$tmp.err")) and no Go to build one.
         Install Go (brew install go, or https://go.dev/dl) and run this again."
fi

"$dir/xscapes" -h >/dev/null 2>&1 || fail "the installed binary does not run: $dir/xscapes"

# The directory as the rc line spells it ($HOME, unexpanded) and as a person
# reads it (~), when it is under the home directory.
case $dir in
  "$HOME"/*) shown='$HOME'${dir#"$HOME"} short='~'${dir#"$HOME"} ;;
  *) shown=$dir short=$dir ;;
esac

on_path=no
case ":$PATH:" in *":$dir:"*) on_path=yes ;; esac

rc=
if [ $on_path = no ]; then
  line="export PATH=\"$shown:\$PATH\""
  case $(basename "${SHELL:-sh}") in
    zsh) rc=$HOME/.zshrc ;;
    bash)
      # Terminal.app opens LOGIN shells, and bash reads .bash_profile for
      # those, never .bashrc; Linux terminals open interactive shells.
      if [ "$os" = darwin ]; then rc=$HOME/.bash_profile; else rc=$HOME/.bashrc; fi ;;
    fish) rc=$HOME/.config/fish/config.fish line="fish_add_path $shown" ;;
  esac
  if [ -n "$rc" ]; then
    mkdir -p "$(dirname "$rc")"
    # Once. A second run, or a line another installer already wrote for the
    # same directory, must not add another.
    if ! grep -qsF -- "$line" "$rc"; then
      printf '\n# xscapes: added by install.sh\n%s\n' "$line" >>"$rc"
    fi
  fi
fi

say ""
say "xscapes installed: $short/xscapes ($how)"
if [ $on_path = yes ]; then
  say "Next:"
  say "  xscapes install claude --apply   # writes its hooks, after a backup"
  say "  xscapes claude                   # the agent on top, the scape underneath"
  say "  xscapes install kimi --apply && xscapes kimi       # Kimi Code CLI"
  say "  hermes plugins install donlucasx/hermes-xscapes --enable && xscapes hermes   # Hermes Agent"
else
  if [ -n "$rc" ]; then
    say "Every new terminal will find it by name (one line added to ~${rc#"$HOME"})."
  else
    say "Add $shown to your PATH so new terminals find it by name."
  fi
  say "In this window, use the path:"
  say "  $short/xscapes install claude --apply   # writes its hooks, after a backup"
  say "  $short/xscapes claude                   # the agent on top, the scape underneath"
  say "  $short/xscapes install kimi --apply && $short/xscapes kimi       # Kimi Code CLI"
  say "  hermes plugins install donlucasx/hermes-xscapes --enable && $short/xscapes hermes   # Hermes Agent"
fi
