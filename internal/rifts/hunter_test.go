package rifts

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedLenses loads minimal specs for the Obelisk's mobs (unit tests load no
// world data) so rooms can be built with them and hunters can appear.
func seedLenses(t *testing.T) {
	t.Helper()
	specs := map[int]*mobs.Mob{}
	for id, name := range map[int]string{
		9835: `Glint Stalker`, 9836: `Spine Lattice`, 9837: `Splitlight`,
		9838: `The Watching Obelisk`, 9839: `Facet Hunter`,
	} {
		m := &mobs.Mob{MobId: mobs.MobId(id), Zone: `Rift Obelisk`, Character: *characters.New()}
		m.Character.Name = name
		m.AutoAggro = id != 9835
		specs[id] = m
	}
	t.Cleanup(mobs.SeedMobsForTest(specs, map[int]*mobs.Mob{}))
}

// setNow moves the rift's clock to at for the rest of the test.
func setNow(t *testing.T, at time.Time) {
	t.Helper()
	orig := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = orig })
}

// plainDoor is a door of rr that needs no key and waits on no puzzle.
func plainDoor(t *testing.T, rr *RiftRoom) string {
	t.Helper()
	for _, name := range rr.DoorOrder {
		if d := rr.Doors[name]; !d.Locked && !d.Sealed {
			return name
		}
	}
	t.Fatalf(`room %d has no plain door`, rr.RoomId)
	return ``
}

// livingMobs lists the mobs standing in roomId.
func livingMobs(roomId int) []*mobs.Mob {
	var out []*mobs.Mob
	if room := rooms.LoadRoom(roomId); room != nil {
		for _, id := range room.GetMobs() {
			if m := mobs.GetInstance(id); m != nil && m.Character.Health > 0 {
				out = append(out, m)
			}
		}
	}
	return out
}

// monsterRoom builds a monster room off the entry room and points one of the
// entry room's doors at it, returning the room and that door.
func monsterRoom(t *testing.T, run *Run) (*RiftRoom, string) {
	t.Helper()
	entry := run.Rooms[run.EntryRoomId]
	rr, err := run.buildRoom(PoolMonster, 1, entry, entry.DoorOrder[0])
	require.NoError(t, err)
	door := entry.Doors[entry.DoorOrder[0]]
	door.Pool, door.Locked, door.DestRoomId = PoolMonster, false, rr.RoomId
	room := rooms.LoadRoom(entry.RoomId)
	info := room.Exits[entry.DoorOrder[0]]
	info.RoomId = rr.RoomId
	room.Exits[entry.DoorOrder[0]] = info
	return rr, entry.DoorOrder[0]
}

// Several of one kind in a room are several mobs: the rift places them
// itself, so the engine's spawn bookkeeping cannot fold them into one.
func TestSpawn_SameKindTwiceIsTwoMobs(t *testing.T) {
	setupRuntime(t)
	seedLenses(t)
	p := GetProfile(`obelisk`)
	p.Mobs.Trash = []int{9836}
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		rr, err := run.buildRoom(PoolMonster, 1, nil, `x`)
		require.NoError(t, err)
		if n := len(livingMobs(rr.RoomId)); n >= 2 {
			return
		}
	}
	t.Fatal(`never more than one Spine Lattice in a monster room`)
}

// Fleeing: a monster room is shut to walkers while its hostiles stand, but a
// flee gets out, and that starts a hunt. Walking out of a room with nothing
// left alive does not.
func TestHunt_FleeGetsOutAndIsHunted(t *testing.T) {
	u := setupRuntime(t)
	seedLenses(t)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	d, _ := monsterRoom(t, run)
	require.NotEmpty(t, livingMobs(d.RoomId), `the monster room has its monsters`)
	walk(t, u, d.RoomId) // builds the rooms behind its doors

	door := plainDoor(t, d)
	route, handled := Router(u.UserId, d.RoomId, door)
	require.True(t, handled)
	assert.Equal(t, run.Profile.Msg(`sealed_hostile`), route.Refusal, `walking out is refused`)

	var fled rooms.ExitRoute
	rooms.WhileFleeing(func() { fled, _ = Router(u.UserId, d.RoomId, door) })
	require.NotZero(t, fled.RoomId, `a flee gets through`)

	hunted, _ := run.Hunted(u.UserId)
	require.False(t, hunted)
	walk(t, u, fled.RoomId)
	hunted, hunter := run.Hunted(u.UserId)
	assert.True(t, hunted, `fleeing a fight starts a hunt`)
	assert.Zero(t, hunter, `the hunter is not out yet`)
}

