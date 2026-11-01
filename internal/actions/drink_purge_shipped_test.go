package actions

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestPurgeableConditionIdsOnTheShippedWorld pins the FINAL set the Purging
// Draught strips, on the real shipped world (lighting plan 5c final review,
// finding 4). The repo-root TestPurgeablePotionSetOnTheShippedWorld pins only
// the items-derived half, items.PotionEffectConditionIds; this one pins what
// purgeableConditionIds makes of it after the 54-75 floor and the carve-outs
// (the two detox items' own conditions, the draught's flavour condition 70
// and the weakness 76 it applies afterwards).
//
// configs.ReloadConfig and the loaders resolve "_datafiles/..." against the
// process CWD, which for a package test is this package's directory, so the
// test changes to the repo root first (t.Chdir restores it). Both registries
// are swapped for empty seeds before loading and restored after, so the
// shipped world does not leak into the rest of the package's tests.
func TestPurgeableConditionIdsOnTheShippedWorld(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	t.Chdir(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{}))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	// The dogmud world ships 433 items on 2026-09-28; the default world ships
	// 110. 400 separates the two, as the root guard does.
	if n := len(items.GetAllItemSpecs()); n < 400 {
		t.Fatalf("loaded only %d items: this test is not seeing the shipped world", n)
	}

	// 93 comes from a potion (Ysolde's Purge), so the derived half holds it
	// and only the detox carve-out removes it; without this the absence check
	// below could pass because 93 was never in play.
	if !items.PotionEffectConditionIds()[93] {
		t.Fatal("condition 93 is no longer potion-derived; the carve-out row below proves nothing")
	}

	set := purgeableConditionIds()
	for _, id := range []int{7, 47, 48, 49, 51, 82, 130} {
		if !set[id] {
			t.Errorf("condition %d is a potion-only effect; the purge must strip it", id)
		}
	}
	for id, why := range map[int]string{
		5:  "also granted by food and bandages; stripping it would undo a meal",
		44: "Herbal Tea's, a cooked drink since #375 phase 7; the purge never strips a meal's",
		70: "the Purging Draught's own flavour condition",
		76: "the weakness the purge applies afterwards",
		93: "a detox item's own condition",
	} {
		if set[id] {
			t.Errorf("condition %d must not be stripped: %s", id, why)
		}
	}
}
