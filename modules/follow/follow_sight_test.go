package follow

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #454: `follow <name>` resolves a typed name only at full sight, as every
// other command that names a creature does (actions.AimBySight). The scene is
// room 1 with Aliceia (user 1, the follower) and Bobrick (user 2, the target).

const followInfraredConditionId = 9455

type followBand int

const (
	followFull followBand = iota
	followShapes
	followDark
)

var followTagPattern = regexp.MustCompile(`<[^>]*>`)

func followTold(userId int) string {
	return followTagPattern.ReplaceAllString(strings.Join(events.DrainQueuedMessagesForTest(userId), ""), "")
}

func followScene(t *testing.T, band followBand) (*FollowModule, *users.UserRecord, *users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(keywords.SeedKeywordsForTest())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		followInfraredConditionId: {ConditionId: followInfraredConditionId, Name: "Test Infrared",
			RoundInterval: 1, TriggerCount: 1, Flags: []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	u1 := users.NewTestUser(1, "alice", "Aliceia", 1001)
	u1.Character.RoomId = 1
	u2 := users.NewTestUser(2, "bob", "Bobrick", 1002)
	u2.Character.RoomId = 1
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u1, 2: u2}))
	room := &rooms.Room{RoomId: 1, Zone: "TestZone", Title: "Square", Biome: "default"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room}, map[string]*rooms.ZoneConfig{
		"TestZone": {Name: "TestZone", RoomId: 1, RoomIds: map[int]struct{}{1: {}}},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default", Name: "Default", Symbol: ".", MovementCost: 1.0, SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	room.AddPlayer(1)
	room.AddPlayer(2)
	if band == followFull {
		room.Lamp = rooms.LampPtr(60)
	} else {
		room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
		require.Equal(t, 0, room.LightLevel(), "the dark bands need a pitch-dark room")
	}
	if band == followShapes {
		require.True(t, u1.Character.Conditions.AddCondition(followInfraredConditionId, true))
	}
	f := &FollowModule{
		plug:         plugins.New(`followsighttest`, `1.0`),
		followed:     make(map[followId][]followId),
		followers:    make(map[followId]followId),
		followLimits: make(map[followId]uint64),
	}
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return f, u1, u2, room
}

func TestFollowSight_ClearSightStartsTheFollow(t *testing.T) {
	f, u1, _, room := followScene(t, followFull)
	_, _ = f.followUserCommand("bobrick", u1, room, events.EventFlag(0))
	assert.True(t, f.isFollowing(followId{userId: 1}), "a lit room follows a named player")
	assert.Contains(t, followTold(1), "You start following Bobrick.")
	assert.Contains(t, followTold(2), "Aliceia is following you.")
}

func TestFollowSight_NoSightFollowsNoOneAndNamesNoOne(t *testing.T) {
	f, u1, _, room := followScene(t, followDark)
	_, _ = f.followUserCommand("bobrick", u1, room, events.EventFlag(0))
	assert.False(t, f.isFollowing(followId{userId: 1}), "a follow started in the dark")
	told := followTold(1)
	assert.Contains(t, told, actions.AimNotHereLine)
	assert.NotContains(t, told, "Bobrick")
	assert.Empty(t, followTold(2), "the target was told someone followed them")
}

func TestFollowSight_ShapesHintsAName(t *testing.T) {
	f, u1, _, room := followScene(t, followShapes)
	_, _ = f.followUserCommand("bobrick", u1, room, events.EventFlag(0))
	assert.False(t, f.isFollowing(followId{userId: 1}), "a typed name followed at shapes")
	told := followTold(1)
	assert.Contains(t, told, "You can only make out shapes here.")
	assert.Contains(t, told, "follow 2.shape")
	assert.NotContains(t, told, "Bobrick")
}

func TestFollowSight_ShapesFollowsAShapeAndNamesNoOne(t *testing.T) {
	f, u1, _, room := followScene(t, followShapes)
	_, _ = f.followUserCommand("1.shape", u1, room, events.EventFlag(0))
	assert.True(t, f.isFollowing(followId{userId: 1}), "shape 1 is Bobrick")
	told := followTold(1)
	assert.Contains(t, told, "You start following a figure.")
	assert.NotContains(t, told, "Bobrick")
	// Bobrick has no infrared: he sees nothing, and reads "Someone".
	recipient := followTold(2)
	assert.Contains(t, recipient, "Someone is following you.")
	assert.NotContains(t, recipient, "Aliceia")
}

// stop and lose resolve no target, so they work in the dark.
func TestFollowSight_StopAndLoseNeedNoSight(t *testing.T) {
	f, u1, _, room := followScene(t, followDark)
	_, _ = f.followUserCommand("stop", u1, room, events.EventFlag(0))
	assert.Contains(t, followTold(1), "You aren't following anyone.")
	_, _ = f.followUserCommand("lose", u1, room, events.EventFlag(0))
	assert.Contains(t, followTold(1), "Nobody is following you.")
}

// #454: `follow stop` told the followed user their OWN name ("Bobrick stopped
// following you."). The line names the follower, hidden at the followed
// user's sight; the follower's own line hides the followed user's name at the
// follower's sight, as the start line does.
func TestFollowSight_StopNamesTheFollowerInALitRoom(t *testing.T) {
	f, u1, _, room := followScene(t, followFull)
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)
	_, _ = f.followUserCommand("stop", u1, room, events.EventFlag(0))
	assert.False(t, f.isFollowing(followId{userId: 1}))
	assert.Contains(t, followTold(1), "You are no longer following Bobrick.")
	followed := followTold(2)
	assert.Contains(t, followed, "Aliceia stopped following you.")
	assert.NotContains(t, followed, "Bobrick stopped")
}

