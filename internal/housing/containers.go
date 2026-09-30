package housing

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"gopkg.in/yaml.v2"
)

// House containers. A container deed places a container in a room of the
// owner's house under a one-word name the owner chooses ("mug", "chest");
// the ordinary container commands (look in, put, get, ...) then work on it,
// because it is an ordinary rooms.Container in the live room. A strongbox
// deed places one only the owner may use.
//
// Where the items live: in the HOUSE RECORD (House.Containers), which is
// living state, never in the room's instance save, which a smoke-test wipe
// clears. The overlay copies them into the room whenever the room is built
// from disk (hydrate), and Capture copies any change back into the record
// and saves it: after every player or mob command run in a house room
// (world.go), and on every room autosave as a backstop (rooms.SetRoomSaveHook).

// Container names: one word, letters only, and nothing a player also types to
// mean something else in the room.
const (
	ContainerNameMin = 2
	ContainerNameMax = 16
)

var reservedContainerNames = map[string]bool{
	`all`: true, `in`: true, `into`: true, `on`: true, `from`: true, `the`: true,
	`a`: true, `an`: true, `my`: true, `me`: true, `self`: true, `here`: true,
	`gold`: true, `coins`: true, `room`: true, `floor`: true, `door`: true,
	`north`: true, `south`: true, `east`: true, `west`: true, `up`: true, `down`: true,
}

func validContainerName(name string) error {
	if len(name) < ContainerNameMin || len(name) > ContainerNameMax {
		return fmt.Errorf(`the name must be one word of %d to %d letters`, ContainerNameMin, ContainerNameMax)
	}
	for _, r := range name {
		if r < 'a' || r > 'z' {
			return fmt.Errorf(`the name must be one word of plain letters`)
		}
	}
	if reservedContainerNames[name] {
		return fmt.Errorf(`%q means something else here; choose another word`, name)
	}
	return nil
}

// ── Placing ────────────────────────────────────────────────────────────────

// useContainerDeed places a container (strongbox when ownerOnly) in the room
// the owner stands in, named by args or, if empty, by a prompt.
func useContainerDeed(user *users.UserRecord, room *rooms.Room, itm items.Item, h House, b Building, ownerOnly bool, args string, rest string, send func(string)) {
	kind := `container`
	if ownerOnly {
		kind = `strongbox`
	}
	if len(h.containersIn(room.RoomId)) >= b.MaxContainersPerRoom {
		send(fmt.Sprintf(`This room already holds as many containers as the letting company allows in one room (%d). Try another of your rooms. The deed stays folded.`, b.MaxContainersPerRoom))
		return
	}

	name := strings.ToLower(strings.TrimSpace(args))
	if name == `` {
		cmdPrompt, _ := user.StartPrompt(`use`, rest)
		q := cmdPrompt.Ask(fmt.Sprintf(`What is your %s? One word, like chest, shelf or vase:`, kind), []string{})
		if !q.Done {
			return
		}
		user.ClearPrompt()
		name = strings.ToLower(strings.TrimSpace(q.Response))
	}
	if strings.ContainsAny(name, " \t") {
		send(`Just one word, please. The deed stays folded.`)
		return
	}
	if err := validContainerName(name); err != nil {
		send(`That will not do: ` + err.Error() + `. The deed stays folded.`)
		return
	}
	if _, taken := room.Exits[name]; taken {
		send(fmt.Sprintf(`%q is already a way out of this room. Choose another word.`, name))
		return
	}
	if _, taken := room.Nouns[name]; taken {
		send(fmt.Sprintf(`There is already a %s here to look at. Choose another word.`, name))
		return
	}
	if _, taken := room.Containers[name]; taken {
		send(fmt.Sprintf(`There is already a %s in this room. Choose another word.`, name))
		return
	}

	mu.Lock()
	cur, owned := roomHouse[room.RoomId]
	if !owned || cur.OwnerUserId != user.UserId {
		mu.Unlock()
		send(`This is not your lodging any more.`)
		return
	}
	if len(cur.containersIn(room.RoomId)) >= b.MaxContainersPerRoom {
		mu.Unlock()
		send(`This room already holds as many containers as the letting company allows in one room. Try another of your rooms.`)
		return
	}
	for _, c := range cur.containersIn(room.RoomId) {
		if c.Name == name {
			mu.Unlock()
			send(fmt.Sprintf(`There is already a %s in this room. Choose another word.`, name))
			return
		}
	}
	next := cur.clone()
	next.Containers = append(next.Containers, HouseContainer{RoomId: room.RoomId, Name: name, OwnerOnly: ownerOnly})
	if err := next.checkLinks(); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.useContainerDeed`, `user`, user.UserId, `error`, err.Error())
		send(`The deed will not take. Tell a member of staff.`)
		return
	}
	if err := saveHouse(next); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.useContainerDeed`, `user`, user.UserId, `error`, err.Error())
		send(`The deed will not take just now. Nothing was used up. Try again later.`)
		return
	}
	nn := next.clone()
	indexLocked(&nn)
	mu.Unlock()

	if user.Character.RemoveItem(itm) {
		events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: itm, Gained: false})
	}
	saveUser(user) // the house is on disk; the spent deed must be too
	if room.Containers == nil {
		room.Containers = map[string]rooms.Container{}
	}
	placed := rooms.Container{}
	if ownerOnly {
		sealContainer(&placed)
	}
	room.Containers[name] = placed
	mudlog.Info(`housing.useContainerDeed`, `user`, user.UserId, `room`, room.RoomId, `name`, name, `ownerOnly`, ownerOnly)

	if ownerOnly {
		send(fmt.Sprintf(`A porter hauls in your %s, sets it down, and fits a lock that knows your palm and nobody else's. Only you can open it. Type <ansi fg="command">look in %s</ansi>, <ansi fg="command">put [item] in %s</ansi> or <ansi fg="command">get [item] from %s</ansi>.`, name, name, name, name))
	} else {
		send(fmt.Sprintf(`A porter hauls in your %s and sets it down. Anyone you let in can use it. Type <ansi fg="command">look in %s</ansi>, <ansi fg="command">put [item] in %s</ansi> or <ansi fg="command">get [item] from %s</ansi>.`, name, name, name, name))
	}
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`A porter hauls in a %s, sets it down, and leaves without a word.`, name), user.UserId)
}

