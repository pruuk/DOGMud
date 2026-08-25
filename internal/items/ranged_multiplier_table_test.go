package items

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestRangedWeaponMultipliers_MatchTheU10dTable pins every Shooting-subtype
// weapon template to the U10d ranged damage line.
//
// Before U10d the bow line sat entirely above the melee line (2.00 - 7.50
// against a melee band that tops out near 1.50 outside the Blackrazor
// outlier). That inflation was compensating for real costs -- a shot is one
// attack where a melee weapon swings several times per round, and reload
// burns a shared cooldown -- so U10d MOVES the compensation rather than
// removing it: templates come down here, and the payback returns as a
// situational bonus for shooting from outside the fight.
//
// The test fails on an unlisted Shooting weapon as well as a mismatched one.
// A bow added later must be a deliberate decision against this line, not a
// silent return to 7.50.
func TestRangedWeaponMultipliers_MatchTheU10dTable(t *testing.T) {
	want := map[string]float64{
		"Ironhorn Warbow":  2.75,
		"Arbalest":         2.55,
		"Relic Sidearm":    2.20,
		"Hunting Bow":      2.00,
		"Training Bow":     1.45,
		"Primitive Pistol": 1.30,
		"Hand Crossbow":    1.10,
		"Sling":            0.75,
	}

	loadRealItemSpecsForTest(t)

	seen := map[string]bool{}
	for _, spec := range GetAllItemSpecs() {
		if spec.Subtype != Shooting {
			continue
		}
		seen[spec.Name] = true

		expected, listed := want[spec.Name]
		if !listed {
			t.Errorf("shooting weapon %q (id %d) is not in the U10d ranged table "+
				"(damage_multiplier %.2f). Add it to the table deliberately -- an "+
				"unlisted bow is how the pre-U10d 7.50 line comes back.",
				spec.Name, spec.ItemId, spec.DamageMultiplier)
			continue
		}
		if math.Abs(spec.DamageMultiplier-expected) > 1e-9 {
			t.Errorf("%q (id %d) damage_multiplier = %.2f, want %.2f",
				spec.Name, spec.ItemId, spec.DamageMultiplier, expected)
		}
	}

	for name := range want {
		if !seen[name] {
			t.Errorf("U10d table lists %q but no Shooting-subtype item with that "+
				"name loaded -- renamed, retyped, or deleted?", name)
		}
	}
}

// loadRealItemSpecsForTest points the item loader at the repo's real data
// files and restores the package-level caches afterwards.
//
// The data path is anchored on runtime.Caller, NOT the working directory:
// every test in a package shares one binary, and internal/actions'
// economy_test.go chdirs to the repo root, so a relative path would pass or
// fail by test ORDER.
func loadRealItemSpecsForTest(t *testing.T) {
	t.Helper()

	mudlog.SetupLogger(nil, "", "", false)

	originalItems, originalAttack, originalDefense := items, attackMessages, defenseMessages
	t.Cleanup(func() {
		items, attackMessages, defenseMessages = originalItems, originalAttack, originalDefense
	})

	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repoRoot, "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	LoadDataFiles()
}
