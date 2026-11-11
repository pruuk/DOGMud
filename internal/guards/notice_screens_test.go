package guards

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
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/language"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/modules/auctions"
	textlang "golang.org/x/text/language"
)

// The notice screens with narrow forms (plan 3b-3): a mail message (the inbox
// and the compose preview), the quest, skill and stat banners, and the four
// auction broadcasts. Each fixture is as wide as the screen gets in play
// (names read from the shipped YAML), or is the screen as the 2026-11-10
// capture showed it. The goldens prove a desktop sees at 80 columns what
// master drew, apart from the changes the owner approved in the mock (mail's
// NOTE lines wrapped under their value).

var updateNoticeScreens = flag.Bool("update-notice-screens", false,
	"rewrite internal/guards/testdata/notice_screens_80/*.golden from the current templates")

// noticeLongestName is a character name at the configured maximum
// (NameSizeMax, 28 from plan 3b-3).
const noticeLongestName = "Bartholomewsworthingtonianus"

// noticeLongestItem is the longest shipped item name (items/, 39 letters).
const noticeLongestItem = "A Salt-Stiff Page of a Sailor's Telling"

// noticeItem is an item whose spec is its own, so no item registry is read.
// A wide one is cursed and enchanted with bonus damage, so NameComplex adds
// its flags and its bonus.
func noticeItem(name, desc string, wide bool) items.Item {
	itm := items.Item{ItemId: 99300, Spec: &items.ItemSpec{ItemId: 99300, Name: name, Description: desc, Type: items.Weapon}}
	if wide {
		itm.Spec.Cursed = true
		itm.Spec.Damage.BonusDamage = 12
		itm.Enchantments = 1
	}
	return itm
}

// osPathFS reads a folder by the OS-separated paths templates asks for
// (util.FilePath), which os.DirFS alone refuses on Windows.
type osPathFS struct{ fs.FS }

func (f osPathFS) Open(name string) (fs.File, error) { return f.FS.Open(filepath.ToSlash(name)) }
func (f osPathFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(f.FS, filepath.ToSlash(name))
}

// withAuctionTemplates reads the auctions module's templates as the server
// does (the module's embedded files are registered beside the world's).
func withAuctionTemplates(t *testing.T) {
	t.Helper()
	templates.SetFSForTest(t, osPathFS{os.DirFS(dogmudDataFilesRoot)},
		osPathFS{os.DirFS(filepath.Join("modules", "auctions", "files", "datafiles"))})
}

// --- mail/message ---

const mailCapturedText = "The bank on the north side keeps odd hours this week, so bring the strongbox key before the bell and do not let the clerk talk you into storage fees."

// mailFixture is a message as the inbox and the compose preview pass it.
func mailFixture(from, text string, gold int, item *items.Item, read bool, sent time.Time) func() any {
	return func() any {
		return users.Message{FromName: from, Message: text, Gold: gold, Item: item, Read: read, DateSent: sent}
	}
}

var (
	// A Tuesday and a Wednesday (the longest day name) at a two-digit hour.
	mailTuesday   = time.Date(2026, 11, 10, 12, 51, 27, 0, time.Local)
	mailWednesday = time.Date(2026, 11, 11, 12, 51, 27, 0, time.Local)

	hullPlate = noticeItem("Hull-Plate Cuirass", "", false)
	pageWide  = noticeItem(noticeLongestItem, "", true)

	// mailCaptured is the message the 2026-11-10 capture read.
	mailCaptured = mailFixture("Screenly", mailCapturedText, 120, &hullPlate, false, mailTuesday)
	// mailPlain is a short message with nothing attached.
	mailPlain = mailFixture("Screenly", "Meet me at the bank at noon.", 0, nil, false, mailTuesday)
	// mailRead is a read message that wraps at 80.
	mailRead = mailFixture("Mailbuddy", mailCapturedText, 0, nil, true, mailWednesday)
	// mailLongWord is a message holding a word wider than a narrow line.
	mailLongWord = mailFixture(noticeLongestName, "Read it here: https://dogmud.org/a/very/long/link/to/the/far/marsh/map and tell me.", 0, nil, false, mailWednesday)
	// mailWide is every field at its widest: the longest name, day, gold
	// and item.
	mailWide = mailFixture(noticeLongestName, mailCapturedText+" "+mailCapturedText, 1234567, &pageWide, false, mailWednesday)
)

