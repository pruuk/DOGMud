package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// longTerraceRoomId is the Long Terrace, where First Shot (quest 50) has the
// student fire at the practice butt and then again from cover.
const longTerraceRoomId = 5351

// TestLongTerraceReadsFacesAtEveryHour pins the 2026-10-17 newbie-area audit
// fix (B1): the Long Terrace is an open cliff shelf, so at night it fell below
// the faces band and a normal-sighted student could not see the butt to aim
// at it. A fixed lamp now keeps it at full sight (faces, not dazzled) for a
// normal observer at every sampled hour of the year.
func TestLongTerraceReadsFacesAtEveryHour(t *testing.T) {
	loadItemBehaviourWorld(t)
	cfg := configs.GetLightingConfig()

	terrace := rooms.LoadRoom(longTerraceRoomId)
	if terrace == nil {
		t.Fatalf("room %d must load", longTerraceRoomId)
	}
	samples := sampleRounds()
	if len(samples) == 0 {
		t.Fatal("no sample rounds, so this test would prove nothing")
	}
	for _, s := range samples {
		util.SetRoundCount(s.Round)
		lvl := terrace.LightLevel()
		if b := messaging.BandThroughWindow(lvl, 0, 0, cfg.BlindBelow, cfg.DimBelow, cfg.DazzleAbove); b != messaging.BandFaces {
			t.Errorf("%s: the Long Terrace reads %d (%s) for a normal observer, want faces", s.Label, lvl, b)
		}
	}
}
