package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// A disarmed weapon goes back into its owner's pack, never to the ground
// and never lost, even when the pack is past what StoreItem would accept
// (owner, 2026-10-06), and the disarm queues the EquipmentChange every other
// equipment change queues (#413), so item trees hear on_unequip and the GMCP
// equipment panel refreshes.
func TestCritDisarmReturnsTheWeaponToThePack(t *testing.T) {
	cases := []struct {
		name       string
		overloaded bool
	}{
		{"room in the pack", false},
		{"pack past twice its capacity", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_ = events.DrainQueuedEquipmentChangesForTest()
			source := characters.New()
			target := characters.New()
			target.MobInstanceId = 4242
			weapon := items.Item{ItemId: 999901, Spec: &items.ItemSpec{NameSimple: "rusty dagger", Weight: 2}}
			target.Equipment.Weapon = weapon
			if c.overloaded {
				anvil := items.Item{ItemId: 999902, Spec: &items.ItemSpec{NameSimple: "anvil", Weight: 100000}}
				target.Items = append(target.Items, anvil)
				if target.StoreItem(items.Item{ItemId: 999903, Spec: &items.ItemSpec{NameSimple: "pebble", Weight: 1}}) {
					t.Fatal("precondition: the overloaded pack must refuse a normal store")
				}
			}

			res := AttemptCritDisarm(source, target, 100.0) // 100% forces success
			if !res.Success {
				t.Fatal("precondition: the disarm must succeed")
			}
			if target.Equipment.Weapon.ItemId != 0 {
				t.Error("the weapon is still in the weapon slot")
			}
			found := false
			for _, it := range target.Items {
				if it.ItemId == weapon.ItemId {
					found = true
				}
			}
			if !found {
				t.Errorf("the disarmed weapon is not in the pack: %+v", target.Items)
			}

			changes := events.DrainQueuedEquipmentChangesForTest()
			if len(changes) != 1 || changes[0].MobInstanceId != 4242 ||
				len(changes[0].ItemsRemoved) != 1 || changes[0].ItemsRemoved[0].ItemId != weapon.ItemId {
				t.Errorf("want one EquipmentChange removing the weapon from mob 4242, got %+v", changes)
			}
		})
	}
}
