package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #940: look (and examine, which expands to look) says a carried copy is
// quest gear, so no one loses one by surprise. The fixtures are in
// quest_given_ways_out_test.go.
func TestQuestGear_LookSaysSo(t *testing.T) {
	t.Run("quest gear", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		_, err := Look("iron sword", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), actions.QuestGearLookLine)
	})
	t.Run("bought copy", func(t *testing.T) {
		user, room := questGearHolder(t, boughtCopy())
		_, err := Look("iron sword", user, room, 0)
		require.NoError(t, err)
		assert.NotContains(t, sentTo(user), actions.QuestGearLookLine)
	})
}
