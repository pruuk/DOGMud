package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Owner, 2026-10-19: the Vitalis Bandolier's effects worked, but their timers
// cycled on the web client's conditions panel and the potions' end lines
// printed while the effect never fell off. Each attuned condition was added
// with the potion's own duration and re-added after it expired. An attuned
// condition is now a permanent record, held while its potion stays slotted
// and the belt stays worn, as a worn item's conditions are.

func attunedAmbient(t *testing.T, id int) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(id, "ambient", "Ambient", uint64(7000+id))
	c := u.Character
	c.Equipment.Belt = items.New(999954)
	c.PotionItems = append(c.PotionItems, items.New(999955)) // condition 54
	tickAmbientPotions(u, 100)                               // attuning
	c.SetMiscData("pinnacle_bandolier_attune_round", uint64(100))
	tickAmbientPotions(u, 150) // attuned
	return u
}

func heldPermanent(u *users.UserRecord, conditionId int) bool {
	for _, held := range u.Character.Conditions.GetConditions(conditionId) {
		if held.Permanent {
			return true
		}
	}
	return false
}

func TestAmbientBandolier_AttunedConditionNeverCountsDown(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer seedSmartAmbient()()

	u := attunedAmbient(t, 730)
	if !heldPermanent(u, 54) {
		t.Fatal("an attuned bandolier condition must be a permanent record, not a timed one that expires and is re-added")
	}
}

func TestAmbientBandolier_TimedRecordBecomesPermanent(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer seedSmartAmbient()()

	// A record applied before the fix, or by drinking the potion by hand.
	u := attunedAmbient(t, 731)
	c := u.Character
	c.RemoveCondition(54)
	_ = c.AddCondition(54, false)
	if heldPermanent(u, 54) {
		t.Fatal("fixture: the record should start timed")
	}
	tickAmbientPotions(u, 151)
	if !heldPermanent(u, 54) {
		t.Fatal("the tick must turn a held timed record permanent")
	}
}

func TestAmbientBandolier_ValidateKeepsItWhileWornAndDropsItWhenRemoved(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer seedSmartAmbient()()

	u := attunedAmbient(t, 732)
	c := u.Character
	_ = c.Validate(true) // an equip or login rebuilds permanent records
	if !heldPermanent(u, 54) {
		t.Fatal("the permanent-condition rebuild stripped the worn bandolier's attuned condition")
	}

	c.Equipment.Belt = items.Item{}
	_ = c.Validate(true)
	// HasCondition also counts a removed record the prune pass has not yet
	// swept, so ask for live records.
	if len(c.Conditions.GetConditions(54)) != 0 {
		t.Fatal("with the bandolier off, the rebuild must drop its condition")
	}
}
