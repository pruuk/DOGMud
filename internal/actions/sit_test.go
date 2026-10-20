package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// Sit and StandFromSitting (#637), shared by players and mobs. The fake actor
// captures the actor's own lines; the room has no players, so the observer
// line goes nowhere here (the hooks tests cover the room seat).

func newSitActor(t *testing.T, isPlayer bool) *sleepFakeActor {
	t.Helper()
	a := newSleepActor(t, false, isPlayer)
	if err := a.char.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return a
}

func lastLine(a *sleepFakeActor) string {
	if len(a.sent) == 0 {
		return ""
	}
	return a.sent[len(a.sent)-1]
}

func TestSit_SeatsAPlayerAndAMob(t *testing.T) {
	for _, isPlayer := range []bool{true, false} {
		a := newSitActor(t, isPlayer)
		if !Sit(a) {
			t.Fatalf("player=%v: Sit refused a standing actor", isPlayer)
		}
		if !a.char.IsSitting() {
			t.Fatalf("player=%v: state %v after Sit, want Sitting", isPlayer, a.char.Position.State())
		}
		if got := lastLine(a); got != "You sit down." {
			t.Errorf("player=%v: actor line %q, want %q", isPlayer, got, "You sit down.")
		}
	}
}

func TestStandFromSitting_StandsForFree(t *testing.T) {
	a := newSitActor(t, true)
	Sit(a)
	stamina := a.char.Stamina
	if !StandFromSitting(a) {
		t.Fatal("StandFromSitting refused a sitter")
	}
	if !a.char.IsStanding() {
		t.Fatalf("state %v after StandFromSitting, want Standing", a.char.Position.State())
	}
	if got := lastLine(a); got != "You stand up." {
		t.Errorf("actor line %q, want %q", got, "You stand up.")
	}
	if a.char.Stamina != stamina {
		t.Errorf("standing from a seat cost stamina: %d -> %d", stamina, a.char.Stamina)
	}
	n := len(a.sent)
	if StandFromSitting(a) {
		t.Error("StandFromSitting on a stander reported a stand")
	}
	if len(a.sent) != n {
		t.Errorf("StandFromSitting on a stander said something: %q", a.sent[n:])
	}
}

func TestSit_Refusals(t *testing.T) {
	cleanup := seedSleepCondition(t)
	defer cleanup()

	cases := []struct {
		name  string
		setup func(a *sleepFakeActor)
		line  string
		want  position.State
	}{
		{"already sitting", func(a *sleepFakeActor) {
			_ = a.char.Position.TransitionToSitting(state.TransitionReason{Trigger: position.TriggerSitCommand})
		}, SitRefuseSitting, position.Sitting},
		{"asleep", func(a *sleepFakeActor) {
			_ = a.char.AddCondition(15, false)
		}, SitRefuseAsleep, position.Standing},
		{"in combat", func(a *sleepFakeActor) {
			a.char.SetAggro(0, 9001, 0)
		}, refusal.WhileFightingText("sit"), position.Standing},
		{"grappled", func(a *sleepFakeActor) {
			_ = a.char.Position.TransitionToClinch(position.GrappleData{Partner: state.ActorRef{UserId: 2}},
				state.TransitionReason{Trigger: position.TriggerGrappleEntry})
		}, SitRefuseGrappled, position.Clinch},
		{"prone", func(a *sleepFakeActor) {
			_ = a.char.Position.TransitionToProne(position.ProneData{},
				state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward})
		}, SitRefuseOnTheFloor, position.Prone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newSitActor(t, true)
			tc.setup(a)
			if !a.char.HasConditionFlag(conditions.Sleeping) && tc.name == "asleep" {
				t.Fatal("fixture: the sleeper is not asleep")
			}
			if Sit(a) {
				t.Fatalf("Sit accepted a character who is %s", tc.name)
			}
			if got := lastLine(a); got != tc.line {
				t.Errorf("refusal %q, want %q", got, tc.line)
			}
			if got := a.char.Position.State(); got != tc.want {
				t.Errorf("a refused sit moved the position to %v, want %v", got, tc.want)
			}
		})
	}
}
