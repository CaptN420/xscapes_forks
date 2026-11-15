package life

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Save is the whole animal on disk: a life, not a log. Everything transient --
// the bout it is in, where it is standing, what it is saying -- is left out on
// purpose, so reopening the scape finds an animal doing something, rather than
// frozen mid-sentence from last Tuesday.
type Save struct {
	Version   int        `json:"version"`
	Seed      int64      `json:"seed"`
	Stage     Stage      `json:"stage"`
	StageSecs float64    `json:"stage_secs"`
	Age       float64    `json:"age"`
	Attrs     Attributes `json:"attributes"`
	Finds     []string   `json:"finds,omitempty"`
	Toys      []string   `json:"toys,omitempty"`
	Memories  []string   `json:"memories,omitempty"`
	LastSaid  string     `json:"last_said,omitempty"`
	// What it has eaten. Two helpings are a taste, not a scoreboard, so these
	// are recorded and never branch anything.
	FoodsEaten int            `json:"foods_eaten,omitempty"`
	FoodSeen   map[string]int `json:"food_seen,omitempty"`
	// Legacy records that this life began as the earlier creature, so the first
	// run after a migration can say so instead of quietly pretending.
	Legacy bool `json:"migrated_from_legacy,omitempty"`
}

// SaveVersion is bumped when the shape changes; Load refuses the future and
// migrates the past.
const SaveVersion = 2

// Snapshot is the brain as a value.
func (b *Brain) Snapshot() Save {
	return Save{
		Version:    SaveVersion,
		Seed:       b.seed,
		Stage:      b.stage,
		StageSecs:  b.stageSecs,
		Age:        b.age,
		Attrs:      b.attrs,
		Finds:      append([]string(nil), b.finds...),
		Toys:       append([]string(nil), b.toys...),
		Memories:   append([]string(nil), b.memories...),
		FoodsEaten: b.foodsEaten,
		FoodSeen:   b.FoodSeen(),
		LastSaid:   b.lastSaid,
	}
}

// Restore puts a snapshot back into a fresh brain.
func (b *Brain) Restore(s Save) {
	b.seed = s.Seed
	if b.seed == 0 {
		b.seed = 1
	}
	b.rng = rng{s: uint64(b.seed) | 1}
	b.stage = s.Stage
	if b.stage < 0 || b.stage >= StageCount {
		b.stage = Baby
	}
	b.stageSecs = s.StageSecs
	b.age = s.Age
	b.attrs = s.Attrs
	b.attrs.clampAll()
	b.finds = append([]string(nil), s.Finds...)
	b.toys = append([]string(nil), s.Toys...)
	b.memories = append([]string(nil), s.Memories...)
	b.lastSaid = s.LastSaid
	b.foodsEaten = s.FoodsEaten
	b.foodSeen = make(map[string]int, len(s.FoodSeen)+len(Foods))
	for k, v := range s.FoodSeen {
		b.foodSeen[k] = v
	}
	b.attrs.Discoveries = len(b.finds)
	b.attrs.Toys = len(b.toys)
	b.attrs.Memories = len(b.memories)
	b.pose = Idle
	b.gapLeft = 2
	b.nextSelf = 4 * 60
	b.publish()
}

// clampAll keeps a hand-edited or migrated save from poisoning the arithmetic.
func (a *Attributes) clampAll() {
	a.Happiness = bump(a.Happiness, 0)
	a.Friendship = bump(a.Friendship, 0)
	a.Curiosity = bump(a.Curiosity, 0)
	a.Exploration = bump(a.Exploration, 0)
	a.Appetite = bump(a.Appetite, 0)
	if a.Discoveries < 0 {
		a.Discoveries = 0
	}
	if a.Toys < 0 {
		a.Toys = 0
	}
	if a.Memories < 0 {
		a.Memories = 0
	}
}

// Load reads a save. A missing file is not an error -- it is a new animal.
func Load(path string) (*Brain, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var s Save
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, false
	}
	if s.Version > SaveVersion {
		return nil, false
	}
	b := New(s.Seed)
	b.Restore(s)
	return b, true
}

// Write puts the animal on disk atomically: a scape killed mid-write must not
// leave a half-written life behind.
func (b *Brain) Write(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b.Snapshot(), "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// legacy is the old creature's file, read only for a migration. It is a plain
// map on purpose: the old struct is gone from the default path, and a
// migration must not depend on its types surviving.
type legacy struct {
	Stage     *int     `json:"stage"`
	StageSecs float64  `json:"stage_secs"`
	Affection float64  `json:"affection"`
	Happiness float64  `json:"happiness"`
	Fed       int      `json:"fed_count"`
	Played    int      `json:"played_count"`
	Pet       int      `json:"petted_count"`
	Cleaned   int      `json:"cleaned_count"`
	Memories  []string `json:"memory_entries"`
}

// Migrate reads the earlier creature's save and carries across what it can:
// the stage it reached, the time already served, and every kindness it was
// shown as friendship. Punishment fields -- health, setbacks, neglect -- are
// dropped on the floor, because the new animal does not have them.
func Migrate(legacyPath string) (*Brain, bool) {
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return nil, false
	}
	var l legacy
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, false
	}
	s := Save{Version: SaveVersion, Seed: seedFromBytes(data)}
	if l.Stage != nil {
		s.Stage = Stage(*l.Stage)
	}
	if s.Stage < 0 || s.Stage >= StageCount {
		s.Stage = Baby
	}
	s.StageSecs = l.StageSecs
	s.Age = l.StageSecs
	s.Attrs.Happiness = l.Happiness
	kindnesses := l.Fed*2 + l.Played*3 + l.Pet*2 + l.Cleaned
	s.Attrs.Friendship = bump(l.Affection/2+float64(kindnesses), 0)
	s.Attrs.Memories = len(l.Memories)
	s.Memories = append([]string(nil), l.Memories...)
	s.Legacy = true
	b := New(s.Seed)
	b.Restore(s)
	b.say("I remember you")
	return b, true
}

// seedFromBytes derives a stable seed from a file, so the same old animal
// becomes the same new animal every time the migration is retried.
func seedFromBytes(b []byte) int64 {
	var h uint64 = 1469598103934665603
	for _, c := range b {
		h ^= uint64(c)
		h *= 1099511628211
	}
	if h == 0 {
		h = 1
	}
	return int64(h >> 1)
}
