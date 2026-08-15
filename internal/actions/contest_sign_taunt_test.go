package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/stats"
)

// ATTACKER-SIDE mirror of internal/combat/contest_sign_test.go (roadmap U3).
//
// That file guards the two DEFENDER-side crit reads, TryStoicResolve and
// TrySpellDeflection. It guards nothing attacker-side, and a final U3 review
// proved the hole with a one-token mutation in combat_taunt.go:
//
//	res.Margin  ->  res.AttackRoll.Margin
//
// which kills taunt criticals permanently while internal/actions,
// internal/combat and internal/hooks all stayed green. Taunt is a site U3 itself
// migrated, into a .Margin read U3 itself introduced. This test is the thing
// that notices.
//
// TRAP 1 -- THE MARGIN GOES TO ZERO. contest.Run rolls via dice.Roll, which
// populates Value/Mean/StdDev/ZScore/Percentile but NOT .Success or .Margin.
// Only the older dice.OpposedRoll populates those two. So res.AttackRoll.Margin
// and res.DefenseRoll.Margin are a constant 0 on every contest this package
// runs. Reading one compiles, and nothing else fails.
//
// TRAP 2 -- THE MARGIN IS SIGNED THE WRONG WAY. contest.Result.Margin is
// ATTACK-positive. ExecuteTaunt performs the ATTACKER's crit check, so it must
// pass the margin UNNEGATED; the defensive mirror in TryStoicResolve negates.
// Getting that backwards also compiles, and puts the crit on the losing side.
//
// THE ONLY OBSERVABLE EITHER TRAP DISTURBS IS TauntResult.Crit. A zeroed margin
// normalises to z = 0 and a flipped margin to a large negative z; both sit below
// combat.ContestCritThreshold forever, so Crit simply stops being true. Damage
// still lands, the taunt still hits, nothing panics.
//
// Hence: drive an overwhelming taunter against a hopeless target and require
// Crit to appear. Counting happens inside the loop; every assertion runs
// unconditionally AFTER it, including a total-count check, so this test can
// never report PASS having checked nothing.

// pinTauntContestKnobs pins every config knob that can reach TauntResult.Crit,
// for the duration of the calling test.
//
// A Go test binary never loads _datafiles/config.yaml, so these are whatever
// validation leaves behind. Pinning them is not paranoia, it is what keeps the
// test from going vacuous:
//
//   - MinAttackCritChance. AttackContestCrit wraps ContestCrit in
//     ApplyCritFloor(_, AttackCritFloor()), which PROMOTES a non-crit to a crit
//     with that probability and knows nothing about the margin. At any non-zero
//     value it would hand out crits regardless of the margin and BOTH traps
//     would sail through: the test would pass against broken code. That vacuous
//     pass has already bitten this arc once. Pinned to 0 so a margin-derived
//     crit is the only possible route to Crit == true.
//
//   - ContestFloor. Taunt resolves through combat.RunContest, and that floor is
//     symmetric: it flips a hopeless attacker to Success carrying the +-1
//     sentinel margin, which cannot crit and simply steals iterations, and it
//     steals them the other way for the defender. Pinned to 0 so every
//     iteration is a genuine contest. Before U6 this pinned the maneuver floor
//     pair, which RunContest no longer reads; a validated ContestFloor is never
//     zero, so the pin now matters even in a test binary.
//
//   - MinDefenseCritChance, for the same reason on the other side. A promoted
//     defensive crit inside TryStoicResolve does not touch Crit, but pinning it
//     keeps the non-crit branch deterministic.
//
//   - SkillWeight, pinned NON-zero at 5. It is the multiplier on both sides'
//     rhetoric, so it sets the score gap the iteration-count arithmetic below is
//     computed from. Left unpinned, a retune of the shipped knob would silently
//     move the crit probability this test budgets against.
func pinTauntContestKnobs(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()

	c.Balance.MinAttackCritChance = 0
	c.Balance.MinDefenseCritChance = 0

	c.Balance.ContestFloor = 0

	c.Balance.SkillWeight = tauntSkillWeight

	configs.SetConfigForTest(t, c)
}

