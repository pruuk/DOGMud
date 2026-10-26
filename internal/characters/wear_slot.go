package characters

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// SlotChoice is where ChooseWornSlot puts an item: Slots[0] receives it and
// every later entry is cleared (a two-hander's partner slot, a stray behind
// a two-hander, or the two-hander an even arm breaks). Displaced is
// everything that comes off, in the order Wear hands it back.
type SlotChoice struct {
	Slots     []WornSlot
	Displaced []items.Item
}

// apply writes i into the chosen slot and clears the rest.
func (ch SlotChoice) apply(i items.Item) {
	for n, s := range ch.Slots {
		if n == 0 {
			*s.Item = i
			continue
		}
		*s.Item = items.Item{}
	}
}

// CursedRefusal is the one statement of the equip curse rule: a worn item
// that is cursed cannot be displaced by putting something else on. It has no
// Health and no Spellcasting condition, because player equip has honoured
// neither (spec ruling 8); remove keeps its own exception
// (actions.CursedHolds), so a Spellcasting-4 wearer frees the slot with
// `remove` first.
func (c *Character) CursedRefusal(it items.Item) string {
	if it.ItemId > 0 && it.IsCursed() {
		return `Your ` + it.DisplayName() + ` is cursed and prevents you from removing it.`
	}
	return ``
}

// slotCandidate is one place an item could go.
type slotCandidate struct {
	slots     []*items.Item // [0] receives the item; the rest are cleared
	displaced []items.Item  // what comes off, in Wear's return order
	guarded   []items.Item  // the same items in the order the curse check reads them
}

// occupant is a candidate that writes one slot and takes off what is in it.
func occupant(p *items.Item) slotCandidate {
	cand := slotCandidate{slots: []*items.Item{p}}
	if p.ItemId > 0 {
		cand.displaced = []items.Item{*p}
		cand.guarded = cand.displaced
	}
	return cand
}

// ChooseWornSlot decides where a ring, wrist, weapon or shield goes, over the
// arms and wrists this character actually has (2 to 6 arms). Wear, WearInArm
// and the itemvalue scorer all call it, so they agree for any arm count.
//
// arm is 0 for the automatic choice and 1 to 6 for `equip X armN`. The rule,
// over an ordered candidate list: fill the first empty candidate; else swap
// the first candidate none of whose displaced items CursedRefusal refuses;
// else refuse with CursedRefusal of the first candidate's first cursed item.
// The first candidate is always today's fallback, so with nothing cursed the
// choice is today's (the golden test pins it), apart from the two changes
// rulings 10 and 13 made on purpose.
//
// It never mutates. For any other item type it returns an empty choice and
// no refusal.
func (c *Character) ChooseWornSlot(i items.Item, arm int) (choice SlotChoice, refusal string) {
	spec := i.GetSpec()
	if arm > 0 {
		return c.chooseArm(i, spec, arm)
	}
	switch spec.Type {
	case items.Weapon, items.Offhand:
		if c.HandsRequired(i) > 2 {
			return SlotChoice{}, `That requires too many hands.`
		}
		fill, swap, none := c.handCandidates(i, spec)
		return c.chooseFrom(fill, swap, none)
	case items.Ring:
		e := &c.Equipment
		if e.Ring.IsDisabled() && e.Ring2.IsDisabled() {
			return SlotChoice{}, `You can't wear rings.`
		}
		var list []slotCandidate
		for _, p := range []*items.Item{&e.Ring, &e.Ring2} {
			if !p.IsDisabled() {
				list = append(list, occupant(p))
			}
		}
		return c.chooseFrom(list, list, `You can't wear rings.`)
	case items.Wrist:
		e := &c.Equipment
		if e.Wrist1.IsDisabled() && e.Wrist2.IsDisabled() {
			return SlotChoice{}, `You can't wear things on your wrists.`
		}
		wrists := []*items.Item{&e.Wrist1, &e.Wrist2}
		for n, p := range []*items.Item{&e.ExtraWrist1, &e.ExtraWrist2, &e.ExtraWrist3, &e.ExtraWrist4} {
			if c.ExtraArms >= n+1 {
				wrists = append(wrists, p)
			}
		}
		var list []slotCandidate
		for _, p := range wrists {
			if !p.IsDisabled() {
				list = append(list, occupant(p))
			}
		}
		return c.chooseFrom(list, list, `You can't wear things on your wrists.`)
	}
	return SlotChoice{}, ``
}

