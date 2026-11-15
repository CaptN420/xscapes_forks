//go:build windows

package event

import (
	"path/filepath"
	"testing"
)

// A live Windows session can run for hours. Its lock must therefore be held
// by the process handle, not expire merely because the lock file is old.
func TestWindowsSessionFileLockIsExclusiveAndReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.busy")
	first, err := openSessionLock(path)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if _, err := openSessionLock(path); err != ErrBusy {
		t.Fatalf("second lock = %v, want ErrBusy", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}
	third, err := openSessionLock(path)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("release third lock: %v", err)
	}
}
