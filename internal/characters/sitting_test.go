package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/stats"
)

// Sitting (#637): a rest posture that boosts regen by SitRegenMultiplier
// (below sleep), is not a knockdown, and is stood up by combat.

func newSitterForTest(t *testing.T) *Character {
	t.Helper()
	c := New()
	c.Validate()
	c.HealthMax = stats.StatInfo{Value: 1000}
	c.StaminaMax = stats.StatInfo{Value: 1000}
	c.ConvictionMax = stats.StatInfo{Value: 1000}
	if err := c.Position.TransitionToSitting(state.TransitionReason{Trigger: position.TriggerSitCommand}); err != nil {
		t.Fatalf("sit transition: %v", err)
	}
	return c
}

func pinRestRegen(t *testing.T, sit, sleep float64) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.SitRegenMultiplier = configs.ConfigFloat(sit)
	cfg.Balance.SleepRegenMultiplier = configs.ConfigFloat(sleep)
	configs.SetConfigForTest(t, cfg)
}

// Every pool is boosted by exactly the sit knob, and by less than sleep.
func TestSittingRegen_BoostedByTheKnobAndBelowSleep(t *testing.T) {
	cleanup := seedSleepingCondition()
	defer cleanup()
	pinRestRegen(t, 2.0, 5.0)

	c := newSitterForTest(t)
	sitHP, sitSP, sitCP := c.HealthPerRound(), c.StaminaPerRound(), c.ConvictionPerRound()

	_ = c.Position.TransitionToStanding(state.TransitionReason{Trigger: position.TriggerStandCommand})
	baseHP, baseSP, baseCP := c.HealthPerRound(), c.StaminaPerRound(), c.ConvictionPerRound()

	for _, p := range []struct {
		name      string
		base, sit int
	}{{"health", baseHP, sitHP}, {"stamina", baseSP, sitSP}, {"conviction", baseCP, sitCP}} {
		if p.sit != p.base*2 {
			t.Errorf("%s regen while sitting = %d, want 2x the standing %d (SitRegenMultiplier 2.0)", p.name, p.sit, p.base)
		}
	}

	applySleepingCondition(c)
	if sleepHP := c.HealthPerRound(); sleepHP <= sitHP {
		t.Errorf("sleep regen %d must beat sit regen %d (owner ruling)", sleepHP, sitHP)
	}
}

// A sleeping sitter regenerates at the sleep rate: the two never stack.
func TestSittingRegen_DoesNotStackWithSleep(t *testing.T) {
	cleanup := seedSleepingCondition()
	defer cleanup()
	pinRestRegen(t, 2.0, 5.0)

	c := newSitterForTest(t)
	if got := c.RestRegenMultiplier(); got != 2.0 {
		t.Fatalf("sitter multiplier = %v, want 2.0", got)
	}
	applySleepingCondition(c)
	if got := c.RestRegenMultiplier(); got != 5.0 {
		t.Fatalf("sleeping sitter multiplier = %v, want the sleep rate 5.0 alone", got)
	}
}

// Sitting is not a knockdown: the #398 recovery path, auto-stand guard
// included, leaves a sitter seated however many rounds pass.
func TestSitting_AttemptRecoveryAndAutoStandIgnoreIt(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.ProneAutoStandRounds = 1
	configs.SetConfigForTest(t, cfg)

	c := newSitterForTest(t)
	for round := 1; round <= 10; round++ {
		if attempted, success := c.AttemptRecovery(nil); attempted || success {
			t.Fatalf("round %d: AttemptRecovery on a sitter = (%v, %v), want (false, false)", round, attempted, success)
		}
	}
	if !c.IsSitting() {
		t.Fatalf("a sitter must stay seated through recovery ticks, now %v", c.Position.State())
	}
}

// Being attacked stands a sitter (RevealForCombat), and RiseForCombat is a
// no-op for anyone not sitting.
func TestSitting_RevealForCombatStandsASitter(t *testing.T) {
	c := newSitterForTest(t)
	c.RevealForCombat()
	if !c.IsStanding() {
		t.Fatalf("an attacked sitter must stand, now %v", c.Position.State())
	}
	if c.RiseForCombat() {
		t.Fatal("RiseForCombat on a stander reported a rise")
	}

	prone := New()
	prone.Validate()
	_ = prone.Position.TransitionToProne(position.ProneData{MinRecoveryRounds: 2},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward})
	if prone.RiseForCombat() || !prone.IsProne() {
		t.Fatal("RiseForCombat must not stand a knocked-down character")
	}
}

// A death runs CancelCombatConditions; that must NOT stand (and so narrate)
// a dying sitter. Only RiseForCombat's two callers stand a sitter.
func TestSitting_CancelCombatConditionsLeavesASitter(t *testing.T) {
	c := newSitterForTest(t)
	c.CancelCombatConditions()
	if !c.IsSitting() {
		t.Fatalf("CancelCombatConditions stood a sitter, now %v", c.Position.State())
	}
}
