package housing

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// RegisterRoomHooks wires housing into the room layer. Call once at boot.
// The hooks read the registry lazily, so registering before LoadDataFiles is
// fine: until houses load, every door refuses and every unit is guarded.
func RegisterRoomHooks() {
	rooms.SetExitRouter(RouteDoor)
	rooms.SetEntryGuard(GuardEntry)
	rooms.SetPrivateRoomCheck(IsUnitRoom)
}

// isStaff lets admins walk into any unit (teleport, to look into a report)
// without owning it. It is resolved per call because roles can change.
var isStaff = func(userId int) bool {
	u := users.GetByUserId(userId)
	return u != nil && u.Role == users.RoleAdmin
}

// landlordName is overridable in tests, which have no mob specs loaded.
var landlordName = func(b Building) string {
	if spec := mobs.GetMobSpec(mobs.MobId(b.LandlordMobId)); spec != nil && spec.Character.Name != `` {
		return spec.Character.Name
	}
	return `the landlord`
}

// RouteDoor is the rooms.ExitRouter. A building's shared door leads each
// owner to their own entry room and refuses everyone else. Every other exit
// is not housing's business.
func RouteDoor(userId int, fromRoomId int, exitName string) (rooms.ExitRoute, bool) {
	mu.RLock()
	var b *Building
	for _, id := range doorBuilding[fromRoomId] {
		if cand := buildings[id]; cand != nil && cand.DoorExit == exitName {
			b = cand
			break
		}
	}
	if b == nil {
		mu.RUnlock()
		return rooms.ExitRoute{}, false
	}
	h, owns := ownerHouse[ownerKey{b.BuildingId, userId}]
	entry := 0
	if owns {
		entry = h.EntryRoom()
	}
	bCopy := *b
	mu.RUnlock()

	if owns && entry > 0 {
		return rooms.ExitRoute{RoomId: entry}, true
	}

	return rooms.ExitRoute{
		Refusal: util.SplitStringNL(fmt.Sprintf(`The <ansi fg="exit">%s</ansi> is locked, and it will not open for you. It opens only for those who lodge here. <ansi fg="mobname">%s</ansi> lets the rooms. Try <ansi fg="command">ask %s about a room</ansi>.`,
			bCopy.DoorExit, landlordName(bCopy), firstWord(landlordName(bCopy))), 80),
	}, true
}

// GuardEntry is the rooms.EntryGuard. A unit room admits its owner and staff,
// and nobody else, however they try to arrive (walking, recall, summons, a
// quest move, or logging in). A refused login lands at the building's door.
func GuardEntry(userId int, toRoomId int) (bool, int, string) {
	mu.RLock()
	buildingId, isUnit := unitBuilding[toRoomId]
	if !isUnit {
		mu.RUnlock()
		return true, 0, ``
	}
	h, owned := roomHouse[toRoomId]
	ownerOk := owned && h.OwnerUserId == userId
	door := 0
	if b := buildings[buildingId]; b != nil {
		door = b.DoorRoom
	}
	mu.RUnlock()

	if ownerOk || isStaff(userId) {
		return true, 0, ``
	}
	return false, door, `That room is not yours to enter.`
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' {
			return s[:i]
		}
	}
	return s
}
