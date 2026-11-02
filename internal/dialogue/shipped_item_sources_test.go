package dialogue

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// Spec ruling 5: a quest item with a keeper beside it comes from the keeper.
func roomSpawns(t *testing.T, path string) []struct {
	ItemId    int    `yaml:"itemid"`
	Container string `yaml:"container"`
} {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/rooms/" + path)
	require.NoError(t, err)
	var room struct {
		SpawnInfo []struct {
			ItemId    int    `yaml:"itemid"`
			Container string `yaml:"container"`
		} `yaml:"spawninfo"`
	}
	require.NoError(t, yaml.Unmarshal(data, &room))
	return room.SpawnInfo
}

func TestTheLedgersComeFromTheirKeepers(t *testing.T) {
	for _, s := range roomSpawns(t, "watchers_crossing/421.yaml") {
		require.NotEqual(t, 21, s.ItemId, "the crossing ledger no longer lies in the Tollhouse")
	}
	for _, s := range roomSpawns(t, "thornwall_city/476.yaml") {
		require.NotEqual(t, 29, s.ItemId, "the tithe ledger no longer lies in the Records Office")
	}

	harn := loadShippedDialogue(t, "watchers_crossing/86.yaml")
	lend := harn.FindTopic("lend_ledger")
	require.NotNil(t, lend)
	require.Equal(t, 21, lend.GivesItem)
	require.Contains(t, menuKeys(harn, heldState("5-start")), "lend_ledger")
	require.NotContains(t, menuKeys(harn, heldState()), "lend_ledger", "only during quest 5")
	require.NotContains(t, menuKeys(harn, heldState("5-start", "5-end")), "lend_ledger")

	pell := loadShippedDialogue(t, "thornwall_city/99.yaml")
	copyTopic := pell.FindTopic("tithe_ledger")
	require.NotNil(t, copyTopic)
	require.Equal(t, 29, copyTopic.GivesItem)
	require.False(t, strings.Contains(copyTopic.Hints, "get ledger"))
}

func TestThePurseLiesInAContainer(t *testing.T) {
	found := false
	for _, s := range roomSpawns(t, "dustwalk_road/408.yaml") {
		if s.ItemId == 16 {
			found = true
			require.Equal(t, "goods", s.Container)
		}
	}
	require.True(t, found, "the Bandit Hollow still holds the purse")
}
