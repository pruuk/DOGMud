package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// U10b-1 Task 10, the ATTACKER half of the melee firing convention.
//
// Before this task the per-weapon loop sat inside `if !wh.CleanHit { continue }`,
// so a weapon whose every swing was deflected or missed outright built no
// progression.Outcome at all. The convention for the slice is that a RESOLVED
// action always produces one event -- full weight on a win, and
// Balance.ProgressionFailureFraction on a loss -- and a swing that missed is a
// contest that resolved and lost, not a contest that never happened.
//
// Two facts decide how these fixtures are built, both verified against source
// rather than assumed:
//
//  1. calculateCombat appends EXACTLY ONE WeaponHitInfo per plan weapon,
//     unconditionally (combat.go:600), and buildAttackPlan takes plan.weapons
//     from collectAttackWeapons with no filtering. So a weapon that swung and
//     never landed is still in the list, and awarding per entry means awarding
//     per weapon that SWUNG.
//  2. wh.CleanHit is OR-aggregated across that weapon's swings, so it means
//     "this weapon landed at least one unanswered blow this round".
//
// Assertions follow internal/hooks/defender_progression_bestof_test.go: bracket
// ProgressionFailureFraction at 0 and 1.0 rather than counting trials (0
// short-circuits the chance to zero before any roll, so "advanced nothing" is
// EXACT; 1.0 makes a loss advance with the same certainty as a win), and read
// the USE COUNTERS, which OnSkillUseScaled tracks unconditionally, to prove an
// award fired and fired the right number of times.
const (
	attackerAwardSkill   = "weapon-combat"
	attackerAwardStat    = "dexterity" // weapon-combat's primary
	attackerUnarmedSkill = "unarmed-combat"
)

// pinCertainAttackerProgressionForTest makes a rank-0 full-weight award CERTAIN
// for BOTH melee skills and pins the failure fraction under test.
//
// BaseProgressionChance is 2.0, not the 1.0 its defender-side twin uses,
// because skills.SkillProgressionMultipliers is a per-skill PACE dial applied
// after the curve and unarmed-combat ships at 0.69. At base 1.0 an unarmed
// full-weight roll would sit at 0.69 and every unarmed assertion below would be
// a coin flip. 2.0 x 0.69 clamps to 1.0; weapon-combat's 1.27 was already
// clamping. Nulling Balance.SkillProgressionMultipliers does NOT remove the
// pace dial -- GetSkillProgressionMultiplier falls back to the hardcoded map on
// a config miss, and a test binary never loads config.yaml at all.
func pinCertainAttackerProgressionForTest(t *testing.T, failureFraction float64) {
	t.Helper()
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	cfg.Balance.BaseProgressionChance = 2.0
	cfg.Balance.StatProgressionRate = 1.0
	cfg.Balance.StatProgressionMultipliers = nil
	cfg.Balance.SkillProgressionMultipliers = nil
	cfg.Balance.ProgressionFailureFraction = configs.ConfigFloat(failureFraction)
	configs.SetConfigForTest(t, cfg)
}

// requireCertainAttackerAward fails the test if the pinned config does not make
// a full-weight award certain, so the exact assertions below cannot silently
// degrade into coin flips.
func requireCertainAttackerAward(t *testing.T, c *characters.Character) {
	t.Helper()
	for _, skill := range []string{attackerAwardSkill, attackerUnarmedSkill} {
		if got := c.ProgressionChanceForSkill(skill, 1.0); got < 1.0 {
			t.Fatalf("precondition: a full-weight %s roll has chance %v, not the pinned certainty of 1.0", skill, got)
		}
	}
	if got := c.ProgressionChanceForStat(attackerAwardStat, 1.0); got < 1.0 {
		t.Fatalf("precondition: a full-weight %s roll has chance %v, not the pinned certainty of 1.0", attackerAwardStat, got)
	}
}

// weaponSwing is a one-line constructor so the fixtures below read as the round
// they describe. hit=false, clean=false is an outright miss; hit=true,
// clean=false is a deflection (real damage dealt, contest lost); clean=true is
// an unanswered blow.
func weaponSwing(skill string, hit, clean bool) combat.WeaponHitInfo {
	return combat.WeaponHitInfo{SkillTag: skill, Hit: hit, CleanHit: clean}
}

