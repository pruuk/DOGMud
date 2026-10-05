package housing

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// guestWorld is a test world with named players who are "online" and a record
// of every move that ejection makes.
type guestWorld struct {
	online map[int]*users.UserRecord
	moves  map[int]int // user -> room they were moved to
}

func newGuestWorld(t *testing.T) *guestWorld {
	t.Helper()
	w := &guestWorld{online: map[int]*users.UserRecord{}, moves: map[int]int{}}
	prevOnline, prevMove := onlineUser, moveUser
	onlineUser = func(id int) *users.UserRecord { return w.online[id] }
	moveUser = func(id, roomId int) error {
		w.moves[id] = roomId
		if u := w.online[id]; u != nil {
			u.Character.RoomId = roomId
		}
		return nil
	}
	t.Cleanup(func() { onlineUser, moveUser = prevOnline, prevMove })
	return w
}

func (w *guestWorld) player(id int, name string, gold int) *users.UserRecord {
	u := testUser(id, gold, 0)
	u.Character.Name = name
	w.online[id] = u
	return u
}

func alleyRoom() *rooms.Room {
	return &rooms.Room{RoomId: testDoor, Exits: map[string]exit.RoomExit{`door`: {RoomId: testDoor}}}
}

func keyIn(u *users.UserRecord) (items.Item, bool) {
	for _, itm := range u.Character.Items {
		if itm.ItemId == testKeyId {
			return itm, true
		}
	}
	return items.Item{}, false
}

// hostGivesKey has host buy a key and hand it to friend.
func hostGivesKey(t *testing.T, host, friend *users.UserRecord) items.Item {
	t.Helper()
	buyKey(host, OfferGuestKey)
	key, ok := keyIn(host)
	if !ok {
		t.Fatal("setup: host got no key")
	}
	host.Character.RemoveItem(key)
	friend.Character.StoreItem(key)
	return key
}

// letIn has host buy a key, give it to friend, and friend use it at the door.
func letIn(t *testing.T, host, friend *users.UserRecord) {
	t.Helper()
	key := hostGivesKey(t, host, friend)
	UseItem(friend, alleyRoom(), key, ``, `guest key`)
	if h, _ := HouseOf(host.UserId, testBldgId); !h.IsGuest(friend.UserId) {
		t.Fatal("setup: friend was not let in")
	}
}

// ── Buying a key ───────────────────────────────────────────────────────────

func TestGuestKey_OnTheListAndMadeOutToTheHouse(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice := w.player(1, `Alice`, 1000)

	if o := offerMap(alice)[OfferGuestKey]; o.Available {
		t.Error("a key is offered to someone with no home")
	}
	buy(t, alice)
	if o := offerMap(alice)[OfferGuestKey]; !o.Available || o.Price != 100 {
		t.Fatalf("key offer = %+v, want 100", o)
	}
	if key, _ := MatchOffer(`guest key`, true); key != OfferGuestKey {
		t.Errorf("buy guest key matched %q", key)
	}

	buyKey(alice, OfferGuestKey)
	key, ok := keyIn(alice)
	if !ok || key.HouseKeyOwner != 1 {
		t.Fatalf("key = %+v, want one made out to account 1", key)
	}
	if alice.Character.Gold != 400 {
		t.Errorf("gold = %d, want 400 (1000 - 500 - 100)", alice.Character.Gold)
	}
	if h, _ := HouseOf(1, testBldgId); len(h.Guests) != 0 {
		t.Error("buying a key must not let anyone in by itself")
	}
}

// ── Using a key ────────────────────────────────────────────────────────────

