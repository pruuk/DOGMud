package guards

import (
	"flag"
	"fmt"
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
	// auctionNpcBidder is a bid by the longest shipped mob name (29 letters,
	// mobs/), wider than a character name and split by its spaces.
	auctionNpcBidder = auctionFixture(coreHeart, noticeLongestName, false, 625, 1234567, "Riss the Cartographer-Scholar")

	// auctionFullLine is a bid whose line is one column wider than the 42
	// form's word rule allows, by a shipped NPC buyer (npc_buyers.go), so a
	// form that wraps a column too wide is caught.
	auctionFullLine = auctionFixture(coreHeart, "Screenly", false, 625, 12345678, "The Crown Assessor")

	// auctionFixtures are every auction fixture but the widest.
	auctionFixtures = []func() any{auctionCaptured, auctionWon, auctionAnonymous, auctionNpcBidder, auctionFullLine}
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

// bannerDrawings are the banners as the owner approved them at 42 and 30
// (narrow-screens-3b-mock/forms/banner_*.txt): rules the full width, the
// text wrapped at indent 1.
var bannerDrawings = map[string]map[int]string{
	"character/questup": {
		42: `
******************************************

 You have been given a new quest:
 Into the Vein!
 type quests to track your progress on
 quests.

******************************************`,
		30: `
******************************

 You have been given a new
 quest: Into the Vein!
 type quests to track your
 progress on quests.

******************************`,
	},
	"character/skillup": {
		42: `
******************************************
******************************************

 Your mastery of weapon-combat has grown —
 you are now novice!
 type help weapon-combat to learn more
 about using this skill.

******************************************
******************************************`,
		30: `
******************************
******************************

 Your mastery of weapon-combat
 has grown — you are now
 novice!
 type help weapon-combat to
 learn more about using this
 skill.

******************************
******************************`,
	},
	"character/statup": {
		42: `
******************************************
******************************************

 You feel notably stronger — your muscles
 carry a new permanence.

******************************************
******************************************`,
		30: `
******************************
******************************

 You feel notably stronger —
 your muscles carry a new
 permanence.

******************************
******************************`,
	},
}

var bannerCaptured = map[string]func() any{
	"character/questup": questupCaptured,
	"character/skillup": skillupCaptured,
	"character/statup":  statupCaptured,
}

func TestBannersAreTheDrawings(t *testing.T) {
	templateRenderSetup(t)
	for name, byWidth := range bannerDrawings {
		for width, want := range byWidth {
			got := strings.TrimSuffix(strings.Join(noticePlain(t, name, bannerCaptured[name](), width), "\n"), "\n")
			if got != want {
				t.Errorf("%s at %d is not the drawing:\n%s", name, width, lineDiff(want, got))
			}
		}
	}
}

// A banner's star rules span the screen (79 at 80 and wider, as a desktop
// always saw them), and no text is wider than its rules (ruling 1).
func TestBannerTextStaysInsideItsRules(t *testing.T) {
	templateRenderSetup(t)
	stars := regexp.MustCompile(`^\*+$`)
	for name, fixtures := range map[string][]func() any{
		"character/questup": {questupCaptured, questupWide},
		"character/skillup": {skillupCaptured, skillupWide},
		"character/statup":  {statupCaptured, statupLong, statupWide},
	} {
		for i, data := range fixtures {
			for _, width := range []int{30, 42, 60, 80, 120} {
				rule := min(width, 80)
				if rule == 80 {
					rule = 79
				}
				rules := 0
				for _, line := range noticePlain(t, name, data(), width) {
					w := templates.WidestLine(line)
					if stars.MatchString(line) {
						rules++
						if w != rule {
							t.Errorf("%s (data %d) at %d has a %d-star rule, want %d", name, i, width, w, rule)
						}
					} else if w > rule {
						t.Errorf("%s (data %d) at %d has a line %d wide inside %d-wide rules: %q", name, i, width, w, rule, line)
					}
				}
				if rules == 0 {
					t.Errorf("%s (data %d) at %d drew no rule", name, i, width)
				}
			}
		}
	}
}

var auctionTemplates = []string{"auctions/auction-bid", "auctions/auction-end", "auctions/auction-start", "auctions/auction-update"}

// An auction's star rule is its form's width (42, 30, or 79 at 80), its word
// rule alternates star and space and so is odd (41, 29, 79), and no text is
// wider than the word rule (ruling 1). Held both as a player's own `auction`
// sees it (measured) and as a broadcast sends it (picked by width alone,
// templates.ProcessForWidth, which a broadcast to a 60-column reader sends
// the 42 form).
func TestAuctionTextStaysInsideItsRules(t *testing.T) {
	templateRenderSetup(t)
	withAuctionTemplates(t)
	stars, word := regexp.MustCompile(`^\*+$`), regexp.MustCompile(`^\* \* \*`)
	for _, name := range auctionTemplates {
		for i, data := range append([]func() any{auctionWide}, auctionFixtures...) {
			for _, width := range []int{30, 35, 42, 60, 80, 120} {
				for _, picked := range []bool{false, true} {
					var out string
					var err error
					if picked {
						out, err = templates.ProcessForWidth(name, data(), false, width)
					} else {
						out, err = templates.ProcessAtWidth(name, data(), width)
					}
					if err != nil {
						t.Fatal(err)
					}
					plain := regexp.MustCompile(`</?ansi[^>]*>`).ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), "")
					lines := strings.Split(plain, "\n")
					starW, wordW := 0, 0
					for _, line := range lines {
						switch {
						case stars.MatchString(line):
							starW = templates.WidestLine(line)
						case word.MatchString(line):
							wordW = templates.WidestLine(line)
						}
					}
					where := fmt.Sprintf("%s (data %d) at %d (picked by width %v)", name, i, width, picked)
					if starW == 0 || wordW == 0 || starW > min(width, 80) || wordW%2 == 0 || (wordW != starW && wordW != starW-1) {
						t.Errorf("%s: star rule %d, word rule %d:\n%s", where, starW, wordW, plain)
						continue
					}
					for _, line := range lines {
						if !stars.MatchString(line) && templates.WidestLine(line) > wordW {
							t.Errorf("%s: a line %d wide inside a %d-wide word rule: %q", where, templates.WidestLine(line), wordW, line)
						}
					}
				}
			}
		}
	}
}

