package housing

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Two lodging houses in two cities run independently: a player may own a
// home in each, gated by each city's own standing, and every deed, key,
// guest and container belongs to one house only.

const (
	quillId   = `test_quill`
	quillDoor = 6570
	quillA    = 6571
	quillB    = 6572
)

func quillBuilding() Building {
	b := testBuilding()
	b.BuildingId = quillId
	b.Name = `the test Quillhouse`
	b.DoorRoom = quillDoor
	b.LandlordMobId = 9802
	b.Faction = `margin`
	b.Proprietor = `Madam Pardew`
	b.VouchedBy = `the Margin`
	b.Location = `Quill Court`
	b.OutsideText = `You find yourself back out in Quill Court.`
	b.UnitRooms = []int{quillA, quillB, 6573}
	b.ExtensionItemId, b.RedecorateItemId, b.GuestKeyItemId, b.ContainerItemId, b.StrongboxItemId = 60, 61, 62, 63, 64
	b.BedItemId, b.StationItemId = 77, 78
	return b
}

// twoCities is setup plus the second building, with standing warm in both.
func twoCities(t *testing.T) {
	t.Helper()
	setup(t)
	AddBuildingForTest(quillBuilding())
}

func buyIn(u *users.UserRecord, buildingId string) PurchaseResult {
	return Purchase(u, func(string) {}, buildingId, `simple`)
}

func buyFrom(u *users.UserRecord, buildingId string, key string) {
	Buy(u, func(string) {}, buildingId, `simple`, key)
}

func TestTwoCities_AHomeInEach(t *testing.T) {
	twoCities(t)
	u := testUser(1, 100000, 0)
	if res := buyIn(u, testBldgId); res != PurchaseOk {
		t.Fatalf("first home: %v", res)
	}
	if res := buyIn(u, quillId); res != PurchaseOk {
		t.Fatalf("a home in the second city was refused: %v", res)
	}
	if res := buyIn(u, quillId); res != PurchaseAlreadyOwner {
		t.Errorf("a second home in the same city: %v", res)
	}
	if n := len(HousesOwnedBy(1)); n != 2 {
		t.Fatalf("owns %d houses, want 2", n)
	}
	np, _ := HouseOf(1, testBldgId)
	q, _ := HouseOf(1, quillId)
	if np.EntryRoom() != testUnitA || q.EntryRoom() != quillA {
		t.Errorf("homes in %d and %d", np.EntryRoom(), q.EntryRoom())
	}

	// Each door sends the lodger to that city's home.
	if r, ok := RouteDoor(1, testDoor, `door`); !ok || r.RoomId != testUnitA {
		t.Errorf("New Plymouth door routed to %+v", r)
	}
	if r, ok := RouteDoor(1, quillDoor, `door`); !ok || r.RoomId != quillA {
		t.Errorf("Confluence door routed to %+v", r)
	}
}

func TestTwoCities_StandingIsPerCity(t *testing.T) {
	twoCities(t)
	restore := SetRepTierForTest(func(faction string, _ int) opinions.Tier {
		if faction == `margin` {
			return opinions.TierNeutral
		}
		return opinions.TierWarm
	})
	defer restore()
	u := testUser(1, 100000, 0)
	if res := buyIn(u, testBldgId); res != PurchaseOk {
		t.Errorf("warm with the Common Quarter, refused: %v", res)
	}
	if res := buyIn(u, quillId); res != PurchaseRepTooLow {
		t.Errorf("neutral with the Margin, got %v", res)
	}
}

