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

	// Resolve the target user record for direct messaging (player targets).
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
		ActeeId: target.UserId,
		Room:    room,
	}

	if result.Hit {
		if hasTail {
			if result.KnockedDown {
				tailDownActee := messaging.NoLine
				if targetChar != nil {
					if canSee {
						tailDownActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> hammers you with their tail, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						tailDownActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something hammers you with a powerful sweep, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.NoLine,
					Actee: tailDownActee,
					Observer: messaging.Say(messaging.CategoryTrip,
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tailsweeps <ansi fg="username">%s</ansi>, sending them crashing to the ground!`, mobName, targetName)),
				}, aud)
			} else {
				tailStayActee := messaging.NoLine
				if targetChar != nil {
					if canSee {
						tailStayActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> sweeps at you with their tail, but you manage to stay upright! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						tailStayActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something sweeps at you powerfully, but you manage to stay upright! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.NoLine,
					Actee: tailStayActee,
					Observer: messaging.Say(messaging.CategoryTrip,
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tailsweeps <ansi fg="username">%s</ansi>, but they keep their footing!`, mobName, targetName)),
				}, aud)
			}
		} else {
			if result.KnockedDown {
				tripDownActee := messaging.NoLine
				if targetChar != nil {
					if canSee {
						tripDownActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> sweeps your legs, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						tripDownActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something sweeps your legs, sending you crashing to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.NoLine,
					Actee: tripDownActee,
					Observer: messaging.Say(messaging.CategoryTrip,
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> trips <ansi fg="username">%s</ansi>, sending them crashing to the ground!`, mobName, targetName)),
				}, aud)
			} else {
				tripStayActee := messaging.NoLine
				if targetChar != nil {
					if canSee {
						tripStayActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to trip you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						tripStayActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something attempts to trip you, but you keep your footing! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.NoLine,
					Actee: tripStayActee,
					Observer: messaging.Say(messaging.CategoryTrip,
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to trip <ansi fg="username">%s</ansi>, but they keep their footing!`, mobName, targetName)),
				}, aud)
			}
		}
	} else if result.Damage > 0 {
		// Defended-partial: personal lines carry the damage; the room line
		// names the defence that blunted the move (U6b Task 9).
		tripAttack := "trip"
		if hasTail {
			tripAttack = "tailsweep"
		}
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, tripAttack)
		if defended {
			sendMoveDefenceShortage(targetChar, defence)
		}
		if hasTail {
			tailPartialActee := messaging.NoLine
			if targetChar != nil {
				if canSee {
					tailPartialActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swings their tail and you keep your feet, but it still cracks into you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					tailPartialActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something sweeps at you and you keep your feet, but it still cracks into you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			tailPartialObserver := messaging.Say(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tailsweeps <ansi fg="username">%s</ansi>, who staggers but keeps their feet!`, mobName, targetName))
			if defended {
				tailPartialObserver = messaging.Say(messaging.CategoryTrip, defence.ToRoom)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    tailPartialActee,
				Observer: tailPartialObserver,
			}, aud)
		} else {
			tripPartialActee := messaging.NoLine
			if targetChar != nil {
				if canSee {
					tripPartialActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to trip you and you keep your feet, but the sweep still catches you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					tripPartialActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`Something tries to trip you and you keep your feet, but the sweep still catches you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			tripPartialObserver := messaging.Say(messaging.CategoryTrip,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to trip <ansi fg="username">%s</ansi>, who staggers but keeps their feet!`, mobName, targetName))
			if defended {
				tripPartialObserver = messaging.Say(messaging.CategoryTrip, defence.ToRoom)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    tripPartialActee,
				Observer: tripPartialObserver,
			}, aud)
		}
	} else {
		// Fully stopped: speak the triad naming the winning defence; fall
		// back to plain miss text when there is no defence to narrate (a
		// fumble). U6b Task 9.
		tripAttack := "trip"
		if hasTail {
			tripAttack = "tailsweep"
		}
		if defence, defended := moveDefenceLines(mob, room, target, result.Defence, tripAttack); defended {
			// A defence stopped it outright: the defender's line and the room
			// line both come from the triad.
			sendMoveDefenceShortage(targetChar, defence)
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryTrip, defence.ToDefender),
				Observer: messaging.Say(messaging.CategoryTrip, defence.ToRoom),
			}, aud)
		} else if hasTail {
			tailMissActee := messaging.NoLine
			if targetChar != nil {
				if canSee {
					tailMissActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swings their tail at you, but you avoid it!`, mobName))
				} else {
					tailMissActee = messaging.Say(messaging.CategoryTrip, `Something sweeps at you powerfully, but you avoid it!`)
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: tailMissActee,
				Observer: messaging.Say(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts a tailsweep on <ansi fg="username">%s</ansi>, but misses!`, mobName, targetName)),
			}, aud)
		} else {
			tripMissActee := messaging.NoLine
			if targetChar != nil {
				if canSee {
					tripMissActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to trip you, but you avoid it!`, mobName))
				} else {
					tripMissActee = messaging.Say(messaging.CategoryTrip, `Something attempts to trip you, but you avoid it!`)
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: tripMissActee,
				Observer: messaging.Say(messaging.CategoryTrip,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to trip <ansi fg="username">%s</ansi>, but misses!`, mobName, targetName)),
			}, aud)
		}
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

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

	whiffActee := messaging.NoLine
	if targetChar != nil {
		if canSeeInDark(targetChar, room) {
			whiffActee = messaging.Say(messaging.CategoryTrip, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> rushes at you and finds only the ground you are already on.`, mobName))
		} else {
			whiffActee = messaging.Say(messaging.CategoryTrip,
				`Something rushes past you and finds only the ground you are already on.`)
		}
	}

	messaging.SendTrio(messaging.Trio{
		Actor: messaging.NoLine,
		Actee: whiffActee,
		Observer: messaging.Say(messaging.CategoryTrip, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> rushes at <ansi fg="username">%s</ansi>, who is already down, and carries straight past.`,
			mobName, target.Name)),
	}, aud)
}
