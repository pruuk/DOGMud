package conversation

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
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
	ps := &dialogue.PlayerState{
		HasQuest: user.Character.HasQuest,
		// Backs requiresItem: what a topic can take, the backpack and the
		// bandolier, where a potion lands by itself (see removeBackpackItem).
		HasItem: func(itemId int) bool {
			for _, itm := range takeableItems(user) {
				if itm.ItemId == itemId {
					return true
				}
			}
			return false
		},
		// Backs itemExcluded: anywhere on the player counts, so a wielded
		// sling or a pouch in the bandolier still means "already has one".
		CarriesItem: user.Character.CarriesItem,
		RemoveItem: func(itemId int) bool {
			_, ok := removeBackpackItem(user, itemId, uuid.UUID{})
			return ok
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
			// state on an item the player never received. The acquire seam
			// raises the gain only when the item lands.
			if !acquire.AcquireItem(user, newItem, acquire.AcquireTopic) {
				send(messaging.CategoryRefusal, fmt.Sprintf(
					`You can't carry the <ansi fg="itemname">%s</ansi>. `+
						`Drop something, then ask again.`, newItem.DisplayName()))
				mudlog.Warn("conversation.GiveItem", "msg", "player could not carry dialogue item",
					"userId", user.UserId, "itemId", itemId)
				return false
			}
			send(messaging.CategoryLoot, fmt.Sprintf(`You receive %s<ansi fg="itemname">%s</ansi>.`, items.ArticlePrefix(newItem.DisplayName()), items.MidSentenceName(newItem.DisplayName())))
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
		QuestOpen: func(questId int) bool {
			return user.Character.QuestOpen(quests.GetQuestById(questId))
		},
		Shelved:     user.Character.IsShelved,
		KnowsRecipe: user.Character.HasRecipe,
		// The same test the craft command makes (actions/craft.go).
		RecipeSkillMet: func(id string) bool {
			r := crafting.GetRecipe(id)
			if r == nil {
				return false
			}
			return user.Character.GetSkillLevel(skills.SkillTag(r.Skill)) >= r.SkillMinimum
		},
		LearnRecipe: func(id string) {
			questengine.LearnRecipeWithNotice(user.Character, id, send)
		},
	}
	// Objective lights a topic a topic_heard trigger of the player's quests
	// would act on right now (a dry run; nothing fires). Its quest checks go
	// through ps.HasQuest as it stands when asked, so a menu that counts a
	// just-granted, still queued token as held (heldAlso) marks that quest's
	// next topic too.
	ps.Objective = func(mobId int, key string) bool {
		player := heldQuests{questengine.NewGameBridge(user, user.Character.RoomId), ps.HasQuest}
		return questengine.GetEngine().WouldFire("topic_heard",
			questengine.TopicHeardDetails(user.UserId, user.Character.RoomId, mobId, key), player)
	}
	return ps
}

// heldQuests is the quest engine's view of the player with HasQuest taken
// from the dialogue PlayerState, so a token heldAlso counts as held reaches
// the engine's dry run.
type heldQuests struct {
	*questengine.GameBridge
	has func(string) bool
}

func (h heldQuests) HasQuest(token string) bool { return h.has(token) }

// takeableItems is what a topic can take from the player, backpack first. A
// potion goes to a worn bandolier by itself (Character.StoreItem), so a hand-in
// reads it there and the player never has to move it. The component pouch stays
// out: a quest item is never is_component.
func takeableItems(user *users.UserRecord) []items.Item {
	return append(user.Character.GetAllBackpackItems(), user.Character.PotionItems...)
}

// removeBackpackItem takes one itemId item from the player's backpack, or from
// the bandolier when the backpack has none, and returns it: the instance prefer
// names when it is there with that id (the one a give named, `give 2.purse`),
// else the first match. It queues the same loss event give.go queues when a
// hand-in is consumed, so item-loss listeners see a talk hand-in too.
func removeBackpackItem(user *users.UserRecord, itemId int, prefer uuid.UUID) (items.Item, bool) {
	pack := takeableItems(user)
	pick := -1
	for i, itm := range pack {
		if itm.ItemId != itemId {
			continue
		}
		if !prefer.IsNil() && itm.UUID == prefer {
			pick = i
			break
		}
		if pick < 0 {
			pick = i
		}
	}
	if pick < 0 || !user.Character.RemoveItem(pack[pick]) {
		return items.Item{}, false
	}
	events.AddToQueue(events.ItemOwnership{
		UserId: user.UserId,
		Item:   pack[pick],
		Gained: false,
	})
	return pack[pick], true
}
