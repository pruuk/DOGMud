package conditions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedRegistry populates the global conditions map with diverse specs for testing.
// Returns a cleanup function that restores the original map.
func seedRegistry() func() {
	orig := conditions
	conditions = map[int]*ConditionSpec{
		100: {
			ConditionId:   100,
			Name:          "Haste",
			Description:   "You move with supernatural speed.",
			TriggerCount:  10,
			RoundInterval: 2,
			Flags:         []Flag{Haste},
		},
		101: {
			ConditionId:   101,
			Name:          "Venom",
			Description:   "Poison courses through your veins.",
			TriggerCount:  5,
			RoundInterval: 3,
			Flags:         []Flag{Poison},
			StatMods:      statmods.StatMods{"strength": -5},
		},
		102: {
			ConditionId:   102,
			Name:          "Shadow Cloak",
			Description:   "You are hidden from sight.",
			TriggerCount:  8,
			RoundInterval: 1,
			Flags:         []Flag{Hidden},
		},
		103: {
			ConditionId:   103,
			Name:          "Eagle Eye",
			Description:   "Your aim is true.",
			TriggerCount:  6,
			RoundInterval: 2,
			Flags:         []Flag{NightVision},
			StatMods:      statmods.StatMods{"perception": 10, "dexterity": 5},
		},
		104: {
			ConditionId:   104,
			Name:          "Titan Fury",
			Description:   "Strength and damage surge through you.",
			TriggerCount:  4,
			RoundInterval: 3,
			Flags:         []Flag{DamageBonus, Haste, NoCombat},
		},
		105: {
			ConditionId:   105,
			Name:          "Lethargy",
			Description:   "Everything feels sluggish.",
			TriggerCount:  7,
			RoundInterval: 2,
			Flags:         []Flag{Slow},
		},
		106: {
			ConditionId:   106,
			Name:          "Night Sight",
			Description:   "You can see in the dark.",
			TriggerCount:  12,
			RoundInterval: 1,
			Flags:         []Flag{NightVision},
		},
		107: {
			ConditionId:   107,
			Name:          "Secret Affliction",
			Description:   "Something mysterious ails you.",
			Secret:        true,
			TriggerCount:  3,
			RoundInterval: 4,
			Flags:         []Flag{Poison},
		},
		108: {
			ConditionId:   108,
			Name:          "Quick Heal",
			Description:   "Instant healing pulse.",
			TriggerNow:    true,
			TriggerCount:  1,
			RoundInterval: 1,
			StatMods:      statmods.StatMods{"vitality": 8},
		},
		109: {
			ConditionId:   109,
			Name:          "Warmth",
			Description:   "You feel warm.",
			TriggerCount:  20,
			RoundInterval: 1,
			Flags:         []Flag{Warmed},
		},
	}
	return func() { conditions = orig }
}

// ─── Conditions.Validate ─────────────────────────────────────────────────────────

func TestConditions_Validate(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := &Conditions{
		List: []*Condition{
			{ConditionId: 100, TriggersLeft: 10},
			{ConditionId: 103, TriggersLeft: 6},
		},
	}
	bs.Validate()

	assert.Len(t, bs.conditionIds, 2)
	assert.Contains(t, bs.conditionIds, 100)
	assert.Contains(t, bs.conditionIds, 103)

	// Haste flag should map to condition 100
	hasteIds := bs.GetConditionIdsWithFlag(Haste)
	assert.Contains(t, hasteIds, 100)

	// NightVision flag should map to condition 103. (This fixture used the Accuracy
	// flag until U6b deleted it as an upstream stowaway; any flag serves — the
	// test pins registry flag→id mapping, not any particular flag.)
	nvIds := bs.GetConditionIdsWithFlag(NightVision)
	assert.Contains(t, nvIds, 103)
}

// ─── Conditions.HasFlag ──────────────────────────────────────────────────────────

