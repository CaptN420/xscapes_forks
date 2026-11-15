// Package companion includes the Tamagotchi-inspired virtual creature module.
//
// This module adds a persistent, interactive creature that lives alongside
// the existing companion system. The creature has its own state, animations,
// and personality, and reacts to player interactions.
package companion

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
)

// Mood represents the creature's current emotional state.
type Mood int

const (
	Happy Mood = iota
	Calm
	Hungry
	Tired
	Sick
	Excited
	Playful
	Thinking
	Lonely
	Ill
	Bored
	Melancholy
	Contemplative
	Joyful
	Furious
)

func (m Mood) String() string {
	switch m {
	case Happy:
		return "happy"
	case Calm:
		return "calm"
	case Hungry:
		return "hungry"
	case Tired:
		return "tired"
	case Sick:
		return "sick"
	case Excited:
		return "excited"
	case Playful:
		return "playful"
	case Thinking:
		return "thinking"
	case Lonely:
		return "lonely"
	case Ill:
		return "ill"
	case Bored:
		return "bored"
	case Melancholy:
		return "melancholy"
	case Contemplative:
		return "contemplative"
	case Joyful:
		return "joyful"
	case Furious:
		return "furious"
	}
	return "unknown"
}

// Personality defines the creature's core traits (0.0 to 1.0).
type Personality struct {
	Reactivity    float64 `json:"reactivity"`     // How reactive is it?
	MoodStability float64 `json:"mood_stability"` // How quickly does mood change?
	Sociality     float64 `json:"sociality"`      // Does it seek interaction?
	Curiosity     float64 `json:"curiosity"`      // Does it explore and comment?
	Attachment    float64 `json:"attachment"`     // How much does it bond?
	Whimsy        float64 `json:"whimsy"`         // Does it do unexpected things?
	Nostalgia     float64 `json:"nostalgia"`      // Does it remember the past?
}

// Habit tracks how the creature perceives player patterns.
type Habit struct {
	Name      string    `json:"name"`
	Frequency int       `json:"frequency"`
	LastTime  time.Time `json:"last_time"`
	Pattern   string    `json:"pattern"` // "morning", "evening", "random"
}

// Memory stores significant moments and preferences.
type Memory struct {
	FavoriteActivities []string `json:"favorite_activities"`
	AvoidedActivities  []string `json:"avoided_activities"`
	PlayerHabits       []*Habit `json:"player_habits"`
	SpecialMoments     []string `json:"special_moments"`
	InsideJokes        []string `json:"inside_jokes"`
}

// TamagotchiState tracks the virtual creature's condition.
type TamagotchiState struct {
	// Core needs (0-100 scale)
	Hunger      float64 `json:"hunger"`      // 0-100: how full the creature is
	Happiness   float64 `json:"happiness"`   // 0-100: overall joy
	Energy      float64 `json:"energy"`      // 0-100: energy level
	Health      float64 `json:"health"`      // 0-100: physical well-being
	Cleanliness float64 `json:"cleanliness"` // 0-100: cleanliness
	Affection   float64 `json:"affection"`   // 0-100: bond with owner

	// Age and growth
	Age         time.Duration `json:"age"`
	LastFed     time.Time     `json:"last_fed"`
	LastPlayed  time.Time     `json:"last_played"`
	LastSleep   time.Time     `json:"last_sleep"`
	LastCleaned time.Time     `json:"last_cleaned"`
	LastPetted  time.Time     `json:"last_petted"`

	// Personality traits (0.0 to 1.0)
	Curiosity    float64 `json:"curiosity"`
	Friendliness float64 `json:"friendliness"`
	EnergyLevel  float64 `json:"energy_level"`

	// Full personality system
	Personality Personality `json:"personality"`
	// Base is the temperament the creature was BORN with. Every stage is a
	// blend from Base towards that stage's archetype, never a blend from the
	// previous stage: a creature that starts out steadier than its archetype
	// must not be pulled down by growing up, which is exactly what blending
	// stage-to-stage did.
	Base   Personality `json:"base_personality"`
	Habits []*Habit    `json:"habits"`
	Memory Memory      `json:"memory"`

	// Current state
	Mood       Mood   `json:"mood"`
	Animation  string `json:"animation"` // "idle", "eat", "play", "sleep", "sick"
	Expression string `json:"expression"`

	// Sleeping is a MODE, not a frame of animation: the creature sleeps until
	// it is woken, so the mood pass below must not overwrite it -- which it
	// used to, one frame after the bed was made.
	Sleeping bool `json:"sleeping"`

	// AnimLeft is how long a one-shot animation holds the creature before the
	// mood takes the face back. Without it the same mood pass overwrote "eat"
	// and "play" on the very next frame, which is why nothing but idle was
	// ever seen; the old code fought that with a goroutine sleeping five
	// minutes, which both leaked and raced the render loop.
	AnimLeft float64 `json:"anim_left"`

	// Speech is what the creature is saying RIGHT NOW, drawn beside it. It is
	// deliberately NOT the scene's bubble: the bubble is the AGENT's channel --
	// needs_input, error, done, and nothing else, which is the whole encoding
	// rule -- and a pet that rings the ask sound because it is peckish would
	// spend the one rare cue the project has on itself. Transient, so it is
	// never written to the save file.
	Speech string `json:"-"`

	// SpeechSecs is how long that sentence stays up. A creature that just grew
	// up gets to finish its sentence; a creature asking for dinner does not
	// hold the line forever. Transient, like the speech itself.
	SpeechSecs float64 `json:"-"`

	// Interaction history
	FedCount     int `json:"fed_count"`
	PlayedCount  int `json:"played_count"`
	SleptCount   int `json:"slept_count"`
	PettedCount  int `json:"petted_count"`
	CleanedCount int `json:"cleaned_count"`

	// Time tracking for absence detection
	LastInteraction time.Time     `json:"last_interaction"`
	TotalIdleTime   time.Duration `json:"total_idle_time"`

	// Spontaneous behavior counters
	NextSpontaneousAt float64 `json:"next_spontaneous_at"` // seconds until next spontaneous action

	// Special states
	IsEvolving     bool `json:"is_evolving"`
	EvolutionPhase int  `json:"evolution_phase"`

	// ---- growth (tamagotchi_evolution.go) ------------------------------
	//
	// Stage is the life the creature is living now. Everything under it is the
	// evidence for the next one, and it is evidence from THIS stage only: a
	// stage starts with a clean clock, so an adult cannot be reached on the
	// strength of a childhood well spent.
	Stage       Stage     `json:"stage"`
	StageBornAt time.Time `json:"stage_born_at"`
	EvolvedAt   time.Time `json:"evolved_at"`

	StageSecs   float64 `json:"stage_secs"`   // seconds lived in this stage, in front of the player
	CareSecs    float64 `json:"care_secs"`    // of which this many with every need up
	CareSum     float64 `json:"care_sum"`     // for the mean care of the stage
	CareSpan    float64 `json:"care_span"`    //
	CaredFor    int     `json:"cared_for"`    // moments of care credited in this stage
	Setbacks    int     `json:"setbacks"`     // stretches of real neglect, each a delay
	NeglectSecs float64 `json:"neglect_secs"` //

	// neglectOpen is transient: it stops one long unhappy stretch being counted
	// as a setback on every single frame of it.
	neglectOpen bool `json:"-"`
}

