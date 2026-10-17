package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// A defensive crit is ONE outcome, so every audience gets ONE line for it.
//
// The 2026-10-17 pre-deploy playtest saw a defence crit print its own line and
// then a miss line from the attack pool for the same swing:
//
//	Your armor shrugs off the pitiful strike with ease!
//	Mire Brute's clumsy swing catches nothing but air!
//
// sendDefenseMessages narrated the crit to all three audiences and the swing
// loop then ran buildAttackMessages anyway, which drew a zero-damage miss line
// for the attacker, the defender and the room.
//
// Driven through resolveCombatRound so the real swing loop decides which
// composers run. Every swing records exactly one per-swing line per participant
// (swingSourceLines / swingTargetLines) and one room line, whatever its
// outcome, except a double fumble, whose comedy lines are not per-swing lines.
func TestResolveCombatRound_DefenceCritSendsOneLinePerAudience(t *testing.T) {
	pinOpeningStrikeBalance(t, 8.0)

	attacker := openingStrikeCombatant(t, "Brute")
	defender := openingStrikeCombatant(t, "Warden")
	// Outclass the attacker so the defence wins decisively and often crits.
	defender.Stats.Dexterity.Base = 900
	defender.Stats.Dexterity.Recalculate()
	ctx := combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull}

	sawDefenceCrit := false
	for i := 0; i < 80; i++ {
		attacker.Stamina = attacker.StaminaMax.Value
		defender.Stamina = defender.StaminaMax.Value
		attacker.Health = attacker.HealthMax.Value
		defender.Health = defender.HealthMax.Value
		if !attacker.Position.IsStanding() {
			attacker.Position = position.NewMachine()
		}
		if !defender.Position.IsStanding() {
			defender.Position = position.NewMachine()
		}
		attacker.SetAggro(0, 1, characters.DefaultAttack)

		result, _ := resolveCombatRound(attacker, defender, User, Mob, ctx)

		swings, doubleFumbles := 0, 0
		for _, se := range result.SwingEvents {
			swings++
			if se.DoubleFumble {
				doubleFumbles++
			}
			if se.DefenseCrit {
				sawDefenceCrit = true
			}
		}
		narrated := swings - doubleFumbles

		if got := len(result.swingTargetLines); got != narrated {
			t.Fatalf("round %d: defender got %d per-swing lines for %d narrated swings (events %+v): %v",
				i, got, narrated, result.SwingEvents, result.MessagesToTarget)
		}
		if got := len(result.swingSourceLines); got != narrated {
			t.Fatalf("round %d: attacker got %d per-swing lines for %d narrated swings (events %+v): %v",
				i, got, narrated, result.SwingEvents, result.MessagesToSource)
		}
		if got := len(result.MessagesToSourceRoom); got != swings {
			t.Fatalf("round %d: room got %d lines for %d swings (events %+v): %v",
				i, got, swings, result.SwingEvents, result.MessagesToSourceRoom)
		}
	}

	if !sawDefenceCrit {
		t.Fatal("no swing in any round was a defensive crit, so this test measured nothing")
	}
}

// A deflection the defence-crit FLOOR promotes is a defensive crit like any
// other, and narrates like one. Narration used to run before the floors, so a
// promoted swing had already sent the deflection's room line and no personal
// lines, and buildAttackMessages then supplied a miss-pool line to all three
// audiences. Floor 1.0 promotes deterministically.
func TestResolveDefenseOutcome_FloorPromotedDefenceCritNarratesAsACrit(t *testing.T) {
	src, tgt := defenceFixture(1000)
	result := &AttackResult{}
	best := defenceWinBest(15*math.Sqrt2, 15) // plain defensive win before the floor

	res := resolveDefenseOutcomeWithFloors(result, best, src, tgt, 2.0, false, false, false, 0, 1.0)

	if !res.defenseCrit || res.defended {
		t.Fatalf("fixture did not promote the deflection: defenseCrit=%v defended=%v", res.defenseCrit, res.defended)
	}
	if got := len(result.MessagesToTarget); got != 1 {
		t.Errorf("defender got %d lines from the promoted crit, want 1: %v", got, result.MessagesToTarget)
	}
	if got := len(result.MessagesToSource); got != 1 {
		t.Errorf("attacker got %d lines from the promoted crit, want 1: %v", got, result.MessagesToSource)
	}
	if got := len(result.MessagesToSourceRoom); got != 1 {
		t.Errorf("room got %d lines from the promoted crit, want 1: %v", got, result.MessagesToSourceRoom)
	}
}
