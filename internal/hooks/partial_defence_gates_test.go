package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
)

// U6 Task 14 — downstream gates that assumed a defence deals zero.
//
// Since Task 10 a defended (non-crit) swing carries res.Hit == true with a
// partial damageMult, so "no hit" stopped meaning "the defence won" and a
// literal zero stopped meaning "the damage a blocked swing dealt".

// on_block procs were dispatched with a literal 0 damage on the reasoning that
// "a successful block is a defended swing (res.Hit == false)". Under the
// defence multiplier a blocked swing deals real damage, so a lifesteal-on-block
// proc would heal ratio * 0.
func TestOnBlockProcs_ReceiveTheDamageActuallyDealt(t *testing.T) {
	res := combat.AttackResult{
		Hit:            true,
		DefenseUsed:    combat.DefenseBlock,
		DamageToTarget: 7,
	}
	if got := onBlockProcDamage(res); got != 7 {
		t.Errorf("onBlockProcDamage = %d, want 7 (the damage the deflected swing actually dealt)", got)
	}

	// A block CRIT fully negates; the proc scales against zero, as before.
	crit := combat.AttackResult{
		Hit:            false,
		DefenseUsed:    combat.DefenseBlock,
		DamageToTarget: 0,
	}
	if got := onBlockProcDamage(crit); got != 0 {
		t.Errorf("onBlockProcDamage on a block crit = %d, want 0", got)
	}
}
