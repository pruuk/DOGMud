package usercommands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/achievements"
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
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

// macrosCaptured are the capture's macros, with the party invite naming a
// character at the 28-letter maximum where the capture had a legacy
// 32-letter name.
var macrosCaptured = map[string]string{
	"=1": "get all;stow;look",
	"=2": "cast identify on cuirass;appraise cuirass",
	"=3": "say meet at the old market gate at dusk, and bring a lantern, a coil of rope and your strongbox key.",
	"=4": "party invite " + strings.ToLower(goScreenLongestName) + ";guild who;who",
}

// macrosDrawn is the drawing at width with the 28-letter name for the
// 32-letter one: at 80 and 42 it takes the old name's place; at 30 it fits
// the line but not the indent of 5 and has no space to split at, so it is
// pulled left to end at the edge, as PackUnits places such a unit (where the
// drawing broke the legacy name after an underscore).
func macrosDrawn(t *testing.T, width int) string {
	drawn := drawing3b3(t, fmt.Sprintf("macros.w%d", width))
	name := strings.ToLower(goScreenLongestName)
	if width == 30 {
		return strings.Replace(drawn, "     wilhelmina_ashgrove_\n     thornbramble", strings.Repeat(" ", 30-len(name))+name, 1)
	}
	return strings.Replace(drawn, "wilhelmina_ashgrove_thornbramble", name, 1)
}

// The macros as drawn (macros in narrow-screens-3b3-mock): at 80 (a
// redesign, since a long macro ran past 80) and 42 the key at 2, its
// commands at 6 continuing at 8; at 30 at 1, 3 and 5. The closing lines
// ride CategorySystem, which the pipeline wraps.
func TestMacrosAreTheDrawing(t *testing.T) {
	user, room := macrosUser(t, macrosCaptured)
	for _, width := range []int{80, 42, 30} {
		out := runScreen(t, user, room, width, Macros, "")
		lines := plainLines(out)
		for i, l := range lines {
			lines[i] = strings.TrimRight(l, " ")
			// The pipeline wraps the two closing CategorySystem lines.
			if strings.HasPrefix(l, "To use") || strings.HasPrefix(l, "Some terminals") {
				lines[i] = messaging.WrapAnsi(l, width)
			}
		}
		got := strings.TrimRight(strings.Join(lines, "\n"), "\n")
		want := macrosDrawn(t, width)
		if got != want {
			t.Errorf("macros at %d is not the drawing:\n%s", width, lineDiffForTest(want, got))
		}
	}
}

// Every macro line fits the form at every width, a command word whole.
func TestMacrosFitEveryWidth(t *testing.T) {
	macros := map[string]string{}
	for k, v := range macrosCaptured {
		macros[k] = v
	}
	// A 13- and a 12-column word side by side at indent 5 end at 31, one
	// past a 30 line, so a packer that overfills by one is caught.
	macros["=5"] = "say abcdefghijkl nopqrstuvwxyz abcdefghijkl"
	// A word wider than the whole line (a link) breaks at the edge.
	macros["=6"] = "say https://example.com/a/very/long/link/that/has/no/spaces/at/all ok"
	user, room := macrosUser(t, macros)
	for _, width := range goScreenWidths {
		out := runScreen(t, user, room, width, Macros, "")
		formWidth := users.RedrawnFormWidth(width)
		for _, line := range plainLines(out) {
			if strings.HasPrefix(line, "To use") || strings.HasPrefix(line, "Some terminals") {
				continue // CategorySystem: the pipeline wraps them
			}
			require.LessOrEqual(t, templates.WidestLine(line), formWidth, "at %d: %q", width, line)
		}
	}
}

// --- channels ---

func TestChannelsKeepsIts80ColumnOutput(t *testing.T) {
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	user, room := getTestUserAndRoom(t)
	checkGolden(t, "channels", runScreen(t, user, room, 80, Channels, ""))
}

