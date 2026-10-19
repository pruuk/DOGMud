package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
)

// #617: a player reads a corpse once. Assess trains Manifestation, so a corpse
// that answered every cooldown was a progression farm. The cooldown is
// cleared between the two calls so the second one is refused by the
// assessed-by mark and not by the cooldown.
func TestAssess_SameCorpseIsReadOncePerPlayer(t *testing.T) {
	u, room, cleanup := seedAssessFixture(t, 4000)
	defer cleanup()

	if _, err := Assess("goblin", u, room, events.CmdSecretly); err != nil {
		t.Fatalf("first assess: %v", err)
	}
	first := strings.Join(events.DrainQueuedMessagesForTest(adUserId), "\n")
	if !strings.Contains(first, "You sense") {
		t.Fatalf("fixture: the first assess gave no reading:\n%s", first)
	}

	u.Character.Cooldowns = nil
	if _, err := Assess("goblin", u, room, events.CmdSecretly); err != nil {
		t.Fatalf("second assess: %v", err)
	}
	second := strings.Join(events.DrainQueuedMessagesForTest(adUserId), "\n")
	if strings.Contains(second, "You sense") {
		t.Errorf("a repeat assess of the same corpse gave a second reading "+
			"(and a second Manifestation award):\n%s", second)
	}
	if !strings.Contains(second, "already studied these remains") {
		t.Errorf("a repeat assess must say why it is refused:\n%s", second)
	}
	if !room.Corpses[0].WasAssessedBy(u.UserId) {
		t.Error("the assessed-by mark did not land on the room's corpse")
	}
}
