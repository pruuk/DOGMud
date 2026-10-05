package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// mobCanSee reports whether a mob can make out anyone in the room it is
// standing in, for its DECISIONS: target acquisition (condPlayersInRoom), the
// enemy count (condMultipleEnemies) and party aggro
// (engageHostilePlayerInRoom).
//
// LIGHTING PLAN 5d, ruling D8 (owner, 2026-09-30): a figure the mob can make
// out is enough. It is true at SightFull and at SightShapes from ANY cause,
// natural dim light or infravision, so a mob in a dim room acts on the shape
// it can see, and a heat-sensing mob acts in darkness its reach reads. This
// overturns slice F's ruling that a mob's sight was SightFull only
// (docs/superpowers/specs/completed/2026-09-11-followup-slice-f-mobs-perceive-darkness-design.md).
//
// 🔑 It calls messaging.ParticipantSight directly and does NOT go through
// messaging.CanSeeSightImpairedOnly any more. That predicate stays SightFull
// only because combat reads it to gate Balance.DarknessCombatPenalty on each
// side of a fight, and widening it would hand every infrared character a
// silent balance change. Only mob decisions widen; the combat darkness
// penalty is untouched.
//
// Night vision still cannot help below its floor: a shifted window reads
// nothing under windowFloor (internal/messaging/window.go), so a nightvision
// mob with no infra reach is blind in an unlit cave. A light carried by ANY
// player or mob lifts the darkness for everyone, because Room.LightLevel
// composes a term for every carried light (internal/rooms/lighting.go).
//
// A nil mob or room returns true. These run on every behaviour tree tick and a
// missing instance must not silently blind the world.
func mobCanSee(mob *mobs.Mob, room *rooms.Room) bool {
	if mob == nil || room == nil {
		return true
	}
	d := messaging.ParticipantSight(&mob.Character, room)
	return d == messaging.SightFull || d == messaging.SightShapes
}
