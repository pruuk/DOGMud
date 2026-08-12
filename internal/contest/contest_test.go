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
