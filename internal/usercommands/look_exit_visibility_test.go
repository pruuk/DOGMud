package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// File: look_exit_visibility_test.go
//
// Graded lighting arc, plan 1 Task 5: look.go now reads two different
// thresholds off the same LightLevel() value. LightBlindBelow gates seeing
// the room at all; LightExitsAbove separately gates seeing THROUGH an exit.
// The parity golden (internal/guards/lighting_parity_golden_test.go) only covers sight
// tiers, not exit-peering, so this is the one automated check for the
// look.go:258 site.
//
// TestLookExit_RoomOnlyLightBlocksExitPeering pins the band the old model
// could not name explicitly: light enough to see the room (at or above
// LightBlindBelow, default 25) but not enough to see down an exit (below
// LightExitsAbove, default 65). The old model expressed this exact band as
// visibility 1: enough to pass the "can't see anything" gate but not the
// "too dark to see anything in that direction" gate.
//
// 🔑 Since graded lighting plan 3a Task 8, LightLevel() composes a
// continuous value rather than landing on one of three named points, so the
// fixture is asserted against the BAND (the property this test actually
// needs), not a fixed constant. A dark biome (cave, sky fraction 0) plus one
// carried light source lands exactly on LightDimBelow (the model lifts a lit
// room to the bottom of the "room readable, exits not" band; see
// internal/rooms/lighting.go's composeLight, term 3), which is
// inside the band asserted below by construction.
func TestLookExit_RoomOnlyLightBlocksExitPeering(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	const torchConditionId = 9101
	restoreConditions := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		torchConditionId: {
			ConditionId:   torchConditionId,
			Name:          "Test Torch",
			RoundInterval: 1,
			TriggerCount:  1,
			Effects:       map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 50}},
		},
	})
	defer restoreConditions()

	user := users.GetByUserId(1)
	require.NotNil(t, user)

	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	room.Biome = "cave"

	user.Character.RoomId = 2
	room.AddPlayer(user.UserId)
	require.NoError(t, user.Character.AddCondition(torchConditionId, false))

	cfg := configs.GetLightingConfig()
	light := room.LightLevel()
	require.True(t, light >= cfg.BlindBelow && light < cfg.ExitsAbove,
		"fixture must land in the room-only band (>= BlindBelow %d, < ExitsAbove %d), got %d; dark biome + a light source, or this test proves nothing",
		cfg.BlindBelow, cfg.ExitsAbove, light)

	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Look("south", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	lines := events.DrainQueuedMessagesForTest(user.UserId)
	require.NotEmpty(t, lines, "look must still respond, since the room itself is lit enough to see")
	joined := strings.Join(lines, "\n")
	require.Contains(t, joined, "too dark to see anything in that direction",
		"room-only light must still block seeing THROUGH an exit")
}

// TestLookExit_FullLightAllowsExitPeering is the sibling positive case: a
// room lit at or above LightExitsAbove must NOT be refused. Without this, a
// defect that always blocks exit-peering would still pass the test above (it
// only checks the blocked case).
//
// The fixture room is whatever biome the seeded test room carries, at a
// pinned, guaranteed daytime round (noon). Since graded lighting plan 3a
// Task 8, LightLevel() at noon is the celestial term (sun near its daily
// peak) attenuated by the room's own sky fraction, so the exact value is not
// a fixed constant; the fixture is asserted against the ExitsAbove
// threshold, the property this test actually needs, and the assertion below
// fails loudly with the real value if the fixture ever stops landing there.
func TestLookExit_FullLightAllowsExitPeering(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	// Midsummer noon, not just any noon: GameDate's day-of-year is
	// floor(round/RoundsPerDay)+1 (internal/gametime/gametime.go), so day 172
	// (midsummer) at roundOfDay 10 (noon, half of RoundsPerDay=20) is round
	// (172-1)*20+10 = 3430. A noon picked at an arbitrary day of year is not
	// enough: this fixture's room has an open (unattenuated) sky, and at this
	// world's latitude an equinox or midwinter noon reads in the low 60s,
	// UNDER LightExitsAbove's default 65, which would make this "positive
	// case" fail for a reason that has nothing to do with the exit-peering
	// gate under test. Midsummer noon is the celestial model's brightest
	// point in the year, comfortably above ExitsAbove.
	const middayOfMidsummer = uint64(3430)
	util.SetRoundCountForTest(middayOfMidsummer)
	t.Cleanup(util.ResetRoundCountForTest)

	user, room := getTestUserAndRoom(t)
	lcfg := configs.GetLightingConfig()
	light := room.LightLevel()
	require.GreaterOrEqual(t, light, lcfg.ExitsAbove,
		"fixture room must be at or above ExitsAbove (%d), got %d; or this test proves nothing",
		lcfg.ExitsAbove, light)

	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Look("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	lines := events.DrainQueuedMessagesForTest(user.UserId)
	joined := strings.Join(lines, "\n")
	require.NotContains(t, joined, "too dark to see anything in that direction",
		"full light must not block seeing THROUGH an exit")
}