// ── Load and capture ───────────────────────────────────────────────────────

// hydrateContainers makes the live room's containers exactly the house's: the
// house record is their only source of truth, so anything an old instance
// save left behind in a unit room is dropped.
func hydrateContainers(r *rooms.Room, house House) {
	mine := house.containersIn(r.RoomId)
	if len(mine) == 0 && len(r.Containers) == 0 {
		return
	}
	fresh := make(map[string]rooms.Container, len(mine))
	for _, c := range mine {
		rc := rooms.Container{
			Items: append([]items.Item(nil), c.Items...),
			Gold:  c.Gold,
		}
		if c.OwnerOnly {
			sealContainer(&rc)
		}
		fresh[c.Name] = rc
	}
	r.Containers = fresh
}

// hydrateFloor puts a room's recorded floor into the live room. A room whose
// floor has never been captured keeps what its instance save held; the first
// capture adopts it into the record.
func hydrateFloor(r *rooms.Room, house House) {
	f, _, ok := house.floorOf(r.RoomId)
	if !ok {
		return
	}
	r.Items = append([]items.Item(nil), f.Items...)
	r.Stash = append([]items.Item(nil), f.Stash...)
	r.Gold = f.Gold
}

// Capture writes a house room's live floor and container contents back into
// its house record when they differ, and reports whether anything changed.
// It is cheap when nothing did: one comparison per container and one for the
// floor. A container missing from the live room (only something outside
// housing could remove one) is put back from the record rather than lost.
func Capture(r *rooms.Room) bool {
	changed, _ := capture(r)
	return changed
}