// tamSeeds makes two creatures born in the same instant different animals.
var tamSeeds int64

// NewTamagotchiState creates a new creature with random personality traits.
func NewTamagotchiState() TamagotchiState {
	// The seed is mixed with a counter as well as the clock. Windows' timer
	// granularity is about fifteen milliseconds, so two creatures created
	// back to back -- `xscapes tamagotchi status` then `feed`, or two scapes
	// launched together -- used to be born with the same personality, to the
	// last decimal. Found by a test that expected two adults to differ.
	r := rand.New(rand.NewSource(time.Now().UnixNano() + atomic.AddInt64(&tamSeeds, 1)*0x9e3779b9))

	// Generate unique personality
	p := Personality{
		Reactivity:    r.Float64(),
		MoodStability: 0.3 + r.Float64()*0.7, // 0.3-1.0 range
		Sociality:     r.Float64(),
		Curiosity:     r.Float64(),
		Attachment:    r.Float64(),
		Whimsy:        r.Float64(),
		Nostalgia:     r.Float64(),
	}

	return TamagotchiState{
		Hunger:       75.0,
		Happiness:    70.0,
		Energy:       85.0,
		Health:       90.0,
		Cleanliness:  80.0,
		Affection:    50.0,
		Age:          0,
		Curiosity:    r.Float64(),
		Friendliness: r.Float64(),
		EnergyLevel:  r.Float64(),
		Personality:  p,
		Base:         p,
		Memory: Memory{
			FavoriteActivities: make([]string, 0),
			AvoidedActivities:  make([]string, 0),
			SpecialMoments:     make([]string, 0),
			InsideJokes:        make([]string, 0),
		},
		LastInteraction:   time.Now(),
		NextSpontaneousAt: 30.0 + r.Float64()*60.0, // 30-90 seconds
		// Everything is born a baby. Stage is the one field in this struct that
		// is not a number to be tuned but a life to be lived.
		Stage:       StageBaby,
		StageBornAt: time.Now(),
	}
}

// UpdateMood calculates the creature's mood based on current needs and personality.
// creditedCare is one moment of care: the growth gates count these, not the
// passing of time, so a creature grows because of what was DONE with it.
func (t *TamagotchiState) creditedCare() {
	t.CaredFor++
}

func (t *TamagotchiState) UpdateMood() {
	// Calculate overall well-being as average of all stats
	avgNeeds := (t.Hunger + t.Happiness + t.Energy + t.Health + t.Cleanliness + t.Affection) / 6.0

	// Priority-based mood determination with personality modifiers
	if t.Health < 30 || t.Hunger < 20 {
		t.Mood = Ill
	} else if t.Hunger < 40 {
		t.Mood = Hungry
	} else if t.Energy < 25 {
		t.Mood = Tired
	} else if t.Cleanliness < 30 {
		t.Mood = Sick
	} else if t.Affection < 20 {
		t.Mood = Lonely
	} else if t.Happiness > 90 && t.Personality.Whimsy > 0.7 {
		t.Mood = Joyful
	} else if avgNeeds > 80 && t.Happiness > 75 {
		t.Mood = Excited
	} else if avgNeeds > 60 && t.Happiness > 50 {
		t.Mood = Happy
	} else if avgNeeds < 30 {
		t.Mood = Thinking
	} else if t.TotalIdleTime > 2*time.Hour && t.Personality.Sociality < 0.3 {
		t.Mood = Melancholy
	} else if t.TotalIdleTime > 4*time.Hour {
		t.Mood = Bored
	} else {
		t.Mood = Calm
	}

	// Update animation based on mood
	t.updateAnimation()
}

