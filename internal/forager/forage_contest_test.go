package forager

import "testing"

// forageFindRate reports the fraction of `trials` attempts that found anything.
// ForageCore is pure, so this needs no fixture beyond the attempt itself.
func forageFindRate(score float64, trials int) float64 {
	found := 0
	for i := 0; i < trials; i++ {
		if ForageCore(ForageAttempt{Biome: "land", SearchScore: score}).Found {
			found++
		}
	}
	return float64(found) / float64(trials)
}

// TestForageResolvesAsAContest pins spec 5.1 for forage: the DIFFICULTY is now
// rolled too, which WIDENS the distribution and compresses outcomes toward 50%.
//
// ⚠️ Not "ratio rather than gap" -- dice.RollStat already drew at
// stdDev = RollSpread * score, so the old form was ratio-only as well. The
// change is exactly a sqrt(2) widening.
//
// Biome "land" has ForageDifficulty 125, the same target as search's tier 1, so
// the spec's published table applies directly: a score of 100 goes from 4.8%
// under the threshold form to ~11.9% as a contest.
func TestForageResolvesAsAContest(t *testing.T) {
	const trials = 4000
	rate := forageFindRate(100, trials)

	if rate < 0.075 || rate > 0.175 {
		t.Fatalf("forage score 100 vs the 125 land difficulty found %.1f%% of "+
			"the time, want ~11.9%% (the contest form). Around 4.8%% means it "+
			"is still dice.RollStat compared to a hardcoded difficulty.", rate*100)
	}
}

// TestForageCompressesAtTheTop is the other direction, without which raising
// every rate would satisfy the test above. The conversion is a COMPRESSION
// toward 50%, so an expert forager must LOSE near-certainty: spec 5.1 puts
// score 175 against a 125 target at 97.2% -> 91.1%.
func TestForageCompressesAtTheTop(t *testing.T) {
	const trials = 4000
	rate := forageFindRate(175, trials)

	if rate < 0.85 || rate > 0.955 {
		t.Fatalf("forage score 175 vs the 125 land difficulty found %.1f%% of "+
			"the time, want ~91.1%% (the contest form). Above 95.5%% means the "+
			"threshold form's near-certainty survived.", rate*100)
	}
}

// TestForageUnknownBiomeStillFindsNothing guards the early returns the
// conversion must not disturb: an unknown biome and an empty pool both return
// before any roll happens, so they cannot be affected by the contest at all.
func TestForageUnknownBiomeStillFindsNothing(t *testing.T) {
	if ForageCore(ForageAttempt{Biome: "not-a-biome", SearchScore: 10000}).Found {
		t.Fatal("an unknown biome must find nothing regardless of score")
	}
}
