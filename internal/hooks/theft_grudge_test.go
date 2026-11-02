package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/goals"
	_ "github.com/GoMudEngine/GoMud/internal/goals/catalog" // main.go's registration: the real predicate and score
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/planners"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/seeders"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The owner's ruling of 2026-11-01, from the steal's success point to the
// behaviour tree's planner: a generic mob's theft grudge is that individual's.
// Copies come from NewMobById, the real spawn path, because its shallow copy
// of the template is the trap a per-copy grudge has to survive.

const (
	theftRoom      = 8801
	theftOtherRoom = 8802
	theftZone      = "Theft Grudge Zone"
	theftThief     = 77

	scavengerTemplate = 88010 // generic: the type robbed
	wolfTemplate      = 88011 // generic: a witness of another type
	guardTemplate     = 88012
	civilianTemplate  = 88013
)

type theftWorld struct {
	here  *rooms.Room
	thief *users.UserRecord
}

func theftTemplate(id int, name string, groups []string, nonCombatant bool) *mobs.Mob {
	m := &mobs.Mob{MobId: mobs.MobId(id), Zone: theftZone, Groups: groups}
	m.Character = *characters.New()
	m.Character.Name = name
	m.Character.Zone = theftZone
	m.Character.NonCombatant = nonCombatant
	m.NonCombatant = nonCombatant
	return m
}

// setupTheftWorld builds two rooms of one zone, a thief standing in the first
// and the template registry, with goal files in a throwaway directory.
func setupTheftWorld(t *testing.T) *theftWorld {
	t.Helper()
	drainEventQueueOnCleanup(t)

	cfg := configs.GetConfig()
	cfg.Balance.GoalSelectTickEnabled = true
	configs.SetConfigForTest(t, cfg)

	dir := filepath.Join(t.TempDir(), "goals")
	prev, had := os.LookupEnv("DOGMUD_GOALS_DIR_OVERRIDE")
	os.Setenv("DOGMUD_GOALS_DIR_OVERRIDE", dir)
	t.Cleanup(func() {
		if had {
			os.Setenv("DOGMUD_GOALS_DIR_OVERRIDE", prev)
		} else {
			os.Unsetenv("DOGMUD_GOALS_DIR_OVERRIDE")
		}
	})
	goals.ClearCache()
	t.Cleanup(goals.ClearCache)

	here := &rooms.Room{RoomId: theftRoom, Zone: theftZone}
	there := &rooms.Room{RoomId: theftOtherRoom, Zone: theftZone}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{theftRoom: here, theftOtherRoom: there},
		map[string]*rooms.ZoneConfig{theftZone: {Name: theftZone}},
	))

	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		scavengerTemplate: theftTemplate(scavengerTemplate, "Scree Scavenger", []string{"beast"}, false),
		wolfTemplate:      theftTemplate(wolfTemplate, "Ridge Wolf", []string{"beast"}, false),
		guardTemplate:     theftTemplate(guardTemplate, "Watchman", []string{"guard"}, false),
		civilianTemplate:  theftTemplate(civilianTemplate, "Weaver", []string{"humanoid"}, true),
	}, map[int]*mobs.Mob{}))

	thief := users.NewTestUser(theftThief, "thief", "Nimble", 0)
	thief.Character.RoomId = theftRoom
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{thief.UserId: thief}))

	return &theftWorld{here: here, thief: thief}
}

// spawnCopy spawns a mob as a room's spawn does and stands it in room.
func spawnCopy(t *testing.T, templateId int, room *rooms.Room) *mobs.Mob {
	t.Helper()
	m := mobs.NewMobByIdFresh(mobs.MobId(templateId), room.RoomId)
	if m == nil {
		t.Fatalf("no copy of template %d", templateId)
	}
	mobs.SetInstanceForTest(m.InstanceId, m)
	m.Character.Zone = theftZone
	room.AddMob(m.InstanceId)
	return m
}

// tickAndPlan runs what a round does for these mobs, on every copy, in the
// order the round and idle hooks do, and reports whether the planner issued m
// a command this round. The same planner call an idle mob gets.
func tickAndPlan(round uint64, m *mobs.Mob, everyone ...*mobs.Mob) bool {
	for _, each := range everyone {
		tickMobRecomputeGoals(each, round)
	}
	behaviortree.RunGoalPlanner(m, round)
	acted, _ := m.GetTempData("goalActedRound").(uint64)
	return acted == round
}

