package main

import (
	"fmt"
	"strings"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/companion"
	"github.com/donlucasx/xscapes/internal/scape"
	"github.com/donlucasx/xscapes/internal/term"
)

// catStyleSheet explores visual style options for the companion cat.
// Shows current quadrant rendering alongside braille, ASCII shade, and
// experimental density variations.

// unicodeShade maps block fill percentage to ░▒▓█ density ramp.
var unicodeShade = []rune(" ░▏▎▍▌▋▊▉█")

// ToUnicodeShade collapses 2x4 pixels into one character from the unicode
// shade ramp, giving smooth tonal gradations across 10 density levels.
func ToUnicodeShade(b *companion.Bitmap) []string {
	var out []string
	for cy := 0; cy < (b.H+3)/4; cy++ {
		var sb strings.Builder
		for cx := 0; cx < (b.W+1)/2; cx++ {
			n := 0
			for dx := 0; dx < 2; dx++ {
				for dy := 0; dy < 4; dy++ {
					if b.At(cx*2+dx, cy*4+dy) {
						n++
					}
				}
			}
			idx := n * (len(unicodeShade) - 1) / 8
			sb.WriteRune(unicodeShade[idx])
		}
		out = append(out, sb.String())
	}
	return out
}

// ToHatch maps filled pixels to diagonal hatching patterns for fur texture.
var hatchChars = []rune(`|\\-/`)

// ToHatch produces a fur-texture rendering using diagonal line characters.
func ToHatch(b *companion.Bitmap) []string {
	var out []string
	for cy := 0; cy < (b.H+3)/4; cy++ {
		var sb strings.Builder
		for cx := 0; cx < (b.W+1)/2; cx++ {
			n := 0
			for dx := 0; dx < 2; dx++ {
				for dy := 0; dy < 4; dy++ {
					if b.At(cx*2+dx, cy*4+dy) {
						n++
					}
				}
			}
			idx := n * (len(hatchChars) - 1) / 8
			sb.WriteRune(hatchChars[idx])
		}
		out = append(out, sb.String())
	}
	return out
}

// doubleCatRows doubles the source bitmap resolution by expanding each '#'
// into a 2x2 block and each space into 2x2 empty cells.
func doubleCatRows(rows []string) []string {
	var out []string
	for _, row := range rows {
		r := []rune(row)
		var doubled strings.Builder
		for _, c := range r {
			if c == '#' {
				doubled.WriteString("####")
			} else {
				doubled.WriteString("    ")
			}
		}
		out = append(out, doubled.String())
		out = append(out, doubled.String())
	}
	return out
}

