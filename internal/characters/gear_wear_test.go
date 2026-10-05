package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Critical hits wear swords and armour but never bows; bows wear on shots.
func TestCritWear(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		1: {ItemId: 1, Name: "Sword", Type: items.Weapon, Subtype: items.Slashing, DamageMultiplier: 1.0},
		2: {ItemId: 2, Name: "Bow", Type: items.Weapon, Subtype: items.Shooting, AmmoTag: `arrows`, DamageMultiplier: 1.0},
		3: {ItemId: 3, Name: "Helm", Type: items.Head, PhysicalMitigation: 5},
	}))
	c := &Character{}
	c.Equipment.Weapon = items.Item{ItemId: 1}
	c.Equipment.Head = items.Item{ItemId: 3}
	for i := 0; i < 200; i++ {
		c.CritWearStriker(c.WieldedWeaponPtr())
		c.CritWearArmor()
	}
	if c.Equipment.Weapon.Wear == 0 || c.Equipment.Head.Wear == 0 {
		t.Errorf("two hundred crits should wear the sword and helm: %d, %d", c.Equipment.Weapon.Wear, c.Equipment.Head.Wear)
	}

	archer := &Character{}
	archer.Equipment.Weapon = items.Item{ItemId: 2}
	for i := 0; i < 200; i++ {
		archer.CritWearStriker(archer.WieldedWeaponPtr())
	}
	if archer.Equipment.Weapon.Wear != 0 {
		t.Error("a bow never wears from critical hits")
	}
	for i := 0; i < 1000; i++ {
		archer.WearBowOnShot(&archer.Equipment.Weapon)
	}
	if archer.Equipment.Weapon.Wear == 0 {
		t.Error("a thousand shots should wear the bow")
	}
}

// Review fix: a critical kick (no striking item) wears nothing, a shield
// bash wears the shield, and only the given hand wears when dual wielding.
func TestCritWearStriker_RightItem(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		1: {ItemId: 1, Name: "Sword", Type: items.Weapon, Subtype: items.Slashing, DamageMultiplier: 1.0},
		4: {ItemId: 4, Name: "Dagger", Type: items.Weapon, Subtype: items.Stabbing, DamageMultiplier: 1.0},
		5: {ItemId: 5, Name: "Buckler", Type: items.Offhand, PhysicalMitigation: 4},
	}))
	c := &Character{}
	c.Equipment.Weapon = items.Item{ItemId: 1}
	for i := 0; i < 200; i++ {
		c.CritWearStriker(nil)
	}
	if c.Equipment.Weapon.Wear != 0 {
		t.Error("a critical kick must not wear the sword")
	}

	c.Equipment.Offhand = items.Item{ItemId: 5}
	for i := 0; i < 200; i++ {
		c.CritWearStriker(c.ShieldPtr())
	}
	if c.Equipment.Offhand.Wear == 0 || c.Equipment.Weapon.Wear != 0 {
		t.Errorf("a shield bash wears the shield only: shield %d, sword %d", c.Equipment.Offhand.Wear, c.Equipment.Weapon.Wear)
	}

	d := &Character{}
	d.Equipment.Weapon = items.Item{ItemId: 1}
	d.Equipment.Offhand = items.Item{ItemId: 4}
	for i := 0; i < 200; i++ {
		d.CritWearStriker(&d.Equipment.Offhand)
	}
	if d.Equipment.Offhand.Wear == 0 || d.Equipment.Weapon.Wear != 0 {
		t.Errorf("only the critting dagger wears: dagger %d, sword %d", d.Equipment.Offhand.Wear, d.Equipment.Weapon.Wear)
	}
}
