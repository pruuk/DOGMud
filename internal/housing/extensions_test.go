package housing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func offerMap(u *users.UserRecord) map[string]Offer {
	out := map[string]Offer{}
	for _, o := range Offers(u, testBldgId, `simple`) {
		out[o.Key] = o
	}
	return out
}

func buyKey(u *users.UserRecord, key string) []string {
	said := []string{}
	Buy(u, func(s string) { said = append(said, s) }, testBldgId, `simple`, key)
	return said
}

// unitRoom builds a unit room the way its template does: one door back out.
func unitRoom(id int) *rooms.Room {
	return &rooms.Room{
		RoomId:      id,
		Title:       `A Bare Lodging Room`,
		Description: `Four whitewashed walls.`,
		Exits:       map[string]exit.RoomExit{`door`: {RoomId: testDoor}},
		Nouns:       map[string]string{`door`: `The inside of the green door.`},
	}
}

func deedIn(u *users.UserRecord) (items.Item, bool) {
	for _, itm := range u.Character.Items {
		if itm.ItemId == testDeedId {
			return itm, true
		}
	}
	return items.Item{}, false
}

// ── The list ───────────────────────────────────────────────────────────────

func TestOffers_BeforeAndAfterAHome(t *testing.T) {
	setup(t)
	u := testUser(1, 10000, 0)

	o := offerMap(u)
	if !o[OfferHome].Available || o[OfferHome].Price != 500 {
		t.Errorf("home = %+v, want available at 500", o[OfferHome])
	}
	if o[OfferExtension].Available || o[OfferRedecorate].Available {
		t.Error("extensions and vouchers need a home first")
	}

	buy(t, u)
	o = offerMap(u)
	if o[OfferHome].Available {
		t.Error("an owner is not offered a second home")
	}
	if !o[OfferExtension].Available || o[OfferExtension].Price != 1500 {
		t.Errorf("deed = %+v, want 1500 (3 x 500)", o[OfferExtension])
	}
	if !o[OfferRedecorate].Available || o[OfferRedecorate].Price != 500 {
		t.Errorf("voucher = %+v, want 500", o[OfferRedecorate])
	}
}

func TestOffers_HomeShowsStandingGate(t *testing.T) {
	setup(t)
	restore := SetRepTierForTest(func(string, int) opinions.Tier { return opinions.TierNeutral })
	defer restore()
	home := offerMap(testUser(1, 10000, 0))[OfferHome]
	if home.Available || !strings.Contains(home.Note, `vouch`) {
		t.Errorf("home = %+v, want unavailable with a standing note", home)
	}
}

func TestMatchOffer(t *testing.T) {
	cases := []struct {
		req  string
		owns bool
		want string
		ok   bool
	}{
		{`home`, false, OfferHome, true},
		{`a room`, false, OfferHome, true},
		{`a room`, true, OfferExtension, true},
		{`deed`, false, OfferExtension, true},
		{`2 deeds from hobb`, true, OfferExtension, true},
		{`extension`, true, OfferExtension, true},
		{`voucher`, true, OfferRedecorate, true},
		{`Redecorating Voucher`, true, OfferRedecorate, true},
		{`iron ingot`, true, ``, false},
	}
	for _, c := range cases {
		got, ok := MatchOffer(c.req, c.owns)
		if got != c.want || ok != c.ok {
			t.Errorf("MatchOffer(%q, %v) = %q, %v; want %q, %v", c.req, c.owns, got, ok, c.want, c.ok)
		}
	}
}

// ── Escalating extension price ─────────────────────────────────────────────

