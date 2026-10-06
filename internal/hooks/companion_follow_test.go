package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

// seedFollowRegistries seeds the minimum state needed for TransportCompanions
// tests: two rooms (1 and 2) and one mob instance (200) in room 1.
// Callers that need a companion-bearing user should call users.NewTestUser
// and users.SeedUsersForTest separately (or use the combined seed below).
func seedFollowRegistries(t *testing.T) func() {
	t.Helper()

	mobSpec := &mobs.Mob{
		MobId: 10,
		Zone:  "TestZone",
		Character: characters.Character{
			Name: "Wolf",
		},
	}
	mobInst := &mobs.Mob{
		MobId:      10,
		InstanceId: 200,
		HomeRoomId: 1,
		Character: characters.Character{
			Name:       "Wolf",
			RoomId:     1,
			Health:     40,
			Conditions: conditions.New(),
			Cooldowns:  map[string]int{},
		},
	}
	mobInst.Character.HealthMax.Value = 40
	mobInst.Character.StaminaMax.Value = 20
	mobInst.Character.Stamina = 20
	mobInst.Character.ConvictionMax.Value = 10
	mobInst.Character.Conviction = 10
	mobInst.Character.Stats.Strength.ValueAdj = 80
	mobInst.Character.Stats.Dexterity.ValueAdj = 80

	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{10: mobSpec},
		map[int]*mobs.Mob{200: mobInst},
	)

	room1 := &rooms.Room{
		RoomId:      1,
		Zone:        "TestZone",
		Title:       "Old Room",
		Description: "Where the companion starts.",
		Exits:       map[string]exit.RoomExit{"east": {RoomId: 2}},
		Biome:       "default",
	}
	room2 := &rooms.Room{
		RoomId:      2,
		Zone:        "TestZone",
		Title:       "New Room",
		Description: "Where the owner went.",
		Exits:       map[string]exit.RoomExit{"west": {RoomId: 1}},
		Biome:       "default",
	}
	cleanupRooms := rooms.SeedRoomsForTest(
		map[int]*rooms.Room{1: room1, 2: room2},
		map[string]*rooms.ZoneConfig{
			"TestZone": {
				Name:    "TestZone",
				RoomId:  1,
				RoomIds: map[int]struct{}{1: {}, 2: {}},
			},
		},
	)
	room1.AddMob(200)
	rooms.MarkRoomOccupancy(1, 0, 1)

	cleanupBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {
			BiomeId:      "default",
			Name:         "Default",
			Symbol:       ".",
			MovementCost: 1.0,
		},
	})

	return func() {
		cleanupBiomes()
		cleanupRooms()
		cleanupMobs()
	}
}

// newOwnerWithCompanion builds a UserRecord whose companion list includes a
// single entry pointing at mob instance 200.
func newOwnerWithCompanion(userId int, roomId int) *users.UserRecord {
	u := users.NewTestUser(userId, "owner", "Owner", 9000)
	u.Character.RoomId = roomId
	u.Character.Companions = []characters.CompanionInfo{
		{InstanceId: 200, Name: "Wolf", MobId: 10},
	}
	return u
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestTransportCompanions_NilOwner verifies that a nil owner does not panic.
func TestTransportCompanions_NilOwner(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	assert.NotPanics(t, func() {
		TransportCompanions(nil, 1, 2)
	})
}

// TestTransportCompanions_SameRoom verifies that no move occurs when
// oldRoomId == newRoomId.
func TestTransportCompanions_SameRoom(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	owner := newOwnerWithCompanion(1, 1)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: owner})
	defer cleanupUsers()

	TransportCompanions(owner, 1, 1)

	mob := mobs.GetInstance(200)
	require.NotNil(t, mob)
	assert.Equal(t, 1, mob.Character.RoomId,
		"companion RoomId must be unchanged when old==new")
}

// TestTransportCompanions_AlreadyInDest verifies that a companion already in
// the destination room is not moved a second time.
func TestTransportCompanions_AlreadyInDest(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	owner := newOwnerWithCompanion(1, 2)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: owner})
	defer cleanupUsers()

	// Manually put companion in room 2 already.
	mob := mobs.GetInstance(200)
	require.NotNil(t, mob)
	mob.Character.RoomId = 2

	TransportCompanions(owner, 1, 2)

	assert.Equal(t, 2, mob.Character.RoomId,
		"companion RoomId must stay at 2 — no double-move")
}

// TestTransportCompanions_CastInterrupt verifies that an in-progress cast is
// aborted and the companion is moved to the destination room.
func TestTransportCompanions_CastInterrupt(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	owner := newOwnerWithCompanion(1, 2)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: owner})
	defer cleanupUsers()

	mob := mobs.GetInstance(200)
	require.NotNil(t, mob)
	mob.Character.Activity = activity.NewMachine()
	_ = mob.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "sparks", FoldsNeeded: 3},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	)

	TransportCompanions(owner, 1, 2)

	assert.True(t, mob.Character.Activity == nil || mob.Character.Activity.IsFree(),
		"Activity must be Free (cast cleared) on move")
	assert.Equal(t, 2, mob.Character.RoomId,
		"companion must be in destination room after cast-interrupt")
}

