package questengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Find Your Footing (quest 31) under the owner's one-command-per-step ruling
// (2026-10-17): the shop step asked for `list` and then `buy cotton shirt`
// in one hint. It is now two steps, list then buy, each its own trigger,
// with the old step ids kept so a saved character resumes.
func TestQuest31FindYourFooting_OneCommandPerStep(t *testing.T) {
	const (
		hubSquare   = 5203
		marketRow   = 5204
		lantern     = 5205
		provisions  = 5207
		schoolHall  = 5213
		cottonShirt = 20008
	)
	q := loadShippedQuest(t, "31-find_your_footing.yaml", 31)
	steps := newLinearPlayer(q).steps
	for _, old := range []string{"start", "school", "shop", "equip", "lantern", "end"} {
		require.Contains(t, steps, old, "step %q must survive so a player mid-quest resumes", old)
	}
	turnIn := turnInRequires(t, "9106.yaml", "footing_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("room_enter", EventDetails{RoomId: schoolHall})
		require.Equal(t, "school", w.p.current)

		// Buying before listing must not skip the list step.
		w.notify("item_gain", EventDetails{ItemId: cottonShirt, RoomId: provisions})
		require.Equal(t, "school", w.p.current, "a purchase before list must not skip the list step")

		// A list anywhere but Onna's shop is not this lesson.
		w.notify("command", EventDetails{Command: "list", RoomId: marketRow})
		require.Equal(t, "school", w.p.current, "list outside Coulee Provisions must not count")

		w.notify("command", EventDetails{Command: "list", RoomId: provisions})
		require.Equal(t, "list", w.p.current)

		// Equipping before buying must not skip the buy step.
		w.notify("command", EventDetails{Command: "equip", RoomId: provisions})
		require.Equal(t, "list", w.p.current, "equip before the purchase must not skip the buy step")

		w.notify("item_gain", EventDetails{ItemId: cottonShirt, RoomId: provisions})
		require.Equal(t, "shop", w.p.current)

		// Walking into the inn before equipping must not skip the equip step.
		w.notify("room_enter", EventDetails{RoomId: lantern})
		require.Equal(t, "shop", w.p.current, "the inn before equip must not skip the equip step")

		// Putting on anything but the shirt is not this lesson (the equip
		// event names its item; 2026-10-17 newbie-duo playtest).
		w.notify("command", EventDetails{Command: "equip", RoomId: provisions, ItemId: 10043})
		require.Equal(t, "shop", w.p.current, "equipping the cudgel must not finish the shirt step")

		w.notify("command", EventDetails{Command: "equip", RoomId: provisions, ItemId: cottonShirt})
		require.Equal(t, "equip", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the inn")

		w.notify("room_enter", EventDetails{RoomId: lantern})
		require.Equal(t, "lantern", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("player saved on start resumes at the school", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("room_enter", EventDetails{RoomId: schoolHall})
		require.Equal(t, "school", w.p.current)
	})

	t.Run("player saved on school resumes with list", func(t *testing.T) {
		w := newQuestWalker(q).at("school")
		require.False(t, holdsAll(w.p, turnIn))
		w.notify("command", EventDetails{Command: "list", RoomId: provisions})
		require.Equal(t, "list", w.p.current)
		w.notify("item_gain", EventDetails{ItemId: cottonShirt, RoomId: provisions})
		require.Equal(t, "shop", w.p.current)
	})

	t.Run("player saved on shop resumes with equip", func(t *testing.T) {
		w := newQuestWalker(q).at("shop")
		w.notify("command", EventDetails{Command: "equip", RoomId: hubSquare, ItemId: cottonShirt})
		require.Equal(t, "equip", w.p.current)
	})

	t.Run("player saved on equip resumes at the inn", func(t *testing.T) {
		w := newQuestWalker(q).at("equip")
		w.notify("room_enter", EventDetails{RoomId: lantern})
		require.Equal(t, "lantern", w.p.current)
	})

	t.Run("player saved on lantern can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("lantern")
		require.True(t, holdsAll(w.p, turnIn))
	})
}
