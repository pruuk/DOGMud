package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
)

// U5b-2 removed the seven retained health floors, so health stores overkill
// consistently at every damage site. U6 reads that magnitude.
//
// This pins the primitive contract all seven sites now rely on: harm floors
// stamina and conviction at zero but deliberately does NOT floor health.
// validate.go carries a matching explicit "No lower Health clamp" comment.
func TestApplyHarm_HealthStoresOverkill(t *testing.T) {
	c := characters.New()
	c.HealthMax.Base = 100
	c.HealthMax.Recalculate()
	c.Health = 10

	applied := c.ApplyHarm(characters.PoolHealth, 35, state.ActorRef{})

	if c.Health != -25 {
		t.Errorf("health after 35 damage to a 10-health character = %d, want -25 (overkill preserved for U6)", c.Health)
	}
	if applied != 35 {
		t.Errorf("applied = %d, want 35 (the full amount, not the amount that fit)", applied)
	}
}

// Every death gate in the game tests `< 1` or `<= 0` -- all 57 health
// comparisons in internal/ and modules/ were audited and none is `== 0` -- so a
// negative value passes all of them. This is the assertion that makes removing
// the floors safe.
func TestApplyHarm_NegativeHealthStillReadsAsDead(t *testing.T) {
	c := characters.New()
	c.HealthMax.Base = 100
	c.HealthMax.Recalculate()
	c.Health = 1

	c.ApplyHarm(characters.PoolHealth, 50, state.ActorRef{})

	if !(c.Health < 1) {
		t.Errorf("health = %d does not satisfy the `< 1` death gate", c.Health)
	}
	if !(c.Health <= 0) {
		t.Errorf("health = %d does not satisfy the `<= 0` death gate", c.Health)
	}
}
