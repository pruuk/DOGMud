package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// wireFleeCancellationMessage closes an admitted flee when combat terminates
// before the next round can resolve it. Target death normally clears Aggro and
// removes the player from NewRound_DoCombat before handlePlayerFlee can run, so
// the CombatPhase transition is the only reliable point for the terminal line.
func wireFleeCancellationMessage(c *characters.Character) {
	c.CombatPhase.Inner().AfterTransition("player_flee_terminal_cancellation",
		func(from, to combatphase.State, r state.TransitionReason) {
			if from != combatphase.Disengaging || to != combatphase.Idle ||
				r.Trigger == combatphase.TriggerFleeSuccess ||
				r.Trigger == combatphase.TriggerSelfDied {
				return
			}
			u := users.GetByUserId(c.GetUserId())
			if u == nil || u.Character != c || !usercommands.CancelFleeAdmission(u) {
				return
			}
			u.SendText(messaging.CategorySystem, `The fight ends before you need to flee.`)
		})
}

func init() {
	characters.OnCharacterCreated(wireFleeCancellationMessage)
}