// hide makes c hidden, as a successful sneak does.
func hide(t *testing.T, c *characters.Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: `rift_test`}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden())
}

// moveTo puts u in roomId the way a walk does, without the handler (the
// hunter follows by the sweep, not by the move).
func moveTo(u *users.UserRecord, roomId int) {
	if r := rooms.LoadRoom(u.Character.RoomId); r != nil {
		r.RemovePlayer(u.UserId)
	}
	rooms.LoadRoom(roomId).AddPlayer(u.UserId)
	u.Character.RoomId = roomId
}

// fleeInto starts a run, walks u into a monster room and flees them out of
// it, returning the run and the room they fled to.
func fleeInto(t *testing.T, u *users.UserRecord) (*Run, *RiftRoom) {
	t.Helper()
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	d, _ := monsterRoom(t, run)
	walk(t, u, d.RoomId)
	var away rooms.ExitRoute
	rooms.WhileFleeing(func() { away, _ = Router(u.UserId, d.RoomId, plainDoor(t, d)) })
	require.NotZero(t, away.RoomId)
	walk(t, u, away.RoomId)
	hunted, _ := run.Hunted(u.UserId)
	require.True(t, hunted)
	return run, run.Rooms[away.RoomId]
}

// The hunter comes room by room: it appears once its quarry has stood in one
// room for the delay, and a quarry who keeps moving stays ahead of it. Out,
// it holds a quarry it can see: no door, no flee. Others may go. When its
// quarry gets away it withdraws, and the SAME hunter (same instance, same
// wounds) comes again in their next room. Destroying it ends the hunt.
func TestHunt_OneHunterFollowsRoomByRoom(t *testing.T) {
	u := setupRuntime(t)
	seedLenses(t)
	start := time.Now()
	setNow(t, start)
	run, here := fleeInto(t, u)
	d := run.Profile.HunterDelaySeconds
	at := func(secs int) { setNow(t, start.Add(time.Duration(secs)*time.Second)); Sweep() }

	at(0) // the count starts in this room
	at(d.Min - 1)
	_, out := run.Hunted(u.UserId)
	assert.Zero(t, out, `not yet`)

	at(d.Max + 1)
	_, out = run.Hunted(u.UserId)
	require.NotZero(t, out, `it steps out of the walls`)
	m := mobs.GetInstance(out)
	require.NotNil(t, m)
	assert.Equal(t, here.RoomId, m.Character.RoomId)
	assert.Equal(t, u.UserId, m.Character.CurrentCombatTarget().UserId, `it is on its quarry`)

	// No way out for a quarry it can see, walking or fleeing.
	for _, name := range here.DoorOrder {
		route, _ := Router(u.UserId, here.RoomId, name)
		assert.Equal(t, run.Profile.Msg(`hunter_holds`), route.Refusal)
		rooms.WhileFleeing(func() { route, _ = Router(u.UserId, here.RoomId, name) })
		assert.Zero(t, route.RoomId, `nor by fleeing`)
	}
	// Someone else is free to go.
	const friend = testUser + 1
	f := users.NewTestUser(friend, `friend`, `Friend`, 0)
	f.Character.RoomId = here.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{testUser: u, friend: f}))
	run.Members[friend] = true
	route, _ := Router(friend, here.RoomId, here.DoorOrder[0])
	assert.NotEqual(t, run.Profile.Msg(`hunter_holds`), route.Refusal)
	delete(run.Members, friend)

	// Its quarry gets away: it withdraws, wounded, and is the one that comes.
	m.Character.Health = 33
	second, err := run.buildRoom(PoolPassage, here.Depth+1, here, `y`)
	require.NoError(t, err)
	moveTo(u, second.RoomId)
	at(d.Max + 2)
	hunted, out := run.Hunted(u.UserId)
	assert.True(t, hunted, `still hunted`)
	assert.Zero(t, out, `withdrawn into the walls`)
	assert.Nil(t, mobs.GetInstance(m.InstanceId), `not standing anywhere`)
	assert.Same(t, m, run.HunterOf(u.UserId), `kept, not destroyed`)

	// A quarry who moves on before the delay is up stays ahead of it.
	at(d.Max + 3) // count starts in the second room
	third, err := run.buildRoom(PoolPassage, second.Depth+1, second, `z`)
	require.NoError(t, err)
	at(d.Max + 3 + d.Min - 1)
	moveTo(u, third.RoomId)
	at(d.Max + 3 + d.Min) // a new room: the count starts again
	_, out = run.Hunted(u.UserId)
	assert.Zero(t, out, `moving kept it at bay`)

	// Standing still lets it catch up: the same creature, still wounded.
	at(d.Max + 3 + d.Min + d.Max + 1)
	_, out = run.Hunted(u.UserId)
	require.Equal(t, m.InstanceId, out, `the same hunter`)
	assert.Equal(t, 33, m.Character.Health, `with the wounds it took`)
	assert.Equal(t, third.RoomId, m.Character.RoomId)

	OnMobDeath(third.RoomId, run.Profile.Mobs.Hunter, out)
	hunted, _ = run.Hunted(u.UserId)
	assert.False(t, hunted, `destroying the hunter ends the hunt`)
}

