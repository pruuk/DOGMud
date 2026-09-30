// Package housing owns persistent player housing: the authored buildings a
// player can take a room in, the living-state record of who owns which room,
// the landlord purchase, and the routing that sends each lodger through a
// building's one shared door to their own room.
//
// Two kinds of data, deliberately kept apart:
//
//   - Buildings are AUTHORED content, one YAML per building under
//     <DataFiles>/housing_buildings/. They name the door, the landlord, the
//     city faction whose standing gates a purchase, the price of each tier,
//     and the pool of blank unit rooms (ordinary authored rooms) the building
//     lets out. A bad building file panics at boot, like any authored content.
//
//   - Houses are LIVING STATE, one YAML per house under
//     <DataFiles>/housing/<buildingid>/<entryroomid>.yaml. They record who owns
//     which unit rooms. They follow the living-state contract
//     (internal/util/livingstate.go): durable atomic writes, absent is not
//     corrupt, a corrupt file is quarantined and never deleted, and every
//     change is persisted before it is published to the in-memory registry.
//
// The unit rooms themselves are real authored rooms, so the room engine
// (lighting, biome, saving the floor, the mapper) treats them like any other.
// A house is a list of room ids, so a later tier can own more than one.
package housing

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/opinions"
)

// Tier is one size of house a building offers.
type Tier struct {
	TierId string `yaml:"tier_id"`
	Name   string `yaml:"name"`  // what the landlord calls it: "a simple room"
	Price  int    `yaml:"price"` // gold, paid once
	Rooms  int    `yaml:"rooms"` // unit rooms a house of this tier occupies
}

// Building is one authored lodging house.
type Building struct {
	BuildingId string `yaml:"building_id"`
	Name       string `yaml:"name"` // "the Back Court lodgings"

	// DoorRoom and DoorExit name the one shared door. The exit is authored in
	// DoorRoom and locked so mobs and strangers cannot use it; the router
	// sends each owner through it to their own entry room.
	DoorRoom int    `yaml:"door_room"`
	DoorExit string `yaml:"door_exit"`

	// LandlordMobId is the NPC who lets the rooms (behavior-tree action
	// buy_housing). Only used for naming him in messages and validation.
	LandlordMobId int `yaml:"landlord_mob_id"`

	// Faction and MinRepTier gate a purchase on the buyer's standing with the
	// city (internal/factions). MinRepTier is one of hostile, cold, neutral,
	// warm, friendly.
	Faction    string `yaml:"faction"`
	MinRepTier string `yaml:"min_rep_tier"`

	Tiers []Tier `yaml:"tiers"`

	// UnitRooms is the pool of blank authored rooms this building lets out,
	// both as homes and as extensions to them. Each must have an exit named
	// DoorExit leading back to DoorRoom.
	UnitRooms []int `yaml:"unit_rooms,flow"`

	// Extensions: the landlord sells an extension deed (ExtensionItemId). Its
	// price is ExtensionPriceMultiplier times everything the owner has paid
	// for rooms so far, home included, so each extension costs more than the
	// last. MaxRooms caps a house's size so one lodger cannot take the whole
	// pool. ExtensionTitle and ExtensionDescription are what a new room looks
	// like until its owner redecorates it.
	ExtensionItemId          int    `yaml:"extension_item_id"`
	ExtensionPriceMultiplier int    `yaml:"extension_price_multiplier"`
	MaxRooms                 int    `yaml:"max_rooms"`
	ExtensionTitle           string `yaml:"extension_title"`
	ExtensionDescription     string `yaml:"extension_description"`

	// Redecorating: a one-use voucher (RedecorateItemId) that sets the
	// description of the room it is used in, at a flat RedecoratePrice.
	RedecorateItemId int `yaml:"redecorate_item_id"`
	RedecoratePrice  int `yaml:"redecorate_price"`

	// Guests: a guest key (GuestKeyItemId), made out to the buyer's house,
	// is given to a friend, who uses it at the door once to be let in from
	// then on. A house admits at most MaxGuests.
	GuestKeyItemId int `yaml:"guest_key_item_id"`
	GuestKeyPrice  int `yaml:"guest_key_price"`
	MaxGuests      int `yaml:"max_guests"`

	// Containers: a container deed (ContainerItemId) places a container the
	// owner names with one word, usable by anyone let in; a strongbox deed
	// (StrongboxItemId) places one only the owner can open. A house holds
	// at most MaxContainers of both kinds together.
	ContainerItemId int `yaml:"container_item_id"`
	ContainerPrice  int `yaml:"container_price"`
	StrongboxItemId int `yaml:"strongbox_item_id"`
	StrongboxPrice  int `yaml:"strongbox_price"`
	MaxContainers   int `yaml:"max_containers"`
}

