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

func Trip(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use trip
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteTrip(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// A target already on the floor cannot be tripped. actions.CommandIsReady
	// gates this for behavior-tree mobs, but the CombatCommands fallback in
	// NewRound_DoCombat_helpers does NOT consult readiness -- it picks a random
	// authored verb -- and seven shipped mobs list trip or charge there. Without
	// this branch those mobs spend the round producing no damage, no message and
	// no cooldown, which is invisible to the player and reads as the game
	// hanging. Narrate the wasted effort instead.
	if res.TargetOnFloor {
		narrateTripWhiffOnProne(mob, room, res.Target)
		return true, nil
	}

	if res.OnCooldown || res.NoTarget || !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult
	hasTail := res.Variant == actions.TripTailsweep

	mobName := mob.Character.Name
	targetName := target.Name
	targetPlayerId := target.UserId

	// Resolve the target user record for direct messaging (player targets).
	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

	canSee := targetChar == nil || canSeeInDark(targetChar, room)
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	if result.Hit {
		if hasTail {
			if result.KnockedDown {
				if targetChar != nil {
					if canSee {
						targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> hammers you with their tail, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something hammers you with a powerful sweep, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				room.SendTextVisual(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tailsweeps <ansi fg="username">%s</ansi>, sending them crashing to the ground!`, mobName, targetName),
					targetPlayerId)
			} else {
				if targetChar != nil {
					if canSee {
						targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> sweeps at you with their tail, but you manage to stay upright! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something sweeps at you powerfully, but you manage to stay upright! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				room.SendTextVisual(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tailsweeps <ansi fg="username">%s</ansi>, but they keep their footing!`, mobName, targetName),
					targetPlayerId)
			}
		} else {
			if result.KnockedDown {
				if targetChar != nil {
					if canSee {
						targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> sweeps your legs, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something sweeps your legs, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				room.SendTextVisual(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> trips <ansi fg="username">%s</ansi>, sending them crashing to the ground!`, mobName, targetName),
					targetPlayerId)
			} else {
				if targetChar != nil {
					if canSee {
						targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to trip you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something attempts to trip you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				room.SendTextVisual(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to trip <ansi fg="username">%s</ansi>, but they keep their footing!`, mobName, targetName),
					targetPlayerId)
			}
		}
	} else if result.Damage > 0 {
		if hasTail {
			if targetChar != nil {
				if canSee {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swings their tail and you keep your feet, but it still cracks into you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something sweeps at you and you keep your feet, but it still cracks into you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			room.SendTextVisual(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tailsweeps <ansi fg="username">%s</ansi>, who staggers but keeps their feet!`, mobName, targetName),
				targetPlayerId)
		} else {
			if targetChar != nil {
				if canSee {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to trip you and you keep your feet, but the sweep still catches you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`Something tries to trip you and you keep your feet, but the sweep still catches you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			room.SendTextVisual(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to trip <ansi fg="username">%s</ansi>, who staggers but keeps their feet!`, mobName, targetName),
				targetPlayerId)
		}
	} else {
		if hasTail {
			if targetChar != nil {
				if canSee {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swings their tail at you, but you avoid it!`, mobName))
				} else {
					targetChar.SendText(messaging.CategoryTrip, `Something sweeps at you powerfully, but you avoid it!`)
				}
			}
			room.SendTextVisual(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts a tailsweep on <ansi fg="username">%s</ansi>, but misses!`, mobName, targetName),
				targetPlayerId)
		} else {
			if targetChar != nil {
				if canSee {
					targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to trip you, but you avoid it!`, mobName))
				} else {
					targetChar.SendText(messaging.CategoryTrip, `Something attempts to trip you, but you avoid it!`)
				}
			}
			room.SendTextVisual(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to trip <ansi fg="username">%s</ansi>, but misses!`, mobName, targetName),
				targetPlayerId)
		}
	}

	return true, nil
}

// narrateTripWhiffOnProne gives the round a visible outcome when a mob throws a
// trip or charge at a target who is already on the floor.
//
// The move itself is refused upstream in actions.ExecuteTrip, so nothing is
// charged, no cooldown is taken and no damage is rolled. The ONLY thing this
// adds is the sentence that stops the round from being invisible. Before U8 the
// trip resolved and dealt reduced damage, so the player at least saw something;
// with the prone gate in place, silence here would be a strict regression in
// legibility even though it is an improvement in mechanics.
func narrateTripWhiffOnProne(mob *mobs.Mob, room *rooms.Room, target actions.AggroTarget) {
	mobName := mob.Character.Name

	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

	if targetChar != nil {
		if canSeeInDark(targetChar, room) {
			targetChar.SendText(messaging.CategoryTrip, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> rushes at you and finds only the ground you are already on.`, mobName))
		} else {
			targetChar.SendText(messaging.CategoryTrip,
				`Something rushes past you and finds only the ground you are already on.`)
		}
	}

	room.SendTextVisual(messaging.CategoryTrip, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> rushes at <ansi fg="username">%s</ansi>, who is already down, and carries straight past.`,
		mobName, target.Name), target.UserId)
}
