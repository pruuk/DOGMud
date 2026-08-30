package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/stretchr/testify/assert"
)

// p = 1.0 must be a TRUE identity, not an approximation. It is the default, so
// any drift here is a silent balance change for every contest in the game.
func TestCompressContestGap_IdentityAtOne(t *testing.T) {
	for _, tc := range []struct{ atk, def float64 }{
		{455, 92}, {105, 185}, {200, 200}, {1, 1},
	} {
		got := compressContestGap(tc.atk, []contest.Entry{{Score: tc.def}}, 1.0)
		assert.Equal(t, tc.def, got[0].Score, "atk=%v def=%v", tc.atk, tc.def)
	}
}

// Compression must never touch a contest the attacker is not winning. This is
// what stops it buffing underdogs, which is a separate design decision.
func TestCompressContestGap_LeavesUnderdogsAlone(t *testing.T) {
	got := compressContestGap(105, []contest.Entry{{Score: 185}}, 0.5)
	assert.Equal(t, 185.0, got[0].Score, "an attacker behind on score must be unchanged")

	got = compressContestGap(200, []contest.Entry{{Score: 200}}, 0.5)
	assert.Equal(t, 200.0, got[0].Score, "an exactly even contest must be unchanged")
}

// The headline behaviour: the defence is raised toward the attacker.
func TestCompressContestGap_CompressesALead(t *testing.T) {
	// gap 363, p=0.5 -> sqrt(363) = 19.05 -> defence rises to 455 - 19.05
	got := compressContestGap(455, []contest.Entry{{Score: 92}}, 0.5)
	assert.InDelta(t, 435.95, got[0].Score, 0.01)

	// gap 363, p=0.75 -> 363^0.75 = 83.163 -> defence rises to 455 - 83.163
	got = compressContestGap(455, []contest.Entry{{Score: 92}}, 0.75)
	assert.InDelta(t, 371.837, got[0].Score, 0.01)
}

// A lower exponent must raise the defence at least as far. Monotonicity is what
// makes the knob dialable in play without surprises.
func TestCompressContestGap_MonotonicInExponent(t *testing.T) {
	prev := compressContestGap(455, []contest.Entry{{Score: 92}}, 1.0)[0].Score
	for _, p := range []float64{0.9, 0.85, 0.8, 0.75, 0.6, 0.5} {
		got := compressContestGap(455, []contest.Entry{{Score: 92}}, p)[0].Score
		assert.Greater(t, got, prev, "p=%v must raise the defence at least as far as the step above", p)
		prev = got
	}
}

// Each defence is compressed independently, and the ordering must survive: a
// stronger defence must stay stronger, or a mixed set could have its best
// defence overtaken by a worse one.
func TestCompressContestGap_PreservesDefenceOrdering(t *testing.T) {
	out := compressContestGap(455, []contest.Entry{
		{Score: 27}, {Score: 92}, {Score: 352},
	}, 0.75)

	assert.Less(t, out[0].Score, out[1].Score)
	assert.Less(t, out[1].Score, out[2].Score)
	for i, e := range out {
		assert.Less(t, e.Score, 455.0, "entry %d must stay below the attack score", i)
	}
}

// It must not mutate the caller's slice.
func TestCompressContestGap_DoesNotMutateInput(t *testing.T) {
	in := []contest.Entry{{Score: 92}}
	_ = compressContestGap(455, in, 0.75)
	assert.Equal(t, 92.0, in[0].Score, "the caller's entries must be untouched")
}

// Degenerate inputs must not produce NaN or a negative score.
func TestCompressContestGap_HandlesDegenerateInput(t *testing.T) {
	assert.Empty(t, compressContestGap(100, nil, 0.75), "no defenders, nothing to compress")
	assert.Empty(t, compressContestGap(100, []contest.Entry{}, 0.75))
	assert.Equal(t, 0.0, compressContestGap(0, []contest.Entry{{Score: 0}}, 0.75)[0].Score)
}