// Stealth is respected. A hidden player slips past a sealed room unseen and
// is not hunted for it. A hunter that arrives beside a hidden quarry does not
// attack them unless it spots them, and does not hold them.
func TestHunt_StealthIsRespected(t *testing.T) {
	u := setupRuntime(t)
	seedLenses(t)
	start := time.Now()
	setNow(t, start)

	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	d, _ := monsterRoom(t, run)
	walk(t, u, d.RoomId)
	hide(t, u.Character)
	route, _ := Router(u.UserId, d.RoomId, plainDoor(t, d))
	require.NotZero(t, route.RoomId, `a hidden player slips past the seal`)
	walk(t, u, route.RoomId)
	hunted, _ := run.Hunted(u.UserId)
	assert.False(t, hunted, `slipping out unseen is not fleeing`)

	// Hunted anyway (some other fight), and hidden when the hunter comes.
	run.startHunt(u)
	Sweep()
	setNow(t, start.Add(time.Duration(run.Profile.HunterDelaySeconds.Max+1)*time.Second))
	Sweep()
	_, out := run.Hunted(u.UserId)
	require.NotZero(t, out)
	m := mobs.GetInstance(out)
	if u.Character.IsHidden() {
		assert.NotEqual(t, u.UserId, m.Character.CurrentCombatTarget().UserId, `it does not attack what it has not found`)
		assert.False(t, run.hunterHolds(u.UserId, u.Character.RoomId), `nor hold it`)
	} else {
		assert.Equal(t, u.UserId, m.Character.CurrentCombatTarget().UserId, `spotted: on them`)
	}
}

// Leaving the rift ends the hunt and takes the hunter with it.
func TestHunt_LeavingTheRiftEndsIt(t *testing.T) {
	u := setupRuntime(t)
	seedLenses(t)
	start := time.Now()
	setNow(t, start)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	run.startHunt(u)
	Sweep()
	setNow(t, start.Add(time.Hour))
	Sweep()
	_, hunter := run.Hunted(u.UserId)
	require.NotZero(t, hunter)

	walk(t, u, testOrigin)
	hunted, _ := run.Hunted(u.UserId)
	assert.False(t, hunted)
	assert.Nil(t, mobs.GetInstance(hunter), `the hunter went with the hunt`)
}

