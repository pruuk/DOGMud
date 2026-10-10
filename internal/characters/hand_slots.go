package characters

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// HandSlot describes one arm slot with its label and a pointer to the
// equipment field.
type HandSlot struct {
	Label   string
	ItemPtr *items.Item
}

// HandPair groups two adjacent arm slots. For half-pairs (odd arm count),
// Second.ItemPtr is nil.
type HandPair struct {
	First  HandSlot
	Second HandSlot
}

// GetHandPairs returns the available hand pairs for this character.
// Pair A is always present. Pairs B and C require extra-arms mutation.
// Odd mutation levels produce a "half pair" where only the First slot
// exists (Second.ItemPtr is nil) — no 2H allowed, only 1H/shield.
func (c *Character) GetHandPairs() []HandPair {
	pairs := []HandPair{
		{
			First:  HandSlot{"wielded", &c.Equipment.Weapon},
			Second: HandSlot{"offhand", &c.Equipment.Offhand},
		},
	}
	if c.ExtraArms >= 1 {
		p := HandPair{
			First: HandSlot{"extra arm 1", &c.Equipment.ExtraArm1},
		}
		if c.ExtraArms >= 2 {
			p.Second = HandSlot{"extra arm 2", &c.Equipment.ExtraArm2}
		}
		pairs = append(pairs, p)
	}
	if c.ExtraArms >= 3 {
		p := HandPair{
			First: HandSlot{"extra arm 3", &c.Equipment.ExtraArm3},
		}
		if c.ExtraArms >= 4 {
			p.Second = HandSlot{"extra arm 4", &c.Equipment.ExtraArm4}
		}
		pairs = append(pairs, p)
	}
	return pairs
}

// ArmLabel is the label of arm N (1 to 6) as GetHandPairs names it
// ("wielded", "offhand", "extra arm 1" ...), or "" when the character has no
// such arm.
func (c *Character) ArmLabel(arm int) string {
	if arm < 1 {
		return ``
	}
	pairs := c.GetHandPairs()
	pairIdx, slotInPair := (arm-1)/2, (arm-1)%2
	if pairIdx >= len(pairs) {
		return ``
	}
	if slotInPair == 0 {
		return pairs[pairIdx].First.Label
	}
	if pairs[pairIdx].IsHalfPair() {
		return ``
	}
	return pairs[pairIdx].Second.Label
}

// ArmDisplayName is how a player reads arm N (1 to 6): "weapon hand",
// "offhand", then "arm 3" to "arm 6", as the equipment list numbers them
// (#270). "" when the character has no such arm. ArmLabel stays the slot key.
func (c *Character) ArmDisplayName(arm int) string {
	if c.ArmLabel(arm) == `` {
		return ``
	}
	switch arm {
	case 1:
		return `weapon hand`
	case 2:
		return `offhand`
	}
	return fmt.Sprintf(`arm %d`, arm)
}

// Is2H returns true if the item in this slot is a 2-handed weapon that
// occupies both slots of the pair.
func (s HandSlot) Is2H(c *Character) bool {
	if s.ItemPtr == nil || s.ItemPtr.ItemId < 1 {
		return false
	}
	return c.HandsRequired(*s.ItemPtr) >= 2
}

// IsEmpty returns true if the slot has no item or is a nil half-pair slot.
func (s HandSlot) IsEmpty() bool {
	return s.ItemPtr == nil || s.ItemPtr.ItemId < 1 || s.ItemPtr.IsDisabled()
}

// IsHalfPair returns true if the second slot of a pair doesn't exist
// (odd number of extra arms).
func (p HandPair) IsHalfPair() bool {
	return p.Second.ItemPtr == nil
}

// BestParryRating returns the highest ParryRating across all equipped
// items in weapon/arm slots.
func (c *Character) BestParryRating() int {
	best := 0
	for _, slot := range c.getWeaponAndArmItems() {
		if slot.ItemId < 1 {
			continue
		}
		if pr := slot.GetSpec().ParryRating; pr > best {
			best = pr
		}
	}
	return best
}

// ParryCapableArmCount returns how many equipped weapon/arm slots hold a
// weapon that can actually deflect a blow.
//
// Fists and claws are excluded: they are weapons by ItemType but the unarmed
// style has nothing to parry WITH, which is the rule IsUnarmedStyle has always
// encoded for the main hand.
//
// ⚠️ This counts EVERY arm, extras included, which is the whole point. The
// defence gate used to be a main-hand ladder: IsUnarmedStyle read
// Equipment.Weapon alone, so claws in hand one suppressed parry AND block for
// all six arms, and IsDualWielding read Weapon+Offhand alone and returned
// before the shield check, so two swords hid a shield in arm three. A player
// with the extra-arms mutation, a tower shield strapped to their third arm and
// claws in front was getting dodge and nothing else.
func (c *Character) ParryCapableArmCount() int {
	n := 0
	for _, slot := range c.getWeaponAndArmItems() {
		if slot.ItemId < 1 {
			continue
		}
		spec := slot.GetSpec()
		if spec.Type != items.Weapon {
			continue
		}
		if spec.Subtype == items.Fist || spec.Subtype == items.Claws {
			continue
		}
		n++
	}
	return n
}

// BestBlockRating returns the highest BlockRating across all equipped
// items in weapon/arm slots.
func (c *Character) BestBlockRating() int {
	best := 0
	for _, slot := range c.getWeaponAndArmItems() {
		if slot.ItemId < 1 {
			continue
		}
		if br := slot.GetSpec().BlockRating; br > best {
			best = br
		}
	}
	return best
}

// HasAnyShield returns true if any arm slot holds a shield-type item.
// An offhand counts as a shield when it is wearable or actually mitigates
// (an offhand holdable that gains mitigation through enchanting shields you
// — same semantics the legacy DamageReduction check carried; every authored
// offhand is subtype wearable, so classification was verified unchanged in
// the 2026-08-03 field migration).
func (c *Character) HasAnyShield() bool {
	slots := c.getWeaponAndArmItems()
	for _, slot := range slots {
		if slot.ItemId < 1 {
			continue
		}
		spec := slot.GetSpec()
		if spec.Type == items.Offhand && (spec.PhysicalMitigation > 0 || spec.Subtype == items.Wearable) {
			return true
		}
	}
	return false
}

// getWeaponAndArmItems returns item copies from all weapon/arm slots.
func (c *Character) getWeaponAndArmItems() []items.Item {
	slots := []items.Item{
		c.Equipment.Weapon,
		c.Equipment.Offhand,
	}
	if c.ExtraArms >= 1 {
		slots = append(slots, c.Equipment.ExtraArm1)
	}
	if c.ExtraArms >= 2 {
		slots = append(slots, c.Equipment.ExtraArm2)
	}
	if c.ExtraArms >= 3 {
		slots = append(slots, c.Equipment.ExtraArm3)
	}
	if c.ExtraArms >= 4 {
		slots = append(slots, c.Equipment.ExtraArm4)
	}
	return slots
}
