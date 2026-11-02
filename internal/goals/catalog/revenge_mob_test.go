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

// A type's shared revenge goal is silent for a copy that has not met the
// target. A goal a mob holds for itself is not: it scores by distance alone.
func TestRevengeMob_ContextScore_SightingGatesOnlyASharedGoal(t *testing.T) {
	const thief, room = 3301, 3302
	u := users.NewTestUser(thief, "thief", "Nimble", 0)
	u.Character.RoomId = room
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{thief: u}))

	meta, _ := goals.LookupGoalType("revenge-mob")
	mob := &mobs.Mob{}
	mob.Character.RoomId = room
	params := map[string]any{"target_kind": "player", "target_id": thief}

	if got := meta.ContextScore(&goals.Goal{Type: "revenge-mob", Params: params}, mob); got != 0 {
		t.Errorf("shared goal, never met: score %v, want 0", got)
	}
	if got := meta.ContextScore(&goals.Goal{Type: "revenge-mob", Params: params, Individual: true}, mob); got != 2.0 {
		t.Errorf("own goal, thief beside it: score %v, want 2", got)
	}
	mob.CombatMemory = mobs.SetCombatMemory(thief, 0, room, util.GetRoundCount())
	if got := meta.ContextScore(&goals.Goal{Type: "revenge-mob", Params: params}, mob); got != 2.0 {
		t.Errorf("shared goal, met: score %v, want 2", got)
	}
}

// Predicate behavior depends on live mobs/users state. Integration coverage
// lands at Task 23 smoke. Unit-test only the registration + DedupKey shape
// and the sighting gate here.
