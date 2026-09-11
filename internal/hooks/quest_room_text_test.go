package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Quest room_text went out RAW on the audio channel: no token substitution, so
// quest 77 showed players a literal {source}; and no sight gate, so a blind
// observer still read "unlocks the strongbox". The behaviour tree reads the same
// key and already did both correctly (behaviortree/actions_dialogue.go).

func TestQuestRoomText_NamesThePlayerToASightedObserver(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	drainPlain(1)
	drainPlain(2)

	questengine.NewGameBridge(users.GetByUserId(1), 1).RoomText("{source} unlocks the strongbox.")

	observer := drainPlain(2)
	assert.Equal(t, 1, countContaining(observer, "Aliceia unlocks the strongbox."))
	assert.Equal(t, 0, countContaining(observer, "{source}"), "a literal token reached a player")
	assert.Equal(t, 0, countContaining(drainPlain(1), "unlocks the strongbox"),
		"the acting player is excluded from their own room line")
}

func TestQuestRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	darken(t, 1)
	drainPlain(2)

	questengine.NewGameBridge(users.GetByUserId(1), 1).RoomText("{source} unlocks the strongbox.")
	assert.Equal(t, 0, countContaining(drainPlain(2), "unlocks the strongbox"))
}

// TestQuestRoomText_InfraredObserverSeesAFigure proves the name carries the tag
// messaging.Anonymize strips. An untagged name would reach an infrared-only
// observer in full.
func TestQuestRoomText_InfraredObserverSeesAFigure(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(heatEyesBuffId, true))
	drainPlain(2)

	questengine.NewGameBridge(users.GetByUserId(1), 1).RoomText("{source} unlocks the strongbox.")

	observer := drainPlain(2)
	require.Equal(t, 1, countContaining(observer, "unlocks the strongbox"))
	for _, line := range observer {
		if strings.Contains(line, "unlocks the strongbox") {
			assert.NotContains(t, line, "Aliceia", "an infrared-only observer read the name")
			// The pipeline capitalises a sentence-initial placeholder.
			assert.Contains(t, strings.ToLower(line), "a figure")
		}
	}
}
