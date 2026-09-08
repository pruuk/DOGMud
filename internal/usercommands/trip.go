package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

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
		Actor:   user,
		ActorId: user.UserId,
		Actee:   acteeRecipient,
		ActeeId: targetPlayerId,
		Room:    room,
	}

	// Send messages
	if result.Hit {
		if hasTail {
			if result.KnockedDown {
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">tailsweep</ansi> sends <ansi fg="mobname">%s</ansi> crashing to the ground! (<ansi fg="damage">%s</ansi>)`, targetName, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
					Actee: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> hammers you with their tail, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
					Observer: messaging.Say(messaging.CategoryTrip,
						fmt.Sprintf(`<ansi fg="username">%s</ansi> tailsweeps <ansi fg="mobname">%s</ansi>, sending them crashing to the ground!`, user.Character.Name, targetName)),
				}, aud)
			} else {
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">tailsweep</ansi> strikes <ansi fg="mobname">%s</ansi>, but they keep their footing! (<ansi fg="damage">%s</ansi>)`, targetName, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
					Actee: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> sweeps at you with their tail, but you manage to stay upright! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
					Observer: messaging.Say(messaging.CategoryTrip,
						fmt.Sprintf(`<ansi fg="username">%s</ansi> tailsweeps <ansi fg="mobname">%s</ansi>, but they keep their footing!`, user.Character.Name, targetName)),
				}, aud)
			}
		} else {
			if result.KnockedDown {
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">trip</ansi> sends <ansi fg="mobname">%s</ansi> crashing to the ground! (<ansi fg="damage">%s</ansi>)`, targetName, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
					Actee: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> sweeps your legs, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
					Observer: messaging.Say(messaging.CategoryTrip,
						fmt.Sprintf(`<ansi fg="username">%s</ansi> trips <ansi fg="mobname">%s</ansi>, sending them crashing to the ground!`, user.Character.Name, targetName)),
				}, aud)
			} else {
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">trip</ansi> strikes <ansi fg="mobname">%s</ansi>, but they stay on their feet! (<ansi fg="damage">%s</ansi>)`, targetName, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
					Actee: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> attempts to trip you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
					Observer: messaging.Say(messaging.CategoryTrip,
						fmt.Sprintf(`<ansi fg="username">%s</ansi> attempts to trip <ansi fg="mobname">%s</ansi>, but they keep their footing!`, user.Character.Name, targetName)),
				}, aud)
			}
		}
	} else if result.Damage > 0 {
		// Defended-partial: personal lines carry the damage; the room line
		// names the defence that blunted the move (U6b Task 9).
		tripAttack := "trip"
		if hasTail {
			tripAttack = "tailsweep"
		}
		defence, defended := moveDefenceLines(user, room, target, result.Defence, tripAttack)
		if defended {
			sendMoveDefenceShortage(targetChar, defence)
		}
		if hasTail {
			tailObserver := messaging.Say(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> tailsweeps <ansi fg="mobname">%s</ansi>, who staggers but keeps their feet!`, user.Character.Name, targetName))
			if defended {
				tailObserver = messaging.Say(messaging.CategoryTrip, defence.ToRoom)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">tailsweep</ansi> fails to trip <ansi fg="mobname">%s</ansi>, but still cracks into them! (<ansi fg="damage">%s</ansi>)`, targetName, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
				Actee:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> swings their tail and you keep your feet, but it still cracks into you! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
				Observer: tailObserver,
			}, aud)
		} else {
			observer := messaging.Say(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> tries to trip <ansi fg="mobname">%s</ansi>, who staggers but keeps their feet!`, user.Character.Name, targetName))
			if defended {
				observer = messaging.Say(messaging.CategoryTrip, defence.ToRoom)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">trip</ansi> fails to take <ansi fg="mobname">%s</ansi> down, but still catches them hard! (<ansi fg="damage">%s</ansi>)`, targetName, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
				Actee:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> tries to trip you and you keep your feet, but the sweep still catches you! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, combat.GetDamageDescription(result.Damage, result.TargetMaxHP))),
				Observer: observer,
			}, aud)
		}
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
				Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryTrip, defence.ToDefender),
				Observer: messaging.Say(messaging.CategoryTrip, defence.ToRoom),
			}, aud)
		} else if hasTail {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">tailsweep</ansi> misses <ansi fg="mobname">%s</ansi>!`, targetName)),
				Actee: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> swings their tail at you, but you avoid it!`, user.Character.Name)),
				Observer: messaging.Say(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> attempts a tailsweep on <ansi fg="mobname">%s</ansi>, but misses!`, user.Character.Name, targetName)),
			}, aud)
		} else {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">trip</ansi> attempt misses <ansi fg="mobname">%s</ansi>!`, targetName)),
				Actee: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> attempts to trip you, but you avoid it!`, user.Character.Name)),
				Observer: messaging.Say(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> attempts to trip <ansi fg="mobname">%s</ansi>, but misses!`, user.Character.Name, targetName)),
			}, aud)
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
