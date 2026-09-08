package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Grapple(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "grapple",
	})
	if handled {
		return true, nil
	}

	res := actions.ExecuteGrapple(actor)
	if res.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(res.Cost))
		return true, nil
	}

	if res.Crafting {
		// Safety net — should have been caught by the pre-reject above.
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't grapple while focused on your work. Finish or be interrupted first.</ansi>`)
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
	if res.TargetGrappling {
		user.SendText(messaging.CategorySystem, fmt.Sprintf("%s is already grappling.", res.Target.Name))
		return true, nil
	}

	if res.GrappleImmune {
		// ⚠️ FOUR different refusals reach here and they are NOT the same
		// thing. Two are about the player's own body and carry NO Target, so the
		// old single message printed an EMPTY name for them. The other two are
		// opposites: incorporeal (nothing to hold) versus immovable (too solid to
		// shift). The Arena Champion is the immovable case -- colossus-form -- and
		// telling that player their hands passed through it was simply false.
		switch res.ImmuneReason {
		case actions.GrappleImmuneSelfSpecies:
			user.SendText(messaging.CategorySystem,
				"Your body has nothing that can take hold. Grappling is not a thing you can do.")
		case actions.GrappleImmuneSelfNoArms:
			user.SendText(messaging.CategorySystem,
				"Grappling means seizing and holding, and you have no arms to do it with.")
		case actions.GrappleImmuneTargetImmovable:
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				"You get both hands on %s and heave. It might as well be a boulder. Nothing shifts.",
				res.Target.Name))
		default:
			// Incorporeal, and the safe landing spot for any reason added later.
			// Guarded on the name because a future no-Target reason must not
			// reintroduce the empty-name bug.
			if res.Target.Name == "" {
				user.SendText(messaging.CategorySystem, "You cannot get a grip on that.")
			} else {
				user.SendText(messaging.CategorySystem, fmt.Sprintf(
					"You reach for %s but your hands pass right through!", res.Target.Name))
			}
		}
		return true, nil
	}

	if !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult
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

	// Send messages based on result
	if result.Success {
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`You <ansi fg="yellow-bold">grapple</ansi> <ansi fg="mobname">%s</ansi>, transitioning to <ansi fg="cyan">%s</ansi> position!`, targetName, result.PositionDesc)),
			Actee: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> <ansi fg="yellow-bold">grapples</ansi> you, transitioning to <ansi fg="cyan">%s</ansi> position!`, user.Character.Name, result.PositionDesc)),
			Observer: messaging.Say(messaging.CategoryGrappleFlow,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> <ansi fg="yellow-bold">grapples</ansi> <ansi fg="mobname">%s</ansi> into <ansi fg="cyan">%s</ansi> position!`, user.Character.Name, targetName, result.PositionDesc)),
		}, aud)

		// Flavor text for prone targets.
		//
		// PRIVATE KNOWLEDGE, so NoLine twice, deliberately. This explains the
		// actor's own roll to the actor. A room line here would invent an
		// observation nobody in the room made, about a grapple the event above
		// already narrated to them.
		if result.PositionPenalty < 0 {
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="yellow">%s was already prone - they had little chance to resist!</ansi>`, targetName)),
				Actee:    messaging.NoLine,
				Observer: messaging.NoLine,
			}, aud)
		}

		// Disarm messaging: a WORLD EVENT, so it carries the full trio, which
		// it already did before the migration.
		if result.DisarmResult != nil {
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, result.DisarmResult.Message),
				Actee:    messaging.Say(messaging.CategorySystem, result.DisarmResult.TargetMsg),
				Observer: messaging.Say(messaging.CategoryGrappleFlow, result.DisarmResult.RoomMessage),
			}, aud)
		}
	} else {
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`Your <ansi fg="yellow-bold">grapple</ansi> attempt against <ansi fg="mobname">%s</ansi> fails!`, targetName)),
			Actee: messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> tries to grapple you, but you slip away!`, user.Character.Name)),
			Observer: messaging.Say(messaging.CategoryGrappleFlow,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> tries to grapple <ansi fg="mobname">%s</ansi>, but fails!`, user.Character.Name, targetName)),
		}, aud)

		// Defense penalty: private knowledge again, same ruling as above.
		if result.DefensePenalty {
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, `<ansi fg="red">Your failed attempt leaves you exposed!</ansi>`),
				Actee:    messaging.NoLine,
				Observer: messaging.NoLine,
			}, aud)
		}

		// Critical failure: a world event, full trio, as before.
		if result.CritFailure != nil {
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, result.CritFailure.Message),
				Actee:    messaging.Say(messaging.CategorySystem, result.CritFailure.TargetMessage),
				Observer: messaging.Say(messaging.CategoryGrappleFlow, result.CritFailure.RoomMessage),
			}, aud)
		}
	}

	return true, nil
}
