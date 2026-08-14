package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
)

// U5b-2: flee moved from refuse to partial charge. An exhausted character MUST
// still get to flee: go.go refuses all movement while in combat, so fleeing is
// the only player-initiated disengage. Refusing it at zero stamina would leave
// no alternative action that changes the character's situation.
func TestFleeCost_ExhaustedCharacterIsChargedPartiallyNotRefused(t *testing.T) {
	c := characters.New()
	c.StaminaMax.Base = 100
	c.StaminaMax.Recalculate()
	c.Stamina = 3

	cost := int(configs.GetBalanceConfig().FleeStaminaCost)
	if cost <= 3 {
		t.Fatalf("test fixture assumes FleeStaminaCost (%d) exceeds the 3 stamina on hand", cost)
	}

	res := c.ApplyCostPartial(characters.PoolStamina, cost)

	if res.Charged != 3 {
		t.Errorf("charged = %d, want 3 (everything that was there)", res.Charged)
	}
	if !res.Short {
		t.Error("Short = false, want true -- U8 reads this to strip the skill term")
	}
	if c.Stamina != 0 {
		t.Errorf("stamina after a partial flee charge = %d, want 0", c.Stamina)
	}
}
