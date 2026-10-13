package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// #515 follow-up (owner ruling C, 2026-10-13): the room's lines about a
// crafter wrap at the reader's width, for a player crafter as for a mob. A
// long crafter name and a 34-character item name push the start line well
// past 80 columns.

const (
	wrapItemId   = 8302
	wrapItemName = "Resplendent Chrysalis-Bound Glaive" // 34 characters
	wrapCrafter  = "Bartholomew-Aldricksworth"
)

var wrapTagPattern = regexp.MustCompile(`</?ansi[^>]*>`)

func TestCraftPhase_TheRoomLineWrapsAt80(t *testing.T) {
	if len(wrapItemName) != 34 {
		t.Fatalf("fixture: item name is %d characters, want 34", len(wrapItemName))
	}
	room := craftScene(t, "city", false)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		wrapItemId: {ItemId: wrapItemId, Name: wrapItemName},
	}))
	t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{"blacksmithing": {
		StartObserver: []string{"{actor} sets to work on {item} with slow and careful hands."},
	}}))
	crafter := users.GetByUserId(phaseCasterId)
	orig := crafter.Character.Name
	crafter.Character.Name = wrapCrafter
	t.Cleanup(func() { crafter.Character.Name = orig })
	events.DrainQueuedMessagesForTest(phaseBystanderId)

	r := craftRecipe()
	r.Output.ItemId = wrapItemId
	craftBy(crafter, room, r, crafting.PhaseStart)

	got := events.DrainQueuedMessagesForTest(phaseBystanderId)
	if len(got) != 1 {
		t.Fatalf("bystander read %d messages, want 1: %q", len(got), got)
	}
	lines := strings.Split(strings.TrimRight(got[0], "\n"), "\n")
	plain := wrapTagPattern.ReplaceAllString(got[0], "")
	if !strings.Contains(strings.Join(strings.Fields(plain), " "), wrapCrafter+" sets to work on "+wrapItemName) {
		t.Fatalf("fixture: the start line did not render as expected: %q", plain)
	}
	// The onlooker color rides the category alias (craft-seen, same value
	// as emote and mob-idle in ansi-aliases.yaml).
	if want := `fg="` + messaging.CategoryCraftSeen.String() + `"`; !strings.Contains(got[0], want) {
		t.Errorf("the room line is not colored as craft-seen: %q", got[0])
	}
	if len(lines) < 2 {
		t.Fatalf("a %d-column room line must fold at 80, got one line: %q", util.VisibleWidth(got[0]), got[0])
	}
	for i, line := range lines {
		if w := util.VisibleWidth(line); w > 80 {
			t.Errorf("line %d is %d visible columns, over 80: %q", i+1, w, line)
		}
	}
}