func TestGuestKey_UsedAtTheDoorLetsTheFriendIn(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 1000), w.player(2, `Bob`, 0)
	buy(t, alice)

	key := hostGivesKey(t, alice, bob)
	UseItem(bob, alleyRoom(), key, ``, `guest key`)

	h, _ := HouseOf(1, testBldgId)
	if !h.IsGuest(2) || h.Guests[0].Name != `Bob` {
		t.Fatalf("guests = %+v", h.Guests)
	}
	if _, still := keyIn(bob); still {
		t.Error("the key should be spent in the lock")
	}
	route, _ := RouteDoor(2, testDoor, `door`)
	if route.RoomId != h.EntryRoom() {
		t.Errorf("Bob's door leads to %d, want Alice's entry %d", route.RoomId, h.EntryRoom())
	}
	if ok, _, _ := GuardEntry(2, h.EntryRoom()); !ok {
		t.Error("the guest is refused at the entry room")
	}
	if ok, _, _ := GuardEntry(3, h.EntryRoom()); ok {
		t.Error("a stranger is let in")
	}
}

func TestGuestKey_WalksEveryRoomOfTheHouse(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 100000), w.player(2, `Bob`, 0)
	buy(t, alice)
	buyKey(alice, OfferExtension)
	deed, _ := deedIn(alice)
	UseItem(alice, unitRoom(testUnitA), deed, `north`, `deed north`)
	letIn(t, alice, bob)

	h, _ := HouseOf(1, testBldgId)
	for _, roomId := range h.RoomIds {
		if ok, _, _ := GuardEntry(2, roomId); !ok {
			t.Errorf("the guest cannot enter room %d of the house", roomId)
		}
	}
}

func TestGuestKey_Refusals(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 1000), w.player(2, `Bob`, 1000)
	buy(t, alice)

	// Away from the door.
	key := hostGivesKey(t, alice, bob)
	UseItem(bob, worldRoom(), key, ``, `guest key`)
	// The owner's own key.
	buyKey(alice, OfferGuestKey)
	own, _ := keyIn(alice)
	UseItem(alice, alleyRoom(), own, ``, `guest key`)
	// A key made out to nobody.
	blank := items.New(testKeyId)
	bob.Character.StoreItem(blank)
	UseItem(bob, alleyRoom(), blank, ``, `guest key`)

	if h, _ := HouseOf(1, testBldgId); len(h.Guests) != 0 {
		t.Fatalf("a refused key let someone in: %+v", h.Guests)
	}
	if _, has := keyIn(bob); !has {
		t.Error("a refused key was spent")
	}
	if _, has := keyIn(alice); !has {
		t.Error("the owner's refused key was spent")
	}

	// Already a guest: the second key is kept.
	UseItem(bob, alleyRoom(), key, ``, `guest key`)
	second := hostGivesKey(t, alice, bob)
	UseItem(bob, alleyRoom(), second, ``, `guest key`)
	if h, _ := HouseOf(1, testBldgId); len(h.Guests) != 1 {
		t.Errorf("guests = %+v, want Bob once", h.Guests)
	}
}

func TestGuestKey_GuestLimit(t *testing.T) {
	setup(t) // the test building allows 2 guests
	w := newGuestWorld(t)
	alice := w.player(1, `Alice`, 10000)
	buy(t, alice)
	letIn(t, alice, w.player(2, `Bob`, 0))
	letIn(t, alice, w.player(3, `Cara`, 0))

	if o := offerMap(alice)[OfferGuestKey]; o.Available {
		t.Error("a key is still for sale at the guest limit")
	}
	gold := alice.Character.Gold
	buyKey(alice, OfferGuestKey)
	if alice.Character.Gold != gold {
		t.Error("charged for a key at the guest limit")
	}
}

// ── Choosing at the door ───────────────────────────────────────────────────

