package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Owner ruling, 2026-11-01: an empty hand is not a wielded weapon. The free-hand
// punch stays, but a weapon plus an empty hand takes no dual-wield penalty, for
// players and mobs alike. Two real weapons still take it.

// emptyHandRanks are the ranks the ruling is held at. The penalty reads the
// Character alone, so a mob at its spawned rank 1 and a player at rank 1 are one
// case; rank 0 is a character whose skills map has no weapon-combat at all.
var emptyHandRanks = []int{0, 1, 10, 25, 50, 200}

func heldWeapon(id int, sub items.ItemSubType) items.Item {
	return items.Item{ItemId: id, Spec: &items.ItemSpec{Type: items.Weapon, Subtype: sub, Hands: 1}}
}

// planPenalties builds the real round plan, the only path to a penalty, and
// returns the penalty each planned weapon pays.
func planPenalties(c *characters.Character) []int {
	plan := buildAttackPlan(c, characters.New())
	out := make([]int, 0, len(plan.weapons))
	for _, ws := range plan.weapons {
		out = append(out, ws.penalty)
	}
	return out
}

func withWeaponRank(c *characters.Character, rank int) *characters.Character {
	c.Skills = map[string]int{string(skills.WeaponCombat): rank}
	return c
}

func TestRuling_WeaponPlusEmptyHandPaysNoDualWieldPenalty(t *testing.T) {
	const largeId = 9701
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		largeId: {SpeciesId: largeId, Name: "golem", Size: species.Large},
	}))
	sword := heldWeapon(10, items.Slashing)

	cases := []struct {
		name string
		set  func(c *characters.Character)
		arms int
		want int // planned entries, so a case cannot pass by planning one swing
	}{
		{"sword in the main hand, empty off hand", func(c *characters.Character) {
			c.Equipment.Weapon = sword
		}, 0, 2},
		{"empty main hand, sword in the off hand", func(c *characters.Character) {
			c.Equipment.Offhand = sword
		}, 0, 2},
		{"a Large wearer's two-hander, empty off hand", func(c *characters.Character) {
			c.SpeciesId = largeId
			c.Equipment.Weapon = items.Item{ItemId: 30, Spec: &items.ItemSpec{
				Type: items.Weapon, Subtype: items.Slashing, Hands: 2}}
		}, 0, 2},
		{"sword and a fist-subtype item", func(c *characters.Character) {
			c.Equipment.Weapon = sword
			c.Equipment.Offhand = heldWeapon(11, items.Fist)
		}, 0, 2},
		{"sword and a claws item", func(c *characters.Character) {
			c.Equipment.Weapon = sword
			c.Equipment.Offhand = heldWeapon(11, items.Claws)
		}, 0, 2},
		{"sword, shield and an empty third arm", func(c *characters.Character) {
			c.Equipment.Weapon = sword
			c.Equipment.Offhand = makeShield(20)
		}, 1, 2},
		{"sword, disabled off hand and an empty third arm", func(c *characters.Character) {
			c.Equipment.Weapon = sword
			c.Equipment.Offhand = items.ItemDisabledSlot
		}, 1, 2},
		{"sword and five empty arms", func(c *characters.Character) {
			c.Equipment.Weapon = sword
		}, 4, 6},
		{"sword in a third arm, every other arm empty", func(c *characters.Character) {
			c.Equipment.ExtraArm1 = sword
		}, 4, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, rank := range emptyHandRanks {
				c := withWeaponRank(newCharWithArms(tc.arms), rank)
				tc.set(c)
				got := planPenalties(c)
				require.Len(t, got, tc.want, "the punch is still planned")
				assert.Equal(t, make([]int, tc.want), got,
					"rank %d: an empty hand is not a wielded weapon, so no swing pays", rank)
			}
		})
	}
}

