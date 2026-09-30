package housing

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

type ownerKey struct {
	buildingId string
	userId     int
}

var (
	mu sync.RWMutex

	buildings    = map[string]*Building{}
	unitBuilding = map[int]string{} // unit room id -> building id
	doorBuilding = map[int][]string{}

	houses     = map[int]*House{} // entry room id -> house
	roomHouse  = map[int]*House{} // every owned room id -> its house
	ownerHouse = map[ownerKey]*House{}

	// held unit rooms are never sold and never entered (except by staff):
	// a house file for them could not be read or did not make sense, and
	// selling the room again would hand a stranger someone else's things.
	held = map[int]string{} // room id -> reason

	// frozen buildings sell no rooms (no homes, no extensions): one of their
	// house files could not be read, so which rooms it owned is unknown, and
	// any vacant-looking unit may hold someone's things.
	frozen = map[string]string{} // building id -> reason

	// heldOwners own a house whose readable file was rejected. They are not
	// sold a second home while it waits for repair.
	heldOwners = map[ownerKey]bool{}

	// unitCoords are each unit room's authored coordinates (x, y, z, plane),
	// read from its template at load. A house's rooms are placed relative to
	// its entry room's, so the overlay never has to load a room.
	unitCoords = map[int][4]int{}
)

// buildingsDirOverride is a test hook; empty uses config.
var buildingsDirOverride string

func buildingsDir() string {
	if buildingsDirOverride != `` {
		return buildingsDirOverride
	}
	return configs.GetFilePathsConfig().DataFiles.String() + `/housing_buildings`
}

// SetBuildingsDirForTest points the authored-building loader at dir and
// returns a restore func.
func SetBuildingsDirForTest(dir string) func() {
	prev := buildingsDirOverride
	buildingsDirOverride = dir
	return func() { buildingsDirOverride = prev }
}

// LoadDataFiles loads the authored buildings, then every house. It is safe to
// call again on a data reload: the registry is rebuilt from disk, which is the
// source of truth because every change is persisted before it is published.
func LoadDataFiles() {
	start := time.Now()
	loaded := loadBuildings()
	coords := validateBuildingsAgainstWorld(loaded)
	nHouses, nHeld := rebuild(loaded, coords)
	mudlog.Info(`housing.LoadDataFiles()`, `buildings`, len(loaded), `houses`, nHouses, `heldRooms`, nHeld, `Time Taken`, time.Since(start))
}

// rebuild replaces the registry with these buildings and the house files, and
// lays the houses over their rooms.
func rebuild(loaded map[string]Building, coords map[int][4]int) (nHouses int, nHeld int) {
	// On a reload, what players did since the last capture is only in the
	// live rooms: write it to the house files first, since the reload
	// rebuilds everything from them.
	keepLive := captureAllLoaded()

	mu.Lock()
	resetLocked()
	unitCoords = coords
	for id, b := range loaded {
		bb := b
		buildings[id] = &bb
		for _, roomId := range bb.UnitRooms {
			unitBuilding[roomId] = id
		}
		doorBuilding[bb.DoorRoom] = append(doorBuilding[bb.DoorRoom], id)
	}
	mu.Unlock()

	nHouses, nHeld = loadHouses()
	applyAllOverlays(keepLive)
	return nHouses, nHeld
}

func resetLocked() {
	buildings = map[string]*Building{}
	unitBuilding = map[int]string{}
	doorBuilding = map[int][]string{}
	houses = map[int]*House{}
	roomHouse = map[int]*House{}
	ownerHouse = map[ownerKey]*House{}
	held = map[int]string{}
	frozen = map[string]string{}
	heldOwners = map[ownerKey]bool{}
	unitCoords = map[int][4]int{}
}

// Frozen reports why a building sells no rooms, if it does not.
func Frozen(buildingId string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	reason, ok := frozen[buildingId]
	return reason, ok
}

// ResetForTest clears all registry state.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	resetLocked()
}

// AddBuildingForTest registers a building without loading files or checking
// the world.
func AddBuildingForTest(b Building) {
	mu.Lock()
	defer mu.Unlock()
	bb := b
	buildings[b.BuildingId] = &bb
	for _, roomId := range b.UnitRooms {
		unitBuilding[roomId] = b.BuildingId
	}
	doorBuilding[b.DoorRoom] = append(doorBuilding[b.DoorRoom], b.BuildingId)
}

