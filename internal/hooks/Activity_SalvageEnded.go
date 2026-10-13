package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

// wireSalvageEndedNarration tells a player salvager that being hurt stopped
// the work (#514). It is the salvage twin of the crafter line in
// wireCraftEndedNarration. Only a damage interrupt speaks here: walking away
// is already told by usercommands/go.go, and a salvage has no room lines yet.
// A mob has no connection to tell, and a dying character is left to the death
// narration. The line waits for the round-end flush
// (work_interrupt_narration.go, owner ruling 2026-10-13) so it reads after the
// hit line that caused it.
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
		queueWorkInterrupt(pendingWorkInterrupt{
			userId:  uid,
			roomId:  c.RoomId,
			salvage: true,
		})
	})
}

func init() {
	characters.OnCharacterCreated(wireSalvageEndedNarration)
}