// A passage sometimes holds a lurker (a Glint Stalker), so no pool is
// always safe; it does not seal the passage.
func TestLurker_InAPassage(t *testing.T) {
	u := setupRuntime(t)
	seedLenses(t)
	p := GetProfile(`obelisk`)
	assert.Equal(t, 10, p.Lurkers.Chance[PoolPassage])
	p.Lurkers.Chance = map[Pool]int{PoolPassage: 100}
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	rr, err := run.buildRoom(PoolPassage, 1, nil, `x`)
	require.NoError(t, err)
	got := livingMobs(rr.RoomId)
	require.Len(t, got, 1)
	assert.Equal(t, 9835, int(got[0].MobId))
	walk(t, u, rr.RoomId)
	for _, name := range rr.DoorOrder {
		route, _ := Router(u.UserId, rr.RoomId, name)
		assert.NotEqual(t, p.Msg(`sealed_hostile`), route.Refusal)
	}
}

// The hunter holds only its quarry: standing in a cleared monster room it
// does not seal the doors for anyone else (and fleeing past it starts no
// hunt for them).
func TestHunt_HunterDoesNotSealTheRoom(t *testing.T) {
	setupRuntime(t)
	seedLenses(t)
	p := GetProfile(`obelisk`)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	d, _ := monsterRoom(t, run)
	room := rooms.LoadRoom(d.RoomId)
	for _, m := range livingMobs(d.RoomId) {
		removeMob(m.InstanceId)
	}
	require.False(t, hostilesIn(p, room, 0), `cleared`)

	spawnMob(room, p.Mobs.Hunter, 10)
	assert.False(t, hostilesIn(p, room, 0), `the hunter does not count`)
	spawnMob(room, p.Mobs.Trash[0], 10)
	assert.True(t, hostilesIn(p, room, 0), `a Lens does`)
}

// Slipping out unseen counts as unseen even when the next room's watchers
// spot the player before the move is handled; and walking away from someone
// else's hunter is not fleeing.
func TestHunt_NoHuntForAnUnseenExitOrForLeavingAnotherHunter(t *testing.T) {
	u := setupRuntime(t)
	seedLenses(t)
	p := GetProfile(`obelisk`)
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	d, _ := monsterRoom(t, run)
	walk(t, u, d.RoomId)
	hide(t, u.Character)
	route, _ := Router(u.UserId, d.RoomId, plainDoor(t, d))
	require.NotZero(t, route.RoomId)
	moveTo(u, route.RoomId)
	// Spotted on arrival: visible by the time the event is handled.
	_ = u.Character.Awareness.TransitionToRevealing(state.TransitionReason{Trigger: `rift_test`})
	require.False(t, u.Character.IsHidden())
	OnRoomChange(u.UserId, d.RoomId, route.RoomId, true)
	hunted, _ := run.Hunted(u.UserId)
	assert.False(t, hunted, `it moved unseen`)

	// Someone else's hunter, fighting this player, left behind: no hunt.
	here, err := run.buildRoom(PoolPassage, 2, nil, `p`)
	require.NoError(t, err)
	moveTo(u, here.RoomId) // set the scene without the move's own handler
	here.present = map[int]bool{u.UserId: true}
	m := spawnMob(rooms.LoadRoom(here.RoomId), p.Mobs.Hunter, 10)
	require.NotNil(t, m)
	targeting.Commit(&m.Character, state.ActorRef{UserId: u.UserId}, targeting.ReasonAttack)
	next, err := run.buildRoom(PoolPassage, here.Depth+1, here, `y`)
	require.NoError(t, err)
	assert.False(t, run.fledFight(u.UserId, false, here, rooms.LoadRoom(here.RoomId)))
	walk(t, u, next.RoomId)
	hunted, _ = run.Hunted(u.UserId)
	assert.False(t, hunted, `walking away from a hunter is not fleeing`)
}
