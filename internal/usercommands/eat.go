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

// Eat is the food command. eat and drink are one command (#375 phase 7,
// owner ruling 2026-10-30): both take food (type food) and cooked drinks
// (type drink), every one of which is subtype edible. Potions are drunk with
// quaff (actions.Quaff); eat or drink aimed at one consumes nothing and
// names quaff.
func Eat(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	return consumeFood(`eat`, rest, user, room)
}

// consumeFood is the body of eat and drink. verb is the word the player
// typed; it words the prompt and the refusals. The success line says eat or
// drink by the item's type, whichever word was typed.
func consumeFood(verb string, rest string, user *users.UserRecord, room *rooms.Room) (bool, error) {
	if strings.TrimSpace(rest) == `` {
		refusal.Prompt(user, verb, refusal.What, verb+` <item>`)
		return true, nil
	}

	if refuseWhileBusy(user, verb) {
		return true, nil
	}

	// Chunk 4e: can't eat or drink while grappled, both hands committed.
	if user.Character.Position != nil && user.Character.Position.IsGrappling() {
		user.SendText(messaging.CategoryRefusal, `<ansi fg="red">Your hands are committed to the grapple, so you can't reach for that.</ansi>`)
		return true, nil
	}

	// Edible-first: skip same-noun inedibles. Next the bandolier, where a
	// Herbal Tea bought while it was a potion may still sit (it was retyped a
	// drink in #375 phase 7). Then the unfiltered match, so the refusals
	// below still fire when nothing edible matches.
	fromBandolier := false
	matchItem, found := user.Character.FindInBackpackWhere(rest, func(it items.Item) bool {
		return it.GetSpec().Subtype == items.Edible
	})
	if !found {
		if it, ok := user.Character.FindInPotions(rest); ok && it.GetSpec().Subtype == items.Edible {
			matchItem, found, fromBandolier = it, true, true
		}
	}
	if !found {
		matchItem, found = user.Character.FindInBackpack(rest)
	}
	if !found {
		// A potion carried only in the bandolier gets the quaff line too.
		if it, ok := user.Character.FindInPotions(rest); ok && it.GetSpec().Subtype == items.Drinkable {
			refusal.QuaffPotion(user, rest)
			return true, nil
		}
		refusal.NotCarrying(user, rest)
		return true, nil
	}

	itemSpec := matchItem.GetSpec()

	if itemSpec.Subtype != items.Edible {
		// A potion: nothing is consumed, and the line names quaff.
		if itemSpec.Subtype == items.Drinkable {
			refusal.QuaffPotion(user, rest)
			return true, nil
		}
		// A cooking ingredient (raw hare meat) says what to do with it
		// (2026-10-17 playtest: the refusal gave no pointer to cooking).
		if crafting.IsIngredientForSkill(matchItem, string(skills.Cooking)) {
			user.SendText(messaging.CategoryRefusal,
				fmt.Sprintf(`You can't %s <ansi fg="itemname">%s</ansi> as it is. Cook it first. Try <ansi fg="command">help cooking</ansi>.`, verb, matchItem.DisplayName()),
			)
			return true, nil
		}
		user.SendText(messaging.CategoryRefusal,
			fmt.Sprintf(`You can't %s <ansi fg="itemname">%s</ansi>.`, verb, matchItem.DisplayName()),
		)
		return true, nil
	}

	// Food that has gone bad is refused; mob eat asks the same (#277).
	if matchItem.IsSpoiledFood(util.GetRoundCount()) {
		user.SendText(messaging.CategoryRefusal, `<ansi fg="red">The food has gone bad! It reeks of decay and is clearly inedible.</ansi>`)
		return true, nil
	}

	user.Character.CancelConditionsWithFlag(conditions.Hidden)

	you, they := `eat`, `eats`
	if itemSpec.Type == items.Drink {
		you, they = `drink`, `drinks`
	}
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`You %s some of the <ansi fg="itemname">%s</ansi>.`, you, matchItem.DisplayName()))
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> %s some <ansi fg="itemname">%s</ansi>.`, user.Character.Name, they, matchItem.DisplayName()), user.UserId)

	// If no more uses, will be lost, so trigger event
	usesLeft := 0
	if fromBandolier {
		usesLeft = user.Character.UseItemFromPotions(matchItem)
	} else {
		usesLeft = user.Character.UseItem(matchItem)
	}
	if usesLeft < 1 {
		events.AddToQueue(events.ItemOwnership{
			UserId: user.UserId,
			Item:   matchItem,
			Gained: false,
		})
	}

	for _, conditionId := range itemSpec.ConditionIds {
		user.AddCondition(conditionId, `food`)
	}

	return true, nil
}
