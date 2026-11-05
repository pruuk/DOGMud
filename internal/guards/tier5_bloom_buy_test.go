package guards

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"gopkg.in/yaml.v2"
)

// The wafer bought from the shipped Sefton, in his shipped room, against the
// shipped config: the stock falls, the purse falls, and the shop and the character
// both save and load, and the limit holds at the purchase.

const (
	seftonMobId = 9802
	seftonRoom  = 5530
	seftonZone  = "New Plymouth Docks"
)

// shippedBalance decodes the Balance block of config.yaml. A test binary reads Go
// defaults, so a figure the game ships comes from here.
func shippedBalance(t *testing.T) (configs.Balance, map[string]any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg configs.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Balance map[string]any `yaml:"Balance"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return cfg.Balance, raw.Balance
}

// seftonWorld loads the world with the shipped shop and Bloom knobs, shop saves in
// a temp folder, and spawns the real Sefton, registering his shop through the
// spawn path, which is RegisterMobShop.
func seftonWorld(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	setupSearchLightWorld(t)
	mobs.LoadDataFiles()

	room := rooms.LoadRoom(seftonRoom) // reads the world, so before the temp folder
	if room == nil {
		t.Fatalf("room %d did not load", seftonRoom)
	}

	shipped, _ := shippedBalance(t)
	cfg := configs.GetConfig()
	cfg.Balance.ShopMaxStockMultiplier = shipped.ShopMaxStockMultiplier
	cfg.Balance.BloomDealerWafersPerDay = shipped.BloomDealerWafersPerDay
	cfg.Balance.BloomDealerWindowHours = shipped.BloomDealerWindowHours
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	configs.SetConfigForTest(t, cfg)

	shops.ClearCache()
	t.Cleanup(shops.ClearCache)

	sefton := mobs.NewMobByIdFresh(seftonMobId, seftonRoom)
	if sefton == nil {
		t.Fatalf("mob %d did not spawn", seftonMobId)
	}
	mobs.SetInstanceForTest(sefton.InstanceId, sefton)
	t.Cleanup(func() { mobs.SetInstanceForTest(sefton.InstanceId, nil) })
	room.AddMob(sefton.InstanceId)
	t.Cleanup(func() { room.RemoveMob(sefton.InstanceId) })
	return sefton, room
}

func waferEntry(t *testing.T) *shops.StockEntry {
	t.Helper()
	inv := shops.GetShopInventory(seftonZone, seftonMobId, seftonRoom)
	if inv == nil {
		t.Fatal("Sefton has no shop")
	}
	entry := inv.GetStock(bloomWaferId)
	if entry == nil {
		t.Fatalf("Sefton's shop has no wafer: %+v", inv.Stock)
	}
	return entry
}

func waferBuyer(t *testing.T, room *rooms.Room, gold int) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(7461, "waferbuyer", "WaferBuyer", 7461)
	u.Character.RoomId = room.RoomId
	u.Character.Gold = gold
	u.Character.Stats.Strength.Base = 200
	u.Character.Stats.Strength.Recalculate()
	u.Character.Stats.Perception.Base = 100
	u.Character.Stats.Perception.Recalculate()
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{u.UserId: u}))
	return u
}

func heldWafers(c *characters.Character) int {
	n := 0
	for _, it := range c.Items {
		if it.ItemId == bloomWaferId {
			n++
		}
	}
	return n
}

// The shelf cap comes out of RegisterMobShop on the real Sefton and the shipped
// multiplier, and a change to either is what this catches. See TestBloomWaferShelfIsCapped.
func TestBloomWafer_ABuyEndToEnd(t *testing.T) {
	sefton, room := seftonWorld(t)
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	u := waferBuyer(t, room, 2000)
	actor := &actions.UserActor{User: u, Room: room}

	startStock := waferEntry(t).Current
	if startStock < 4 {
		t.Fatalf("the shelf seeds %d, too few for this test", startStock)
	}

	res := actions.Buy(actor, actions.BuyOptions{Request: "wafer"})
	if !res.Success || res.ItemId != bloomWaferId {
		t.Fatalf("buy: %+v (messages %q)", res, events.DrainQueuedMessagesForTest(u.UserId))
	}
	if got := waferEntry(t).Current; got != startStock-1 {
		t.Errorf("stock %d, want %d", got, startStock-1)
	}
	paid := 2000 - u.Character.Gold
	if paid < 120 || paid > 180 {
		t.Errorf("paid %d: value 120 at the scarcity curve runs 130 to 169 before barter", paid)
	}
	if heldWafers(u.Character) != 1 {
		t.Errorf("holds %d wafers, want 1", heldWafers(u.Character))
	}

	// The character's save holds the wafer and the limit.
	out, err := yaml.Marshal(u.Character)
	if err != nil {
		t.Fatal(err)
	}
	loaded := characters.New()
	if err := yaml.Unmarshal(out, loaded); err != nil {
		t.Fatal(err)
	}
	if heldWafers(loaded) != 1 || loaded.Gold != u.Character.Gold {
		t.Errorf("save lost the purchase: %d wafers, %d gold", heldWafers(loaded), loaded.Gold)
	}
	if len(loaded.Timers) != 1 {
		t.Errorf("save lost the limit's count: %+v", loaded.Timers)
	}

	// The shop's save holds the fallen stock: drop the cache, read it from disk.
	shops.ClearCache()
	if got := waferEntry(t).Current; got != startStock-1 {
		t.Errorf("after a reload the stock is %d, want %d", got, startStock-1)
	}
	_ = sefton
}

// The shipped limit, at the real seller, across a restart of the character.
func TestBloomWafer_TheShippedDealerStopsAtTheLimit(t *testing.T) {
	sefton, room := seftonWorld(t)
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	limit := int(configs.GetBalanceConfig().BloomDealerWafersPerDay)
	u := waferBuyer(t, room, 100000)
	actor := &actions.UserActor{User: u, Room: room}

	// Past the shelf cap, to show the limit stops this and the shelf does not. He
	// restocks, and a player standing inside buys each refill: a limit on the door
	// alone would not hold.
	for i := 0; i < limit; i++ {
		if res := actions.Buy(actor, actions.BuyOptions{Request: "wafer"}); !res.Success {
			t.Fatalf("buy %d of %d: %+v (told %q, he said %q, stock %+v)", i+1, limit, res,
				events.DrainQueuedMessagesForTest(u.UserId), events.DrainQueuedInputsForTest(sefton.InstanceId), waferEntry(t))
		}
		entry := waferEntry(t)
		entry.Current = entry.MaxStock
	}
	gold, held, stock := u.Character.Gold, heldWafers(u.Character), waferEntry(t).Current
	events.DrainQueuedInputsForTest(sefton.InstanceId)

	res := actions.Buy(actor, actions.BuyOptions{Request: "wafer"})
	if res.Success || res.Reason != actions.BuyReasonLimit {
		t.Fatalf("buy past the limit: %+v", res)
	}
	if u.Character.Gold != gold || heldWafers(u.Character) != held || waferEntry(t).Current != stock {
		t.Errorf("a refused buy moved something: gold %d/%d, held %d/%d, stock %d/%d",
			u.Character.Gold, gold, heldWafers(u.Character), held, waferEntry(t).Current, stock)
	}
	said := strings.Join(events.DrainQueuedInputsForTest(sefton.InstanceId), " ")
	if !strings.Contains(said, "That is enough from me for one day. Come back tomorrow, if you still want to.") {
		t.Errorf("Sefton said %q", said)
	}

	// The list still shows his stock, and tells this player they can buy no more.
	inv := shops.GetShopInventory(seftonZone, seftonMobId, seftonRoom)
	stockRows := characters.Shop{{ItemId: bloomWaferId, Quantity: inv.GetStock(bloomWaferId).Current}}
	if note := actions.BuyLimitNote(u.Character, stockRows, sefton.Character.Name); !strings.Contains(note, "Bloom Wafer") {
		t.Errorf("list note: %q", note)
	}

	// A relog waits out nothing.
	out, err := yaml.Marshal(u.Character)
	if err != nil {
		t.Fatal(err)
	}
	loaded := characters.New()
	if err := yaml.Unmarshal(out, loaded); err != nil {
		t.Fatal(err)
	}
	if !actions.BuyLimitReached(loaded, bloomWaferId) {
		t.Errorf("a relog cleared the limit: %+v", loaded.Timers)
	}
}

// The limit is live on the shipped config, by name: a knob absent from config.yaml
// silently runs on its Go default, and the plan's arithmetic reads these figures.
func TestBloomDealerLimitIsOnTheShippedConfig(t *testing.T) {
	shipped, raw := shippedBalance(t)
	for _, key := range []string{"BloomDealerWafersPerDay", "BloomDealerWindowHours"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("%s is absent from config.yaml", key)
		}
	}
	if shipped.BloomDealerWafersPerDay != 3 || shipped.BloomDealerWindowHours != 24 {
		t.Errorf("the limit ships at %d wafers in %d hours; the owner set 3 in 24, and the plan's numbers read them",
			shipped.BloomDealerWafersPerDay, shipped.BloomDealerWindowHours)
	}
}

// Sefton's own text must keep saying he will not sell much to anyone: the refusal
// is the second time a player hears it, and it must not be the first.
func TestSeftonWarnsOfTheLimitBeforeItBites(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "dialogue", "new_plymouth_docks", "9802.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(strings.Fields(string(data)), " "), "draws notice") {
		t.Error("Sefton no longer says he will not sell much, because a customer who buys a lot draws notice")
	}
}
