package hooks

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
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

// Buff ids for narration tests. Chosen well clear of the fixture's 100 and 101.
const (
	glowBuffId      = 7001 // start_room_text
	shiverBuffId    = 7002 // trigger_room_text, fires every round
	fadeBuffId      = 7003 // end_room_text
	nightEyesBuffId = 7004 // grants NightVision; RoundInterval 0, so it never ticks
	heatEyesBuffId  = 7005 // grants InfraredVision; RoundInterval 0, so it never ticks
	lanternBuffId   = 7006 // a light source with end_room_text
	dozeBuffId      = 7007 // puts the bearer to sleep; RoundInterval 0, so it never ticks
)

// seedNarrationBuffs installs the narration test buffs and returns the restore
// func. Call it AFTER `defer cleanup()` and `defer` its result, so it restores
// before the fixture does: SeedBuffsForTest replaces the whole registry.
func seedNarrationBuffs() func() {
	return buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		glowBuffId: {BuffId: glowBuffId, Name: "Test Glow", RoundInterval: 5, TriggerCount: 3,
			StartRoomText: "{source} glows."},
		shiverBuffId: {BuffId: shiverBuffId, Name: "Test Shiver", RoundInterval: 1, TriggerCount: 3,
			TriggerRoomText: "{source} shivers."},
		fadeBuffId: {BuffId: fadeBuffId, Name: "Test Fade", RoundInterval: 5, TriggerCount: 3,
			EndRoomText: "{source} fades."},
		nightEyesBuffId: {BuffId: nightEyesBuffId, Name: "Test Night Eyes",
			Flags: []buffs.Flag{buffs.NightVision}},
		heatEyesBuffId: {BuffId: heatEyesBuffId, Name: "Test Heat Eyes",
			Flags: []buffs.Flag{buffs.InfraredVision}},
		lanternBuffId: {BuffId: lanternBuffId, Name: "Test Lantern", RoundInterval: 5, TriggerCount: 3,
			Flags: []buffs.Flag{buffs.EmitsLight}, EndRoomText: "{source}'s light gutters out."},
		dozeBuffId: {BuffId: dozeBuffId, Name: "Test Doze",
			Flags: []buffs.Flag{buffs.Sleeping}},
	})
}

// darken turns a fixture room into an unlit cave. The fixture seeds `cave` as
// DarkArea, and GetVisibility reads the biome registry, so setting the field is
// enough. Asserts the room really is unlit, so a lane cannot pass by accident
// in a lit room.
func darken(t *testing.T, roomId int) {
	t.Helper()
	room := rooms.LoadRoom(roomId)
	require.NotNil(t, room)
	room.Biome = "cave"
	require.Zero(t, room.GetVisibility(), "room %d must actually be unlit", roomId)
}
