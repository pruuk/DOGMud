package characters

import (
	"github.com/GoMudEngine/GoMud/internal/items"
)

// The Vitalis Bandolier's rules (#794, owner ruling 2026-11-06). A belt with
// preserves_contents holds every potion it carries at peak strength, and a
// potion taken out of it rots at once. A belt with ambient_potions takes only
// potions whose held effect helps the wearer.
//
// Rot needs no hook on any removal path. A preserving bandolier marks each
// potion it holds Preserved; its wearer reads that potion at peak through
// PotionAging, and items.Item.AgingAt reads a Preserved potion anywhere else
// as spoiled. Whatever takes the potion out (get, give, sell, salvage, a
// quest hand-in, taking the belt off) carries the mark with it.

// holdsInBandolier reports whether i is one of the potions in the worn
// bandolier.
func (c *Character) holdsInBandolier(i items.Item) bool {
	for idx := range c.PotionItems {
		if c.PotionItems[idx].Equals(i) {
			return true
		}
	}
	return false
}

// PotionAging is the age of a potion this character carries, at round now:
// peak while a preserving bandolier holds it, else the potion's own reading
// (items.Item.AgingAt, where a potion that has left a preserving bandolier is
// spoiled).
func (c *Character) PotionAging(i items.Item, now uint64) items.Aging {
	if c.Equipment.Belt.GetSpec().PreservesContents && c.holdsInBandolier(i) {
		return i.PeakAging()
	}
	return i.AgingAt(now)
}

// AmbientBandolierRefuses reports whether the worn belt is an ambient
// bandolier that will not take i: a potion whose held effect does not help
// (items.ItemSpec.AmbientBeneficial), or one that has spoiled. A spoiled
// potion is refused so that taking a potion out (which rots it) and putting
// it back cannot bring it back to peak. Any other belt refuses nothing here.
func (c *Character) AmbientBandolierRefuses(i items.Item, now uint64) bool {
	belt := c.Equipment.Belt.GetSpec()
	if !belt.AmbientPotions {
		return false
	}
	spec := i.GetSpec()
	return !spec.AmbientBeneficial() || i.AgingAt(now).Spoiled()
}

// slotPotion puts i in the bandolier, marking it Preserved when the belt
// preserves its contents.
func (c *Character) slotPotion(i items.Item) {
	if c.Equipment.Belt.GetSpec().PreservesContents {
		i.Preserved = true
	}
	c.PotionItems = append(c.PotionItems, i)
}

// PreserveBandolier keeps the worn bandolier's rules on what it already
// holds: an ambient bandolier lets go of any potion it would not take, which
// goes to the backpack unmarked (a save from before #794 can hold a throwable
// or a detox in one, and a belt swap can bring in a spoiled potion), and a
// preserving bandolier marks every potion it keeps Preserved. It returns the
// potions let go. The pinnacle tick runs it every round and the login path
// once.
func (c *Character) PreserveBandolier(now uint64) []items.Item {
	belt := c.Equipment.Belt.GetSpec()
	var released []items.Item
	if belt.AmbientPotions {
		kept := c.PotionItems[:0:0]
		for _, p := range c.PotionItems {
			// A Preserved potion here was marked by this belt (a belt swap
			// spills every other one, spillPreservedOnBeltSwap). Any other
			// potion is held to what StoreItem would take: a helpful kind,
			// not spoiled, so a spoiled potion carried in from an ordinary
			// bandolier is not brought back to peak.
			if !p.Preserved && c.AmbientBandolierRefuses(p, now) {
				released = append(released, p)
				continue
			}
			kept = append(kept, p)
		}
		if len(released) > 0 {
			c.PotionItems = kept
			c.Items = append(c.Items, released...)
		}
	}
	if belt.PreservesContents {
		for idx := range c.PotionItems {
			c.PotionItems[idx].Preserved = true
		}
	}
	return released
}

// spillPreservedOnBeltSwap runs once a belt has replaced another (wear, after
// its reverts, since it writes outside c.Equipment). A
// potion leaving a preserving belt has left its preservation and rots, so
// when the old belt preserves, every potion it holds goes to the backpack
// (as taking the belt off spills them, RemoveFromBody). When the new belt
// preserves, any Preserved potion still slotted (one that rotted earlier and
// was slotted into an ordinary bandolier) goes to the backpack too, so the
// new belt never reads a rotted potion as its own. BeltChangeRots names, in
// advance, the potions the swap rots.
func (c *Character) spillPreservedOnBeltSwap(oldBelt, newBelt items.Item) {
	oldPreserves := oldBelt.GetSpec().PreservesContents
	newPreserves := newBelt.GetSpec().PreservesContents
	if !oldPreserves && !newPreserves {
		return
	}
	kept := c.PotionItems[:0:0]
	for _, p := range c.PotionItems {
		if oldPreserves || p.Preserved {
			c.Items = append(c.Items, p)
			continue
		}
		kept = append(kept, p)
	}
	c.PotionItems = kept
}

// BeltChangeRots lists the potions that rot if the worn belt is taken off or
// replaced: every potion a preserving belt holds. Taking the Vitalis
// Bandolier off counts as taking each potion out (they leave its
// preservation for the backpack), so it rots them all. The remove and equip
// paths read it first, to tell the player.
func (c *Character) BeltChangeRots() []items.Item {
	if !c.Equipment.Belt.GetSpec().PreservesContents {
		return nil
	}
	return append([]items.Item(nil), c.PotionItems...)
}

// RotsOnRemoval reports whether taking i out of the bandolier now rots it:
// a preserving bandolier holds it. A caller that moves a potion out reads
// this first, to tell the player.
func (c *Character) RotsOnRemoval(i items.Item) bool {
	return c.Equipment.Belt.GetSpec().PreservesContents && c.holdsInBandolier(i)
}
