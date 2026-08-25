package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
)

// wireAwarenessFromCombatPhase registers cascade handlers on
// each character that bridge Combat Phase transitions to
// Awareness state changes, and mirror Awareness state to buff #9.
//
// Two cascade directions:
//
//  1. Combat Phase → Awareness:
//     - Idle → Engaging: Hidden → Revealing
//
//  2. Awareness → Buff #9:
//     - Entering Hidden: AddBuff(9, false)
//     - Leaving Hidden (to Revealing or Visible): CancelBuffsWithFlag(Hidden)
//
// Stealth breaks the instant the ambusher engages — there is no
// surprise round to protect any more. The opening strike keys off
// Aggro.Type, which SetAggro writes before the Combat Phase
// transition that fires this cascade, so revealing here does not
// cost the ambusher their opening strike.
func wireAwarenessFromCombatPhase(c *characters.Character) {
	// 1a. Combat Phase Idle → Engaging cascade.
	c.CombatPhase.Inner().AfterTransition("awareness_combat_cascade",
		func(from, to combatphase.State, r state.TransitionReason) {
			if from != combatphase.Idle || to != combatphase.Engaging {
				return
			}
			if c.Awareness.State() == awareness.Hidden {
				_ = c.Awareness.TransitionToRevealing(state.TransitionReason{
					Trigger: awareness.TriggerCombatEntered,
				})
			}
		})

	// 2. Awareness state → buff #9 mirror.
	c.Awareness.Inner().AfterTransition("awareness_buff_mirror",
		func(from, to awareness.State, r state.TransitionReason) {
			switch {
			case to == awareness.Hidden:
				// Apply buff #9 as permanent — the awareness state
				// machine owns lifecycle. Buff #9 has no triggerrate
				// (dropped in d282c4ab), so TriggerCount=0 would
				// otherwise mark TriggersLeft=0 and the buff would
				// prune on the next NewTurn (firing "You no longer
				// feel sneaky." while the awareness state still
				// reports Hidden — split source of truth).
				// The Revealing/Visible transitions cancel it via
				// CancelBuffsWithFlag(Hidden) below.
				_ = c.AddBuff(9, true)
			case from == awareness.Hidden &&
				(to == awareness.Revealing || to == awareness.Visible):
				// Remove buff #9 via cancel-on-flag mechanism.
				c.CancelBuffsWithFlag(buffs.Hidden)
			}
		})
}

func init() {
	characters.OnCharacterCreated(wireAwarenessFromCombatPhase)
}
