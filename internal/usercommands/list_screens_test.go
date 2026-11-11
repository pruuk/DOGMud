package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/achievements"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The list screens of the second 3b-3 mock (narrow-screens-3b3-mock):
// achievements, macros, channels, `skills extra` and a container's recipes
// in look. Each is run through its real command; at 80 columns, where its
// lines fit, a golden recorded from master pins it byte for byte.

// runScreen runs cmd for user at width and returns what was queued.
func runScreen(t *testing.T, user *users.UserRecord, room *rooms.Room, width int,
	cmd func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error), rest string) string {
	t.Helper()
	orig := user.LineWidth
	t.Cleanup(func() { user.LineWidth = orig })
	user.LineWidth = width
	events.DrainQueuedMessagesForTest(user.UserId)
	handled, err := cmd(rest, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
}

// --- achievements ---

// achievementsUser is test user 1 with the shipped achievements loaded and
// the 3b-3 capture's earned ones (first-blood, the-last-warden, wanderer,
// coin-purse, well-off, a-swallowed-stone, errand-runner, key-in-the-dark)
// marked earned.
func achievementsUser(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	t.Cleanup(achievements.LoadDataFiles) // runs after the config is restored
	useDogmudTemplates(t)
	achievements.LoadDataFiles()
	require.NotEmpty(t, achievements.All(), "the shipped achievements must load")
	user, room := getTestUserAndRoom(t)
	orig := user.Character.Achievements
	t.Cleanup(func() { user.Character.Achievements = orig })
	user.Character.Achievements = map[string]uint64{}
	for _, d := range achievements.All() {
		switch d.Name {
		case "First Blood", "The Last Warden", "Wanderer", "Coin Purse", "Well-Off",
			"A Swallowed Stone", "Errand Runner", "Key in the Dark":
			user.Character.Achievements[d.Id] = 1
		}
	}
	require.Len(t, user.Character.Achievements, 8, "the captured earned achievements are not all shipped")
	return user, room
}

func TestAchievementsKeepsIts80ColumnOutput(t *testing.T) {
	user, room := achievementsUser(t)
	checkGolden(t, "achievements", runScreen(t, user, room, 80, Achievements, ""))
}

// --- macros ---

// macrosFitting are macros whose lines all fit 80 columns, the capture's
// but for =3, whose long say ran past 80.
var macrosFitting = map[string]string{
	"=1": "get all;stow;look",
	"=2": "cast identify on cuirass;appraise cuirass",
	"=4": "party invite " + strings.ToLower(goScreenLongestName) + ";guild who;who",
}

func macrosUser(t *testing.T, macros map[string]string) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	user, room := getTestUserAndRoom(t)
	orig := user.Macros
	t.Cleanup(func() { user.Macros = orig })
	user.Macros = macros
	return user, room
}

func TestMacrosKeepsIts80ColumnOutput(t *testing.T) {
	user, room := macrosUser(t, macrosFitting)
	checkGolden(t, "macros", runScreen(t, user, room, 80, Macros, ""))
}

// --- channels ---

func TestChannelsKeepsIts80ColumnOutput(t *testing.T) {
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	user, room := getTestUserAndRoom(t)
	checkGolden(t, "channels", runScreen(t, user, room, 80, Channels, ""))
}

// --- container recipes ---

// containerRecipeItems are the capture's binding press items: the output
// (a name that opens with its own article) and three inputs.
var containerRecipeItems = map[int]*items.ItemSpec{
	97001: {ItemId: 97001, Name: "A Fair Copy of the Compiled Survey", Type: items.Object},
	97002: {ItemId: 97002, Name: "A Salt-Stiff Page of a Sailor's Telling", Type: items.Object},
	97003: {ItemId: 97003, Name: "A Stained Page with a Deathbed Word", Type: items.Object},
	97004: {ItemId: 97004, Name: "A Page About Where the Scrub Stops", Type: items.Object},
}

// containerRecipeLook is `look press` in a room whose binding press takes
// one recipe of a single input, so its output order is fixed (look ranges
// over maps).
func containerRecipeLook(t *testing.T, width int, recipe map[int][]int, inside ...int) string {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	t.Cleanup(items.SeedItemsForTest(containerRecipeItems))
	useDogmudTemplates(t)
	user, room := getTestUserAndRoom(t)
	origC := room.Containers
	t.Cleanup(func() { room.Containers = origC })
	press := rooms.Container{Recipes: recipe}
	for _, id := range inside {
		press.Items = append(press.Items, items.New(id))
	}
	room.Containers = map[string]rooms.Container{"binding press": press}
	return runScreen(t, user, room, width, Look, "binding press")
}

func TestContainerRecipeKeepsIts80ColumnOutput(t *testing.T) {
	out := containerRecipeLook(t, 80, map[int][]int{97001: {97002}}, 97002)
	require.Contains(t, out, "To receive", "the recipe lines did not print")
	checkGolden(t, "container_recipe", out)
}
