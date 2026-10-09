package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Blindness that runs out on its own must give the player their sight back.
// The round's prune pass drops the spent record and calls Validate; it never
// goes through RemoveCondition, which used to be the only path that restored
// sight (playtest 2026-10-08: "Your vision slowly returns to normal." and then
// "You can't see anything!" until a relog).
func TestPruneConditions_ExpiredBlindnessRestoresSight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		perception.ConditionIdBlinded:            {ConditionId: perception.ConditionIdBlinded, Name: "Blinded", RoundInterval: 1, TriggerCount: 3, EndUserText: "Your vision slowly returns to normal."},
		perception.ConditionIdFlashbangBlindness: {ConditionId: perception.ConditionIdFlashbangBlindness, Name: "Flashbang Blindness", RoundInterval: 1, TriggerCount: 2},
	})
	defer restore()

	for _, id := range []int{perception.ConditionIdBlinded, perception.ConditionIdFlashbangBlindness} {
		holder := users.GetByUserId(1)
		require.NoError(t, holder.Character.Validate()) // the test user starts with no runtime machines
		require.NoError(t, holder.Character.AddCondition(id, false))
		require.Equal(t, perception.Blinded, holder.Character.Perception.State(), "condition %d blinds", id)

		expire(t, holder.Character.Conditions.List, id)
		PruneConditions(events.NewTurn{TurnNumber: 1})

		assert.Equal(t, perception.Sighted, holder.Character.Perception.State(), "condition %d ran out, so sight is back", id)
		drainPlain(1)
	}
}
