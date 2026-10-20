package auctions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 0, Task 15: the auction confirm is a two-entry menu on a
// non-modal prompt. The command runs, a typed line goes through
// RoutePromptInput the way world.go routes it, and the command runs again.

const buyoutQuestion = `Set a buyout (buy-it-now) price in gold?`

func auctionSeller(t *testing.T) (*AuctionsModule, *users.UserRecord) {
	t.Helper()
	old := auctionMinListValue
	t.Cleanup(func() { auctionMinListValue = old })
	auctionMinListValue = 0
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		1: {ItemId: 1, Name: "silver ring", NameSimple: "ring", Value: 500},
	}))
	seller := users.NewTestUser(9051, "seller5", "Seller5", 0)
	seller.Character.Items = []items.Item{items.New(1)}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{seller.UserId: seller}))
	t.Cleanup(seller.ClearPrompt)
	events.DrainQueuedMessagesForTest(seller.UserId)
	return &AuctionsModule{auctionMgr: AuctionManager{}}, seller
}

// lastAsked is the question the open prompt is waiting on, or "" when closed.
func lastAsked(u *users.UserRecord) string {
	p := u.GetPrompt()
	if p == nil || len(p.Questions) == 0 {
		return ""
	}
	return p.Questions[len(p.Questions)-1].Question
}

func TestConfirmMenu_Auction(t *testing.T) {
	for _, tc := range []struct {
		line      string
		continues bool
	}{
		{"1", true},
		{"y", true},
		{"", false},
	} {
		t.Run("line "+tc.line, func(t *testing.T) {
			mod, seller := auctionSeller(t)
			room := &rooms.Room{RoomId: 1, Zone: "TestZone"}
			run := func() { _, _ = mod.auctionCommand("silver ring", seller, room, 0) }

			run()
			out := strings.Join(events.DrainQueuedMessagesForTest(seller.UserId), "\n")
			assert.Contains(t, out, "Yes, auction it")

			_, had := seller.RoutePromptInput(tc.line, true)
			require.True(t, had)
			run()
			out = strings.Join(events.DrainQueuedMessagesForTest(seller.UserId), "\n")
			if tc.continues {
				assert.Equal(t, buyoutQuestion, lastAsked(seller), "yes moves on to the buyout price")
			} else {
				assert.Nil(t, seller.GetPrompt(), "Enter declines and closes")
				assert.Contains(t, out, "Aborting auction")
			}
			assert.Len(t, seller.Character.Items, 1, "nothing is listed yet")
		})
	}

	t.Run("a command closes it", func(t *testing.T) {
		mod, seller := auctionSeller(t)
		_, _ = mod.auctionCommand("silver ring", seller, &rooms.Room{RoomId: 1, Zone: "TestZone"}, 0)
		require.NotNil(t, seller.GetPrompt())
		next, had := seller.RoutePromptInput("north", true)
		assert.True(t, had)
		assert.Equal(t, "north", next, "the line runs as a command")
		assert.Nil(t, seller.GetPrompt())
	})
}
