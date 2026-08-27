package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
)

// U10b-1 Task 8: AwardDefenceProgression carries the RESOLVED OUTCOME of the
// defence, so a defence that lost can be awarded at
// Balance.ProgressionFailureFraction instead of full weight.
//
// PARRY is the fixture defence throughout, for three reasons:
//
//   - its skill is weapon-combat, whose hardcoded progression multiplier is
//     1.27, so a rank-0 roll at multiplier 1.0 clamps to chance 1.0 and is
//     CERTAIN. unarmed-combat (dodge) ships 0.69 and would make every dodge
//     assertion statistical;
//   - it is the ONE two-stat defence, so it exercises the extra strength roll
//     that is the easiest thing to drop while refactoring this function;
//   - weapon-combat's primary stat is dexterity and DefenceSkillAndStat also
//     names dexterity, so parry rolls dexterity TWICE. That is pre-existing
//     behaviour and the use-count assertions below pin it, so a later change
//     that "tidies" one of the two rolls away fails loudly.
const (
	defenceAwardSkill      = "weapon-combat"
	defenceAwardStat       = "dexterity" // parry's stat AND weapon-combat's primary
	defenceAwardSecondStat = "strength"  // parry's SECOND stat
)

// pinCertainDefenceProgressionForTest makes a rank-0 defence award at
// multiplier 1.0 succeed with CERTAINTY, and pins the failure fraction to the
// value the caller wants to measure.
//
// It is the internal/combat twin of internal/characters'
// pinCertainStatProgressionForTest, which cannot be shared: Go test helpers are
// not visible across packages and that one is unexported.
//
// With BaseProgressionChance and StatProgressionRate at 1.0 and no per-stat
// override, ProgressionChanceForStat(rank 0, mult 1.0) is exactly 1.0, and
// ProgressionChanceForSkill is 1.0*1.27 clamped to 1.0. ProgressionRollThreshold
// then equals the roll denominator, which util.Rand can never reach. Only rank 0
// is certain, so every test below uses a FRESH character.
//
// SkillProgressionMultipliers is pinned nil so the hardcoded
// skills.SkillProgressionMultipliers map is what applies -- that is already what
// a test binary sees (it never loads config.yaml), stated explicitly so a future
// config default cannot silently make these assertions statistical.
func pinCertainDefenceProgressionForTest(t *testing.T, failureFraction float64) {
	t.Helper()
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	cfg.Balance.BaseProgressionChance = 1.0
	cfg.Balance.StatProgressionRate = 1.0
	cfg.Balance.StatProgressionMultipliers = nil
	cfg.Balance.SkillProgressionMultipliers = nil
	cfg.Balance.ProgressionFailureFraction = configs.ConfigFloat(failureFraction)
	configs.SetConfigForTest(t, cfg)
}

// requireCertainDefenceAward fails the test if the pinned config does not make a
// full-weight award certain. Without it the exact assertions below would
// silently degrade into coin flips.
func requireCertainDefenceAward(t *testing.T, c *characters.Character) {
	t.Helper()
	if got := c.ProgressionChanceForSkill(defenceAwardSkill, 1.0); got < 1.0 {
		t.Fatalf("precondition: a full-weight %s roll has chance %v, not the pinned certainty of 1.0", defenceAwardSkill, got)
	}
	for _, stat := range []string{defenceAwardStat, defenceAwardSecondStat} {
		if got := c.ProgressionChanceForStat(stat, 1.0); got < 1.0 {
			t.Fatalf("precondition: a full-weight %s roll has chance %v, not the pinned certainty of 1.0", stat, got)
		}
	}
}