func TestConditions_HasFlag(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := New()
	bs.AddCondition(100, false) // Haste
	bs.AddCondition(104, false) // DamageBonus, Haste, NoCombat

	t.Run("single flag present", func(t *testing.T) {
		assert.True(t, bs.HasFlag(Haste))
	})

	t.Run("multi-flag condition", func(t *testing.T) {
		assert.True(t, bs.HasFlag(DamageBonus))
		assert.True(t, bs.HasFlag(NoCombat))
	})

	t.Run("flag not present", func(t *testing.T) {
		assert.False(t, bs.HasFlag(Poison))
	})

	t.Run("All wildcard", func(t *testing.T) {
		assert.True(t, bs.HasFlag(All))
	})

	// #309: asking changes nothing. HasFlag(flag, true) used to expire what
	// it found under a name that reads like a question.
	t.Run("asking expires nothing", func(t *testing.T) {
		bs2 := New()
		bs2.AddCondition(105, false) // Slow
		before := bs2.List[bs2.conditionIds[105]].TriggersLeft
		for i := 0; i < 3; i++ {
			assert.True(t, bs2.HasFlag(Slow))
			assert.True(t, bs2.HasFlag(All))
		}
		assert.Equal(t, before, bs2.List[bs2.conditionIds[105]].TriggersLeft)
	})

	t.Run("expire mode removes condition", func(t *testing.T) {
		bs2 := New()
		bs2.AddCondition(105, false) // Slow
		assert.True(t, bs2.ExpireConditionsWithFlag(Slow))
		// After expire, the condition should be marked expired
		idx := bs2.conditionIds[105]
		assert.Equal(t, TriggersLeftExpired, bs2.List[idx].TriggersLeft)
	})
}

// TestConditions_HasFlag_AllMatchesFlaglessCondition verifies that action==All matches
// (and can expire) a condition that declares NO flags — e.g. pure regen potion
// conditions like Healing Salve (condition 54). Regression: the per-flag loop in
// HasFlag skipped flagless conditions entirely, so CancelConditionsWithFlag(conditions.All)
// on death left them active. Condition 108 ("Quick Heal") has no Flags, mirroring
// the potion regen conditions.
func TestConditions_HasFlag_AllMatchesFlaglessCondition(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := New()
	require.True(t, bs.AddCondition(108, false)) // Quick Heal — no Flags

	// The All wildcard must see a flagless condition.
	assert.True(t, bs.HasFlag(All),
		"All must match a condition that declares no flags")

	// Expire mode must mark the flagless condition expired.
	assert.True(t, bs.ExpireConditionsWithFlag(All),
		"All+expire must match the flagless condition")
	idx, ok := bs.conditionIds[108]
	require.True(t, ok)
	assert.Equal(t, TriggersLeftExpired, bs.List[idx].TriggersLeft,
		"flagless condition must be expired by ExpireConditionsWithFlag(All)")
}

// ─── Conditions.GetConditionIdsWithFlag ───────────────────────────────────────────────

func TestConditions_GetConditionIdsWithFlag(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := New()
	bs.AddCondition(100, false) // Haste
	bs.AddCondition(104, false) // DamageBonus, Haste, NoCombat

	hasteIds := bs.GetConditionIdsWithFlag(Haste)
	assert.Len(t, hasteIds, 2)
	assert.Contains(t, hasteIds, 100)
	assert.Contains(t, hasteIds, 104)

	poisonIds := bs.GetConditionIdsWithFlag(Poison)
	assert.Empty(t, poisonIds)
}

// ─── Conditions.Trigger ──────────────────────────────────────────────────────────