// The penalty reaches the roll: at rank 1 and the shipped skill weight, a sword
// beside an empty hand scores what the same fighter scores with fists. It read
// 56 against 105 before the ruling.
func TestRuling_WeaponPlusEmptyHandScoresWithoutThePenalty(t *testing.T) {
	pinSkillWeightForTest(t, 5.0)
	c := withWeaponRank(characters.New(), 1)
	c.Stats.Dexterity.Base = 100
	c.Stats.Dexterity.Recalculate()
	c.Equipment.Weapon = heldWeapon(10, items.Slashing)
	c.Validate()
	c.Stamina = c.StaminaMax.Value
	target := characters.New()

	plan := buildAttackPlan(c, target)
	require.Len(t, plan.weapons, 2)
	got := calcAttackScore(c, target, plan.weapons[0].weapon, plan.weapons[0].penalty, combatContext{})
	want := float64(c.GetEffectiveDexterity()) + 5.0
	assert.InDelta(t, want, got, 1e-9, "the sword's score carries no dual-wield penalty")
}

// The other half of the ruling: the free hand still swings. The round resolves
// every planned swing, the fist's included, and the fist swings at
// unarmed-combat rather than at the sword's rank.
func TestRuling_WeaponPlusEmptyHandStillThrowsThePunch(t *testing.T) {
	atk, def := swingDefencePopulationCombatants()
	atk.Equipment.Weapon = heldWeapon(10, items.Slashing)

	plan := buildAttackPlan(atk, def)
	require.Len(t, plan.weapons, 2, "sword plus the free hand")
	fist := plan.weapons[1]
	assert.Equal(t, 0, fist.weapon.ItemId, "the second entry is the fist")
	assert.Equal(t, items.Unarmed, fist.weaponSubType)
	assert.True(t, fist.isOffhand)
	require.GreaterOrEqual(t, fist.swingCount, 1, "the fist swings")

	res := calculateCombat(atk, def, User, Mob, plan, combatContext{
		sourceSight: messaging.SightFull, targetSight: messaging.SightFull,
	})
	assert.Equal(t, plan.totalSwings, res.SwingsThrown, "every planned swing resolved")
	assert.Greater(t, res.SwingsThrown, plan.weapons[0].swingCount, "more swings than the sword alone")
}

// Guard: two real weapons stay exactly as they were, the formula's truncation
// and floor included. The numbers are written out rather than recomputed, so a
// retune of the Go defaults fails here. Empty hands beside two real weapons keep
// their old numbers too; see the note on calcDualWieldPenalty.
func TestGuard_TwoRealWeaponsStillPayTheDualWieldPenalty(t *testing.T) {
	const largeId = 9702
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		largeId: {SpeciesId: largeId, Name: "golem", Size: species.Large},
	}))
	base := map[int]int{0: 50, 1: 49, 10: 42, 25: 30, 49: 10, 50: 10, 200: 10}
	sword := heldWeapon(10, items.Slashing)
	dagger := heldWeapon(11, items.Stabbing)

	cases := []struct {
		name  string
		arms  int
		set   func(c *characters.Character)
		extra []int // added to the base, per planned entry
	}{
		{"sword and dagger", 0, func(c *characters.Character) {
			c.Equipment.Weapon, c.Equipment.Offhand = sword, dagger
		}, []int{0, 0}},
		{"a Large wearer's two-hander and a dagger", 0, func(c *characters.Character) {
			c.SpeciesId = largeId
			c.Equipment.Weapon = items.Item{ItemId: 30, Spec: &items.ItemSpec{
				Type: items.Weapon, Subtype: items.Slashing, Hands: 2}}
			c.Equipment.Offhand = dagger
		}, []int{0, 0}},
		{"three weapons", 1, func(c *characters.Character) {
			c.Equipment.Weapon, c.Equipment.Offhand, c.Equipment.ExtraArm1 = sword, dagger, sword
		}, []int{0, 0, 20}},
		{"four weapons", 2, func(c *characters.Character) {
			c.Equipment.Weapon, c.Equipment.Offhand = sword, dagger
			c.Equipment.ExtraArm1, c.Equipment.ExtraArm2 = sword, dagger
		}, []int{0, 0, 20, 40}},
		{"two weapons and an empty third arm", 1, func(c *characters.Character) {
			c.Equipment.Weapon, c.Equipment.Offhand = sword, dagger
		}, []int{0, 0, 20}},
		{"two weapons held in extra arms", 2, func(c *characters.Character) {
			c.Equipment.ExtraArm1, c.Equipment.ExtraArm2 = sword, dagger
		}, []int{0, 0, 20, 40}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for rank, b := range base {
				c := withWeaponRank(newCharWithArms(tc.arms), rank)
				tc.set(c)
				want := make([]int, len(tc.extra))
				for i, e := range tc.extra {
					want[i] = b + e
				}
				assert.Equal(t, want, planPenalties(c), "rank %d", rank)
			}
		})
	}
}

