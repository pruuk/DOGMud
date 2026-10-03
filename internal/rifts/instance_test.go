package rifts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// atSite puts a new online player in the site room.
func atSite(t *testing.T, all map[int]*users.UserRecord, id int, name string) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(id, name, name, 0)
	u.Character.RoomId = testOrigin
	rooms.LoadRoom(testOrigin).AddPlayer(id)
	all[id] = u
	t.Cleanup(users.SeedUsersForTest(all))
	return u
}

// enter takes u through the crystal the way `enter crystal` does: routed,
// then guarded, then moved.
func enter(t *testing.T, u *users.UserRecord) (*Run, bool) {
	t.Helper()
	p := GetProfile(`obelisk`)
	route, handled := Router(u.UserId, testOrigin, p.PortalExit)
	require.True(t, handled)
	if route.RoomId == 0 {
		return nil, false
	}
	allowed, _, _ := EntryGuard(u.UserId, route.RoomId)
	if !allowed {
		return nil, false
	}
	walk(t, u, route.RoomId)
	return RunForRoom(route.RoomId), true
}

// Strangers at one crystal each get a run of their own; a party shares one;
// nobody gets into anybody else's.
func TestInstances_StrangersNeverShare(t *testing.T) {
	a := setupRuntime(t)
	all := map[int]*users.UserRecord{testUser: a}
	_, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	b := atSite(t, all, 2, `Bee`)
	c := atSite(t, all, 3, `Cee`)
	d := atSite(t, all, 4, `Dee`)

	party := parties.New(a.UserId)
	require.True(t, party.InvitePlayer(c.UserId))
	require.True(t, party.AcceptInvite(c.UserId))
	t.Cleanup(party.Disband)

	runA, ok := enter(t, a)
	require.True(t, ok)
	runB, ok := enter(t, b)
	require.True(t, ok)
	assert.NotEqual(t, runA.Id, runB.Id, `a stranger at the same crystal gets a run of their own`)
	runC, ok := enter(t, c)
	require.True(t, ok)
	assert.Equal(t, runA.Id, runC.Id, `a party member joins the party's run`)
	runD, ok := enter(t, d)
	require.True(t, ok)
	assert.NotEqual(t, runA.Id, runD.Id)
	assert.NotEqual(t, runB.Id, runD.Id)

	// Nobody can be put into another's run by any move that is not the
	// portal: a member of one run, or someone elsewhere in the world.
	for _, rr := range runA.Rooms {
		allowed, _, _ := EntryGuard(b.UserId, rr.RoomId)
		assert.False(t, allowed, `room %d of A's run`, rr.RoomId)
	}
	allowed, _, _ := EntryGuard(a.UserId, runB.EntryRoomId)
	assert.False(t, allowed, `A cannot reach B's run`)
}

// A claim made at the portal lets the claimant's party follow a moment
// later, before the claimant's arrival is handled; a stranger in that moment
// still gets a run of their own. Claims lapse.
func TestInstances_ClaimsAndFollowers(t *testing.T) {
	a := setupRuntime(t)
	all := map[int]*users.UserRecord{testUser: a}
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	b := atSite(t, all, 2, `Bee`)
	c := atSite(t, all, 3, `Cee`)
	party := parties.New(a.UserId)
	require.True(t, party.InvitePlayer(c.UserId))
	require.True(t, party.AcceptInvite(c.UserId))
	t.Cleanup(party.Disband)

	allowed, _, _ := EntryGuard(a.UserId, run.EntryRoomId)
	require.True(t, allowed, `A commits to the waiting run`)
	p := run.Profile
	route, _ := Router(c.UserId, testOrigin, p.PortalExit)
	assert.Equal(t, run.EntryRoomId, route.RoomId, `A's party follows into A's run`)
	route, _ = Router(b.UserId, testOrigin, p.PortalExit)
	assert.NotEqual(t, run.EntryRoomId, route.RoomId, `a stranger is sent elsewhere`)
	allowed, _, _ = EntryGuard(b.UserId, run.EntryRoomId)
	assert.False(t, allowed)

	// Nobody fleeing goes into a rift.
	rooms.WhileFleeing(func() { route, _ = Router(b.UserId, testOrigin, p.PortalExit) })
	assert.Zero(t, route.RoomId)

	// A claim that is never followed by an arrival lapses.
	saved := now
	now = func() time.Time { return saved().Add(2 * claimFor) }
	t.Cleanup(func() { now = saved })
	assert.False(t, run.claimed(a.UserId))
	assert.Empty(t, run.company())
}

// Someone who left the party cannot bring a stranger in beside those still
// in it: a joiner must be in a party with everyone the run belongs to.
func TestInstances_JoinNeedsTheWholeCompany(t *testing.T) {
	a := setupRuntime(t)
	all := map[int]*users.UserRecord{testUser: a}
	_, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	x := atSite(t, all, 2, `Ex`)
	s := atSite(t, all, 3, `Es`)

	party := parties.New(a.UserId)
	require.True(t, party.InvitePlayer(x.UserId))
	require.True(t, party.AcceptInvite(x.UserId))
	run, ok := enter(t, a)
	require.True(t, ok)
	got, ok := enter(t, x)
	require.True(t, ok)
	require.Equal(t, run.Id, got.Id)
	party.Disband()

	other := parties.New(x.UserId)
	require.NotNil(t, other)
	require.True(t, other.InvitePlayer(s.UserId))
	require.True(t, other.AcceptInvite(s.UserId))
	t.Cleanup(other.Disband)
	assert.False(t, run.mayJoin(s.UserId), `S is in a party with X but not with A`)
}

// An invite nobody accepted joins no one: a stranger invited at the crystal
// who walks in alone does not open their run to the party that invited them.
func TestInstances_UnacceptedInviteJoinsNoOne(t *testing.T) {
	a := setupRuntime(t)
	all := map[int]*users.UserRecord{testUser: a}
	_, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	s := atSite(t, all, 2, `Es`)

	party := parties.New(a.UserId)
	require.True(t, party.InvitePlayer(s.UserId))
	t.Cleanup(party.Disband)

	runS, ok := enter(t, s)
	require.True(t, ok)
	runA, ok := enter(t, a)
	require.True(t, ok)
	assert.NotEqual(t, runS.Id, runA.Id, `the inviter does not land in the invitee's run`)
	assert.False(t, runS.mayJoin(a.UserId))
}
