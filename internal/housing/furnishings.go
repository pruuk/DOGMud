package housing

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Furnishings: a bed deed puts a bed in one room of the owner's lodging, and
// a station deed installs one crafting station there. Each room takes at most
// one of each. Like containers, they are placed only by the owner, inside
// their own lodging, with a deed from that lodging's own building. Once in,
// anyone let into the lodging may use them: sleep in the bed, craft at the
// station.
//
// Both live in the HOUSE RECORD (House.Beds, House.Stations). What they do to
// a room is laid on by the overlay: a station sets rooms.Room.Station, which
// the craft commands already read, and each adds a noun and a sentence to the
// room's description. A bed changes sleep: actions.Sleep, told the sleeper is
// in a bed (RoomHasBed), adds the Sleeping in a Bed condition, whose statmods
// double the sleeping regen.

// HouseStation is one crafting station installed in a house.
type HouseStation struct {
	RoomId  int    `yaml:"room_id"`
	Station string `yaml:"station"` // a recipe station id: forge, loom, ...
}

// stationTypes lists every station a recipe needs, sorted: what a station
// deed can install. A variable so tests need no recipe files.
var stationTypes = func() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range crafting.GetAll() {
		if r.Station != `` && !seen[r.Station] {
			seen[r.Station] = true
			out = append(out, r.Station)
		}
	}
	sort.Strings(out)
	return out
}

// StationName is how a station id reads to a player: "alchemy_bench" is
// "alchemy bench".
func StationName(station string) string {
	return strings.ReplaceAll(station, `_`, ` `)
}

// stationNoun is the word a player looks at a station by: the last word of
// its name ("look bench", "look forge").
func stationNoun(station string) string {
	words := strings.Fields(StationName(station))
	if len(words) == 0 {
		return station
	}
	return words[len(words)-1]
}

// furnishingNouns are words a bed or any station may take as a room noun, so
// no container may be named with one (reservedContainerNames).
var furnishingNouns = []string{`bed`, `forge`, `bench`, `circle`, `fire`, `loom`, `station`}

// validStationId is the shape of a station id: lower case words joined by
// underscores.
func validStationId(s string) bool {
	if s == `` || strings.HasPrefix(s, `_`) || strings.HasSuffix(s, `_`) {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '_' {
			return false
		}
	}
	return true
}

// HasBed reports whether roomId holds a bed.
func (h House) HasBed(roomId int) bool {
	for _, id := range h.Beds {
		if id == roomId {
			return true
		}
	}
	return false
}

// StationIn returns the station installed in roomId, if any.
func (h House) StationIn(roomId int) (string, bool) {
	for _, s := range h.Stations {
		if s.RoomId == roomId {
			return s.Station, true
		}
	}
	return ``, false
}

// furnishingSpace is how many rooms of the house still lack a bed (kind bed)
// or a station (kind station).
func (h House) furnishingSpace(kind string) int {
	n := 0
	for _, roomId := range h.RoomIds {
		if kind == OfferBed && !h.HasBed(roomId) {
			n++
		}
		if _, has := h.StationIn(roomId); kind == OfferStation && !has {
			n++
		}
	}
	return n
}

// RoomHasBed reports whether roomId is a lodging room with a bed in it.
func RoomHasBed(roomId int) bool {
	mu.RLock()
	defer mu.RUnlock()
	h, ok := roomHouse[roomId]
	return ok && h.HasBed(roomId)
}

// furnishingItem returns the building's deed item and price for kind.
func furnishingItem(b Building, kind string) (itemId int, price int) {
	if kind == OfferBed {
		return b.BedItemId, b.BedPrice
	}
	return b.StationItemId, b.StationPrice
}

// carriedFurnishingDeeds counts the deeds of kind of this building in a
// player's backpack: bought, not yet used.
func carriedFurnishingDeeds(user *users.UserRecord, b Building, kind string) int {
	itemId, _ := furnishingItem(b, kind)
	n := 0
	for _, itm := range user.Character.Items {
		if itm.ItemId == itemId {
			n++
		}
	}
	return n
}

