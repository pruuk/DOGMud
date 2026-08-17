package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/stats"
)

// Regression coverage for the two SILENT traps in the defensive side of the
// non-physical channels (roadmap U3 task 1b, re-pointed by U6 Task 12).
//
// It was written against TrySpellDeflection and TryStoicResolve, the two flat
// avoidance functions that ran a SECOND independent contest on top of each
// channel's primary roll. U6 Task 12 deleted both and folded the spell and
// social channels onto ResolveChannelDefence. The traps did not go anywhere:
// that function performs the same DEFENDER-side crit read on the same
// attack-positive margin, so the guard was re-pointed rather than dropped.
//
// Both tests here are DEFENDER-side. The attacker-side mirror is
// internal/actions/contest_sign_taunt_test.go, added when a U3 review found that
// mutating combat_taunt.go's res.Margin to res.AttackRoll.Margin killed taunt
// criticals with every package still green.
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
// THE ONLY OBSERVABLE EITHER TRAP DISTURBS IS THE 0.0 RETURN.
// ResolveChannelDefence returns 1.0 (defender lost), a partial multiplier in
// (0.0, 0.5] (defender won), or 0.0 (defender won decisively = defensive crit =
// full negation). A zeroed margin normalises to z = 0 and a flipped margin
// normalises to a large NEGATIVE z; both sit below ContestCritThreshold forever,
// so 0.0 simply stops happening -- AND the partial multiplier silently pins to
// its 0.5 floor, since DefenceMitigation clamps a non-positive margin. Every
// other assertion one might write about the return ("between 0 and 1") stays
// true under both mutations, which is why this file asserts the 0.0 explicitly.
//
// U6 Task 12 also made the partial a CURVE rather than a flat configured value.
// TestResolveChannelDefence_PartialIsACurveNotAStep is what keeps it one: under
// either sign mutation every partial pins to the 0.5 bare-win floor, because
// DefenceMitigation clamps a non-positive margin, and that test fails on the
// absence of any value strictly between the endpoints.
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
//   - ContestFloor. A hit floor can flip a hopeless attacker to Success with
//     the +-1 sentinel margin, which returns 1.0 and simply steals iterations.
//     Measured 0 in a test binary today, but config.yaml ships ContestFloor at
//     0.125, so a future config-loading TestMain would start eating a chunk of
//     runs. (Pre-U6 this pinned four per-channel knobs; U6 collapsed them into
//     this one symmetric floor -- see the pinning code below.)
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
//   - The five per-defence effectiveness knobs, pinned to 1.0. U6 Task 12 made
//     ResolveChannelDefence scale each defence's score by its knob before the
//     contest, so an unpinned QuellEffectiveness or DefyEffectiveness would move
//     the defence score the iteration-count arithmetic below is computed from.
//     They validate to 1.0 in a test binary today; pinned so that stays true.
//
// The two flat avoidance multipliers this used to pin are gone: U6 Task 12
// deleted SpellAvoidanceDamageMultiplier and RhetoricAvoidanceDamageMultiplier
// along with the functions that read them. The partial return is now the
// DefenceMitigation curve, whose endpoints are structural rather than tunable,
// so there is nothing left to pin on that side.
func pinAvoidanceContestKnobs(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()

	// U6: both paths resolve through RunContest, which reads ContestFloor and
	// no longer looks at the per-channel Min*Chance knobs this used to pin.
	c.Balance.ContestFloor = 0

	c.Balance.MinAttackCritChance = 0
	c.Balance.MinDefenseCritChance = 0

	c.Balance.QuellEffectiveness = 1.0
	c.Balance.DefyEffectiveness = 1.0
	c.Balance.DodgeEffectiveness = 1.0
	c.Balance.ParryEffectiveness = 1.0
	c.Balance.BlockEffectiveness = 1.0

	// SkillWeight multiplies both sides' skill. Both characters below are built
	// skill-less so it cannot move either score, but pinning it keeps that true
	// if the fixtures ever grow a skill.
	c.Balance.SkillWeight = 5.0

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
// ResolveChannelDefence rolls BOTH sides with the ATTACKER's standard deviation,
// through contest.Run, which does `stdDev := dice.StdDevFor(atkScore)` and rolls
// every entry with it. RollSpread is 0.15 in a test binary (dice's package-level
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

// avoidanceOutcomes tallies what ResolveChannelDefence returned across a run.
//
// The three buckets are the three OUTCOMES, not three values: the partial is a
// curve since U6 Task 12, so anything strictly inside (0, 1) is a partial and
// only the two endpoints are exact.
type avoidanceOutcomes struct {
	fullNegations int // 0.0 -- defensive crit
	bareWins      int // exactly 0.5 -- floored save, or a margin at/below zero
	curvedWins    int // strictly between 0.0 and 0.5 -- the curve did something
	attackWins    int // 1.0
	outOfRange    int // anything else; always a bug
}

func (o avoidanceOutcomes) total() int {
	return o.fullNegations + o.bareWins + o.curvedWins + o.attackWins + o.outOfRange
}

// runAvoidanceContest drives ResolveChannelDefence n times and buckets the
// returns. Counting happens inside the loop and every assertion in the callers
// runs unconditionally after it, so no caller can report PASS having checked
// nothing.
func runAvoidanceContest(t *testing.T, n int, channel AttackChannel, attacker, defender *characters.Character) avoidanceOutcomes {
	t.Helper()

	var out avoidanceOutcomes
	for i := 0; i < n; i++ {
		// The defender carries no userId, which is the no-user sentinel: the
		// progression calls inside look up nothing and must not panic.
		result := ResolveChannelDefence(channel, attacker, defender)
		mult := result.DamageMultiplier
		if result.DefenceType == "" {
			t.Errorf("iteration %d returned no selected defence for a contested channel", i)
		}
		if mult < 1.0 && !result.Defended {
			t.Errorf("iteration %d returned defensive multiplier %v with Defended=false", i, mult)
		}
		if mult == 1.0 && result.Defended {
			t.Errorf("iteration %d returned attack-win multiplier with Defended=true", i)
		}
		if result.DefensiveCrit != (mult == 0.0) {
			t.Errorf("iteration %d returned multiplier %v with DefensiveCrit=%v", i, mult, result.DefensiveCrit)
		}
		if mult == 1.0 && result.NormalizedDefenceMargin != 0 {
			t.Errorf("iteration %d attack win normalized margin = %v, want zero", i, result.NormalizedDefenceMargin)
		}
		switch {
		case mult == 0.0:
			out.fullNegations++
		case mult == 1.0:
			out.attackWins++
		case mult == 0.5:
			out.bareWins++
		case mult > 0.0 && mult < 0.5:
			out.curvedWins++
		default:
			out.outOfRange++
			t.Errorf("iteration %d returned %v, which is outside the contract: a "+
				"defensive win yields at most 0.5 (DefenceMitigation's bare-win "+
				"floor) and an attack win yields exactly 1.0", i, mult)
		}
	}
	return out
}

// TestResolveChannelDefence_SpellMentalFullNegationStillReachable guards the
// DEFENDER-side crit read for the mental-spell channel, answered by quell.
//
// ResolveChannelDefence passes `-res.Margin` to DefenseContestCrit. Dropping the
// negation, or reverting to a `res.DefenseRoll.Margin` read that contest.Run
// never populates, kills the 0.0 return without failing anything else.
//
// Note the defender's stat. The deleted TrySpellDeflection contested the
// defender's PERCEPTION; quell contests WILLPOWER. Losing perception as a
// spell-defence stat is the intended outcome of the unification, and this line
// is where that shows up in the guard.
func TestResolveChannelDefence_SpellMentalFullNegationStillReachable(t *testing.T) {
	pinAvoidanceContestKnobs(t)

	attacker := characters.New()
	setStatBase(t, &attacker.Stats.Willpower, avoidanceAttackStat)

	defender := characters.New()
	setStatBase(t, &defender.Stats.Willpower, avoidanceDefenseStat)

	out := runAvoidanceContest(t, avoidanceIterations, ChannelSpellMental, attacker, defender)

	// Unconditional. Nothing below is inside a branch that a roll could skip.
	if got := out.total(); got != avoidanceIterations {
		t.Fatalf("counted %d outcomes, want %d -- the loop did not run to completion",
			got, avoidanceIterations)
	}
	if out.fullNegations == 0 {
		t.Errorf("ResolveChannelDefence(ChannelSpellMental) never returned 0.0 across %d "+
			"iterations (full negations=%d bare=%d curved=%d attack wins=%d); expected "+
			"~99.7%% of iterations to fully negate at attack score %d vs defence score %d. "+
			"The defensive crit derives from the contest margin, so this is what a zeroed "+
			"margin (res.DefenseRoll.Margin is never populated by contest.Run) or a margin "+
			"passed to DefenseContestCrit with the wrong sign looks like",
			avoidanceIterations, out.fullNegations, out.bareWins, out.curvedWins,
			out.attackWins, avoidanceAttackStat, avoidanceDefenseStat)
	}
}

// TestResolveChannelDefence_SocialFullNegationStillReachable guards the same
// read on the social channel, answered by defy.
//
// The path it replaces (TryStoicResolve) was migrated in U3 from a
// dice.OpposedRollStatWithFloors result whose defRoll.Margin was already
// DEFENCE-positive and correctly unnegated, onto a contest.Result whose Margin
// is ATTACK-positive and must be negated. Both halves of that had to change
// together, silently, and this test is what would have noticed.
func TestResolveChannelDefence_SocialFullNegationStillReachable(t *testing.T) {
	pinAvoidanceContestKnobs(t)

	attacker := characters.New()
	setStatBase(t, &attacker.Stats.Charisma, avoidanceAttackStat)

	defender := characters.New()
	setStatBase(t, &defender.Stats.Willpower, avoidanceDefenseStat)

	out := runAvoidanceContest(t, avoidanceIterations, ChannelSocial, attacker, defender)

	if got := out.total(); got != avoidanceIterations {
		t.Fatalf("counted %d outcomes, want %d -- the loop did not run to completion",
			got, avoidanceIterations)
	}
	if out.fullNegations == 0 {
		t.Errorf("ResolveChannelDefence(ChannelSocial) never returned 0.0 across %d "+
			"iterations (full negations=%d bare=%d curved=%d attack wins=%d); expected "+
			"~99.7%% of iterations to fully negate at attack score %d vs defence score %d. "+
			"The defensive crit derives from the contest margin, so this is what a zeroed "+
			"margin (contest.Run does not populate RollResult.Margin) or a margin passed "+
			"to DefenseContestCrit with the wrong sign looks like",
			avoidanceIterations, out.fullNegations, out.bareWins, out.curvedWins,
			out.attackWins, avoidanceAttackStat, avoidanceDefenseStat)
	}
}

// TestResolveChannelDefence_OverwhelmingAttackerIsNeverNegated pins the OTHER
// side of the sign.
//
// The two tests above prove a decisive DEFENDER can reach 0.0. They cannot, on
// their own, prove that a decisive ATTACKER does not: a total inversion of the
// convention satisfies both by symmetry. Reverse the scores and require the
// mirror -- every iteration an attack win, and no full negation ever.
//
// The floor is pinned to 0 in pinAvoidanceContestKnobs, so there is no
// last-resort save to steal iterations here.
func TestResolveChannelDefence_OverwhelmingAttackerIsNeverNegated(t *testing.T) {
	pinAvoidanceContestKnobs(t)

	attacker := characters.New()
	setStatBase(t, &attacker.Stats.Willpower, avoidanceDefenseStat)

	defender := characters.New()
	setStatBase(t, &defender.Stats.Willpower, avoidanceAttackStat)

	out := runAvoidanceContest(t, avoidanceIterations, ChannelSpellMental, attacker, defender)

	if got := out.total(); got != avoidanceIterations {
		t.Fatalf("counted %d outcomes, want %d -- the loop did not run to completion",
			got, avoidanceIterations)
	}
	if out.fullNegations != 0 {
		t.Errorf("ResolveChannelDefence fully negated %d of %d iterations with the "+
			"attacker at score %d against a defender at %d. A crit on the losing side "+
			"is what an inverted margin convention looks like",
			out.fullNegations, avoidanceIterations, avoidanceDefenseStat, avoidanceAttackStat)
	}
	if out.attackWins == 0 {
		t.Errorf("ResolveChannelDefence never returned 1.0 across %d iterations against "+
			"a defender outscored two to one (full negations=%d bare=%d curved=%d); "+
			"res.Success is being read backwards",
			avoidanceIterations, out.fullNegations, out.bareWins, out.curvedWins)
	}
}

// TestResolveChannelDefence_PartialIsACurveNotAStep is the guard the flat
// avoidance multipliers did not need and the curve does.
//
// A defensive win mitigates 50% at a bare win, rising to 100% at
// ContestCritThreshold, so the attacker's multiplier runs from 0.5 down to 0.0
// CONTINUOUSLY. Both sign mutations collapse that: DefenceMitigation clamps a
// non-positive margin to the 0.5 floor, so a zeroed or inverted margin makes
// every partial come back at exactly 0.5 and the curve silently becomes a step
// back to the pre-U6 flat multiplier. Neither test above notices, because both
// count a 0.5 as a defensive win.
//
// Run at PARITY, where all three outcomes are common: the attack wins about
// half the time, the defence wins the rest with margins spread across the curve,
// and about 2.3% of contests clear two sigma into a full negation. Over 2000
// iterations the chance of missing any one bucket is far below the 1e-6
// false-failure budget.
func TestResolveChannelDefence_PartialIsACurveNotAStep(t *testing.T) {
	pinAvoidanceContestKnobs(t)

	const parityIterations = 2000

	attacker := characters.New()
	setStatBase(t, &attacker.Stats.Willpower, avoidanceAttackStat)

	defender := characters.New()
	setStatBase(t, &defender.Stats.Willpower, avoidanceAttackStat)

	out := runAvoidanceContest(t, parityIterations, ChannelSpellMental, attacker, defender)

	if got := out.total(); got != parityIterations {
		t.Fatalf("counted %d outcomes, want %d -- the loop did not run to completion",
			got, parityIterations)
	}
	if out.curvedWins == 0 {
		t.Errorf("no defensive win across %d parity iterations landed strictly between "+
			"0.0 and 0.5 (full negations=%d bare=%d attack wins=%d). Every partial pinned "+
			"to the bare-win floor, which is what DefenceMitigation returns for a margin "+
			"that is zero or wrongly signed -- the curve has become a step",
			parityIterations, out.fullNegations, out.bareWins, out.attackWins)
	}
	if out.attackWins == 0 {
		t.Errorf("the attacker won none of %d parity iterations, so the fixture is not "+
			"at parity and the curve assertion above proves less than it claims",
			parityIterations)
	}
}
