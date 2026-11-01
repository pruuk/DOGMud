package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/species"
)

const (
	sandPurseItemId   = 60
	pocketDummyMobId  = 9127
	reedJettyRoomId   = 5218
	jettyLockboxName  = "lockbox"
	lockboxDifficulty = 3
)

// TestPocketDummyIsATeachingProp holds the Coulee's practice mark (#375 phase
// 7) to what it is for: a prop a student robs over and over, read from the
// shipped data. The purse has no price at any merchant, and the dummy's only
// loot is that purse. The jetty's lockbox is the trap-free practice lock its
// noun describes, and stays empty, since nothing restocks a room container.
func TestPocketDummyIsATeachingProp(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	items.LoadDataFiles()
	species.LoadDataFiles()

	spec := items.GetItemSpec(sandPurseItemId)
	if spec == nil {
		t.Fatalf("item %d is not in the data", sandPurseItemId)
	}
	if spec.Value != 0 || len(spec.VendorCategories) != 0 || len(spec.SalvageReturns) != 0 {
		t.Errorf("the purse must be worth nothing: value %d, categories %v, salvage %v",
			spec.Value, spec.VendorCategories, spec.SalvageReturns)
	}

	// A merchant with no living-economy stock list prices from what it already
	// sells, so this one stocks the purse's own type and subtype. The control
	// proves it would pay for an ordinary object.
	stocked := &items.ItemSpec{ItemId: 975601, Name: "stocked", Type: spec.Type, Subtype: spec.Subtype}
	unstocked := &items.ItemSpec{ItemId: 975602, Name: "unstocked", Type: spec.Type, Subtype: spec.Subtype}
	if err := stocked.Validate(); err != nil {
		t.Fatalf("stocked spec: %v", err)
	}
	if err := unstocked.Validate(); err != nil {
		t.Fatalf("unstocked spec: %v", err)
	}
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		sandPurseItemId: spec, stocked.ItemId: stocked, unstocked.ItemId: unstocked,
	}))
	merchant := &mobs.Mob{BuysGeneral: true}
	merchant.Character.Shop = characters.Shop{{ItemId: stocked.ItemId, QuantityMax: 5, Quantity: 5}}
	if q := actions.QuoteSale(nil, nil, merchant, nil, items.New(unstocked.ItemId), false); q.Price < 1 {
		t.Fatalf("the control object must sell here, or this proves nothing: %+v", q)
	}
	purse := items.New(sandPurseItemId)
	if q := actions.QuoteSale(nil, nil, merchant, nil, purse, false); q.Price != 0 {
		t.Errorf("a merchant stocking the purse's type offers %d for it", q.Price)
	}
	shop := &shops.ShopInventory{Gold: 100000, StartingGold: 100000, CraftSupport: "general"}
	if q := actions.QuoteSale(nil, nil, merchant, shop, purse, false); q.Price != 0 {
		t.Errorf("a general shop offers %d for the purse", q.Price)
	}

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", dataPath, err)
	}
	dummy := templates[pocketDummyMobId]
	if dummy == nil {
		t.Fatalf("mob %d is not in the data", pocketDummyMobId)
	}
	if dummy.Character.Gold != 0 {
		t.Errorf("the dummy carries %d gold, want none", dummy.Character.Gold)
	}
	if !dummy.CharmImmune {
		t.Errorf("a charmed dummy would walk off the jetty and leave students no mark")
	}
	if len(dummy.Character.Items) != 1 || dummy.Character.Items[0].ItemId != sandPurseItemId {
		t.Errorf("the dummy's only carried item must be the purse: %+v", dummy.Character.Items)
	}
	if sp := species.GetSpecies(dummy.Character.SpeciesId); sp == nil || !sp.EmptyPockets {
		t.Errorf("species %d must have empty_pockets", dummy.Character.SpeciesId)
	}

	room := rooms.LoadRoomTemplate(reedJettyRoomId)
	if room == nil {
		t.Fatalf("room %d is not in the data", reedJettyRoomId)
	}
	box, ok := room.Containers[jettyLockboxName]
	if !ok {
		t.Fatalf("room %d has no %s", reedJettyRoomId, jettyLockboxName)
	}
	if box.Lock.Difficulty != lockboxDifficulty || box.Lock.RelockInterval == `` || len(box.Lock.TrapConditionIds) != 0 {
		t.Errorf("the lockbox must be a trap-free difficulty %d lock that relocks: %+v",
			lockboxDifficulty, box.Lock)
	}
	if len(box.Items) != 0 || box.Gold != 0 {
		t.Errorf("the lockbox must ship empty: %d items, %d gold", len(box.Items), box.Gold)
	}
}
