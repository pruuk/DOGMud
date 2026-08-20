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
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Throttle(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "throttle",
	})
	if handled {
		return true, nil
	}

	// Delegate core resolution to the shared action.
	res := actions.ExecuteThrottle(actor)
	if res.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(res.Cost))
		return true, nil
	}

	if res.NotFanged {
		user.SendText(messaging.CategorySystem, "You have no fangs to throttle with.")
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

	targetName := res.Target.Name

	// Resolve player target for direct messaging.
	var targetChar *users.UserRecord
	if res.Target.UserId > 0 {
		targetChar = users.GetByUserId(res.Target.UserId)
	}

	dmgDesc := combat.GetDamageDescription(res.MoveResult.Damage, res.MoveResult.TargetMaxHP)

	if res.MoveResult.Hit {
		hitMsgs := []string{
			`Your fangs clamp around <ansi fg="mobname">%s</ansi>'s throat, cutting off their air! (<ansi fg="damage">%s</ansi>)`,
			`You seize <ansi fg="mobname">%s</ansi> by the throat with savage fangs, crushing their windpipe! (<ansi fg="damage">%s</ansi>)`,
			`Your jaws lock around <ansi fg="mobname">%s</ansi>'s neck in a crushing choke! (<ansi fg="damage">%s</ansi>)`,
			`You drive your fangs into <ansi fg="mobname">%s</ansi>'s throat and squeeze! (<ansi fg="damage">%s</ansi>)`,
		}
		hitTargetMsgs := []string{
			`<ansi fg="username">%s</ansi>'s fangs clamp around your throat, cutting off your air! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="username">%s</ansi> seizes your throat with savage fangs, crushing your windpipe! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="username">%s</ansi>'s jaws lock around your neck in a crushing choke! (<ansi fg="damage">%s</ansi>)`,
		}
		hitRoomMsgs := []string{
			`<ansi fg="username">%s</ansi> clamps fangs around <ansi fg="mobname">%s</ansi>'s throat in a savage choke!`,
			`<ansi fg="username">%s</ansi> seizes <ansi fg="mobname">%s</ansi> by the throat with crushing fangs!`,
		}

		user.SendText(messaging.CategorySystem, fmt.Sprintf(hitMsgs[util.Rand(len(hitMsgs))], targetName, dmgDesc))
		if targetChar != nil {
			targetChar.SendText(messaging.CategorySystem, fmt.Sprintf(hitTargetMsgs[util.Rand(len(hitTargetMsgs))], user.Character.Name, dmgDesc))
		}

		if res.InterruptedCast {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="mobname">%s</ansi>'s spell collapses as they fight for air!`, targetName))
			if targetChar != nil {
				targetChar.SendText(messaging.CategorySystem, `Your spell collapses as you fight for air!`)
			}
		}

		room.SendTextVisual(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(hitRoomMsgs[util.Rand(len(hitRoomMsgs))], user.Character.Name, targetName),
			user.UserId, res.Target.UserId)
	} else if res.MoveResult.Damage > 0 {
		partialMsgs := []string{
			`Your throttle lunge mostly misses <ansi fg="mobname">%s</ansi>'s throat, but your fangs still graze it! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="mobname">%s</ansi> pulls mostly free of your grip, but your fangs still catch their throat! (<ansi fg="damage">%s</ansi>)`,
		}
		partialTargetMsgs := []string{
			`<ansi fg="username">%s</ansi> lunges for your throat and you pull mostly free, but the fangs still catch you! (<ansi fg="damage">%s</ansi>)`,
			`You twist mostly away as <ansi fg="username">%s</ansi> snaps at your throat, but not all the way! (<ansi fg="damage">%s</ansi>)`,
		}
		partialRoomMsgs := []string{
			`<ansi fg="username">%s</ansi> lunges for <ansi fg="mobname">%s</ansi>'s throat, who pulls mostly free but still gets grazed!`,
		}

		user.SendText(messaging.CategorySystem, fmt.Sprintf(partialMsgs[util.Rand(len(partialMsgs))], targetName, dmgDesc))
		if targetChar != nil {
			targetChar.SendText(messaging.CategorySystem, fmt.Sprintf(partialTargetMsgs[util.Rand(len(partialTargetMsgs))], user.Character.Name, dmgDesc))
		}
		if !sendMoveDefenceTriad(user, room, res.Target, res.MoveResult.Defence, "throttle lunge", messaging.CategoryHitNaturalSharp, true) {
			room.SendTextVisual(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(partialRoomMsgs[util.Rand(len(partialRoomMsgs))], user.Character.Name, targetName),
				user.UserId, res.Target.UserId)
		}
	} else if !sendMoveDefenceTriad(user, room, res.Target, res.MoveResult.Defence, "throttle lunge", messaging.CategoryHitNaturalSharp, false) {
		missMsgs := []string{
			`Your throttle lunge misses <ansi fg="mobname">%s</ansi>'s throat!`,
			`You snap at <ansi fg="mobname">%s</ansi>'s throat but they pull away!`,
			`<ansi fg="mobname">%s</ansi> twists away before your fangs can find their throat!`,
		}
		missTargetMsgs := []string{
			`<ansi fg="username">%s</ansi> lunges for your throat but misses!`,
			`You twist away as <ansi fg="username">%s</ansi> snaps at your throat!`,
		}
		missRoomMsgs := []string{
			`<ansi fg="username">%s</ansi> lunges for <ansi fg="mobname">%s</ansi>'s throat but misses!`,
		}

		user.SendText(messaging.CategorySystem, fmt.Sprintf(missMsgs[util.Rand(len(missMsgs))], targetName))
		if targetChar != nil {
			targetChar.SendText(messaging.CategorySystem, fmt.Sprintf(missTargetMsgs[util.Rand(len(missTargetMsgs))], user.Character.Name))
		}
		room.SendTextVisual(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(missRoomMsgs[util.Rand(len(missRoomMsgs))], user.Character.Name, targetName),
			user.UserId, res.Target.UserId)
	}

	return true, nil
}