func TestTwoCities_DeedsAndPricesAreSeparate(t *testing.T) {
	twoCities(t)
	u := testUser(1, 100000, 0)
	buyIn(u, testBldgId)
	buyIn(u, quillId)

	buyFrom(u, testBldgId, OfferExtension) // 1500 in New Plymouth
	if got := offersIn(u, quillId)[OfferExtension].Price; got != 1500 {
		t.Errorf("the Confluence's first deed costs %d, want 1500 whatever was spent elsewhere", got)
	}
	buyFrom(u, quillId, OfferExtension)
	npDeed, qDeed := itemOf(u, testDeedId), itemOf(u, 60)
	if npDeed.ItemId == 0 || qDeed.ItemId == 0 {
		t.Fatal("setup: missing a deed")
	}

	// The New Plymouth deed does nothing in the Confluence home.
	qEntry := unitRoom(quillA)
	ApplyOverlay(qEntry)
	UseItem(u, qEntry, npDeed, `north`, `deed north`)
	if q, _ := HouseOf(1, quillId); len(q.RoomIds) != 1 || !has(u, testDeedId) {
		t.Fatal("a New Plymouth deed built a room in the Confluence")
	}
	// Its own deed does.
	UseItem(u, qEntry, qDeed, `north`, `deed north`)
	if q, _ := HouseOf(1, quillId); len(q.RoomIds) != 2 {
		t.Fatal("the Confluence deed did not build")
	}
	if np, _ := HouseOf(1, testBldgId); np.Outstanding(testBuilding()) != 1 || len(np.RoomIds) != 1 {
		t.Errorf("New Plymouth's ledger changed: %+v", np)
	}
}

func TestTwoCities_KeysOpenOnlyTheirOwnDoor(t *testing.T) {
	twoCities(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 100000), w.player(2, `Bob`, 1000)
	buyIn(alice, testBldgId)
	buyIn(alice, quillId)
	buyFrom(alice, quillId, OfferGuestKey)
	key := itemOf(alice, 62)
	alice.Character.RemoveItem(key)
	bob.Character.StoreItem(key)

	// Presented at the New Plymouth door, the Confluence key does nothing.
	UseItem(bob, alleyRoom(), key, ``, `guest key`)
	if h, _ := HouseOf(1, testBldgId); h.IsGuest(2) {
		t.Fatal("a Confluence key let Bob into the New Plymouth lodging")
	}
	quillCourt := &rooms.Room{RoomId: quillDoor}
	UseItem(bob, quillCourt, key, ``, `guest key`)
	if h, _ := HouseOf(1, quillId); !h.IsGuest(2) {
		t.Fatal("the key did not work at its own door")
	}
	if h, _ := HouseOf(1, testBldgId); h.IsGuest(2) {
		t.Error("the Confluence key also let Bob in at New Plymouth")
	}
	if r, _ := RouteDoor(2, testDoor, `door`); r.RoomId != 0 || len(r.Choices) != 0 {
		t.Errorf("Bob routed through the New Plymouth door: %+v", r)
	}
}

func TestTwoCities_RevokeAndLeaveCoverBothHomes(t *testing.T) {
	twoCities(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 100000), w.player(2, `Bob`, 1000)
	buyIn(alice, testBldgId)
	buyIn(alice, quillId)
	for _, c := range []struct {
		building string
		keyId    int
		door     int
	}{{testBldgId, testKeyId, testDoor}, {quillId, 62, quillDoor}} {
		buyFrom(alice, c.building, OfferGuestKey)
		k := itemOf(alice, c.keyId)
		alice.Character.RemoveItem(k)
		bob.Character.StoreItem(k)
		UseItem(bob, &rooms.Room{RoomId: c.door}, k, ``, `guest key`)
	}
	if len(GuestOf(2)) != 2 {
		t.Fatalf("setup: Bob is a guest of %d lodgings", len(GuestOf(2)))
	}

	_, where, err := Revoke(1, `bob`)
	if err != nil || len(where) != 2 || len(GuestOf(2)) != 0 {
		t.Fatalf("revoke: %v %v; still a guest of %d", err, where, len(GuestOf(2)))
	}

	// Let back in to both, then Bob leaves both at once.
	for _, c := range []struct {
		building string
		keyId    int
		door     int
	}{{testBldgId, testKeyId, testDoor}, {quillId, 62, quillDoor}} {
		buyFrom(alice, c.building, OfferGuestKey)
		k := itemOf(alice, c.keyId)
		alice.Character.RemoveItem(k)
		bob.Character.StoreItem(k)
		UseItem(bob, &rooms.Room{RoomId: c.door}, k, ``, `guest key`)
	}
	if _, where, err := Leave(2, `ali`); err != nil || len(where) != 2 || len(GuestOf(2)) != 0 {
		t.Fatalf("leave: %v %v; still a guest of %d", err, where, len(GuestOf(2)))
	}
}

