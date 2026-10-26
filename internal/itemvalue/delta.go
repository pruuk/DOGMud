package itemvalue

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
)

// canonicalRank returns the sort order for a SlotName, used as
// a tiebreaker when two slots produce equal Score. Lower rank
// = preferred. Weapon=0 wins over Offhand=1 when scores tie
// (so a wand placed on an empty-handed mob lands in main hand).
func canonicalRank(s SlotName) int {
	for i, n := range canonicalSlotOrder {
		if n == s {
			return i
		}
	}
	return len(canonicalSlotOrder)
}

var canonicalSlotOrder = []SlotName{
	SlotWeapon, SlotOffhand,
	SlotExtraArm1, SlotExtraArm2, SlotExtraArm3, SlotExtraArm4,
	SlotHead, SlotNeck, SlotShoulders, SlotBody, SlotBack, SlotBelt,
	SlotWrist1, SlotWrist2,
	SlotExtraWrist1, SlotExtraWrist2, SlotExtraWrist3, SlotExtraWrist4,
	SlotGloves,
	SlotRing, SlotRing2,
	SlotLegs, SlotFeet,
	SlotTail, SlotComponentBag,
	SlotLight,
}

// hasTailMutation reports whether the character has the Tail
// mutation active.
func hasTailMutation(char *characters.Character) bool {
	return mutations.HasMutation(char.Mutations, "tail")
}

// chooserSlotName maps an AllSlots key to its SlotName, for the four item
// types whose slot Character.ChooseWornSlot picks.
var chooserSlotName = map[string]SlotName{
	`weapon`: SlotWeapon, `offhand`: SlotOffhand,
	`extraarm1`: SlotExtraArm1, `extraarm2`: SlotExtraArm2, `extraarm3`: SlotExtraArm3, `extraarm4`: SlotExtraArm4,
	`wrist1`: SlotWrist1, `wrist2`: SlotWrist2,
	`extrawrist1`: SlotExtraWrist1, `extrawrist2`: SlotExtraWrist2, `extrawrist3`: SlotExtraWrist3, `extrawrist4`: SlotExtraWrist4,
	`ring`: SlotRing, `ring2`: SlotRing2,
}

// usesChooser is true for the item types whose slot Wear picks through
// Character.ChooseWornSlot (slice 5a): the scorer asks the same helper, so a
// scored upgrade is exactly the swap Wear makes, for any arm count.
func usesChooser(t items.ItemType) bool {
	return t == items.Weapon || t == items.Offhand || t == items.Ring || t == items.Wrist
}

// compatibleSlotsFor returns the list of SlotNames where the candidate could
// be placed. For weapons, shields, rings and wrists that is the one slot
// Character.ChooseWornSlot picks (nothing when it refuses); for every other
// type, the type's own slot. Returns nil when the item is not equippable.
func compatibleSlotsFor(candidate items.Item, char *characters.Character) []SlotName {
	spec := candidate.GetSpec()
	if usesChooser(spec.Type) {
		choice, refusal := char.ChooseWornSlot(candidate, 0)
		if refusal != `` || len(choice.Slots) == 0 {
			return nil
		}
		return []SlotName{chooserSlotName[choice.Slots[0].Key]}
	}
	switch spec.Type {
	case items.Head:
		return []SlotName{SlotHead}
	case items.Neck:
		return []SlotName{SlotNeck}
	case items.Shoulders:
		return []SlotName{SlotShoulders}
	case items.Body:
		return []SlotName{SlotBody}
	case items.Back:
		return []SlotName{SlotBack}
	case items.Belt:
		return []SlotName{SlotBelt}
	case items.Gloves:
		return []SlotName{SlotGloves}
	case items.Legs:
		return []SlotName{SlotLegs}
	case items.Feet:
		return []SlotName{SlotFeet}
	case items.Tail:
		if hasTailMutation(char) {
			return []SlotName{SlotTail}
		}
		return nil
	case items.ComponentBag:
		return []SlotName{SlotComponentBag}
	case items.Light:
		return []SlotName{SlotLight}
	}
	return nil
}

