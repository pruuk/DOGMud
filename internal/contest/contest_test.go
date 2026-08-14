package contest

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRun_MarginIsAttackPositive pins the convention the whole arc depends on.
// dice.OpposedRoll is attack-positive and bestDefenseResult is defence-positive;
// mixing them compiles cleanly and puts crit on the losing side. The core picks
// attack-positive because 33 of 34 call sites already use it.
func TestRun_MarginIsAttackPositive(t *testing.T) {
	// Attacker massively outclasses a single defender.
	res := Run(1000, []Entry{{Name: "dodge", Score: 1}})

	assert.True(t, res.Contested, "one entry means a contest happened")
	assert.Greater(t, res.Margin, 0.0, "attacker won, so margin must be POSITIVE")
	assert.Equal(t, "dodge", res.Winner)
}

// TestRun_MarginMatchesTheRolls guards the invariant that the reported margin is
// derived from the two reported rolls. If they ever drift apart, every
// downstream effect that scales by margin silently uses a number that does not
// correspond to what was rolled.
func TestRun_MarginMatchesTheRolls(t *testing.T) {
	for i := 0; i < 500; i++ {
		res := Run(100, []Entry{{Name: "a", Score: 90}, {Name: "b", Score: 110}})
		want := res.AttackRoll.Value - res.DefenseRoll.Value
		assert.InDelta(t, want, res.Margin, 1e-9, "margin must equal atk - def of the reported rolls")
	}
}

// TestRun_KeepsTheBestDefence verifies best-of-N selection: the entry that
// defended most successfully is the one reported, which is the entry with the
// SMALLEST attack-positive margin.
func TestRun_KeepsTheBestDefence(t *testing.T) {
	// "wall" scores so far above the others that it must win essentially always.
	strongWins := 0
	for i := 0; i < 200; i++ {
		res := Run(100, []Entry{
			{Name: "paper", Score: 1},
			{Name: "wall", Score: 10000},
		})
		if res.Winner == "wall" {
			strongWins++
		}
	}
	assert.Equal(t, 200, strongWins, "the far stronger defence must always win the selection")
}

// TestRun_NoEntriesIsNotAContest covers the trap that produced a real bug:
// bestDefenseResult initialises margin to math.Inf(-1) and only overwrites it
// inside the defence loop, so a defender with no defences left +Inf in place.
// Negated under margin-derived crit that reads as an infinitely decisive attack
// and crits EVERY swing. The core must never emit an infinity.
func TestRun_NoEntriesIsNotAContest(t *testing.T) {
	res := Run(100, nil)

	assert.False(t, res.Contested, "no entries means nothing was contested")
	assert.Equal(t, "", res.Winner)
	assert.Equal(t, 0.0, res.Margin, "margin must be a neutral zero, never an infinity")
	assert.False(t, math.IsInf(res.Margin, 0), "margin must never be infinite")
	assert.NotZero(t, res.AttackRoll.StdDev, "the attack roll still happens and is still reported")
}

// TestRun_EmptySliceBehavesLikeNil is the same guard for the other empty form.
func TestRun_EmptySliceBehavesLikeNil(t *testing.T) {
	res := Run(100, []Entry{})
	assert.False(t, res.Contested)
	assert.Equal(t, 0.0, res.Margin)
}

// TestRun_BothSidesUseTheAttackersStdDev pins the normaliser assumption every
// crit calculation in the game depends on: because both rolls share the
// attacker's stdDev, their difference has stdDev*sqrt(2). If a defence were
// rolled with its OWN stdDev, dividing by stdDev*sqrt(2) downstream would be
// wrong and crit rates would shift everywhere.
func TestRun_BothSidesUseTheAttackersStdDev(t *testing.T) {
	res := Run(200, []Entry{{Name: "d", Score: 50}})
	assert.Equal(t, res.AttackRoll.StdDev, res.DefenseRoll.StdDev,
		"defence must be rolled with the ATTACKER's stdDev")
}

// TestRun_ZeroScoreAttackerDoesNotPanic — dice.StdDevFor floors at 1.0 for any
// mean below 1.0, so a zero-score attacker still rolls with a real spread rather
// than degenerating. This pins that the core does not produce NaN when scores
// collapse to zero.
func TestRun_ZeroScoreAttackerDoesNotPanic(t *testing.T) {
	res := Run(0, []Entry{{Name: "d", Score: 0}})
	assert.True(t, res.Contested)
	assert.False(t, math.IsNaN(res.Margin), "margin must not be NaN")
}