// Guard: natural weapons on their own never paid and still do not.
func TestGuard_NaturalWeaponsStillPayNoDualWieldPenalty(t *testing.T) {
	cases := []struct {
		name string
		arms int
		set  func(c *characters.Character)
		want int
	}{
		{"two empty hands", 0, func(c *characters.Character) {}, 2},
		{"two claws", 0, func(c *characters.Character) {
			c.Equipment.Weapon = heldWeapon(10, items.Claws)
			c.Equipment.Offhand = heldWeapon(11, items.Claws)
		}, 2},
		{"two fist items", 0, func(c *characters.Character) {
			c.Equipment.Weapon = heldWeapon(10, items.Fist)
			c.Equipment.Offhand = heldWeapon(11, items.Fist)
		}, 2},
		{"a claw and an empty hand", 0, func(c *characters.Character) {
			c.Equipment.Weapon = heldWeapon(10, items.Claws)
		}, 2},
		{"six empty hands", 4, func(c *characters.Character) {}, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, rank := range emptyHandRanks {
				c := withWeaponRank(newCharWithArms(tc.arms), rank)
				tc.set(c)
				assert.Equal(t, make([]int, tc.want), planPenalties(c), "rank %d", rank)
			}
		})
	}
}

// Guard: a shield is not a weapon and makes no fist, and a two-hander fills the
// partner hand. Each plans one swing at no penalty, before and after the ruling.
func TestGuard_ShieldAndTwoHanderUnchanged(t *testing.T) {
	const mediumId = 9703
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		mediumId: {SpeciesId: mediumId, Name: "human", Size: species.Medium},
	}))
	cases := []struct {
		name string
		set  func(c *characters.Character)
	}{
		{"sword and shield", func(c *characters.Character) {
			c.Equipment.Weapon = heldWeapon(10, items.Slashing)
			c.Equipment.Offhand = makeShield(20)
		}},
		{"shield and an empty main hand", func(c *characters.Character) {
			c.Equipment.Offhand = makeShield(20)
		}},
		{"a two-hander", func(c *characters.Character) {
			c.SpeciesId = mediumId
			c.Equipment.Weapon = items.Item{ItemId: 30, Spec: &items.ItemSpec{
				Type: items.Weapon, Subtype: items.Slashing, Hands: 2}}
		}},
		{"a two-hander over a blocked off-hand weapon", func(c *characters.Character) {
			c.SpeciesId = mediumId
			c.Equipment.Weapon = items.Item{ItemId: 30, Spec: &items.ItemSpec{
				Type: items.Weapon, Subtype: items.Slashing, Hands: 2}}
			c.Equipment.Offhand = heldWeapon(11, items.Stabbing)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, rank := range emptyHandRanks {
				c := withWeaponRank(newCharWithArms(0), rank)
				tc.set(c)
				assert.Equal(t, []int{0}, planPenalties(c), "rank %d", rank)
			}
		})
	}
}
