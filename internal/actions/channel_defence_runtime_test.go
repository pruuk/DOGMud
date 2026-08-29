package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/require"
)

func TestExecuteTauntDefensiveCritPreservesAggroPullForPlayerAndMobActors(t *testing.T) {
	for _, player := range []bool{true, false} {
		name := "mob"
		if player {
			name = "player"
		}
		t.Run(name, func(t *testing.T) {
			cfg := configs.GetConfig()
			cfg.Balance.ContestFloor = 0
			cfg.Balance.MinAttackCritChance = 0
			configs.SetConfigForTest(t, cfg)

			// U6b Task 5: the old gate stub (runTauntContest) is gone with the
			// gate. Force a decisive DEFENSIVE crit through the one seam
			// contest instead: a -4 sigma attack margin fully negates
			// (DamageMultiplier 0), with clean roll z-scores so neither a
			// fumble nor a defence fumble fires by accident.
			restore := combat.SetChannelAttackContestRunnerForTest(tauntDeterministicRunner(t, -4, 0.5, 2.5))
			t.Cleanup(restore)

			actor, _, target := newRhetoricActor(t, player, 100, 0)
			target.SetAggro(4242, 0, characters.DefaultAttack)
			startingConviction := target.Conviction

			result := ExecuteTaunt(actor)

			require.True(t, result.Executed)
			require.True(t, result.Hit)
			require.True(t, result.Defence.DefensiveCrit)
			require.Zero(t, result.Damage)
			require.Equal(t, startingConviction-result.Defence.Cost.Charged, target.Conviction,
				"only the admitted defy cost may reduce conviction on a zero-injury defensive crit")
			require.True(t, result.AggroPulled)
			if player {
				require.Equal(t, actor.GetUserId(), target.CurrentCombatTarget().UserId)
			} else {
				require.Equal(t, actor.GetMobInstanceId(), target.CurrentCombatTarget().MobInstanceId)
			}
		})
	}
}