// TestTransportCompanions_StaleCompanionEntry verifies that a companion whose
// mob instance no longer exists is silently skipped without panicking.
func TestTransportCompanions_StaleCompanionEntry(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	// Build owner with a stale instance id (999 does not exist in the registry).
	u := users.NewTestUser(1, "owner", "Owner", 9001)
	u.Character.RoomId = 2
	u.Character.Companions = []characters.CompanionInfo{
		{InstanceId: 999, Name: "Ghost", MobId: 10},
	}
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: u})
	defer cleanupUsers()

	assert.NotPanics(t, func() {
		TransportCompanions(u, 1, 2)
	})
}

// TestTransportCompanions_MovesCompanionAndClearsAggro verifies end-to-end
// transport: companion moves from room 1 to room 2 and aggro targeting a mob
// that stays behind is cleared.
func TestTransportCompanions_MovesCompanionAndClearsAggro(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	owner := newOwnerWithCompanion(1, 2)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: owner})
	defer cleanupUsers()

	mob := mobs.GetInstance(200)
	require.NotNil(t, mob)

	// Companion is aggro to mob instance 99, which is NOT in room 2.
	mob.Character.SetAggro(0, 99, characters.DefaultAttack)

	TransportCompanions(owner, 1, 2)

	assert.Equal(t, 2, mob.Character.RoomId,
		"companion must be in destination room")
	assert.False(t, mob.Character.IsInCombat(),
		"aggro must be cleared because target (99) is not in dest room")
}

// #414: the "follows" line is visual and names two creatures; a watcher who
// makes out only shapes must not read either name, and one who sees clearly
// still reads both.
func TestTransportCompanions_FollowLineHidesNamesFromAWatcherInTheDim(t *testing.T) {
	for _, c := range []struct {
		name  string
		lamp  int
		names bool
	}{
		{"lit room", 60, true},
		{"dim room", 38, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			cleanup := seedFollowRegistries(t)
			defer cleanup()
			old := rooms.LoadRoom(1)
			old.SkyLight = rooms.SkyLightPtr(0)
			old.Lamp = rooms.LampPtr(c.lamp)

			owner := newOwnerWithCompanion(1, 2)
			watcher := users.NewTestUser(2, "watcher", "Watcher", 9001)
			watcher.Character.RoomId = 1
			defer users.SeedUsersForTest(map[int]*users.UserRecord{1: owner, 2: watcher})()
			old.AddPlayer(2)
			_ = events.DrainQueuedMessagesForTest(2)

			TransportCompanions(owner, 1, 2)

			got := strings.Join(events.DrainQueuedMessagesForTest(2), "")
			if !strings.Contains(got, "follows") {
				t.Fatalf("the watcher read no follow line: %q", got)
			}
			hasNames := strings.Contains(got, "Wolf") && strings.Contains(got, "Owner")
			if hasNames != c.names {
				t.Errorf("names shown = %v, want %v: %q", hasNames, c.names, got)
			}
			if !c.names && (strings.Contains(got, "Wolf") || strings.Contains(got, "Owner")) {
				t.Errorf("a watcher who sees only shapes read a name: %q", got)
			}
		})
	}
}

// ─── PushCompanionsToRoom tests ───────────────────────────────────────────────

// TestPushCompanionsToRoom_NilOwner verifies that a nil owner does not panic.
func TestPushCompanionsToRoom_NilOwner(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	assert.NotPanics(t, func() {
		PushCompanionsToRoom(nil, 2)
	})
}

