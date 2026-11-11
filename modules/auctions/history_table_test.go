package auctions

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// `auction history` (tables/generic) at 30, 42 and 80 columns (plan 3b-3,
// ruling 6): its two-line form leads with the Item, and where its bordered
// form fits 80 it is pinned to what master drew.

var updateAuctionTables = flag.Bool("update-auction-tables", false,
	"rewrite modules/auctions/testdata/*.golden from the current output")

// auctionHistoryAt runs `auction history` over past at width.
func auctionHistoryAt(t *testing.T, width int, past ...PastAuctionItem) string {
	t.Helper()
	root := filepath.Join("..", "..", "_datafiles", "world", "dogmud")
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(root)
	configs.SetConfigForTest(t, cfg)
	templates.SetFSForTest(t, os.DirFS(root).(fs.ReadFileFS))
	u := users.NewTestUser(9501, "reader", "Reader", 9501)
	u.LineWidth = width
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{u.UserId: u}))
	mod := &AuctionsModule{auctionMgr: AuctionManager{PastAuctions: past}}
	events.DrainQueuedMessagesForTest(u.UserId)
	_, err := mod.auctionCommand("history", u, nil, 0)
	require.NoError(t, err)
	return strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "")
}

var regexpTags = regexp.MustCompile(`</?ansi[^>]*>`)

var auctionEnded = time.Date(2026, 11, 10, 18, 30, 0, 0, time.UTC)

// auctionHistoryFits is a history whose bordered table fits 80.
func auctionHistoryFits() []PastAuctionItem {
	return []PastAuctionItem{
		{ItemName: "a lantern", WinningBid: 40, SellerName: "Screenly", BuyerName: "Halix", EndTime: auctionEnded},
		{ItemName: "a rope", WinningBid: 12, Anonymous: true, SellerName: "Screenly", BuyerName: "Halix", EndTime: auctionEnded},
	}
}

func checkAuctionGolden(t *testing.T, name, got string) {
	t.Helper()
	got = strings.ReplaceAll(got, "\r\n", "\n")
	path := filepath.Join("testdata", name+".golden")
	if *updateAuctionTables {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "record the golden from master with -update-auction-tables")
	require.Equal(t, string(want), got, "%s changed", name)
}

// auctionHistoryWide is a history as wide as play makes it: the longest
// shipped item name, a legacy 32-letter seller, the longest NPC buyer.
func auctionHistoryWide() []PastAuctionItem {
	return []PastAuctionItem{
		{ItemName: "A Salt-Stiff Page of a Sailor's Telling", WinningBid: 12400, SellerName: "Wilhelmina_Ashgrove_Thornbramble", BuyerName: "The Merchants' Guild", EndTime: auctionEnded},
		{ItemName: "a lantern", WinningBid: 40, SellerName: "Screenly", BuyerName: "Bartholomewsworthingtonianus", EndTime: auctionEnded},
	}
}

// At 30 and at 80 (its plain columns are wider than 80) the history takes
// its two-line form: each item is headed by the Item, and the detail lists
// the Date, Seller, Buyer and Winning Bid, every line within the width but
// a legacy 32-letter name alone on its line (ruling 8).
func TestAuctionHistoryLeadsWithTheItem(t *testing.T) {
	for _, width := range []int{30, 42, 80} {
		out := strings.ReplaceAll(auctionHistoryAt(t, width, auctionHistoryWide()...), "\r\n", "\n")
		plain := regexpTags.ReplaceAllString(out, "")
		lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
		for _, line := range lines {
			if templates.WidestLine(line) > width && strings.TrimSpace(line) != "Wilhelmina_Ashgrove_Thornbramble" {
				t.Errorf("at %d a line is %d wide: %q in:\n%s", width, templates.WidestLine(line), line, plain)
			}
			// The " ·" between pairs stays on its pair's last word, so
			// no line starts with one (the 28-letter Buyer once left a
			// bare "·" at 30).
			if strings.HasPrefix(strings.TrimSpace(line), "·") {
				t.Errorf("at %d a line starts with a dot: %q in:\n%s", width, line, plain)
			}
		}
		require.NotContains(t, plain, "│", "at %d the history is not the two-line form:\n%s", width, plain)
		// Newest first, as the bordered form lists them.
		leads, details := []string{}, []string{}
		for _, line := range lines[1:] {
			switch {
			case line == "":
			case strings.HasPrefix(line, "  "):
				details[len(details)-1] += " " + strings.TrimSpace(line)
			case len(details) > 0 && details[len(details)-1] == "":
				leads[len(leads)-1] += " " + line
			default:
				leads = append(leads, line)
				details = append(details, "")
			}
		}
		require.Equal(t, []string{"a lantern", "A Salt-Stiff Page of a Sailor's Telling"}, leads, "at %d:\n%s", width, plain)
		for _, detail := range details {
			for _, want := range []string{"Date: 2026-11-10 18:30:00", "Seller: ", "Buyer: ", "Winning Bid: "} {
				require.Contains(t, detail, want, "at %d", width)
			}
		}
	}
}

// The bordered history at 80, and its plain-column form one column
// narrower, are what master drew.
func TestAuctionHistoryKeepsItsMasterOutput(t *testing.T) {
	at80 := auctionHistoryAt(t, 80, auctionHistoryFits()...)
	require.Contains(t, at80, "╒")
	checkAuctionGolden(t, "auction_history_w80", at80)
	plain := auctionHistoryAt(t, templates.WidestLine(at80)-1, auctionHistoryFits()...)
	require.NotContains(t, plain, "╒")
	checkAuctionGolden(t, "auction_history_w40form", plain)
}
