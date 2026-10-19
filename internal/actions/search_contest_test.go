package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// newSecretExitRoom builds a room whose only feature is one secret exit, so a
// Search resolves exactly one tier-1 contest and nothing else.
func newSecretExitRoom(roomId int) *rooms.Room {
	r := &rooms.Room{RoomId: roomId}
	r.Exits = map[string]exit.RoomExit{
		"north": {Secret: true},
	}
	return r
}

// searchFindRate runs `trials` independent searches against a fresh room and a
// fresh actor each time, and reports the fraction that found the secret exit.
//
// A FRESH ACTOR PER TRIAL is required, not an optimisation: Search is
// cooldown-gated per character (TestSearch_CooldownGate), so looping one actor
// would measure the cooldown rather than the roll.
//
// The room carries a lamp at 60, inside a normal observer's comfort band, so
// the sight ramp (lighting plan 5b) is unity and the score is exactly what
// the rate tests below compute. searchFindRateAt takes the lamp as a knob.
func searchFindRate(t *testing.T, perception int, trials int) float64 {
	t.Helper()
	return searchFindRateAt(t, perception, 60, trials)
}

func searchFindRateAt(t *testing.T, perception, lamp int, trials int) float64 {
	t.Helper()
	found := 0
	for i := 0; i < trials; i++ {
		room := newSecretExitRoom(9300 + i)
		room.Lamp = rooms.LampPtr(lamp)
		actor := newSearchFakeActor("ContestSearcher", room, false, 0)
		actor.char.Stats.Perception.ValueAdj = perception
		if i == 0 && lamp == 60 {
			if m := messaging.SightMult(actor.char, room); m != 1.0 {
				t.Fatalf("fixture premise: a lamp-60 room must be comfortable (SightMult %v, light %d)", m, room.LightLevel())
			}
		}
		if len(Search(actor, SearchOptions{}).HiddenExitsFound) > 0 {
			found++
		}
	}
	return float64(found) / float64(trials)
}

// TestSearchTier1ResolvesAsAContest pins spec 5.1: a search tier is a CONTEST
// against the difficulty, not a roll compared to a hardcoded number.
//
// The observable difference is that contest.Run rolls the DIFFICULTY side too,
// at a stdDev derived from the attacker's score, which widens the distribution
// and compresses outcomes toward 50%. Against the 125 tier-1 target at a search
// score of 100 the spec's table gives:
//
//	threshold form (before): 4.8%
//	contest form   (after):  11.9%
//
// Search score is Perception + SkillMultiplier(rank)*25, and SkillMultiplier at
// rank 0 is SkillMultiplierBase (1.0), so Perception 75 gives a score of exactly
// 100. The fake actor has no search skill, so rank is 0.
//
// The band is deliberately wide. It only has to separate 4.8% from 11.9%, and a
// tight band on a stochastic quantity is a flaky test, which this repo has
// already paid for once this week.
func TestSearchTier1ResolvesAsAContest(t *testing.T) {
	const trials = 3000
	rate := searchFindRate(t, 75, trials) // score 100 against the 125 target

	if rate < 0.075 || rate > 0.175 {
		t.Fatalf("search score 100 vs the 125 tier found the exit %.1f%% of the "+
			"time, want ~11.9%% (the contest form). Around 4.8%% means it is "+
			"still dice.RollStat compared to a hardcoded 125.", rate*100)
	}
}

// TestSearchTier1CompressesAtTheTop is the other half of the same change, and
// it is what makes the test a real check rather than a one-sided one. An expert
// searcher LOSES near-certainty, because the difficulty now gets a roll of its
// own. Spec 5.1: score 175 goes 97.2% -> 91.1%.
//
// Without this, raising every rate would pass the test above; the conversion is
// specifically a COMPRESSION toward 50%, in both directions.
func TestSearchTier1CompressesAtTheTop(t *testing.T) {
	const trials = 3000
	rate := searchFindRate(t, 150, trials) // score 175 against the 125 target

	if rate < 0.85 || rate > 0.955 {
		t.Fatalf("search score 175 vs the 125 tier found the exit %.1f%% of the "+
			"time, want ~91.1%% (the contest form). Above 95.5%% means the "+
			"threshold form's near-certainty survived.", rate*100)
	}
}

// TestSearchTierPaysTheSearchersEyes (lighting plan 5b): a dazzled searcher's
// score pays SightMult 0.88, so score 175 plays as 154 against the 125 tier.
// The bar is sightRampRateGap's computed one (sight_ramp_sites_test.go), so
// the test cannot pass if the ramp never reaches the AgainstDifficulty tiers.
func TestSearchTierPaysTheSearchersEyes(t *testing.T) {
	comfortable := searchFindRateAt(t, 150, 60, sightRampTrials)
	dazzled := searchFindRateAt(t, 150, 90, sightRampTrials)
	sightRampRateGap(t, "searcher", comfortable, dazzled)
}

// TestSearch_AFoundSecretExitIsNotRolledAgain closes the secret-exit search
// farm.
//
// 🔴 THE FARM: tier 1 set rolledAgainstSomething for every secret exit in the
// room and NEVER skipped one already found, because secret exits called
// AddDiscovery nowhere — hidden containers (tier 2) and hidden nouns (tier 6)
// both guard with HasDiscovery and record on a find. So a room with a secret
// exit was a PERMANENT progression candidate at a 2-round cooldown, roughly
// 450 uses an hour, against the ~150/hr that `search`'s own multiplier was
// solved on. The comment stating that assumption is in config.yaml and
// skills.go.
//
// The observable is the AWARD, not the message: progression is gated on
// rolledAgainstSomething, so in a room whose only feature is a found exit the
// second search must resolve against nothing and train nothing.
func TestSearch_AFoundSecretExitIsNotRolledAgain(t *testing.T) {
	// Seeded (#630): contest.AgainstDifficulty has no floor and no ceiling,
	// so perception 400 made the first find very likely but not certain, and
	// the fixture missed once in a full-suite run. One seed pins every dice
	// draw Search makes, so the first find either always lands or always
	// fails loudly below; it can no longer fail at random.
	t.Cleanup(util.SetRandForTest(630))

	room := newSecretExitRoom(9600)
	actor := newSearchFakeActor("FarmSearcher", room, false, 0)
	actor.char.Stats.Perception.ValueAdj = 400

	first := Search(actor, SearchOptions{})
	if len(first.HiddenExitsFound) == 0 {
		t.Fatal("fixture failed to find the exit under seed 630; pick a seed " +
			"whose first search finds it, or the rest of this test asserts nothing")
	}
	if len(actor.awards) != 1 {
		t.Fatalf("first search produced %d awards, want 1", len(actor.awards))
	}

	// Second search, same character, same room.
	actor.awards = nil
	actor.char.Cooldowns = nil // clear the search cooldown; a second search must actually run
	second := Search(actor, SearchOptions{})

	if len(actor.awards) != 0 {
		t.Errorf("a re-search of an already-found secret exit produced %d awards, "+
			"want 0. The exit is still a live progression candidate and the farm "+
			"is open.", len(actor.awards))
	}
	// It is still REPORTED, deliberately: unlike a hidden container, a found
	// secret exit has no other way to be seen until the player walks through it,
	// so search stays a way to be reminded of the name. It just cannot be
	// farmed for progression.
	if len(second.HiddenExitsFound) == 0 {
		t.Error("a re-search should still REPORT the known exit; only the roll " +
			"and the award are suppressed")
	}
}
