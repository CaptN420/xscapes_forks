package canvas

import (
	"strconv"

	"github.com/donlucasx/xscapes/internal/term"
)

// A Screen turns a canvas into the SHORTEST stream of bytes that makes a
// terminal show it, given what the terminal is already showing.
//
// The live loop used to reprint every cell of every frame -- 80x24 cells of
// glyphs and colour escapes, at every tick, through ConPTY on Windows, where
// each byte is parsed twice: once by the terminal and once by the ConPTY relay
// that sits between the program and it. A scene changes a few dozen cells in a
// frame. The rest of the frame is a bill the user pays in CPU, in bandwidth
// down a pipe, and in a laptop fan that never stops.
//
// So: walk the canvas, compare each cell with what the terminal was last told,
// and emit a cursor move plus that one cell for the cells that differ. Colour is
// restated only when it changes from the cell before, exactly as Render does,
// so a still stretch of sky costs nothing at all.
//
// It is not a cache of the FRAME -- the canvas is still painted in full every
// frame, because the scene is procedural and has no idea what changed. It is a
// cache of what the TERMINAL knows, which is the thing that costs money to
// change.
type Screen struct {
	w, h   int
	prev   []screenCell
	buf    []byte
	dirty  int
	frames int
}

// screenCell is everything about one cell that a terminal can see the
// difference in. Two cells with the same struct look identical on screen, which
// is the only question this file asks.
type screenCell struct {
	ch    rune
	fg    term.RGB
	bg    term.RGB
	glyph bool
}

// NewScreen returns a Screen that has never drawn anything, so its first frame
// is a full repaint.
func NewScreen() *Screen { return &Screen{} }

// Invalidate forgets what the terminal is showing. Call it after clearing the
// screen or resizing the window behind the Screen's back: it does not clear
// anything itself, it only stops the next frame from being a diff against a
// picture that is no longer there.
func (s *Screen) Invalidate() { s.w, s.h, s.prev = 0, 0, nil }

// Emit returns the bytes that bring the terminal from the last frame to this
// one, and records this frame as the current one. An identical frame emits
// nothing at all.
func (s *Screen) Emit(c *Canvas, p term.Profile) []byte {
	if c.W != s.w || c.H != s.h {
		s.Invalidate()
		s.w, s.h = c.W, c.H
	}
	if len(s.prev) != c.W*c.H {
		s.prev = make([]screenCell, c.W*c.H)
	}

	s.buf = s.buf[:0]
	dirty := 0
	var lastFG, lastBG term.RGB
	var lastGlyph, haveColor bool
	// Where the cursor will be after the bytes written so far, so a run of
	// changed cells in one row needs no move between them.
	curX, curY := -1, -1

	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			r := c.resolve(x, y, p)
			// Compare what the terminal will SHOW, not what was painted: two
			// colours that land on the same palette entry are one picture, and
			// repainting between them is pure waste. The raw colours are what
			// gets emitted; the quantised ones are what gets remembered.
			k := visibleCell(r, p)
			i := y*c.W + x
			if s.prev[i] == k {
				continue
			}
			s.prev[i] = k
			dirty++
			if x != curX || y != curY {
				s.buf = appendCursor(s.buf, x, y)
			}
			if !haveColor || r.bg != lastBG {
				s.buf = p.AppendBG(s.buf, r.bg)
				lastBG = r.bg
				haveColor = true
			}
			if !haveColor || r.fg != lastFG || r.glyph != lastGlyph {
				if r.glyph {
					s.buf = p.AppendFG(s.buf, r.fg)
				} else {
					s.buf = p.AppendFGRaw(s.buf, r.fg)
				}
				lastFG, lastGlyph = r.fg, r.glyph
			}
			s.buf = append(s.buf, string(r.ch)...)
			curX, curY = x+cellWidth(r.ch), y
		}
	}

	s.dirty, s.frames = dirty, s.frames+1

	// A diff is not always the cheaper message. Every changed cell has to be
	// addressed -- a cursor move and a full colour state, twenty-odd bytes --
	// while a full render states a colour once and then runs glyphs together.
	// On a frame where the whole sea moves, the diff cost 11,691 bytes against
	// a 2,519-byte render: measured, in the audit. So take the cheaper of the
	// two, every frame. prev is already the new frame either way, so the next
	// diff is still against what the terminal is showing.
	if full := c.Render(p); len(full)+len(homeCursor) < len(s.buf) {
		s.buf = append(s.buf[:0], homeCursor...)
		s.buf = append(s.buf, full...)
	}
	return s.buf
}

// homeCursor is "go to the top left": the cheapest way to begin a full repaint.
const homeCursor = "\x1b[H"

// visibleCell is the part of a resolved cell a terminal can see.
func visibleCell(r resolved, p term.Profile) screenCell {
	fg := r.fg
	if r.glyph {
		fg = p.Quantise(fg, true)
	} else {
		fg = p.Quantise(fg, false)
	}
	return screenCell{ch: r.ch, fg: fg, bg: p.Quantise(r.bg, false), glyph: r.glyph}
}

// Dirty reports how many cells the last Emit repainted. Tests and studies ask;
// the loop does not care.
func (s *Screen) Dirty() int { return s.dirty }

// Frames reports how many frames this Screen has emitted.
func (s *Screen) Frames() int { return s.frames }

// appendCursor writes a 1-based cursor move. Cells are 0-based everywhere else
// in this package; the conversion lives here so it happens exactly once.
func appendCursor(dst []byte, x, y int) []byte {
	dst = append(dst, "\x1b["...)
	dst = strconv.AppendInt(dst, int64(y+1), 10)
	dst = append(dst, ';')
	dst = strconv.AppendInt(dst, int64(x+1), 10)
	return append(dst, 'H')
}

// cellWidth is how many columns a glyph occupies. Every glyph the scenes draw
// is one column wide -- blocks, braille, box drawing, ASCII -- but a wide
// glyph would desynchronise the cursor, and a desynchronised cursor is a diff
// that writes the wrong picture rather than a slow one, so the common wide
// ranges are handled rather than assumed away.
func cellWidth(r rune) int {
	switch {
	case r < 0x1100:
		return 1
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0xA4CF, // CJK radicals .. Yi
		r >= 0xAC00 && r <= 0xD7A3, // Hangul syllables
		r >= 0xF900 && r <= 0xFAFF, // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE6F, // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60, // fullwidth forms
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1FAFF, // emoji and pictographs
		r >= 0x20000 && r <= 0x3FFFD: // CJK extensions
		return 2
	}
	return 1
}