// 500 for the home makes the first deed 1500; 500 + 1500 makes the second
// 6000; 8000 paid makes the third 24000. Buying a deed raises the price even
// if it is never used, so deeds cannot be stockpiled at the cheap price.
func TestExtensionPriceEscalates(t *testing.T) {
	setup(t)
	u := testUser(1, 100000, 0)
	buy(t, u)

	where := []string{`north`, `east`}
	for i, want := range []int{1500, 6000} {
		if got := offerMap(u)[OfferExtension].Price; got != want {
			t.Fatalf("next deed = %d, want %d", got, want)
		}
		before := u.Character.Gold
		buyKey(u, OfferExtension)
		if paid := before - u.Character.Gold; paid != want {
			t.Fatalf("charged %d, want %d", paid, want)
		}
		deed, ok := deedIn(u)
		if !ok || deed.BoundUserId != 1 {
			t.Fatalf("deed = %+v, %v; want one bound to 1", deed, ok)
		}
		h, _ := HouseOf(1, testBldgId)
		r := unitRoom(h.RoomIds[len(h.RoomIds)-1])
		ApplyOverlay(r)
		UseItem(u, r, deed, where[i], `deed `+where[i])
	}
	h, _ := HouseOf(1, testBldgId)
	if h.RoomsPaid != 500+1500+6000 || len(h.RoomIds) != 3 || h.DeedsIssued != 2 {
		t.Errorf("house = paid %d, rooms %v, deeds %d", h.RoomsPaid, h.RoomIds, h.DeedsIssued)
	}
	if o := offerMap(u)[OfferExtension]; o.Available {
		t.Errorf("still offered at max_rooms: %+v", o)
	}
}

// A deed is a promise of one room. No second deed is sold while one is
// unused, a lost one is replaced free, and only one of the copies is
// honoured.
func TestExtensionDeeds_OneUnusedAtATime(t *testing.T) {
	setup(t)
	u := testUser(1, 100000, 0)
	buy(t, u)
	buyKey(u, OfferExtension)
	gold := u.Character.Gold

	said := buyKey(u, OfferExtension)
	if u.Character.Gold != gold || len(u.Character.Items) != 1 {
		t.Fatalf("a second deed was sold while one is unused: %q", said)
	}
	if o := offerMap(u)[OfferExtension]; o.Available || o.Price != 0 {
		t.Errorf("offered while one is unused: %+v", o)
	}

	// Lost (put in storage, say): a free replacement.
	stored, _ := deedIn(u)
	u.Character.RemoveItem(stored)
	buyKey(u, OfferExtension)
	fresh, ok := deedIn(u)
	if !ok || u.Character.Gold != gold {
		t.Fatalf("no free replacement: gold %d, deed %v", u.Character.Gold, ok)
	}
	h, _ := HouseOf(1, testBldgId)
	if h.DeedsIssued != 1 || h.RoomsPaid != 2000 {
		t.Errorf("the replacement changed the ledger: %+v", h)
	}

	// Either copy works once; the other is waste paper after.
	entry := unitRoom(testUnitA)
	ApplyOverlay(entry)
	UseItem(u, entry, fresh, `north`, `deed north`)
	u.Character.StoreItem(stored)
	UseItem(u, entry, stored, `east`, `deed east`)
	if h, _ := HouseOf(1, testBldgId); len(h.RoomIds) != 2 {
		t.Errorf("two rooms from one paid deed: %v", h.RoomIds)
	}
}

// Every unused deed in the building holds a vacant unit, so a deed is never
// sold that could not be used.
func TestExtensionDeeds_NeverSoldWithoutAUnitForThem(t *testing.T) {
	setup(t)
	a, b := testUser(1, 100000, 0), testUser(2, 100000, 0)
	buy(t, a)
	buy(t, b) // units: A, B owned; C vacant
	buyKey(a, OfferExtension)
	gold := b.Character.Gold
	buyKey(b, OfferExtension)
	if b.Character.Gold != gold {
		t.Error("sold a deed with the last vacant unit already promised")
	}
	if o := offerMap(b)[OfferExtension]; o.Available {
		t.Errorf("offered with no unit to spare: %+v", o)
	}
}

func TestNormalizeDeeds_LegacyHouses(t *testing.T) {
	b := testBuilding()
	// Paid 500 + 1500 + 6000 for two deeds, used one of them.
	h := House{TierId: `simple`, RoomIds: []int{testUnitA, testUnitB}, PricePaid: 500, RoomsPaid: 8000}
	h.normalizeDeeds(b)
	if h.DeedsIssued != 2 || h.Outstanding(b) != 1 {
		t.Errorf("deeds %d outstanding %d, want 2 and 1", h.DeedsIssued, h.Outstanding(b))
	}
	// Rooms with no spend recorded at all still count as used deeds.
	h = House{TierId: `simple`, RoomIds: []int{testUnitA, testUnitB}, PricePaid: 500}
	h.normalizeDeeds(b)
	if h.DeedsIssued != 1 || h.Outstanding(b) != 0 {
		t.Errorf("deeds %d outstanding %d, want 1 and 0", h.DeedsIssued, h.Outstanding(b))
	}
}

