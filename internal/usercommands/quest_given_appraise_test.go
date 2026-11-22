package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #940 (playtest 2026-11-22): appraising a quest copy charged the fee and
// quoted a value no shop pays. It now refuses before the fee, with sell's own
// line, and appraises a bought copy of the same item when the player has
// one. The fixture is list_sight_test.go's listSightRoom (merchant "Keeper",
// a lit shop, the tin cup 84001).
func TestQuestGear_Appraise(t *testing.T) {
	// The copies are made after listSightRoom seeds the cup's spec.
	run := func(t *testing.T, questFlags ...bool) (string, int) {
		t.Helper()
		user, room := listSightRoom(t, "city")
		for _, quest := range questFlags {
			itm := items.New(84001)
			itm.QuestGiven = quest
			user.Character.Items = append(user.Character.Items, itm)
		}
		user.Character.Gold = 100
		events.DrainQueuedMessagesForTest(user.UserId)
		_, err := Appraise("tin cup", user, room, 0)
		require.NoError(t, err)
		return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"), user.Character.Gold
	}

	t.Run("a quest copy is refused before the fee", func(t *testing.T) {
		out, gold := run(t, true)
		assert.Contains(t, out, actions.SellTellQuestGiven)
		assert.NotContains(t, out, "to appraise")
		assert.Equal(t, 100, gold, "no fee")
	})
	t.Run("a bought copy beside it is appraised", func(t *testing.T) {
		out, gold := run(t, true, false)
		assert.NotContains(t, out, actions.SellTellQuestGiven)
		assert.Contains(t, out, "to appraise")
		assert.Equal(t, 80, gold)
	})
	t.Run("a bought copy alone is appraised as before", func(t *testing.T) {
		out, gold := run(t, false)
		assert.Contains(t, out, "to appraise")
		assert.Equal(t, 80, gold)
	})
}
