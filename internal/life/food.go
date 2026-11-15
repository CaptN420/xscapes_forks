package life

// Food, and what the animal does about it.
//
// This is the smallest part of the companion and deliberately the silliest: a
// reaction is chosen by what was fed, varied by a seeded draw, and played once.
// Nothing here can go wrong for the animal -- there is no bad food, no poison, no
// overfeeding penalty. A spicy snack is not a mistake; it is a fire.

// Food is one thing that can be given. Glyph is the one-cell mark it makes.
type Food struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Glyph    rune    `json:"glyph"`
	Reaction string  `json:"reaction"`
	Relief   float64 `json:"relief"` // how much appetite it settles
	Line     string  `json:"line"`   // what the animal says, before variation
}

// The reactions a food can trigger. These are drawn by the scape; the engine only
// names them, so a species with no fire-breathing art can simply map Fire to a
// hop and nothing breaks.
const (
	ReactDance     = "dance"
	ReactCatch     = "catch"
	ReactFire      = "fire"
	ReactDizzy     = "dizzy"
	ReactCelebrate = "celebrate"
)

// Foods is the whole larder, in the order a person sees it.
var Foods = []Food{
	{ID: "berry", Name: "a sweet berry", Glyph: 'o', Reaction: ReactDance, Relief: 30,
		Line: "Sweet! *wobbles with delight*"},
	{ID: "bug", Name: "a fat beetle", Glyph: '~', Reaction: ReactCatch, Relief: 45,
		Line: "Got it! *snap*"},
	{ID: "spicy", Name: "a spicy snack", Glyph: '^', Reaction: ReactFire, Relief: 25,
		Line: "...hot. HOT! *breathes a little fire*"},
	{ID: "mushroom", Name: "a mystery mushroom", Glyph: '@', Reaction: ReactDizzy, Relief: 35,
		Line: "*sees colours for a moment*"},
	{ID: "treat", Name: "a special treat", Glyph: '*', Reaction: ReactCelebrate, Relief: 50,
		Line: "*performs a small, unrequested celebration*"},
}

// FoodByID finds one of the larder, and whether it exists. An unknown id is not
// an error the animal cares about; callers fall back to a random food.
func FoodByID(id string) (Food, bool) {
	for _, f := range Foods {
		if f.ID == id {
			return f, true
		}
	}
	return Food{}, false
}

// eat is the feed path, with or without a named food. It is the one place a
// reaction is chosen, so every way of feeding produces one.
func (b *Brain) eat(id string) Result {
	f, ok := FoodByID(id)
	if !ok {
		f = Foods[b.rng.intn(len(Foods))]
	}
	// A full animal still gets the whole show: refusing food would be the only
	// thing in this package that could read as a scolding.
	b.attrs.Appetite -= f.Relief
	if b.attrs.Appetite < 0 {
		b.attrs.Appetite = 0
	}
	b.attrs.Happiness = bump(b.attrs.Happiness, 4)
	b.attrs.Friendship = bump(b.attrs.Friendship, 2)
	b.foodsEaten++
	b.foodSeen[f.ID]++
	b.touch()
	b.startReaction(f.Reaction)
	b.setPose(Excited, 3.5)
	b.say(f.Line)
	b.remember("ate " + f.Name)
	return Result{OK: true, Message: "It eats the " + f.Name[2:] + ". " + f.Line, Speech: b.speech}
}

// startReaction arms a visual reaction. The variant is a seeded number so two
// helpings of the same food do not look identical, while a replay of the same
// life does.
func (b *Brain) startReaction(kind string) {
	d := 1.8
	switch kind {
	case ReactFire:
		d = 2.2
	case ReactCelebrate:
		d = 2.6
	case ReactDance:
		d = 2.4
	}
	b.react = kind
	b.reactTotal = d
	b.reactLeft = d
	b.reactVariant = b.rng.intn(1000)
}

// Reaction is the reaction in flight, and how far through it is (0..1).
func (b *Brain) Reaction() (string, float64, int) {
	if b.reactLeft <= 0 || b.reactTotal <= 0 {
		return "", 0, 0
	}
	phase := 1 - b.reactLeft/b.reactTotal
	if phase < 0 {
		phase = 0
	}
	if phase > 1 {
		phase = 1
	}
	return b.react, phase, b.reactVariant
}

// FoodsEaten is how many helpings it has had, and FoodSeen the per-food count,
// which the CLI shows as its taste so far.
func (b *Brain) FoodsEaten() int { return b.foodsEaten }
func (b *Brain) FoodSeen() map[string]int {
	out := make(map[string]int, len(b.foodSeen))
	for k, v := range b.foodSeen {
		out[k] = v
	}
	return out
}
