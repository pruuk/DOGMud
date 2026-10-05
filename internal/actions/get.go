package actions

import (
	"errors"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// ErrHouseholdBauble refuses taking a household's bauble off the floor: it
// belongs to the house, and taking it is theft, which only `steal`
// attempts (stealHouseholdBauble). Every taker is held to it, a player's
// `get`, a mob's, a companion's or a scavenger's (owner ruling 2026-09-29).
var ErrHouseholdBauble = errors.New(`that belongs to this household`)

// ErrTooDark refuses a pickup by an actor who sees nothing at all here
// (slice 5a). Shapes are enough to grope for an item, so only SightNone
// refuses.
var ErrTooDark = errors.New(`too dark to find anything`)

// ErrExploding refuses an item that is about to explode; a sweep stops on it.
var ErrExploding = errors.New(`it is about to explode`)

// ErrFixture refuses a fixture: an item fixed to the room's floor (ItemSpec
// fixture, lighting 5e Rule 10). Every taker is held to it, a player's
// `get`, a mob's, a companion's or a scavenger's; `get all`, `steal` and a
// mob's floor equip never reach for one at all.
var ErrFixture = errors.New(`it is fixed in place`)

// TooDarkToGet is the one statement of the pickup sight rule, for both
// actors. The player's `get` also asks it first, so its container, corpse
// and bag branches stay refused in the dark.
func TooDarkToGet(actor Actor) bool {
	return messaging.ParticipantSight(actor.GetCharacter(), actor.GetRoom()) == messaging.SightNone
}

// GetItemResult is the result of a GetItemFromFloor call.
type GetItemResult struct {
	Item  items.Item
	Found bool
	Err   error
}

// TakeFloorItem moves an item already found on the floor (or in the stash)
// into the actor's backpack, through every pickup gate in the player's order:
// ErrTooDark, ErrFixture, ErrExploding, ErrHouseholdBauble, then the transfer
// (which fires ItemOwnership, or rolls back on a full pack).
func TakeFloorItem(actor Actor, item items.Item, stash bool) error {
	if TooDarkToGet(actor) {
		return ErrTooDark
	}
	if item.IsFixture() {
		return ErrFixture
	}
	if item.HasAdjective(`exploding`) {
		return ErrExploding
	}
	room := actor.GetRoom()
	// A household's bauble is never picked up (it is only ever on the floor,
	// never in a stash).
	if !stash && item.BaubleBelongsTo(room.RoomId) {
		return ErrHouseholdBauble
	}
	return TransferItemToBackpack(
		item,
		actor.GetCharacter(),
		actor.GetUserId(),
		actor.GetMobInstanceId(),
		func(i items.Item) { room.RemoveItem(i, stash) },
		func(i items.Item) { room.AddItem(i, stash) },
	)
}

// FindTakeableOnFloor is room.FindOnFloor for a pickup: a match that is not
// a fixture wins over a fixture the same words name, wherever the fixture
// sits in floor order. Only when fixtures are all the words match does it
// return one, so the caller can refuse it as fixed in place.
func FindTakeableOnFloor(room *rooms.Room, itemName string, stash bool) (items.Item, bool) {
	if !stash {
		loose := make([]items.Item, 0, len(room.Items))
		for _, it := range room.Items {
			if !it.IsFixture() {
				loose = append(loose, it)
			}
		}
		if len(loose) != len(room.Items) {
			closeMatch, match := items.FindMatchIn(itemName, loose...)
			if match.ItemId != 0 {
				return match, true
			}
			if closeMatch.ItemId != 0 {
				return closeMatch, true
			}
		}
	}
	return room.FindOnFloor(itemName, stash)
}

// GetItemFromFloor searches the room floor (or stash) for an item matching
// itemName and takes it through TakeFloorItem. In the dark it finds nothing
// (Found false, ErrTooDark): the actor learns nothing about the floor. Every
// other refusal returns the item found, Found, and the gate's error.
func GetItemFromFloor(actor Actor, itemName string, stash bool) GetItemResult {
	if TooDarkToGet(actor) {
		return GetItemResult{Found: false, Err: ErrTooDark}
	}
	matchItem, found := FindTakeableOnFloor(actor.GetRoom(), itemName, stash)
	if !found {
		return GetItemResult{Found: false}
	}
	return GetItemResult{Item: matchItem, Found: true, Err: TakeFloorItem(actor, matchItem, stash)}
}

// GetGoldFromFloor moves gold on the room floor into the actor's wallet
// (FloorPickupGold validates the amount); refused with ErrTooDark when the
// actor sees nothing.
func GetGoldFromFloor(actor Actor, amount int) error {
	if TooDarkToGet(actor) {
		return ErrTooDark
	}
	return FloorPickupGold(amount, actor.GetRoom(), actor.GetCharacter())
}