// A house bought before extensions existed has no rooms_paid; its home price
// stands in for it.
func TestExtensionPrice_LegacyHouse(t *testing.T) {
	setup(t)
	saveHouse(House{BuildingId: testBldgId, OwnerUserId: 1, TierId: `simple`, RoomIds: []int{testUnitA}, PricePaid: 500})
	loadHouses()
	if got := offerMap(testUser(1, 0, 0))[OfferExtension].Price; got != 1500 {
		t.Errorf("deed = %d, want 1500", got)
	}
}

func TestBuyExtension_RefusalsChargeNothing(t *testing.T) {
	setup(t)

	stranger := testUser(1, 5000, 0)
	buyKey(stranger, OfferExtension)
	if stranger.Character.Gold != 5000 || len(stranger.Character.Items) != 0 {
		t.Error("a player with no home was sold a deed")
	}

	poor := testUser(2, 600, 0)
	buy(t, poor) // 100 left
	buyKey(poor, OfferExtension)
	if poor.Character.Gold != 100 {
		t.Error("charged without enough gold")
	}
	if _, has := deedIn(poor); has {
		t.Error("handed a deed without payment")
	}
	if h, _ := HouseOf(2, testBldgId); h.Spent() != 500 {
		t.Errorf("room spend moved on a refusal: %d", h.Spent())
	}
}

func TestBuyExtension_WriteFailureChargesNothing(t *testing.T) {
	dir := setup(t)
	u := testUser(1, 5000, 0)
	buy(t, u)
	// Make the house file unwritable by replacing its folder with a file.
	bdir := filepath.Join(dir, testBldgId)
	os.RemoveAll(bdir)
	os.WriteFile(bdir, []byte(`x`), 0644)

	buyKey(u, OfferExtension)
	if u.Character.Gold != 4500 {
		t.Errorf("gold = %d, want 4500", u.Character.Gold)
	}
	if _, has := deedIn(u); has {
		t.Error("deed handed over despite failed write")
	}
}

func TestBuyRedecorate(t *testing.T) {
	setup(t)
	u := testUser(1, 1200, 0)
	buyKey(u, OfferRedecorate)
	if u.Character.Gold != 1200 {
		t.Error("sold a voucher to someone with no home")
	}
	buy(t, u)
	buyKey(u, OfferRedecorate)
	if u.Character.Gold != 200 {
		t.Errorf("gold = %d, want 200", u.Character.Gold)
	}
	found := false
	for _, itm := range u.Character.Items {
		found = found || itm.ItemId == testVoucherId
	}
	if !found {
		t.Error("no voucher in the backpack")
	}
}

// ── Placing a room ─────────────────────────────────────────────────────────

func ownerWithDeed(t *testing.T) (*users.UserRecord, items.Item) {
	t.Helper()
	u := testUser(1, 100000, 0)
	buy(t, u)
	buyKey(u, OfferExtension)
	deed, ok := deedIn(u)
	if !ok {
		t.Fatal("setup: no deed")
	}
	return u, deed
}

func TestUseDeed_AddsARoomInThatDirection(t *testing.T) {
	setup(t)
	u, deed := ownerWithDeed(t)
	entry := unitRoom(testUnitA)

	if !UseItem(u, entry, deed, `north`, `deed north`) {
		t.Fatal("deed not handled")
	}
	h, _ := HouseOf(1, testBldgId)
	if len(h.RoomIds) != 2 {
		t.Fatalf("rooms = %v, want two", h.RoomIds)
	}
	added := h.RoomIds[1]
	if h.exitsOf(testUnitA)[`north`] != added || h.exitsOf(added)[`south`] != testUnitA {
		t.Errorf("doorways = %+v", h.Links)
	}
	if _, still := deedIn(u); still {
		t.Error("the deed should be used up")
	}
	if entry.Exits[`north`].RoomId != added {
		t.Error("the live room did not get its new exit")
	}
	if got := h.offsets()[added]; got != [3]int{0, -1, 0} {
		t.Errorf("new room placed at %v, want one step north", got)
	}
	if ok, _, _ := GuardEntry(1, added); !ok {
		t.Error("owner cannot enter their new room")
	}
	if ok, _, _ := GuardEntry(2, added); ok {
		t.Error("a stranger can enter the new room")
	}
}

