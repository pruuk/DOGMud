package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// The 2026-10-17 pre-deploy playtest measured quest reward lines at 143
// (Waking to Gaius) and 131 (The Awakening) characters on one line: the
// completing player's reward line rides CategorySystem, which the pipeline
// never wraps. HandleQuestUpdate now wraps it to the player's width.

const rewardWrapQuestReward = "Dewey's voice stays with you a moment after the grey lets go, " +
	"steady, warm, already a little proud. You are awake now, and the world is real."

const rewardWrapQuestYAML = `
questid: 999002
name: Test Reward Wrap
description: Fixture proving the reward actor line wraps.
steps:
  - id: start
    description: "Started."
  - id: end
    description: "Done."
rewards:
  actor: "` + rewardWrapQuestReward + `"
`

var rewardWrapAnsiTag = regexp.MustCompile(`</?ansi[^>]*>`)

func TestHandleQuestUpdate_RewardLineWrapsToThePlayersWidth(t *testing.T) {
	drainEventQueueOnCleanup(t)
	tmp := t.TempDir()
	questsDir := filepath.Join(tmp, "quests")
	require.NoError(t, os.MkdirAll(questsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(questsDir, "999002-test_reward_wrap.yaml"), []byte(rewardWrapQuestYAML), 0644))
	origDataFiles := configs.GetFilePathsConfig().DataFiles.String()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tmp}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": origDataFiles})
	})
	quests.LoadDataFiles()
	questengine.LoadDataFiles()

	u := users.NewTestUser(504, "wrapuser", "WrapUser", 5004)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{504: u}))
	require.True(t, u.Character.GiveQuestToken("999002-start"))
	events.DrainQueuedMessagesForTest(504)

	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 504, QuestToken: "999002-end"}))

	var rewardLines []string
	for _, msg := range events.DrainQueuedMessagesForTest(504) {
		plain := rewardWrapAnsiTag.ReplaceAllString(msg, "")
		lines := strings.Split(strings.ReplaceAll(plain, "\r\n", "\n"), "\n")
		if !strings.Contains(plain, "Dewey's voice") {
			continue
		}
		for _, line := range lines {
			require.LessOrEqual(t, util.VisibleWidth(line), u.GetLineWidth(),
				"reward line wider than the player's width: %q", line)
			if line = strings.TrimSpace(line); line != "" {
				rewardLines = append(rewardLines, line)
			}
		}
	}
	require.Greater(t, len(rewardLines), 1, "the reward line must reach the player, wrapped")
	require.Equal(t, rewardWrapQuestReward, strings.Join(rewardLines, " "))
}