func catStylePage(seed int64) string {
	bm := companion.ParseBitmap(companion.CatBody)
	warm := term.RGB{R: 236, G: 228, B: 210}   // furCol
	cool := term.RGB{R: 180, G: 200, B: 220}    // cool blue-grey
	charcoal := term.RGB{R: 100, G: 100, B: 110} // monochrome

	// Three renderings of the same art
	quadrants := bm.ToQuadrant()
	braille := bm.ToBraille()
	shades := bm.ToChars()
	unicode := ToUnicodeShade(bm)
	hatched := ToHatch(bm)

	var b strings.Builder
	b.WriteString(`<style>.win{border:1px solid #2a2a32;border-radius:6px;overflow:hidden}</style>`)
	b.WriteString(`<h1>xscapes &mdash; cat visual styles</h1>`)

	// Show each style enlarged and on-beach
	stYLES := []struct {
		name string
		note string
		rows []string
		col  term.RGB
	}{
		{"quadrants (shipped)", "2x2 pixels per cell · 12×7 cells · ▘▝▀▖▌█ family", quadrants, warm},
		{"braille", "2x4 pixels per cell · 6×14 cells · ⠁⠃⠆⠇⠋⠛ family", braille, warm},
		{"ascii shade", "2x4 pixels per cell · 6×7 cells · .:-=+*#@ ramp", shades, warm},
		{"unicode shade", "2x4 pixels per cell · 6×7 cells · ░▏▎▍▌▋▊▉█ smooth density", unicode, warm},
		{"hatched fur", "2x4 pixels per cell · 6×7 cells · |\\-/ diagonal texture", hatched, warm},
	}

	for _, s := range stYLES {
		b.WriteString(styleCard(s.name, s.note, s.rows, s.col, seed))
	}

	// Color variations on the same art
	b.WriteString(`<h2>color variations</h2>`)
	colorStyles := []struct {
		name string
		note string
		rows []string
		col  term.RGB
	}{
		{"warm fur", "shipped tone · #ECA4", quadrants, warm},
		{"cool blue", "moonlit grey-blue · #B4C8DC", quadrants, cool},
		{"charcoal", "monochrome · #64646E", quadrants, charcoal},
		{"unicode shade warm", "smooth density on warm coat", unicode, warm},
		{"unicode shade cool", "smooth density on cool coat", unicode, cool},
	}
	for _, s := range colorStyles {
		b.WriteString(styleCard(s.name, s.note, s.rows, s.col, seed))
	}

	// Background color variations using SetBGColor
	b.WriteString(`<h2>background colors</h2>`)
	bgColors := []struct {
		name string
		note string
		bg   term.RGB
	}{
		{"blue-gray", "custom · #1E2D3C", term.RGB{R: 30, G: 45, B: 60}},
		{"dark green", "custom · #141E14", term.RGB{R: 20, G: 30, B: 20}},
		{"burgundy", "custom · #28141E", term.RGB{R: 40, G: 20, B: 30}},
		{"navy", "custom · #0F1423", term.RGB{R: 15, G: 20, B: 35}},
		{"gray", "custom · #1E1E1E", term.RGB{R: 30, G: 30, B: 30}},
	}
	for _, bc := range bgColors {
		b.WriteString(bgCard(bc.name, bc.note, bc.bg, seed))
	}

	// Experimental: higher density by doubling source first
	doubledRows := doubleCatRows(companion.CatBody)
	b.WriteString(`<h2>higher density</h2>`)
	for _, fn := range []struct {
		name string
		note string
		fn   func(*companion.Bitmap) []string
	}{
		{"quadrants ×2", "48×56 source → 12×7 cells · same glyph set, more detail", func(b *companion.Bitmap) []string { return b.ToQuadrant() }},
		{"braille ×2", "48×56 source → 6×14 cells · finer braille dots", func(b *companion.Bitmap) []string { return b.ToBraille() }},
		{"unicode shade ×2", "48×56 source → 6×7 cells · smooth density at 2x", func(b *companion.Bitmap) []string { return ToUnicodeShade(b) }},
		{"hatched ×2", "48×56 source → 6×7 cells · fine fur texture", func(b *companion.Bitmap) []string { return ToHatch(b) }},
	} {
		rows := fn.fn(companion.ParseBitmap(doubledRows))
		b.WriteString(styleCard(fn.name, fn.note, rows, warm, seed))
	}

	return canvas.HTMLPage("xscapes — cat visual styles", b.String())
}

func styleCard(name, note string, rows []string, col term.RGB, seed int64) string {
	s := companion.Sprite{Rows: rows, Body: col}
	sw, sh := s.Size()

	big := canvas.New(sw+2, sh+2, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	for y := 0; y < big.H; y++ {
		for x := 0; x < big.W; x++ {
			big.SetBG(x, y, term.RGB{R: 27, G: 27, B: 33})
		}
	}
	s.Draw(big.Near(), 1, 1)

	live := canvas.New(80, 24, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	scape.NewShore(seed, false).Update(live, 2.0, scape.Activity{})
	top := live.H - 2 - sh
	s.Draw(live.Near(), 8, top)

	return fmt.Sprintf(`<div class="card"><div class="meta"><div class="nm">%s</div>`+
		`<div class="rg">%d rows × %d cols</div><div class="nt">%s</div></div>`+
		`<div class="big"><div class="lbl">enlarged</div>%s</div>`+
		`<div><div class="lbl">on shore, 1:1</div>%s</div></div>`,
		name, sh, len([]rune(rows[0])), note, big.HTMLFragment(24), live.HTMLFragment(10))
}

// bgCard shows the cat with a custom background color set before drawing.
func bgCard(name, note string, bg term.RGB, seed int64) string {
	cat := companion.NewCat()

	live := canvas.New(80, 24, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	scape.NewShore(seed, false).Update(live, 2.0, scape.Activity{})
	_, chh := cat.Size()
	top := live.H - 2 - chh
	// Paint the background behind where the cat will sit.
	for dy := 0; dy < chh; dy++ {
		for dx := 0; dx < 12; dx++ {
			live.SetBG(8+dx, top+dy, bg)
		}
	}
	cat.Draw(live.Near(), 8, top, 3.0, companion.Working)

	big := canvas.New(14, 9, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
	for y := 0; y < big.H; y++ {
		for x := 0; x < big.W; x++ {
			big.SetBG(x, y, bg)
		}
	}
	cat.Draw(big.Near(), 1, 1, 3.0, companion.Working)

	return fmt.Sprintf(`<div class="card"><div class="meta"><div class="nm">%s</div>`+
		`<div class="rg">custom background</div><div class="nt">%s</div></div>`+
		`<div class="big"><div class="lbl">enlarged</div>%s</div>`+
		`<div><div class="lbl">on shore, 1:1</div>%s</div></div>`,
		name, note, big.HTMLFragment(24), live.HTMLFragment(10))
}
