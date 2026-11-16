package characters

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// The golden oracle for slot choice (sight and gates slice 5a, spec
// "Slot choice"). The legacy* functions below are VERBATIM copies of the
// placement code as it stood at 3152749b0: wearWeaponOrShield, the three
// Find* helpers, and wearArmorSlot's Ring and Wrist cases. The tests diff
// Character.Wear against them over every non-cursed loadout at 2, 3, 4, 5 and
// 6 arms, plus a disabled-slot variant of every hand layout (every empty
// slot disabled instead, ItemId < 0, the way a species or a missing extra
// arm actually disables one), so a refactor of the slot choice provably
// moves nothing when nothing is cursed. Do not "fix" the legacy copies: they
// are the record.

func legacyFindFirstEmptySlot(c *Character, pairs []HandPair, isShield bool) *HandSlot {
	for pi := range pairs {
		p := &pairs[pi]
		if pi == 0 && isShield {
			if !p.IsHalfPair() && p.Second.IsEmpty() && !p.First.Is2H(c) {
				return &p.Second
			}
			continue
		}
		if p.First.Is2H(c) {
			continue
		}
		if p.First.IsEmpty() {
			return &p.First
		}
		if !p.IsHalfPair() && p.Second.IsEmpty() {
			return &p.Second
		}
	}
	return nil
}

func legacyFindFirstFreePair(pairs []HandPair) *HandPair {
	for i := range pairs {
		p := pairs[i]
		if !p.IsHalfPair() && p.First.IsEmpty() && p.Second.IsEmpty() {
			return &pairs[i]
		}
	}
	return nil
}

func legacyPairOccupantCount(p HandPair) int {
	count := 0
	if !p.First.IsEmpty() {
		count++
	}
	if !p.IsHalfPair() && !p.Second.IsEmpty() {
		count++
	}
	return count
}

func legacyFindCheapestPairToDisplace(pairs []HandPair) *HandPair {
	var best *HandPair
	bestCount := 3
	for i := range pairs {
		if pairs[i].IsHalfPair() {
			continue
		}
		count := legacyPairOccupantCount(pairs[i])
		if count < bestCount {
			bestCount = count
			best = &pairs[i]
		}
	}
	return best
}

func legacyWearWeaponOrShield(c *Character, i items.Item, spec items.ItemSpec, iHandsRequired int, canDualWield bool) (returnItems []items.Item, newItemWorn bool, failureReason string) {
	pairs := c.GetHandPairs()
	isShield := spec.Type == items.Offhand

	if iHandsRequired >= 2 {
		freePair := legacyFindFirstFreePair(pairs)
		if freePair == nil {
			freePair = legacyFindCheapestPairToDisplace(pairs)
		}
		if freePair == nil {
			return returnItems, false, `You have no free pair of hands for a two-handed weapon.`
		}
		if !freePair.First.IsEmpty() && freePair.First.ItemPtr.IsCursed() {
			return returnItems, false, `Your ` + freePair.First.ItemPtr.DisplayName() + ` is cursed and prevents you from removing it.`
		}
		if !freePair.Second.IsEmpty() && freePair.Second.ItemPtr.IsCursed() {
			return returnItems, false, `Your ` + freePair.Second.ItemPtr.DisplayName() + ` is cursed and prevents you from removing it.`
		}
		if !freePair.First.IsEmpty() {
			returnItems = append(returnItems, *freePair.First.ItemPtr)
		}
		if !freePair.IsHalfPair() && !freePair.Second.IsEmpty() {
			returnItems = append(returnItems, *freePair.Second.ItemPtr)
		}
		*freePair.First.ItemPtr = i
		if !freePair.IsHalfPair() {
			*freePair.Second.ItemPtr = items.Item{}
		}
		c.reapplyPermanentConditions()
		return returnItems, true, ``
	}

	if isShield {
		slot := legacyFindFirstEmptySlot(c, pairs, true)
		if slot != nil {
			*slot.ItemPtr = i
			c.reapplyPermanentConditions()
			return returnItems, true, ``
		}
		if pairs[0].First.Is2H(c) {
			return returnItems, false, `Your two-handed weapon leaves no room for a shield.`
		}
		if pairs[0].Second.ItemPtr.IsCursed() {
			return returnItems, false, `Your ` + pairs[0].Second.ItemPtr.DisplayName() + ` is cursed and prevents you from removing it.`
		}
		returnItems = append(returnItems, *pairs[0].Second.ItemPtr)
		*pairs[0].Second.ItemPtr = i
		c.reapplyPermanentConditions()
		return returnItems, true, ``
	}

	bothMartial := spec.Subtype == items.Claws && c.Equipment.Weapon.GetSpec().Subtype == items.Claws

	slot := legacyFindFirstEmptySlot(c, pairs, false)
	if slot != nil {
		if slot.Label == "offhand" && !canDualWield && !bothMartial {
			slot = nil
			for pi := 1; pi < len(pairs); pi++ {
				p := &pairs[pi]
				if p.First.Is2H(c) {
					continue
				}
				if p.First.IsEmpty() {
					slot = &p.First
					break
				}
				if !p.IsHalfPair() && p.Second.IsEmpty() {
					slot = &p.Second
					break
				}
			}
		}
		if slot != nil {
			*slot.ItemPtr = i
			c.reapplyPermanentConditions()
			return returnItems, true, ``
		}
	}

	if c.Equipment.Weapon.IsCursed() {
		return returnItems, false, `Your ` + c.Equipment.Weapon.DisplayName() + ` is cursed and prevents you from removing it.`
	}
	if pairs[0].First.Is2H(c) && !pairs[0].Second.IsEmpty() {
		returnItems = append(returnItems, *pairs[0].Second.ItemPtr)
		*pairs[0].Second.ItemPtr = items.Item{}
	}
	returnItems = append(returnItems, c.Equipment.Weapon)
	c.Equipment.Weapon = i
	c.reapplyPermanentConditions()
	return returnItems, true, ``
}

