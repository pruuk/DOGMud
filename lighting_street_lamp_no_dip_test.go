package main

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// TestStreetLampsNeverLeaveAStreetBelowFaces is the guard on the lamplighter's
// rule (lighting plan 6, owner ruling O4 as amended, and the review fix that
// followed it). A street lamp goes out only once the clear sky, as the
// dimmest street-lamp biome sees it through its sky fraction, reads at least
// LightDimBelow, so in clear weather the lamps going out never leave a street
// below faces, not even for the round they go out. A street whose lamp reads
// faces (the thoroughfare's 52) therefore reads faces at every round; a
// backstreet, whose lamp 35 is meant to leave it at shapes by night, is held
// to faces only while its lamps are out.
//
// The first rule tested the clear sky itself against the edge. A street sees
// that sky through its 0.95 fraction, so for a round at every dawn and dusk
// the lamps were out and the street read 49, shapes: about 240 dips a year.
//
// It walks every round of both solstices and an equinox on the live clock
// (LightLevel reads gametime.CelestialLight and gametime.LampsLit) for one
// plain room of each street-lamp biome, a room with no lamp or sky override
// of its own, and nothing prepared in it.
func TestStreetLampsNeverLeaveAStreetBelowFaces(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	// Timing pinned as the day-cycle golden pins it. TRAP: the Go default
	// NightHours is 0, a world with no night in it.
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 900
	cfg.Timing.NightHours = 8
	cfg.Timing.RoundSeconds = 4
	cfg.Timing.Validate()
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()

	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	originalRound := util.GetRoundCount()
	t.Cleanup(func() {
		util.SetRoundCount(originalRound)
		gametime.ClearDateCacheForTest()
		gametime.ClearCelestialMemoForTest()
	})

	streetLamp := map[string]bool{}
	for _, b := range rooms.GetAllBiomes() {
		if b.StreetLamp && b.HasLamp() {
			streetLamp[b.BiomeId] = true
		}
	}
	if len(streetLamp) < 2 {
		t.Fatalf("found %d street-lamp biomes, want the thoroughfare and the backstreet at least", len(streetLamp))
	}

	ids := rooms.GetAllRoomIds()
	sort.Ints(ids)
	probe := map[string]*rooms.Room{}
	for _, id := range ids {
		r := rooms.LoadRoom(id)
		if r == nil || r.Lamp != nil || r.SkyLight != nil {
			continue
		}
		b := r.GetBiome()
		if b == nil || !streetLamp[b.BiomeId] || probe[b.BiomeId] != nil {
			continue
		}
		probe[b.BiomeId] = r
	}
	for biome := range streetLamp {
		if probe[biome] == nil {
			t.Fatalf("no plain room of street-lamp biome %s to probe", biome)
		}
	}
	biomes := make([]string, 0, len(probe))
	for b := range probe {
		biomes = append(biomes, b)
	}
	sort.Strings(biomes)

	dim := configs.GetLightingConfig().DimBelow
	rpd := uint64(cfg.Timing.RoundsPerDay)
	lampsOut, failures := 0, 0
	for _, doy := range []uint64{356, 81, 172} {
		for r := (doy - 1) * rpd; r < doy*rpd; r++ {
			util.SetRoundCount(r)
			lit := gametime.LampsLit()
			if !lit {
				lampsOut++
			}
			for _, b := range biomes {
				// A lamp below faces (the backstreet's 35) is meant to leave
				// its street at shapes by night; only the lamps going out may
				// never do it.
				lamp, _ := probe[b].GetBiome().LampValue()
				if lit && lamp < dim {
					continue
				}
				if lvl := probe[b].LightLevel(); lvl < dim {
					failures++
					if failures <= 20 {
						t.Errorf("day %d round %d: %s room %d reads %d, below faces (%d); lamps lit %v",
							doy, r, b, probe[b].RoomId, lvl, dim, gametime.LampsLit())
					}
				}
			}
		}
	}
	if failures > 20 {
		t.Errorf("... and %d more", failures-20)
	}
	// The lamps must actually go out on some rounds, or the sky half of the
	// rule went untested and a green run proves nothing.
	if lampsOut == 0 {
		t.Fatalf("the street lamps never went out across three days; the guard tested only lamplight")
	}
}

// TestShadedStreetLampRoomsCarryTheirOwnLamp guards the case the no-dip test
// skips. The registered street-lamp sky fraction (the dimmest street-lamp
// biome's, which gametime.LampsLit reads) assumes every street-lamp room sees
// at least that much sky. A room in such a biome whose own skylight is below
// it (an inn under eaves, say) can read dark while the lamps are out, so it
// must carry its own lamp: override and lamp, not the biome's alone.
func TestShadedStreetLampRoomsCarryTheirOwnLamp(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	fraction := gametime.StreetLampSkyFraction()
	if fraction >= 1 {
		t.Fatalf("registered street-lamp sky fraction is %v; no street-lamp biome loaded", fraction)
	}

	ids := rooms.GetAllRoomIds()
	sort.Ints(ids)
	shaded := 0
	for _, id := range ids {
		r := rooms.LoadRoom(id)
		if r == nil || r.SkyLight == nil {
			continue
		}
		b := r.GetBiome()
		if b == nil || !b.StreetLamp || b.Lamp == nil {
			continue
		}
		if *r.SkyLight >= fraction {
			continue
		}
		shaded++
		if r.Lamp == nil {
			t.Errorf("room %d (%s) is in street-lamp biome %s with skylight %v, below the street-lamp fraction %v, but has no lamp of its own",
				r.RoomId, r.Title, b.BiomeId, *r.SkyLight, fraction)
		}
	}
	// At least one shaded room ships today (the Travelers' Inn, 6252); if
	// none is found the walk tested nothing.
	if shaded == 0 {
		t.Fatalf("found no shaded street-lamp room; the guard tested nothing")
	}
}
