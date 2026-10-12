package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

const (
	tauntSilencedTargetId    = 3510
	tauntSilencedConditionId = 9501
)

// A silenced taunter is refused before the target, the cooldown and the cost
// (M6 slice 3): nothing is paid. The unsilenced control proves the fixture
// does pay, so an unchanged conviction means something.
func TestExecuteTauntSilencedPaysNothing(t *testing.T) {
	pinTauntCollapseKnobs(t)
	t.Cleanup(func() { mobs.SetInstanceForTest(tauntSilencedTargetId, nil) })
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		tauntSilencedConditionId: {ConditionId: tauntSilencedConditionId, Name: "Test Silenced", Flags: []conditions.Flag{conditions.Silenced}},
	}))

	t.Run("control: an unsilenced taunter pays", func(t *testing.T) {
		attacker, _ := newTauntCollapsePair(t, tauntSilencedTargetId, 200, 10, 20, 0)
		before := attacker.Character.Conviction
		res := ExecuteTaunt(&MobActor{Mob: attacker})
		require.True(t, res.Executed, "control must execute (cost %+v)", res.Cost)
		require.False(t, res.Silenced)
		require.Less(t, attacker.Character.Conviction, before, "control must pay conviction")
	})

	t.Run("silenced taunter is refused and pays nothing", func(t *testing.T) {
		attacker, _ := newTauntCollapsePair(t, tauntSilencedTargetId, 200, 10, 20, 0)
		require.NoError(t, attacker.Character.AddCondition(tauntSilencedConditionId, true))
		before := attacker.Character.Conviction
		res := ExecuteTaunt(&MobActor{Mob: attacker})
		require.True(t, res.Silenced)
		require.False(t, res.Executed)
		require.Equal(t, before, attacker.Character.Conviction, "a silenced taunt must not spend conviction")
	})
}
