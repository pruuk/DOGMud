package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Playtest 2026-08-19 (u6b flip sweep, BUG-4 / the "attack the darkness"
// family): in The Drill Yard, with a "Training Dummy corpse" on the ground
// and a freshly respawned live Training Dummy present, named attacks
// intermittently failed — sometimes silently, sometimes as "You attack the
// darkness!" — while `look` still listed the mob. These tests pin the
// resolution contract through the full kill → corpse → respawn cycle, and
// reproduce the one deterministic way a room gets stuck in "look lists it,
// nothing can target it": a stale entry in r.players.
//
// The mob-side twin of the stale-id bug was fixed after the 2026-08-08
// "Sala the Mender" incident (see stale_mob_ids_test.go). findPlayerByName
// kept the exact same unguarded dereference: FindByName runs findMobByName
// FIRST (which succeeds), then findPlayerByName panics on the stale user id,
// the panic discards the already-resolved mob id, and the recover() in
// events.invokeListenerSafely swallows the whole command with zero output.
// look/GetRoomDetails never calls findPlayerByName, so the room renders
// normally — the poison is invisible until every named command dies.

const (
	dyMobIdDummy = 20
	dyMobIdVorn  = 21

	dyInstVorn   = 300
	dyInstDummy1 = 301
	dyInstRocky  = 302
	dyInstFleshy = 303
	dyInstDummy2 = 350 // the respawn

	dyRoomId = 5227
)

func dyMobInstance(instId, mobId int, name string) *mobs.Mob {
	m := &mobs.Mob{
		MobId:      mobs.MobId(mobId),
		InstanceId: instId,
		HomeRoomId: dyRoomId,
		Character: characters.Character{
			Name:      name,
			RoomId:    dyRoomId,
			Health:    50,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
	m.Character.HealthMax.Value = 50
	return m
}

// seedDrillYard builds the pre-kill room: Drillmaster Vorn, one live
// Training Dummy (tracked by SpawnInfo), and two charmed companions.
func seedDrillYard(t *testing.T) (*Room, func()) {
	t.Helper()

	specs := map[int]*mobs.Mob{
		dyMobIdDummy: {MobId: dyMobIdDummy, Zone: "Pothole Coulee", Character: characters.Character{Name: "Training Dummy"}},
		dyMobIdVorn:  {MobId: dyMobIdVorn, Zone: "Pothole Coulee", Character: characters.Character{Name: "Drillmaster Vorn"}},
	}
	instances := map[int]*mobs.Mob{
		dyInstVorn:   dyMobInstance(dyInstVorn, dyMobIdVorn, "Drillmaster Vorn"),
		dyInstDummy1: dyMobInstance(dyInstDummy1, dyMobIdDummy, "Training Dummy"),
		dyInstRocky:  dyMobInstance(dyInstRocky, 22, "Rocky the Flesh Golem"),
		dyInstFleshy: dyMobInstance(dyInstFleshy, 22, "Fleshy the Flesh Golem"),
	}
	instances[dyInstRocky].Character.Charm(1, 100, ``)
	instances[dyInstFleshy].Character.Charm(1, 100, ``)
	cleanupMobs := mobs.SeedMobsForTest(specs, instances)

	u1 := users.NewTestUser(1, "tester", "Tester", 9001)
	u1.Character.RoomId = dyRoomId
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: u1})

	room := &Room{
		RoomId: dyRoomId,
		Zone:   "Pothole Coulee",
		Title:  "The Drill Yard",
		SpawnInfo: []SpawnInfo{
			{MobId: dyMobIdDummy, InstanceId: dyInstDummy1, RespawnRate: "1 round"},
		},
	}
	cleanupRooms := SeedRoomsForTest(
		map[int]*Room{dyRoomId: room},
		map[string]*ZoneConfig{"Pothole Coulee": {
			Name:    "Pothole Coulee",
			RoomId:  dyRoomId,
			RoomIds: map[int]struct{}{dyRoomId: {}},
		}},
	)

	room.AddPlayer(1)
	room.mobs = []int{dyInstVorn, dyInstDummy1, dyInstRocky, dyInstFleshy}

	return room, func() {
		cleanupRooms()
		cleanupUsers()
		cleanupMobs()
	}
}

// dyKillDummy mirrors hooks.scheduleMobDespawnFromLife exactly: corpse first,
// then DestroyInstance, CleanupMobSpawns on the home room, RemoveMob.
func dyKillDummy(room *Room) {
	room.Items = append(room.Items, items.Item{ItemId: 999}) // the "Training Dummy corpse"
	mobs.DestroyInstance(dyInstDummy1)
	room.CleanupMobSpawns(false)
	room.RemoveMob(dyInstDummy1)
}

// dyRespawnDummy mirrors the state Prepare() leaves after NewMobById +
// listMobInRoom: a brand-new instance id, template name, RoomId/HomeRoomId
// set, appended to r.mobs, SpawnInfo re-pointed.
func dyRespawnDummy(room *Room) {
	mobs.SetInstanceForTest(dyInstDummy2, dyMobInstance(dyInstDummy2, dyMobIdDummy, "Training Dummy"))
	room.mobs = append(room.mobs, dyInstDummy2)
	room.SpawnInfo[0].InstanceId = dyInstDummy2
	room.SpawnInfo[0].DespawnedRound = 0
}

// The headline fact pattern: kill, corpse on the ground, respawn while the
// player never left. Multi-word, single-word, and second-word resolution must
// all find the respawned instance.
func TestFindByName_RespawnAfterCorpse_MultiWordResolves(t *testing.T) {
	room, cleanup := seedDrillYard(t)
	defer cleanup()

	// Pre-kill sanity.
	if _, mobId := room.FindByName("training dummy"); mobId != dyInstDummy1 {
		t.Fatalf(`pre-kill FindByName("training dummy") = %d, want %d`, mobId, dyInstDummy1)
	}

	dyKillDummy(room)

	// Dead window: no dummy resolves, corpse present.
	if _, mobId := room.FindByName("training dummy"); mobId != 0 {
		t.Fatalf(`dead-window FindByName("training dummy") = %d, want 0`, mobId)
	}

	dyRespawnDummy(room)

	for _, search := range []string{"training dummy", "training", "dummy", "Training Dummy"} {
		if _, mobId := room.FindByName(search); mobId != dyInstDummy2 {
			t.Errorf(`post-respawn FindByName(%q) = %d, want the respawned instance %d`, search, mobId, dyInstDummy2)
		}
	}

	// look-parity: the respawned dummy renders with no duplicate index.
	if idx := room.GetMobDuplicateIndex(dyInstDummy2); idx != 0 {
		t.Errorf("GetMobDuplicateIndex(respawn) = %d, want 0 (name is unique)", idx)
	}
}

// Duplicate-index cell: TWO live dummies plus the corpse. Plain name takes
// the first, 2.name and name#2 take the second — matching the per-name
// duplicate index look displays.
func TestFindByName_RespawnAfterCorpse_DuplicateIndexCell(t *testing.T) {
	room, cleanup := seedDrillYard(t)
	defer cleanup()

	dyKillDummy(room)
	dyRespawnDummy(room)

	// A second live dummy (e.g. a second spawn slot).
	const dummy3 = 351
	mobs.SetInstanceForTest(dummy3, dyMobInstance(dummy3, dyMobIdDummy, "Training Dummy"))
	defer mobs.SetInstanceForTest(dummy3, nil)
	room.mobs = append(room.mobs, dummy3)

	if _, mobId := room.FindByName("training dummy"); mobId != dyInstDummy2 {
		t.Errorf(`FindByName("training dummy") = %d, want first dummy %d`, mobId, dyInstDummy2)
	}
	if _, mobId := room.FindByName("2.training dummy"); mobId != dummy3 {
		t.Errorf(`FindByName("2.training dummy") = %d, want second dummy %d`, mobId, dummy3)
	}
	if _, mobId := room.FindByName("training dummy#2"); mobId != dummy3 {
		t.Errorf(`FindByName("training dummy#2") = %d, want second dummy %d`, mobId, dummy3)
	}
}

// THE deterministic poison: a stale user id in r.players. findMobByName has
// guarded this class since 2026-08-08; findPlayerByName had not. FindByName
// must survive and still return the mob it already resolved.
func TestFindByName_StalePlayerId_DoesNotEatMobResolution(t *testing.T) {
	room, cleanup := seedDrillYard(t)
	defer cleanup()

	dyKillDummy(room)
	dyRespawnDummy(room)

	// A user id with no user record behind it.
	room.AddPlayer(999)

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("FindByName panicked on a stale player id (this panic is swallowed by the event loop in production, killing the command silently): %v", rec)
		}
	}()

	playerId, mobId := room.FindByName("training dummy")
	if mobId != dyInstDummy2 {
		t.Errorf(`FindByName("training dummy") mob = %d, want %d`, mobId, dyInstDummy2)
	}
	if playerId != 0 {
		t.Errorf(`FindByName("training dummy") player = %d, want 0`, playerId)
	}

	// Player-name resolution must also survive the stale id and still find
	// the real player listed after it.
	room.RemovePlayer(1)
	room.AddPlayer(1) // stale id 999 now sits BEFORE the live player
	if uid, err := room.findPlayerByName("tester"); err != nil || uid != 1 {
		t.Errorf(`findPlayerByName("tester") = (%d, %v), want (1, nil)`, uid, err)
	}
}
