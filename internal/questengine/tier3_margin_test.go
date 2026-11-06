package questengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func loadTier3Quest(t *testing.T, id string) *quests.Quest {
	t.Helper()
	paths, err := filepath.Glob("../../_datafiles/world/dogmud/quests/" + id + "-*.yaml")
	require.NoError(t, err)
	require.Len(t, paths, 1)
	data, err := os.ReadFile(paths[0])
	require.NoError(t, err)
	var q quests.Quest
	require.NoError(t, yaml.Unmarshal(data, &q))
	return &q
}

// Rule 14: a step completes on something the player does, never a typed noun.
func TestTheMarginsRecordsAreFoundBySearch(t *testing.T) {
	for id, sites := range map[string]map[int]string{
		"73": {6226: "73-chart", 6131: "73-scrivener", 6143: "73-map"},
		"74": {6189: "74-ledger", 6188: "74-record", 6198: "74-survey", 6215: "74-reveal"},
	} {
		q := loadTier3Quest(t, id)
		found := map[int]bool{}
		for _, tr := range q.Triggers {
			require.NotContains(t, []string{"look_at", "search_at"}, tr.Event, "rule 14: no typed noun in quest %s", id)
			if tr.Event != "command" || tr.Command != "search" {
				continue
			}
			for _, a := range tr.Actions {
				if a.Grant != "" {
					require.Equal(t, sites[tr.Room], a.Grant, "quest %s room %d", id, tr.Room)
					found[tr.Room] = true
				}
			}
		}
		require.Len(t, found, len(sites), "quest %s", id)
	}
}

func TestTheOldSkyGivesTheRubbing(t *testing.T) {
	q := loadTier3Quest(t, "74")
	for _, tr := range q.Triggers {
		if tr.Event == "command" && tr.Room == 6215 {
			for _, a := range tr.Actions {
				if a.GiveItem == 40142 {
					return
				}
			}
		}
	}
	t.Fatal("the search at 6215 gives no rubbing")
}