// The channel list as drawn (channels in narrow-screens-3b3-mock): at 42 it
// is the 80-column screen (37 wide); at 30 the heading breaks between words
// and continues 2 in. Between 30 and 37 the heading's form is the 30 one.
func TestChannelsAreTheDrawing(t *testing.T) {
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	user, room := getTestUserAndRoom(t)
	for _, width := range []int{42, 30} {
		got := strings.Join(plainLines(runScreen(t, user, room, width, Channels, "")), "\n")
		want := drawing3b3(t, fmt.Sprintf("channels.w%d", width))
		if got != want {
			t.Errorf("channels at %d is not the drawing:\n%s", width, lineDiffForTest(want, got))
		}
	}
	for width := 30; width <= 42; width++ {
		for _, line := range plainLines(runScreen(t, user, room, width, Channels, "")) {
			require.LessOrEqual(t, templates.WidestLine(line), width, "at %d: %q", width, line)
		}
	}
}

// --- skills extra ---

// `skills extra` closes with the cooldowns being tracked. It showed a raw
// round count ("special-move: 4"); it now names each timer and its time as
// the `cooldowns` screen does (cooldownsForDisplay, durationQuality), sorted
// by name, and fits every width: a row wider than the reader puts the time
// under the name, 3 in (the 30 form of `cooldowns`).
func TestSkillsExtraNamesCooldownsInWords(t *testing.T) {
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	useDogmudTemplates(t)
	user, room := getTestUserAndRoom(t)
	orig := user.Character.Cooldowns
	t.Cleanup(func() { user.Character.Cooldowns = orig })
	// The longest shipped tracking tag (internal/hooks
	// NewRound_UserRoundTick.go), 41 columns with its time.
	user.Character.Cooldowns = characters.Cooldowns{
		actions.SpecialMoveCooldownTag: 4,
		"enchant-tierup-blocked":       40,
	}
	for _, width := range []int{30, 40, 41, 42, 60, 80} {
		out := runScreen(t, user, room, width, Skills, "extra")
		lines := plainLines(out)
		at := -1
		for i, l := range lines {
			if l == "Cooldown Tracking:" {
				at = i
			}
		}
		require.GreaterOrEqual(t, at, 0, "at %d no cooldown heading:\n%s", width, out)
		tail := strings.Join(lines[at:], "\n")
		want := "Cooldown Tracking:\n enchant-tierup-blocked: well established\n special moves: briefly active"
		if width < 41 {
			want = "Cooldown Tracking:\n enchant-tierup-blocked:\n   well established\n special moves: briefly active"
		}
		require.Equal(t, want, tail, "at %d", width)
		require.NotRegexp(t, `: \d`, tail, "a raw round count")
	}
}

// --- container recipes ---

