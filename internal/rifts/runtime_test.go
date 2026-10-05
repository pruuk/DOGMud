package rifts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	testOrigin = 100
	testUser   = 1
)

// setupRuntime seeds an overworld room, one online user standing in it, and
// the shipped profiles (structurally validated; world ids are not loaded in a
// unit test). It returns the user and a cleanup.
func setupRuntime(t *testing.T) *users.UserRecord {
	t.Helper()
	loaded, err := loadFrom(shippedDir)
	require.NoError(t, err)
	for _, p := range loaded {
		require.NoError(t, p.validate(false))
	}
	origProfiles := profiles
	profiles = loaded

	// The key and the light, so keys can be given and purged.
	specs := map[int]*items.ItemSpec{}
	for _, p := range loaded {
		specs[p.KeyItemId] = &items.ItemSpec{ItemId: p.KeyItemId, Name: `Facet Key`, Type: items.Object, Subtype: items.Mundane}
		if p.LightItemId != 0 {
			specs[p.LightItemId] = &items.ItemSpec{ItemId: p.LightItemId, Name: `Glowstone`, Type: items.Light, Subtype: items.Wearable}
		}
	}
	restoreItems := items.SeedItemsForTest(specs)

	origin := &rooms.Room{RoomId: testOrigin, Zone: `Overworld`, Title: `Clearing`, Exits: map[string]exit.RoomExit{}}
	restoreRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{testOrigin: origin}, map[string]*rooms.ZoneConfig{})

	u := users.NewTestUser(testUser, `tester`, `Tester`, 0)
	u.Character.RoomId = testOrigin
	origin.AddPlayer(testUser)
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{testUser: u})

	t.Cleanup(func() {
		for _, run := range Runs() {
			for id := range run.Rooms {
				if r := rooms.LoadRoom(id); r != nil {
					for _, uid := range r.GetPlayers() {
						r.RemovePlayer(uid)
					}
				}
			}
			run.Members = map[int]bool{}
			run.PortalOpen = false
			run.end()
		}
		runs, runByRoom = map[int]*Run{}, map[int]*Run{}
		sites, sitesDay = map[int]*Site{}, ``
		profiles = origProfiles
		restoreItems()
		restoreUsers()
		restoreRooms()
	})
	return u
}

// walk moves u from their room to toRoomId the way MoveToRoom would, then
// runs the RoomChange handler.
func walk(t *testing.T, u *users.UserRecord, toRoomId int) {
	t.Helper()
	from := u.Character.RoomId
	if r := rooms.LoadRoom(from); r != nil {
		r.RemovePlayer(u.UserId)
	}
	to := rooms.LoadRoom(toRoomId)
	require.NotNil(t, to, `room %d`, toRoomId)
	to.AddPlayer(u.UserId)
	u.Character.RoomId = toRoomId
	OnRoomChange(u.UserId, from, toRoomId, u.Character.IsHidden())
}

// firstOpenDoor returns a door of rr the router lets u through, with its
// destination.
func firstOpenDoor(t *testing.T, u *users.UserRecord, rr *RiftRoom) (string, int) {
	t.Helper()
	for _, name := range rr.DoorOrder {
		if route, handled := Router(u.UserId, rr.RoomId, name); handled && route.RoomId != 0 {
			return name, route.RoomId
		}
	}
	t.Fatalf(`room %d (%s) has no door open to the player: %v`, rr.RoomId, rr.Pool, rr.DoorOrder)
	return ``, 0
}

