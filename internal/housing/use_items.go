package housing

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Owner-written descriptions are plain prose: tags are escaped, whitespace is
// folded, and the length is bounded so a room still reads like a room.
const (
	DescriptionMinLen = 20
	DescriptionMaxLen = 1000
)

// UseItem handles "use <deed|voucher> [args]". rest is the whole text after
// "use", which the prompt system needs to re-run the command with each answer.
// It reports false when itm is not a housing item, so the caller carries on.
func UseItem(user *users.UserRecord, room *rooms.Room, itm items.Item, args string, rest string) bool {
	if !IsHousingItem(itm.ItemId) {
		return false
	}
	send := func(s string) { user.SendText(messaging.CategorySystem, util.SplitStringNL(s, 80)) }

	h, inHouse := HouseForRoom(room.RoomId)
	if !inHouse || h.OwnerUserId != user.UserId {
		send(fmt.Sprintf(`You can only use the <ansi fg="itemname">%s</ansi> inside your own lodging.`, itm.DisplayName()))
		return true
	}
	b, ok := GetBuilding(h.BuildingId)
	if !ok {
		return true
	}

	switch itm.ItemId {
	case b.ExtensionItemId:
		useExtension(user, room, itm, h, b, args, rest, send)
	case b.RedecorateItemId:
		useRedecorate(user, room, itm, h, args, rest, send)
	default:
		send(fmt.Sprintf(`The <ansi fg="itemname">%s</ansi> was issued for a different building.`, itm.DisplayName()))
	}
	return true
}

// directionOrder is the order walls are offered in the deed's menu.
var directionOrder = []string{`north`, `east`, `south`, `west`, `up`, `down`}

// freeWalls lists the directions from roomId where a new room could go: no
// exit that way already, and no other room of the house in that cell.
func freeWalls(room *rooms.Room, h House) []string {
	offsets := h.offsets()
	here := offsets[room.RoomId]
	occupied := map[[3]int]bool{}
	for _, pos := range offsets {
		occupied[pos] = true
	}
	free := []string{}
	for _, dir := range directionOrder {
		if _, taken := room.Exits[dir]; taken {
			continue
		}
		d := directionDeltas[dir]
		if occupied[[3]int{here[0] + d[0], here[1] + d[1], here[2] + d[2]}] {
			continue
		}
		free = append(free, dir)
	}
	return free
}

func useExtension(user *users.UserRecord, room *rooms.Room, itm items.Item, h House, b Building, args string, rest string, send func(string)) {
	if itm.BoundUserId != 0 && itm.BoundUserId != user.UserId {
		send(`The deed is made out to somebody else, in a clerk's hand that does not smudge. It is no use to you.`)
		return
	}
	if len(h.RoomIds) >= b.MaxRooms {
		send(`Your lodging is already as large as the letting company allows. The deed stays folded.`)
		return
	}
	free := freeWalls(room, h)

	// "use deed" alone offers a menu of the walls that can take a room;
	// "use deed north" goes straight there.
	if strings.TrimSpace(args) == `` {
		if len(free) == 0 {
			send(`Every wall of this room already opens onto something. Try the deed in another of your rooms.`)
			return
		}
		cmdPrompt, _ := user.StartPrompt(`use`, rest)
		q := cmdPrompt.Ask(`Which way should the new room go?`, append(append([]string{}, free...), `cancel`), `cancel`)
		if !q.Done {
			return
		}
		user.ClearPrompt()
		args = q.Response
		if strings.EqualFold(args, `cancel`) {
			send(`You fold the deed away for another day.`)
			return
		}
	}

	dir, ok := ParseDirection(args)
	if !ok {
		send(`Use the deed toward the wall you want opened: <ansi fg="command">use deed north</ansi> (or south, east, west, up, down), or just <ansi fg="command">use deed</ansi> to choose from a list.`)
		return
	}
	if _, taken := room.Exits[dir]; taken {
		send(fmt.Sprintf(`There is already a way %s from here. Pick somewhere bare.`, dir))
		return
	}
	isFree := false
	for _, f := range free {
		if f == dir {
			isFree = true
		}
	}
	if !isFree {
		send(fmt.Sprintf(`Beyond %s is another of your own rooms already. Build somewhere else.`, wallPhrase(dir)))
		return
	}

	// Choose, write and publish under the registry lock, so two lodgers
	// extending in the same moment cannot be handed the same room. The house
	// is re-read here and the checks that depend on it repeated.
	mu.Lock()
	cur, still := ownerHouse[ownerKey{b.BuildingId, user.UserId}]
	dirTaken := false
	if still {
		_, dirTaken = cur.exitsOf(room.RoomId)[dir]
	}
	if !still || !cur.HasRoom(room.RoomId) || dirTaken || len(cur.RoomIds) >= b.MaxRooms {
		mu.Unlock()
		send(`Something changed while you were unfolding the deed. Try again.`)
		return
	}
	vacant := vacantUnitsLocked(b.BuildingId)
	if len(vacant) == 0 {
		mu.Unlock()
		send(`Every room in the building is let, so there is nothing to knock through into. Keep the deed and try again later.`)
		return
	}
	newRoom := vacant[0]
	next := cur.clone()
	next.RoomIds = append(next.RoomIds, newRoom)
	next.Links = append(next.Links, RoomLink{From: room.RoomId, Direction: dir, To: newRoom})
	if err := next.checkLinks(); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.useExtension`, `user`, user.UserId, `error`, err.Error())
		send(`The deed will not take. Tell a member of staff.`)
		return
	}
	if err := saveHouse(next); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.useExtension`, `user`, user.UserId, `error`, err.Error())
		send(`The deed will not take just now. Nothing was used up. Try again later.`)
		return
	}
	nn := next.clone()
	indexLocked(&nn)
	mu.Unlock()

	if user.Character.RemoveItem(itm) {
		events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: itm, Gained: false})
	}
	ApplyOverlay(room) // the room the lodger stands in, whatever LoadRoom returns
	refreshHouse(next, newRoom)
	mudlog.Info(`housing.useExtension`, `user`, user.UserId, `building`, b.BuildingId, `from`, room.RoomId, `direction`, dir, `newRoom`, newRoom)

	send(fmt.Sprintf(`You press the deed flat against %s. It is taken from your hand by a workman who was, apparently, waiting outside. An hour of dust, hammering and cheerful swearing later, %s to a new room.`, wallPhrase(dir), openingPhrase(dir)))
	room.SendTextVisual(messaging.CategoryMobEmote,
		fmt.Sprintf(`Workmen troop in, knock through %s, and leave again, trailing plaster dust.`, wallPhrase(dir)), user.UserId)
	user.Command(`look`)
}

