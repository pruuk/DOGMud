package rooms

import "sync"

// The template room to zone index (#375 phase 3b slice 4). Rooms load
// lazily, so ZoneConfig.RoomIds holds only rooms in memory; this index holds
// every template room's zone from boot, for areas and for the quest
// engine's zone: filter. It is built in loadAllRoomZones and kept current by
// addRoomToMemory (a builder's new or moved room). Like roomIdToFileCache it
// is written from goroutines that hold only the read side of the MUD lock,
// so it has its own mutex.
var (
	templateZoneMu sync.RWMutex
	templateZone   = map[int]string{}
)

func setTemplateZone(roomId int, zone string) {
	if IsEphemeralRoomId(roomId) {
		return
	}
	templateZoneMu.Lock()
	templateZone[roomId] = zone
	templateZoneMu.Unlock()
}

// TemplateRoomZone returns the zone of a template room, "" when the id is
// not a loaded template room (an instance room id included: map it through
// OriginalRoomId first).
func TemplateRoomZone(roomId int) string {
	templateZoneMu.RLock()
	defer templateZoneMu.RUnlock()
	return templateZone[roomId]
}

// TemplateRoomZones returns a copy of the whole index: template room id to
// zone name.
func TemplateRoomZones() map[int]string {
	templateZoneMu.RLock()
	defer templateZoneMu.RUnlock()
	out := make(map[int]string, len(templateZone))
	for id, z := range templateZone {
		out[id] = z
	}
	return out
}
