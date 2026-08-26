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

const (
	sidearmId      = 10049
	sidearmPreU10d = 6.00
	sidearmU10d    = 2.20
)

func seedSidearmTemplate(t *testing.T) {
	t.Helper()
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		sidearmId: {
			ItemId:           sidearmId,
			Name:             "Relic Sidearm",
			Type:             Weapon,
			Subtype:          Shooting,
			DamageMultiplier: sidearmU10d,
		},
	}))
}

// TestMigrateDetunedBow_SpareAHighAffixInstanceSidearm is the regression test
// for the value guard's live false positive.
//
// Item 10049 drops from the Core Guardian in Crash Site Interior, an
// `instanced: true` zone, so it can be affix-scaled. The affix budget is
// floor(LootBudgetScalar * sqrt(goldPaid)) and goldPaid has NO upper bound, so
// a post-detune sidearm can be lifted past its OLD template value of 6.00 by 76
// ranks of damage_mult_phys -- roughly a 10% outcome at 40k gold and 49% at
// 100k. On a value-only guard the sweep then multiplies a legitimately earned,
// gold-bought item by 0.367: a 63% loss, exactly the bug SpecBaseline exists to
// prevent.
//
// The item's own DetuneMigrated stamp settles this: identity is recorded, not
// inferred from a value that affixes can move.
func TestMigrateDetunedBow_SparesAHighAffixInstanceSidearm(t *testing.T) {
	seedSidearmTemplate(t)

	// Minted post-detune (New stamps it), then affixed well past the old 6.00.
	itm := New(sidearmId)
	if !itm.DetuneMigrated {
		t.Fatal("New() did not stamp a detuned-bow id as already migrated")
	}
	itm.Affixed = true
	itm.Spec = &ItemSpec{DamageMultiplier: 6.50}

	if MigrateDetunedBow(&itm) {
		t.Error("rescaled a post-detune instance that affixes had lifted above the " +
			"old template value")
	}
	if got := itm.Spec.DamageMultiplier; !nearly(got, 6.50) {
		t.Errorf("high-affix sidearm = %.4f, want 6.5000 untouched. The player paid "+
			"gold for that scaling; rescaling costs them 63%% of the item.", got)
	}
}

// TestMigrateDetunedBow_StillMigratesAnUnstampedLegacySidearm is the other half:
// the `>= old` fallback must keep working for saves written before the stamp
// existed, where no post-detune item can exist.
func TestMigrateDetunedBow_StillMigratesAnUnstampedLegacySidearm(t *testing.T) {
	seedSidearmTemplate(t)

	itm := &Item{ItemId: sidearmId, Affixed: true, Spec: &ItemSpec{DamageMultiplier: 6.50}}

	if !MigrateDetunedBow(itm) {
		t.Fatal("legacy unstamped sidearm was not migrated")
	}
	want := 6.50 * (sidearmU10d / sidearmPreU10d)
	if got := itm.Spec.DamageMultiplier; !nearly(got, want) {
		t.Errorf("legacy sidearm = %.4f, want %.4f", got, want)
	}
	if !itm.DetuneMigrated {
		t.Error("migration did not stamp the item it rescaled, so a later pass has " +
			"only the value fallback to protect it")
	}
}

// TestMigrateDetunedBow_StampsEvenWhenItSkips pins that a bow the fallback
// declines to touch is still stamped, so the fallback is consulted exactly once
// per item across the item's whole lifetime.
func TestMigrateDetunedBow_StampsEvenWhenItSkips(t *testing.T) {
	seedWarbowTemplate(t)

	itm := &Item{ItemId: warbowId, Spec: &ItemSpec{DamageMultiplier: 4.00}}
	if MigrateDetunedBow(itm) {
		t.Error("rescaled an instance already below its pre-detune template")
	}
	if !itm.DetuneMigrated {
		t.Error("a skipped bow was left unstamped")
	}
	if got := itm.Spec.DamageMultiplier; !nearly(got, 4.00) {
		t.Errorf("admin-set value = %.4f, want 4.0000 preserved", got)
	}
}

// TestNew_StampsOnlyDetunedBowIds keeps the stamp out of every other item's
// save footprint.
func TestNew_StampsOnlyDetunedBowIds(t *testing.T) {
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		warbowId: {ItemId: warbowId, Name: "Ironhorn Warbow", Type: Weapon, Subtype: Shooting, DamageMultiplier: warbowU10d},
		10026:    {ItemId: 10026, Name: "Bandit's Longsword", Type: Weapon, DamageMultiplier: 1.10},
	}))

	if bow := New(warbowId); !bow.DetuneMigrated {
		t.Error("a newly minted bow was not stamped, so the first migration would " +
			"judge it on value alone")
	}
	if sword := New(10026); sword.DetuneMigrated {
		t.Error("a non-bow was stamped; the flag would then bloat every item in " +
			"every save")
	}
}

// TestMigrateDetunedBow_JudgesSpecAndBaselineTogether pins that the two fields
// are treated as one quantity.
//
// ApplyTier sets spec = baseline + tierBonus, and damage_multiplier_bonus
// reaches +0.30, DOUBLED on a two-hander. So a baseline just under the
// threshold can carry a spec just over it. Guarding them independently would
// cut the spec to ~0.367x while the baseline kept the old value, leaving the
// item wrong until the next tier-up snapped it back.
func TestMigrateDetunedBow_JudgesSpecAndBaselineTogether(t *testing.T) {
	seedWarbowTemplate(t)

	// Baseline below the threshold, spec above it via a doubled 2H tier bonus.
	itm := &Item{
		ItemId:          warbowId,
		EnchantType:     "keen",
		EnchantTier:     3,
		Spec:            &ItemSpec{DamageMultiplier: 7.60},
		EnchantBaseline: &SpecBaseline{DamageMultiplier: 7.00},
	}

	MigrateDetunedBow(itm)

	spec, baseline := itm.Spec.DamageMultiplier, itm.EnchantBaseline.DamageMultiplier
	if nearly(baseline, 7.00) && !nearly(spec, 7.60) {
		t.Errorf("spec was rescaled to %.4f while the baseline kept %.4f -- the two "+
			"fields desynced; the next tier-up would snap the item back", spec, baseline)
	}
	if !nearly(spec, 7.60) && !nearly(baseline, 7.00) {
		// Both moved: they must have moved by the same ratio.
		if !nearly(spec/7.60, baseline/7.00) {
			t.Errorf("spec and baseline rescaled by different ratios: %.6f vs %.6f",
				spec/7.60, baseline/7.00)
		}
	}
}
