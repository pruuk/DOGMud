package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// A live playtest of From the Dark: the seat attacked in the open, killed the
// brute, and the step did not advance and said nothing. This drives the real
// quest file through the engine. A kill before the ambush says why it did not
// count and moves nothing; the same kill after the ambush is the kill step,
// with no such line.
func TestQuest100_KillBeforeTheAmbushIsExplainedAndMovesNothing(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "quests", "100-from_the_dark.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var q QuestDef
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.Equal(t, 100, q.QuestId)

	const sump = 5257
	t.Cleanup(areas.InstallForTest([]areas.Area{{Key: "coulee_mine", Name: "the old mine", Rooms: []int{sump}}},
		map[int]string{sump: "Pothole Coulee"}))
	const tooSoon = "does not count"

	kill := func(tokens ...string) (*fullMockPlayer, string) {
		e := NewEngine()
		qq := q
		e.RegisterQuest(&qq)
		player := newFullMockPlayer(sump)
		for _, tok := range tokens {
			player.quests[tok] = true
		}
		ctx := newFullMockActionContext(1, player)
		e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9121, RoomId: sump}, player, ctx)
		return player, strings.Join(ctx.sentTexts, "\n")
	}

	t.Run("after the hide, before the strike", func(t *testing.T) {
		player, said := kill("100-start", "100-hidden")
		require.Contains(t, said, tooSoon, "the student must read why the kill did not count")
		require.False(t, player.HasQuest("100-ambush"), "a kill is not an ambush")
		require.False(t, player.HasQuest("100-kill"), "a kill in the open must not finish the quest")
	})

	t.Run("after the strike", func(t *testing.T) {
		player, said := kill("100-start", "100-hidden", "100-ambush")
		require.NotContains(t, said, tooSoon, "a kill after the ambush did count")
		require.True(t, player.HasQuest("100-kill"), "this kill is the step")
	})

	t.Run("before the hide", func(t *testing.T) {
		player, said := kill("100-start")
		require.NotContains(t, said, tooSoon, "the student has not been told to hide yet")
		require.False(t, player.HasQuest("100-kill"))
	})
}