// updateAnimation sets the appropriate animation for the current mood.
//
// Two things outrank the mood, and both of them are what the creature is DOING
// rather than how it is: the sleeping mode, and a held one-shot (eat, play, the
// pet). Without those exceptions this function ran on every frame -- Decay
// calls UpdateMood, which calls this -- and put the face straight back to idle,
// so no animation could be seen at all.
func (t *TamagotchiState) updateAnimation() {
	if t.Sleeping {
		t.Animation = ClipSleep
		t.Expression = "zzz"
		return
	}
	if t.AnimLeft > 0 {
		return
	}
	switch t.Mood {
	case Happy, Excited, Joyful:
		t.Animation = ClipIdle
		t.Expression = "smile"
	case Hungry:
		t.Animation = ClipSad
		t.Expression = "worried"
	case Tired:
		t.Animation = ClipSleep
		t.Expression = "closed_eyes"
	case Sick, Ill:
		t.Animation = ClipSick
		t.Expression = "unwell"
	case Lonely, Melancholy:
		t.Animation = ClipSad
		t.Expression = "sad"
	case Playful:
		t.Animation = ClipPlay
		t.Expression = "excited"
	case Bored:
		t.Animation = ClipIdle
		t.Expression = "bored"
	case Contemplative, Thinking:
		t.Animation = ClipIdle
		t.Expression = "thinking"
	case Furious:
		t.Animation = ClipSad
		t.Expression = "furious"
	default:
		t.Animation = ClipIdle
		t.Expression = "neutral"
	}
}

// PlayAnim holds a one-shot animation for hold seconds. The mood takes the face
// back when the hold runs out (see Tick), so an animation cannot get stuck on.
func (t *TamagotchiState) PlayAnim(clip string, hold float64) {
	if clip == "" {
		return
	}
	t.Animation = clip
	if hold > t.AnimLeft {
		t.AnimLeft = hold
	}
}

// Tick advances the held-animation clock by dt seconds.
func (t *TamagotchiState) Tick(dt float64) {
	if dt <= 0 {
		return
	}
	// Growth rides the same clock as everything else, and it is advanced even
	// while an animation is holding the face: a creature eating its dinner is
	// still a creature growing up.
	t.Grow(dt)
	t.ClampStats()
	// A sentence runs down on the same clock as everything else.
	if t.SpeechSecs > 0 {
		t.SpeechSecs -= dt
		if t.SpeechSecs <= 0 {
			t.SpeechSecs = 0
			t.Speech = ""
		}
	}
	if t.AnimLeft <= 0 {
		return
	}
	t.AnimLeft -= dt
	if t.AnimLeft <= 0 {
		t.AnimLeft = 0
		t.UpdateMood()
	}
}

// InteractionResult tracks the outcome of an interaction.
type InteractionResult struct {
	Success    bool
	Message    string
	Animation  string
	Expression string
	Speech     string
	Contextual string // Additional contextual comment
}

// Feed increases hunger satisfaction and slightly boosts happiness.
func (t *TamagotchiState) Feed() InteractionResult {
	// Check if creature is too full
	if t.Hunger >= 95 {
		return InteractionResult{
			Success:    false,
			Message:    "I'm already full! Not hungry right now.",
			Animation:  "idle",
			Expression: "full",
			Speech:     "No thanks, I'm stuffed!",
			Contextual: t.generateContextualComment("full"),
		}
	}

	// Check if creature is sleeping
	if t.Sleeping {
		return InteractionResult{
			Success:    false,
			Message:    "Shh... I'm sleeping!",
			Animation:  "sleep",
			Expression: "zzz",
			Speech:     "*yawn* Not now...",
			Contextual: t.generateContextualComment("sleeping"),
		}
	}

	t.Hunger = math.Min(100.0, t.Hunger+25.0)
	t.Happiness = math.Min(100.0, t.Happiness+5.0)
	t.LastFed = time.Now()
	t.FedCount++
	t.recordHabit("fed")
	t.creditedCare()
	t.Mood = Happy
	t.Expression = "yum"
	// The eating animation, held for as long as it takes to read: the clip is
	// 1.6 s, and the state gives it 2.4 s so the last bite is not cut off. The
	// old code set the same two fields and then spawned a goroutine that
	// slept five minutes to undo them, which raced the render loop.
	t.PlayAnim(ClipEat, 2.4)

	// Track favorite activity if repeated
	if t.FedCount%5 == 0 {
		t.addFavoriteActivity("feeding")
	}

	return InteractionResult{
		Success:    true,
		Message:    "Yum! That was delicious!",
		Animation:  ClipEat,
		Expression: "yum",
		Speech:     t.generateEatResponse(),
		Contextual: t.generateContextualComment("fed"),
	}
}

// generateEatResponse creates varied responses based on personality.
func (t *TamagotchiState) generateEatResponse() string {
	responses := []string{
		"*chomp chomp* Mmm!",
		"*nom nom nom* Delicious!",
		"*gulp* That hit the spot!",
		"*happy munching* Yummy!",
		"*licks lips* More?",
	}
	if t.Personality.Whimsy > 0.7 {
		responses = append(responses, "*dramatic eating noises* *finishes in one bite*")
	}
	return responses[rand.Intn(len(responses))]
}

