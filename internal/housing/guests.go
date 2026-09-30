package housing

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Guests. The owner buys a guest key made out to their house
// (items.Item.HouseKeyOwner) and gives it to a friend. The friend presents it
// at the building's door, once: the house records them as a guest and the key
// is spent. From then on the house record alone decides; nothing need be
// carried, a lost key grants nothing, and the owner revokes by name. Like the
// owner, a guest is an ACCOUNT.

// OfferGuestKey is the landlord's list key for a guest key.
const OfferGuestKey = `key`

// moveUser is how a player who has lost access is put back outside. A seam
// for tests, which have no room manager.
var moveUser = func(userId int, roomId int) error { return rooms.MoveToRoom(userId, roomId) }

// onlineUser finds a logged-in account. A seam for tests.
var onlineUser = users.GetByUserId

func buildingForKey(itemId int) (Building, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, b := range buildings {
		if b.GuestKeyItemId == itemId {
			return *b, true
		}
	}
	return Building{}, false
}

// BuildingForDoor returns the building whose shared door is in roomId.
func BuildingForDoor(roomId int) (Building, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, id := range doorBuilding[roomId] {
		if b := buildings[id]; b != nil {
			return *b, true
		}
	}
	return Building{}, false
}

// HousesOwnedBy returns every house userId owns, one per building.
func HousesOwnedBy(userId int) []House {
	out := []House{}
	for _, h := range AllHouses() {
		if h.OwnerUserId == userId {
			out = append(out, h)
		}
	}
	return out
}

// GuestOf returns every house in any building where userId is a guest.
func GuestOf(userId int) []House {
	out := []House{}
	for _, h := range AllHouses() {
		if h.IsGuest(userId) {
			out = append(out, h)
		}
	}
	return out
}

// buyGuestKey sells a key made out to the buyer's own house. Nothing is
// recorded until the key is used, so an unused key costs the house nothing.
func buyGuestKey(user *users.UserRecord, say func(string), buildingId string) {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return
	}
	house, owns := HouseOf(user.UserId, b.BuildingId)
	if !owns {
		say(b.Line(`key.no_home`))
		return
	}
	if len(house.Guests) >= b.MaxGuests {
		say(b.Line(`key.max`))
		return
	}
	if user.Character.Gold+user.Character.Bank < b.GuestKeyPrice {
		say(b.Line(`key.no_gold`, `price`, b.GuestKeyPrice))
		return
	}
	key := items.New(b.GuestKeyItemId)
	if key.ItemId == 0 {
		mudlog.Error(`housing.buyGuestKey`, `building`, b.BuildingId, `error`, `guest key item does not exist`)
		return
	}
	key.HouseKeyOwner = user.UserId
	if !user.Character.StoreItem(key) {
		say(b.Line(`key.too_heavy`))
		return
	}
	fromGold, fromBank := chargeGold(user, b.GuestKeyPrice)
	events.AddToQueue(events.EquipmentChange{UserId: user.UserId, GoldChange: -fromGold, BankChange: -fromBank})
	events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: key, Gained: true})
	say(b.Line(`key.sold`, `price`, b.GuestKeyPrice))
}

