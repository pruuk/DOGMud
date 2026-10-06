package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: darkness_gates_sight_test.go
//
// Lighting plan 5c final review, finding 1. The player darkness gates on
// look, look-in-a-direction, get and loot tested the nightvision FLAG, not
// the observer's sight. An infravision character (flag infraredvision only,
// with a reach) was told "You can't see anything!" in a faint room where the
// Game window gave them shapes, and a nightvision holder at light 0 passed
// every gate although messaging.ParticipantSight has read them as SightNone
// there since plan 2. Each gate now refuses exactly when the observer's sight
// is SightNone.

const (
	gateInfraConditionId = 9611
	gateNightConditionId = 9612
	gateNightFlagOnlyId  = 9613
)

// seedDarknessGateRoom puts user 1 alone in the seeded cave room at a pinned
// light, with the three vision conditions registered. The cave biome has no
// sky, so the room's lamp alone decides the light.
func seedDarknessGateRoom(t *testing.T, lamp int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)

	restoreConditions := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		gateInfraConditionId: {ConditionId: gateInfraConditionId, Name: "Test Heat Sight", RoundInterval: 1, TriggerCount: 10,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		gateNightConditionId: {ConditionId: gateNightConditionId, Name: "Test Strong Night Sight", RoundInterval: 1, TriggerCount: 10,
			Flags:   []conditions.Flag{conditions.NightVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectNightVisionStrength: {Literal: 24}}},
		gateNightFlagOnlyId: {ConditionId: gateNightFlagOnlyId, Name: "Test Bare Night Sight", RoundInterval: 1, TriggerCount: 10,
			Flags: []conditions.Flag{conditions.NightVision}},
	})
	t.Cleanup(restoreConditions)

	user := users.GetByUserId(1)
	require.NotNil(t, user)
	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	room.Biome = "cave"
	room.Lamp = rooms.LampPtr(lamp)

	rooms.LoadRoom(1).RemovePlayer(user.UserId)
	user.Character.RoomId = 2
	room.AddPlayer(user.UserId)
	require.Equal(t, lamp, room.LightLevel(), "fixture room must sit at the pinned light")

	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

func runGate(t *testing.T, user *users.UserRecord, cmd func() (bool, error)) string {
	t.Helper()
	events.DrainQueuedMessagesForTest(user.UserId)
	handled, err := cmd()
	require.NoError(t, err)
	require.True(t, handled)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
}

// gateOutputs runs look, get and loot and returns each one's output.
func gateOutputs(t *testing.T, user *users.UserRecord, room *rooms.Room) map[string]string {
	t.Helper()
	return map[string]string{
		"look": runGate(t, user, func() (bool, error) { return Look("nothing_here", user, room, 0) }),
		"get":  runGate(t, user, func() (bool, error) { return Get("", user, room, 0) }),
		"loot": runGate(t, user, func() (bool, error) { return Loot("", user, room, 0) }),
	}
}

var gateRefusals = map[string]string{
	"look": tooDarkToSeeLine,
	"get":  "You can't see anything to pick up!",
	"loot": "You can't see anything to loot!",
}

func TestDarknessGates_ReadSightNotTheNightVisionFlag(t *testing.T) {
	cases := []struct {
		name      string
		lamp      int
		condition int // 0 = none
		refused   bool
	}{
		{"normal eyes at light 10 are refused", 10, 0, true},
		{"infravision at light 10 reads shapes and is let through", 10, gateInfraConditionId, false},
		{"infravision below minus its reach is refused", -40, gateInfraConditionId, true},
		{"nightvision at light 0 is refused (the window has a floor)", 0, gateNightConditionId, true},
		{"bare nightvision flag at light 10 is refused (default shift 12 leaves 13)", 10, gateNightFlagOnlyId, true},
		{"strong nightvision at light 10 reads shapes and is let through", 10, gateNightConditionId, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, room := seedDarknessGateRoom(t, c.lamp)
			if c.condition != 0 {
				require.True(t, user.Character.Conditions.AddCondition(c.condition, true))
			}
			for verb, out := range gateOutputs(t, user, room) {
				if c.refused {
					require.Contains(t, out, gateRefusals[verb], "%s must refuse", verb)
				} else {
					require.NotContains(t, out, gateRefusals[verb], "%s must not refuse", verb)
				}
			}
		})
	}
}

// Seeing THROUGH an exit keeps LightExitsAbove as its threshold for normal
// eyes. Nightvision shifts that edge down exactly as it shifts the blind and
// dim edges (by its strength, capped at the window shift cap); infra reach
// reads heat in the observer's own room and does not reach the next one.
func TestLookDirection_ExitThresholdShiftsWithNightVisionStrength(t *testing.T) {
	const tooDark = "too dark to see anything in that direction"
	cases := []struct {
		name      string
		lamp      int
		condition int
		refused   bool
	}{
		{"normal eyes at 45 are refused (edge 65)", 45, 0, true},
		{"strength 24 at 45 peers (edge 41)", 45, gateNightConditionId, false},
		{"bare flag at 45 is refused (edge 53)", 45, gateNightFlagOnlyId, true},
		{"infravision at 45 is refused (heat does not reach the next room)", 45, gateInfraConditionId, true},
		{"strength 24 at light 0 is refused (it could see nothing here)", 0, gateNightConditionId, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, room := seedDarknessGateRoom(t, c.lamp)
			if c.condition != 0 {
				require.True(t, user.Character.Conditions.AddCondition(c.condition, true))
			}
			out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
			if c.refused {
				require.True(t, strings.Contains(out, tooDark) || strings.Contains(out, tooDarkToSeeLine),
					"must refuse; got:\n%s", out)
			} else {
				require.NotContains(t, out, tooDark)
				require.NotContains(t, out, tooDarkToSeeLine)
				require.Contains(t, out, "You peer toward the south")
			}
		})
	}
}
