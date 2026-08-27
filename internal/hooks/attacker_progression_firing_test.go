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
// after the curve and unarmed-combat ships at 1.01 (0.69 before U10b-1 Task
// 23's re-solve). At base 1.0 a pre-re-solve unarmed full-weight roll sat at
// 0.69 and every unarmed assertion below would have been a coin flip. Base 2.0
// clamps it to 1.0 with room to spare either way, which is why this pin is
// deliberately NOT tuned to the exact shipped multiplier: it has to survive a
// retune, and it has now survived one. Nulling Balance.SkillProgressionMultipliers does NOT remove the
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

// A two-weapon round awards ONCE for the round, not once per weapon.
//
// U10b-1 Task 11 inverted this test. It previously pinned the opposite -- two
// entries, two awards -- on the reasoning that each weapon is its own resolved
// action. That reasoning did not survive contact with what WeaponHits actually
// contains: one entry per HAND SLOT, including a synthesised fist for every
// empty hand and up to four more from the extra-arms mutation. Paying per entry
// paid per hand, so a two-handed weapon (one entry) trained at a sixth the rate
// of a six-armed attacker, and weapon SPEED -- the thing a player would call
// "more swings" -- contributed nothing at all, because every swing of one
// weapon folds into that weapon's single entry.
//
// Both entries are losses at fraction 0, so nothing advances and the use
// counter is the whole assertion -- which is the point: the per-weapon model
// would leave the count at 2.
func TestProcessAttackerProgression_ATwoWeaponRoundAwardsOnceForTheRound(t *testing.T) {
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

	if got := c.GetSkillUseCount(attackerAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; two weapons of the same skill collapse to ONE candidate and one award", attackerAwardSkill, got)
	}
	if got := c.GetStatUseCount(attackerAwardStat); got != 1 {
		t.Errorf("%s use count = %d, want 1; the round's single award rolls the skill's primary once", attackerAwardStat, got)
	}
}

// The UNARMED case, which is why the per-weapon model had to go.
//
// collectAttackWeapons (internal/combat/combat_helpers.go) contributes a fist
// for EACH empty hand slot, and CombatSkillTagForItem maps ItemId 0 to
// unarmed-combat -- so a both-hands-empty attacker produces TWO unarmed-combat
// WeaponHits entries. Paying per entry gave a bare-handed fighter two certain
// awards a round against a two-handed weapon user's one, for no reason a player
// could see.
//
// Both fists now collapse into ONE unarmed-combat candidate carrying the better
// of their two rolls, so the bare-handed rate matches everyone else's.
func TestProcessAttackerProgression_BothFistsOfAnUnarmedRoundCollapseToOne(t *testing.T) {
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

	if got := c.GetSkillUseCount(attackerUnarmedSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; both fists are one skill and collapse to one award", attackerUnarmedSkill, got)
	}
}

// Six entries -- the extra-arms L4 worst case -- still award ONCE.
//
// This is the shape the per-weapon model priced at six awards a round. It is
// the strongest single guard against a regression to per-entry payment, because
// no plausible wrong implementation lands on 1 by accident.
func TestProcessAttackerProgression_SixArmsStillAwardOnce(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 0.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	hits := make([]combat.WeaponHitInfo, 0, 6)
	for i := 0; i < 6; i++ {
		hits = append(hits, weaponSwing(attackerAwardSkill, true, false))
	}

	processAttackerProgression(c, 88, combat.AttackResult{
		Hit:          true,
		SwingsThrown: 12,
		WeaponHits:   hits,
	})

	if got := c.GetSkillUseCount(attackerAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; six arms are still one resolved round", attackerAwardSkill, got)
	}
}

