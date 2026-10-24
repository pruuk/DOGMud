package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// TestRoomIsLit pins the room-level predicate: IsLit() is true iff the room's
// CURRENT LightLevel() is at or above LightBlindBelow. It is defined purely
// against LightLevel, so it asserts nothing about the light model itself; the
// model's own arithmetic is pinned by lighting_model_test.go and its shipped
// numbers by testdata/lighting_daycycle.golden.
//
// Timing and the round count must both be pinned explicitly, because
// gametime.IsNight() and GetBiome() otherwise read ambient test-binary state:
// a bare binary loads Go defaults, and the day/night boundary now derives from
// WorldLatitude, so an unpinned round is whatever the ambient counter says. And
// gametime.ClearDateCacheForTest() is called before sampling: the
// round->date cache is keyed only on the round number, not on Timing
// config, so a round another test in this package already asked about would
// silently return that test's answer instead of being recomputed here.
func TestRoomIsLit(t *testing.T) {
	cleanupBiomes := SeedBiomesForTest(map[string]*BiomeInfo{
		"default": {BiomeId: "default", Name: "Default"},
		"cave":    {BiomeId: "cave", Name: "Cave", SkyLight: SkyLightPtr(0.0)},
	})
	defer cleanupBiomes()

	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	cfg.Timing.NightHours = 8
	configs.SetConfigForTest(t, cfg)

	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)

	// A round number this package's other lighting test does not also ask
	// about (that one uses 500000), landing on hour 12 (always day) by the
	// same dayCycleBase + RoundsPerDay/2 arithmetic documented there.
	const dayCycleBase = uint64(900000)
	dayRound := dayCycleBase + uint64(cfg.Timing.RoundsPerDay)/2
	if gametime.GetDate(dayRound).Night {
		t.Fatalf("dayRound %d did not compute as day under pinned Timing (RoundsPerDay=%d, NightHours=%d); the test fixture's arithmetic is wrong",
			dayRound, cfg.Timing.RoundsPerDay, cfg.Timing.NightHours)
	}
	util.SetRoundCountForTest(dayRound)

	tests := []struct {
		name string
		room *Room
		want bool
	}{
		// Cave biome forces LightDark (0) regardless of time of day, well
		// below the default LightBlindBelow (25): a dark room reads unlit.
		{"cave reads unlit", &Room{RoomId: 20, Biome: "cave"}, false},
		// Default biome at day resolves to LightFull (70), at or above the
		// default LightBlindBelow (25): a lit room reads lit.
		{"default biome at day reads lit", &Room{RoomId: 21}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.room.IsLit(); got != tc.want {
				t.Errorf("IsLit() = %v, want %v (LightLevel=%d, BlindBelow=%d)",
					got, tc.want, tc.room.LightLevel(), configs.GetLightingConfig().BlindBelow)
			}
		})
	}
}

// TestLightLevelMapsTheOldModel is RETIRED as of Task 8.
//
// It pinned the mapping that made graded lighting plan 1 behaviour
// preserving: every shipped room landing on one of exactly three values
// (LightDark/LightRoomOnly/LightFull), computed from legacyVisibility. Task 8
// deletes legacyVisibility and those three constants on purpose: LightLevel
// now composes a continuous value from the celestial term, the room's sky
// fraction and lamp, and any carried light, so there is no fixed three-point
// mapping left to assert. Coverage for "a cave stays dark" and "a fully open
// room reads lit" moves to TestRoomIsLit below (behaviour, not exact value)
// and to internal/rooms/lighting_model_test.go (the model's arithmetic
// directly). The day-cycle golden (internal/guards/lighting_daycycle_golden_test.go,
// package guards) is what now guards the model's actual numbers room-for-room.