// The plan's required test. A round whose swings were ALL deflected still
// trains the attacker's weapon skill, at the fraction.
//
// The zero half of the bracket: at fraction 0 the chance short-circuits before
// any roll, so "advanced nothing" is EXACT, while the use counter proves the
// award was still MADE at reduced weight rather than skipped entirely.
func TestProcessAttackerProgression_ADeflectedRoundAwardsAtTheFraction_ZeroAwardsNothing(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 0.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	result := combat.AttackResult{
		Hit:          true, // deflected: damage landed, the contest was lost
		CleanHit:     false,
		SwingsThrown: 2,
		WeaponHits:   []combat.WeaponHitInfo{weaponSwing(attackerAwardSkill, true, false)},
	}

	before := c.Skills[attackerAwardSkill]
	processAttackerProgression(c, 81, result)

	if got := c.Skills[attackerAwardSkill] - before; got != 0 {
		t.Errorf("%s advanced by %d at failure fraction 0, want 0", attackerAwardSkill, got)
	}
	if got := c.GetSkillUseCount(attackerAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; a weapon whose swings were all deflected must still fire ONE award at reduced weight", attackerAwardSkill, got)
	}
	// Exactly one stat roll, not two. CandidateFor leaves Candidate.Stat empty,
	// meaning "the skill's primary", and OnSkillUseScaled already rolls that
	// primary -- so naming dexterity explicitly on the Outcome as the pre-task
	// code did is equivalent, and neither form may pay a SECOND roll.
	if got := c.GetStatUseCount(attackerAwardStat); got != 1 {
		t.Errorf("%s use count = %d, want 1; the award paid a second stat roll on top of the skill's primary", attackerAwardStat, got)
	}
}

// The other half of the bracket: at fraction 1.0 the same deflected round
// advances with certainty. Together with the zero case this pins the weight to
// the KNOB rather than to a hardcoded constant -- an implementation that awards
// nothing on a loss passes one and fails the other.
func TestProcessAttackerProgression_ADeflectedRoundReadsTheKnob_FullFractionAdvances(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 1.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	result := combat.AttackResult{
		Hit:          true,
		SwingsThrown: 1,
		WeaponHits:   []combat.WeaponHitInfo{weaponSwing(attackerAwardSkill, true, false)},
	}

	before := c.Skills[attackerAwardSkill]
	processAttackerProgression(c, 82, result)

	if got := c.Skills[attackerAwardSkill] - before; got != 1 {
		t.Errorf("%s advanced by %d on a DEFLECTED round at failure fraction 1.0, want 1", attackerAwardSkill, got)
	}
}

// An outright MISS awards too, on the same terms as a deflection.
//
// This is the case the plan's title ("a defended melee swing") does not name
// and nothing else pins. Replacing the `continue` awards on everything that is
// !CleanHit, which is both a deflection AND a miss, and that is correct: the
// slice's convention is that a resolved action pays a fraction on a loss, and a
// missed swing is a contest that resolved and lost. A special case that paid a
// deflection but not a miss would be a different rule.
//
// The fixture is deliberately Hit:false as well as CleanHit:false -- a
// deflection has Hit true, so an implementation that keyed on Hit rather than
// CleanHit would pass the two tests above and fail this one.
func TestProcessAttackerProgression_AnOutrightMissAwardsAtTheFractionToo(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 1.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	result := combat.AttackResult{
		Hit:          false, // nothing landed at all
		CleanHit:     false,
		SwingsThrown: 3,
		WeaponHits:   []combat.WeaponHitInfo{weaponSwing(attackerAwardSkill, false, false)},
	}

	before := c.Skills[attackerAwardSkill]
	processAttackerProgression(c, 83, result)

	if got := c.GetSkillUseCount(attackerAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; a weapon that swung and missed every time must still fire ONE award", attackerAwardSkill, got)
	}
	if got := c.Skills[attackerAwardSkill] - before; got != 1 {
		t.Errorf("%s advanced by %d on a fully MISSED round at failure fraction 1.0, want 1", attackerAwardSkill, got)
	}
}

