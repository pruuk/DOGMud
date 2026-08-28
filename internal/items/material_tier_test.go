package items

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMaterialTierMultiplierBand pins the owner's design (spec 5.1.2): five
// authored buckets mapping onto a 0.75 to 1.25 band, with tier 3 neutral.
//
// The band is deliberately narrow. Material tier is a MODIFIER on a difficulty
// that recipe.SkillMinimum already carries, so an authoring slip moves a recipe
// by one bucket rather than defining its odds outright. That is what makes a
// 208-file backfill safe to land incrementally.
func TestMaterialTierMultiplierBand(t *testing.T) {
	tests := []struct {
		tier int
		want float64
	}{
		{1, 0.75},
		{2, 0.875},
		{3, 1.0},
		{4, 1.125},
		{5, 1.25},
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
	if got := MaterialTierMultiplier(9); got != 1.25 {
		t.Errorf("MaterialTierMultiplier(9) = %v, want 1.25 (clamped to the top bucket)", got)
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

// TestNewMaterialsDeclareATier fails any NEW material authored without
// material_tier, while grandfathering the files that predate the field.
//
// 🎯 THIS TEST IS THE BACKFILL'S COMPLETION CHECK. Every file removed from
// materialTierBackfillOwed below must gain a real `material_tier:`. When the set
// is empty, the backfill is done and this test enforces total coverage on its
// own -- no roadmap prose, no checklist. U11 must not be declared done while the
// set is non-empty.
//
// That framing is deliberate: U6 was declared done with two criteria false,
// and nothing failed, because they were prose in a roadmap rather than a test.
func TestNewMaterialsDeclareATier(t *testing.T) {
	dir := materialsDirForTest(t)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	var missing []string
	seen := map[string]bool{}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		seen[e.Name()] = true
		if materialTierBackfillOwed[e.Name()] {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		if !strings.Contains(string(b), "material_tier:") {
			missing = append(missing, e.Name())
		}
	}

	for _, f := range missing {
		t.Errorf("%s has no material_tier. New materials MUST declare one "+
			"(1 common to 5 rarest). It is NOT rarity_tier -- that is a vendor "+
			"stock cap whose scale runs the other way.", f)
	}

	// A stale grandfather entry is its own bug: it silently exempts nothing, and
	// worse, it makes the backfill look incomplete when it is not.
	for name := range materialTierBackfillOwed {
		if !seen[name] {
			t.Errorf("materialTierBackfillOwed lists %q, which no longer exists. "+
				"Remove it -- a stale entry hides the true remaining count.", name)
		}
	}

	if len(materialTierBackfillOwed) == 0 {
		t.Log("BACKFILL COMPLETE: every material carries a tier and this test " +
			"now enforces total coverage.")
	}
}
