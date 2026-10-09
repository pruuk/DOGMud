package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #251: a scout tree's try_scan (actTryScan) promotes the first hostile
// sighting of actions.Scan to its SoftTarget, and every player is hostile
// (isHostileToMob). Scan's structured result follows the scanner's sight
// (scanReach, then listedOccupants); these tests drive the btree action
// itself, so a scout cannot pick a target it cannot make out.
const (
	scoutNextRoomId = 8102
	scoutPlayerId   = 8160
)

// scoutScanScene is sightScene's scout, standing in the lit room 8100 (city,
// lamp 90) with Night Vision as a goblin scout has, beside room 8102 (cave,
// lamp nextLamp) to the north, which holds one player and nobody else.
func scoutScanScene(t *testing.T, nextLamp int) (*EvalContext, *users.UserRecord) {
	t.Helper()
	m, here := sightScene(t, "city")
	require.NoError(t, m.Character.AddCondition(sightNightVisionConditionId, true))
	here.Exits = map[string]exit.RoomExit{"north": {RoomId: scoutNextRoomId}}
	next := &rooms.Room{RoomId: scoutNextRoomId, Biome: "cave", Lamp: rooms.LampPtr(nextLamp),
		Exits: map[string]exit.RoomExit{"south": {RoomId: here.RoomId}}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{here.RoomId: here, scoutNextRoomId: next},
		map[string]*rooms.ZoneConfig{}))
	require.Equal(t, nextLamp, next.LightLevel(), "fixture: the next room's light is pinned")

	u := users.NewTestUser(scoutPlayerId, "kesh", "Kesh", 98160)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{scoutPlayerId: u}))
	u.Character.RoomId = scoutNextRoomId
	next.AddPlayer(scoutPlayerId)

	return &EvalContext{InstanceId: m.InstanceId, RoomId: here.RoomId}, u
}

// The control: a visible player in a lit next room is promoted. Without it
// the two refusals below could pass on a scene the scan never reaches.
func TestActTryScan_PromotesAVisiblePlayerInALitRoom(t *testing.T) {
	ctx, u := scoutScanScene(t, 90)
	require.Equal(t, Success, actTryScan(map[string]any{}, ctx))
	require.Equal(t, state.ActorRef{UserId: u.UserId}, ctx.SoftTarget)
}

// A visible, unlit player in a pitch-dark next room is out of the scout's
// sight: Night Vision shifts the window but cannot lift light 0.
func TestActTryScan_IgnoresAPlayerInAPitchDarkRoom(t *testing.T) {
	ctx, _ := scoutScanScene(t, 0)
	require.Equal(t, Failure, actTryScan(map[string]any{}, ctx))
	require.True(t, ctx.SoftTarget.IsZero(), "no SoftTarget for a player the scout cannot see")
}

// A sneak-hidden player in a lit next room is never a sighting: the scout
// has no see-hidden, so it does not perceive them.
func TestActTryScan_IgnoresAHiddenPlayerInALitRoom(t *testing.T) {
	ctx, u := scoutScanScene(t, 90)
	hideForTest(t, u.Character)
	require.Equal(t, Failure, actTryScan(map[string]any{}, ctx))
	require.True(t, ctx.SoftTarget.IsZero(), "no SoftTarget for a player the scout does not perceive")
}