// itemInSlot returns the currently-equipped item in a given
// slot (or the zero-value items.Item if the slot is empty).
func itemInSlot(slot SlotName, char *characters.Character) items.Item {
	e := &char.Equipment
	switch slot {
	case SlotWeapon:
		return e.Weapon
	case SlotOffhand:
		return e.Offhand
	case SlotExtraArm1:
		return e.ExtraArm1
	case SlotExtraArm2:
		return e.ExtraArm2
	case SlotExtraArm3:
		return e.ExtraArm3
	case SlotExtraArm4:
		return e.ExtraArm4
	case SlotHead:
		return e.Head
	case SlotNeck:
		return e.Neck
	case SlotShoulders:
		return e.Shoulders
	case SlotBody:
		return e.Body
	case SlotBack:
		return e.Back
	case SlotBelt:
		return e.Belt
	case SlotWrist1:
		return e.Wrist1
	case SlotWrist2:
		return e.Wrist2
	case SlotExtraWrist1:
		return e.ExtraWrist1
	case SlotExtraWrist2:
		return e.ExtraWrist2
	case SlotExtraWrist3:
		return e.ExtraWrist3
	case SlotExtraWrist4:
		return e.ExtraWrist4
	case SlotGloves:
		return e.Gloves
	case SlotRing:
		return e.Ring
	case SlotRing2:
		return e.Ring2
	case SlotLegs:
		return e.Legs
	case SlotFeet:
		return e.Feet
	case SlotTail:
		return e.Tail
	case SlotComponentBag:
		return e.ComponentBag
	case SlotLight:
		return e.Light
	}
	return items.Item{}
}

// slotOf returns the slot in which the given equipped item
// currently resides, or "" if the item is not currently
// equipped on char.
func slotOf(item items.Item, char *characters.Character) SlotName {
	if item.ItemId == 0 {
		return ""
	}
	e := &char.Equipment
	pairs := []struct {
		slot SlotName
		got  items.Item
	}{
		{SlotWeapon, e.Weapon},
		{SlotOffhand, e.Offhand},
		{SlotExtraArm1, e.ExtraArm1},
		{SlotExtraArm2, e.ExtraArm2},
		{SlotExtraArm3, e.ExtraArm3},
		{SlotExtraArm4, e.ExtraArm4},
		{SlotHead, e.Head},
		{SlotNeck, e.Neck},
		{SlotShoulders, e.Shoulders},
		{SlotBody, e.Body},
		{SlotBack, e.Back},
		{SlotBelt, e.Belt},
		{SlotWrist1, e.Wrist1},
		{SlotWrist2, e.Wrist2},
		{SlotExtraWrist1, e.ExtraWrist1},
		{SlotExtraWrist2, e.ExtraWrist2},
		{SlotExtraWrist3, e.ExtraWrist3},
		{SlotExtraWrist4, e.ExtraWrist4},
		{SlotGloves, e.Gloves},
		{SlotRing, e.Ring},
		{SlotRing2, e.Ring2},
		{SlotLegs, e.Legs},
		{SlotFeet, e.Feet},
		{SlotTail, e.Tail},
		{SlotComponentBag, e.ComponentBag},
		{SlotLight, e.Light},
	}
	for _, p := range pairs {
		if p.got.ItemId == item.ItemId &&
			p.got.Uses == item.Uses &&
			p.got.EnchantType == item.EnchantType &&
			p.got.EnchantTier == item.EnchantTier {
			return p.slot
		}
	}
	return ""
}

// displacedItemsForSlot returns the items that come off when candidate is
// placed at targetSlot: the helper's Displaced for the chooser types (so a
// two-hander, a stray behind one, or an extra arm is modelled exactly as
// Wear does it), else the slot's own occupant.
func displacedItemsForSlot(char *characters.Character, targetSlot SlotName, candidate items.Item) []items.Item {
	if usesChooser(candidate.GetSpec().Type) {
		choice, _ := char.ChooseWornSlot(candidate, 0)
		return choice.Displaced
	}
	current := itemInSlot(targetSlot, char)
	if current.ItemId > 0 {
		return []items.Item{current}
	}
	return nil
}

// placementBonus returns the additional score a spec earns
// when placed at slot, evaluated against pre-swap char state.
// Used SYMMETRICALLY for candidate and displaced. DualWieldBonus
// is conditional on the pre-swap main hand holding a weapon that
// leaves the offhand free for THIS wearer: the slot-aware hand rule
// (IsBlockedBy2H), so a Large species or a character grown Large
// dual wields beside a two-hander (#680).
func placementBonus(profile WeightProfile, spec items.ItemSpec, slot SlotName, char *characters.Character) float64 {
	bonus := 0.0

	if spec.Hands == items.TwoHanded {
		bonus += profile.TwoHandedBonus
	}

	if slot == SlotOffhand {
		if spec.Type == items.Weapon {
			if char.Equipment.Weapon.ItemId > 0 &&
				!char.IsBlockedBy2H(`offhand`) {
				bonus += profile.DualWieldBonus
			}
		} else if spec.Type == items.Offhand {
			bonus += profile.ShieldBonus
		}
	}

	return bonus
}

