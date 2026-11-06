package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #794: the Vitalis Bandolier held every slotted item's conditions on its
// wearer. Ysolde's Purge is a potion, so it slots, and its carrier sat in
// Bloom Detox for as long as the cup stayed in the bandolier. Throwables slot
// too, so a carried toxic flask poisoned its own carrier every round. The
// bandolier holds only what a potion drunk for its own effect gives.

const (
	harmfulVitalisId    = 999954
	harmfulVigorId      = 999955
	harmfulPurgeId      = 40109 // Ysolde's Purge, a detox (items.IsDetox)
	harmfulDraughtId    = 30052 // the Purging Draught, the other detox
	harmfulToxicFlaskId = 999957
)

func seedHarmfulAmbient() func() {
	restoreI := items.SeedItemsForTest(map[int]*items.ItemSpec{
		harmfulVitalisId: {ItemId: harmfulVitalisId, Name: "Vitalis Bandolier", Type: items.Belt,
			IsBandolier: true, BandolierCapacity: 4, AmbientPotions: true},
		harmfulVigorId: {ItemId: harmfulVigorId, Name: "vigor potion", Type: items.Potion,
			Subtype: items.Drinkable, ConditionIds: []int{54}},
		harmfulPurgeId: {ItemId: harmfulPurgeId, Name: "Ysolde's Purge", Type: items.Potion,
			Subtype: items.Drinkable, ConditionIds: []int{93}},
		harmfulDraughtId: {ItemId: harmfulDraughtId, Name: "Purging Draught", Type: items.Potion,
			Subtype: items.Drinkable, ConditionIds: []int{70}},
		harmfulToxicFlaskId: {ItemId: harmfulToxicFlaskId, Name: "toxic flask", Type: items.Object,
			Subtype: items.Throwable, ConditionIds: []int{78}},
	})
	restoreC := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		54: {ConditionId: 54, Name: "Vigor", Description: "vigor", RoundInterval: 5, TriggerCount: 100},
		70: {ConditionId: 70, Name: "Purging Draught", Description: "purge", RoundInterval: 1, TriggerCount: 1},
		78: {ConditionId: 78, Name: "Toxic Cloud", Description: "toxic", RoundInterval: 1, TriggerCount: 5},
		93: {ConditionId: 93, Name: "Bloom Detox", Description: "detox", RoundInterval: 1, TriggerCount: 50,
			StatMods: map[string]int{"strength": -12}},
	})
	return func() { restoreI(); restoreC() }
}

func liveCondition(u *users.UserRecord, conditionId int) bool {
	return len(u.Character.Conditions.GetConditions(conditionId)) > 0
}

func TestAmbientBandolier_HoldsNoDetoxOrThrowableCondition(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer seedHarmfulAmbient()()

	u := users.NewTestUser(740, "harmful", "Harmful", 7740)
	c := u.Character
	c.Equipment.Belt = items.New(harmfulVitalisId)
	c.PotionItems = append(c.PotionItems,
		items.New(harmfulVigorId),
		items.New(harmfulPurgeId),
		items.New(harmfulDraughtId),
		items.New(harmfulToxicFlaskId),
	)

	tickAmbientPotions(u, 100) // attuning
	c.SetMiscData("pinnacle_bandolier_attune_round", uint64(100))
	tickAmbientPotions(u, 150) // attuned

	if !liveCondition(u, 54) {
		t.Fatal("fixture: the drinkable potion's condition should be held once attuned")
	}
	for _, tc := range []struct {
		name        string
		conditionId int
	}{
		{"Ysolde's Purge (Bloom Detox)", 93},
		{"Purging Draught", 70},
		{"toxic flask (Toxic Cloud)", 78},
	} {
		if liveCondition(u, tc.conditionId) {
			t.Errorf("a carried %s must not put its condition %d on the bandolier's wearer", tc.name, tc.conditionId)
		}
	}
}
