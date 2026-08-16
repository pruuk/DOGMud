package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/assert"
)

// TestCompanionEconomy_Calibration encodes the spec §5 rows end-to-end
// (reserve -> budget), so a knob change that breaks the intended progression
// fails loudly rather than silently drifting the game balance.
//
// U7b rebased both halves of this. The per-type base cost is no longer an
// authored figure (350 / 440 / 735); it is CompanionReserveDefault scaled by the
// pet's own multiplier, so the rows below name pet multipliers instead. And the
// budget is no longer "must fit inside ConvictionMax" (which is what the deleted
// CanAffordCompanion checked, CompanionCastingFloorPct being 0); it is the
// PoolReservationCapPct ceiling, two thirds of the pool.
func TestCompanionEconomy_Calibration(t *testing.T) {
	seedMut := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"broodmaster": {
			MutationId: "broodmaster", Name: "Broodmaster", Rarity: 5, Pole: "belief",
			Pros: []mutations.MutationEffect{{Type: "companion_reserve_reduction"}},
		},
	})
	defer seedMut()

	// fieldN counts how many companions of the given PET MULTIPLIER this
	// character can field at the given ConvictionMax before the reservation
	// ceiling (or the soft count backstop) refuses the next one. This is the
	// same pair of gates every live summon path now applies at its call site.
	fieldN := func(c *Character, petMultiplier float64, convMax int) int {
		c.ConvictionMax.Value = convMax
		c.Companions = nil
		n := 0
		for {
			if len(c.Companions) >= c.GetMaxCompanions() {
				break
			}
			r := c.CalcCompanionReserve(CompanionReserveBase(petMultiplier))
			if c.WouldBreachReservationCap(PoolConviction, r) {
				break
			}
			n++
			c.AddCompanion(CompanionInfo{InstanceId: n, Name: "p", ConvictionReserve: r})
		}
		return n
	}

	// Newbie: manif 5, ConvMax 440, ceiling floor(440*0.66) = 290.
	// Reserve factor = (1 - 0.05) * 1.08 = 1.026.
	//   steppe spirit (0.75 -> 210): 215 each  -> 1 fits, 430 does not.
	//   magma         (1.25 -> 350): 359 each  -> the first one already breaches.
	// A novice summoner cannot hold the top conjure tier at all, which is the
	// pet-power ceiling doing exactly what D9 built it to do.
	nb := New()
	nb.Skills[string(skills.Manifestation)] = 5
	assert.Equal(t, 1, fieldN(nb, 0.75, 440), "newbie fields exactly 1 steppe spirit")
	assert.Equal(t, 0, fieldN(nb, 1.25, 440), "newbie cannot hold a magma elemental at all")

	// Meirok: manif 48, no mutation, ConvMax 547, ceiling floor(547*0.66) = 361.
	// Reserve factor = (1 - 0.48) * 0.816 = 0.42432.
	//   golem (1.00 -> 280): 119 each -> 357 fits, 476 does not.
	//   magma (1.25 -> 350): 149 each -> 298 fits, 447 does not.
	me := New()
	me.Skills[string(skills.Manifestation)] = 48
	assert.Equal(t, 3, fieldN(me, 1.00, 547), "Meirok fields 3 flesh golems")
	assert.Equal(t, 2, fieldN(me, 1.25, 547), "the dearer pet buys one fewer, which is the whole point")

	// Fully archetyped: manif 55 + rank-4 mutation, ConvMax 600, ceiling 396.
	// Reserve factor = 0.21 * 0.76 = 0.1596, so 45 per golem and 56 per magma.
	// Both sit far enough under the ceiling that the soft count backstop binds
	// first: past the reduction caps, pet choice stops costing companion slots.
	ar := New()
	ar.Skills[string(skills.Manifestation)] = 55
	ar.Mutations = map[string]int{"broodmaster": 4}
	assert.Equal(t, 5, fieldN(ar, 1.00, 600), "archetyped fields 5 golems (soft cap binds)")
	assert.Equal(t, 5, fieldN(ar, 1.25, 600), "archetyped fields 5 magma elementals (soft cap binds)")

	// Absolute unit: manif 65 + rank-4, ConvMax 850, ceiling 561.
	// Reserve factor = 0.21 * 0.68 = 0.1428, so 50 per magma elemental.
	un := New()
	un.Skills[string(skills.Manifestation)] = 65
	un.Mutations = map[string]int{"broodmaster": 4}
	assert.Equal(t, 5, fieldN(un, 1.25, 850), "absolute unit fields 5 magma elementals")
}
