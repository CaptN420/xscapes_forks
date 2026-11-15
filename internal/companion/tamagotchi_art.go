package companion

import "github.com/donlucasx/xscapes/internal/term"

// ---------------------------------------------------------------------------
// THE CREATURE'S ART.
//
// Authored the way every other companion in this package is: as a 1-bit bitmap
// at twice the character resolution (ParseBitmap, '#' is ink), drawn with
// ToQuadrant, so the picture gets square pixels at 2x2 rather than the 1:2
// stretched cells of a character grid. 24 source columns by 28 rows is the
// same 12x7 character box the cat and the crab live in.
//
// The body is split in two, exactly as the crab's shell is (crab.go): the LOWER
// half -- the feet -- never changes, and the UPPER half is what a pose swaps.
// Four uppers and one lower carry every animation in tamagotchi_anim.go.
//
// THE EYES AND THE MOUTH ARE A SEPARATE PASS, like the crab's near eyes
// (crab_near.go), and for the same measured reason: a single character cell
// cannot carry an expression on a 12-cell face -- it is a speck of dirt. They
// are 4 source px by 4, which is 2 cells by 1, drawn with PlotOn and a coat
// ground so the ring's hole shows fur and not the sea behind the head.
// ---------------------------------------------------------------------------

// tamBodyCols and tamBodyRows are the authored size. Every row below is
// tamBodyCols runes; ParseBitmap panics on anything else, and the test for this
// file asserts it before the panic can happen at run time.
const (
	tamBodyCols  = 24
	tamBodyRows  = 28
	tamUpperRows = 16 // rows 0..15 of the 28
	tamEyeColA   = 2  // left eye, cell column (two cells wide: 2-3)
	tamEyeColB   = 7  // right eye, cells 7-8
	tamEyeRow    = 2  // the eye band, in CELL rows, two rows tall
	tamMouthRow  = 4  // over the top of the lower half, which is where a chin is
	tamMouthCol  = 5  // cells 5-6
)

// tamLowerArt is the feet. Shared by every pose.
var tamLowerArt = []string{
	".....##############.....",
	"....################....",
	"....################....",
	"....################....",
	"....################....",
	"....################....",
	".....##############.....",
	".....##############.....",
	"......############......",
	".......####..####.......",
	".......####..####.......",
	".......####..####.......",
}

// tamUpperArt is one upper half per body in tamagotchi_anim.go's Body* order:
// stand, arms (shoulders out, antennae spread), curl (antennae drawn in, head
// sunk a cell), lean (head bowed forward), puff (swollen, antennae wide).
//
// The silhouette is deliberately NARROWER than the 12 cells it lives in -- 8 at
// its widest. A blob that fills its box has nowhere to put an arm: the first
// version of this art did fill its box, so every pose was the same rectangle,
// and the two most different bodies differed by 16 pixels out of 672.
var tamUpperArt = [BodyCount][]string{
	{ // BodyStand -- antennae up, round
		".......##......##.......",
		".......##......##.......",
		"......############......",
		".....##############.....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		".....##############.....",
		"......############......",
	},
	{ // BodyArms -- shoulders out and antennae spread: joy and play
		".......##......##.......",
		".......##......##.......",
		"......############......",
		"....################....",
		"...##################...",
		"..####################..",
		"..####################..",
		"...##################...",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		".....##############.....",
		"......############......",
	},
	{ // BodyCurl -- antennae gone, head sunk a cell: sad, tired, sleep
		"........................",
		"........................",
		"........................",
		"........................",
		"......############......",
		".....##############.....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		".....##############.....",
		"......############......",
	},
	{ // BodyLean -- the head pitched forward and to the right, for eating
		"........................",
		".........##......##.....",
		"........############....",
		".......##############...",
		"......##############....",
		".....###############....",
		".....###############....",
		".....###############....",
		".....###############....",
		".....###############....",
		".....###############....",
		"....################....",
		"....################....",
		"....################....",
		"....################....",
		".....##############.....",
	},
	{ // BodyPuff -- swollen and spread, antennae wide: unwell
		"........................",
		"....##............##....",
		"....################....",
		"...##################...",
		"...##################...",
		"...##################...",
		"...##################...",
		"...##################...",
		"...##################...",
		"...##################...",
		"...##################...",
		"...##################...",
		"...##################...",
		"...##################...",
		"....################....",
		".....##############.....",
	},
}

// ---------------------------------------------------------------------------
// THE FACE. Nine eyes and eight mouths, 4x4 source px each, in the Eye*/Mouth*
// order of tamagotchi_anim.go.
// ---------------------------------------------------------------------------

