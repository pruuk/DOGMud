package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/pets"
)

const (
	testWarbowId      = 10046
	testWarbowPreU10d = 7.50
	testWarbowU10d    = 2.75
)

func seedWarbowTemplateForCharacter(t *testing.T) {
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

func preDetuneBow() items.Item {
	return items.Item{
		ItemId: testWarbowId,
		Spec:   &items.ItemSpec{DamageMultiplier: testWarbowPreU10d},
	}
}

func nearlyEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestMigrateDetunedRangedWeapons_ReachesEveryCarriedCollection covers the
// populations the sweep claims. Pet inventory is the one most easily missed:
// Pet.StoreItem accepts any item with ItemId >= 1 with NO type filter, and both
// get.go and give.go route items into it, so a pack pet is first-class player
// storage that can hold a bow indefinitely.
func TestMigrateDetunedRangedWeapons_ReachesEveryCarriedCollection(t *testing.T) {
	seedWarbowTemplateForCharacter(t)

	c := &Character{
		Name:           "TestArcher",
		Items:          []items.Item{preDetuneBow()},
		ComponentItems: []items.Item{preDetuneBow()},
		PotionItems:    []items.Item{preDetuneBow()},
		Pet:            pets.Pet{Type: "packmule", Capacity: 4, Items: []items.Item{preDetuneBow()}},
	}
	weapon := preDetuneBow()
	c.Equipment.Weapon = weapon

	c.MigrateDetunedRangedWeapons()

	for _, tc := range []struct {
		where string
		got   float64
	}{
		{"backpack", c.Items[0].Spec.DamageMultiplier},
		{"component bag", c.ComponentItems[0].Spec.DamageMultiplier},
		{"potion bandolier", c.PotionItems[0].Spec.DamageMultiplier},
		{"pet inventory", c.Pet.Items[0].Spec.DamageMultiplier},
		{"equipped weapon", c.Equipment.Weapon.Spec.DamageMultiplier},
	} {
		if !nearlyEq(tc.got, testWarbowU10d) {
			t.Errorf("%s bow = %.4f, want %.4f -- this collection is not in the sweep",
				tc.where, tc.got, testWarbowU10d)
		}
	}
}

// TestMigrateDetunedRangedWeapons_SurvivesACharacterWithNoMarker is the
// regression test for the create-path corruption.
//
// characters.New() seeds an EMPTY MiscData map, so a freshly created alt -- and
// a brand-new account, which plays its whole first session on an in-memory
// record that never passes through LoadUser -- reaches its first migration with
// no marker of any kind. If it acquired a post-detune bow in the meantime
// (enchanting, an affix roll, a rename and a worn buff all materialise
// Item.Spec), a marker-guarded migration would rescale a CORRECT item to about
// 1.01, silently and permanently.
//
// The migration is therefore value-guarded, not marker-guarded.
func TestMigrateDetunedRangedWeapons_SurvivesACharacterWithNoMarker(t *testing.T) {
	seedWarbowTemplateForCharacter(t)

	c := New() // the real alt-creation path
	c.Name = "FreshAlt"

	if c.GetMiscData("migration-u10d-bow-detune-done") != nil {
		t.Fatal("premise broken: a new character already carries the bow marker")
	}

	// A bow acquired and enchanted entirely after the detune shipped.
	c.Items = []items.Item{{
		ItemId:          testWarbowId,
		EnchantType:     "keen",
		EnchantTier:     1,
		Spec:            &items.ItemSpec{DamageMultiplier: testWarbowU10d},
		EnchantBaseline: &items.SpecBaseline{DamageMultiplier: testWarbowU10d},
	}}

	c.MigrateDetunedRangedWeapons()

	if got := c.Items[0].Spec.DamageMultiplier; !nearlyEq(got, testWarbowU10d) {
		t.Errorf("post-detune bow on a never-migrated character = %.4f, want %.4f",
			got, testWarbowU10d)
	}
	if got := c.Items[0].EnchantBaseline.DamageMultiplier; !nearlyEq(got, testWarbowU10d) {
		t.Errorf("post-detune baseline on a never-migrated character = %.4f, want %.4f",
			got, testWarbowU10d)
	}
}

// TestMigrateDetunedRangedWeapons_RunsEveryLoadWithoutCompounding pins the
// absence of a run-once marker. The sweep runs on every load on purpose, so a
// pre-detune bow reaching the player LATER (looted from a mob instance, bought
// from stale shop stock, pulled from a corpse) still migrates rather than being
// frozen at its old value by a marker set on some earlier login.
func TestMigrateDetunedRangedWeapons_RunsEveryLoadWithoutCompounding(t *testing.T) {
	seedWarbowTemplateForCharacter(t)

	c := &Character{Name: "TestArcher", Items: []items.Item{preDetuneBow()}}

	c.MigrateDetunedRangedWeapons()
	afterFirst := c.Items[0].Spec.DamageMultiplier
	if !nearlyEq(afterFirst, testWarbowU10d) {
		t.Fatalf("first load = %.4f, want %.4f", afterFirst, testWarbowU10d)
	}

	// Second login. Also: a pre-detune bow looted in the meantime.
	c.Items = append(c.Items, preDetuneBow())
	c.MigrateDetunedRangedWeapons()

	if got := c.Items[0].Spec.DamageMultiplier; !nearlyEq(got, afterFirst) {
		t.Errorf("already-migrated bow compounded across loads: %.4f -> %.4f",
			afterFirst, got)
	}
	if got := c.Items[1].Spec.DamageMultiplier; !nearlyEq(got, testWarbowU10d) {
		t.Errorf("later-acquired pre-detune bow = %.4f, want %.4f. A run-once marker "+
			"would strand this at its old value forever.", got, testWarbowU10d)
	}
}
