package acquire

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #696: an item that already exists for the receiver (made, bought, earned)
// and is too much to carry falls at their feet, and a player is told so. It
// used to vanish: the crafted dagger, after its materials were spent.
func TestAcquireOrDrop_TooMuchToCarryFallsAtTheirFeet(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	room := &rooms.Room{RoomId: 6961}
	defer rooms.SeedRoomsForTest(map[int]*rooms.Room{6961: room}, nil)()

	char := newWeakChar()
	char.RoomId = 6961
	user := &users.UserRecord{UserId: 6961, Character: char}
	defer users.SeedUsersForTest(map[int]*users.UserRecord{6961: user})()

	require.False(t, AcquireItemOrDrop(user, newHeavyTestItem(69601), AcquireCraft), "not carried")
	require.Equal(t, 0, countCharItems(char), "the player does not hold it")
	require.Len(t, room.Items, 1, "it lies on the floor")
	require.Equal(t, 69601, room.Items[0].ItemId)

	msgs := events.DrainQueuedMessagesForTest(6961)
	require.Len(t, msgs, 1)
	require.True(t, strings.Contains(msgs[0], "falls at your feet"), "%q", msgs[0])
	require.Empty(t, events.DrainQueuedItemOwnershipForTest(6961), "no gain for an item never held")
}

// An item the receiver can carry is carried, and nothing falls.
func TestAcquireOrDrop_CarriedWhenItFits(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	room := &rooms.Room{RoomId: 6962}
	defer rooms.SeedRoomsForTest(map[int]*rooms.Room{6962: room}, nil)()

	char := characters.New()
	char.RoomId = 6962
	require.True(t, AcquireOrDrop(char, 6962, 0, newTestItem(69602), AcquireForage))
	require.Equal(t, 1, countCharItems(char))
	require.Empty(t, room.Items)
	require.Empty(t, events.DrainQueuedMessagesForTest(6962))
}

// A creature's item falls the same way; there is no player to tell.
func TestAcquireOrDrop_ACreatureDropsItToo(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	room := &rooms.Room{RoomId: 6963}
	defer rooms.SeedRoomsForTest(map[int]*rooms.Room{6963: room}, nil)()

	char := newWeakChar()
	char.RoomId = 6963
	require.False(t, AcquireOrDrop(char, 0, 42, newHeavyTestItem(69603), AcquireCraft))
	require.Len(t, room.Items, 1)
}
