package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// U10b-1 Task 9, channel half. resolveChannelAttackWithRunner has always
// awarded the winning defence candidate "whenever the contest ran, win or
// lose" -- but it passed a literal true for the OUTCOME, so a defence that lost
// was paid as though it had won. This file pins the real outcome.
//
// DEFY is the fixture defence: ChannelSocial's defence set is defy alone, so
// exactly one candidate enters the contest and the assertions cannot be
// confounded by a sibling defence. Its skill is rhetoric, whose hardcoded
// progression multiplier is 4.98, so the chance clamps to CERTAINTY under the
// pin below.
//
// Defy rolls TWO stats, and they are different ones: willpower is defy's own
// stat per DefenceSkillAndStat, while charisma is rhetoric's primary and is
// rolled by OnSkillUseScaled. (Parry, the internal/hooks fixture, is the
// opposite case -- its stat and its skill's primary are both dexterity, so it
// rolls the same stat twice.)
const (
	channelDefenceSkill     = "rhetoric"
	channelDefenceStat      = "willpower" // defy's own stat
	channelDefenceSkillStat = "charisma"  // rhetoric's primary
)

// channelDefenceProgressionCharacters builds a FRESH defender, at the rank
// characters.New() ships (1) rather than the rhetoric 30 that
// defenceAdmissionCharacters' defender carries. It deliberately does NOT reuse
// that fixture: rhetoric 30 is far enough up the decay curve that the
// progression chance is well under 1.0 and every assertion below would become a
// coin flip. The two preconditions assert the certainty directly rather than
// inferring it from the rank.
func channelDefenceProgressionCharacters(t *testing.T, userId int) (*characters.Character, *characters.Character) {
	t.Helper()
	attacker := characters.New()
	attacker.Stats.Charisma.Base = 100
	attacker.Stats.Charisma.Recalculate()

	defender := characters.New()
	defender.Stats.Willpower.Base = 100
	defender.Stats.Willpower.Recalculate()
	defender.SetUserId(userId)

	if got := defender.ProgressionChanceForSkill(channelDefenceSkill, 1.0); got < 1.0 {
		t.Fatalf("precondition: a full-weight %s roll has chance %v, not the pinned certainty of 1.0", channelDefenceSkill, got)
	}
	for _, stat := range []string{channelDefenceStat, channelDefenceSkillStat} {
		if got := defender.ProgressionChanceForStat(stat, 1.0); got < 1.0 {
			t.Fatalf("precondition: a full-weight %s roll has chance %v, not the pinned certainty of 1.0", stat, got)
		}
	}
	return attacker, defender
}

// pinChannelContestResult swaps the contest runner behind ResolveChannelAttack
// for one that reports a fixed outcome against whatever entry list the seam
// built, so the progression assertions are deterministic.
func pinChannelContestResult(t *testing.T, attackerWon bool) {
	t.Helper()
	margin := -30.0
	if attackerWon {
		margin = 30.0
	}
	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			name := ""
			if len(entries) > 0 {
				name = entries[0].Name
			}
			return contest.Result{
				Success:     attackerWon,
				Contested:   true,
				Winner:      name,
				Margin:      margin,
				AttackRoll:  dice.RollResult{StdDev: 10},
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	t.Cleanup(restore)
}

// A LOST channel defence is awarded at ProgressionFailureFraction, and awarded
// exactly ONCE.
//
// Pinned to fraction 0 the "advanced nothing" half is EXACT: the chance
// short-circuits before any roll. The use counters, which are tracked
// unconditionally, prove the award still FIRED (a loss is a reduced award, not
// a skipped one) and that it fired only once.
func TestResolveChannelAttack_ALostDefenceAwardsAtTheFractionExactlyOnce(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.0)
	events.DrainQueuedSkillUsedForTest(0)

	const userId = 81
	attacker, defender := channelDefenceProgressionCharacters(t, userId)
	pinChannelContestResult(t, true) // the ATTACKER won, so the defence lost

	beforeSkill := defender.GetSkillLevel(skills.SkillTag(channelDefenceSkill))
	beforeStat := defender.GetStatTraining(channelDefenceStat)

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)
	if out.DefenceType != characters.DefenseDefy {
		t.Fatalf("precondition: the contest resolved on %q, want %q", out.DefenceType, characters.DefenseDefy)
	}

	if got := defender.GetSkillLevel(skills.SkillTag(channelDefenceSkill)) - beforeSkill; got != 0 {
		t.Errorf("%s advanced by %d at failure fraction 0, want 0; the lost defence was awarded at full weight", channelDefenceSkill, got)
	}
	if got := defender.GetStatTraining(channelDefenceStat) - beforeStat; got != 0 {
		t.Errorf("%s training advanced by %d at failure fraction 0, want 0", channelDefenceStat, got)
	}
	if got := defender.GetSkillUseCount(channelDefenceSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; a lost channel defence must fire exactly one reduced-weight award", channelDefenceSkill, got)
	}
	if got := defender.GetStatUseCount(channelDefenceStat); got != 1 {
		t.Errorf("%s use count = %d, want 1 (defy's own stat); anything higher means the award fired twice", channelDefenceStat, got)
	}
	if got := defender.GetStatUseCount(channelDefenceSkillStat); got != 1 {
		t.Errorf("%s use count = %d, want 1 (rhetoric's primary); anything higher means the award fired twice", channelDefenceSkillStat, got)
	}
	if got := events.DrainQueuedSkillUsedForTest(userId); len(got) != 0 {
		t.Errorf("a LOST channel defence queued %d SkillUsed events, want 0", len(got))
	}
}

