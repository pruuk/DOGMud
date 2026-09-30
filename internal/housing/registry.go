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
	validateBuildingsAgainstWorld(loaded)

	mu.Lock()
	resetLocked()
	for id, b := range loaded {
		bb := b
		buildings[id] = &bb
		for _, roomId := range bb.UnitRooms {
			unitBuilding[roomId] = id
		}
		doorBuilding[bb.DoorRoom] = append(doorBuilding[bb.DoorRoom], id)
	}
	mu.Unlock()

	nHouses, nHeld := loadHouses()
	mudlog.Info(`housing.LoadDataFiles()`, `buildings`, len(loaded), `houses`, nHouses, `heldRooms`, nHeld, `Time Taken`, time.Since(start))
}

func resetLocked() {
	buildings = map[string]*Building{}
	unitBuilding = map[int]string{}
	doorBuilding = map[int][]string{}
	houses = map[int]*House{}
	roomHouse = map[int]*House{}
	ownerHouse = map[ownerKey]*House{}
	held = map[int]string{}
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
// mobs or factions that do not exist, or at a door and units that do not
// connect. Authored content fails loudly at boot, like rooms and ferries.
func validateBuildingsAgainstWorld(loaded map[string]Building) {
	unitOwner := map[int]string{}
	ids := make([]string, 0, len(loaded))
	for id := range loaded {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		b := loaded[id]
		door := rooms.LoadRoom(b.DoorRoom)
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
		for _, roomId := range b.UnitRooms {
			if other, dup := unitOwner[roomId]; dup {
				panic(fmt.Sprintf(`housing building %s: unit room %d is also a unit of %s`, id, roomId, other))
			}
			unitOwner[roomId] = id
			unit := rooms.LoadRoom(roomId)
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
		}
	}
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

// VacantUnits returns the building's unit rooms that are neither owned nor
// held, in authored order.
func VacantUnits(buildingId string) []int {
	mu.RLock()
	defer mu.RUnlock()
	return vacantUnitsLocked(buildingId)
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
