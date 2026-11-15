package life

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// The guarantees this package exists to keep. Each test names the sentence in
// the brief it is holding the code to.
// ---------------------------------------------------------------------------

// "Ignoring the companion must never negatively affect it."
func TestADayOfBeingIgnoredCostsNothing(t *testing.T) {
	b := New(7)
	w := World{Working: true, Level: 0.4}

	start := b.Attributes()
	minHappiness, minFriendship, minCuriosity := start.Happiness, start.Friendship, start.Curiosity

	// A whole day, one second at a time, with the user never there.
	for i := 0; i < 24*3600; i++ {
		b.Tick(1, w)
		a := b.Attributes()
		if a.Happiness < minHappiness {
			t.Fatalf("happiness fell from %.1f to %.1f at second %d: ignoring it punished it",
				minHappiness, a.Happiness, i)
		}
		if a.Friendship < minFriendship {
			t.Fatalf("friendship fell at second %d", i)
		}
		if a.Curiosity < minCuriosity {
			t.Fatalf("curiosity fell at second %d", i)
		}
		minHappiness, minFriendship, minCuriosity = a.Happiness, a.Friendship, a.Curiosity
		if a.Happiness > 100 || a.Appetite > 100 || a.Exploration > 100 || a.Curiosity > 100 {
			t.Fatalf("an attribute escaped its range at second %d: %+v", i, a)
		}
		if a.Appetite < 0 {
			t.Fatalf("appetite went negative at second %d", i)
		}
	}

	// And it is alive, grown, and still content -- not merely un-punished.
	if st, _ := b.Stage(); st != Adult {
		t.Errorf("a day of life left it at %s: time alone must be enough", st)
	}
	if b.Attributes().Happiness < 40 {
		t.Errorf("a day of being ignored left it at %.0f%% happiness, which reads as sad",
			b.Attributes().Happiness)
	}
	// It also did something with its time.
	if len(b.Memories()) == 0 {
		t.Error("a whole day passed and it remembered nothing")
	}
}

// "Progression should reward engagement without requiring it."
func TestTimeIsEnoughAndEngagementIsABonus(t *testing.T) {
	// Ignored: exactly the stage times, plus a hair.
	ignored := New(11)
	for i := 0; i < int(StageSecs[Baby])+2; i++ {
		ignored.Tick(1, World{})
	}
	if st, _ := ignored.Stage(); st != Child {
		t.Errorf("after %d s of plain time it is %s, want Child", int(StageSecs[Baby]), st)
	}

	// Engaged: the same clock goes by faster, but not absurdly faster.
	engaged := New(11)
	w := World{}
	perTouch := 0.0
	for i := 0; i < int(StageSecs[Baby])+2; i++ {
		perTouch += 1
		if perTouch >= 60 { // a touch a minute keeps engagement saturated
			perTouch = 0
			engaged.Do(CmdPet)
		}
		engaged.Tick(1, w)
	}
	if st, _ := engaged.Stage(); st != Child {
		t.Errorf("the petted animal is %s after the same time", st)
	}
	// Measured: engagement runs growth at 1.5x at most, so the saving is a third
	// and never more. A creature that is played with must not skip stages.
	if engaged.attrs.Friendship <= ignored.attrs.Friendship {
		t.Error("petting did not raise friendship")
	}
}

// "Avoid evolutions that are too fast", and a huge dt must not jump a stage.
func TestLifeCannotSkipAHundredYearsInAFrame(t *testing.T) {
	b := New(3)
	b.Tick(1e9, World{})
	if st, _ := b.Stage(); st != Baby {
		t.Errorf("a single absurd dt carried it to %s", st)
	}
	if b.age > 1.01 {
		t.Errorf("a single tick aged it %.1f seconds: dt is not clamped", b.age)
	}
}

