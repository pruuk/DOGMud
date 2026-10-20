package characters

import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// BusyStateChanged, when set, is told a player's user id whenever one of the
// machines that decide "busy" transitions. internal/users sets it at init so
// an open menu closes the moment its player becomes busy (#375). It is the
// push half of the menu gate; the shared busy check (#640) replaces the gate.
var BusyStateChanged func(userId int)

// busyWiring remembers which machine instances already carry the observer,
// so wiring twice registers once and a replaced machine is wired again
// (state.Machine has no unsubscribe).
type busyWiring struct {
	combat   *combatphase.Machine
	life     *life.Machine
	activity *activity.Machine
	control  *control.Machine
}

// Position is deliberately not observed: off their feet is not a menu gate
// (owner ruling 2026-10-20). A grapple moves the Control machine too.

// WireBusyObservers registers the busy observer on each present machine not
// yet wired. Safe to call repeatedly; Validate calls it (and New goes through
// Validate). Mob characters have userId 0 and notify nothing.
func (c *Character) WireBusyObservers() {
	notify := func() {
		if BusyStateChanged != nil && c.userId > 0 {
			BusyStateChanged(c.userId)
		}
	}
	const name = "menu_busy_gate"
	if c.CombatPhase != nil && c.busyWired.combat != c.CombatPhase {
		c.busyWired.combat = c.CombatPhase
		c.CombatPhase.RegisterObserver(name, func(_, _ combatphase.State, _ state.TransitionReason) { notify() })
	}
	if c.Life != nil && c.busyWired.life != c.Life {
		c.busyWired.life = c.Life
		c.Life.RegisterObserver(name, func(_, _ life.State, _ state.TransitionReason) { notify() })
	}
	if c.Activity != nil && c.busyWired.activity != c.Activity {
		c.busyWired.activity = c.Activity
		c.Activity.RegisterObserver(name, func(_, _ activity.State, _ state.TransitionReason) { notify() })
	}
	if c.Control != nil && c.busyWired.control != c.Control {
		c.busyWired.control = c.Control
		c.Control.RegisterObserver(name, func(_, _ control.State, _ state.TransitionReason) { notify() })
	}
}
