package engine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/modules/weather/content"
	"github.com/GoMudEngine/GoMud/modules/weather/sim"
)

// TestEmitAmbient_ReadsConfiguredStrongFeltThreshold proves EmitAmbient
// forwards the LIVE Balance.WeatherStrongFeltThreshold into content.Tables.Pick
// rather than a hardcoded literal -- the thing test (a) in the content
// package cannot show, because content has no configs import to prove wrong.
//
// Setup: one front centered ON the room's own zone with MaxFrontRadius 0, so
// sim.Covering resolves felt to exactly the front's Intensity (0.6) with no
// real graph edges needed (zonesWithin always seeds the center at hop 0).
// The indoor pool's Mild band is left empty (silence, the store's standing
// rule) and Strong carries one line, so the threshold's effect is visible
// purely through EmitAmbient's returned sent count: Strong reachable means
// sent 1, Mild (empty) means sent 0. This avoids needing a real connected
// user session to inspect the rendered text.
func TestEmitAmbient_ReadsConfiguredStrongFeltThreshold(t *testing.T) {
	cleanupBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"house": {BiomeId: "house", Name: "House", Indoor: true},
	})
	defer cleanupBiomes()

	room := &rooms.Room{RoomId: 1, Zone: "zoneA", Biome: "house"}
	room.AddPlayer(9001)
	cleanupRooms := rooms.SeedRoomsForTest(
		map[int]*rooms.Room{1: room},
		map[string]*rooms.ZoneConfig{},
	)
	defer cleanupRooms()
	rooms.MarkRoomOccupancy(1, 1, 0)

	tables := content.Tables{
		"rain": {
			Weather: "rain",
			TableSection: content.TableSection{
				Indoor: map[string]content.IndoorPool{
					"house": {Strong: []string{"STRONG LINE"}}, // Mild left empty
				},
			},
		},
	}

	g := &sim.Graph{}
	fronts := []sim.Front{{Id: 1, Zone: "zoneA", Intensity: 0.6}}
	simCfg := sim.Config{MaxFrontRadius: 0, CoverageFalloff: 1, MinProjected: 0}
	weather := map[sim.ZoneId]sim.WeatherType{"zoneA": "rain"}
	roll := func(n int) int { return 0 } // never skips the cadence roll; index 0 of any pool

	run := func(t *testing.T, threshold float64) int {
		t.Helper()
		c := configs.GetConfig()
		c.Balance.WeatherStrongFeltThreshold = configs.ConfigFloat(threshold)
		configs.SetConfigForTest(t, c)

		return EmitAmbient(g, fronts, simCfg, weather, nil, tables, nil, 100, 100, 3, roll)
	}

	t.Run("shipped 0.5: felt 0.6 reaches Strong", func(t *testing.T) {
		if sent := run(t, 0.5); sent != 1 {
			t.Fatalf("felt 0.6 >= threshold 0.5 must select Strong (sent=1), got sent=%d", sent)
		}
	})

	t.Run("raised to 0.9: the SAME felt 0.6 now falls in Mild (empty, silent)", func(t *testing.T) {
		if sent := run(t, 0.9); sent != 0 {
			t.Fatalf("felt 0.6 < threshold 0.9 must select the empty Mild pool (sent=0), got sent=%d", sent)
		}
	})
}
