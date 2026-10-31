package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #686: a plain equip that lands in the offhand names it; one that lands in
// the weapon hand stays silent about the hand.
func TestEquipItem_NamesTheHandWhenNotTheWeaponHand(t *testing.T) {
	char := newTestChar()
	room := newTestRoom()
	actor := newStubActor(char, room)

	char.Equipment.Weapon = newWeaponItemNamed(41, "practice sword")
	char.Items = append(char.Items, newWeaponItemNamed(42, "second sword"))

	result := EquipItem(actor, "second sword")
	require.True(t, result.Equipped)
	require.Equal(t, 42, char.Equipment.Offhand.ItemId, "fixture: the second sword goes to the offhand (weapon %d)", char.Equipment.Weapon.ItemId)
	assert.Equal(t, "offhand", result.ArmLabel)
}

func TestEquipItem_WeaponHandNeedsNoLabel(t *testing.T) {
	char := newTestChar()
	room := newTestRoom()
	actor := newStubActor(char, room)

	char.Items = append(char.Items, newWeaponItemNamed(43, "only sword"))

	result := EquipItem(actor, "only sword")
	require.True(t, result.Equipped)
	assert.Equal(t, "", result.ArmLabel)
}
