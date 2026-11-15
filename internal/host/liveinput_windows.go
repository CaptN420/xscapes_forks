//go:build windows

package host

import (
	"os"
	"syscall"
)

// EnterLiveInput puts the console into raw mode for the standalone scape
// (-live), with mouse reporting left ON -- unlike the host's own raw mode,
// which clears ENABLE_MOUSE_INPUT because there the agent's terminal owns the
// mouse. Here the scape wants the clicks: a press on the companion is an
// interaction.
//
// ENABLE_VIRTUAL_TERMINAL_INPUT turns the console's mouse events into the same
// SGR reports every other terminal sends, so one parser reads both platforms.
//
// The returned function restores the console exactly as it was.
func EnterLiveInput() (func(), error) {
	in := syscall.Handle(os.Stdin.Fd())
	inMode, err := getConsoleMode(in)
	if err != nil {
		// Not a console: a pipe or a CI runner. There is nothing to switch
		// and nothing to restore, and the scape is still worth running.
		return func() {}, nil
	}
	raw := inMode
	raw &^= enableEchoInput | enableLineInput | enableProcessedInput
	raw |= enableVirtualTermInput | enableWindowInput | enableMouseInput
	if err := setConsoleMode(in, raw); err != nil {
		return func() {}, err
	}

	out := syscall.Handle(syscall.Stdout)
	outOld, haveOut := uint32(0), false
	if m, e := getConsoleMode(out); e == nil {
		outOld, haveOut = m, true
		_ = setConsoleMode(out, m|enableProcessedOutput|enableVirtualTermOutput)
	}
	return func() {
		_ = setConsoleMode(in, inMode)
		if haveOut {
			_ = setConsoleMode(out, outOld)
		}
	}, nil
}
