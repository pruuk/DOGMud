package goals

import (
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// The in-package half of a goal one mob holds for itself: test types registered
// here carry a predicate and a score, so these need no catalog.

func registerIndividualTestType(t *testing.T, name string, meta GoalTypeMeta) {
	t.Helper()
	RegisterGoalType(name, meta)
	t.Cleanup(func() {
		registryMu.Lock()
		delete(typeRegistry, name)
		registryMu.Unlock()
	})
}

func individualMob(mobId, instanceId int) *mobs.Mob {
	m := &mobs.Mob{MobId: mobs.MobId(mobId), InstanceId: instanceId}
	m.Character.Name = "individual_test"
	return m
}

func TestAddIndividual_IsThatMobsAloneAndTouchesNoFile(t *testing.T) {
	ClearCache()
	t.Cleanup(ClearCache)
	const typ = "individual-test-own"
	registerIndividualTestType(t, typ, GoalTypeMeta{})

	one, two := individualMob(98001, 1), individualMob(98001, 2)
	if _, err := AddIndividual(one, &Goal{Type: typ, Priority: 50}); err != nil {
		t.Fatalf("AddIndividual: %v", err)
	}

	if got := CurrentGoalFor(one); got == nil || got.Type != typ {
		t.Fatalf("holder: current = %+v", got)
	}
	if got := CurrentGoalFor(two); got != nil {
		t.Fatalf("other copy: current = %+v, want nil", got)
	}
	if got := GoalsOf(98001, "individual_test"); len(got) != 0 {
		t.Fatalf("the type's goals = %+v, want none", got)
	}
	if body, err := os.ReadFile(goalPath(98001, "individual_test")); err == nil && strings.Contains(string(body), typ) {
		t.Fatalf("the goal reached the type's file:\n%s", body)
	}
}

func TestAddIndividual_DedupsByTheTypesKey(t *testing.T) {
	const typ = "individual-test-dedup"
	registerIndividualTestType(t, typ, GoalTypeMeta{
		AllowMultiple: true,
		DedupKey:      func(g *Goal) string { s, _ := g.Params["k"].(string); return s },
	})
	m := individualMob(98002, 1)

	first, _ := AddIndividual(m, &Goal{Type: typ, Priority: 10, Params: map[string]any{"k": "a"}})
	if first == nil {
		t.Fatal("first was not added")
	}
	if again, _ := AddIndividual(m, &Goal{Type: typ, Priority: 99, Params: map[string]any{"k": "a"}}); again != nil {
		t.Fatalf("a repeat was added: %+v", again)
	}
	if other, _ := AddIndividual(m, &Goal{Type: typ, Priority: 20, Params: map[string]any{"k": "b"}}); other == nil {
		t.Fatal("a second target was refused")
	}
	if got := IndividualGoalsOf(m); len(got) != 2 || got[0].Priority != 10 {
		t.Fatalf("goals = %+v, want a at its first priority, and b", got)
	}
}

func TestAddIndividual_ValidatesParams(t *testing.T) {
	const typ = "individual-test-params"
	registerIndividualTestType(t, typ, GoalTypeMeta{
		Params: []ParamSchema{{Key: "need", Required: true, GoType: "int"}},
	})
	m := individualMob(98003, 1)
	if _, err := AddIndividual(m, &Goal{Type: typ}); err == nil {
		t.Fatal("a goal missing a required param was accepted")
	}
	if got := IndividualGoalsOf(m); len(got) != 0 {
		t.Fatalf("goals = %+v, want none", got)
	}
}

func TestIndividualGoalsOf_DropsSatisfiedForGood(t *testing.T) {
	const typ = "individual-test-satisfied"
	done := false
	registerIndividualTestType(t, typ, GoalTypeMeta{
		Predicate: func(*Goal, *mobs.Mob) bool { return done },
	})
	m := individualMob(98004, 1)
	if _, err := AddIndividual(m, &Goal{Type: typ}); err != nil {
		t.Fatal(err)
	}
	done = true
	if got := IndividualGoalsOf(m); len(got) != 0 {
		t.Fatalf("satisfied goal kept: %+v", got)
	}
	done = false
	if got := IndividualGoalsOf(m); len(got) != 0 {
		t.Fatalf("a finished goal came back: %+v", got)
	}
}

func TestCurrentGoalFor_ScoresAgainstTheTypesGoal(t *testing.T) {
	ClearCache()
	t.Cleanup(ClearCache)
	const shared, own, silent = "individual-test-shared", "individual-test-mine", "individual-test-silent"
	registerIndividualTestType(t, shared, GoalTypeMeta{})
	registerIndividualTestType(t, own, GoalTypeMeta{})
	registerIndividualTestType(t, silent, GoalTypeMeta{
		ContextScore: func(*Goal, *mobs.Mob) float64 { return 0 },
	})
	m := individualMob(98005, 1)
	if _, err := Add(98005, "individual_test", &Goal{Type: shared, Priority: 50}); err != nil {
		t.Fatal(err)
	}

	mustAdd := func(g *Goal) {
		t.Helper()
		if _, err := AddIndividual(m, g); err != nil {
			t.Fatal(err)
		}
	}
	mustAdd(&Goal{Type: silent, Priority: 900})
	if got := CurrentGoalFor(m); got == nil || got.Type != shared {
		t.Fatalf("own goal scoring zero: current = %+v, want the type's", got)
	}
	mustAdd(&Goal{Type: own, Priority: 50})
	if got := CurrentGoalFor(m); got == nil || got.Type != shared {
		t.Fatalf("a tie: current = %+v, want the type's", got)
	}
	m.SetTempData(individualGoalsKey, nil)
	mustAdd(&Goal{Type: own, Priority: 51})
	if got := CurrentGoalFor(m); got == nil || got.Type != own {
		t.Fatalf("own goal outscoring: current = %+v, want its own", got)
	}
}

func TestDropStaleGoals_RunsTheCheckOnLoadOnly(t *testing.T) {
	ClearCache()
	t.Cleanup(ClearCache)
	t.Cleanup(func() { SetStaleGoalCheck(nil) })
	const typ = "individual-test-stale"
	registerIndividualTestType(t, typ, GoalTypeMeta{AllowMultiple: true, DedupKey: func(g *Goal) string { return g.Id }})

	if _, err := Add(98006, "stale_test", &Goal{Type: typ, Priority: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(98006, "stale_test", &Goal{Type: typ, Priority: 20}); err != nil {
		t.Fatal(err)
	}
	SetStaleGoalCheck(func(_ int, g *Goal) bool { return g.Priority == 10 })
	if got := GoalsOf(98006, "stale_test"); len(got) != 2 {
		t.Fatalf("a cached file was pruned: %+v", got)
	}

	ClearCache()
	got := GoalsOf(98006, "stale_test")
	if len(got) != 1 || got[0].Priority != 20 {
		t.Fatalf("after load: %+v, want the priority 20 goal", got)
	}
	ClearCache()
	SetStaleGoalCheck(nil)
	if got := GoalsOf(98006, "stale_test"); len(got) != 1 {
		t.Fatalf("the removal was not saved: %+v", got)
	}
}

func TestDropStaleGoals_APanickingCheckKeepsEveryGoal(t *testing.T) {
	ClearCache()
	t.Cleanup(ClearCache)
	t.Cleanup(func() { SetStaleGoalCheck(nil) })
	const typ = "individual-test-panic"
	registerIndividualTestType(t, typ, GoalTypeMeta{})
	if _, err := Add(98007, "panic_test", &Goal{Type: typ, Priority: 10}); err != nil {
		t.Fatal(err)
	}
	SetStaleGoalCheck(func(int, *Goal) bool { panic("no") })
	ClearCache()
	if got := GoalsOf(98007, "panic_test"); len(got) != 1 {
		t.Fatalf("goals = %+v, want the one kept", got)
	}
}
