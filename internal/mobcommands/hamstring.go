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

// Hamstring is a wolf physical attack that applies a bleed condition.
func Hamstring(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if !mob.Character.IsInCombat() {
		return true, nil
	}

	res := actions.ExecuteHamstring(&actions.MobActor{Mob: mob, Room: room})
	if res.Cost.Status == characters.CostRefused {
		return true, nil
	}

	if res.OnCooldown || res.NoTarget || !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult

	mobName := mob.Character.Name
	targetPlayerId := target.UserId

	// Resolve the target user record for direct messaging (player targets).
	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

	canSee := targetChar == nil || canSeeInDark(targetChar, room)

	dmgDesc := combat.GetDamageDescription(result.Damage, result.TargetMaxHP)

	if result.Hit {
		if targetChar != nil {
			if canSee {
				targetChar.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> rakes its fangs across your legs, opening deep wounds! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				targetChar.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something rakes its fangs across your legs, opening deep wounds! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		room.SendTextVisual(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges low and rakes its fangs across <ansi fg="username">%s</ansi>'s legs!`, mobName, target.Name),
			targetPlayerId)
	} else if result.Damage > 0 {
		if targetChar != nil {
			if canSee {
				targetChar.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges at your legs and you dodge most of it, but the fangs still catch you! (<ansi fg="damage">%s</ansi>)`, mobName, dmgDesc))
			} else {
				targetChar.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`Something lunges at your legs and you dodge most of it, but it still catches you! (<ansi fg="damage">%s</ansi>)`, dmgDesc))
			}
		}
		if !sendMoveDefenceTriad(mob, room, target, result.Defence, "hamstring slash", messaging.CategoryHitNaturalSharp, true) {
			room.SendTextVisual(messaging.CategoryHitNaturalSharp,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges at <ansi fg="username">%s</ansi>'s legs, who mostly dodges but still gets caught!`, mobName, target.Name),
				targetPlayerId)
		}
	} else if !sendMoveDefenceTriad(mob, room, target, result.Defence, "hamstring slash", messaging.CategoryHitNaturalSharp, false) {
		if targetChar != nil {
			if canSee {
				targetChar.SendText(messaging.CategoryHitNaturalSharp, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges at your legs, but you sidestep the attack!`, mobName))
			} else {
				targetChar.SendText(messaging.CategoryHitNaturalSharp, `Something lunges at your legs, but you sidestep the attack!`)
			}
		}
		room.SendTextVisual(messaging.CategoryHitNaturalSharp,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> lunges at <ansi fg="username">%s</ansi>'s legs, but misses!`, mobName, target.Name),
			targetPlayerId)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(&actions.MobActor{Mob: mob, Room: room}, res.Counter)

	return true, nil
}
