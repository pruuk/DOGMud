package health_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/economy/health"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"gopkg.in/yaml.v2"
)

// A fence's shop is typed "fence" on the dashboard (baubles slice D), from
// its mob template, while its snapshot keeps its real craft_support.
func TestCaptureSnapshot_FenceShopIsTypedFence(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.BaubleFenceGroups = configs.ConfigSliceString{"fence"}
	configs.SetConfigForTest(t, cfg)
	shops.IsolateForTest(t)
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		104: {MobId: 104, Groups: []string{"fence"}, Character: characters.Character{Name: "Fence Dealer Siv"}},
		108: {MobId: 108, Character: characters.Character{Name: "Jeweler Tess"}},
	}, map[int]*mobs.Mob{}))
	shops.RegisterShop("testzone", 104, 475, shops.ShopInventory{Gold: 500, StartingGold: 500, CraftSupport: shops.CraftSupportGeneral})
	shops.RegisterShop("testzone", 108, 482, shops.ShopInventory{Gold: 500, StartingGold: 500, CraftSupport: shops.CraftSupportJewelcrafting})

	byMob := map[int]health.ShopSnapshot{}
	for _, s := range health.CaptureSnapshot().Shops {
		byMob[s.MobId] = s
	}
	fence, tess := byMob[104], byMob[108]
	if !fence.Fence || fence.Type() != "fence" || fence.CraftSupport != "general" {
		t.Errorf("fence: Fence=%v Type=%q CraftSupport=%q, want true, fence, general", fence.Fence, fence.Type(), fence.CraftSupport)
	}
	if tess.Fence || tess.Type() != "jewelcrafting" {
		t.Errorf("jeweller: Fence=%v Type=%q, want false, jewelcrafting", tess.Fence, tess.Type())
	}
}

// A snapshot saved before Fence existed has no fence key: it decodes with
// Fence false and Type() is its craft_support; a false Fence writes no key.
func TestShopSnapshot_PreFenceYAMLTypesByCraftSupport(t *testing.T) {
	var s health.ShopSnapshot
	if err := yaml.Unmarshal([]byte("zone: thornwall\nmob_id: 104\ncraft_support: general\n"), &s); err != nil {
		t.Fatal(err)
	}
	if s.Fence || s.Type() != "general" {
		t.Errorf("old snapshot: Fence=%v Type=%q, want false, general", s.Fence, s.Type())
	}
	out, err := yaml.Marshal(health.ShopSnapshot{CraftSupport: "general"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "fence") {
		t.Errorf("omitempty keeps a non-fence snapshot unchanged:\n%s", out)
	}
	// The dashboard reads the JSON (s.fence in index.html).
	js, err := json.Marshal(health.ShopSnapshot{CraftSupport: "general"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(js), `"fence"`) {
		t.Errorf("json omitempty: a non-fence snapshot carries no fence key: %s", js)
	}
	js, err = json.Marshal(health.ShopSnapshot{CraftSupport: "general", Fence: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"fence":true`) {
		t.Errorf("json: a fence's snapshot says so as fence: %s", js)
	}
}

// Fences group under "fence" whatever their craft_support, in the rollup
// and on the per-shop rows (baubles slice D).
func TestScore_FenceIsItsOwnType(t *testing.T) {
	snap := health.Snapshot{Shops: []health.ShopSnapshot{
		{CraftSupport: "general", Fence: true, Stock: []health.StockSnapshot{{RestockQty: 1, Current: 2, Max: 10}}}, // 20
		{CraftSupport: "general", Stock: []health.StockSnapshot{{RestockQty: 1, Current: 6, Max: 10}}},              // 60
	}}
	scores := health.PerCraftSupportScores(snap)
	if scores["fence"] < 19.99 || scores["fence"] > 20.01 || scores["general"] < 59.99 || scores["general"] > 60.01 {
		t.Errorf("rollup: %v, want fence 20 and general 60", scores)
	}
	rows := health.ScoreWithConfig(&snap, nil, testScoringCfg).PerShop
	if rows[0].CraftSupport != "fence" || rows[1].CraftSupport != "general" {
		t.Errorf("per-shop types: %q %q, want fence general", rows[0].CraftSupport, rows[1].CraftSupport)
	}
}
