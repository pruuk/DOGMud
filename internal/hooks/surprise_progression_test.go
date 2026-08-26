package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// U10d restored the skullduggery faucet that the deleted surprise-attack burst
// used to carry. The whole point of these tests is the ORDERING trap:
// applyCombatProgression is Phase 5 and runs AFTER calculateCombat has already
// DEMOTED Aggro.Type from SurpriseAttack to DefaultAttack, so the signal has to
// travel out on AttackResult.WasSurpriseAttack. Nothing about a condition
// written against Aggro.Type here would fail loudly -- it would simply never
// fire.
//
// Two consequences for how these tests are written:
//
//  1. Every fixture that expects an award carries WasSurpriseAttack: true, and
//     none of them seeds SurpriseAttack. The shared newCombatPairForTest (in
//     progression_duplication_test.go) does seed Aggro.Type -- DefaultAttack --
//     but that seed is inert here, because Phase 5 never reads Aggro.Type at
//     all. Seeding SurpriseAttack on a Phase-5-only fixture would lend false
//     confidence: it would make these tests read as if they were exercising the
//     firing condition whether or not the feature exists at all.
//  2. Assertions are on the USE COUNTER (GetSkillUseCount), never on whether a
//     rank moved. Progression is probabilistic; a rank assertion flakes.
//     OnSkillUseScaled calls TrackSkillUse unconditionally, before the
//     UseSkillProgression gate, so the counter is a reliable "an ordinary
//     event fired" observable.

// surpriseWeaponHits builds N clean weapon-combat hits. Two entries are the
// minimum needed to tell "awarded once per landed weapon" apart from "awarded
// once for the round".
// forceSkullduggeryToWinBestOf makes the surprise candidate beat any weapon
// candidate with certainty.
//
// U10b-1 Task 11 made this necessary. Skullduggery no longer takes an award of
// its own on an ambush; it COMPETES with the round's weapon skills in a single
// Best-of, and on an ordinary character it usually loses. Every test below that
// asserts "skullduggery was awarded" therefore has to state WHY it won, or it
// is asserting a coin flip.
//
// It wins here because characters.CandidateFor rolls
// dice.RollStat(dexterity + skullduggeryLevel*SkillWeight), which at this level
// is orders of magnitude above any attack roll a fixture weapon produces.
//
// Skills is lazily allocated, so GetSkillLevel runs first to create the map --
// assigning into it directly panics on a fresh character.
func forceSkullduggeryToWinBestOf(t *testing.T, c *characters.Character) {
	t.Helper()
	_ = c.GetSkillLevel(skills.Skullduggery)
	c.Skills[string(skills.Skullduggery)] = 5000
}

func surpriseWeaponHits(n int, clean bool) []combat.WeaponHitInfo {
	hits := make([]combat.WeaponHitInfo, 0, n)
	for i := 0; i < n; i++ {
		hits = append(hits, combat.WeaponHitInfo{
			SkillTag: string(skills.WeaponCombat),
			Hit:      true,
			CleanHit: clean,
		})
	}
	return hits
}

// runProgressionPhaseForTest drives ONLY Phase 5 with a caller-supplied
// AttackResult. Driving the full orchestrator would leave WeaponHits and
// CleanHit at the mercy of the dice; the point here is to pin the firing
// condition, and the flag's survival through the real attack path is covered
// separately by TestSurpriseFlag_SurvivesTheAggroDemotionToPhase5.
func runProgressionPhaseForTest(t *testing.T, atk, def actions.Actor, res combat.AttackResult) {
	t.Helper()

	prevRound := util.GetRoundCount()
	util.SetRoundCountForTest(util.RoundCountMinimum)
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })

	applyCombatProgression(atk, def, &res)
	events.ProcessEvents()
}

// Case 1: a landed surprise attack trains skullduggery ONCE for the round, not
// once per weapon that landed. The fixture lands TWO clean weapon hits so the
// two readings are distinguishable.
func TestSurpriseAttack_AwardsSkullduggeryOncePerRound(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	forceSkullduggeryToWinBestOf(t, atk.GetCharacter())
	before := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery))

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(2, true),
		DefenderWasAttacked: true,
	})

	if got := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery)) - before; got != 1 {
		t.Errorf("skullduggery tracked %d times for a two-weapon surprise round, want exactly 1", got)
	}
}

// Case 2: success-only. A surprise attack that never got through the defence
// teaches no ambush craft.
func TestSurpriseAttack_NoCleanHitAwardsNothing(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	before := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery))

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true, // deflected: damage dealt, contest lost
		CleanHit:            false,
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(2, false),
		DefenderWasAttacked: true,
	})

	if got := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery)) - before; got != 0 {
		t.Errorf("skullduggery tracked %d times for a surprise round that never landed, want 0", got)
	}
}

// Case 3: an ambush produces ONE award for the round, not one per weapon plus
// a bonus.
//
// U10b-1 Task 11 inverted this test's premise. It used to assert that
// skullduggery arrived as an ADDITIONAL progression.Outcome beside a per-weapon
// one, because Outcome carries exactly one AttackerSkill and folding the two
// together would have silently stopped the combat skill progressing. Under the
// Best-of firing convention that is no longer the shape: skullduggery COMPETES
// with the weapon skill in a single contest, and exactly one of them earns the
// round's single event.
//
// What the old test was really protecting -- that adding skullduggery must not
// silently delete the weapon award -- is still protected, by asserting the
// total is exactly 1 rather than 0. A regression that dropped the weapon
// candidate entirely on an ambush would leave skullduggery alone in the
// contest, which this cannot distinguish; TestSurpriseAttack_WeaponSkillWins
// below pins that half by making the weapon candidate win outright.
func TestSurpriseAttack_AwardsExactlyOnceForTheRound(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	beforeWeapon := atk.GetCharacter().GetSkillUseCount(string(skills.WeaponCombat))
	beforeSkull := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery))

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WasSurpriseAttack:   true,
		WeaponHits:          surpriseWeaponHits(2, true),
		DefenderWasAttacked: true,
	})

	gotWeapon := atk.GetCharacter().GetSkillUseCount(string(skills.WeaponCombat)) - beforeWeapon
	gotSkull := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery)) - beforeSkull

	if total := gotWeapon + gotSkull; total != 1 {
		t.Errorf("an ambush round awarded %d events (weapon-combat %d, skullduggery %d), want exactly 1",
			total, gotWeapon, gotSkull)
	}
	if gotWeapon == 2 {
		t.Error("weapon-combat was awarded once per weapon; Task 11 collapsed that to one award per round")
	}
}