// The no-regression half: a CLEAN hit still awards FULL weight. Pinned to
// failure fraction 0, so only a win can advance anything and this cannot pass
// by accident on the loss path.
func TestProcessAttackerProgression_ACleanHitStillAwardsFullWeight(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 0.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	result := combat.AttackResult{
		Hit:          true,
		CleanHit:     true,
		SwingsThrown: 1,
		WeaponHits:   []combat.WeaponHitInfo{weaponSwing(attackerAwardSkill, true, true)},
	}

	before := c.Skills[attackerAwardSkill]
	processAttackerProgression(c, 84, result)

	if got := c.Skills[attackerAwardSkill] - before; got != 1 {
		t.Errorf("%s advanced by %d on a CLEAN hit at failure fraction 0, want 1", attackerAwardSkill, got)
	}
	if got := c.GetStatTraining(attackerAwardStat); got < 1 {
		t.Errorf("%s training = %d on a clean hit, want at least 1", attackerAwardStat, got)
	}
}

// A two-weapon round awards TWICE, once per weapon.
//
// This pins the decision NOT to collapse the loop into one Best-of across
// weapons. Each weapon is its own resolved action, and the pre-U9 firing
// condition was per weapon; Best-of applies WITHIN one resolved action (the
// weapon-plus-skullduggery case is Task 11), not across a round's weapons.
//
// Both entries are losses at fraction 0, so nothing advances and the counter is
// the whole assertion -- which is the point: a collapse to Best-of would leave
// the count at 1.
func TestProcessAttackerProgression_ATwoWeaponRoundAwardsOncePerWeapon(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 0.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	result := combat.AttackResult{
		Hit:          true,
		SwingsThrown: 4,
		WeaponHits: []combat.WeaponHitInfo{
			weaponSwing(attackerAwardSkill, true, false),
			weaponSwing(attackerAwardSkill, false, false),
		},
	}

	processAttackerProgression(c, 85, result)

	if got := c.GetSkillUseCount(attackerAwardSkill); got != 2 {
		t.Errorf("%s use count = %d, want 2; a two-weapon round awards once per weapon, not once for the round", attackerAwardSkill, got)
	}
	if got := c.GetStatUseCount(attackerAwardStat); got != 2 {
		t.Errorf("%s use count = %d, want 2; each weapon's award rolls the skill's primary once", attackerAwardStat, got)
	}
}

// The UNARMED consequence, pinned so the design decision it raises has a test
// to change.
//
// collectAttackWeapons (internal/combat/combat_helpers.go) contributes a fist
// for EACH empty hand slot, and CombatSkillTagForItem maps ItemId 0 to
// unarmed-combat -- so a both-hands-empty attacker produces TWO unarmed-combat
// WeaponHits entries and therefore TWO awards per round, where a two-handed
// weapon user produces one. Under the pre-task CleanHit gate those two entries
// were two CHANCES at an award; they are now two certainties.
//
// Whether unarmed should instead be Best-of'd across fists, or paid once per
// round -- and the same question for dual-wielders -- is a design decision, not
// a cleanup. This test records what the code does today so that decision has
// something to break.
func TestProcessAttackerProgression_BothFistsOfAnUnarmedRoundAwardSeparately(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 0.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	// What collectAttackWeapons builds for empty main hand + empty offhand.
	result := combat.AttackResult{
		Hit:          true,
		SwingsThrown: 4,
		WeaponHits: []combat.WeaponHitInfo{
			weaponSwing(attackerUnarmedSkill, true, false),
			weaponSwing(attackerUnarmedSkill, false, false),
		},
	}

	processAttackerProgression(c, 86, result)

	if got := c.GetSkillUseCount(attackerUnarmedSkill); got != 2 {
		t.Errorf("%s use count = %d, want 2; both fists of a bare-handed round award separately", attackerUnarmedSkill, got)
	}
}

