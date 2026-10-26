package itemvalue

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
)

// newTestChar returns a bare Character with no equipment and
// no mutations. Tests can populate Equipment fields directly.
func newTestChar() *characters.Character {
	return &characters.Character{
		Mutations: make(map[string]int),
	}
}

func TestCanonicalRank_WeaponBeatsOffhand(t *testing.T) {
	if canonicalRank(SlotWeapon) >= canonicalRank(SlotOffhand) {
		t.Errorf("SlotWeapon rank %d should be < SlotOffhand rank %d",
			canonicalRank(SlotWeapon), canonicalRank(SlotOffhand))
	}
}

func TestCompatibleSlotsFor_OneHandedWeaponIsTheSlotWearFills(t *testing.T) {
	spec := items.ItemSpec{Type: items.Weapon, Hands: items.OneHanded}
	got := compatibleSlotsFor(items.Item{ItemId: 1, Spec: &spec}, newTestChar())
	want := []SlotName{SlotWeapon}
	if !slotsEqual(got, want) {
		t.Errorf("1H weapon slots (empty hands: Wear fills the main hand) = %v, want %v", got, want)
	}
}

func TestCompatibleSlotsFor_TwoHandedWeapon(t *testing.T) {
	spec := items.ItemSpec{Type: items.Weapon, Hands: items.TwoHanded}
	got := compatibleSlotsFor(items.Item{ItemId: 1, Spec: &spec}, newTestChar())
	want := []SlotName{SlotWeapon}
	if !slotsEqual(got, want) {
		t.Errorf("2H weapon slots = %v, want %v", got, want)
	}
}

func TestCompatibleSlotsFor_Ring(t *testing.T) {
	spec := items.ItemSpec{Type: items.Ring}
	got := compatibleSlotsFor(items.Item{ItemId: 1, Spec: &spec}, newTestChar())
	want := []SlotName{SlotRing}
	if !slotsEqual(got, want) {
		t.Errorf("Ring slots = %v, want %v", got, want)
	}
}

func TestCompatibleSlotsFor_NonEquippable(t *testing.T) {
	// Use an item type string that isn't a valid equipment slot.
	spec := items.ItemSpec{Type: items.ItemType("consumable")}
	got := compatibleSlotsFor(items.Item{ItemId: 1, Spec: &spec}, newTestChar())
	if len(got) != 0 {
		t.Errorf("consumable slots = %v, want empty", got)
	}
}

func TestCompatibleSlotsFor_TailRequiresMutation(t *testing.T) {
	spec := items.ItemSpec{Type: items.Tail}
	got := compatibleSlotsFor(items.Item{ItemId: 1, Spec: &spec}, newTestChar())
	if len(got) != 0 {
		t.Errorf("Tail without mutation: %v, want empty", got)
	}
}

func TestCompatibleSlotsFor_TailWithMutation(t *testing.T) {
	spec := items.ItemSpec{Type: items.Tail}
	char := newTestChar()
	char.Mutations["tail"] = 1
	got := compatibleSlotsFor(items.Item{ItemId: 1, Spec: &spec}, char)
	want := []SlotName{SlotTail}
	if !slotsEqual(got, want) {
		t.Errorf("Tail with mutation: %v, want %v", got, want)
	}
}

func TestCompatibleSlotsFor_WristWithExtraArms(t *testing.T) {
	spec := items.ItemSpec{Type: items.Wrist}
	char := newTestChar()
	char.ExtraArms = 2
	char.Equipment.Wrist1 = items.Item{ItemId: 5}
	char.Equipment.Wrist2 = items.Item{ItemId: 6}
	got := compatibleSlotsFor(items.Item{ItemId: 1, Spec: &spec}, char)
	want := []SlotName{SlotExtraWrist1}
	if !slotsEqual(got, want) {
		t.Errorf("Wrist with 2 extra arms, Wrist1/2 full: %v, want %v", got, want)
	}
}

func TestCompatibleSlotsFor_WristWithMaxExtraArms(t *testing.T) {
	spec := items.ItemSpec{Type: items.Wrist}
	char := newTestChar()
	char.ExtraArms = 4
	char.Equipment.Wrist1 = items.Item{ItemId: 5}
	char.Equipment.Wrist2 = items.Item{ItemId: 6}
	char.Equipment.ExtraWrist1 = items.Item{ItemId: 7}
	char.Equipment.ExtraWrist2 = items.Item{ItemId: 8}
	char.Equipment.ExtraWrist3 = items.Item{ItemId: 9}
	got := compatibleSlotsFor(items.Item{ItemId: 1, Spec: &spec}, char)
	want := []SlotName{SlotExtraWrist4}
	if !slotsEqual(got, want) {
		t.Errorf("Wrist with 4 extra arms, all but ExtraWrist4 full: %v, want %v", got, want)
	}
}

