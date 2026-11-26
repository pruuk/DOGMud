package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
)

// #781, owner rulings 1, 2 and 13: a secret exit is known once found, and
// KnowsSecretExit is the one rule every exit list and every mover asks. It
// reads the same discovery record `search` writes.
func TestKnowsSecretExit_OnlyTheExitFoundInTheRoomItWasFound(t *testing.T) {
	c := &Character{}
	if c.KnowsSecretExit(10, "west") {
		t.Fatal("a fresh character knows no secret exit")
	}
	c.LearnSecretExit(10, "west")
	if !c.KnowsSecretExit(10, "west") {
		t.Fatal("a learned secret exit must be known")
	}
	if !c.HasDiscovery(10, SecretExitDiscoveryKey("west")) {
		t.Error("LearnSecretExit must write the record search writes")
	}
	if c.KnowsSecretExit(10, "north") {
		t.Error("learning west must not reveal north")
	}
	if c.KnowsSecretExit(11, "west") {
		t.Error("learning west in room 10 must not reveal west in room 11")
	}
	c.LearnSecretExit(10, "west")
	if got := len(c.Discoveries[10]); got != 1 {
		t.Errorf("learning twice stored %d records, want 1", got)
	}
}

func TestKnowsSecretExit_NilCharacterKnowsNone(t *testing.T) {
	var c *Character
	if c.KnowsSecretExit(10, "west") {
		t.Error("a nil character knows no secret exit")
	}
}

// #781 Task 11b: a player's new find refreshes their map and buttons at once
// (one RoomExitsChanged marked as a find); a re-find and an NPC's find queue
// nothing.
func TestLearnSecretExit_QueuesARefreshOnlyForAPlayersNewFind(t *testing.T) {
	const userId = 9781
	events.DrainQueuedRoomExitsChangedForTest(0)
	t.Cleanup(func() { events.DrainQueuedRoomExitsChangedForTest(0) })

	player := &Character{}
	player.SetUserId(userId)
	player.LearnSecretExit(10, "west")
	got := events.DrainQueuedRoomExitsChangedForTest(userId)
	if len(got) != 1 || !got[0].SecretExitFound {
		t.Fatalf("a player's new find queues one refresh marked as a find, got %+v", got)
	}

	player.LearnSecretExit(10, "west")
	if got := events.DrainQueuedRoomExitsChangedForTest(userId); len(got) != 0 {
		t.Errorf("a re-find queues nothing, got %+v", got)
	}

	npc := &Character{MobInstanceId: 781}
	npc.LearnSecretExit(10, "west")
	if !npc.KnowsSecretExit(10, "west") {
		t.Fatal("the NPC's find is recorded")
	}
	if got := events.DrainQueuedRoomExitsChangedForTest(0); len(got) != 0 {
		t.Errorf("an NPC's find queues nothing, got %+v", got)
	}
}
