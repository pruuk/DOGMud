package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// TestShopVerbsRefuseAnOffShiftKeeper holds the owner ruling of 2026-10-17:
// an off-shift shopkeeper does not trade. In the playtest Blacksmith Kerra,
// whose schedule takes her from the forge to the tavern of an evening,
// answered list, buy, sell and offer there with her full stock. Each shop
// verb must answer instead with the keeper's own off-shift line, through the
// one closed-shop gate, and trade nothing.
func TestShopVerbsRefuseAnOffShiftKeeper(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	instId, merchantCleanup := seedMerchantInRoom(t, user, 1000)
	defer merchantCleanup()

	const line = `Not here. Come by the forge tomorrow.`
	mobs.RegisterScheduleForTest(&mobs.Schedule{Id: "test_offshift_smith", Segments: []mobs.ScheduleSegment{
		// Every hour sends the keeper to another room: off shift all day.
		{Start: 0, End: 24, TargetRoom: room.RoomId + 7000, OffShiftLine: line},
	}})
	defer mobs.UnregisterScheduleForTest("test_offshift_smith")
	m := mobs.GetInstance(instId)
	require.NotNil(t, m)
	m.HomeRoomId = room.RoomId
	m.ScheduleId = "test_offshift_smith"
	require.True(t, m.OffShift(), "the fixture keeper must be off shift")

	require.True(t, user.Character.StoreItem(items.New(testSellItemId)))
	startGold := user.Character.Gold

	for _, v := range []struct {
		name string
		run  func() (bool, error)
	}{
		{"list", func() (bool, error) { return List("", user, room, 0) }},
		{"buy", func() (bool, error) { return Buy("iron sword", user, room, 0) }},
		{"sell", func() (bool, error) { return Sell("iron sword", user, room, 0) }},
		{"offer", func() (bool, error) { return Offer("iron sword", user, room, 0) }},
		{"appraise", func() (bool, error) { return Appraise("iron sword", user, room, 0) }},
	} {
		msgs, stop := captureAllMessages(t)
		handled, err := v.run()
		stop()
		require.True(t, handled, v.name)
		require.NoError(t, err, v.name)
		var mine []string
		for _, msg := range *msgs {
			if msg.UserId == user.UserId {
				mine = append(mine, msg.Text)
			}
		}
		all := strings.Join(mine, "")
		require.Contains(t, all, `Merchant</ansi> says, "`+line+`"`, "%s must refuse with the keeper's off-shift line, got %q", v.name, all)
		require.NotContains(t, all, "gold", "%s must quote or trade nothing, got %q", v.name, all)
	}
	require.Equal(t, startGold, user.Character.Gold, "no gold may change hands")
	require.Equal(t, 1, countSellItems(user.Character, testSellItemId), "the item must stay in the pack")
}