func TestTheft_TheRobbedCopyHuntsAndAnotherCopyDoesNot(t *testing.T) {
	w := setupTheftWorld(t)
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	other := spawnCopy(t, scavengerTemplate, w.here)

	seeders.OnTheft(theftThief, robbed, items.Item{})

	if !tickAndPlan(1000, robbed, robbed, other) {
		t.Fatalf("the robbed copy did not hunt the thief")
	}
	if tickAndPlan(1001, other, robbed, other) {
		t.Fatalf("a copy that was not robbed hunted the thief")
	}
}

// The way every copy came to hunt on master: one copy has fought the thief, so
// its memory lets the type's revenge goal become the type's current goal, and
// every copy reads that.
func TestTheft_ACopyThatFoughtTheThiefDoesNotSetTheOthersHunting(t *testing.T) {
	w := setupTheftWorld(t)
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	other := spawnCopy(t, scavengerTemplate, w.here)
	robbed.CombatMemory = mobs.SetCombatMemory(theftThief, 0, theftRoom, util.GetRoundCount())

	seeders.OnTheft(theftThief, robbed, items.Item{})

	if !tickAndPlan(1000, robbed, robbed, other) {
		t.Fatalf("the robbed copy did not hunt the thief")
	}
	if tickAndPlan(1001, other, robbed, other) {
		t.Fatalf("a copy that was not robbed hunted the thief")
	}
}

func TestTheft_AWitnessOfAnotherTypeHuntsAloneAndGuardsAndCiviliansDoNot(t *testing.T) {
	w := setupTheftWorld(t)
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	witness := spawnCopy(t, wolfTemplate, w.here)
	guard := spawnCopy(t, guardTemplate, w.here)
	civilian := spawnCopy(t, civilianTemplate, w.here)
	elsewhere := &rooms.Room{RoomId: theftOtherRoom, Zone: theftZone}
	sameTypeAway := spawnCopy(t, wolfTemplate, elsewhere)

	seeders.OnTheft(theftThief, robbed, items.Item{})

	everyone := []*mobs.Mob{robbed, witness, guard, civilian, sameTypeAway}
	if !tickAndPlan(1000, witness, everyone...) {
		t.Errorf("the witness did not hunt the thief")
	}
	if tickAndPlan(1001, sameTypeAway, everyone...) {
		t.Errorf("a copy of the witness's type, in another room, hunted the thief")
	}
	if tickAndPlan(1002, guard, everyone...) {
		t.Errorf("a guard hunted the thief: the crime record is its answer")
	}
	if tickAndPlan(1003, civilian, everyone...) {
		t.Errorf("a noncombatant hunted the thief: it takes fright and leaves")
	}
}

func TestTheft_TheGrudgeDiesWithTheMobAndItsRespawnDoesNotCarryIt(t *testing.T) {
	w := setupTheftWorld(t)
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	robbed.CombatMemory = mobs.SetCombatMemory(theftThief, 0, theftRoom, util.GetRoundCount())

	seeders.OnTheft(theftThief, robbed, items.Item{})
	if !tickAndPlan(1000, robbed, robbed) {
		t.Fatalf("the robbed copy did not hunt the thief, so its death proves nothing")
	}

	if err := robbed.Character.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	robbed.Character.Die(state.ActorRef{UserId: theftThief}, life.TriggerHealthZero)
	if mobs.GetInstance(robbed.InstanceId) != nil {
		t.Fatalf("the robbed copy outlived its death")
	}

	respawn := spawnCopy(t, scavengerTemplate, w.here)
	if tickAndPlan(1001, respawn, respawn) {
		t.Fatalf("the respawn hunted the thief")
	}

	// A restart is a respawn with an empty cache: nothing of it reached the
	// type's goal file.
	goals.ClearCache()
	for _, g := range goals.GoalsOf(scavengerTemplate, "scree_scavenger") {
		if g.Type == "revenge-mob" {
			t.Fatalf("the type's goal file holds the grudge: %+v", g)
		}
	}
}

