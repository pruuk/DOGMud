package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Charge is a boar trip variant — same mechanics as trip, charge-specific
// narration. Trip resolution (skill move, knockdown roll, prone application,
// analytics, round consumption) is delegated to actions.ExecuteTrip; only
// the charge-specific messages are handled here.
func Charge(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteTrip(actions.NewMobActorInRoom(mob, room))
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// See narrateTripWhiffOnProne in trip.go. `charge` is the verb most exposed
	// to this: it is authored into CombatCommands on the boar and its kin, and
	// that dispatch path never consults actions.CommandIsReady, so a charge WILL
	// be selected against a player who is already down.
	if res.TargetOnFloor {
		narrateChargeWhiffOnProne(mob, room, res.Target)
		return true, nil
	}

	if res.OnCooldown || res.NoTarget || !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult

	mobName := mob.Character.Name
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Resolve the target user record for direct messaging (player targets only).
	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mobName),
		ActorPlain: mobName,
		Actee:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, target.Name),
		ActeePlain: target.Name,
	}

	// Declared as the interface and left unset when the target is not a player.
	// Assigning a typed-nil *users.UserRecord would make it a non-nil interface
	// value. There is no Actor: a mob has no client.
	var acteeRecipient messaging.Recipient
	if targetChar != nil {
		acteeRecipient = targetChar
	}
	aud := messaging.Audience{
		ActorName: mobName,
		Actee:     acteeRecipient,
		ActeeId:   target.UserId,
		ActeeName: target.Name,
		Room:      room,
	}

	damageTokens := map[string]string{movenarration.TokenDamage: dmgDesc}

	if result.Hit {
		if result.KnockedDown {
			sendMoveEvent("charge", "knockdown", ids, aud, messaging.CategoryTrip, damageTokens)
		} else {
			sendMoveEvent("charge", "hit", ids, aud, messaging.CategoryTrip, damageTokens)
		}
	} else if result.Damage > 0 {
		// Defended-partial: every role reads the partial tier (sendPartialMove).
		defence, _ := moveDefenceLines(mob, room, target, result.Defence, "charge")
		sendPartialMove("charge", "partial", ids, aud, messaging.CategoryTrip, damageTokens, defence.Shortage)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, "charge"); defended {
		sendMoveDefenceShortage(messaging.Say(messaging.CategoryTrip, defence.Shortage), aud)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryTrip, defence.ToDefender, mobName),
			Observer: messaging.Say(messaging.CategoryTrip, defence.ToRoom),
		}, aud)
	} else {
		sendMoveEvent("charge", "miss", ids, aud, messaging.CategoryTrip, nil)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actions.NewMobActorInRoom(mob, room), res.Counter)

	return true, nil
}

// narrateChargeWhiffOnProne is the charge-flavoured sibling of
// narrateTripWhiffOnProne. A charging animal that commits to a target already
// lying down overruns them; that is the picture the player should get, rather
// than a round in which nothing at all appears to happen.
func narrateChargeWhiffOnProne(mob *mobs.Mob, room *rooms.Room, target actions.AggroTarget) {
	mobName := mob.Character.Name

	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mobName),
		ActorPlain: mobName,
		Actee:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, target.Name),
		ActeePlain: target.Name,
	}

	// Declared as the interface and left unset when the target is not a player.
	// Assigning a typed-nil *users.UserRecord would make it a non-nil interface
	// value. There is no Actor: a mob has no client.
	var acteeRecipient messaging.Recipient
	if targetChar != nil {
		acteeRecipient = targetChar
	}
	aud := messaging.Audience{
		ActorName: mobName,
		Actee:     acteeRecipient,
		ActeeId:   target.UserId,
		ActeeName: target.Name,
		Room:      room,
	}

	sendMoveEvent("charge", "whiff_on_prone", ids, aud, messaging.CategoryTrip, nil)
}