// furnishingOffers are the list lines for a bed deed and a station deed.
func furnishingOffers(user *users.UserRecord, b Building, house House, owns bool) []Offer {
	bed := Offer{Key: OfferBed, Name: itemName(b.BedItemId, `Bed Deed`)}
	station := Offer{Key: OfferStation, Name: itemName(b.StationItemId, `Crafting Station Deed`)}
	for _, o := range []*Offer{&bed, &station} {
		switch {
		case !owns:
			o.Note = `Needs a home here first`
		case house.furnishingSpace(o.Key) <= carriedFurnishingDeeds(user, b, o.Key):
			if o.Key == OfferBed {
				o.Note = `Every room of yours has a bed`
			} else {
				o.Note = `Every room of yours has a station`
			}
		default:
			_, o.Price = furnishingItem(b, o.Key)
			o.Available = true
			if o.Key == OfferBed {
				o.Note = `A bed. Sleep in it to rest twice as fast`
			} else {
				o.Note = `A crafting station of your choice`
			}
		}
	}
	return []Offer{bed, station}
}

// buyFurnishing sells a bed deed or a station deed at the building's flat
// price. Like a container deed it is not bound: it can only be used inside
// its holder's own lodging anyway.
func buyFurnishing(user *users.UserRecord, say func(string), buildingId string, kind string) {
	b, ok := GetBuilding(buildingId)
	if !ok {
		return
	}
	itemId, price := furnishingItem(b, kind)
	what := `bed`
	if kind == OfferStation {
		what = `crafting station`
	}
	house, owns := HouseOf(user.UserId, b.BuildingId)
	if !owns {
		say(b.Line(`furnish.no_home`, `what`, what))
		return
	}
	if space := house.furnishingSpace(kind); space <= carriedFurnishingDeeds(user, b, kind) {
		if space == 0 {
			say(b.Line(`furnish.full`, `what`, what))
		} else {
			say(b.Line(`furnish.carrying`, `what`, what))
		}
		return
	}
	if user.Character.Gold+user.Character.Bank < price {
		say(b.Line(`furnish.no_gold`, `price`, price, `what`, what))
		return
	}
	deed := items.New(itemId)
	if deed.ItemId == 0 {
		mudlog.Error(`housing.buyFurnishing`, `building`, b.BuildingId, `error`, kind+` item does not exist`)
		return
	}
	if !user.Character.StoreItem(deed) {
		say(b.Line(`furnish.too_heavy`))
		return
	}
	fromGold, fromBank := chargeGold(user, price)
	events.AddToQueue(events.EquipmentChange{UserId: user.UserId, GoldChange: -fromGold, BankChange: -fromBank})
	events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: deed, Gained: true})
	mudlog.Info(`housing.buyFurnishing`, `user`, user.UserId, `building`, b.BuildingId, `kind`, kind, `price`, price)
	if kind == OfferBed {
		say(b.Line(`furnish.bed_sold`, `price`, price))
	} else {
		say(b.Line(`furnish.station_sold`, `price`, price))
	}
}

// ── Placing ────────────────────────────────────────────────────────────────

