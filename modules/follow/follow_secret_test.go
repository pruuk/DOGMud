package follow

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// secretNorth makes the square's north exit secret and teaches it to the
// leader (Bobrick, user 2).
func secretNorth(t *testing.T, square *rooms.Room, leader *characters.Character) {
	t.Helper()
	north := square.Exits["north"]
	north.Secret = true
	square.Exits["north"] = north
	leader.LearnSecretExit(square.RoomId, "north")
}

// Ruling 2: a player following someone through a secret exit learns it.
func TestFollowSecret_APlayerFollowerLearnsTheExit(t *testing.T) {
	f, u1, u2, square, cave := followWalkScene(t, true)
	secretNorth(t, square, u2.Character)
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)

	walkNorth(f, u2, square, cave)

	assert.Equal(t, []string{"north"}, events.DrainQueuedUserInputsForTest(1), "the follower steps after the leader")
	assert.True(t, u1.Character.KnowsSecretExit(square.RoomId, "north"), "the follower learned the exit")
}

// Ruling 13: a companion moving with its leader learns the exit; any other
// NPC following a player stops at a secret exit it has not found.
func TestFollowSecret_ACompanionLearnsAndAStrangerStops(t *testing.T) {
	f, _, u2, square, cave := followWalkScene(t, true)
	secretNorth(t, square, u2.Character)

	pet := &mobs.Mob{InstanceId: 9781}
	pet.Character.Name = "Loyal Hound"
	pet.Character.RoomId = square.RoomId
	pet.Character.Charmed = characters.NewCharm(2, -1, "")
	stray := &mobs.Mob{InstanceId: 9782}
	stray.Character.Name = "Curious Cat"
	stray.Character.RoomId = square.RoomId
	mobs.SetInstanceForTest(9781, pet)
	mobs.SetInstanceForTest(9782, stray)
	t.Cleanup(func() {
		mobs.SetInstanceForTest(9781, nil)
		mobs.SetInstanceForTest(9782, nil)
	})
	square.AddMob(9781)
	square.AddMob(9782)
	f.startFollow(followId{userId: 2}, followId{mobInstanceId: 9781}, 0)
	f.startFollow(followId{userId: 2}, followId{mobInstanceId: 9782}, 0)
	events.DrainQueuedInputsForTest(9781)
	events.DrainQueuedInputsForTest(9782)

	walkNorth(f, u2, square, cave)

	require.Equal(t, []string{"north"}, events.DrainQueuedInputsForTest(9781), "the companion follows")
	assert.True(t, pet.Character.KnowsSecretExit(square.RoomId, "north"), "the companion learned the exit")
	assert.Equal(t, []string{"follow stop"}, events.DrainQueuedInputsForTest(9782), "the stray stops")
	assert.False(t, stray.Character.KnowsSecretExit(square.RoomId, "north"))
	assert.False(t, f.isFollowing(followId{mobInstanceId: 9782}))
}
