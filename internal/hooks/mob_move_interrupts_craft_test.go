package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #512, owner ruling 2026-10-13: a mob's move cancels its craft, as a
// player's does. The stop line is told to the room the mob is leaving, never
// the room it walks into, and the mob itself is told nothing.

const (
	mobMoveDestRoomId   = 7363
	mobMoveDestWatcher  = 7364
	mobMoveOwnerId      = 7365
	mobMoveCrafterMobId = 98362
)

// mobMoveCraftScene builds on craftEndedScene (room 7360 with the watcher
// Tob) and adds a second lit room with its own watcher. linked joins the two
// rooms by an east/west exit pair; unlinked leaves them apart, as a forced
// `go <roomId>` needs. The crafting mob stands in room 7360, mid craft.
func mobMoveCraftScene(t *testing.T, linked bool) *mobs.Mob {
	t.Helper()
	craftEndedScene(t)
	drainEventQueueOnCleanup(t)
	t.Cleanup(keywords.SeedKeywordsForTest())
	from := rooms.LoadRoom(craftEndedRoomId)
	dest := &rooms.Room{RoomId: mobMoveDestRoomId, Biome: "city"}
	if linked {
		from.Exits = map[string]exit.RoomExit{"east": {RoomId: mobMoveDestRoomId}}
		dest.Exits = map[string]exit.RoomExit{"west": {RoomId: craftEndedRoomId}}
	}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		craftEndedRoomId:  from,
		mobMoveDestRoomId: dest,
	}, map[string]*rooms.ZoneConfig{}))

	watcher := users.NewTestUser(mobMoveDestWatcher, "ivo", "Ivo", 97364)
	watcher.Character.RoomId = mobMoveDestRoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		craftEndedCrafterId: users.GetByUserId(craftEndedCrafterId),
		craftEndedWatcherId: users.GetByUserId(craftEndedWatcherId),
		mobMoveDestWatcher:  watcher,
	}))
	dest.AddPlayer(mobMoveDestWatcher)

	m := &mobs.Mob{InstanceId: mobMoveCrafterMobId}
	m.Character.Name = "Smith"
	m.Character.MobInstanceId = mobMoveCrafterMobId
	m.Character.RoomId = craftEndedRoomId
	m.Character.Validate() // fires OnCharacterCreated, wiring the hook
	m.Character.Health = 10
	m.Character.HealthMax.Value = 10
	m.Character.Stamina = 50
	m.Character.StaminaMax.Value = 50
	mobs.SetInstanceForTest(mobMoveCrafterMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(mobMoveCrafterMobId, nil) })
	from.AddMob(mobMoveCrafterMobId)

	if err := m.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	); err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	for _, id := range []int{craftEndedCrafterId, craftEndedWatcherId, mobMoveDestWatcher} {
		events.DrainQueuedMessagesForTest(id)
	}
	return m
}

// assertStopToldToTheRoomLeft checks the mob moved, stopped crafting, and
// that only the watcher in the room it left read the stop line.
func assertStopToldToTheRoomLeft(t *testing.T, m *mobs.Mob) {
	t.Helper()
	if m.Character.RoomId != mobMoveDestRoomId {
		t.Fatalf("fixture: mob is in room %d, want %d (it did not move)", m.Character.RoomId, mobMoveDestRoomId)
	}
	if !m.Character.Activity.IsFree() {
		t.Errorf("Activity = %v after the move, want Free", m.Character.Activity.State())
	}
	left := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
	if !containsLine(left, "Smith stops forging.") {
		t.Errorf("watcher in the room left read %q, want the stop line %q", left, "Smith stops forging.")
	}
	entered := craftEndedPlain(events.DrainQueuedMessagesForTest(mobMoveDestWatcher))
	if containsLine(entered, "Smith stops forging.") {
		t.Errorf("watcher in the room entered read the stop line: %q", entered)
	}
}

// Walking and a successful flee both end in actions.RelocateMob.
func TestMobMove_RelocateStopsTheCraftInTheRoomLeft(t *testing.T) {
	m := mobMoveCraftScene(t, true)
	actions.RelocateMob(m, rooms.LoadRoom(craftEndedRoomId), "east", rooms.LoadRoom(mobMoveDestRoomId), false)
	assertStopToldToTheRoomLeft(t, m)
}

// A walk through an exit name: wander, patrol, pathto steps and a party
// leader's pull all issue this command.
func TestMobMove_GoThroughAnExitStopsTheCraft(t *testing.T) {
	m := mobMoveCraftScene(t, true)
	if _, err := mobcommands.Go("east", m, rooms.LoadRoom(craftEndedRoomId)); err != nil {
		t.Fatalf("go east: %v", err)
	}
	assertStopToldToTheRoomLeft(t, m)
}

// callforhelp's `go <roomId>` to a room with no exit between: the mob runs
// off of its own accord.
func TestMobMove_GoToARoomIdStopsTheCraft(t *testing.T) {
	m := mobMoveCraftScene(t, false)
	if _, err := mobcommands.Go("7363", m, rooms.LoadRoom(craftEndedRoomId)); err != nil {
		t.Fatalf("go 7363: %v", err)
	}
	assertStopToldToTheRoomLeft(t, m)
}

// A companion following its owner moves of its own will, as a player
// following a party leader does through go.
func TestMobMove_CompanionFollowingStopsTheCraft(t *testing.T) {
	m := mobMoveCraftScene(t, true)
	owner := users.NewTestUser(mobMoveOwnerId, "own", "Owner", 97365)
	owner.Character.RoomId = mobMoveDestRoomId
	owner.Character.Companions = []characters.CompanionInfo{{InstanceId: mobMoveCrafterMobId, Name: "Smith"}}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		craftEndedCrafterId: users.GetByUserId(craftEndedCrafterId),
		craftEndedWatcherId: users.GetByUserId(craftEndedWatcherId),
		mobMoveDestWatcher:  users.GetByUserId(mobMoveDestWatcher),
		mobMoveOwnerId:      owner,
	}))

	TransportCompanions(owner, craftEndedRoomId, mobMoveDestRoomId)
	assertStopToldToTheRoomLeft(t, m)
}

// A casting mob that walks keeps casting: movement interrupts only work.
func TestMobMove_ACastingMobWalksOnCasting(t *testing.T) {
	m := mobMoveCraftScene(t, true)
	if err := m.Character.Activity.TransitionToFree(state.TransitionReason{Trigger: activity.TriggerCraftComplete}); err != nil {
		t.Fatalf("fixture: free: %v", err)
	}
	if err := m.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "sparks", FoldsNeeded: 3},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	); err != nil {
		t.Fatalf("fixture: begin cast: %v", err)
	}
	actions.RelocateMob(m, rooms.LoadRoom(craftEndedRoomId), "east", rooms.LoadRoom(mobMoveDestRoomId), false)
	if !m.Character.Activity.IsCasting() {
		t.Errorf("Activity = %v after the walk, want Casting", m.Character.Activity.State())
	}
}
