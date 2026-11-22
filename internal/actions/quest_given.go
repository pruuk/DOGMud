package actions

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Quest gear (#940, owner ruling 2026-11-22). A copy a quest hands out is
// marked quest-given (items.Item.IsQuestGiven; acquire.AcquireItemForChar sets
// it). While its holder keeps it, it is ordinary gear. Whenever it leaves
// their hands it fades away: drop, give to another player, put in a container,
// stash, storage and stow, mail, guild donate, auction, plant. Shops never buy
// it (QuoteSale) and a trade-in never takes it as payment. Giving it to an NPC
// that did not ask for it gets it handed back (usercommands.Give); a companion
// takes it as usual. internal/guards/quest_given_ways_out_guard_test.go keeps
// every RemoveItem call site classified, so a new way out cannot skip this.

// QuestGearFadesLine is what the holder reads when a quest-given copy fades
// away instead of leaving their hands. The item name arrives untagged.
const QuestGearFadesLine = `The <ansi fg="itemname">%s</ansi> fades away. Quest gear lasts only while you carry it.`

// questGearFadesRoomLine is what the room sees: the holder's name, tagged,
// and the item's name.
const questGearFadesRoomLine = `%s's <ansi fg="itemname">%s</ansi> fades away.`

// QuestGearSalvageLine is what the salvager reads when the item salvaged is
// quest gear: it is destroyed and yields nothing (salvageItem, and the
// salvage command, which does it at once).
const QuestGearSalvageLine = `The <ansi fg="itemname">%s</ansi> crumbles to nothing. Quest gear holds nothing worth salvaging.`

// QuestGearListTag is the short mark an inventory row puts after a quest's
// copy, in brackets: "Firebomb (quest gear)".
const QuestGearListTag = `quest gear`

// QuestGearLookLine is the extra line look and examine show under a
// quest-given copy, so no one loses one by surprise.
const QuestGearLookLine = `This is quest gear. It fades away if it leaves your hands.`

// ErrQuestGivenVanished is returned by a move (TransferItemToFloor,
// GiveItemToChar) whose quest-given item faded away instead of moving. The
// holder has been told (VanishQuestGiven); the caller prints nothing more for
// that item. Test with errors.Is.
var ErrQuestGivenVanished = errors.New("the quest-given item faded away instead of leaving its holder")

// VanishQuestGiven is the one way a quest-given copy leaves its holder. When
// itm is quest-given it removes it from char, queues the loss event, tells a
// player holder QuestGearFadesLine and, when room is non-nil, tells the room
// it faded; it then reports true and the caller must not move the item. When
// itm is not quest-given it does nothing and reports false, so a way out
// calls it first and moves the item only on false.
//
// Pass room when the way out would have shown the room the move (drop, give,
// put); pass nil for a quiet one (stash, storage, mail, guild, auction,
// plant).
func VanishQuestGiven(char *characters.Character, userId int, mobInstanceId int, itm items.Item, room *rooms.Room) bool {
	if !itm.IsQuestGiven() {
		return false
	}
	if char == nil || !char.RemoveItem(itm) {
		// Not held: nothing to fade, and nothing may move it either.
		return true
	}
	fadeQuestGiven(char, userId, mobInstanceId, itm, room)
	return true
}

// FadeQuestGivenOffHand is VanishQuestGiven for a copy already out of its
// holder's carry lists, on its way to the floor without going through
// RemoveItem: displaced by an equip while overloaded, taken off while over
// twice carry capacity, refused by the pet it came from, or a spoiled bomb
// out of the bandolier. For a quest-given itm it fades (the loss and the
// lines, as VanishQuestGiven) and reports true, and the caller must not put
// it on the floor; for any other item it does nothing and reports false.
// char names the holder for the room line and may be nil.
func FadeQuestGivenOffHand(char *characters.Character, userId int, mobInstanceId int, itm items.Item, room *rooms.Room) bool {
	if !itm.IsQuestGiven() {
		return false
	}
	fadeQuestGiven(char, userId, mobInstanceId, itm, room)
	return true
}

// fadeQuestGiven is the fade itself, once the copy is out of the holder's
// carry lists: the loss event, the holder's line and the room's.
func fadeQuestGiven(char *characters.Character, userId int, mobInstanceId int, itm items.Item, room *rooms.Room) {
	events.AddToQueue(events.ItemOwnership{
		UserId:        userId,
		MobInstanceId: mobInstanceId,
		Item:          itm,
		Gained:        false,
	})
	if userId > 0 {
		if u := users.GetByUserId(userId); u != nil {
			u.SendText(messaging.CategoryWarning, fmt.Sprintf(QuestGearFadesLine, itm.DisplayName()))
		}
	}
	if room != nil && char != nil {
		nameTag := `mobname`
		if userId > 0 {
			nameTag = `username`
		}
		room.SendTextVisualHidingNames(messaging.CategoryLoot,
			fmt.Sprintf(questGearFadesRoomLine, `<ansi fg="`+nameTag+`">`+char.Name+`</ansi>`, itm.DisplayName()),
			[]string{char.Name},
			userId)
	}
}

// VanishQuestGivenFrom is VanishQuestGiven for an Actor.
func VanishQuestGivenFrom(actor Actor, itm items.Item, room *rooms.Room) bool {
	return VanishQuestGiven(actor.GetCharacter(), actor.GetUserId(), actor.GetMobInstanceId(), itm, room)
}

// questGivenLeaves reports whether a quest-given copy moving from one
// character to another leaves the side of the player it belongs with, and
// so fades (TransferItemBetweenChars). The copy stays when:
//   - a behaviour tree hands back the item it was just given
//     (acquire.AcquireTreeGive, the tree's return_item);
//   - a player hands it to their own companion (a mob they charmed);
//   - a companion hands it back to the player who charmed it.
//
// Anything else lets it go: a player to another player or to any other mob
// (usercommands.Give never sends an NPC's give here: it hands quest gear
// back itself), and a mob to anyone but its charmer.
func questGivenLeaves(fromChar *characters.Character, fromUserId int, toChar *characters.Character, toUserId int, reason acquire.AcquireReason) bool {
	if reason == acquire.AcquireTreeGive {
		return false
	}
	if fromUserId > 0 {
		if toUserId > 0 {
			return toUserId != fromUserId
		}
		return toChar == nil || !toChar.IsCharmed(fromUserId)
	}
	charmer := 0
	if fromChar != nil {
		charmer = fromChar.GetCharmedUserId()
	}
	return charmer == 0 || toUserId != charmer
}
