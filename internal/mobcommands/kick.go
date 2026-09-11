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

func Kick(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use kick; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	// Delegate core kick logic to the shared action (includes stomp/knee variant
	// detection so mobs now use the appropriate variant automatically).
	res := actions.ExecuteKick(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// Any early-exit condition: silently return.
	if !res.Executed {
		return true, nil
	}

	// Format and send darkness-aware messages.
	target := res.Target
	result := res.MoveResult
	mobName := mob.Character.Name
	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	// Look up target player record for darkness-aware personal messaging.
	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}
	canSee := targetUser == nil || canSeeInDark(targetUser, room)

	// Declared as the interface and left unset when the target is not a player.
	// Assigning a typed-nil *users.UserRecord would make it a non-nil interface
	// value. There is no Actor: a mob has no client.
	var acteeRecipient messaging.Recipient
	if targetUser != nil {
		acteeRecipient = targetUser
	}
	aud := messaging.Audience{
		ActorName: mobName,
		Actee:     acteeRecipient,
		ActeeId:   target.UserId,
		ActeeName: target.Name,
		Room:      room,
	}

	// Attack name for the defence triad renderer (U6b Task 9).
	attackName := "kick"
	switch res.Variant {
	case actions.KickStomp:
		attackName = "stomp"
	case actions.KickKnee:
		attackName = "knee strike"
	}

	if result.Hit {
		switch res.Variant {
		case actions.KickStomp:
			stompHitActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					stompHitActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> stomps on you while you're down! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					stompHitActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`Something stomps on you while you're down! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: stompHitActee,
				Observer: messaging.Say(messaging.CategoryKick,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> stomps on the downed <ansi fg="username">%s</ansi>!`, mobName, target.Name)),
			}, aud)

		case actions.KickKnee:
			kneeHitActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					kneeHitActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> drives a knee into you in the grapple! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					kneeHitActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`Something drives a knee into you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: kneeHitActee,
				Observer: messaging.Say(messaging.CategoryKick,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> drives a knee into <ansi fg="username">%s</ansi>!`, mobName, target.Name)),
			}, aud)

		default: // KickStandard
			if result.KnockedDown {
				downActee := messaging.NoLine
				if targetUser != nil {
					if canSee {
						downActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi>'s powerful <ansi fg="yellow-bold">kick</ansi> knocks you to the ground! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						downActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`Something's powerful <ansi fg="yellow-bold">kick</ansi> knocks you to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.NoLine,
					Actee: downActee,
					Observer: messaging.Say(messaging.CategoryKick,
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> kicks <ansi fg="username">%s</ansi>, knocking them to the ground!`, mobName, target.Name)),
				}, aud)
			} else {
				hitActee := messaging.NoLine
				if targetUser != nil {
					if canSee {
						hitActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> kicks you hard! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
					} else {
						hitActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`Something kicks you hard! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
					}
				}
				messaging.SendTrio(messaging.Trio{
					Actor: messaging.NoLine,
					Actee: hitActee,
					Observer: messaging.Say(messaging.CategoryKick,
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> kicks <ansi fg="username">%s</ansi>!`, mobName, target.Name)),
				}, aud)
			}
		}
	} else if result.Damage > 0 {
		// Defended-partial: personal lines carry the damage; the room line
		// names the defence that blunted the kick (U6b Task 9).
		switch res.Variant {
		case actions.KickStomp:
			stompPartialActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					stompPartialActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to stomp you, and you roll aside, but the heel still catches you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					stompPartialActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`Something tries to stomp you, and you roll aside, but the heel still catches you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			defence, defended := moveDefenceLines(mob, room, target, result.Defence, attackName)
			stompPartialObserver := messaging.Say(messaging.CategoryKick,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to stomp <ansi fg="username">%s</ansi>, who rolls mostly clear but still gets caught!`, mobName, target.Name))
			if defended {
				stompPartialObserver = messaging.Say(messaging.CategoryKick, defence.ToRoom)
				sendMoveDefenceShortage(targetUser, defence)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    stompPartialActee,
				Observer: stompPartialObserver,
			}, aud)

		case actions.KickKnee:
			kneePartialActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					kneePartialActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to knee you, and you block most of it, but it still lands! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					kneePartialActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`Something tries to knee you, and you block most of it, but it still lands! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			defence, defended := moveDefenceLines(mob, room, target, result.Defence, attackName)
			kneePartialObserver := messaging.Say(messaging.CategoryKick,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to knee <ansi fg="username">%s</ansi> in the grapple, who blocks most of it but still takes the hit!`, mobName, target.Name))
			if defended {
				kneePartialObserver = messaging.Say(messaging.CategoryKick, defence.ToRoom)
				sendMoveDefenceShortage(targetUser, defence)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    kneePartialActee,
				Observer: kneePartialObserver,
			}, aud)

		default: // KickStandard
			partialActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					partialActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to kick you, and you slip most of it, but the boot still clips you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					partialActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`Something attempts to kick you, and you slip most of it, but it still clips you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			defence, defended := moveDefenceLines(mob, room, target, result.Defence, attackName)
			partialObserver := messaging.Say(messaging.CategoryKick,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swings a kick at <ansi fg="username">%s</ansi>, who mostly dodges but still gets clipped!`, mobName, target.Name))
			if defended {
				partialObserver = messaging.Say(messaging.CategoryKick, defence.ToRoom)
				sendMoveDefenceShortage(targetUser, defence)
			}
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.NoLine,
				Actee:    partialActee,
				Observer: partialObserver,
			}, aud)
		}
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, attackName); defended {
		// A defence stopped it outright: the defender's line and the room line
		// both come from the triad.
		sendMoveDefenceShortage(targetUser, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    acteeDefenceLine(targetUser, room, messaging.CategoryKick, defence.ToDefender),
			Observer: messaging.Say(messaging.CategoryKick, defence.ToRoom),
		}, aud)
	} else {
		switch res.Variant {
		case actions.KickStomp:
			stompMissActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					stompMissActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to stomp you, but you roll aside!`, mobName))
				} else {
					stompMissActee = messaging.Say(messaging.CategoryKick, `Something tries to stomp you, but you roll aside!`)
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: stompMissActee,
				Observer: messaging.Say(messaging.CategoryKick,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to stomp <ansi fg="username">%s</ansi>, but misses!`, mobName, target.Name)),
			}, aud)

		case actions.KickKnee:
			kneeMissActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					kneeMissActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to knee you, but you block it!`, mobName))
				} else {
					kneeMissActee = messaging.Say(messaging.CategoryKick, `Something tries to knee you, but you block it!`)
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: kneeMissActee,
				Observer: messaging.Say(messaging.CategoryKick,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to knee <ansi fg="username">%s</ansi>, but misses!`, mobName, target.Name)),
			}, aud)

		default: // KickStandard
			missActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					missActee = messaging.Say(messaging.CategoryKick, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to kick you, but misses!`, mobName))
				} else {
					missActee = messaging.Say(messaging.CategoryKick, `Something attempts to kick you, but misses!`)
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: missActee,
				Observer: messaging.Say(messaging.CategoryKick,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> attempts to kick <ansi fg="username">%s</ansi>, but misses!`, mobName, target.Name)),
			}, aud)
		}
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
