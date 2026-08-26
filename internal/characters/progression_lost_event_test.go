package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/progression"
)

// U10b-1 makes progression fire Best-of: a resolved action awards on a LOSS
// too, at ProgressionFailureFraction. That roughly DOUBLES how often
// OnSkillUseScaled is called, so its two side effects that never scaled have
// to be dealt with, each by its own mechanism:
//
//	mutation cluster drift -> SCALED by bonusMultiplier
//	the SkillUsed quest event -> GATED on the loss
//
// The tests below pin both, and pin the reason they are two mechanisms and
// not one.

// Mutation acquisition is a major character-identity system tuned for
// sustained play. If drift stayed at a flat MutationAffinityPerSkillUse while
// the call rate doubled, mutations would arrive roughly twice as fast.
//
// The non-zero guard on the win is load-bearing: without it an implementation
// that granted zero affinity in BOTH cases would satisfy "loss < win"
// trivially.
func TestOnSkillUseScaled_LossDriftsClusterAffinityLessThanAWin(t *testing.T) {
	pinConfigForTest(t)

	win := &Character{Name: "Winner"}
	win.OnSkillUseScaled("spellcasting", 0, 1.0, false)

	loss := &Character{Name: "Loser"}
	loss.OnSkillUseScaled("spellcasting", 0, 0.35, true)

	gotWin := win.ClusterAffinity["ethereal"]
	gotLoss := loss.ClusterAffinity["ethereal"]

	if gotWin <= 0 {
		t.Fatalf("win drifted %v ethereal affinity, want > 0 (a zero-vs-zero comparison proves nothing)", gotWin)
	}
	if !(gotLoss < gotWin) {
		t.Fatalf("loss drifted %v ethereal affinity, want strictly less than the win's %v", gotLoss, gotWin)
	}
}

// A "use this skill N times" quest must not become "fail at it N times".
// Both halves are asserted: an "emits nothing on a loss" assertion alone
// passes against a function that emits nothing ever.
func TestOnSkillUseScaled_LossEmitsNoSkillUsedButAWinDoes(t *testing.T) {
	pinConfigForTest(t)
	events.DrainQueuedSkillUsedForTest(0) // clear anything a prior test left

	c := &Character{Name: "T"}

	c.OnSkillUseScaled("spellcasting", 7, 1.0, false)
	if got := events.DrainQueuedSkillUsedForTest(7); len(got) != 1 {
		t.Fatalf("a winning use emitted %d SkillUsed events, want 1", len(got))
	}

	c.OnSkillUseScaled("spellcasting", 7, 0.35, true)
	if got := events.DrainQueuedSkillUsedForTest(7); len(got) != 0 {
		t.Fatalf("a losing use emitted %d SkillUsed events, want 0 -- skill_use quests would tick on failure", len(got))
	}
}

// THE regression guard for this task. SelfCastProgressionMultiplier ships at
// 0.5, so a self-buff cast is a WINNING action that arrives with a sub-1.0
// multiplier. Gating the quest event on "bonusMultiplier < 1.0" instead of on
// the loss silently stops every self-buff cast from ticking skill_use quests,
// with no error anywhere. This test is what goes red if anyone does that.
func TestOnSkillUseScaled_WinningSubOneMultiplierStillEmitsSkillUsed(t *testing.T) {
	pinConfigForTest(t)
	events.DrainQueuedSkillUsedForTest(0)

	c := &Character{Name: "T"}
	c.OnSkillUseScaled("spellcasting", 9, 0.5, false)

	if got := events.DrainQueuedSkillUsedForTest(9); len(got) != 1 {
		t.Fatalf("a winning self-buff-style cast (multiplier 0.5) emitted %d SkillUsed events, want 1", len(got))
	}
}

// Event.Lost has to actually reach OnSkillUseScaled through ApplyProgression,
// which is the seam Tasks 5-7 will start setting.
func TestApplyProgression_ThreadsEventLostThroughToTheSkillPath(t *testing.T) {
	pinConfigForTest(t)
	events.DrainQueuedSkillUsedForTest(0)

	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Class: progression.ClassOrdinary, Multiplier: 0.35, Lost: true,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 11, 1)

	if got := events.DrainQueuedSkillUsedForTest(11); len(got) != 0 {
		t.Fatalf("a lost ordinary event emitted %d SkillUsed events, want 0", len(got))
	}
}
