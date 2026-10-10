package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	familyTestWardA  = 9321 // ward family
	familyTestWardB  = 9322 // ward family
	familyTestHeal   = 9323 // heal family
	familyTestPotion = 9324 // no family, a potion-like statmod record
)

func seedFamilyTestSpecs(t *testing.T) {
	t.Helper()
	restore := SeedConditionsForTest(map[int]*ConditionSpec{
		familyTestWardA:  {ConditionId: familyTestWardA, Name: "Ward A", Family: FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
		familyTestWardB:  {ConditionId: familyTestWardB, Name: "Ward B", Family: FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
		familyTestHeal:   {ConditionId: familyTestHeal, Name: "Heal", Family: FamilyHeal, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
		familyTestPotion: {ConditionId: familyTestPotion, Name: "Potion", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
	})
	t.Cleanup(restore)
}

// R4 and R6: landing a ward removes every other ward first, on every add
// door, and the newest stands alone.
func TestFamily_NewestReplacesTheOlderOnEveryDoor(t *testing.T) {
	seedFamilyTestSpecs(t)

	doors := map[string]func(bs *Conditions, id int) bool{
		"AddCondition":          func(bs *Conditions, id int) bool { return bs.AddCondition(id, false) },
		"AddConditionScaled":    func(bs *Conditions, id int) bool { return bs.AddConditionScaled(id, 2.0) },
		"AddConditionMagnitude": func(bs *Conditions, id int) bool { return bs.AddConditionMagnitude(id, 4, 10) },
	}
	for name, add := range doors {
		t.Run(name, func(t *testing.T) {
			bs := New()
			require.True(t, add(&bs, familyTestWardA))
			require.True(t, add(&bs, familyTestPotion))
			require.True(t, add(&bs, familyTestWardB))

			assert.False(t, bs.HasCondition(familyTestWardA), "the older ward is gone")
			assert.True(t, bs.HasCondition(familyTestWardB), "the newest ward stands")
			assert.True(t, bs.HasCondition(familyTestPotion), "a record with no family is untouched")
			assert.Len(t, bs.List, 2)
		})
	}
}

// Re-landing the same id is a refresh, not a replacement: the record stays
// and nothing else is removed.
func TestFamily_SameIdIsARefresh(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	require.True(t, bs.AddConditionMagnitude(familyTestWardA, 4, 10))
	require.True(t, bs.AddConditionMagnitude(familyTestWardA, 6, 12))

	require.Len(t, bs.List, 1)
	assert.Equal(t, 6, bs.List[0].TriggersLeft)
	assert.Equal(t, 12.0, bs.List[0].Magnitude)
}

// Families are separate: a heal does not replace a ward.
func TestFamily_DifferentFamiliesCoexist(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	require.True(t, bs.AddCondition(familyTestWardA, false))
	require.True(t, bs.AddCondition(familyTestHeal, false))
	assert.True(t, bs.HasCondition(familyTestWardA))
	assert.True(t, bs.HasCondition(familyTestHeal))
}

// FamilyRivals names the live records an add of conditionId would replace,
// for the replacement line; an expired, unpruned rival is not news.
func TestFamilyRivals_ListsLiveRivalsOnly(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	assert.Empty(t, bs.FamilyRivals(familyTestWardB), "nothing held")

	require.True(t, bs.AddCondition(familyTestWardA, false))
	require.True(t, bs.AddCondition(familyTestPotion, false))
	rivals := bs.FamilyRivals(familyTestWardB)
	require.Len(t, rivals, 1)
	assert.Equal(t, familyTestWardA, rivals[0].ConditionId)

	assert.Empty(t, bs.FamilyRivals(familyTestWardA), "the same id is a refresh, never its own rival")
	assert.Empty(t, bs.FamilyRivals(familyTestPotion), "a record with no family has no rivals")

	bs.RemoveCondition(familyTestWardA)
	assert.Empty(t, bs.FamilyRivals(familyTestWardB), "an expired rival is left to the prune pass")
}

// An expired, unpruned rival is not discarded by a replacement: it stays in
// the list so the prune pass returns it and its own end line still fires.
func TestFamily_ExpiredRivalIsLeftForPrune(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	require.True(t, bs.AddCondition(familyTestWardA, false))
	bs.RemoveCondition(familyTestWardA) // expired this round, not yet pruned

	require.True(t, bs.AddCondition(familyTestWardB, false))

	pruned := bs.Prune()
	require.Len(t, pruned, 1, "the expired ward reaches the prune pass")
	assert.Equal(t, familyTestWardA, pruned[0].ConditionId)
	assert.True(t, bs.HasCondition(familyTestWardB), "the newest ward stands")
}

// A refused add keeps the rival: the poison check runs before the discard.
func TestFamily_RefusedAddKeepsTheRival(t *testing.T) {
	const (
		poisonWard = 9327
		stoneId    = 9328
	)
	restore := SeedConditionsForTest(map[int]*ConditionSpec{
		familyTestWardA: {ConditionId: familyTestWardA, Name: "Ward A", Family: FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
		poisonWard:      {ConditionId: poisonWard, Name: "Poison Ward", Family: FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5, Flags: []Flag{Poison}},
		stoneId:         {ConditionId: stoneId, Name: "Stone", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5, Flags: []Flag{PoisonImmunity}},
	})
	t.Cleanup(restore)

	bs := New()
	require.True(t, bs.AddCondition(familyTestWardA, false))
	require.True(t, bs.AddCondition(stoneId, false))

	assert.False(t, bs.AddCondition(poisonWard, false), "the immune holder refuses the poison ward")
	assert.True(t, bs.HasCondition(familyTestWardA), "the refused add leaves the rival held")
	assert.False(t, bs.HasCondition(poisonWard))
}

// A rival held after an unrelated record: the discard reindexes, so the
// unrelated record still resolves.
func TestFamily_DiscardKeepsTheIndexForOtherRecords(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	require.True(t, bs.AddCondition(familyTestPotion, false))
	require.True(t, bs.AddCondition(familyTestWardA, false))

	require.True(t, bs.AddCondition(familyTestWardB, false))

	assert.False(t, bs.HasCondition(familyTestWardA), "the rival is gone")
	assert.True(t, bs.HasCondition(familyTestPotion), "the unrelated record still resolves")
	require.Len(t, bs.GetConditions(familyTestPotion), 1)
	require.Len(t, bs.GetConditions(familyTestWardB), 1)
}

// HasFamily is the mob AI's "already shielded" and "already healing" test.
func TestHasFamily(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	assert.False(t, bs.HasFamily(FamilyWard))
	require.True(t, bs.AddCondition(familyTestWardA, false))
	assert.True(t, bs.HasFamily(FamilyWard))
	assert.False(t, bs.HasFamily(FamilyHeal))
	bs.RemoveCondition(familyTestWardA)
	assert.False(t, bs.HasFamily(FamilyWard), "an expired record is not held")
}

// A family name the engine does not know is a typo that would make the
// record stack silently; the load refuses it.
func TestValidate_RefusesAnUnknownFamily(t *testing.T) {
	spec := &ConditionSpec{ConditionId: 9325, Name: "Typo", Family: "wards"}
	err := spec.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"wards"`)

	for _, f := range AllFamilies {
		ok := &ConditionSpec{ConditionId: 9326, Name: "Fine", Family: f}
		assert.NoError(t, ok.Validate(), f)
	}
}