// chooseFrom applies the one rule. fill and swap are the same list except
// where today fills in one order and swaps in another (2H pairs, ruling 13).
func (c *Character) chooseFrom(fill, swap []slotCandidate, none string) (SlotChoice, string) {
	for _, cand := range fill {
		if len(cand.displaced) == 0 {
			return c.slotChoice(cand), ``
		}
	}
	for _, cand := range swap {
		if c.cursedAmong(cand.guarded) == `` {
			return c.slotChoice(cand), ``
		}
	}
	if len(swap) > 0 {
		return SlotChoice{}, c.cursedAmong(swap[0].guarded)
	}
	return SlotChoice{}, none
}

func (c *Character) cursedAmong(its []items.Item) string {
	for _, it := range its {
		if r := c.CursedRefusal(it); r != `` {
			return r
		}
	}
	return ``
}

// slotChoice names a candidate's slots by their AllSlots entries.
func (c *Character) slotChoice(cand slotCandidate) SlotChoice {
	ch := SlotChoice{Displaced: cand.displaced}
	all := c.Equipment.AllSlots()
	for _, p := range cand.slots {
		for _, s := range all {
			if s.Item == p {
				ch.Slots = append(ch.Slots, s)
				break
			}
		}
	}
	return ch
}

// handCandidates builds the weapon and shield lists (spec "Slot choice").
func (c *Character) handCandidates(i items.Item, spec items.ItemSpec) (fill, swap []slotCandidate, none string) {
	pairs := c.GetHandPairs()

	// Two-hander: whole pairs only, a free pair first, then fewest
	// occupants, the earlier pair on a tie (today's FindFirstFreePair then
	// FindCheapestPairToDisplace, as one stable sort).
	if c.HandsRequired(i) >= 2 {
		for _, p := range pairs {
			if p.IsHalfPair() {
				continue
			}
			cand := slotCandidate{slots: []*items.Item{p.First.ItemPtr, p.Second.ItemPtr}}
			if !p.First.IsEmpty() {
				cand.displaced = append(cand.displaced, *p.First.ItemPtr)
			}
			if !p.Second.IsEmpty() {
				cand.displaced = append(cand.displaced, *p.Second.ItemPtr)
			}
			cand.guarded = cand.displaced
			fill = append(fill, cand)
		}
		swap = append([]slotCandidate(nil), fill...)
		sort.SliceStable(swap, func(a, b int) bool { return len(swap[a].displaced) < len(swap[b].displaced) })
		// #680: a character grown Large without Colossus Form holds a
		// two-hander alone in the weapon hand only, so that hand is offered
		// first, as one slot.
		if c.HandsRequiredAt(i, pairs[0].First.Label) < 2 {
			main := occupant(pairs[0].First.ItemPtr)
			fill = append([]slotCandidate{main}, fill...)
			swap = append([]slotCandidate{main}, swap...)
		}
		return fill, swap, `You have no free pair of hands for a two-handed weapon.`
	}

	weapon2H := pairs[0].First.Is2H(c)

	// Shield: Offhand unless the main hands hold a two-hander, then the
	// extra arms in arm order.
	if spec.Type == items.Offhand {
		if !weapon2H {
			fill = append(fill, occupant(pairs[0].Second.ItemPtr))
		}
		fill = append(fill, c.handSlotCandidates(pairs, 1)...)
		if !weapon2H {
			return fill, fill, ``
		}
		// Ruling 13: beside a two-hander in the main hands, with no hand
		// empty, the shield takes the LAST available hand: the highest arm,
		// counting down, that is not part of a two-hander.
		for n := len(pairs) - 1; n >= 1; n-- {
			p := pairs[n]
			if p.First.Is2H(c) {
				continue
			}
			if !p.IsHalfPair() {
				swap = append(swap, occupant(p.Second.ItemPtr))
			}
			swap = append(swap, occupant(p.First.ItemPtr))
		}
		return fill, swap, `Your two-handed weapon leaves no room for a shield.`
	}

	// One-hander: arms in order. Offhand only for a dual wielder or claws
	// over claws; a pair holding a two-hander offers only its First.
	bothMartial := spec.Subtype == items.Claws && c.Equipment.Weapon.GetSpec().Subtype == items.Claws
	if weapon2H {
		fill = append(fill, c.twoHanderSlot(pairs[0]))
	} else {
		fill = append(fill, occupant(pairs[0].First.ItemPtr))
		if c.CanDualWield() || bothMartial {
			fill = append(fill, occupant(pairs[0].Second.ItemPtr))
		}
	}
	fill = append(fill, c.handSlotCandidates(pairs, 1)...)
	return fill, fill, `You have no free hand for that.`
}

