package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// wireSitRise keeps Sitting (#637) out of combat, for players and mobs alike.
//
//  1. Combat Phase Idle to Engaging: a sitter who enters combat (attacking,
//     or turning on an attacker) stands, through Character.RiseForCombat.
//     Being attacked before engaging is the other half, RevealForCombat,
//     which every melee round, special move, grapple and taunt calls on its
//     defender.
//  2. Position Sitting to Standing under position.TriggerCombatRise: narrate
//     the rise ("You leap to your feet!"), whichever of the two stood them.
//     The stand command and walking off use TriggerStandCommand and narrate
//     themselves (actions.StandFromSitting).
func wireSitRise(c *characters.Character) {
	c.CombatPhase.Inner().AfterTransition("sit_rise_on_combat",
		func(from, to combatphase.State, r state.TransitionReason) {
			if from != combatphase.Idle || to != combatphase.Engaging {
				return
			}
			c.RiseForCombat()
		})

	c.Position.Inner().AfterTransition("sit_rise_narration",
		func(from, to position.State, r state.TransitionReason) {
			if from != position.Sitting || to != position.Standing ||
				r.Trigger != position.TriggerCombatRise {
				return
			}
			if actor := sitRiseActor(c); actor != nil {
				actions.NarrateCombatRise(actor)
			}
		})
}

// sitRiseActor resolves the character to the Actor that narrates its rise, in
// the room it is in now. Nil when the user or mob instance is gone.
func sitRiseActor(c *characters.Character) actions.Actor {
	room := rooms.LoadRoom(c.RoomId)
	if room == nil {
		return nil
	}
	if uid := c.GetUserId(); uid > 0 {
		if u := users.GetByUserId(uid); u != nil {
			return actions.NewUserActorInRoom(u, room)
		}
		return nil
	}
	if m := mobs.GetInstance(c.MobInstanceId); m != nil {
		return actions.NewMobActorInRoom(m, room)
	}
	return nil
}

func init() {
	characters.OnCharacterCreated(wireSitRise)
}