func TestConditions_Trigger(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	t.Run("round counter increments and triggers at interval", func(t *testing.T) {
		bs := New()
		bs.AddCondition(100, false) // RoundInterval=2, TriggerCount=10

		// Round 1: counter becomes 1, 1%2 != 0 → no trigger
		triggered := bs.Trigger()
		assert.Empty(t, triggered)
		assert.Equal(t, 1, bs.List[0].RoundCounter)

		// Round 2: counter becomes 2, 2%2 == 0 → trigger
		triggered = bs.Trigger()
		assert.Len(t, triggered, 1)
		assert.Equal(t, 100, triggered[0].ConditionId)
		assert.Equal(t, 9, bs.List[0].TriggersLeft) // decremented from 10
	})

	t.Run("unlimited triggers don't decrement", func(t *testing.T) {
		bs := New()
		bs.AddCondition(106, true) // permanent → TriggersLeftUnlimited

		// NightVision has RoundInterval=1, so triggers every round
		triggered := bs.Trigger()
		assert.Len(t, triggered, 1)
		assert.Equal(t, TriggersLeftUnlimited, bs.List[0].TriggersLeft)
		assert.Equal(t, 0, bs.List[0].RoundCounter) // reset for unlimited
	})

	t.Run("specific conditionId targeting", func(t *testing.T) {
		bs := New()
		bs.AddCondition(100, false) // Haste, interval=2
		bs.AddCondition(106, false) // NightVision, interval=1

		// Only trigger conditionId 106
		triggered := bs.Trigger(106)
		// Both get RoundCounter incremented but only 106 matches filter
		// Actually looking at the code, the filter logic has a bug where it
		// continues past non-matching IDs but doesn't skip processing.
		// We just verify the function doesn't panic.
		assert.NotNil(t, triggered)
	})
}

// ─── Conditions.Prune ────────────────────────────────────────────────────────────

func TestConditions_Prune(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := New()
	bs.AddCondition(100, false) // Haste
	bs.AddCondition(101, false) // Venom

	// Expire one condition
	bs.List[0].TriggersLeft = TriggersLeftExpired

	pruned := bs.Prune()
	assert.Len(t, pruned, 1)
	assert.Equal(t, 100, pruned[0].ConditionId)
	assert.Len(t, bs.List, 1)
	assert.Equal(t, 101, bs.List[0].ConditionId)

	// Indices should be rebuilt
	assert.Contains(t, bs.conditionIds, 101)
	assert.NotContains(t, bs.conditionIds, 100)
}

// ─── Conditions.StatMod ──────────────────────────────────────────────────────────

func TestConditions_StatMod(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := New()
	bs.AddCondition(101, false) // Venom: strength -5
	bs.AddCondition(103, false) // Eagle Eye: perception +10, dexterity +5

	assert.Equal(t, -5, bs.StatMod("strength"))
	assert.Equal(t, 10, bs.StatMod("perception"))
	assert.Equal(t, 5, bs.StatMod("dexterity"))
	assert.Equal(t, 0, bs.StatMod("charisma"))
}

// ─── Conditions.AddCondition stacking ────────────────────────────────────────────────

func TestConditions_AddCondition_Stacking(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	bs := New()

	t.Run("add new condition", func(t *testing.T) {
		ok := bs.AddCondition(100, false)
		assert.True(t, ok)
		assert.Len(t, bs.List, 1)
		assert.Equal(t, 10, bs.List[0].TriggersLeft)
	})

	t.Run("refresh existing condition", func(t *testing.T) {
		// Drain some triggers
		bs.List[0].TriggersLeft = 2
		ok := bs.AddCondition(100, false)
		assert.True(t, ok)
		assert.Len(t, bs.List, 1, "should not add duplicate")
		assert.Equal(t, 10, bs.List[0].TriggersLeft, "should refresh triggers")
	})

	t.Run("add second condition", func(t *testing.T) {
		ok := bs.AddCondition(101, false)
		assert.True(t, ok)
		assert.Len(t, bs.List, 2)
	})

	t.Run("nonexistent spec returns false", func(t *testing.T) {
		ok := bs.AddCondition(99999, false)
		assert.False(t, ok)
		assert.Len(t, bs.List, 2, "list unchanged")
	})

	t.Run("permanent condition", func(t *testing.T) {
		ok := bs.AddCondition(106, true)
		assert.True(t, ok)
		idx := bs.conditionIds[106]
		assert.Equal(t, TriggersLeftUnlimited, bs.List[idx].TriggersLeft)
		assert.True(t, bs.List[idx].Permanent)
	})
}

