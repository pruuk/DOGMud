package characters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// #681: whether a two-hander blocks its pair partner is decided by the
// wearer's HandsRequired, the rule HandSlot.Is2H already uses. A Large wearer
// holds a two-hander in one hand, so the partner slot stays free.

func TestIsBlockedBy2H_LargeWearerTwoHanderLeavesOffhandFree(t *testing.T) {
	seedGoldenSpecies(t)
	c := goldenChar(4, goldenLarge, true)
	c.Equipment.Weapon = goldenItem(9101, goldenGreat, false)
	c.Equipment.Offhand = goldenItem(9102, goldenSword, false)
	c.Equipment.ExtraArm1 = goldenItem(9103, goldenGreat, false)
	c.Equipment.ExtraArm3 = goldenItem(9104, goldenGreat, false)

	for _, slot := range []string{"offhand", "extra arm 2", "extra arm 4"} {
		assert.False(t, c.IsBlockedBy2H(slot),
			"a Large wearer's two-hander needs one hand; %q must not be blocked", slot)
	}
}

func TestIsBlockedBy2H_MediumWearerTwoHanderBlocksPartner(t *testing.T) {
	seedGoldenSpecies(t)
	c := goldenChar(4, goldenMedium, true)
	c.Equipment.Weapon = goldenItem(9111, goldenGreat, false)
	c.Equipment.ExtraArm1 = goldenItem(9113, goldenGreat, false)
	c.Equipment.ExtraArm3 = goldenItem(9114, goldenGreat, false)

	for _, slot := range []string{"offhand", "extra arm 2", "extra arm 4"} {
		assert.True(t, c.IsBlockedBy2H(slot),
			"a Medium wearer's two-hander fills both hands; %q must be blocked", slot)
	}
}

func TestIsBlockedBy2H_SmallWearerOneHanderBlocksPartner(t *testing.T) {
	// A Small wearer needs two hands for a one-hander (HandsRequired), so the
	// partner is occupied, the same answer Is2H gives.
	seedGoldenSpecies(t)
	c := goldenChar(0, goldenSmall, true)
	c.Equipment.Weapon = goldenItem(9121, goldenSword, false)

	assert.True(t, c.IsBlockedBy2H("offhand"))
}

func TestIsBlockedBy2H_EmptyPartnerOrUnpairedSlotNeverBlocked(t *testing.T) {
	seedGoldenSpecies(t)
	c := goldenChar(4, goldenMedium, true)

	assert.False(t, c.IsBlockedBy2H("offhand"), "empty main hand blocks nothing")
	assert.False(t, c.IsBlockedBy2H("wielded"), "the main hand is never a partner slot")
	assert.False(t, c.IsBlockedBy2H("extra arm 1"))
	assert.False(t, c.IsBlockedBy2H("worn - head"))
}
