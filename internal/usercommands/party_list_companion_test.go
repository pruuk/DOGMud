package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #244: a charmed companion is in GetCharmIds AND in Companions, so party
// list showed it twice, once as ♥friend and once as ♦companion.
func TestPartyList_CharmedCompanionListedOnce(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)

	const ownerId, compId, friendId = 9244, 9245, 9246
	u := users.NewTestUser(ownerId, "keeper", "Keeper", uint64(ownerId))
	u.Character.HealthMax.Value = 100
	u.Character.Companions = []characters.CompanionInfo{{
		MobId: 1, InstanceId: compId, Name: "Bandit Scout", SourceType: characters.CompanionCharmed,
	}}
	u.Character.CharmedMobs = []int{compId, friendId}

	mk := func(id int, name string) *mobs.Mob {
		m := &mobs.Mob{MobId: 1, InstanceId: id}
		m.Character.Name = name
		m.Character.RoomId = 1
		m.Character.Health = 50
		m.Character.HealthMax.Value = 100
		m.Character.Charm(ownerId, 100, "")
		return m
	}
	t.Cleanup(mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{
		compId:   mk(compId, "Bandit Scout"),
		friendId: mk(friendId, "Stray Hound"),
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{ownerId: u}))

	party := parties.New(ownerId)
	require.NotNil(t, party, "no party left over for this id")
	t.Cleanup(party.Disband)
	events.DrainQueuedMessagesForTest(ownerId)

	cmdPartyList(u, party)

	out := strings.Join(events.DrainQueuedMessagesForTest(ownerId), "\n")
	assert.Equal(t, 1, strings.Count(out, "Bandit Scout"), "a charmed companion is one row:\n%s", out)
	assert.Contains(t, out, "companion")
	assert.Equal(t, 1, strings.Count(out, "Stray Hound"), "a charmed creature that is no companion still shows:\n%s", out)
}
