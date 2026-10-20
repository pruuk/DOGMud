package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #631: a beast-move bleed on a player ticks once per round for its stack's
// rounds and then says it stopped. The playtest read eight "Blood seeps from
// your wounds!" lines and no end line; this drives the round tick and the
// turn prune the way the server interleaves them and pins both halves: one
// line per round while the stack lives, then the record's end line, once.
func TestBleed_BeastMoveStackEndsWithItsEndLine_Player(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	u.Character.HealthMax.Base = 100
	// A one-point stack, as a weak beast's hamstring lands it.
	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 8, -1, "hamstring")
	u.Character.Health = 80
	_ = drainPlain(1)

	end := conditions.GetConditionSpec(conditions.ConditionIdBleeding).EndUserNotice()
	require.NotEmpty(t, end)

	ticks, ends := 0, 0
	for round := 1; round <= 10; round++ {
		UserRoundTick(events.NewRound{RoundNumber: uint64(round)})
		PruneConditions(events.NewTurn{TurnNumber: uint64(round)})
		lines := drainPlain(1)
		ticks += countContaining(lines, "Blood seeps from your wounds!")
		ends += countContaining(lines, end)
	}
	assert.Equal(t, 8, ticks, "one bleed line per round of the stack")
	assert.Equal(t, 1, ends, "the bleed says once that it stopped")
	assert.False(t, u.Character.HasCondition(conditions.ConditionIdBleeding))
}
