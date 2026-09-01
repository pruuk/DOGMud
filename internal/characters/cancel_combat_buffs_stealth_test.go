package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// A cross-room sniper stayed hidden shot after shot, killing a target from an
// adjacent room with impunity (reported from play 2026-08-31).
//
// usercommands/shoot.go carries a guard whose comment says it exists precisely
// to stop that: "a cross-room shooter never enters that loop, so a hidden
// sniper would stay hidden forever ... a cross-room shot that deals ANY damage
// drops stealth." It called CancelCombatBuffs and did nothing.
//
// The reason: buff 9 Hidden carries BOTH `hidden` and `cancel-on-combat`, so
// CancelCombatBuffs -> CancelBuffsWithFlag(CancelIfCombat) genuinely stripped
// the buff, but the awareness rescue inside CancelBuffsWithFlag was guarded by
// `buffFlag == buffs.Hidden` -- a question about the ARGUMENT, not about
// whether stealth had actually ended. IsHidden() reads the FSM, so the shooter
// stayed hidden.
//
// This pins the fix at the primitive rather than at the shoot call site,
// because every CancelCombatBuffs caller inherited the same hole.
func TestCancelCombatBuffs_DrivesAwarenessOutOfHidden(t *testing.T) {
	restore := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		9: {
			BuffId: 9,
			Name:   "Hidden",
			Flags:  []buffs.Flag{buffs.Hidden, buffs.CancelIfCombat},
		},
	})
	defer restore()

	c := New()
	c.Awareness = awareness.NewMachine()
	if err := c.Awareness.TransitionToConcealing(
		awareness.ConcealingData{RoundsUntil: 0},
		state.TransitionReason{Trigger: "test"}); err != nil {
		t.Fatalf("could not enter Concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, state.TransitionReason{Trigger: "test"})
	if c.Awareness.State() != awareness.Hidden {
		t.Fatalf("expected Hidden, got %v", c.Awareness.State())
	}
	if err := c.AddBuff(9, true); err != nil {
		t.Fatalf("applying buff 9 failed: %v", err)
	}
	if !c.IsHidden() {
		t.Fatal("precondition: the character should be hidden")
	}

	c.CancelCombatBuffs()

	if c.IsHidden() {
		t.Error("CancelCombatBuffs must end stealth: IsHidden() reads the " +
			"awareness FSM, so cancelling the mirror buff alone is not enough")
	}
	if c.HasBuffFlag(buffs.Hidden) {
		t.Error("the Hidden buff should also be gone")
	}
}
