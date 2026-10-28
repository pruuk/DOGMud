package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// resumeQuestYAML has quest 50's handout shape (#375 phase 5): a
// quest_granted trigger on the start token, gated only on missing_item,
// gives a valued item (quest 50's pouch of shot 30064, value 40).
const resumeQuestYAML = `
questid: 999050
name: Test First Shot
description: Fixture for resuming a shelved quest through the dialogue grant path.
steps:
  - id: start
    description: "Started."
  - id: shoot
    description: "Shoot."
  - id: end
    description: "Done."
triggers:
  - event: quest_granted
    quest_token: "999050-start"
    conditions:
      missing_item: 999051
    actions:
      - give_item: 999051
`

const resumePouchId = 999051

func setupResumeQuest(t *testing.T) {
	t.Helper()
	drainEventQueueOnCleanup(t)

	tmp := t.TempDir()
	questsDir := filepath.Join(tmp, "quests")
	require.NoError(t, os.MkdirAll(questsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(questsDir, "999050-test_first_shot.yaml"),
		[]byte(resumeQuestYAML), 0644))

	origDataFiles := configs.GetFilePathsConfig().DataFiles.String()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tmp}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": origDataFiles})
	})
	quests.LoadDataFiles()
	questengine.LoadDataFiles()

	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		resumePouchId: {ItemId: resumePouchId, Name: "pouch of shot", Type: items.Object,
			Subtype: items.Mundane, Value: 40},
	}))
}

func resumeUser(t *testing.T, id int) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(id, "resume", "Resume", uint64(id))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{id: u}))
	return u
}

func carries(u *users.UserRecord, itemId int) bool {
	for _, itm := range u.Character.GetAllCarriedItems() {
		if itm.ItemId == itemId {
			return true
		}
	}
	return false
}

func questLogText(u *users.UserRecord) string {
	var out []string
	for e := range u.EventLog.Items {
		if e.Category == `quest` {
			out = append(out, e.What)
		}
	}
	return strings.Join(out, "\n")
}

// A dialogue grant of a shelved quest's start resumes it at the saved step,
// says so once, and its quest_granted chain gives nothing of value (owner
// ruling 2). The same event for a player who never took the quest gives the
// pouch and the new-quest banner.
func TestHandleQuestUpdate_ResumeGivesNothingOfValue(t *testing.T) {
	setupResumeQuest(t)

	u := resumeUser(t, 9505)
	u.Character.QuestProgress = map[int]string{999050: "shoot"}
	require.True(t, u.Character.ShelveQuest(999050))

	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9505, QuestToken: "999050-start"}))
	require.Equal(t, "shoot", u.Character.GetQuestProgress()[999050])
	require.False(t, u.Character.IsShelved(999050), "the shelf is empty")
	require.False(t, carries(u, resumePouchId), "the pouch does not come again")
	require.False(t, u.Character.TakeResumed(999050), "the hook consumed the resume mark")
	log := questLogText(u)
	require.Equal(t, 1, strings.Count(log, "Took up a quest again"))
	require.NotContains(t, log, "Given a new quest")

	v := resumeUser(t, 9506)
	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9506, QuestToken: "999050-start"}))
	require.Equal(t, "start", v.Character.GetQuestProgress()[999050])
	require.True(t, carries(v, resumePouchId), "a first accept gets the pouch")
	log = questLogText(v)
	require.Contains(t, log, "Given a new quest")
	require.NotContains(t, log, "Took up a quest again")
}

// A behaviour tree grants by calling GiveQuestToken and then queueing the
// event: the resume already happened, so the hook only says so.
func TestHandleQuestUpdate_ResumeAfterSynchronousGrant(t *testing.T) {
	setupResumeQuest(t)

	u := resumeUser(t, 9507)
	u.Character.QuestProgress = map[int]string{999050: "shoot"}
	require.True(t, u.Character.ShelveQuest(999050))
	require.True(t, u.Character.GiveQuestToken("999050-start"))

	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9507, QuestToken: "999050-start"}))
	require.Equal(t, "shoot", u.Character.GetQuestProgress()[999050])
	log := questLogText(u)
	require.Contains(t, log, "Took up a quest again")
	require.NotContains(t, log, "Given a new quest")
}

// An engine grant resumes and prints its own banner in GrantQuest; the
// MarkerOnly event it queues reaches the hook and prints nothing more.
func TestHandleQuestUpdate_EngineResumeBannerOnce(t *testing.T) {
	setupResumeQuest(t)

	u := resumeUser(t, 9508)
	u.Character.QuestProgress = map[int]string{999050: "shoot"}
	require.True(t, u.Character.ShelveQuest(999050))

	b := questengine.NewGameBridge(u, u.Character.RoomId)
	b.GrantQuest("999050-start")
	require.True(t, b.Resumed())
	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 9508,
		QuestToken: "999050-start", MarkerOnly: true}))

	log := questLogText(u)
	require.Equal(t, 1, strings.Count(log, "take up"), log)
	require.NotContains(t, log, "given a new quest")
	require.NotContains(t, log, "Given a new quest")
}
