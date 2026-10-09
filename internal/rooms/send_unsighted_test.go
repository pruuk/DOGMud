package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// SendTextUnsighted is the sound half of an event that is both seen and
// heard (#242, owner ruling R4): it reaches only a player who cannot make out
// even shapes. A shapes reader gets the visual line instead, and an excluded
// player (the actor) reads its own line.
func TestSendTextUnsighted_ReachesOnlyThoseWhoSeeNothing(t *testing.T) {
	r := sightTestRoom(t, "cave")
	if !users.GetByUserId(7412).Character.Conditions.AddCondition(sightTestInfraredConditionId, true) {
		t.Fatal("precondition: Bobrick should now carry infrared")
	}

	r.SendTextUnsighted(messaging.CategorySpellDisruption, messaging.SoundChantBreaksOff, 7411)

	if got := events.DrainQueuedMessagesForTest(7411); len(got) != 0 {
		t.Fatalf("an excluded player got %q", got)
	}
	if got := events.DrainQueuedMessagesForTest(7412); len(got) != 0 {
		t.Fatalf("a shapes reader gets the visual line, not the sound: got %q", got)
	}
	got := sightTestPlain(events.DrainQueuedMessagesForTest(7413))
	if len(got) != 1 || got[0] != messaging.SoundChantBreaksOff {
		t.Fatalf("a reader who sees nothing read %q, want %q", got, messaging.SoundChantBreaksOff)
	}
}

func TestSendTextUnsighted_LitRoomIsSilent(t *testing.T) {
	r := sightTestRoom(t, "city")
	r.SendTextUnsighted(messaging.CategorySpellDisruption, messaging.SoundChantBreaksOff)
	for _, id := range []int{7411, 7412, 7413} {
		if got := events.DrainQueuedMessagesForTest(id); len(got) != 0 {
			t.Fatalf("user %d sees the room and still got the sound: %q", id, got)
		}
	}
}