// generateContextualComment creates contextual comments based on state and time.
func (t *TamagotchiState) generateContextualComment(action string) string {
	hour := time.Now().Hour()

	// Time-based comments
	if hour >= 6 && hour < 12 {
		return "Morning vibes~"
	} else if hour >= 12 && hour < 18 {
		return "Afternoon energy!"
	} else if hour >= 18 && hour < 22 {
		return "Evening chill..."
	} else {
		return "*stargazes* Night time..."
	}
}

// Play increases happiness and decreases energy.
func (t *TamagotchiState) Play() InteractionResult {
	// Check if creature is too tired
	if t.Energy < 20 {
		return InteractionResult{
			Success:    false,
			Message:    "I'm too tired to play...",
			Animation:  "sleepy",
			Expression: "tired",
			Speech:     "*yawn* Maybe later...",
			Contextual: t.generateContextualComment("tired"),
		}
	}

	// Check if creature is sleeping
	if t.Sleeping {
		return InteractionResult{
			Success:    false,
			Message:    "Zzz... Let me sleep first.",
			Animation:  "sleep",
			Expression: "zzz",
			Speech:     "*mumbles in sleep*",
			Contextual: t.generateContextualComment("sleeping"),
		}
	}

	// Check if creature is sick
	if t.Mood == Sick || t.Mood == Ill {
		return InteractionResult{
			Success:    false,
			Message:    "I don't feel well enough to play...",
			Animation:  "sick",
			Expression: "unwell",
			Speech:     "*cough* Not now...",
			Contextual: t.generateContextualComment("sick"),
		}
	}

	t.Happiness = math.Min(100.0, t.Happiness+15.0)
	t.Energy = math.Max(0.0, t.Energy-10.0)
	t.Affection = math.Min(100.0, t.Affection+8.0)
	t.LastPlayed = time.Now()
	t.PlayedCount++
	t.recordHabit("played")
	t.creditedCare()
	t.Mood = Excited

	// Random expression based on personality
	if t.Friendliness > 0.7 {
		t.Expression = "laugh"
	} else if t.Curiosity > 0.7 {
		t.Expression = "curious"
	} else {
		t.Expression = "happy"
	}

	t.Animation = ClipPlay
	t.PlayAnim(ClipPlay, 2.8)

	// Track favorite activity
	if t.PlayedCount%5 == 0 {
		t.addFavoriteActivity("playing")
	}

	return InteractionResult{
		Success:    true,
		Message:    "Wheee! That was fun!",
		Animation:  ClipPlay,
		Expression: "laugh",
		Speech:     t.generatePlayResponse(),
		Contextual: t.generateContextualComment("played"),
	}
}

// generatePlayResponse creates varied responses based on personality.
func (t *TamagotchiState) generatePlayResponse() string {
	responses := []string{
		"*giggles* Again! Again!",
		"*zooms around* So much fun!",
		"*happy bouncing* Let's do that again!",
		"*chases tail* Hehe!",
		"*pounces* Gotcha!",
	}
	if t.Personality.Whimsy > 0.7 {
		responses = append(responses, "*dramatic pose* *strike* *pose*")
	}
	return responses[rand.Intn(len(responses))]
}

// Sleep restores energy and slightly improves health.
func (t *TamagotchiState) Sleep() InteractionResult {
	// Check if already well-rested
	if t.Energy >= 90 {
		return InteractionResult{
			Success:    false,
			Message:    "I'm not tired! I want to keep playing!",
			Animation:  "idle",
			Expression: "happy",
			Speech:     "No thanks, I have energy!",
			Contextual: t.generateContextualComment("energetic"),
		}
	}

	t.Energy = math.Min(100.0, t.Energy+35.0)
	t.Health = math.Min(100.0, t.Health+5.0)
	t.LastSleep = time.Now()
	t.SleptCount++
	t.recordHabit("slept")
	t.creditedCare()
	t.Mood = Calm
	t.Sleeping = true
	t.Animation = ClipSleep
	t.Expression = "zzz"

	return InteractionResult{
		Success:    true,
		Message:    "Goodnight... *yawn*",
		Animation:  ClipSleep,
		Expression: "zzz",
		Speech:     t.generateSleepResponse(),
		Contextual: t.generateContextualComment("sleeping"),
	}
}

// generateSleepResponse creates varied sleep responses.
func (t *TamagotchiState) generateSleepResponse() string {
	responses := []string{
		"*snore* Zzz...",
		"*yawns widely* Time to rest...",
		"*curls up* Sweet dreams...",
		"*sighs contentedly* Goodnight...",
	}
	if t.Personality.Nostalgia > 0.6 {
		responses = append(responses, "*mumbles about yesterday*... Zzz...")
	}
	return responses[rand.Intn(len(responses))]
}

// WakeUp ends sleep mode.
func (t *TamagotchiState) WakeUp() InteractionResult {
	t.Sleeping = false
	t.AnimLeft = 0
	t.Animation = ClipIdle
	t.Expression = "wake"
	if t.Energy > 80 {
		t.Mood = Happy
	} else {
		t.Mood = Calm
	}

	return InteractionResult{
		Success:    true,
		Message:    "*stretches* I'm awake!",
		Animation:  ClipIdle,
		Expression: "wake",
		Speech:     t.generateWakeResponse(),
		Contextual: t.generateContextualComment("woken"),
	}
}