// A one-handed weapon plus an empty offhand trains BOTH skills in the same
// round: the weapon entry awards weapon-combat and the offhand fist entry
// awards unarmed-combat. Under the old gate the fist only trained on the rounds
// it happened to land.
//
// Fraction 1.0 so both awards advance with certainty, which pins that the two
// entries really are awarded independently rather than one displacing the
// other -- a progression.Outcome carries exactly one AttackerSkill, so a
// round-level collapse would silently drop whichever skill lost.
func TestProcessAttackerProgression_AWeaponAndAnEmptyOffhandTrainBothSkills(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 1.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	result := combat.AttackResult{
		Hit:          true,
		SwingsThrown: 3,
		WeaponHits: []combat.WeaponHitInfo{
			weaponSwing(attackerAwardSkill, true, false),
			weaponSwing(attackerUnarmedSkill, false, false),
		},
	}

	beforeWeapon := c.Skills[attackerAwardSkill]
	beforeUnarmed := c.Skills[attackerUnarmedSkill]
	processAttackerProgression(c, 87, result)

	if got := c.Skills[attackerAwardSkill] - beforeWeapon; got != 1 {
		t.Errorf("%s advanced by %d, want 1", attackerAwardSkill, got)
	}
	if got := c.Skills[attackerUnarmedSkill] - beforeUnarmed; got != 1 {
		t.Errorf("%s advanced by %d, want 1; the offhand fist is its own resolved action", attackerUnarmedSkill, got)
	}
}

// A round with no weapon entries at all awards NOTHING.
//
// This state is UNREACHABLE in production -- collectAttackWeapons cannot return
// empty (every hand slot contributes, and a final fallback appends a bare fist
// when nothing else did), buildAttackPlan filters none of it, and calculateCombat
// appends one entry per plan weapon unconditionally. The assertion is therefore
// a guard on the LOOP's shape, not coverage of a live path: it says the
// function's whole output comes from WeaponHits and that no consolation award
// is synthesised from the round-level CleanHit/Hit aggregates beside it. The
// deleted `len(WeaponHits) == 0 && res.CleanHit` fallback was exactly such a
// synthesis, and it was dead for the same reason.
func TestProcessAttackerProgression_NoWeaponEntriesAwardsNothing(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 1.0)

	c := characters.New()

	result := combat.AttackResult{
		Hit:          true,
		CleanHit:     true,
		SwingsThrown: 2,
	}

	processAttackerProgression(c, 88, result)

	if len(c.SkillUseCount) != 0 {
		t.Errorf("a round with no weapon entries recorded skill uses %v, want none", c.SkillUseCount)
	}
	if len(c.StatUseCount) != 0 {
		t.Errorf("a round with no weapon entries recorded stat uses %v, want none", c.StatUseCount)
	}
}

// An entry whose SkillTag names no real skill awards nothing rather than
// reaching CheckSkillProgression with an unrecognised name, which banners it to
// the player verbatim. CandidateFor returns the zero Candidate for a skill with
// no primary stat and BestOf reports false on it.
//
// Not reachable from CombatSkillTagForItem, which returns one of three real
// tags -- this guards the seam, so a future weapon category that forgets its
// mapping degrades to silence instead of to player-visible garbage.
func TestProcessAttackerProgression_AnUnknownSkillTagAwardsNothing(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 1.0)

	c := characters.New()

	result := combat.AttackResult{
		Hit:          true,
		SwingsThrown: 1,
		WeaponHits:   []combat.WeaponHitInfo{weaponSwing("siege-combat", true, true)},
	}

	processAttackerProgression(c, 89, result)

	if len(c.SkillUseCount) != 0 {
		t.Errorf("an unknown weapon skill tag recorded skill uses %v, want none", c.SkillUseCount)
	}
}

// Belt and braces on the fixture's premise: the constants above name real
// skills, so a rename that left the tests compiling could not quietly turn
// every assertion into the unknown-skill no-op case.
func TestAttackerAwardFixtureNamesRealSkills(t *testing.T) {
	for _, skill := range []string{attackerAwardSkill, attackerUnarmedSkill} {
		if got := skills.GetSkillPrimaryStat(skill); got == "" {
			t.Errorf("fixture skill %q has no primary stat, so every award in this file is a no-op", skill)
		}
	}
	if got := skills.GetSkillPrimaryStat(attackerAwardSkill); got != attackerAwardStat {
		t.Errorf("%s primary stat = %q, want %q", attackerAwardSkill, got, attackerAwardStat)
	}
}
