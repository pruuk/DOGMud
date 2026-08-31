package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/assert"
)

const (
	tSword  = 1
	tShield = 2
	tClaws  = 3
	tFist   = 4
)

func seedDefenceGearForTest(t *testing.T) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		tSword:  {ItemId: tSword, Name: "sword", Type: items.Weapon, Subtype: items.Slashing, ParryRating: 4},
		tShield: {ItemId: tShield, Name: "shield", Type: items.Offhand, Subtype: items.Wearable, BlockRating: 12},
		tClaws:  {ItemId: tClaws, Name: "claws", Type: items.Weapon, Subtype: items.Claws, ParryRating: 2},
		tFist:   {ItemId: tFist, Name: "knuckles", Type: items.Weapon, Subtype: items.Fist},
	}))
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "human", BodyParts: []string{"arms"}},
	}))
}

// hands builds a character with the given items in weapon / offhand / extraarm1.
// 0 means empty.
func hands(extraArms int, weapon, offhand, extra1 int) *characters.Character {
	c := characters.New()
	c.SpeciesId = 1
	c.ExtraArms = extraArms
	if weapon > 0 {
		c.Equipment.Weapon = items.New(weapon)
	}
	if offhand > 0 {
		c.Equipment.Offhand = items.New(offhand)
	}
	if extra1 > 0 {
		c.Equipment.ExtraArm1 = items.New(extra1)
	}
	return c
}

// ⚠️ PARITY GUARD. The gate became per-slot on 2026-08-30. Every two-handed
// shape the old main-hand ladder produced must come out identical, or this is a
// silent rebalance of every character in the game rather than a fix for the
// extra-arms mutation.
func TestEquipmentGatedMeleeDefences_TwoHandedParity(t *testing.T) {
	seedDefenceGearForTest(t)
	D, P, B := characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock

	for _, tc := range []struct {
		name            string
		weapon, offhand int
		want            []string
	}{
		{"empty hands", 0, 0, []string{D}},
		{"claws only", tClaws, 0, []string{D}},
		{"fists only", tFist, 0, []string{D}},
		{"sword only", tSword, 0, []string{D, P}},
		{"dual wield", tSword, tSword, []string{D, P, P}},
		{"sword + shield", tSword, tShield, []string{D, P, B}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := equipmentGatedMeleeDefences(hands(0, tc.weapon, tc.offhand, 0))
			assert.Equal(t, tc.want, got)
		})
	}
}

// The reported bug: a tower shield on the third arm behind claws.
func TestEquipmentGatedMeleeDefences_ExtraArmContributes(t *testing.T) {
	seedDefenceGearForTest(t)
	D, P, B := characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock

	t.Run("claws + claws + shield in 3rd hand (the live prod loadout)", func(t *testing.T) {
		got := equipmentGatedMeleeDefences(hands(1, tClaws, tClaws, tShield))
		assert.Equal(t, []string{D, B}, got,
			"a shield on the third arm must block; the old ladder returned dodge alone")
	})

	t.Run("two swords + shield in 3rd hand", func(t *testing.T) {
		got := equipmentGatedMeleeDefences(hands(1, tSword, tSword, tShield))
		assert.Equal(t, []string{D, P, P, B}, got,
			"IsDualWielding used to return before the shield check, hiding arm three")
	})

	t.Run("sword in the 3rd hand adds a parry", func(t *testing.T) {
		got := equipmentGatedMeleeDefences(hands(1, tSword, 0, tSword))
		assert.Equal(t, []string{D, P, P}, got,
			"the parry count was hardcoded at two and could never see arm three")
	})

	t.Run("claws in front, sword in the 3rd hand still parries", func(t *testing.T) {
		got := equipmentGatedMeleeDefences(hands(1, tClaws, 0, tSword))
		assert.Equal(t, []string{D, P}, got,
			"the main hand must not veto an armed extra arm")
	})

	// ⚠️ The slot is only real when the mutation has unlocked it.
	t.Run("locked slot contributes nothing", func(t *testing.T) {
		got := equipmentGatedMeleeDefences(hands(0, tSword, 0, tShield))
		assert.Equal(t, []string{D, P}, got,
			"ExtraArms=0 means the arm does not exist yet")
	})
}

// ⚠️ The one intended change beyond extra arms, pinned so it is a decision on
// the record rather than a side effect somebody trips over later.
func TestEquipmentGatedMeleeDefences_UnarmedWithShieldNowBlocks(t *testing.T) {
	seedDefenceGearForTest(t)
	D, B := characters.DefenseDodge, characters.DefenseBlock

	assert.Equal(t, []string{D, B}, equipmentGatedMeleeDefences(hands(0, tClaws, tShield, 0)),
		"claws + shield now blocks (ladder gave dodge alone)")
	assert.Equal(t, []string{D, B}, equipmentGatedMeleeDefences(hands(0, 0, tShield, 0)),
		"shield with no weapon now blocks (ladder gave dodge alone)")

	// The build skills.go solves WeaponCombat 1.34 against must NOT move.
	assert.Equal(t, []string{D}, equipmentGatedMeleeDefences(hands(0, 0, 0, 0)),
		"TWO EMPTY HANDS must stay dodge-only; the unarmed progression solve depends on it")
}

// NaturalBash is a species shield and must keep working without an item.
func TestEquipmentGatedMeleeDefences_NaturalBash(t *testing.T) {
	seedDefenceGearForTest(t)
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "elemental", BodyParts: []string{"arms"}, NaturalBash: true},
	}))
	D, P, B := characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock

	assert.Equal(t, []string{D, P, B}, equipmentGatedMeleeDefences(hands(0, tSword, 0, 0)),
		"an armed natural-bash species blocks with no shield item")
}
