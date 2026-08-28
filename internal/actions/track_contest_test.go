package actions

import (
	"testing"
)

// trackDetectRate reports the fraction of `trials` tracks that saw anything.
//
// Fresh actor per trial: Track is cooldown-gated per character, so looping one
// actor would measure the cooldown rather than the roll.
func trackDetectRate(t *testing.T, perception int, trials int) float64 {
	t.Helper()
	detected := 0
	for i := 0; i < trials; i++ {
		room := newTrackTestRoom(9400 + i)
		actor := newTrackFakeActor("ContestTracker", room, false, 0)
		actor.char.Stats.Perception.ValueAdj = perception
		if Track(actor, TrackOptions{}).Reason != "lost the trail-detection contest" {
			detected++
		}
	}
	return float64(detected) / float64(trials)
}

// TestTrailDetectionResolvesAsAContest pins that reading a room's trail is a
// static-difficulty CONTEST, aligned with search and forage (owner ruling
// 2026-08-28). The difficulty is rolled too, so outcomes compress toward 50%.
//
// CalcSearchScore is Perception + SkillMultiplier(rank)*25, and SkillMultiplier
// at rank 0 is 1.0, so Perception 75 gives a score of exactly 100 against the
// 125 detection band: 4.8% as a threshold, ~11.9% as a contest.
func TestTrailDetectionResolvesAsAContest(t *testing.T) {
	const trials = 3000
	rate := trackDetectRate(t, 75, trials)

	if rate < 0.075 || rate > 0.175 {
		t.Fatalf("track score 100 vs the 125 band detected %.1f%% of the time, "+
			"want ~11.9%% (the contest form). Around 4.8%% means it is still a "+
			"threshold compare.", rate*100)
	}
}

// TestTrailDetectionCompressesAtTheTop is the other direction. Without it, any
// change that raised every rate would satisfy the test above, and the
// conversion is specifically a COMPRESSION toward 50% in BOTH directions.
func TestTrailDetectionCompressesAtTheTop(t *testing.T) {
	const trials = 3000
	rate := trackDetectRate(t, 150, trials) // score 175 vs the 125 band

	if rate < 0.85 || rate > 0.955 {
		t.Fatalf("track score 175 vs the 125 band detected %.1f%% of the time, "+
			"want ~91.1%% (the contest form). Above 95.5%% means the threshold "+
			"form's near-certainty survived.", rate*100)
	}
}

// TestTrailDetailBandsAreNested is THE regression guard for the defect that got
// the first attempt at this conversion reverted.
//
// Contesting only the coarse gate and leaving the finer bands reading the raw
// roll DECOUPLED them: at score 100, 73.8% of successful reads carried a roll
// below the 125 every band comparison still treated as "no tracks", and 0.70%
// of high rolls lost the gate outright while reading in the top band.
//
// Resolving the ladder nests it by construction, and this asserts that directly
// over many samples: SeesExits implies SeesAll implies SeesAnything, always.
func TestTrailDetailBandsAreNested(t *testing.T) {
	for _, score := range []float64{60, 100, 125, 150, 175, 260} {
		sawAll, sawExits, sawAny := 0, 0, 0
		for i := 0; i < 4000; i++ {
			d := resolveTrailDetail(score)
			if d.SeesAnything {
				sawAny++
			}
			if d.SeesAll {
				sawAll++
			}
			if d.SeesExits {
				sawExits++
			}
			if d.SeesExits && !d.SeesAll {
				t.Fatalf("score %v: SeesExits without SeesAll — the ladder is not nested", score)
			}
			if d.SeesAll && !d.SeesAnything {
				t.Fatalf("score %v: SeesAll without SeesAnything — the ladder is not nested", score)
			}
		}
		// The counts must also be ordered, which catches a nesting that holds
		// per-sample but was built from independent rolls that happen to agree.
		if !(sawExits <= sawAll && sawAll <= sawAny) {
			t.Errorf("score %v: band counts out of order (any=%d all=%d exits=%d)",
				score, sawAny, sawAll, sawExits)
		}
	}
}

// TestTrailDetailIsReachableAtBothEnds guards against a nested ladder that is
// vacuously nested because a band never fires. A guard that cannot distinguish
// outcomes protects nothing.
func TestTrailDetailIsReachableAtBothEnds(t *testing.T) {
	lowAny := 0
	for i := 0; i < 3000; i++ {
		if resolveTrailDetail(60).SeesAnything {
			lowAny++
		}
	}
	if lowAny > 300 {
		t.Errorf("a score-60 tracker saw something %d times in 3000; the 125 band is not gating", lowAny)
	}

	highExits := 0
	for i := 0; i < 3000; i++ {
		if resolveTrailDetail(300).SeesExits {
			highExits++
		}
	}
	if highExits < 1500 {
		t.Errorf("a score-300 tracker read exits only %d times in 3000; the top band is unreachable", highExits)
	}
}
