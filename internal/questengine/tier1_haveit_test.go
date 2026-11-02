package questengine

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// Each giver of an item quest has a have_it topic for a save resting on the
// first step with the item in hand (spec, "Old saves"). The quest must move
// that save on: a topic_heard trigger for the giver's topic, gated on the start
// token, granting the deliver step.
func TestHaveItGrantsTheDeliverStep(t *testing.T) {
	for _, c := range []struct {
		quest             int
		mob               int
		start, deliverTok string
	}{
		{4, 83, "4-start", "4-evidence"},
		{5, 84, "5-start", "5-deliver"},
		{7, 89, "7-start", "7-deliver"},
		{9, 95, "9-start", "9-deliver"},
		{15, 250, "15-start", "15-return"},
	} {
		files, err := filepath.Glob("../../_datafiles/world/dogmud/quests/" + strconv.Itoa(c.quest) + "-*.yaml")
		require.NoError(t, err)
		require.Len(t, files, 1, "quest %d", c.quest)
		data, err := os.ReadFile(files[0])
		require.NoError(t, err)
		var q quests.Quest
		require.NoError(t, yaml.Unmarshal(data, &q))

		found := false
		for _, tr := range q.Triggers {
			if tr.Event != "topic_heard" || tr.Mob != c.mob || tr.Topic != "have_it" {
				continue
			}
			found = true
			require.True(t, slices.Contains(tr.Conditions.Has, c.start), "quest %d has %s", c.quest, c.start)
			require.True(t, slices.Contains(tr.Conditions.Missing, c.deliverTok), "quest %d misses %s", c.quest, c.deliverTok)
			var grants []string
			for _, a := range tr.Actions {
				if a.Grant != "" {
					grants = append(grants, a.Grant)
				}
			}
			require.Equal(t, []string{c.deliverTok}, grants, "quest %d", c.quest)
		}
		require.True(t, found, "quest %d has a topic_heard trigger for mob %d have_it", c.quest, c.mob)
	}
}
