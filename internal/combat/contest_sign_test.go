package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/stats"
)

// Regression coverage for the two SILENT traps in the defensive-avoidance pair
// (roadmap U3 task 1b). Written against the PRE-migration code deliberately:
// TrySpellDeflection is already on internal/contest, TryStoicResolve is still on
// dice.OpposedRollStatWithFloors, and this file must pass for BOTH shapes so it
// can prove the later migration of TryStoicResolve was a no-op.
//
// TRAP 1 -- THE MARGIN GOES TO ZERO. contest.Run rolls via dice.Roll, which
// populates Value/Mean/StdDev/ZScore/Percentile but NOT .Success or .Margin. The
// older dice.OpposedRoll DOES populate those two. So the instant a call site
// migrates, any surviving `someRoll.Margin` read silently becomes a constant 0.
// It compiles. Nothing fails.
//
// TRAP 2 -- THE MARGIN IS SIGNED THE WRONG WAY. contest.Result.Margin is
// ATTACK-positive; dice's defRoll.Margin is DEFENCE-positive. These are exact
// opposites, and DefenseContestCrit takes a defence-signed margin. Passing the
// attack-signed one compiles cleanly and puts the crit on the losing side.
//
// THE ONLY OBSERVABLE EITHER TRAP DISTURBS IS THE 0.0 RETURN. Both functions
// return 1.0 (defender lost), a configured partial-avoidance multiplier
// (defender won), or 0.0 (defender won decisively = defensive crit = full
// negation). A zeroed margin normalises to z = 0 and a flipped margin normalises
// to a large NEGATIVE z; both sit below ContestCritThreshold forever, so 0.0
// simply stops happening. The existing avoidance_test.go assertions are all
// `mult >= 0.0`, which is true of every possible return, so they cannot see it.
//
// Hence: drive an overwhelming defender against a hopeless attacker and require
// the 0.0 to appear. Counting happens inside the loop; every assertion runs
// unconditionally AFTER it, so this test can never report PASS having checked
// nothing.

// pinAvoidanceContestKnobs pins every config knob that can reach the 0.0 return,
// for the duration of the calling test.
//
// A Go test binary never loads _datafiles/config.yaml, so these are whatever
// validation leaves behind -- and pinning them is not paranoia, it is what keeps
// the test from going vacuous:
//
//   - The four contest floors. A hit floor can flip a hopeless attacker to
//     Success with the +-1 sentinel margin, which returns 1.0 and simply steals
//     iterations. Measured 0/0 for both pairs in a test binary today, but
//     config.yaml ships MinManeuverResistChance / MinSpellResistChance at 0.05,
//     so a future config-loading TestMain would start eating ~5% of runs.
//
//   - MinDefenseCritChance. DefenseContestCrit wraps ContestCrit in
//     ApplyCritFloor(_, DefenseCritFloor()), which PROMOTES a non-crit to a crit
//     with that probability and knows nothing about the margin. Measured 0 in a
//     test binary today (validateCombat only substitutes 0.01 when the value is
//     < 0 or > 1.0, and the zero value survives that range check) -- but at any
//     non-zero value it would hand out 0.0 returns independently of the margin
//     and BOTH traps would sail through. Pinned to 0 so a margin-derived crit is
//     the only possible route to 0.0.
//
//   - The two avoidance multipliers, pinned NON-zero. These are the "defender
//     won but not decisively" return. If either were 0 the 0.0 observable would
//     be ambiguous and the mutation runs below would pass while broken. They
//     measure 0.5 in a test binary today; pinned so that stays true.
func pinAvoidanceContestKnobs(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()

	c.Balance.MinManeuverHitChance = 0
	c.Balance.MinManeuverResistChance = 0
	c.Balance.MinSpellHitChance = 0
	c.Balance.MinSpellResistChance = 0

	c.Balance.MinAttackCritChance = 0
	c.Balance.MinDefenseCritChance = 0

	c.Balance.SpellAvoidanceDamageMultiplier = 0.5
	c.Balance.RhetoricAvoidanceDamageMultiplier = 0.5

	configs.SetConfigForTest(t, c)
}

// setStatBase sets a stat through its INPUT field and derives the rest.
//
// Base is the input; Value and ValueAdj are DERIVED by Recalculate(). Assigning
// ValueAdj directly (as the older avoidance tests do) appears to work until
// something inside the loop triggers a recalculation: OnStatUse can reach
// IncreaseStat, which bumps Training and calls Character.Validate() ->
// RecalculateStats() -> StatInfo.Recalculate(), overwriting ValueAdj from Base.
// With Base set, that same recalculation is harmless and can only ever raise the
// value, which for this test's defender is the safe direction.
func setStatBase(t *testing.T, si *stats.StatInfo, base int) {
	t.Helper()
	si.Base = base
	si.Recalculate()
	if si.ValueAdj != base {
		t.Fatalf("stat setup failed: ValueAdj = %d, want %d (Training=%d Mods=%d) -- "+
			"a fresh character was expected to derive ValueAdj straight from Base",
			si.ValueAdj, base, si.Training, si.Mods)
	}
}

