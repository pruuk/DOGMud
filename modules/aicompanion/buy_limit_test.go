package aicompanion

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The wafer limit is any player's (owner ruling 2026-11-04), so a companion's
// companion-buy counts against its owner's window and is refused the same way past
// it. Otherwise a companion would be three more wafers a day.

const (
	limitWaferId   = 40108
	limitOwnerId   = 9711
	limitKeeperId  = 9712
	limitHerId     = 9713
	limitKeeperTpl = 9802
	limitRoomId    = 9710
)

type limitScene struct {
	owner  *users.UserRecord
	her    *mobs.Mob
	keeper *mobs.Mob
	room   *rooms.Room
	shop   *shops.ShopInventory
}

func limitFixture(t *testing.T) *limitScene {
	t.Helper()
	enabled := module.cfg.Enabled
	module.cfg.Enabled = true
	t.Cleanup(func() { module.cfg.Enabled = enabled })
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		limitWaferId: {ItemId: limitWaferId, Name: `Bloom Wafer`, Type: items.Potion, Subtype: items.Drinkable, Uses: 1, Value: 120, Weight: 0.1},
	}))
	cfg := configs.GetConfig()
	cfg.Balance.BloomDealerWafersPerDay = 3
	cfg.Balance.BloomDealerWindowHours = 24
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	configs.SetConfigForTest(t, cfg)

	room := &rooms.Room{RoomId: limitRoomId, Zone: `LimitZone`, Title: `Behind`, Biome: `city`, Lamp: rooms.LampPtr(90)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{limitRoomId: room},
		map[string]*rooms.ZoneConfig{`LimitZone`: {Name: `LimitZone`, RoomId: limitRoomId, RoomIds: map[int]struct{}{limitRoomId: {}}}}))

	owner := users.NewTestUser(limitOwnerId, `limitowner`, `LimitOwner`, 9711)
	owner.Character.RoomId = limitRoomId
	owner.Character.Gold = 100000
	owner.Character.Stats.Strength.Base = 200
	owner.Character.Stats.Strength.Recalculate()
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{limitOwnerId: owner}))
	room.AddPlayer(limitOwnerId)

	keeper := &mobs.Mob{MobId: limitKeeperTpl, InstanceId: limitKeeperId, HomeRoomId: limitRoomId, Zone: `LimitZone`,
		Character: characters.Character{Name: `Sefton`, RoomId: limitRoomId, Conditions: conditions.New(),
			Shop: characters.Shop{{ItemId: limitWaferId}}}}
	mobs.SetInstanceForTest(limitKeeperId, keeper)
	t.Cleanup(func() { mobs.SetInstanceForTest(limitKeeperId, nil) })
	room.AddMob(limitKeeperId)

	her := &mobs.Mob{MobId: 1, InstanceId: limitHerId, HomeRoomId: limitRoomId, Zone: `LimitZone`,
		Character: characters.Character{Name: `Mara`, RoomId: limitRoomId, Gold: 100000, Conditions: conditions.New()}}
	her.Character.Stats.Strength.ValueAdj = 200
	her.Character.Stats.Perception.ValueAdj = 100
	her.Character.Charm(limitOwnerId, characters.CharmPermanent, ``)
	mobs.SetInstanceForTest(limitHerId, her)
	t.Cleanup(func() { mobs.SetInstanceForTest(limitHerId, nil) })
	room.AddMob(limitHerId)

	shops.ClearCache()
	t.Cleanup(shops.ClearCache)
	shop := shops.RegisterShop(`LimitZone`, limitKeeperTpl, limitRoomId, shops.ShopInventory{Gold: 500, CraftSupport: shops.CraftSupportAlchemy,
		Stock: []shops.StockEntry{{ItemId: limitWaferId, RestockQty: 50, MaxStock: 50, Current: 50}}})
	return &limitScene{owner: owner, her: her, keeper: keeper, room: room, shop: shop}
}

func (s *limitScene) companionBuys(t *testing.T) {
	t.Helper()
	handled, err := mobCompanionBuy(`9712 1 bloom wafer`, s.her, s.room)
	require.NoError(t, err)
	require.True(t, handled)
}

func (s *limitScene) ownerBuys() actions.BuyResult {
	return actions.Buy(&actions.UserActor{User: s.owner, Room: s.room}, actions.BuyOptions{Request: `bloom wafer`})
}

func countWafers(c *characters.Character) int {
	n := 0
	for _, it := range c.Items {
		if it.ItemId == limitWaferId {
			n++
		}
	}
	return n
}

// Owner at the limit: the companion is refused as the owner would be.
func TestCompanionBuy_OwnerAtTheLimitRefusesHer(t *testing.T) {
	s := limitFixture(t)
	for i := 0; i < 3; i++ {
		require.True(t, s.ownerBuys().Success, `owner buy %d`, i+1)
	}
	gold, stock := s.her.Character.Gold, s.shop.GetStock(limitWaferId).Current
	events.DrainQueuedInputsForTest(limitKeeperId)

	s.companionBuys(t)

	assert.Equal(t, gold, s.her.Character.Gold, `charges nothing`)
	assert.Equal(t, stock, s.shop.GetStock(limitWaferId).Current, `stock is unchanged`)
	assert.Zero(t, countWafers(&s.her.Character), `moves nothing`)
	said := strings.Join(events.DrainQueuedInputsForTest(limitKeeperId), ` `)
	assert.Contains(t, said, `That is enough from me for one day.`)
}

// Her purchase spends one of her owner's, and none of her own.
func TestCompanionBuy_UsesOneOfTheOwners(t *testing.T) {
	s := limitFixture(t)
	s.companionBuys(t)
	require.Equal(t, 1, countWafers(&s.her.Character), `she bought one`)
	assert.Len(t, s.owner.Character.Timers, 1, `it counts on the owner`)
	assert.Empty(t, s.her.Character.Timers, `and not on her`)

	require.True(t, s.ownerBuys().Success)
	require.True(t, s.ownerBuys().Success)
	res := s.ownerBuys()
	assert.Equal(t, actions.BuyReasonLimit, res.Reason, `her one and his two make three`)
}

// A mob buyer with no owner keeps its own count, as before.
func TestCompanionBuy_AMobWithNoOwnerCountsForItself(t *testing.T) {
	s := limitFixture(t)
	s.her.Character.Charmed = nil
	s.companionBuys(t)
	require.Equal(t, 1, countWafers(&s.her.Character))
	assert.Len(t, s.her.Character.Timers, 1)
	assert.Empty(t, s.owner.Character.Timers)
}
