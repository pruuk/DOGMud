package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// mobCanSee reports whether a mob can make out the room it is standing in.
//
// It is the SAME predicate combat already uses: combat/combat.go builds
// combatContext.sourceCanSee from CanSeeSightImpairedOnly, so a mob's decisions
// and its darkness combat penalty cannot disagree about whether it can see.
//
// Two things flow through here for free. Restoring buff 29 works because the
// predicate ends by reading the NightVision flag, and a light carried by ANY
// player or mob lifts the darkness for everyone, because Room.GetVisibility
// adds +1 when someone in the room has buffs.EmitsLight. That second one is
// what keeps the Ironwind cave bosses attacking: neither has night vision.
//
// A nil mob or room returns true. These run on every behaviour tree tick and a
// missing instance must not silently blind the world.
func mobCanSee(mob *mobs.Mob, room *rooms.Room) bool {
	if mob == nil || room == nil {
		return true
	}
	return messaging.CanSeeSightImpairedOnly(&mob.Character, room)
}
