package rooms

import (
	"slices"
	"sync"
)

// The mob spawn index (#375 phase 4): which template rooms' spawninfo place
// each mob template, so a quest destination can fall back to where an NPC
// lives. Built in loadAllRoomZones, replaced for a room by SaveRoomTemplate,
// dropped by DeleteRoomTemplate. A move or rename does not change a room's
// spawns. Slices are replaced, never changed in place, so a slice handed to
// a reader stays valid.
var (
	spawnMu    sync.RWMutex
	spawnRooms = map[int][]int{} // mob template id -> template room ids, sorted
	roomSpawns = map[int][]int{} // template room id -> mob template ids it places
)

func setTemplateSpawns(roomId int, spawns []SpawnInfo) {
	if IsEphemeralRoomId(roomId) {
		return
	}
	spawnMu.Lock()
	defer spawnMu.Unlock()
	clearRoomSpawnsLocked(roomId)
	var here []int
	for _, si := range spawns {
		if si.MobId <= 0 || slices.Contains(here, si.MobId) {
			continue
		}
		here = append(here, si.MobId)
		ids := append(slices.Clone(spawnRooms[si.MobId]), roomId)
		slices.Sort(ids)
		spawnRooms[si.MobId] = ids
	}
	if len(here) > 0 {
		roomSpawns[roomId] = here
	}
}

func deleteTemplateSpawns(roomId int) {
	spawnMu.Lock()
	defer spawnMu.Unlock()
	clearRoomSpawnsLocked(roomId)
}

func clearRoomSpawnsLocked(roomId int) {
	for _, mobId := range roomSpawns[roomId] {
		ids := slices.DeleteFunc(slices.Clone(spawnRooms[mobId]), func(id int) bool { return id == roomId })
		if len(ids) == 0 {
			delete(spawnRooms, mobId)
		} else {
			spawnRooms[mobId] = ids
		}
	}
	delete(roomSpawns, roomId)
}

// SpawnRooms lists the template rooms whose spawninfo place mobId, sorted;
// nil when none does. The slice is shared: do not change it.
func SpawnRooms(mobId int) []int {
	spawnMu.RLock()
	defer spawnMu.RUnlock()
	return spawnRooms[mobId]
}