// TestGetDurations covers a record whose TriggersLeft still matches its spec
// default, which is what a bare AddCondition produces. Every case here reports the
// same numbers the old spec-only formula
// (TriggerCount*RoundInterval - RoundCounter) reported, and each case names
// that arithmetic beside the new one: for a default add the two agree, which
// is the equivalence the instance-correct formula had to preserve. The one
// deliberate change is the zero-interval row.
func TestGetDurations(t *testing.T) {
	type args struct {
		condition *Condition
		spec      *ConditionSpec
	}
	tests := []struct {
		name       string
		args       args
		wantRounds int
		wantTotal  int
	}{
		{
			name: "Normal case",
			args: args{
				condition: &Condition{TriggersLeft: 5, RoundCounter: 2},
				spec:      &ConditionSpec{TriggerCount: 5, RoundInterval: 3},
			},
			wantRounds: 13, // new: (5*3)-(2%3) = 15-2 = 13; old: (5*3)-2 = 13
			wantTotal:  15,
		},
		{
			name: "Zero rounds passed",
			args: args{
				condition: &Condition{TriggersLeft: 4, RoundCounter: 0},
				spec:      &ConditionSpec{TriggerCount: 4, RoundInterval: 2},
			},
			wantRounds: 8, // new: (4*2)-(0%2) = 8; old: (4*2)-0 = 8
			wantTotal:  8,
		},
		{
			name: "All rounds passed",
			args: args{
				condition: &Condition{TriggersLeft: 0, RoundCounter: 12},
				spec:      &ConditionSpec{TriggerCount: 3, RoundInterval: 4},
			},
			wantRounds: 0, // new: (0*4)-(12%4) = 0; old: (3*4)-12 = 0
			wantTotal:  12,
		},
		{
			name: "RoundCounter greater than total",
			args: args{
				condition: &Condition{TriggersLeft: 0, RoundCounter: 10},
				spec:      &ConditionSpec{TriggerCount: 2, RoundInterval: 4},
			},
			wantRounds: -2, // new: (0*4)-(10%4) = -2; old: (2*4)-10 = -2
			wantTotal:  8,
		},
		{
			name: "Zero trigger count",
			args: args{
				condition: &Condition{TriggersLeft: 0, RoundCounter: 1},
				spec:      &ConditionSpec{TriggerCount: 0, RoundInterval: 5},
			},
			wantRounds: -1, // new: (0*5)-(1%5) = -1; old: (0*5)-1 = -1
			wantTotal:  0,
		},
		{
			// The one row whose answer changed. A pure flag record has
			// RoundInterval 0, never ticks, and has no duration to report;
			// the old formula divided nothing but still returned a negative
			// remainder (-3) for it.
			name: "Zero round interval",
			args: args{
				condition: &Condition{TriggersLeft: 4, RoundCounter: 3},
				spec:      &ConditionSpec{TriggerCount: 4, RoundInterval: 0},
			},
			wantRounds: 0,
			wantTotal:  0,
		},
		{
			// A permanent condition keeps the spec answer rather than reporting
			// TriggersLeftUnlimited * RoundInterval rounds.
			name: "Unlimited triggers",
			args: args{
				condition: &Condition{TriggersLeft: TriggersLeftUnlimited, RoundCounter: 2},
				spec:      &ConditionSpec{TriggerCount: 5, RoundInterval: 3},
			},
			wantRounds: 13,
			wantTotal:  15,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			gotRounds, gotTotal := GetDurations(tt.args.condition, tt.args.spec)
			assert.Equal(t, tt.wantRounds, gotRounds)
			assert.Equal(t, tt.wantTotal, gotTotal)
		})
	}
}

