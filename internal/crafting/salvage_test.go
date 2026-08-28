package crafting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalcSalvageRounds(t *testing.T) {
	assert.Equal(t, 1, CalcSalvageRounds(5, 10, 5))   // 5g = 1 round (minimum)
	assert.Equal(t, 2, CalcSalvageRounds(20, 10, 5))  // 20g = 2 rounds
	assert.Equal(t, 5, CalcSalvageRounds(200, 10, 5)) // 200g = capped at 5
	assert.Equal(t, 1, CalcSalvageRounds(0, 10, 5))   // 0g = 1 round (minimum)
}
