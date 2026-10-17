package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The 2026-10-17 newbie quest audit (S3): the craft quest event went out when
// a timed craft BEGAN, and the contest that decides it runs at the last round,
// so a failed brew still granted quest 38's brew step. Owner rule: a quest
// step advances only when the action succeeds. The event now goes out from
// tickUserCrafting, on a craft that succeeds. Quests 35, 38, 42, 54 and 55
// listen for it with `event: command, command: craft` and a room filter, so
// it still names the crafter's room.

const craftQuestProbeId = 999251

// listenForCraftCompletion registers a one-trigger quest granting
// craftQuestProbeId-start on a `craft` command event in the crafter's room,
// and returns a probe reporting whether it fired.
func listenForCraftCompletion(t *testing.T, user *users.UserRecord) func() bool {
	t.Helper()
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	const token = "999251-start"
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: craftQuestProbeId,
		Name:    "craft completion probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:   "command",
			Command: "craft",
			Room:    craftEndedRoomId,
			Actions: []questengine.ActionDef{{Grant: token}},
		}},
	})
	user.Character.ClearQuestToken(token)
	t.Cleanup(func() { user.Character.ClearQuestToken(token) })
	return func() bool { return user.Character.HasQuest(token) }
}

// craftQuestScene is craftEndedScene's crafter on the last round of a craft,
// with stats a contest can be won on (a zero score rolls a deterministic 0).
func craftQuestScene(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	crafter := craftEndedScene(t)
	c := crafter.Character
	c.Stats.Strength.Base = 500
	c.Stats.Perception.Base = 500
	c.Stats.Dexterity.Base = 500
	c.RecalculateStats()
	if err := c.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 1},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	); err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	return crafter, rooms.LoadRoom(craftEndedRoomId)
}

func TestTickUserCrafting_ASucceedingCraftSendsTheQuestEvent(t *testing.T) {
	pinCraftChance(t, 100)
	crafter, room := craftQuestScene(t)
	fired := listenForCraftCompletion(t, crafter)

	tickUserCrafting(crafter, room)

	if crafter.Character.Activity.IsCrafting() {
		t.Fatal("fixture: the craft did not complete on its last round")
	}
	if !fired() {
		t.Fatal("a craft that succeeds must send the craft command event")
	}
}

func TestTickUserCrafting_AFailedCraftSendsNoQuestEvent(t *testing.T) {
	pinCraftChance(t, 0)
	crafter, room := craftQuestScene(t)
	fired := listenForCraftCompletion(t, crafter)

	tickUserCrafting(crafter, room)

	if crafter.Character.Activity.IsCrafting() {
		t.Fatal("fixture: the craft did not complete on its last round")
	}
	if fired() {
		t.Fatal("a craft that fails must not send the craft command event")
	}
}

// A craft still under way sends nothing either.
func TestTickUserCrafting_AnUnfinishedCraftSendsNoQuestEvent(t *testing.T) {
	pinCraftChance(t, 100)
	crafter := craftEndedScene(t)
	if err := crafter.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 3},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	); err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	fired := listenForCraftCompletion(t, crafter)

	tickUserCrafting(crafter, rooms.LoadRoom(craftEndedRoomId))

	if !crafter.Character.Activity.IsCrafting() {
		t.Fatal("fixture: a three-round craft finished after one round")
	}
	if fired() {
		t.Fatal("a craft still under way must not send the craft command event")
	}
}
