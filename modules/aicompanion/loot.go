package aicompanion

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Two mob commands the engine does not have, registered by this module so
// a bonded companion can do what a player does with bodies and containers.
// They mirror usercommands/loot.go and the container branch of
// usercommands/get.go, with the same gates:
//
//   - companion-loot takes from a body only what the companion's OWNER is
//     entitled to (kill ownership and party loot mode), never more.
//   - companion-takeout takes from an unhidden, unlocked container.
//
// Both find their target by identity (corpse fields, container name and item
// id) chosen by the module from the companion's own scene, never by a
// creature name lookup.

const (
	cmdCompanionLoot    = `companion-loot`
	cmdCompanionTakeout = `companion-takeout`
	cmdCompanionUnlock  = `companion-unlock`
	cmdCompanionBuy     = `companion-buy`
	cmdCompanionFollow  = `companion-follow`
)

// corpseRef encodes a corpse's identity for companion-loot.
func corpseRef(c *rooms.Corpse) string {
	return fmt.Sprintf(`%d:%d:%d`, c.MobId, c.UserId, c.RoundCreated)
}

// corpseAt is the corpse with this identity in the room, or nil.
func corpseAt(room *rooms.Room, ref string) *rooms.Corpse {
	if room == nil {
		return nil
	}
	if i := findCorpseByRef(room, ref); i >= 0 {
		return &room.Corpses[i]
	}
	return nil
}

// findCorpseByRef returns the index of the corpse with this identity.
func findCorpseByRef(room *rooms.Room, ref string) int {
	for i := range room.Corpses {
		if !room.Corpses[i].Prunable && corpseRef(&room.Corpses[i]) == ref {
			return i
		}
	}
	return -1
}

// mobCompanionLoot is the companion-loot mob command.
func mobCompanionLoot(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	if !module.cfg.Enabled {
		return true, nil
	}
	ownerId := mob.Character.GetCharmedUserId()
	if ownerId == 0 || room == nil {
		return true, nil
	}
	idx := findCorpseByRef(room, strings.TrimSpace(rest))
	if idx < 0 {
		return true, nil
	}
	corpse := &room.Corpses[idx]
	now := util.GetRoundCount()
	if !corpse.LootAllowed(ownerId, now) || !corpse.HasLoot() {
		return true, nil
	}

	took := false
	for _, it := range append([]items.Item{}, corpse.Loot.Items...) {
		if !corpse.CanTakeItem(it.UUID.String(), ownerId, now) {
			continue
		}
		if !mob.Character.StoreItem(it) {
			break // too heavy to carry more
		}
		events.AddToQueue(events.ItemOwnership{MobInstanceId: mob.InstanceId, Item: it, Gained: true})
		corpse.Loot.RemoveItem(it)
		took = true
	}
	if corpse.Loot.Gold > 0 {
		amt := corpse.Loot.Gold
		corpse.Loot.Gold = 0
		// Gold follows the owner's rules, not the companion's: in a party it
		// goes into the shared pool, exactly as usercommands/loot.go does
		// for a player. Solo, the companion carries it.
		if p := parties.Get(ownerId); p != nil {
			p.AddGold(amt)
		} else {
			mob.Character.Gold += amt
		}
		took = true
	}
	if took {
		mob.Character.CancelConditionsWithFlag(conditions.Hidden)
		room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> loots the <ansi fg="mob-corpse">%s</ansi>.`, mob.Character.Name, corpse.DisplayName()))
	}
	return true, nil
}

// mobCompanionTakeout is the companion-takeout mob command:
// "companion-takeout <itemId>:<uuid> <container name>". The uuid names the
// exact instance, so two identical items in one chest cannot be confused.
func mobCompanionTakeout(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	if !module.cfg.Enabled {
		return true, nil
	}
	if room == nil {
		return true, nil
	}
	parts := strings.SplitN(strings.TrimSpace(rest), ` `, 2)
	if len(parts) != 2 {
		return true, nil
	}
	idPart, uuidPart, _ := strings.Cut(parts[0], `:`)
	itemId, err := strconv.Atoi(idPart)
	if err != nil || itemId <= 0 {
		return true, nil
	}
	name := parts[1]
	container, ok := room.Containers[name]
	if !ok || container.Hidden || container.Lock.IsLocked() {
		return true, nil
	}
	// A merchant's chest is not the companion's to empty: taking from it is
	// theft, watched by the room (actions.WatchMerchantChest), and a
	// companion errand must not be a way around that.
	if actions.IsMerchantChest(room, name) {
		return true, nil
	}
	it, found := findContainerItem(container, itemId, uuidPart)
	if !found || !mob.Character.StoreItem(it) {
		return true, nil
	}
	events.AddToQueue(events.ItemOwnership{MobInstanceId: mob.InstanceId, Item: it, Gained: true})
	container.RemoveItem(it)
	room.Containers[name] = container

	mob.Character.CancelConditionsWithFlag(conditions.Hidden)
	room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> takes the <ansi fg="itemname">%s</ansi> from the <ansi fg="container">%s</ansi>.`,
		mob.Character.Name, it.DisplayName(), name))
	return true, nil
}