// Scores, and the arithmetic that picks the iteration count.
//
// Both code paths roll BOTH sides with the ATTACKER's standard deviation --
// contest.Run does `stdDev := dice.StdDevFor(atkScore)` and rolls each entry
// with it, and dice.OpposedRollStatRaw does `OpposedRoll(atk, def,
// StdDevFor(atk))`. RollSpread is 0.15 in a test binary (dice's package-level
// default; nothing calls SetRollSpread here), which was confirmed by observing
// StdDevFor(100) == 15.
//
// With attack score A = 100 and defence score D = 200, and neither side holding
// any skill (so SkillWeight cannot move either score):
//
//	s          = StdDevFor(100)               = 15
//	s*sqrt(2)  = spread of (atk - def)        = 21.21
//
// The attacker wins only when atk - def > 0, and atk - def ~ N(-100, 21.21):
//
//	z = 100 / 21.21 = 4.71   ->   p(attacker wins) ~ 1.2e-6
//
// A defensive crit needs the DEFENCE-signed margin to clear
// ContestCritThreshold (2.0) sigma of the difference, i.e.
// def - atk >= 2 * 21.21 = 42.43, against def - atk ~ N(100, 21.21):
//
//	z = (42.43 - 100) / 21.21 = -2.71   ->   p(crit | contested) = 0.9967
//
// So p(one iteration returns 0.0) ~ 0.9967, and the chance of NEVER seeing a 0.0
// across 200 iterations is 0.0033^200, on the order of 1e-495 -- far below the
// 1e-6 false-failure budget.
//
// The gap is also comfortably robust to RollSpread drifting: even at 0.30 the
// per-iteration crit probability is still ~0.75, which leaves 0.25^200.
const (
	avoidanceAttackStat  = 100
	avoidanceDefenseStat = 200
	avoidanceIterations  = 200
)

// TestTrySpellDeflection_FullNegationStillReachable guards the ALREADY-migrated
// spell path. It passes `-res.Margin` to DefenseContestCrit; dropping the
// negation, or reverting to a `res.DefenseRoll.Margin` read that contest.Run
// never populates, kills the 0.0 return without failing anything else.
func TestTrySpellDeflection_FullNegationStillReachable(t *testing.T) {
	pinAvoidanceContestKnobs(t)

	attacker := characters.New()
	setStatBase(t, &attacker.Stats.Willpower, avoidanceAttackStat)

	defender := characters.New()
	setStatBase(t, &defender.Stats.Perception, avoidanceDefenseStat)

	fullNegations, partials, undeflected, unexpected := 0, 0, 0, 0
	for i := 0; i < avoidanceIterations; i++ {
		// userId 0 is the no-user sentinel: the progression calls inside look up
		// nothing and must not panic.
		switch mult := TrySpellDeflection(attacker, defender, 0); mult {
		case 0.0:
			fullNegations++
		case 0.5:
			partials++
		case 1.0:
			undeflected++
		default:
			unexpected++
			t.Errorf("iteration %d returned %v, want one of 0.0 / 0.5 / 1.0 -- "+
				"SpellAvoidanceDamageMultiplier is pinned to 0.5", i, mult)
		}
	}

	// Unconditional. Nothing below is inside a branch that a roll could skip.
	if got := fullNegations + partials + undeflected + unexpected; got != avoidanceIterations {
		t.Fatalf("counted %d outcomes, want %d -- the loop did not run to completion",
			got, avoidanceIterations)
	}
	if fullNegations == 0 {
		t.Errorf("TrySpellDeflection never returned 0.0 across %d iterations "+
			"(full negations=%d partial=%d undeflected=%d); expected ~99.7%% of iterations "+
			"to fully negate at attack score %d vs defence score %d. The defensive crit "+
			"derives from the contest margin, so this is what a zeroed margin "+
			"(res.DefenseRoll.Margin is never populated by contest.Run) or a margin "+
			"passed to DefenseContestCrit with the wrong sign looks like",
			avoidanceIterations, fullNegations, partials, undeflected,
			avoidanceAttackStat, avoidanceDefenseStat)
	}
}

// TestTryStoicResolve_FullNegationStillReachable guards the NOT-YET-migrated
// conviction path. It currently passes `defRoll.Margin` from
// dice.OpposedRollStatWithFloors, which is already defence-positive and so is
// correctly UNNEGATED. When U3 moves it onto contest.RunWithManeuverFloors that
// field stops being populated at all, and the call must become `-res.Margin`.
// This test is the thing that notices if it does not.
func TestTryStoicResolve_FullNegationStillReachable(t *testing.T) {
	pinAvoidanceContestKnobs(t)

	attacker := characters.New()
	setStatBase(t, &attacker.Stats.Charisma, avoidanceAttackStat)

	defender := characters.New()
	setStatBase(t, &defender.Stats.Willpower, avoidanceDefenseStat)

	fullNegations, partials, unresisted, unexpected := 0, 0, 0, 0
	for i := 0; i < avoidanceIterations; i++ {
		switch mult := TryStoicResolve(attacker, defender, 0); mult {
		case 0.0:
			fullNegations++
		case 0.5:
			partials++
		case 1.0:
			unresisted++
		default:
			unexpected++
			t.Errorf("iteration %d returned %v, want one of 0.0 / 0.5 / 1.0 -- "+
				"RhetoricAvoidanceDamageMultiplier is pinned to 0.5", i, mult)
		}
	}

	if got := fullNegations + partials + unresisted + unexpected; got != avoidanceIterations {
		t.Fatalf("counted %d outcomes, want %d -- the loop did not run to completion",
			got, avoidanceIterations)
	}
	if fullNegations == 0 {
		t.Errorf("TryStoicResolve never returned 0.0 across %d iterations "+
			"(full negations=%d partial=%d unresisted=%d); expected ~99.7%% of iterations "+
			"to fully negate at attack score %d vs defence score %d. The defensive crit "+
			"derives from the contest margin, so this is what a zeroed margin "+
			"(contest.Run does not populate RollResult.Margin) or a margin passed to "+
			"DefenseContestCrit with the wrong sign looks like",
			avoidanceIterations, fullNegations, partials, unresisted,
			avoidanceAttackStat, avoidanceDefenseStat)
	}
}