// --- character/questup, skillup, statup ---

type skillUpData struct {
	SkillName  string
	SkillLevel int
}

func noticeData(v any) func() any { return func() any { return v } }

var (
	questupCaptured = noticeData(`You have been given a new quest: <ansi fg="questname">Into the Vein</ansi>!`)
	// The longest shipped quest name (quests/, 31 letters) in the longest
	// banner sentence.
	questupWide     = noticeData(`You've made progress on the quest: <ansi fg="questname">The City Watch's Missing Person</ansi>!`)
	skillupCaptured = noticeData(skillUpData{"weapon-combat", 1})
	// The longest skill name and the longest rank word.
	skillupWide    = noticeData(skillUpData{"unarmed-combat", 70})
	statupCaptured = noticeData("You feel notably stronger — your muscles carry a new permanence.")
	statupLong     = noticeData("Your mind firms, a quiet resolve that wasn't there before now anchoring you.")
	// The longest stat message (statGainDescriptions, 79 letters): one
	// column wider than the 79-star rule at 80 with its indent.
	statupWide = noticeData("The world comes into slightly sharper focus, details you once missed now clear.")
)

// --- auctions ---

const auctionCapturedDesc = "Drawn from the deepest of the guardians, the one that stood over the place where all the buried quiet gathers. It is a fist of grey material folded around a single mote of trapped brightness, and it is never wholly still -- the light within drifts and settles like something breathing in its sleep. Hold it and you feel a slow pulse answer in your own wrist, patient and enormous. Whatever it once guarded, it guards its last warmth now. A relic beyond price, fit only for work that will outlast the one who makes it."

// auctionFixture is an auction as the module passes it. It ends 171 seconds
// (and a half, so a slow render still reads 171) from when it is built.
func auctionFixture(item items.Item, seller string, anon bool, minBid, bid int, bidder string) func() any {
	return func() any {
		return &auctions.AuctionItem{ItemData: item, SellerName: seller, Anonymous: anon,
			EndTime: time.Now().Add(171500 * time.Millisecond), MinimumBid: minBid,
			HighestBid: bid, HighestBidderName: bidder}
	}
}

var (
	coreHeart = noticeItem("Core Guardian Heart", auctionCapturedDesc, false)
	// auctionCaptured is the lot the capture showed, before any bid.
	auctionCaptured = auctionFixture(coreHeart, "Screenly", false, 625, 0, "")
	// auctionWon is the captured lot won by an NPC buyer.
	auctionWon = auctionFixture(coreHeart, "Screenly", false, 625, 625, "Master Ordwin")
	// auctionWide is every field at its widest: the longest item name with
	// its flags and bonus, the longest names, and bids in the millions.
	auctionWide = auctionFixture(noticeItem(noticeLongestItem, auctionCapturedDesc, true),
		noticeLongestName, false, 1234567, 1234567, noticeLongestName)
	// auctionAnonymous hides both names.
	auctionAnonymous = auctionFixture(coreHeart, "Screenly", true, 625, 1234567, noticeLongestName)
)

