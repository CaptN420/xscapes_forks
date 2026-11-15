package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// version is stamped by the release build (site/release.sh passes
// -ldflags "-X main.version=vX.Y.Z"). A `go install` or a plain `go build`
// leaves it empty, and printVersion falls back to what the Go toolchain
// recorded in the binary: the module version or the VCS revision.
var version = ""

// versionString is one line: "xscapes v0.4.2 (windows/amd64, go1.27.4)".
func versionString() string {
	v := version
	rev, dirty := "", ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			v = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
				if len(rev) > 9 {
					rev = rev[:9]
				}
			case "vcs.modified":
				if s.Value == "true" {
					dirty = "+dirty"
				}
			}
		}
	}
	if v == "" {
		v = "dev"
		if rev != "" {
			v += "-" + rev + dirty
		}
	}
	return fmt.Sprintf("xscapes %s (%s/%s, %s)", v, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// wantsVersion is the bare version gesture: `xscapes version`, `-version`,
// `--version`, and nothing else, so it never collides with a flag value.
func wantsVersion(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "version", "-version", "--version", "-v":
		return true
	}
	return false
}
