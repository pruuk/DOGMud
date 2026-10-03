package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An owned chunk hands out ids one room at a time, frees single rooms, is
// never touched by the generic ephemeral cleanup, and is free again once
// released.
func TestOwnedChunk_Lifecycle(t *testing.T) {
	restore := SeedRoomsForTest(map[int]*Room{}, map[string]*ZoneConfig{})
	defer restore()

	c, err := ReserveOwnedChunk(`Rift Test`)
	require.NoError(t, err)
	defer c.Release()

	assert.False(t, chunkFree(c.Id()), `a reserved chunk must not look free`)
	assert.True(t, GetPlaneRegistry().IsNonEuclidean(c.Plane()))

	a := &Room{Zone: `Rift Test`, Title: `A`, Exits: map[string]exit.RoomExit{}}
	b := &Room{Zone: `Rift Test`, Title: `B`, Exits: map[string]exit.RoomExit{}}
	idA, err := c.AddRoom(a)
	require.NoError(t, err)
	idB, err := c.AddRoom(b)
	require.NoError(t, err)
	assert.True(t, IsEphemeralRoomId(idA))
	assert.Equal(t, idA+1, idB)
	assert.Equal(t, c.Plane(), a.Plane)
	assert.Same(t, a, LoadRoom(idA))
	assert.Equal(t, 2, c.RoomCount())

	// The generic cleanup leaves an owned chunk alone, even with nobody in it.
	assert.Empty(t, TryEphemeralCleanup(idA))
	assert.NotNil(t, LoadRoom(idA))

	// A room with a player in it cannot be removed.
	a.players = []int{7}
	assert.False(t, c.RemoveRoom(idA))
	a.players = nil

	assert.True(t, c.RemoveRoom(idA))
	assert.Nil(t, getRoomFromMemory(idA))
	assert.Equal(t, 1, c.RoomCount())

	// A freed slot is reused.
	idC, err := c.AddRoom(&Room{Zone: `Rift Test`, Title: `C`})
	require.NoError(t, err)
	assert.Equal(t, idA, idC)

	assert.Zero(t, c.Release())
	assert.Nil(t, getRoomFromMemory(idB))
	assert.True(t, chunkFree(c.Id()))
	_, err = c.AddRoom(&Room{Zone: `Rift Test`})
	assert.Error(t, err, `a released chunk takes no more rooms`)
}

// Added routers and guards run after the primary ones; the first router to
// handle an exit decides, and the first guard to refuse decides.
func TestRoutingHooks_AddedHooksChain(t *testing.T) {
	SetExitRouter(func(userId, fromRoomId int, exitName string) (ExitRoute, bool) {
		if exitName == `door` {
			return ExitRoute{RoomId: 1}, true
		}
		return ExitRoute{}, false
	})
	SetEntryGuard(nil)
	defer SetExitRouter(nil)
	defer ClearAddedRoutingHooks()

	AddExitRouter(func(userId, fromRoomId int, exitName string) (ExitRoute, bool) {
		if exitName == `door` || exitName == `arch` {
			return ExitRoute{RoomId: 2, PickRefusal: `no picking`}, true
		}
		return ExitRoute{}, false
	})
	AddEntryGuard(func(userId, toRoomId int) (bool, int, string) {
		if toRoomId == 50 {
			return false, 3, `closed`
		}
		return true, 0, ``
	})

	r, ok := RouteExit(1, 10, `door`)
	assert.True(t, ok)
	assert.Equal(t, 1, r.RoomId, `the primary router decides first`)

	r, ok = RouteExit(1, 10, `arch`)
	assert.True(t, ok)
	assert.Equal(t, 2, r.RoomId)
	assert.Equal(t, `no picking`, r.PickRefusal)

	_, ok = RouteExit(1, 10, `west`)
	assert.False(t, ok)

	allowed, redirect, why := checkEntry(1, 50)
	assert.False(t, allowed)
	assert.Equal(t, 3, redirect)
	assert.Equal(t, `closed`, why)
	allowed, _, _ = checkEntry(1, 51)
	assert.True(t, allowed)
}

// GetRandomExitFor skips routed exits the router refuses and reports the
// routed destination for the ones it allows.
func TestGetRandomExitFor_RespectsRouters(t *testing.T) {
	defer ClearAddedRoutingHooks()
	AddExitRouter(func(userId, fromRoomId int, exitName string) (ExitRoute, bool) {
		switch exitName {
		case `sealed`:
			return ExitRoute{Refusal: `sealed`}, true
		case `open`:
			return ExitRoute{RoomId: 77}, true
		}
		return ExitRoute{}, false
	})
	r := &Room{RoomId: 10, Exits: map[string]exit.RoomExit{
		`sealed`: {RoomId: 11},
		`open`:   {RoomId: 12},
	}}
	for i := 0; i < 200; i++ {
		name, id := r.GetRandomExitFor(1)
		assert.Equal(t, `open`, name)
		assert.Equal(t, 77, id)
	}
	r.Exits = map[string]exit.RoomExit{`sealed`: {RoomId: 11}}
	name, id := r.GetRandomExitFor(1)
	assert.Empty(t, name)
	assert.Zero(t, id)
}

// A companion left behind in a room that closes goes to its owner; one
// whose owner is gone goes with the room. Nothing is left pointing at a
// freed id.
func TestOwnedChunk_CompanionLeftBehind(t *testing.T) {
	defer SeedRoomsForTest(map[int]*Room{}, map[string]*ZoneConfig{})()
	pet := &mobs.Mob{InstanceId: 301, Character: characters.Character{Name: `pet`}}
	pet.Character.Charmed = characters.NewCharm(5, -1, ``)
	stray := &mobs.Mob{InstanceId: 302, Character: characters.Character{Name: `stray`}}
	stray.Character.Charmed = characters.NewCharm(6, -1, ``)
	defer mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{301: pet, 302: stray})()

	c, err := ReserveOwnedChunk(`Rift Test`)
	require.NoError(t, err)
	defer c.Release()
	left := &Room{Zone: `Rift Test`, Exits: map[string]exit.RoomExit{}}
	safe := &Room{Zone: `Rift Test`, Exits: map[string]exit.RoomExit{}}
	idLeft, _ := c.AddRoom(left)
	idSafe, _ := c.AddRoom(safe)
	left.AddMob(301)
	left.AddMob(302)

	owner := users.NewTestUser(5, `Owner`, `Owner`, 0)
	owner.Character.RoomId = idSafe
	owner.Character.TrackCharmed(301, true)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{5: owner})()

	require.True(t, c.RemoveRoom(idLeft))
	assert.Equal(t, idSafe, pet.Character.RoomId, `the companion went to its owner`)
	assert.Contains(t, safe.GetMobs(), 301)
	assert.Nil(t, mobs.GetInstance(302), `an ownerless companion goes with the room`)
}