// housingItemIds lists every item the building sells, for validation.
func (b Building) housingItemIds() []int {
	return []int{b.ExtensionItemId, b.RedecorateItemId, b.GuestKeyItemId, b.ContainerItemId, b.StrongboxItemId}
}

func (b Building) Id() string       { return b.BuildingId }
func (b Building) Filepath() string { return b.BuildingId + `.yaml` }

// Validate covers intrinsic checks. World checks (rooms, mob and faction
// exist) run in LoadBuildings once those are loaded.
func (b Building) Validate() error {
	if b.BuildingId == `` {
		return fmt.Errorf(`housing building missing building_id`)
	}
	if b.Name == `` {
		return fmt.Errorf(`housing building %s: missing name`, b.BuildingId)
	}
	if b.DoorRoom <= 0 || b.DoorExit == `` {
		return fmt.Errorf(`housing building %s: door_room and door_exit are required`, b.BuildingId)
	}
	if b.LandlordMobId <= 0 {
		return fmt.Errorf(`housing building %s: landlord_mob_id is required`, b.BuildingId)
	}
	if b.Faction == `` {
		return fmt.Errorf(`housing building %s: faction is required`, b.BuildingId)
	}
	if _, ok := ParseRepTier(b.MinRepTier); !ok {
		return fmt.Errorf(`housing building %s: min_rep_tier %q is not one of hostile, cold, neutral, warm, friendly`, b.BuildingId, b.MinRepTier)
	}
	if len(b.Tiers) == 0 {
		return fmt.Errorf(`housing building %s: at least one tier is required`, b.BuildingId)
	}
	seenTier := map[string]bool{}
	for _, t := range b.Tiers {
		if t.TierId == `` || t.Name == `` {
			return fmt.Errorf(`housing building %s: every tier needs tier_id and name`, b.BuildingId)
		}
		if seenTier[t.TierId] {
			return fmt.Errorf(`housing building %s: duplicate tier %s`, b.BuildingId, t.TierId)
		}
		seenTier[t.TierId] = true
		if t.Price <= 0 {
			return fmt.Errorf(`housing building %s: tier %s price must be positive`, b.BuildingId, t.TierId)
		}
		if t.Rooms < 1 {
			return fmt.Errorf(`housing building %s: tier %s must occupy at least one room`, b.BuildingId, t.TierId)
		}
	}
	if len(b.UnitRooms) == 0 {
		return fmt.Errorf(`housing building %s: unit_rooms is empty`, b.BuildingId)
	}
	seenRoom := map[int]bool{}
	for _, id := range b.UnitRooms {
		if id <= 0 || seenRoom[id] {
			return fmt.Errorf(`housing building %s: unit room %d is invalid or listed twice`, b.BuildingId, id)
		}
		if id == b.DoorRoom {
			return fmt.Errorf(`housing building %s: the door room cannot also be a unit`, b.BuildingId)
		}
		seenRoom[id] = true
	}
	seenItem := map[int]bool{}
	for _, id := range b.housingItemIds() {
		if id <= 0 || seenItem[id] {
			return fmt.Errorf(`housing building %s: extension_item_id, redecorate_item_id, guest_key_item_id, container_item_id and strongbox_item_id are required and must all differ`, b.BuildingId)
		}
		seenItem[id] = true
	}
	if b.ContainerPrice <= 0 || b.StrongboxPrice <= 0 || b.MaxContainers < 1 {
		return fmt.Errorf(`housing building %s: container_price and strongbox_price must be positive and max_containers at least 1`, b.BuildingId)
	}
	if b.GuestKeyPrice <= 0 || b.MaxGuests < 1 {
		return fmt.Errorf(`housing building %s: guest_key_price must be positive and max_guests at least 1`, b.BuildingId)
	}
	if b.ExtensionPriceMultiplier < 1 {
		return fmt.Errorf(`housing building %s: extension_price_multiplier must be at least 1`, b.BuildingId)
	}
	for _, t := range b.Tiers {
		if b.MaxRooms < t.Rooms {
			return fmt.Errorf(`housing building %s: max_rooms %d is smaller than tier %s`, b.BuildingId, b.MaxRooms, t.TierId)
		}
	}
	if b.RedecoratePrice <= 0 {
		return fmt.Errorf(`housing building %s: redecorate_price must be positive`, b.BuildingId)
	}
	if strings.TrimSpace(b.ExtensionTitle) == `` || strings.TrimSpace(b.ExtensionDescription) == `` {
		return fmt.Errorf(`housing building %s: extension_title and extension_description are required`, b.BuildingId)
	}
	return nil
}

