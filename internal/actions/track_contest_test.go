package actions

import (
	"testing"
)

// trackDetectRate runs `trials` independent tracks against a fresh room and a
// fresh actor, reporting the fraction that cleared the 125 detection gate.
//
// A track below 125 returns early with "You don't see any tracks." and sets
// Reason. That Reason string is the cleanest observable for the gate: it does
// not depend on the room holding any trail data.
//
// Fresh actor per trial for the same reason as search: Track is cooldown-gated
// per character, so looping one actor measures the cooldown, not the roll.
func trackDetectRate(t *testing.T, perception int, trials int) float64 {
	t.Helper()
	detected := 0
	for i := 0; i < trials; i++ {
		room := newTrackTestRoom(9400 + i)
		actor := newTrackFakeActor("ContestTracker", room, false, 0)
		actor.char.Stats.Perception.ValueAdj = perception
		if Track(actor, TrackOptions{}).Reason != "roll below detection threshold" {
			detected++
		}
	}
	return float64(detected) / float64(trials)
}

// TestTrackDetectionGateResolvesAsAContest pins spec 5.1 for track's 125 gate:
// the DIFFICULTY is now rolled too, so outcomes compress toward 50%.
//
// Search score is Perception + SkillMultiplier(rank)*25 and SkillMultiplier at
// rank 0 is 1.0, so Perception 75 gives a score of exactly 100 against the 125
// gate: 4.8% under the threshold form, ~11.9% as a contest.
func TestTrackDetectionGateResolvesAsAContest(t *testing.T) {
	const trials = 3000
	rate := trackDetectRate(t, 75, trials)

	if rate < 0.075 || rate > 0.175 {
		t.Fatalf("track score 100 vs the 125 gate detected %.1f%% of the time, "+
			"want ~11.9%% (the contest form). Around 4.8%% means it is still "+
			"dice.RollStat compared to a hardcoded 125.", rate*100)
	}
}
