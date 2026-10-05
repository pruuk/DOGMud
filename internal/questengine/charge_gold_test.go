package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChargeGold_Deducts(t *testing.T) {
	ctx := newMockActionContext(1)
	ctx.gold = 100

	err := ExecuteAction(ActionDef{ChargeGold: 30}, ctx)

	assert.NoError(t, err)
	assert.Equal(t, 70, ctx.gold)
}

// A charge the player cannot cover takes nothing and fails, and ExecuteAction
// surfaces the refusal so the engine abandons the trigger's remaining actions.
func TestChargeGold_RefusesWhenShort(t *testing.T) {
	ctx := newMockActionContext(1)
	ctx.gold = 20

	err := ExecuteAction(ActionDef{ChargeGold: 50}, ctx)

	assert.Error(t, err)
	assert.Equal(t, 20, ctx.gold, "a refused charge takes nothing")
}

func TestHasGold_Condition(t *testing.T) {
	rich := newMockPlayer(100)
	rich.gold = 60
	assert.True(t, EvalConditions(Conditions{HasGold: 50}, rich))

	poor := newMockPlayer(100)
	poor.gold = 40
	assert.False(t, EvalConditions(Conditions{HasGold: 50}, poor))
}