// Every verb answers, and every verb produces something to see.
func TestEveryInteractionAnswersAndShows(t *testing.T) {
	type verb struct {
		cmd  string
		want Pose
	}
	verbs := []verb{
		{CmdFeed, Excited},
		{CmdPet, Resting},
		{CmdPlay, Excited},
		{CmdWalk, Walking},
		{CmdToy, Excited},
	}
	for _, v := range verbs {
		b := New(5)
		b.attrs.Appetite = 60 // so feeding is welcome
		before := b.Attributes()
		res := b.Do(v.cmd)
		if !res.OK {
			t.Errorf("%s was refused: %s", v.cmd, res.Message)
		}
		if res.Message == "" || res.Speech == "" {
			t.Errorf("%s answered with nothing to read", v.cmd)
		}
		if b.pose != v.want {
			t.Errorf("%s posed %s, want %s", v.cmd, b.pose, v.want)
		}
		if b.boutLeft <= 0 {
			t.Errorf("%s did not hold a pose", v.cmd)
		}
		after := b.Attributes()
		if after.Happiness < before.Happiness || after.Friendship < before.Friendship {
			t.Errorf("%s made it worse: %+v -> %+v", v.cmd, before, after)
		}
	}

	// Feeding a full animal is still a little show, not a refusal: the only
	// thing in this package that could read as a scolding is turning down food.
	b := New(5)
	b.attrs.Appetite = 2
	res := b.Do(CmdFeed)
	if !res.OK {
		t.Errorf("a full animal refused food as an error: %s", res.Message)
	}
	if b.attrs.Appetite != 0 {
		t.Errorf("appetite should settle at zero, not go negative: %.1f", b.attrs.Appetite)
	}
	if b.react == "" {
		t.Error("a full animal ate without any reaction")
	}

	// Explore sends it out; calling brings it back; calling again is harmless.
	b = New(5)
	if !b.Do(CmdExplore).OK {
		t.Fatal("explore was refused")
	}
	if !b.away {
		t.Fatal("explore did not send it out")
	}
	if b.Do(CmdExplore).OK {
		t.Error("it was sent out twice")
	}
	if !b.Do(CmdCall).OK || b.away {
		t.Error("calling it home did not bring it back")
	}
	if !b.Do(CmdCall).OK {
		t.Error("calling a present animal is not an error")
	}
}

// "Return with small stories or discoveries."
func TestEveryAdventureBringsSomethingBack(t *testing.T) {
	b := New(9)
	b.Do(CmdExplore)
	if b.Intent().Pose != Away {
		t.Fatalf("an away animal intends %s", b.Intent().Pose)
	}
	// It is away for a bounded time, and then it is home with a find.
	for i := 0; i < 400 && b.away; i++ {
		b.Tick(1, World{})
	}
	if b.away {
		t.Fatal("it never came back")
	}
	if len(b.Finds()) != 1 {
		t.Fatalf("it came back with %d finds", len(b.Finds()))
	}
	if b.Intent().Carrying == "" {
		t.Error("it came back empty-handed in the drawing")
	}
	if _, ok := FindGlyphs[b.Finds()[0]]; !ok {
		t.Errorf("the find %q has no glyph to draw", b.Finds()[0])
	}
	if b.Attributes().Exploration < 8 {
		t.Error("an adventure did not raise exploration")
	}
}

// The intent a renderer receives is small enough for the strip the layout
// reserves, and legal at every moment.
func TestTheIntentStaysInsideTheReservedStrip(t *testing.T) {
	b := New(21)
	for i := 0; i < 3*3600; i++ {
		b.Tick(1, World{Working: i%600 < 300, Level: float64(i%100) / 100})
		in := b.Intent()
		if in.DX < -1 || in.DX > 1 {
			t.Fatalf("at second %d it wants to stand %d cells from home", i, in.DX)
		}
		if in.Pose < Idle || in.Pose > ReturningPhase {
			t.Fatalf("at second %d the pose is %d", i, in.Pose)
		}
		if len(in.Speech) > 48 {
			t.Fatalf("a line too long for the strip: %q", in.Speech)
		}
	}
}

// Two animals from one seed live the same life; two from different seeds do not.
func TestTheCreatureIsDeterministic(t *testing.T) {
	a, c := New(42), New(42)
	for i := 0; i < 600; i++ {
		w := World{Working: i%60 < 30}
		a.Tick(0.5, w)
		c.Tick(0.5, w)
		if a.Intent() != c.Intent() {
			t.Fatalf("two animals from seed 42 diverged at step %d", i)
		}
	}
	d := New(43)
	diverge := false
	for i := 0; i < 600; i++ {
		d.Tick(0.5, World{})
		same := d.Intent() == a.Intent()
		if !same {
			diverge = true
		}
	}
	if !diverge {
		t.Error("seed 42 and seed 43 behaved identically: the seed does nothing")
	}
}

