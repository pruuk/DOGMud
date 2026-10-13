package hooks

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Messaging M6 row 7, R4: a craft that ends without finishing tells the room.

const (
	craftEndedRoomId     = 7360
	craftEndedCrafterId  = 7361
	craftEndedWatcherId  = 7362
	craftEndedRecipeId   = "craft-ended-test-recipe"
	craftEndedSkill      = "blacksmithing"
	craftEndedTagPattern = `<[^>]*>`
)

var craftEndedTag = regexp.MustCompile(craftEndedTagPattern)

func craftEndedPlain(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimSpace(craftEndedTag.ReplaceAllString(l, "")))
	}
	return out
}

// craftEndedScene stands a crafter and a watcher in one lit room. The crafter's
// Character comes from characters.New(), so OnCharacterCreated has wired the
// craft-ended hook exactly as in production.
func craftEndedScene(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "Human", Size: species.Medium, Selectable: true},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"city": {BiomeId: "city", Lamp: rooms.LampPtr(90)},
	}))
	room := &rooms.Room{RoomId: craftEndedRoomId, Biome: "city"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{craftEndedRoomId: room}, map[string]*rooms.ZoneConfig{}))

	crafter := users.NewTestUser(craftEndedCrafterId, "kael", "Kael", 97361)
	crafter.Character = characters.New()
	crafter.Character.Name = "Kael"
	crafter.Character.RoomId = craftEndedRoomId
	crafter.Character.SetUserId(craftEndedCrafterId)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		craftEndedCrafterId: crafter,
		craftEndedWatcherId: users.NewTestUser(craftEndedWatcherId, "tob", "Tob", 97362),
	}))
	for _, id := range []int{craftEndedCrafterId, craftEndedWatcherId} {
		room.AddPlayer(id)
		users.GetByUserId(id).Character.RoomId = craftEndedRoomId
		events.DrainQueuedMessagesForTest(id)
	}

	t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{
		craftEndedSkill: {
			// One line per pool: the sender picks at random.
			StartObserver:   []string{"{actor} starts {item}."},
			SuccessObserver: []string{"{actor} finishes {item}."},
			FailureObserver: []string{"{actor} ruins it."},
			CancelObserver:  []string{"{actor} stops forging."},
			StartHeard:      []string{"Hammering starts."},
			FinishHeard:     []string{"The hammering stops."},
		},
	}))
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: craftEndedRecipeId, Skill: craftEndedSkill})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest(craftEndedRecipeId) })
	return crafter
}

func endCraftWith(t *testing.T, crafter *users.UserRecord, trigger string) {
	t.Helper()
	c := crafter.Character
	err := c.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	if err := c.Activity.TransitionToFree(state.TransitionReason{Trigger: trigger}); err != nil {
		t.Fatalf("free: %v", err)
	}
}

func TestCraftEnded_RoomHearsWhenTheCraftStopsShort(t *testing.T) {
	for _, trigger := range []string{
		activity.TriggerCraftCancel,
		activity.TriggerMovementInterrupt,
		activity.TriggerDamageInterrupt,
	} {
		t.Run(trigger, func(t *testing.T) {
			crafter := craftEndedScene(t)
			endCraftWith(t, crafter, trigger)

			got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
			if len(got) != 1 || got[0] != "Kael stops forging." {
				t.Fatalf("watcher read %q, want one line %q", got, "Kael stops forging.")
			}
			if own := events.DrainQueuedMessagesForTest(craftEndedCrafterId); len(own) != 0 {
				t.Fatalf("crafter was sent a room line: %q", own)
			}
		})
	}
}

func TestCraftEnded_CompletionAndDeathSendNoCancelLine(t *testing.T) {
	for _, trigger := range []string{activity.TriggerCraftComplete, activity.TriggerDeath} {
		t.Run(trigger, func(t *testing.T) {
			crafter := craftEndedScene(t)
			endCraftWith(t, crafter, trigger)

			if got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId)); len(got) != 0 {
				t.Fatalf("watcher read %q, want nothing", got)
			}
		})
	}
}
