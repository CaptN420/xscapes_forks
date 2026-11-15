package scenes

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"testing"

	"github.com/donlucasx/xscapes/internal/canvas"
	"github.com/donlucasx/xscapes/internal/term"
)

// TestTheStudyVistaIsUnchanged holds the study painter -- the page's vista
// clip -- byte-identical across the layout refactor that made it a scape
// (s35). The hash was taken from HEAD f875b43 before a line of forest.go
// moved; four hours x three work levels x 24 frames at 80x24, every cell's
// glyph, foreground and background on the cube.
//
// If this fails on purpose (the vista is redrawn), re-take the hash and say
// why in the commit. If it fails by accident, the page's clip has changed.
func TestTheStudyVistaIsUnchanged(t *testing.T) {
	// The original golden does not reproduce on amd64 (Windows 11, go1.27.4;
	// identical with GOAMD64=v3, and at the repo's first commit, so it is not
	// a regression). Suspected cause: float math fused on arm64 only; not
	// isolated. One golden per architecture, each from a real run there: arm64
	// is the original recording, amd64 was taken 2026-11-14. An unknown GOARCH
	// skips rather than fail against someone else's floats.
	goldens := map[string]string{
		"arm64": "24c6c0f42e5ac13187b4c3ad6ad294d2c00a07faebe279b03ab5125550c6dc9b",
		"amd64": "00810ba69ac183e59cde87c458791beb6439ef4fd53067006a7e112f2d4d81c4",
	}
	want, ok := goldens[runtime.GOARCH]
	if !ok {
		t.Skipf("no golden recorded for GOARCH=%s", runtime.GOARCH)
	}
	h := sha256.New()
	for _, tod := range []float64{0, 0.24, 0.5, 0.78} {
		for _, lv := range []float64{0, 0.5, 1} {
			for k := 0; k < 24; k++ {
				c := canvas.New(80, 24, canvas.AlphaFar, canvas.AlphaMid, canvas.AlphaNear)
				Forest[0].Paint(c, tod, float64(k)/6, lv, 7)
				for y := 0; y < 24; y++ {
					for x := 0; x < 80; x++ {
						r, fg, bg := c.ResolveAt(x, y, term.Profile256)
						fmt.Fprintf(h, "%d:%d:%d:%d:%d:%d ", x, y, r, fg.Index256(), bg.Index256(), 0)
					}
				}
			}
		}
	}
	if got := fmt.Sprintf("%x", h.Sum(nil)); got != want {
		t.Errorf("the study vista's frames changed: %s, want %s", got, want)
	}
}
