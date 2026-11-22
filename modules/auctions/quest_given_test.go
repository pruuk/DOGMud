package auctions

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #940 (owner ruling 2026-11-22): quest gear never goes on the block. It fades
// as the seller lets it go, after the confirm and the price, where a bought
// copy is listed.
func TestQuestGear_Auction(t *testing.T) {
	for _, tc := range []struct {
		name  string
		quest bool
	}{
		{"quest gear fades", true},
		{"bought copy is listed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mod, seller := auctionSeller(t)
			mod.plug = &plugins.Plugin{} // its config reads as unset: the defaults
			seller.Character.Items[0].QuestGiven = tc.quest
			room := &rooms.Room{RoomId: 1, Zone: "TestZone"}
			run := func() { _, _ = mod.auctionCommand("silver ring", seller, room, 0) }

			run()
			for _, line := range []string{"1", "500"} {
				_, had := seller.RoutePromptInput(line, true)
				require.True(t, had)
				run()
			}
			out := strings.Join(events.DrainQueuedMessagesForTest(seller.UserId), "\n")

			assert.Empty(t, seller.Character.Items, "it leaves the seller either way")
			if tc.quest {
				assert.Contains(t, out, fmt.Sprintf(actions.QuestGearFadesLine, "Silver Ring"))
				assert.NotContains(t, out, "Auctioning your")
				assert.Nil(t, mod.auctionMgr.GetCurrentAuction(), "nothing is listed")
			} else {
				assert.Contains(t, out, "Auctioning your")
				require.NotNil(t, mod.auctionMgr.GetCurrentAuction())
			}
		})
	}
}