// A WON parry awards its skill and BOTH of its stats at full weight, and still
// emits the SkillUsed quest event.
func TestAwardDefenceProgression_AWonParryAwardsSkillAndBothStatsAtFullWeight(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.35) // the shipped fraction; unused on a win
	events.DrainQueuedSkillUsedForTest(0)        // start from a clean queue

	const userId = 91
	c := characters.New()
	requireCertainDefenceAward(t, c)

	beforeSkill := c.Skills[defenceAwardSkill]
	AwardDefenceProgression(c, userId, characters.DefenseParry, true)

	if got := c.Skills[defenceAwardSkill] - beforeSkill; got != 1 {
		t.Errorf("%s advanced by %d on a WON parry, want 1; the chance is pinned to certainty", defenceAwardSkill, got)
	}
	if got := c.GetStatTraining(defenceAwardStat); got < 1 {
		t.Errorf("%s training = %d on a WON parry, want at least 1", defenceAwardStat, got)
	}
	if got := c.GetStatTraining(defenceAwardSecondStat); got < 1 {
		t.Errorf("%s training = %d on a WON parry, want at least 1; parry is the one two-stat defence", defenceAwardSecondStat, got)
	}

	// Exact regardless of any roll: the use counters are tracked
	// unconditionally, so they pin exactly how many calls the function made.
	if got := c.GetSkillUseCount(defenceAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1", defenceAwardSkill, got)
	}
	if got := c.GetStatUseCount(defenceAwardStat); got != 2 {
		t.Errorf("%s use count = %d, want 2 (once as weapon-combat's primary, once as parry's own stat)", defenceAwardStat, got)
	}
	if got := c.GetStatUseCount(defenceAwardSecondStat); got != 1 {
		t.Errorf("%s use count = %d, want 1; parry's second stat roll is missing", defenceAwardSecondStat, got)
	}

	queued := events.DrainQueuedSkillUsedForTest(userId)
	if len(queued) != 1 {
		t.Fatalf("a WON defence queued %d SkillUsed events, want 1", len(queued))
	}
	if string(queued[0].Skill) != defenceAwardSkill {
		t.Errorf("SkillUsed named %q, want %q", queued[0].Skill, defenceAwardSkill)
	}
}

// A LOST defence is scaled by ProgressionFailureFraction. Pinned to ZERO the
// award becomes inert -- CheckSkillProgression and CheckStatProgression both
// return false as soon as the chance is <= 0, before any roll -- so this half of
// the bracket is EXACT rather than statistical.
//
// The use-count assertions are what stop this passing against an implementation
// that simply returns early on a loss: the rolls must still be MADE, at zero
// weight. Progression telemetry is meant to record that the defence happened.
func TestAwardDefenceProgression_ALostDefenceScalesByTheFailureFraction_ZeroAwardsNothing(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.0)
	events.DrainQueuedSkillUsedForTest(0)

	const userId = 92
	c := characters.New()
	requireCertainDefenceAward(t, c)

	beforeSkill := c.Skills[defenceAwardSkill]
	AwardDefenceProgression(c, userId, characters.DefenseParry, false)

	if got := c.Skills[defenceAwardSkill] - beforeSkill; got != 0 {
		t.Errorf("%s advanced by %d on a LOST parry at failure fraction 0, want 0; the outcome is not scaling the skill roll", defenceAwardSkill, got)
	}
	if got := c.GetStatTraining(defenceAwardStat); got != 0 {
		t.Errorf("%s training = %d on a LOST parry at failure fraction 0, want 0; the outcome is not scaling the stat roll", defenceAwardStat, got)
	}
	if got := c.GetStatTraining(defenceAwardSecondStat); got != 0 {
		t.Errorf("%s training = %d on a LOST parry at failure fraction 0, want 0; parry's SECOND stat roll is not being scaled", defenceAwardSecondStat, got)
	}

	// It still FIRED. A loss is a reduced award, not a skipped one.
	if got := c.GetSkillUseCount(defenceAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; a lost defence must still fire its roll at reduced weight", defenceAwardSkill, got)
	}
	if got := c.GetStatUseCount(defenceAwardStat); got != 2 {
		t.Errorf("%s use count = %d, want 2", defenceAwardStat, got)
	}
	if got := c.GetStatUseCount(defenceAwardSecondStat); got != 1 {
		t.Errorf("%s use count = %d, want 1", defenceAwardSecondStat, got)
	}
}

