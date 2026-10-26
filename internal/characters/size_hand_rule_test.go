package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedGrowthMutations(t *testing.T) {
	t.Helper()
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7, MaxRank: 4,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1, 1, 1, 1},
			Pros: []mutations.MutationEffect{{Type: "size", Target: "large", Value: 3}}},
		"colossus-form": {MutationId: "colossus-form", Name: "Colossus Form", Rarity: 9, MaxRank: 1,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1},
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "colossal-grip", Value: 1}}},
		"extra-arms": {MutationId: "extra-arms", Name: "Extra Arms", Rarity: 8, MaxRank: 4,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1, 1, 1, 2}},
	}))
}

func grown(t *testing.T, extraArms, titan int, colossus bool) *Character {
	t.Helper()
	seedGoldenSpecies(t)
	seedGrowthMutations(t)
	c := goldenChar(extraArms, goldenMedium, true)
	c.Mutations["titan-growth"] = titan
	if colossus {
		c.Mutations["colossus-form"] = 1
	}
	return c
}

func TestSizeStepsToLargeAtTitanThree(t *testing.T) {
	assert.Equal(t, species.Medium, grown(t, 0, 2, false).Size())
	assert.Equal(t, species.Large, grown(t, 0, 3, false).Size())
	seedGoldenSpecies(t)
	assert.Equal(t, species.Large, goldenChar(0, goldenLarge, true).Size())
	assert.Equal(t, species.Small, goldenChar(0, goldenSmall, true).Size())
}

func TestGrownWithoutColossusGripsInTheWeaponHandOnly(t *testing.T) {
	c := grown(t, 2, 3, false)
	great := goldenItem(9201, goldenGreat, false)
	assert.Equal(t, 1, c.HandsRequiredAt(great, "wielded"))
	assert.Equal(t, 2, c.HandsRequiredAt(great, "extra arm 1"))
	assert.Equal(t, 2, c.HandsRequired(great), "no named hand: the grip every hand has")

	c.Equipment.Weapon = great
	c.Equipment.ExtraArm1 = goldenItem(9202, goldenGreat, false)
	assert.False(t, c.IsBlockedBy2H("offhand"))
	assert.True(t, c.IsBlockedBy2H("extra arm 2"))
}

func TestColossusGripsInEveryHand(t *testing.T) {
	c := grown(t, 2, 4, true)
	great := goldenItem(9211, goldenGreat, false)
	assert.Equal(t, 1, c.HandsRequiredAt(great, "extra arm 1"))
	c.Equipment.Weapon = great
	c.Equipment.ExtraArm1 = goldenItem(9212, goldenGreat, false)
	assert.False(t, c.IsBlockedBy2H("offhand"))
	assert.False(t, c.IsBlockedBy2H("extra arm 2"))
}

func TestColossusOnAMediumCharacterGivesNoGrip(t *testing.T) {
	// Authored sets may skip prerequisites (owner, 2026-10-26): Colossus Form
	// at Titan Growth 1 adds its stats only.
	c := grown(t, 0, 1, true)
	assert.Equal(t, 2, c.HandsRequiredAt(goldenItem(9221, goldenGreat, false), "wielded"))
}

func TestGrownAutoEquipPutsATwoHanderInTheWeaponHandAlone(t *testing.T) {
	c := grown(t, 0, 3, false)
	choice, refusal := c.ChooseWornSlot(goldenItem(9231, goldenGreat, false), 0)
	require.Empty(t, refusal)
	require.Len(t, choice.Slots, 1)
	assert.Same(t, &c.Equipment.Weapon, choice.Slots[0].Item)
}

func TestGrownArmEquipRules(t *testing.T) {
	c := grown(t, 0, 3, false)
	great := goldenItem(9241, goldenGreat, false)

	choice, refusal := c.ChooseWornSlot(great, 1)
	require.Empty(t, refusal)
	assert.Len(t, choice.Slots, 1, "arm 1 takes the two-hander alone")

	_, refusal = c.ChooseWornSlot(great, 2)
	assert.Contains(t, refusal, "needs a pair of arms", "the offhand never grips a two-hander alone")
}
