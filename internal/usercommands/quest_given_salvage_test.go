package usercommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #940 (owner ruling 2026-11-22): `salvage` takes a bought copy before the
// quest's, and salvaging a quest copy destroys it at once and recovers
// nothing, with no salvage rounds. The fixtures are in
// quest_given_ways_out_test.go.
func TestQuestGear_SalvageCommand(t *testing.T) {
	const cudgelId = 94901

	t.Run("the bought copy goes first", func(t *testing.T) {
		user, room := questGearHolder(t, items.Item{})
		t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
			cudgelId: {ItemId: cudgelId, Name: "Drillyard Cudgel", NameSimple: "cudgel", Type: items.Weapon,
				SalvageReturns: []items.SalvageReturn{{ItemTag: "wood", Quantity: 1}}},
		}))
		quest := items.New(cudgelId)
		quest.QuestGiven = true
		bought := items.New(cudgelId)
		user.Character.Items = []items.Item{quest, bought}
		user.Character.Activity = activity.NewMachine()
		sentTo(user)

		_, err := Salvage("cudgel", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, "You begin carefully disassembling")
		assert.NotContains(t, out, "crumbles")
		assert.False(t, user.Character.IsFree(), "the salvage rounds began")
		assert.Len(t, user.Character.Items, 2, "nothing is destroyed until the rounds end")
	})
	t.Run("a quest copy crumbles at once", func(t *testing.T) {
		user, room := questGearHolder(t, items.Item{})
		t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
			cudgelId: {ItemId: cudgelId, Name: "Drillyard Cudgel", NameSimple: "cudgel", Type: items.Weapon,
				SalvageReturns: []items.SalvageReturn{{ItemTag: "wood", Quantity: 1}}},
		}))
		quest := items.New(cudgelId)
		quest.QuestGiven = true
		user.Character.Items = []items.Item{quest}
		user.Character.Activity = activity.NewMachine()
		sentTo(user)

		_, err := Salvage("cudgel", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		// The line wraps at the client's width, so read it unwrapped.
		assert.Contains(t, strings.Join(strings.Fields(out), " "),
			strings.Join(strings.Fields(fmt.Sprintf(actions.QuestGearSalvageLine, "Drillyard Cudgel")), " ")[:60])
		assert.Contains(t, out, "crumbles to nothing")
		assert.NotContains(t, out, "disassembling")
		assert.True(t, user.Character.IsFree(), "no salvage rounds")
		assert.Empty(t, user.Character.Items, "destroyed")
	})
}
