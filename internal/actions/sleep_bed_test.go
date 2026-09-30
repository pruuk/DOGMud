package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/statmods"
)

// A sleeper in a housing bed gets Sleeping in a Bed alongside Sleeping: its
// recovery statmods are what doubles the rest. Anything that wakes a sleeper
// (every waker cancels by the sleeping flag) ends it too.
func seedBedConditions(t *testing.T) func() {
	t.Helper()
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		15: {ConditionId: 15, Name: `Sleeping`, Flags: []conditions.Flag{conditions.Sleeping, conditions.SilentStart}, TriggerCount: 1000000},
		BedSleepConditionId: {ConditionId: BedSleepConditionId, Name: `Sleeping in a Bed`, TriggerCount: 1000000,
			Flags:    []conditions.Flag{conditions.Sleeping, conditions.SilentStart},
			StatMods: statmods.StatMods{`healthrecovery`: 2, `staminarecovery`: 2, `convictionrecovery`: 2}},
	})
}

func TestSleep_InABedAddsTheBedCondition(t *testing.T) {
	defer seedBedConditions(t)()
	actor := newSleepActor(t, false, true)
	actor.char.Name = `Sleeper`

	if res := Sleep(actor, SleepOptions{InBed: true}); !res.Success {
		t.Fatalf("sleep failed: %+v", res)
	}
	if !actor.char.HasCondition(BedSleepConditionId) {
		t.Fatal("a sleeper in a bed did not get Sleeping in a Bed")
	}
	if got := actor.char.StatMod(`healthrecovery`); got != 2 {
		t.Errorf("health recovery while in bed = %d, want 2", got)
	}

	// Waking (stand, shout, a thief...) cancels by the sleeping flag. The
	// records are pruned on the next round; the bonus stops at once.
	actor.char.CancelConditionsWithFlag(conditions.Sleeping)
	if actor.char.HasConditionFlag(conditions.Sleeping) || actor.char.StatMod(`healthrecovery`) != 0 {
		t.Error("waking left the sleeper with the bed's bonus")
	}
}

func TestSleep_OnTheFloorGetsNoBed(t *testing.T) {
	defer seedBedConditions(t)()
	actor := newSleepActor(t, false, true)
	Sleep(actor, SleepOptions{})
	if !actor.char.HasConditionFlag(conditions.Sleeping) {
		t.Fatal("did not sleep")
	}
	if actor.char.HasCondition(BedSleepConditionId) {
		t.Error("a sleeper on the floor got the bed's bonus")
	}
	if strings.Contains(strings.Join(actor.sent, ` `), `bed`) {
		t.Errorf("a sleeper on the floor was told about a bed: %q", actor.sent)
	}
}
