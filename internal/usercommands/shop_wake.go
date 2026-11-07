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

// wakeKeeper wakes one sleeping keeper, relights their lantern at once (it
// went dark while they slept), and says so to the room.
func wakeKeeper(m *mobs.Mob, room *rooms.Room, customer *users.UserRecord) {
	m.Character.CancelConditionsWithFlag(conditions.Sleeping)
	mobs.OnSleeperWoken(&m.Character)
	if lamp := m.Character.Equipment.Light; lamp.ItemId > 0 && lamp.HasBehavior() {
		behaviortree.TryItemBehavior(behaviortree.EventContext{EventType: "item_idle"}, behaviortree.ItemSubject{
			UUID: lamp.UUID, ItemId: lamp.ItemId, MobInstanceId: m.InstanceId,
			RoomId: m.Character.RoomId, Slot: "light"})
	}
	exclude := []int{}
	if customer != nil {
		customer.SendText(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wakes as you come in.`, m.Character.Name))
		exclude = append(exclude, customer.UserId)
	}
	room.SendText(messaging.CategoryMobEmote,
		fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wakes up.`, m.Character.Name), exclude...)
}

// sleepingKeepers returns the merchants asleep in room.
func sleepingKeepers(room *rooms.Room) []*mobs.Mob {
	var out []*mobs.Mob
	for _, id := range room.GetMobs(rooms.FindMerchant) {
		if m := mobs.GetInstance(id); m != nil && m.Character.HasConditionFlag(conditions.Sleeping) {
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
		wakeKeeper(m, room, user)
	}
}

// shopOpenFor wakes a sleeping keeper for a shop command, hidden customer or
// not, then applies the usual closed-shop rules (off shift). It reports
// whether the command may go on, the opposite of actions.RefuseIfShopClosed.
func shopOpenFor(room *rooms.Room, user *users.UserRecord) bool {
	if room != nil && actions.ShopClosedForSleep(room) != nil {
		for _, m := range sleepingKeepers(room) {
			wakeKeeper(m, room, user)
		}
	}
	return !actions.RefuseIfShopClosed(room, user)
}
