package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/statmods"
)

func TestPredictFight_StrongerSideCostsLess(t *testing.T) {
	pinParityBalance(t)
	strong := parityCombatant(t, "strong")
	strong.Stats.Strength.Base, strong.Stats.Dexterity.Base = 160, 160
	strong.Stats.Strength.Recalculate()
	strong.Stats.Dexterity.Recalculate()
	weak := parityCombatant(t, "weak")
	strong.Health, weak.Health = 300, 300
	strong.HealthMax.Value, weak.HealthMax.Value = 300, 300

	p := PredictFight(strong, weak)
	q := PredictFight(weak, strong)
	if !(p.Cost < q.Cost && p.Dent > q.Dent) {
		t.Fatalf("strong reads cost %v dent %v; weak reads cost %v dent %v", p.Cost, p.Dent, q.Cost, q.Dent)
	}
	if p.RoundsToKillThem >= q.RoundsToKillThem {
		t.Fatalf("strong needs %v rounds, weak %v", p.RoundsToKillThem, q.RoundsToKillThem)
	}
}

// Zero damage is infinity, explicitly: no division by zero, no NaN.
func TestPredictFight_ZeroDamageIsInfinite(t *testing.T) {
	p := FightPrediction{}
	finish(&p, 100, 100)
	if !math.IsInf(p.RoundsToKillThem, 1) || !math.IsInf(p.RoundsToKillYou, 1) || p.Cost != 0 || p.Dent != 0 {
		t.Fatalf("zero both ways = %+v", p)
	}

	// You can hurt them, they cannot hurt you: you take nothing, and you would
	// deal them everything given forever.
	q := FightPrediction{YourDamagePerRound: 5}
	finish(&q, 100, 100)
	if q.Cost != 0 || !math.IsInf(q.Dent, 1) || q.RoundsToKillThem != 20 {
		t.Fatalf("one-way damage = %+v", q)
	}
}

// Lifesteal on your weapon comes off the damage you take.
func TestPredictFight_LifestealReducesIncoming(t *testing.T) {
	pinParityBalance(t)
	you := parityCombatant(t, "you")
	them := parityCombatant(t, "them")
	you.Stats.Strength.Base = 160
	you.Stats.Strength.Recalculate()
	you.Health, them.Health = 300, 300
	before := PredictFight(you, them).TheirDamagePerRound

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9301: {ConditionId: 9301, Name: "test lifesteal", TriggerCount: 1, StatMods: statmods.StatMods{"lifesteal_pct": 50}},
	}))
	you.Conditions.List = append(you.Conditions.List, &conditions.Condition{ConditionId: 9301, TriggersLeft: 1000})
	you.Conditions.Validate(true)

	after := PredictFight(you, them).TheirDamagePerRound
	want := 0.5 * meleeDamagePerRound(you, them, User)
	if want <= 0 {
		t.Fatalf("test needs your melee above zero, got %v", want)
	}
	if got := before - after; math.Abs(got-want) > 1e-9 {
		t.Fatalf("lifesteal took %v off incoming (%v -> %v), want %v", got, before, after, want)
	}
}
