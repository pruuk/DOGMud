package dialogue

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// TestHadwenMidRite walks Cleric Hadwen (9100) through quest 30. The
// playtest (seat D) talked to him during the rite and got the pre-rite
// opener ("The pool is ready when you are") and the "I'm ready" line again.
// Topics cannot read the tree's per-player rite_running state, so the rite
// now grants the quest step 30-rite as it starts, and his dialogue gates on
// that: a mid-rite opener, no begin line, and one mid-rite topic that asks
// whether the rite is still going (the tree answers it, and restarts a rite
// that a logout or a reboot cut short).
func TestHadwenMidRite(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml")
	require.NoError(t, err)
	var df DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))

	menuIds := func(ps *PlayerState) string {
		var ids []string
		for _, e := range df.Menu("Cleric Hadwen", ps).Entries {
			ids = append(ids, e.Id)
		}
		return strings.Join(ids, " ")
	}

	t.Run("before the rite", func(t *testing.T) {
		ps := velkState(map[string]string{"30": "start"})
		text, _ := df.OpenerFor(ps)
		require.Contains(t, text, "The pool is ready when you are.")
		ids := strings.Fields(menuIds(ps))
		require.Contains(t, ids, "begin")
		require.Contains(t, ids, "rite_waiting")
		require.NotContains(t, ids, "rite_wait")
	})
	t.Run("during the rite", func(t *testing.T) {
		ps := velkState(map[string]string{"30": "rite"})
		text, _ := df.OpenerFor(ps)
		require.NotContains(t, text, "The pool is ready when you are.")
		require.Contains(t, text, "The rite is under way")
		ids := strings.Fields(menuIds(ps))
		require.NotContains(t, ids, "begin", "the I'm ready line is offered mid-rite")
		require.NotContains(t, ids, "rite_waiting", "the pre-rite explanation is offered mid-rite")
		require.Contains(t, ids, "rite_wait")
		require.True(t, df.FindTopic("rite_wait").Behavior, "the tree answers the mid-rite topic")
		require.NotContains(t, ids, "rite_report", "the hand-in waits for the rite's close")
	})
	t.Run("rite done, not yet told", func(t *testing.T) {
		ps := velkState(map[string]string{"30": "woken"})
		text, _ := df.OpenerFor(ps)
		require.Contains(t, text, "Come, tell me how it went")
		ids := strings.Fields(menuIds(ps))
		require.Contains(t, ids, "rite_report", "the hand-in is open once the rite is over (#375 phase 7)")
		require.NotContains(t, ids, "rite_wait", "the rite is over")
		require.NotContains(t, ids, "begin")
		require.NotContains(t, ids, "road_toke", "the roads wait for the hand-in")
		require.NotContains(t, ids, "another_way")
	})
	t.Run("after the hand-in", func(t *testing.T) {
		ps := velkState(map[string]string{"30": "end"})
		text, _ := df.OpenerFor(ps)
		require.Contains(t, text, "How does it sit")
		ids := strings.Fields(menuIds(ps))
		require.NotContains(t, ids, "begin")
		require.NotContains(t, ids, "rite_wait")
		require.NotContains(t, ids, "rite_report")
		require.Contains(t, ids, "road_toke")
		require.Contains(t, ids, "road_esk")
		require.Contains(t, ids, "another_way", "the skip path is an info topic")
	})
}
