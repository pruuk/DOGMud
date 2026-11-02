package rooms

import "github.com/GoMudEngine/GoMud/internal/configs"

// SeedRoomsForTest replaces the global roomManager with a fresh instance
// populated from the supplied maps and returns a cleanup function.
// Intended for cross-package integration tests (hooks, commands).
func SeedRoomsForTest(roomMap map[int]*Room, zoneMap map[string]*ZoneConfig) func() {
	orig := roomManager

	mgr := &RoomManager{
		rooms:             roomMap,
		zones:             zoneMap,
		roomsWithUsers:    make(map[int]int),
		roomsWithMobs:     make(map[int]int),
		roomIdToFileCache: make(map[int]string),
	}

	roomManager = mgr

	return func() {
		roomManager = orig
	}
}

// UnloadRoomForTest drops a room from the in-memory room cache, as idle-room
// unloading (RoomMaintenance) does, so the next LoadRoom reads it from disk
// again. It skips the rest of removeRoomFromMemory on purpose: no instance
// save is written, no mobs are torn down, and the file-path cache keeps its
// entry, which matches production (an unloaded room's path stays cached).
// Intended for cross-package tests that need a "cold" room.
func UnloadRoomForTest(roomId int) {
	delete(roomManager.rooms, roomId)
}

// SeedTemplateSpawnsForTest makes roomId's spawninfo place mobIds in the spawn
// index, as loading that template room does. The returned cleanup removes it.
// Intended for cross-package tests of SpawnRooms.
func SeedTemplateSpawnsForTest(roomId int, mobIds ...int) func() {
	spawns := make([]SpawnInfo, 0, len(mobIds))
	for _, id := range mobIds {
		spawns = append(spawns, SpawnInfo{MobId: id})
	}
	setTemplateSpawns(roomId, spawns)
	return func() { deleteTemplateSpawns(roomId) }
}

// SetOriginalRoomIdForTest records roomId as an instance copy of templateId,
// as CreateEphemeralRoomIds does, so OriginalRoomId(roomId) returns
// templateId. The returned cleanup restores the previous entry.
// Intended for cross-package tests of template-room matching.
func SetOriginalRoomIdForTest(roomId, templateId int) func() {
	prev, had := originalRoomIdLookups[roomId]
	originalRoomIdLookups[roomId] = templateId
	return func() {
		if had {
			originalRoomIdLookups[roomId] = prev
		} else {
			delete(originalRoomIdLookups, roomId)
		}
	}
}

// MarkRoomOccupancy updates the roomManager tracking maps so that
// GetRoomsWithPlayers / GetRoomsWithMobs return correct results in tests.
// Call this after AddPlayer/AddMob on the room objects.
func MarkRoomOccupancy(roomId int, playerCt int, mobCt int) {
	if playerCt > 0 {
		roomManager.roomsWithUsers[roomId] = playerCt
	}
	if mobCt > 0 {
		roomManager.roomsWithMobs[roomId] = mobCt
	}
}

// SeedBiomesForTest replaces the global biomes map with the supplied test data
// and returns a cleanup function that restores the original.
// Intended for cross-package integration tests (hooks, commands).
func SeedBiomesForTest(biomeMap map[string]*BiomeInfo) func() {
	orig := biomes
	biomes = biomeMap
	registerStreetLampSky()
	return func() {
		biomes = orig
		registerStreetLampSky()
	}
}

// SkyLightPtr and LampPtr let a test build a BiomeInfo{} literal with an
// explicit SkyLight/Lamp value inline, without declaring a local variable at
// the call site just to take its address. Both fields are pointers because
// zero is a meaningful, distinct-from-unset value for each (see BiomeInfo's
// doc comments), so a plain literal cannot express "authored as 0".
func SkyLightPtr(f float64) *float64 { return &f }
func LampPtr(n int) *int             { return &n }

// MultiplierPtr returns a pointer to v for a zone or mob multiplier in a
// test fixture (#317).
func MultiplierPtr(v float64) *float64 { return &v }

// LightTermsAtForTest composes this room's light with the celestial term,
// whether the street lamps are lit (gametime.LampsLitAt on the caller's own
// night and celestial) and the weather sky filter supplied, rather than read
// from the clock and the active mutators. A cross-package golden uses it to
// pin moon states that no single round of the real clock can produce (every
// moon new, or every moon full). Everything else (lamp, fixtures, carried
// light) is read as LightTerms reads it.
func (r *Room) LightTermsAtForTest(celestial float64, lampsLit bool, skyFilter float64) LightTerms {
	return r.composeLight(configs.GetLightingConfig(), celestial, lampsLit, skyFilter)
}
