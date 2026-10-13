package conditions

import "testing"

// PoolTickPerRound averages each live tick record's cached amount over its
// interval (#301): a heal of 6 every 3 rounds is 2 a round, a bleed of -4
// every round is -4, a spent record (TriggersLeft 0) and a record on another
// pool count nothing.
func TestPoolTickPerRound(t *testing.T) {
	t.Cleanup(SeedConditionsForTest(map[int]*ConditionSpec{
		9101: {ConditionId: 9101, Name: "test heal", TickPool: "health", RoundInterval: 3},
		9102: {ConditionId: 9102, Name: "test bleed", TickPool: "health", RoundInterval: 1},
		9103: {ConditionId: 9103, Name: "test stamina", TickPool: "stamina", RoundInterval: 1},
	}))
	v := New()
	cs := &v
	cs.List = append(cs.List,
		&Condition{ConditionId: 9101, TriggersLeft: 5, TickAmount: 6},
		&Condition{ConditionId: 9102, TriggersLeft: 5, TickAmount: -4},
		&Condition{ConditionId: 9102, TriggersLeft: 0, TickAmount: -100},
		&Condition{ConditionId: 9103, TriggersLeft: 5, TickAmount: 50},
	)
	if got := cs.PoolTickPerRound("health"); got != -2 {
		t.Fatalf("PoolTickPerRound(health) = %v, want -2 (2 heal, -4 bleed)", got)
	}
}
