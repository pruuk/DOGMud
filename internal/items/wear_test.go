package items

import "testing"

// Weapons and armour wear to their durability and break once; worn and
// broken gear works worse; repair puts it right.
func TestGearWearAndCondition(t *testing.T) {
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		1: {ItemId: 1, Name: "Sword", Type: Weapon, Subtype: Slashing, DamageMultiplier: 1.0},
		2: {ItemId: 2, Name: "Helm", Type: Head, PhysicalMitigation: 20},
		3: {ItemId: 3, Name: "Ring", Type: Ring, PhysicalMitigation: 2},
		4: {ItemId: 4, Name: "Club", Type: Weapon, Subtype: Bludgeoning, DamageMultiplier: 1.0, Durability: 5},
	}))
	sword := Item{ItemId: 1}
	d := sword.Durability()
	if d <= 0 {
		t.Fatalf("a weapon wears, durability %d", d)
	}
	if (&Item{ItemId: 1, Quality: QualityPristine}).Durability() != 2*d {
		t.Error("a pristine weapon lasts twice as long")
	}
	if (&Item{ItemId: 3}).Durability() != 0 {
		t.Error("jewelry does not wear")
	}
	if (&Item{ItemId: 4}).Durability() != 5 {
		t.Error("an authored durability wins")
	}

	broke := 0
	for i := 0; i < d+5; i++ {
		if sword.AddWear(1) {
			broke++
		}
	}
	if broke != 1 || !sword.IsBroken() || sword.Wear != d {
		t.Fatalf("breaks exactly once and stays at its durability: broke %d wear %d/%d", broke, sword.Wear, d)
	}
	if got := sword.GetSpec().DamageMultiplier; got >= 0.5 {
		t.Errorf("a broken sword hits badly, multiplier %v", got)
	}
	sword.Repair()
	if sword.IsBroken() || sword.GetSpec().DamageMultiplier != 1.0 {
		t.Error("a repaired sword works as new")
	}

	helm := Item{ItemId: 2}
	hd := helm.Durability()
	helm.Wear = hd * 9 / 10 // badly worn
	if got := helm.GetSpec().PhysicalMitigation; got >= 20 {
		t.Errorf("a badly worn helm protects less, got %d", got)
	}
	helm.Wear = hd / 10
	if got := helm.GetSpec().PhysicalMitigation; got != 20 {
		t.Errorf("a lightly worn helm works as new, got %d", got)
	}
}
