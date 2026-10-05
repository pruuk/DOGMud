package rifts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// A site holds a joinable run only while someone stands at it, gives a party
// a join window after the first one goes in, then makes a fresh run for the
// next comer. The run remembers where it was entered.
func TestSite_JoinWindowAndFreshRun(t *testing.T) {
	u := setupRuntime(t)
	require.NoError(t, AddSite(`obelisk`, testOrigin))
	site := SiteAt(testOrigin)
	require.NotNil(t, site)
	first := site.run
	require.NotNil(t, first, `a player is standing there, so the site holds a run`)
	assert.True(t, first.PortalOpen)
	assert.Equal(t, testOrigin, first.OriginRoomId)

	// The portal exit is routed into the joinable run.
	route, handled := Router(testUser, testOrigin, `crystal`)
	require.True(t, handled)
	assert.Equal(t, first.EntryRoomId, route.RoomId)
	_, handled = Router(testUser, testOrigin, `north`)
	assert.False(t, handled, `the site claims only its portal exit`)
	route, _ = Router(0, testOrigin, `crystal`)
	assert.Zero(t, route.RoomId, `mobs do not enter`)
	exitName, ok := PortalNoun(rooms.LoadRoom(testOrigin), `crystal`)
	assert.True(t, ok)
	assert.Equal(t, `crystal`, exitName)

	// The first one in starts the join window; the portal stays open for it.
	walk(t, u, first.EntryRoomId)
	assert.True(t, first.Entered)
	assert.True(t, first.PortalUntil.After(time.Now()))
	Sweep()
	assert.True(t, first.PortalOpen, `still inside the join window`)

	// Once the window has passed, the run closes to newcomers.
	first.PortalUntil = time.Now().Add(-time.Second)
	Sweep()
	assert.False(t, first.PortalOpen)

	// A second player at the site gets a fresh run, not the closed one.
	u2 := users.NewTestUser(2, `second`, `Second`, 0)
	u2.Character.RoomId = testOrigin
	rooms.LoadRoom(testOrigin).AddPlayer(2)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{testUser: u, 2: u2})()
	Sweep()
	require.NotNil(t, site.run)
	assert.NotEqual(t, first.Id, site.run.Id)
	route, _ = Router(2, testOrigin, `crystal`)
	assert.Equal(t, site.run.EntryRoomId, route.RoomId)

	// The first run's way out still leads to where it was entered, even after
	// the site is gone.
	RemoveSite(testOrigin)
	assert.Nil(t, SiteAt(testOrigin))
	assert.Equal(t, testOrigin, first.exitRoomId())
	rooms.LoadRoom(testOrigin).RemovePlayer(2)
}

// An unentered run is let go as soon as nobody is waiting at the site.
func TestSite_IdleRunReleased(t *testing.T) {
	u := setupRuntime(t)
	require.NoError(t, AddSite(`obelisk`, testOrigin))
	run := SiteAt(testOrigin).run
	require.NotNil(t, run)

	rooms.LoadRoom(testOrigin).RemovePlayer(u.UserId)
	Sweep()
	assert.Nil(t, GetRun(run.Id))
	_, open := rooms.LoadRoom(testOrigin).ExitsTemp[`crystal`]
	assert.False(t, open)
	assert.NotNil(t, SiteAt(testOrigin), `the site itself stays for the day`)
}

// Saved sites come back the same day and not on another.
func TestSite_RestoreOnlySameDay(t *testing.T) {
	setupRuntime(t)
	assert.False(t, Restore(SiteState{Day: `1999-01-01`, Sites: []SiteRecord{{RoomId: testOrigin, ProfileId: `obelisk`}}}))
	assert.Empty(t, Sites())
	assert.True(t, Restore(SiteState{Day: Today(), Sites: []SiteRecord{{RoomId: testOrigin, ProfileId: `obelisk`, Region: `Test`}}}))
	require.NotNil(t, SiteAt(testOrigin))
	st := State()
	assert.Equal(t, Today(), st.Day)
	assert.Equal(t, []SiteRecord{{RoomId: testOrigin, ProfileId: `obelisk`, Region: `Test`}}, st.Sites)
	assert.False(t, DailyTick(SiteSettings{}), `nothing to do on the day the sites were chosen`)
}

// Lore: distinct objects count, repeats do not, and the needed number makes
// the player a reader for good. Writings then read; before, they do not.
func TestLore_StudyUnlocksWritings(t *testing.T) {
	u := setupRuntime(t)
	p := GetProfile(`obelisk`)
	require.NotNil(t, p)

	readWriting(u, p, 0)
	assert.Empty(t, FragmentsRead(u, p), `a writing is not read before the lore is studied`)

	for i := 0; i < p.Lore.Needed-1; i++ {
		studyLore(u, p, `tok`+string(rune('a'+i)))
		studyLore(u, p, `tok`+string(rune('a'+i))) // a repeat does not count
		assert.Equal(t, i+1, LoreStudied(u, p))
		assert.False(t, IsReader(u, p))
	}
	// The saved form ([]any after a reload) is read back the same.
	seen := u.Character.GetMiscData(loreSeenKey(p)).([]string)
	asAny := make([]any, len(seen))
	for i, s := range seen {
		asAny[i] = s
	}
	u.Character.SetMiscData(loreSeenKey(p), asAny)
	assert.Equal(t, p.Lore.Needed-1, LoreStudied(u, p))

	studyLore(u, p, `last`)
	assert.True(t, IsReader(u, p), `the tag is set`)
	assert.Nil(t, u.Character.GetMiscData(loreSeenKey(p)), `the counting list is dropped once unlocked`)

	readWriting(u, p, 2)
	readWriting(u, p, 2)
	readWriting(u, p, 0)
	assert.Equal(t, []int{3, 1}, FragmentsRead(u, p))
}