// The auction forms as drawn (narrow-screens-3b-mock/forms/auction_*.txt),
// with the description continuing 2 past its label (owner, 2026-11-11), the
// labels unpadded at 30, and the units kept whole: a count with its "!", a
// command, a name.
func TestAuctionFormsAsDrawn(t *testing.T) {
	templateRenderSetup(t)
	withAuctionTemplates(t)
	for _, tc := range []struct {
		name  string
		data  func() any
		width int
		want  []string
	}{
		{"auctions/auction-update", auctionCaptured, 42, []string{
			"\n******************************************\n* * * AUCTION * AUCTION * AUCTION * * * *\n\n  The auction will end in 171 seconds!\n\n",
			"\n  Owner:       Screenly\n  Item:        Core Guardian Heart\n  Description: Drawn from the deepest\n    of the guardians,",
			"\n\n  Highest Bid: none\n  Minimum Bid: 625 gold\n\n  bid (gold amount) to bid on this\n  auction.\n\n* * * AUCTION * AUCTION * AUCTION * * * *\n******************************************\n"}},
		{"auctions/auction-update", auctionCaptured, 30, []string{
			"\n******************************\n* * * AUCTION * AUCTION * * *\n\n The auction will end in\n 171 seconds!\n\n",
			"\n Owner: Screenly\n Item: Core Guardian Heart\n Description: Drawn from\n   the deepest",
			"\n\n Highest Bid: none\n Minimum Bid: 625 gold\n\n bid (gold amount) to bid on\n this auction.\n\n* * * AUCTION * AUCTION * * *\n******************************\n"}},
		{"auctions/auction-end", auctionWon, 42, []string{
			"\n******************************************\n* * * * AUCTION END * AUCTION END * * * *\n\n  The auction has ENDED!\n\n  Winner:      Master Ordwin\n  Bid:         625 gold\n\n  Item:        Core Guardian Heart\n",
			"\n\n  Type help auction to learn more about\n  auctions.\n\n* * * * AUCTION END * AUCTION END * * * *\n******************************************\n"}},
		{"auctions/auction-end", auctionWon, 30, []string{
			"\n******************************\n* * * * AUCTION END * * * * *\n\n The auction has ENDED!\n\n Winner: Master Ordwin\n Bid: 625 gold\n\n Item: Core Guardian Heart\n",
			"\n\n Type help auction to learn\n more about auctions.\n\n* * * * AUCTION END * * * * *\n******************************\n"}},
		{"auctions/auction-start", auctionCaptured, 42, []string{
			"\n  An auction has started!\n  The auction will end in 171 seconds!\n\n  Owner:       Screenly\n",
			"\n\n  Minimum Bid: 625 gold\n\n  bid (gold amount) to bid on this\n  auction.\n"}},
		{"auctions/auction-bid", auctionWon, 30, []string{
			"\n******************************\n* * * AUCTION * AUCTION * * *\n\n Master Ordwin has bid\n 625 gold\n\n* * * AUCTION * AUCTION * * *\n******************************\n"}},
		{"auctions/auction-bid", auctionWide, 42, []string{"\n  " + noticeLongestName + " has bid\n  1234567 gold\n"}},
		{"auctions/auction-bid", auctionWide, 30, []string{"\n " + noticeLongestName + "\n has bid 1234567 gold\n"}},
		{"auctions/auction-update", auctionWide, 30, []string{"\n Owner:\n " + noticeLongestName + "\n", "\n Highest Bid: 1234567 gold by\n " + noticeLongestName + "\n"}},
		{"auctions/auction-update", auctionWide, 42, []string{"\n  Owner:\n    " + noticeLongestName + "\n", "\n  Highest Bid: 1234567 gold by\n    " + noticeLongestName + "\n"}},
	} {
		plain := strings.Join(noticePlain(t, tc.name, tc.data(), tc.width), "\n")
		for _, want := range tc.want {
			if !strings.Contains(plain, want) {
				t.Errorf("%s at %d lacks %q:\n%s", tc.name, tc.width, want, plain)
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