// noticeScreenGoldens are the screens whose 80-column output is pinned. All
// but the ones marked were recorded from master before plan 3b-3.
var noticeScreenGoldens = map[string]struct {
	name    string
	fixture templateFixture
}{
	"mail_plain":              {"mail/message", templateFixture{data: mailPlain}},
	"mail_read":               {"mail/message", templateFixture{data: mailRead}},
	"mail_longword":           {"mail/message", templateFixture{data: mailLongWord}},
	"questup_captured":        {"character/questup", templateFixture{data: questupCaptured}},
	"questup_wide":            {"character/questup", templateFixture{data: questupWide}},
	"skillup_captured":        {"character/skillup", templateFixture{data: skillupCaptured}},
	"skillup_wide":            {"character/skillup", templateFixture{data: skillupWide}},
	"statup_captured":         {"character/statup", templateFixture{data: statupCaptured}},
	"statup_long":             {"character/statup", templateFixture{data: statupLong}},
	"auction-bid_wide":        {"auctions/auction-bid", templateFixture{setup: withAuctionTemplates, data: auctionWide}},
	"auction-bid_anon":        {"auctions/auction-bid", templateFixture{setup: withAuctionTemplates, data: auctionAnonymous}},
	"auction-end_captured":    {"auctions/auction-end", templateFixture{setup: withAuctionTemplates, data: auctionWon}},
	"auction-end_unsold":      {"auctions/auction-end", templateFixture{setup: withAuctionTemplates, data: auctionCaptured}},
	"auction-end_wide":        {"auctions/auction-end", templateFixture{setup: withAuctionTemplates, data: auctionWide}},
	"auction-start_captured":  {"auctions/auction-start", templateFixture{setup: withAuctionTemplates, data: auctionCaptured}},
	"auction-start_wide":      {"auctions/auction-start", templateFixture{setup: withAuctionTemplates, data: auctionWide}},
	"auction-update_captured": {"auctions/auction-update", templateFixture{setup: withAuctionTemplates, data: auctionCaptured}},
	"auction-update_wide":     {"auctions/auction-update", templateFixture{setup: withAuctionTemplates, data: auctionWide}},
	"auction-update_anon":     {"auctions/auction-update", templateFixture{setup: withAuctionTemplates, data: auctionAnonymous}},
}

// noticePlain renders name at width and returns its lines with colour tags
// removed.
func noticePlain(t *testing.T, name string, data any, width int) []string {
	t.Helper()
	out, err := templates.ProcessAtWidth(name, data, width)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(regexp.MustCompile(`</?ansi[^>]*>`).ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), ""), "\n")
}

// mailDrawn80 is the captured message as the owner approved it at 80
// (narrow-screens-3b-mock/forms/mail_read.w80.txt): the NOTE lines wrap
// under their value, and nothing else changes.
var mailDrawn80 = []string{
	"         *NEW MESSAGE*",
	"Sent:    Tuesday, 10-Nov-2026 12:51:27PM",
	"From:    Screenly",
	"",
	"Message: The bank on the north side keeps odd hours this week, so bring the",
	"         strongbox key before the bell and do not let the clerk talk you into",
	"         storage fees.",
	"",
	"NOTE:    This message had 120 gold attached, which was added to your bank",
	"         balance.",
	"NOTE:    This message came with one Hull-Plate Cuirass attached, which was added",
	"         to your inventory.",
}

func TestMailAt80IsTheDrawing(t *testing.T) {
	templateRenderSetup(t)
	got := strings.Join(noticePlain(t, "mail/message", mailCaptured(), 80), "\n")
	if want := "\n" + strings.Join(mailDrawn80, "\n"); got != want {
		t.Errorf("mail at 80 is not the drawing:\n%s", lineDiff(want, got))
	}
}

