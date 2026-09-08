package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

var stageSpecialMoveTarget = actions.StageMeleeTarget

func Bash(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "bash",
	})
	if handled {
		return true, nil
	}

	// Delegate core bash logic to the shared action.
	bashResult := actions.ExecuteBash(actor)
	if bashResult.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(bashResult.Cost))
		return true, nil
	}

	if bashResult.Crafting {
		// Safety net — should have been caught by the pre-reject above.
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't bash while focused on your work. Finish or be interrupted first.</ansi>`)
		return true, nil
	}

	switch {
	case bashResult.NoShield:
		user.SendText(messaging.CategorySystem, "You need a shield equipped to perform a shield bash!")
		return true, nil
	case bashResult.OnCooldown:
		user.SendText(messaging.CategorySystem, "You need a moment to recover before attempting another special move.")
		return true, nil
	case bashResult.NoTarget:
		user.SendText(messaging.CategorySystem, "Your target is gone!")
		return true, nil
	}

	// Format and send messages.
	target := bashResult.Target
	result := bashResult.MoveResult
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Look up the target player record for personal messaging (nil if mob target).
	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	// Declared as the interface and left unset for a mob target. Assigning a
	// typed-nil *users.UserRecord would make it a non-nil interface value.
	var acteeRecipient messaging.Recipient
	if targetUser != nil {
		acteeRecipient = targetUser
	}
	aud := messaging.Audience{
		Actor:   user,
		ActorId: user.UserId,
		Actee:   acteeRecipient,
		ActeeId: target.UserId,
		Room:    room,
	}

	if result.Hit {
		if result.KnockedDown {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(messaging.CategorySystem,
					fmt.Sprintf(`Your <ansi fg="yellow-bold">shield bash</ansi> knocks <ansi fg="mobname">%s</ansi> to the ground! (<ansi fg="damage">%s</ansi>)`, target.Name, dmgDesc)),
				Actee: messaging.Say(messaging.CategorySystem,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>'s <ansi fg="yellow-bold">shield bash</ansi> knocks you to the ground! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, dmgDesc)),
				Observer: messaging.Say(messaging.CategoryBash,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>'s <ansi fg="yellow-bold">shield bash</ansi> knocks <ansi fg="mobname">%s</ansi> to the ground!`, user.Character.Name, target.Name)),
			}, aud)
		} else {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(messaging.CategorySystem,
					fmt.Sprintf(`Your <ansi fg="yellow-bold">shield bash</ansi> strikes <ansi fg="mobname">%s</ansi>! (<ansi fg="damage">%s</ansi>)`, target.Name, dmgDesc)),
				Actee: messaging.Say(messaging.CategorySystem,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>'s <ansi fg="yellow-bold">shield bash</ansi> strikes you! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, dmgDesc)),
				Observer: messaging.Say(messaging.CategoryBash,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> bashes <ansi fg="mobname">%s</ansi> with their shield!`, user.Character.Name, target.Name)),
			}, aud)
		}
	} else if result.Damage > 0 {
		// Defended-partial: the personal lines carry the damage, and the room
		// line names the defence that blunted the bash (U6b Task 9), falling
		// back to plain stagger text when there was no defence to name.
		defence, defended := moveDefenceLines(user, room, target, result.Defence, "shield bash")
		observer := messaging.Say(messaging.CategoryBash,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> bashes <ansi fg="mobname">%s</ansi> with their shield, who staggers but stays up!`, user.Character.Name, target.Name))
		if defended {
			observer = messaging.Say(messaging.CategoryBash, defence.ToRoom)
			sendMoveDefenceShortage(targetUser, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySystem,
				fmt.Sprintf(`Your <ansi fg="yellow-bold">shield bash</ansi> fails to floor <ansi fg="mobname">%s</ansi>, but still slams into them! (<ansi fg="damage">%s</ansi>)`, target.Name, dmgDesc)),
			Actee: messaging.Say(messaging.CategorySystem,
				fmt.Sprintf(`<ansi fg="username">%s</ansi>'s <ansi fg="yellow-bold">shield bash</ansi> fails to floor you, but still slams into you! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, dmgDesc)),
			Observer: observer,
		}, aud)
	} else if defence, defended := moveDefenceLines(user, room, target, result.Defence, "shield bash"); defended {
		// A defence stopped it outright: all three lines come from the triad.
		sendMoveDefenceShortage(targetUser, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategoryBash, defence.ToAttacker),
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryBash, defence.ToDefender),
			Observer: messaging.Say(messaging.CategoryBash, defence.ToRoom),
		}, aud)
	} else {
		// No defence to narrate (e.g. a fumbled swing): plain miss text.
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySystem,
				fmt.Sprintf(`Your <ansi fg="yellow-bold">shield bash</ansi> misses <ansi fg="mobname">%s</ansi>!`, target.Name)),
			Actee: messaging.Say(messaging.CategorySystem,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> attempts to bash you with their shield, but misses!`, user.Character.Name)),
			Observer: messaging.Say(messaging.CategoryBash,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> attempts to bash <ansi fg="mobname">%s</ansi>, but misses!`, user.Character.Name, target.Name)),
		}, aud)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actor, bashResult.Counter)

	return true, nil
}