// useBedDeed puts a bed in the room the owner stands in.
func useBedDeed(user *users.UserRecord, room *rooms.Room, itm items.Item, h House, send func(string)) {
	if h.HasBed(room.RoomId) {
		send(`This room already has a bed. Try the deed in another of your rooms. The deed stays folded.`)
		return
	}
	if _, taken := room.Containers[`bed`]; taken {
		send(`There is a container called bed in this room already, and a real bed would be confused with it. Try another room.`)
		return
	}
	placed := placeFurnishing(user, room, itm, func(cur House) (House, string) {
		if cur.HasBed(room.RoomId) {
			return cur, `This room already has a bed.`
		}
		next := cur.clone()
		next.Beds = append(next.Beds, room.RoomId)
		return next, ``
	}, send)
	if !placed {
		return
	}
	mudlog.Info(`housing.useBedDeed`, `user`, user.UserId, `room`, room.RoomId)
	send(`Two porters carry in a bed, frame first and then the mattress, set it up against the wall, and leave. Anyone you let in can sleep in it. Type <ansi fg="command">sleep</ansi> here to rest twice as fast as on a floor.`)
	room.SendTextVisual(messaging.CategoryMobEmote, `Two porters carry in a bed, set it up against the wall, and leave without a word.`, user.UserId)
}

// useStationDeed installs a crafting station, named by args or chosen from a
// list, in the room the owner stands in.
func useStationDeed(user *users.UserRecord, room *rooms.Room, itm items.Item, h House, args string, rest string, send func(string)) {
	if have, ok := h.StationIn(room.RoomId); ok {
		send(fmt.Sprintf(`This room already has a %s. A room takes one crafting station. Try the deed in another of your rooms. The deed stays folded.`, StationName(have)))
		return
	}
	types := stationTypes()
	if len(types) == 0 {
		send(`There is nothing to install just now. The deed stays folded.`)
		return
	}
	names := make([]string, 0, len(types)+1)
	for _, s := range types {
		names = append(names, StationName(s))
	}

	choice := strings.ToLower(strings.Join(strings.Fields(args), ` `))
	if choice == `` {
		cmdPrompt, _ := user.StartPrompt(`use`, rest)
		q := cmdPrompt.Ask(`Which crafting station should be installed?`, append(append([]string{}, names...), `cancel`), `cancel`)
		if !q.Done {
			return
		}
		user.ClearPrompt()
		choice = strings.ToLower(strings.Join(strings.Fields(q.Response), ` `))
		if choice == `cancel` {
			send(`You fold the deed away for another day.`)
			return
		}
	}
	station := ``
	for _, s := range types {
		if choice == StationName(s) || choice == s {
			station = s
		}
	}
	if station == `` {
		send(`That is not a station the deed can install. Choose one of: ` + strings.Join(names, `, `) + `. Type <ansi fg="command">use station deed</ansi> to choose from a list. The deed stays folded.`)
		return
	}
	if _, taken := room.Containers[stationNoun(station)]; taken {
		send(fmt.Sprintf(`There is a container called %s in this room already, and the %s would be confused with it. Try another room.`, stationNoun(station), StationName(station)))
		return
	}

	placed := placeFurnishing(user, room, itm, func(cur House) (House, string) {
		if _, has := cur.StationIn(room.RoomId); has {
			return cur, `This room already has a crafting station.`
		}
		next := cur.clone()
		next.Stations = append(next.Stations, HouseStation{RoomId: room.RoomId, Station: station})
		return next, ``
	}, send)
	if !placed {
		return
	}
	mudlog.Info(`housing.useStationDeed`, `user`, user.UserId, `room`, room.RoomId, `station`, station)
	send(fmt.Sprintf(`Workmen carry in the parts of a %s and spend an hour fitting it together. Anyone you let in can work at it. Type <ansi fg="command">craft</ansi> here to see what you can make.`, StationName(station)))
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`Workmen carry in a %s, fit it together, and leave.`, StationName(station)), user.UserId)
}

