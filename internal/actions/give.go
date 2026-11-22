package actions

import (
	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// GiveItemResult is the result of a GiveItemToChar call.
type GiveItemResult struct {
	Item  items.Item
	Found bool
	Err   error
}

// GiveItemToChar searches the actor's backpack for an item matching itemName,
// then atomically transfers it to toChar using TransferItemBetweenChars.
// If no matching item is found, Found is false and no transfer occurs.
// A quest-given copy that would leave the player's side (another player, or
// a mob giving it to anyone but its charmer) fades away instead and Err is
// ErrQuestGivenVanished (#940); questGivenLeavesOnGive says when.
func GiveItemToChar(actor Actor, itemName string, toChar *characters.Character, toUserId int, toMobInstanceId int) GiveItemResult {
	fromChar := actor.GetCharacter()

	matchItem, found := fromChar.FindInBackpack(itemName)
	if !found {
		return GiveItemResult{Found: false}
	}

	if matchItem.IsQuestGiven() && questGivenLeavesOnGive(actor, toUserId) {
		VanishQuestGivenFrom(actor, matchItem, actor.GetRoom())
		return GiveItemResult{Item: matchItem, Found: true, Err: ErrQuestGivenVanished}
	}

	err := TransferItemBetweenChars(
		matchItem,
		fromChar,
		actor.GetUserId(),
		actor.GetMobInstanceId(),
		toChar,
		toUserId,
		toMobInstanceId,
		acquire.AcquireGive,
	)

	return GiveItemResult{Item: matchItem, Found: true, Err: err}
}

// GiveGoldToChar transfers the given amount of gold from the actor's character
// to the target character. Delegates to TransferGold for validation.
func GiveGoldToChar(actor Actor, amount int, to *characters.Character) error {
	return TransferGold(amount, actor.GetCharacter(), to)
}
