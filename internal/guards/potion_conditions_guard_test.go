package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// loadShippedConditionsAndItems follows lighting_parity_golden_test.go: the
// real config (so DataFiles is the dogmud world), then conditions before
// items, the boot order in main.go.
func loadShippedConditionsAndItems(t *testing.T) {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	// The dogmud world ships 433 items on 2026-09-28; the default world
	// ships 110. 400 separates the two with room for removals.
	if len(items.GetAllItemSpecs()) < 400 {
		t.Fatalf("loaded only %d items: the guard is not seeing the world", len(items.GetAllItemSpecs()))
	}
}

// TestPotionMagnitudeIsDeclared: a potion whose condition reads its
// magnitude (lighting plan 5c) with no item Magnitude would apply a
// zero-strength record, which reads as no effect at all. Checked here, at
// build time, because a load-time check would break every test binary that
// loads items without conditions.
func TestPotionMagnitudeIsDeclared(t *testing.T) {
	loadShippedConditionsAndItems(t)
	checked := 0
	for _, spec := range items.GetAllItemSpecs() {
		for _, id := range spec.ConditionIds {
			cs := conditions.GetConditionSpec(id)
			if cs == nil {
				continue
			}
			if _, scaled := cs.ScaledKind(); scaled {
				checked++
				if spec.Magnitude <= 0 {
					t.Errorf("item %d (%s) applies condition %d, which reads its magnitude, but declares no magnitude", spec.ItemId, spec.Name, id)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no shipped item applies a magnitude condition: the Pitsense Tincture is missing, so this guard proves nothing")
	}
}

// TestPurgeablePotionSetOnTheShippedWorld pins the derived set the Purging
// Draught strips against the 2026-09-28 audit.
func TestPurgeablePotionSetOnTheShippedWorld(t *testing.T) {
	loadShippedConditionsAndItems(t)
	set := items.PotionEffectConditionIds()
	for _, id := range []int{7, 44, 47, 48, 49, 51, 82, 130} {
		if !set[id] {
			t.Errorf("condition %d is granted only by a potion; it must be in the set", id)
		}
	}
	// 70 is the draught's own flavour condition. 75 is nausea, which the
	// drink path applies on an overdrink (drink.go) and no item names in its
	// conditionids, so the item-derived set cannot hold it; the purge still
	// strips it through the 54-75 floor in purgeableConditionIds.
	for id := 54; id <= 75; id++ {
		if id == 70 || id == 75 {
			continue
		}
		if !set[id] {
			t.Errorf("condition %d from the old block left the set", id)
		}
	}
	if set[5] {
		t.Error("condition 5 is also granted by food and bandages; it must not be in the set")
	}
}