// setTauntStatBase sets a stat through its INPUT field and derives the rest.
//
// Base is the input; Value and ValueAdj are DERIVED by Recalculate(). Assigning
// ValueAdj directly appears to work until something triggers a recalculation:
// OnSkillUse reaches OnStatUse, which can reach IncreaseStat, which bumps
// Training and calls Character.Validate() -> RecalculateStats() ->
// StatInfo.Recalculate(), overwriting ValueAdj from Base.
func setTauntStatBase(t *testing.T, si *stats.StatInfo, base int) {
	t.Helper()
	si.Base = base
	si.Recalculate()
	if si.ValueAdj != base {
		t.Fatalf("stat setup failed: ValueAdj = %d, want %d (Training=%d Mods=%d) -- "+
			"a fresh mob was expected to derive ValueAdj straight from Base",
			si.ValueAdj, base, si.Training, si.Mods)
	}
}

// Scores, and the arithmetic that picks the iteration count.
//
// contest.Run rolls BOTH sides with the ATTACKER's standard deviation
// (stdDev := dice.StdDevFor(atkScore)), so the difference of the two rolls has
// standard deviation stdDev*sqrt(2). RollSpread is dice's package-level 0.15 in
// a test binary; nothing calls SetRollSpread here, and the test asserts that
// rather than assuming it.
//
// ExecuteTaunt builds:
//
//	A = Charisma + rhetoric*SkillWeight, then * convMult
//	D = Willpower + rhetoric*SkillWeight
//
// convMult is ResourceMultiplier(Conviction, ConvictionMax, ConvictionPenaltyMax),
// and newTestMob leaves the taunter at 999 of 999, so the ratio is 1.0 and
// convMult is exactly 1.0 whatever the penalty knob says. With the values below:
//
//	A          = 200 + 10*5                  = 250
//	D          =  20 +  0*5                  = 20
//	s          = StdDevFor(250) = 250 * 0.15 = 37.5
//	s*sqrt(2)  = spread of (atk - def)       = 53.03
//
// contest.Result.Margin ~ N(A - D, s*sqrt(2)) = N(230, 53.03). AttackContestCrit
// fires when Margin / (s*sqrt(2)) >= combat.ContestCritThreshold (2.0), i.e.
// when Margin >= 106.07:
//
//	z = (106.07 - 230) / 53.03 = -2.34   ->   p(crit | contested) = 0.990
//
// A fumble (AttackRoll.ZScore <= -2.0, about 2.3%) returns before the crit check
// is reached, and a miss needs Margin < 0 at z = -4.34 (about 7e-6). So per
// iteration p(Crit) is about 0.977 * 0.990 = 0.967, and the chance of NEVER
// seeing a crit across 100 iterations is 0.033^100, on the order of 1e-148 --
// far below the 1e-6 false-failure budget.
//
// The gap is robust to RollSpread drifting, because s scales with A and cancels:
// the per-iteration crit z depends only on D/A, at (D/A - 0.5757) / 0.2121. Even
// at D/A twice this test's 0.08 the probability is still about 0.97.
const (
	tauntSkillWeight      = 5.0
	tauntAttackCharisma   = 200
	tauntAttackRhetoric   = 10
	tauntDefenseWillpower = 20
	tauntIterations       = 100

	// Instance id the target mob is registered under. Kept clear of the 200-221
	// block command_readiness_drift_test.go uses.
	tauntTargetInstanceId = 3300
)

