package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

var updateBalanceSpread = flag.Bool("update-lighting-balance-spread", false,
	"re-record testdata/lighting_balance_spread.golden")

// balanceSpreadReviewRooms are the 45 rooms of the plan 6 review page (rooms
// whose text promises light), plus 5105 and 5106 (the holding cells whose
// night reading went negative under the old arithmetic), 5803 (a room whose
// text says the street lamps are lit in the evening) and 4111 (the North Gate).
// Fixtures are deliberately unlit here: nothing is prepared, so 4111 reads
// without its arch lantern, and the fixture day-cycle golden
// (lighting_fixture_daycycle_golden_test.go) covers the lantern.
var balanceSpreadReviewRooms = []int{
	// Room lamp proposals.
	3109, 301, 310, 314, 317, 6032, 204, 6200, 488, 493, 496, 497, 498, 503,
	// Sky fraction proposals.
	3101, 3102, 6403, 6404, 6407, 6411, 5255, 4127, 490,
	// Left as they are.
	3106, 3108, 304, 6024, 6025, 6034, 6039, 6405, 6406, 6409, 6410, 6413,
	6417, 6419, 5257, 5258, 5259, 5261, 6468, 6469, 6204, 507,
	// Named in the spec.
	5105, 5106, 5803, 4111,
}

// balanceSpreadSample is one moment the golden composes every spread room at:
// a day of the year, an hour, and every moon new or every moon full.
type balanceSpreadSample struct {
	Label string
	Doy   int
	Hour  float64
	Moons float64
}

func balanceSpreadSamples() []balanceSpreadSample {
	out := []balanceSpreadSample{}
	for _, d := range []struct {
		name string
		doy  int
	}{{"midwinter", 356}, {"equinox", 81}, {"midsummer", 172}} {
		for _, h := range []struct {
			name string
			hour float64
		}{{"midnight", 0}, {"dawn", 6}, {"noon", 12}, {"dusk", 18}} {
			for _, m := range []struct {
				name string
				full float64
			}{{"new", 0}, {"full", 1}} {
				out = append(out, balanceSpreadSample{
					Label: d.name + "-" + h.name + "-moons-" + m.name,
					Doy:   d.doy, Hour: h.hour, Moons: m.full,
				})
			}
		}
	}
	return out
}

// TestLightingBalanceSpread is lighting plan 6's before-and-after record
// (spec, Testing). The day-cycle golden records integer levels at whatever
// moons each round happens to give; this one records the RAW composed light
// to a thousandth over a fixed spread of rooms, with the moons pinned new and
// full and the weather clear, so the arithmetic rebuild can be checked for
// "nothing at or above 37 moves 0.5 or more" rather than eyeballed through
// rounding.
//
// The spread is every biome's lowest-numbered room, every room with its own
// lamp or skylight override, and balanceSpreadReviewRooms. The celestial term
// comes from gametime.CelestialLightAt, the pure function gametime.CelestialLight
// itself calls, at pinned moons, because no round of the real clock gives all
// three moons new (or all full) at once.
//
// It is allowed to move, but only by a diff whose shape was predicted first:
// see the failure message.
func TestLightingBalanceSpread(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// The real shipped config, as the day-cycle golden reads it. A bare test
	// binary would otherwise walk the `default` fixture world.
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	conditions.LoadDataFiles()
	mutators.LoadDataFiles()

	ids := rooms.GetAllRoomIds()
	if len(ids) < 1000 {
		t.Fatalf("loaded only %d rooms: the walk is not seeing the world, so a green run proves nothing", len(ids))
	}
	sort.Ints(ids)

	want := map[int]string{}
	for _, id := range balanceSpreadReviewRooms {
		want[id] = "review"
	}
	firstOfBiome := map[string]bool{}
	for _, id := range ids {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load: the golden would silently lose coverage", id)
		}
		biome := ``
		if b := r.GetBiome(); b != nil {
			biome = b.BiomeId
		}
		if !firstOfBiome[biome] {
			firstOfBiome[biome] = true
			if _, ok := want[id]; !ok {
				want[id] = "biome"
			}
		}
		if r.Lamp != nil || r.SkyLight != nil {
			if _, ok := want[id]; !ok {
				want[id] = "override"
			}
		}
	}
	spread := make([]int, 0, len(want))
	for id := range want {
		spread = append(spread, id)
	}
	sort.Ints(spread)

	cfg := configs.GetLightingConfig()
	var b strings.Builder
	for _, s := range balanceSpreadSamples() {
		celestial := gametime.CelestialLightAt(cfg, s.Doy, s.Hour, s.Moons, s.Moons, s.Moons)
		// The lamps follow the sky being composed: the lamplighter sees
		// this sample's moons, not the live clock's.
		lampsLit := gametime.LampsLitAt(gametime.NightAt(cfg.WorldLatitude, s.Doy, s.Hour), celestial, gametime.StreetLampSkyFraction(), cfg)
		fmt.Fprintf(&b, "== %s\n", s.Label)
		for _, id := range spread {
			r := rooms.LoadRoom(id)
			if r == nil {
				t.Fatalf("room %d failed to load", id)
			}
			biome := ``
			if bi := r.GetBiome(); bi != nil {
				biome = bi.BiomeId
			}
			terms := r.LightTermsAtForTest(celestial, lampsLit, 1)
			// The spread records the sky, the lamp and the weather only:
			// nothing is prepared, so a carried light, a carried darkness
			// or a lit fixture here means the world load changed under
			// the golden and the readings no longer mean what they say.
			if terms.Carried || terms.CarriedLight != 0 || terms.Darkened || terms.Dark != 0 || terms.Fixture != 0 {
				t.Fatalf("room %d at %s carries light %v (%v), darkness %v (%v) or a fixture %v: the spread must hold none",
					id, s.Label, terms.Carried, terms.CarriedLight, terms.Darkened, terms.Dark, terms.Fixture)
			}
			fmt.Fprintf(&b, "room %d %s biome=%s raw=%.3f level=%d\n", id, want[id], biome, terms.Raw, terms.Level)
		}
	}
	got := b.String()

	goldenPath := filepath.Join("testdata", "lighting_balance_spread.golden")
	if *updateBalanceSpread {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("recorded %s", goldenPath)
		return
	}
	wantGolden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (record it with -update-lighting-balance-spread)", err)
	}
	if got != string(wantGolden) {
		t.Errorf("lighting balance spread golden moved.\n\n" +
			"This golden was recorded before lighting plan 6 rebuilt the light " +
			"arithmetic. It is allowed to move, but every move must be one you " +
			"predicted: the rebuild only raises the dim end (a reading at or above " +
			"37 moves by under 0.5), the street lamp drops street rooms to daylight " +
			"alone while the clear sky shows faces, and the room data pass relights the rooms " +
			"the review page approved.\n\n" +
			"Work out the expected diff first, compare, and only then:\n" +
			"  go test . -run TestLightingBalanceSpread -update-lighting-balance-spread -v")
	}
}
