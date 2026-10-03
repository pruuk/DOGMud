package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/targeting"
)

// actions_vanish.go: `vanish`, the hit-and-hide primitive.
//
// The ambusher archetype hides only when it is idle, and flees to another
// room when hurt. A creature that cannot leave its room (a rift construct:
// rift doors refuse mobs) has to break off and hide where it stands, in the
// middle of a fight. `vanish` does that: the mob drops its own target,
// everyone fighting it lets go of it (retargeting another foe in the room if
// one is attacking them, exactly as when a mob leaves the room), and it tries
// to hide with the ordinary sneak (actions.Sneak: the Awareness machine, a
// roll against everyone watching). Sneaking is refused in a fight, which is
// why the fight has to end first.
//
// The break-off always happens, so the round is spent: Success. Whether the
// hide worked is the sneak's roll; a mob left in plain sight is the tree's to
// deal with (the Glint Stalker lies low a couple of rounds either way, then
// comes back in).
//
// From hiding, the tree's own branches (a surprise attack on player_enter,
// or on mob_idle with a player in the room) bring it back. Players can also
// `search` for it.

func init() {
	actionRegistry["vanish"] = actVanish
}

func actVanish(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil || mob.Character.Health <= 0 || mob.Character.IsHidden() {
		return Failure
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return Failure
	}
	targeting.Release(&mob.Character, targeting.ReasonDisengage)
	actions.ClearRoomAggroOnDeparture(room, mob.InstanceId)
	actions.Sneak(actions.NewMobActorInRoom(mob, room))
	return Success
}
