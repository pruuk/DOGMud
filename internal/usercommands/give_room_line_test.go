package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The room's line names the item by its simple noun ("a pin"), and the
// proper_name flag describes the full name, not the noun: an onlooker still
// reads "gives Bobrick a pin", never "gives Bobrick pin" (#799 review).
func TestGive_RoomLineKeepsItsArticleForAProperNameItem(t *testing.T) {
	const pinId = 91901
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		pinId: {ItemId: pinId, Name: "Soren's Iron Pin", NameSimple: "pin", ProperName: true, Type: items.Object, Weight: 0.1, Value: 1},
	}))

	giver, room := getTestUserAndRoom(t)
	receiver := users.GetByUserId(2)
	onlooker := users.NewTestUser(3, "carol", "Carolyn", 1003)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: giver, 2: receiver, 3: onlooker}))
	room.AddPlayer(3)

	giver.Character.Items = nil
	require.True(t, giver.Character.StoreItem(items.New(pinId)))
	events.DrainQueuedMessagesForTest(3)

	handled, err := Give("pin bobrick", giver, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	out := strings.Join(events.DrainQueuedMessagesForTest(3), "\n")
	require.Contains(t, out, `gives`, "the onlooker saw the give line")
	require.Contains(t, out, `a <ansi fg="itemname">pin</ansi>`, out)
}