// Tier returns the named tier.
func (b Building) Tier(tierId string) (Tier, bool) {
	for _, t := range b.Tiers {
		if t.TierId == tierId {
			return t, true
		}
	}
	return Tier{}, false
}

// MinTier returns the parsed minimum standing. Validate guarantees it parses.
func (b Building) MinTier() opinions.Tier {
	t, _ := ParseRepTier(b.MinRepTier)
	return t
}

// ParseRepTier maps a tier name to an opinions.Tier.
func ParseRepTier(name string) (opinions.Tier, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case `hostile`:
		return opinions.TierHostile, true
	case `cold`:
		return opinions.TierCold, true
	case `neutral`:
		return opinions.TierNeutral, true
	case `warm`:
		return opinions.TierWarm, true
	case `friendly`:
		return opinions.TierFriendly, true
	}
	return opinions.TierNeutral, false
}

// House is one player's home in one building. It is living state.
type House struct {
	BuildingId string `yaml:"building_id"`
	// OwnerUserId is the ACCOUNT (UserRecord) id, so a house is shared by an
	// account's alts, exactly like the bank.
	OwnerUserId int `yaml:"owner_user_id"`
	// OwnerName is the character that bought it. For staff reading the file;
	// never used to decide access.
	OwnerName string `yaml:"owner_name"`
	TierId    string `yaml:"tier_id"`
	// RoomIds are the unit rooms this house occupies. RoomIds[0] is the entry
	// room the door leads to. Later tiers append rooms here.
	RoomIds     []int     `yaml:"room_ids,flow"`
	PricePaid   int       `yaml:"price_paid"`
	PurchasedAt time.Time `yaml:"purchased_at"`

	// RoomsPaid is everything paid for rooms: the home plus every extension
	// deed bought, placed or not. The next deed costs a multiple of it.
	// Houses written before extensions existed have it 0; Spent covers that.
	RoomsPaid int `yaml:"rooms_paid,omitempty"`

	// Links are the doorways between this house's own rooms, one entry per
	// doorway; the way back is implied (north from A to B is south from B to
	// A). Placed by extension deeds.
	Links []RoomLink `yaml:"links,omitempty"`

	// Descriptions are owner-written room descriptions by room id, set by a
	// redecorating voucher. A room without one shows its default text.
	Descriptions map[int]string `yaml:"descriptions,omitempty"`

	// Guests are the accounts the owner has let in with a guest key. Like
	// the owner, a guest is an ACCOUNT, so a guest's alts share the access.
	Guests []Guest `yaml:"guests,omitempty"`

	// Containers are the owner-named containers placed by deeds, WITH their
	// contents. This record, not the room's instance save, is where their
	// items live: the overlay puts them into the room on load and Capture
	// writes changes back.
	Containers []HouseContainer `yaml:"containers,omitempty"`
}

