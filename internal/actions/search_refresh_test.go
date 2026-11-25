package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
)

// #781 Task 11b: a `search` that finds a secret exit records it through the
// one recorder (Character.LearnSecretExit), so the player's map and buttons
// refresh at once: one RoomExitsChanged marked as a find. Searching again
// reports the exit but queues nothing, and an NPC's find queues nothing.
func TestSearch_ANewSecretExitFindRefreshesThePlayerAtOnce(t *testing.T) {
	const userId = 9782
	pinSearchRoll(t, true)
	events.DrainQueuedRoomExitsChangedForTest(0)
	t.Cleanup(func() { events.DrainQueuedRoomExitsChangedForTest(0) })

	room := newSecretExitRoom(9782)
	actor := newSearchFakeActor("Finder", room, true, userId)
	actor.char.SetUserId(userId)

	if got := Search(actor, SearchOptions{}); len(got.HiddenExitsFound) != 1 {
		t.Fatalf("the pinned roll finds the exit: %+v", got)
	}
	got := events.DrainQueuedRoomExitsChangedForTest(userId)
	if len(got) != 1 || !got[0].SecretExitFound {
		t.Fatalf("a new find queues one refresh marked as a find, got %+v", got)
	}

	actor.char.Cooldowns = nil // a second search must actually run
	if again := Search(actor, SearchOptions{}); len(again.HiddenExitsFound) != 1 {
		t.Fatalf("a re-search still reports the exit: %+v", again)
	}
	if got := events.DrainQueuedRoomExitsChangedForTest(0); len(got) != 0 {
		t.Errorf("a re-find queues nothing, got %+v", got)
	}

	mob := newSearchMobActor("Prowler", newSecretExitRoom(9783), 7783)
	mob.char.Stats.Perception.ValueAdj = 100
	if got := Search(mob, SearchOptions{}); len(got.HiddenExitsFound) != 1 {
		t.Fatalf("the pinned roll finds the exit for the NPC too: %+v", got)
	}
	if got := events.DrainQueuedRoomExitsChangedForTest(0); len(got) != 0 {
		t.Errorf("an NPC's find queues nothing, got %+v", got)
	}
}
