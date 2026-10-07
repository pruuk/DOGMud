package main

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// TestNoShippedRoomReadsBelowZeroWithoutDarkness is lighting plan 6's world
// guard (owner rulings O1 and O2): 0 is the darkest a room can be without
// active magical darkness, so with no darkness source present no shipped room
// may read below 0 at any hour, season, moon state or weather.
//
// It loads the real world the way the night-trade guard
// (shop_night_trade_guard_test.go) and the lighting goldens do, and composes
// every room through rooms.LightTermsAtForTest with the celestial term built
// on the same operator gametime.CelestialLight uses. Nothing is prepared, so
// no mob, carried source or fixture (light or darkness) is in any room: the
// only terms are the sky, the room's own lamp and the weather.
//
// Samples: three days (both solstices and an equinox), every hour, the moons
// all new, all half and all full, and every distinct skylight fraction a
// shipped weather mutator declares, plus clear sky, the worst two stacked,
// and a near-total 0.001 as a probe past anything shipped.
//
// It also pins the O3 floor: a room reads either exactly 0 (no light at all)
// or at least LightRealMinimum.
func TestNoShippedRoomReadsBelowZeroWithoutDarkness(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
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

	filterSet := map[float64]bool{1: true, 0.001: true}
	worst := 1.0
	for _, spec := range mutators.GetAllMutatorSpecs() {
		if spec.SkyLight != nil {
			filterSet[*spec.SkyLight] = true
			worst = min(worst, *spec.SkyLight)
		}
	}
	if len(filterSet) < 4 {
		t.Fatalf("found only %d sky filters: the mutator walk is not seeing the shipped weather", len(filterSet))
	}
	second := 1.0
	for f := range filterSet {
		if f > worst && f < 1 && f != 0.001 {
			second = min(second, f)
		}
	}
	filterSet[worst*second] = true

	cfg := configs.GetLightingConfig()
	floor := float64(cfg.RealMinimum)
	type sample struct {
		celestial float64
	}
	var samples []sample
	for _, doy := range []int{356, 81, 172} {
		for hour := 0; hour < 24; hour++ {
			for _, m := range []float64{0, 0.5, 1} {
				samples = append(samples, sample{
					celestial: lightscale.Combine(cfg.DoublingStep,
						gametime.SunLight(cfg, doy, float64(hour)),
						gametime.MoonLight(cfg, m, m, m)),
				})
			}
		}
	}

	checked, failures := 0, 0
	for _, id := range ids {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		for si, s := range samples {
			for f := range filterSet {
				terms := r.LightTermsAtForTest(s.celestial, f)
				checked++
				bad := terms.Raw < 0 || terms.Level < 0 || (terms.Raw > 0 && terms.Raw < floor)
				if bad {
					failures++
					if failures <= 20 {
						t.Errorf("room %d sample %d filter %v: Raw %.3f Level %d (want 0, or at least %v)",
							id, si, f, terms.Raw, terms.Level, floor)
					}
				}
			}
		}
	}
	if failures > 20 {
		t.Errorf("... and %d more", failures-20)
	}
	t.Logf("checked %d room readings", checked)
}