func legacyWearRingOrWrist(c *Character, i items.Item, spec items.ItemSpec) (returnItems []items.Item, newItemWorn bool, failureReason string) {
	switch spec.Type {
	case items.Ring:
		if c.Equipment.Ring.IsDisabled() && c.Equipment.Ring2.IsDisabled() {
			return returnItems, false, `You can't wear rings.`
		}
		if !c.Equipment.Ring.IsDisabled() && c.Equipment.Ring.ItemId == 0 {
			c.Equipment.Ring = i
		} else if !c.Equipment.Ring2.IsDisabled() && c.Equipment.Ring2.ItemId == 0 {
			c.Equipment.Ring2 = i
		} else {
			returnItems = append(returnItems, c.Equipment.Ring)
			c.Equipment.Ring = i
		}
	case items.Wrist:
		if c.Equipment.Wrist1.IsDisabled() && c.Equipment.Wrist2.IsDisabled() {
			return returnItems, false, `You can't wear things on your wrists.`
		}
		if !c.Equipment.Wrist1.IsDisabled() && c.Equipment.Wrist1.ItemId == 0 {
			c.Equipment.Wrist1 = i
		} else if !c.Equipment.Wrist2.IsDisabled() && c.Equipment.Wrist2.ItemId == 0 {
			c.Equipment.Wrist2 = i
		} else if c.ExtraArms >= 1 && !c.Equipment.ExtraWrist1.IsDisabled() && c.Equipment.ExtraWrist1.ItemId == 0 {
			c.Equipment.ExtraWrist1 = i
		} else if c.ExtraArms >= 2 && !c.Equipment.ExtraWrist2.IsDisabled() && c.Equipment.ExtraWrist2.ItemId == 0 {
			c.Equipment.ExtraWrist2 = i
		} else if c.ExtraArms >= 3 && !c.Equipment.ExtraWrist3.IsDisabled() && c.Equipment.ExtraWrist3.ItemId == 0 {
			c.Equipment.ExtraWrist3 = i
		} else if c.ExtraArms >= 4 && !c.Equipment.ExtraWrist4.IsDisabled() && c.Equipment.ExtraWrist4.ItemId == 0 {
			c.Equipment.ExtraWrist4 = i
		} else {
			returnItems = append(returnItems, c.Equipment.Wrist1)
			c.Equipment.Wrist1 = i
		}
	}
	return returnItems, true, ``
}

// legacyWear is Wear's preamble (the gates before placement that these
// fixtures can reach) in front of the legacy placement.
func legacyWear(c *Character, i items.Item) ([]items.Item, bool, string) {
	i.Validate()
	spec := i.GetSpec()
	hands := c.HandsRequired(i)
	if hands > 2 {
		return nil, false, `That requires too many hands.`
	}
	if spec.Type == items.Weapon || spec.Type == items.Offhand {
		return legacyWearWeaponOrShield(c, i, spec, hands, c.CanDualWield())
	}
	return legacyWearRingOrWrist(c, i, spec)
}

