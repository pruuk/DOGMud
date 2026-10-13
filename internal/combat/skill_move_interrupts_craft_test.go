package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/require"
)

// #514: a special move (gore, kick, bash, maul) that lands damage on a crafter
// ends the craft. It used to survive because only melee and the damage-over-time
// ticks called a hand-placed cancel; the cancel now lives in
// Character.ApplyHarm, which ExecuteSkillMove reaches.
//
// Each iteration is one move on a fresh crafting defender: the craft must end
// exactly when the move took health, and not otherwise (a fully defended
// attempt leaves the craft running).
func TestExecuteSkillMove_DamageEndsTheDefendersCraft(t *testing.T) {
	pinDefenceAdmissionConfig(t)

	const iterations = 500
	damaged := 0
	for i := 0; i < iterations; i++ {
		attacker := characters.New()
		defender := characters.New()
		defender.HealthMax.Value = 100000
		defender.Health = 100000
		defender.StaminaMax.Value = 200
		defender.Stamina = 200

		require.NoError(t, defender.Activity.TransitionToCrafting(
			activity.CraftingData{RecipeId: "skill-move-craft-test", RoundsTotal: 5},
			state.TransitionReason{Trigger: activity.TriggerCraftBegin},
		))

		healthBefore := defender.Health
		ExecuteSkillMove(makePartialTestParams(attacker, defender))

		if defender.Health < healthBefore {
			damaged++
			require.Equal(t, activity.Free, defender.Activity.State(),
				"a special move that took health must end the craft")
		} else {
			require.Equal(t, activity.Crafting, defender.Activity.State(),
				"a move that took no health must leave the craft running")
		}
	}
	require.Greater(t, damaged, 0, "no move in %d tries landed damage, so the test proved nothing", iterations)
}
