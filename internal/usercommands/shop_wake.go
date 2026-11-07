package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Shopkeepers wake for customers (owner ruling 2026-11-07). This replaces,
// for shopkeepers only, the 2026-07-30 rule that a sleeping NPC refuses
// rather than wakes (actions.RefuseIfAsleep): a keeper asleep in their own
// shop wakes when a player who is not hidden walks in, and wakes and serves a
// customer's shop command. It lives here, not in actions, because waking must
// relight the keeper's lantern through its item tree, and behaviortree
// imports actions.

// relightLantern runs the item_idle tree of the keeper's light slot, so a
// lantern that went dark while they slept relights at once rather than on
// the next round tick. room is where the keeper stands.
func relightLantern(m *mobs.Mob, room *rooms.Room) {
	lamp := m.Character.Equipment.Light
	if lamp.ItemId <= 0 || !lamp.HasBehavior() {
		return
	}
	roomId := m.Character.RoomId
	if room != nil {
		roomId = room.RoomId
	}
	behaviortree.TryItemBehavior(behaviortree.EventContext{EventType: "item_idle"}, behaviortree.ItemSubject{
		UUID: lamp.UUID, ItemId: lamp.ItemId, MobInstanceId: m.InstanceId,
		RoomId: roomId, Slot: "light"})
}

// wakeKeeper wakes one sleeping keeper, relights their lantern, and says so.
// The customer, if any, reads "<keeper> wakes as you come in." when cameIn
// (they just walked in) and "<keeper> wakes up." otherwise; the rest of the
// room reads "<keeper> wakes up.". A nil customer sends no customer line.
func wakeKeeper(m *mobs.Mob, room *rooms.Room, customer *users.UserRecord, cameIn bool) {
	m.Character.CancelConditionsWithFlag(conditions.Sleeping)
	mobs.OnSleeperWoken(&m.Character)
	relightLantern(m, room)
	wokeLine := fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wakes up.`, m.Character.Name)
	exclude := []int{}
	if customer != nil {
		line := wokeLine
		if cameIn {
			line = fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wakes as you come in.`, m.Character.Name)
		}
		customer.SendText(messaging.CategoryMobEmote, line)
		exclude = append(exclude, customer.UserId)
	}
	if room != nil {
		room.SendText(messaging.CategoryMobEmote, wokeLine, exclude...)
	}
}

// keeperInOwnShop reports whether m is a shopkeeper standing in the home
// room their shop belongs to. rooms.FindMerchant also matches a keeper asleep
// elsewhere (a sleeping keeper is never off shift), and a keeper asleep in
// their bedroom is no shopkeeper there: they neither wake for a customer nor
// trade (review 2026-11-07).
func keeperInOwnShop(m *mobs.Mob) bool {
	return m != nil && m.HasShop() && m.Character.RoomId == m.HomeRoomId
}

// sleepingKeepers returns the merchants asleep in room who keep their shop
// there.
func sleepingKeepers(room *rooms.Room) []*mobs.Mob {
	var out []*mobs.Mob
	for _, id := range room.GetMobs(rooms.FindMerchant) {
		if m := mobs.GetInstance(id); keeperInOwnShop(m) && m.Character.HasConditionFlag(conditions.Sleeping) {
			out = append(out, m)
		}
	}
	return out
}

// wakeKeepersOnEntry wakes every sleeping keeper in room when a player who is
// not hidden walks in. A hidden player leaves them asleep (spec).
func wakeKeepersOnEntry(room *rooms.Room, user *users.UserRecord) {
	if room == nil || user == nil || user.Character.IsHidden() {
		return
	}
	for _, m := range sleepingKeepers(room) {
		wakeKeeper(m, room, user, true)
	}
}

// shopOpenFor wakes a sleeping keeper for a shop command, hidden customer or
// not, then applies the usual closed-shop rules (off shift). It reports
// whether the command may go on, the opposite of actions.RefuseIfShopClosed.
func shopOpenFor(room *rooms.Room, user *users.UserRecord) bool {
	if room != nil && actions.ShopClosedForSleep(room) != nil {
		for _, m := range sleepingKeepers(room) {
			wakeKeeper(m, room, user, false)
		}
	}
	return !actions.RefuseIfShopClosed(room, user)
}