// ---- fixtures, shared with wear_slot_test.go and wear_curse_test.go ----

const (
	goldenMedium = 0
	goldenSmall  = 1
	goldenLarge  = 2
)

func seedGoldenSpecies(t *testing.T) {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		goldenMedium: {SpeciesId: goldenMedium, Name: "human", Size: species.Medium},
		goldenSmall:  {SpeciesId: goldenSmall, Name: "halfling", Size: species.Small},
		goldenLarge:  {SpeciesId: goldenLarge, Name: "ogre", Size: species.Large},
	}))
}

var (
	goldenSword  = items.ItemSpec{Name: "sword", Type: items.Weapon, Subtype: items.Slashing, Hands: items.OneHanded}
	goldenClaws  = items.ItemSpec{Name: "claws", Type: items.Weapon, Subtype: items.Claws, Hands: items.OneHanded}
	goldenGreat  = items.ItemSpec{Name: "greatsword", Type: items.Weapon, Subtype: items.Slashing, Hands: items.TwoHanded}
	goldenShield = items.ItemSpec{Name: "buckler", Type: items.Offhand, Subtype: items.Wearable, Hands: items.OneHanded}
	goldenRing   = items.ItemSpec{Name: "ring", Type: items.Ring, Subtype: items.Wearable}
	goldenBracer = items.ItemSpec{Name: "bracer", Type: items.Wrist, Subtype: items.Wearable}
)

// goldenItem is a distinct instance: every placed item gets its own id, so a
// comparison can tell two swords apart.
func goldenItem(id int, spec items.ItemSpec, cursed bool) items.Item {
	s := spec
	s.ItemId = id
	s.Cursed = cursed
	return items.Item{ItemId: id, Spec: &s}
}

// goldenChar is a fresh character with extraArms arms beyond two (the
// mutation slots enabled exactly as Validate enables them), a species and
// optionally dual wield.
func goldenChar(extraArms, speciesId int, dual bool) *Character {
	c := New()
	c.SpeciesId = speciesId
	c.Mutations = map[string]int{}
	if extraArms > 0 {
		c.Mutations["extra-arms"] = extraArms
	}
	c.validateMutationSlots()
	// New() seeds every skill at rank 1 (ensureAllSkills), which is already
	// dual wield, so the non-dual case must clear it explicitly.
	if dual {
		c.SetSkill(string(skills.WeaponCombat), 1)
	} else {
		c.Skills[string(skills.WeaponCombat)] = 0
	}
	return c
}

// handSlotsInArmOrder lists the hand slots the character has, arm 1 first.
func handSlotsInArmOrder(c *Character) []*items.Item {
	var out []*items.Item
	for _, p := range c.GetHandPairs() {
		out = append(out, p.First.ItemPtr)
		if !p.IsHalfPair() {
			out = append(out, p.Second.ItemPtr)
		}
	}
	return out
}

// goldenHandLayouts enumerates every non-cursed hand layout: one code per
// hand slot in arm order. e empty, s sword, c claws (main hand only), b
// buckler (never the main hand), g greatsword (a full pair's First; its
// Second is then e).
func goldenHandLayouts(c *Character) []string {
	layouts := []string{``}
	for pi, p := range c.GetHandPairs() {
		firsts := "esb"
		if pi == 0 {
			firsts = "esc"
		}
		var opts []string
		for _, f := range firsts {
			if p.IsHalfPair() {
				opts = append(opts, string(f))
				continue
			}
			for _, s := range "esb" {
				opts = append(opts, string(f)+string(s))
			}
		}
		if !p.IsHalfPair() {
			opts = append(opts, "ge")
		}
		var next []string
		for _, l := range layouts {
			for _, o := range opts {
				next = append(next, l+o)
			}
		}
		layouts = next
	}
	return layouts
}

// goldenDisabledVariant turns every empty ('e') slot in a hand layout into a
// disabled one ('d'): the same ItemId < 0 marker the game actually writes,
// whether from species.DisabledSlots (the wielded pair) or an extra-arms
// mutation level below the slot's arm (ExtraArm1-4). Run alongside the plain
// layout so the oracle also pins how Wear treats a hand it cannot use at
// all, not just one that is merely empty.
func goldenDisabledVariant(layout string) string {
	return strings.ReplaceAll(layout, "e", "d")
}

var goldenCode = map[rune]items.ItemSpec{'s': goldenSword, 'c': goldenClaws, 'b': goldenShield, 'g': goldenGreat, 'r': goldenRing, 'w': goldenBracer}

