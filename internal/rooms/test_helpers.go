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
	return func() {
		biomes = orig
	}
}

// SkyLightPtr and LampPtr let a test build a BiomeInfo{} literal with an
// explicit SkyLight/Lamp value inline, without declaring a local variable at
// the call site just to take its address. Both fields are pointers because
// zero is a meaningful, distinct-from-unset value for each (see BiomeInfo's
// doc comments), so a plain literal cannot express "authored as 0".
func SkyLightPtr(f float64) *float64 { return &f }
func LampPtr(n int) *int             { return &n }

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
