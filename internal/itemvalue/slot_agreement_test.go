package itemvalue

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The scorer and Wear make one slot choice (spec rulings 10 and 12): for any
// arm count and any mix of cursed and plain gear, ItemValueDelta reports the
// slot Wear fills and the items Wear displaces, or both refuse.

var (
	agSword  = items.ItemSpec{Name: "sword", Type: items.Weapon, Subtype: items.Slashing, Hands: items.OneHanded}
	agGreat  = items.ItemSpec{Name: "greatsword", Type: items.Weapon, Subtype: items.Slashing, Hands: items.TwoHanded}
	agShield = items.ItemSpec{Name: "buckler", Type: items.Offhand, Subtype: items.Wearable, Hands: items.OneHanded}
	agRing   = items.ItemSpec{Name: "ring", Type: items.Ring, Subtype: items.Wearable}
	agBracer = items.ItemSpec{Name: "bracer", Type: items.Wrist, Subtype: items.Wearable}
)

func agItem(id int, spec items.ItemSpec, cursed bool) items.Item {
	s := spec
	s.ItemId, s.Cursed = id, cursed
	return items.Item{ItemId: id, Spec: &s}
}

func seedAgreementSpecies(t *testing.T) {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", Size: species.Medium},
		1: {SpeciesId: 1, Name: "halfling", Size: species.Small},
		2: {SpeciesId: 2, Name: "ogre", Size: species.Large},
	}))
}

// agreementChar builds the same random loadout for the same seed.
func agreementChar(extra int, seed int64) *characters.Character {
	rng := rand.New(rand.NewSource(seed))
	c := characters.New()
	c.SpeciesId = rng.Intn(3)
	c.Mutations = map[string]int{}
	if extra > 0 {
		c.Mutations["extra-arms"] = extra
	}
	c.Validate()
	if rng.Intn(2) == 1 {
		c.SetSkill("weapon-combat", 1)
	}
	id := 1000
	put := func(p *items.Item, spec items.ItemSpec) {
		id++
		*p = agItem(id, spec, rng.Intn(3) == 0)
	}
	for _, p := range c.GetHandPairs() {
		switch rng.Intn(5) {
		case 0:
		case 1:
			put(p.First.ItemPtr, agSword)
		case 2:
			put(p.First.ItemPtr, agShield)
		case 3:
			if !p.IsHalfPair() {
				put(p.First.ItemPtr, agGreat)
				continue
			}
		case 4:
			put(p.First.ItemPtr, agSword)
		}
		if !p.IsHalfPair() && rng.Intn(2) == 1 {
			put(p.Second.ItemPtr, [...]items.ItemSpec{agSword, agShield}[rng.Intn(2)])
		}
	}
	for _, p := range []*items.Item{&c.Equipment.Ring, &c.Equipment.Ring2} {
		if rng.Intn(3) > 0 {
			put(p, agRing)
		}
	}
	wrists := []*items.Item{&c.Equipment.Wrist1, &c.Equipment.Wrist2, &c.Equipment.ExtraWrist1, &c.Equipment.ExtraWrist2, &c.Equipment.ExtraWrist3, &c.Equipment.ExtraWrist4}
	for _, p := range wrists[:2+extra] {
		if rng.Intn(3) > 0 {
			put(p, agBracer)
		}
	}
	return c
}

func agIds(list []items.Item) string {
	s := ""
	for _, it := range list {
		s += fmt.Sprintf("%d,", it.ItemId)
	}
	return s
}

func TestScorerAndWearAgreeOnEverySlotChoice(t *testing.T) {
	seedAgreementSpecies(t)
	rng := rand.New(rand.NewSource(20260929))
	checked, refused := 0, 0
	for _, extra := range []int{0, 1, 2, 4} {
		for range 1500 {
			seed := rng.Int63()
			for _, cs := range []items.ItemSpec{agSword, agGreat, agShield, agRing, agBracer} {
				delta := ItemValueDelta(agreementChar(extra, seed), PhysicalBruiser, agItem(9000, cs, false))
				worn := agreementChar(extra, seed)
				ret, ok, why := worn.Wear(agItem(9000, cs, false))
				name := fmt.Sprintf("arms=%d seed=%d item=%s", 2+extra, seed, cs.Name)
				checked++
				if !ok {
					refused++
					assert.Equal(t, SlotName(""), delta.Slot, "%s: Wear refused (%q) but the scorer offered %s", name, why, delta.Slot)
					continue
				}
				placed := SlotName("")
				for _, s := range worn.Equipment.AllSlots() {
					if s.Item.ItemId == 9000 {
						placed = chooserSlotName[s.Key]
					}
				}
				require.Equal(t, placed, delta.Slot, "%s", name)
				assert.Equal(t, agIds(ret), agIds(delta.Displaced), "%s", name)
			}
		}
	}
	t.Logf("checked %d choices, %d refused by both", checked, refused)
	require.Greater(t, refused, 0, "no refusal was generated: the curse side is untested")
}

// With both rings full and nothing cursed the scorer weighs the swap Wear
// makes (Ring), even when Ring2 holds the weaker ring; with Ring cursed it
// weighs Ring2; with both cursed it offers nothing (spec testing, 5a ring).
func TestItemValueDelta_RingFollowsTheHelper(t *testing.T) {
	c := &characters.Character{Mutations: map[string]int{}}
	strong := agItem(1, agRing, false)
	strong.Spec.StatMods = map[string]int{"strength": 10}
	c.Equipment.Ring = strong
	c.Equipment.Ring2 = agItem(2, agRing, false)
	cand := agItem(3, agRing, false)
	cand.Spec.StatMods = map[string]int{"strength": 5}

	assert.Equal(t, SlotRing, ItemValueDelta(c, PhysicalBruiser, cand).Slot)

	c.Equipment.Ring.Spec.Cursed = true
	assert.Equal(t, SlotRing2, ItemValueDelta(c, PhysicalBruiser, cand).Slot)

	c.Equipment.Ring2.Spec.Cursed = true
	assert.Equal(t, SwapDelta{}, ItemValueDelta(c, PhysicalBruiser, cand))
	assert.False(t, IsUpgrade(c, PhysicalBruiser, cand))
}

// A single-slot swap a curse would refuse is never an upgrade (E8).
func TestItemValueDelta_SkipsACursedSingleSlot(t *testing.T) {
	c := &characters.Character{Mutations: map[string]int{}}
	c.Equipment.Head = agItem(1, items.ItemSpec{Name: "hexed helm", Type: items.Head, Subtype: items.Wearable}, true)
	cand := agItem(2, items.ItemSpec{Name: "fine cap", Type: items.Head, Subtype: items.Wearable, StatMods: map[string]int{"strength": 5}}, false)
	assert.Equal(t, SwapDelta{}, ItemValueDelta(c, PhysicalBruiser, cand))
}
