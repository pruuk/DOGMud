package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// #270: the same extra arm read "extra arm 1" on equip, "Arm 3" in the
// equipment list and "extra arm" on look. Owner call 2026-10-10: arms read
// arm 3 to arm 6, wrists read wrist 1 to wrist 6.
func TestArmDisplayName_MatchesTheEquipmentList(t *testing.T) {
	c := New()
	c.ExtraArms = 4

	assert.Equal(t, "weapon hand", c.ArmDisplayName(1))
	assert.Equal(t, "offhand", c.ArmDisplayName(2))
	for arm, want := range map[int]string{3: "arm 3", 4: "arm 4", 5: "arm 5", 6: "arm 6"} {
		assert.Equal(t, want, c.ArmDisplayName(arm))
	}
	assert.Equal(t, "", c.ArmDisplayName(7))

	c.ExtraArms = 0
	assert.Equal(t, "", c.ArmDisplayName(3), "no such arm")
}

func TestAllSlots_WristsAreNumbered(t *testing.T) {
	var w Worn
	labels := map[string]string{}
	for _, s := range w.AllSlots() {
		labels[s.Key] = s.Label
	}
	assert.Equal(t, "Wrist 1", labels["wrist1"])
	assert.Equal(t, "Wrist 2", labels["wrist2"])
	assert.Equal(t, "Wrist 3", labels["extrawrist1"])
	assert.Equal(t, "Arm 3", labels["extraarm1"])
}

func TestFindItem_ArmAndWristSourcesUseSlotNumbers(t *testing.T) {
	c := New()
	mk := func(id int, name string) items.Item {
		return items.Item{ItemId: id, Spec: &items.ItemSpec{ItemId: id, Name: name, Type: items.Object}}
	}
	c.Equipment.ExtraArm1 = mk(97001, "Probe Club")
	c.Equipment.Wrist2 = mk(97002, "Probe Bangle")
	c.Equipment.ExtraWrist1 = mk(97003, "Probe Cuff")

	_, src, ok := c.FindItem("probe club")
	assert.True(t, ok)
	assert.Equal(t, "arm 3", src)
	_, src, _ = c.FindItem("probe bangle")
	assert.Equal(t, "worn - wrist 2", src)
	_, src, _ = c.FindItem("probe cuff")
	assert.Equal(t, "worn - wrist 3", src)
}
