package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// wireSalvageEndedNarration tells a player salvager that being hurt stopped
// the work (#514). It is the salvage twin of the crafter line in
// wireCraftEndedNarration. Only a damage interrupt speaks here: walking away
// is already told by usercommands/go.go, and a salvage has no room lines yet.
// A mob has no connection to tell, and a dying character is left to the death
// narration.
func wireSalvageEndedNarration(c *characters.Character) {
	if c.Activity == nil {
		return
	}
	c.Activity.OnSalvageEnded("salvage_ended_damage_line", func(_ activity.SalvagingData, r state.TransitionReason) {
		if r.Trigger != activity.TriggerDamageInterrupt {
			return
		}
		if c.Health < 1 || !c.IsAlive() {
			return
		}
		uid := c.ActorRef().UserId
		if uid <= 0 {
			return
		}
		if u := users.GetByUserId(uid); u != nil {
			u.SendText(messaging.CategorySystem, `<ansi fg="red">The pain interrupts your salvaging.</ansi>`)
		}
	})
}

func init() {
	characters.OnCharacterCreated(wireSalvageEndedNarration)
}
