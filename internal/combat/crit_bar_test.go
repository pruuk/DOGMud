package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// pinCritBarConfig pins the three crit-bar knobs at their shipped values so
// the cases below stay meaningful if the test binary's defaults ever move.
func pinCritBarConfig(t *testing.T, slope, floor, ceiling float64) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.CritBarSkillSlope = configs.ConfigFloat(slope)
	cfg.Balance.CritBarFloor = configs.ConfigFloat(floor)
	cfg.Balance.CritBarCeiling = configs.ConfigFloat(ceiling)
	configs.SetConfigForTest(t, cfg)
}

// barChar builds a character whose combat skill is combatSkill. With no weapon
// equipped GetCombatSkillTag() resolves to unarmed-combat, so that is the key
// the melee pair actually reads (the plan draft wrote weapon-combat, which an
// unarmed fixture never consults).
func barChar(t *testing.T, combatSkill int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Skills["unarmed-combat"] = combatSkill
	return c
}

// CritBarFor is pure arithmetic on the CHANNEL's skill pair. These cases
// assume shipped values slope 0.05, floor 1.5, ceiling 3.0.
func TestCritBarFor(t *testing.T) {
	pinCritBarConfig(t, 0.05, 1.5, 3.0)
	cases := []struct {
		name             string
		atkRank, defRank int
		want             float64
	}{
		{"parity", 30, 30, 2.0},
		{"attacker out-skills: pins at floor", 69, 1, 1.5},
		{"defender out-skills: rises", 30, 40, 2.5},
		{"defender far out-skills: CEILING binds", 1, 69, 3.0},
		{"boss case: skill-1 mob vs spellcasting 52", 1, 52, 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CritBarFor(tc.atkRank, tc.defRank); got != tc.want {
				t.Errorf("CritBarFor(%d,%d)=%v want %v", tc.atkRank, tc.defRank, got, tc.want)
			}
		})
	}
}

// Ceiling 0 means UNCAPPED — the documented off-switch. A validator that
// "corrects" 0 back to 3.0 would make uncapping impossible; pin it.
func TestCritBarFor_ZeroCeilingIsUncapped(t *testing.T) {
	pinCritBarConfig(t, 0.05, 1.5, 0)
	if got := CritBarFor(1, 69); got != 2.0+0.05*68 {
		t.Errorf("uncapped bar = %v, want %v", got, 2.0+0.05*68)
	}
}

// #578 (owner 2026-10-19): the bar reads the skill of the hand that swings.
// A swordsman with weapon-combat 40 and an untrained fist (unarmed-combat 1)
// against a defender at 20: the sword swing pins at the floor, the off-hand
// punch sits near the ceiling. Before the fix both read the sword's 40.
func TestCalcCritThreshold_ReadsTheSwingingHand(t *testing.T) {
	pinCritBarConfig(t, 0.05, 1.5, 3.0)
	atk := characters.New()
	atk.Skills["weapon-combat"] = 40
	atk.Skills["unarmed-combat"] = 1
	sword := items.Item{ItemId: 999578} // no spec: a plain weapon, weapon-combat
	atk.Equipment.Weapon = sword
	def := barChar(t, 20)

	if got := calcCritThreshold(atk, def, sword); got != 1.5 {
		t.Errorf("sword swing bar = %v, want 1.5 (weapon-combat 40 vs 20, floored)", got)
	}
	if got, want := calcCritThreshold(atk, def, items.Item{}), 2.0+0.05*19; got != want {
		t.Errorf("off-hand punch bar = %v, want %v (unarmed-combat 1 vs 20), not the sword's", got, want)
	}
}

// Melee routes through the same function on its combat-skill pair. Identical
// to the old bar at and below 3.0; ABOVE 3.0 the new ceiling binds — the named
// melee change (a stat-rich skill-1 mob vs a veteran now caps at 3.0 instead
// of 5.4). Accuracy/Blink are gone: no branch to test.
func TestCalcCritThreshold_MeleePair(t *testing.T) {
	pinCritBarConfig(t, 0.05, 1.5, 3.0)
	cases := []struct {
		name               string
		atkSkill, defSkill int
		want               float64
	}{
		{"parity", 30, 30, 2.0},
		{"skill advantage pins at floor", 69, 1, 1.5},
		{"mob vs veteran: ceiling binds (was 5.4 uncapped)", 1, 69, 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			atk, def := barChar(t, tc.atkSkill), barChar(t, tc.defSkill)
			if got := calcCritThreshold(atk, def, atk.Equipment.Weapon); got != tc.want {
				t.Errorf("calcCritThreshold=%v want %v", got, tc.want)
			}
		})
	}
}
