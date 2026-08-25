package characters

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

const (
	warbowId      = 10046
	warbowPreU10d = 7.50
	warbowU10d    = 2.75
)

// seedWarbowTemplate installs a minimal item table holding the post-U10d
// Ironhorn Warbow template, and restores the real one afterwards.
func seedWarbowTemplate(t *testing.T) {
	t.Helper()
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		warbowId: {
			ItemId:           warbowId,
			Name:             "Ironhorn Warbow",
			Type:             items.Weapon,
			Subtype:          items.Shooting,
			DamageMultiplier: warbowU10d,
		},
	})
	t.Cleanup(cleanup)
}

// Test 1: an ENCHANTED bow carrying the pre-detune value in both its instance
// override and its enchant baseline lands on the new line in both places.
//
// The baseline matters as much as the spec: enchantments.ApplyTier does an
// unconditional EnchantBaseline.RestoreInto(&newSpec), so a stale baseline
// re-installs the old multiplier on the very next enchant pass.
func TestMigrateDetunedRangedWeapons_EnchantedBowLandsOnTheNewLine(t *testing.T) {
	seedWarbowTemplate(t)

	c := &Character{
		Name: "TestArcher",
		Items: []items.Item{{
			ItemId:          warbowId,
			EnchantType:     "keen",
			EnchantTier:     1,
			Spec:            &items.ItemSpec{DamageMultiplier: warbowPreU10d},
			EnchantBaseline: &items.SpecBaseline{DamageMultiplier: warbowPreU10d},
		}},
	}

	// The bank is swept by users.Storage.MigrateDetunedRangedWeapons under its
	// own account-scoped marker; here we exercise the shared pure sweep.
	banked := &items.Item{
		ItemId: warbowId,
		Spec:   &items.ItemSpec{DamageMultiplier: warbowPreU10d},
	}

	c.MigrateDetunedRangedWeapons()
	MigrateDetunedRangedWeaponItems([]*items.Item{banked})

	if got := c.Items[0].Spec.DamageMultiplier; !nearly(got, warbowU10d) {
		t.Errorf("carried bow spec = %.4f, want %.4f", got, warbowU10d)
	}
	if got := c.Items[0].EnchantBaseline.DamageMultiplier; !nearly(got, warbowU10d) {
		t.Errorf("carried bow enchant baseline = %.4f, want %.4f", got, warbowU10d)
	}
	if got := banked.Spec.DamageMultiplier; !nearly(got, warbowU10d) {
		t.Errorf("banked bow spec = %.4f, want %.4f", got, warbowU10d)
	}
}

// Test 2: an AFFIXED bow keeps the scaling its owner paid gold for.
//
// This is the regression test for the bug documented in items.SpecBaseline:
// resetting an instance to the bare template "silently destroyed everything an
// instance had earned above it", observed on prod as about a 16% damage drop
// on a set of affixed claws. affixgen.applyBonus writes +0.05 per rank, so
// 7.85 is a warbow carrying 0.35 of paid affix budget. Ids 10046 and 10049
// exist only in loot pools, so this is the population the migration targets.
//
// An assignment-based migration (spec.DamageMultiplier = tmpl.DamageMultiplier)
// flattens this to 2.75 and FAILS here.
func TestMigrateDetunedRangedWeapons_AffixedBowKeepsItsPaidScaling(t *testing.T) {
	seedWarbowTemplate(t)

	const affixed = 7.85 // template 7.50 + 0.35 of paid affix scaling

	c := &Character{
		Name: "TestArcher",
		Items: []items.Item{{
			ItemId:  warbowId,
			Affixed: true,
			Spec:    &items.ItemSpec{DamageMultiplier: affixed},
		}},
	}

	c.MigrateDetunedRangedWeapons()

	got := c.Items[0].Spec.DamageMultiplier

	// Proportional rescale: 7.85 * (2.75 / 7.50) = 2.878333..., i.e. ~2.88.
	want := affixed * (warbowU10d / warbowPreU10d)
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

// Test 3: the migration is NOT idempotent -- it multiplies by a ratio -- so it
// must be guarded. Running it twice must leave the value untouched.
//
// Keying on ItemId does NOT make this safe; only the run-once marker does.
func TestMigrateDetunedRangedWeapons_IsGuardedAgainstReRunning(t *testing.T) {
	seedWarbowTemplate(t)

	c := &Character{
		Name: "TestArcher",
		Items: []items.Item{{
			ItemId: warbowId,
			Spec:   &items.ItemSpec{DamageMultiplier: warbowPreU10d},
		}},
	}

	c.MigrateDetunedRangedWeapons()
	afterFirst := c.Items[0].Spec.DamageMultiplier
	if !nearly(afterFirst, warbowU10d) {
		t.Fatalf("first pass = %.4f, want %.4f", afterFirst, warbowU10d)
	}

	c.MigrateDetunedRangedWeapons()
	afterSecond := c.Items[0].Spec.DamageMultiplier

	if !nearly(afterSecond, afterFirst) {
		t.Errorf("second pass changed the value: %.4f -> %.4f. The run-once guard "+
			"is missing or broken; a re-detuned bow loses %.0f%% more damage on "+
			"every load.", afterFirst, afterSecond,
			(1-warbowU10d/warbowPreU10d)*100)
	}
}

// TestPreDetuneBowTable_MatchesTheRealTemplates keeps the migration table and
// the shipped YAML from drifting apart. Every id the migration knows about must
// still resolve to a Shooting weapon, and the whole Shooting set must be
// covered -- a bow added later without a table entry would migrate as a no-op.
func TestPreDetuneBowTable_MatchesTheRealTemplates(t *testing.T) {
	// SeedItemsForTest captures the real table; the cleanup restores it even
	// though LoadDataFiles reassigns the package var underneath us.
	t.Cleanup(items.SeedItemsForTest(nil))

	// Data path anchored on runtime.Caller, NOT the working directory: all
	// tests in a package share one binary and internal/actions' economy_test.go
	// chdirs to the repo root, so relative paths pass or fail by test ORDER.
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repoRoot, "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	items.LoadDataFiles()

	for id, old := range preDetuneBowMultipliers {
		spec := items.GetItemSpec(id)
		if spec == nil {
			t.Errorf("preDetuneBowMultipliers has id %d but no such item template", id)
			continue
		}
		if spec.Subtype != items.Shooting {
			t.Errorf("item %d (%s) is in the pre-detune table but is subtype %q, not shooting",
				id, spec.Name, spec.Subtype)
		}
		if spec.DamageMultiplier >= old {
			t.Errorf("item %d (%s) template multiplier %.2f is not below its pre-detune "+
				"value %.2f -- the detune was reverted, or the table is stale",
				id, spec.Name, spec.DamageMultiplier, old)
		}
	}

	for _, spec := range items.GetAllItemSpecs() {
		if spec.Subtype != items.Shooting {
			continue
		}
		if _, ok := preDetuneBowMultipliers[spec.ItemId]; !ok {
			t.Errorf("shooting weapon %q (id %d) has no preDetuneBowMultipliers entry, "+
				"so existing instances of it would never be migrated", spec.Name, spec.ItemId)
		}
	}
}

func nearly(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
