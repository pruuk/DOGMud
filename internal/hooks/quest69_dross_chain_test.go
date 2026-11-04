package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const shippedWorld = "../../_datafiles/world/dogmud"

// The whole of Dross's hand-in, on the shipped dialogue, quest and items: `give`
// plays give_40116, the topic takes the reading and queues 69-end, and the hook
// bridges that token to the quest_granted trigger that takes the rubbing. Each
// piece has its own test; only this one crosses the joins.
func TestQuest69GiveReachesTheRubbingsConsume(t *testing.T) {
	const (
		rubbing = 40115
		reading = 40116
	)
	cleanup := seedAllRegistries()
	defer cleanup()
	drainEventQueueOnCleanup(t)

	// The real quest, loaded into both systems as the boot does.
	src, err := os.ReadFile(filepath.Join(shippedWorld, "quests", "69-the_gallery_cipher.yaml"))
	require.NoError(t, err)
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "quests"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "quests", "69-the_gallery_cipher.yaml"), src, 0644))
	origDataFiles := configs.GetFilePathsConfig().DataFiles.String()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tmp}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": origDataFiles})
	})
	quests.LoadDataFiles()
	questengine.LoadDataFiles()

	// The real items.
	specs := map[int]*items.ItemSpec{}
	for id, file := range map[int]string{
		rubbing: "40115-cipher_rubbing.yaml",
		reading: "40116-lyshas_annotated_reading.yaml",
	} {
		data, err := os.ReadFile(filepath.Join(shippedWorld, "items", "materials-40000", file))
		require.NoError(t, err)
		var spec items.ItemSpec
		require.NoError(t, yaml.Unmarshal(data, &spec))
		specs[id] = &spec
	}
	t.Cleanup(items.SeedItemsForTest(specs))

	// The real dialogue, on the fixture's mob.
	data, err := os.ReadFile(filepath.Join(shippedWorld, "dialogue", "new_plymouth_temple", "9360.yaml"))
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(data, &df))
	t.Cleanup(dialogue.InstallForTest(&df))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.MobId = mobs.MobId(df.MobId)
	mob.Zone = df.Zone

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	room.Lamp = rooms.LampPtr(90)
	u.Character.QuestProgress = map[int]string{69: "reading"}
	require.True(t, u.Character.StoreItem(items.New(rubbing)))
	require.True(t, u.Character.StoreItem(items.New(reading)))
	events.DrainQueuedQuestsForTest()

	count := func(id int) int {
		n := 0
		for _, itm := range u.Character.GetAllBackpackItems() {
			if itm.ItemId == id {
				n++
			}
		}
		return n
	}

	_, err = usercommands.Give("reading skeleton", u, room, 0)
	require.NoError(t, err)
	require.Equal(t, 0, count(reading), "the topic takes the reading")
	require.Equal(t, 1, count(rubbing), "the topic takes one item; the hook takes the other")

	granted := events.DrainQueuedQuestsForTest()
	require.Len(t, granted, 1)
	require.Equal(t, "69-end", granted[0].QuestToken)

	require.Equal(t, events.Continue, HandleQuestUpdate(granted[0]))
	require.True(t, u.Character.HasQuest("69-end"))
	require.Equal(t, 0, count(rubbing), "the quest_granted trigger takes the rubbing")
}
