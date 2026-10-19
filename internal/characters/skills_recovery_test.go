package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
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

// pinAutoStandRounds pins ProneAutoStandRounds for one test. Tests read the
// Go default, never the shipped config.yaml, so every auto-stand test states
// the value it relies on rather than inheriting it.
func pinAutoStandRounds(t *testing.T, rounds int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ProneAutoStandRounds = configs.ConfigInt(rounds)
	configs.SetConfigForTest(t, cfg)
}

// #398: a character who keeps losing the recovery contest stands on the Nth
// round tick of the knockdown, not before, and the forced stand reports
// (true, true) so the caller prints the won-recovery lines.
func TestAttemptRecovery_AutoStandAfterExactlyNLostRounds(t *testing.T) {
	pinAutoStandRounds(t, 5)
	c := newProneCharForTest(t, 0)
	lose := func() bool { return false }

	for round := 1; round < 5; round++ {
		if attempted, success := c.AttemptRecovery(lose); !attempted || success {
			t.Fatalf("round %d: lost contest should be attempted=true success=false, got %v/%v", round, attempted, success)
		}
		if !c.IsProne() {
			t.Fatalf("round %d: stood up before ProneAutoStandRounds (5) was reached", round)
		}
	}
	before := c.GetSkillUseCount(string(skills.UnarmedCombat))
	if attempted, success := c.AttemptRecovery(lose); !attempted || !success {
		t.Fatalf("round 5: auto-stand should report attempted=true success=true, got %v/%v", attempted, success)
	}
	if !c.IsStanding() {
		t.Fatal("round 5: must be standing once ProneAutoStandRounds is reached, whatever the contest says")
	}
	if got := c.GetSkillUseCount(string(skills.UnarmedCombat)); got != before {
		t.Fatalf("the forced stand runs no contest and must award nothing: %d -> %d", before, got)
	}
}

// Supine counts the same way, and the guard overrides the MinRecoveryRounds
// window too: a stomp loop is the same death spiral as a lost-roll loop.
func TestAttemptRecovery_AutoStandOverridesMinRecoveryWindow(t *testing.T) {
	pinAutoStandRounds(t, 3)
	c := New()
	c.Validate()
	if err := c.Position.TransitionToSupine(
		position.SupineData{MinRecoveryRounds: 50},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceBackward},
	); err != nil {
		t.Fatalf("supine transition: %v", err)
	}
	for round := 1; round < 3; round++ {
		c.AttemptRecovery(nil)
		if !c.IsSupine() {
			t.Fatalf("round %d: stood up before the guard fired", round)
		}
	}
	if attempted, success := c.AttemptRecovery(nil); !attempted || !success || !c.IsStanding() {
		t.Fatalf("round 3: guard must stand a supine character inside MinRecoveryRounds, got %v/%v standing=%v",
			attempted, success, c.IsStanding())
	}
}

// 0 is the off-switch: a hopeless character stays down.
func TestAttemptRecovery_AutoStandZeroDisables(t *testing.T) {
	pinAutoStandRounds(t, 0)
	c := newProneCharForTest(t, 0)
	for round := 1; round <= 20; round++ {
		c.AttemptRecovery(func() bool { return false })
	}
	if !c.IsProne() {
		t.Fatal("ProneAutoStandRounds 0 must disable the guard")
	}
}

// A pinned grapple is not Prone or Supine: recovery never reaches it, so the
// guard cannot stand a character out of a hold. Escaping is the grapple
// system's job.
func TestAttemptRecovery_AutoStandSkipsGrapple(t *testing.T) {
	pinAutoStandRounds(t, 2)
	c := newProneCharForTest(t, 0)
	if err := c.Position.TransitionToMount(
		position.GrappleData{Partner: state.ActorRef{MobInstanceId: 1}},
		state.TransitionReason{Trigger: position.TriggerMountProneTarget},
	); err != nil {
		t.Fatalf("mount transition: %v", err)
	}
	for round := 1; round <= 10; round++ {
		if attempted, _ := c.AttemptRecovery(nil); attempted {
			t.Fatalf("round %d: a grappled character must not make a recovery attempt", round)
		}
	}
	if !c.Position.IsMount() {
		t.Fatal("the guard must never stand a character out of a pinned grapple")
	}
}

// The count belongs to one knockdown: standing and being knocked down again
// starts it over.
func TestAttemptRecovery_AutoStandCountsPerKnockdown(t *testing.T) {
	pinAutoStandRounds(t, 3)
	c := newProneCharForTest(t, 0)
	lose := func() bool { return false }
	c.AttemptRecovery(lose)
	c.AttemptRecovery(lose)
	if err := c.Position.TransitionToStanding(state.TransitionReason{Trigger: position.TriggerStandCommand}); err != nil {
		t.Fatalf("stand: %v", err)
	}
	if err := c.Position.TransitionToProne(position.ProneData{},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward}); err != nil {
		t.Fatalf("second knockdown: %v", err)
	}
	c.AttemptRecovery(lose)
	c.AttemptRecovery(lose)
	if !c.IsProne() {
		t.Fatal("a fresh knockdown must restart the count, not carry the earlier rounds")
	}
	c.AttemptRecovery(lose)
	if !c.IsStanding() {
		t.Fatal("third round of the fresh knockdown must stand")
	}
}