// generateWakeResponse creates varied wake responses.
func (t *TamagotchiState) generateWakeResponse() string {
	hour := time.Now().Hour()
	if hour < 12 {
		return "*yawn* Good morning!"
	} else if hour < 18 {
		return "*stretches* Afternoon already?"
	}
	return "*rubbing eyes* Hey there..."
}

// Clean restores cleanliness and slightly improves health.
func (t *TamagotchiState) Clean() InteractionResult {
	// Check if creature is already clean
	if t.Cleanliness >= 90 {
		return InteractionResult{
			Success:    false,
			Message:    "I'm already clean!",
			Animation:  "idle",
			Expression: "happy",
			Speech:     "I don't need a bath!",
			Contextual: t.generateContextualComment("clean"),
		}
	}

	// Check if creature is sleeping
	if t.Sleeping {
		return InteractionResult{
			Success:    false,
			Message:    "Zzz... Can't clean now.",
			Animation:  "sleep",
			Expression: "zzz",
			Speech:     "*mumbles* Not now...",
			Contextual: t.generateContextualComment("sleeping"),
		}
	}

	t.Cleanliness = math.Min(100.0, t.Cleanliness+30.0)
	t.Health = math.Min(100.0, t.Health+3.0)
	t.LastCleaned = time.Now()
	t.CleanedCount++
	t.recordHabit("cleaned")
	t.creditedCare()
	t.Mood = Happy
	t.Expression = "clean"
	// A bath used to be the one interaction with no reaction at all: it set the
	// numbers and left the creature standing there. It hops and sparkles now.
	t.PlayAnim(ClipJoy, 2.5)

	return InteractionResult{
		Success:    true,
		Message:    "Sparkling clean!",
		Animation:  ClipJoy,
		Expression: "clean",
		Speech:     "*splish splash* Ah, fresh!",
		Contextual: t.generateContextualComment("cleaned"),
	}
}

// Pet increases affection and happiness.
func (t *TamagotchiState) Pet() InteractionResult {
	// Check if creature is sleeping
	if t.Sleeping {
		return InteractionResult{
			Success:    false,
			Message:    "*stirs but stays asleep*",
			Animation:  "sleep",
			Expression: "zzz",
			Speech:     "*soft snores*",
			Contextual: t.generateContextualComment("sleeping"),
		}
	}

	// Check if creature is sick
	if t.Mood == Sick || t.Mood == Ill {
		return InteractionResult{
			Success:    false,
			Message:    "*whimpers* I'm not feeling well...",
			Animation:  "sick",
			Expression: "unwell",
			Speech:     "*cough*",
			Contextual: t.generateContextualComment("sick"),
		}
	}

	t.Affection = math.Min(100.0, t.Affection+12.0)
	t.Happiness = math.Min(100.0, t.Happiness+8.0)
	t.LastPetted = time.Now()
	t.PettedCount++
	t.recordHabit("petted")
	t.creditedCare()
	t.Mood = Happy
	t.Expression = "happy"
	// The affection reaction: it leans into the hand, blushes and hearts rise.
	t.PlayAnim(ClipPet, 2.6)

	// Track favorite activity
	if t.PettedCount%10 == 0 {
		t.addFavoriteActivity("being petted")
	}

	return InteractionResult{
		Success:    true,
		Message:    "*purrs* That feels nice!",
		Animation:  ClipPet,
		Expression: "happy",
		Speech:     t.generatePetResponse(),
		Contextual: t.generateContextualComment("petted"),
	}
}

// generatePetResponse creates varied petting responses.
func (t *TamagotchiState) generatePetResponse() string {
	responses := []string{
		"*happy noises*",
		"*leans into touch* Mmm...",
		"*purrs loudly* More please!",
		"*nuzzles hand* Thank you...",
	}
	if t.Personality.Attachment > 0.7 {
		responses = append(responses, "*follows your hand* Don't stop...")
	}
	return responses[rand.Intn(len(responses))]
}

// Talk sends a message to the creature and gets a response.
func (t *TamagotchiState) Talk(message string) InteractionResult {
	response := t.respondToMessage(message)

	return InteractionResult{
		Success:    true,
		Message:    response,
		Animation:  "idle",
		Expression: "neutral",
		Speech:     response,
	}
}

// respondToMessage generates a contextual response to player input.
func (t *TamagotchiState) respondToMessage(message string) string {
	msg := strings.ToLower(strings.TrimSpace(message))

	// Update last interaction time
	t.LastInteraction = time.Now()
	t.TotalIdleTime = 0

	// Greetings with personality
	if strings.Contains(msg, "hello") || strings.Contains(msg, "hi") || strings.Contains(msg, "hey") {
		return t.generateGreetingResponse()
	}

	// Commands
	if strings.Contains(msg, "feed") || strings.Contains(msg, "eat") {
		result := t.Feed()
		if result.Success {
			return result.Speech
		}
		return result.Speech
	}

	if strings.Contains(msg, "play") || strings.Contains(msg, "game") {
		result := t.Play()
		if result.Success {
			return result.Speech
		}
		return result.Speech
	}

	if strings.Contains(msg, "sleep") || strings.Contains(msg, "nap") {
		result := t.Sleep()
		if result.Success {
			return result.Speech
		}
		return result.Speech
	}

	if strings.Contains(msg, "clean") || strings.Contains(msg, "bath") {
		result := t.Clean()
		if result.Success {
			return result.Speech
		}
		return result.Speech
	}

	if strings.Contains(msg, "pet") || strings.Contains(msg, "stroke") {
		result := t.Pet()
		if result.Success {
			return result.Speech
		}
		return result.Speech
	}

	// Questions about state with personality
	if strings.Contains(msg, "how are you") || strings.Contains(msg, "feel") {
		return t.generateStateResponse()
	}

	if strings.Contains(msg, "name") {
		return "My name is Xscape! What should I call you?"
	}

	if strings.Contains(msg, "love") || strings.Contains(msg, "like you") {
		t.Affection = math.Min(100.0, t.Affection+5.0)
		return t.generateLoveResponse()
	}

	// Check for inside jokes
	for _, joke := range t.Memory.InsideJokes {
		if strings.Contains(msg, joke) {
			return fmt.Sprintf("*remembers* %s", joke)
		}
	}

	// Default responses with personality variation
	return t.generateDefaultResponse()
}

