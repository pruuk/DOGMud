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

// tripCategories: every role rides CategoryTrip, the player's own lines
// included, so they wrap and colour like the room's (#449).
//
// Kept on ONE line (rather than the more usual one-field-per-line struct
// literal): send_trio_only_guard_test.go scans line by line for a guarded
// category (Kick/Trip/Bash) paired with a recognised producer shape on that
// SAME line, and moveCategories{...} is one of those shapes.
var tripCategories = moveCategories{Actor: messaging.CategoryTrip, Actee: messaging.CategoryTrip, Observer: messaging.CategoryTrip}

func Trip(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb:         "trip",
		CraftingVerb: "trip someone",
	})
	if handled {
		return true, nil
	}

	res := actions.ExecuteTrip(actor)
	if res.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(res.Cost))
		return true, nil
	}

	if res.Crafting {
		// Safety net — should have been caught by the pre-reject above.
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't trip someone while focused on your work. Finish or be interrupted first.</ansi>`)
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
	if res.TargetOnFloor {
		user.SendText(messaging.CategorySystem, fmt.Sprintf("%s is already on the ground.", res.Target.Name))
		return true, nil
	}

	if !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult
	hasTail := res.Variant == actions.TripTailsweep

	targetName := target.Name
	targetPlayerId := target.UserId

	// Resolve the target user record for direct messaging (player targets).
	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
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
		ActeeId:   targetPlayerId,
		ActeeName: targetName,
		Room:      room,
	}

	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, user.Character.Name),
		ActorPlain: user.Character.Name,
		Actee:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, targetName),
		ActeePlain: targetName,
	}
	damageTokens := map[string]string{movenarration.TokenDamage: combat.GetDamageDescription(result.Damage, result.TargetMaxHP)}

	// Send messages
	if result.Hit {
		if hasTail {
			if result.KnockedDown {
				sendMoveEvent("trip", "player_tailsweep_knockdown", ids, aud, tripCategories, damageTokens)
			} else {
				sendMoveEvent("trip", "player_tailsweep_hit", ids, aud, tripCategories, damageTokens)
			}
		} else {
			if result.KnockedDown {
				sendMoveEvent("trip", "player_trip_knockdown", ids, aud, tripCategories, damageTokens)
			} else {
				sendMoveEvent("trip", "player_trip_hit", ids, aud, tripCategories, damageTokens)
			}
		}
	} else if result.Damage > 0 {
		// Defended-partial: personal lines carry the damage; the room line
		// names the defence that blunted the move (U6b Task 9).
		var tripAttack string
		var partialEvent movenarration.EventKey
		if hasTail {
			tripAttack = "tailsweep"
			partialEvent = "player_tailsweep_partial"
		} else {
			tripAttack = "trip"
			partialEvent = "player_trip_partial"
		}
		roles, _ := renderMoveEvent("trip", partialEvent, ids, damageTokens)
		defence, defended := moveDefenceLines(user, room, target, result.Defence, tripAttack)
		if defended {
			sendMoveDefenceShortage(targetChar, defence)
		}
		observer := lineOrNone(messaging.CategoryTrip, roles.Observer)
		if defended {
			observer = messaging.Say(messaging.CategoryTrip, defence.ToRoom)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    lineOrNone(tripCategories.Actor, roles.Actor),
			Actee:    lineOrNone(tripCategories.Actee, roles.Actee),
			Observer: observer,
		}, aud)
	} else {
		// Fully stopped: speak the triad naming the winning defence; fall
		// back to plain miss text when there is no defence to narrate (a
		// fumble). U6b Task 9.
		tripAttack := "trip"
		if hasTail {
			tripAttack = "tailsweep"
		}
		if defence, defended := moveDefenceLines(user, room, target, result.Defence, tripAttack); defended {
			// A defence stopped it outright: all three lines come from the triad.
			sendMoveDefenceShortage(targetChar, defence)
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategoryTrip, defence.ToAttacker),
				Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryTrip, defence.ToDefender, user.Character.Name),
				Observer: messaging.Say(messaging.CategoryTrip, defence.ToRoom),
			}, aud)
		} else if hasTail {
			sendMoveEvent("trip", "player_tailsweep_miss", ids, aud, tripCategories, nil)
		} else {
			sendMoveEvent("trip", "player_trip_miss", ids, aud, tripCategories, nil)
		}
	}

	// Quest engine: command notification
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "trip",
	}, bridge, bridge)

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actor, res.Counter)

	return true, nil
}
