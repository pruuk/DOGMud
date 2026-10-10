package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// `equip X armN` goes through Wear (spec ruling 11): Wear's MinStrength,
// reservation and curse gates, EquipItem's full-pack floor rule, and the
// arm branch's own shape refusals and lines.

func armItem(id int, name string, t items.ItemType, hands int, cursed bool) items.Item {
	sub := items.Wearable
	if t == items.Weapon {
		sub = items.Slashing
	}
	return items.Item{ItemId: id, Spec: &items.ItemSpec{ItemId: id, Name: name, Type: t, Subtype: sub, Hands: hands, Cursed: cursed}}
}

func armUser(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.Equipment.Weapon = items.Item{}
	user.Character.Equipment.Offhand = items.Item{}
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

func armOut(t *testing.T, user *users.UserRecord, room *rooms.Room, rest string) string {
	t.Helper()
	handled, err := Equip(rest, user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
}

func TestEquipArm_TooWeakIsRefusedFirst(t *testing.T) {
	user, room := armUser(t)
	heavy := armItem(96001, "arbalest", items.Weapon, 1, false)
	heavy.Spec.MinStrength = 500
	require.True(t, user.Character.StoreItem(heavy))
	out := armOut(t, user, room, "arbalest arm3")
	assert.Contains(t, out, "You aren't strong enough to handle Arbalest.")
	_, inPack := user.Character.FindInBackpack("arbalest")
	assert.True(t, inPack)
}

func TestEquipArm_CursedItemInTheNamedArmRefuses(t *testing.T) {
	user, room := armUser(t)
	user.Character.Equipment.Offhand = armItem(96010, "hexed dagger", items.Weapon, 1, true)
	require.True(t, user.Character.StoreItem(armItem(96011, "knife", items.Weapon, 1, false)))
	out := armOut(t, user, room, "knife arm2")
	assert.Contains(t, out, "Your Hexed Dagger is cursed and prevents you from removing it.")
	assert.Equal(t, 96010, user.Character.Equipment.Offhand.ItemId)
	_, inPack := user.Character.FindInBackpack("knife")
	assert.True(t, inPack)
}

func TestEquipArm_CursedTwoHandedPartnerRefuses(t *testing.T) {
	user, room := armUser(t)
	user.Character.Equipment.Weapon = armItem(96020, "hexed maul", items.Weapon, 2, true)
	require.True(t, user.Character.StoreItem(armItem(96021, "knife", items.Weapon, 1, false)))
	out := armOut(t, user, room, "knife arm2")
	assert.Contains(t, out, "Your Hexed Maul is cursed and prevents you from removing it.")
	assert.Equal(t, 96020, user.Character.Equipment.Weapon.ItemId)
}

func TestEquipArm_CursedSecondSlotUnderATwoHanderRefuses(t *testing.T) {
	user, room := armUser(t)
	user.Character.Equipment.Offhand = armItem(96030, "hexed buckler", items.Offhand, 1, true)
	require.True(t, user.Character.StoreItem(armItem(96031, "maul", items.Weapon, 2, false)))
	out := armOut(t, user, room, "maul arm1")
	assert.Contains(t, out, "Your Hexed Buckler is cursed and prevents you from removing it.")
}

func TestEquipArm_ShapeRefusalsKeepTheirWording(t *testing.T) {
	user, room := armUser(t)
	require.True(t, user.Character.StoreItem(armItem(96040, "cap", items.Head, 0, false)))
	require.True(t, user.Character.StoreItem(armItem(96041, "buckler", items.Offhand, 1, false)))
	require.True(t, user.Character.StoreItem(armItem(96042, "knife", items.Weapon, 1, false)))
	require.True(t, user.Character.StoreItem(armItem(96043, "maul", items.Weapon, 2, false)))
	assert.Contains(t, armOut(t, user, room, "cap arm1"), "You can only wield weapons or shields in arm slots.")
	assert.Contains(t, armOut(t, user, room, "buckler arm1"), "You can't put a shield in your primary weapon hand (arm 1).")
	assert.Contains(t, armOut(t, user, room, "knife arm3"), "You don't have arm 3.")
	assert.Contains(t, armOut(t, user, room, "maul arm2"), "A two-handed weapon needs a pair of arms. Try arm 1, 3, or 5.")
}

func TestEquipArm_SuccessLines(t *testing.T) {
	user, room := armUser(t)
	require.True(t, user.Character.StoreItem(armItem(96050, "knife", items.Weapon, 1, false)))
	require.True(t, user.Character.StoreItem(armItem(96051, "buckler", items.Offhand, 1, false)))
	out1 := armOut(t, user, room, "knife arm1")
	assert.Contains(t, out1, "You wield your")
	assert.Contains(t, out1, "in your weapon hand.")
	assert.Equal(t, 96050, user.Character.Equipment.Weapon.ItemId)
	out := armOut(t, user, room, "buckler arm2")
	assert.Contains(t, out, "You equip your")
	assert.Contains(t, out, "in your offhand.")
	assert.Equal(t, 96051, user.Character.Equipment.Offhand.ItemId)
}

// #270: an extra arm reads "arm 3" in the equip line, as the equipment list
// numbers it, not the internal slot key "extra arm 1".
func TestEquipArm_ExtraArmLineUsesTheArmNumber(t *testing.T) {
	user, room := armUser(t)
	origMutations, origExtraArms := user.Character.Mutations, user.Character.ExtraArms
	origExtraArm1 := user.Character.Equipment.ExtraArm1
	t.Cleanup(func() {
		user.Character.Mutations = origMutations
		user.Character.ExtraArms = origExtraArms
		user.Character.Equipment.ExtraArm1 = origExtraArm1
	})
	// Validate re-derives ExtraArms from the mutation, so set that instead.
	// A fresh map is assigned (not written into), so origMutations is never aliased.
	user.Character.Mutations = map[string]int{"extra-arms": 2}
	user.Character.ExtraArms = 2
	require.True(t, user.Character.StoreItem(armItem(96060, "club", items.Weapon, 1, false)))
	out := armOut(t, user, room, "club arm3")
	assert.Contains(t, out, "in your arm 3.")
	assert.NotContains(t, out, "extra arm")
	assert.Equal(t, 96060, user.Character.Equipment.ExtraArm1.ItemId)
}

// An item knocked off a full pack lands on the floor instead of vanishing.
//
// Weight 100 (not the plan's 5000): equipped gear counts at half weight
// (characters/inventory.go GetCarriedWeight), so storing the knife first, at
// full Strength, must stay under capacity*2 for the require below to hold;
// dropping Strength to 1 afterward then makes the anvil's full weight (it is
// no longer equipped once displaced) exceed the shrunken capacity, so the
// restock genuinely fails rather than the item never having been storable at
// all.
func TestEquipArm_DisplacedItemOnAFullPackLandsOnTheFloor(t *testing.T) {
	user, room := armUser(t)
	anvil := armItem(96060, "anvil sword", items.Weapon, 1, false)
	anvil.Spec.Weight = 100
	user.Character.Equipment.Weapon = anvil
	require.True(t, user.Character.StoreItem(armItem(96061, "knife", items.Weapon, 1, false)))
	user.Character.Stats.Strength.ValueAdj = 1
	armOut(t, user, room, "knife arm1")
	require.Equal(t, 96061, user.Character.Equipment.Weapon.ItemId)
	_, onFloor := room.FindOnFloor("anvil sword", false)
	assert.True(t, onFloor, "the anvil sword must be on the floor, not lost")
}

// A reservation overage caused by the equip is reverted and refused, as the
// ordinary equip path already is: the buckler goes back to the pack, the
// offhand stays empty, and the refusal names reservation as the cause.
func TestEquipArm_ReservationOverageIsRefused(t *testing.T) {
	user, room := armUser(t)
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		996070: {ItemId: 996070, Name: "hungry collar", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.60},
		996071: {ItemId: 996071, Name: "buckler", Type: items.Offhand, Subtype: items.Wearable, ReserveStaminaPct: 0.30},
	})()
	user.Character.StaminaMax.Base = 100
	user.Character.Equipment.Neck = items.New(996070)
	require.NoError(t, user.Character.Validate())
	require.True(t, user.Character.StoreItem(items.New(996071)))

	out := armOut(t, user, room, "buckler arm2")
	assert.Contains(t, strings.ToLower(out), "reserve")
	assert.Equal(t, 0, user.Character.Equipment.Offhand.ItemId)
	_, inPack := user.Character.FindInBackpack("buckler")
	assert.True(t, inPack)
}
