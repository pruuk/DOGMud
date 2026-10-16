package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

func TestCost(t *testing.T) {
	f := &fakeRecipient{}
	Cost(f, characters.CostCommitResult{Status: characters.CostRefused, Pool: characters.PoolStamina})
	assert.Equal(t, "You are too spent to manage that right now.", one(t, f, messaging.CategoryRefusal))
	assert.Equal(t, "You cannot muster the resolve for that right now.",
		CostText(characters.CostCommitResult{Status: characters.CostRefused, Pool: characters.PoolConviction}))

	f = &fakeRecipient{}
	Cost(f, characters.CostCommitResult{Status: characters.CostPaid, Pool: characters.PoolStamina})
	assert.Empty(t, f.got, "a cost that was paid sends nothing")
}