// TestPushCompanionsToRoom_MovesCompanionGearIntact is the load-bearing
// gear-safety test: a companion carrying equipped gear + backpack items is
// pushed to a new room. The companion mob instance must still EXIST
// (not destroyed) and its Equipment/Items must be byte-for-byte unchanged,
// just relocated to the new room.
func TestPushCompanionsToRoom_MovesCompanionGearIntact(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	owner := newOwnerWithCompanion(1, 1)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: owner})
	defer cleanupUsers()

	mob := mobs.GetInstance(200)
	require.NotNil(t, mob)

	// Give the companion gear-safety-relevant state: equipped weapon +
	// backpack item.
	mob.Character.Equipment.Weapon = items.Item{ItemId: 555, Uses: 3}
	mob.Character.Items = []items.Item{{ItemId: 777, Uses: 1}}

	PushCompanionsToRoom(owner, 2)

	// Companion instance must still exist post-move (relocated, not
	// destroyed).
	movedMob := mobs.GetInstance(200)
	require.NotNil(t, movedMob, "companion mob instance must still exist after sweep")

	assert.Equal(t, 2, movedMob.Character.RoomId,
		"companion must now be in the destination room")
	assert.Equal(t, 555, movedMob.Character.Equipment.Weapon.ItemId,
		"equipped weapon must survive the sweep untouched")
	assert.Equal(t, 3, movedMob.Character.Equipment.Weapon.Uses,
		"equipped weapon uses must survive the sweep untouched")
	require.Len(t, movedMob.Character.Items, 1)
	assert.Equal(t, 777, movedMob.Character.Items[0].ItemId,
		"backpack item must survive the sweep untouched")

	// Companion must no longer be tracked in the old room.
	oldRoom := rooms.LoadRoom(1)
	require.NotNil(t, oldRoom)
	assert.NotContains(t, oldRoom.GetMobs(), 200,
		"companion must be removed from the old room's mob list")

	// Companion must be tracked in the new room.
	newRoom := rooms.LoadRoom(2)
	require.NotNil(t, newRoom)
	assert.Contains(t, newRoom.GetMobs(), 200,
		"companion must be added to the destination room's mob list")
}

// TestPushCompanionsToRoom_AlreadyInDest verifies a companion already in the
// destination room is left alone (no duplicate move).
func TestPushCompanionsToRoom_AlreadyInDest(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	owner := newOwnerWithCompanion(1, 2)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: owner})
	defer cleanupUsers()

	mob := mobs.GetInstance(200)
	require.NotNil(t, mob)
	mob.Character.RoomId = 2

	PushCompanionsToRoom(owner, 2)

	assert.Equal(t, 2, mob.Character.RoomId,
		"companion RoomId must stay at 2 — no double-move")
}

// TestPushCompanionsToRoom_CastInterrupt verifies an in-progress companion
// cast is aborted by the sweep (mirrors TransportCompanions).
func TestPushCompanionsToRoom_CastInterrupt(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	owner := newOwnerWithCompanion(1, 1)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: owner})
	defer cleanupUsers()

	mob := mobs.GetInstance(200)
	require.NotNil(t, mob)
	mob.Character.Activity = activity.NewMachine()
	_ = mob.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "sparks", FoldsNeeded: 3},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	)

	PushCompanionsToRoom(owner, 2)

	assert.True(t, mob.Character.Activity == nil || mob.Character.Activity.IsFree(),
		"Activity must be Free (cast cleared) on sweep")
	assert.Equal(t, 2, mob.Character.RoomId,
		"companion must be in destination room after cast-interrupt")
}

// TestPushCompanionsToRoom_StaleCompanionEntry verifies a companion whose mob
// instance no longer exists is silently skipped without panicking.
func TestPushCompanionsToRoom_StaleCompanionEntry(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	u := users.NewTestUser(1, "owner", "Owner", 9002)
	u.Character.RoomId = 1
	u.Character.Companions = []characters.CompanionInfo{
		{InstanceId: 999, Name: "Ghost", MobId: 10},
	}
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: u})
	defer cleanupUsers()

	assert.NotPanics(t, func() {
		PushCompanionsToRoom(u, 2)
	})
}

// TestTransportCompanions_HoldKeepsOnlyTheHeldCompanion verifies that the
// companionai follow hold is asked per companion: the one it holds stays
// behind and every other companion of the same owner still follows.
func TestTransportCompanions_HoldKeepsOnlyTheHeldCompanion(t *testing.T) {
	cleanup := seedFollowRegistries(t)
	defer cleanup()

	other := &mobs.Mob{
		MobId:      10,
		InstanceId: 201,
		HomeRoomId: 1,
		Character: characters.Character{
			Name:       "Hound",
			RoomId:     1,
			Health:     40,
			Conditions: conditions.New(),
			Cooldowns:  map[string]int{},
		},
	}
	other.Character.HealthMax.Value = 40
	mobs.SetInstanceForTest(201, other)
	defer mobs.SetInstanceForTest(201, nil)
	rooms.LoadRoom(1).AddMob(201)

	owner := newOwnerWithCompanion(1, 2)
	owner.Character.Companions = append(owner.Character.Companions,
		characters.CompanionInfo{InstanceId: 201, Name: "Hound", MobId: 10})
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: owner})
	defer cleanupUsers()

	// Hold instance 200 only, the way the aicompanion module holds its
	// bonded companion and nothing else.
	companionai.SetHolder(func(userId int, mobInstanceId int) bool {
		return userId == 1 && mobInstanceId == 200
	})
	defer companionai.SetHolder(nil)

	TransportCompanions(owner, 1, 2)

	assert.Equal(t, 1, mobs.GetInstance(200).Character.RoomId,
		"the held companion must stay where it was")
	assert.Equal(t, 2, other.Character.RoomId,
		"an unheld companion of the same owner must still follow")
}
