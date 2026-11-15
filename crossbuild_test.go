package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// The tree must build for every platform it ships on, not just the one it is
// being developed on.
//
// This is the check that was missing. A helper written for one platform and
// called from shared code compiles perfectly on the host and breaks the others
// with "undefined: <helper>": exactly how release() -- defined only in
// sock_windows.go -- left GOOS=linux and GOOS=darwin unable to build
// internal/event. Nothing here builds for another GOOS, so nothing noticed.
//
// Build, not vet: the failure mode is a compile error, and build is what a
// release does. Results are discarded for a multi-package build, so nothing is
// written into the tree.
func TestTheTreeBuildsForEveryPlatform(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-compile check skipped under -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}

	targets := []struct{ goos, goarch string }{
		{"windows", "amd64"}, // the port this tree is being developed on
		{"darwin", "arm64"},  // the owner's machine
		{"linux", "amd64"},
	}
	for _, tgt := range targets {
		if tgt.goos == runtime.GOOS && tgt.goarch == runtime.GOARCH {
			continue // the host is built by every other test already
		}
		cmd := exec.Command("go", "build", "./internal/...", ".")
		cmd.Env = append(os.Environ(),
			"GOOS="+tgt.goos, "GOARCH="+tgt.goarch, "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("GOOS=%s GOARCH=%s does not build:\n%s",
				tgt.goos, tgt.goarch, strings.TrimSpace(string(out)))
		}
	}
}