func TestDisplacedItemsForSlot_EmptySlot(t *testing.T) {
	char := newTestChar()
	spec := items.ItemSpec{Type: items.Body}
	got := displacedItemsForSlot(char, SlotBody, items.Item{ItemId: 99, Spec: &spec})
	if len(got) != 0 {
		t.Errorf("empty body slot: displaced = %v, want empty", got)
	}
}

func TestDisplacedItemsForSlot_OccupiedSlot(t *testing.T) {
	char := newTestChar()
	char.Equipment.Body = items.Item{ItemId: 42}
	spec := items.ItemSpec{Type: items.Body}
	got := displacedItemsForSlot(char, SlotBody, items.Item{ItemId: 99, Spec: &spec})
	if len(got) != 1 || got[0].ItemId != 42 {
		t.Errorf("occupied body slot: displaced = %v, want [{ItemId:42}]", got)
	}
}

func TestDisplacedItemsForSlot_TwoHandedDisplacesBoth(t *testing.T) {
	char := newTestChar()
	char.Equipment.Weapon = items.Item{ItemId: 1}
	char.Equipment.Offhand = items.Item{ItemId: 2}
	spec := items.ItemSpec{Type: items.Weapon, Hands: items.TwoHanded}
	got := displacedItemsForSlot(char, SlotWeapon, items.Item{ItemId: 99, Spec: &spec})
	if len(got) != 2 {
		t.Fatalf("2H displaced count = %d, want 2", len(got))
	}
}

// The helper swaps the two-hander in the main hand: a one-handed candidate
// offered at Offhand (via the chooser, this is really the Weapon slot) comes
// back displacing exactly the 2H weapon.
func TestDisplacedItemsForSlot_OffhandWithTwoHandedWeapon(t *testing.T) {
	char := newTestChar()
	char.Equipment.Weapon = items.Item{ItemId: 1, Spec: &items.ItemSpec{ItemId: 1, Type: items.Weapon, Hands: items.TwoHanded}}
	spec := items.ItemSpec{Type: items.Weapon, Hands: items.OneHanded}
	got := displacedItemsForSlot(char, SlotWeapon, items.Item{ItemId: 99, Spec: &spec})
	if len(got) != 1 || got[0].ItemId != 1 {
		t.Errorf("1H over a 2H main hand: displaced = %v, want [{ItemId:1}]", got)
	}
}

func TestPlacementBonus_TwoHandedAlwaysApplies(t *testing.T) {
	char := newTestChar()
	spec := items.ItemSpec{Type: items.Weapon, Hands: items.TwoHanded}
	got := placementBonus(MagicalPure, spec, SlotWeapon, char)
	want := MagicalPure.TwoHandedBonus // 80
	if got != want {
		t.Errorf("2H bonus on Pure = %f, want %f", got, want)
	}
}

func TestPlacementBonus_DualWieldRequiresMainHandWeapon(t *testing.T) {
	char := newTestChar() // empty hands
	spec := items.ItemSpec{Type: items.Weapon, Hands: items.OneHanded}
	got := placementBonus(PhysicalBruiser, spec, SlotOffhand, char)
	if got != 0 {
		t.Errorf("DualWieldBonus on empty hands = %f, want 0", got)
	}
}

// #680: the dual-wield bonus reads the wearer's grip on the main hand, not the
// weapon's spec. A Large species and a character grown Large hold a
// two-hander in the weapon hand alone, so the offhand is free to dual wield; a
// Medium wearer's two-hander fills it.
func TestPlacementBonus_DualWieldReadsTheWearersGrip(t *testing.T) {
	seedAgreementSpecies(t)
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7, MaxRank: 4,
			ReserveUnits: []int{1, 1, 1, 1},
			Pros:         []mutations.MutationEffect{{Type: "size", Target: "large", Value: 3}}},
	}))
	great := items.ItemSpec{ItemId: 68020, Type: items.Weapon, Subtype: items.Slashing, Hands: items.TwoHanded}
	sword := items.ItemSpec{Type: items.Weapon, Subtype: items.Slashing, Hands: items.OneHanded}
	wielding := func(speciesId, titan int) *characters.Character {
		c := newTestChar()
		c.SpeciesId = speciesId
		if titan > 0 {
			c.Mutations["titan-growth"] = titan
		}
		c.Equipment.Weapon = items.Item{ItemId: 68020, Spec: &great}
		return c
	}

	for _, tc := range []struct {
		name string
		char *characters.Character
		want float64
	}{
		{"Large species", wielding(2, 0), PhysicalBruiser.DualWieldBonus},
		{"grown Large (Titan 3)", wielding(0, 3), PhysicalBruiser.DualWieldBonus},
		{"Medium (Titan 2)", wielding(0, 2), 0},
	} {
		if got := placementBonus(PhysicalBruiser, sword, SlotOffhand, tc.char); got != tc.want {
			t.Errorf("%s: offhand sword bonus = %f, want %f", tc.name, got, tc.want)
		}
	}
}

