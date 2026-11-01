package actions

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// ConsumeFood is the one food body for players and mobs: eat and drink are one
// command (#375 phase 7, owner ruling 2026-10-30), and both take food (type
// food) and cooked drinks (type drink), every one of which is subtype edible.
// Potions are drunk with quaff (Quaff); eat or drink aimed at one consumes
// nothing and names quaff. verb is the word typed; it words the prompt and the
// refusals. The success line says eat or drink by the item's type, whichever
// word was typed. SendText is a no-op for a mob, so a mob's refusals are silent
// and it eats silently apart from the room line.
func ConsumeFood(actor Actor, verb string, rest string) {
	char := actor.GetCharacter()

	room := actor.GetRoom()
	if room == nil {
		room = rooms.LoadRoom(char.RoomId)
	}

	if strings.TrimSpace(rest) == `` {
		refusal.Prompt(actor, verb, refusal.What, verb+` <item>`)
		return
	}

	if char.IsActing() {
		refusal.WhileBusy(actor, verb)
		return
	}

	// Chunk 4e: can't eat or drink while grappled, both hands committed.
	if char.Position != nil && char.Position.IsGrappling() {
		actor.SendText(messaging.CategoryRefusal, `<ansi fg="red">Your hands are committed to the grapple, so you can't reach for that.</ansi>`)
		return
	}

	matchItem, fromBandolier, found := findFood(char, rest)
	if !found {
		refusal.NotCarrying(actor, rest)
		return
	}

	itemSpec := matchItem.GetSpec()

	if itemSpec.Subtype != items.Edible {
		// A potion: nothing is consumed, and the line names quaff.
		if itemSpec.Subtype == items.Drinkable {
			refusal.QuaffPotion(actor, rest)
			return
		}
		// A cooking ingredient (raw hare meat) says what to do with it
		// (2026-10-17 playtest: the refusal gave no pointer to cooking).
		if crafting.IsIngredientForSkill(matchItem, string(skills.Cooking)) {
			actor.SendText(messaging.CategoryRefusal,
				fmt.Sprintf(`You can't %s <ansi fg="itemname">%s</ansi> as it is. Cook it first. Try <ansi fg="command">help cooking</ansi>.`, verb, matchItem.DisplayName()),
			)
			return
		}
		actor.SendText(messaging.CategoryRefusal,
			fmt.Sprintf(`You can't %s <ansi fg="itemname">%s</ansi>.`, verb, matchItem.DisplayName()),
		)
		return
	}

	// Food that has gone bad is refused, to a mob as to a player (#277).
	if matchItem.IsSpoiledFood(util.GetRoundCount()) {
		actor.SendText(messaging.CategoryRefusal, `<ansi fg="red">The food has gone bad! It reeks of decay and is clearly inedible.</ansi>`)
		return
	}

	char.CancelConditionsWithFlag(conditions.Hidden)

	// The room line names a player in the username colour and excludes them
	// (they read their own line); a mob is named in the mob colour and the
	// line goes to everyone. As in Quaff, the line goes out through
	// SendTextVisualHidingNames with the eater's name, so an observer at shapes
	// reads "a figure" even where the name stands without its tag.
	nameColor := `mobname`
	var roomExclude []int
	if actor.IsPlayer() {
		nameColor = `username`
		roomExclude = []int{actor.GetUserId()}
	}

	you, they := `eat`, `eats`
	if itemSpec.Type == items.Drink {
		you, they = `drink`, `drinks`
	}
	actor.SendText(messaging.CategorySystem, fmt.Sprintf(`You %s some of the <ansi fg="itemname">%s</ansi>.`, you, matchItem.DisplayName()))
	if room != nil {
		room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="%s">%s</ansi> %s some <ansi fg="itemname">%s</ansi>.`, nameColor, char.Name, they, matchItem.DisplayName()), []string{char.Name}, roomExclude...)
	}

	usesLeft := 0
	if fromBandolier {
		usesLeft = char.UseItemFromPotions(matchItem)
	} else {
		usesLeft = char.UseItem(matchItem)
	}
	// Last use gone: the item is lost. Only a player's inventory listens.
	if usesLeft < 1 && actor.IsPlayer() {
		events.AddToQueue(events.ItemOwnership{
			UserId: actor.GetUserId(),
			Item:   matchItem,
			Gained: false,
		})
	}

	for _, conditionId := range itemSpec.ConditionIds {
		actor.AddCondition(conditionId, `food`)
	}

	// Quest engine: command notification, after the eat lines and the meal's
	// effects, so a step line reads after "You eat" (the order #697 asked of
	// quaff). eat and drink are one command, so both verbs raise "eat". Quest
	// 42's eat step listens for it (#375 phase 7). Only a player has quests,
	// as in Quaff, so a mob eating raises nothing.
	if ua, ok := actor.(*UserActor); ok && room != nil {
		questengine.NotifyCommand(ua.User, room.RoomId, "eat", questengine.TargetItem(matchItem.ItemId))
	}
}

// findFood resolves name for the food body: edible in the pack, then edible in
// the bandolier (where a Herbal Tea bought while it was a potion may still
// sit; it was retyped a drink in #375 phase 7), then a potion in either, then
// anything else carried, so the refusals still fire when nothing edible
// matches. An edible match always wins, so `drink tea` drinks the tea even with
// a "Lake-tonic of Steady Hand" beside it. fromBandolier says which store to
// consume from.
func findFood(char *characters.Character, name string) (matchItem items.Item, fromBandolier bool, found bool) {
	isEdible := func(it items.Item) bool { return it.GetSpec().Subtype == items.Edible }
	isPotion := func(it items.Item) bool { return it.GetSpec().Subtype == items.Drinkable }

	for _, keep := range []func(items.Item) bool{isEdible, isPotion, nil} {
		if it, ok := char.FindInBackpackWhere(name, keep); ok {
			return it, false, true
		}
		if it, ok := char.FindInPotionsWhere(name, keep); ok {
			return it, true, true
		}
	}
	return items.Item{}, false, false
}
