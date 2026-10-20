package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

func goCrafting(t *testing.T, c *Character) {
	t.Helper()
	if err := c.Activity.Inner().TransitionTo(activity.Crafting, state.TransitionReason{}); err != nil {
		t.Fatalf("transition: %v", err)
	}
}

func TestBusyObservers_NotifyTheUserOnTransition(t *testing.T) {
	var got []int
	old := BusyStateChanged
	BusyStateChanged = func(userId int) { got = append(got, userId) }
	defer func() { BusyStateChanged = old }()

	c := New()
	c.SetUserId(7)
	c.WireBusyObservers()
	goCrafting(t, c)
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("notified %v, want [7]", got)
	}
}

func TestBusyObservers_WireTwiceRegistersOnce(t *testing.T) {
	n := 0
	old := BusyStateChanged
	BusyStateChanged = func(int) { n++ }
	defer func() { BusyStateChanged = old }()

	c := New()
	c.SetUserId(7)
	c.WireBusyObservers()
	c.WireBusyObservers()
	goCrafting(t, c)
	if n != 1 {
		t.Fatalf("notified %d times, want 1", n)
	}
}

func TestBusyObservers_ReplacedMachineIsRewired(t *testing.T) {
	n := 0
	old := BusyStateChanged
	BusyStateChanged = func(int) { n++ }
	defer func() { BusyStateChanged = old }()

	c := New()
	c.SetUserId(7)
	c.WireBusyObservers()
	c.Activity = activity.NewMachine()
	c.WireBusyObservers()
	goCrafting(t, c)
	if n != 1 {
		t.Fatalf("notified %d times, want 1", n)
	}
}

func TestBusyObservers_MobCharacterIsSilent(t *testing.T) {
	n := 0
	old := BusyStateChanged
	BusyStateChanged = func(int) { n++ }
	defer func() { BusyStateChanged = old }()

	c := New() // userId 0
	c.WireBusyObservers()
	goCrafting(t, c)
	if n != 0 {
		t.Fatalf("mob character notified %d times", n)
	}
}
