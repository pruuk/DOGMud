package housing

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// These pin down where deeds and vouchers work: only in a room of a house
// owned by the player using them. Everywhere else they are refused and never
// used up, and the player's house does not change.

func worldRoom() *rooms.Room {
	return &rooms.Room{
		RoomId:      5615, // the Back Court: an ordinary street, not a unit
		Title:       `The Back Court`,
		Description: `A court between blocks.`,
		Exits:       map[string]exit.RoomExit{`west`: {RoomId: testDoor}},
	}
}

func has(u *users.UserRecord, itemId int) bool {
	for _, itm := range u.Character.Items {
		if itm.ItemId == itemId {
			return true
		}
	}
	return false
}

func itemOf(u *users.UserRecord, itemId int) items.Item {
	for _, itm := range u.Character.Items {
		if itm.ItemId == itemId {
			return itm
		}
	}
	return items.Item{}
}

// ownerWithBoth is a lodger holding one deed and one voucher.
func ownerWithBoth(t *testing.T, userId int) *users.UserRecord {
	t.Helper()
	u := testUser(userId, 100000, 0)
	if res, _ := buy(t, u); res != PurchaseOk {
		t.Fatalf("setup: home purchase %v", res)
	}
	buyKey(u, OfferExtension)
	buyKey(u, OfferRedecorate)
	if !has(u, testDeedId) || !has(u, testVoucherId) {
		t.Fatal("setup: missing deed or voucher")
	}
	return u
}

func assertUnchanged(t *testing.T, where string, u *users.UserRecord, before House) {
	t.Helper()
	after, _ := HouseOf(u.UserId, testBldgId)
	if len(after.RoomIds) != len(before.RoomIds) || len(after.Links) != len(before.Links) || len(after.Descriptions) != len(before.Descriptions) {
		t.Errorf("%s: the house changed: %+v", where, after)
	}
	if !has(u, testDeedId) || !has(u, testVoucherId) {
		t.Errorf("%s: an item was used up", where)
	}
}

func TestItemsRefusedInARegularWorldRoom(t *testing.T) {
	setup(t)
	u := ownerWithBoth(t, 1)
	before, _ := HouseOf(1, testBldgId)
	street := worldRoom()

	UseItem(u, street, itemOf(u, testDeedId), `north`, `deed north`)
	UseItem(u, street, itemOf(u, testDeedId), ``, `deed`) // the menu must not open either
	UseItem(u, street, itemOf(u, testVoucherId), newDesc, `voucher `+newDesc)
	UseItem(u, street, itemOf(u, testVoucherId), ``, `voucher`)

	assertUnchanged(t, `world room`, u, before)
	if u.GetPrompt() != nil && u.GetPrompt().GetNextQuestion() != nil {
		t.Error("a prompt was opened outside the house")
	}
	if street.Description != `A court between blocks.` || len(street.Exits) != 1 {
		t.Error("the street room itself was changed")
	}
}

func TestItemsRefusedInTheDoorRoomAndAVacantUnit(t *testing.T) {
	setup(t)
	u := ownerWithBoth(t, 1)
	before, _ := HouseOf(1, testBldgId)

	alley := &rooms.Room{RoomId: testDoor, Exits: map[string]exit.RoomExit{`door`: {RoomId: testDoor}}}
	vacant := unitRoom(testUnitC) // a unit nobody owns
	for _, r := range []*rooms.Room{alley, vacant} {
		UseItem(u, r, itemOf(u, testDeedId), `north`, `deed north`)
		UseItem(u, r, itemOf(u, testVoucherId), newDesc, `voucher `+newDesc)
	}
	assertUnchanged(t, `alley or vacant unit`, u, before)
}

