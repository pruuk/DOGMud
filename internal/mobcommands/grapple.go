package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Grapple(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use grapple
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteGrapple(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	if res.OnCooldown || res.NoTarget || res.GrappleImmune || !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult
	mobName := mob.Character.Name
	targetName := target.Name

	// Resolve the target user record for direct messaging (player targets).
	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

	canSee := targetChar == nil || canSeeInDark(targetChar, room)

	// Declared as the interface and left unset when the target is not a player.
	// Assigning a typed-nil *users.UserRecord would make it a non-nil interface
	// value. There is no Actor: a mob has no client.
	var acteeRecipient messaging.Recipient
	if targetChar != nil {
		acteeRecipient = targetChar
	}
	aud := messaging.Audience{
		Actee:   acteeRecipient,
		ActeeId: target.UserId,
		Room:    room,
	}

	// Send messages based on result
	if result.Success {
		successActee := messaging.NoLine
		if targetChar != nil {
			if canSee {
				successActee = messaging.Say(messaging.CategoryGrappleFlow, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> <ansi fg="yellow-bold">grapples</ansi> you, transitioning to <ansi fg="cyan">%s</ansi> position!`, mobName, result.PositionDesc))
			} else {
				successActee = messaging.Say(messaging.CategoryGrappleFlow, fmt.Sprintf(`Something <ansi fg="yellow-bold">grapples</ansi> you, transitioning to <ansi fg="cyan">%s</ansi> position!`, result.PositionDesc))
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: successActee,
			Observer: messaging.Say(messaging.CategoryGrappleFlow,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> <ansi fg="yellow-bold">grapples</ansi> <ansi fg="username">%s</ansi> into <ansi fg="cyan">%s</ansi> position!`, mobName, targetName, result.PositionDesc)),
		}, aud)

		// Disarm messaging: a WORLD EVENT, so it carries the full trio.
		if result.DisarmResult != nil {
			disarmActee := messaging.NoLine
			if targetChar != nil {
				disarmActee = messaging.Say(messaging.CategoryGrappleFlow, result.DisarmResult.TargetMsg)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    disarmActee,
				Observer: messaging.Say(messaging.CategoryGrappleFlow, result.DisarmResult.RoomMessage),
			}, aud)
		}
	} else {
		failActee := messaging.NoLine
		if targetChar != nil {
			if canSee {
				failActee = messaging.Say(messaging.CategoryGrappleFlow, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to grapple you, but you slip away!`, mobName))
			} else {
				failActee = messaging.Say(messaging.CategoryGrappleFlow, `Something tries to grapple you, but you slip away!`)
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: failActee,
			Observer: messaging.Say(messaging.CategoryGrappleFlow,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to grapple <ansi fg="username">%s</ansi>, but fails!`, mobName, targetName)),
		}, aud)

		// Critical failure messaging: a world event, full trio.
		if result.CritFailure != nil {
			critActee := messaging.NoLine
			if targetChar != nil {
				critActee = messaging.Say(messaging.CategoryGrappleFlow, result.CritFailure.TargetMessage)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    critActee,
				Observer: messaging.Say(messaging.CategoryGrappleFlow, result.CritFailure.RoomMessage),
			}, aud)
		}
	}

	return true, nil
}
