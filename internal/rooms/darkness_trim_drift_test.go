package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// The 5d playtest: an Umbral Lantern bearer standing still on a sunlit street
// just before 1 PM fell from shapes to dark as the sky dimmed, because the
// trim had parked the room exactly on the blind edge. The trim now lands one
// point inside the bearer's range, so a small drift in sky light after the
// trim (here 0.75 of a point, enough to round a room on the edge below it)
// leaves the bearer reading shapes.
func TestADarknessTrimSurvivesASmallSkyDrift(t *testing.T) {
	us := seedDarknessTrim(t)
	setClock(172, 12.93) // midsummer, 12:56 PM

	open := 1.0
	room := &Room{RoomId: darkFirstRoom + 70, Biome: "cave", SkyLight: &open}
	bearer := us.add(darkFirstUser+70, darkUmbralId)
	room.AddPlayer(bearer.UserId)
	room.TrimLightFor(bearer.Character)

	if rec := bearer.Character.Conditions.DarknessSources()[0]; rec.LightTrim != conditions.LightTrimmed {
		t.Fatalf("the noon sky did not make the lantern trim (state %q); the scenario needs a trimmed darkness", rec.LightTrim)
	}
	cfg := configs.GetLightingConfig()
	if got := messaging.SightThroughWindow(room.LightLevel(), 0, 0, cfg.BlindBelow, cfg.DimBelow); got != messaging.SightShapes {
		t.Fatalf("right after the trim the bearer reads %v, want shapes", got)
	}

	before := room.LightTerms().Raw
	// The sky dims by 0.75 of a point: on the log scale a fraction f takes
	// step*log2(1/f) points off the sky term, and with no lamp the sky is the
	// room's only light, so the net light falls by the same amount.
	dimmer := math.Exp2(-0.75 / cfg.DoublingStep)
	room.SkyLight = &dimmer
	drop := before - room.LightTerms().Raw
	if drop <= 0.5 || drop >= 1 {
		t.Fatalf("the sky drift moved the net light by %v, want between 0.5 and 1 point", drop)
	}

	if got := messaging.SightThroughWindow(room.LightLevel(), 0, 0, cfg.BlindBelow, cfg.DimBelow); got != messaging.SightShapes {
		t.Errorf("after a %.2f point sky drift the bearer reads %v at light %d, want shapes", drop, got, room.LightLevel())
	}
}
