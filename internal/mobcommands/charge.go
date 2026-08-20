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

	if result.Hit {
		if result.KnockedDown {
			if targetChar != nil {
				if canSee {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges and slams into you, sending you sprawling! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something charges and slams into you, sending you sprawling! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			room.SendTextVisual(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges and slams into <ansi fg="username">%s</ansi>, sending them sprawling!`, mobName, targetName),
				targetPlayerId)
		} else {
			if targetChar != nil {
				if canSee {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something charges at you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			room.SendTextVisual(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at <ansi fg="username">%s</ansi>, but they keep their footing!`, mobName, targetName),
				targetPlayerId)
		}
	} else if result.Damage > 0 {
		if targetChar != nil {
			if canSee {
				targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges and you dodge the worst of it, but the impact still clips you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something charges and you dodge the worst of it, but the impact still clips you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		if !sendMoveDefenceTriad(mob, room, target, result.Defence, "charge", messaging.CategoryTrip, true) {
			room.SendTextVisual(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at <ansi fg="username">%s</ansi>, who mostly dodges but still gets clipped!`, mobName, targetName),
				targetPlayerId)
		}
	} else if !sendMoveDefenceTriad(mob, room, target, result.Defence, "charge", messaging.CategoryTrip, false) {
		if targetChar != nil {
			if canSee {
				targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges past you, missing entirely!`, mobName))
			} else {
				targetChar.SendText(messaging.CategoryTrip, `Something charges past you, missing entirely!`)
			}
		}
		room.SendTextVisual(messaging.CategoryTrip,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges past <ansi fg="username">%s</ansi>, missing entirely!`, mobName, targetName),
			targetPlayerId)
	}

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

	if targetChar != nil {
		if canSeeInDark(targetChar, room) {
			targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> thunders in, but you are already down, and it overruns you.`, mobName))
		} else {
			targetChar.SendText(messaging.CategoryTrip,
				`Something thunders in, but you are already down, and it overruns you.`)
		}
	}

	room.SendTextVisual(messaging.CategoryTrip, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> charges <ansi fg="username">%s</ansi>, who is already down, and overruns them.`,
		mobName, target.Name), target.UserId)
}