// mobCompanionUnlock opens a locked exit with a key the companion carries,
// the key half of usercommands/unlock.go. Picking locks is a player skill
// path with its own minigame and is deliberately not copied here: without a
// key the way stays shut and the route is planned around it.
func mobCompanionUnlock(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	if !module.cfg.Enabled {
		return true, nil
	}
	if room == nil {
		return true, nil
	}
	exitName, _ := room.FindExitByName(strings.TrimSpace(rest))
	if exitName == `` {
		return true, nil
	}
	exitInfo, ok := room.GetExitInfo(exitName)
	if !ok || !exitInfo.Lock.IsLocked() {
		return true, nil
	}
	lockId := fmt.Sprintf(`%d-%s`, room.RoomId, exitName)
	if _, hasKey := mob.Character.FindKeyInBackpack(lockId); !hasKey {
		return true, nil
	}
	exitInfo.Lock.SetUnlocked()
	room.SetExitLock(exitName, false)
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="mobname">%s</ansi> uses a key to unlock the <ansi fg="exit">%s</ansi> lock.`, mob.Character.Name, exitName))
	return true, nil
}

// findContainerItem picks one exact item out of a container: the instance
// with this uuid when one is named, and otherwise the first of that kind.
func findContainerItem(container rooms.Container, itemId int, uuid string) (items.Item, bool) {
	if uuid != `` {
		for _, it := range container.Items {
			if it.ItemId == itemId && it.UUID.String() == uuid {
				return it, true
			}
		}
	}
	return container.FindItemById(itemId)
}

// mobCompanionBuy is "companion-buy <merchantInstanceId> <qty> <item name>".
// The ordinary buy command picks whichever merchant in the room will sell
// the thing; a companion has already read one merchant's stock and checked
// its purse against that merchant's price, so it buys from that one or not
// at all.
func mobCompanionBuy(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	if !module.cfg.Enabled {
		return true, nil
	}
	if room == nil {
		return true, nil
	}
	parts := strings.SplitN(strings.TrimSpace(rest), ` `, 3)
	if len(parts) != 3 {
		return true, nil
	}
	merchantId, err := strconv.Atoi(parts[0])
	if err != nil || merchantId <= 0 {
		return true, nil
	}
	if merchant := mobs.GetInstance(merchantId); merchant == nil || merchant.Character.RoomId != room.RoomId {
		return true, nil
	}
	actions.Buy(actions.NewMobActorInRoom(mob, room), actions.BuyOptions{
		Request:                     parts[1] + ` ` + parts[2],
		TargetMerchantMobInstanceId: merchantId,
	})
	return true, nil
}

// mobCompanionFollow is "companion-follow <fromRoomId> <toRoomId>": the
// step a companion takes a moment after its owner walks out, so the rooms
// either side see an ordinary departure and arrival.
//
// It is issued with a delay, and the world can change in that moment: the
// owner may walk on again and the engine carry her along, or she may be
// dragged into a fight. So the step checks, at the moment it runs, that it
// is still standing where it set off from and that its owner is still next
// door. If either has changed it does nothing, which is what stops a
// companion walking one room past its owner.
func mobCompanionFollow(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	if room == nil {
		return true, nil
	}
	parts := strings.Fields(strings.TrimSpace(rest))
	if len(parts) != 2 {
		return true, nil
	}
	fromRoom, err1 := strconv.Atoi(parts[0])
	toRoom, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return true, nil
	}
	if mob.Character.RoomId != fromRoom || mob.Character.RoomId == toRoom {
		return true, nil // already moved, by the engine or otherwise
	}
	owner := users.GetByUserId(mob.Character.GetCharmedUserId())
	if owner == nil || owner.Character == nil || owner.Character.RoomId != toRoom {
		return true, nil // they did not stay where they went
	}
	exitName := room.FindExitTo(toRoom)
	if exitName == `` {
		return true, nil
	}
	mob.Command(`go ` + util.EscapeAnsiTags(exitName))
	return true, nil
}