// The no-regression half: a WON channel defence still awards at full weight and
// still emits SkillUsed. Pinned to fraction 0 so nothing on the loss path could
// produce this result by accident.
func TestResolveChannelAttack_AWonDefenceStillAwardsFullWeight(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.0)
	events.DrainQueuedSkillUsedForTest(0)

	const userId = 82
	attacker, defender := channelDefenceProgressionCharacters(t, userId)
	pinChannelContestResult(t, false) // the DEFENCE won

	beforeSkill := defender.GetSkillLevel(skills.SkillTag(channelDefenceSkill))
	beforeStat := defender.GetStatTraining(channelDefenceStat)

	ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)

	if got := defender.GetSkillLevel(skills.SkillTag(channelDefenceSkill)) - beforeSkill; got != 1 {
		t.Errorf("%s advanced by %d on a WON defence, want 1", channelDefenceSkill, got)
	}
	if got := defender.GetStatTraining(channelDefenceStat) - beforeStat; got < 1 {
		t.Errorf("%s training advanced by %d on a WON defence, want at least 1", channelDefenceStat, got)
	}
	queued := events.DrainQueuedSkillUsedForTest(userId)
	if len(queued) != 1 {
		t.Fatalf("a WON channel defence queued %d SkillUsed events, want 1", len(queued))
	}
	if string(queued[0].Skill) != channelDefenceSkill {
		t.Errorf("SkillUsed named %q, want %q", queued[0].Skill, channelDefenceSkill)
	}
}

// ForceCrit is why the award predicate cannot be a bare !res.Success.
//
// A sleeping victim's defence may still take the MARGIN -- the contest reports
// Success false -- but ForceCrit forces the attack win anyway (see the
// DamageMultiplier restore further down resolveChannelAttackWithRunner). The
// defence was quoted, charged and progressed, but it did NOT win, so it must be
// awarded at the failure fraction.
//
// An implementation reading bare !res.Success awards full weight here and fails
// on the training assertion; the pre-Task-9 literal true fails the same way.
func TestResolveChannelAttack_ForcedCritIsALostDefenceEvenThoughTheContestSaysOtherwise(t *testing.T) {
	pinCertainDefenceProgressionForTest(t, 0.0)
	events.DrainQueuedSkillUsedForTest(0)

	const userId = 83
	attacker, defender := channelDefenceProgressionCharacters(t, userId)
	pinChannelContestResult(t, false) // the contest says the DEFENCE took the margin

	beforeSkill := defender.GetSkillLevel(skills.SkillTag(channelDefenceSkill))
	beforeStat := defender.GetStatTraining(channelDefenceStat)

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100, ForceCrit: true}, attacker, defender)

	if !out.AttackerCrit {
		t.Fatalf("precondition: ForceCrit did not produce an attacker crit; the fixture is not exercising the forced-win path")
	}
	if out.Defended {
		t.Fatalf("precondition: the forced crit still reported Defended; the fixture is not exercising the forced-win path")
	}

	if got := defender.GetSkillLevel(skills.SkillTag(channelDefenceSkill)) - beforeSkill; got != 0 {
		t.Errorf("%s advanced by %d against a FORCED crit at failure fraction 0, want 0; the predicate is reading bare !res.Success", channelDefenceSkill, got)
	}
	if got := defender.GetStatTraining(channelDefenceStat) - beforeStat; got != 0 {
		t.Errorf("%s training advanced by %d against a FORCED crit at failure fraction 0, want 0", channelDefenceStat, got)
	}
	if got := defender.GetSkillUseCount(channelDefenceSkill); got != 1 {
		t.Errorf("%s use count = %d, want 1; the defence still rolls, at reduced weight", channelDefenceSkill, got)
	}
	if got := events.DrainQueuedSkillUsedForTest(userId); len(got) != 0 {
		t.Errorf("a FORCED-crit channel defence queued %d SkillUsed events, want 0", len(got))
	}
}
