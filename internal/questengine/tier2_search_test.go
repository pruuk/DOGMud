package questengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func loadTier2Quest(t *testing.T, id string) *quests.Quest {
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
// Tier 2 ruling: each quest 20 site is found with a real search in that room.
func TestQuest20SitesAreFoundBySearch(t *testing.T) {
	q := loadTier2Quest(t, "20")
	sites := map[int]string{
		4138: "20-workshop_carving",
		4122: "20-temple_pillars",
		4124: "20-garden_marker",
		4144: "20-chapel_altar",
		4145: "20-wardstone_altar",
		4130: "20-bone_shoals_cache",
		4140: "20-kingfisher_found",
	}
	found := map[int]bool{}
	for _, tr := range q.Triggers {
		require.NotEqual(t, "room_interact", tr.Event, "rule 14: no typed noun")
		if tr.Event != "command" || tr.Command != "search" {
			continue
		}
		grant := ""
		for _, a := range tr.Actions {
			if a.Grant != "" {
				grant = a.Grant
			}
		}
		if grant == "" {
			continue // the lost kingfisher's replacement grants nothing
		}
		require.Equal(t, sites[tr.Room], grant, "room %d", tr.Room)
		found[tr.Room] = true
	}
	require.Len(t, found, len(sites))
}

// A lost kingfisher is replaced by searching at the grave again.
func TestQuest20LostKingfisherComesBackBySearch(t *testing.T) {
	q := loadTier2Quest(t, "20")
	for _, tr := range q.Triggers {
		if tr.Event == "command" && tr.Room == 4140 && tr.Conditions.MissingItem == 40060 {
			require.Contains(t, tr.Conditions.Has, "20-kingfisher_found")
			return
		}
	}
	t.Fatal("no replacement trigger for a lost kingfisher")
}

func TestQuest18BedrollIsFoundBySearch(t *testing.T) {
	q := loadTier2Quest(t, "18")
	found := false
	for _, tr := range q.Triggers {
		require.NotEqual(t, "room_interact", tr.Event, "rule 14: no typed noun")
		if tr.Event == "command" && tr.Command == "search" && tr.Room == 4050 {
			found = true
			require.Contains(t, tr.Conditions.Missing, "18-start")
		}
	}
	require.True(t, found)
}