// generateGreetingResponse creates varied greetings based on mood and personality.
func (t *TamagotchiState) generateGreetingResponse() string {
	hour := time.Now().Hour()
	greetings := []string{
		"Hi there! *waves happily*",
		"Hello! *perks up*",
		"Hey! *tail wags*",
	}

	switch t.Mood {
	case Happy, Excited, Joyful:
		return greetings[0]
	case Calm:
		return "Hello!"
	case Hungry:
		return "*looks up* Hi... do you have food?"
	case Tired:
		return "*yawns* Hey... not now..."
	case Sick, Ill:
		return "*coughs* Hi... I'm not feeling well."
	case Lonely, Melancholy:
		return "*ears droop* Oh... hello..."
	case Bored:
		return "*tilts head* Hey. Anything interesting happening?"
	}

	if hour < 12 {
		return "Good morning! *bright eyes*"
	} else if hour < 18 {
		return "Afternoon! *stretches*"
	}
	return "Evening! *yawns*"
}

// generateStateResponse creates varied state responses.
func (t *TamagotchiState) generateStateResponse() string {
	switch t.Mood {
	case Happy, Excited, Joyful:
		return "I'm feeling great! *bounces* What about you?"
	case Calm:
		return "I'm doing okay. Peaceful today."
	case Hungry:
		return "I'm a bit hungry... Could use a snack. Or three."
	case Tired:
		return "*yawns* I'm so sleepy... But I'll push through!"
	case Sick, Ill:
		return "I don't feel very well... Need some care..."
	case Lonely, Melancholy:
		return "I miss you when you're gone... *looks down*"
	case Bored:
		return "Nothing much to do... Want to play something?"
	case Contemplative, Thinking:
		return "*ponders* I've been thinking about the nature of existence..."
	}
	return "I'm fine! Just living my best life~"
}

// generateLoveResponse creates varied love responses.
func (t *TamagotchiState) generateLoveResponse() string {
	if t.Personality.Attachment > 0.8 {
		return "*blushes deeply* I love you too! You're my favorite human..."
	} else if t.Personality.Attachment > 0.5 {
		return "*blushes* I love you too! *purrs*"
	}
	return "*tilts head* Love? Is that what we're feeling? ...I think I like it."
}

// generateDefaultResponse creates varied default responses.
func (t *TamagotchiState) generateDefaultResponse() string {
	if t.Personality.Curiosity > 0.7 {
		return "*tilts head curiously* Tell me more about that!"
	} else if t.Personality.Sociality > 0.7 {
		return "*listens attentively* Oh? What else?"
	}

	responses := []string{
		"*tilts head* Hmm?",
		"I don't understand...",
		"*listens attentively*",
		"Tell me more!",
		"*wags tail hopefully*",
	}
	if t.Personality.Whimsy > 0.7 {
		responses = append(responses, "*suddenly interested in a dust mote*")
	}
	return responses[rand.Intn(len(responses))]
}

// Decay simulates needs decreasing over time with smooth transitions.
func (t *TamagotchiState) Decay(hourlyRate float64) {
	// A negative rate is not a decay, it is a clock that went backwards -- a
	// laptop waking with its clock reset, or a caller that passed a negative
	// dt. Subtracting a negative number FED the creature: a single -1e9 step
	// left it at 32,000,075% hunger. Measured, not feared; see the audit in
	// tamagotchi_audit_test.go. The application clamps dt at the call site, and
	// this refuses to be the second place that has to.
	if !(hourlyRate > 0) {
		return
	}
	// Base decay rates (per hour), modified by personality
	baseDecay := hourlyRate * 0.8 // 0.8x to make it slower

	// Hunger decays fastest, then energy, then others
	t.Hunger = math.Max(0.0, t.Hunger-baseDecay*0.15)
	t.Energy = math.Max(0.0, t.Energy-baseDecay*0.08)
	t.Happiness = math.Max(0.0, t.Happiness-baseDecay*0.05)
	t.Health = math.Max(0.0, t.Health-baseDecay*0.03)
	t.Cleanliness = math.Max(0.0, t.Cleanliness-baseDecay*0.04)
	t.Affection = math.Max(0.0, t.Affection-baseDecay*0.02)

	// Cross-effects: low stats affect others
	if t.Hunger < 30 {
		t.Happiness = math.Max(0.0, t.Happiness-0.5)
		t.Health = math.Max(0.0, t.Health-0.3)
	}
	if t.Energy < 20 {
		t.Happiness = math.Max(0.0, t.Happiness-0.3)
	}
	if t.Cleanliness < 30 {
		t.Health = math.Max(0.0, t.Health-0.5)
	}

	t.ClampStats()
	t.UpdateMood()
}