// useGuestKey handles "use guest key" at a building's door.
func useGuestKey(user *users.UserRecord, room *rooms.Room, itm items.Item, b Building, send func(string)) {
	host := itm.HouseKeyOwner
	if host == 0 {
		send(`The key was never made out to anybody's lodging. The lock will not take it.`)
		return
	}
	if room.RoomId != b.DoorRoom {
		send(fmt.Sprintf(`Present the key at the door of %s. The lock there is the only one that knows it.`, b.Name))
		return
	}
	if host == user.UserId {
		send(`It is a key to your own lodging. Give it to someone you want to let in.`)
		return
	}

	mu.Lock()
	cur, owned := ownerHouse[ownerKey{b.BuildingId, host}]
	switch {
	case !owned:
		mu.Unlock()
		send(`The lock does not know the lodging this key was cut for any more.`)
		return
	case cur.IsGuest(user.UserId):
		mu.Unlock()
		send(`The lock already knows your palm for that lodging. Keep the key, or give it back.`)
		return
	case len(cur.Guests) >= b.MaxGuests:
		mu.Unlock()
		send(`The lock will not take another palm. That lodging already admits as many guests as the letting company allows.`)
		return
	}
	next := cur.clone()
	next.Guests = append(next.Guests, Guest{UserId: user.UserId, Name: user.Character.Name, AddedAt: now().UTC()})
	if err := saveHouse(next); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.useGuestKey`, `user`, user.UserId, `host`, host, `error`, err.Error())
		send(`The lock will not take the key just now. Nothing was used up. Try again later.`)
		return
	}
	nn := next.clone()
	indexLocked(&nn)
	mu.Unlock()

	if user.Character.RemoveItem(itm) {
		events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: itm, Gained: false})
	}
	saveUser(user) // the house is on disk; the spent key must be too
	mudlog.Info(`housing.useGuestKey`, `guest`, user.UserId, `host`, host, `building`, b.BuildingId)
	send(fmt.Sprintf(`You press the key into the brass plate. It sinks in and is gone, and the lock ticks, once. It knows your palm now. You can visit %s's lodging through the %s whenever you like.`, next.OwnerName, b.DoorExit))
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> presses a small brass key into the plate of the %s. The lock ticks.`, user.Character.Name, b.DoorExit), user.UserId)
	if owner := onlineUser(host); owner != nil {
		owner.SendText(messaging.CategorySystem, util.SplitStringNL(fmt.Sprintf(`The lock of your lodging has learned a new palm: <ansi fg="username">%s</ansi> can come and go now. Type <ansi fg="command">house guests</ansi> to see everyone who can.`, user.Character.Name), 80))
	}
}

// matchGuest finds a guest of h by name, case-insensitive, whole name first
// and then a unique prefix.
func matchGuest(h House, name string) (Guest, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == `` {
		return Guest{}, false
	}
	for _, g := range h.Guests {
		if strings.ToLower(g.Name) == name {
			return g, true
		}
	}
	var found []Guest
	for _, g := range h.Guests {
		if strings.HasPrefix(strings.ToLower(g.Name), name) {
			found = append(found, g)
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return Guest{}, false
}

// Revoke takes a guest's access away from ownerId's house(s). The house is
// written first; then, if the guest is inside, they are put outside the
// building's door. It returns the guest removed, or an error to show.
// It takes the guest's access away from every lodging of the owner's that
// they hold a key to (a lodger with homes in two cities may have given the
// same friend a key to both), and returns the names of those buildings.
func Revoke(ownerId int, name string) (Guest, []string, error) {
	var revoked Guest
	where := []string{}
	for _, h := range HousesOwnedBy(ownerId) {
		g, ok := matchGuest(h, name)
		if !ok || (revoked.UserId != 0 && g.UserId != revoked.UserId) {
			continue
		}
		if err := removeGuest(h.BuildingId, ownerId, g.UserId); err != nil {
			return Guest{}, nil, err
		}
		b, _ := GetBuilding(h.BuildingId)
		if u := onlineUser(g.UserId); u != nil {
			u.SendText(messaging.CategorySystem, util.SplitStringNL(fmt.Sprintf(`The lock of %s's lodging in %s no longer knows your palm.`, h.OwnerName, b.Name), 80))
		}
		ejectIfInside(g.UserId, h, b)
		revoked = g
		where = append(where, b.Name)
	}
	if revoked.UserId == 0 {
		return Guest{}, nil, fmt.Errorf(`no guest of yours is called %q`, strings.TrimSpace(name))
	}
	return revoked, where, nil
}

// Leave gives up userId's guest access to the house of the owner named. It
// returns the house left, or an error to show.
// A guest of one owner's lodgings in two cities gives up both. It returns one
// of the houses left and the names of their buildings.
func Leave(userId int, ownerName string) (House, []string, error) {
	want := strings.ToLower(strings.TrimSpace(ownerName))
	exact, prefix := map[int][]House{}, map[int][]House{}
	for _, h := range GuestOf(userId) {
		name := strings.ToLower(h.OwnerName)
		if name == want {
			exact[h.OwnerUserId] = append(exact[h.OwnerUserId], h)
		} else if want != `` && strings.HasPrefix(name, want) {
			prefix[h.OwnerUserId] = append(prefix[h.OwnerUserId], h)
		}
	}
	owners := exact
	if len(owners) == 0 {
		owners = prefix
	}
	if len(owners) != 1 {
		return House{}, nil, fmt.Errorf(`you are not a guest of anyone called %q`, strings.TrimSpace(ownerName))
	}
	var left House
	where := []string{}
	for _, houses := range owners {
		for _, h := range houses {
			if err := removeGuest(h.BuildingId, h.OwnerUserId, userId); err != nil {
				return House{}, nil, err
			}
			b, _ := GetBuilding(h.BuildingId)
			ejectIfInside(userId, h, b)
			left = h
			where = append(where, b.Name)
		}
	}
	return left, where, nil
}

// removeGuest writes the house without guestId, then publishes it.
func removeGuest(buildingId string, ownerId int, guestId int) error {
	mu.Lock()
	defer mu.Unlock()
	cur, ok := ownerHouse[ownerKey{buildingId, ownerId}]
	if !ok {
		return fmt.Errorf(`that lodging no longer exists`)
	}
	next := cur.clone()
	next.Guests = next.Guests[:0]
	for _, g := range cur.Guests {
		if g.UserId != guestId {
			next.Guests = append(next.Guests, g)
		}
	}
	if err := saveHouse(next); err != nil {
		mudlog.Error(`housing.removeGuest`, `owner`, ownerId, `guest`, guestId, `error`, err.Error())
		return fmt.Errorf(`the lock will not change just now; try again later`)
	}
	nn := next.clone()
	indexLocked(&nn)
	mudlog.Info(`housing.removeGuest`, `owner`, ownerId, `guest`, guestId, `building`, buildingId)
	return nil
}

// ejectIfInside puts a player who may no longer be in h outside the
// building's door, if they are online and standing in one of its rooms.
// Players offline inside are handled at login by GuardEntry's redirect.
func ejectIfInside(userId int, h House, b Building) {
	u := onlineUser(userId)
	if u == nil || !h.HasRoom(u.Character.RoomId) || b.DoorRoom == 0 {
		return
	}
	if err := moveUser(userId, b.DoorRoom); err != nil {
		mudlog.Error(`housing.ejectIfInside`, `user`, userId, `error`, err.Error())
		return
	}
	u.SendText(messaging.CategorySystem, util.SplitStringNL(b.OutsideText, 80))
	u.Command(`look`)
}
