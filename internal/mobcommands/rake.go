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

// Rake is a clawed beast attack that deals damage and applies a bleed
// condition.
func Rake(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Must be in combat to use rake; silently skip if not in combat.
	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteRake(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// Anatomy/identity refusal: silently swallow so the btree can fall through.
	if res.NotClawed {
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
		if targetUser != nil {
			if canSee {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> rakes its claws across you, opening bleeding wounds! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something rakes its claws across you, opening bleeding wounds! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		room.SendTextVisual(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> rakes its claws across <ansi fg="username">%s</ansi>!`, mobName, target.Name),
			target.UserId)
	} else if result.Damage > 0 {
		if targetUser != nil {
			if canSee {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swipes its claws and you dodge most of it, but they still scratch you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something swipes at you and you dodge most of it, but it still scratches you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		if !sendMoveDefenceTriad(mob, room, target, result.Defence, "claw rake", messaging.CategoryHitNaturalSharp, true) {
			room.SendTextVisual(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swipes its claws at <ansi fg="username">%s</ansi>, who mostly dodges but still gets scratched!`, mobName, target.Name),
				target.UserId)
		}
	} else if !sendMoveDefenceTriad(mob, room, target, result.Defence, "claw rake", messaging.CategoryHitNaturalSharp, false) {
		if targetUser != nil {
			if canSee {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swipes its claws at you, but misses!`, mobName))
			} else {
				targetUser.SendText(messaging.CategoryHitNaturalSharp, `Something swipes its claws at you, but misses!`)
			}
		}
		room.SendTextVisual(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> swipes its claws at <ansi fg="username">%s</ansi>, but misses!`, mobName, target.Name),
			target.UserId)
	}

	return true, nil
}
