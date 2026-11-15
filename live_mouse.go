package main

import (
	"io"
	"strings"

	"github.com/donlucasx/xscapes/internal/envx"
	"github.com/donlucasx/xscapes/internal/life"
)

// Clicking the companion.
//
// The scape is otherwise a read-only spectator: the creatures judge the code
// and touch nothing. The animal is the one thing on the strip that is the
// user's, so a click on IT is allowed to land -- a left click pets it, a right
// click feeds it. Nothing else on screen is clickable, and a click anywhere
// else is ignored, so the strip stays a picture you can watch without
// disturbing it.
//
// Mouse reporting is opt-OUT (XSCAPES_MOUSE=off), because a terminal that
// reports clicks no longer lets the mouse select text while the scape is up.

var mouseWanted = !isOff(envx.Lookup("MOUSE"))

// SGR mouse reporting: 1000h reports button presses and releases, 1006h selects
// the SGR encoding (decimal, no 223-column limit -- the far right of a wide
// frame is reachable). Turning both off is what puts the terminal's own
// selection back.
const (
	mouseOn  = "\x1b[?1000h\x1b[?1006h"
	mouseOff = "\x1b[?1006l\x1b[?1000l"
)

// hitBox is a rectangle of cells, 0-based, in the frame's own coordinates.
type hitBox struct {
	x, y, w, h int
	ok         bool
}

func (b hitBox) contains(x, y int) bool {
	return b.ok && b.w > 0 && b.h > 0 &&
		x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h
}

// lifeHit is where the companion was drawn on the last frame. It is written by
// the renderers (live.go, vista_live.go) and read by the input drain, both on
// the loop's own goroutine, so it needs no lock.
var lifeHit hitBox

// setLifeHit records the companion's box for this frame, or clears it when the
// animal is not on screen -- a click must not land on where it used to be.
func setLifeHit(x, y, w, h int) {
	if !lifeLive() {
		lifeHit = hitBox{}
		return
	}
	lifeHit = hitBox{x: x, y: y, w: w, h: h, ok: true}
}

// mouseClick is one SGR report, decoded. col and row are the terminal's own,
// 1-based; the conversion happens at the hit test.
type mouseClick struct {
	col, row int
	button   int
	press    bool
}

// liveInput is what the reader goroutine hands the loop: decoded clicks, and a
// quit for a bare Ctrl-C (raw mode delivers 0x03 as a byte, not a signal).
type liveInput struct {
	clicks chan mouseClick
	quit   chan struct{}
}

// parseMouse reads one SGR mouse report at the start of b: ESC [ < b ; x ; y
// (M|m). It returns the decoded click, the bytes consumed, and -- when the
// report is cut off by the end of the buffer -- need=true so the caller keeps
// the bytes and waits for more.
func parseMouse(b []byte) (c mouseClick, n int, need, ok bool) {
	if len(b) < 3 {
		return c, 0, len(b) >= 1 && b[0] == 0x1b, false
	}
	if b[0] != 0x1b || b[1] != '[' || b[2] != '<' {
		return c, 0, false, false
	}
	end := -1
	for i := 3; i < len(b); i++ {
		if b[i] == 'M' || b[i] == 'm' {
			end = i
			break
		}
		if (b[i] < '0' || b[i] > '9') && b[i] != ';' {
			return c, 0, false, false // not a report; do not swallow bytes
		}
	}
	if end == -1 {
		if len(b) > 24 { // far too long to be a report
			return c, 0, false, false
		}
		return c, 0, true, false
	}
	parts := strings.Split(string(b[3:end]), ";")
	if len(parts) != 3 {
		return c, end + 1, false, false
	}
	btn, ok1 := mouseAtoi(parts[0])
	col, ok2 := mouseAtoi(parts[1])
	row, ok3 := mouseAtoi(parts[2])
	if !ok1 || !ok2 || !ok3 || col < 1 || row < 1 {
		return c, end + 1, false, false
	}
	return mouseClick{col: col, row: row, button: btn & 0x03, press: b[end] == 'M'}, end + 1, false, true
}

// mouseAtoi reads the small non-negative decimal fields of an SGR report.
//
// It is hand-rolled because package main cannot import strconv: a test file in
// this package declares its own `strconv` string helper, which shadows the
// package (see heropage_test.go, and itoa/lifeNum for the same reason).
func mouseAtoi(s string) (int, bool) {
	if s == "" || len(s) > 6 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, true
}

// scanMouse consumes what it can from buf and returns the unconsumed tail (a
// report cut in half by the read boundary). A leading Ctrl-C quits.
func scanMouse(buf []byte, in *liveInput) []byte {
	for i := 0; i < len(buf); {
		if buf[i] == 0x03 {
			select {
			case in.quit <- struct{}{}:
			default:
			}
			i++
			continue
		}
		if buf[i] == 0x1b {
			c, n, need, ok := parseMouse(buf[i:])
			if need {
				return buf[i:]
			}
			if ok {
				select {
				case in.clicks <- c:
				default: // a burst faster than we drain: drop, do not block the read
				}
				i += n
			} else {
				i += n
				if n == 0 {
					i++ // a lone ESC or a stray glyph: skip one byte
				}
			}
			continue
		}
		i++ // every other byte: the scape takes no keyboard input
	}
	return buf[:0]
}

// readMouse feeds raw terminal bytes through scanMouse until the reader ends.
func readMouse(r io.Reader, in *liveInput) {
	buf := make([]byte, 0, 256)
	tmp := make([]byte, 256)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = scanMouse(append(buf, tmp[:n]...), in)
		}
		if err != nil {
			return
		}
	}
}

// drainInput applies everything waiting on the input channel, once per frame.
// quit is the same shutdown the signal handler runs.
func drainInput(in *liveInput, quit func()) {
	for {
		select {
		case <-in.quit:
			quit()
			return
		case c := <-in.clicks:
			if c.press {
				clickLife(c)
			}
		default:
			return
		}
	}
}

// clickLife turns a click that landed on the companion into an interaction:
// left pets it, right feeds it. Anywhere else -- or nothing drawn -- it does
// nothing, which is the whole point of the hit test.
func clickLife(c mouseClick) bool {
	if !lifeLive() {
		return false
	}
	x, y := c.col-1, c.row-1 // the terminal is 1-based; the frame is 0-based
	if !lifeHit.contains(x, y) {
		return false
	}
	switch c.button {
	case 0: // left
		lifeBrain.Do(life.CmdPet)
	case 2: // right
		lifeBrain.Do(life.CmdFeed)
	default:
		return false
	}
	saveLife()
	return true
}
