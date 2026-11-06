package characters

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// The Vitalis Bandolier's rules (#794, owner ruling 2026-11-06). A belt with
// preserves_contents holds every potion it carries at peak strength, and a
// potion taken out of it rots at once. A belt with ambient_potions takes only
// potions whose held effect helps the wearer, one per held effect, up to its
// capacity.
//
// Rot needs no hook on any removal path. A preserving bandolier marks each
// potion it holds Preserved; its wearer reads that potion at peak through
// PotionAging, and items.Item.AgingAt reads a Preserved potion anywhere else
// as spoiled. Whatever takes the potion out (get, give, sell, salvage, a
// quest hand-in, taking the belt off) carries the mark with it. A potion is
// only ever at peak, or rotted, once it carries the mark: one carried in by
// a belt swap reads its own age until the swap's PreserveBandolier marks it.

// BandolierRefusal is why a bandolier will not take, or lets go of, a potion.
type BandolierRefusal int

const (
	BandolierTakes     BandolierRefusal = iota // no refusal
	BandolierWrongKind                         // its held effect does not help (items.ItemSpec.AmbientBeneficial)
	BandolierSpoiled                           // it has spoiled
	BandolierDuplicate                         // the bandolier already holds a potion with that effect
	BandolierFull                              // no room left
)

// Why is the reason a player reads, after a colon: plain words, no numbers.
func (r BandolierRefusal) Why() string {
	switch r {
	case BandolierSpoiled:
		return `it will not take a spoiled potion`
	case BandolierDuplicate:
		return `it already holds a potion with that effect`
	case BandolierFull:
		return `it has no room left`
	}
	return `it holds only potions that strengthen the one who wears it`
}

// BandolierRelease is one potion a bandolier let go of, and why.
type BandolierRelease struct {
	Item items.Item
	Why  BandolierRefusal
}

// BandolierRefusalLine tells the player why a bandolier will not take a
// potion they put to it (sort).
func BandolierRefusalLine(beltName string, itm items.Item, why BandolierRefusal) string {
	return fmt.Sprintf(`Your <ansi fg="itemname">%s</ansi> will not take the <ansi fg="itemname">%s</ansi>: %s.`,
		beltName, itm.DisplayName(), why.Why())
}

// BandolierReleaseLine tells the player a bandolier let go of a potion it
// already held (a belt swap, a save from before #794). The potion was never
// in its hold, so it has not soured, and the line says so.
func BandolierReleaseLine(beltName, potionName string, why BandolierRefusal) string {
	return fmt.Sprintf(`Your <ansi fg="itemname">%s</ansi> lets go of the <ansi fg="itemname">%s</ansi>: %s. You stow it in your backpack, still sound.`,
		beltName, potionName, why.Why())
}

// BandolierReleaseNote is a release kept for a later line: the potion's name
// and why, not the item, so no item walker has a second copy to visit.
type BandolierReleaseNote struct {
	Name string
	Why  BandolierRefusal
}

// BandolierRotLine tells the player a potion soured as it left a preserving
// bandolier. Every path that takes one out prints it.
func BandolierRotLine(beltName, potionName string) string {
	return fmt.Sprintf(`The <ansi fg="itemname">%s</ansi> sours the moment it leaves your <ansi fg="itemname">%s</ansi>.`,
		potionName, beltName)
}

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

// heldPreserved reports whether a preserving belt holds i under its mark.
func (c *Character) heldPreserved(i items.Item) bool {
	if !c.Equipment.Belt.GetSpec().PreservesContents {
		return false
	}
	for idx := range c.PotionItems {
		if c.PotionItems[idx].Equals(i) {
			return c.PotionItems[idx].Preserved
		}
	}
	return false
}

// PotionAging is the age of a potion this character carries, at round now:
// peak while a preserving bandolier holds it under its mark, else the
// potion's own reading (items.Item.AgingAt, where a potion that has left a
// preserving bandolier is spoiled).
func (c *Character) PotionAging(i items.Item, now uint64) items.Aging {
	if c.heldPreserved(i) {
		return i.PeakAging()
	}
	return i.AgingAt(now)
}

// HeldInPreservingBandolier reports whether a preserving bandolier holds i
// under its mark: taking it out rots it. Sell refuses such a potion, and
// every path that takes one out tells the player it soured.
func (c *Character) HeldInPreservingBandolier(i items.Item) bool {
	return c.heldPreserved(i)
}

// BandolierSustainsAny reports whether an ambient bandolier holds any of
// conditionIds as its own permanent record (the attuned set,
// AmbientBandolierConditions). The drink path refuses such a potion: one
// record per condition means the hold would absorb the drink.
func (c *Character) BandolierSustainsAny(conditionIds []int) bool {
	held := map[int]bool{}
	for _, id := range c.AmbientBandolierConditions() {
		held[id] = true
	}
	for _, id := range conditionIds {
		if !held[id] {
			continue
		}
		for _, rec := range c.Conditions.GetConditions(id) {
			if rec.Permanent {
				return true
			}
		}
	}
	return false
}

// sharesHeldEffect reports whether a potion in the bandolier (other than i)
// already carries one of i's conditions. An ambient bandolier holds one record
// per condition, so a second potion with the same effect is a wasted slot.
func (c *Character) sharesHeldEffect(i items.Item) bool {
	ids := map[int]bool{}
	for _, id := range i.GetSpec().ConditionIds {
		ids[id] = true
	}
	for idx := range c.PotionItems {
		if c.PotionItems[idx].Equals(i) {
			continue
		}
		for _, id := range c.PotionItems[idx].GetSpec().ConditionIds {
			if ids[id] {
				return true
			}
		}
	}
	return false
}

