package items

import (
	"math"
	"testing"
)

const (
	warbowId      = 10046
	warbowPreU10d = 7.50
	warbowU10d    = 2.75
)

// seedWarbowTemplate installs a minimal item table holding the POST-U10d
// Ironhorn Warbow template and restores the real one afterwards.
func seedWarbowTemplate(t *testing.T) {
	t.Helper()
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		warbowId: {
			ItemId:           warbowId,
			Name:             "Ironhorn Warbow",
			Type:             Weapon,
			Subtype:          Shooting,
			DamageMultiplier: warbowU10d,
		},
	}))
}

func nearly(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// Test 1: an ENCHANTED bow carrying the pre-detune value in both its instance
// override and its enchant baseline lands on the new line in both places.
//
// The baseline matters as much as the spec: enchantments.ApplyTier does an
// unconditional EnchantBaseline.RestoreInto(&newSpec), so a stale baseline
// re-installs the old multiplier on the very next enchant pass.
func TestMigrateDetunedBow_EnchantedBowLandsOnTheNewLine(t *testing.T) {
	seedWarbowTemplate(t)

	itm := &Item{
		ItemId:          warbowId,
		EnchantType:     "keen",
		EnchantTier:     1,
		Spec:            &ItemSpec{DamageMultiplier: warbowPreU10d},
		EnchantBaseline: &SpecBaseline{DamageMultiplier: warbowPreU10d},
	}

	if !MigrateDetunedBow(itm) {
		t.Fatal("migration reported no change")
	}
	if got := itm.Spec.DamageMultiplier; !nearly(got, warbowU10d) {
		t.Errorf("spec = %.4f, want %.4f", got, warbowU10d)
	}
	if got := itm.EnchantBaseline.DamageMultiplier; !nearly(got, warbowU10d) {
		t.Errorf("enchant baseline = %.4f, want %.4f -- the unconditional "+
			"RestoreInto in ApplyTier will re-install this over the fix", got, warbowU10d)
	}
}

// Test 2: an AFFIXED bow keeps the scaling its owner paid gold for.
//
// Regression test for the bug documented in SpecBaseline: resetting an instance
// to the bare template "silently destroyed everything an instance had earned
// above it", observed on prod as about a 16% damage drop on a set of affixed
// claws. applyBonus writes +0.05 per rank, so 7.85 is a warbow carrying 0.35 of
// paid affix budget. Ids 10046 and 10049 exist only in loot pools, so this is
// the population the migration targets.
//
// An assignment-based migration (spec.DamageMultiplier = tmpl.DamageMultiplier)
// flattens this to 2.75 and FAILS here.
func TestMigrateDetunedBow_AffixedBowKeepsItsPaidScaling(t *testing.T) {
	seedWarbowTemplate(t)

	const affixed = 7.85 // template 7.50 + 0.35 of paid affix scaling

	itm := &Item{
		ItemId:  warbowId,
		Affixed: true,
		Spec:    &ItemSpec{DamageMultiplier: affixed},
	}
	MigrateDetunedBow(itm)
	got := itm.Spec.DamageMultiplier

	want := affixed * (warbowU10d / warbowPreU10d) // 2.878333...
	if !nearly(got, want) {
		t.Errorf("affixed bow = %.4f, want %.4f", got, want)
	}
	if math.Abs(got-2.88) > 0.005 {
		t.Errorf("affixed bow = %.4f, expected it to round to 2.88", got)
	}
	if nearly(got, warbowU10d) {
		t.Errorf("affixed bow was flattened to the bare template %.2f -- the paid "+
			"affix scaling was deleted with no message and no refund. Rescale by "+
			"the ratio; never assign the template value.", warbowU10d)
	}

	// The earned delta must survive IN PROPORTION, not as an absolute.
	oldDelta := (affixed - warbowPreU10d) / warbowPreU10d
	newDelta := (got - warbowU10d) / warbowU10d
	if !nearly(oldDelta, newDelta) {
		t.Errorf("earned delta drifted: was %.6f above template, now %.6f", oldDelta, newDelta)
	}
}

// Test 3: the rescale is a MULTIPLICATION, so re-running it must be a no-op.
// This is the property the whole design rests on -- there is deliberately no
// run-once marker anywhere in this migration.
func TestMigrateDetunedBow_IsIdempotent(t *testing.T) {
	seedWarbowTemplate(t)

	for _, tc := range []struct {
		name string
		spec float64
	}{
		{"plain pre-detune bow", warbowPreU10d},
		{"affixed pre-detune bow", 7.85},
	} {
		t.Run(tc.name, func(t *testing.T) {
			itm := &Item{ItemId: warbowId, Spec: &ItemSpec{DamageMultiplier: tc.spec}}

			MigrateDetunedBow(itm)
			afterFirst := itm.Spec.DamageMultiplier

			if MigrateDetunedBow(itm) {
				t.Error("second pass reported a change; it must be a no-op")
			}
			if got := itm.Spec.DamageMultiplier; !nearly(got, afterFirst) {
				t.Errorf("second pass changed the value: %.4f -> %.4f. Every pass "+
					"multiplies by the ratio, so without the >= threshold a bow is "+
					"detuned again on every single load.", afterFirst, got)
			}
		})
	}
}

// Test 4: an item that has NEVER been migrated but already carries the
// post-detune value must be left alone.
//
// This is the case no run-once marker can cover, and it hits the two most
// common situations in the game. characters.New() (alt creation) and CreateUser
// both produce empty MiscData, and a brand-new account plays its ENTIRE first
// session on an in-memory record that never passes through LoadUser. Anything
// that materialises Item.Spec -- an enchant, an affix roll, a rename, a worn
// buff -- pins the bow at the new 2.75. The first migration afterwards would
// then rescale an already-correct item down to about 1.01.
func TestMigrateDetunedBow_LeavesAPostDetuneItemAlone(t *testing.T) {
	seedWarbowTemplate(t)

	// As produced by enchanting a bow acquired after the detune shipped.
	itm := &Item{
		ItemId:          warbowId,
		EnchantType:     "keen",
		EnchantTier:     1,
		Spec:            &ItemSpec{DamageMultiplier: warbowU10d},
		EnchantBaseline: &SpecBaseline{DamageMultiplier: warbowU10d},
	}

	if MigrateDetunedBow(itm) {
		t.Error("migration touched an already-post-detune item")
	}
	if got := itm.Spec.DamageMultiplier; !nearly(got, warbowU10d) {
		t.Errorf("post-detune bow spec = %.4f, want %.4f left untouched. A character "+
			"created after the detune carries no marker, so a marker-guarded "+
			"migration corrupts its correct items on first load.", got, warbowU10d)
	}
	if got := itm.EnchantBaseline.DamageMultiplier; !nearly(got, warbowU10d) {
		t.Errorf("post-detune bow baseline = %.4f, want %.4f left untouched", got, warbowU10d)
	}
}

// TestMigrateDetunedBow_SkipsAnAdminLoweredInstance pins the deliberate
// fail-safe edge: an instance an admin set BELOW its old template value is
// indistinguishable from an already-migrated one, so it keeps the value the
// admin chose rather than being rescaled.
func TestMigrateDetunedBow_SkipsAnAdminLoweredInstance(t *testing.T) {
	seedWarbowTemplate(t)

	itm := &Item{ItemId: warbowId, Spec: &ItemSpec{DamageMultiplier: 4.00}}
	if MigrateDetunedBow(itm) {
		t.Error("rescaled an instance already below its pre-detune template")
	}
	if got := itm.Spec.DamageMultiplier; !nearly(got, 4.00) {
		t.Errorf("admin-set value = %.4f, want 4.0000 preserved", got)
	}
}

// TestMigrateDetunedRangedWeapons_CountsAndIgnoresNonBows checks the set
// wrapper: nil entries and unrelated items are skipped, not counted.
func TestMigrateDetunedRangedWeapons_CountsAndIgnoresNonBows(t *testing.T) {
	seedWarbowTemplate(t)

	bow := &Item{ItemId: warbowId, Spec: &ItemSpec{DamageMultiplier: warbowPreU10d}}
	sword := &Item{ItemId: 10026, Spec: &ItemSpec{DamageMultiplier: 1.10}}

	if got := MigrateDetunedRangedWeapons([]*Item{nil, bow, sword, nil}); got != 1 {
		t.Errorf("updated count = %d, want 1", got)
	}
	if !nearly(sword.Spec.DamageMultiplier, 1.10) {
		t.Errorf("non-bow was modified: %.4f", sword.Spec.DamageMultiplier)
	}
}

// TestPreDetuneBowTable_MatchesTheRealTemplates keeps the migration table and
// the shipped YAML from drifting apart. Every id the migration knows about must
// still resolve to a Shooting weapon whose template came DOWN, and the whole
// Shooting set must be covered -- a bow added later without a table entry would
// migrate as a silent no-op.
func TestPreDetuneBowTable_MatchesTheRealTemplates(t *testing.T) {
	loadRealItemSpecsForTest(t)

	for id, old := range preDetuneBowMultipliers {
		spec := GetItemSpec(id)
		if spec == nil {
			t.Errorf("preDetuneBowMultipliers has id %d but no such item template", id)
			continue
		}
		if spec.Subtype != Shooting {
			t.Errorf("item %d (%s) is in the pre-detune table but is subtype %q, not shooting",
				id, spec.Name, spec.Subtype)
		}
		if spec.DamageMultiplier >= old {
			t.Errorf("item %d (%s) template multiplier %.2f is not below its pre-detune "+
				"value %.2f -- the detune was reverted, or the table is stale",
				id, spec.Name, spec.DamageMultiplier, old)
		}
	}

	for _, spec := range GetAllItemSpecs() {
		if spec.Subtype != Shooting {
			continue
		}
		if _, ok := preDetuneBowMultipliers[spec.ItemId]; !ok {
			t.Errorf("shooting weapon %q (id %d) has no preDetuneBowMultipliers entry, "+
				"so existing instances of it would never be migrated", spec.Name, spec.ItemId)
		}
	}
}
