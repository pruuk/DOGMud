package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// newProneCharForTest: validated character, prone, with the given
// MinRecoveryRounds. Validate() attaches the Position FSM.
func newProneCharForTest(t *testing.T, minRounds int) *Character {
	t.Helper()
	c := New()
	c.Validate()
	if err := c.Position.TransitionToProne(
		position.ProneData{MinRecoveryRounds: minRounds},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward},
	); err != nil {
		t.Fatalf("prone transition: %v", err)
	}
	return c
}

func TestAttemptRecovery_NoOpponentAutoStands(t *testing.T) {
	c := newProneCharForTest(t, 0)
	before := c.GetSkillUseCount(string(skills.UnarmedCombat))
	attempted, success := c.AttemptRecovery(nil)
	if !attempted || !success {
		t.Fatalf("free recovery: attempted=%v success=%v, want true/true", attempted, success)
	}
	if !c.IsStanding() {
		t.Fatal("should be standing after a free recovery")
	}
	if c.GetSkillUseCount(string(skills.UnarmedCombat)) != before {
		t.Fatal("a free stand is not a contest and must fire no progression")
	}
}

func TestAttemptRecovery_ContestOutcomesAndProgression(t *testing.T) {
	c := newProneCharForTest(t, 0)
	before := c.GetSkillUseCount(string(skills.UnarmedCombat))
	if attempted, success := c.AttemptRecovery(func() bool { return false }); !attempted || success {
		t.Fatalf("lost contest: attempted=%v success=%v, want true/false", attempted, success)
	}
	if c.IsStanding() {
		t.Fatal("must stay down on a lost contest")
	}
	// U10b-1 Task 18c inverted this assertion. A LOST recovery used to fire
	// nothing at all, so a character who failed to scramble to their feet
	// learned nothing from the attempt -- and failing to stand is exactly the
	// situation that teaches you to. It now awards at
	// ProgressionFailureFraction, which the use counter records like any other
	// award.
	if got := c.GetSkillUseCount(string(skills.UnarmedCombat)); got != before+1 {
		t.Fatalf("a LOST recovery must still fire ONE unarmed event, at the failure fraction: %d -> %d", before, got)
	}
	if attempted, success := c.AttemptRecovery(func() bool { return true }); !attempted || !success {
		t.Fatalf("won contest: attempted=%v success=%v, want true/true", attempted, success)
	}
	if got := c.GetSkillUseCount(string(skills.UnarmedCombat)); got != before+2 {
		t.Fatalf("a WON recovery fires exactly one more unarmed event: %d -> %d", before+1, got)
	}
}

func TestAttemptRecovery_MinRoundsGateBeforeContest(t *testing.T) {
	c := newProneCharForTest(t, 1)
	if attempted, success := c.AttemptRecovery(nil); attempted || success {
		t.Fatalf("still in min-recovery: attempted=%v success=%v, want false/false", attempted, success)
	}
	if c.IsStanding() {
		t.Fatal("must stay down while MinRecoveryRounds is still positive")
	}
	if attempted, success := c.AttemptRecovery(nil); !attempted || !success {
		t.Fatalf("after min rounds consumed: attempted=%v success=%v, want true/true", attempted, success)
	}
	if !c.IsStanding() {
		t.Fatal("should be standing once min rounds are consumed and nobody holds it down")
	}
}
