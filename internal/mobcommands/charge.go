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

// Charge is a boar trip variant — same mechanics as trip, charge-specific
// narration. Trip resolution (skill move, knockdown roll, prone application,
// analytics, round consumption) is delegated to actions.ExecuteTrip; only
// the charge-specific messages are handled here.
func Charge(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteTrip(actions.NewMobActorInRoom(mob, room))
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// See narrateTripWhiffOnProne in trip.go. `charge` is the verb most exposed
	// to this: it is authored into CombatCommands on the boar and its kin, and
	// that dispatch path never consults actions.CommandIsReady, so a charge WILL
	// be selected against a player who is already down.
	if res.TargetOnFloor {
		narrateChargeWhiffOnProne(mob, room, res.Target)
		return true, nil
	}

	if res.OnCooldown || res.NoTarget || !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult

	mobName := mob.Character.Name
	targetName := target.Name
	targetPlayerId := target.UserId

	// Resolve the target user record for direct messaging (player targets only).
	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

	canSee := targetChar == nil || canSeeInDark(targetChar, room)
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Declared as the interface and left unset when the target is not a player.
	// Assigning a typed-nil *users.UserRecord would make it a non-nil interface
	// value. There is no Actor: a mob has no client.
	var acteeRecipient messaging.Recipient
	if targetChar != nil {
		acteeRecipient = targetChar
	}
	aud := messaging.Audience{
		Actee:   acteeRecipient,
		ActeeId: targetPlayerId,
		Room:    room,
	}

	if result.Hit {
		if result.KnockedDown {
			downActee := messaging.NoLine
			if targetChar != nil {
				if canSee {
					downActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges and slams into you, sending you sprawling! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					downActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something charges and slams into you, sending you sprawling! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: downActee,
				Observer: messaging.Say(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges and slams into <ansi fg="username">%s</ansi>, sending them sprawling!`, mobName, targetName)),
			}, aud)
		} else {
			hitActee := messaging.NoLine
			if targetChar != nil {
				if canSee {
					hitActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					hitActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something charges at you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: hitActee,
				Observer: messaging.Say(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at <ansi fg="username">%s</ansi>, but they keep their footing!`, mobName, targetName)),
			}, aud)
		}
	} else if result.Damage > 0 {
		partialActee := messaging.NoLine
		if targetChar != nil {
			if canSee {
				partialActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges and you dodge the worst of it, but the impact still clips you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				partialActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something charges and you dodge the worst of it, but the impact still clips you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, "charge")
		partialObserver := messaging.Say(messaging.CategoryTrip,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at <ansi fg="username">%s</ansi>, who mostly dodges but still gets clipped!`, mobName, targetName))
		if defended {
			partialObserver = messaging.Say(messaging.CategoryTrip, defence.ToRoom)
			sendMoveDefenceShortage(targetChar, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    partialActee,
			Observer: partialObserver,
		}, aud)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, "charge"); defended {
		sendMoveDefenceShortage(targetChar, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryTrip, defence.ToDefender),
			Observer: messaging.Say(messaging.CategoryTrip, defence.ToRoom),
		}, aud)
	} else {
		missActee := messaging.NoLine
		if targetChar != nil {
			if canSee {
				missActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges past you, missing entirely!`, mobName))
			} else {
				missActee = messaging.Say(messaging.CategoryTrip, `Something charges past you, missing entirely!`)
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: missActee,
			Observer: messaging.Say(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges past <ansi fg="username">%s</ansi>, missing entirely!`, mobName, targetName)),
		}, aud)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actions.NewMobActorInRoom(mob, room), res.Counter)

	return true, nil
}

// narrateChargeWhiffOnProne is the charge-flavoured sibling of
// narrateTripWhiffOnProne. A charging animal that commits to a target already
// lying down overruns them; that is the picture the player should get, rather
// than a round in which nothing at all appears to happen.
func narrateChargeWhiffOnProne(mob *mobs.Mob, room *rooms.Room, target actions.AggroTarget) {
	mobName := mob.Character.Name

	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

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

	proneActee := messaging.NoLine
	if targetChar != nil {
		if canSeeInDark(targetChar, room) {
			proneActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> thunders in, but you are already down, and it overruns you.`, mobName))
		} else {
			proneActee = messaging.Say(messaging.CategoryTrip,
				`Something thunders in, but you are already down, and it overruns you.`)
		}
	}

	messaging.SendTrio(messaging.Trio{
		Actor: messaging.NoLine,
		Actee: proneActee,
		Observer: messaging.Say(messaging.CategoryTrip, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> charges <ansi fg="username">%s</ansi>, who is already down, and overruns them.`,
			mobName, target.Name)),
	}, aud)
}