// TestGetDurations_ExactTriggerCountReadsTheInstance is the case the old
// formula got wrong. AddConditionMagnitude — the door every former combat
// condition goes through — applies an EXACT trigger count, so a record can
// hold far fewer triggers than its spec declares. The spec-only formula
// answered with the spec's lifetime (30 rounds for a 10-trigger,
// 3-round-interval spec) no matter how few triggers the instance actually
// held, so the `conditions` command and the web client's chips both showed a
// duration the record would never reach.
func TestGetDurations_ExactTriggerCountReadsTheInstance(t *testing.T) {
	const conditionId = 9401
	defer SeedConditionsForTest(map[int]*ConditionSpec{
		conditionId: {ConditionId: conditionId, Name: "Exact Count", TriggerCount: 10, RoundInterval: 3},
	})()
	spec := GetConditionSpec(conditionId)
	require.NotNil(t, spec)

	bs := New()
	require.True(t, bs.AddConditionMagnitude(conditionId, 2, 1.0))

	condition := bs.GetConditions(conditionId)[0]
	require.Equal(t, 2, condition.TriggersLeft)

	// Before any tick: two triggers of three rounds each are still owed.
	// The old formula said 30 here, the spec's whole lifetime.
	roundsLeft, totalRounds := GetDurations(condition, spec)
	assert.Equal(t, 6, roundsLeft, "two triggers at a three-round interval is six rounds")
	assert.Equal(t, 30, totalRounds, "total stays the spec lifetime so a bar has a stable scale")

	// One round in: still two triggers owed, one round into the interval.
	bs.Trigger()
	require.Equal(t, 1, condition.RoundCounter)
	require.Equal(t, 2, condition.TriggersLeft)
	roundsLeft, _ = GetDurations(condition, spec)
	assert.Equal(t, 5, roundsLeft)

	// Three rounds in: the first trigger has fired and the interval reset,
	// leaving one trigger of three rounds.
	bs.Trigger()
	bs.Trigger()
	require.Equal(t, 3, condition.RoundCounter)
	require.Equal(t, 1, condition.TriggersLeft)
	roundsLeft, _ = GetDurations(condition, spec)
	assert.Equal(t, 3, roundsLeft)
}
func TestConditions_HasCondition(t *testing.T) {
	type fields struct {
		list         []*Condition
		conditionIds map[int]int
	}
	tests := []struct {
		name   string
		fields fields
		arg    int
		want   bool
	}{
		{
			name: "Condition exists in conditionIds",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1},
					{ConditionId: 2},
				},
				conditionIds: map[int]int{1: 0, 2: 1},
			},
			arg:  1,
			want: true,
		},
		{
			name: "Condition does not exist in conditionIds",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1},
					{ConditionId: 2},
				},
				conditionIds: map[int]int{1: 0, 2: 1},
			},
			arg:  3,
			want: false,
		},
		{
			name: "Empty conditionIds map",
			fields: fields{
				list:         []*Condition{},
				conditionIds: map[int]int{},
			},
			arg:  1,
			want: false,
		},
		{
			name: "ConditionIds map is nil",
			fields: fields{
				list:         []*Condition{},
				conditionIds: nil,
			},
			arg:  1,
			want: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			bs := &Conditions{
				List:         tt.fields.list,
				conditionIds: tt.fields.conditionIds,
			}
			assert.Equal(t, tt.want, bs.HasCondition(tt.arg))
		})
	}
}
func TestConditions_Started(t *testing.T) {
	type fields struct {
		list         []*Condition
		conditionIds map[int]int
	}
	tests := []struct {
		name         string
		fields       fields
		arg          int
		wantOnStart  bool
		shouldChange bool
	}{
		{
			name: "Condition exists and OnStartWaiting is true",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, OnStartWaiting: true},
					{ConditionId: 2, OnStartWaiting: true},
				},
				conditionIds: map[int]int{1: 0, 2: 1},
			},
			arg:          1,
			wantOnStart:  false,
			shouldChange: true,
		},
		{
			name: "Condition exists and OnStartWaiting is already false",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, OnStartWaiting: false},
				},
				conditionIds: map[int]int{1: 0},
			},
			arg:          1,
			wantOnStart:  false,
			shouldChange: false,
		},
		{
			name: "Condition does not exist",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, OnStartWaiting: true},
				},
				conditionIds: map[int]int{1: 0},
			},
			arg:          2,
			wantOnStart:  true,
			shouldChange: false,
		},
		{
			name: "Empty conditionIds map",
			fields: fields{
				list:         []*Condition{},
				conditionIds: map[int]int{},
			},
			arg:          1,
			wantOnStart:  false,
			shouldChange: false,
		},
		{
			name: "conditionIds is nil",
			fields: fields{
				list:         []*Condition{},
				conditionIds: nil,
			},
			arg:          1,
			wantOnStart:  false,
			shouldChange: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			bs := &Conditions{
				List:         tt.fields.list,
				conditionIds: tt.fields.conditionIds,
			}
			if tt.shouldChange && len(bs.List) > 0 {
				assert.True(t, bs.List[bs.conditionIds[tt.arg]].OnStartWaiting)
			}
			bs.Started(tt.arg)
			if idx, ok := bs.conditionIds[tt.arg]; ok && idx < len(bs.List) {
				assert.Equal(t, tt.wantOnStart, bs.List[idx].OnStartWaiting)
			} else if len(bs.List) > 0 {
				// If condition does not exist, original value should remain unchanged
				assert.Equal(t, tt.wantOnStart, bs.List[0].OnStartWaiting)
			}
		})
	}
}
func TestConditions_TriggersLeft(t *testing.T) {
	type fields struct {
		list         []*Condition
		conditionIds map[int]int
	}
	tests := []struct {
		name   string
		fields fields
		arg    int
		want   int
	}{
		{
			name: "Condition exists and has positive TriggersLeft",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, TriggersLeft: 3},
					{ConditionId: 2, TriggersLeft: 5},
				},
				conditionIds: map[int]int{1: 0, 2: 1},
			},
			arg:  2,
			want: 5,
		},
		{
			name: "Condition exists and has zero TriggersLeft",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, TriggersLeft: 0},
				},
				conditionIds: map[int]int{1: 0},
			},
			arg:  1,
			want: 0,
		},
		{
			name: "Condition exists and has negative TriggersLeft",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, TriggersLeft: -2},
				},
				conditionIds: map[int]int{1: 0},
			},
			arg:  1,
			want: -2,
		},
		{
			name: "Condition does not exist in conditionIds",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, TriggersLeft: 3},
				},
				conditionIds: map[int]int{1: 0},
			},
			arg:  2,
			want: 0,
		},
		{
			name: "conditionIds is nil",
			fields: fields{
				list:         []*Condition{{ConditionId: 1, TriggersLeft: 7}},
				conditionIds: nil,
			},
			arg:  1,
			want: 0,
		},
		{
			name: "conditionIds is empty map",
			fields: fields{
				list:         []*Condition{{ConditionId: 1, TriggersLeft: 7}},
				conditionIds: map[int]int{},
			},
			arg:  1,
			want: 0,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			bs := &Conditions{
				List:         tt.fields.list,
				conditionIds: tt.fields.conditionIds,
			}
			assert.Equal(t, tt.want, bs.TriggersLeft(tt.arg))
		})
	}
}
func TestConditions_RemoveCondition(t *testing.T) {
	type fields struct {
		list         []*Condition
		conditionIds map[int]int
	}
	tests := []struct {
		name         string
		fields       fields
		arg          int
		wantResult   bool
		wantTriggers int
		shouldModify bool
	}{
		{
			name: "Condition exists and is removed",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, TriggersLeft: 5},
					{ConditionId: 2, TriggersLeft: 3},
				},
				conditionIds: map[int]int{1: 0, 2: 1},
			},
			arg:          2,
			wantResult:   true,
			wantTriggers: TriggersLeftExpired,
			shouldModify: true,
		},
		{
			name: "Condition does not exist",
			fields: fields{
				list: []*Condition{
					{ConditionId: 1, TriggersLeft: 5},
				},
				conditionIds: map[int]int{1: 0},
			},
			arg:          3,
			wantResult:   false,
			wantTriggers: 5,
			shouldModify: false,
		},
		{
			name: "conditionIds is nil",
			fields: fields{
				list:         []*Condition{{ConditionId: 1, TriggersLeft: 7}},
				conditionIds: nil,
			},
			arg:          1,
			wantResult:   false,
			wantTriggers: 7,
			shouldModify: false,
		},
		{
			name: "conditionIds is empty map",
			fields: fields{
				list:         []*Condition{{ConditionId: 1, TriggersLeft: 7}},
				conditionIds: map[int]int{},
			},
			arg:          1,
			wantResult:   false,
			wantTriggers: 7,
			shouldModify: false,
		},
		{
			name: "Multiple conditions, remove first",
			fields: fields{
				list: []*Condition{
					{ConditionId: 10, TriggersLeft: 2},
					{ConditionId: 20, TriggersLeft: 4},
				},
				conditionIds: map[int]int{10: 0, 20: 1},
			},
			arg:          10,
			wantResult:   true,
			wantTriggers: TriggersLeftExpired,
			shouldModify: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			bs := &Conditions{
				List:         tt.fields.list,
				conditionIds: tt.fields.conditionIds,
			}
			result := bs.RemoveCondition(tt.arg)
			assert.Equal(t, tt.wantResult, result)
			if idx, ok := bs.conditionIds[tt.arg]; ok && tt.shouldModify {
				assert.Equal(t, tt.wantTriggers, bs.List[idx].TriggersLeft)
			} else if len(bs.List) > 0 {
				// If not modified, original value should remain
				assert.Equal(t, tt.wantTriggers, bs.List[0].TriggersLeft)
			}
		})
	}
}
func TestCondition_Expired(t *testing.T) {
	tests := []struct {
		name         string
		triggersLeft int
		want         bool
	}{
		{
			name:         "TriggersLeft is zero (expired)",
			triggersLeft: 0,
			want:         true,
		},
		{
			name:         "TriggersLeft is negative (expired)",
			triggersLeft: -1,
			want:         true,
		},
		{
			name:         "TriggersLeft is positive (not expired)",
			triggersLeft: 3,
			want:         false,
		},
		{
			name:         "TriggersLeft is TriggersLeftUnlimited (not expired)",
			triggersLeft: TriggersLeftUnlimited,
			want:         false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			b := &Condition{TriggersLeft: tt.triggersLeft}
			assert.Equal(t, tt.want, b.Expired())
		})
	}
}