// TestAgainstDifficulty covers the 12 sites that contest a fixed number rather
// than an opponent: search, track, forage, knockdown, prone recovery. Without
// this they cannot use the core and stay on a parallel path.
func TestAgainstDifficulty(t *testing.T) {
	// A hopeless attempt against a very high bar.
	res := AgainstDifficulty(10, 10000)
	assert.True(t, res.Contested, "a difficulty check is still a contest")
	assert.Less(t, res.Margin, 0.0, "failing badly means a negative attack-positive margin")

	// A trivial attempt against a very low bar.
	res = AgainstDifficulty(10000, 10)
	assert.Greater(t, res.Margin, 0.0)
}

// TestAgainstDifficulty_HasNoWinnerName documents that a difficulty contest has
// no named defender, which is why callers must test Contested rather than
// Winner to ask whether a contest occurred.
func TestAgainstDifficulty_HasNoWinnerName(t *testing.T) {
	res := AgainstDifficulty(100, 100)
	assert.True(t, res.Contested)
	assert.Equal(t, "", res.Winner, "a static difficulty has no name")
}

// TestRun_SuccessTracksMargin — Success is simply "the attacker won", which is
// margin > 0. It exists so floored contests can report an outcome that no
// longer matches their margin.
func TestRun_SuccessTracksMargin(t *testing.T) {
	win := Run(10000, []Entry{{Name: "d", Score: 1}})
	assert.True(t, win.Success, "a decisive attacker win must report Success")
	assert.Greater(t, win.Margin, 0.0)

	lose := Run(1, []Entry{{Name: "d", Score: 10000}})
	assert.False(t, lose.Success, "a decisive attacker loss must not report Success")
	assert.Less(t, lose.Margin, 0.0)
}

// TestRunWithFloors_ZeroFloorsMatchRun — with both floors off, the floored
// variant must behave exactly like Run. This is the property that makes the
// migration safe for any caller passing 0.
func TestRunWithFloors_ZeroFloorsMatchRun(t *testing.T) {
	for i := 0; i < 500; i++ {
		res := RunWithFloors(10000, []Entry{{Name: "d", Score: 1}}, 0)
		assert.True(t, res.Success, "attacker should win on merit with no floors")
		assert.Greater(t, res.Margin, 0.0, "margin must be the real one, not a sentinel")
		assert.False(t, res.Floored, "no floor was configured, so nothing was floored")
	}
}

// TestRunWithFloors_SuccessFloorRescuesAHopelessAttack pins the 5.9 guarantee:
// a hopelessly outclassed attacker still lands roughly floorSuccess of the time.
func TestRunWithFloors_SuccessFloorRescuesAHopelessAttack(t *testing.T) {
	const samples = 20000
	wins := 0
	for i := 0; i < samples; i++ {
		// Attacker cannot win on merit at these scores.
		if RunWithFloors(1, []Entry{{Name: "d", Score: 100000}}, 0.15).Success {
			wins++
		}
	}
	rate := float64(wins) / samples
	assert.InDelta(t, 0.15, rate, 0.02, "success floor should rescue ~15% of hopeless attacks")
}

// TestRunWithFloors_ResistFloorSavesADoomedDefender is the mirror.
func TestRunWithFloors_ResistFloorSavesADoomedDefender(t *testing.T) {
	const samples = 20000
	saves := 0
	for i := 0; i < samples; i++ {
		if !RunWithFloors(100000, []Entry{{Name: "d", Score: 1}}, 0.15).Success {
			saves++
		}
	}
	rate := float64(saves) / samples
	assert.InDelta(t, 0.15, rate, 0.02, "resist floor should save ~15% of doomed defences")
}

