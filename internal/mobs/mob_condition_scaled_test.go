package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
)

func TestMobAddConditionScaledQueuesTheMultiplier(t *testing.T) {
	m := &Mob{InstanceId: 88101}
	events.DrainQueuedConditionsForTest(0) // clear
	m.AddConditionScaled(55, 1.3, "drink")
	var got *events.Condition
	for _, e := range events.DrainQueuedMobConditionsForTest(88101) {
		got = &e
	}
	if got == nil || got.ConditionId != 55 || got.DurationMult != 1.3 || got.Source != "drink" {
		t.Fatalf("queued %+v, want condition 55 at DurationMult 1.3 from drink", got)
	}
}

func TestMobAddConditionScaledNormalisesANonPositiveMultiplier(t *testing.T) {
	m := &Mob{InstanceId: 88102}
	events.DrainQueuedConditionsForTest(0) // clear
	m.AddConditionScaled(55, 0, "drink")
	got := events.DrainQueuedMobConditionsForTest(88102)
	if len(got) != 1 || got[0].DurationMult != 1.0 {
		t.Fatalf("queued %+v, want one condition at DurationMult 1.0", got)
	}
}