func TestUseDeed_Refusals(t *testing.T) {
	setup(t)
	u, deed := ownerWithDeed(t)
	entry := unitRoom(testUnitA)

	// A bad direction, not in your house, and a deed made out to someone else
	// all leave the deed unused.
	UseItem(u, entry, deed, `sideways`, `deed sideways`)
	UseItem(u, unitRoom(testUnitC), deed, `north`, `deed north`)
	other := deed
	other.BoundUserId = 99
	UseItem(u, entry, other, `north`, `deed north`)
	if h, _ := HouseOf(1, testBldgId); len(h.RoomIds) != 1 {
		t.Fatalf("a refused deed added a room: %v", h.RoomIds)
	}
	if _, still := deedIn(u); !still {
		t.Fatal("a refused deed was used up")
	}

	// A wall that already has a way through.
	entry.Exits[`east`] = exit.RoomExit{RoomId: 1}
	UseItem(u, entry, deed, `east`, `deed east`)
	if h, _ := HouseOf(1, testBldgId); len(h.RoomIds) != 1 {
		t.Error("built through a wall that already has an exit")
	}
}

// North, then east, then south brings you beside the entry: building west
// from there would land on the entry room itself, so that wall is refused
// and the menu does not offer it.
func TestUseDeed_WallBackingOntoOwnRoom(t *testing.T) {
	setup(t)
	b := testBuilding()
	b.MaxRooms = 8
	b.UnitRooms = []int{6470, 6471, 6472, 6473, 6474, 6475}
	ResetForTest()
	AddBuildingForTest(b)

	u := testUser(1, 1000000, 0)
	buy(t, u)
	here := unitRoom(testUnitA)
	for _, dir := range []string{`north`, `east`, `south`} {
		buyKey(u, OfferExtension)
		deed, _ := deedIn(u)
		UseItem(u, here, deed, dir, `deed `+dir)
		h, _ := HouseOf(1, testBldgId)
		next := unitRoom(h.RoomIds[len(h.RoomIds)-1])
		ApplyOverlay(next)
		here = next
	}
	h, _ := HouseOf(1, testBldgId)
	if got := h.offsets()[here.RoomId]; got != [3]int{1, 0, 0} {
		t.Fatalf("third room at %v, want beside the entry", got)
	}
	for _, dir := range freeWalls(here, h) {
		if dir == `west` {
			t.Error("the menu offers a wall that backs onto the entry")
		}
	}
	buyKey(u, OfferExtension)
	deed, _ := deedIn(u)
	UseItem(u, here, deed, `west`, `deed west`)
	if h2, _ := HouseOf(1, testBldgId); len(h2.RoomIds) != len(h.RoomIds) {
		t.Error("built a room on top of the entry")
	}
}

func TestUseDeed_MenuWhenNoDirection(t *testing.T) {
	setup(t)
	u, deed := ownerWithDeed(t)
	entry := unitRoom(testUnitA)

	UseItem(u, entry, deed, ``, `deed`)
	p := u.GetPrompt()
	if p == nil || p.GetNextQuestion() == nil {
		t.Fatal("no menu was offered")
	}
	q := p.GetNextQuestion()
	if strings.Join(q.Options, `,`) != `north,east,south,west,up,down,cancel` {
		t.Errorf("options = %v", q.Options)
	}

	q.Answer(`e`) // prefix of east
	UseItem(u, entry, deed, ``, `deed`)
	h, _ := HouseOf(1, testBldgId)
	if len(h.RoomIds) != 2 || h.exitsOf(testUnitA)[`east`] == 0 {
		t.Fatalf("menu choice did not build east: %+v", h.Links)
	}
	if u.GetPrompt() != nil {
		t.Error("the menu was not cleared")
	}
}

