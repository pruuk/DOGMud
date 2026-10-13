package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// wireCraftEndedNarration tells the room when a craft ends without finishing
// (messaging M6 row 7, owner ruling R4): cancel, walking away, damage, or any
// later path that leaves Crafting. Completion narrates its own outcome at the
// round tick, and death has its own narration, so both are skipped. One hook
// rather than a call at each cancel site, so a new cancel path cannot be
// silent.
func wireCraftEndedNarration(c *characters.Character) {
	if c.Activity == nil {
		return
	}
	c.Activity.OnCraftEnded("craft_ended_room_line", func(d activity.CraftingData, r state.TransitionReason) {
		if r.Trigger == activity.TriggerCraftComplete || r.Trigger == activity.TriggerDeath {
			return
		}
		// A killing blow that also interrupts the craft sends no stop line
		// (owner ruling 2026-10-13): the death narration covers it. The damage
		// cancel runs before the queued CharacterDied is processed, so the
		// crafter is "dying" (Health < 1 while still Alive, the predicate
		// pools.go documents), not yet Dead. Cover both.
		if c.Health < 1 || !c.IsAlive() {
			return
		}
		// Tell a player crafter they were hurt, as walking away does
		// (usercommands/go.go). "Pain", not "blow": a bleed tick lands here too,
		// with no blow (#515). A mob has no connection to tell. The room line
		// below is sent in addition.
		if r.Trigger == activity.TriggerDamageInterrupt {
			if uid := c.ActorRef().UserId; uid > 0 {
				if u := users.GetByUserId(uid); u != nil {
					u.SendText(messaging.CategorySystem, `<ansi fg="red">The pain interrupts your crafting.</ansi>`)
				}
			}
		}
		recipe := crafting.GetRecipe(d.RecipeId)
		room := rooms.LoadRoom(c.RoomId)
		if recipe == nil || room == nil {
			return
		}
		var crafter actions.Actor
		ref := c.ActorRef()
		switch {
		case ref.UserId > 0:
			if u := users.GetByUserId(ref.UserId); u != nil {
				crafter = actions.NewUserActorInRoom(u, room)
			}
		case ref.MobInstanceId > 0:
			if m := mobs.GetInstance(ref.MobInstanceId); m != nil {
				crafter = actions.NewMobActorInRoom(m, room)
			}
		}
		if crafter == nil {
			return
		}
		actions.NarrateCraftPhase(crafter, recipe, crafting.PhaseCancel, room)
	})
}

func init() {
	characters.OnCharacterCreated(wireCraftEndedNarration)
}