func TestDoor_OwnerWhoIsAlsoAGuestGetsAChoice(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 1000), w.player(2, `Bob`, 1000)
	buy(t, alice)
	buy(t, bob)
	letIn(t, alice, bob)

	route, handled := RouteDoor(2, testDoor, `door`)
	if !handled || route.RoomId != 0 || len(route.Choices) != 2 {
		t.Fatalf("route = %+v, want two choices and no single room", route)
	}
	bobs, _ := HouseOf(2, testBldgId)
	alices, _ := HouseOf(1, testBldgId)
	if route.Choices[0].Label != `home` || route.Choices[0].RoomId != bobs.EntryRoom() {
		t.Errorf("first choice = %+v, want home", route.Choices[0])
	}
	if route.Choices[1].Label != `Alice` || route.Choices[1].RoomId != alices.EntryRoom() {
		t.Errorf("second choice = %+v, want Alice", route.Choices[1])
	}
	if !route.Leads(alices.EntryRoom()) || !route.Leads(bobs.EntryRoom()) || route.Leads(testUnitC) {
		t.Error("Leads disagrees with the choices")
	}

	// Alice, who is nobody's guest, still goes straight home.
	if r, _ := RouteDoor(1, testDoor, `door`); r.RoomId != alices.EntryRoom() {
		t.Errorf("owner route = %+v, want straight home", r)
	}
}

// ── Revoking and leaving ───────────────────────────────────────────────────

func TestRevoke_TakesAccessAwayAndPutsTheGuestOutside(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 1000), w.player(2, `Bob`, 0)
	buy(t, alice)
	letIn(t, alice, bob)
	h, _ := HouseOf(1, testBldgId)
	bob.Character.RoomId = h.EntryRoom() // Bob is inside

	g, _, err := Revoke(1, `bo`) // a unique prefix works
	if err != nil || g.UserId != 2 {
		t.Fatalf("Revoke = %+v, %v", g, err)
	}
	if after, _ := HouseOf(1, testBldgId); after.IsGuest(2) {
		t.Error("Bob is still a guest")
	}
	if ok, _, _ := GuardEntry(2, h.EntryRoom()); ok {
		t.Error("a revoked guest is still let in")
	}
	if w.moves[2] != testDoor {
		t.Errorf("Bob was moved to %d, want the alley %d", w.moves[2], testDoor)
	}
	if r, _ := RouteDoor(2, testDoor, `door`); r.RoomId != 0 || len(r.Choices) != 0 {
		t.Errorf("the door still opens for Bob: %+v", r)
	}
}

func TestRevoke_GuestOutsideIsNotMoved(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 1000), w.player(2, `Bob`, 0)
	buy(t, alice)
	letIn(t, alice, bob)
	bob.Character.RoomId = 5615
	Revoke(1, `Bob`)
	if _, moved := w.moves[2]; moved {
		t.Error("a guest who was not inside was moved")
	}
}

func TestRevoke_UnknownOrAmbiguousName(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice := w.player(1, `Alice`, 1000)
	buy(t, alice)
	letIn(t, alice, w.player(2, `Bram`, 0))
	letIn(t, alice, w.player(3, `Brin`, 0))
	if _, _, err := Revoke(1, `zed`); err == nil {
		t.Error("revoked a name that is not a guest")
	}
	if _, _, err := Revoke(1, `br`); err == nil {
		t.Error("revoked on an ambiguous prefix")
	}
	if _, _, err := Revoke(2, `Brin`); err == nil {
		t.Error("a guest revoked someone from a house they do not own")
	}
	if h, _ := HouseOf(1, testBldgId); len(h.Guests) != 2 {
		t.Errorf("guests = %+v, want both still", h.Guests)
	}
}

func TestLeave_GuestGivesUpAccess(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice, bob := w.player(1, `Alice`, 1000), w.player(2, `Bob`, 0)
	buy(t, alice)
	letIn(t, alice, bob)
	h, _ := HouseOf(1, testBldgId)
	bob.Character.RoomId = h.EntryRoom()

	if _, _, err := Leave(2, `ali`); err != nil {
		t.Fatalf("Leave: %v", err)
	}
	if after, _ := HouseOf(1, testBldgId); after.IsGuest(2) {
		t.Error("Bob is still a guest")
	}
	if w.moves[2] != testDoor {
		t.Error("Bob was not put outside")
	}
	if _, _, err := Leave(2, `ali`); err == nil {
		t.Error("left a house twice")
	}
}

// ── What guests cannot do ──────────────────────────────────────────────────