// Units stay whole at every width (ruling 3): the sender, the date-time, the
// attached item. At 30 the labels drop their padding, the date-time goes
// under the value, and prose after a label continues 2 past it (ruling 2).
func TestMailKeepsItsUnitsWhole(t *testing.T) {
	templateRenderSetup(t)
	for _, width := range templateWidths {
		for _, tc := range []struct {
			data  func() any
			units []string
		}{
			{mailCaptured, []string{"10-Nov-2026 12:51:27PM", "Screenly", "Hull-Plate Cuirass"}},
			{mailWide, []string{"11-Nov-2026 12:51:27PM", noticeLongestName}},
		} {
			lines := noticePlain(t, "mail/message", tc.data(), width)
			plain := strings.Join(lines, "\n")
			for _, unit := range tc.units {
				if !strings.Contains(plain, unit) {
					t.Errorf("at %d %q is split:\n%s", width, unit, plain)
				}
			}
		}
	}
	at30 := strings.Join(noticePlain(t, "mail/message", mailCaptured(), 30), "\n")
	for _, want := range []string{"\nSent: Tuesday,\n      10-Nov-2026 12:51:27PM\n", "\nFrom: Screenly\n",
		"\nNOTE: This message had\n  120 gold attached,", "\nMessage: The bank on the\n  north side"} {
		if !strings.Contains(at30, want) {
			t.Errorf("at 30 the mail lacks %q:\n%s", want, at30)
		}
	}
	at42 := strings.Join(noticePlain(t, "mail/message", mailCaptured(), 42), "\n")
	for _, want := range []string{"\nSent:    Tuesday, 10-Nov-2026 12:51:27PM\n", "\nNOTE:    This message had 120 gold\n  attached,"} {
		if !strings.Contains(at42, want) {
			t.Errorf("at 42 the mail lacks %q:\n%s", want, at42)
		}
	}
}

// Another language's labels keep their own padding at 42 and 80 and drop it
// at 30, and the screen still fits (the Chinese labels are wider).
func TestMailFitsInChinese(t *testing.T) {
	templateRenderSetup(t)
	setLanguage := func(lang textlang.Tag, code string) {
		language.InitTranslation(language.BundleCfg{
			DefaultLanguage: textlang.English, Language: lang,
			LanguagePaths: []string{"_datafiles/localize", "_datafiles/world/dogmud/localize"},
		})
		if err := configs.AddOverlayOverrides(map[string]any{"Translation.Language": code}); err != nil {
			t.Fatal(err)
		}
	}
	setLanguage(textlang.Chinese, "zh")
	t.Cleanup(func() { setLanguage(textlang.English, "en") })
	for _, data := range []func() any{mailCaptured, mailWide} {
		for _, width := range templateWidths {
			lines := noticePlain(t, "mail/message", data(), width)
			plain := strings.Join(lines, "\n")
			if !strings.Contains(plain, "发件人:") {
				t.Fatalf("the Chinese labels did not load:\n%s", plain)
			}
			if w := templates.WidestLine(plain); w > width {
				t.Errorf("in Chinese at %d a line is %d wide:\n%s", width, w, plain)
			}
			padded := strings.Contains(plain, "发件人:   ")
			if padded != (width > 30) {
				t.Errorf("in Chinese at %d the From label padded is %v:\n%s", width, padded, plain)
			}
		}
	}
}

func noticeScreenGoldenPath(key string) string {
	return testdataPath(filepath.Join("notice_screens_80", key+".golden"))
}

// A desktop sees what it always did: at 80 columns each screen renders
// exactly the bytes in its golden, line ends aside.
func TestNoticeScreensKeepTheir80ColumnOutput(t *testing.T) {
	templateRenderSetup(t)
	for key, g := range noticeScreenGoldens {
		t.Run(key, func(t *testing.T) {
			if g.fixture.setup != nil {
				g.fixture.setup(t)
			}
			out, err := templates.ProcessAtWidth(g.name, g.fixture.data(), 80)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.ReplaceAll(out, "\r\n", "\n")
			path := noticeScreenGoldenPath(key)
			if *updateNoticeScreens {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (record it from master with -update-notice-screens)", err)
			}
			if w := templates.WidestLine(string(want)); w > 80 {
				t.Fatalf("the golden is %d wide: its fixture no longer fits 80, so it pins a form, not the base", w)
			}
			if got != string(want) {
				t.Errorf("%s at 80 changed:\n%s", key, lineDiff(string(want), got))
			}
		})
	}
}
