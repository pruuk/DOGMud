package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Maren's letter (item 40041) hides behind the loose stone by the hearth in
// Maren's Cottage (room 4023). It used to come from a room behaviour tree
// that answered push, move, pull, shift and press; since #375 phase 3a (owner
// ruling 5: look and search are the only interaction verbs) quest 76 carries
// a `verb: search` room_interact on the stone, for any player, on or off the
// quest. `search stone` runs the search and its cooldown first; a plain
// search can find it at the room difficulty.
func TestMarenLetter_SearchTheStone(t *testing.T) {
	const cottage, letter = 4023, 40041
	q := loadShippedQuest(t, "76-the_disc.yaml", 76)
	searchStone := EventDetails{UserId: 1, RoomId: cottage, Verb: "search", Noun: "stone", RoomNoun: "stone"}

	run := func(p *fullMockPlayer, d EventDetails) (bool, *mockActionContext) {
		e := NewEngine()
		e.RegisterQuest(q)
		ctx := newMockActionContext(1)
		return e.Notify("room_interact", d, p, ctx).Handled, ctx
	}

	t.Run("a player without the letter finds it, quest or not", func(t *testing.T) {
		handled, ctx := run(newFullMockPlayer(cottage), searchStone)
		require.True(t, handled)
		require.Equal(t, []int{letter}, ctx.givenItems)
		require.Len(t, ctx.sentTexts, 1)
		require.Contains(t, ctx.sentTexts[0], "letter")
		require.Empty(t, ctx.granted, "the letter is no quest step")
	})

	t.Run("a player who carries it finds nothing more", func(t *testing.T) {
		p := newFullMockPlayer(cottage)
		p.items[letter] = true
		handled, ctx := run(p, searchStone)
		require.False(t, handled, "the search's own reply stands")
		require.Empty(t, ctx.givenItems)
	})

	t.Run("looking at the stone does not move it", func(t *testing.T) {
		look := searchStone
		look.Verb = "look"
		handled, ctx := run(newFullMockPlayer(cottage), look)
		require.False(t, handled)
		require.Empty(t, ctx.givenItems)
	})

	t.Run("the room tree that answered push is gone", func(t *testing.T) {
		_, here, _, ok := runtime.Caller(0)
		require.True(t, ok)
		_, err := os.Stat(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "behaviors", "rooms", "ashwick", "4023.yaml"))
		require.True(t, os.IsNotExist(err), "push, move, pull, shift and press must not reach the stone any more (stat: %v)", err)
	})

	t.Run("the stone hints and never instructs", func(t *testing.T) {
		data := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "rooms", "ashwick", "4023.yaml")))
		require.NotContains(t, data, "pushed aside")
		require.Contains(t, data, "darker than the wall should be")
	})
}
