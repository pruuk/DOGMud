package housing

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	testDoor   = 5625
	testUnitA  = 6470
	testUnitB  = 6471
	testUnitC  = 6472
	testBldgId = `test_lodgings`

	testDeedId    = 55
	testVoucherId = 56
	testKeyId     = 57
	testBoxId     = 58
	testSafeId    = 59
)

func testBuilding() Building {
	return Building{
		BuildingId:    testBldgId,
		Name:          `the test lodgings`,
		DoorRoom:      testDoor,
		DoorExit:      `door`,
		LandlordMobId: 9801,
		Faction:       `np_commonfolk`,
		MinRepTier:    `warm`,
		Proprietor:    `the Widow`,
		VouchedBy:     `the Common Quarter`,
		StandingHint:  `Do some good round the Common Quarter and come back.`,
		Location:      `Pennock's Alley`,
		OutsideText:   `You find yourself back out in the alley.`,
		Tiers:         []Tier{{TierId: `simple`, Name: `a simple room`, Price: 500, Rooms: 1}},
		UnitRooms:     []int{testUnitA, testUnitB, testUnitC},

		ExtensionItemId:          testDeedId,
		ExtensionPriceMultiplier: 3,
		MaxRooms:                 3,
		ExtensionTitle:           `A Bare Back Room`,
		ExtensionDescription:     `A plain room knocked through from the rest of the lodging.`,
		RedecorateItemId:         testVoucherId,
		RedecoratePrice:          500,
		GuestKeyItemId:           testKeyId,
		GuestKeyPrice:            100,
		MaxGuests:                2,
		ContainerItemId:          testBoxId,
		ContainerPrice:           250,
		StrongboxItemId:          testSafeId,
		StrongboxPrice:           500,
		MaxContainersPerRoom:     2,
	}
}

