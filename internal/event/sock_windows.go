//go:build windows

package event

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// THE SPOOL IS THE ONLY PATH ON WINDOWS.
//
// Go's net package has no AF_UNIX support on Windows (the OS has AF_UNIX since
// 1803, the runtime does not expose it), so sendSock fails by construction and
// every event goes to the session's JSON-lines spool, which the bus already
// follows -- that is the same two-path design the other platforms have, with
// the fast path simply absent. The cost a viewer can see is spoolPoll: 250 ms
// between an event being written and the scene noticing it, against a 12 fps
// scape.

// errNoSock is what sendSock reports: the caller treats every error the same
// way and falls to the spool, so the only thing this has to be is non-nil.
var errNoSock = errors.New("unix sockets are not available on windows; using the spool")

type noSock struct {
	lock       *os.File
	overlapped syscall.Overlapped
	closeOnce  sync.Once
}

// read blocks until the bus is closed. Returning an error immediately would
// end readSock at once and the goroutine would spin down silently, which is
// fine too -- but this way the shape matches the unix one: one read loop, one
// Close, nothing that looks like a failure.
func (noSock) read(buf []byte) (int, error)    { time.Sleep(time.Second); return 0, errNoSock }
func (noSock) setReadDeadline(time.Time) error { return nil }
func (n *noSock) Close() error {
	var closeErr error
	n.closeOnce.Do(func() {
		if n.lock == nil {
			return
		}
		// Closing the handle also releases the range lock if UnlockFileEx
		// itself fails; keep close as the final guarantee.
		r, _, e := procUnlockFileEx.Call(
			n.lock.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&n.overlapped)))
		if r == 0 {
			closeErr = fmt.Errorf("UnlockFileEx: %v", e)
		}
		if err := n.lock.Close(); closeErr == nil {
			closeErr = err
		}
		n.lock = nil
	})
	return closeErr
}

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	errorLockViolation      = syscall.Errno(33) // ERROR_LOCK_VIOLATION
)

var (
	modKernel32Event = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modKernel32Event.NewProc("LockFileEx")
	procUnlockFileEx = modKernel32Event.NewProc("UnlockFileEx")
)

func sendSock(session string, line []byte) error { return errNoSock }

// sessionTracker tracks which sessions are currently bound so bind() can
// return ErrBusy when a second listener claims the same one. Two mechanisms
// cooperate: an in-memory map catches same-process duplicates, and a Win32
// byte-range file lock catches other processes. The kernel releases the latter
// if a process exits unexpectedly.
var (
	mu     sync.Mutex
	active = make(map[string]struct{}) // tag -> present
)

// lockInfo extracts the session tag from a socket path and returns the path of
// the per-session busy-lock file. p has the shape <run_dir>/<tag>.sock.
func lockInfo(p string) (tag string, lockPath string, err error) {
	base := filepath.Base(p)
	if len(base) < 5 || base[len(base)-5:] != ".sock" {
		return "", "", errors.New("unexpected socket path shape")
	}
	tag = base[:len(base)-5]
	d, err := EnsureRunDir()
	if err != nil {
		return "", "", err
	}
	return tag, filepath.Join(d, tag+".busy"), nil
}

// bind claims a session. It returns ErrBusy when another engine (this process
// or a sibling) already holds the session, mirroring the unix socket-bind
// semantics that the spool alone cannot enforce.
func bind(p string) (sockConn, error) {
	tag, lockPath, err := lockInfo(p)
	if err != nil {
		return nil, err
	}

	mu.Lock()
	if _, held := active[tag]; held {
		mu.Unlock()
		return nil, ErrBusy
	}
	active[tag] = struct{}{}
	mu.Unlock()

	// Cross-process: take a byte-range lock held by the open file handle.
	// Windows releases it automatically if the process exits unexpectedly,
	// so unlike an age-based lock file it stays valid for long-running sessions.
	conn, err := openSessionLock(lockPath)
	if err != nil {
		mu.Lock()
		delete(active, tag)
		mu.Unlock()
		return nil, err
	}

	return conn, nil
}

// release unclaims a session in this process. The byte-range lock is released
// by noSock.Close; the tiny lock file itself remains as a stable lock target.
func release(p string) {
	tag, _, err := lockInfo(p)
	if err != nil {
		return
	}
	mu.Lock()
	delete(active, tag)
	mu.Unlock()
}

// openSessionLock holds a one-byte exclusive range lock for the lifetime of
// the bus. The empty lock file is deliberately left in place: deleting it
// after unlocking races a new process that has already opened the old inode
// with another process that creates a new file at the same path.
func openSessionLock(path string) (*noSock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	n := &noSock{lock: f}
	r, _, callErr := procLockFileEx.Call(
		f.Fd(), lockfileExclusiveLock|lockfileFailImmediately, 0, 1, 0,
		uintptr(unsafe.Pointer(&n.overlapped)))
	if r == 0 {
		_ = f.Close()
		if callErr == errorLockViolation {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("LockFileEx %s: %v", path, callErr)
	}
	return n, nil
}