func useRedecorate(user *users.UserRecord, room *rooms.Room, itm items.Item, h House, args string, rest string, send func(string)) {
	cmdPrompt, _ := user.StartPrompt(`use`, rest)

	text := strings.TrimSpace(args)
	if text == `` {
		q := cmdPrompt.Ask(`Write the new description of this room, as one paragraph:`, []string{})
		if !q.Done {
			return
		}
		text = q.Response
	}
	text = cleanDescription(text)
	if n := len(text); n < DescriptionMinLen || n > DescriptionMaxLen {
		send(fmt.Sprintf(`A description must be between %d and %d characters long. Yours has %d. The voucher is still yours.`, DescriptionMinLen, DescriptionMaxLen, n))
		user.ClearPrompt()
		return
	}

	confirm := cmdPrompt.Ask(`Spend your voucher on this description?`, []string{`yes`, `no`}, `no`)
	if !confirm.Done {
		// Starts on a fresh line: the prompt line before it has no newline.
		user.SendText(messaging.CategorySystem, term.CRLFStr+`Here is how this room would read.`)
		send(text)
		if h.Descriptions[room.RoomId] != `` {
			send(`It replaces the description you wrote before.`)
		}
		return
	}
	user.ClearPrompt()
	if !strings.HasPrefix(strings.ToLower(confirm.Response), `y`) {
		send(`You fold the voucher away for another day.`)
		return
	}

	// The voucher might have left the backpack while the question was open.
	if _, still := user.Character.FindInBackpackWhere(itm.GetSpec().Name, func(i items.Item) bool { return i.Equals(itm) }); !still {
		send(`You no longer have the voucher.`)
		return
	}

	mu.Lock()
	cur, owned := roomHouse[room.RoomId]
	if !owned || cur.OwnerUserId != user.UserId {
		mu.Unlock()
		send(`This is not your room any more.`)
		return
	}
	next := cur.clone()
	if next.Descriptions == nil {
		next.Descriptions = map[int]string{}
	}
	next.Descriptions[room.RoomId] = text
	if err := saveHouse(next); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.useRedecorate`, `user`, user.UserId, `error`, err.Error())
		send(`The voucher will not take just now. Nothing was used up. Try again later.`)
		return
	}
	nn := next.clone()
	indexLocked(&nn)
	mu.Unlock()

	if user.Character.RemoveItem(itm) {
		events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: itm, Gained: false})
	}
	ApplyOverlay(room)
	mudlog.Info(`housing.useRedecorate`, `user`, user.UserId, `room`, room.RoomId, `length`, len(text))

	send(`You hand the voucher to the decorator who has appeared at your elbow. She reads it, sniffs, and gets to work. By the time the paint smell fades, the room looks the way you wrote it.`)
	room.SendTextVisual(messaging.CategoryMobEmote, `A decorator bustles through with brushes and a ladder, and the room is not quite the room it was.`, user.UserId)
	user.Command(`look`)
}

// wallPhrase names the surface a deed is used on.
func wallPhrase(dir string) string {
	switch dir {
	case `up`:
		return `the ceiling`
	case `down`:
		return `the floor`
	}
	return `the ` + dir + ` wall`
}

// openingPhrase describes the new way through, finishing "... later, %s to a
// new room."
func openingPhrase(dir string) string {
	switch dir {
	case `up`:
		return `a narrow stair climbs up`
	case `down`:
		return `a narrow stair leads down`
	}
	return `a doorway stands open to the ` + dir
}

// cleanDescription makes player text safe to show as a room description:
// markup is escaped and all whitespace, newlines included, folds to single
// spaces so the room display wraps it like any other description.
func cleanDescription(s string) string {
	return strings.Join(strings.Fields(util.EscapeAnsiTags(s)), ` `)
}
