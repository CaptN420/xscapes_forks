package life

// The things it can bring back, and the things it can be given.
//
// Every one of these is something a crab could actually find on a shore, in the
// scene's own vocabulary -- no invented fantasy objects, nothing that would
// need art the world does not have. The find is drawn as a single glyph on the
// sand in the same language the litter already uses, so a discovery costs one
// cell and no new visual style.
var finds = []string{
	"a gull's feather",
	"a green glass bottle",
	"a smooth grey stone",
	"a piece of driftwood",
	"a curled shell",
	"a sea-bean",
	"a rusty key",
	"a shred of fishing line",
	"a tiny pearl",
	"a blue bottle cap",
	"a starfish arm",
	"a splinter of bone",
}

// FindGlyph is the one cell a find puts on the sand. Deliberately from the
// scene's existing glyph set: a speck, a shell, a star.
var FindGlyphs = map[string]rune{
	"a gull's feather":        '~',
	"a green glass bottle":    'o',
	"a smooth grey stone":     '·',
	"a piece of driftwood":    '=',
	"a curled shell":          '@',
	"a sea-bean":              'o',
	"a rusty key":             '?',
	"a shred of fishing line": '"',
	"a tiny pearl":            '°',
	"a blue bottle cap":       'o',
	"a starfish arm":          '*',
	"a splinter of bone":      '|',
}

var toyNames = []string{
	"pebble", "little shell", "red string", "bell", "spoon", "bottle cap",
}

// QuietLines are what it says when it is left alone for a long time: an
// observation, never a request. They exist so a person who comes back after
// hours finds something new to read instead of a reproach.
var QuietLines = []string{
	"The tide came in and went out again",
	"A gull argued with another gull",
	"Something moved under the sand",
	"Long shadows now",
	"The water sounds different at this hour",
}
