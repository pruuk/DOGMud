package guards

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// riftChamberRoomId is the Rift Chamber, where the riftkeeper Sable (mob
// 315) tends the archway; openLandRoomId is an open-sky land room.
const (
	riftChamberRoomId = 5000
	openLandRoomId    = 139
)

// TestRiftChamberLitLikeWinterDaylight holds the owner ruling of 2026-10-17:
// the portals make a lot of light, so the Rift Chamber is lit "fairly bright,
// full winter-ish daylight equivalent". Its lamp (it is a dungeon, with no
// sky) must equal an open-sky land room's light at a clear midwinter noon on
// the real lighting model, and a normal-sighted player must read faces there
// at every sampled hour, with the Rift Stone's pulse (20 to 36) at either
// end, so Sable is seen and named. At lamp 38 she read as a shape (38 to 45).
func TestRiftChamberLitLikeWinterDaylight(t *testing.T) {
	loadItemBehaviourWorld(t)
	cfg := configs.GetLightingConfig()

	land := rooms.LoadRoom(openLandRoomId)
	rift := rooms.LoadRoom(riftChamberRoomId)
	if land == nil || rift == nil {
		t.Fatalf("rooms %d and %d must load", openLandRoomId, riftChamberRoomId)
	}

	var winterNoon int
	for _, s := range sampleRounds() {
		util.SetRoundCount(s.Round)
		lvl := rift.LightLevel()
		if s.Label == "midwinter-noon" {
			winterNoon = land.LightLevel()
			if lvl != winterNoon {
				t.Errorf("Rift Chamber reads %d, an open land room at a clear midwinter noon reads %d: they must match", lvl, winterNoon)
			}
		}
		for _, stone := range []float64{20, 36} {
			withStone := int(math.Round(lightscale.Combine(float64(cfg.DoublingStep), float64(lvl), stone)))
			if b := messaging.BandThroughWindow(withStone, 0, 0, cfg.BlindBelow, cfg.DimBelow, cfg.DazzleAbove); b != messaging.BandFaces {
				t.Errorf("%s with the Rift Stone at %.0f: light %d reads %s for a normal observer, want faces", s.Label, stone, withStone, b)
			}
		}
	}
	if winterNoon == 0 {
		t.Fatal("no midwinter-noon sample, so the daylight comparison never ran")
	}
}
