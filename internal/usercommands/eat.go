package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Eat(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if strings.TrimSpace(rest) == `` {
		refusal.Prompt(user, `eat`, refusal.What, `eat <item>`)
		return true, nil
	}

	if refuseWhileBusy(user, `eat`) {
		return true, nil
	}

	// Chunk 4e: can't eat while grappled — both hands committed.
	if user.Character.Position != nil && user.Character.Position.IsGrappling() {
		user.SendText(messaging.CategoryRefusal, `<ansi fg="red">Your hands are committed to the grapple, so you can't reach for that.</ansi>`)
		return true, nil
	}

	// Check whether the user has an item in their inventory that matches.
	// Edible-first: skip same-noun inedibles, but fall back to the
	// unfiltered match so the "can't eat that" rejection still fires when
	// nothing edible matches.
	matchItem, found := user.Character.FindInBackpackWhere(rest, func(it items.Item) bool {
		return it.GetSpec().Subtype == items.Edible
	})
	if !found {
		matchItem, found = user.Character.FindInBackpack(rest)
	}

	if !found {
		refusal.NotCarrying(user, rest)
	} else {

		itemSpec := matchItem.GetSpec()

		if itemSpec.Subtype != items.Edible {
			// A cooking ingredient (raw hare meat) says what to do with it
			// (2026-10-17 playtest: the refusal gave no pointer to cooking).
			if crafting.IsIngredientForSkill(matchItem, string(skills.Cooking)) {
				user.SendText(messaging.CategoryRefusal,
					fmt.Sprintf(`You can't eat <ansi fg="itemname">%s</ansi> as it is. Cook it first. Try <ansi fg="command">help cooking</ansi>.`, matchItem.DisplayName()),
				)
				return true, nil
			}
			user.SendText(messaging.CategoryRefusal,
				fmt.Sprintf(`You can't eat <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()),
			)
			return true, nil
		}

		// Food that has gone bad is refused; mob eat asks the same (#277).
		if matchItem.IsSpoiledFood(util.GetRoundCount()) {
			user.SendText(messaging.CategoryRefusal, `<ansi fg="red">The food has gone bad! It reeks of decay and is clearly inedible.</ansi>`)
			return true, nil
		}

		user.Character.CancelConditionsWithFlag(conditions.Hidden)

		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You eat some of the <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()))
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> eats some <ansi fg="itemname">%s</ansi>.`, user.Character.Name, matchItem.DisplayName()), user.UserId)

		// If no more uses, will be lost, so trigger event
		if usesLeft := user.Character.UseItem(matchItem); usesLeft < 1 {

			events.AddToQueue(events.ItemOwnership{
				UserId: user.UserId,
				Item:   matchItem,
				Gained: false,
			})

		}

		for _, conditionId := range itemSpec.ConditionIds {
			user.AddCondition(conditionId, `food`)
		}

	}

	return true, nil
}