// A one-handed weapon plus an empty offhand trains exactly ONE skill: whichever
// ROLLED BEST. The other trains nothing.
//
// U10b-1 Task 11 inverted this test too. It used to assert both trained, which
// meant a sword-and-nothing fighter silently trained unarmed-combat every round
// off the hand they were not using. The selector is the roll that ACTUALLY
// HAPPENED (WeaponHitInfo.BestRoll), the same principle bestSwingDefence
// applies on the defender's side.
//
// Run twice with the rolls swapped. One direction alone would pass against an
// implementation that always picks the first entry, or always picks the weapon
// skill.
func TestProcessAttackerProgression_TheBestRollTakesTheRound(t *testing.T) {
	for _, tc := range []struct {
		name              string
		weaponRoll        float64
		fistRoll          float64
		wantWeaponTrained bool
	}{
		{"the weapon out-rolls the fist", 900, 100, true},
		{"the fist out-rolls the weapon", 100, 900, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinCertainAttackerProgressionForTest(t, 1.0)

			c := characters.New()
			requireCertainAttackerAward(t, c)

			weapon := weaponSwing(attackerAwardSkill, true, false)
			weapon.BestRoll = tc.weaponRoll
			fist := weaponSwing(attackerUnarmedSkill, false, false)
			fist.BestRoll = tc.fistRoll

			processAttackerProgression(c, 87, combat.AttackResult{
				Hit:          true,
				SwingsThrown: 3,
				WeaponHits:   []combat.WeaponHitInfo{weapon, fist},
			})

			gotWeapon := c.GetSkillUseCount(attackerAwardSkill)
			gotFist := c.GetSkillUseCount(attackerUnarmedSkill)

			if total := gotWeapon + gotFist; total != 1 {
				t.Fatalf("the round awarded %d events (%s %d, %s %d), want exactly 1",
					total, attackerAwardSkill, gotWeapon, attackerUnarmedSkill, gotFist)
			}
			if tc.wantWeaponTrained && gotWeapon != 1 {
				t.Errorf("%s use count = %d, want 1; it rolled %.0f against the fist's %.0f",
					attackerAwardSkill, gotWeapon, tc.weaponRoll, tc.fistRoll)
			}
			if !tc.wantWeaponTrained && gotFist != 1 {
				t.Errorf("%s use count = %d, want 1; it rolled %.0f against the weapon's %.0f",
					attackerUnarmedSkill, gotFist, tc.fistRoll, tc.weaponRoll)
			}
		})
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

// The award's weight follows the WINNING SKILL's own outcome, not the round's.
//
// This is the divergence the first draft of Task 11 had wrong. It passed
// AttackResult.CleanHit -- "did anything land this round" -- so a one-handed
// fighter whose empty offhand fist happened to land, while the sword itself
// never won a contest, was paid FULL weight for weapon-combat. At the measured
// rates that is roughly one round in four for that build.
//
// The weapon wins SELECTION here (a far higher roll) but did NOT clean-hit,
// while the fist did. Full weight has to mean "you succeeded with THIS skill",
// so the award must land at the fraction. Bracketed at fraction 0, where a
// losing award advances nothing while still being MADE -- the use counter
// proves it fired.
func TestProcessAttackerProgression_WeightFollowsTheWinningSkillNotTheRound(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 0.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	weapon := weaponSwing(attackerAwardSkill, true, false) // rolled best, did NOT clean-hit
	weapon.BestRoll = 900
	fist := weaponSwing(attackerUnarmedSkill, true, true) // clean-hit, but out-rolled
	fist.BestRoll = 100

	before := c.Skills[attackerAwardSkill]
	processAttackerProgression(c, 89, combat.AttackResult{
		Hit:          true,
		CleanHit:     true, // the ROUND landed, via the fist
		SwingsThrown: 4,
		WeaponHits:   []combat.WeaponHitInfo{weapon, fist},
	})

	if got := c.GetSkillUseCount(attackerAwardSkill); got != 1 {
		t.Fatalf("%s use count = %d, want 1; it out-rolled and must take the round's award", attackerAwardSkill, got)
	}
	if got := c.Skills[attackerAwardSkill] - before; got != 0 {
		t.Errorf("%s advanced by %d at failure fraction 0, want 0; the winning skill never clean-hit, so its award is a LOSS even though the round landed", attackerAwardSkill, got)
	}
}

// The mirror: when the winning skill DID clean-hit, the award is full weight.
// Without this, an implementation that always reported a loss would pass the
// test above.
func TestProcessAttackerProgression_TheWinningSkillsOwnCleanHitPaysFull(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 0.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	weapon := weaponSwing(attackerAwardSkill, true, true) // rolled best AND clean-hit
	weapon.BestRoll = 900
	fist := weaponSwing(attackerUnarmedSkill, true, false)
	fist.BestRoll = 100

	before := c.Skills[attackerAwardSkill]
	processAttackerProgression(c, 90, combat.AttackResult{
		Hit:          true,
		CleanHit:     true,
		SwingsThrown: 4,
		WeaponHits:   []combat.WeaponHitInfo{weapon, fist},
	})

	if got := c.Skills[attackerAwardSkill] - before; got != 1 {
		t.Errorf("%s advanced by %d, want 1; the winning skill clean-hit, so the award is full weight and certain", attackerAwardSkill, got)
	}
}

// Two entries of the SAME skill: either landing counts as that skill landing.
//
// Pins the `clean[tag] || wh.CleanHit` aggregation. A dual-wielder whose
// offhand landed while the mainhand missed has still succeeded with
// weapon-combat, exactly as AttackResult.CleanHit aggregates across one
// weapon's swings.
func TestProcessAttackerProgression_EitherEntryOfASkillCountsAsThatSkillLanding(t *testing.T) {
	pinCertainAttackerProgressionForTest(t, 0.0)

	c := characters.New()
	requireCertainAttackerAward(t, c)

	main := weaponSwing(attackerAwardSkill, true, false) // rolled best, missed
	main.BestRoll = 900
	off := weaponSwing(attackerAwardSkill, true, true) // same skill, landed
	off.BestRoll = 100

	before := c.Skills[attackerAwardSkill]
	processAttackerProgression(c, 91, combat.AttackResult{
		Hit:          true,
		CleanHit:     true,
		SwingsThrown: 4,
		WeaponHits:   []combat.WeaponHitInfo{main, off},
	})

	if got := c.Skills[attackerAwardSkill] - before; got != 1 {
		t.Errorf("%s advanced by %d, want 1; one entry of the skill clean-hit, so the skill landed", attackerAwardSkill, got)
	}
}
