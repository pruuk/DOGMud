package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// GameBridge.ChargeGold takes the whole charge or nothing. A charge larger
// than the player's gold takes nothing, sends no "You pay" line, and fails
// the action, so the engine abandons the rest of the trigger.

const cgUserId = 9112

func seedChargeGoldUser(t *testing.T, gold int) (*users.UserRecord, func()) {
	t.Helper()
	u := users.NewTestUser(cgUserId, "chargetest", "Chargetest", uint64(cgUserId))
	u.Character.Gold = gold
	clean := users.SeedUsersForTest(map[int]*users.UserRecord{cgUserId: u})
	events.DrainQueuedMessagesForTest(cgUserId)
	return u, func() {
		events.DrainQueuedMessagesForTest(cgUserId)
		clean()
	}
}

func TestGameBridge_ChargeGold_RefusesWhenShort(t *testing.T) {
	u, cleanup := seedChargeGoldUser(t, 1400)
	defer cleanup()

	err := NewGameBridge(u, 1).ChargeGold(25000)

	if err == nil {
		t.Error("ChargeGold returned nil for a charge larger than the player's gold")
	}
	if u.Character.Gold != 1400 {
		t.Errorf("gold = %d; want 1400 (a refused charge takes nothing)", u.Character.Gold)
	}
	if msgs := strings.Join(events.DrainQueuedMessagesForTest(cgUserId), "\n"); strings.Contains(msgs, "You pay") {
		t.Errorf("player was told they paid on a refused charge:\n%s", msgs)
	}
}

func TestGameBridge_ChargeGold_DeductsWhenAffordable(t *testing.T) {
	u, cleanup := seedChargeGoldUser(t, 25000)
	defer cleanup()

	if err := NewGameBridge(u, 1).ChargeGold(25000); err != nil {
		t.Fatalf("ChargeGold returned %v for an affordable charge", err)
	}
	if u.Character.Gold != 0 {
		t.Errorf("gold = %d; want 0", u.Character.Gold)
	}
	if msgs := strings.Join(events.DrainQueuedMessagesForTest(cgUserId), "\n"); !strings.Contains(msgs, "25,000 gold") {
		t.Errorf("payment line should read 25,000 gold:\n%s", msgs)
	}
}

func TestGameBridge_GiveGold_ThousandsSeparator(t *testing.T) {
	u, cleanup := seedChargeGoldUser(t, 0)
	defer cleanup()

	NewGameBridge(u, 1).GiveGold(12500)

	if msgs := strings.Join(events.DrainQueuedMessagesForTest(cgUserId), "\n"); !strings.Contains(msgs, "12,500 gold") {
		t.Errorf("gift line should read 12,500 gold:\n%s", msgs)
	}
}

// The learned line names the recipe the way `craft list` does, by its
// display name, not its slug. An unknown slug still names itself.
func TestGameBridge_LearnRecipe_UsesDisplayName(t *testing.T) {
	u, cleanup := seedChargeGoldUser(t, 0)
	defer cleanup()
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: "test-hungering-guard", Name: "Hungering Guard", SkillMinimum: 50})
	defer crafting.UnregisterRecipeForTest("test-hungering-guard")

	b := NewGameBridge(u, 1)
	b.LearnRecipe("test-hungering-guard")
	b.LearnRecipe("no-such-recipe-slug")

	msgs := strings.Join(events.DrainQueuedMessagesForTest(cgUserId), "\n")
	if !strings.Contains(msgs, ">Hungering Guard<") || strings.Contains(msgs, "test-hungering-guard") {
		t.Errorf("learned line should name Hungering Guard, not the slug:\n%s", msgs)
	}
	if !strings.Contains(msgs, "no-such-recipe-slug") {
		t.Errorf("an unknown recipe should fall back to its slug:\n%s", msgs)
	}
}