var tamEyeArt = [EyeCount][]string{
	EyeOpen: { // the resting eye: a rounded block, two cells by two
		".##.",
		"####",
		"####",
		"####",
		"####",
		"####",
		"####",
		".##.",
	},
	EyeHappy: { // "^ ^", the content eye: an arch
		"....",
		".##.",
		"#..#",
		"#..#",
		"#..#",
		"#..#",
		"....",
		"....",
	},
	EyeSparkle: { // a diamond, for joy and play
		"..#.",
		".##.",
		"####",
		"####",
		"####",
		".##.",
		"..#.",
		"....",
	},
	EyeLove: { // the affection reaction
		".##.",
		"####",
		"####",
		"####",
		".##.",
		"....",
		"....",
		"....",
	},
	EyeSad: { // the lid heavy, looking down
		"####",
		"####",
		"#..#",
		"#..#",
		"#..#",
		"....",
		"....",
		"....",
	},
	EyeWide: { // the ring: surprise. Painted amber, as the crab's ask is
		"####",
		"#..#",
		"#..#",
		"#..#",
		"#..#",
		"#..#",
		"#..#",
		"####",
	},
	EyeShut: { // asleep, and the blink: a lid
		"....",
		"....",
		"....",
		"####",
		"####",
		"####",
		"....",
		"....",
	},
	EyeSleepy: { // a slanted lid, for the sigh
		"....",
		"....",
		"#...",
		"##..",
		".##.",
		"..##",
		"....",
		"....",
	},
	EyeSick: { // unwell
		"#..#",
		"#..#",
		".##.",
		".##.",
		"#..#",
		"#..#",
		"....",
		"....",
	},
}

var tamMouthArt = [MouthCount][]string{
	MouthFlat: {
		"....",
		"....",
		"####",
		"####",
	},
	MouthSmile: {
		"....",
		"#..#",
		".##.",
		"....",
	},
	MouthOpen: {
		"####",
		"#..#",
		"#..#",
		"####",
	},
	MouthChew: {
		".##.",
		".##.",
		".##.",
		".##.",
	},
	MouthTiny: {
		"....",
		".##.",
		".##.",
		"....",
	},
	MouthFrown: {
		".##.",
		".##.",
		"#..#",
		"....",
	},
	MouthWide: {
		"#..#",
		"####",
		"####",
		"#..#",
	},
}

// The eye's own colours. eyeCol, EyeAlert and eyeWorried are the companion
// package's existing inks (cat.go), reused so a Tamagotchi's alarm reads the
// same amber as the crab's.
func tamEyeInk(e int) term.RGB {
	switch e {
	case EyeWide:
		return EyeAlert
	case EyeSad, EyeSick:
		return eyeWorried
	case EyeShut, EyeSleepy:
		return shade(eyeCol, -0.45)
	}
	return eyeCol
}

// tamMouthInk is the same muted rose the cat's nose is drawn in.
var tamMouthInk = term.RGB{R: 226, G: 138, B: 148}

// tamMouthOpenInk is the inside of an open mouth, and it is the one place on
// the creature that is allowed to be dark: a mouth that is not a line.
var tamMouthOpenInk = term.RGB{R: 150, G: 84, B: 96}

// ---------------------------------------------------------------------------
// THE OVERLAYS. Hearts, sparkles, a tear, a crumb -- all 1-bit art, so they sit
// in the same register as the creature instead of looking like punctuation.
// ---------------------------------------------------------------------------

var (
	tamHeartArt = []string{
		".##.##",
		"######",
		"######",
		".####.",
		"..##..",
		"......",
	}
	tamSparkleArt = []string{
		"..#...",
		".###..",
		"#####.",
		".###..",
		"..#...",
		"......",
	}
	tamTearArt = []string{
		"..#.",
		".##.",
		"####",
		".##.",
	}
	tamCrumbArt = []string{
		"....",
		".##.",
		".##.",
		"....",
	}
)

// tamEyeShineFor is how much of a shine an eye can carry. It is derived from
// the eye rather than stored in the pose, so an expression cannot forget it.
func tamEyeShineFor(e int) float64 {
	switch e {
	case EyeOpen:
		return 1
	case EyeWide:
		return 0.9
	case EyeLove:
		return 0.8
	case EyeSparkle:
		return 0.7
	case EyeHappy:
		return 0.6
	case EyeSad:
		return 0.4
	case EyeSick:
		return 0.3
	case EyeSleepy:
		return 0.25
	}
	return 0
}

// tamHeartInk is warm rose, tamSparkleInk the same gold the shore's foam
// highlights use, tamTearInk a cool blue, tamCrumbInk the food colour.
var (
	tamHeartInk   = term.RGB{R: 244, G: 150, B: 176}
	tamSparkleInk = term.RGB{R: 255, G: 226, B: 120}
	tamTearInk    = term.RGB{R: 150, G: 200, B: 236}
	tamCrumbInk   = term.RGB{R: 214, G: 170, B: 118}
	tamBlushInk   = term.RGB{R: 236, G: 156, B: 166}
	// tamShineInk is the highlight inside an eye -- the byte that turns a dark
	// slab into a look. Pale, slightly warm, never pure white: at 2x2 cells a
	// white pixel reads as a hole in the animal, not as a shine.
	tamShineInk = term.RGB{R: 232, G: 240, B: 236}
	tamZzzInk   = term.RGB{R: 176, G: 186, B: 226}
)
