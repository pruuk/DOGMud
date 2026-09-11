package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// spellAudience is the messaging.Audience for a spell between two parties.
// Either user may be nil (a mob caster or a mob target). actorName and
// acteeName must be the exact strings the lines print.
//
// A nil *users.UserRecord is never stored in the Recipient fields: a typed nil
// would make the interface non-nil, and SendTrio would call through it. A nil
// room is not stored either, for the same reason.
func spellAudience(caster *users.UserRecord, actorName string, target *users.UserRecord, acteeName string, room *rooms.Room) messaging.Audience {
	aud := messaging.Audience{
		ActorName: actorName,
		ActeeName: acteeName,
	}
	if caster != nil {
		aud.Actor = caster
		aud.ActorId = caster.UserId
	}
	if target != nil {
		aud.Actee = target
		aud.ActeeId = target.UserId
	}
	if room != nil {
		aud.Room = room
	}
	return aud
}
