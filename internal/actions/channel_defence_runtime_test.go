package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
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
			cfg.Balance.DefyEffectiveness = 1_000_000
			cfg.Balance.ContestFloor = 0
			configs.SetConfigForTest(t, cfg)

			originalContest := runTauntContest
			runTauntContest = func(float64, []contest.Entry) contest.Result {
				return contest.Result{
					AttackRoll:  dice.RollResult{Value: 101, Mean: 100, StdDev: 15},
					DefenseRoll: dice.RollResult{Value: 100, Mean: 100, StdDev: 15},
					Margin:      1, Contested: true, Success: true,
				}
			}
			t.Cleanup(func() { runTauntContest = originalContest })

			actor, _, target := newRhetoricActor(t, player, 100, 0)
			target.Aggro = &characters.Aggro{UserId: 4242}
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
				require.Equal(t, actor.GetUserId(), target.Aggro.UserId)
			} else {
				require.Equal(t, actor.GetMobInstanceId(), target.Aggro.MobInstanceId)
			}
		})
	}
}
