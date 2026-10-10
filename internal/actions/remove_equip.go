package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// EquipItemResult is the result of an EquipItem call.
type EquipItemResult struct {
	Item           items.Item
	DisplacedItems []items.Item
	Found          bool
	Equipped       bool
	FailureReason  string
	// ArmLabel is how the player reads the arm the item went into ("weapon
	// hand", "offhand", "arm 3"), set only by EquipItemInArm.
	ArmLabel string
}

// EquipItem takes a named item from the actor's backpack and equips it.
// Displaced items (swapped-out gear) are stored back to the backpack, or
// dropped to the floor if the backpack is full: no item loss allowed.
// The reveal, Validate(), and EquipmentChange are all handled here.
// Messaging, condition onStart triggers, and quest-engine notifications
// remain in the callers.
func EquipItem(actor Actor, itemName string) EquipItemResult {
	return equipItem(actor, itemName, (*characters.Character).Wear)
}

// EquipItemInArm is `equip X armN` (spec ruling 11): EquipItem with the
// placement confined to arm N through Character.WearInArm, so the arm path
// meets every gate Wear has. It has one caller, the player's equip.
func EquipItemInArm(actor Actor, itemName string, arm int) EquipItemResult {
	res := equipItem(actor, itemName, func(c *characters.Character, i items.Item) ([]items.Item, bool, string) {
		return c.WearInArm(i, arm)
	})
	if res.Equipped {
		res.ArmLabel = actor.GetCharacter().ArmDisplayName(arm)
	}
	return res
}

func equipItem(actor Actor, itemName string, wear func(*characters.Character, items.Item) ([]items.Item, bool, string)) EquipItemResult {
	char := actor.GetCharacter()

	// Equippable-first, unfiltered fallback: mirrors usercommands/equip.go
	// so mob equips and gearup get the same preference.
	matchItem, found := char.FindInBackpackWhere(itemName, func(it items.Item) bool {
		spec := it.GetSpec()
		return spec.Type == items.Weapon || spec.Subtype == items.Wearable
	})
	if !found {
		matchItem, found = char.FindInBackpack(itemName)
	}
	if !found {
		return EquipItemResult{Found: false}
	}

	// Tentatively remove from backpack so the wear step can place it on the body.
	char.RemoveItem(matchItem)

	displaced, newItemWorn, failureReason := wear(char, matchItem)
	if !newItemWorn {
		// Put it back: equip failed.
		char.StoreItem(matchItem)
		return EquipItemResult{
			Item:          matchItem,
			Found:         true,
			Equipped:      false,
			FailureReason: failureReason,
		}
	}

	char.Awareness.TransitionToRevealing(state.TransitionReason{
		Trigger: awareness.TriggerForceVisible,
	})

	// Return displaced gear to backpack; drop to floor on overflow.
	for _, di := range displaced {
		if di.ItemId != 0 {
			if !char.StoreItem(di) {
				actor.GetRoom().AddItem(di, false)
			}
		}
	}

	char.Validate()

	events.AddToQueue(events.EquipmentChange{
		UserId:        actor.GetUserId(),
		MobInstanceId: actor.GetMobInstanceId(),
		ItemsWorn:     []items.Item{matchItem},
		ItemsRemoved:  displaced,
	})

	return EquipItemResult{
		Item:           matchItem,
		DisplacedItems: displaced,
		Found:          true,
		Equipped:       true,
	}
}

// RemoveEquipResult is the result of a RemoveEquipment call.
type RemoveEquipResult struct {
	Item    items.Item
	Found   bool
	Removed bool // true when RemoveFromBody succeeded and item was stored/dropped
	// Busy: refused because the actor is focused on work (IsActing), before
	// anything is looked up.
	Busy bool
	// Cursed: refused because the curse holds (CursedHolds).
	Cursed bool
	// CursedOverridden: the item was cursed but the actor's Spellcasting
	// lifted it; it came off.
	CursedOverridden bool
	Err              error
}

// CursedHolds is the one statement of the remove curse rule: a cursed item
// stays on a living wearer, unless their Spellcasting is 4 or more, which
// overrides it. Both remove wrappers and the AI companion ask it. (Equip has
// its own rule, characters.CursedRefusal, with no exception.)
func CursedHolds(char *characters.Character, item items.Item) (holds, overridden bool) {
	if !item.IsCursed() || char.Health <= 0 {
		return false, false
	}
	if char.GetSkillLevel(skills.Spellcasting) >= 4 {
		return false, true
	}
	return true, false
}

// RemoveEquipment takes a worn item off and stores it in the backpack
// (dropping it on the floor if the backpack is full). Gates, in order: Busy
// (before the lookup, as the player's command always refused first), not
// found, Cursed. It reveals the actor, fires EquipmentChange and calls
// Validate(). Messaging stays in the callers.
func RemoveEquipment(actor Actor, itemName string) RemoveEquipResult {
	char := actor.GetCharacter()
	if char.IsActing() {
		return RemoveEquipResult{Busy: true}
	}
	matchItem, found := char.FindOnBody(itemName)
	if !found || matchItem.ItemId < 1 {
		return RemoveEquipResult{Found: false}
	}
	return removeWorn(actor, matchItem)
}

// removeWorn takes one worn item off through the curse gate.
func removeWorn(actor Actor, matchItem items.Item) RemoveEquipResult {
	char := actor.GetCharacter()
	room := actor.GetRoom()

	holds, overridden := CursedHolds(char, matchItem)
	if holds {
		return RemoveEquipResult{Item: matchItem, Found: true, Cursed: true}
	}

	char.Awareness.TransitionToRevealing(state.TransitionReason{
		Trigger: awareness.TriggerForceVisible,
	})

	if !char.RemoveFromBody(matchItem) {
		// RemoveFromBody failed: item is still on body
		return RemoveEquipResult{Item: matchItem, Found: true, CursedOverridden: overridden}
	}

	if !char.StoreItem(matchItem) {
		// Backpack full: drop to floor as safety net
		room.AddItem(matchItem, false)
	}

	events.AddToQueue(events.EquipmentChange{
		UserId:        actor.GetUserId(),
		MobInstanceId: actor.GetMobInstanceId(),
		ItemsRemoved:  []items.Item{matchItem},
	})

	char.Validate()

	return RemoveEquipResult{Item: matchItem, Found: true, Removed: true, CursedOverridden: overridden}
}

// RemoveAllResult is the result of a RemoveAllEquipment call.
type RemoveAllResult struct {
	Busy    bool
	Removed []items.Item
	Cursed  []items.Item // left on: the curse holds
}

// RemoveAllEquipment is `remove all` for both actors: the busy gate once,
// then every worn item through removeWorn by identity (not by name, so two
// same-named pieces cannot confuse it). A cursed item is skipped and listed;
// the rest come off, one EquipmentChange each.
func RemoveAllEquipment(actor Actor) RemoveAllResult {
	char := actor.GetCharacter()
	if char.IsActing() {
		return RemoveAllResult{Busy: true}
	}
	var out RemoveAllResult
	for _, item := range char.Equipment.GetAllItems() {
		res := removeWorn(actor, item)
		switch {
		case res.Cursed:
			out.Cursed = append(out.Cursed, res.Item)
		case res.Removed:
			out.Removed = append(out.Removed, res.Item)
		}
	}
	return out
}