// Standing in another lodger's room (only staff can, but the rule must not
// depend on that) is not your house.
func TestItemsRefusedInSomeoneElsesHouse(t *testing.T) {
	setup(t)
	u := ownerWithBoth(t, 1)
	other := testUser(2, 1000, 0)
	buy(t, other)
	theirs, _ := HouseOf(2, testBldgId)
	theirRoom := unitRoom(theirs.EntryRoom())
	before, _ := HouseOf(1, testBldgId)

	UseItem(u, theirRoom, itemOf(u, testDeedId), `north`, `deed north`)
	UseItem(u, theirRoom, itemOf(u, testDeedId), ``, `deed`)
	if p := u.GetPrompt(); p != nil && p.GetNextQuestion() != nil {
		p.GetNextQuestion().Answer(`north`) // push through a menu if one opened
		UseItem(u, theirRoom, itemOf(u, testDeedId), ``, `deed`)
	}
	UseItem(u, theirRoom, itemOf(u, testVoucherId), newDesc, `voucher `+newDesc)
	if p := u.GetPrompt(); p != nil && p.GetNextQuestion() != nil {
		p.GetNextQuestion().Answer(`yes`) // push through a confirmation if one opened
		UseItem(u, theirRoom, itemOf(u, testVoucherId), newDesc, `voucher `+newDesc)
	}

	assertUnchanged(t, `someone else's house`, u, before)
	if after, _ := HouseOf(2, testBldgId); len(after.RoomIds) != 1 || len(after.Descriptions) != 0 {
		t.Errorf("the other lodger's house changed: %+v", after)
	}
}

// A deed handed to another lodger does not work for them, even at home.
func TestDeedRefusedForAnotherAccountEvenAtHome(t *testing.T) {
	setup(t)
	payer := ownerWithBoth(t, 1)
	other := testUser(2, 1000, 0)
	buy(t, other)
	home, _ := HouseOf(2, testBldgId)

	deed := itemOf(payer, testDeedId)
	other.Character.StoreItem(deed)
	UseItem(other, unitRoom(home.EntryRoom()), deed, `north`, `deed north`)

	if after, _ := HouseOf(2, testBldgId); len(after.RoomIds) != 1 {
		t.Error("a deed bought by someone else built a room")
	}
	if !has(other, testDeedId) {
		t.Error("the refused deed was used up")
	}
}

// And the positive control, so the refusals above are known to be refusals
// and not a deed that never works: the same items work in the owner's room.
func TestItemsWorkInTheOwnersOwnRoom(t *testing.T) {
	setup(t)
	u := ownerWithBoth(t, 1)
	entry := unitRoom(testUnitA)

	UseItem(u, entry, itemOf(u, testDeedId), `north`, `deed north`)
	UseItem(u, entry, itemOf(u, testVoucherId), newDesc, `voucher `+newDesc)
	u.GetPrompt().GetNextQuestion().Answer(`yes`)
	UseItem(u, entry, itemOf(u, testVoucherId), newDesc, `voucher `+newDesc)

	h, _ := HouseOf(1, testBldgId)
	if len(h.RoomIds) != 2 || h.Descriptions[testUnitA] != newDesc {
		t.Fatalf("items did not work at home: %+v", h)
	}
	if has(u, testDeedId) || has(u, testVoucherId) {
		t.Error("items were not used up")
	}
}

// A second home is refused however it is asked for (the list's buy home, the
// ask path, or Purchase directly), and costs nothing. Alts share the account
// id, so this covers them too.
func TestSecondHomeRefusedEveryWay(t *testing.T) {
	setup(t)
	u := testUser(1, 5000, 0)
	if res, _ := buy(t, u); res != PurchaseOk {
		t.Fatal("setup: first home")
	}
	gold := u.Character.Gold

	if res, _ := buy(t, u); res != PurchaseAlreadyOwner {
		t.Errorf("Purchase: got %v, want PurchaseAlreadyOwner", res)
	}
	buyKey(u, OfferHome)
	if key, _ := MatchOffer(`home`, true); key == OfferHome {
		Buy(u, func(string) {}, testBldgId, `simple`, key)
	}

	if u.Character.Gold != gold {
		t.Errorf("gold changed from %d to %d", gold, u.Character.Gold)
	}
	if h, _ := HouseOf(1, testBldgId); len(h.RoomIds) != 1 {
		t.Errorf("rooms = %v, want still one", h.RoomIds)
	}
	if len(VacantUnits(testBldgId)) != 2 {
		t.Error("a second unit was taken")
	}
	if o := offerMap(u)[OfferHome]; o.Available || o.Price != 0 {
		t.Errorf("the list still offers a home to an owner: %+v", o)
	}
}
