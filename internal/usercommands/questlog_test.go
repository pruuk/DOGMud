package usercommands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// chdirRepoRootForTest points the test at the real repo config + data tree
// (test binaries run with CWD = their own package dir).
func chdirRepoRootForTest(t *testing.T) {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := wd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
}

func questlogFixture(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 42, Name: "The Hunt", Steps: []quests.QuestStep{{Id: "start"}, {Id: "track"}, {Id: "end"}}},
		&quests.Quest{QuestId: 28, Name: "Waking to Gaius", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 85, Name: "Secret", Secret: true, Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	))
	t.Cleanup(dialogue.SetOfferIndexForTest(map[int][]int{42: {9137}, 85: {9584}}))
	t.Cleanup(setGiverNameForTest(map[int]string{9137: "Hunter Delk"}))
	u := users.NewTestUser(97101, "ql", "Questlog", 97101)
	u.Character.QuestProgress = map[int]string{42: "track", 28: "start", 85: "start"}
	return u
}

func TestAbandonQuest(t *testing.T) {
	u := questlogFixture(t)

	ok, line := AbandonQuest(u, 28)
	require.False(t, ok)
	require.Equal(t, "Nobody would take this back from you. It has to be finished.", line)

	ok, _ = AbandonQuest(u, 85)
	require.False(t, ok, "a secret quest cannot be abandoned")

	events.DrainQueuedQuestsForTest()
	ok, line = AbandonQuest(u, 42)
	require.True(t, ok)
	require.Equal(t, "You set The Hunt aside. Talk to Hunter Delk to take it up again. You will pick up where you left off.", line)
	require.True(t, u.Character.IsShelved(42))

	ok, line = AbandonQuest(u, 42)
	require.False(t, ok)
	require.Equal(t, "You have no quest called that.", line)
}

// The panel is re-sent by the gmcp events.Quest listener, which reads only
// the UserId; a MarkerOnly event is what the hook also turns into a marker
// refresh. A refusal queues nothing.
func TestAbandonQuest_QueuesPanelRefresh(t *testing.T) {
	u := questlogFixture(t)
	events.DrainQueuedQuestsForTest()

	_, _ = AbandonQuest(u, 28)
	require.Empty(t, events.DrainQueuedQuestsForTest(), "a refusal refreshes nothing")

	_, _ = AbandonQuest(u, 42)
	got := events.DrainQueuedQuestsForTest()
	require.Len(t, got, 1)
	require.Equal(t, u.UserId, got[0].UserId)
	require.True(t, got[0].MarkerOnly)
}

func TestAbandonQuest_EmptyGiverNameFallsBack(t *testing.T) {
	u := questlogFixture(t)
	t.Cleanup(setGiverNameForTest(map[int]string{}))
	_, line := AbandonQuest(u, 42)
	require.Equal(t, "You set The Hunt aside. Talk to the one who gave it to you to take it up again. You will pick up where you left off.", line)
}

func TestFindActiveQuest(t *testing.T) {
	u := questlogFixture(t)
	require.Equal(t, 42, findActiveQuest(u, "hunt"))
	require.Equal(t, 42, findActiveQuest(u, "The Hunt"))
	require.Equal(t, 0, findActiveQuest(u, "secret"), "secret quests are never matched")
	require.Equal(t, 0, findActiveQuest(u, "dragon"))
}

func TestFocusQuest(t *testing.T) {
	u := questlogFixture(t)
	require.True(t, FocusQuest(u, 42))
	require.Equal(t, 42, u.Character.LastQuestId)
	require.False(t, FocusQuest(u, 85))
	require.False(t, FocusQuest(u, 7))
}

func TestQuestsCommand_AbandonAndFocusWords(t *testing.T) {
	require.Equal(t, [2]string{"abandon", "the hunt"}, questsSubcommand("abandon the hunt"))
	require.Equal(t, [2]string{"focus", "hunt"}, questsSubcommand("focus hunt"))
	require.Equal(t, [2]string{"", "all"}, questsSubcommand("all"))
}

func TestShelvedRows(t *testing.T) {
	u := questlogFixture(t)
	_, _ = AbandonQuest(u, 42)
	rows := shelvedRows(u.Character)
	require.Len(t, rows, 1)
	require.Equal(t, "The Hunt", rows[0].Name)
	require.Equal(t, "Set aside. Talk to Hunter Delk to take it up again.", rows[0].Description)
}

// A set-aside row has no bar and no percentage; the template must not print a
// stray percent sign for it.
func TestShelvedRowRendersWithoutBarOrPercent(t *testing.T) {
	useDogmudTemplates(t)
	u := questlogFixture(t)
	_, _ = AbandonQuest(u, 42)
	type info struct {
		QuestsTotal int
		QuestsFound int
		Records     []questRecord
	}
	txt, err := templates.Process("character/quests", info{QuestsTotal: 1, QuestsFound: 1, Records: shelvedRows(u.Character)}, u.UserId)
	require.NoError(t, err)
	require.Contains(t, txt, "The Hunt")
	require.Contains(t, txt, "Set aside. Talk to Hunter Delk to take it up again.")
	require.NotContains(t, txt, "%")
	t.Log("\n" + txt)
}

func TestQuestsCommand_WiresAbandonAndFocus(t *testing.T) {
	u := questlogFixture(t)

	ok, err := Quests("focus hunt", u, nil, 0)
	require.True(t, ok)
	require.NoError(t, err)
	require.Equal(t, 42, u.Character.LastQuestId)

	_, _ = Quests("abandon waking", u, nil, 0)
	require.False(t, u.Character.IsShelved(28), "quest 28 has no offer")

	_, _ = Quests("abandon hunt", u, nil, 0)
	require.True(t, u.Character.IsShelved(42))
	require.Equal(t, 0, u.Character.LastQuestId, "abandoning the focused quest clears focus")
}

func TestHelpQuestsNamesFocusAndAbandon(t *testing.T) {
	useDogmudTemplates(t)
	txt, err := templates.Process("help/quests", nil, 0)
	require.NoError(t, err)
	require.Contains(t, txt, "quests focus <name>")
	require.Contains(t, txt, "quests abandon <name>")
	for _, ln := range strings.Split(txt, "\n") {
		require.LessOrEqual(t, len([]rune(stripAnsiTags(ln))), 80, ln)
	}
}

func stripAnsiTags(s string) string {
	return regexp.MustCompile(`<[^>]*ansi[^>]*>|</ansi>`).ReplaceAllString(s, ``)
}

// On shipped data, quest 28 (started by walking into a room) has no offer.
func TestShippedOfferIndex_Quest28HasNoOffer(t *testing.T) {
	chdirRepoRootForTest(t)
	files, _ := dialogue.LoadAllFiles()
	t.Cleanup(dialogue.SetOfferIndexForTest(nil))
	dialogue.BuildOfferIndex(files)
	require.Empty(t, dialogue.Offerers(28))
	require.NotEmpty(t, dialogue.Offerers(42))
}