// The other half of the bracket: at a failure fraction of 1.0 a LOST defence
// advances with the same certainty a win does.
//
// Tested together with the zero case above, this pins the multiplier to the
// KNOB rather than to a hardcoded constant. A "loss awards nothing" or "loss
// awards a literal 0.35" implementation passes one of the two and fails the
// other, and neither can be made exact on its own. The shipped 0.35 sits
// strictly between the two ends by construction; asserting on 0.35 directly
// could only ever be a trial count, and this bracket is stronger.
func TestAwardDefenceProgression_ALostDefenceReadsTheKnob_FullFractionAdvances(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 1.0)

	c := characters.New()
	requireCertainDefenceAward(t, c)

	beforeSkill := c.Skills[defenceAwardSkill]
	AwardDefenceProgression(c, 0, characters.DefenseParry, false)

	if got := c.Skills[defenceAwardSkill] - beforeSkill; got != 1 {
		t.Errorf("%s advanced by %d on a LOST parry at failure fraction 1.0, want 1; the multiplier is hardcoded rather than read from the knob", defenceAwardSkill, got)
	}
	if got := c.GetStatTraining(defenceAwardStat); got < 1 {
		t.Errorf("%s training = %d on a LOST parry at failure fraction 1.0, want at least 1", defenceAwardStat, got)
	}
	if got := c.GetStatTraining(defenceAwardSecondStat); got < 1 {
		t.Errorf("%s training = %d on a LOST parry at failure fraction 1.0, want at least 1", defenceAwardSecondStat, got)
	}
}

// A LOST defence must not emit SkillUsed. Awarding progression on losses would
// otherwise turn every "use this skill N times" quest into "fail at it N times"
// -- which is why OnSkillUseScaled takes isLoss as a SEPARATE argument from the
// multiplier, and why this function takes the outcome rather than a bare
// multiplier it would have to keep in agreement with a second flag.
//
// Exact at the SHIPPED fraction: the event is gated on the loss alone, so no
// roll is involved.
func TestAwardDefenceProgression_ALostDefenceEmitsNoSkillUsed(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.35)
	events.DrainQueuedSkillUsedForTest(0)

	const userId = 93
	c := characters.New()

	AwardDefenceProgression(c, userId, characters.DefenseParry, false)

	if got := c.GetSkillUseCount(defenceAwardSkill); got != 1 {
		t.Fatalf("%s use count = %d, want 1; the award did not fire at all, so the event assertion below would pass vacuously", defenceAwardSkill, got)
	}
	if got := events.DrainQueuedSkillUsedForTest(userId); len(got) != 0 {
		t.Errorf("a LOST defence queued %d SkillUsed events, want 0", len(got))
	}
}

// Parry is the ONLY defence that awards a second stat. Dodge trains
// unarmed-combat and dexterity; it must never pick up parry's strength roll.
func TestAwardDefenceProgression_OnlyParryAwardsASecondStat(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.35)

	c := characters.New()
	AwardDefenceProgression(c, 0, characters.DefenseDodge, true)

	if got := c.GetSkillUseCount("unarmed-combat"); got != 1 {
		t.Errorf("unarmed-combat use count = %d, want 1", got)
	}
	if got := c.GetStatUseCount(defenceAwardStat); got != 2 {
		t.Errorf("dexterity use count = %d, want 2 (unarmed-combat's primary plus dodge's own stat)", got)
	}
	if got := c.GetStatUseCount(defenceAwardSecondStat); got != 0 {
		t.Errorf("strength use count = %d, want 0; only parry awards a second stat", got)
	}
}

// An unrecognised defence awards nothing, on a win and on a loss alike. Passing
// an empty skill on is not inert: CheckSkillProgression("") takes the roll and a
// success banners no skill at all.
func TestAwardDefenceProgression_UnrecognisedDefenceAwardsNothing(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.35)
	events.DrainQueuedSkillUsedForTest(0)

	const userId = 94
	for _, won := range []bool{true, false} {
		c := characters.New()
		AwardDefenceProgression(c, userId, "flail-wildly", won)

		if len(c.SkillUseCount) != 0 {
			t.Errorf("won=%v: unrecognised defence recorded skill uses %v, want none", won, c.SkillUseCount)
		}
		if len(c.StatUseCount) != 0 {
			t.Errorf("won=%v: unrecognised defence recorded stat uses %v, want none", won, c.StatUseCount)
		}
	}
	if got := events.DrainQueuedSkillUsedForTest(userId); len(got) != 0 {
		t.Errorf("unrecognised defence queued %d SkillUsed events, want 0", len(got))
	}
}

// A nil character is a no-op, not a panic. Both defence paths can reach this
// with a defender that has already been torn down.
func TestAwardDefenceProgression_NilCharacterDoesNotPanic(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.35)
	AwardDefenceProgression(nil, 5, characters.DefenseParry, true)
	AwardDefenceProgression(nil, 5, characters.DefenseParry, false)
}
