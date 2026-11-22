package actions

import (
	"errors"
	"fmt"

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
	if room != nil {
		nameTag := `mobname`
		if userId > 0 {
			nameTag = `username`
		}
		room.SendTextVisualHidingNames(messaging.CategoryLoot,
			fmt.Sprintf(questGearFadesRoomLine, `<ansi fg="`+nameTag+`">`+char.Name+`</ansi>`, itm.DisplayName()),
			[]string{char.Name},
			userId)
	}
	return true
}

// VanishQuestGivenFrom is VanishQuestGiven for an Actor.
func VanishQuestGivenFrom(actor Actor, itm items.Item, room *rooms.Room) bool {
	return VanishQuestGiven(actor.GetCharacter(), actor.GetUserId(), actor.GetMobInstanceId(), itm, room)
}

// questGivenLeavesOnGive reports whether a quest-given copy the actor gives
// leaves the player's side, and so fades. A player giving to another player
// lets it go; a player giving to an NPC is decided by usercommands.Give
// before it gets here (a topic, a tree handler, a companion, or a hand-back).
// A mob gives it away unless it hands it to the player who charmed it: a
// companion returning gear to its master keeps the copy.
func questGivenLeavesOnGive(actor Actor, toUserId int) bool {
	if actor.IsPlayer() {
		return toUserId > 0 && toUserId != actor.GetUserId()
	}
	charmer := actor.GetCharacter().GetCharmedUserId()
	return charmer == 0 || toUserId != charmer
}
