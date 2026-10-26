// Package acquire is the one way an item is acquired (#375 phase 3a). It is
// a leaf package, not part of internal/actions, because actions imports
// internal/questengine and the quest bridge's give_item must acquire too.
package acquire

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// AcquireReason names how an item reached a player (#375 phase 3a). It rides
// on events.ItemOwnership so a later trigger filter can tell "crafted it" from
// "bought it" without another refactor.
type AcquireReason string

const (
	AcquireGet        AcquireReason = `get`
	AcquireGive       AcquireReason = `give`
	AcquireBuy        AcquireReason = `buy`
	AcquireLoot       AcquireReason = `loot`
	AcquireForage     AcquireReason = `forage`
	AcquireHarvest    AcquireReason = `harvest`
	AcquireSteal      AcquireReason = `steal`
	AcquireSearch     AcquireReason = `search_find`
	AcquireStorage    AcquireReason = `storage`
	AcquireVault      AcquireReason = `vault`
	AcquireAuction    AcquireReason = `auction`
	AcquireTopic      AcquireReason = `topic_give`
	AcquireCraft      AcquireReason = `craft`
	AcquireSalvage    AcquireReason = `salvage`
	AcquireReward     AcquireReason = `reward`
	AcquireQuestGive  AcquireReason = `quest_give`
	AcquireTreeGive   AcquireReason = `tree_give`
	AcquireMail       AcquireReason = `mail`
	AcquireShieldSwap AcquireReason = `shield_swap`
	AcquirePlant      AcquireReason = `plant` // a thief slipped it into the holder's pack
)

// AcquireItem is the one way an item is acquired by a player: it stores the
// item and, only when it landed, raises ItemOwnership{Gained} so quest
// item_gain triggers and every other listener hear it. A guard
// (internal/guards/acquire_seam_guard_test.go) forbids StoreItem anywhere
// else except named non-acquisition moves.
func AcquireItem(user *users.UserRecord, itm items.Item, reason AcquireReason) bool {
	if user == nil {
		return false
	}
	return AcquireItemForChar(user.Character, user.UserId, 0, itm, reason)
}

// AcquireItemForChar is AcquireItem for a route that holds a bare character
// and its ids rather than a UserRecord. The transfer helpers
// (actions/transfer.go) are the reason it exists: they take a character plus userId and
// mobInstanceId, and they serve mob receivers (a player giving to a mob) as
// well as players, so the gain event keeps whichever id the caller passed.
func AcquireItemForChar(char *characters.Character, userId int, mobInstanceId int, itm items.Item, reason AcquireReason) bool {
	if char == nil || !char.StoreItem(itm) {
		return false
	}
	events.AddToQueue(events.ItemOwnership{
		UserId:        userId,
		MobInstanceId: mobInstanceId,
		Item:          itm,
		Gained:        true,
		Reason:        string(reason),
	})
	return true
}
