package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
)

// seedTailAndGreaves registers a tail mutation carrying the disable-legs flag
// (as tail.yaml ships) and two items: greaves for the legs slot and a load
// heavy enough to put the character past StoreItem's 2x carry limit.
func seedTailAndGreaves(t *testing.T) {
	t.Helper()
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"tail": {MutationId: "tail", Name: "Tail",
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "tail", Value: 1}},
			Cons: []mutations.MutationEffect{{Type: "flag", Target: "disable-legs", Value: 1}},
		},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999480: {ItemId: 999480, Name: "iron greaves", Type: items.Legs, Weight: 10},
		999481: {ItemId: 999481, Name: "anvil", Type: items.Object, Weight: 1000},
	}))
}

// #481: a tail mutation that disables the legs slot must never destroy the
// item it pushes out. StoreItem refuses past 2x carry capacity and the old
// code ignored that refusal, so the greaves simply vanished.
func TestValidate_TailNeverDestroysLegArmourOverCarryLimit(t *testing.T) {
	seedTailAndGreaves(t)

	c := New()
	c.carryCapacityOverride = 10
	c.Items = append(c.Items, items.New(999481)) // far past 2x capacity already
	c.Equipment.Legs = items.New(999480)
	if c.StoreItem(items.New(999480)) {
		t.Fatal("fixture: StoreItem must refuse the greaves for this test to mean anything")
	}

	c.Mutations = map[string]int{"tail": 1}
	c.validateMutationSlots()

	if !c.Equipment.Legs.IsDisabled() {
		t.Errorf("legs slot should be disabled under a tail, got item %d", c.Equipment.Legs.ItemId)
	}
	found := false
	for _, it := range c.Items {
		if it.ItemId == 999480 {
			found = true
		}
	}
	if !found {
		t.Fatal("the greaves were destroyed: not in the pack after the tail displaced them")
	}
}

// The displacement is recorded for a player so the gain hook can tell them,
// once: TakeDisplacedGear drains the record.
func TestValidate_TailRecordsDisplacedLegArmourForAPlayer(t *testing.T) {
	seedTailAndGreaves(t)

	c := New()
	c.SetUserId(4810)
	c.Equipment.Legs = items.New(999480)
	c.Mutations = map[string]int{"tail": 1}
	c.validateMutationSlots()

	got := c.TakeDisplacedGear()
	if len(got) != 1 || !strings.EqualFold(got[0].ItemName, "iron greaves") || got[0].Reason != DisplacedByDisabledLegs {
		t.Fatalf("want one greaves record with reason %q, got %+v", DisplacedByDisabledLegs, got)
	}
	if again := c.TakeDisplacedGear(); len(again) != 0 {
		t.Errorf("TakeDisplacedGear must drain the record, second call got %+v", again)
	}

	// A later validate with the slot already disabled records nothing new.
	c.validateMutationSlots()
	if again := c.TakeDisplacedGear(); len(again) != 0 {
		t.Errorf("an already disabled slot displaced again: %+v", again)
	}
}

// A mob's displacement is not recorded: nothing would ever drain it.
func TestValidate_TailRecordsNothingForAMob(t *testing.T) {
	seedTailAndGreaves(t)

	c := New()
	c.Equipment.Legs = items.New(999480)
	c.Mutations = map[string]int{"tail": 1}
	c.validateMutationSlots()

	if got := c.TakeDisplacedGear(); len(got) != 0 {
		t.Errorf("a character with no user id recorded %+v", got)
	}
}