// capture is Capture reporting a failed write, so a caller about to replace
// the live room from the record knows the record is behind.
func capture(r *rooms.Room) (changed bool, err error) {
	if r == nil {
		return false, nil
	}
	mu.Lock()
	defer mu.Unlock()
	cur, owned := roomHouse[r.RoomId]
	if !owned {
		return false, nil
	}
	var next *House
	change := func() *House {
		if next == nil {
			n := cur.clone()
			next = &n
		}
		return next
	}

	for i, c := range cur.Containers {
		if c.RoomId != r.RoomId {
			continue
		}
		live, ok := r.Containers[c.Name]
		if !ok {
			if r.Containers == nil {
				r.Containers = map[string]rooms.Container{}
			}
			rc := rooms.Container{Items: append([]items.Item(nil), c.Items...), Gold: c.Gold}
			if c.OwnerOnly {
				sealContainer(&rc)
			}
			r.Containers[c.Name] = rc
			continue
		}
		if live.Gold == c.Gold && sameItems(live.Items, c.Items) {
			continue
		}
		n := change()
		n.Containers[i].Items = append([]items.Item(nil), live.Items...)
		n.Containers[i].Gold = live.Gold
	}

	floor, fi, recorded := cur.floorOf(r.RoomId)
	liveEmpty := len(r.Items) == 0 && len(r.Stash) == 0 && r.Gold == 0
	if (recorded && (floor.Gold != r.Gold || !sameItems(floor.Items, r.Items) || !sameItems(floor.Stash, r.Stash))) ||
		(!recorded && !liveEmpty) {
		n := change()
		fresh := HouseFloor{
			RoomId: r.RoomId,
			Items:  append([]items.Item(nil), r.Items...),
			Stash:  append([]items.Item(nil), r.Stash...),
			Gold:   r.Gold,
		}
		if recorded {
			n.Floors[fi] = fresh
		} else {
			n.Floors = append(n.Floors, fresh)
		}
	}

	if next == nil {
		return false, nil
	}
	if err := saveHouse(*next); err != nil {
		// Keep the live room as it is; the next capture retries.
		mudlog.Error(`housing.Capture`, `room`, r.RoomId, `error`, err.Error())
		return false, err
	}
	nn := next.clone()
	indexLocked(&nn)
	return true, nil
}

// captureAllLoaded captures every loaded room of every house, before a data
// reload replaces the registry from disk. It returns the rooms whose capture
// failed: their live state is newer than their file, so the reload must not
// overwrite it from the file.
func captureAllLoaded() map[int]bool {
	failed := map[int]bool{}
	for _, h := range AllHouses() {
		for _, id := range h.RoomIds {
			if !roomLoaded(id) {
				continue
			}
			if _, err := capture(liveRoom(id)); err != nil {
				failed[id] = true
			}
		}
	}
	return failed
}

// sameItems reports whether two item lists would save the same: the same
// instances (UUID) in the same order, each with the same saved state. The
// saved state is compared as YAML, exactly what the house file would hold, so
// any in-place change to a saved field is noticed and nothing unsaved
// (temporary data) causes a rewrite.
func sameItems(a, b []items.Item) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equals(b[i]) {
			return false
		}
	}
	for i := range a {
		ya, errA := yaml.Marshal(a[i])
		yb, errB := yaml.Marshal(b[i])
		if errA != nil || errB != nil || !bytes.Equal(ya, yb) {
			return false
		}
	}
	return true
}

// saveUser writes an actor's record right after their command moved items
// into or out of a house container, so the item cannot be both in the
// container and still in the player's saved pack after a crash. A seam for
// tests.
var saveUser = func(u *users.UserRecord) {
	if err := users.SaveUser(u); err != nil {
		mudlog.Error(`housing.saveUser`, `user`, u.UserId, `error`, err.Error())
	}
}

// Seams for tests: the live room, whether a room is in memory, a mob's room
// and whether a mob is a companion of a given player.
var (
	liveRoom   = rooms.LoadRoom
	roomLoaded = rooms.IsRoomLoaded
	mobRoom    = func(mobInstanceId int) (int, bool) {
		if m := mobs.GetInstance(mobInstanceId); m != nil {
			return m.Character.RoomId, true
		}
		return 0, false
	}
	mobCharmedBy = func(mobInstanceId int, userId int) bool {
		m := mobs.GetInstance(mobInstanceId)
		return m != nil && m.Character.IsCharmed(userId)
	}
)

// captureHouseAt captures every loaded room of the house that roomId belongs
// to: a command can change a neighbouring room too (something thrown
// through a doorway), and a player who has just walked out of a room left
// its last change behind them. Rooms not in memory are skipped, never
// loaded. It reports whether anything changed.
func captureHouseAt(roomId int) bool {
	h, owned := HouseForRoom(roomId)
	if !owned {
		return false
	}
	changed := false
	for _, id := range h.RoomIds {
		if !roomLoaded(id) {
			continue
		}
		if Capture(liveRoom(id)) {
			changed = true
		}
	}
	return changed
}