func TestTwoCities_ItemIssuedHere(t *testing.T) {
	twoCities(t)
	cases := []struct {
		item, room int
		want       bool
	}{
		{testDeedId, testUnitA, true}, {testDeedId, testDoor, true}, {testDeedId, quillA, false},
		{60, quillA, true}, {62, quillDoor, true}, {62, testDoor, false}, {60, 5615, false},
	}
	for _, c := range cases {
		if got := ItemIssuedHere(c.item, c.room); got != c.want {
			t.Errorf("ItemIssuedHere(%d, %d) = %v, want %v", c.item, c.room, got, c.want)
		}
	}
}

func offersIn(u *users.UserRecord, buildingId string) map[string]Offer {
	out := map[string]Offer{}
	for _, o := range Offers(u, buildingId, `simple`) {
		out[o.Key] = o
	}
	return out
}

// Everything every landlord sells is registered as never bought by a
// merchant, so what was paid for it cannot come back through a shop.
func TestLoad_EveryHousingItemIsNeverBought(t *testing.T) {
	setup(t)
	defer items.SetNeverBought(nil)
	rebuild(map[string]Building{testBldgId: testBuilding(), quillId: quillBuilding()}, nil)
	for _, id := range []int{55, 56, 57, 58, 59, 60, 61, 62, 63, 64} {
		if !items.IsNeverBought(id) {
			t.Errorf("item %d can be sold to a merchant", id)
		}
	}
	if items.IsNeverBought(10001) {
		t.Error("an ordinary item was marked never bought")
	}
}

// A building with no faction checks no standing at all: a player every city
// despises is still sold a home there.
func TestNoStandingCheck_AnybodyMayLodge(t *testing.T) {
	setup(t)
	wild := quillBuilding()
	wild.BuildingId, wild.DoorRoom = `test_wild`, 6771
	wild.UnitRooms = []int{6772, 6773}
	wild.Faction, wild.MinRepTier, wild.VouchedBy, wild.StandingHint = ``, ``, ``, ``
	wild.ExtensionItemId, wild.RedecorateItemId, wild.GuestKeyItemId, wild.ContainerItemId, wild.StrongboxItemId = 70, 71, 72, 73, 74
	wild.BedItemId, wild.StationItemId = 81, 82
	if err := wild.Validate(); err != nil {
		t.Fatalf("a building with no standing check is invalid: %v", err)
	}
	AddBuildingForTest(wild)
	restore := SetRepTierForTest(func(string, int) opinions.Tier { return opinions.TierHostile })
	defer restore()

	u := testUser(1, 100000, 0)
	if o := offersIn(u, `test_wild`)[OfferHome]; !o.Available {
		t.Errorf("home not offered: %+v", o)
	}
	said := []string{}
	DescribeTerms(u, func(s string) { said = append(said, s) }, `test_wild`, `simple`)
	if len(said) == 0 || said[len(said)-1] != wild.Line(`terms.open`) {
		t.Errorf("terms = %q", said)
	}
	if res := buyIn(u, `test_wild`); res != PurchaseOk {
		t.Errorf("hostile everywhere, refused: %v", res)
	}
	// The same player is still refused in a city that checks.
	if res := buyIn(u, testBldgId); res != PurchaseRepTooLow {
		t.Errorf("a city skipped its check: %v", res)
	}

	bad := wild
	bad.MinRepTier = `warm`
	if err := bad.Validate(); err == nil {
		t.Error("a standing tier with no faction was accepted")
	}
}
