package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// U6 Task 8 — the contest floor must gate the WINNER, before any crit is
// derived from the outcome.
//
// The defect these tests pin: resolveDefenseOutcomeCore used to resolve crits
// in step 2 and apply MinAttackHitChance / MinDefenseChance in step 3. Every
// crit branch returned, so against a defender who reliably defence-crit the
// attack floor was only ever evaluated on the non-crit remainder. The Core
// Guardian defence-crit 96.8% of swings, so the floor saw the other 3.2%, and
// the 2.2% hit rate a new player actually observed was defence FUMBLES rather
// than the floor doing its job.
//
// Both tests build a hopeless matchup on purpose: attack score 10 against a
// dexterity-1000 defender. The normalized defence margin there is in the
// hundreds of standard deviations, so the defence crits on essentially every
// unfloored swing -- which is exactly the matchup the floor exists for.

const (
	hopelessAttackScore = 10.0
	hopelessDefenderDex = 1000
)

// hopelessMatchup builds the outmatched attacker and the reliably-critting
// defender. Characters are built once and reused across iterations: nothing in
// GetDefenseScore reads the defence counter or the stamina pool, so per-swing
// state does not leak between samples in a way that moves the measured rate.
func hopelessMatchup() (*characters.Character, *characters.Character) {
	attacker, defender := defenceFixture(1000)
	attacker.Stats.Strength.ValueAdj = 10
	attacker.Stats.Dexterity.ValueAdj = 10
	defender.Stats.Dexterity.ValueAdj = hopelessDefenderDex
	return attacker, defender
}

// TestResolveDefenseOutcome_FloorReachesACrittingDefender is the floor
// reachability guard. A hopelessly outmatched attacker must still connect at
// roughly the contest floor rate even when the defender crits nearly every
// swing.
//
// The bar is deliberately well under the floor itself (ContestFloor validates
// to 0.125 in a Go test binary, which never loads config.yaml). Under the old
// ordering -- crit resolution returning before the floor was ever consulted --
// the only hits available were the ~2.3% of swings on which the defence
// fumbled.
//
// U6 Task 10 CHANGED THE MEASURE, and had to. This counted res.hit, which meant
// "the attack got through" right up until a defensive win started dealing
// partial damage and carrying res.hit == true. Left alone, the test would have
// gone vacuous: the ~90% of swings the defence wins now all count as hits and
// the bar clears without the floor doing anything at all. damageMult == 1.0 is
// the same set of swings res.hit used to name -- every other path (both fumble
// branches, defence crit, deflection) leaves it below 1.0 -- so the assertion
// still measures the floor and not the deflection.
func TestResolveDefenseOutcome_FloorReachesACrittingDefender(t *testing.T) {
	const (
		iterations = 20000
		minHitRate = 0.10
	)

	attacker, defender := hopelessMatchup()
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}
	critThreshold := calcCritThreshold(attacker, defender)

	hits := 0
	for i := 0; i < iterations; i++ {
		result := &AttackResult{}
		best := runBestOfAllDefense(result, attacker, defender,
			[]string{characters.DefenseDodge}, hopelessAttackScore, false, ctx)
		res := resolveDefenseOutcome(result, best, attacker, defender,
			critThreshold, false, false)
		if res.hit && res.damageMult == 1.0 {
			hits++
		}
	}

	hitRate := float64(hits) / float64(iterations)
	t.Logf("hopeless attacker vs reliably-critting defender: hit rate %.4f over %d swings", hitRate, iterations)

	if hitRate < minHitRate {
		t.Errorf("hit rate %.4f is below the floor-reachability bar %.2f; the contest floor "+
			"is not gating the winner -- crit resolution is consuming the outcome first",
			hitRate, minHitRate)
	}
}

// TestResolveDefenseOutcome_FlooredOutcomesNeverCrit pins the other half of the
// ordering. A floored outcome carries the +-1 SENTINEL margin, not a real one,
// so promoting it to a crit would hand a decisive result to the side that
// actually lost the roll.
//
// This exercises resolveDefenseOutcomeCore rather than resolveDefenseOutcome on
// purpose. The 1%-of-hits crit floors (MinAttackCritChance /
// MinDefenseCritChance) are a separate, declared mechanism that runs after the
// core and legitimately promotes crits on any settled hit, floored or not.
// Asserting against the wrapper would fail on those and say nothing about
// resolution order.
func TestResolveDefenseOutcome_FlooredOutcomesNeverCrit(t *testing.T) {
	const iterations = 20000

	attacker, defender := hopelessMatchup()
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}
	critThreshold := calcCritThreshold(attacker, defender)

	floored := 0
	for i := 0; i < iterations; i++ {
		result := &AttackResult{}
		best := runBestOfAllDefense(result, attacker, defender,
			[]string{characters.DefenseDodge}, hopelessAttackScore, false, ctx)
		res := resolveDefenseOutcomeCore(result, best, attacker, defender,
			critThreshold, false, false)

		if !best.floored {
			continue
		}
		floored++

		if res.crit {
			t.Fatalf("iteration %d: a floored outcome produced an attack crit; the sentinel "+
				"margin is not a real margin and must never be promoted", i)
		}
		if res.defenseCrit {
			t.Fatalf("iteration %d: a floored outcome produced a defence crit; the sentinel "+
				"margin is not a real margin and must never be promoted", i)
		}

		// The floor's decision is FINAL. This is the assertion that pins the
		// deleted MinDefenseChance branch: it used to sit downstream of the
		// contest floor and take back roughly one in seven of the hits the floor
		// had just granted, so the two floors partly cancelled and neither knob
		// meant what it said. A fumble is the attacker's own blunder and settles
		// the swing before any of this, so it is excluded.
		if best.margin <= 0 && !res.fumble && !res.hit {
			t.Fatalf("iteration %d: the floor handed the attacker the win and something "+
				"downstream took it away; the floor must be the last word", i)
		}
	}

	if floored == 0 {
		t.Fatal("no swing was floored across the sample; the assertion above never ran, " +
			"which makes this test vacuous -- check that ContestFloor is non-zero")
	}
	t.Logf("%d of %d swings were floored, none of them critted", floored, iterations)
}