func TestFollowSight_StopReadsSomeoneInTheDark(t *testing.T) {
	f, u1, _, room := followScene(t, followDark)
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)
	_, _ = f.followUserCommand("stop", u1, room, events.EventFlag(0))
	assert.False(t, f.isFollowing(followId{userId: 1}))
	follower := followTold(1)
	assert.Contains(t, follower, "You are no longer following")
	assert.NotContains(t, follower, "Bobrick")
	followed := followTold(2)
	assert.Contains(t, followed, "Someone stopped following you.")
	assert.NotContains(t, followed, "Aliceia")
	assert.NotContains(t, followed, "Bobrick")
}

// A lost follower reads the leader's name hidden at the follower's own sight.
func TestFollowSight_LoseHidesTheLeaderFromADarkFollower(t *testing.T) {
	f, u1, _, room := followScene(t, followDark)
	f.startFollow(followId{userId: 1}, followId{userId: 2}, 0)
	_, _ = f.followUserCommand("lose", u1, room, events.EventFlag(0))
	assert.False(t, f.isFollowing(followId{userId: 2}))
	told := followTold(2)
	assert.Contains(t, told, "You are no longer following")
	assert.NotContains(t, told, "Aliceia")
}

func TestFollowSight_LoseNamesTheLeaderInALitRoom(t *testing.T) {
	f, u1, _, room := followScene(t, followFull)
	f.startFollow(followId{userId: 1}, followId{userId: 2}, 0)
	_, _ = f.followUserCommand("lose", u1, room, events.EventFlag(0))
	assert.Contains(t, followTold(2), "You are no longer following Aliceia.")
}

// followMob puts a mob named Rat (instance 700) in the scene room.
func followMob(t *testing.T, room *rooms.Room) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{InstanceId: 700, Character: characters.Character{Name: "Rat", RoomId: room.RoomId, Conditions: conditions.New()}}
	mobs.SetInstanceForTest(700, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(700, nil) })
	room.AddMob(700)
	return m
}

// #454 review G1: a mob's `follow stop` told the followed player their OWN
// name ("Bobrick stopped following you."). It names the mob, hidden at the
// player's sight.
func TestFollowSight_MobStopNamesTheMobHiddenAtSight(t *testing.T) {
	for _, band := range []followBand{followFull, followDark} {
		f, _, _, room := followScene(t, band)
		rat := followMob(t, room)
		f.startFollow(followId{userId: 2}, followId{mobInstanceId: 700}, 0)
		_, _ = f.followMobCommand("stop", rat, room)
		told := followTold(2)
		assert.NotContains(t, told, "Bobrick", "band %d: the followed player read their own name", band)
		if band == followFull {
			assert.Contains(t, told, "Rat stopped following you.")
		} else {
			assert.Contains(t, told, "Something stopped following you.")
			assert.NotContains(t, told, "Rat")
		}
	}
}

// A mob's `follow lose` tells each lost player the mob's name at their sight.
func TestFollowSight_MobLoseHidesTheMobFromADarkFollower(t *testing.T) {
	for _, band := range []followBand{followFull, followDark} {
		f, _, _, room := followScene(t, band)
		rat := followMob(t, room)
		f.startFollow(followId{mobInstanceId: 700}, followId{userId: 2}, 0)
		_, _ = f.followMobCommand("lose", rat, room)
		told := followTold(2)
		if band == followFull {
			assert.Contains(t, told, "You are no longer following Rat.")
		} else {
			assert.Contains(t, told, "You are no longer following something.")
			assert.NotContains(t, told, "Rat")
		}
	}
}

// A follow that runs out on a new round tells each side the other's name at
// its own sight: player following player, mob following player, player
// following mob.
func TestFollowSight_ExpiryLinesHideNamesAtSight(t *testing.T) {
	for _, band := range []followBand{followFull, followDark} {
		f, _, _, room := followScene(t, band)
		followMob(t, room)
		f.startFollow(followId{userId: 2}, followId{userId: 1}, 1)
		f.onNewRound(events.NewRound{RoundNumber: 2})
		follower, followed := followTold(1), followTold(2)
		if band == followFull {
			assert.Contains(t, follower, "You are no longer following Bobrick.")
			assert.Contains(t, followed, "Aliceia stopped following you.")
		} else {
			assert.NotContains(t, follower, "Bobrick", "a dark follower read the name at expiry")
			assert.Contains(t, followed, "Someone stopped following you.")
			assert.NotContains(t, followed, "Aliceia")
		}

		f.startFollow(followId{userId: 2}, followId{mobInstanceId: 700}, 1)
		f.onNewRound(events.NewRound{RoundNumber: 2})
		followed = followTold(2)
		if band == followFull {
			assert.Contains(t, followed, "Rat stopped following you.")
		} else {
			assert.Contains(t, followed, "Something stopped following you.")
		}

		f.startFollow(followId{mobInstanceId: 700}, followId{userId: 1}, 1)
		f.onNewRound(events.NewRound{RoundNumber: 2})
		follower = followTold(1)
		if band == followFull {
			assert.Contains(t, follower, "You are no longer following Rat.")
		} else {
			assert.Contains(t, follower, "You are no longer following something.")
		}
	}
}

// Stop after following a shape: the follower never learned the name.
func TestFollowSight_StopAfterFollowingAShapeReadsAFigure(t *testing.T) {
	f, u1, _, room := followScene(t, followShapes)
	_, _ = f.followUserCommand("1.shape", u1, room, events.EventFlag(0))
	require.True(t, f.isFollowing(followId{userId: 1}))
	followTold(1)
	followTold(2)
	_, _ = f.followUserCommand("stop", u1, room, events.EventFlag(0))
	told := followTold(1)
	assert.Contains(t, told, "You are no longer following a figure.")
	assert.NotContains(t, told, "Bobrick")
}
