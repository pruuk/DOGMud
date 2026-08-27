package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
)

// U10b-1 Task 9, melee half: the round's defender award is ONE award, for the
// defence that ROLLED BEST across the round's swings, at full weight when that
// defence won and at Balance.ProgressionFailureFraction when it lost.
//
// Before this task processDefenderProgression looped combat.AwardDefenceProgression
// once per defence TYPE that WON (up to three per round, keyed on
// SwingEvent.DefenseUsed, which is stamped only on a defensive win). So a round
// in which every defence lost trained nothing at all, and a defender with
// dodge, parry and block took three rolls where a bare-handed one took one.
//
// PARRY is the fixture defence for the same three reasons the internal/combat
// twin of this file gives: weapon-combat's hardcoded progression multiplier
// (1.34 since U10b-1 Task 23's re-solve, 1.27 before it) clamps a rank-0 chance
// to CERTAINTY under this file's pin, where unarmed-combat's does not, it is the one two-stat defence, and its stat is dexterity, which is
// also weapon-combat's primary, so parry rolls dexterity twice.
const (
	defenderAwardSkill      = "weapon-combat"
	defenderAwardStat       = "dexterity" // parry's stat AND weapon-combat's primary
	defenderAwardSecondStat = "strength"  // parry's SECOND stat
	dodgeAwardSkill         = "unarmed-combat"
)

