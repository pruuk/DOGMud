package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/stretchr/testify/assert"
)

// k = 0 must be a TRUE identity, not an approximation. It is the default and
// what an absent config key unmarshals to, so any drift here is a silent
// balance change for every contest in the game.
func TestCompressContestGap_IdentityAtZero(t *testing.T) {
	for _, tc := range []struct{ atk, def float64 }{
		{455, 92}, {105, 185}, {200, 200}, {1, 1}, {455, 0}, {455, -50},
	} {
		got := compressContestGap(tc.atk, []contest.Entry{{Score: tc.def}}, 0)
		assert.Equal(t, tc.def, got[0].Score, "atk=%v def=%v", tc.atk, tc.def)
	}
}

// Compression must never touch a contest the attacker is not winning. This is
// what stops it buffing underdogs, which is a separate design decision.
func TestCompressContestGap_LeavesUnderdogsAlone(t *testing.T) {
	got := compressContestGap(105, []contest.Entry{{Score: 185}}, 2.8)
	assert.Equal(t, 185.0, got[0].Score, "an attacker behind on score must be unchanged")

	got = compressContestGap(200, []contest.Entry{{Score: 200}}, 2.8)
	assert.Equal(t, 200.0, got[0].Score, "an exactly even contest must be unchanged")
}

// The headline behaviour: the defence is raised toward the attacker.
func TestCompressContestGap_CompressesALead(t *testing.T) {
	// gap 363, A 455, k 2.8: 363*455/(455 + 2.8*363) = 165165/1471.4 = 112.25
	// so the defence rises from 92 to 455 - 112.25.
	got := compressContestGap(455, []contest.Entry{{Score: 92}}, 2.8)
	assert.InDelta(t, 342.75, got[0].Score, 0.01)
}

// A higher k must raise the defence at least as far. Monotonicity is what makes
// the knob dialable in play without surprises.
func TestCompressContestGap_MonotonicInSaturation(t *testing.T) {
	prev := compressContestGap(455, []contest.Entry{{Score: 92}}, 0)[0].Score
	for _, k := range []float64{0.5, 1.0, 2.0, 2.8, 4.0, 10.0} {
		got := compressContestGap(455, []contest.Entry{{Score: 92}}, k)[0].Score
		assert.Greater(t, got, prev, "k=%v must raise the defence at least as far as the step below", k)
		prev = got
	}
}

// ⚠️ THE REASON THIS FORM EXISTS. The normalized margin must depend ONLY on the
// ratio defence/attack, never on the absolute magnitude of the scores.
//
// The original implementation was `attack - gap^p`, which mixes units: gap^p is
// in score^p but the roll spread it is measured against is in score. That had
// two consequences this test exists to prevent ever returning:
//
//  1. Raising your own score could LOWER your win rate, because the normalized
//     margin (A-D)^p/(0.15*A*sqrt2) turns over at A = D/(1-p). At the shipped
//     exponent that was A = 5D, so a character at 455 facing a defence of 86 was
//     already past the peak. Measured: win fell 0.784 -> 0.725 and crit 17.8% ->
//     10.9% as the attack rose from 455 to 5000. Getting stronger made you worse.
//  2. The knob's strength depended on the absolute score scale, so it could not
//     be tuned once for both a newbie and a veteran.
func TestCompressContestGap_IsScaleFree(t *testing.T) {
	const k = 2.8
	normalized := func(atk, def float64) float64 {
		out := compressContestGap(atk, []contest.Entry{{Score: def}}, k)
		return (atk - out[0].Score) / atk // compressed gap as a fraction of attack
	}

	for _, ratio := range []float64{0.1, 0.19, 0.5, 0.9} {
		want := normalized(100, 100*ratio)
		for _, atk := range []float64{10, 50, 455, 2000, 50000} {
			assert.InDelta(t, want, normalized(atk, atk*ratio), 1e-9,
				"ratio %v must give the same normalized margin at attack %v", ratio, atk)
		}
	}
}

// The direct consequence of scale-freedom, stated as the property players care
// about: more power is never worse. Pinned separately because this is the
// defect that disqualified the previous implementation.
func TestCompressContestGap_MorePowerIsNeverWorse(t *testing.T) {
	const k = 2.8
	const def = 86.0
	prev := -1.0
	for _, atk := range []float64{100, 200, 455, 800, 2000, 5000, 20000} {
		out := compressContestGap(atk, []contest.Entry{{Score: def}}, k)
		z := (atk - out[0].Score) / (0.15 * atk * math.Sqrt2)
		assert.Greater(t, z, prev, "attack %v must not have a smaller normalized margin than the step below", atk)
		prev = z
	}
}

// Each defence is compressed independently, and the ordering must survive: a
// stronger defence must stay stronger, or a mixed set could have its best
// defence overtaken by a worse one.
func TestCompressContestGap_PreservesDefenceOrdering(t *testing.T) {
	out := compressContestGap(455, []contest.Entry{
		{Score: 27}, {Score: 92}, {Score: 352},
	}, 2.8)

	assert.Less(t, out[0].Score, out[1].Score)
	assert.Less(t, out[1].Score, out[2].Score)
	for i, e := range out {
		assert.Less(t, e.Score, 455.0, "entry %d must stay below the attack score", i)
		assert.Greater(t, e.Score, 0.0, "entry %d must stay positive", i)
	}
}

