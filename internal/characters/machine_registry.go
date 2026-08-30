package characters

import (
	"github.com/GoMudEngine/GoMud/internal/state"
)

// ActorRef returns this Character's identity as a state.ActorRef, carrying
// BOTH id fields.
//
// This exists because engagement_storage.go built its actor ref as
// state.ActorRef{UserId: c.userId} alone. Nothing calls SetUserId on a mob, so
// a mob's ref was the zero value and combatphase.RecordInboundAttacker
// early-returned on ActorRef.IsZero() -- a second, independent break sitting
// behind the empty machine registry. Build refs with this method, never by
// hand.
func (c *Character) ActorRef() state.ActorRef {
	return state.ActorRef{
		UserId:        c.userId,
		MobInstanceId: c.MobInstanceId,
	}
}