// pinCertainDefenderProgressionForTest is the internal/hooks copy of
// internal/combat's pinCertainDefenceProgressionForTest: Go test helpers are not
// visible across packages. It makes a rank-0 full-weight award CERTAIN and pins
// the failure fraction the caller wants to measure.
func pinCertainDefenderProgressionForTest(t *testing.T, failureFraction float64) {
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

// requireCertainDefenderAward fails the test if the pinned config does not make
// a full-weight award certain, so the exact assertions below cannot silently
// degrade into coin flips.
func requireCertainDefenderAward(t *testing.T, c *characters.Character) {
	t.Helper()
	if got := c.ProgressionChanceForSkill(defenderAwardSkill, 1.0); got < 1.0 {
		t.Fatalf("precondition: a full-weight %s roll has chance %v, not the pinned certainty of 1.0", defenderAwardSkill, got)
	}
	for _, stat := range []string{defenderAwardStat, defenderAwardSecondStat} {
		if got := c.ProgressionChanceForStat(stat, 1.0); got < 1.0 {
			t.Fatalf("precondition: a full-weight %s roll has chance %v, not the pinned certainty of 1.0", stat, got)
		}
	}
}

// swingDefence is a one-line constructor so the fixtures below read as the
// round they describe.
func swingDefence(defence string, roll float64, won bool) combat.SwingDefence {
	return combat.SwingDefence{Defence: combat.DefenseType(defence), Roll: roll, Won: won}
}

// A round in which NO defence landed still trains the defence that rolled best.
// The zero half of the bracket: at ProgressionFailureFraction 0 the chance
// short-circuits before any roll, so "advanced nothing" is EXACT rather than
// statistical -- while the use counters, which are tracked unconditionally,
// prove the roll was still MADE at reduced weight rather than skipped.
func TestProcessDefenderProgression_ALostRoundStillFiresTheBestDefenceAtTheFraction_ZeroAwardsNothing(t *testing.T) {
	pinCertainDefenderProgressionForTest(t, 0.0)

	const userId = 71
	c := characters.New()
	requireCertainDefenderAward(t, c)

	// Nothing landed: DefenseUsed is never stamped, so SwingEvents carries no
	// defence at all. Only SwingDefences records what the defender put up.
	result := combat.AttackResult{
		SwingEvents: []combat.SwingEvent{{Hit: true}, {Hit: true}},
		SwingDefences: []combat.SwingDefence{
			swingDefence(characters.DefenseParry, 118, false),
			swingDefence(characters.DefenseParry, 94, false),
		},
	}

	before := c.Skills[defenderAwardSkill]
	processDefenderProgression(c, userId, result)

	if got := c.Skills[defenderAwardSkill] - before; got != 0 {
		t.Errorf("%s advanced by %d at failure fraction 0, want 0", defenderAwardSkill, got)
	}
	if got := c.GetSkillUseCount(defenderAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; a round where every defence lost must still fire ONE award at reduced weight", defenderAwardSkill, got)
	}
	if got := c.GetStatUseCount(defenderAwardStat); got != 2 {
		t.Errorf("%s use count = %d, want 2 (weapon-combat's primary plus parry's own stat)", defenderAwardStat, got)
	}
	if got := c.GetStatUseCount(defenderAwardSecondStat); got != 1 {
		t.Errorf("%s use count = %d, want 1; parry's second stat roll is missing", defenderAwardSecondStat, got)
	}
}

// The other half of the bracket: at fraction 1.0 the same lost round advances
// with certainty. Together with the zero case above this pins the weight to the
// KNOB rather than to a hardcoded constant -- an implementation awarding
// nothing on a loss passes one and fails the other.
func TestProcessDefenderProgression_ALostRoundReadsTheKnob_FullFractionAdvances(t *testing.T) {
	pinCertainDefenderProgressionForTest(t, 1.0)

	c := characters.New()
	requireCertainDefenderAward(t, c)

	result := combat.AttackResult{
		SwingDefences: []combat.SwingDefence{swingDefence(characters.DefenseParry, 118, false)},
	}

	before := c.Skills[defenderAwardSkill]
	processDefenderProgression(c, 72, result)

	if got := c.Skills[defenderAwardSkill] - before; got != 1 {
		t.Errorf("%s advanced by %d on a LOST round at failure fraction 1.0, want 1", defenderAwardSkill, got)
	}
	if got := c.GetStatTraining(defenderAwardSecondStat); got < 1 {
		t.Errorf("%s training = %d, want at least 1; parry's second stat is not being awarded", defenderAwardSecondStat, got)
	}
}

// A round using THREE defence types awards ONCE, not three times.
//
// The fixture populates SwingEvents exactly as production would, so the old
// per-type loop has real work to find: reading DefenseUsed it would award
// dodge, parry AND block, which is TWO weapon-combat rolls (parry and block
// share the skill) plus one unarmed-combat roll. Best-of collapses that to the
// single highest roll.
func TestProcessDefenderProgression_ThreeDefenceTypesAwardOnce(t *testing.T) {
	pinCertainDefenderProgressionForTest(t, 0.35)

	c := characters.New()
	requireCertainDefenderAward(t, c)

	result := combat.AttackResult{
		SwingEvents: []combat.SwingEvent{
			{Hit: true, DefenseUsed: combat.DefenseDodge},
			{Hit: true, DefenseUsed: combat.DefenseParry},
			{Hit: true, DefenseUsed: combat.DefenseBlock},
		},
		SwingDefences: []combat.SwingDefence{
			swingDefence(characters.DefenseDodge, 90, true),
			swingDefence(characters.DefenseParry, 121, true),
			swingDefence(characters.DefenseBlock, 60, true),
		},
	}

	processDefenderProgression(c, 73, result)

	if got := c.GetSkillUseCount(defenderAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; the round awarded per defence TYPE instead of once", defenderAwardSkill, got)
	}
	if got := c.GetSkillUseCount(dodgeAwardSkill); got != 0 {
		t.Errorf("%s use count = %d, want 0; only the best-rolling defence earns the round's one award", dodgeAwardSkill, got)
	}
	if got := c.GetStatUseCount(defenderAwardStat); got != 2 {
		t.Errorf("%s use count = %d, want 2; parry alone rolls it twice", defenderAwardStat, got)
	}
	if got := c.GetStatUseCount(defenderAwardSecondStat); got != 1 {
		t.Errorf("%s use count = %d, want 1; only parry awards a second stat", defenderAwardSecondStat, got)
	}
}

// Best-of picks by ROLL, not by which defence happened to win. A dodge that
// rolled 200 and lost outranks a parry that rolled 50 and won, so the round
// trains unarmed-combat at the fraction and weapon-combat not at all.
//
// This is the assertion that separates Best-of from "the first defence that
// won", which is what the old loop effectively did.
func TestProcessDefenderProgression_TheHighestRollWinsTheRoundEvenWhenItLost(t *testing.T) {
	pinCertainDefenderProgressionForTest(t, 0.0)

	c := characters.New()

	result := combat.AttackResult{
		SwingEvents: []combat.SwingEvent{
			{Hit: true},
			{Hit: true, DefenseUsed: combat.DefenseParry},
		},
		SwingDefences: []combat.SwingDefence{
			swingDefence(characters.DefenseDodge, 200, false),
			swingDefence(characters.DefenseParry, 50, true),
		},
	}

	processDefenderProgression(c, 74, result)

	if got := c.GetSkillUseCount(dodgeAwardSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; the highest-rolling defence must earn the round's award even though it lost", dodgeAwardSkill, got)
	}
	if got := c.GetSkillUseCount(defenderAwardSkill); got != 0 {
		t.Errorf("%s use count = %d, want 0; the lower-rolling defence must not be awarded as well", defenderAwardSkill, got)
	}
	// It followed the CHOSEN candidate's outcome, not the round's. At fraction
	// 0 the lost dodge advances nothing.
	if got := c.GetStatTraining("dexterity"); got != 0 {
		t.Errorf("dexterity training = %d, want 0; the award did not take the losing candidate's outcome", got)
	}
}

// A WON defence still awards at FULL weight. The no-regression half: pinned to
// failure fraction 0, only a win can advance anything, so this cannot pass by
// accident on the loss path.
func TestProcessDefenderProgression_AWonDefenceStillAwardsFullWeight(t *testing.T) {
	pinCertainDefenderProgressionForTest(t, 0.0)
	events.DrainQueuedSkillUsedForTest(0)

	const userId = 75
	c := characters.New()
	requireCertainDefenderAward(t, c)

	result := combat.AttackResult{
		SwingEvents: []combat.SwingEvent{{Hit: true, DefenseUsed: combat.DefenseParry}},
		SwingDefences: []combat.SwingDefence{
			swingDefence(characters.DefenseParry, 118, true),
		},
	}

	before := c.Skills[defenderAwardSkill]
	processDefenderProgression(c, userId, result)

	if got := c.Skills[defenderAwardSkill] - before; got != 1 {
		t.Errorf("%s advanced by %d on a WON defence, want 1", defenderAwardSkill, got)
	}
	if got := c.GetStatTraining(defenderAwardSecondStat); got < 1 {
		t.Errorf("%s training = %d on a WON parry, want at least 1", defenderAwardSecondStat, got)
	}
	queued := events.DrainQueuedSkillUsedForTest(userId)
	if len(queued) != 1 {
		t.Fatalf("a WON defence queued %d SkillUsed events, want 1", len(queued))
	}
	if string(queued[0].Skill) != defenderAwardSkill {
		t.Errorf("SkillUsed named %q, want %q", queued[0].Skill, defenderAwardSkill)
	}
}

// An UNCONTESTED round awards NOTHING. getAvailableDefenses came back empty, so
// no defence was quoted, contest.Result.Winner is "" and the swing loop appends
// no SwingDefence at all. Awarding on an empty defence name is not inert:
// AwardDefenceProgression would return early, but a caller that synthesised an
// empty entry into the Best-of could displace a real candidate.
func TestProcessDefenderProgression_AnUncontestedRoundAwardsNothing(t *testing.T) {
	pinCertainDefenderProgressionForTest(t, 1.0)

	c := characters.New()

	result := combat.AttackResult{
		SwingEvents: []combat.SwingEvent{{Hit: true}, {Hit: true}},
	}

	processDefenderProgression(c, 76, result)

	if len(c.SkillUseCount) != 0 {
		t.Errorf("an uncontested round recorded skill uses %v, want none", c.SkillUseCount)
	}
	if len(c.StatUseCount) != 0 {
		t.Errorf("an uncontested round recorded stat uses %v, want none", c.StatUseCount)
	}
}
