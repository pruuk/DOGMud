package behaviortree

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// #690: the quest 48 taunt line says Garrow quits the green. His tree's
// yield branch must make that true. A `flee` is a roll that can fail and
// leave a real fight running (the player could not walk south), so the
// branch speaks, emotes and despawns him; the player's aggro is released on
// the next round because the target is gone (hooks.ValidateAggro), and the
// Mustering Green's spawn list brings him back for the next student.
func TestGarrowYieldLeavesTheGreen(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	world := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud")
	treePath := filepath.Join(world, "behaviors", "pothole_coulee", "9158-sullen_garrow.yaml")
	_, err := LoadTreeFromFile(treePath)
	require.NoError(t, err, "Garrow's tree must load")

	var doc struct {
		Tree struct {
			Children []struct {
				Event    string `yaml:"event"`
				Children []struct {
					Do   string `yaml:"do"`
					Cmd  string `yaml:"cmd"`
					Text string `yaml:"text"`
				} `yaml:"children"`
			} `yaml:"children"`
		} `yaml:"tree"`
	}
	data, err := os.ReadFile(treePath)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &doc))
	var steps []string
	found := false
	for _, br := range doc.Tree.Children {
		if br.Event != "mob_combat_round" {
			continue
		}
		found = true
		for _, a := range br.Children {
			step := a.Do
			if a.Cmd != "" {
				step += ":" + a.Cmd
			}
			steps = append(steps, step)
			require.NotContains(t, a.Text, "--", "no spaced dash in Garrow's lines")
		}
	}
	require.True(t, found, "Garrow's tree has a mob_combat_round yield branch")
	require.Equal(t, []string{"say", "emote", "command:despawn"}, steps, "he yields, leaves, and the green respawns him")

	var room struct {
		SpawnInfo []struct {
			MobId       int    `yaml:"mobid"`
			RespawnRate string `yaml:"respawnrate"`
			Message     string `yaml:"message"`
		} `yaml:"spawninfo"`
	}
	data, err = os.ReadFile(filepath.Join(world, "rooms", "pothole_coulee", "5335.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &room))
	seen := false
	for _, s := range room.SpawnInfo {
		if s.MobId == 9158 {
			seen = true
			require.NotEmpty(t, s.RespawnRate, "Garrow comes back for the next student")
			require.NotEmpty(t, s.Message, "his return is narrated")
		}
	}
	require.True(t, seen, "the Mustering Green spawns Garrow")
}