// AmbientBandolierRefusal is why the worn belt will not take i, or
// BandolierTakes. Only an ambient bandolier refuses here: a kind
// items.ItemSpec.AmbientBeneficial rejects, a spoiled potion (so a rotted one
// cannot be slotted back to peak), or one whose effect a slotted potion
// already gives. Capacity is the caller's own check.
func (c *Character) AmbientBandolierRefusal(i items.Item, now uint64) BandolierRefusal {
	if !c.Equipment.Belt.GetSpec().AmbientPotions {
		return BandolierTakes
	}
	spec := i.GetSpec()
	switch {
	case !spec.AmbientBeneficial():
		return BandolierWrongKind
	case i.AgingAt(now).Spoiled():
		return BandolierSpoiled
	case c.sharesHeldEffect(i):
		return BandolierDuplicate
	}
	return BandolierTakes
}

// AmbientBandolierRefuses reports AmbientBandolierRefusal != BandolierTakes.
func (c *Character) AmbientBandolierRefuses(i items.Item, now uint64) bool {
	return c.AmbientBandolierRefusal(i, now) != BandolierTakes
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
// holds. An ambient or preserving bandolier keeps what it marked, then takes
// the rest in order as StoreItem would (a helpful kind, not spoiled, one per
// held effect, up to its capacity) and lets go of every other potion to the
// backpack, unmarked: it never sat in the hold, so it has not soured. A belt
// swap (wear) and a save from before #794 are how such potions arrive. A
// preserving bandolier then marks every potion it keeps. It returns what it
// let go. The wear path runs it right after a belt swap, the pinnacle tick
// every round, and the login path once.
func (c *Character) PreserveBandolier(now uint64) []BandolierRelease {
	belt := c.Equipment.Belt.GetSpec()
	if !belt.AmbientPotions && !belt.PreservesContents {
		return nil
	}
	var released []BandolierRelease
	held := c.PotionItems
	c.PotionItems = make([]items.Item, 0, len(held))
	// What the belt already marked stays: it was taken by these rules.
	var pending []items.Item
	for _, p := range held {
		if p.Preserved && belt.PreservesContents {
			c.PotionItems = append(c.PotionItems, p)
			continue
		}
		pending = append(pending, p)
	}
	for _, p := range pending {
		why := c.AmbientBandolierRefusal(p, now)
		if why == BandolierTakes && belt.BandolierCapacity > 0 && len(c.PotionItems) >= belt.BandolierCapacity {
			why = BandolierFull
		}
		if why != BandolierTakes {
			released = append(released, BandolierRelease{Item: p, Why: why})
			c.Items = append(c.Items, p)
			continue
		}
		c.PotionItems = append(c.PotionItems, p)
	}
	if belt.PreservesContents {
		for idx := range c.PotionItems {
			c.PotionItems[idx].Preserved = true
		}
	}
	return released
}

// TakeBandolierReleases returns, and clears, what the last wear's
// PreserveBandolier let go of, for the equip command to tell the player.
func (c *Character) TakeBandolierReleases() []BandolierReleaseNote {
	out := c.bandolierReleases
	c.bandolierReleases = nil
	return out
}

// noteBandolierReleases keeps the wear path's releases as names for
// TakeBandolierReleases.
func (c *Character) noteBandolierReleases(rels []BandolierRelease) {
	c.bandolierReleases = nil
	for _, rel := range rels {
		c.bandolierReleases = append(c.bandolierReleases, BandolierReleaseNote{Name: rel.Item.DisplayName(), Why: rel.Why})
	}
}

// spillPreservedOnBeltSwap runs once a belt has replaced another (wear, after
// its reverts, since it writes outside c.Equipment). A potion leaving a
// preserving belt has left its preservation and rots, so when the old belt
// preserves, every potion it holds goes to the backpack (as taking the belt
// off spills them, RemoveFromBody). When the new belt preserves, any
// Preserved potion still slotted (one that rotted earlier and was slotted
// into an ordinary bandolier) goes to the backpack too, so the new belt
// never reads a rotted potion as its own. BeltChangeRots names, in advance,
// the potions the swap rots. The wear path then runs PreserveBandolier.
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

// spillBandolier empties the bandolier into the backpack, as taking the belt
// off does (RemoveFromBody, a species that loses its belt slot).
func (c *Character) spillBandolier() {
	for _, pi := range c.PotionItems {
		c.Items = append(c.Items, pi)
		c.IndexTreedItem(pi)
	}
	c.PotionItems = nil
}

// BeltChangeRots lists the potions that rot if the worn belt is taken off or
// replaced: every potion a preserving belt holds under its mark. Taking the
// Vitalis Bandolier off counts as taking each potion out (they leave its
// preservation for the backpack), so it rots them all. The remove and equip
// paths read it first, to tell the player.
func (c *Character) BeltChangeRots() []items.Item {
	if !c.Equipment.Belt.GetSpec().PreservesContents {
		return nil
	}
	var out []items.Item
	for _, p := range c.PotionItems {
		if p.Preserved {
			out = append(out, p)
		}
	}
	return out
}

// RotsOnRemoval reports whether taking i out of the bandolier now rots it:
// a preserving bandolier holds it under its mark. A caller that moves a
// potion out reads this first, to tell the player.
func (c *Character) RotsOnRemoval(i items.Item) bool {
	return c.heldPreserved(i)
}
