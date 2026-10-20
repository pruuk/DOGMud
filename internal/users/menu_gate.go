package users

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/state/control"
)

// quitConditionId is condition 0, "Meditating": the quit timer that `quit`
// starts (conditions/0-meditating.yaml).
const quitConditionId = 0

// menuGate reports the first rule that stops a game menu opening or closes
// an open one, or menu.CloseNone. Owner rules 2026-10-20. Built from the same
// sources as the shared busy check filed as #640; move onto it when it lands.
func menuGate(c *characters.Character) menu.CloseReason {
	switch {
	case c == nil:
		return menu.CloseNone
	case !c.IsAlive():
		return menu.CloseDead
	case c.IsInCombat():
		return menu.CloseCombat
	case c.IsActing():
		return menu.CloseBusy
	case c.HasConditionFlag(conditions.Sleeping):
		return menu.CloseAsleep
	case c.HasCondition(quitConditionId):
		return menu.CloseQuitting
	case !c.IsStanding():
		return menu.CloseOffFeet
	case c.IsBeingControlled(), c.Control != nil && c.Control.State() == control.BecomingControlled:
		return menu.CloseControlled
	}
	return menu.CloseNone
}
