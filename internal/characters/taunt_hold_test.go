package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// A taunt forces the target onto the taunter and locks it there for a few
// rounds, so reactive re-aggro (the per-round `attack` re-target) can't
// immediately flip the target back. This is the core of the taunt-holds-aggro
// fix.

func TestTauntHold_BlocksReaggroToDifferentTargetWhileActive(t *testing.T) {
	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	c := &Character{}
	// Enemy is currently fighting player 1.
	c.SetAggro(1, 0, DefaultAttack, 0)
	if !c.IsInCombat() || c.CurrentCombatTarget().UserId != 1 {
		t.Fatalf("setup: expected aggro on user 1, got %+v", c.CurrentCombatTarget())
	}

	// A companion golem (mob instance 50) taunts: force aggro + 4-round hold.
	c.SetTauntHold(0, 50, 4)
	c.SetAggro(0, 50, DefaultAttack)
	if !c.IsInCombat() || c.CurrentCombatTarget().MobInstanceId != 50 || c.CurrentCombatTarget().UserId != 0 {
		t.Fatalf("taunt: expected aggro forced to mob 50, got %+v", c.CurrentCombatTarget())
	}

	// The player keeps swinging → reactive re-aggro to player 1. While the
	// taunt hold is active this MUST be ignored (golem keeps the aggro).
	c.SetAggro(1, 0, DefaultAttack, 0)
	if c.CurrentCombatTarget().MobInstanceId != 50 || c.CurrentCombatTarget().UserId != 0 {
		t.Fatalf("hold active: expected aggro to stay on mob 50, got %+v", c.CurrentCombatTarget())
	}
}

func TestTauntHold_ExpiresAfterHoldRounds(t *testing.T) {
	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	c := &Character{}
	c.SetTauntHold(0, 50, 4)
	c.SetAggro(0, 50, DefaultAttack) // hold until round 104

	// Advance to the expiry round; the hold should no longer block.
	util.SetRoundCountForTest(104)
	c.SetAggro(1, 0, DefaultAttack, 0)
	if !c.IsInCombat() || c.CurrentCombatTarget().UserId != 1 {
		t.Fatalf("after hold expiry: expected re-aggro to user 1, got %+v", c.CurrentCombatTarget())
	}
}

func TestTauntHold_NewerTauntOverridesActiveHold(t *testing.T) {
	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	c := &Character{}
	c.SetTauntHold(0, 50, 4)
	c.SetAggro(0, 50, DefaultAttack)
	// A second taunter (mob 60) taunts while the first hold is still active —
	// the newer taunt must win.
	c.SetTauntHold(0, 60, 4)
	c.SetAggro(0, 60, DefaultAttack)
	if !c.IsInCombat() || c.CurrentCombatTarget().MobInstanceId != 60 {
		t.Fatalf("re-taunt: expected aggro on mob 60, got %+v", c.CurrentCombatTarget())
	}
}

func TestTauntHold_ClearedByEndAggro(t *testing.T) {
	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	c := &Character{}
	c.SetTauntHold(0, 50, 4)
	c.SetAggro(0, 50, DefaultAttack)
	// Taunter dies / leaves the room → ValidateAggro calls EndAggro, which
	// must clear the hold so the enemy can re-acquire a target instead of
	// standing locked onto a gone taunter.
	c.EndAggro()
	c.SetAggro(1, 0, DefaultAttack, 0)
	if !c.IsInCombat() || c.CurrentCombatTarget().UserId != 1 {
		t.Fatalf("after EndAggro: expected re-aggro to user 1, got %+v", c.CurrentCombatTarget())
	}
}

func TestTauntHold_SpellcastReaggroNotBlocked(t *testing.T) {
	util.SetRoundCountForTest(100)
	defer util.ResetRoundCountForTest()

	c := &Character{}
	c.SetTauntHold(0, 50, 4)
	c.SetAggro(0, 50, DefaultAttack)
	// A SpellCast aggro (self/room-directed, no target) must not be blocked
	// by the hold — the hold only pins basic-attack re-targets.
	c.SetAggro(0, 0, SpellCast, 0)
	if !c.IsInCombat() || c.Aggro.Type != SpellCast {
		t.Fatalf("spellcast: expected SpellCast aggro to pass through, got %+v", c.CurrentCombatTarget())
	}
}