// HouseContainer is one container placed in a house.
type HouseContainer struct {
	RoomId int    `yaml:"room_id"`
	Name   string `yaml:"name"` // one word, lower case: what players type
	// OwnerOnly marks a strongbox: nobody but the house owner may look in it,
	// take from it or put into it.
	OwnerOnly bool         `yaml:"owner_only,omitempty"`
	Items     []items.Item `yaml:"items,omitempty"`
	Gold      int          `yaml:"gold,omitempty"`
}

// WalkItems visits every item the house holds, for the bauble sweep and
// TestItemWalkersVisitEveryItemField.
func (h *House) WalkItems(visit func(*items.Item)) {
	for ci := range h.Containers {
		items.WalkSlice(h.Containers[ci].Items, visit)
	}
}

// containersIn returns the house's containers placed in roomId.
func (h House) containersIn(roomId int) []HouseContainer {
	out := []HouseContainer{}
	for _, c := range h.Containers {
		if c.RoomId == roomId {
			out = append(out, c)
		}
	}
	return out
}

// Guest is one account let into a house.
type Guest struct {
	UserId int `yaml:"user_id"`
	// Name is the character that used the key. For lists and revoking by
	// name; access is decided by UserId only.
	Name    string    `yaml:"name"`
	AddedAt time.Time `yaml:"added_at"`
}

// IsGuest reports whether userId (an account) is one of the house's guests.
func (h House) IsGuest(userId int) bool {
	for _, g := range h.Guests {
		if g.UserId == userId {
			return true
		}
	}
	return false
}

// MayEnter reports whether userId owns the house or is one of its guests.
func (h House) MayEnter(userId int) bool {
	return h.OwnerUserId == userId || h.IsGuest(userId)
}

// RoomLink is one doorway between two rooms of the same house.
type RoomLink struct {
	From      int    `yaml:"from"`
	Direction string `yaml:"direction"`
	To        int    `yaml:"to"`
}

// EntryRoom is the room the building door leads this owner to.
func (h House) EntryRoom() int {
	if len(h.RoomIds) == 0 {
		return 0
	}
	return h.RoomIds[0]
}

// Spent is what the owner has paid for rooms so far.
func (h House) Spent() int {
	if h.RoomsPaid > 0 {
		return h.RoomsPaid
	}
	return h.PricePaid
}

// HasRoom reports whether roomId is one of this house's rooms.
func (h House) HasRoom(roomId int) bool {
	for _, id := range h.RoomIds {
		if id == roomId {
			return true
		}
	}
	return false
}

// clone returns a deep copy, so a change can be built and saved without
// touching the published record (persist before publishing).
func (h House) clone() House {
	c := h
	c.RoomIds = append([]int(nil), h.RoomIds...)
	c.Links = append([]RoomLink(nil), h.Links...)
	c.Guests = append([]Guest(nil), h.Guests...)
	c.Containers = make([]HouseContainer, len(h.Containers))
	for i, hc := range h.Containers {
		hc.Items = append([]items.Item(nil), hc.Items...)
		c.Containers[i] = hc
	}
	if len(h.Containers) == 0 {
		c.Containers = nil
	}
	if h.Descriptions != nil {
		c.Descriptions = make(map[int]string, len(h.Descriptions))
		for k, v := range h.Descriptions {
			c.Descriptions[k] = v
		}
	}
	return c
}

// directions a house can grow in, with their map deltas (engine frame: north
// is y-1, up is z+1, matching the mapper) and opposites.
var directionDeltas = map[string][3]int{
	`north`: {0, -1, 0},
	`south`: {0, 1, 0},
	`east`:  {1, 0, 0},
	`west`:  {-1, 0, 0},
	`up`:    {0, 0, 1},
	`down`:  {0, 0, -1},
}

