package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

func repeatQuest(limit int) *quests.Quest {
	return &quests.Quest{QuestId: 999053, Name: "Cull Test", Repeatable: true,
		CooldownRounds: 10, RepeatLimit: limit,
		Steps: []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "end"}}}
}

func runOnce(t *testing.T, userId int) {
	t.Helper()
	HandleQuestUpdate(events.Quest{UserId: userId, QuestToken: "999053-start"})
	HandleQuestUpdate(events.Quest{UserId: userId, QuestToken: "999053-end"})
}

func TestRepeatLimit_CountsEachCompletion(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer quests.SeedQuestsForTest(repeatQuest(2))()
	u := users.NewTestUser(531, "repeatuser", "RepeatUser", 5031)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{531: u})()
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(1000)

	runOnce(t, 531)
	require.Equal(t, 1, u.Character.QuestCompletions[999053])
	require.False(t, u.Character.HasQuest("999053-start"), "progress cleared for the repeat")
	require.True(t, u.Character.QuestCooldownActive(999053))
}

func TestRepeatLimit_RefusesStartAtCap(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer quests.SeedQuestsForTest(repeatQuest(2))()
	u := users.NewTestUser(532, "capuser", "CapUser", 5032)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{532: u})()
	u.Character.QuestCompletions = map[int]int{999053: 2}
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(5000)
	events.DrainQueuedMessagesForTest(532)

	HandleQuestUpdate(events.Quest{UserId: 532, QuestToken: "999053-start"})
	require.False(t, u.Character.HasQuest("999053-start"))
	lines := strings.Join(events.DrainQueuedMessagesForTest(532), "\n")
	require.Contains(t, lines, "You have done all the work there is here.")
}

// The repeat-limit tests set the round count below util.RoundCountMinimum;
// each must put it back for the tests that follow.
func TestRepeatLimit_TestsRestoreTheRoundCount(t *testing.T) {
	for name, fn := range map[string]func(*testing.T){
		"CountsEachCompletion":  TestRepeatLimit_CountsEachCompletion,
		"RefusesStartAtCap":     TestRepeatLimit_RefusesStartAtCap,
		"NoLimitKeepsRepeating": TestRepeatLimit_NoLimitKeepsRepeating,
	} {
		util.ResetRoundCountForTest()
		t.Run(name, fn)
		require.Equal(t, uint64(util.RoundCountMinimum), util.GetRoundCount(), "%s left the round count changed", name)
	}
}

func TestRepeatLimit_NoLimitKeepsRepeating(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer quests.SeedQuestsForTest(repeatQuest(0))()
	u := users.NewTestUser(533, "againuser", "AgainUser", 5033)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{533: u})()
	t.Cleanup(util.ResetRoundCountForTest)
	for i, round := range []uint64{1000, 1100, 1200} {
		util.SetRoundCountForTest(round)
		runOnce(t, 533)
		require.Equal(t, i+1, u.Character.QuestCompletions[999053])
	}
}
