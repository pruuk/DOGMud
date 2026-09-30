package housing

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
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
	if room.Containers == nil {
		room.Containers = map[string]rooms.Container{}
	}
	room.Containers[name] = rooms.Container{}
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
		fresh[c.Name] = rooms.Container{
			Items: append([]items.Item(nil), c.Items...),
			Gold:  c.Gold,
		}
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
	if r == nil {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	cur, owned := roomHouse[r.RoomId]
	if !owned {
		return false
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
			r.Containers[c.Name] = rooms.Container{Items: append([]items.Item(nil), c.Items...), Gold: c.Gold}
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
		return false
	}
	if err := saveHouse(*next); err != nil {
		// Keep the live room as it is; the next capture retries.
		mudlog.Error(`housing.Capture`, `room`, r.RoomId, `error`, err.Error())
		return false
	}
	nn := next.clone()
	indexLocked(&nn)
	return true
}

func sameItems(a, b []items.Item) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equals(b[i]) || a[i].Uses != b[i].Uses {
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

// ── Owner-only containers ──────────────────────────────────────────────────

// Commands that can reach into a container. Anything else ("say nice mug")
// is not a container use and is never refused.
var containerVerbs = map[string]bool{
	`look`: true, `get`: true, `put`: true, `unlock`: true, `lock`: true,
	`picklock`: true, `steal`: true, `plant`: true, `use`: true, `defuse`: true,
	`remove`: true, // "remove ring from coffer" is a get (usercommands.Remove)
}

var containerStopWords = map[string]bool{
	`in`: true, `into`: true, `from`: true, `on`: true, `at`: true, `the`: true,
	`a`: true, `an`: true, `my`: true, `all`: true, `to`: true, `of`: true,
}

// strongboxNamed reports the name of an owner-only container in r that the
// command text would reach, if any. It resolves words with the room's own
// FindContainerByName, exactly as the container commands do, so no spelling
// that reaches a strongbox can slip past.
func strongboxNamed(r *rooms.Room, h House, rest string) (string, bool) {
	private := map[string]bool{}
	for _, c := range h.containersIn(r.RoomId) {
		if c.OwnerOnly {
			private[c.Name] = true
		}
	}
	if len(private) == 0 {
		return ``, false
	}
	candidates := []string{strings.TrimSpace(rest)}
	for _, w := range strings.Fields(strings.ToLower(rest)) {
		w = strings.Trim(w, `.,!?"'()`)
		if len(w) < 1 || containerStopWords[w] {
			continue
		}
		candidates = append(candidates, w)
	}
	for _, c := range candidates {
		if c == `` {
			continue
		}
		if name := r.FindContainerByName(c); name != `` && private[name] {
			return name, true
		}
	}
	return ``, false
}

// GuardUserCommand refuses a player's container command that would reach a
// strongbox they do not own. usercommands.TryCommand calls it after alias
// expansion, before anything else can act.
func GuardUserCommand(userId int, roomId int, cmd string, rest string) (string, bool) {
	if !containerVerbs[cmd] {
		return ``, false
	}
	h, owned := HouseForRoom(roomId)
	if !owned || h.OwnerUserId == userId || isStaff(userId) {
		return ``, false
	}
	r := liveRoom(roomId)
	if r == nil {
		return ``, false
	}
	if name, hit := strongboxNamed(r, h, rest); hit {
		return fmt.Sprintf(`The <ansi fg="container">%s</ansi> is locked, and it opens for its owner and nobody else.`, name), true
	}
	return ``, false
}

// GuardMobCommand refuses a mob's command that would reach a strongbox,
// unless the mob is a companion of the house's owner. Mobs have no verb
// list: any command that names a strongbox is refused.
func GuardMobCommand(mobInstanceId int, roomId int, rest string) bool {
	h, owned := HouseForRoom(roomId)
	if !owned {
		return false
	}
	if mobCharmedBy(mobInstanceId, h.OwnerUserId) {
		return false
	}
	r := liveRoom(roomId)
	if r == nil {
		return false
	}
	_, hit := strongboxNamed(r, h, rest)
	return hit
}

// CaptureOnSave is the rooms.RoomSaveHook: the autosave backstop, in case a
// container changed outside a command.
func CaptureOnSave(r *rooms.Room) {
	if IsUnitRoom(r.RoomId) {
		Capture(r)
	}
}
