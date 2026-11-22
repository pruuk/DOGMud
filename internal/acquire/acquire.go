// Package acquire is the one way an item is acquired (#375 phase 3a). It is
// a leaf package, not part of internal/actions, because actions imports
// internal/questengine and the quest bridge's give_item must acquire too.
package acquire

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// AcquireReason names how an item reached a player (#375 phase 3a). It rides
// on events.ItemOwnership, and an item_gain trigger's reason filter reads it
// (#375 phase 7), so "stole it" and "picked it up" are different events to a
// quest. quests.TriggerGainReasons lists the same words for the loader; a guard
// fails if the two differ, so a new reason goes in both.
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
//
// It is also the one place a copy is marked quest-given (#940, owner ruling
// 2026-11-22): a copy a quest hands out (AcquireTopic, AcquireQuestGive)
// fades away whenever it leaves its holder (items.Item.IsQuestGiven). A quest
// reward (AcquireReward) is paid once and is not marked.
func AcquireItemForChar(char *characters.Character, userId int, mobInstanceId int, itm items.Item, reason AcquireReason) bool {
	if MarksQuestGiven(reason) {
		itm.QuestGiven = true
	}
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

// MarksQuestGiven reports whether an item acquired for reason is a quest's
// hand-out, which AcquireItemForChar marks quest-given: a dialogue topic's
// givesItem and a quest trigger's give_item. Nothing else is marked.
func MarksQuestGiven(reason AcquireReason) bool {
	return reason == AcquireTopic || reason == AcquireQuestGive
}

// TooMuchToCarryLine is what a player reads when a new item falls at their
// feet because they cannot carry it (AcquireOrDrop).
const TooMuchToCarryLine = `Too much to carry: <ansi fg="itemname">%s</ansi> falls at your feet.`

// AcquireOrDrop acquires an item that already exists for the receiver (made,
// bought, foraged, salvaged, earned or given): when it is too much to carry,
// it falls at the receiver's feet instead of vanishing (#696), as a stolen
// bauble does (actions/steal.go). A player is told so (TooMuchToCarryLine).
// It reports whether the receiver carries it. A route that can still refuse
// cleanly (a pickup, a purchase it can roll back) checks AcquireItemForChar
// itself instead; no route may ignore a refusal
// (internal/guards/acquire_result_guard_test.go).
func AcquireOrDrop(char *characters.Character, userId int, mobInstanceId int, itm items.Item, reason AcquireReason) bool {
	// An item with no definition (a reward naming a retired id) is refused
	// for that, not its weight: nothing falls (#925 review).
	if itm.ItemId < 1 {
		return false
	}
	if AcquireItemForChar(char, userId, mobInstanceId, itm, reason) {
		return true
	}
	if char == nil {
		return false
	}
	if room := rooms.LoadRoom(char.RoomId); room != nil {
		room.AddItem(itm, false)
	}
	if user := users.GetByUserId(userId); userId > 0 && user != nil {
		user.SendText(messaging.CategoryWarning, fmt.Sprintf(TooMuchToCarryLine, itm.DisplayName()))
	}
	return false
}

// AcquireItemOrDrop is AcquireOrDrop for a player.
func AcquireItemOrDrop(user *users.UserRecord, itm items.Item, reason AcquireReason) bool {
	if user == nil {
		return false
	}
	return AcquireOrDrop(user.Character, user.UserId, 0, itm, reason)
}