func loadBuildings() map[string]Building {
	dir := buildingsDir()
	// No directory means no buildings are authored yet. Anything wrong with a
	// file that does exist is broken authored content.
	if _, err := os.Stat(filepath.FromSlash(dir)); os.IsNotExist(err) {
		return map[string]Building{}
	}
	loaded, err := fileloader.LoadAllFlatFiles[string, Building](dir)
	if err != nil {
		panic(fmt.Sprintf(`housing.LoadDataFiles: %v`, err))
	}
	return loaded
}

// validateBuildingsAgainstWorld panics on authored data that points at rooms,
// mobs, factions or items that do not exist, or at a door and units that do
// not connect. Authored content fails loudly at boot, like rooms and ferries.
//
// It reads room TEMPLATES, not live rooms: on a data reload the live units
// already carry their houses' overlays (an extension has no door of its own),
// and the authored shape is what is being checked. It returns every unit's
// authored coordinates.
func validateBuildingsAgainstWorld(loaded map[string]Building) map[int][4]int {
	coords := map[int][4]int{}
	unitOwner := map[int]string{}
	ids := make([]string, 0, len(loaded))
	for id := range loaded {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		b := loaded[id]
		door := rooms.LoadRoomTemplate(b.DoorRoom)
		if door == nil {
			panic(fmt.Sprintf(`housing building %s: door_room %d does not exist`, id, b.DoorRoom))
		}
		doorExit, ok := door.Exits[b.DoorExit]
		if !ok {
			panic(fmt.Sprintf(`housing building %s: door_room %d has no exit named %q`, id, b.DoorRoom, b.DoorExit))
		}
		// The authored lock is what keeps mobs (which never consult the
		// router) from walking through the shared door.
		if !doorExit.HasLock() {
			panic(fmt.Sprintf(`housing building %s: the %q exit in room %d must be locked so mobs cannot use it`, id, b.DoorExit, b.DoorRoom))
		}
		if mobs.GetMobSpec(mobs.MobId(b.LandlordMobId)) == nil {
			panic(fmt.Sprintf(`housing building %s: landlord_mob_id %d does not exist`, id, b.LandlordMobId))
		}
		if factions.GetDefinition(b.Faction) == nil {
			panic(fmt.Sprintf(`housing building %s: faction %q does not exist`, id, b.Faction))
		}
		for _, itemId := range b.housingItemIds() {
			if items.GetItemSpec(itemId) == nil {
				panic(fmt.Sprintf(`housing building %s: item %d does not exist`, id, itemId))
			}
		}
		for _, roomId := range b.UnitRooms {
			if other, dup := unitOwner[roomId]; dup {
				panic(fmt.Sprintf(`housing building %s: unit room %d is also a unit of %s`, id, roomId, other))
			}
			unitOwner[roomId] = id
			unit := rooms.LoadRoomTemplate(roomId)
			if unit == nil {
				panic(fmt.Sprintf(`housing building %s: unit room %d does not exist`, id, roomId))
			}
			back, ok := unit.Exits[b.DoorExit]
			if !ok || back.RoomId != b.DoorRoom {
				panic(fmt.Sprintf(`housing building %s: unit room %d needs a %q exit leading to room %d`, id, roomId, b.DoorExit, b.DoorRoom))
			}
			if len(unit.SpawnInfo) > 0 {
				panic(fmt.Sprintf(`housing building %s: unit room %d must not spawn mobs`, id, roomId))
			}
			for dir := range directionDeltas {
				if _, taken := unit.Exits[dir]; taken {
					panic(fmt.Sprintf(`housing building %s: unit room %d must not have an authored %s exit; extensions place those`, id, roomId, dir))
				}
			}
			coords[roomId] = [4]int{unit.X, unit.Y, unit.Z, unit.Plane}
		}
	}
	return coords
}

// Building returns a copy of the named building.
func GetBuilding(buildingId string) (Building, bool) {
	mu.RLock()
	defer mu.RUnlock()
	b, ok := buildings[buildingId]
	if !ok {
		return Building{}, false
	}
	return *b, true
}