func TestRun_PortalEntryAndGuard(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)

	origin := rooms.LoadRoom(testOrigin)
	portal, ok := origin.ExitsTemp[run.Profile.PortalExit]
	require.True(t, ok, `the portal exit is added to the overworld room`)
	assert.Equal(t, run.EntryRoomId, portal.RoomId)

	entry := run.Rooms[run.EntryRoomId]
	require.NotNil(t, entry)
	assert.Equal(t, run.Profile.StartPool, entry.Pool)
	assert.False(t, entry.Expanded, `nothing behind the doors is built before anyone enters`)
	assert.Len(t, run.Rooms, 1)

	// A room holds one site; opening again hands back the same waiting run.
	again, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	assert.Equal(t, run.Id, again.Id)
	assert.Len(t, Sites(), 1)

	// The guard: a player at the site may enter a fresh run's entry room,
	// and doing so makes it theirs.
	allowed, _, _ := EntryGuard(testUser, run.EntryRoomId)
	assert.True(t, allowed)
	assert.True(t, run.claimed(testUser))

	// Doors are unsettled until someone is inside.
	route, handled := Router(testUser, run.EntryRoomId, entry.DoorOrder[0])
	assert.True(t, handled)
	assert.Zero(t, route.RoomId)
	assert.Equal(t, run.Profile.Msg(`unsettled`), route.Refusal)

	walk(t, u, run.EntryRoomId)
	assert.True(t, run.Members[testUser])
	assert.True(t, entry.Expanded)
	assert.Len(t, run.Rooms, 1+len(entry.DoorOrder))
	room := rooms.LoadRoom(run.EntryRoomId)
	for _, name := range entry.DoorOrder {
		door := entry.Doors[name]
		require.NotZero(t, door.DestRoomId)
		assert.Equal(t, door.DestRoomId, room.Exits[name].RoomId, `exits point at the built rooms`)
		assert.Equal(t, entry.RoomId, run.Rooms[door.DestRoomId].ParentRoomId)
	}

	// The guard keeps a stranger out of every room but the open entry.
	someChild := entry.Doors[entry.DoorOrder[0]].DestRoomId
	allowed, redirect, why := EntryGuard(2, someChild)
	assert.False(t, allowed)
	assert.Equal(t, testOrigin, redirect)
	assert.Equal(t, run.Profile.Msg(`closed`), why)
	allowed, _, _ = EntryGuard(testUser, someChild)
	assert.True(t, allowed, `members may go anywhere in their run`)
}

func TestRouter_Rules(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	entry := run.Rooms[run.EntryRoomId]
	name := entry.DoorOrder[0]
	door := entry.Doors[name]

	// Not a rift room, or not a door: not ours.
	_, handled := Router(testUser, testOrigin, `rift`)
	assert.False(t, handled)
	_, handled = Router(testUser, run.EntryRoomId, `nosuchdoor`)
	assert.False(t, handled)

	// Mobs never pass.
	route, handled := Router(0, run.EntryRoomId, name)
	assert.True(t, handled)
	assert.Zero(t, route.RoomId)

	// A locked door wants a key; an unlocked one does not.
	door.Locked, door.Unlocked = true, false
	route, _ = Router(testUser, run.EntryRoomId, name)
	assert.Zero(t, route.RoomId)
	assert.Equal(t, run.Profile.Msg(`locked`), route.Refusal)
	assert.Equal(t, run.Profile.Msg(`pick_refusal`, name), route.PickRefusal)
	door.Unlocked = true
	route, _ = Router(testUser, run.EntryRoomId, name)
	assert.Equal(t, door.DestRoomId, route.RoomId)

	// A puzzle-sealed door waits for the puzzle.
	door.Sealed = true
	route, _ = Router(testUser, run.EntryRoomId, name)
	assert.Equal(t, run.Profile.Msg(`sealed_puzzle`), route.Refusal)
	entry.PuzzleSolved = true
	route, _ = Router(testUser, run.EntryRoomId, name)
	assert.Equal(t, door.DestRoomId, route.RoomId)
}

func TestRun_NoWayBackAndTeardown(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	entry := run.Rooms[run.EntryRoomId]
	siblings := []int{}
	for _, name := range entry.DoorOrder {
		siblings = append(siblings, entry.Doors[name].DestRoomId)
	}

	_, next := firstOpenDoor(t, u, entry)
	walk(t, u, next)
	Sweep()

	// The room left behind is gone, with every other room it led to. The
	// portal stays open for the rest of the join window, then closes.
	assert.Nil(t, run.Rooms[entry.RoomId])
	assert.Nil(t, rooms.LoadRoom(entry.RoomId))
	for _, id := range siblings {
		if id != next {
			assert.Nil(t, run.Rooms[id], `unvisited sibling %d torn down`, id)
		}
	}
	assert.True(t, run.PortalOpen, `the party still has the join window`)
	run.PortalUntil = time.Now().Add(-time.Second)
	Sweep()
	assert.False(t, run.PortalOpen)
	_, stillThere := rooms.LoadRoom(testOrigin).ExitsTemp[run.Profile.PortalExit]
	assert.False(t, stillThere, `the portal exit is gone from the world`)

	// The current room stands and has been expanded.
	cur := run.Rooms[next]
	require.NotNil(t, cur)
	assert.True(t, cur.Expanded)

	// Walking back out to the world ends the run.
	walk(t, u, testOrigin)
	assert.False(t, run.Members[testUser])
	Sweep()
	assert.Nil(t, GetRun(run.Id))
	assert.False(t, IsRiftRoom(next))
}

