package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/statmods"
)

// meleePredictionTolerance bounds how far the fight read's melee damage per
// round may sit from what live combat measures. The prediction applies the
// contest floor and gap compression but skips fumbles and partial deflection
// by design, so it is loose; a combat change that bypasses the shared helpers moves the
// measurement far past it. Do not widen it to make a red run pass: report the
// deltas to the owner instead (the spec's bands assume this roughness).
const meleePredictionTolerance = 0.30

const meleePredictionRounds = 40000

func statCombatant(t *testing.T, name string, stat, rank int) *characters.Character {
	t.Helper()
	c := parityCombatant(t, name)
	c.Stats.Strength.Base, c.Stats.Dexterity.Base, c.Stats.Vitality.Base = stat, stat, stat
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Stats.Vitality.Recalculate()
	c.Skills[string(skills.UnarmedCombat)] = rank
	return c
}

// Drift guard 3 (#301): the predicted melee damage per round lands within
// tolerance of the real seam, resolveCombatRound, over many rounds.
func TestMeleePredictionMatchesCombat(t *testing.T) {
	if testing.Short() {
		t.Skip("statistical guard; skipped in -short")
	}
	pinParityBalance(t)

	cases := []struct {
		name                       string
		atkStat, atkRank           int
		defStat, defRank, defMitPc int
	}{
		{"even pair", 100, 30, 100, 30, 0},
		{"outclassed attacker", 80, 10, 140, 40, 0},
		{"armoured defender", 120, 30, 100, 20, 40},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			attacker := statCombatant(t, "attacker", c.atkStat, c.atkRank)
			attacker.SetAggro(0, 1, characters.DefaultAttack)
			defender := statCombatant(t, "defender", c.defStat, c.defRank)
			if c.defMitPc > 0 {
				t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
					9401: {ConditionId: 9401, Name: "test mitigation", TriggerCount: 1,
						StatMods: statmods.StatMods{"physical_mitigation": c.defMitPc}},
				}))
				defender.Conditions.List = append(defender.Conditions.List,
					&conditions.Condition{ConditionId: 9401, TriggersLeft: 1000000000})
				defender.Conditions.Validate(true)
			}

			want := meleeDamagePerRound(attacker, defender, User)
			ctx := neutralCtx()
			total := 0.0
			for i := 0; i < meleePredictionRounds; i++ {
				attacker.Stamina, attacker.Health = attacker.StaminaMax.Value, attacker.HealthMax.Value
				defender.Stamina, defender.Health = defender.StaminaMax.Value, defender.HealthMax.Value
				if !attacker.Position.IsStanding() {
					attacker.Position = position.NewMachine()
				}
				if !defender.Position.IsStanding() {
					defender.Position = position.NewMachine()
				}
				result, _ := resolveCombatRound(attacker, defender, User, User, ctx)
				for _, ev := range result.SwingEvents {
					total += float64(ev.Damage)
				}
			}
			got := total / meleePredictionRounds
			delta := (want - got) / got
			t.Logf("%s: predicted %.3f measured %.3f per round (delta %+.1f%%)", c.name, want, got, delta*100)
			if math.Abs(delta) > meleePredictionTolerance {
				t.Errorf("prediction %.3f is %+.1f%% from measured %.3f (tolerance %.0f%%)",
					want, delta*100, got, meleePredictionTolerance*100)
			}
		})
	}
}
