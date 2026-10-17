package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/require"
)

// A dodge-crit SWEEP against a swinger who is ALREADY on the floor cannot
// knock them down (ExecuteSkillMove skips the knockdown for a body that is not
// Standing), so no audience may be told they "stay up" or "keep their
// footing". The 2026-10-17 new-player playtest saw exactly that against a
// prone attacker.
func TestApplyCritEffects_SweepOnAProneSwingerDoesNotSayTheyStayUp(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)

	falseClaims := []string{"stay up", "stays up", "keep their footing", "keep your footing"}

	cases := []struct {
		name   string
		runner func(*testing.T) func(float64, []contest.Entry) contest.Result
	}{
		{"the sweep connects", attackWinContest},
		{"the sweep is deflected but still catches them", func(t *testing.T) func(float64, []contest.Entry) contest.Result {
			return deterministicContestRunner(t, -0.3, 0.0, 0.3)
		}},
		{"the sweep is answered", alwaysDefensiveCritContest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restore := combat.SetChannelAttackContestRunnerForTest(tc.runner(t))
			t.Cleanup(restore)

			attacker := proneCritEffectFighter(t, "Hound")
			defender := critEffectFighter("Tessaly", 100)

			res := applyCritEffects(attacker, defender,
				combat.AttackResult{DodgeCritDetected: true}, nil)
			require.True(t, res.AutoTrip, "a dodge crit must still fire the sweep")

			for who, line := range map[string]string{
				"defender": res.DefenderMsg, "attacker": res.AttackerMsg, "room": res.RoomMsg,
			} {
				require.NotEmpty(t, line, "%s got no sweep line", who)
				lower := strings.ToLower(line)
				for _, claim := range falseClaims {
					require.NotContains(t, lower, claim,
						"%s line claims a prone swinger is still on their feet: %q", who, line)
				}
			}
		})
	}
}

func proneCritEffectFighter(t *testing.T, name string) *characters.Character {
	t.Helper()
	c := critEffectFighter(name, 100)
	c.Position = position.NewMachine()
	require.NoError(t, c.Position.TransitionToProne(position.ProneData{},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward}))
	require.True(t, c.IsOnFloor(), "fixture is not on the floor")
	return c
}