func cursedAmong(char *characters.Character, displaced []items.Item) bool {
	for _, d := range displaced {
		if char.CursedRefusal(d) != `` {
			return true
		}
	}
	return false
}

// ItemValueDelta returns the net effect of equipping candidate
// over char's current loadout under the given profile. The slot
// picked is the slot Wear itself would pick for weapons,
// shields, rings and wrists. Returns
// SwapDelta{Score: 0, Slot: "", Displaced: nil} when candidate
// is not equippable on this character.
func ItemValueDelta(char *characters.Character, profile WeightProfile, candidate items.Item) SwapDelta {
	candidateSpec := candidate.GetSpec()
	candidateRaw := ItemValue(candidateSpec, profile)

	// Apply gear-effectiveness multiplier from char's mutations.
	// Incorporeal characters see all gear values scaled toward
	// zero, so equip-if-better naturally returns false at high
	// incorporeal ranks.
	gearMul := mutations.GearEffectivenessMultiplier(char.Mutations)
	candidateRaw *= gearMul

	slots := compatibleSlotsFor(candidate, char)
	if len(slots) == 0 {
		return SwapDelta{}
	}

	best := SwapDelta{Slot: "", Displaced: nil}
	bestSet := false
	bestRank := -1

	for _, slot := range slots {
		displaced := displacedItemsForSlot(char, slot, candidate)
		// A swap a curse would refuse is no upgrade (slice 5a, E8): Wear
		// will not make it. The chooser types never get here with one,
		// because the helper already skipped or refused the slot.
		if cursedAmong(char, displaced) {
			continue
		}

		candidateAt := candidateRaw + placementBonus(profile, candidateSpec, slot, char)

		displacedTotal := 0.0
		for _, d := range displaced {
			dSpec := d.GetSpec()
			currentSlot := slotOf(d, char)
			displacedTotal += (ItemValue(dSpec, profile) +
				placementBonus(profile, dSpec, currentSlot, char)) * gearMul
		}

		netScore := candidateAt - displacedTotal

		netScore -= encumbranceTierPenalty(char, displaced, candidate, profile)

		rank := canonicalRank(slot)

		if !bestSet ||
			netScore > best.Score ||
			(netScore == best.Score && rank < bestRank) {
			best = SwapDelta{
				Score:     netScore,
				Slot:      slot,
				Displaced: displaced,
			}
			bestRank = rank
			bestSet = true
		}
	}

	return best
}

// encumbranceTier returns a tier index 0..4 from a
// carryWeight/capacity ratio. Higher index = worse.
// Thresholds match userrecord.prompt.go:518-527.
//
//	0 = light (ratio ≤ 0.25)
//	1 = moderate (ratio ≤ 0.50)
//	2 = heavy (ratio ≤ 0.75)
//	3 = overburdened (ratio ≤ 1.00)
//	4 = crushed (ratio > 1.00)
func encumbranceTier(ratio float64) int {
	switch {
	case ratio <= 0.25:
		return 0
	case ratio <= 0.50:
		return 1
	case ratio <= 0.75:
		return 2
	case ratio <= 1.00:
		return 3
	default:
		return 4
	}
}

// encumbranceTierPenalty returns the score penalty (positive =
// worse) for crossing tiers in the swap. If the swap reduces
// the tier (net less weight), the return is negative (a score
// bonus). Magnitude = profile.EncumbranceTierPenalty × number
// of tiers crossed.
func encumbranceTierPenalty(char *characters.Character, displaced []items.Item, candidate items.Item, profile WeightProfile) float64 {
	capacity := char.CarryCapacity()
	if capacity <= 0 {
		return 0
	}

	currentWeight := char.GetCarriedWeight()
	weightDelta := candidate.GetSpec().GetWeight()
	for _, d := range displaced {
		weightDelta -= d.GetSpec().GetWeight()
	}
	newWeight := currentWeight + weightDelta

	preTier := encumbranceTier(currentWeight / capacity)
	postTier := encumbranceTier(newWeight / capacity)

	tiersCrossed := postTier - preTier
	if tiersCrossed == 0 {
		return 0
	}
	return float64(tiersCrossed) * profile.EncumbranceTierPenalty
}