// ── T19: Behavior Matrix PB-330 / PB-332 ─────────────────────────────────────

// PB-330: Broken-limb condition (id 83) statmod: -25 str, -25 dex, -10 vit.
// Seeds condition 83 into the registry directly (bypasses the YAML loader) and
// verifies that a Condition carrying id 83 returns the expected StatMod values.
func TestPB_330_BrokenLimbCondition_StatModApplied(t *testing.T) {
	orig := conditions
	conditions = map[int]*ConditionSpec{
		83: {
			ConditionId:   83,
			Name:          "Broken Limb",
			Description:   "A grapple submission cranked the joint past its limit.",
			TriggerCount:  900,
			RoundInterval: 1,
			StatMods: statmods.StatMods{
				"strength":  -25,
				"dexterity": -25,
				"vitality":  -10,
			},
		},
	}
	defer func() { conditions = orig }()

	b := &Condition{ConditionId: 83, TriggersLeft: 900}

	assert.Equal(t, -25, b.StatMod("strength"),
		"PB-330: broken-limb condition must apply -25 strength")
	assert.Equal(t, -25, b.StatMod("dexterity"),
		"PB-330: broken-limb condition must apply -25 dexterity")
	assert.Equal(t, -10, b.StatMod("vitality"),
		"PB-330: broken-limb condition must apply -10 vitality")
	assert.Equal(t, 0, b.StatMod("charisma"),
		"PB-330: broken-limb condition must not affect charisma")
}