// setup gives each test a clean registry, a temp living-state dir, one
// building, a standing lookup that says "warm", and no mob specs.
func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	restoreDir := SetDataDirForTest(dir)
	restoreRep := SetRepTierForTest(func(string, int) opinions.Tier { return opinions.TierWarm })
	prevName := landlordName
	landlordName = func(Building) string { return `Hobb Pennock` }
	prevStaff := isStaff
	isStaff = func(int) bool { return false }
	prevSave := saveUser
	saveUser = func(*users.UserRecord) {}
	ResetForTest()
	AddBuildingForTest(testBuilding())
	// Weightless, because a test character has no strength to carry with.
	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		testDeedId:    {ItemId: testDeedId, Name: `Room Extension Deed`, NameSimple: `deed`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		testVoucherId: {ItemId: testVoucherId, Name: `Redecorating Voucher`, NameSimple: `voucher`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		testKeyId:     {ItemId: testKeyId, Name: `Guest Key`, NameSimple: `guest key`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		testBoxId:     {ItemId: testBoxId, Name: `Container Deed`, NameSimple: `container deed`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		testSafeId:    {ItemId: testSafeId, Name: `Strongbox Deed`, NameSimple: `strongbox deed`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		// The second building's (two_buildings_test.go).
		60: {ItemId: 60, Name: `Quillhouse Extension Deed`, NameSimple: `deed`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		61: {ItemId: 61, Name: `Quillhouse Redecorating Voucher`, NameSimple: `voucher`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		62: {ItemId: 62, Name: `Quillhouse Guest Key`, NameSimple: `guest key`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		63: {ItemId: 63, Name: `Quillhouse Container Deed`, NameSimple: `container deed`, Type: items.Object, Subtype: items.Usable, Uses: 1},
		64: {ItemId: 64, Name: `Quillhouse Strongbox Deed`, NameSimple: `strongbox deed`, Type: items.Object, Subtype: items.Usable, Uses: 1},
	})
	t.Cleanup(func() {
		restoreItems()
		restoreDir()
		restoreRep()
		landlordName = prevName
		isStaff = prevStaff
		saveUser = prevSave
		ResetForTest()
	})
	return dir
}

func testUser(userId int, gold, bank int) *users.UserRecord {
	return &users.UserRecord{
		UserId:    userId,
		Character: &characters.Character{Name: `Tester`, Gold: gold, Bank: bank},
	}
}

func buy(t *testing.T, u *users.UserRecord) (PurchaseResult, []string) {
	t.Helper()
	said := []string{}
	res := Purchase(u, func(s string) { said = append(said, s) }, testBldgId, `simple`)
	return res, said
}

// ── Building validation ────────────────────────────────────────────────────

func TestBuildingValidate(t *testing.T) {
	good := testBuilding()
	if err := good.Validate(); err != nil {
		t.Fatalf("good building: %v", err)
	}

	cases := map[string]func(*Building){
		`no id`:          func(b *Building) { b.BuildingId = `` },
		`no door exit`:   func(b *Building) { b.DoorExit = `` },
		`bad rep tier`:   func(b *Building) { b.MinRepTier = `beloved` },
		`no tiers`:       func(b *Building) { b.Tiers = nil },
		`free tier`:      func(b *Building) { b.Tiers[0].Price = 0 },
		`zero rooms`:     func(b *Building) { b.Tiers[0].Rooms = 0 },
		`two-room tier`:  func(b *Building) { b.Tiers[0].Rooms = 2 },
		`semicolon said`: func(b *Building) { b.StandingHint = `Help out; then come back.` },
		`no units`:       func(b *Building) { b.UnitRooms = nil },
		`duplicate unit`: func(b *Building) { b.UnitRooms = []int{1, 1} },
		`door is a unit`: func(b *Building) { b.UnitRooms = []int{testDoor} },
	}
	for name, mutate := range cases {
		b := testBuilding()
		b.Tiers = append([]Tier{}, b.Tiers...)
		mutate(&b)
		if err := b.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

// ── Purchase ───────────────────────────────────────────────────────────────

func TestPurchase_Succeeds_PersistsBeforePublishing(t *testing.T) {
	dir := setup(t)
	u := testUser(7, 600, 0)

	res, _ := buy(t, u)
	if res != PurchaseOk {
		t.Fatalf("got %v, want PurchaseOk", res)
	}
	if u.Character.Gold != 100 || u.Character.Bank != 0 {
		t.Errorf("gold/bank = %d/%d, want 100/0", u.Character.Gold, u.Character.Bank)
	}
	h, ok := HouseOf(7, testBldgId)
	if !ok || h.EntryRoom() != testUnitA {
		t.Fatalf("house = %+v, %v; want entry %d", h, ok, testUnitA)
	}
	if _, err := os.Stat(filepath.Join(dir, testBldgId, `6470.yaml`)); err != nil {
		t.Fatalf("house file not written: %v", err)
	}
}

func TestPurchase_TakesCarriedGoldFirstThenBank(t *testing.T) {
	setup(t)
	u := testUser(7, 200, 1000)
	if res, _ := buy(t, u); res != PurchaseOk {
		t.Fatalf("got %v", res)
	}
	if u.Character.Gold != 0 || u.Character.Bank != 700 {
		t.Errorf("gold/bank = %d/%d, want 0/700", u.Character.Gold, u.Character.Bank)
	}
}

func TestPurchase_RefusesLowStanding(t *testing.T) {
	setup(t)
	restore := SetRepTierForTest(func(string, int) opinions.Tier { return opinions.TierNeutral })
	defer restore()

	u := testUser(7, 600, 0)
	res, said := buy(t, u)
	if res != PurchaseRepTooLow {
		t.Fatalf("got %v, want PurchaseRepTooLow", res)
	}
	if u.Character.Gold != 600 {
		t.Errorf("gold changed on refusal: %d", u.Character.Gold)
	}
	if len(said) == 0 || !strings.Contains(said[0], `vouch`) {
		t.Errorf("landlord should explain standing, said %q", said)
	}
	if _, ok := HouseOf(7, testBldgId); ok {
		t.Error("no house should exist")
	}
}

func TestPurchase_RefusesWithoutGold(t *testing.T) {
	setup(t)
	u := testUser(7, 300, 199)
	if res, _ := buy(t, u); res != PurchaseNoGold {
		t.Fatalf("got %v, want PurchaseNoGold", res)
	}
	if u.Character.Gold != 300 || u.Character.Bank != 199 {
		t.Error("gold changed on refusal")
	}
}

func TestPurchase_OnePerAccountPerBuilding(t *testing.T) {
	setup(t)
	u := testUser(7, 2000, 0)
	buy(t, u)
	if res, _ := buy(t, u); res != PurchaseAlreadyOwner {
		t.Fatalf("got %v, want PurchaseAlreadyOwner", res)
	}
	if u.Character.Gold != 1500 {
		t.Errorf("second attempt charged: gold %d", u.Character.Gold)
	}
}

func TestPurchase_EachBuyerGetsTheirOwnRoom_UntilFull(t *testing.T) {
	setup(t)
	seen := map[int]bool{}
	for id := 1; id <= 3; id++ {
		u := testUser(id, 500, 0)
		if res, _ := buy(t, u); res != PurchaseOk {
			t.Fatalf("buyer %d: got %v", id, res)
		}
		h, _ := HouseOf(id, testBldgId)
		if seen[h.EntryRoom()] {
			t.Fatalf("room %d sold twice", h.EntryRoom())
		}
		seen[h.EntryRoom()] = true
	}
	late := testUser(4, 500, 0)
	if res, _ := buy(t, late); res != PurchaseNoVacancy {
		t.Fatalf("got %v, want PurchaseNoVacancy", res)
	}
	if late.Character.Gold != 500 {
		t.Error("charged with no vacancy")
	}
}

// Persist before publishing: if the write fails the player keeps their gold
// and no house appears in memory.
func TestPurchase_WriteFailureChargesNothing(t *testing.T) {
	dir := setup(t)
	// A regular file where the building's folder should be makes MkdirAll fail.
	if err := os.WriteFile(filepath.Join(dir, testBldgId), []byte(`x`), 0644); err != nil {
		t.Fatal(err)
	}
	u := testUser(7, 600, 0)
	if res, _ := buy(t, u); res != PurchaseError {
		t.Fatalf("got %v, want PurchaseError", res)
	}
	if u.Character.Gold != 600 {
		t.Errorf("charged despite failed write: %d", u.Character.Gold)
	}
	if _, ok := HouseOf(7, testBldgId); ok {
		t.Error("house published despite failed write")
	}
	if len(VacantUnits(testBldgId)) != 3 {
		t.Error("a unit was consumed by a failed purchase")
	}
}

// ── Door routing and entry guard ───────────────────────────────────────────

func TestRouteDoor_OwnersGoToTheirOwnRoom(t *testing.T) {
	setup(t)
	buy(t, testUser(1, 500, 0))
	buy(t, testUser(2, 500, 0))

	r1, h1 := RouteDoor(1, testDoor, `door`)
	r2, h2 := RouteDoor(2, testDoor, `door`)
	if !h1 || !h2 {
		t.Fatal("door not handled")
	}
	if r1.RoomId == 0 || r2.RoomId == 0 || r1.RoomId == r2.RoomId {
		t.Fatalf("owners routed to %d and %d; want two different rooms", r1.RoomId, r2.RoomId)
	}
}

func TestRouteDoor_StrangersAreRefused(t *testing.T) {
	setup(t)
	route, handled := RouteDoor(99, testDoor, `door`)
	if !handled {
		t.Fatal("door not handled for a stranger")
	}
	if route.RoomId != 0 || !strings.Contains(route.Refusal, `Hobb Pennock`) {
		t.Errorf("route = %+v; want a refusal naming the landlord", route)
	}
}

func TestRouteDoor_IgnoresOtherExits(t *testing.T) {
	setup(t)
	if _, handled := RouteDoor(1, testDoor, `east`); handled {
		t.Error("an ordinary exit in the door room was routed")
	}
	if _, handled := RouteDoor(1, 1234, `door`); handled {
		t.Error("a door exit in some other room was routed")
	}
}

func TestGuardEntry(t *testing.T) {
	setup(t)
	buy(t, testUser(1, 500, 0))
	h, _ := HouseOf(1, testBldgId)

	if ok, _, _ := GuardEntry(1, h.EntryRoom()); !ok {
		t.Error("owner refused their own room")
	}
	ok, redirect, refusal := GuardEntry(2, h.EntryRoom())
	if ok || redirect != testDoor || refusal == `` {
		t.Errorf("stranger: ok=%v redirect=%d refusal=%q; want refused to the door", ok, redirect, refusal)
	}
	// A vacant unit admits nobody.
	if ok, _, _ := GuardEntry(1, testUnitC); ok {
		t.Error("a vacant unit let someone in")
	}
	// Rooms that are not units are none of housing's business.
	if ok, _, _ := GuardEntry(2, testDoor); !ok {
		t.Error("the door room itself was guarded")
	}
	// Staff may go anywhere.
	isStaff = func(int) bool { return true }
	if ok, _, _ := GuardEntry(2, h.EntryRoom()); !ok {
		t.Error("staff refused")
	}
}

func TestPrivateRooms(t *testing.T) {
	setup(t)
	for _, id := range []int{testUnitA, testUnitB, testUnitC} {
		if !IsUnitRoom(id) {
			t.Errorf("unit %d not private", id)
		}
	}
	if IsUnitRoom(testDoor) {
		t.Error("the door room is not private")
	}
}

// ── Living-state load ──────────────────────────────────────────────────────

func TestLoad_RoundTrip(t *testing.T) {
	setup(t)
	buy(t, testUser(1, 500, 0))
	want, _ := HouseOf(1, testBldgId)

	// A fresh registry (a reboot) rebuilds the same state from disk alone.
	ResetForTest()
	AddBuildingForTest(testBuilding())
	if n, held := loadHouses(); n != 1 || held != 0 {
		t.Fatalf("loaded %d houses, %d held; want 1, 0", n, held)
	}
	got, ok := HouseOf(1, testBldgId)
	if !ok || !reflect.DeepEqual(got.RoomIds, want.RoomIds) || !got.PurchasedAt.Equal(want.PurchasedAt) {
		t.Fatalf("after reload: %+v, want %+v", got, want)
	}
}

// A corrupt file is quarantined (never deleted) and its room is held back
// from sale, so a stranger is never sold a room with someone else's things.
func TestLoad_CorruptFileIsQuarantinedAndItsRoomHeld(t *testing.T) {
	dir := setup(t)
	bdir := filepath.Join(dir, testBldgId)
	os.MkdirAll(bdir, 0755)
	bad := filepath.Join(bdir, `6470.yaml`)
	os.WriteFile(bad, []byte("building_id: [unclosed\n"), 0644)

	n, held := loadHouses()
	if n != 0 || held != 1 {
		t.Fatalf("loaded %d, held %d; want 0, 1", n, held)
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Error("corrupt file should have been moved aside")
	}
	matches, _ := filepath.Glob(bad + `.corrupt-*`)
	if len(matches) != 1 {
		t.Errorf("want one quarantined copy, found %v", matches)
	}
	for _, id := range VacantUnits(testBldgId) {
		if id == testUnitA {
			t.Error("held room is for sale")
		}
	}
	if ok, _, _ := GuardEntry(1, testUnitA); ok {
		t.Error("a held room let someone in")
	}
	// The next boot skips the quarantined copy rather than re-reading it.
	ResetForTest()
	AddBuildingForTest(testBuilding())
	if n, _ := loadHouses(); n != 0 {
		t.Errorf("quarantined copy was loaded")
	}
}

// A readable file that disagrees with the world is left in place for staff
// and its rooms held.
func TestLoad_InconsistentFileIsKeptAndHeld(t *testing.T) {
	dir := setup(t)
	h := House{BuildingId: testBldgId, OwnerUserId: 1, TierId: `simple`, RoomIds: []int{testUnitA, 1234}, PurchasedAt: time.Now()}
	if err := saveHouse(h); err != nil {
		t.Fatal(err)
	}
	n, _ := loadHouses()
	if n != 0 {
		t.Fatal("inconsistent house was indexed")
	}
	if _, err := os.Stat(filepath.Join(dir, testBldgId, `6470.yaml`)); err != nil {
		t.Error("inconsistent file should stay where it is")
	}
	if _, isHeld := HeldRooms()[testUnitA]; !isHeld {
		t.Error("its room should be held")
	}
}

func TestLoad_SecondHouseForSameOwnerIsHeld(t *testing.T) {
	setup(t)
	saveHouse(House{BuildingId: testBldgId, OwnerUserId: 1, TierId: `simple`, RoomIds: []int{testUnitA}})
	saveHouse(House{BuildingId: testBldgId, OwnerUserId: 1, TierId: `simple`, RoomIds: []int{testUnitB}})
	if n, _ := loadHouses(); n != 1 {
		t.Fatalf("loaded %d, want 1", n)
	}
	if _, isHeld := HeldRooms()[testUnitB]; !isHeld {
		t.Error("the duplicate's room should be held")
	}
}