// A save is a life; a broken save is not a catastrophe.
func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "life.json")

	b := New(77)
	b.attrs.Appetite = 50
	b.Do(CmdFeed)
	b.Do(CmdExplore)
	for i := 0; i < 500 && b.away; i++ {
		b.Tick(1, World{})
	}
	for i := 0; i < 900; i++ {
		b.Tick(1, World{})
	}
	b.Do(CmdToy)
	if err := b.Write(path); err != nil {
		t.Fatal(err)
	}

	back, ok := Load(path)
	if !ok {
		t.Fatal("a save this program wrote could not be read")
	}
	if bs, _ := back.Stage(); !sameStage(bs, b) {
		t.Errorf("stage did not survive")
	}
	if back.Attributes() != b.Attributes() {
		t.Errorf("attributes did not survive:\n %+v\n %+v", back.Attributes(), b.Attributes())
	}
	if len(back.Finds()) != len(b.Finds()) || len(back.Toys()) != len(b.Toys()) {
		t.Errorf("its history did not survive: %d finds, %d toys", len(back.Finds()), len(back.Toys()))
	}
	if back.Seed() != b.Seed() {
		t.Error("the seed did not survive: the animal would be a different one")
	}

	// Corrupt file: no load, no panic, and a fresh animal is still made.
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load(path); ok {
		t.Error("a corrupt save reported success")
	}
	if _, ok := Load(filepath.Join(dir, "nope.json")); ok {
		t.Error("a missing save reported success")
	}

	// A future version is refused rather than half-read.
	future := Save{Version: SaveVersion + 5, Seed: 1}
	data, _ := json.Marshal(future)
	os.WriteFile(path, data, 0o600)
	if _, ok := Load(path); ok {
		t.Error("a save from a newer version was accepted")
	}
}

// The earlier creature's file becomes a life, and its punishments are dropped.
func TestAnOldCreatureBecomesThisOne(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "tamagotchi.json")
	doc := map[string]interface{}{
		"stage": 2, "stage_secs": 400.0,
		"happiness": 63.0, "affection": 40.0,
		"fed_count": 5, "played_count": 2, "petted_count": 3, "cleaned_count": 1,
		"health": 12.0, "setbacks": 9, "memory_entries": []string{"grew up", "found a shell"},
	}
	data, _ := json.Marshal(doc)
	if err := os.WriteFile(old, data, 0o600); err != nil {
		t.Fatal(err)
	}
	b, ok := Migrate(old)
	if !ok {
		t.Fatal("the old save was not migrated")
	}
	if st, _ := b.Stage(); st != Teen {
		t.Errorf("it arrived as %s, want the stage it had reached", st)
	}
	if b.Attributes().Happiness != 63 {
		t.Errorf("happiness was lost in migration")
	}
	if len(b.Memories()) != 2 {
		t.Errorf("its memories were lost: %v", b.Memories())
	}
	if b.Attributes().Friendship < 20 {
		t.Errorf("the kindnesses it was shown did not become friendship: %.1f",
			b.Attributes().Friendship)
	}
	// And the things that used to punish it are simply not part of it.
	if b.attrs.Appetite == 0 && b.Attributes().Happiness == 0 {
		t.Error("the migration produced an empty animal")
	}
}

// The status line is legible, and says what the next stage is waiting for
// without ever making it a demand.
func TestTheStatusLineHasNoOrdersInIt(t *testing.T) {
	b := New(2)
	s := b.Status()
	if s == "" {
		t.Fatal("no status")
	}
	for _, demand := range []string{"need", "must", "hurry", "starving", "sick"} {
		if contains(s, demand) {
			t.Errorf("the status line makes a demand (%q): %s", demand, s)
		}
	}
	if !contains(b.WhatNext(), "more time") {
		t.Errorf("what-next reads %q", b.WhatNext())
	}
	b.stage = Adult
	if !contains(b.WhatNext(), "fully grown") {
		t.Errorf("an adult's what-next reads %q", b.WhatNext())
	}
}

// sameStage compares a stage against a brain's own.
func sameStage(st Stage, b *Brain) bool {
	own, _ := b.Stage()
	return st == own
}

func contains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// The frame path must not allocate: a scape runs this twelve times a second for
// hours, and the companion is not allowed to be why the terminal gets hot.
func TestATickDoesNotAllocate(t *testing.T) {
	b := New(8)
	b.attrs.Appetite = 60
	w := World{Working: true, Level: 0.6}
	// Warm up past the first bout decision and the first excursion.
	for i := 0; i < 200; i++ {
		b.Tick(1.0/12.0, w)
	}
	allocs := testing.AllocsPerRun(2000, func() { b.Tick(1.0/12.0, w) })
	if allocs > 0.05 {
		t.Errorf("a tick allocates %.2f times per call", allocs)
	}
}

func BenchmarkBrainTick(b *testing.B) {
	br := New(5)
	w := World{Working: true, Level: 0.5}
	dt := 1.0 / 12.0
	for i := 0; i < b.N; i++ {
		br.Tick(dt, w)
	}
}
