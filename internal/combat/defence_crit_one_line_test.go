package combat

import (
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/util"
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

// A defensive crit still rolls the feint it rolled before 75dd16773 moved its
// narration (U6 Task 14: a defensive crit is a zero-damage outcome a skilled
// attacker can read as a deliberate feint). The feint stands in for the
// defence lines, so the swing still has one line per audience. Seeded, and
// driven through resolveCombatRound so the real swing loop makes the roll. At
// unarmed 100 the feint chance is the 75% cap, so across many defensive
// crits a run with no feint at all means the roll is gone.
func TestResolveCombatRound_DefenceCritCanStillFeint(t *testing.T) {
	pinOpeningStrikeBalance(t, 8.0)
	t.Cleanup(util.SetRandForTest(1017))

	attacker := openingStrikeCombatant(t, "Brute")
	defender := openingStrikeCombatant(t, "Warden")
	attacker.Skills[string(skills.UnarmedCombat)] = 100
	defender.Stats.Dexterity.Base = 900
	defender.Stats.Dexterity.Recalculate()
	ctx := combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull}

	isFeintLine := func(text string) bool {
		for _, fm := range feintMessages {
			prefix := fm.toAttacker
			if i := strings.IndexAny(prefix, "<{"); i > 0 {
				prefix = prefix[:i]
			}
			if strings.HasPrefix(text, prefix) {
				return true
			}
		}
		return false
	}

	defenceCrits, feints := 0, 0
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
		narrated := 0
		for _, se := range result.SwingEvents {
			if se.DefenseCrit {
				defenceCrits++
			}
			if !se.DoubleFumble {
				narrated++
			}
		}
		if got := len(result.swingSourceLines); got != narrated {
			t.Fatalf("round %d: attacker got %d per-swing lines for %d narrated swings", i, got, narrated)
		}
		for _, idx := range result.swingSourceLines {
			if isFeintLine(result.MessagesToSource[idx].Text) {
				feints++
			}
		}
	}

	if defenceCrits < 20 {
		t.Fatalf("only %d defensive crits; the fixture cannot measure the feint roll", defenceCrits)
	}
	if feints == 0 {
		t.Fatalf("%d defensive crits at the 75%% feint cap produced no feint: the defensive crit's feint roll is gone", defenceCrits)
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

	res := resolveDefenseOutcomeWithFloors(result, best, src, tgt, 2.0, false, false, false, 0, 1.0, nil)

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
