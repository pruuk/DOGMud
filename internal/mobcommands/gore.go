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

	if result.Hit {
		if result.KnockedDown {
			// Hit + knockdown: horned charge tosses target off their feet.
			if targetUser != nil {
				if canSee {
					targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lowers its head and charges into you, driving you to the ground! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something charges into you with bone-jarring force, hurling you to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			room.SendTextVisual(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges into <ansi fg="username">%s</ansi> and hurls them to the ground!`, mobName, target.Name),
				target.UserId)
		} else {
			// Hit but no knockdown: the charge connects but target stays up.
			if targetUser != nil {
				if canSee {
					targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> drives its horns into you with a powerful charge! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
				} else {
					targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something drives into you with a powerful charge! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
				}
			}
			room.SendTextVisual(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> drives its horns into <ansi fg="username">%s</ansi>!`, mobName, target.Name),
				target.UserId)
		}
	} else if result.Damage > 0 {
		if targetUser != nil {
			if canSee {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges you and you sidestep most of it, but the horns still catch you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something charges you and you sidestep most of it, but it still catches you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		if !sendMoveDefenceTriad(mob, room, target, result.Defence, "goring charge", messaging.CategoryHitNaturalSharp, true) {
			room.SendTextVisual(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges <ansi fg="username">%s</ansi>, who mostly dodges but still gets grazed by the horns!`, mobName, target.Name),
				target.UserId)
		}
	} else if !sendMoveDefenceTriad(mob, room, target, result.Defence, "goring charge", messaging.CategoryHitNaturalSharp, false) {
		if targetUser != nil {
			if canSee {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at you, but you sidestep the gore!`, mobName))
			} else {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, `Something charges at you, but you sidestep!`)
			}
		}
		room.SendTextVisual(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> charges at <ansi fg="username">%s</ansi>, but misses!`, mobName, target.Name),
			target.UserId)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