// handSlotCandidates lists pairs[from:] in arm order. A pair whose First
// holds a two-hander offers only that First: its Second is consumed.
func (c *Character) handSlotCandidates(pairs []HandPair, from int) []slotCandidate {
	var out []slotCandidate
	for _, p := range pairs[from:] {
		if p.First.Is2H(c) {
			out = append(out, c.twoHanderSlot(p))
			continue
		}
		out = append(out, occupant(p.First.ItemPtr))
		if !p.IsHalfPair() {
			out = append(out, occupant(p.Second.ItemPtr))
		}
	}
	return out
}

// twoHanderSlot is the First of a pair holding a two-hander: writing it takes
// the two-hander off, and any stray left behind it in the Second too. The
// stray comes back first (today's order); the curse check reads the
// two-hander first (today's order, which never read the stray at all).
func (c *Character) twoHanderSlot(p HandPair) slotCandidate {
	cand := slotCandidate{slots: []*items.Item{p.First.ItemPtr}}
	var stray []items.Item
	if !p.IsHalfPair() && !p.Second.IsEmpty() {
		cand.slots = append(cand.slots, p.Second.ItemPtr)
		stray = []items.Item{*p.Second.ItemPtr}
	}
	cand.displaced = append(append([]items.Item(nil), stray...), *p.First.ItemPtr)
	cand.guarded = append([]items.Item{*p.First.ItemPtr}, stray...)
	return cand
}

// chooseArm is `equip X armN` (ruling 11 and 12): the player's shape
// refusals, then the one slot the arm names. A cursed item there refuses; it
// never falls through to another arm.
func (c *Character) chooseArm(i items.Item, spec items.ItemSpec, arm int) (SlotChoice, string) {
	if spec.Type != items.Weapon && spec.Type != items.Offhand {
		return SlotChoice{}, `You can only wield weapons or shields in arm slots.`
	}
	if spec.Type == items.Offhand && arm == 1 {
		return SlotChoice{}, `You can't put a shield in your primary weapon hand (arm 1).`
	}
	pairs := c.GetHandPairs()
	pairIdx, slotInPair := (arm-1)/2, (arm-1)%2
	if pairIdx >= len(pairs) || (slotInPair == 1 && pairs[pairIdx].IsHalfPair()) {
		return SlotChoice{}, fmt.Sprintf(`You don't have arm %d.`, arm)
	}
	if c.HandsRequired(i) > 2 {
		return SlotChoice{}, `That requires too many hands.`
	}
	pair := pairs[pairIdx]
	target := pair.First
	if slotInPair == 1 {
		target = pair.Second
	}
	twoHanded := c.HandsRequiredAt(i, target.Label) >= 2
	if twoHanded {
		if slotInPair != 0 {
			return SlotChoice{}, `A two-handed weapon needs a pair of arms. Try arm 1, 3, or 5.`
		}
		if pair.IsHalfPair() {
			return SlotChoice{}, `That arm doesn't have a partner for a two-handed weapon.`
		}
	}

	cand := slotCandidate{slots: []*items.Item{target.ItemPtr}}
	// The curse check reads what the arm branch read, in its order: the
	// target, a two-hander's second slot, the pair's First, then a two-handed
	// partner beside an even arm.
	if !target.IsEmpty() {
		cand.guarded = append(cand.guarded, *target.ItemPtr)
	}
	if twoHanded {
		if !pair.Second.IsEmpty() {
			cand.guarded = append(cand.guarded, *pair.Second.ItemPtr)
		}
		if !pair.First.IsEmpty() {
			cand.guarded = append(cand.guarded, *pair.First.ItemPtr)
		}
	}
	if slotInPair == 1 && pair.First.Is2H(c) {
		cand.guarded = append(cand.guarded, *pair.First.ItemPtr)
		cand.displaced = append(cand.displaced, *pair.First.ItemPtr)
		cand.slots = append(cand.slots, pair.First.ItemPtr)
	}
	if !target.IsEmpty() {
		cand.displaced = append(cand.displaced, *target.ItemPtr)
	}
	if twoHanded && !pair.Second.IsEmpty() {
		cand.displaced = append(cand.displaced, *pair.Second.ItemPtr)
		cand.slots = append(cand.slots, pair.Second.ItemPtr)
	}
	if r := c.cursedAmong(cand.guarded); r != `` {
		return SlotChoice{}, r
	}
	return c.slotChoice(cand), ``
}
