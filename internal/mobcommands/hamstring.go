package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Hamstring is a wolf physical attack that applies a bleed condition.
func Hamstring(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteHamstring(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	if res.OnCooldown || res.NoTarget || !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult

	mobName := mob.Character.Name

	// Resolve the target user record for direct messaging (player targets).
	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	canSee := targetUser == nil || canSeeInDark(targetUser, room)

	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Declared as the interface and left unset when the target is not a player.
	// Assigning a typed-nil *users.UserRecord would make it a non-nil interface
	// value. There is no Actor: a mob has no client.
	var acteeRecipient messaging.Recipient
	if targetUser != nil {
		acteeRecipient = targetUser
	}
	aud := messaging.Audience{
		Actee:   acteeRecipient,
		ActeeId: target.UserId,
		Room:    room,
	}

	if result.Hit {
		hitActee := messaging.NoLine
		if targetUser != nil {
			if canSee {
				hitActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> rakes its fangs across your legs, opening deep wounds! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				hitActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something rakes its fangs across your legs, opening deep wounds! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: hitActee,
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges low and rakes its fangs across <ansi fg="username">%s</ansi>'s legs!`, mobName, target.Name)),
		}, aud)
	} else if result.Damage > 0 {
		partialActee := messaging.NoLine
		if targetUser != nil {
			if canSee {
				partialActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges at your legs and you dodge most of it, but the fangs still catch you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				partialActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something lunges at your legs and you dodge most of it, but it still catches you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, "hamstring slash")
		partialObserver := messaging.Say(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges at <ansi fg="username">%s</ansi>'s legs, who mostly dodges but still gets caught!`, mobName, target.Name))
		if defended {
			partialObserver = messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom)
			sendMoveDefenceShortage(targetUser, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    partialActee,
			Observer: partialObserver,
		}, aud)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, "hamstring slash"); defended {
		// A defence stopped it outright: the defender's line and the room line
		// both come from the triad.
		sendMoveDefenceShortage(targetUser, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryHitNaturalSharp, defence.ToDefender),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
		missActee := messaging.NoLine
		if targetUser != nil {
			if canSee {
				missActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges at your legs, but you sidestep the attack!`, mobName))
			} else {
				missActee = messaging.Say(messaging.CategoryHitNaturalSharp, `Something lunges at your legs, but you sidestep the attack!`)
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: missActee,
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges at <ansi fg="username">%s</ansi>'s legs, but misses!`, mobName, target.Name)),
		}, aud)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
