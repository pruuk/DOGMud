package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
)

// wireAwarenessFromCombatPhase registers cascade handlers on
// each character that bridge Combat Phase transitions to
// Awareness state changes, and mirror Awareness state to condition #9.
//
// Two cascade directions:
//
//  1. Combat Phase → Awareness:
//     - Idle → Engaging: Hidden → Revealing
//
//  2. Awareness → Condition #9:
//     - Entering Hidden: AddCondition(9, false)
//     - Leaving Hidden (to Revealing or Visible): CancelConditionsWithFlag(Hidden)
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

	// 2. Awareness state → condition #9 mirror.
	c.Awareness.Inner().AfterTransition("awareness_condition_mirror",
		func(from, to awareness.State, r state.TransitionReason) {
			switch {
			case to == awareness.Hidden:
				// A shroud hide carries record 31, not 9 (#444): its end
				// is told once, by 31's own end line, and a timed 31 must
				// not pin a permanent 9 that outlives it. Its data is
				// stored before this cascade runs (ResolveConcealmentAs).
				if d, ok := c.Awareness.HiddenData(); ok && d.Source == awareness.HideShroud {
					return
				}
				// Apply condition #9 as permanent — the awareness state
				// machine owns lifecycle. Condition #9 has no triggerrate
				// (dropped in d282c4ab), so TriggerCount=0 would
				// otherwise mark TriggersLeft=0 and the condition would
				// prune on the next NewTurn (firing "You no longer
				// feel sneaky." while the awareness state still
				// reports Hidden — split source of truth).
				// The Revealing/Visible transitions cancel it via
				// CancelConditionsWithFlag(Hidden) below.
				_ = c.AddCondition(9, true)
			case from == awareness.Hidden &&
				(to == awareness.Revealing || to == awareness.Visible):
				// Remove condition #9 via cancel-on-flag mechanism. That
				// cancel also spends a spawn-time hide (conditionids: [9])
				// before its Validate(true), so the record is not revived
				// on the now visible mob (sight gates playtest fixes, F5).
				c.CancelConditionsWithFlag(conditions.Hidden)
				// The sneak is over however it ended (combat, a cancel, a
				// spot, a shroud running out): usercommands.Go reads this
				// flag and would otherwise move a visible player as a
				// sneaker. Sneak sets it again on its next success (#444).
				c.SetMiscData(`sneaking`, nil)
			}
		})
}

func init() {
	characters.OnCharacterCreated(wireAwarenessFromCombatPhase)
}