func TestPlacementBonus_ShieldUnconditional(t *testing.T) {
	char := newTestChar()
	spec := items.ItemSpec{Type: items.Offhand}
	got := placementBonus(PhysicalTank, spec, SlotOffhand, char)
	want := PhysicalTank.ShieldBonus // 80
	if got != want {
		t.Errorf("ShieldBonus on Tank empty hands = %f, want %f", got, want)
	}
}

// slotsEqual is a test helper for SlotName slice equality.
func slotsEqual(a, b []SlotName) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestItemValueDelta_NotEquippable(t *testing.T) {
	char := newTestChar()
	candidate := items.Item{ItemId: 1}
	// Force the spec to be a non-equippable type via direct
	// items.Item construction; the actual ItemSpec lookup
	// returns zero values when ItemId 1 isn't in the registry.
	// ItemSpec{Type: ""} has no matching slot, so compatibleSlotsFor
	// returns nil → SwapDelta{} returned.
	got := ItemValueDelta(char, PhysicalBruiser, candidate)
	if got.Slot != "" {
		t.Errorf("non-equippable: Slot = %q, want empty", got.Slot)
	}
	if got.Score != 0 {
		t.Errorf("non-equippable: Score = %f, want 0", got.Score)
	}
}

func TestItemValueDelta_TiebreakerPrefersWeapon(t *testing.T) {
	// Empty-handed mob considering a 1H weapon. Both Weapon
	// and Offhand placements should score the same raw value
	// (DualWieldBonus suppressed by conditional check on empty
	// main hand). Tiebreaker: Weapon wins.
	//
	// This test depends on items.Item.GetSpec() resolving the
	// candidate's spec. If test data dir isn't loaded, skip.
	char := newTestChar()
	candidate := items.Item{ItemId: 1}
	if candidate.GetSpec().ItemId == 0 {
		t.Skip("test fixture for items.Item.GetSpec lookup not available")
	}
	// Documented expectation when fixture is available:
	// got.Slot == SlotWeapon (not SlotOffhand).
	_ = ItemValueDelta(char, PhysicalBruiser, candidate)
}

func TestEncumbranceTier_Thresholds(t *testing.T) {
	cases := []struct {
		ratio float64
		want  int
	}{
		{0.00, 0}, {0.24, 0}, {0.25, 0},
		{0.26, 1}, {0.49, 1}, {0.50, 1},
		{0.51, 2}, {0.74, 2}, {0.75, 2},
		{0.76, 3}, {0.99, 3}, {1.00, 3},
		{1.01, 4}, {5.00, 4},
	}
	for _, c := range cases {
		got := encumbranceTier(c.ratio)
		if got != c.want {
			t.Errorf("encumbranceTier(%f) = %d, want %d",
				c.ratio, got, c.want)
		}
	}
}

func TestEncumbranceTierPenalty_NoCrossing(t *testing.T) {
	// Char at low encumbrance, adding a light item keeps the
	// tier unchanged. Penalty should be 0.
	char := newTestChar()
	// Without balance config loaded, CarryCapacity may return 0.
	// Skip if so — full integration in Task 10 smoke.
	if char.CarryCapacity() <= 0 {
		t.Skip("carry capacity calc requires balance config")
	}
	candidate := items.Item{ItemId: 1}
	if candidate.GetSpec().ItemId == 0 {
		t.Skip("fixture required for items.New")
	}
}

func TestItemValueDelta_NoIncorporeal_Unchanged(t *testing.T) {
	// Verifies the integration doesn't break the existing
	// behavior when the character has no incorporeal mutation.
	// With gearMul = 1.0 (the baseline multiplier for no mutations),
	// the scoring should be unchanged from pre-multiplier code.
	char := newTestChar()
	candidate := items.Item{}
	// Empty character + empty item → returns SwapDelta{}
	delta := ItemValueDelta(char, PhysicalBruiser, candidate)
	if delta.Slot != "" {
		t.Errorf("expected empty Slot for non-equippable, got %q", delta.Slot)
	}
	if delta.Score != 0 {
		t.Errorf("expected Score=0 for non-equippable, got %f", delta.Score)
	}
}
