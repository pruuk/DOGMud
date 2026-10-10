package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedTwoCorpses lays an older Lookout corpse (holding an Iron Sword) and a
// newer Skeleton corpse (holding Chain Mail) in user 1's room at midsummer
// noon, so nothing is refused as blind.
func seedTwoCorpses(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	util.SetRoundCountForTest(uint64(3430))
	t.Cleanup(util.ResetRoundCountForTest)

	user, room := getTestUserAndRoom(t)
	origItems, origGold, origCorpses := user.Character.Items, user.Character.Gold, room.Corpses
	t.Cleanup(func() {
		user.Character.Items, user.Character.Gold, room.Corpses = origItems, origGold, origCorpses
	})
	user.Character.Items = nil

	lookout := rooms.Corpse{MobId: 1, Loot: rooms.Container{Items: []items.Item{items.New(10001)}}}
	lookout.Character.Name = "Lookout"
	skeleton := rooms.Corpse{MobId: 1, Loot: rooms.Container{Items: []items.Item{items.New(20001)}}}
	skeleton.Character.Name = "Skeleton"
	room.Corpses = []rooms.Corpse{lookout, skeleton} // appended on death: the skeleton is newest

	// The bug needs a bare "corpse" to mean the skeleton and the full name
	// to mean the lookout; without both the tests below prove nothing.
	require.Equal(t, 1, room.FindCorpseIndex("corpse", user.Character), "precondition: corpse means the newest")
	require.Equal(t, 0, room.FindCorpseIndex("lookout corpse", user.Character), "precondition: the name reaches the lookout")
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

// #217: `get all <corpse>` resolved the corpse from its last word, so
// `get all lookout corpse` swept whichever corpse was newest.
func TestGetAll_NamedCorpseTakesFromThatCorpse(t *testing.T) {
	user, room := seedTwoCorpses(t)
	handled, err := Get("all lookout corpse", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	assert.Empty(t, room.Corpses[0].Loot.Items, "the lookout is emptied")
	assert.Len(t, room.Corpses[1].Loot.Items, 1, "the skeleton is untouched")
	require.Len(t, user.Character.Items, 1)
	assert.Equal(t, 10001, user.Character.Items[0].ItemId)
}

// The form from the issue: `get all from <corpse>`.
func TestGetAllFrom_NamedCorpseTakesFromThatCorpse(t *testing.T) {
	user, room := seedTwoCorpses(t)
	Get("all from lookout corpse", user, room, 0)
	assert.Empty(t, room.Corpses[0].Loot.Items, "the lookout is emptied")
	assert.Len(t, room.Corpses[1].Loot.Items, 1, "the skeleton is untouched")
}

// A bare `get all corpse` still means the newest corpse.
func TestGetAll_BareCorpseStillMeansTheNewest(t *testing.T) {
	user, room := seedTwoCorpses(t)
	Get("all corpse", user, room, 0)
	assert.Len(t, room.Corpses[0].Loot.Items, 1, "the lookout is untouched")
	assert.Empty(t, room.Corpses[1].Loot.Items, "the skeleton is emptied")
	require.Len(t, user.Character.Items, 1)
	assert.Equal(t, 20001, user.Character.Items[0].ItemId)
}
