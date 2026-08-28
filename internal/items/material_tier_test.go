package items

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMaterialTierMultiplierBand pins the five authored buckets onto the
// 0.95 to 1.05 band, tier 3 neutral.
//
// 🔴 NARROWED FROM 0.75-1.25 (owner, 2026-08-28). The tier term is
// SCALE-INVARIANT — it moves every recipe by a fixed z regardless of skill or
// stat — so at the old width it was a bigger lever than nine levels of mastery,
// and on a skill_minimum 65 recipe it pushed the 50/50 point out to 86, past
// the skill soft cap of 50. At this band the shift is about a level, which is
// what a MODIFIER on a difficulty SkillMinimum already carries should be.
func TestMaterialTierMultiplierBand(t *testing.T) {
	tests := []struct {
		tier int
		want float64
	}{
		{1, 0.95},
		{2, 0.975},
		{3, 1.0},
		{4, 1.025},
		{5, 1.05},
	}
	for _, tt := range tests {
		if got := MaterialTierMultiplier(tt.tier); got != tt.want {
			t.Errorf("MaterialTierMultiplier(%d) = %v, want %v", tt.tier, got, tt.want)
		}
	}
}

// TestUntieredMaterialIsNeutral pins the ruling that makes the backfill safe to
// land one file at a time: an ABSENT tier is 1.0, NOT the cheapest bucket.
//
// If 0 mapped to 0.75, every un-backfilled material would be quietly making its
// recipes EASIER, and the backfill would look like a difficulty increase rather
// than the difficulty model coming online. Partial coverage has to be inert.
func TestUntieredMaterialIsNeutral(t *testing.T) {
	if got := MaterialTierMultiplier(0); got != 1.0 {
		t.Fatalf("untiered material = %v, want 1.0 (neutral, NOT the cheapest tier)", got)
	}
}

// TestMaterialTierMultiplierClampsOutOfRange pins that a nonsense tier cannot
// produce a nonsense multiplier. The authoring guard rejects new files without
// a tier, but nothing stops a typo writing 7, and the loader is lenient.
func TestMaterialTierMultiplierClampsOutOfRange(t *testing.T) {
	if got := MaterialTierMultiplier(9); got != 1.05 {
		t.Errorf("MaterialTierMultiplier(9) = %v, want 1.05 (clamped to the top bucket)", got)
	}
	if got := MaterialTierMultiplier(-3); got != 1.0 {
		t.Errorf("MaterialTierMultiplier(-3) = %v, want 1.0 (negative is not a tier; treat as untiered)", got)
	}
}

// materialsDirForTest resolves the materials folder from THIS file's location.
//
// Anchored on runtime.Caller, not the working directory: every test in a
// package shares one binary and the CWD is not reliably the package dir, so a
// relative path here passes or fails by test ORDER.
func materialsDirForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the repo root")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	dir := filepath.Join(root, "_datafiles", "world", "dogmud", "items", "materials-40000")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("materials dir not found at %s: %v", dir, err)
	}
	return dir
}

// TestEveryCraftingComponentDeclaresATier requires material_tier on every item
// a recipe can actually name as an ingredient, and it has NO EXEMPTION LIST.
//
// 🎯 THIS IS THE BACKFILL'S COMPLETION CHECK, and it is already green: all 138
// component-tagged materials carry a tier. It ships with no grandfathered set
// at all, so there is nothing left to shrink and no way for coverage to regress
// quietly. U11's obligation is discharged by this test rather than by roadmap
// prose -- U6 was declared done with two criteria false, and nothing failed,
// because they were prose.
//
// SCOPE IS component_tag, NOT the folder. materials-40000/ holds 208 files but
// only 138 are crafting components; the rest are boss gear (40232 is a
// `type: neck` with never_drops), keys and potions, where a craft-difficulty
// tier would mean nothing. A recipe names ingredients by `item_tag`, matched
// against `component_tag`, so an item without one can never BE an ingredient.
//
// The narrower scope has no hole. An item authored with no component_tag needs
// no tier, and the moment a later commit gives it one, this test starts
// requiring the tier. Verified at authoring time that the split is clean: every
// `is_component: true` file also carries a `component_tag`.
func TestEveryCraftingComponentDeclaresATier(t *testing.T) {
	dir := materialsDirForTest(t)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	components := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		body := string(b)
		if !strings.Contains(body, "\ncomponent_tag:") && !strings.HasPrefix(body, "component_tag:") {
			continue // not nameable by a recipe; a tier would do nothing
		}
		components++
		if !strings.Contains(body, "material_tier:") {
			t.Errorf("%s is a crafting component with no material_tier. Declare "+
				"one, 1 (common) to 5 (rarest). It is NOT rarity_tier -- that is "+
				"a vendor stock cap whose scale runs the OTHER WAY.", e.Name())
		}
	}

	// If this ever reads 0 the scope test has silently stopped testing anything,
	// which is the failure mode a coverage guard is least able to notice.
	if components == 0 {
		t.Fatal("found no component-tagged materials at all; the scan is broken, " +
			"not the data")
	}
	t.Logf("%d crafting components, all tiered", components)
}
