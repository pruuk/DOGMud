package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func tier5Search(room int) EventDetails { return EventDetails{Command: "search", RoomId: room} }

// Rule 14: a step completes on something the player does, never a typed noun.
func TestTier5QuestsHaveNoTypedNouns(t *testing.T) {
	for file, id := range map[string]int{
		"63-dock_rat.yaml": 63,
	} {
		for _, tr := range loadShippedQuest(t, file, id).Triggers {
			require.NotEqual(t, "room_interact", tr.Event, "quest %d", id)
		}
	}
}

func TestQuest63TheBookIsFoundBySearch(t *testing.T) {
	const office, book = 5526, 40100
	q := loadShippedQuest(t, "63-dock_rat.yaml", 63)
	require.Equal(t, []string{"start", "evidence", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())

	w := newQuestWalker(q).at("start")
	w.notify("command", tier5Search(office+1))
	require.Equal(t, "start", w.p.current)
	w.notify("command", tier5Search(office))
	require.Equal(t, "evidence", w.p.current)
	require.Equal(t, []int{book}, w.ctx.givenItems, "the search gives one book")

	lost := newQuestWalker(q).at("evidence")
	lost.notify("command", tier5Search(office))
	require.Equal(t, []int{book}, lost.ctx.givenItems, "a lost book has a spare")

	held := newQuestWalker(q).at("evidence")
	held.p.items[book] = true
	held.notify("command", tier5Search(office))
	require.Empty(t, held.ctx.givenItems, "no second book while one is held")
}

// C3: a save on a removed step must keep going, so each leaves by rename.
func TestTier5RemovedStepsAreRenamed(t *testing.T) {
	m, err := quests.ParseStepManifest(repoFile(t, "_datafiles", "world", "dogmud", "quests", "step-ids.txt"))
	require.NoError(t, err)
	require.Equal(t, "63-evidence", m.Renamed["63-report"])
}
