package aicompanion

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
)

// browse mirrors list (baubles slice D): the companion sees a shop's listed
// secondhand shelf after its stock, in shelf order, never a held entry. The
// names reach the model, so they are ModelName: a bauble whose text a
// player's key wrote reads as its carrier, finder-only or moderated. Shelf
// rows are never remembered: each is one of a kind, and they share ItemIds
// with regular stock (every bauble is item 900).
func TestBrowseShopsShowsTheShelfInTheModelsView(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: `Curious Trinket`, NameSimple: `trinket`, Type: items.Object, Subtype: items.Mundane, Value: 1},
		96001:              {ItemId: 96001, Name: `iron sword`, Type: items.Weapon, Value: 100},
	}))
	baubles.SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	configs.SetConfigForTest(t, cfg)

	room := &rooms.Room{RoomId: 9601, Zone: `TestZone`, Title: `Shop`}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9601: room},
		map[string]*rooms.ZoneConfig{`TestZone`: {Name: `TestZone`, RoomId: 9601, RoomIds: map[int]struct{}{9601: {}}}}))
	keeper := &mobs.Mob{MobId: 961, InstanceId: 9602, HomeRoomId: 9601, Zone: `TestZone`,
		Character: characters.Character{Name: `Keeper`, RoomId: 9601, Conditions: conditions.New(),
			Shop: characters.Shop{{ItemId: 96001, Price: 100}}}}
	keeper.Character.HealthMax.Value, keeper.Character.Health = 100, 100
	mobs.SetInstanceForTest(9602, keeper)
	t.Cleanup(func() { mobs.SetInstanceForTest(9602, nil) })
	room.AddMob(9602)

	shops.IsolateForTest(t)
	si := shops.RegisterShop(`TestZone`, 961, 9601, shops.ShopInventory{Gold: 100, CraftSupport: shops.CraftSupportGeneral,
		Stock: []shops.StockEntry{{ItemId: 96001, RestockQty: 1, MaxStock: 2, Current: 1}}})

	bauble := func(name string, moderated bool) items.Item {
		rec, err := baubles.Create(baubles.Record{Name: name, NameSimple: `horse`, Tier: baubles.TierAverage, Value: 12,
			WeightLbs: 0.5, Description: `A toy horse.`, Status: baubles.StatusReady, PlayerKey: true, Moderated: moderated, FoundByUserId: 7})
		if err != nil {
			t.Fatal(err)
		}
		it := items.New(items.BaubleItemId)
		it.Bauble = rec.Id
		return it
	}
	now := time.Now()
	si.AffixedStock = []shops.AffixedStockEntry{
		{Item: bauble(`Painted Wooden Horse`, false), Price: 12, AddedAt: now},                           // finder-only
		{Item: bauble(`Carved Oak Horse`, true), Price: 14, AddedAt: now, HoldUntil: now.Add(time.Hour)}, // held
		{Item: bauble(`Glass Horse`, true), Price: 15, AddedAt: now},                                     // moderated player-key text
	}

	listings := browseShops(room)
	if len(listings) != 1 {
		t.Fatalf("one open merchant: %+v", listings)
	}
	wares := listings[0].Wares
	var shelf []ware
	for _, w := range wares {
		if w.Secondhand {
			shelf = append(shelf, w)
		}
	}
	if len(shelf) != 2 {
		t.Fatalf("the two listed shelf rows, not the held one: %+v", shelf)
	}
	for _, w := range shelf {
		if w.Name != `Curious Trinket` {
			t.Errorf("a bauble a player's key wrote reads as its carrier to the model: %+v", w)
		}
	}
	if shelf[0].Price != 12 || shelf[1].Price != 15 {
		t.Errorf("shelf order, each at its own price: %+v", shelf)
	}
	if !wares[len(wares)-1].Secondhand || wares[0].Secondhand {
		t.Errorf("the stock first, then the shelf, as list shows them: %+v", wares)
	}

	text := describeListing(listings[0], nil)
	for _, hidden := range []string{`Painted`, `Glass`, `Carved`} {
		if strings.Contains(text, hidden) {
			t.Errorf("the model never reads a player's text (%s): %q", hidden, text)
		}
	}
	if !strings.Contains(text, `Curious Trinket for 12 gold (secondhand)`) {
		t.Errorf("listing wording: %q", text)
	}

	mind := &Mind{}
	mind.rememberShop(listings[0], 9601, now.Unix())
	if w := mind.Shops[961].Wares[items.BaubleItemId]; w != nil {
		t.Errorf("shelf rows are not remembered: %+v", w)
	}
	if mind.Shops[961].Wares[96001] == nil {
		t.Error("the stock is remembered as before")
	}
}
