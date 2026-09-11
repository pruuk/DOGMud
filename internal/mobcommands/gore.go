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

// Gore is a horned creature's charging strike: it deals damage and attempts
// to knock the target backward (Supine). Requires a gore natural attack
// (i.e. the species has horns). No bleed — gore is a knockdown opener.
func Gore(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use gore; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteGore(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// Anatomy/identity refusal: silently swallow so the btree can fall through.
	if res.NotHorned {
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
			// Hit + knockdown: horned charge tosses target off their feet.
			downActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					downActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lowers its head and charges into you, driving you to the ground! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					downActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something charges into you with bone-jarring force, hurling you to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: downActee,
				Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges into <ansi fg="username">%s</ansi> and hurls them to the ground!`, mobName, target.Name)),
			}, aud)
		} else {
			// Hit but no knockdown: the charge connects but target stays up.
			hitActee := messaging.NoLine
			if targetUser != nil {
				if canSee {
					hitActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> drives its horns into you with a powerful charge! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					hitActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something drives into you with a powerful charge! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.NoLine,
				Actee: hitActee,
				Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> drives its horns into <ansi fg="username">%s</ansi>!`, mobName, target.Name)),
			}, aud)
		}
	} else if result.Damage > 0 {
		partialActee := messaging.NoLine
		if targetUser != nil {
			if canSee {
				partialActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges you and you sidestep most of it, but the horns still catch you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				partialActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something charges you and you sidestep most of it, but it still catches you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		defence, defended := moveDefenceLines(mob, room, target, result.Defence, "goring charge")
		partialObserver := messaging.Say(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges <ansi fg="username">%s</ansi>, who mostly dodges but still gets grazed by the horns!`, mobName, target.Name))
		if defended {
			partialObserver = messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom)
			sendMoveDefenceShortage(targetUser, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    partialActee,
			Observer: partialObserver,
		}, aud)
	} else if defence, defended := moveDefenceLines(mob, room, target, result.Defence, "goring charge"); defended {
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
				missActee = messaging.Say(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at you, but you sidestep the gore!`, mobName))
			} else {
				missActee = messaging.Say(messaging.CategoryHitNaturalSharp, `Something charges at you, but you sidestep!`)
			}
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.NoLine,
			Actee: missActee,
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at <ansi fg="username">%s</ansi>, but misses!`, mobName, target.Name)),
		}, aud)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