// Building a room places lore, writings (one of each per run first) and
// leaves the lore noun lookable through OnLook.
func TestBuild_PlacesLoreAndWritings(t *testing.T) {
	u := setupRuntime(t)
	p := GetProfile(`obelisk`)
	p.Lore.Chance = map[Pool]int{PoolFeature: 100}
	p.WritingChance = map[Pool]int{PoolFeature: 100}

	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	seen := map[int]bool{}
	for idx := range run.placedWritings { // the entry room may already hold one
		seen[idx] = true
	}
	for len(seen) < len(p.Writings) {
		rr, err := run.buildRoom(PoolFeature, 1, nil, `x`)
		require.NoError(t, err)
		room := rooms.LoadRoom(rr.RoomId)
		assert.NotEmpty(t, rr.LoreToken)
		assert.Contains(t, room.Nouns, p.Lore.Noun)
		require.GreaterOrEqual(t, rr.Writing, 0)
		assert.Contains(t, room.Nouns, p.Writings[rr.Writing].Noun)
		assert.False(t, seen[rr.Writing], `a fragment repeats before all have been placed`)
		seen[rr.Writing] = true
	}

	// OnLook in a lore room teaches.
	rr, err := run.buildRoom(PoolFeature, 1, nil, `x`)
	require.NoError(t, err)
	OnLook(u.UserId, rr.RoomId, p.Lore.Noun)
	assert.Equal(t, 1, LoreStudied(u, p))
	OnLook(u.UserId, rr.RoomId, p.Lore.Noun)
	assert.Equal(t, 1, LoreStudied(u, p), `the same prism twice counts once`)
}

// Rubble: placed by the pool's chance, searched once for a find of the
// pool's tier, then picked over; anything else (another noun, a room with no
// pile, a room outside any rift) is left to the normal search.
func TestRubble_PlacedAndClaimedOnce(t *testing.T) {
	u := setupRuntime(t)
	p := GetProfile(`obelisk`)
	require.NotNil(t, p)
	assert.Equal(t, 5, p.Rubble.Chance[PoolPassage], `hallways rarely hold rubble`)
	assert.Equal(t, 50, p.Rubble.Chance[PoolBoss], `boss rooms often do`)

	p.Rubble.Chance = map[Pool]int{PoolBoss: 100, PoolPassage: 0}
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)

	bare, err := run.buildRoom(PoolPassage, 1, nil, `x`)
	require.NoError(t, err)
	assert.False(t, bare.Rubble)
	_, _, handled := ClaimRubble(u, bare.RoomId, p.Rubble.Noun)
	assert.False(t, handled, `no pile, so the normal search runs`)

	rr, err := run.buildRoom(PoolBoss, 1, nil, `x`)
	require.NoError(t, err)
	require.True(t, rr.Rubble)
	room := rooms.LoadRoom(rr.RoomId)
	assert.Contains(t, room.Nouns, p.Rubble.Noun)
	assert.Contains(t, room.Description, p.Rubble.Mention)

	_, _, handled = ClaimRubble(u, rr.RoomId, `wall`)
	assert.False(t, handled, `another feature is not the pile`)

	tier, claimed, handled := ClaimRubble(u, rr.RoomId, `Rubble`)
	assert.True(t, handled)
	assert.True(t, claimed)
	assert.Equal(t, p.Rubble.Tier[PoolBoss], tier)

	_, claimed, handled = ClaimRubble(u, rr.RoomId, p.Rubble.Noun)
	assert.True(t, handled, `a picked-over pile still answers the search`)
	assert.False(t, claimed, `but only once yields anything`)

	_, _, handled = ClaimRubble(u, testOrigin, p.Rubble.Noun)
	assert.False(t, handled, `outside a rift nothing is intercepted`)
}

// Every rift room carries the profile's shared ambience (shown on the world's
// ambient timing), and tells roomlife its own chance and setting.
func TestAmbience_SharedLinesAndPlace(t *testing.T) {
	setupRuntime(t)
	p := GetProfile(`obelisk`)
	require.GreaterOrEqual(t, len(p.IdleMessages), 10)
	assert.Equal(t, 20, p.AmbientGeneratedChance)
	assert.NotEmpty(t, p.AmbientSetting)

	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	room := rooms.LoadRoom(run.EntryRoomId)
	for _, line := range p.IdleMessages {
		assert.Contains(t, room.IdleMessages, line)
	}

	chance, setting, ok := AmbientPlace(run.EntryRoomId)
	assert.True(t, ok)
	assert.Equal(t, 20, chance)
	assert.Equal(t, p.AmbientSetting, setting)
	_, _, ok = AmbientPlace(testOrigin)
	assert.False(t, ok, `the world keeps its own`)
}
