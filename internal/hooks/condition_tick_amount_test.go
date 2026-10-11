package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A condition applied through the event queue reaches the holder with
// TickAmount 0. The player round tick filled it in; the mob round tick skipped
// every record whose TickAmount was 0, so a mob's heal-over-time never healed
// and its damage-over-time never hurt (drink path unification).
func TestMobRoundTickFillsAZeroTickAmount(t *testing.T) {
	drainEventQueueOnCleanup(t)
	const (
		regenId = 88201
		rotId   = 88202
	)
	restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		// TickPercent is a FRACTION of the pool: ComputeTickAmount multiplies
		// maxPool by it directly, so 0.10 is ten percent.
		regenId: {ConditionId: regenId, Name: "Test Regen", RoundInterval: 1, TriggerCount: 3,
			TickPool: "health", TickPercent: 0.10},
		rotId: {ConditionId: rotId, Name: "Test Rot", RoundInterval: 1, TriggerCount: 3,
			TickPool: "health", TickPercent: -0.10},
	})
	defer restore()

	cases := []struct {
		name        string
		conditionId int
		want        int
	}{
		{"heal over time heals", regenId, 60},
		{"damage over time hurts", rotId, 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mob := &mobs.Mob{InstanceId: 88200, Character: characters.Character{
				Health:     50,
				Conditions: conditions.New(),
			}}
			mob.Character.HealthMax.Value = 100
			require.True(t, mob.Character.Conditions.AddCondition(tc.conditionId, false))
			require.Equal(t, 0, mob.Character.Conditions.List[0].TickAmount,
				"precondition: a record added without the applier's snapshot carries TickAmount 0")

			tickMobConditions(mob, mob.InstanceId)

			assert.Equal(t, tc.want, mob.Character.Health)
			assert.Equal(t, tc.want-50, mob.Character.Conditions.List[0].TickAmount,
				"the computed amount is cached on the record, as the player tick does")
		})
	}
}
