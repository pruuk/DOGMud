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

// Pounce is a quadruped predator's leaping opener: it deals bonus damage and
// attempts to knock the target backward (Supine). Requires legs and a
// fanged or clawed natural attack.
func Pounce(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use pounce; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecutePounce(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// Grappling gate: silently swallow so the btree can fall through.
	if res.Grappling {
		return true, nil
	}
	// Anatomy/identity refusal: silently swallow so the btree can fall through.
	if res.NotPredator {
		return true, nil
	}
	// Any other early-exit condition (OnCooldown, NoTarget): silently return.
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

	if result.Hit {
		if result.KnockedDown {
			// Hit + knockdown: predator bears prey to the ground.
			downActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					downActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaps at you and slams you to the ground! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					downActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something leaps at you and slams you to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: downActee,
				Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaps at <ansi fg="username">%s</ansi> and slams them to the ground!`, mobName, target.Name)),
			}, aud)
		} else {
			// Hit but no knockdown: landed but target kept their feet.
			hitActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					hitActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> springs at you and crashes into your body! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					hitActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something springs at you and crashes into your body! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: hitActee,
				Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> springs at <ansi fg="username">%s</ansi> and crashes into them!`, mobName, target.Name)),
			}, aud)
		}
	} else if result.Damage > 0 {
		partialActee := messaging.NoLine
		if targetUser != nil {
			if canSee {
				partialActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaps at you and you sidestep most of it, but it still clips you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				partialActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something leaps at you and you sidestep most of it, but it still clips you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, "pounce")
		partialObserver := messaging.Say(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaps at <ansi fg="username">%s</ansi>, who mostly dodges but still gets clipped!`, mobName, target.Name))
		if defended {
			partialObserver = messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom)
			sendMoveDefenceShortage(targetUser, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    partialActee,
			Observer: partialObserver,
		}, aud)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, "pounce"); defended {
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
				missActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaps at you, but you sidestep the pounce!`, mobName))
			} else {
				missActee = messaging.Say(messaging.CategoryHitNaturalSharp, `Something leaps at you, but you sidestep!`)
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: missActee,
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaps at <ansi fg="username">%s</ansi>, but misses!`, mobName, target.Name)),
		}, aud)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