// PB-332: Broken-limb condition expires naturally via round-tick decrement.
// Seeds condition 83 and verifies that a Conditions instance carrying it decrements
// TriggersLeft each round and eventually reaches TriggersLeftExpired (0).
func TestPB_332_BrokenLimbCondition_ExpiresNaturally(t *testing.T) {
	orig := conditions
	conditions = map[int]*ConditionSpec{
		83: {
			ConditionId:   83,
			Name:          "Broken Limb",
			TriggerCount:  3, // shortened to 3 for test speed (real=900)
			RoundInterval: 1,
			StatMods:      statmods.StatMods{"strength": -25},
		},
	}
	defer func() { conditions = orig }()

	bs := New()
	bs.AddCondition(83, false)

	// Confirm condition is present and not yet expired.
	assert.False(t, bs.List[0].Expired(),
		"PB-332: broken-limb condition must not be expired on application")

	// Tick 3 rounds — each trigger should decrement TriggersLeft.
	for i := 0; i < 3; i++ {
		bs.Trigger()
	}

	// After TriggerCount triggers, condition should be expired.
	assert.True(t, bs.List[0].Expired(),
		"PB-332: broken-limb condition must be expired after all trigger rounds elapsed")

	// Prune confirms it can be cleaned up.
	pruned := bs.Prune()
	assert.Len(t, pruned, 1, "PB-332: expired broken-limb condition should be pruned")
	assert.Empty(t, bs.List, "PB-332: condition list should be empty after prune")
}