// It must not mutate the caller's slice.
func TestCompressContestGap_DoesNotMutateInput(t *testing.T) {
	in := []contest.Entry{{Score: 92}}
	_ = compressContestGap(455, in, 2.8)
	assert.Equal(t, 92.0, in[0].Score, "the caller's entries must be untouched")
}

// Degenerate inputs must not produce NaN or a negative score.
//
// ⚠️ The sub-1 gap case is here on purpose. The previous `gap^p` form EXPANDED
// any gap below 1 (x^p > x for x<1, p<1) and returned a NEGATIVE effective
// defence for atk=0.5, def=0 -- directly contradicting this test's stated
// intent, which the old version never actually exercised. The saturating form
// cannot do that: compressedGap = gap*A/(A+k*gap) < gap for all k>0, gap>0.
func TestCompressContestGap_HandlesDegenerateInput(t *testing.T) {
	assert.Empty(t, compressContestGap(100, nil, 2.8), "no defenders, nothing to compress")
	assert.Empty(t, compressContestGap(100, []contest.Entry{}, 2.8))
	assert.Equal(t, 0.0, compressContestGap(0, []contest.Entry{{Score: 0}}, 2.8)[0].Score)

	sub := compressContestGap(0.5, []contest.Entry{{Score: 0}}, 2.8)[0].Score
	assert.GreaterOrEqual(t, sub, 0.0, "a sub-1 gap must not drive the defence negative")
	assert.Less(t, sub, 0.5, "a sub-1 gap must still compress, not expand")

	// Negative and NaN saturation must both read as identity rather than
	// poisoning every score. NaN fails every ordinary comparison, which is why
	// the guard is written !(k > 0).
	assert.Equal(t, 92.0, compressContestGap(455, []contest.Entry{{Score: 92}}, -1)[0].Score)
	nan := compressContestGap(455, []contest.Entry{{Score: 92}}, math.NaN())[0].Score
	assert.False(t, math.IsNaN(nan), "NaN saturation must not produce a NaN score")
	assert.Equal(t, 92.0, nan)
}

// Parity must be invariant at EVERY saturation. Compression only ever touches
// mismatches, and that property is what makes this safe to ship behind a knob:
// it structurally cannot disturb an even fight.
func TestRunContest_ParityUnaffectedByCompression(t *testing.T) {
	const trials = 40000
	const score = 200.0

	rate := func(k float64) float64 {
		setContestGapSaturationForTest(t, k)
		wins := 0
		for i := 0; i < trials; i++ {
			if RunContest(score, []contest.Entry{{Score: score}}).Success {
				wins++
			}
		}
		return float64(wins) / trials
	}

	for _, k := range []float64{0, 2.8, 10} {
		assert.InDelta(t, 0.50, rate(k), 0.02, "parity win rate at k=%v", k)
	}
}

// The headline: a large lead wins less overwhelmingly once compressed.
func TestRunContest_CompressionReducesLopsidedWins(t *testing.T) {
	const trials = 40000

	rate := func(k float64) float64 {
		setContestGapSaturationForTest(t, k)
		wins := 0
		for i := 0; i < trials; i++ {
			if RunContest(455, []contest.Entry{{Score: 92}}).Success {
				wins++
			}
		}
		return float64(wins) / trials
	}

	full := rate(0)
	compressed := rate(2.8)

	// ⚠️ ContestFloor is 0.125 and flips that fraction of ALL contested
	// outcomes, so 0.875 is the CEILING on any win rate, not 1.0. An earlier
	// draft of this test asserted > 0.99 and was arithmetically impossible.
	assert.Greater(t, full, 0.85, "uncompressed, a 455-vs-92 attacker should sit near the 0.875 floor ceiling")

	// ⚠️ MARGIN REQUIRED, not a bare Less. Two independent 40k samples of the
	// SAME quantity differ by a coin flip, so `assert.Less(compressed, full)`
	// passed about half the time with the production call site reverted --
	// measured PASS/FAIL/FAIL/FAIL/FAIL/PASS over six runs. The true separation
	// here is ~9pp against a sampling sigma near 0.002, so 0.05 is a wide moat
	// that still cannot be crossed by noise.
	assert.Less(t, compressed, full-0.05, "compression must reduce a lopsided win rate")
	assert.Greater(t, compressed, 0.60, "but the shipped value should still leave a strong attacker clearly dominant")
}

// An underdog's outcome must be untouched at every saturation -- the ahead-only
// rule is what keeps this a crit fix rather than a buff to weak attackers.
func TestRunContest_UnderdogUnaffected(t *testing.T) {
	const trials = 40000

	rate := func(k float64) float64 {
		setContestGapSaturationForTest(t, k)
		wins := 0
		for i := 0; i < trials; i++ {
			if RunContest(105, []contest.Entry{{Score: 185}}).Success {
				wins++
			}
		}
		return float64(wins) / trials
	}

	assert.InDelta(t, rate(0), rate(2.8), 0.01,
		"an attacker behind on score must be unaffected at any saturation")
}