var oppositeDirection = map[string]string{
	`north`: `south`, `south`: `north`,
	`east`: `west`, `west`: `east`,
	`up`: `down`, `down`: `up`,
}

var directionAliases = map[string]string{
	`n`: `north`, `s`: `south`, `e`: `east`, `w`: `west`, `u`: `up`, `d`: `down`,
}

// ParseDirection normalises a direction a house can grow in.
func ParseDirection(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if full, ok := directionAliases[s]; ok {
		s = full
	}
	_, ok := directionDeltas[s]
	return s, ok
}

// exitsOf returns this house's doorways out of roomId, direction -> room.
func (h House) exitsOf(roomId int) map[string]int {
	out := map[string]int{}
	for _, l := range h.Links {
		if l.From == roomId {
			out[l.Direction] = l.To
		}
		if l.To == roomId {
			out[oppositeDirection[l.Direction]] = l.From
		}
	}
	return out
}

// offsets places every room of the house relative to its entry room by
// walking the doorways. Rooms not reached by a doorway sit at the entry.
func (h House) offsets() map[int][3]int {
	pos := map[int][3]int{h.EntryRoom(): {0, 0, 0}}
	queue := []int{h.EntryRoom()}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for dir, next := range h.exitsOf(cur) {
			if _, seen := pos[next]; seen {
				continue
			}
			d := directionDeltas[dir]
			p := pos[cur]
			pos[next] = [3]int{p[0] + d[0], p[1] + d[1], p[2] + d[2]}
			queue = append(queue, next)
		}
	}
	return pos
}

// checkLinks reports the first thing wrong with the house's doorways: a room
// outside the house, a bad direction, a slot used twice, or a doorway to
// itself. A house file that fails it is held, never guessed at.
func (h House) checkLinks() error {
	used := map[int]map[string]bool{}
	take := func(room int, dir string) error {
		if used[room] == nil {
			used[room] = map[string]bool{}
		}
		if used[room][dir] {
			return fmt.Errorf(`room %d has two doorways %s`, room, dir)
		}
		used[room][dir] = true
		return nil
	}
	for _, l := range h.Links {
		if _, ok := directionDeltas[l.Direction]; !ok {
			return fmt.Errorf(`doorway direction %q is not one of north, south, east, west, up, down`, l.Direction)
		}
		if l.From == l.To || !h.HasRoom(l.From) || !h.HasRoom(l.To) {
			return fmt.Errorf(`doorway %d %s %d is not between two rooms of this house`, l.From, l.Direction, l.To)
		}
		if err := take(l.From, l.Direction); err != nil {
			return err
		}
		if err := take(l.To, oppositeDirection[l.Direction]); err != nil {
			return err
		}
	}
	for roomId := range h.Descriptions {
		if !h.HasRoom(roomId) {
			return fmt.Errorf(`description for room %d, which is not in this house`, roomId)
		}
	}
	seenContainer := map[string]bool{}
	for _, c := range h.Containers {
		if !h.HasRoom(c.RoomId) {
			return fmt.Errorf(`container %q is in room %d, which is not in this house`, c.Name, c.RoomId)
		}
		if err := validContainerName(c.Name); err != nil {
			return fmt.Errorf(`container %q: %v`, c.Name, err)
		}
		key := fmt.Sprintf(`%d/%s`, c.RoomId, c.Name)
		if seenContainer[key] {
			return fmt.Errorf(`two containers called %q in room %d`, c.Name, c.RoomId)
		}
		seenContainer[key] = true
	}
	seenGuest := map[int]bool{}
	for _, g := range h.Guests {
		if g.UserId <= 0 || g.UserId == h.OwnerUserId || seenGuest[g.UserId] {
			return fmt.Errorf(`guest %d is invalid, the owner, or listed twice`, g.UserId)
		}
		seenGuest[g.UserId] = true
	}
	return nil
}
