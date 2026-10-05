package merchantchests

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveSettings mirrors the shipped merchant_chests.yaml settings, except
// that the intervals are in rounds: under test defaults a game hour or day
// can be zero rounds long.
func liveSettings() Settings {
	return Settings{
		RestockInterval:        `300 rounds`,
		RelockInterval:         `50 rounds`,
		GoldValueRatio:         0.5,
		GoldSpread:             0.5,
		ItemsMin:               1,
		ItemsMax:               3,
		LockBase:               4,
		LockPerDoubling:        2,
		LockMin:                6,
		LockMax:                20,
		PerceptionBase:         100,
		PerceptionPerDoubling:  4,
		SleepingPerceptionMult: 0.5,
	}
}

func TestLockDifficulty_ScalesWithStockValueAndClamps(t *testing.T) {
	s := liveSettings()
	assert.Equal(t, 6, LockDifficulty(s, 0), "no stock value: the floor")
	assert.Equal(t, 6, LockDifficulty(s, 1))
	assert.Equal(t, 9, LockDifficulty(s, 5))
	assert.Equal(t, 16, LockDifficulty(s, 60))
	assert.Equal(t, 18, LockDifficulty(s, 144))
	assert.Equal(t, 20, LockDifficulty(s, 100000), "clamped to lock_max")

	prev := 0
	for _, avg := range []float64{1, 2, 4, 8, 16, 32, 64, 128} {
		d := LockDifficulty(s, avg)
		assert.GreaterOrEqual(t, d, prev, "never easier for richer stock (avg %.0f)", avg)
		prev = d
	}
}

func TestTargetPerception_RicherStockSharperEye(t *testing.T) {
	s := liveSettings()
	assert.Equal(t, 104, TargetPerception(s, 1))
	assert.Equal(t, 110, TargetPerception(s, 5))
	assert.Equal(t, 124, TargetPerception(s, 60))
	assert.Equal(t, 129, TargetPerception(s, 144))
	assert.Less(t, TargetPerception(s, 3), TargetPerception(s, 30))
}

func TestGoldFor_HalfTheAverageWithinSpread(t *testing.T) {
	s := liveSettings()
	assert.Equal(t, 25, GoldFor(s, 100, 0), "low end: avg/2 x 0.5")
	assert.Equal(t, 50, GoldFor(s, 100, 0.5), "middle: avg/2")
	assert.Equal(t, 75, GoldFor(s, 100, 1), "high end: avg/2 x 1.5")
	assert.Equal(t, 1, GoldFor(s, 0.5, 0), "never less than 1")
}

func TestValidate(t *testing.T) {
	good := Catalog{
		Settings: liveSettings(),
		Generic:  Chest{Name: `chest`, Description: `A chest.`},
		Chests:   []Chest{{MobId: 2, RoomId: 1, Name: `strongbox`}, {MobId: 3, RoomId: 1}},
	}
	require.NoError(t, Validate(good))
	require.NoError(t, Validate(Catalog{}), "an empty catalog is valid")

	bad := good
	bad.Chests = append([]Chest(nil), good.Chests...)
	bad.Chests = append(bad.Chests, Chest{MobId: 4, RoomId: 1, Name: `Strongbox`})
	err := Validate(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `already has a "strongbox" chest`)

	bad = good
	bad.Chests = []Chest{{MobId: 2, RoomId: 1, Name: `iron chest`}}
	require.Error(t, Validate(bad), "a two-word name cannot be typed as one container noun")

	bad = good
	bad.Settings.LockMax = 40
	require.Error(t, Validate(bad), "pins beyond GetLockSequence's range")

	bad = good
	bad.Chests = []Chest{{MobId: 2, RoomId: 1, Difficulty: 1}}
	require.Error(t, Validate(bad))
}

func TestSet_ChestsFallBackToGeneric(t *testing.T) {
	defer SetForTest(Catalog{
		Settings: liveSettings(),
		Generic:  Chest{Name: `Chest`, Description: "A generic chest.\n"},
		Chests: []Chest{
			{MobId: 2, RoomId: 1},
			{MobId: 3, RoomId: 1, Name: `Vault`, Description: `A vault.`},
		},
	})()

	ch, ok := ChestAt(1, `chest`)
	require.True(t, ok)
	assert.Equal(t, `A generic chest.`, ch.Description)
	v, ok := ChestAt(1, `vault`)
	require.True(t, ok, "names are lower-cased")
	assert.Equal(t, 3, v.MobId)
	_, ok = ChestAt(2, `chest`)
	assert.False(t, ok)
}

const (
	testItemA = 71001
	testItemB = 71002
	testItemC = 71003
	testMobId = 2
)

