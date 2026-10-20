package conversation

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// PlayerState creates a PlayerState from the user's character for
// quest/item gating in dialogue calls.
func PlayerState(user *users.UserRecord) *dialogue.PlayerState {
	return playerState(user, user.SendText)
}

// playerState is PlayerState with the callbacks' private lines (a delivery,
// gold received or paid, a refusal) sent through send. The talk flow passes
// a lineBuffer's add so those lines land after the handover line and before
// the NPC's reply.
func playerState(user *users.UserRecord, send func(messaging.Category, string)) *dialogue.PlayerState {
	return &dialogue.PlayerState{
		HasQuest: user.Character.HasQuest,
		HasItem: func(itemId int) bool {
			for _, itm := range user.Character.GetAllBackpackItems() {
				if itm.ItemId == itemId {
					return true
				}
			}
			return false
		},
		// Backs itemExcluded: anywhere on the player counts, so a wielded
		// sling or a pouch in the bandolier still means "already has one".
		CarriesItem: func(itemId int) bool {
			for _, itm := range user.Character.GetAllCarriedItems() {
				if itm.ItemId == itemId {
					return true
				}
			}
			for _, itm := range user.Character.GetAllWornItems() {
				if itm.ItemId == itemId {
					return true
				}
			}
			return false
		},
		RemoveItem: func(itemId int) bool {
			for _, itm := range user.Character.GetAllBackpackItems() {
				if itm.ItemId != itemId {
					continue
				}
				if !user.Character.RemoveItem(itm) {
					return false
				}
				// The same loss event give.go queues when a hand-in is
				// consumed, so item-loss listeners see a talk hand-in too.
				events.AddToQueue(events.ItemOwnership{
					UserId: user.UserId,
					Item:   itm,
					Gained: false,
				})
				return true
			}
			return false
		},
		GiveQuest: func(token string) {
			events.AddToQueue(events.Quest{
				UserId:     user.UserId,
				QuestToken: token,
			})
		},
		GiveItem: func(itemId int) bool {
			newItem := items.New(itemId)
			if newItem.ItemId <= 0 {
				// Data bug (givesItem points at no item). Report failure so
				// the engine also withholds grantsQuest: granting a quest
				// whose item can never exist is the soft-lock, loudly.
				mudlog.Error("conversation.GiveItem", "msg", "dialogue givesItem references invalid item",
					"userId", user.UserId, "itemId", itemId)
				return false
			}
			// StoreItem refuses past twice carry capacity (the "crushed"
			// encumbrance tier). Claiming delivery anyway, and worse,
			// queueing ItemOwnership{Gained:true}, which fires item_gain
			// quest triggers and auto-grants QuestTokens, advanced quest
			// state on an item the player never received.
			if !user.Character.StoreItem(newItem) {
				send(messaging.CategoryRefusal, fmt.Sprintf(
					`You can't carry the <ansi fg="itemname">%s</ansi>. `+
						`Drop something, then ask again.`, newItem.DisplayName()))
				mudlog.Warn("conversation.GiveItem", "msg", "player could not carry dialogue item",
					"userId", user.UserId, "itemId", itemId)
				return false
			}
			send(messaging.CategoryLoot, fmt.Sprintf(`You receive %s<ansi fg="itemname">%s</ansi>.`, items.ArticlePrefix(newItem.DisplayName()), items.MidSentenceName(newItem.DisplayName())))
			events.AddToQueue(events.ItemOwnership{
				UserId: user.UserId,
				Item:   newItem,
				Gained: true,
			})
			return true
		},
		GetQuestFlag: func(key string) string {
			return user.Character.GetQuestFlag(key)
		},
		SetQuestFlag: func(key, value string) {
			user.Character.SetQuestFlag(key, value)
		},
		BumpRep: func(faction string, delta int) {
			factions.BumpRep(faction, user.UserId, delta)
		},
		GiveGold: func(amount int) {
			user.Character.Gold += amount
			send(messaging.CategoryLoot, fmt.Sprintf(`You receive <ansi fg="gold">%s gold</ansi>.`, util.FormatNumber(amount)))
			events.AddToQueue(events.EquipmentChange{
				UserId:     user.UserId,
				GoldChange: amount,
			})
		},
		HasOwnMasterwork: func(skillMin int) bool {
			return user.Character.HasOwnMasterwork(skillMin)
		},
		HasGold: func(amount int) bool {
			return user.Character.Gold >= amount
		},
		// ChargeGold backs dialogue chargesGold: the whole price or nothing,
		// checked and taken in the same call, before the entry's other effects.
		ChargeGold: func(amount int) bool {
			if amount <= 0 || user.Character.Gold < amount {
				return false
			}
			user.Character.Gold -= amount
			send(messaging.CategoryLoot, fmt.Sprintf(`You pay <ansi fg="gold">%s gold</ansi>.`, util.FormatNumber(amount)))
			events.AddToQueue(events.EquipmentChange{
				UserId:     user.UserId,
				GoldChange: -amount,
			})
			return true
		},
		HasHeard:  user.Character.HasHeard,
		MarkHeard: user.Character.MarkHeard,
	}
}
