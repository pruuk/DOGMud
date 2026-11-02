package catalog

import (
	"testing"

	goals "github.com/GoMudEngine/GoMud/internal/goals"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func TestRevengeMob_Registered(t *testing.T) {
	if _, ok := goals.LookupGoalType("revenge-mob"); !ok {
		t.Fatalf("revenge-mob not registered")
	}
}

func TestRevengeMob_DedupKey_DifferentTargets_Distinct(t *testing.T) {
	meta, _ := goals.LookupGoalType("revenge-mob")
	g1 := &goals.Goal{Type: "revenge-mob", Params: map[string]any{"target_kind": "mob", "target_id": 5}}
	g2 := &goals.Goal{Type: "revenge-mob", Params: map[string]any{"target_kind": "mob", "target_id": 7}}
	if meta.DedupKey(g1) == meta.DedupKey(g2) {
		t.Errorf("dedup keys collide for different targets")
	}
}

func TestRevengeMob_DedupKey_DifferentKinds_Distinct(t *testing.T) {
	meta, _ := goals.LookupGoalType("revenge-mob")
	g1 := &goals.Goal{Type: "revenge-mob", Params: map[string]any{"target_kind": "mob", "target_id": 5}}
	g2 := &goals.Goal{Type: "revenge-mob", Params: map[string]any{"target_kind": "player", "target_id": 5}}
	if meta.DedupKey(g1) == meta.DedupKey(g2) {
		t.Errorf("dedup keys collide for mob:5 vs player:5")
	}
}

func TestRevengeMob_DedupKey_SameTarget_Match(t *testing.T) {
	meta, _ := goals.LookupGoalType("revenge-mob")
	g1 := &goals.Goal{Type: "revenge-mob", Params: map[string]any{"target_kind": "player", "target_id": 3}}
	g2 := &goals.Goal{Type: "revenge-mob", Params: map[string]any{"target_kind": "player", "target_id": 3}}
	if meta.DedupKey(g1) != meta.DedupKey(g2) {
		t.Errorf("dedup keys differ for same target")
	}
}

// A revenge goal is silent for a mob that has not been fighting the target, and
// a goal the mob holds for itself is no exception: a clean theft starts no hunt.
func TestRevengeMob_ContextScore_SightingGatesEveryGoal(t *testing.T) {
	const thief, room = 3301, 3302
	u := users.NewTestUser(thief, "thief", "Nimble", 0)
	u.Character.RoomId = room
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{thief: u}))

	meta, _ := goals.LookupGoalType("revenge-mob")
	mob := &mobs.Mob{}
	mob.Character.RoomId = room
	params := map[string]any{"target_kind": "player", "target_id": thief}

	if got := meta.ContextScore(&goals.Goal{Type: "revenge-mob", Params: params}, mob); got != 0 {
		t.Errorf("never fought, thief beside it: score %v, want 0", got)
	}
	mob.CombatMemory = mobs.SetCombatMemory(thief, 0, room, util.GetRoundCount())
	if got := meta.ContextScore(&goals.Goal{Type: "revenge-mob", Params: params}, mob); got != 2.0 {
		t.Errorf("fought, thief beside it: score %v, want 2", got)
	}
}

// Predicate behavior depends on live mobs/users state. Integration coverage
// lands at Task 23 smoke. Unit-test only the registration + DedupKey shape
// and the sighting gate here.