// ClampStats holds every need inside 0..100. One door for the whole file: the
// cross-effects, the interactions that add, and a save file somebody edited all
// arrive here, and a need outside its range stops the mood, the care average and
// the growth gates from meaning anything.
func (t *TamagotchiState) ClampStats() {
	t.Hunger = clamp100(t.Hunger)
	t.Happiness = clamp100(t.Happiness)
	t.Energy = clamp100(t.Energy)
	t.Health = clamp100(t.Health)
	t.Cleanliness = clamp100(t.Cleanliness)
	t.Affection = clamp100(t.Affection)
}

// clamp100 keeps a need in range. A NaN is a zero: a need nobody can read is
// not a need, and letting one in poisons the mean, the mood and the gates.
func clamp100(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// AgeProgresses ages the creature and potentially triggers evolution.
func (t *TamagotchiState) AgeProgresses(hours float64) {
	t.Age += time.Duration(hours * float64(time.Hour))

	// Growing up used to happen here, at 24 and 72 hours of AGE -- and Age runs
	// at sixteen times real time (TamDecayPerSecond), so the first evolution
	// fired ninety seconds in, mid-session, and the second before anyone had
	// looked twice. It is judged on care now, in Grow, once per frame.
}

// GetSpeech returns a speech bubble based on current state.
func (t *TamagotchiState) GetSpeech() string {
	switch t.Mood {
	case Hungry:
		return "I'm hungry..."
	case Tired:
		return "So sleepy..."
	case Sick:
		return "I don't feel well..."
	case Happy:
		if rand.Float64() > 0.7 {
			return "Let's play!"
		}
		return "I'm happy!"
	case Excited:
		return "Yay! Fun time!"
	case Playful:
		return "Wanna play?"
	case Thinking:
		if rand.Float64() > 0.5 {
			return "*yawn*"
		}
		return "..."
	default:
		return ""
	}
}

// DrawASCII renders the creature as ASCII art based on current state.
func (t *TamagotchiState) DrawASCII(width, height int) [][]rune {
	rows := height
	cols := width

	// Base pattern - a simple creature shape
	pattern := []string{
		"  .-.  ",
		" (o o) ",
		" (   ) ",
		" '---' ",
	}

	result := make([][]rune, rows)
	for i := range result {
		result[i] = make([]rune, cols)
		for j := range result[i] {
			result[i][j] = ' '
		}
	}

	// Center the creature
	startX := (cols - 7) / 2
	startY := (rows - 4) / 2

	// Draw based on animation state
	switch t.Animation {
	case "eat":
		pattern = []string{
			"  .-.  ",
			" (o o) ",
			" ( ^ ) ",
			" 'o-o' ",
		}
	case "sleep":
		pattern = []string{
			"  .-.  ",
			" (- -) ",
			" (   ) ",
			" '---' ",
		}
	case "sick":
		pattern = []string{
			"  .-.  ",
			" (x x) ",
			" (   ) ",
			" '---' ",
		}
	case "play":
		pattern = []string{
			"  .-.  ",
			" ( o ) ",
			" ( ^ ) ",
			" ' - ' ",
		}
	}

	// Place pattern in result
	for i, line := range pattern {
		if startY+i >= rows {
			break
		}
		for j, ch := range line {
			if startX+j < cols && startX+j >= 0 {
				result[startY+i][startX+j] = ch
			}
		}
	}

	return result
}

// GetExpression returns the current facial expression as a string.
func (t *TamagotchiState) GetExpression() string {
	switch t.Expression {
	case "smile":
		return "^ ^"
	case "sad":
		return "v v"
	case "worried":
		return "? ?"
	case "closed_eyes":
		return "- -"
	case "unwell":
		return "x x"
	case "happy":
		return "o o"
	case "excited":
		return "* *"
	case "curious":
		return "O O"
	case "yum":
		return "3 3"
	case "laugh":
		return "D D"
	case "wake":
		return "O o"
	case "clean":
		return "~ ~"
	case "zzz":
		return "Z Z"
	default:
		return "o o"
	}
}

// GetStatusReport returns a text status report of the creature.
func (t *TamagotchiState) GetStatusReport() string {
	next, why := t.NextEvolution()
	growth := fmt.Sprintf("%s, %d%% to %s (%s)",
		t.Stage, int(t.StageProgress()*100+0.5), next, why)
	if t.Stage >= StageAdult {
		growth = "adult, fully grown"
	}
	return fmt.Sprintf(
		"%s | Mood: %s | Hunger:%d%% Happiness:%d%% Energy:%d%% Health:%d%% Clean:%d%% Aff:%d%%",
		growth, t.Mood.String(),
		int(t.Hunger), int(t.Happiness), int(t.Energy),
		int(t.Health), int(t.Cleanliness), int(t.Affection),
	)
}

// GetPersonalityReport returns a text report of the creature's personality.
func (t *TamagotchiState) GetPersonalityReport() string {
	p := t.Personality
	return fmt.Sprintf(
		"Personality:\n  Reactivity: %.0f%%\n  Mood Stability: %.0f%%\n  Sociality: %.0f%%\n  Curiosity: %.0f%%\n  Attachment: %.0f%%\n  Whimsy: %.0f%%\n  Nostalgia: %.0f%%",
		p.Reactivity*100, p.MoodStability*100, p.Sociality*100,
		p.Curiosity*100, p.Attachment*100, p.Whimsy*100, p.Nostalgia*100,
	)
}

// GetHabitReport returns a text report of the creature's observed habits.
func (t *TamagotchiState) GetHabitReport() string {
	if len(t.Habits) == 0 {
		return "No habits observed yet."
	}

	report := "Observed Habits:\n"
	for _, habit := range t.Habits {
		report += fmt.Sprintf("  %s: %d times\n", habit.Name, habit.Frequency)
	}

	if len(t.Memory.FavoriteActivities) > 0 {
		report += "\nFavorite Activities:\n"
		for _, fav := range t.Memory.FavoriteActivities {
			report += fmt.Sprintf("  - %s\n", fav)
		}
	}

	return report
}

// GetMemoryReport returns a text report of the creature's memories.
func (t *TamagotchiState) GetMemoryReport() string {
	report := "Memories:\n"

	if len(t.Memory.SpecialMoments) > 0 {
		report += "Special Moments:\n"
		for _, moment := range t.Memory.SpecialMoments {
			report += fmt.Sprintf("  - %s\n", moment)
		}
	}

	if len(t.Memory.InsideJokes) > 0 {
		report += "Inside Jokes:\n"
		for _, joke := range t.Memory.InsideJokes {
			report += fmt.Sprintf("  - %s\n", joke)
		}
	}

	if len(t.Habits) == 0 {
		report += "\nNo habits observed yet. Interact more to build memories!"
	}

	return report
}

// AddMemory records a special moment or inside joke.
func (t *TamagotchiState) AddMemory(moment string, isJoke bool) {
	if isJoke {
		t.Memory.InsideJokes = append(t.Memory.InsideJokes, moment)
	} else {
		t.Memory.SpecialMoments = append(t.Memory.SpecialMoments, moment)
	}
}

// GetIdleTime returns how long the creature has been idle.
func (t *TamagotchiState) GetIdleTime() time.Duration {
	return time.Since(t.LastInteraction)
}

// recordHabit tracks how often the player performs certain actions.
func (t *TamagotchiState) recordHabit(action string) {
	t.LastInteraction = time.Now()

	for _, habit := range t.Habits {
		if habit.Name == action {
			habit.Frequency++
			habit.LastTime = time.Now()
			return
		}
	}

	// Add new habit
	t.Habits = append(t.Habits, &Habit{
		Name:      action,
		Frequency: 1,
		LastTime:  time.Now(),
	})
}

// addFavoriteActivity tracks activities the creature enjoys.
func (t *TamagotchiState) addFavoriteActivity(activity string) {
	for _, fav := range t.Memory.FavoriteActivities {
		if fav == activity {
			return // Already tracked
		}
	}
	t.Memory.FavoriteActivities = append(t.Memory.FavoriteActivities, activity)
}

// generateSpontaneousAction creates unexpected but consistent behavior.
func (t *TamagotchiState) generateSpontaneousAction() string {
	if rand.Float64() > t.Personality.Whimsy {
		return ""
	}

	actions := []string{
		"*suddenly notices a fly* Chase!",
		"*finds a shiny object* Mine!",
		"*remembers something* Oh! I remember that!",
		"*does a little dance* Who needs reasons?",
		"*stares at nothing* ...",
		"*sneezes* Bless me!",
		"*rolls over* Look at me go!",
	}

	if t.Personality.Nostalgia > 0.6 {
		actions = append(actions, "*remembers old times* Those were the days...")
	}

	return actions[rand.Intn(len(actions))]
}

// GenerateIdleAnimation returns a random idle animation based on personality.
func (t *TamagotchiState) GenerateIdleAnimation() string {
	if t.Animation != "idle" && t.Animation != "" {
		return ""
	}

	// Check for absence reactions
	idleTime := t.GetIdleTime()
	if idleTime > 2*time.Hour {
		return "*ears droop* You've been gone so long..."
	} else if idleTime > 1*time.Hour {
		return "*tilts head* Are you still there?"
	} else if idleTime > 30*time.Minute {
		return "*stretches* Oh, you're back!"
	}

	// Random idle behaviors based on personality
	switch {
	case t.Personality.Whimsy > 0.7 && rand.Float64() > 0.5:
		return "*suddenly interested in a dust mote*"
	case t.Personality.Curiosity > 0.7 && rand.Float64() > 0.6:
		return "*sniffs the air curiously*"
	case t.Personality.Sociality > 0.7 && rand.Float64() > 0.5:
		return "*wags tail hopefully* Want to play?"
	case t.Mood == Bored && rand.Float64() > 0.4:
		return "*yawns* This is so boring..."
	case t.Mood == Contemplative && rand.Float64() > 0.5:
		return "*ponders the meaning of existence*"
	case t.Mood == Happy && rand.Float64() > 0.6:
		return "*happy little bounce* Hehe!"
	case t.Mood == Excited && rand.Float64() > 0.5:
		return "*zooms around* So much energy!"
	default:
		return ""
	}
}

// GetStatBar returns a visual bar representation of a stat (0-100).
func GetStatBar(value float64, width int) string {
	filled := int(value / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", width-filled) + "]"
}