func TestRun_LogoutLeavesAndPortalTimesOut(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)

	OnPlayerDespawn(testUser, run.EntryRoomId)
	assert.False(t, run.Members[testUser])
	allowed, redirect, _ := EntryGuard(testUser, run.Rooms[run.EntryRoomId].Doors[run.Rooms[run.EntryRoomId].DoorOrder[0]].DestRoomId)
	assert.False(t, allowed, `a player who logged out does not get back in`)
	assert.Equal(t, testOrigin, redirect)

	// The despawn hook empties the room; the sweep then ends the run.
	rooms.LoadRoom(run.EntryRoomId).RemovePlayer(testUser)
	u.Character.RoomId = testOrigin
	run.PortalUntil = time.Now().Add(-time.Minute)
	Sweep()
	assert.Nil(t, GetRun(run.Id))

	// An unentered portal times out and its run goes with it.
	run2, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	run2.PortalUntil = time.Now().Add(-time.Second)
	Sweep()
	assert.Nil(t, GetRun(run2.Id))
	_, stillThere := rooms.LoadRoom(testOrigin).ExitsTemp[run2.Profile.PortalExit]
	assert.False(t, stillThere)
}

func TestPuzzle_SequenceAndRiddle(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)

	// Build a puzzle room by hand from the shipped chimes room, given a
	// sequence puzzle (no shipped room uses one now; the engine still
	// supports the kind), as the C pool's only template.
	cPool := run.Profile.templates[PoolPuzzle]
	for _, tp := range cPool {
		if tp.Id == `c-chamber-of-four-chimes` {
			seq := *tp
			seq.Id = `c-test-sequence`
			seq.Puzzle = &PuzzleSpec{Kind: `sequence`, SealedDoor: `seal`, Sequence: []string{`black`, `red`, `blue`, `green`},
				Hard: true, Reward: `none`, Solved: `It opens.`, Wrong: `It does not.`, WrongConditionIds: []int{3}}
			run.Profile.templates[PoolPuzzle] = []*Template{&seq}
		}
	}
	t.Cleanup(func() { run.Profile.templates[PoolPuzzle] = cPool })
	require.Len(t, run.Profile.templates[PoolPuzzle], 1)
	rr, err := run.buildRoom(PoolPuzzle, 1, run.Rooms[run.EntryRoomId], `test`)
	require.NoError(t, err)
	room := rooms.LoadRoom(rr.RoomId)
	ps := rr.Template.Puzzle
	require.NotNil(t, ps)
	require.Equal(t, `sequence`, ps.Kind)
	sealed := rr.Doors[ps.SealedDoor]
	require.NotNil(t, sealed, `the sealed door is always among the room's doors`)
	assert.True(t, sealed.Sealed)

	// A wrong touch resets progress.
	assert.True(t, Touch(u, room, ps.Sequence[1]))
	assert.Zero(t, rr.SeqProgress)
	// Touching something outside the sequence is harmless.
	assert.True(t, Touch(u, room, ps.SealedDoor))
	assert.Zero(t, rr.SeqProgress)
	assert.False(t, Touch(u, room, `nonsense`))

	for i, noun := range ps.Sequence {
		assert.True(t, Touch(u, room, noun))
		if i < len(ps.Sequence)-1 {
			assert.Equal(t, i+1, rr.SeqProgress)
		}
	}
	assert.True(t, rr.PuzzleSolved)
	assert.False(t, rr.KeyAwarded, `only a lens table gives a key`)

	// Riddles normalise case, punctuation and leading articles.
	assert.Equal(t, `moon`, normalizeAnswer(`  The MOON!  `))
	assert.Equal(t, `echo`, normalizeAnswer(`an echo.`))
	assert.False(t, Answer(u, room, `the moon`), `a sequence room holds no riddle`)
}

