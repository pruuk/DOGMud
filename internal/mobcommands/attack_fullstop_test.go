package mobcommands

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

var fullStopTagPattern = regexp.MustCompile(`<[^>]*>`)

// requireRoomAggroLinesEndWithFullStop checks every captured "prepares to
// fight" ROOM line (not the victim's own "prepares to fight you!") ends with a
// full stop once tags are stripped. The #382 playtest read "A figure prepares
// to fight a figure" with none, unlike the player command's lines.
func requireRoomAggroLinesEndWithFullStop(t *testing.T, captured *[]events.Message) {
	t.Helper()
	events.ProcessEvents()
	seen := 0
	for _, m := range *captured {
		plain := strings.TrimSpace(fullStopTagPattern.ReplaceAllString(m.Text, ""))
		if !strings.Contains(plain, "prepares to fight") || strings.Contains(plain, "prepares to fight you") {
			continue
		}
		seen++
		require.True(t, strings.HasSuffix(plain, "."), "room aggro line lacks a full stop: %q", plain)
	}
	require.NotZero(t, seen, "no room aggro line was captured; the probe cannot fail")
}

func TestMobAttackMob_RoomLineEndsWithFullStop(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	attacker, room := getTestMobAndRoom(t)
	var targetInstanceId int
	for _, id := range room.GetMobs() {
		if id != attacker.InstanceId {
			targetInstanceId = id
			break
		}
	}
	require.NotZero(t, targetInstanceId, "need a second mob in the room to attack")

	captured, _, done := captureAnnounces(t)
	defer done()

	_, err := Attack(fmt.Sprintf("#%d", targetInstanceId), attacker, room)
	require.NoError(t, err)
	requireRoomAggroLinesEndWithFullStop(t, captured)
}

func TestMobAttackPlayer_RoomLineEndsWithFullStop(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"city": {BiomeId: "city"},
	}))

	room := &rooms.Room{RoomId: 8300, Biome: "city"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{8300: room}, map[string]*rooms.ZoneConfig{}))

	m := &mobs.Mob{
		MobId: 8300, InstanceId: 8301, HomeRoomId: 8300,
		Character: characters.Character{
			Name: "Lurker", RoomId: 8300, Health: 100,
			Conditions: conditions.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	m.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(8301, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(8301, nil) })
	room.AddMob(8301)

	victim := users.NewTestUser(8310, "kesh", "Kesh", 98310)
	onlooker := users.NewTestUser(8311, "oryn", "Oryn", 98311)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8310: victim, 8311: onlooker}))
	room.AddPlayer(8310)
	room.AddPlayer(8311)

	captured, _, done := captureAnnounces(t)
	defer done()

	_, err := Attack("kesh", m, room)
	require.NoError(t, err)
	requireRoomAggroLinesEndWithFullStop(t, captured)
}