// TestRunWithFloors_StampsTheSentinelMargin is load-bearing. A floored outcome
// carries margin +-1, NOT its real margin. Downstream, ContestCrit normalises
// that sentinel to a near-zero z, which is the only reason a floor-granted hit
// cannot also be a critical hit.
func TestRunWithFloors_StampsTheSentinelMargin(t *testing.T) {
	// 0.5 is the clamp CEILING, not a certainty, so each call is a coin flip on
	// whether the floor fires at all. This must therefore loop AND prove it
	// actually observed both branches. A single draw per branch would execute
	// no assertions at all on roughly a quarter of runs and still report PASS —
	// measured at 502/2000 — which is precisely how a "load-bearing" test
	// silently stops guarding anything while staying green.
	sawSuccess, sawResist := false, false

	for i := 0; i < 200; i++ {
		granted := RunWithFloors(1, []Entry{{Name: "d", Score: 100000}}, 0.5)
		if granted.Success {
			sawSuccess = true
			assert.Equal(t, 1.0, granted.Margin, "a floor-granted success carries the +1 sentinel")
			assert.True(t, granted.Floored, "a flipped outcome must report Floored")
		}

		resisted := RunWithFloors(100000, []Entry{{Name: "d", Score: 1}}, 0.5)
		if !resisted.Success {
			sawResist = true
			assert.Equal(t, -1.0, resisted.Margin, "a floor-granted resist carries the -1 sentinel")
			assert.True(t, resisted.Floored, "a flipped outcome must report Floored")
		}
	}

	assert.True(t, sawSuccess, "success floor never fired in 200 draws — the test asserted nothing")
	assert.True(t, sawResist, "resist floor never fired in 200 draws — the test asserted nothing")
}

// TestRunWithFloors_ClampsFloors — above 0.5 a floor stops being a last resort
// and becomes the dominant term, so dice.OpposedRollStatWithFloors clamps to
// [0, 0.5]. The core must clamp identically or migrated callers change
// behaviour.
func TestRunWithFloors_ClampsFloors(t *testing.T) {
	const samples = 20000
	wins := 0
	for i := 0; i < samples; i++ {
		// 5.0 must be clamped to 0.5, not treated as "always".
		if RunWithFloors(1, []Entry{{Name: "d", Score: 100000}}, 5.0).Success {
			wins++
		}
	}
	rate := float64(wins) / samples
	assert.InDelta(t, 0.5, rate, 0.03, "a floor above 0.5 must clamp to 0.5")

	// A negative floor must clamp to 0, never rescue anything.
	for i := 0; i < 500; i++ {
		assert.False(t, RunWithFloors(1, []Entry{{Name: "d", Score: 100000}}, -1.0).Success,
			"a negative floor must clamp to 0")
	}
}

// TestRunWithFloors_UncontestedIsUntouched — no entries means no contest, so
// there is nothing for a floor to flip.
func TestRunWithFloors_UncontestedIsUntouched(t *testing.T) {
	res := RunWithFloors(100, nil, 0.5)
	assert.False(t, res.Contested)
	assert.Equal(t, 0.0, res.Margin, "an uncontested result must not be stamped with a sentinel")
	assert.False(t, res.Floored)
}

// A symmetric floor bounds the outcome to [F, 1-F] from BOTH directions. The
// hopeless attacker is rescued at rate F; the overwhelming attacker is stopped
// at rate F. One parameter expresses both.
func TestRunWithFloors_SymmetricBoundsBothDirections(t *testing.T) {
	const iterations = 20000
	const floor = 0.125

	hopeless := 0
	for i := 0; i < iterations; i++ {
		if RunWithFloors(10, []Entry{{Score: 10000}}, floor).Success {
			hopeless++
		}
	}
	rate := float64(hopeless) / iterations
	if rate < 0.10 || rate > 0.15 {
		t.Fatalf("hopeless attacker should win about %v of the time, got %v", floor, rate)
	}

	overwhelming := 0
	for i := 0; i < iterations; i++ {
		if !RunWithFloors(10000, []Entry{{Score: 10}}, floor).Success {
			overwhelming++
		}
	}
	rate = float64(overwhelming) / iterations
	if rate < 0.10 || rate > 0.15 {
		t.Fatalf("overwhelming attacker should be stopped about %v of the time, got %v", floor, rate)
	}
}

// A floored outcome must carry the sentinel margin, which is the only reason a
// floor-granted hit cannot also crit.
func TestRunWithFloors_FlippedOutcomeCarriesSentinel(t *testing.T) {
	for i := 0; i < 5000; i++ {
		res := RunWithFloors(10, []Entry{{Score: 10000}}, 0.5)
		if res.Floored {
			if res.Margin != 1 && res.Margin != -1 {
				t.Fatalf("a floored outcome must carry the +-1 sentinel, got %v", res.Margin)
			}
			return
		}
	}
	t.Fatal("no floored outcome observed in 5000 attempts at floor 0.5")
}
