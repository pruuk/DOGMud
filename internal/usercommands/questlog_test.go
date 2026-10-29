package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// chdirRepoRootForTest points the test at the real dogmud data tree. It does
// not chdir or reload the shipped config: this package's tests rely on the Go
// defaults, and a ReloadConfig leaks into every later test in the binary
// (taunt, shoot). useDogmudTemplates scopes the data path to this test.
func chdirRepoRootForTest(t *testing.T) {
	t.Helper()
	useDogmudTemplates(t)
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
	for name, want := range map[string]int{"hunt": 42, "The Hunt": 42, "secret": 0, "dragon": 0} {
		id, ambiguous := findActiveQuest(u, name)
		require.Equal(t, want, id, name)
		require.False(t, ambiguous, name)
	}
}

// An exact name wins, a partial name that fits one quest wins, and a partial
// name that fits several is refused as ambiguous rather than picking one by
// map order (#375 phase 5 review).
func TestFindActiveQuest_Ambiguous(t *testing.T) {
	u := questlogFixture(t)
	t.Cleanup(quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 43, Name: "The Hunt Beyond", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 44, Name: "The Hunter's Due", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	))
	u.Character.QuestProgress[43] = "start"
	u.Character.QuestProgress[44] = "start"

	for range 20 { // map order varies run to run
		id, ambiguous := findActiveQuest(u, "the hunt")
		require.Equal(t, 42, id, "an exact match wins")
		require.False(t, ambiguous)

		id, ambiguous = findActiveQuest(u, "beyond")
		require.Equal(t, 43, id, "a unique partial match wins")
		require.False(t, ambiguous)

		id, ambiguous = findActiveQuest(u, "the hun")
		require.Equal(t, 0, id, "several partial matches pick none")
		require.True(t, ambiguous)
	}

	id, ambiguous := findActiveQuest(u, "dragon")
	require.Equal(t, 0, id)
	require.False(t, ambiguous, "no match is not ambiguous")

	events.DrainQueuedMessagesForTest(u.UserId)
	_, _ = Quests("abandon the hun", u, nil, 0)
	msgs := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "")
	require.Contains(t, msgs, "More than one quest matches that. Type more of its name.")
	require.False(t, u.Character.IsShelved(42) || u.Character.IsShelved(43) || u.Character.IsShelved(44))

	_, _ = Quests("focus the hun", u, nil, 0)
	msgs = strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "")
	require.Contains(t, msgs, "More than one quest matches that. Type more of its name.")

	_, _ = Quests("focus dragon", u, nil, 0)
	msgs = strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "")
	require.Contains(t, msgs, "You have no quest called that.")
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

// The quest log marks the quest the map and hint follow, so `quests focus`
// shows its effect in text too (the web panel marks it with its dot; the
// phase 5 playtest found the text log showed nothing).
func TestQuestsLog_MarksTheFollowedQuest(t *testing.T) {
	useDogmudTemplates(t)
	u := questlogFixture(t)
	require.True(t, FocusQuest(u, 42))
	type info struct {
		QuestsTotal int
		QuestsFound int
		Records     []questRecord
	}
	hunt, _ := activeQuestRecord(u, quests.GetQuestById(42), "track")
	waking, _ := activeQuestRecord(u, quests.GetQuestById(28), "start")
	require.True(t, hunt.Following)
	require.False(t, waking.Following)
	txt, err := templates.Process("character/quests", info{QuestsTotal: 2, QuestsFound: 2, Records: []questRecord{hunt, waking}}, u.UserId)
	require.NoError(t, err)
	require.Contains(t, stripAnsiTags(txt), "The Hunt (following)")
	require.NotContains(t, stripAnsiTags(txt), "Waking to Gaius (following)")
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

// The abandon line runs past 80 columns and CategorySystem is not wrapped by
// the pipeline, so the sender wraps it (seen breaking mid-word in the web
// client during the phase 5 screenshots).
func TestSendQuestLogLine_WrapsToTheReader(t *testing.T) {
	u := questlogFixture(t)
	events.DrainQueuedMessagesForTest(u.UserId)
	_, line := AbandonQuest(u, 42)
	events.DrainQueuedMessagesForTest(u.UserId)
	require.Greater(t, len(line), 80)
	SendQuestLogLine(u, true, line)
	msgs := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, msgs, 1)
	for _, ln := range strings.Split(strings.TrimRight(msgs[0], "\r\n"), "\n") {
		require.LessOrEqual(t, len([]rune(stripAnsiTags(strings.TrimRight(ln, "\r")))), 80, ln)
	}
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

// An offer that hides on another quest's token cannot take its quest back
// off the shelf, so that quest is not abandonable (#375 phase 5).
func TestAbandonable_NeedsAResumingOffer(t *testing.T) {
	u := questlogFixture(t)
	dialogue.BuildOfferIndex([]*dialogue.DialogueFile{{MobId: 9137, Topics: []dialogue.Topic{
		{Key: "hunt", Kind: dialogue.KindOffer, Gates: dialogue.Gates{QuestExcluded: []string{"42-start", "31-start"}},
			Effects: dialogue.Effects{GrantsQuest: "42-start"}},
	}}})
	require.False(t, Abandonable(u.Character, 42))
	ok, line := AbandonQuest(u, 42)
	require.False(t, ok)
	require.Equal(t, questNoWayBackLine, line)
	require.False(t, u.Character.IsShelved(42))
}

// On shipped data, quest 88's only offer (road_toke) excludes 31-start and
// 29-end, so 88 is not abandonable; quest 42's offer excludes only its own
// tokens.
func TestShippedOfferIndex_Quest88NotAbandonable(t *testing.T) {
	chdirRepoRootForTest(t)
	files, _ := dialogue.LoadAllFiles()
	t.Cleanup(dialogue.SetOfferIndexForTest(nil))
	dialogue.BuildOfferIndex(files)
	u := users.NewTestUser(97102, "ql2", "Questlog", 97102)
	u.Character.QuestProgress = map[int]string{42: "track", 88: "start"}
	require.NotEmpty(t, dialogue.Offerers(88))
	require.False(t, Abandonable(u.Character, 88))
	require.True(t, Abandonable(u.Character, 42))
	t.Log("offered but not abandonable:", dialogue.Unresumable())
}
