package questengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestUndertowHasARealStrongbox(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/quests/14-the_undertow.yaml")
	require.NoError(t, err)
	var q quests.Quest
	require.NoError(t, yaml.Unmarshal(data, &q))
	steps := []string{}
	for _, s := range q.Steps {
		steps = append(steps, s.Id)
	}
	require.Equal(t, []string{"start", "explore", "confront", "loot", "unlock", "opened", "evidence", "end"}, steps)
	for _, tr := range q.Triggers {
		require.NotEqual(t, "room_interact", tr.Event, "rule 14: no typed noun")
	}
	data, err = os.ReadFile("../../_datafiles/world/dogmud/rooms/thornwall_city/498.yaml")
	require.NoError(t, err)
	room := strings.ReplaceAll(string(data), "\r\n", "\n") // a Windows checkout may hold CRLF
	require.Contains(t, room, "strongbox:\n    lock:")
	require.Contains(t, room, "container: strongbox")
	key, err := filepath.Glob("../../_datafiles/world/dogmud/items/*/40034-*.yaml")
	require.NoError(t, err)
	require.Len(t, key, 1)
	data, err = os.ReadFile(key[0])
	require.NoError(t, err)
	require.Contains(t, string(data), "keylockid: 498-strongbox")
}
