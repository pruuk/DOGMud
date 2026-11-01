package actions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// R22 (#375): a player may sell the crafted Iron Dagger (item 10009) to Smith
// Rusk (mob 9116) instead of showing it to her. Since #375 phase 7 the sale no
// longer completes quest 35 "First Heat" (every quest ends at a hand-in): the
// player tells Rusk afterwards. The sale itself must still go through on a
// fresh world, where her mob template says `gold: 0`. This test runs the real
// Sell path against the REAL item, mob and quest files, with a fresh (empty)
// shop store.
//
// Why she can pay: RegisterMobShop (internal/mobs/crafter.go) floors a shop's
// seed gold at 500 whatever the YAML says, so Rusk opens with Gold 500 and
// StartingGold 500. EvaluateBuyRules keeps a reserve of StartingGold *
// ShopGoldReserveRatio (0.50 shipped) = 250, and the dagger (value 15) is not
// in her stock list, so it is priced flat at ceil(15 * ShopBuyRatio 0.50) = 8.
// 500 - 8 >= 250, so the sale goes through.
//
// The shipped balance numbers are the ones loaded from _datafiles/config.yaml
// (ReloadConfig), not the Go defaults; they are asserted below so a retune of
// either knob that could change the outcome shows up here, by name.

const (
	ruskMobId   = 9116
	ruskRoomId  = 5245
	ruskDagger  = 10009
	firstHeatId = 35
)

func TestSell_RuskBuysTheCraftedDaggerOnAFreshWorld(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repo := filepath.Join(filepath.Dir(here), "..", "..")
	world := filepath.Join(repo, "_datafiles", "world", "dogmud")

	// Shipped config (not Go defaults), then the real item and mob files.
	t.Chdir(repo)
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())
	cfg := configs.GetConfig()
	require.Equal(t, 0.50, float64(cfg.Balance.ShopBuyRatio), "shipped ShopBuyRatio")
	require.Equal(t, 0.50, float64(cfg.Balance.ShopGoldReserveRatio), "shipped ShopGoldReserveRatio")
	cfg.FilePaths.DataFiles = configs.ConfigString(world)
	configs.SetConfigForTest(t, cfg)

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{}))
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{}))
	conditions.LoadDataFiles()
	species.LoadDataFiles()
	items.LoadDataFiles()
	mobs.LoadDataFiles()
	// The real quest registry is global with no seed helper: the step order
	// GiveQuestToken enforces comes from it. Reloaded empty on cleanup below.
	quests.LoadDataFiles()
	require.NotNil(t, items.GetItemSpec(ruskDagger), "real item 10009 must load")
	require.Equal(t, "Iron Dagger", items.GetItemSpec(ruskDagger).Name)

	// A fresh world: no shop save file and an empty cache. The shop store
	// (and every SaveShop the sale makes) lives in a temp dir, never the repo.
	tmpData := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpData, "quests"), 0o755))
	cfg.FilePaths.DataFiles = configs.ConfigString(tmpData)
	configs.SetConfigForTest(t, cfg)
	// Runs before the config restore, so this reload reads the empty temp dir
	// and leaves the quest registry as the rest of the package expects it.
	t.Cleanup(quests.LoadDataFiles)
	shops.ClearCache()
	t.Cleanup(shops.ClearCache)

	// Room 5245 in Rusk's zone, lit above the faces band (the real room's
	// lamp is 50; the sell sight gate is not under test).
	room := &rooms.Room{
		RoomId: ruskRoomId,
		Zone:   "Pothole Coulee",
		Title:  "The Coulee Smithy",
		Exits:  map[string]exit.RoomExit{},
		Lamp:   rooms.LampPtr(60),
	}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{ruskRoomId: room},
		map[string]*rooms.ZoneConfig{
			"Pothole Coulee": {Name: "Pothole Coulee", RoomId: ruskRoomId, RoomIds: map[int]struct{}{ruskRoomId: {}}},
		},
	))

	// Spawn the REAL Rusk exactly as the world does (NewMobById registers her shop).
	rusk := mobs.NewMobByIdFresh(mobs.MobId(ruskMobId), ruskRoomId)
	require.NotNil(t, rusk, "real mob 9116 must spawn")
	require.Equal(t, 0, rusk.Character.Gold, "the template ships gold: 0; this test is about that case")
	room.AddMob(rusk.InstanceId)
	t.Cleanup(func() { room.RemoveMob(rusk.InstanceId); mobs.SetInstanceForTest(rusk.InstanceId, nil) })

	inv := shops.GetShopInventory(rusk.Zone, int(rusk.MobId), rusk.HomeRoomId)
	require.NotNil(t, inv, "Rusk's shop must be registered on spawn")
	require.Equal(t, "blacksmithing", inv.CraftSupport)
	require.Equal(t, 500, inv.Gold, "a gold: 0 mob seeds at the 500 floor")
	require.Equal(t, 500, inv.StartingGold)
	require.Nil(t, inv.GetStock(ruskDagger), "the dagger is not in her stock list, so it is priced flat")

	// The player: holds the dagger and 35-craft, with the real quest 35 live.
	u := users.NewTestUser(1, "smithling", "Smithling", 1)
	u.Character.RoomId = ruskRoomId
	u.Character.Gold = 0
	u.Character.Conditions = conditions.New()
	require.True(t, u.Character.StoreItem(items.New(ruskDagger)))
	require.True(t, u.Character.GiveQuestToken("35-start"))
	require.True(t, u.Character.GiveQuestToken("35-craft"))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{u.UserId: u}))
	seller := &UserActor{User: u, Room: room}

	t.Cleanup(questengine.ResetEngineForTest())
	data, err := os.ReadFile(filepath.Join(world, "quests", "35-first_heat.yaml"))
	require.NoError(t, err)
	var q questengine.QuestDef
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.Equal(t, firstHeatId, q.QuestId)
	questengine.GetEngine().RegisterQuest(&q)
	require.False(t, u.Character.HasQuest("35-end"))

	res := Sell(seller, SellOptions{ItemName: "dagger", Quantity: 1})

	require.Equal(t, 1, res.Sold, "Rusk must buy the crafted dagger (reason=%v)", res.Reason)
	assert.Equal(t, 8, res.TotalGold, "flat price ceil(15 * 0.50)")
	_, still := u.Character.FindInBackpack("dagger")
	assert.False(t, still, "the dagger leaves the player's pack")
	assert.Equal(t, 500-8, inv.Gold, "her shop purse pays, not her mob gold")
	assert.False(t, u.Character.HasQuest("35-end"), "a sale no longer turns First Heat in; Rusk's hand-in does")
	assert.Equal(t, 8, u.Character.Gold, "the player is paid the sale price")
}
