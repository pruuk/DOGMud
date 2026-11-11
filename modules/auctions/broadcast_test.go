package auctions

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// broadcastReader is a telnet reader whose line is width columns wide.
func broadcastReader(id int, width int, auctionsOn bool) *users.UserRecord {
	u := users.NewTestUser(id, "reader", "Reader", uint64(id))
	u.LineWidth = width
	if !auctionsOn {
		u.SetConfigOption(`auction`, false)
	}
	return u
}

// An auction broadcast renders once per distinct (screen reader, width)
// among its readers, by width alone, and each reader gets the text for its
// own width; a reader who turned auctions off causes no render and gets
// nothing (spec ruling 11, plan 3b-3). The templates are read from the
// module's embedded files through the plugin registry, as the server reads
// them, so the forms are proven to ship in the embed.
func TestAuctionBroadcastRendersOncePerView(t *testing.T) {
	templates.SetFSForTest(t, plugins.GetPluginRegistry())
	lot := &AuctionItem{
		ItemData:   items.Item{ItemId: 99301, Spec: &items.ItemSpec{ItemId: 99301, Name: "Core Guardian Heart", Description: "A fist of grey material."}},
		SellerName: "Screenly", MinimumBid: 625, HighestBid: 700, HighestBidderName: "Master Ordwin",
		EndTime: time.Now().Add(time.Minute),
	}
	narrowA, narrowB := broadcastReader(9401, 30, true), broadcastReader(9402, 30, true)
	wide := broadcastReader(9403, 80, true)
	off := broadcastReader(9404, 42, false)
	readers := []*users.UserRecord{narrowA, wide, off, narrowB}

	for _, fname := range []string{"auctions/auction-bid", "auctions/auction-end", "auctions/auction-start", "auctions/auction-update"} {
		t.Run(fname, func(t *testing.T) {
			events.DrainAllQueuedEventsForTest()
			renders := map[[2]int]int{}
			prev := renderAuction
			renderAuction = func(name string, data any, screenReader bool, width int) (string, error) {
				sr := 0
				if screenReader {
					sr = 1
				}
				renders[[2]int{sr, width}]++
				return prev(name, data, screenReader, width)
			}
			t.Cleanup(func() { renderAuction = prev })

			broadcastAuction(fname, lot, readers)

			assert.Equal(t, map[[2]int]int{{0, 30}: 1, {0, 80}: 1}, renders, "one render per view, none for the reader who turned auctions off")
			for _, tc := range []struct {
				u     *users.UserRecord
				width int
				rule  string
			}{
				{narrowA, 30, strings.Repeat("*", 30)},
				{narrowB, 30, strings.Repeat("*", 30)},
				{wide, 80, strings.Repeat("*", 79)},
			} {
				got := strings.Join(events.DrainQueuedMessagesForTest(tc.u.UserId), "")
				lines := strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n")
				hasRule := false
				for _, line := range lines {
					plain := templates.WidestLine(line)
					assert.LessOrEqual(t, plain, tc.width, "reader %d: %q", tc.u.UserId, line)
					if strings.Contains(line, tc.rule) && !strings.Contains(line, tc.rule+"*") {
						hasRule = true
					}
				}
				assert.True(t, hasRule, "reader %d at %d did not get its own width's text:\n%s", tc.u.UserId, tc.width, got)
			}
			assert.Empty(t, events.DrainQueuedMessagesForTest(off.UserId), "auctions off")
		})
	}
}
