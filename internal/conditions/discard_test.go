package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Discard deletes a record outright, so the prune pass never narrates its end
// (#444: one hide handing over to another without a reveal). RemoveCondition,
// by contrast, leaves an expired record for the prune.
func TestConditions_DiscardLeavesNothingToPrune(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := New()
	bs.AddCondition(100, false) // Haste
	bs.AddCondition(102, false) // Shadow Cloak, Hidden
	require.True(t, bs.HasFlag(Hidden, false))

	require.True(t, bs.Discard(102))
	assert.False(t, bs.HasCondition(102), "a discarded record is gone, not expired")
	assert.False(t, bs.HasFlag(Hidden, false), "the flag lookup was rebuilt")
	assert.True(t, bs.HasFlag(Haste, false), "the other record keeps its flag")
	assert.Empty(t, bs.Prune(), "nothing is left for the prune pass to narrate")
	assert.False(t, bs.Discard(102), "a second discard finds nothing")

	bs.AddCondition(102, false)
	require.True(t, bs.RemoveCondition(102))
	pruned := bs.Prune()
	require.Len(t, pruned, 1, "RemoveCondition still goes through the prune")
	assert.Equal(t, 102, pruned[0].ConditionId)
}