// placeFurnishing writes the change add makes to the owner's house, under the
// registry lock and against the house as it is now, then spends the deed and
// re-lays the room. add returns a refusal instead when the room no longer
// has space. It reports whether the furnishing went in.
func placeFurnishing(user *users.UserRecord, room *rooms.Room, itm items.Item, add func(cur House) (House, string), send func(string)) bool {
	mu.Lock()
	cur, owned := roomHouse[room.RoomId]
	if !owned || cur.OwnerUserId != user.UserId {
		mu.Unlock()
		send(`This is not your lodging any more.`)
		return false
	}
	next, refusal := add(*cur)
	if refusal != `` {
		mu.Unlock()
		send(refusal + ` The deed stays folded.`)
		return false
	}
	if err := next.checkLinks(); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.placeFurnishing`, `user`, user.UserId, `error`, err.Error())
		send(`The deed will not take. Tell a member of staff.`)
		return false
	}
	if err := saveHouse(next); err != nil {
		mu.Unlock()
		mudlog.Error(`housing.placeFurnishing`, `user`, user.UserId, `error`, err.Error())
		send(`The deed will not take just now. Nothing was used up. Try again later.`)
		return false
	}
	nn := next.clone()
	indexLocked(&nn)
	mu.Unlock()

	if user.Character.RemoveItem(itm) {
		events.AddToQueue(events.ItemOwnership{UserId: user.UserId, Item: itm, Gained: false})
	}
	saveUser(user) // the house is on disk; the spent deed must be too
	applyLayout(room)
	return true
}

// ── Overlay ────────────────────────────────────────────────────────────────

// furnishTextKey is the room temp-data key holding the furnishing sentences
// the overlay last appended to the description, so a re-lay replaces them
// instead of appending them twice.
const furnishTextKey = `housing.furnishings`

// furnishRoom lays a house's bed and station over one of its rooms: the
// room's Station, a noun for each, and a sentence for each at the end of the
// description. Called by the overlay after the description is set.
func furnishRoom(r *rooms.Room, house House) {
	stripFurnishings(r)
	r.Station = ``
	text := ``
	if house.HasBed(r.RoomId) {
		setNoun(r, `bed`, `A sturdy bed with a straw-stuffed mattress, a bolster and a thick wool blanket. Whoever sleeps here rests twice as fast as on a floor.`)
		text += ` A <ansi fg="itemname">bed</ansi> stands against one wall, its blanket folded at the foot.`
	}
	if station, ok := house.StationIn(r.RoomId); ok {
		r.Station = station
		name, noun := StationName(station), stationNoun(station)
		setNoun(r, noun, fmt.Sprintf(`A %s, installed for whoever lodges here. Type craft beside it to see what you can make.`, name))
		text += fmt.Sprintf(` %s %s<ansi fg="itemname">%s</ansi> has been installed here.`, capitalise(article(name)), strings.TrimSuffix(name, noun), noun)
	}
	if text != `` {
		r.Description = strings.TrimRight(r.Description, " \n") + text
		r.SetTempData(furnishTextKey, text)
	}
}

// stripFurnishings takes back what furnishRoom laid on a room, for a room
// whose house changed or that is vacant again.
func stripFurnishings(r *rooms.Room) {
	if prev, ok := r.GetTempData(furnishTextKey).(string); ok && prev != `` {
		r.Description = strings.TrimSuffix(r.Description, prev)
		r.SetTempData(furnishTextKey, nil)
	}
	for _, noun := range furnishingNouns {
		if _, ours := r.GetTempData(furnishTextKey + `.noun.` + noun).(bool); ours {
			delete(r.Nouns, noun)
			r.SetTempData(furnishTextKey+`.noun.`+noun, nil)
		}
	}
	r.Station = ``
}

// setNoun adds a furnishing noun, remembering that it is ours to take back.
// A noun the room's template already has is left alone.
func setNoun(r *rooms.Room, noun string, text string) {
	if r.Nouns == nil {
		r.Nouns = map[string]string{}
	}
	if _, authored := r.Nouns[noun]; authored {
		if _, ours := r.GetTempData(furnishTextKey + `.noun.` + noun).(bool); !ours {
			return
		}
	}
	r.Nouns[noun] = text
	r.SetTempData(furnishTextKey+`.noun.`+noun, true)
}

func article(s string) string {
	if s != `` && strings.ContainsRune(`aeiou`, rune(s[0])) {
		return `an`
	}
	return `a`
}
