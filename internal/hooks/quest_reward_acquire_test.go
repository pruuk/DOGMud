package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 3a: quest reward items (rewards.itemid and rewards.item_info)
// acquire through the seam, so each lands with exactly one
// ItemOwnership{Gained, Reason: reward}. A reward item that carries a
// QuestToken grants that token exactly once, through CheckItemQuests.

const (
	rewardAcqItem      = 975201 // rewards.itemid, carries a QuestToken
	rewardAcqGrantItem = 975202 // rewards.item_info, two of them
	rewardAcqToken     = "999005-start"
)

const rewardAcqQuestYAML = `
questid: 999004
name: Test Reward Acquire
description: Fixture proving reward items acquire through the seam.
steps:
  - id: start
    description: "Started."
  - id: end
    description: "Done."
rewards:
  actor: "Done."
  itemid: 975201
  item_info: "975202:2"
`

func seedRewardAcquireQuest(t *testing.T) {
	t.Helper()
	drainEventQueueOnCleanup(t)
	events.DrainAllQueuedEventsForTest()
	tmp := t.TempDir()
	questsDir := filepath.Join(tmp, "quests")
	require.NoError(t, os.MkdirAll(questsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(questsDir, "999004-test_reward_acquire.yaml"), []byte(rewardAcqQuestYAML), 0644))
	origDataFiles := configs.GetFilePathsConfig().DataFiles.String()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tmp}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": origDataFiles})
	})
	quests.LoadDataFiles()
	questengine.LoadDataFiles()

	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		rewardAcqItem:      {ItemId: rewardAcqItem, Name: "sealed letter", Type: items.Object, Weight: 0.1, QuestToken: rewardAcqToken},
		rewardAcqGrantItem: {ItemId: rewardAcqGrantItem, Name: "ration", Type: items.Object, Weight: 0.1},
	}))
}

func TestHandleQuestUpdate_RewardItemsAcquireWithRewardReason(t *testing.T) {
	seedRewardAcquireQuest(t)
	u := users.NewTestUser(508, "rewardacq", "RewardAcq", 5008)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{508: u}))
	require.True(t, u.Character.GiveQuestToken("999004-start"))
	events.DrainAllQueuedEventsForTest()

	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 508, QuestToken: "999004-end"}))

	got := events.DrainQueuedItemOwnershipForTest(508)
	require.Len(t, got, 3, "one gain per reward item: the itemid reward and two item_info grants")
	byItem := map[int]int{}
	for _, e := range got {
		assert.True(t, e.Gained)
		assert.Equal(t, "reward", e.Reason)
		byItem[e.Item.ItemId]++
	}
	assert.Equal(t, 1, byItem[rewardAcqItem])
	assert.Equal(t, 2, byItem[rewardAcqGrantItem])
}

// The reward item's QuestToken used to be hand-fired at the reward site,
// because no ItemOwnership was raised there. Now the seam raises one and
// CheckItemQuests grants the token, so the token is queued exactly once.
func TestHandleQuestUpdate_RewardItemQuestTokenGrantedOnce(t *testing.T) {
	seedRewardAcquireQuest(t)
	u := users.NewTestUser(509, "rewardtok", "RewardTok", 5009)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{509: u}))
	require.True(t, u.Character.GiveQuestToken("999004-start"))
	events.DrainAllQueuedEventsForTest()

	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 509, QuestToken: "999004-end"}))

	// Run the ItemOwnership listener over what the reward raised, as the
	// event loop would, then count every queued grant of the token.
	for _, e := range events.DrainQueuedItemOwnershipForTest(509) {
		CheckItemQuests(e)
	}
	n := 0
	for _, q := range events.DrainQueuedQuestsForTest() {
		if q.UserId == 509 && q.QuestToken == rewardAcqToken {
			n++
		}
	}
	assert.Equal(t, 1, n, "the reward item's QuestToken is granted exactly once")
}
