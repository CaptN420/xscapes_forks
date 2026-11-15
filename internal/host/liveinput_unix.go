//go:build !windows

package host

import "os"

// EnterLiveInput puts the terminal into raw mode for the standalone scape
// (-live), which -- unlike the host, which is a pass-through -- reads the mouse
// itself, so a click can land on the companion.
//
// Raw mode is the same one the host uses (makeRaw), and for the same reason:
// without it the terminal echoes and line-buffers, and a mouse report -- an
// escape sequence, not a line -- would sit in the buffer until Enter.
//
// The returned function restores the terminal exactly as it was.
func EnterLiveInput() (func(), error) {
	fd := os.Stdin.Fd()
	old, err := makeRaw(fd)
	if err != nil {
		return func() {}, err
	}
	return func() { _ = restoreTermios(fd, old) }, nil
}
