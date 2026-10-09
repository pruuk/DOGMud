package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// kickCategories: every role rides CategoryKick, the player's own lines
// included, so they wrap and colour like the room's (#449).
var kickCategories = moveCategories{Actor: messaging.CategoryKick, Actee: messaging.CategoryKick, Observer: messaging.CategoryKick}

func Kick(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "kick",
	})
	if handled {
		return true, nil
	}

	// Delegate core resolution to the shared action.
	res := actions.ExecuteKick(actor)
	if res.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(res.Cost))
		return true, nil
	}

	if res.Crafting {
		// Safety net — should have been caught by the pre-reject above.
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't kick while focused on your work. Finish or be interrupted first.</ansi>`)
		return true, nil
	}

	if res.OnCooldown {
		user.SendText(messaging.CategorySystem, "You need a moment to recover before attempting another special move.")
		return true, nil
	}
	if res.NoTarget {
		user.SendText(messaging.CategorySystem, "You have no target!")
		return true, nil
	}

	target := res.Target
	result := res.MoveResult
	targetName := target.Name

	// Resolve player target for direct messaging.
	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

	// Send messages.
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Attack name for the defence triad renderer (U6b Task 9).
	attackName := "kick"
	switch res.Variant {
	case actions.KickStomp:
		attackName = "stomp"
	case actions.KickKnee:
		attackName = "knee strike"
	}

	// Declared as the interface and left unset for a mob target. Assigning a
	// typed-nil *users.UserRecord would make it a non-nil interface value.
	var acteeRecipient messaging.Recipient
	if targetChar != nil {
		acteeRecipient = targetChar
	}
	aud := messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   target.UserId,
		ActeeName: targetName,
		Room:      room,
	}

	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, user.Character.Name),
		ActorPlain: user.Character.Name,
		Actee:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, targetName),
		ActeePlain: targetName,
	}
	damageTokens := map[string]string{movenarration.TokenDamage: dmgDesc}

	if result.Hit {
		switch res.Variant {
		case actions.KickStomp:
			sendMoveEvent("kick", "player_stomp_hit", ids, aud, kickCategories, damageTokens)

		case actions.KickKnee:
			sendMoveEvent("kick", "player_knee_hit", ids, aud, kickCategories, damageTokens)

		default: // KickStandard
			if result.KnockedDown {
				sendMoveEvent("kick", "player_standard_knockdown", ids, aud, kickCategories, damageTokens)
			} else {
				sendMoveEvent("kick", "player_standard_hit", ids, aud, kickCategories, damageTokens)
			}
		}
	} else if result.Damage > 0 {
		// Defended-partial: personal lines carry the damage; the room line
		// names the defence that blunted the kick (U6b Task 9).
		var partialEvent movenarration.EventKey
		switch res.Variant {
		case actions.KickStomp:
			partialEvent = "player_stomp_partial"
		case actions.KickKnee:
			partialEvent = "player_knee_partial"
		default: // KickStandard
			partialEvent = "player_standard_partial"
		}
		roles, _ := renderMoveEvent("kick", partialEvent, ids, damageTokens)
		defence, defended := moveDefenceLines(user, room, target, result.Defence, attackName)
		observer := lineOrNone(messaging.CategoryKick, roles.Observer)
		if defended {
			observer = messaging.Say(messaging.CategoryKick, defence.ToRoom)
			sendMoveDefenceShortage(targetChar, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    lineOrNone(kickCategories.Actor, roles.Actor),
			Actee:    lineOrNone(kickCategories.Actee, roles.Actee),
			Observer: observer,
		}, aud)
	} else if defence, defended := moveDefenceLines(user, room, target, result.Defence, attackName); defended {
		// A defence stopped it outright: all three lines come from the triad.
		sendMoveDefenceShortage(targetChar, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategoryKick, defence.ToAttacker),
			Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryKick, defence.ToDefender, user.Character.Name),
			Observer: messaging.Say(messaging.CategoryKick, defence.ToRoom),
		}, aud)
	} else {
		// No defence to narrate (e.g. a fumbled kick): plain miss text.
		switch res.Variant {
		case actions.KickStomp:
			sendMoveEvent("kick", "player_stomp_miss", ids, aud, kickCategories, nil)

		case actions.KickKnee:
			sendMoveEvent("kick", "player_knee_miss", ids, aud, kickCategories, nil)

		default: // KickStandard
			sendMoveEvent("kick", "player_standard_miss", ids, aud, kickCategories, nil)
		}
	}

	// Quest engine: command notification
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "kick",
	}, bridge, bridge)

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actor, res.Counter)

	return true, nil
}
