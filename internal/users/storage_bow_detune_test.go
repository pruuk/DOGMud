package users

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"gopkg.in/yaml.v2"
)

const (
	testWarbowId      = 10046
	testWarbowPreU10d = 7.50
	testWarbowU10d    = 2.75
)

func seedWarbowTemplateForStorage(t *testing.T) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		testWarbowId: {
			ItemId:           testWarbowId,
			Name:             "Ironhorn Warbow",
			Type:             items.Weapon,
			Subtype:          items.Shooting,
			DamageMultiplier: testWarbowU10d,
		},
	}))
}

func bankedWarbow() items.Item {
	return items.Item{
		ItemId: testWarbowId,
		Spec:   &items.ItemSpec{DamageMultiplier: testWarbowPreU10d},
	}
}

func nearlyEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestStorageMigrateDetunedRangedWeapons_ReachesBothSlotsAndLegacyItems checks
// the sweep covers the canonical Slots list AND the legacy Items list, since
// MigrateStorageSlots only folds Items into Slots later in the load sequence.
func TestStorageMigrateDetunedRangedWeapons_ReachesBothSlotsAndLegacyItems(t *testing.T) {
	seedWarbowTemplateForStorage(t)

	s := &Storage{
		Slots: []StorageSlot{{Item: bankedWarbow(), Count: 1}},
		Items: []items.Item{bankedWarbow()},
	}

	if !s.MigrateDetunedRangedWeapons() {
		t.Fatal("migration reported no change")
	}

	if got := s.Slots[0].Item.Spec.DamageMultiplier; !nearlyEq(got, testWarbowU10d) {
		t.Errorf("slot bow = %.4f, want %.4f", got, testWarbowU10d)
	}
	if got := s.Items[0].Spec.DamageMultiplier; !nearlyEq(got, testWarbowU10d) {
		t.Errorf("legacy-list bow = %.4f, want %.4f -- AllItemPtrs is missing "+
			"Storage.Items, which MigrateStorageSlots has not folded away yet",
			got, testWarbowU10d)
	}
}

// TestStorageMigrateDetunedRangedWeapons_SurvivesAnAltSwap is the regression
// test for the account/character scoping hazard.
//
// Alt characters live in <userId>.alts.yaml and each carry their OWN MiscData,
// but the entire account shares ONE ItemStorage. UserRecord.SwapToAlt promotes
// an alt to u.Character, so the next LoadUser sees a character that has never
// run this migration. The bank must neither be rescaled twice by that second
// pass, nor be permanently skipped for bows deposited after some earlier login.
func TestStorageMigrateDetunedRangedWeapons_SurvivesAnAltSwap(t *testing.T) {
	seedWarbowTemplateForStorage(t)

	s := &Storage{Slots: []StorageSlot{{Item: bankedWarbow(), Count: 1}}}

	// First login, on the original character.
	s.MigrateDetunedRangedWeapons()
	afterFirst := s.Slots[0].Item.Spec.DamageMultiplier
	if !nearlyEq(afterFirst, testWarbowU10d) {
		t.Fatalf("first pass = %.4f, want %.4f", afterFirst, testWarbowU10d)
	}

	// SwapToAlt replaces u.Character wholesale, and that alt deposits a bow it
	// still carries at the pre-detune value. Next login sweeps the same bank.
	s.AddItem(bankedWarbow())
	s.MigrateDetunedRangedWeapons()

	if got := s.Slots[0].Item.Spec.DamageMultiplier; !nearlyEq(got, afterFirst) {
		t.Errorf("banked bow re-detuned across an alt swap: %.4f -> %.4f. Every pass "+
			"multiplies by the ratio; a player with alts would lose damage on every "+
			"bow in the bank, once per alt.", afterFirst, got)
	}

	var found bool
	for _, slot := range s.Slots {
		if got := slot.Item.Spec.DamageMultiplier; nearlyEq(got, testWarbowU10d) {
			found = true
		} else if !nearlyEq(got, afterFirst) {
			t.Errorf("banked bow at an unexpected multiplier %.4f", got)
		}
	}
	if !found {
		t.Error("the bow deposited by the un-migrated alt was never rescaled -- a " +
			"run-once bank marker strands it at 7.50 forever")
	}
}

// TestStorageBowMigrationSurvivesAYamlRoundTrip drives the rescaled bank
// through the real serialisation path, since a migration that does not persist
// is a migration that silently re-runs forever.
func TestStorageBowMigrationSurvivesAYamlRoundTrip(t *testing.T) {
	seedWarbowTemplateForStorage(t)

	u := &UserRecord{
		UserId:      1,
		Username:    "archer",
		Character:   characters.New(),
		ItemStorage: Storage{Slots: []StorageSlot{{Item: bankedWarbow(), Count: 1}}},
	}
	u.ItemStorage.MigrateDetunedRangedWeapons()

	blob, err := yaml.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back UserRecord
	if err := yaml.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(back.ItemStorage.Slots) != 1 {
		t.Fatalf("slots after round trip = %d, want 1", len(back.ItemStorage.Slots))
	}
	spec := back.ItemStorage.Slots[0].Item.Spec
	if spec == nil {
		t.Fatal("the `overrides:` block did not survive the round trip, so the " +
			"rescaled multiplier was not persisted at all")
	}
	if !nearlyEq(spec.DamageMultiplier, testWarbowU10d) {
		t.Errorf("persisted multiplier = %.4f, want %.4f", spec.DamageMultiplier, testWarbowU10d)
	}
}
