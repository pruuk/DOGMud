package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// wanderTelemetryFixture: the package fixture with telemetry on, the
// Skeleton (instance 100, room 1) rested and free to wander, and room 1's
// only exit north into room 2 in the same zone.
func wanderTelemetryFixture(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.AmbientTelemetry = true
	cfg.Balance.PackRoamingEnabled = true
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())

	mob, room := getTestMobAndRoom(t)
	north, ok := room.Exits["north"]
	require.True(t, ok, "fixture premise: room 1 has a north exit")
	origExits := room.Exits
	room.Exits = map[string]exit.RoomExit{"north": north}
	t.Cleanup(func() { room.Exits = origExits })
	mob.Character.Zone = rooms.LoadRoom(north.RoomId).Zone

	mob.MaxWander = -1
	mob.WanderCount = 0
	mob.Character.ActionPointsMax.Value = 200
	mob.Character.ActionPointsSettled = false
	mob.Character.StaminaMax.Value = 100
	mob.Character.Stamina = 100
	return mob, room
}

// Telemetry counts a wander that leaves the room, once, in the room it
// leaves.
func TestWanderTelemetry_StepCountsOnce(t *testing.T) {
	mob, room := wanderTelemetryFixture(t)
	_, err := Wander("", mob, room)
	require.NoError(t, err)
	require.Equal(t, 1, ambient.MovesForTest(1)[ambient.MoveIdle])
}

// A wander that goes nowhere is no move: telemetry counts only a committed
// wander step.
func TestWanderTelemetry_NoStepCountsNothing(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(m *mobs.Mob)
	}{
		{"too tired to pay the step", func(m *mobs.Mob) { m.Character.ActionPointsMax.Value = 5 }},
		{"MaxWander 0 never wanders", func(m *mobs.Mob) { m.MaxWander = 0 }},
		{"scattered pack member", func(m *mobs.Mob) { m.ScatterRounds = 3 }},
		{"non-alpha pack member", func(m *mobs.Mob) { m.PackAlphaId = 200 }},
		{"only exit leaves the zone", func(m *mobs.Mob) { m.Character.Zone = "ElsewhereZone" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mob, room := wanderTelemetryFixture(t)
			tc.setup(mob)
			_, err := Wander("", mob, room)
			require.NoError(t, err)
			require.Zero(t, ambient.MovesForTest(1)[ambient.MoveIdle])
		})
	}
}

// A pack alpha's wander counts the alpha and each follower that goes with
// it; a follower that cannot pay its step stays behind and is not counted.
func TestWanderTelemetry_PackFollowersCountWhenTheyMove(t *testing.T) {
	for _, tc := range []struct {
		name          string
		followerAPMax int
		want          int
	}{
		{"follower goes too", 200, 2},
		{"tired follower stays", 5, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alpha, room := wanderTelemetryFixture(t)
			alpha.IsPackAlpha = true
			f := mobs.GetInstance(200)
			require.NotNil(t, f)
			f.PackAlphaId = alpha.InstanceId
			f.MaxWander = -1
			f.Character.Zone = alpha.Character.Zone
			f.Character.ActionPointsMax.Value = tc.followerAPMax
			f.Character.ActionPointsSettled = false
			f.Character.StaminaMax.Value = 100
			f.Character.Stamina = 100

			_, err := Wander("", alpha, room)
			require.NoError(t, err)
			require.Equal(t, tc.want, ambient.MovesForTest(1)[ambient.MoveIdle])
		})
	}
}

// Sneak rides the idle movement roll but hides in place: it is no move.
func TestWanderTelemetry_SneakCountsNothing(t *testing.T) {
	mob, room := wanderTelemetryFixture(t)
	_, err := Sneak("", mob, room)
	require.NoError(t, err)
	require.Empty(t, ambient.MovesForTest(1))
}

// A mob that steps through its own dark portal changes room: counted once.
func TestPortalTelemetry_GoingThroughCountsOnce(t *testing.T) {
	mob, room := wanderTelemetryFixture(t)
	_, err := Portal("2", mob, room)
	require.NoError(t, err)
	require.Empty(t, ambient.MovesForTest(1), "opening a portal is not a move")

	_, err = Go("portal", mob, room)
	require.NoError(t, err)
	require.Equal(t, 2, mob.Character.RoomId, "fixture premise: the mob went through")
	require.Equal(t, 1, ambient.MovesForTest(1)[ambient.MoveIdle])
}
