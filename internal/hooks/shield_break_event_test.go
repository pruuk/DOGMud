package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// A shield that breaks leaves the offhand slot without the EquipmentChange
// every other equipment change queues; it queues one now (#413's sibling),
// so the GMCP equipment panel refreshes and an item tree hears on_unequip.
func TestOffhandBreakQueuesAnEquipmentChange(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	_ = events.DrainQueuedEquipmentChangesForTest()

	shield := items.Item{ItemId: 999904, Spec: &items.ItemSpec{NameSimple: "brittle shield", Type: items.Offhand, BreakChance: 100}}
	def := &characters.Character{MobInstanceId: 4343}
	def.Equipment.Offhand = shield

	result := tryWeaponBreak(def, dummyAttackResult(true, false), rooms.LoadRoom(1))
	if !result.Broke {
		t.Fatal("precondition: a 100% break chance on a clean hit must break the shield")
	}
	changes := events.DrainQueuedEquipmentChangesForTest()
	if len(changes) != 1 || changes[0].MobInstanceId != 4343 ||
		len(changes[0].ItemsRemoved) != 1 || changes[0].ItemsRemoved[0].ItemId != shield.ItemId {
		t.Errorf("want one EquipmentChange removing the shield from mob 4343, got %+v", changes)
	}
}