// A late joiner within the join window lands beside the party once it has
// moved on from the entry room, and may be placed there.
func TestRun_LateJoinerFollowsParty(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	_, next := firstOpenDoor(t, u, run.Rooms[run.EntryRoomId])
	walk(t, u, next)
	Sweep()
	require.Nil(t, run.Rooms[run.EntryRoomId], `the entry room is gone`)
	require.True(t, run.PortalOpen)

	const late = testUser + 1
	l := users.NewTestUser(late, `late`, `Late`, 0)
	l.Character.RoomId = testOrigin
	rooms.LoadRoom(testOrigin).AddPlayer(late)
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{testUser: u, late: l})
	defer restore()

	// Not in the party: a run of their own, never the party's.
	route, handled := Router(late, testOrigin, run.Profile.PortalExit)
	require.True(t, handled)
	assert.NotEqual(t, next, route.RoomId, `a stranger is not sent to the party`)
	allowed, _, _ := EntryGuard(late, next)
	assert.False(t, allowed, `nor let in beside it`)

	party := parties.New(testUser)
	require.NotNil(t, party)
	require.True(t, party.InvitePlayer(late))
	require.True(t, party.AcceptInvite(late))
	defer party.Disband()

	route, handled = Router(late, testOrigin, run.Profile.PortalExit)
	require.True(t, handled)
	assert.Equal(t, next, route.RoomId, `routed to where the party stands`)
	allowed, _, _ = EntryGuard(late, next)
	assert.True(t, allowed)
	walk(t, l, next)
	assert.True(t, run.Members[late])
	rooms.LoadRoom(next).RemovePlayer(late)
	l.Character.RoomId = testOrigin
	OnRoomChange(late, next, testOrigin, false)
}

// A sweep that runs after a move but before its RoomChange is handled must
// not tear down the room being left: the handler still needs it to take the
// leaver's keys (and to spend one on the door they came through).
func TestRun_SweepBeforeRoomChangeKeepsKeysInside(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	GiveKeyTo(u, run)
	require.True(t, hasKey(u, run.Profile))

	// Move out, sweep, and only then deliver the event.
	from := run.EntryRoomId
	rooms.LoadRoom(from).RemovePlayer(u.UserId)
	rooms.LoadRoom(testOrigin).AddPlayer(u.UserId)
	u.Character.RoomId = testOrigin
	Sweep()
	assert.NotNil(t, run.Rooms[from], `the room waits for its RoomChange`)
	OnRoomChange(u.UserId, from, testOrigin, false)

	assert.False(t, hasKey(u, run.Profile), `the key did not leave the rift`)
	assert.False(t, run.Members[u.UserId])
	run.PortalUntil = time.Now().Add(-time.Second)
	Sweep()
	assert.Nil(t, GetRun(run.Id), `and the run is released`)
}

// A member whose departure is never seen is let go by the sweep, keys and
// all; ForceEnd takes keys from those it moves out.
func TestRun_MissedDepartureAndForceEnd(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	GiveKeyTo(u, run)
	run.ForceEnd()
	assert.False(t, hasKey(u, run.Profile), `ForceEnd purges keys`)
	assert.Nil(t, GetRun(run.Id))
	assert.Zero(t, originOf(u))

	run2, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run2.EntryRoomId)
	GiveKeyTo(u, run2)
	run2.Rooms[run2.EntryRoomId].present = nil // the event was lost
	rooms.LoadRoom(run2.EntryRoomId).RemovePlayer(u.UserId)
	u.Character.RoomId = testOrigin
	Sweep()
	assert.False(t, run2.Members[u.UserId])
	assert.False(t, hasKey(u, run2.Profile))
}

// Logging back in after logging out inside a rift puts the player at the
// portal they came in by, even after the run is gone.
func TestRun_LoginReturnsToOrigin(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	assert.Equal(t, testOrigin, originOf(u))
	inside := run.EntryRoomId

	OnPlayerDespawn(u.UserId, inside)
	rooms.LoadRoom(inside).RemovePlayer(u.UserId)
	run.PortalUntil = time.Now().Add(-time.Second)
	Sweep()
	require.Nil(t, GetRun(run.Id))

	// Login: the saved room is gone.
	u.Character.RoomId = inside
	OnPlayerSpawn(u.UserId)
	assert.Equal(t, testOrigin, u.Character.RoomId)
	assert.Zero(t, originOf(u))
}

// A Glowstone does not leave the rift either, carried or worn.
func TestRun_LightsStayInside(t *testing.T) {
	u := setupRuntime(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	giveItem(u, run.Profile.LightItemId, nil)
	giveItem(u, run.Profile.LightItemId, nil)
	require.Len(t, u.Character.GetAllBackpackItems(), 2)
	u.Character.Equipment.Light = u.Character.GetAllBackpackItems()[0]
	u.Character.RemoveItem(u.Character.Equipment.Light)

	walk(t, u, testOrigin)
	assert.Empty(t, u.Character.GetAllBackpackItems())
	assert.False(t, u.Character.Equipment.Light.IsValid(), `the worn one goes too`)
}