func TestGuest_CannotUseDeedsOrVouchersInTheHostsHouse(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice := w.player(1, `Alice`, 1000)
	bob := w.player(2, `Bob`, 100000)
	buy(t, alice)
	buy(t, bob)
	buyKey(bob, OfferExtension)
	buyKey(bob, OfferRedecorate)
	letIn(t, alice, bob)

	hostRoom := unitRoom(func() int { h, _ := HouseOf(1, testBldgId); return h.EntryRoom() }())
	UseItem(bob, hostRoom, itemOf(bob, testDeedId), `north`, `deed north`)
	UseItem(bob, hostRoom, itemOf(bob, testVoucherId), newDesc, `voucher `+newDesc)
	if p := bob.GetPrompt(); p != nil && p.GetNextQuestion() != nil {
		p.GetNextQuestion().Answer(`yes`)
		UseItem(bob, hostRoom, itemOf(bob, testVoucherId), newDesc, `voucher `+newDesc)
	}

	host, _ := HouseOf(1, testBldgId)
	if len(host.RoomIds) != 1 || len(host.Descriptions) != 0 {
		t.Errorf("a guest changed the host's house: %+v", host)
	}
	if !has(bob, testDeedId) || !has(bob, testVoucherId) {
		t.Error("a guest's refused item was used up")
	}
}

// ── Persistence ────────────────────────────────────────────────────────────

func TestGuests_SurviveReload(t *testing.T) {
	setup(t)
	w := newGuestWorld(t)
	alice := w.player(1, `Alice`, 1000)
	buy(t, alice)
	letIn(t, alice, w.player(2, `Bob`, 0))

	ResetForTest()
	AddBuildingForTest(testBuilding())
	loadHouses()
	h, _ := HouseOf(1, testBldgId)
	if !h.IsGuest(2) || h.Guests[0].Name != `Bob` {
		t.Errorf("guests after reload = %+v", h.Guests)
	}
}

func TestLoad_BadGuestsAreHeld(t *testing.T) {
	setup(t)
	saveHouse(House{BuildingId: testBldgId, OwnerUserId: 1, TierId: `simple`, RoomIds: []int{testUnitA},
		Guests: []Guest{{UserId: 1, Name: `Alice`}}}) // the owner as her own guest
	if n, _ := loadHouses(); n != 0 {
		t.Fatal("a house listing its owner as a guest was loaded")
	}
	if _, held := HeldRooms()[testUnitA]; !held {
		t.Error("its room should be held")
	}
}

func TestHouseCommandsNeverPanicWithNoHouse(t *testing.T) {
	setup(t)
	if _, _, err := Revoke(9, `anyone`); err == nil || !strings.Contains(err.Error(), `no guest`) {
		t.Errorf("Revoke with no house: %v", err)
	}
	if _, _, err := Leave(9, `anyone`); err == nil {
		t.Error("Leave with no access should error")
	}
}

// Keys bought before the limit was reached are still refused at the lock
// once the house is full, and the refused key is kept.
func TestGuestKey_LimitHeldAtTheLock(t *testing.T) {
	setup(t) // the test building allows 2 guests
	w := newGuestWorld(t)
	alice := w.player(1, `Alice`, 10000)
	buy(t, alice)
	friends := []*users.UserRecord{w.player(2, `Bob`, 0), w.player(3, `Cara`, 0), w.player(4, `Dov`, 0)}
	keys := []items.Item{}
	for _, f := range friends {
		keys = append(keys, hostGivesKey(t, alice, f)) // all three bought up front
	}
	for i, f := range friends {
		UseItem(f, alleyRoom(), keys[i], ``, `guest key`)
	}
	h, _ := HouseOf(1, testBldgId)
	if len(h.Guests) != 2 || h.IsGuest(4) {
		t.Errorf("guests = %+v, want the first two only", h.Guests)
	}
	if _, kept := keyIn(friends[2]); !kept {
		t.Error("the refused third key was spent")
	}
}
