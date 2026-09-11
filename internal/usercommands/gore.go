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

func Gore(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "gore",
	})
	if handled {
		return true, nil
	}

	// Delegate core resolution to the shared action.
	res := actions.ExecuteGore(actor)
	if res.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(res.Cost))
		return true, nil
	}

	if res.NotHorned {
		user.SendText(messaging.CategorySystem, "You have no horns to gore with.")
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
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   res.Target.UserId,
		ActeeName: targetName,
		Room:      room,
	}

	if res.MoveResult.Hit {
		if res.MoveResult.KnockedDown {
			// Hit + knockdown: horned charge tosses target off their feet.
			goreMsgs := []string{
				`You lower your head and charge <ansi fg="mobname">%s</ansi>, driving your horns in and sending them flying! (<ansi fg="damage">%s</ansi>)`,
				`Your charge slams into <ansi fg="mobname">%s</ansi> with bone-jarring force, hurling them to the ground! (<ansi fg="damage">%s</ansi>)`,
				`You thunder forward and gore <ansi fg="mobname">%s</ansi>, tossing them aside like a ragdoll! (<ansi fg="damage">%s</ansi>)`,
				`Your horns catch <ansi fg="mobname">%s</ansi> and you heave them backward off their feet! (<ansi fg="damage">%s</ansi>)`,
			}
			goreTargetMsgs := []string{
				`<ansi fg="username">%s</ansi> lowers their head and charges you — the impact sends you sprawling! (<ansi fg="damage">%s</ansi>)`,
				`<ansi fg="username">%s</ansi> drives their horns into you with shattering force and hurls you to the ground! (<ansi fg="damage">%s</ansi>)`,
				`<ansi fg="username">%s</ansi>'s charge catches you and tosses you off your feet! (<ansi fg="damage">%s</ansi>)`,
			}
			goreRoomMsgs := []string{
				`<ansi fg="username">%s</ansi> charges <ansi fg="mobname">%s</ansi> and hurls them to the ground!`,
				`<ansi fg="username">%s</ansi> drives their horns into <ansi fg="mobname">%s</ansi> and sends them flying!`,
				`<ansi fg="username">%s</ansi>'s horned charge tosses <ansi fg="mobname">%s</ansi> off their feet!`,
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(goreMsgs[util.Rand(len(goreMsgs))], targetName, dmgDesc)),
				Actee:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(goreTargetMsgs[util.Rand(len(goreTargetMsgs))], user.Character.Name, dmgDesc)),
				Observer: messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(goreRoomMsgs[util.Rand(len(goreRoomMsgs))], user.Character.Name, targetName)),
			}, aud)
		} else {
			// Hit but no knockdown: the charge connects but target stays up.
			goreMsgs := []string{
				`You ram your horns into <ansi fg="mobname">%s</ansi> with a powerful charge! (<ansi fg="damage">%s</ansi>)`,
				`Your horned charge drives into <ansi fg="mobname">%s</ansi>, rocking them back! (<ansi fg="damage">%s</ansi>)`,
				`You lower your head and slam your horns into <ansi fg="mobname">%s</ansi>! (<ansi fg="damage">%s</ansi>)`,
				`Your charge catches <ansi fg="mobname">%s</ansi> with your horns, drawing a pained grunt! (<ansi fg="damage">%s</ansi>)`,
			}
			goreTargetMsgs := []string{
				`<ansi fg="username">%s</ansi> charges and drives their horns into you! (<ansi fg="damage">%s</ansi>)`,
				`<ansi fg="username">%s</ansi>'s horned charge slams into you and rocks you back! (<ansi fg="damage">%s</ansi>)`,
				`<ansi fg="username">%s</ansi> lowers their head and rams their horns into you! (<ansi fg="damage">%s</ansi>)`,
			}
			goreRoomMsgs := []string{
				`<ansi fg="username">%s</ansi> charges <ansi fg="mobname">%s</ansi> and drives their horns in!`,
				`<ansi fg="username">%s</ansi>'s horned charge slams into <ansi fg="mobname">%s</ansi>!`,
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(goreMsgs[util.Rand(len(goreMsgs))], targetName, dmgDesc)),
				Actee:    messaging.Say(messaging.CategorySystem, fmt.Sprintf(goreTargetMsgs[util.Rand(len(goreTargetMsgs))], user.Character.Name, dmgDesc)),
				Observer: messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(goreRoomMsgs[util.Rand(len(goreRoomMsgs))], user.Character.Name, targetName)),
			}, aud)
		}
	} else if res.MoveResult.Damage > 0 {
		partialMsgs := []string{
			`Your charge at <ansi fg="mobname">%s</ansi> mostly misses, but your horns still graze them! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="mobname">%s</ansi> dodges the worst of your charge, but your horns still catch them! (<ansi fg="damage">%s</ansi>)`,
		}
		partialTargetMsgs := []string{
			`<ansi fg="username">%s</ansi> charges you and you sidestep most of it, but the horns still catch you! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="username">%s</ansi> thunders toward you; you dodge most of the gore, but not all! (<ansi fg="damage">%s</ansi>)`,
		}
		partialRoomMsgs := []string{
			`<ansi fg="username">%s</ansi> charges <ansi fg="mobname">%s</ansi>, who mostly dodges but still gets grazed by the horns!`,
		}

		defence, defended := moveDefenceLines(user, room, res.Target, res.MoveResult.Defence, "goring charge")
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
	} else if defence, defended := moveDefenceLines(user, room, res.Target, res.MoveResult.Defence, "goring charge"); defended {
		// A defence stopped it outright: all three lines come from the triad.
		sendMoveDefenceShortage(targetChar, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToAttacker),
			Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryHitNaturalSharp, defence.ToDefender),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
		missMsgs := []string{
			`Your charge at <ansi fg="mobname">%s</ansi> misses as they sidestep your horns!`,
			`You thunder toward <ansi fg="mobname">%s</ansi> but they dodge your gore!`,
			`Your horned charge carries you past <ansi fg="mobname">%s</ansi> — they step aside!`,
		}
		missTargetMsgs := []string{
			`<ansi fg="username">%s</ansi> charges you but you sidestep their horns!`,
			`<ansi fg="username">%s</ansi> thunders toward you, but you dodge the gore!`,
		}
		missRoomMsgs := []string{
			`<ansi fg="username">%s</ansi> charges <ansi fg="mobname">%s</ansi>, but misses!`,
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
