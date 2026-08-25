package users

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
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
// an alt to u.Character, so the next LoadUser sees a character with no bow
// migration marker. If the bank were guarded by that character's MiscData, the
// sweep would run a SECOND time and multiply every banked bow by the detune
// ratio again -- silently, permanently, and with no message to the player.
//
// The bank's marker must therefore live on Storage itself.
func TestStorageMigrateDetunedRangedWeapons_SurvivesAnAltSwap(t *testing.T) {
	seedWarbowTemplateForStorage(t)

	s := &Storage{Slots: []StorageSlot{{Item: bankedWarbow(), Count: 1}}}

	// First login, on the original character.
	s.MigrateDetunedRangedWeapons()
	afterFirst := s.Slots[0].Item.Spec.DamageMultiplier
	if !nearlyEq(afterFirst, testWarbowU10d) {
		t.Fatalf("first pass = %.4f, want %.4f", afterFirst, testWarbowU10d)
	}

	// SwapToAlt replaces u.Character wholesale; the alt has never carried a bow
	// migration marker. The bank, however, is the same object. Next login:
	if s.MigrateDetunedRangedWeapons() {
		t.Error("bank migration re-ran after an alt swap")
	}

	afterSecond := s.Slots[0].Item.Spec.DamageMultiplier
	if !nearlyEq(afterSecond, afterFirst) {
		t.Errorf("banked bow re-detuned across an alt swap: %.4f -> %.4f. The bank "+
			"guard is character-scoped when it must be account-scoped; a player "+
			"with alts loses damage on every bow in the bank, once per alt.",
			afterFirst, afterSecond)
	}
}

// TestStorageMigrationMarkerPersistsThroughTheYamlField guards the marker's
// serialisation: an in-memory-only marker would let the migration re-run on
// every single login, not just after an alt swap.
func TestStorageMigrationMarkerPersistsThroughTheYamlField(t *testing.T) {
	s := &Storage{}
	s.MarkMigrationApplied("u10d-bow-detune")

	if !s.MigrationApplied("u10d-bow-detune") {
		t.Fatal("marker did not stick")
	}
	if s.MigrationsDone == nil || !s.MigrationsDone["u10d-bow-detune"] {
		t.Error("marker is not in the yaml-tagged MigrationsDone map, so it will " +
			"not survive SaveUser")
	}
	if s.MigrationApplied("some-other-migration") {
		t.Error("unrelated migration key reported as applied")
	}
}

func nearlyEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
