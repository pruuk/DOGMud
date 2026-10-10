package actions

import "github.com/GoMudEngine/GoMud/internal/state"

// ActorRefOf names an actor for harm attribution: a player by user id, a mob
// by instance id, nobody for nil. A combat move's bleed carries it as the
// record's caster, so a bleed that kills credits the attacker (#240).
func ActorRefOf(a Actor) state.ActorRef {
	if a == nil {
		return state.ActorRef{}
	}
	return state.ActorRef{UserId: a.GetUserId(), MobInstanceId: a.GetMobInstanceId()}
}
