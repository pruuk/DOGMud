package follow

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
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

// Plan D17: an NPC following a mob leader through a secret exit learns it and
// follows, as a packmate behind its alpha does.
func TestFollowSecret_AnNPCBehindAMobLeaderLearns(t *testing.T) {
	f, _, _, square, cave := followWalkScene(t, true)
	north := square.Exits["north"]
	north.Secret = true
	square.Exits["north"] = north

	alpha := &mobs.Mob{InstanceId: 9783}
	alpha.Character.Name = "Grey Wolf"
	alpha.Character.RoomId = square.RoomId
	pup := &mobs.Mob{InstanceId: 9784}
	pup.Character.Name = "Wolf Pup"
	pup.Character.RoomId = square.RoomId
	mobs.SetInstanceForTest(9783, alpha)
	mobs.SetInstanceForTest(9784, pup)
	t.Cleanup(func() {
		mobs.SetInstanceForTest(9783, nil)
		mobs.SetInstanceForTest(9784, nil)
	})
	square.AddMob(9784)
	cave.AddMob(9783)
	alpha.Character.RoomId = cave.RoomId
	f.startFollow(followId{mobInstanceId: 9783}, followId{mobInstanceId: 9784}, 0)
	events.DrainQueuedInputsForTest(9784)

	f.roomChangeHandler(events.RoomChange{MobInstanceId: 9783, FromRoomId: square.RoomId, ToRoomId: cave.RoomId})

	require.Equal(t, []string{"north"}, events.DrainQueuedInputsForTest(9784), "the pup follows its alpha")
	assert.True(t, pup.Character.KnowsSecretExit(square.RoomId, "north"), "the pup learned the exit")
	assert.True(t, f.isFollowing(followId{mobInstanceId: 9784}))
}

// #781: a follower steps through an exit the leader could have used, so it
// never learns a secret exit the leader did not take. Here a secret "crack"
// the leader never found and a temporary "gap" both lead to the cave; the
// leader went by the gap.
func TestFollowSecret_TheFollowerTakesTheLeadersExit(t *testing.T) {
	f, u1, u2, square, cave := followWalkScene(t, true)
	delete(square.Exits, "north")
	square.Exits["crack"] = exit.RoomExit{RoomId: cave.RoomId, Secret: true}
	square.ExitsTemp = map[string]exit.TemporaryRoomExit{"gap": {RoomId: cave.RoomId, Title: "a narrow gap"}}
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)

	walkNorth(f, u2, square, cave)

	assert.Equal(t, []string{"gap"}, events.DrainQueuedUserInputsForTest(1), "the follower takes the gap")
	assert.False(t, u1.Character.KnowsSecretExit(square.RoomId, "crack"), "the leader never used the crack")
}