// containerRecipeItems are the capture's binding press items: the output
// (a name that opens with its own article) and three inputs.
var containerRecipeItems = map[int]*items.ItemSpec{
	97001: {ItemId: 97001, Name: "A Fair Copy of the Compiled Survey", Type: items.Object},
	97002: {ItemId: 97002, Name: "A Salt-Stiff Page of a Sailor's Telling", Type: items.Object},
	97003: {ItemId: 97003, Name: "A Stained Page with a Deathbed Word", Type: items.Object},
	97004: {ItemId: 97004, Name: "A Page About Where the Scrub Stops", Type: items.Object},
	// 37 letters: at indent 6 it ends at 43, one past a 42 line, so a
	// packer that overfills by one is caught.
	97005: {ItemId: 97005, Name: "A Ledger Kept by the Salt Way Tollmen", Type: items.Object},
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

// containerRecipeScreen is the recipe part of `look binding press` at width
// for the capture's recipe (one output from four inputs, two of them the
// same page) with two of the inputs inside: the intro and the recipe lines,
// up to the "Inside" list.
func containerRecipeScreen(t *testing.T, width int, more ...int) string {
	out := containerRecipeLook(t, width, map[int][]int{97001: append([]int{97002, 97003, 97003, 97004}, more...)}, 97002, 97003)
	lines := []string{}
	for _, l := range plainLines(out) {
		if strings.Contains(l, "┌") || strings.HasPrefix(l, "─") {
			break
		}
		lines = append(lines, l)
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

// The recipe as drawn (container_recipe in narrow-screens-3b3-mock), with
// the copy fixes: the intro ends at its colon (it read `inside:.`), and the
// output takes its article mid-sentence, `To receive a Fair Copy`, as `You
// receive` lines do (items.ArticlePrefix, items.MidSentenceName), where it
// read `To receive 1 A Fair Copy`. A name wider than the room at its indent
// splits at a space and keeps the indent (ruling 3; the drawing pulled the
// 39-letter page left to column 3). Inputs list in the recipe's order.
// A quest item's name opens with a coloured star: its article is judged on
// the name after the star, so the line reads `To receive ★an Old Key:`
// where it read `To receive a ★An Old Key:` (review of plan 3b-3).
func TestContainerRecipeArticleSkipsTheQuestStar(t *testing.T) {
	itm := items.Item{ItemId: 97010, Spec: &items.ItemSpec{ItemId: 97010, Name: "An Old Key", QuestToken: "q-1"}}
	lines := containerRecipeLines(itm.DisplayName(), nil, 80)
	require.Equal(t, []string{"    To receive ★an Old Key:"}, plainLines(strings.Join(lines, "\n")))
}

func TestContainerRecipeIsTheDrawing(t *testing.T) {
	for _, tc := range []struct {
		width int
		want  string
	}{
		{80, `You can use the binding press if you put the following objects inside:

    To receive a Fair Copy of the Compiled Survey:
        [1/1] A Salt-Stiff Page of a Sailor's Telling
        [1/2] A Stained Page with a Deathbed Word
        [0/1] A Page About Where the Scrub Stops`},
		{42, `You can use the binding press if you put
the following objects inside:

  To receive
    a Fair Copy of the Compiled Survey:
    [1/1]
      A Salt-Stiff Page of a Sailor's
      Telling
    [1/2]
      A Stained Page with a Deathbed Word
    [0/1]
      A Page About Where the Scrub Stops`},
		{30, `You can use the binding press
if you put the following
objects inside:

 To receive
   a Fair Copy of the Compiled
   Survey:
  [1/1]
    A Salt-Stiff Page of a
    Sailor's Telling
  [1/2]
    A Stained Page with a
    Deathbed Word
  [0/1]
    A Page About Where the
    Scrub Stops`},
	} {
		got := containerRecipeScreen(t, tc.width)
		if got != tc.want {
			t.Errorf("container recipe at %d is not the drawing:\n%s", tc.width, lineDiffForTest(tc.want, got))
		}
	}
}

// Every recipe line fits at every width, with an input whose name exactly
// fills a 42 line at its indent.
func TestContainerRecipeFitsEveryWidth(t *testing.T) {
	for _, width := range []int{30, 35, 42, 60, 80} {
		for _, line := range strings.Split(containerRecipeScreen(t, width, 97005), "\n") {
			require.LessOrEqual(t, templates.WidestLine(line), width, "at %d: %q", width, line)
		}
	}
}

func TestContainerRecipeKeepsIts80ColumnOutput(t *testing.T) {
	out := containerRecipeLook(t, 80, map[int][]int{97001: {97002}}, 97002)
	require.Contains(t, out, "To receive", "the recipe lines did not print")
	checkGolden(t, "container_recipe", out)
}

// drawing3b3 is the approved drawing name (say "achievements.w42"), copied
// from _archive/notes/reports/narrow-screens-3b3-mock/forms into testdata,
// as lines with no trailing CR.
func drawing3b3(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "drawings_3b3", name+".txt"))
	require.NoError(t, err)
	return strings.TrimRight(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
}

// achievementsCaptured is the screen the 3b-3 capture showed, in its order.
var achievementsCaptured = []achievementEntry{
	capturedAchievement("combat", "Cutting Teeth", 10, "Cut down a hundred foes.", "0/100"),
	capturedAchievement("combat", "Duelist", 15, "Best ten rivals in single combat.", "0/10"),
	capturedAchievement("combat", "First Blood", 5, "Defeat your first foe.", ""),
	capturedAchievement("combat", "Hard to Kill", 10, "Rise again after ten deaths.", "0/10"),
	capturedAchievement("combat", "The Last Warden", 30, "Bring down the Core Guardian.", ""),
	capturedAchievement("combat", "Veteran", 25, "Fell a thousand foes.", "0/1000"),
	capturedAchievement("exploration", "Cartographer", 20, "Chart two hundred and fifty rooms.", "2/250"),
	capturedAchievement("exploration", "Pathfinder", 10, "Explore a hundred rooms.", "2/100"),
	capturedAchievement("exploration", "The Torn Record", 25, "Read every torn page of the old account.", "0/30"),
	capturedAchievement("exploration", "Wanderer", 5, "Set foot in twenty-five rooms.", ""),
	capturedAchievement("wealth", "Coin Purse", 5, "Amass a thousand gold.", ""),
	capturedAchievement("wealth", "Magnate", 30, "Amass a hundred thousand gold.", "15250/100000"),
	capturedAchievement("wealth", "Well-Off", 15, "Amass ten thousand gold.", ""),
	capturedAchievement("progression", "Chimeric", 20, "Bear five mutations.", "0/5"),
	capturedAchievement("progression", "Decorated", 15, "Earn fifty achievement points.", "not yet"),
	capturedAchievement("progression", "Formidable", 20, "Push a talent past mortal limits.", "111/160"),
	capturedAchievement("progression", "Honed", 10, "Sharpen a talent to formidable heights.", "111/130"),
	capturedAchievement("progression", "Legend", 25, "Earn a hundred and fifty achievement points.", "not yet"),
	capturedAchievement("progression", "Skilled", 15, "Master a discipline.", "1/25"),
	capturedAchievement("progression", "The Pinnacle", 40, "Acquire a piece of legendary gear.", "not yet"),
	capturedAchievement("progression", "Twice-Touched", 10, "Bear two mutations.", "0/2"),
	capturedAchievement("quests", "A Swallowed Stone", 30, "Learn what the buried thing knows.", ""),
	capturedAchievement("quests", "Adventurer", 15, "Complete five quests.", "1/5"),
	capturedAchievement("quests", "Errand Runner", 5, "Complete your first quest.", ""),
	capturedAchievement("quests", "Hero of the Realm", 30, "Complete fifteen quests.", "1/15"),
	capturedAchievement("quests", "Key in the Dark", 20, "Finish a lost man's work.", ""),
}

// capturedAchievement is one captured row; an empty hint marks it earned.
func capturedAchievement(cat, name string, points int, desc, hint string) achievementEntry {
	return achievementEntry{
		def:    achievements.Definition{Name: name, Points: points, Description: desc, Category: cat},
		earned: hint == "",
		hint:   hint,
	}
}

// The screen as drawn (achievements in narrow-screens-3b3-mock): rows wrap
// 2 past `[x]`; a locked row's hint stays two spaces after the description
// when it fits there, else takes a line of its own at the continuation.
func TestAchievementsAreTheDrawing(t *testing.T) {
	for _, width := range []int{42, 30} {
		msgs := achievementsScreen(achievementsCaptured, 115, false, "", width)
		got := strings.Join(plainLines(strings.Join(msgs, "\n")), "\n")
		want := drawing3b3(t, fmt.Sprintf("achievements.w%d", width))
		if got != want {
			t.Errorf("achievements at %d is not the drawing:\n%s", width, lineDiffForTest(want, got))
		}
	}
}

// The shipped achievements fit at every width, every description whole
// between its line breaks, and a wide progress hint too; at 80 they are
// master's screen (TestAchievementsKeepsIts80ColumnOutput).
func TestAchievementsFitEveryWidth(t *testing.T) {
	user, room := achievementsUser(t)
	origGold := user.Character.Gold
	t.Cleanup(func() { user.Character.Gold = origGold })
	user.Character.Gold = 15250
	for _, width := range goScreenWidths {
		out := runScreen(t, user, room, width, Achievements, "")
		guildFitsAt(t, out, width)
		flat := strings.Join(strings.Fields(strings.Join(plainLines(out), " ")), " ")
		for _, d := range achievements.All() {
			require.Contains(t, flat, d.Name+" ("+strconv.Itoa(d.Points)+"): "+d.Description, "at %d", width)
		}
		require.Contains(t, flat, "(15250/100000)", "at %d", width)
	}
}
