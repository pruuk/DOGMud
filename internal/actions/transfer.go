package actions

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// ErrInventoryFull is returned by TransferItemBetweenChars (and so
// GiveItemToChar) and by TransferItemToBackpack when the receiver cannot
// carry the item. It is a refusal for the giver, not a server fault; test
// with errors.Is.
var ErrInventoryFull = errors.New("destination inventory capacity exceeded")

// TransferItemToFloor moves an item from a character's inventory to the room floor.
// Returns an error if the item cannot be removed from the character's inventory.
func TransferItemToFloor(item items.Item, char *characters.Character, userId int, mobInstanceId int, room *rooms.Room) error {
	// Remove item from character
	if !char.RemoveItem(item) {
		return fmt.Errorf("item not found in inventory")
	}

	// Add item to room floor (always succeeds)
	room.AddItem(item, false)

	// Fire ItemOwnership event for loss
	events.AddToQueue(events.ItemOwnership{
		UserId:        userId,
		MobInstanceId: mobInstanceId,
		Item:          item,
		Gained:        false,
	})

	return nil
}

// TransferItemToBackpack moves an item from a source (e.g., room floor) to a character's backpack.
// Parameters:
//   - item: the item being transferred
//   - toChar: the character receiving the item
//   - toUserId: user ID of the receiving character (0 for mobs)
//   - toMobInstanceId: mob instance ID of the receiving character (0 for players)
//   - removeFrom: function to remove the item from the source
//   - rollback: function to restore the item to the source if storage fails
//
// Returns an error if capacity is exceeded.
func TransferItemToBackpack(item items.Item, toChar *characters.Character, toUserId int, toMobInstanceId int, removeFrom func(items.Item), rollback func(items.Item)) error {
	// Remove from source
	removeFrom(item)

	// Store through the acquire seam, which raises the gain event only when
	// the item lands. Its one caller is the floor pickup (TakeFloorItem).
	if !acquire.AcquireItemForChar(toChar, toUserId, toMobInstanceId, item, acquire.AcquireGet) {
		// Restore to source on failure
		rollback(item)
		return ErrInventoryFull
	}

	return nil
}

// TransferItemBetweenChars moves an item from one character to another.
// Rolls back if the destination cannot accept the item.
// Returns an error if the item cannot be removed from the source or stored in the destination.
// reason rides on the receiver's gain event: give for player and mob give,
// tree_give for the behaviour tree's return_item.
func TransferItemBetweenChars(item items.Item, fromChar *characters.Character, fromUserId int, fromMobInstanceId int, toChar *characters.Character, toUserId int, toMobInstanceId int, reason acquire.AcquireReason) error {
	// Remove from source
	if !fromChar.RemoveItem(item) {
		return fmt.Errorf("item not found in source inventory")
	}

	// Store through the acquire seam, which raises the receiver's gain event
	// only when the item lands, carrying the caller's reason.
	if !acquire.AcquireItemForChar(toChar, toUserId, toMobInstanceId, item, reason) {
		// Rollback: restore to source. Not an acquisition: the item never
		// left the source's ownership as far as any listener heard.
		if !fromChar.StoreItem(item) {
			// This should never happen since we just removed it
			return fmt.Errorf("critical error: failed to rollback item to source")
		}
		return ErrInventoryFull
	}

	// The giver's loss event. It now queues after the receiver's gain (the
	// seam queues on store); listeners key on each event's own owner, so the
	// order between two owners' events carries nothing.
	events.AddToQueue(events.ItemOwnership{
		UserId:        fromUserId,
		MobInstanceId: fromMobInstanceId,
		Item:          item,
		Gained:        false,
	})

	return nil
}

// TransferGold moves gold from one character to another.
// Validates that the source has sufficient gold.
// Returns an error if the amount is invalid or the source has insufficient funds.
func TransferGold(amount int, fromChar *characters.Character, toChar *characters.Character) error {
	if amount <= 0 {
		return fmt.Errorf("transfer amount must be positive")
	}

	if fromChar.Gold < amount {
		return fmt.Errorf("insufficient gold")
	}

	fromChar.Gold -= amount
	toChar.Gold += amount

	return nil
}

// FloorPickupGold moves gold from the room floor to a character.
// Validates that the amount is positive and available on the floor.
// Returns an error if the amount is invalid or insufficient gold is on the floor.
func FloorPickupGold(amount int, room *rooms.Room, char *characters.Character) error {
	if amount <= 0 {
		return fmt.Errorf("pickup amount must be positive")
	}

	if room.Gold < amount {
		return fmt.Errorf("insufficient gold on floor")
	}

	room.Gold -= amount
	char.Gold += amount

	return nil
}

// FloorDropGold moves gold from a character to the room floor.
// Validates that the character has sufficient gold and the amount is positive.
// Returns an error if the amount is invalid or the character has insufficient gold.
func FloorDropGold(amount int, char *characters.Character, room *rooms.Room) error {
	if amount <= 0 {
		return fmt.Errorf("drop amount must be positive")
	}

	if char.Gold < amount {
		return fmt.Errorf("insufficient gold in inventory")
	}

	char.Gold -= amount
	room.Gold += amount

	return nil
}
