package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// `remove` never said the reservation came back.
//
// U7b added a line on equip ("Putting that on sets part of you aside...") and
// nothing at all on remove, so a 2026-08-16 re-check playtest took the item off
// and heard only "You remove your Blackrayor and return it to your backpack."
// The disclosure being CORRECT in internal/characters is worth nothing if the
// command never asks for it, so this drives the real `remove` command.

func removeAndCollect(t *testing.T, itemId int, name string) []string {
	t.Helper()

	user, room := getTestUserAndRoom(t)

	// NewTestUser writes the pool maxima straight to .Value, which is DERIVED:
	// the Validate() inside the remove path recomputes it from .Base and would
	// leave every pool at 1, making a percentage reservation floor to nothing.
	user.Character.HealthMax.Base = 400
	user.Character.StaminaMax.Base = 200
	user.Character.ConvictionMax.Base = 500
	user.Character.Equipment.Neck = items.New(itemId)
	user.Character.Validate(true)

	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Remove(name, user, room, 0)
	if err != nil || !handled {
		t.Fatalf("remove errored: handled=%v err=%v", handled, err)
	}
	if user.Character.Equipment.Neck.ItemId != 0 {
		t.Fatalf("fixture: %q is still worn (neck holds %d)", name,
			user.Character.Equipment.Neck.ItemId)
	}

	return events.DrainQueuedMessagesForTest(user.UserId)
}

func TestRemove_DisclosesTheReservationComingBack(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedDisclosureItems(t)()

	line := findReservationLine(removeAndCollect(t, 999960, "draining collar"))
	if line == `` {
		t.Fatal("taking off an item that reserved health, stamina and conviction " +
			"said nothing about the reservation coming back")
	}

	// The collar was all this character had, so the line collapses to the plain
	// statement rather than reciting the bottom rung once per pool. Which pools
	// get named on a PARTIAL release is covered in internal/characters; what
	// this test is for is that the command asks for the line at all.
	if !strings.Contains(line, "Nothing you carry holds any part of you in reserve now") {
		t.Errorf("a character left holding nothing must be told so plainly: %q", line)
	}
	if !strings.Contains(line, "gives part of you back") {
		t.Errorf("the remove line does not mirror the equip line's framing: %q", line)
	}
	if strings.ContainsAny(line, "0123456789") {
		t.Errorf("the remove line shows a raw number: %q", line)
	}
	if strings.Contains(line, "%") {
		t.Errorf("the remove line shows a percentage: %q", line)
	}
	if strings.ContainsAny(line, "–—") {
		t.Errorf("the remove line contains an en or em dash: %q", line)
	}
}

// The half that keeps it off every ordinary remove. Without this, appending the
// line unconditionally would still pass the test above.
func TestRemove_SaysNothingWhenNothingWasReserved(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedDisclosureItems(t)()

	if line := findReservationLine(removeAndCollect(t, 999961, "plain pendant")); line != `` {
		t.Errorf("an ordinary remove produced a reservation disclosure: %q", line)
	}
}