// seedWorld seeds three items, a merchant template selling them, a room it
// stands in, and a catalog with one strongbox there.
func seedWorld(t *testing.T) (*rooms.Room, Chest) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		testItemA: {ItemId: testItemA, Name: `iron ingot`, Type: items.Object, Value: 10},
		testItemB: {ItemId: testItemB, Name: `steel ingot`, Type: items.Object, Value: 30},
		testItemC: {ItemId: testItemC, Name: `arbalest`, Type: items.Weapon, Value: 200},
	}))
	merchant := &mobs.Mob{
		MobId: testMobId,
		Zone:  `TestZone`,
		Character: characters.Character{
			Name: `Smith Brindle`,
			Shop: characters.Shop{
				{ItemId: testItemA}, {ItemId: testItemB}, {ItemId: testItemC}, {ItemId: testItemA},
			},
		},
	}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{testMobId: merchant}, map[int]*mobs.Mob{}))
	room := &rooms.Room{RoomId: 1, Zone: `TestZone`, Title: `Smithy`, Exits: map[string]exit.RoomExit{}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room},
		map[string]*rooms.ZoneConfig{`TestZone`: {Name: `TestZone`, RoomId: 1, RoomIds: map[int]struct{}{1: {}}}}))
	t.Cleanup(SetForTest(Catalog{
		Settings: liveSettings(),
		Generic:  Chest{Name: `chest`, Description: `A chest.`},
		Chests:   []Chest{{MobId: testMobId, RoomId: 1, Name: `strongbox`, Description: `An iron strongbox.`}},
	}))
	ch, ok := ChestAt(1, `strongbox`)
	require.True(t, ok)
	return rooms.LoadRoom(1), ch
}

func TestStockItemIdsAndAverage_DistinctShopItems(t *testing.T) {
	seedWorld(t)
	assert.Equal(t, []int{testItemA, testItemB, testItemC}, StockItemIds(testMobId), "a repeated shop entry counts once")
	assert.InDelta(t, 80.0, AverageStockValue(testMobId), 0.001, "(10+30+200)/3")
	assert.Equal(t, 0.0, AverageStockValue(999), "unknown merchant")
}

func TestRestock_FillsLocksAndMarksGoods(t *testing.T) {
	room, ch := seedWorld(t)
	orig := util.GetRoundCount()
	defer util.SetRoundCount(orig)
	util.SetRoundCount(5000)

	for i := 0; i < 50; i++ {
		Restock(room, ch, util.GetRoundCount())

		c, ok := room.Containers[`strongbox`]
		require.True(t, ok, "the chest is placed")
		assert.Equal(t, `An iron strongbox.`, c.Description)
		assert.Equal(t, uint8(LockDifficulty(liveSettings(), 80)), c.Lock.Difficulty)
		assert.True(t, c.Lock.IsLocked(), "a restocked chest is locked")
		assert.Equal(t, uint64(i+1), c.Lock.RotationSeed, "each restock rotates the combination")

		assert.GreaterOrEqual(t, c.Gold, GoldFor(liveSettings(), 80, 0))
		assert.LessOrEqual(t, c.Gold, GoldFor(liveSettings(), 80, 1))

		require.GreaterOrEqual(t, len(c.Items), 1)
		require.LessOrEqual(t, len(c.Items), 3)
		seen := map[int]bool{}
		for _, it := range c.Items {
			assert.Contains(t, []int{testItemA, testItemB, testItemC}, it.ItemId, "drawn from the shop list")
			assert.False(t, seen[it.ItemId], "no repeats in one restock")
			seen[it.ItemId] = true
			assert.Equal(t, `Smith Brindle`, it.StolenFrom, "the merchant's property")
			assert.Equal(t, testMobId, it.StolenFromMob)
			assert.True(t, it.IsMerchantGoods())
			assert.False(t, it.IsStolen(), "not stolen until someone takes it out")
		}
	}
	assert.Equal(t, uint64(5000), lastRestocked(room, `strongbox`))
}

// tickAt is the first round Tick looks at, at or after r.
func tickAt(r uint64) uint64 {
	if r%tickEveryRounds == 0 {
		return r
	}
	return r - r%tickEveryRounds + tickEveryRounds
}

func TestEnsureAll_PlacesButDoesNotStock_FirstTickStocks(t *testing.T) {
	room, _ := seedWorld(t)
	orig := util.GetRoundCount()
	defer util.SetRoundCount(orig)

	EnsureAll()
	c, ok := room.Containers[`strongbox`]
	require.True(t, ok, "boot places the chest")
	assert.Empty(t, c.Items, "boot stocks nothing: the saved round is not loaded yet")
	assert.True(t, c.HasLock())

	Tick(tickAt(9000))
	c = room.Containers[`strongbox`]
	assert.NotEmpty(t, c.Items, "the first tick stocks a new chest")
	assert.Equal(t, tickAt(9000), lastRestocked(room, `strongbox`))
}

