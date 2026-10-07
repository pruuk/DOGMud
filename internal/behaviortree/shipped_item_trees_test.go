package behaviortree

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// loadShippedItemWorld points the engine at the shipped world, loads its
// conditions and items, validates every item tree, and pins the shipped day.
// Returns the round midwinter's dusk falls in.
func loadShippedItemWorld(t *testing.T) uint64 {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3 // condition 0 refuses a 0 trigger count
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	// items.LoadDataFiles also replaces the combat and defence message
	// stores; snapshot them so later tests read their own seeds.
	t.Cleanup(items.SeedAttackMessagesForTest(nil))
	t.Cleanup(items.SeedDefenseMessagesForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	for _, name := range []string{"keeper_lantern", "sunstone", "dusk_to_dawn", "rift_pulse"} {
		GetEngine().EvictItemTree(name)
		t.Cleanup(func() { GetEngine().EvictItemTree(name) })
	}
	if err := ValidateItemBehaviors(); err != nil {
		t.Fatalf("the shipped item trees do not validate: %v", err)
	}
	t.Cleanup(ResetItemBTreeStatesForTest())
	t.Cleanup(itemlight.ResetForTest())
	room := newProbeRoom(t)
	room.RoomId = engineProbeRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{engineProbeRoomId: room}, map[string]*rooms.ZoneConfig{}))
	return pinAfterDuskClock(t)
}

// wearShipped puts a shipped light item in user 1's light slot with its
// worn condition applied, as an equip does.
func wearShipped(t *testing.T, itemId int) (*users.UserRecord, ItemSubject) {
	t.Helper()
	u := users.NewTestUser(1, "probe", "Probe", 0)
	u.Character.RoomId = engineProbeRoomId
	it := items.New(itemId)
	u.Character.Equipment.Light = it
	for _, id := range it.GetSpec().WornConditionIds {
		u.Character.Conditions.AddCondition(id, true)
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))
	return u, ItemSubject{UUID: it.UUID, ItemId: itemId, UserId: 1, Slot: "light"}
}

func shippedLightNow(t *testing.T, u *users.UserRecord, conditionId int) (float64, bool) {
	t.Helper()
	recs := u.Character.Conditions.GetConditions(conditionId)
	if len(recs) != 1 {
		t.Fatalf("holder carries %d records of condition %d, want 1", len(recs), conditionId)
	}
	return recs[0].LightNow(conditions.GetConditionSpec(conditionId))
}

// R2, R3: the shared Oil Lantern is fully dark while its holder sleeps, and
// back at full on the first round after waking.
func TestShippedKeeperLanternIsDarkWhileItsHolderSleeps(t *testing.T) {
	loadShippedItemWorld(t)
	u, lantern := wearShipped(t, 40038)
	idle := EventContext{EventType: "item_idle"}

	TryItemBehavior(idle, lantern)
	if v, ok := shippedLightNow(t, u, 125); !ok || v != 52 {
		t.Errorf("awake: the lantern gives %v, %v; want 52", v, ok)
	}
	u.Character.Conditions.AddCondition(15, false) // the shipped Sleeping
	TryItemBehavior(idle, lantern)
	if _, ok := shippedLightNow(t, u, 125); ok || u.Character.EmitsLight() {
		t.Error("asleep: the lantern still gives light")
	}
	u.Character.Conditions.RemoveCondition(15)
	TryItemBehavior(idle, lantern)
	if v, ok := shippedLightNow(t, u, 125); !ok || v != 52 {
		t.Errorf("woken: the lantern gives %v, %v; want 52 again", v, ok)
	}
}

// The sunstone: full by day, a faint 30 for an hour after dusk, dark
// through the night.
func TestShippedSunstoneFollowsTheSun(t *testing.T) {
	dusk := loadShippedItemWorld(t)
	u, stone := wearShipped(t, 20099)
	idle := EventContext{EventType: "item_idle"}
	for _, c := range []struct {
		name  string
		round uint64
		want  float64 // -1: dark
	}{
		{"noon", 355*900 + 450, 46},
		{"the dusk round", dusk, 30},
		{"half an hour after dusk", dusk + 19, 30},
		{"an hour and more after dusk", dusk + 40, -1},
		{"midnight", 356 * 900, -1},
	} {
		util.SetRoundCountForTest(c.round)
		TryItemBehavior(idle, stone)
		v, ok := shippedLightNow(t, u, 134)
		switch {
		case c.want < 0 && ok:
			t.Errorf("%s: the sunstone gives %v, want dark", c.name, v)
		case c.want >= 0 && (!ok || v != c.want):
			t.Errorf("%s: the sunstone gives %v, %v; want %v", c.name, v, ok, c.want)
		}
	}
}

// The arch lantern (55) is lit at 52 while the street lamps burn (period:
// lamplit: at night, and while the clear sky is too dim to read a face, so a
// midwinter 08:00 keeps it lit); the Rift Stone (56) pulses within 20 to 36.
func TestShippedFixtureTrees(t *testing.T) {
	dusk := loadShippedItemWorld(t)
	idle := EventContext{EventType: "item_idle"}
	arch := ItemSubject{UUID: uuid.UUID{0x61}, ItemId: 55, RoomId: engineProbeRoomId, OnFloor: true}
	stone := ItemSubject{UUID: uuid.UUID{0x62}, ItemId: 56, RoomId: engineProbeRoomId, OnFloor: true}

	util.SetRoundCountForTest(355*900 + 450) // noon
	TryItemBehavior(idle, arch)
	if itemlight.Lit(engineProbeRoomId, arch.UUID) {
		t.Error("noon: the arch lantern is lit")
	}
	util.SetRoundCountForTest(dusk)
	TryItemBehavior(idle, arch)
	if v, _ := itemlight.Get(engineProbeRoomId, arch.UUID); v != 52 {
		t.Errorf("dusk: the arch lantern reads %v, want 52", v)
	}
	util.SetRoundCountForTest(355*900 + 300) // 08:00, day but a dim sky
	TryItemBehavior(idle, arch)
	if v, _ := itemlight.Get(engineProbeRoomId, arch.UUID); v != 52 {
		t.Errorf("midwinter 08:00: the arch lantern reads %v, want 52 with the street lamps", v)
	}
	for r := uint64(0); r < 24; r++ {
		util.SetRoundCountForTest(dusk + r)
		TryItemBehavior(idle, stone)
		v, _ := itemlight.Get(engineProbeRoomId, stone.UUID)
		if v < 20 || v > 36 || math.IsInf(v, -1) {
			t.Errorf("round %d: the Rift Stone reads %v, want within 20 to 36", dusk+r, v)
		}
	}
}
