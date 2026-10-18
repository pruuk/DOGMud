package follow

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-10-17 (the Malia party playtest): following keys off the
// FOLLOWED person's light. A follower who carries no light follows one who
// carries a light; nobody follows an unlit leader in true darkness. `follow`
// works inside a party, and says so when you already follow that person.

const followLanternId = 9618

// followWalkScene is a square (room 1) with a north exit to a cave (room 2),
// Aliceia (user 1, the follower) and Bobrick (user 2) in the square. lit
// lamps both rooms; otherwise both are pitch dark.
func followWalkScene(t *testing.T, lit bool) (*FollowModule, *users.UserRecord, *users.UserRecord, *rooms.Room, *rooms.Room) {
	t.Helper()
	t.Cleanup(keywords.SeedKeywordsForTest())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		followLanternId: {ConditionId: followLanternId, Name: "Test Lantern", TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 54}}},
	}))
	u1 := users.NewTestUser(1, "alice", "Aliceia", 1001)
	u1.Character.RoomId = 1
	u2 := users.NewTestUser(2, "bob", "Bobrick", 1002)
	u2.Character.RoomId = 1
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u1, 2: u2}))
	square := &rooms.Room{RoomId: 1, Zone: "TestZone", Title: "Square", Biome: "default",
		Exits: map[string]exit.RoomExit{"north": {RoomId: 2}}}
	cave := &rooms.Room{RoomId: 2, Zone: "TestZone", Title: "Cave", Biome: "default",
		Exits: map[string]exit.RoomExit{"south": {RoomId: 1}}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1: square, 2: cave}, map[string]*rooms.ZoneConfig{
		"TestZone": {Name: "TestZone", RoomId: 1, RoomIds: map[int]struct{}{1: {}, 2: {}}},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default", Name: "Default", Symbol: ".", MovementCost: 1.0, SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	square.AddPlayer(1)
	square.AddPlayer(2)
	for _, r := range []*rooms.Room{square, cave} {
		if lit {
			r.Lamp = rooms.LampPtr(60)
		} else {
			r.SkyLight, r.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
			require.Equal(t, 0, r.LightLevel(), "fixture premise: pitch dark")
		}
	}
	t.Cleanup(func() {
		for _, uid := range []int{1, 2} {
			if p := parties.Get(uid); p != nil {
				p.Disband()
			}
		}
	})
	f := &FollowModule{
		plug:         plugins.New(`followlighttest`, `1.0`),
		followed:     make(map[followId][]followId),
		followers:    make(map[followId]followId),
		followLimits: make(map[followId]uint64),
	}
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	events.DrainQueuedUserInputsForTest(1)
	return f, u1, u2, square, cave
}

// walkNorth moves u from the square to the cave and runs the follow
// module's room-change handler as the event queue would.
func walkNorth(f *FollowModule, u *users.UserRecord, square, cave *rooms.Room) {
	square.RemovePlayer(u.UserId)
	cave.AddPlayer(u.UserId)
	u.Character.RoomId = cave.RoomId
	f.roomChangeHandler(events.RoomChange{UserId: u.UserId, FromRoomId: square.RoomId, ToRoomId: cave.RoomId})
}

func TestFollowLight_UnlitFollowerFollowsALitLeader(t *testing.T) {
	f, u1, u2, square, cave := followWalkScene(t, false)
	require.True(t, u2.Character.Conditions.AddCondition(followLanternId, true))
	require.Empty(t, u1.Character.Conditions.LightAndDarknessSources(), "fixture premise: the follower carries no light")
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)

	walkNorth(f, u2, square, cave)

	assert.Equal(t, []string{"north"}, events.DrainQueuedUserInputsForTest(1), "the follower steps after the lit leader")
	assert.True(t, f.isFollowing(followId{userId: 1}))
}

func TestFollowLight_UnlitFollowerLosesAnUnlitLeaderInTrueDarkness(t *testing.T) {
	f, u1, u2, square, cave := followWalkScene(t, false)
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)

	walkNorth(f, u2, square, cave)

	assert.Empty(t, events.DrainQueuedUserInputsForTest(1), "nobody follows an unlit leader in true darkness")
	assert.False(t, f.isFollowing(followId{userId: 1}), "losing sight of the leader ends the follow")
	assert.Contains(t, followTold(1), FollowLostLine)
	recipient := followTold(2)
	assert.Contains(t, recipient, "Someone stopped following you.")
	assert.NotContains(t, recipient, "Aliceia")
	_ = u1
}

func TestFollowLight_FollowWorksInsideAParty(t *testing.T) {
	f, u1, _, square, _ := followWalkScene(t, true)
	p := parties.New(1)
	require.NotNil(t, p)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))

	_, _ = f.followUserCommand("bobrick", u1, square, events.EventFlag(0))

	assert.True(t, f.isFollowing(followId{userId: 1}), "a party leader can follow a member")
	told := followTold(1)
	assert.Contains(t, told, "You start following Bobrick.")
	assert.NotContains(t, told, "while in a party")
}

func TestFollowLight_AMemberAlreadyFollowsTheirPartyLeader(t *testing.T) {
	f, u1, _, square, _ := followWalkScene(t, true)
	p := parties.New(2)
	require.NotNil(t, p)
	require.True(t, p.InvitePlayer(1))
	require.True(t, p.AcceptInvite(1))

	_, _ = f.followUserCommand("bobrick", u1, square, events.EventFlag(0))

	assert.False(t, f.isFollowing(followId{userId: 1}), "no second follow on top of party auto-follow")
	assert.Contains(t, followTold(1), "You're already following Bobrick.")
	assert.Empty(t, followTold(2), "the leader is told nothing new")
}

func TestFollowLight_FollowingTheSamePersonTwiceSaysSo(t *testing.T) {
	f, u1, _, square, _ := followWalkScene(t, true)
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)

	_, _ = f.followUserCommand("bobrick", u1, square, events.EventFlag(0))

	assert.True(t, f.isFollowing(followId{userId: 1}))
	assert.Contains(t, followTold(1), "You're already following Bobrick.")
	assert.Empty(t, followTold(2), "the target is not told again")
}

// A party leader following a member: the leader walks, party auto-follow
// brings the member, and the member's arrival must not end the leader's
// follow (the leader is already where the member went).
func TestFollowLight_APartyWalkingTogetherKeepsTheFollow(t *testing.T) {
	f, u1, u2, square, cave := followWalkScene(t, true)
	p := parties.New(1)
	require.NotNil(t, p)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)

	// The leader walks first; then the member's auto-follow step lands.
	square.RemovePlayer(1)
	cave.AddPlayer(1)
	u1.Character.RoomId = cave.RoomId
	walkNorth(f, u2, square, cave)

	assert.Empty(t, events.DrainQueuedUserInputsForTest(1), "no stray step or follow stop for the leader")
	assert.True(t, f.isFollowing(followId{userId: 1}), "the follow survives the party walking together")
}