// Case 3b: the weapon candidate is really in the contest, and can win it.
//
// Case 3a's "exactly one" cannot tell "both competed and skullduggery won" from
// "the weapon candidate was dropped and skullduggery walked over". Here the two
// weapon entries carry an enormous BestRoll, which no CandidateFor roll on a
// rank-0 skullduggery can reach, so the weapon skill must take the award.
func TestSurpriseAttack_WeaponSkillWinsWhenItOutrolls(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	beforeWeapon := atk.GetCharacter().GetSkillUseCount(string(skills.WeaponCombat))
	beforeSkull := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery))

	hits := surpriseWeaponHits(2, true)
	for i := range hits {
		hits[i].BestRoll = 1e6
	}

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WasSurpriseAttack:   true,
		WeaponHits:          hits,
		DefenderWasAttacked: true,
	})

	if got := atk.GetCharacter().GetSkillUseCount(string(skills.WeaponCombat)) - beforeWeapon; got != 1 {
		t.Errorf("weapon-combat tracked %d times, want 1 -- it out-rolled skullduggery and must take the award", got)
	}
	if got := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery)) - beforeSkull; got != 0 {
		t.Errorf("skullduggery tracked %d times, want 0 -- it lost the Best-of", got)
	}
}

// Case 4: an ordinary round awards no skullduggery. This is the fixture that
// makes the whole feature falsifiable -- it is identical to case 1 except for
// WasSurpriseAttack, so deleting the `res.WasSurpriseAttack &&` term from the
// firing condition must break it.
func TestOrdinaryRound_AwardsNoSkullduggery(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	before := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery))

	runProgressionPhaseForTest(t, atk, def, combat.AttackResult{
		Hit:                 true,
		CleanHit:            true,
		WasSurpriseAttack:   false,
		WeaponHits:          surpriseWeaponHits(2, true),
		DefenderWasAttacked: true,
	})

	if got := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery)) - before; got != 0 {
		t.Errorf("an ordinary round tracked skullduggery %d times, want 0", got)
	}
}

// The regression guard for the whole "the demotion ate my signal" class of bug.
//
// This one drives the REAL attack path (handleCombatRound -> calculateCombat ->
// ... -> Phase 5) with Aggro.Type seeded to SurpriseAttack, and pins BOTH
// halves of the trap at once:
//
//   - calculateCombat really does demote Aggro.Type to DefaultAttack, so a
//     Phase 5 condition keyed on it would read the wrong thing; and
//   - the award still happened anyway, which is only possible if
//     WasSurpriseAttack survived every by-value copy between calculateCombat
//     and applyCombatProgression.
//
// If someone re-keys the condition on Aggro.Type, the demotion assertion keeps
// passing and the award assertion starts failing.
func TestSurpriseFlag_SurvivesTheAggroDemotionToPhase5(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	atkChar := atk.GetCharacter()

	// The only place in this file that seeds SurpriseAttack: here it is the
	// input under test, not a shortcut for the firing condition.
	atkChar.SetAggro(0, def.GetMobInstanceId(), characters.SurpriseAttack)
	if atkChar.Aggro == nil || atkChar.Aggro.Type != characters.SurpriseAttack {
		t.Fatal("fixture failed to arm a surprise attack")
	}

	// U10b-1 Task 11 made this fixture load-bearing. Skullduggery no longer
	// takes an award of its own; it COMPETES with the weapon skill in one
	// Best-of, so "skullduggery was tracked" stopped being a reliable proxy for
	// "the flag survived" -- on an ordinary character it usually LOSES the roll
	// and the test would flake without the flag being lost at all. Forcing the
	// win restores the property this test needs: the assertion below fails for
	// exactly one reason again, that no skullduggery candidate was OFFERED,
	// i.e. WasSurpriseAttack did not survive to Phase 5.
	forceSkullduggeryToWinBestOf(t, atkChar)

	before := atkChar.GetSkillUseCount(string(skills.Skullduggery))

	runOneCombatRoundForTest(t, atk, def) // forceCrit: every swing is a clean hit

	// Nil is NOT the demotion. A nil Aggro would satisfy a `!= nil &&` form of
	// this check without the demotion ever having run, certifying a premise the
	// test never actually observed -- so assert non-nil first and fail hard on
	// it.
	require.NotNil(t, atkChar.Aggro, "the round ended aggro entirely; the demotion premise was never observed")
	if atkChar.Aggro.Type == characters.SurpriseAttack {
		t.Error("calculateCombat no longer demotes Aggro.Type; this test's premise needs revisiting")
	}
	if got := atkChar.GetSkillUseCount(string(skills.Skullduggery)) - before; got != 1 {
		t.Errorf("skullduggery tracked %d times after a real surprise round, want 1 -- the flag did not survive to Phase 5", got)
	}
}
