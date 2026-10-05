package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixtureCmdItemId = 999991
	pebbleCmdItemId  = 999992
)

// seedFixtureRoom puts a fixture (Arch Lantern) and a pebble on user 1's
// floor at midsummer noon, so nothing is refused as blind.
func seedFixtureRoom(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	util.SetRoundCountForTest(uint64(3430))
	t.Cleanup(util.ResetRoundCountForTest)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		fixtureCmdItemId: {ItemId: fixtureCmdItemId, Name: "Arch Lantern", NameSimple: "lantern", Type: items.Object,
			Fixture: items.FixtureLight, Value: 1, NotSalable: true},
		pebbleCmdItemId: {ItemId: pebbleCmdItemId, Name: "Grey Pebble", NameSimple: "pebble", Type: items.Object,
			Weight: 0.1, Value: 1},
	}))
	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 50
	origItems := user.Character.Items
	user.Character.Items = nil
	t.Cleanup(func() { user.Character.Items = origItems })
	lantern, pebble := items.New(fixtureCmdItemId), items.New(pebbleCmdItemId)
	room.AddItem(lantern, false)
	room.AddItem(pebble, false)
	t.Cleanup(func() {
		room.RemoveItem(lantern, false)
		room.RemoveItem(pebble, false)
	})
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

func sentTo(user *users.UserRecord) string {
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
}

// X10: `get <fixture>` says it is fixed in place and takes nothing.
func TestGetAFixtureIsRefused(t *testing.T) {
	user, room := seedFixtureRoom(t)
	_, err := Get("lantern", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "The <ansi fg=\"itemname\">Arch Lantern</ansi> is fixed in place.")
	assert.Empty(t, user.Character.Items, "nothing taken")
}

// X10: `get all` sweeps past a fixture without a word about it, and
// `get all lantern` names it as fixed rather than "you don't see any".
func TestGetAllLeavesAFixture(t *testing.T) {
	user, room := seedFixtureRoom(t)
	Get("all", user, room, 0)
	out := sentTo(user)
	require.Len(t, user.Character.Items, 1, "the pebble is taken")
	assert.Equal(t, pebbleCmdItemId, user.Character.Items[0].ItemId)
	assert.NotContains(t, out, "fixed in place", "a sweep does not name what it leaves fixed")
	_, still := room.FindOnFloor("lantern", false)
	assert.True(t, still, "the lantern stays")

	Get("all lantern", user, room, 0)
	assert.Contains(t, sentTo(user), "is fixed in place.")
}

// X10: `steal <fixture>` names it as fixed in place.
func TestStealAFixtureIsRefused(t *testing.T) {
	user, room := seedFixtureRoom(t)
	user.Character.SetSkill(string(skills.Skullduggery), 2)
	Steal("lantern", user, room, 0)
	assert.Contains(t, sentTo(user), "is fixed in place.")
	assert.Empty(t, user.Character.Items, "nothing taken")
}

// X10: with no sight of the floor, `steal <fixture>` does not confirm an
// unseen item by its name; it answers as it would for anything unknown.
func TestStealAFixtureInTheDarkSaysNothingOfIt(t *testing.T) {
	user, room := seedFixtureRoom(t)
	user.Character.SetSkill(string(skills.Skullduggery), 2)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = nil
	require.True(t, actions.TooDarkToGet(&actions.UserActor{User: user, Room: room}),
		"fixture: the room must be too dark to see the floor")
	Steal("lantern", user, room, 0)
	out := sentTo(user)
	assert.NotContains(t, out, "Arch Lantern")
	assert.NotContains(t, out, "fixed in place")
	assert.Contains(t, out, "Steal from whom?")
}

// A fixture that is first on the floor must not shadow an ordinary item the
// same name matches: the sweep never offers the fixture at all.
func TestGetAllNamedSweepSkipsAShadowingFixture(t *testing.T) {
	user, room := seedFixtureRoom(t)
	const brassId = 999993
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		fixtureCmdItemId: {ItemId: fixtureCmdItemId, Name: "Arch Lantern", NameSimple: "lantern", Type: items.Object,
			Fixture: items.FixtureLight, Value: 1, NotSalable: true},
		brassId: {ItemId: brassId, Name: "Brass Lantern", NameSimple: "lantern", Type: items.Object,
			Weight: 0.1, Value: 1},
	}))
	brass := items.New(brassId)
	room.AddItem(brass, false)
	t.Cleanup(func() { room.RemoveItem(brass, false) })
	events.DrainQueuedMessagesForTest(user.UserId)

	Get("all lantern", user, room, 0)
	out := sentTo(user)
	assert.NotContains(t, out, "overloaded", "the fixture was offered to the sweep")
	require.Len(t, user.Character.Items, 1)
	assert.Equal(t, brassId, user.Character.Items[0].ItemId)
}