func TestTick_RestocksOnlyWhenDue(t *testing.T) {
	room, ch := seedWorld(t)
	EnsureAll()
	Restock(room, ch, 9000)
	seed := room.Containers[`strongbox`].Lock.RotationSeed

	// Emptied by a thief; not due yet, so nothing refills.
	c := room.Containers[`strongbox`]
	c.Items, c.Gold = nil, 0
	room.Containers[`strongbox`] = c

	due := dueRound(liveSettings(), 9000)
	require.Equal(t, uint64(9300), due)
	Tick(tickAt(9001))
	assert.Empty(t, room.Containers[`strongbox`].Items, "not due yet")

	Tick(tickAt(due))
	after := room.Containers[`strongbox`]
	assert.NotEmpty(t, after.Items, "restocked once due")
	assert.Greater(t, after.Lock.RotationSeed, seed)
}

func TestEnsureAll_ReadsTheSavedRestockRound(t *testing.T) {
	room, ch := seedWorld(t)
	Restock(room, ch, 9000)
	c := room.Containers[`strongbox`]
	c.Items = nil
	room.Containers[`strongbox`] = c
	// A saved instance hands the round back as a plain int.
	room.SetLongTermData(restockKey(`strongbox`), 9000)

	EnsureAll() // a reboot
	Tick(tickAt(9001))
	assert.Empty(t, room.Containers[`strongbox`].Items, "the saved clock survives: not due")
	Tick(tickAt(9300))
	assert.NotEmpty(t, room.Containers[`strongbox`].Items)
}

func TestTick_RoundCounterResetRestocksAtOnce(t *testing.T) {
	room, ch := seedWorld(t)
	EnsureAll()
	Restock(room, ch, 90000)
	c := room.Containers[`strongbox`]
	c.Items = nil
	room.Containers[`strongbox`] = c

	Tick(tickAt(500)) // the counter went backwards
	assert.NotEmpty(t, room.Containers[`strongbox`].Items, "a record from the future is due, not frozen")
}

func TestTick_RelocksAPickedChest(t *testing.T) {
	room, ch := seedWorld(t)
	orig := util.GetRoundCount()
	defer util.SetRoundCount(orig)

	EnsureAll()
	Restock(room, ch, 9000)
	util.SetRoundCount(9010)
	c := room.Containers[`strongbox`]
	c.Lock.SetUnlocked()
	room.Containers[`strongbox`] = c
	MarkOpened(1, `strongbox`, 9010)
	seed := c.Lock.RotationSeed

	Tick(tickAt(9011))
	assert.False(t, room.Containers[`strongbox`].Lock.IsLocked(), "still open before relock_interval")

	Tick(tickAt(9060))
	got := room.Containers[`strongbox`].Lock
	assert.True(t, got.IsLocked(), "locked again 50 rounds after it was picked")
	assert.Greater(t, got.RotationSeed, seed, "with a new combination")
	assert.NotEmpty(t, room.Containers[`strongbox`].Items, "relocking leaves the contents alone")

	MarkOpened(1, `crate`, 9060) // not a merchant chest: ignored
}

func TestSyncContainer_KeepsContentsAndOpenState(t *testing.T) {
	room, ch := seedWorld(t)
	Restock(room, ch, 100)
	c := room.Containers[`strongbox`]
	c.Lock.SetUnlocked()
	c.Lock.Difficulty = 3 // a saved instance with a stale lock
	c.Description = ``
	room.Containers[`strongbox`] = c
	nItems := len(c.Items)

	syncContainer(room, ch, liveSettings())
	got := room.Containers[`strongbox`]
	assert.Equal(t, uint8(LockDifficulty(liveSettings(), 80)), got.Lock.Difficulty, "the catalog retunes the lock")
	assert.Equal(t, `An iron strongbox.`, got.Description)
	assert.Len(t, got.Items, nItems, "contents untouched")
	assert.NotZero(t, got.Lock.UnlockedRound, "an open chest stays open")
}

func TestCheckWorld(t *testing.T) {
	seedWorld(t)
	errs, warns := CheckWorld(func(mobId, roomId int) bool { return true })
	assert.Empty(t, errs)
	require.Len(t, warns, 1, "the seeded merchant has no perception base yet")
	assert.True(t, strings.Contains(warns[0], "stock value says 125"), warns[0])

	errs, _ = CheckWorld(func(mobId, roomId int) bool { return false })
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0], "does not spawn there")
}
