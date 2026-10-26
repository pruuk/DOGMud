package characters

import (
	"math/rand"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutations"
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

// Dense Muscles is seeded and owned, so the only prerequisite Colossus Form
// lacks at Titan Growth 1 is Titan Growth 4. The control at Titan Growth 4
// proves the draw can reach Colossus Form once that prerequisite is met.
func TestBloomSeedHonoursPrerequisites(t *testing.T) {
	withDense := func(titan int) *Character {
		c := grown(t, 0, titan, false)
		specs := map[string]*mutations.MutationSpec{
			"dense-muscles": {MutationId: "dense-muscles", Name: "Dense Muscles", Rarity: 3,
				Clusters: []string{"colossus"}},
		}
		for _, id := range []string{"titan-growth", "colossus-form", "extra-arms"} {
			specs[id] = mutations.GetMutation(id)
		}
		t.Cleanup(mutations.SeedMutationsForTest(specs))
		c.Mutations["dense-muscles"] = 1
		return c
	}

	sawColossus := false
	for i := int64(0); i < 50; i++ {
		id, _ := withDense(1).BloomSeedNewMutation(rand.New(rand.NewSource(i)))
		assert.NotEqual(t, "colossus-form", id, "Titan Growth 1 does not meet Colossus Form")
		got, _ := withDense(4).BloomSeedNewMutation(rand.New(rand.NewSource(i)))
		sawColossus = sawColossus || got == "colossus-form"
	}
	assert.True(t, sawColossus, "control: Titan Growth 4 and Dense Muscles 1 can seed Colossus Form")
}
