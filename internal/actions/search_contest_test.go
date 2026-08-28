package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
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
func searchFindRate(t *testing.T, perception int, trials int) float64 {
	t.Helper()
	found := 0
	for i := 0; i < trials; i++ {
		room := newSecretExitRoom(9300 + i)
		actor := newSearchFakeActor("ContestSearcher", room, false, 0)
		actor.char.Stats.Perception.ValueAdj = perception
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
