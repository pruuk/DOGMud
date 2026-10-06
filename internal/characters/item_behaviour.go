package characters

import "github.com/GoMudEngine/GoMud/internal/items"

// IndexTreedItem enters this character's mob in the item tick's holder
// index when the item names a behaviour tree (lighting 5e, Rule 5). A player
// is never indexed: the tick walks every online player anyway. Called from
// every path that hands a mob an item: StoreItem, Wear, the spills in
// RemoveFromBody. The only direct append outside this package
// (internal/actions/steal_pocket.go) is covered through StoreItem, which
// indexes before its capacity check.
func (c *Character) IndexTreedItem(i items.Item) {
	if c.MobInstanceId > 0 && i.HasBehavior() {
		items.IndexMobHolder(c.MobInstanceId)
	}
}

// IndexTreedItems enters this character's mob in the holder index when any
// worn or backpack item names a behaviour tree. Mob spawn and the companion
// gear restore call it once the mob's items are in place.
func (c *Character) IndexTreedItems() {
	if c.MobInstanceId <= 0 {
		return
	}
	for _, s := range c.Equipment.AllSlots() {
		if s.Item.ItemId > 0 && s.Item.HasBehavior() {
			items.IndexMobHolder(c.MobInstanceId)
			return
		}
	}
	for _, it := range c.Items {
		if it.HasBehavior() {
			items.IndexMobHolder(c.MobInstanceId)
			return
		}
	}
}
