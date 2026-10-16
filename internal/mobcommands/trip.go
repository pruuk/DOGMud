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

func Trip(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use trip
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteTrip(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// A target already on the floor cannot be tripped. actions.CommandIsReady
	// gates this for behavior-tree mobs, but the CombatCommands fallback in
	// NewRound_DoCombat_helpers does NOT consult readiness -- it picks a random
	// authored verb -- and seven shipped mobs list trip or charge there. Without
	// this branch those mobs spend the round producing no damage, no message and
	// no cooldown, which is invisible to the player and reads as the game
	// hanging. Narrate the wasted effort instead.
	if res.TargetOnFloor {
		narrateTripWhiffOnProne(mob, room, res.Target)
		return true, nil
	}

	if res.OnCooldown || res.NoTarget || !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult
	hasTail := res.Variant == actions.TripTailsweep

	mobName := mob.Character.Name
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Resolve the target user record for direct messaging (player targets).
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

	// Variant prefix for the store's event keys (kick's stomp_/knee_/standard_
	// split, applied to trip's tailsweep_/trip_ split).
	prefix := "trip_"
	tripAttack := "trip"
	if hasTail {
		prefix = "tailsweep_"
		tripAttack = "tailsweep"
	}

	damageTokens := map[string]string{movenarration.TokenDamage: dmgDesc}

	if result.Hit {
		if result.KnockedDown {
			sendMoveEvent("trip", movenarration.EventKey(prefix+"knockdown"), ids, aud, messaging.CategoryTrip, damageTokens)
		} else {
			sendMoveEvent("trip", movenarration.EventKey(prefix+"hit"), ids, aud, messaging.CategoryTrip, damageTokens)
		}
	} else if result.Damage > 0 {
		// Defended-partial: the actee line still carries the damage from the
		// store; the room line names the defence that blunted the move (U6b
		// Task 9), so it is swapped for the defence triad's ToRoom text when a
		// defence actually fired.
		roles, _ := renderMoveEvent("trip", movenarration.EventKey(prefix+"partial"), ids, damageTokens)
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, tripAttack)
		partialObserver := lineOrNone(messaging.CategoryTrip, roles.Observer)
		if defended {
			partialObserver = messaging.Say(messaging.CategoryTrip, defence.ToRoom)
			sendMoveDefenceShortage(messaging.Say(messaging.CategoryTrip, defence.Shortage), aud)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    lineOrNone(messaging.CategoryTrip, roles.Actee),
			Observer: partialObserver,
		}, aud)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, tripAttack); defended {
		// A defence stopped it outright: the defender's line and the room
		// line both come from the triad.
		sendMoveDefenceShortage(messaging.Say(messaging.CategoryTrip, defence.Shortage), aud)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryTrip, defence.ToDefender, mobName),
			Observer: messaging.Say(messaging.CategoryTrip, defence.ToRoom),
		}, aud)
	} else {
		sendMoveEvent("trip", movenarration.EventKey(prefix+"miss"), ids, aud, messaging.CategoryTrip, nil)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}

// narrateTripWhiffOnProne gives the round a visible outcome when a mob throws a
// trip or charge at a target who is already on the floor.
//
// The move itself is refused upstream in actions.ExecuteTrip, so nothing is
// charged, no cooldown is taken and no damage is rolled. The ONLY thing this
// adds is the sentence that stops the round from being invisible. Before U8 the
// trip resolved and dealt reduced damage, so the player at least saw something;
// with the prone gate in place, silence here would be a strict regression in
// legibility even though it is an improvement in mechanics.
func narrateTripWhiffOnProne(mob *mobs.Mob, room *rooms.Room, target actions.AggroTarget) {
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

	sendMoveEvent("trip", "whiff_on_prone", ids, aud, messaging.CategoryTrip, nil)
}