// The hunt is the revenge planner's and goes where the thief goes: a thief in
// another room of the zone is walked to, and the attack comes in the same room.
func TestTheft_TheHuntIsTheRevengePlanner(t *testing.T) {
	w := setupTheftWorld(t)
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	seeders.OnTheft(theftThief, robbed, items.Item{})

	goal := goals.CurrentGoalFor(robbed)
	if goal == nil || goal.Type != "revenge-mob" {
		t.Fatalf("current goal = %+v, want its own revenge-mob", goal)
	}
	plan := planners.LookupPlanner(goal.Type)
	if got := plan(robbed, goal).Command; got != "attack Nimble" {
		t.Errorf("thief beside it: command %q, want the attack", got)
	}
	w.thief.Character.RoomId = theftOtherRoom
	if got := plan(robbed, goal).Command; got != "pathto 8802" {
		t.Errorf("thief next door: command %q, want the walk", got)
	}
}

// The grudge ends the way a revenge goal always has: with the thief dead or
// gone from the game.
func TestTheft_TheGrudgeEndsWithTheThiefDeadOrOffline(t *testing.T) {
	w := setupTheftWorld(t)
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	seeders.OnTheft(theftThief, robbed, items.Item{})
	if goals.CurrentGoalFor(robbed) == nil {
		t.Fatalf("no grudge to end")
	}

	w.thief.Character.Health = 0
	if g := goals.CurrentGoalFor(robbed); g != nil {
		t.Fatalf("thief dead: still holds %+v", g)
	}
	w.thief.Character.Health = 100
	if g := goals.CurrentGoalFor(robbed); g != nil {
		t.Fatalf("thief back on their feet: the grudge came back: %+v", g)
	}

	seeders.OnTheft(theftThief, robbed, items.Item{})
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{}))
	if g := goals.CurrentGoalFor(robbed); g != nil {
		t.Fatalf("thief offline: still holds %+v", g)
	}
}

// A second theft is the same grudge, not a second one, and a robbed witness
// keeps the priority it had, as a type's goal does.
func TestTheft_ARepeatTheftIsTheSameGrudge(t *testing.T) {
	w := setupTheftWorld(t)
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	seeders.OnTheft(theftThief, robbed, items.Item{})
	seeders.OnTheft(theftThief, robbed, items.Item{})
	if got := goals.IndividualGoalsOf(robbed); len(got) != 1 {
		t.Fatalf("own goals = %d, want one", len(got))
	}
}

// A grudge of its own competes with what the type is doing on the type's own
// score, so a mob whose type has a stronger pull goes on with that.
func TestTheft_ATypesStrongerGoalOutranksTheGrudge(t *testing.T) {
	w := setupTheftWorld(t)
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	if _, err := goals.Add(scavengerTemplate, "scree_scavenger",
		&goals.Goal{Type: "theft-test-strong-goal", Priority: 500}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	goals.Recompute(scavengerTemplate, "scree_scavenger", robbed, 1000)

	seeders.OnTheft(theftThief, robbed, items.Item{})

	if g := goals.CurrentGoalFor(robbed); g == nil || g.Type != "theft-test-strong-goal" {
		t.Fatalf("current goal = %+v, want the type's stronger one", g)
	}
	if len(goals.IndividualGoalsOf(robbed)) != 1 {
		t.Fatalf("the grudge should still be held")
	}
}

// Guard: a one-of-a-kind NPC keeps the grudge on its type, exactly as before
// the ruling. It passes on master by design; it states what must not change.
func TestTheft_AOneOfAKindKeepsTheGrudgeOnTheType(t *testing.T) {
	w := setupTheftWorld(t)
	t.Cleanup(rooms.SeedTemplateSpawnsForTest(theftRoom, scavengerTemplate))
	t.Cleanup(dialogue.InstallForTest(&dialogue.DialogueFile{MobId: scavengerTemplate, Zone: theftZone}))
	robbed := spawnCopy(t, scavengerTemplate, w.here)
	robbed.CombatMemory = mobs.SetCombatMemory(theftThief, 0, theftRoom, util.GetRoundCount())

	seeders.OnTheft(theftThief, robbed, items.Item{})

	found := false
	for _, g := range goals.GoalsOf(scavengerTemplate, "scree_scavenger") {
		found = found || g.Type == "revenge-mob"
	}
	if !found {
		t.Fatalf("the type's file does not hold the grudge")
	}
	if !tickAndPlan(1000, robbed, robbed) {
		t.Fatalf("the NPC did not hunt the thief")
	}
	respawn := spawnCopy(t, scavengerTemplate, w.here)
	if !tickAndPlan(1001, respawn, respawn) {
		t.Fatalf("for a one-of-a-kind, type and individual are the same: a new body keeps the grudge")
	}
}
