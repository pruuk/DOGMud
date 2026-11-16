package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/goals"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

func TestTickMobRecomputeGoals_EmptyGoalList_NoOp(t *testing.T) {
	goals.IsolateForTest(t)

	goals.ClearCache()
	mob := &mobs.Mob{MobId: mobs.MobId(99201)}
	mob.Character.Name = "tick_empty_test"
	// No goals seeded — should return early without panic or write.
	tickMobRecomputeGoals(mob, 1000)
	// Nothing to assert beyond "doesn't panic"; the cheap-path early
	// return is the whole behavior.
}

func TestTickMobRecomputeGoals_WithGoals_RecomputesCurrent(t *testing.T) {
	goals.IsolateForTest(t)

	goals.ClearCache()
	mob := &mobs.Mob{MobId: mobs.MobId(99202)}
	mob.Character.Name = "tick_with_goals_test"
	if _, err := goals.Add(int(mob.MobId), "tick_with_goals_test",
		&goals.Goal{Type: "wealth", Priority: 50}); err != nil {
		t.Fatalf("seed Add: %v", err)
	}
	// Re-clear cache to force GoalsOf to reload from disk on the next
	// access, confirming the full disk-round-trip path works. Because
	// setGoalsTempDir pointed the override at a real temp dir, Add's
	// saveToDisk succeeded and the file is readable.
	goals.ClearCache()
	tickMobRecomputeGoals(mob, 1000)
	cur := goals.CurrentGoalOf(int(mob.MobId), "tick_with_goals_test")
	if cur == nil {
		t.Errorf("CurrentGoalOf nil after tick — hook did not Recompute")
	}
}

func TestShouldPruneGoals_CadenceAndStagger(t *testing.T) {
	// interval 0 disables.
	if shouldPruneGoals(100, 5, 0) {
		t.Error("interval 0 must disable pruning")
	}
	// (nowRound + templateId) % interval == 0 → prune.
	// nowRound=45, templateId=5, interval=50 → 50%50==0 → true.
	if !shouldPruneGoals(45, 5, 50) {
		t.Error("expected prune on cadence boundary")
	}
	// nowRound=46, templateId=5, interval=50 → 51%50==1 → false.
	if shouldPruneGoals(46, 5, 50) {
		t.Error("expected no prune off cadence")
	}
}
