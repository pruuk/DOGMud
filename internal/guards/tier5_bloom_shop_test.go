package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"gopkg.in/yaml.v2"
)

const bloomWaferId = 40108

// The dealer's shelf is capped on purpose. Malk's torn page (tier 40) rose from
// 19 to 80 in an hour through EffectiveMaxStock, so the wafer's cap is set here,
// and it is the only tier 10 item, so RestockCadenceTier10Days is its own clock.
func TestBloomWaferShelfIsCapped(t *testing.T) {
	setupSearchLightWorld(t)
	cfg := configs.GetConfig()
	cfg.Balance.ShopMaxStockMultiplier = 2.0 // config.yaml:1600
	configs.SetConfigForTest(t, cfg)

	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "mobs", "new_plymouth_docks", "9802-sefton.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var mob struct {
		StockMultiplier float64 `yaml:"stock_multiplier"`
	}
	if err := yaml.Unmarshal(data, &mob); err != nil {
		t.Fatal(err)
	}
	if got := shops.EffectiveMaxStock(bloomWaferId, mob.StockMultiplier); got != 5 {
		t.Errorf("the wafer's shelf cap is %d, want 5", got)
	}

	tier10 := regexp.MustCompile(`(?m)^rarity_tier:\s*10\s*$`)
	found := map[string]bool{}
	root := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "items")
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err == nil && tier10.Match(b) {
			found[filepath.Base(p)] = true
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || !found["40108-bloom_wafer.yaml"] {
		t.Errorf("tier 10 items %v: RestockCadenceTier10Days must stay the wafer's own clock", found)
	}
}

// A shop's shelf is keyed by mob and room, so one spawn room is one shelf for
// the whole server. A second room spawning Sefton would be a second shelf, and
// would double every figure the owner approved.
func TestBloomWaferHasOneShelf(t *testing.T) {
	spawn := regexp.MustCompile(`(?m)^\s*-\s*mobid:\s*9802\s*$`)
	var found []string
	root := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "rooms")
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err == nil && spawn.Match(b) {
			found = append(found, filepath.Base(p))
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0] != "5530.yaml" {
		t.Errorf("rooms spawning Sefton: %v, want only 5530.yaml", found)
	}
}

// What stops a resale is having no vendor_categories, not not_salable, which
// only skips a validator. It holds at the dealer who stocks it and at every
// other kind of shop.
func TestBloomWaferCannotBeSoldBack(t *testing.T) {
	setupSearchLightWorld(t)
	spec := items.GetItemSpec(bloomWaferId)
	if spec == nil || len(spec.VendorCategories) != 0 || !spec.NotSalable {
		t.Fatalf("wafer spec %+v", spec)
	}
	for _, support := range shops.ValidCraftSupports {
		inv := &shops.ShopInventory{Gold: 5000, StartingGold: 5000, CraftSupport: support,
			Stock: []shops.StockEntry{{ItemId: bloomWaferId, RestockQty: 5, MaxStock: 5, Current: 1}}}
		offer := shops.EvaluateBuyRules(items.New(bloomWaferId), inv, "", false, shops.PricingConfigFromBalance(), nil)
		if offer.Price != 0 || offer.Refused != shops.BuyRefusedNoTrade {
			t.Errorf("%s shop: offer %+v, want refused with no trade", support, offer)
		}
	}
}
