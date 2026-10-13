package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Messaging M6 row 7: a shop keeper's background craft goes through the shared
// craft sender (success and failure only, no start line). The result-to-lines
// step is narrateShopCraft, tested directly: forcing TickMobCraft to a verdict
// needs a shop inventory and a pinned contest, which is not what is under test.

const shopCraftItemId = 98371

func shopCraftScene(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	craftEndedScene(t) // lit room, watcher, pools for "blacksmithing", recipe craftEndedRecipeId
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		shopCraftItemId: {ItemId: shopCraftItemId, Name: "iron dagger"},
	}))
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{
		RecipeId: craftEndedRecipeId, Name: "Test Dagger", Skill: craftEndedSkill,
		Output: crafting.RecipeOutput{ItemId: shopCraftItemId, Quantity: 1},
	})
	room := rooms.LoadRoom(craftEndedRoomId)
	m := &mobs.Mob{InstanceId: craftEndedMobId}
	m.Character.Name = "Smith"
	m.Character.RoomId = craftEndedRoomId
	return m, room
}

func watcherLines(t *testing.T) []string {
	t.Helper()
	return craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
}

func TestNarrateShopCraft_SuccessReadsThePoolLineNamingTheItem(t *testing.T) {
	m, room := shopCraftScene(t)
	narrateShopCraft(m, room, &mobs.CraftResult{Success: true, RecipeId: craftEndedRecipeId, RecipeName: "Test Dagger"})
	got := watcherLines(t)
	if len(got) != 1 || got[0] != "Smith finishes Iron Dagger." {
		t.Fatalf("watcher read %q, want one line %q", got, "Smith finishes Iron Dagger.")
	}
}

func TestNarrateShopCraft_FailureReadsTheFailureLine(t *testing.T) {
	m, room := shopCraftScene(t)
	narrateShopCraft(m, room, &mobs.CraftResult{Success: false, RecipeId: craftEndedRecipeId, RecipeName: "Test Dagger"})
	got := watcherLines(t)
	if len(got) != 1 || got[0] != "Smith ruins it." {
		t.Fatalf("watcher read %q, want one line %q", got, "Smith ruins it.")
	}
}

func TestNarrateShopCraft_SalvageKeepsItsOwnLine(t *testing.T) {
	m, room := shopCraftScene(t)
	narrateShopCraft(m, room, &mobs.CraftResult{Success: true, Salvaged: true, OutputItemId: shopCraftItemId})
	got := watcherLines(t)
	if len(got) != 1 || got[0] != "Smith finishes crafting and sets a new item on the shelf." {
		t.Fatalf("watcher read %q, want the salvage line", got)
	}
}

func TestNarrateShopCraft_RestockOnlyReadsASupplyCartLine(t *testing.T) {
	m, room := shopCraftScene(t)
	narrateShopCraft(m, room, &mobs.CraftResult{Restocked: true})
	got := watcherLines(t)
	if len(got) != 1 || !strings.Contains(got[0], "Smith") ||
		!(strings.Contains(got[0], "supply cart") || strings.Contains(got[0], "crate of supplies") || strings.Contains(got[0], "runner drops off")) {
		t.Fatalf("watcher read %q, want a supply-cart line naming Smith", got)
	}
}

func TestNarrateShopCraft_UnknownRecipeSendsNothing(t *testing.T) {
	m, room := shopCraftScene(t)
	narrateShopCraft(m, room, &mobs.CraftResult{Success: true, RecipeId: "no-such-recipe", RecipeName: "Ghost"})
	if got := watcherLines(t); len(got) != 0 {
		t.Fatalf("watcher read %q, want nothing", got)
	}
}
