package hooks

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Counting lines uses countContaining, which already lives in
// combat_verbosity_wiring_test.go and matches case-insensitively.

// narrationTagPattern matches one ANSI tag. Delivered lines KEEP their tags, so
// a name arrives as `<ansi fg="username">Aliceia</ansi>'s` and a plain
// substring such as "Aliceia's" never matches raw text.
var narrationTagPattern = regexp.MustCompile(`<[^>]*>`)

// plainText strips ANSI tags and surrounding whitespace from a delivered line.
func plainText(line string) string {
	return strings.TrimSpace(narrationTagPattern.ReplaceAllString(line, ""))
}

// drainPlain drains a user's queued messages and returns them tag-stripped.
func drainPlain(userId int) []string {
	raw := events.DrainQueuedMessagesForTest(userId)
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		out = append(out, plainText(line))
	}
	return out
}

// Condition ids for narration tests. Chosen well clear of the fixture's 100 and 101.
const (
	glowConditionId      = 7001 // start_observer
	shiverConditionId    = 7002 // trigger_observer, fires every round
	fadeConditionId      = 7003 // end_observer
	nightEyesConditionId = 7004 // grants NightVision; RoundInterval 0, so it never ticks
	heatEyesConditionId  = 7005 // grants InfraredVision; RoundInterval 0, so it never ticks
	lanternConditionId   = 7006 // a light source with end_observer
	dozeConditionId      = 7007 // puts the bearer to sleep; RoundInterval 0, so it never ticks
	shadeConditionId     = 7008 // end_observer with a BARE {actee_plain}, mirrors shipped condition 9
	emberConditionId     = 7009 // a light source whose end_observer has a BARE {actee_plain}, mirrors shipped condition 1
	gloomConditionId     = 7010 // a darkness source with end_observer
	wickConditionId      = 7011 // a light too faint for normal eyes, end_observer with a BARE {actee_plain}
)

// seedNarrationConditions installs the narration test conditions and returns the restore
// func. Call it AFTER `defer cleanup()` and `defer` its result, so it restores
// before the fixture does: SeedConditionsForTest replaces the whole registry.
func seedNarrationConditions() func() {
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		glowConditionId: {ConditionId: glowConditionId, Name: "Test Glow", RoundInterval: 5, TriggerCount: 3,
			StartRoomText: "{actee} glows."},
		shiverConditionId: {ConditionId: shiverConditionId, Name: "Test Shiver", RoundInterval: 1, TriggerCount: 3,
			TriggerRoomText: "{actee} shivers."},
		fadeConditionId: {ConditionId: fadeConditionId, Name: "Test Fade", RoundInterval: 5, TriggerCount: 3,
			EndRoomText: "{actee} fades."},
		nightEyesConditionId: {ConditionId: nightEyesConditionId, Name: "Test Night Eyes",
			Flags: []conditions.Flag{conditions.NightVision}},
		// GRADED LIGHTING PLAN 2: a bare InfraredVision flag reads reach 0 by
		// design (internal/characters/vision.go), so this fixture declares
		// an explicit infra_reach, matching shipped condition 85, or heat
		// eyes would stop being shapes-only in the dark at all.
		heatEyesConditionId: {ConditionId: heatEyesConditionId, Name: "Test Heat Eyes",
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		lanternConditionId: {ConditionId: lanternConditionId, Name: "Test Lantern", RoundInterval: 5, TriggerCount: 3,
			EndRoomText: "{actee}'s light gutters out.",
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 50}}},
		dozeConditionId: {ConditionId: dozeConditionId, Name: "Test Doze",
			Flags: []conditions.Flag{conditions.Sleeping}},
		shadeConditionId: {ConditionId: shadeConditionId, Name: "Test Shade", RoundInterval: 5, TriggerCount: 3,
			EndRoomText: "{actee_plain} emerges from the shadows."},
		emberConditionId: {ConditionId: emberConditionId, Name: "Test Ember", RoundInterval: 5, TriggerCount: 3,
			EndRoomText: "The glow surrounding {actee_plain} fades away.",
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 50}}},
		gloomConditionId: {ConditionId: gloomConditionId, Name: "Test Gloom", RoundInterval: 5, TriggerCount: 3,
			EndRoomText: "The gloom around {actee} lifts.",
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 90}}},
		wickConditionId: {ConditionId: wickConditionId, Name: "Test Wick", RoundInterval: 5, TriggerCount: 3,
			EndRoomText: "The wick held by {actee_plain} gutters out.",
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 10}}},
	})
}

// darken turns a fixture room into an unlit cave. The fixture seeds `cave`
// with a zero SkyLight, and LightLevel reads the biome registry, so setting
// the field is enough. Asserts the room really is unlit, so a lane cannot
// pass by accident in a lit room.
func darken(t *testing.T, roomId int) {
	t.Helper()
	room := rooms.LoadRoom(roomId)
	require.NotNil(t, room)
	room.Biome = "cave"
	require.Equal(t, 0, room.LightLevel(), "room %d must actually be unlit", roomId)
}
