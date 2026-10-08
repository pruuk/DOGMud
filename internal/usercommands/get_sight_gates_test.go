package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The player's get keeps today's lines through the shared gates (slice 5a).

func TestGet_ExplodingItemKeepsItsLine(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 90)
	bomb := items.Item{ItemId: 96201, Spec: &items.ItemSpec{ItemId: 96201, Name: "bomb"}, Adjectives: []string{`exploding`}}
	room.Items = append(room.Items, bomb)
	out := runGate(t, user, func() (bool, error) { return Get("bomb", user, room, 0) })
	assert.Contains(t, out, "You can't pick that up, it's about to explode!")
	_, onFloor := room.FindOnFloor("bomb", false)
	assert.True(t, onFloor)
}

func TestGet_SweepStopsSilentlyOnAnExplodingItem(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 90)
	room.Items = append(room.Items,
		items.Item{ItemId: 96202, Spec: &items.ItemSpec{ItemId: 96202, Name: "stone"}},
		items.Item{ItemId: 96203, Spec: &items.ItemSpec{ItemId: 96203, Name: "stone"}, Adjectives: []string{`exploding`}},
	)
	out := runGate(t, user, func() (bool, error) { return Get("all stone", user, room, 0) })
	assert.Contains(t, out, "You pick up 1 item.")
	_, carried := user.Character.FindInBackpack("stone")
	require.True(t, carried)
}

func TestGet_DarkRefusalUnchanged(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 0)
	out := runGate(t, user, func() (bool, error) { return Get("all", user, room, 0) })
	assert.Contains(t, out, "You can't see anything to pick up!")
}

// A plain `get X` that auto-detects X in the player's own stash (no `from
// stash` on the command line) now refuses an exploding item with the same
// line an explicit `get X from stash` already used. Before the shared
// pickup, this auto-detect path had no exploding check and took the item.
func TestGet_AutoDetectedStashExplodingItemKeepsItsLine(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 90)
	bomb := items.Item{ItemId: 96204, Spec: &items.ItemSpec{ItemId: 96204, Name: "bomb"}, Adjectives: []string{`exploding`}}
	bomb.StashedBy = user.UserId
	room.Stash = append(room.Stash, bomb)
	out := runGate(t, user, func() (bool, error) { return Get("bomb", user, room, 0) })
	assert.Contains(t, out, "You can't pick that up, it's about to explode!")
	_, stillStashed := room.FindOnFloor("bomb", true)
	assert.True(t, stillStashed)
}