// BuildingForUnit reports which building a unit room belongs to.
func BuildingForUnit(roomId int) (Building, bool) {
	mu.RLock()
	defer mu.RUnlock()
	id, ok := unitBuilding[roomId]
	if !ok {
		return Building{}, false
	}
	return *buildings[id], true
}

// LandlordBuildings returns the buildings a mob template lets, in id order.
func LandlordBuildings(mobId int) []Building {
	mu.RLock()
	defer mu.RUnlock()
	out := []Building{}
	for _, b := range buildings {
		if b.LandlordMobId == mobId {
			out = append(out, *b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BuildingId < out[j].BuildingId })
	return out
}

// IsHousingItem reports whether itemId is anything a building sells: an
// extension deed, redecorating voucher, guest key, container deed or
// strongbox deed.
func IsHousingItem(itemId int) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, b := range buildings {
		for _, id := range b.housingItemIds() {
			if id == itemId {
				return true
			}
		}
	}
	return false
}

// IsUnitRoom reports whether roomId is one of any building's unit rooms,
// whether or not it is currently owned.
func IsUnitRoom(roomId int) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := unitBuilding[roomId]
	return ok
}

// HouseOf returns the house userId (an account id) owns in buildingId.
func HouseOf(userId int, buildingId string) (House, bool) {
	mu.RLock()
	defer mu.RUnlock()
	h, ok := ownerHouse[ownerKey{buildingId, userId}]
	if !ok {
		return House{}, false
	}
	return *h, true
}

// HouseForRoom returns the house that owns roomId.
func HouseForRoom(roomId int) (House, bool) {
	mu.RLock()
	defer mu.RUnlock()
	h, ok := roomHouse[roomId]
	if !ok {
		return House{}, false
	}
	return *h, true
}

// AllHouses returns a copy of every house, ordered by entry room.
func AllHouses() []House {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]House, 0, len(houses))
	for _, h := range houses {
		out = append(out, *h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EntryRoom() < out[j].EntryRoom() })
	return out
}

// HostsOf returns the houses in buildingId where userId is a guest, ordered
// by owner name so menus are stable.
func HostsOf(userId int, buildingId string) []House {
	mu.RLock()
	defer mu.RUnlock()
	return hostsOfLocked(userId, buildingId)
}

func hostsOfLocked(userId int, buildingId string) []House {
	out := []House{}
	for _, h := range houses {
		if h.BuildingId == buildingId && h.IsGuest(userId) {
			out = append(out, h.clone())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OwnerName != out[j].OwnerName {
			return out[i].OwnerName < out[j].OwnerName
		}
		return out[i].EntryRoom() < out[j].EntryRoom()
	})
	return out
}

// VacantUnits returns the building's unit rooms that are neither owned nor
// held, in authored order.
func VacantUnits(buildingId string) []int {
	mu.RLock()
	defer mu.RUnlock()
	return vacantUnitsLocked(buildingId)
}

// vacantUnitsAll lists the vacant units of every building.
func vacantUnitsAll() []int {
	mu.RLock()
	defer mu.RUnlock()
	out := []int{}
	for id := range buildings {
		out = append(out, vacantUnitsLocked(id)...)
	}
	return out
}

func vacantUnitsLocked(buildingId string) []int {
	b, ok := buildings[buildingId]
	if !ok {
		return nil
	}
	out := []int{}
	for _, roomId := range b.UnitRooms {
		if _, owned := roomHouse[roomId]; owned {
			continue
		}
		if _, isHeld := held[roomId]; isHeld {
			continue
		}
		out = append(out, roomId)
	}
	return out
}

// indexLocked publishes a house to the registry. Callers hold mu.
func indexLocked(h *House) {
	houses[h.EntryRoom()] = h
	for _, roomId := range h.RoomIds {
		roomHouse[roomId] = h
	}
	ownerHouse[ownerKey{h.BuildingId, h.OwnerUserId}] = h
}

// holdLocked marks a room as unavailable. Callers hold mu.
func holdLocked(roomId int, reason string) {
	if roomId <= 0 {
		return
	}
	held[roomId] = reason
}

// HeldRooms returns the held rooms and why, for staff tools and tests.
func HeldRooms() map[int]string {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[int]string, len(held))
	for k, v := range held {
		out[k] = v
	}
	return out
}
