package dialogue

import (
	"slices"
)

// checkQuestGate returns true if the player satisfies quest/item/flag conditions.
//
// When ps is nil every check passes (backward compat for mob-to-mob and other
// non-user contexts). Production callers always pass a fully-populated state
// built by conversation.PlayerState.
//
// Every callback field is optional and must be nil-checked before use. Four of
// them (HasQuest, HasItem, RemoveItem, GiveQuest) were previously invoked
// unguarded, so a partially-populated state panicked on those and only those.
//
// A missing *interrogative* callback fails the gate closed: with no way to ask
// whether the player holds a token or an item, we must not conclude that they
// do. The exception is questExcluded, where the same reasoning runs the other
// way: we cannot confirm the player holds the excluding token, so the topic
// stays available rather than silently vanishing.
//
// goldRequired is interrogative too and fails closed on a nil HasGold: a
// priced topic must never open for a player whose gold cannot be checked.
// The gate only hides the topic; it takes nothing. The charge is the topic's
// chargesGold, which Apply takes before any other effect.
func checkQuestGate(questRequired, questExcluded []string, requiresItem int, flagRequired, flagExcluded map[string]string, masterworkRequired, goldRequired int, ps *PlayerState) bool {
	if ps == nil {
		return true
	}

	if len(questRequired) > 0 && ps.HasQuest == nil {
		return false
	}
	for _, token := range questRequired {
		if !ps.HasQuest(token) {
			return false
		}
	}

	if ps.HasQuest != nil && slices.ContainsFunc(questExcluded, ps.HasQuest) {
		return false
	}

	if requiresItem > 0 {
		if ps.HasItem == nil || !ps.HasItem(requiresItem) {
			return false
		}
	}

	if masterworkRequired > 0 && ps.HasOwnMasterwork != nil && !ps.HasOwnMasterwork(masterworkRequired) {
		return false
	}

	if goldRequired > 0 {
		if ps.HasGold == nil || !ps.HasGold(goldRequired) {
			return false
		}
	}

	// Quest flag checks
	if ps.GetQuestFlag != nil {
		for key, val := range flagRequired {
			if ps.GetQuestFlag(key) != val {
				return false
			}
		}
		for key, val := range flagExcluded {
			if ps.GetQuestFlag(key) == val {
				return false
			}
		}
	}

	return true
}

// itemExcludedBlocks reports whether an itemExcluded gate hides an entry: the
// player already carries the item, so a handout topic does not give a second
// one while the quest is open. CarriesItem (anywhere on the player) is
// preferred; a PlayerState without it falls back to HasItem (the backpack).
// A nil PlayerState skips the check, as every other gate does.
func itemExcludedBlocks(itemId int, ps *PlayerState) bool {
	if itemId <= 0 || ps == nil {
		return false
	}
	if ps.CarriesItem != nil {
		return ps.CarriesItem(itemId)
	}
	if ps.HasItem != nil {
		return ps.HasItem(itemId)
	}
	return false
}

// tryCharge takes a topic's chargesGold before its other effects. A failed
// charge makes Apply report false with nothing applied. Nil ps keeps the
// package's skip-checks contract; a nil ChargeGold fails closed.
func tryCharge(amount int, ps *PlayerState) bool {
	if amount <= 0 || ps == nil {
		return true
	}
	if ps.ChargeGold == nil {
		return false
	}
	return ps.ChargeGold(amount)
}

// applyPaidEffects applies a chosen topic's effects after tryCharge took
// charged gold. If the effects abort (an item that could not be delivered),
// the charge is given back so the player never pays for nothing.
func applyPaidEffects(charged int, grantsQuest string, requiresItem int, givesItem int, flagSet *QuestFlagSet, bumpsRep []RepBump, givesGold int, ps *PlayerState) bool {
	if applyQuestEffects(grantsQuest, requiresItem, givesItem, flagSet, bumpsRep, givesGold, ps) {
		return true
	}
	if charged > 0 && ps != nil && ps.GiveGold != nil {
		ps.GiveGold(charged)
	}
	return false
}

// applyQuestEffects fires a chosen topic's side effects. It returns false,
// applying NOTHING else, when the topic gives an item that could not be
// delivered: granting the quest anyway would soft-lock the player (token
// held, item missing, topic then hidden by questExcluded), and removing
// requiresItem first would eat their hand-in. Delivery is therefore
// attempted before any other effect. A nil GiveItem callback keeps the
// package's skip-checks contract and counts as delivered.
func applyQuestEffects(grantsQuest string, requiresItem int, givesItem int, flagSet *QuestFlagSet, bumpsRep []RepBump, givesGold int, ps *PlayerState) bool {
	if ps == nil {
		return true
	}
	if givesItem > 0 && ps.GiveItem != nil {
		if !ps.GiveItem(givesItem) {
			return false
		}
	}
	if requiresItem > 0 && ps.RemoveItem != nil {
		ps.RemoveItem(requiresItem)
	}
	if grantsQuest != "" && ps.GiveQuest != nil {
		ps.GiveQuest(grantsQuest)
	}
	if flagSet != nil && ps.SetQuestFlag != nil {
		ps.SetQuestFlag(flagSet.Key, flagSet.Value)
	}
	if ps.BumpRep != nil {
		for _, rb := range bumpsRep {
			if rb.Faction != "" && rb.Delta != 0 {
				ps.BumpRep(rb.Faction, rb.Delta)
			}
		}
	}
	if givesGold > 0 && ps.GiveGold != nil {
		ps.GiveGold(givesGold)
	}
	return true
}