// applyLayout writes one item per code into slots, numbering from *nextId.
// Upper case is the same item cursed; d writes the disabled-slot marker.
func applyLayout(slots []*items.Item, layout string, nextId *int) {
	for n, code := range layout {
		switch code {
		case 'e':
			continue
		case 'd':
			*slots[n] = items.ItemDisabledSlot
			continue
		}
		lower := []rune(strings.ToLower(string(code)))[0]
		*nextId++
		*slots[n] = goldenItem(*nextId, goldenCode[lower], lower != code)
	}
}

func goldenSnapshot(c *Character) string {
	var b strings.Builder
	for _, s := range c.Equipment.AllSlots() {
		fmt.Fprintf(&b, "%s=%d ", s.Key, s.Item.ItemId)
	}
	return b.String()
}

func goldenIds(list []items.Item) string {
	var b strings.Builder
	for _, it := range list {
		fmt.Fprintf(&b, "%d,", it.ItemId)
	}
	return b.String()
}

// sanctionedDivergence names the two placements the owner changed on purpose
// with nothing cursed: the legacy ring and wrist fallback that writes a
// disabled Ring or Wrist1 (it hands back the ItemId -1 marker; ruling 10
// never writes a disabled slot), and the shield beside a two-hander refused
// with every hand full at 3 or more arms (ruling 13 swaps the last available
// hand). Both have explicit tests in wear_slot_test.go. The ruling-13 case is
// not skipped blindly: ruling13Check below still asserts the real result
// against ruling 13 itself.
func sanctionedDivergence(extraArms int, legacyReturned []items.Item, legacyWhy string) bool {
	for _, it := range legacyReturned {
		if it.ItemId < 0 {
			return true
		}
	}
	return extraArms > 0 && legacyWhy == `Your two-handed weapon leaves no room for a shield.`
}

// ruling13Check runs where goldenCompare skips because legacy refused a
// shield with "no room" and there are extra arms (sanctionedDivergence):
// ruling 13 says the shield should take the highest arm that is not part of
// a two-hander, or refuse with the same line when every extra pair is itself
// two-handed. It reads the target pair from subject BEFORE calling Wear
// (Wear only ever touches that one pair), then checks Wear's real result
// against it, so the skip is no longer blind.
func ruling13Check(t *testing.T, name string, subject *Character, cand items.ItemSpec, failures *int) {
	t.Helper()
	report := func(format string, args ...any) {
		*failures++
		if *failures <= 20 {
			t.Errorf("%s: "+format, append([]any{name}, args...)...)
		}
	}
	pairs := subject.GetHandPairs()
	var target *HandPair
	for n := len(pairs) - 1; n >= 1; n-- {
		if !pairs[n].First.Is2H(subject) {
			target = &pairs[n]
			break
		}
	}
	_, sWorn, sWhy := subject.Wear(goldenItem(9000, cand, false))
	const noRoom = `Your two-handed weapon leaves no room for a shield.`
	if target == nil {
		if sWorn || sWhy != noRoom {
			report("ruling 13: want a refusal with no eligible hand, got worn=%v why=%q", sWorn, sWhy)
		}
		return
	}
	slot := target.First
	if !target.IsHalfPair() {
		slot = target.Second
	}
	if !sWorn {
		report("ruling 13: want the shield to take the last available hand (%s), got refused %q", slot.Label, sWhy)
		return
	}
	if slot.ItemPtr.ItemId != 9000 {
		report("ruling 13: want the shield in %s, got item %d there", slot.Label, slot.ItemPtr.ItemId)
	}
}

// goldenCompare runs the legacy oracle and Wear on two identically built
// characters and reports any difference.
func goldenCompare(t *testing.T, name string, extraArms int, build func() *Character, cand items.ItemSpec, failures *int) (compared bool) {
	t.Helper()
	oracle, subject := build(), build()
	oRet, oWorn, oWhy := legacyWear(oracle, goldenItem(9000, cand, false))
	if sanctionedDivergence(extraArms, oRet, oWhy) {
		const noRoom = `Your two-handed weapon leaves no room for a shield.`
		if extraArms > 0 && oWhy == noRoom {
			ruling13Check(t, name, subject, cand, failures)
		}
		return false
	}
	sRet, sWorn, sWhy := subject.Wear(goldenItem(9000, cand, false))
	report := func(format string, args ...any) {
		*failures++
		if *failures <= 20 {
			t.Errorf("%s: "+format, append([]any{name}, args...)...)
		}
	}
	if oWorn != sWorn || oWhy != sWhy {
		report("worn/reason legacy (%v, %q), Wear (%v, %q)", oWorn, oWhy, sWorn, sWhy)
	}
	if goldenIds(oRet) != goldenIds(sRet) {
		report("returned legacy [%s], Wear [%s]", goldenIds(oRet), goldenIds(sRet))
	}
	if a, b := goldenSnapshot(oracle), goldenSnapshot(subject); a != b {
		report("equipment\n legacy %s\n Wear   %s", a, b)
	}
	return true
}

