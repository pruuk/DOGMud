package refusal

import (
	"regexp"
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

// TestCostText_PoolAwareProseOnlyForRefusal catches a renderer that exposes
// cost values, confuses pools, or turns a paid result into a refusal message.
func TestCostText_PoolAwareProseOnlyForRefusal(t *testing.T) {
	digit := regexp.MustCompile(`\d`)
	for _, tc := range []struct {
		name   string
		result characters.CostCommitResult
		phrase string
	}{
		{"stamina", characters.CostCommitResult{Status: characters.CostRefused, Pool: characters.PoolStamina}, "too spent"},
		{"conviction", characters.CostCommitResult{Status: characters.CostRefused, Pool: characters.PoolConviction}, "cannot muster the resolve"},
		{"paid", characters.CostCommitResult{Status: characters.CostPaid, Pool: characters.PoolStamina}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CostText(tc.result)
			if tc.phrase == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, tc.phrase)
			assert.False(t, digit.MatchString(got), "refusal prose must not expose a number")
		})
	}
}
