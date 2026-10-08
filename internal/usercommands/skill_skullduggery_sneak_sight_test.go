package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Sight gates close-out, #215: a sneaker spotted in pitch dark by an observer
// who heard them is not told the observer's name ("... but Sil Vantage
// notices you." in the 2026-09-30 playtest). The name follows the sneaker's
// own sight: here, none.
func TestSneakCommand_SpotterNotNamedInPitchDark(t *testing.T) {
	const superCond = 9941
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0
	c.Balance.ContestGapSaturation = 0
	configs.SetConfigForTest(t, c)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		superCond: {ConditionId: superCond, Name: "Test Sharp Ears", RoundInterval: 1, TriggerCount: 50,
			Flags: []conditions.Flag{conditions.SuperHearing}},
	}))

	room := &rooms.Room{RoomId: 9940, Zone: "SneakSight", SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(0)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9940: room}, map[string]*rooms.ZoneConfig{}))

	sneaker := users.NewTestUser(9942, "sneak", "Sneak", 0)
	sneaker.Character.Skills = map[string]int{string(skills.Skullduggery): 1}
	sneaker.Character.Stats.Dexterity.ValueAdj = 0
	watcher := users.NewTestUser(9943, "watcher", "Watcher", 0)
	watcher.Character.Stats.Perception.ValueAdj = 1000
	require.True(t, watcher.Character.Conditions.AddCondition(superCond, true))
	for _, u := range []*users.UserRecord{sneaker, watcher} {
		u.Character.RoomId = room.RoomId
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9942: sneaker, 9943: watcher}))
	room.AddPlayer(9942)
	room.AddPlayer(9943)
	events.DrainQueuedMessagesForTest(9942)

	handled, err := Sneak("", sneaker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(
		strings.Join(events.DrainQueuedMessagesForTest(9942), "\n"), "")
	require.Contains(t, out, "You try to blend into the shadows but something notices you.")
	require.NotContains(t, out, "Watcher", "the sneaker sees nothing, so learns no name")
}
