package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// TestQuest28_RefusableStepsNeedTheCommandToSucceed locks the 2026-10-17
// pre-deploy playtest fix. Tutorial steps of Waking to Gaius listened on
// command_issued, which fires whenever a command is dispatched, so a refused
// bare `say` ("Say what?") and a refused `trip` ("already on the ground")
// both finished their step. Each step whose command can be refused now
// listens on the success notification instead. For every such step this
// asserts that the bare command_issued no longer grants the step and that
// the success event does.
func TestQuest28_RefusableStepsNeedTheCommandToSucceed(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "quests", "28-waking_to_gaius.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "must be able to read the real quest 28 YAML")
	var q QuestDef
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.Equal(t, 28, q.QuestId, "loaded the wrong quest file")

	cases := []struct {
		name     string
		has      string // the token that opens the step
		grants   string // the step's token
		issued   EventDetails
		success  string
		succeeds EventDetails
	}{
		{"say", "28-speaks_prompt", "28-said",
			EventDetails{Command: "say"},
			"command", EventDetails{Command: "say"}},
		// The ask step (28-asked) is Dewey's talk topic asked_28 since #375
		// Task 19; no quest trigger grants it (TestQuest28_AskReplyComesBeforeTheNextPrompt).
		{"warcry", "28-proving_prompt", "28-warcried",
			EventDetails{Command: "warcry"},
			"command", EventDetails{Command: "warcry"}},
		{"cast spike", "28-attacked", "28-cast_spike",
			EventDetails{Command: "cast", Noun: "spike"},
			"command", EventDetails{Command: "cast", Noun: "conviction-spike"}},
		{"trip", "28-cast_spike", "28-tripped",
			EventDetails{Command: "trip"},
			"command", EventDetails{Command: "trip"}},
		{"flee", "28-tripped", "28-fled",
			EventDetails{Command: "flee"},
			"command", EventDetails{Command: "flee"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEngine()
			qq := q
			e.RegisterQuest(&qq)

			player := newFullMockPlayer(6262)
			player.quests["28-start"] = true
			player.quests[tc.has] = true
			ctx := newFullMockActionContext(1, player)

			tc.issued.UserId = 1
			e.Notify("command_issued", tc.issued, player, ctx)
			require.False(t, player.HasQuest(tc.grants),
				"typing %q alone (which a refusal also does) must not grant %s", tc.name, tc.grants)

			tc.succeeds.UserId = 1
			e.Notify(tc.success, tc.succeeds, player, ctx)
			require.True(t, player.HasQuest(tc.grants),
				"a successful %q must grant %s", tc.name, tc.grants)
		})
	}

	// The cast step wants the spike, not just any spell.
	t.Run("cast of another spell", func(t *testing.T) {
		e := NewEngine()
		qq := q
		e.RegisterQuest(&qq)
		player := newFullMockPlayer(6262)
		player.quests["28-start"] = true
		player.quests["28-attacked"] = true
		ctx := newFullMockActionContext(1, player)
		e.Notify("command", EventDetails{UserId: 1, Command: "cast", Noun: "chrysalis-glow"}, player, ctx)
		require.False(t, player.HasQuest("28-cast_spike"))
	})
}
