package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The proc parity record (item behaviour slice 3, spec "Slice 3" gates).
// testdata/item_proc_parity.golden was recorded on the Pinnacle proc path
// (dispatchItemProcs) BEFORE the procs moved into item trees, and is
// frozen: the tree path must reproduce it, the same outcomes on the same
// rounds, under one seeded random source (util.SetRandForTest). One
// scenario per shipped proc item and trigger, hits and misses, cooldown
// windows and rounds where the effect has nothing to act on, plus a kill
// scenario with every proc item worn. After each round a probe draw is
// recorded, so a path that draws more or fewer numbers moves the record.
//
// Set DOGMUD_RECORD_PROC_PARITY=1 to rewrite the record. It was written
// once, on the Pinnacle path; do not rewrite it on the tree path.

const (
	procParityUserId = 1
	procParityRoomId = 9972
	procParityFirst  = 2000 // the first scenario round
	procParityRounds = 60
	procParitySeed   = 7331
	procParityMobA   = 9801 // the bearer's foe
	procParityMobB   = 9802 // a second hostile in the room (aoe_stun)
)

// procDispatch fires a trigger's procs for owner against other: the
// dispatcher under test.
type procDispatch func(trigger string, owner, other *characters.Character, room *rooms.Room, damage int)

// procParityPaths are the dispatchers the record is checked against.
var procParityPaths = map[string]procDispatch{
	"pinnacle": dispatchItemProcs,
}

// loadProcParityWorld loads the shipped conditions and items, points the
// engine at the shipped item trees, enables procs and seeds one room.
func loadProcParityWorld(t *testing.T) *rooms.Room {
	t.Helper()
	// The overlay first: AddOverlayOverrides rebuilds the live config, so
	// it would drop a data path set before it.
	setItemProcsEnabled(t, true)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3 // condition 0 refuses a 0 trigger count
	configs.SetConfigForTest(t, cfg)
	// items.LoadDataFiles also replaces the combat and defence message
	// stores; snapshot them too.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(items.SeedAttackMessagesForTest(nil))
	t.Cleanup(items.SeedDefenseMessagesForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	for id := range procParityItems {
		if spec := items.GetItemSpec(id); spec != nil && spec.Behavior != `` {
			name := spec.Behavior
			behaviortree.GetEngine().EvictItemTree(name)
			t.Cleanup(func() { behaviortree.GetEngine().EvictItemTree(name) })
		}
	}

	room := rooms.NewRoom("procparity")
	room.RoomId = procParityRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{procParityRoomId: room}, map[string]*rooms.ZoneConfig{}))
	return room
}

// setItemProcsEnabled sets ItemProcsEnabled for the test and puts it back.
func setItemProcsEnabled(t *testing.T, on bool) {
	t.Helper()
	prev := bool(configs.GetConfig().GamePlay.ItemProcsEnabled)
	if err := configs.AddOverlayOverrides(map[string]any{"GamePlay.ItemProcsEnabled": on}); err != nil {
		t.Fatalf("failed to set ItemProcsEnabled=%v: %v", on, err)
	}
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"GamePlay.ItemProcsEnabled": prev})
	})
}

// procParityItems are the shipped proc items and the slot each is worn in.
var procParityItems = map[int]string{
	40183: "weapon",  // The Blackrazor: on_hit lifesteal
	40185: "offhand", // Aegis of Mockery: on_block aoe_stun
	40186: "body",    // Thornwall Harness: on_grapple bleed
	40189: "weapon",  // Staff of the Hollow Choir: on_spell_hit steal_pool
}

// procParityBearer seeds a fresh user 1 in room wearing a fresh instance of
// each of itemIds, with fresh item state.
func procParityBearer(t *testing.T, room *rooms.Room, itemIds ...int) *users.UserRecord {
	t.Helper()
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	t.Cleanup(behaviortree.ResetItemListenerCapsForTest())
	u := users.NewTestUser(procParityUserId, "bearer", "Bearer", 0)
	u.Character.RoomId = room.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{procParityUserId: u}))
	room.AddPlayer(procParityUserId)
	for _, id := range itemIds {
		it := items.New(id)
		switch procParityItems[id] {
		case "weapon":
			u.Character.Equipment.Weapon = it
		case "offhand":
			u.Character.Equipment.Offhand = it
		case "body":
			u.Character.Equipment.Body = it
		default:
			t.Fatalf("procParityBearer: item %d has no slot", id)
		}
	}
	return u
}

// procParityMob registers a hostile mob instance in room.
func procParityMob(t *testing.T, room *rooms.Room, instanceId int) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{
		InstanceId: instanceId,
		HomeRoomId: room.RoomId,
		Character: characters.Character{
			Name:       "Parity Beast",
			RoomId:     room.RoomId,
			Conditions: conditions.New(),
			Cooldowns:  map[string]int{},
		},
	}
	m.Character.HealthMax.Value = 500
	m.Character.Health = 500
	m.Character.ConvictionMax.Value = 100
	m.Character.Conviction = 30
	mobs.SetInstanceForTest(instanceId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instanceId, nil) })
	room.AddMob(instanceId)
	return m
}

