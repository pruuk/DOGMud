package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The perception helpers answer "who must not read a line about this
// character" once, for hooks and actions alike (messaging M6 slice 4).
func TestUnseenBy_ListsOnlyPlayersWhoDoNotPerceiveAHiddenCharacter(t *testing.T) {
	seedParityWorld(t)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7371: users.NewTestUser(7371, "ana", "Ana", 97371),
		7372: users.NewTestUser(7372, "bo", "Bo", 97372),
	}))
	// hiddenMobRoom (search_stealth_test.go) builds room 7370 holding a
	// hidden mob, instance 98371.
	room := hiddenMobRoom(t, 7370, 98371, 50)
	for _, id := range []int{7371, 7372} {
		room.AddPlayer(id)
		events.DrainQueuedMessagesForTest(id)
	}
	hidden := mobs.GetInstance(98371)
	if hidden == nil {
		t.Fatal("fixture mob missing")
	}
	if got := UnseenBy(room, &hidden.Character); len(got) != 2 {
		t.Fatalf("neither player perceives the hidden mob, want both listed, got %v", got)
	}
	hidden.Character.Awareness = nil // IsHidden reads the Awareness FSM: nil is visible
	if got := UnseenBy(room, &hidden.Character); got != nil {
		t.Fatalf("a visible character is unseen by nobody, got %v", got)
	}
}
