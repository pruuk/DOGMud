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
		ActeeId: res.Target.UserId,
		Room:    room,
	}

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

		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(hitMsgs[util.Rand(len(hitMsgs))], targetName, dmgDesc)),
			Actee:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(hitTargetMsgs[util.Rand(len(hitTargetMsgs))], user.Character.Name, dmgDesc)),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(hitRoomMsgs[util.Rand(len(hitRoomMsgs))], user.Character.Name, targetName)),
		}, aud)

		// A detail line riding on the hit above. Observer is NoLine only
		// because that is today's behaviour; a collapsing spell is a world
		// event under the detail-line ruling and gains a room line in its own
		// commit, so the text change is reviewable on its own.
		//
		// The room line now precedes this rather than following it. No
		// individual recipient sees a different order: the room line is never
		// delivered to the actor or the actee, and the observer never receives
		// these two, so only the cross-recipient interleaving moved.
		if res.InterruptedCast {
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="mobname">%s</ansi>'s spell collapses as they fight for air!`, targetName)),
				Actee:    messaging.Say(messaging.CategorySystem, `Your spell collapses as you fight for air!`),
				Observer: messaging.NoLine,
			}, aud)
		}
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

		defence, defended := moveDefenceLines(user, room, res.Target, res.MoveResult.Defence, "throttle lunge")
		observer := messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(partialRoomMsgs[util.Rand(len(partialRoomMsgs))], user.Character.Name, targetName))
		if defended {
			observer = messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom)
			sendMoveDefenceShortage(targetChar, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(partialMsgs[util.Rand(len(partialMsgs))], targetName, dmgDesc)),
			Actee:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(partialTargetMsgs[util.Rand(len(partialTargetMsgs))], user.Character.Name, dmgDesc)),
			Observer: observer,
		}, aud)
	} else if defence, defended := moveDefenceLines(user, room, res.Target, res.MoveResult.Defence, "throttle lunge"); defended {
		// A defence stopped it outright: all three lines come from the triad.
		sendMoveDefenceShortage(targetChar, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToAttacker),
			Actee:    messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToDefender),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
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

		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(missMsgs[util.Rand(len(missMsgs))], targetName)),
			Actee:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(missTargetMsgs[util.Rand(len(missTargetMsgs))], user.Character.Name)),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(missRoomMsgs[util.Rand(len(missRoomMsgs))], user.Character.Name, targetName)),
		}, aud)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actor, res.Counter)

	return true, nil
}