// procParityState is one round's observable outcome.
func procParityState(owner *characters.Character, foes ...*mobs.Mob) string {
	var b strings.Builder
	fmt.Fprintf(&b, "hp=%d conv=%d", owner.Health, owner.Conviction)
	for _, m := range foes {
		c := &m.Character
		fmt.Fprintf(&b, " | %s hp=%d conv=%d stun=%t", c.Name, c.Health, c.Conviction, c.HasCondition(84))
		for _, rec := range c.GetConditions(conditions.ConditionIdBleeding) {
			fmt.Fprintf(&b, " bleed=%d/%d/%d", len(rec.Stacks), rec.TriggersLeft, rec.TickAmount)
		}
	}
	return b.String()
}

// procParityScenario is one recorded run: run seeds its bearer and foes and
// returns their record, firing each round through fire.
type procParityScenario struct {
	name string
	run  func(t *testing.T, room *rooms.Room, fire procDispatch) string
}

// runProcRounds plays procParityRounds rounds under the seed, calling
// round(i) each round and recording the state after it, then a probe draw.
func runProcRounds(t *testing.T, round func(i int), state func() string) string {
	t.Helper()
	restoreRand := util.SetRandForTest(procParitySeed)
	defer restoreRand()
	defer util.ResetRoundCountForTest()
	var b strings.Builder
	for i := 0; i < procParityRounds; i++ {
		util.SetRoundCountForTest(uint64(procParityFirst + i))
		round(i)
		fmt.Fprintf(&b, "%d: %s probe=%d\n", i, state(), util.Rand(1000))
	}
	return b.String()
}

var procParityScenarios = []procParityScenario{
	{"blackrazor_on_hit", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40183)
		c := u.Character
		c.HealthMax.Value = 1000
		c.Health = 100
		foe := procParityMob(t, room, procParityMobA)
		damage := []int{12, 0, 40, 3, 1}
		return runProcRounds(t, func(i int) {
			fire("on_hit", c, &foe.Character, room, damage[i%len(damage)])
		}, func() string { return procParityState(c, foe) })
	}},
	{"aegis_on_block", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40185)
		c := u.Character
		foe := procParityMob(t, room, procParityMobA)
		other := procParityMob(t, room, procParityMobB)
		return runProcRounds(t, func(i int) {
			// No hostile in the room for rounds 40 to 50: a stun that
			// finds nobody does not burn the cooldown.
			switch i {
			case 40:
				room.RemoveMob(procParityMobA)
				room.RemoveMob(procParityMobB)
			case 51:
				room.AddMob(procParityMobA)
				room.AddMob(procParityMobB)
			}
			// A fresh condition set each round: RemoveCondition only
			// expires a record until the prune pass, so the stun would
			// read as still held.
			foe.Character.Conditions = conditions.New()
			other.Character.Conditions = conditions.New()
			fire("on_block", c, &foe.Character, room, 5)
		}, func() string { return procParityState(c, foe, other) })
	}},
	{"harness_on_grapple", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40186)
		c := u.Character
		foe := procParityMob(t, room, procParityMobA)
		return runProcRounds(t, func(i int) {
			// Both sides of the hold, as Position_GrappleTick fires them;
			// the foe wears no harness.
			if i%2 == 0 {
				fire("on_grapple", c, &foe.Character, nil, 0)
				fire("on_grapple", &foe.Character, c, nil, 0)
			} else {
				fire("on_grapple", &foe.Character, c, nil, 0)
				fire("on_grapple", c, &foe.Character, nil, 0)
			}
		}, func() string { return procParityState(c, foe) })
	}},
	{"staff_on_spell_hit", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40189)
		c := u.Character
		c.ConvictionMax.Value = 1000
		c.Conviction = 0
		foe := procParityMob(t, room, procParityMobA)
		return runProcRounds(t, func(i int) {
			// The foe runs dry, then is refilled at round 40.
			if i == 40 {
				foe.Character.Conviction = 30
			}
			fire("on_spell_hit", c, &foe.Character, nil, 20)
		}, func() string { return procParityState(c, foe) })
	}},
	{"all_on_kill", func(t *testing.T, room *rooms.Room, fire procDispatch) string {
		u := procParityBearer(t, room, 40183, 40185, 40186)
		c := u.Character
		c.HealthMax.Value = 1000
		c.Health = 100
		foe := procParityMob(t, room, procParityMobA)
		return runProcRounds(t, func(i int) {
			if i%3 == 0 {
				MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{u.UserId: 1}})
			}
		}, func() string { return procParityState(c, foe) })
	}},
}

// runProcParity plays every scenario on one dispatcher and returns the
// whole record.
func runProcParity(t *testing.T, fire procDispatch) string {
	t.Helper()
	var b strings.Builder
	for _, s := range procParityScenarios {
		var out string
		t.Run(s.name, func(t *testing.T) {
			room := loadProcParityWorld(t)
			out = s.run(t, room, fire)
		})
		fmt.Fprintf(&b, "## %s\n%s", s.name, out)
	}
	return b.String()
}

func TestItemProcParity(t *testing.T) {
	path := filepath.Join("testdata", "item_proc_parity.golden")
	if os.Getenv("DOGMUD_RECORD_PROC_PARITY") == "1" {
		if err := os.WriteFile(path, []byte(runProcParity(t, dispatchItemProcs)), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skip("recorded " + path)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, fire := range procParityPaths {
		if got := runProcParity(t, fire); got != string(want) {
			t.Errorf("%s path moved off the record.\n--- want\n%s\n--- got\n%s", name, want, got)
		}
	}
}
