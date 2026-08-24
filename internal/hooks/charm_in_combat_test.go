package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
)

// Spec 4.1 lists charm's in-combat penalty as UNCHANGED across the U10c
// rewrite. Slice B deleted it with resolveCharmSpell and replaced it with
// nothing, which was a silent balance change: no channel-level penalty covers
// a social attack on a target already fighting (SituationalAttackMult returns
// 1.0 for everything but melee and ranged), so charm got easier mid-fight while
// three separate files kept telling players it had got harder.
func charmTargetInCombatWith(t *testing.T, targetUserId int) *characters.Character {
	t.Helper()
	c := &characters.Character{}
	c.SetAggro(targetUserId, 0, characters.DefaultAttack)
	return c
}

func TestCharmInCombatMult_TargetFightingCasterTakesSteepestPenalty(t *testing.T) {
	const casterId = 7
	target := charmTargetInCombatWith(t, casterId)

	if !target.IsInCombat() {
		t.Fatalf("fixture: target should be in combat")
	}
	if got := charmInCombatMult(target, casterId); got != 0.75 {
		t.Errorf("charmInCombatMult = %v against a target fighting the caster, want 0.75", got)
	}
}

func TestCharmInCombatMult_TargetFightingSomeoneElseTakesModeratePenalty(t *testing.T) {
	target := charmTargetInCombatWith(t, 99) // fighting a different player

	if got := charmInCombatMult(target, 7); got != 0.85 {
		t.Errorf("charmInCombatMult = %v against a target fighting someone else, want 0.85", got)
	}
}

func TestCharmInCombatMult_IdleTargetIsUnpenalised(t *testing.T) {
	idle := &characters.Character{}
	if idle.IsInCombat() {
		t.Fatalf("fixture: a fresh character should not be in combat")
	}
	if got := charmInCombatMult(idle, 7); got != 1.0 {
		t.Errorf("charmInCombatMult = %v against an idle target, want 1.0", got)
	}
}

func TestCharmInCombatMult_NilTargetIsUnpenalised(t *testing.T) {
	if got := charmInCombatMult(nil, 7); got != 1.0 {
		t.Errorf("charmInCombatMult = %v for a nil target, want 1.0", got)
	}
}

// Mult 0 is the ZERO VALUE, and AttackSide.score() reads it as "unset, 1.0"
// (defence_multiplier.go:78-83). So `side.Mult *= 0.75` on a side that never set
// Mult yields 0, which reads back as 1.0 -- the penalty vanishes while producing
// an entirely plausible attack score. This pins the normalisation that prevents
// it, because the bug is invisible in every other way.
func TestCharmInCombat_UnsetMultIsNormalisedBeforeScaling(t *testing.T) {
	const casterId = 7
	target := charmTargetInCombatWith(t, casterId)

	side := combat.AttackSide{Stat: 100, Mult: 0}

	// The production sequence from resolveAgainstMob.
	if side.Mult == 0 {
		side.Mult = 1.0
	}
	side.Mult *= charmInCombatMult(target, casterId)

	if side.Mult != 0.75 {
		t.Errorf("Mult = %v after penalising an unset side, want 0.75; a bare "+
			"multiply would leave 0 here, which score() reads back as 1.0 and "+
			"the penalty disappears silently", side.Mult)
	}
}
