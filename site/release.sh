#!/bin/sh
# release.sh builds the binaries install.sh downloads and publishes them as
# the GitHub release for one tag:
#
#     sh site/release.sh -n v0.4.2   # build dist/ and list it, publish nothing
#     sh site/release.sh v0.4.2      # build dist/, push the tag, create the release
#
# Run it from a CLEAN tree with HEAD at the tag: go build stamps the
# revision, and a dirty tree stamps vcs.modified=true and the wrong SHA,
# which is the same rule the installed binary follows. The targets are
# the platforms internal/host has a pty file for (darwin and linux, arm64 and
# amd64) plus Windows (amd64 and arm64), built with cgo off so the build runs
# anywhere. The version is stamped into the binary: `xscapes version`.
#
# install.sh fetches <release>/latest/download/xscapes-<os>-<arch>, so the
# asset names here are part of the contract.
set -eu
dry=no
if [ "${1:-}" = -n ]; then
  dry=yes
  shift
fi
tag=${1:?usage: sh site/release.sh [-n] vX.Y.Z}
root=$(git rev-parse --show-toplevel)
cd "$root"
if [ -n "$(git status --porcelain)" ]; then
  echo "release.sh: the tree is not clean; commit first" >&2
  exit 1
fi
at=$(git describe --exact-match --tags HEAD 2>/dev/null || true)
if [ "$at" != "$tag" ]; then
  echo "release.sh: HEAD is tagged '${at:-nothing}', not $tag; tag first" >&2
  exit 1
fi
rm -rf dist
mkdir dist
# Six targets: the platforms internal/host has a pty file for (darwin and
# linux, arm64 and amd64) plus Windows on ConPTY (amd64 and arm64). Windows
# assets keep the .exe, and install.ps1 asks for exactly these names.
for t in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  os=${t%/*}
  arch=${t#*/}
  ext=
  [ "$os" = windows ] && ext=.exe
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$tag"     -o "dist/xscapes-$os-$arch$ext" .
done
# sha256sum on Linux and Git Bash, shasum on macOS; one file, two spaces, the
# format both installers parse.
if command -v sha256sum >/dev/null 2>&1; then
  (cd dist && sha256sum xscapes-* >checksums.txt)
else
  (cd dist && shasum -a 256 xscapes-* >checksums.txt)
fi
ls -l dist
go version -m dist/xscapes-linux-amd64 | grep -E 'vcs\.(revision|modified)'
if [ $dry = yes ]; then
  exit 0
fi
# The active gh account is host-global and flips back to another login
# between commands; switch immediately before every use.
gh auth switch --user donlucasx
git push origin "refs/tags/$tag"
gh release create "$tag" dist/* --verify-tag --title "$tag" \
  --notes "Binaries for the install line: \`curl -fsSL https://donlucasx.github.io/xscapes/install.sh | sh\`"
echo "released $tag: https://github.com/donlucasx/xscapes/releases/tag/$tag"