// AfterUserCommand captures the house of the room a player is in, and of
// the room they were in before the command if the command moved them (a
// player who drops something and leaves in one line). world.go calls it
// after every player command with the room from before the command.
func AfterUserCommand(userId int, roomBefore int) {
	u := onlineUser(userId)
	if u == nil {
		return
	}
	changed := false
	if IsUnitRoom(u.Character.RoomId) {
		changed = captureHouseAt(u.Character.RoomId)
	}
	if roomBefore != u.Character.RoomId && IsUnitRoom(roomBefore) {
		if captureHouseAt(roomBefore) {
			changed = true
		}
	}
	if changed {
		saveUser(u)
	}
}

// AfterMobCommand captures the house of the room a mob is in, and of the
// room it was in before the command (a companion putting down, picking up
// or carrying something). world.go calls it after every mob command.
func AfterMobCommand(mobInstanceId int, roomBefore int) {
	roomId, ok := mobRoom(mobInstanceId)
	if ok && IsUnitRoom(roomId) {
		captureHouseAt(roomId)
	}
	if roomBefore != roomId && IsUnitRoom(roomBefore) {
		captureHouseAt(roomBefore)
	}
}

// ── Strongboxes ────────────────────────────────────────────────────────────
//
// A strongbox is a rooms.Container marked Sealed and kept under a lock
// (rooms.SealedLockDifficulty) at all times. Every engine path that honours
// container locks (look, get, put, use, tab completion, companions) therefore
// refuses it with no housing code involved, and the paths that could force a
// lock (picklock, unlock with a key, steal, plant) refuse a sealed container
// outright. The owner's own command, and a command of a companion charmed by
// the owner, unlocks the strongboxes of the room they stand in for the length
// of that one command (usercommands.TryCommand, mobcommands.TryCommand). The
// game loop runs one command at a time, so nobody else can act while one is
// open.

// sealContainer shuts a container as a strongbox.
func sealContainer(c *rooms.Container) {
	c.Sealed = true
	c.Lock = gamelock.Lock{Difficulty: rooms.SealedLockDifficulty}
}

// openStrongboxes unlocks every shut strongbox in roomId when the house's
// owner (or staff) is the one acting, and returns what shuts them again.
func openStrongboxes(roomId int, mayOpen func(h House) bool) (shut func()) {
	noop := func() {}
	h, owned := HouseForRoom(roomId)
	if !owned || !mayOpen(h) {
		return noop
	}
	r := liveRoom(roomId)
	if r == nil {
		return noop
	}
	opened := []string{}
	for name, c := range r.Containers {
		if c.IsSealedShut() {
			c.Lock = gamelock.Lock{}
			r.Containers[name] = c
			opened = append(opened, name)
		}
	}
	if len(opened) == 0 {
		return noop // nothing shut, or an outer command already opened them
	}
	return func() {
		for _, name := range opened {
			if c, ok := r.Containers[name]; ok {
				sealContainer(&c)
				r.Containers[name] = c
			}
		}
	}
}

// OpenStrongboxesForUser unlocks the strongboxes of the room a player stands
// in if they own it (or are staff), for the command about to run. The caller
// must call the returned func when the command is done, whatever happens.
func OpenStrongboxesForUser(userId int, roomId int) (shut func()) {
	return openStrongboxes(roomId, func(h House) bool {
		return h.OwnerUserId == userId || isStaff(userId)
	})
}

// OpenStrongboxesForMob does the same for a mob charmed by the owner.
func OpenStrongboxesForMob(mobInstanceId int, roomId int) (shut func()) {
	return openStrongboxes(roomId, func(h House) bool {
		return mobCharmedBy(mobInstanceId, h.OwnerUserId)
	})
}

// CaptureOnSave is the rooms.RoomSaveHook: the autosave backstop, in case a
// container changed outside a command.
func CaptureOnSave(r *rooms.Room) {
	if IsUnitRoom(r.RoomId) {
		Capture(r)
	}
}