func TestUseDeed_MenuCancel(t *testing.T) {
	setup(t)
	u, deed := ownerWithDeed(t)
	entry := unitRoom(testUnitA)
	UseItem(u, entry, deed, ``, `deed`)
	u.GetPrompt().GetNextQuestion().Answer(`cancel`)
	UseItem(u, entry, deed, ``, `deed`)
	if h, _ := HouseOf(1, testBldgId); len(h.RoomIds) != 1 {
		t.Error("cancel built a room")
	}
	if _, still := deedIn(u); !still {
		t.Error("cancel used up the deed")
	}
}

func TestUseDeed_RespectsMaxRooms(t *testing.T) {
	setup(t) // test building allows 3 rooms
	u := testUser(1, 1000000, 0)
	buy(t, u)
	entry := unitRoom(testUnitA)
	for _, dir := range []string{`north`, `south`} {
		buyKey(u, OfferExtension)
		deed, _ := deedIn(u)
		UseItem(u, entry, deed, dir, `deed `+dir)
	}
	if o := offerMap(u)[OfferExtension]; o.Available {
		t.Error("a deed is still offered at the room cap")
	}
}

// ── Redecorating ───────────────────────────────────────────────────────────

func ownerWithVoucher(t *testing.T) (*users.UserRecord, items.Item) {
	t.Helper()
	u := testUser(1, 10000, 0)
	buy(t, u)
	buyKey(u, OfferRedecorate)
	for _, itm := range u.Character.Items {
		if itm.ItemId == testVoucherId {
			return u, itm
		}
	}
	t.Fatal("setup: no voucher")
	return nil, items.Item{}
}

const newDesc = `A small, warm room with a braided rug and a shelf of chipped blue cups.`

func TestUseVoucher_InlineThenConfirm(t *testing.T) {
	setup(t)
	u, voucher := ownerWithVoucher(t)
	entry := unitRoom(testUnitA)

	UseItem(u, entry, voucher, newDesc, `voucher `+newDesc)
	q := u.GetPrompt().GetNextQuestion()
	if q == nil {
		t.Fatal("no confirmation asked")
	}
	q.Answer(`yes`)
	UseItem(u, entry, voucher, newDesc, `voucher `+newDesc)

	h, _ := HouseOf(1, testBldgId)
	if h.Descriptions[testUnitA] != newDesc {
		t.Fatalf("description = %q", h.Descriptions[testUnitA])
	}
	if entry.Description != newDesc {
		t.Error("the live room did not change")
	}
	for _, itm := range u.Character.Items {
		if itm.ItemId == testVoucherId {
			t.Error("the voucher should be used up")
		}
	}
}

func TestUseVoucher_PromptsForTextAndOverwrites(t *testing.T) {
	setup(t)
	u, voucher := ownerWithVoucher(t)
	entry := unitRoom(testUnitA)

	UseItem(u, entry, voucher, ``, `voucher`)
	u.GetPrompt().GetNextQuestion().Answer(`First try at a description, long enough to count.`)
	UseItem(u, entry, voucher, ``, `voucher`)
	u.GetPrompt().GetNextQuestion().Answer(`y`)
	UseItem(u, entry, voucher, ``, `voucher`)

	buyKey(u, OfferRedecorate)
	var second items.Item
	for _, itm := range u.Character.Items {
		if itm.ItemId == testVoucherId {
			second = itm
		}
	}
	UseItem(u, entry, second, newDesc, `voucher `+newDesc)
	u.GetPrompt().GetNextQuestion().Answer(`yes`)
	UseItem(u, entry, second, newDesc, `voucher `+newDesc)

	h, _ := HouseOf(1, testBldgId)
	if h.Descriptions[testUnitA] != newDesc {
		t.Errorf("second voucher did not overwrite: %q", h.Descriptions[testUnitA])
	}
}

func TestUseVoucher_RefusesBadTextAndKeepsVoucher(t *testing.T) {
	setup(t)
	u, voucher := ownerWithVoucher(t)
	entry := unitRoom(testUnitA)

	UseItem(u, entry, voucher, `too short`, `voucher too short`)
	UseItem(u, entry, voucher, strings.Repeat(`a`, DescriptionMaxLen+1), `voucher long`)
	if h, _ := HouseOf(1, testBldgId); len(h.Descriptions) != 0 {
		t.Error("a bad description was saved")
	}
	found := false
	for _, itm := range u.Character.Items {
		found = found || itm.ItemId == testVoucherId
	}
	if !found {
		t.Error("a refused voucher was used up")
	}
}