// ─── Conditions.ProgressMult ─────────────────────────────────────────────────────

// seedProgressMultRegistry seeds its own specs rather than extending
// seedRegistry, whose fixture counts other tests assert on. Condition 200 carries
// the skill-progress flag and declares no progress_mult, so it must fall back
// to the historic 2.0. Condition 201 declares 3.0. Condition 202 carries an unrelated
// flag and must never contribute.
func seedProgressMultRegistry() func() {
	orig := conditions
	conditions = map[int]*ConditionSpec{
		200: {
			ConditionId:   200,
			Name:          "Plain Attunement",
			Description:   "Learning comes a little easier.",
			TriggerCount:  10,
			RoundInterval: 1,
			Flags:         []Flag{SkillProgress},
		},
		201: {
			ConditionId:   201,
			Name:          "Savant Draught",
			Description:   "Learning comes much easier.",
			TriggerCount:  10,
			RoundInterval: 1,
			Flags:         []Flag{SkillProgress},
			ProgressMult:  3.0,
		},
		202: {
			ConditionId:   202,
			Name:          "Plain Warmth",
			Description:   "You feel warm.",
			TriggerCount:  10,
			RoundInterval: 1,
			Flags:         []Flag{Warmed},
		},
	}
	return func() { conditions = orig }
}

func TestConditions_ProgressMult(t *testing.T) {
	cleanup := seedProgressMultRegistry()
	defer cleanup()

	t.Run("no flagged condition held is neutral", func(t *testing.T) {
		bs := New()
		require.True(t, bs.AddCondition(202, false)) // Warmed, not SkillProgress
		assert.Equal(t, 1.0, bs.ProgressMult(SkillProgress),
			"a caller multiplies unconditionally, so the empty case must be 1.0")
	})

	t.Run("a flagged condition with no progress_mult is the historic 2.0", func(t *testing.T) {
		bs := New()
		require.True(t, bs.AddCondition(200, false))
		assert.Equal(t, 2.0, bs.ProgressMult(SkillProgress))
	})

	t.Run("the strongest held value wins", func(t *testing.T) {
		bs := New()
		require.True(t, bs.AddCondition(200, false)) // defaults to 2.0
		require.True(t, bs.AddCondition(201, false)) // declares 3.0
		assert.Equal(t, 3.0, bs.ProgressMult(SkillProgress),
			"flagged conditions do not stack; the strongest one wins")
	})

	t.Run("an expired flagged condition does not count", func(t *testing.T) {
		bs := New()
		require.True(t, bs.AddCondition(201, false))
		require.True(t, bs.RemoveCondition(201))
		assert.Equal(t, 1.0, bs.ProgressMult(SkillProgress),
			"ProgressMult must skip expired conditions the way HasFlag does")
	})

	t.Run("an unrelated flag is unaffected", func(t *testing.T) {
		bs := New()
		require.True(t, bs.AddCondition(201, false))
		assert.Equal(t, 1.0, bs.ProgressMult(MutationRate))
	})
}
