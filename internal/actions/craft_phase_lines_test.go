package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Messaging M6 row 7: every craft's room lines go through one body.
// phaseScene stands Kael (7351, the crafter), Mira (7352) and Tob (7353).

const (
	craftItemId  = 8301
	craftSleepId = 7363
)

func craftScene(t *testing.T, biome string, infrared bool) *rooms.Room {
	t.Helper()
	room := phaseScene(t, biome, infrared)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		craftItemId: {ItemId: craftItemId, Name: "iron dagger"},
	}))
	t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{"blacksmithing": {
		StartObserver:   []string{"{actor} begins forging {item}."},
		SuccessObserver: []string{"{actor} finishes {item}."},
		FailureObserver: []string{"{actor} ruins the work."},
		StartHeard:      []string{"You hear hammering close by."},
		FinishHeard:     []string{"The hammering stops."},
	}}))
	return room
}

func craftRecipe() *crafting.RecipeSpec {
	return &crafting.RecipeSpec{RecipeId: "test-dagger", Name: "Test Dagger", Skill: "blacksmithing",
		Output: crafting.RecipeOutput{ItemId: craftItemId, Quantity: 1}}
}

func craftBy(u *users.UserRecord, room *rooms.Room, r *crafting.RecipeSpec, p crafting.Phase) {
	NarrateCraftPhase(NewUserActorInRoom(u, room), r, p, room)
}

func TestCraftPhase_FullSightReadsTheItemAndTheCrafterReadsNothing(t *testing.T) {
	room := craftScene(t, "city", false)
	craftBy(users.GetByUserId(phaseCasterId), room, craftRecipe(), crafting.PhaseStart)

	requireOneLine(t, "bystander", phaseLines(phaseBystanderId), "Kael begins forging Iron Dagger.")
	if got := phaseLines(phaseCasterId); len(got) != 0 {
		t.Fatalf("the crafter read a room line: %q", got)
	}
}

func TestCraftPhase_ShapesReadsAFigureAndSomething(t *testing.T) {
	room := craftScene(t, "cave", true)
	craftBy(users.GetByUserId(phaseCasterId), room, craftRecipe(), crafting.PhaseSuccess)
	requireOneLine(t, "shapes bystander", phaseLines(phaseBystanderId), "A figure finishes something.")
}

func TestCraftPhase_NoSightHearsTheStartAndTheSameFinishEitherWay(t *testing.T) {
	room := craftScene(t, "cave", false)
	crafter := users.GetByUserId(phaseCasterId)

	craftBy(crafter, room, craftRecipe(), crafting.PhaseStart)
	requireOneLine(t, "dark bystander, start", phaseLines(phaseBystanderId), "You hear hammering close by.")
	craftBy(crafter, room, craftRecipe(), crafting.PhaseSuccess)
	requireOneLine(t, "dark bystander, success", phaseLines(phaseBystanderId), "The hammering stops.")
	craftBy(crafter, room, craftRecipe(), crafting.PhaseFailure)
	requireOneLine(t, "dark bystander, failure", phaseLines(phaseBystanderId), "The hammering stops.")
}

func TestCraftPhase_SleeperHearsTheCraft(t *testing.T) {
	room := craftScene(t, "city", false)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		craftSleepId: {ConditionId: craftSleepId, Name: "Test Sleep", Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	if !users.GetByUserId(phaseBystanderId).Character.Conditions.AddCondition(craftSleepId, true) {
		t.Fatal("precondition: asleep")
	}
	craftBy(users.GetByUserId(phaseCasterId), room, craftRecipe(), crafting.PhaseStart)
	requireOneLine(t, "sleeping bystander", phaseLines(phaseBystanderId), "You hear hammering close by.")
	requireOneLine(t, "awake bystander", phaseLines(phaseTargetId), "Kael begins forging Iron Dagger.")
}

// R3 as for spells: a reader who does not perceive a hidden crafter hears the
// craft and never reads a name, even in a lit room.
func TestCraftPhase_HiddenCrafterIsHeardNotSeen(t *testing.T) {
	room := craftScene(t, "city", false)
	crafter := users.GetByUserId(phaseCasterId)
	phaseHide(t, crafter.Character)

	craftBy(crafter, room, craftRecipe(), crafting.PhaseStart)
	for _, id := range []int{phaseTargetId, phaseBystanderId} {
		requireOneLine(t, "lit non-perceiver", phaseLines(id), "You hear hammering close by.")
	}
}

func TestCraftPhase_SeeHiddenReaderReadsAHiddenMobCrafter(t *testing.T) {
	room := craftScene(t, "city", false)
	m := phaseMob(t, room, true)
	if !users.GetByUserId(phaseTargetId).Character.Conditions.AddCondition(phaseSeeHiddenId, true) {
		t.Fatal("precondition: see hidden")
	}

	NarrateCraftPhase(NewMobActorInRoom(m, room), craftRecipe(), crafting.PhaseSuccess, room)

	got := phaseLines(phaseTargetId)
	if len(got) != 1 || !strings.Contains(got[0], "Shaman") || !strings.HasSuffix(got[0], "finishes Iron Dagger.") {
		t.Fatalf("a perceiving reader reads the named line, got %q", got)
	}
	requireOneLine(t, "non-perceiver", phaseLines(phaseBystanderId), "The hammering stops.")
}

func TestCraftPhase_RecipeWithNoOutputItemReadsSomething(t *testing.T) {
	room := craftScene(t, "city", false)
	r := craftRecipe()
	r.Output.ItemId, r.EnchantType = 0, "test-enchant"
	craftBy(users.GetByUserId(phaseCasterId), room, r, crafting.PhaseStart)
	requireOneLine(t, "bystander", phaseLines(phaseBystanderId), "Kael begins forging something.")
}

func TestCraftPhase_SkillWithNoPoolsSendsNothing(t *testing.T) {
	room := craftScene(t, "city", false)
	r := craftRecipe()
	r.Skill = "tailoring"
	craftBy(users.GetByUserId(phaseCasterId), room, r, crafting.PhaseStart)
	if got := phaseLines(phaseBystanderId); len(got) != 0 {
		t.Fatalf("no pools for the skill, yet the room read %q", got)
	}
}
