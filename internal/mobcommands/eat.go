package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Eat(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Chunk 4e: can't eat while grappled — both hands committed.
	if mob.Character.Position != nil && mob.Character.Position.IsGrappling() {
		return true, nil
	}

	if matchItem, found := mob.Character.FindInBackpack(rest); found {

		itemSpec := matchItem.GetSpec()

		if itemSpec.Subtype != items.Edible {
			return true, nil
		}

		// A mob leaves food that has gone bad, as a player is refused it.
		// No message: nobody asked it why (#277).
		if matchItem.IsSpoiledFood(util.GetRoundCount()) {
			return true, nil
		}

		mob.Character.CancelConditionsWithFlag(conditions.Hidden)

		mob.Character.UseItem(matchItem)

		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> eats some <ansi fg="itemname">%s</ansi>.`, mob.Character.Name, matchItem.DisplayName()))

		for _, conditionId := range itemSpec.ConditionIds {
			mob.AddCondition(conditionId, `food`)
		}
	}

	return true, nil
}
