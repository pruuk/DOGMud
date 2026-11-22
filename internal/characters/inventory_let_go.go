package characters

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The let-go lookups (#940). A copy a quest handed out
// (items.Item.IsQuestGiven) fades when it leaves its holder, so a command
// that lets an item go (drop, give, put, stash, storage, mail, auction,
// plant, guild donate, sell, a trade-in) and is named by a word that fits
// both a quest copy and an ordinary copy of the same item takes the ordinary
// one. A handle (@uuid) or an explicit count the player typed (`N.item` or
// `item#N`, 1 included) is the player's own pick and is kept as typed. Only a copy of the same item is ever swapped in, so the
// narrower search never lands on some other item the name also fits.

// FindInBackpackToLetGo is FindInBackpack for a command that lets the item go.
func (c *Character) FindInBackpackToLetGo(itemName string) (items.Item, bool) {
	return preferLettable(itemName, c.FindInBackpack, c.FindInBackpackWhere)
}

// FindInBackpackToLetGoWhere is FindInBackpackToLetGo over only the backpack
// items keep accepts (nil keep = no filter), for a way out with a filter of
// its own (storage passes over a hot stolen bauble).
func (c *Character) FindInBackpackToLetGoWhere(itemName string, keep func(items.Item) bool) (items.Item, bool) {
	if keep == nil {
		return c.FindInBackpackToLetGo(itemName)
	}
	return preferLettable(itemName,
		func(name string) (items.Item, bool) { return c.FindInBackpackWhere(name, keep) },
		func(name string, also func(items.Item) bool) (items.Item, bool) {
			return c.FindInBackpackWhere(name, func(i items.Item) bool { return keep(i) && also(i) })
		})
}

// FindInPotionsToLetGo is FindInPotions for a command that lets the item go.
func (c *Character) FindInPotionsToLetGo(itemName string) (items.Item, bool) {
	return preferLettable(itemName, c.FindInPotions, c.FindInPotionsWhere)
}

// FindInComponentsToLetGo is FindInComponents for a command that lets the
// item go.
func (c *Character) FindInComponentsToLetGo(itemName string) (items.Item, bool) {
	return preferLettable(itemName, c.FindInComponents, c.FindInComponentsWhere)
}

// FindInComponentsWhere is FindInComponents over only the component bag
// items keep accepts (nil keep = no filter).
func (c *Character) FindInComponentsWhere(itemName string, keep func(items.Item) bool) (items.Item, bool) {
	if keep == nil {
		return c.FindInComponents(itemName)
	}
	pool := make([]items.Item, 0, len(c.ComponentItems))
	for _, itm := range c.ComponentItems {
		if keep(itm) {
			pool = append(pool, itm)
		}
	}
	if itemName == `` || len(pool) == 0 {
		return items.Item{}, false
	}
	closeMatchItem, matchItem := items.FindMatchIn(itemName, pool...)
	if matchItem.ItemId != 0 {
		return matchItem, true
	}
	if closeMatchItem.ItemId != 0 {
		return closeMatchItem, true
	}
	return items.Item{}, false
}

// typedCount reports whether the player typed an explicit count, `N.item`
// or `item#N`, any N including 1. "all.item" is not a count.
func typedCount(itemName string) bool {
	name, n := util.GetMatchNumber(itemName)
	return n >= 1 && name != strings.TrimSpace(strings.ToLower(itemName))
}

// preferLettable runs find, and when it lands on a quest copy that the
// player did not pick out by handle or count, looks for an ordinary copy of
// the same item with where.
func preferLettable(itemName string,
	find func(string) (items.Item, bool),
	where func(string, func(items.Item) bool) (items.Item, bool)) (items.Item, bool) {

	itm, found := find(itemName)
	if !found || !itm.IsQuestGiven() {
		return itm, found
	}
	if _, isHandle := isItemHandle(itemName); isHandle {
		return itm, found
	}
	if typedCount(itemName) {
		return itm, found
	}
	if other, ok := where(itemName, func(o items.Item) bool {
		return o.ItemId == itm.ItemId && !o.IsQuestGiven()
	}); ok {
		return other, true
	}
	return itm, found
}
