package position_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// Sitting (#637) is a chosen rest posture: in from Standing only, out to
// Standing, or flat on the floor when a knockdown lands on a sitter.

func sitReason() state.TransitionReason {
	return state.TransitionReason{Trigger: position.TriggerSitCommand}
}

func TestSitting_StandingToSittingAndBack(t *testing.T) {
	m := position.NewMachine()
	if err := m.TransitionToSitting(sitReason()); err != nil {
		t.Fatalf("Standing -> Sitting refused: %v", err)
	}
	if !m.IsSitting() || m.IsStanding() {
		t.Fatalf("state = %v, want Sitting", m.State())
	}
	if m.IsOnFloor() || m.IsGrappling() || m.IsProne() || m.IsSupine() {
		t.Errorf("Sitting must not read as a knockdown or a grapple")
	}
	if err := m.TransitionToStanding(state.TransitionReason{Trigger: position.TriggerStandCommand}); err != nil {
		t.Fatalf("Sitting -> Standing refused: %v", err)
	}
	if !m.IsStanding() {
		t.Errorf("state = %v, want Standing", m.State())
	}
}

func TestSitting_KnockdownLandsOnASitter(t *testing.T) {
	m := position.NewMachine()
	_ = m.TransitionToSitting(sitReason())
	if err := m.TransitionToSupine(position.SupineData{MinRecoveryRounds: 1},
		state.TransitionReason{Trigger: position.TriggerKnockdownSpell}); err != nil {
		t.Fatalf("Sitting -> Supine refused: %v", err)
	}
	m2 := position.NewMachine()
	_ = m2.TransitionToSitting(sitReason())
	if err := m2.TransitionToProne(position.ProneData{},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward}); err != nil {
		t.Fatalf("Sitting -> Prone refused: %v", err)
	}
}

func TestSitting_NoEdgeFromTheFloorOrAGrapple(t *testing.T) {
	prone := position.NewMachine()
	_ = prone.TransitionToProne(position.ProneData{}, state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward})
	if err := prone.TransitionToSitting(sitReason()); err == nil {
		t.Errorf("Prone -> Sitting accepted; a downed character must stand first")
	}
	clinch := position.NewMachine()
	_ = clinch.TransitionToClinch(position.GrappleData{Partner: state.ActorRef{UserId: 1}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry})
	if err := clinch.TransitionToSitting(sitReason()); err == nil {
		t.Errorf("Clinch -> Sitting accepted")
	}
	sitting := position.NewMachine()
	_ = sitting.TransitionToSitting(sitReason())
	if err := sitting.TransitionToSitting(sitReason()); err == nil {
		t.Errorf("Sitting -> Sitting accepted; the command refuses an already seated character itself")
	}
	if err := sitting.TransitionToClinch(position.GrappleData{Partner: state.ActorRef{UserId: 1}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry}); err == nil {
		t.Errorf("Sitting -> Clinch accepted; a grapple stands its target first")
	}
}

// Sitting is not a knockdown: the #398 auto-stand counter does not run for it.
func TestSitting_CountRoundDownIgnoresSitting(t *testing.T) {
	m := position.NewMachine()
	_ = m.TransitionToSitting(sitReason())
	for i := 0; i < 10; i++ {
		if got := m.CountRoundDown(); got != 0 {
			t.Fatalf("CountRoundDown on a sitter = %d, want 0", got)
		}
	}
}

func TestSitting_CombatTablesAreNeutral(t *testing.T) {
	if got := position.TargetSideHitModifier(position.Sitting, control.Neutral); got != 1.0 {
		t.Errorf("TargetSideHitModifier(Sitting) = %v, want 1.0", got)
	}
	if got := position.AttackerSelfHitModifier(position.Sitting, control.Neutral); got != 1.0 {
		t.Errorf("AttackerSelfHitModifier(Sitting) = %v, want 1.0", got)
	}
	if got := position.PositionDisruptionDmgEquiv(position.Sitting, control.Neutral); got != 0 {
		t.Errorf("PositionDisruptionDmgEquiv(Sitting) = %v, want 0", got)
	}
}
