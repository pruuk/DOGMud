package characters

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBloomAdvanceSkipsOverBudgetRanks(t *testing.T) {
	// Six units: neither Titan Growth 4 nor Extra Arms 4 fits, and the only
	// unowned seeded mutation (Colossus Form) lacks Titan Growth 4.
	c := grown(t, 3, 3, false)
	id, _ := c.BloomAdvanceMutation(rand.New(rand.NewSource(1)))
	assert.Equal(t, "", id)
	assert.Equal(t, 3, c.Mutations["titan-growth"])
	assert.Equal(t, 3, c.Mutations["extra-arms"])
}

func TestBloomSeedHonoursPrerequisites(t *testing.T) {
	for i := int64(0); i < 50; i++ {
		c := grown(t, 0, 1, false)
		id, _ := c.BloomSeedNewMutation(rand.New(rand.NewSource(i)))
		assert.NotEqual(t, "colossus-form", id, "Titan Growth 1 does not meet Colossus Form")
	}
}
