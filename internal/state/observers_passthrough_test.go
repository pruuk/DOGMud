package state_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

func TestRegisterObserver_ActivityFiresOnTransition(t *testing.T) {
	m := activity.NewMachine()
	fired := 0
	m.RegisterObserver("probe", func(from, to activity.State, r state.TransitionReason) { fired++ })
	if err := m.Inner().TransitionTo(activity.Crafting, state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if fired != 1 {
		t.Fatalf("observer fired %d times, want 1", fired)
	}
}

func TestRegisterObserver_LifeFiresOnTransition(t *testing.T) {
	m := life.NewMachine()
	fired := 0
	m.RegisterObserver("probe", func(from, to life.State, r state.TransitionReason) { fired++ })
	if err := m.Inner().TransitionTo(life.Dead, state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if fired != 1 {
		t.Fatalf("observer fired %d times, want 1", fired)
	}
}

func TestRegisterObserver_PositionFiresOnTransition(t *testing.T) {
	m := position.NewMachine()
	fired := 0
	m.RegisterObserver("probe", func(from, to position.State, r state.TransitionReason) { fired++ })
	if err := m.Inner().TransitionTo(position.Sitting, state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if fired != 1 {
		t.Fatalf("observer fired %d times, want 1", fired)
	}
}

func TestRegisterObserver_CombatPhaseFiresOnTransition(t *testing.T) {
	m := combatphase.NewMachine()
	fired := 0
	m.RegisterObserver("probe", func(from, to combatphase.State, r state.TransitionReason) { fired++ })
	if err := m.Inner().TransitionTo(combatphase.Engaging, state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if fired != 1 {
		t.Fatalf("observer fired %d times, want 1", fired)
	}
}

// Control has no public Inner(); its public API is TransitionToControlling,
// which crosses the LosingControl boundary: Neutral to LosingControl to
// Controlling, so two transitions fire.
func TestRegisterObserver_ControlFiresOnTransition(t *testing.T) {
	m := control.NewMachine()
	fired := 0
	m.RegisterObserver("probe", func(from, to control.State, r state.TransitionReason) { fired++ })
	if err := m.TransitionToControlling(state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if fired != 2 {
		t.Fatalf("observer fired %d times, want 2", fired)
	}
}

func TestRegisterObserver_NilMachineIsANoOp(t *testing.T) {
	var a *activity.Machine
	a.RegisterObserver("probe", func(from, to activity.State, r state.TransitionReason) {})
	var l *life.Machine
	l.RegisterObserver("probe", func(from, to life.State, r state.TransitionReason) {})
	var p *position.Machine
	p.RegisterObserver("probe", func(from, to position.State, r state.TransitionReason) {})
	var c *combatphase.Machine
	c.RegisterObserver("probe", func(from, to combatphase.State, r state.TransitionReason) {})
	var k *control.Machine
	k.RegisterObserver("probe", func(from, to control.State, r state.TransitionReason) {})
}
