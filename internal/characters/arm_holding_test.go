package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// #686: the equip line names the hand an automatic placement chose, found by
// comparing the hands before and after.
func TestArmFilled_FindsTheArmThatChanged(t *testing.T) {
	c := New()
	c.ExtraArms = 2
	c.Equipment.Weapon = items.Item{ItemId: 10001, UUID: items.NewItemUUID()}
	before := c.HandItems()
	assert.Len(t, before, 4, "two pairs of arms")

	c.Equipment.ExtraArm2 = items.Item{ItemId: 10001, UUID: items.NewItemUUID()} // same spec as the weapon
	assert.Equal(t, 4, ArmFilled(before, c.HandItems(), 10001), "the weapon hand did not change; arm 4 did")

	c.Equipment.Offhand = items.Item{ItemId: 10036, UUID: items.NewItemUUID()}
	assert.Equal(t, 2, ArmFilled(before, c.HandItems(), 10036))
	assert.Equal(t, 0, ArmFilled(before, before, 10001), "nothing changed")
}