func TestCleanDescription(t *testing.T) {
	got := cleanDescription("  A room\nwith <ansi fg=\"red\">red</ansi>   walls. ")
	if strings.Contains(got, "\n") || strings.Contains(got, "  ") || strings.Contains(got, `<ansi`) {
		t.Errorf("cleanDescription = %q", got)
	}
}

// ── Overlay and persistence ────────────────────────────────────────────────

func TestOverlay_EntryAndExtension(t *testing.T) {
	setup(t)
	mu.Lock()
	unitCoords[testUnitA] = [4]int{5, 0, 0, 14}
	mu.Unlock()

	h := House{BuildingId: testBldgId, OwnerUserId: 1, TierId: `simple`,
		RoomIds:      []int{testUnitA, testUnitB},
		Links:        []RoomLink{{From: testUnitA, Direction: `up`, To: testUnitB}},
		Descriptions: map[int]string{testUnitA: newDesc}}
	saveHouse(h)
	loadHouses()

	entry, ext := unitRoom(testUnitA), unitRoom(testUnitB)
	ApplyOverlay(entry)
	ApplyOverlay(ext)
	ApplyOverlay(ext) // idempotent

	if entry.Exits[`door`].RoomId != testDoor || entry.Exits[`up`].RoomId != testUnitB {
		t.Errorf("entry exits = %+v", entry.Exits)
	}
	if entry.Description != newDesc {
		t.Error("entry lost its owner description")
	}
	if _, has := ext.Exits[`door`]; has {
		t.Error("an extension kept a door to the street")
	}
	if _, has := ext.Nouns[`door`]; has {
		t.Error("an extension kept the door noun")
	}
	if ext.Exits[`down`].RoomId != testUnitA || len(ext.Exits) != 1 {
		t.Errorf("extension exits = %+v", ext.Exits)
	}
	if ext.Title != `A Bare Back Room` || !strings.Contains(ext.Description, `knocked through`) {
		t.Errorf("extension text = %q / %q", ext.Title, ext.Description)
	}
	if ext.X != 5 || ext.Y != 0 || ext.Z != 1 || ext.Plane != 14 {
		t.Errorf("extension at %d,%d,%d plane %d; want 5,0,1 plane 14", ext.X, ext.Y, ext.Z, ext.Plane)
	}

	vacant := unitRoom(testUnitC)
	ApplyOverlay(vacant)
	if vacant.Title != `A Bare Lodging Room` || len(vacant.Exits) != 1 {
		t.Error("a vacant unit was changed")
	}
}

func TestLoad_BadLinksAreHeld(t *testing.T) {
	setup(t)
	saveHouse(House{BuildingId: testBldgId, OwnerUserId: 1, TierId: `simple`,
		RoomIds: []int{testUnitA, testUnitB},
		Links: []RoomLink{
			{From: testUnitA, Direction: `north`, To: testUnitB},
			{From: testUnitA, Direction: `north`, To: testUnitB},
		}})
	if n, _ := loadHouses(); n != 0 {
		t.Fatal("a house with two north doorways was loaded")
	}
	if _, isHeld := HeldRooms()[testUnitB]; !isHeld {
		t.Error("its rooms should be held")
	}
}

func TestLoad_RoundTripWithLinksAndDescriptions(t *testing.T) {
	setup(t)
	u, deed := ownerWithDeed(t)
	UseItem(u, unitRoom(testUnitA), deed, `west`, `deed west`)
	want, _ := HouseOf(1, testBldgId)

	ResetForTest()
	AddBuildingForTest(testBuilding())
	if n, _ := loadHouses(); n != 1 {
		t.Fatal("house did not reload")
	}
	got, _ := HouseOf(1, testBldgId)
	if len(got.Links) != 1 || got.Links[0] != want.Links[0] || got.Spent() != want.Spent() {
		t.Errorf("reloaded %+v, want %+v", got, want)
	}
}