// TestExecuteTaunt_CriticalStillReachable guards the ATTACKER-side crit read in
// ExecuteTaunt. It passes res.Margin to combat.AttackContestCrit UNNEGATED;
// negating it, or reverting to a res.AttackRoll.Margin read that contest.Run
// never populates, kills TauntResult.Crit without failing anything else.
func TestExecuteTaunt_CriticalStillReachable(t *testing.T) {
	pinTauntContestKnobs(t)

	// The iteration count above is computed from these two; assert them rather
	// than assume them, so a change to either fails here instead of quietly
	// eroding the false-failure budget.
	if combat.ContestCritThreshold != 2.0 {
		t.Fatalf("combat.ContestCritThreshold = %v, want 2.0 -- the iteration-count "+
			"arithmetic in this file is derived from 2.0", combat.ContestCritThreshold)
	}
	const wantAttackScore = float64(tauntAttackCharisma + tauntAttackRhetoric*tauntSkillWeight)
	if got, want := dice.StdDevFor(wantAttackScore), wantAttackScore*0.15; got != want {
		t.Fatalf("dice.StdDevFor(%v) = %v, want %v -- this file's arithmetic assumes "+
			"RollSpread is dice's package-level 0.15 in a test binary", wantAttackScore, got, want)
	}

	t.Cleanup(func() { mobs.SetInstanceForTest(tauntTargetInstanceId, nil) })

	crits, hitsNoCrit, misses, fumbles, notExecuted := 0, 0, 0, 0, 0
	for i := 0; i < tauntIterations; i++ {
		// Fresh pair every iteration. ExecuteTaunt fires OnSkillUse after the
		// roll, which can reach IncreaseStat -> RecalculateStats and drift the
		// scores the arithmetic above is computed from. Rebuilding removes the
		// drift entirely rather than reasoning about which direction it goes.
		target := newTestMob(t, func(m *mobs.Mob) {
			m.InstanceId = tauntTargetInstanceId
			m.Character.Name = "Target"
			setTauntStatBase(t, &m.Character.Stats.Willpower, tauntDefenseWillpower)
			setCombatPositionParallel(&m.Character, position.Standing)
		})
		mobs.SetInstanceForTest(tauntTargetInstanceId, target)

		attacker := newTestMob(t, func(m *mobs.Mob) {
			m.Character.Name = "Taunter"
			setTauntStatBase(t, &m.Character.Stats.Charisma, tauntAttackCharisma)
			m.Character.Skills = map[string]int{string(skills.Rhetoric): tauntAttackRhetoric}
			m.Character.SetAggro(0, tauntTargetInstanceId, characters.DefaultAttack)
		})

		if i == 0 {
			// One-shot setup guard. Verified once because the pair is rebuilt
			// identically every iteration.
			if got := attacker.Character.GetSkillLevel(skills.Rhetoric); got != tauntAttackRhetoric {
				t.Fatalf("attacker rhetoric = %d, want %d -- a StatMod is leaking into "+
					"the score the iteration count was computed from", got, tauntAttackRhetoric)
			}
			if got := target.Character.GetSkillLevel(skills.Rhetoric); got != 0 {
				t.Fatalf("target rhetoric = %d, want 0 -- the defence score would exceed "+
					"the one the iteration count was computed from", got)
			}
		}

		res := ExecuteTaunt(&MobActor{Mob: attacker})
		switch {
		case !res.Executed:
			notExecuted++
		case res.Fumble:
			fumbles++
		case res.Crit:
			crits++
		case res.Hit:
			hitsNoCrit++
		default:
			misses++
		}
	}

	// Unconditional. Nothing below is inside a branch that a roll could skip.
	if got := crits + hitsNoCrit + misses + fumbles + notExecuted; got != tauntIterations {
		t.Fatalf("counted %d outcomes, want %d -- the loop did not run to completion",
			got, tauntIterations)
	}
	if notExecuted != 0 {
		t.Fatalf("%d of %d iterations never executed the taunt (cooldown, no aggro or "+
			"unresolved target) -- the test setup is broken, not the crit path",
			notExecuted, tauntIterations)
	}
	if crits == 0 {
		t.Errorf("ExecuteTaunt never reported Crit across %d iterations "+
			"(crits=%d hits=%d misses=%d fumbles=%d); expected about 96.7%% of iterations "+
			"to crit at attack score %d vs defence score %d. The attack crit derives from "+
			"the contest margin, so this is what a zeroed margin (res.AttackRoll.Margin is "+
			"never populated by contest.Run) or a margin passed to AttackContestCrit with "+
			"the wrong sign looks like",
			tauntIterations, crits, hitsNoCrit, misses, fumbles,
			tauntAttackCharisma+tauntAttackRhetoric*int(tauntSkillWeight), tauntDefenseWillpower)
	}
}