func TestWearSlotChoice_GoldenHands(t *testing.T) {
	seedGoldenSpecies(t)
	candidates := []items.ItemSpec{goldenSword, goldenClaws, goldenGreat, goldenShield}
	compared, skipped, failures := 0, 0, 0
	for _, extra := range []int{0, 1, 2, 3, 4} {
		for _, sp := range []int{goldenMedium, goldenSmall, goldenLarge} {
			for _, dual := range []bool{false, true} {
				for _, layout := range goldenHandLayouts(goldenChar(extra, sp, dual)) {
					for _, variant := range []string{layout, goldenDisabledVariant(layout)} {
						build := func() *Character {
							c := goldenChar(extra, sp, dual)
							id := 1000
							applyLayout(handSlotsInArmOrder(c), variant, &id)
							return c
						}
						for _, cand := range candidates {
							name := fmt.Sprintf("arms=%d species=%d dual=%v hands=%s item=%s", 2+extra, sp, dual, variant, cand.Name)
							if goldenCompare(t, name, extra, build, cand, &failures) {
								compared++
							} else {
								skipped++
							}
						}
					}
				}
			}
		}
	}
	t.Logf("compared %d hand placements, skipped %d sanctioned", compared, skipped)
	if compared < 20000 {
		t.Fatalf("the sweep compared only %d placements: it is too small to pin anything", compared)
	}
	if failures > 0 {
		t.Fatalf("%d placement(s) differ from the legacy oracle", failures)
	}
}

func TestWearSlotChoice_GoldenRingsAndWrists(t *testing.T) {
	seedGoldenSpecies(t)
	compared, skipped, failures := 0, 0, 0
	for _, extra := range []int{0, 1, 2, 4} {
		// Rings: Ring and Ring2 each empty, full or disabled.
		for _, r1 := range "erd" {
			for _, r2 := range "erd" {
				layout := string(r1) + string(r2)
				build := func() *Character {
					c := goldenChar(extra, goldenMedium, false)
					id := 1000
					applyLayout([]*items.Item{&c.Equipment.Ring, &c.Equipment.Ring2}, layout, &id)
					return c
				}
				if goldenCompare(t, fmt.Sprintf("arms=%d rings=%s", 2+extra, layout), extra, build, goldenRing, &failures) {
					compared++
				} else {
					skipped++
				}
			}
		}
		// Wrists: Wrist1 and Wrist2 empty, full or disabled; each enabled
		// extra wrist empty or full.
		var layouts []string
		for _, w1 := range "ewd" {
			for _, w2 := range "ewd" {
				layouts = append(layouts, string(w1)+string(w2))
			}
		}
		for range extra {
			var next []string
			for _, l := range layouts {
				next = append(next, l+"e", l+"w")
			}
			layouts = next
		}
		for _, layout := range layouts {
			build := func() *Character {
				c := goldenChar(extra, goldenMedium, false)
				slots := []*items.Item{&c.Equipment.Wrist1, &c.Equipment.Wrist2,
					&c.Equipment.ExtraWrist1, &c.Equipment.ExtraWrist2, &c.Equipment.ExtraWrist3, &c.Equipment.ExtraWrist4}
				id := 1000
				applyLayout(slots[:len(layout)], layout, &id)
				return c
			}
			if goldenCompare(t, fmt.Sprintf("arms=%d wrists=%s", 2+extra, layout), extra, build, goldenBracer, &failures) {
				compared++
			} else {
				skipped++
			}
		}
	}
	t.Logf("compared %d ring and wrist placements, skipped %d sanctioned", compared, skipped)
	if skipped == 0 {
		t.Fatalf("no disabled-slot fallback was generated: the carve-out is untested")
	}
	if failures > 0 {
		t.Fatalf("%d placement(s) differ from the legacy oracle", failures)
	}
}
