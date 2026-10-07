package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// TestRoomsThatPromiseLightCarryTheApprovedLight pins lighting plan 6's room
// data (spec section 4, owner ruling O7): the 14 room lamps and 9 sky
// fractions the review page approved, and dense_forest's movement cost (O8).
// A typo in a room YAML (a lamp on the wrong room, a fraction off by a digit)
// otherwise passes every other test.
func TestRoomsThatPromiseLightCarryTheApprovedLight(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	lamps := map[int]int{
		3109: 35, 310: 35, 503: 45, 488: 30, 317: 50, 314: 40, 6032: 45,
		204: 60, 497: 60, 498: 64, 493: 30, 6200: 30, 301: 28, 496: 28,
	}
	skies := map[int]float64{
		3101: 0.50, 5255: 0.50, 6403: 0.35, 6411: 0.25, 490: 0.25,
		4127: 0.25, 3102: 0.20, 6407: 0.15, 6404: 0.10,
	}
	for id, want := range lamps {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		if r.Lamp == nil || *r.Lamp != want {
			t.Errorf("room %d lamp = %v, want %d", id, r.Lamp, want)
		}
		if r.SkyLight != nil {
			t.Errorf("room %d gained a skylight %v; the review approved a lamp only", id, *r.SkyLight)
		}
	}
	for id, want := range skies {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		if r.SkyLight == nil || *r.SkyLight != want {
			t.Errorf("room %d skylight = %v, want %v", id, r.SkyLight, want)
		}
		if r.Lamp != nil {
			t.Errorf("room %d gained a lamp %d; the review approved a sky fraction only", id, *r.Lamp)
		}
	}

	b, ok := rooms.GetBiome("dense_forest")
	if !ok {
		t.Fatal("dense_forest biome is not shipped")
	}
	if got := b.GetMovementCost(); got != 1.4 {
		t.Errorf("dense_forest movement cost = %v, want 1.4", got)
	}
}
