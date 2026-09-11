package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// sendCritEffectTrio delivers a crit effect's lines (riposte, sweep, shield
// slam; built by applyCritEffects) through messaging.SendTrio. The DEFENDER
// performs the effect, so the defender is the Actor and the original attacker
// the Actee.
//
// A reader who cannot see the other combatant reads "something", and the room
// line is visual. The SWEEP room line used to go out on the audio channel and
// named both combatants to players standing in the dark.
func sendCritEffectTrio(atk, def actions.Actor, room *rooms.Room, crit CritEffectResult) {
	var atkRecipient, defRecipient messaging.Recipient
	if atk.IsPlayer() {
		atkRecipient = atk
	}
	if def.IsPlayer() {
		defRecipient = def
	}
	aud := messaging.Audience{
		Actor:     defRecipient,
		ActorId:   def.GetUserId(),
		ActorName: def.GetCharacter().Name,
		Actee:     atkRecipient,
		ActeeId:   atk.GetUserId(),
		ActeeName: atk.GetCharacter().Name,
	}
	if room != nil {
		aud.Room = room
	}
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategoryHitMelee, crit.DefenderMsg),
		Actee:    messaging.Say(messaging.CategoryHitMelee, crit.AttackerMsg),
		Observer: messaging.Say(messaging.CategoryHitMelee, crit.RoomMsg),
	}, aud)
}
