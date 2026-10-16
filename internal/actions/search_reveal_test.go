package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevealSpotted_PlayerSearcherEndsAHiddenMobsHiding(t *testing.T) {
	room := hiddenMobRoom(t, 7901, 7902, 0)
	seeker := newSearchFakeActor("Seeker", room, true, 0)

	revealSpotted(seeker, SearchResult{HiddenMobsFound: []int{7902}}, room)
	require.False(t, mobs.GetInstance(7902).Character.IsHidden(),
		"a player's find drags the mob out of hiding for everyone")
}

func TestRevealSpotted_MobSearcherEndsNothing(t *testing.T) {
	room := hiddenMobRoom(t, 7903, 7904, 0)
	scout := newSearchFakeActor("Scout", room, false, 0)

	revealSpotted(scout, SearchResult{HiddenMobsFound: []int{7904}}, room)
	require.True(t, mobs.GetInstance(7904).Character.IsHidden(),
		"mobs perceiving hidden creatures is slice F")
}

func revealPlayerScene(t *testing.T, biome string) (*rooms.Room, *users.UserRecord) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave":    {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
		"city":    {BiomeId: "city"},
		"default": {BiomeId: "default"},
	}))
	hider := users.NewTestUser(7911, "kesh", "Kesh", 97911)
	viewerTestHide(t, hider.Character)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7911: hider}))
	room := &rooms.Room{RoomId: 7910, Biome: biome}
	room.AddPlayer(7911)
	events.DrainQueuedMessagesForTest(7911)
	return room, hider
}

func revealTold(lines []string, want string) bool {
	for _, l := range lines {
		if strings.Contains(l, want) {
			return true
		}
	}
	return false
}

func TestRevealSpotted_AHiddenPlayerIsRevealedAndTold(t *testing.T) {
	room, hider := revealPlayerScene(t, "city")
	revealSpotted(newSearchFakeActor("Seeker", room, true, 0), SearchResult{HiddenPlayersFound: []int{7911}}, room)

	require.False(t, hider.Character.IsHidden())
	assert.True(t, revealTold(events.DrainQueuedMessagesForTest(7911), "searches the room and spots you!"))
}

// #371 slice 4 review: the spotted line rides the searcher's kind, Emote for
// a player and MobEmote for a mob, and a lowercase mob name still opens the
// line with a capital (neither category runs the capitalize stage).
func TestEndHidingOnSpot_RidesTheSearchersKind(t *testing.T) {
	room, hider := revealPlayerScene(t, "city")
	room.Lamp = rooms.LampPtr(90) // lit whatever the ambient hour
	endHidingOnSpot("goblin scout", false, hider.Character, hider, room)
	got := strings.Join(events.DrainQueuedMessagesForTest(7911), "")
	assert.Contains(t, got, `fg="mob-emote"`, "a mob searcher's line rides CategoryMobEmote")
	assert.Contains(t, got, `<ansi fg="mobname">Goblin scout</ansi> searches the room and spots you!`,
		"a lowercase mob name opens the line with a capital, in the mob name colour")

	room, hider = revealPlayerScene(t, "city")
	room.Lamp = rooms.LampPtr(90)
	endHidingOnSpot("Seeker", true, hider.Character, hider, room)
	got = strings.Join(events.DrainQueuedMessagesForTest(7911), "")
	assert.Contains(t, got, `fg="emote"`, "a player searcher's line rides CategoryEmote")
	assert.Contains(t, got, `<ansi fg="username">Seeker</ansi> searches the room and spots you!`)
	assert.NotContains(t, got, `fg="mob-emote"`)
}

func TestRevealSpotted_AHiderWhoCannotSeeIsNotToldTheName(t *testing.T) {
	room, hider := revealPlayerScene(t, "cave")
	revealSpotted(newSearchFakeActor("Seeker", room, true, 0), SearchResult{HiddenPlayersFound: []int{7911}}, room)

	require.False(t, hider.Character.IsHidden())
	lines := events.DrainQueuedMessagesForTest(7911)
	assert.True(t, revealTold(lines, "Someone searches the room and spots you!"))
	assert.False(t, revealTold(lines, "Seeker"))
}
