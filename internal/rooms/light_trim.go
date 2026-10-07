package rooms

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// TrimLightFor trims every adjustable, unhooded light AND darkness record c
// holds to c's own eyes. A light takes the least cut from full strength that
// keeps this room from dazzling them; a darkness the least cut that keeps the
// room at or above the bottom of their usable range (lighting plan 5d). The
// records trim one after another in held order, each seeing the room as the
// previous trims left it. A pre-pass sets the bearer's eligible sources off
// first, which already keeps each source out of its own combine; the exclude
// argument to composeLightExcluding is belt and braces, kept because it
// states the intent.
//
// A light solves Trim on the light combine against its target plus the
// room's darkness: a light in a darkened room may run brighter before it
// dazzles, which is what the room does (ruling D3). A darkness solves
// TrimDarkness against the light and the other darkness (ruling D2).
//
// It is the only trim trigger. MoveToRoom and AddMob call it once the mover
// is in the room, so arrivals trim in entry order, and nobody already here
// re-trims when someone else walks in. Nothing calls it on a round tick: a
// room that changes around a standing bearer leaves their sources as they
// were.
func (r *Room) TrimLightFor(c *characters.Character) {
	if r == nil || c == nil {
		return
	}
	type trimmable struct {
		rec  *conditions.Condition
		spec *conditions.ConditionSpec
		dark bool
	}
	var todo []trimmable
	for _, rec := range c.Conditions.LightAndDarknessSources() {
		spec := conditions.GetConditionSpec(rec.ConditionId)
		if spec == nil || rec.Hooded || !slices.Contains(spec.Flags, conditions.Adjustable) {
			continue
		}
		todo = append(todo, trimmable{rec, spec, spec.IsDarknessSource()})
	}
	// A bearer with nothing adjustable (a plain torch, or no source at all)
	// pays nothing on a move.
	if len(todo) == 0 {
		return
	}

	cfg := configs.GetLightingConfig()
	celestial := gametime.CelestialLight()
	lampsLit := gametime.LampsLitAt(gametime.IsNight(), celestial, gametime.StreetLampSkyFraction(), cfg)
	skyFilter := r.mutatorSkyFilter()
	strength := c.NightVisionStrength()
	lightTarget := messaging.LightTrimTarget(strength, cfg.DazzleAbove)
	darkFloor := messaging.DarknessTrimTarget(strength, c.InfraReach(), cfg.BlindBelow)

	// Every source about to trim leaves the room first. Otherwise a source
	// would see the later ones still at full strength, trim to nothing, and
	// hand the room to whichever source comes last: held order inverted. With
	// them out, each source sees the room with the earlier trims and nothing
	// of its own that has not trimmed yet.
	for _, t := range todo {
		t.rec.SetLightOutput(lightscale.Absent())
	}
	for _, t := range todo {
		terms := r.composeLightExcluding(cfg, celestial, lampsLit, skyFilter, t.rec)
		full := t.rec.LightMax(t.spec)
		var out float64
		if t.dark {
			out = lightscale.TrimDarkness(cfg.DoublingStep, terms.Light, terms.Dark, full, darkFloor)
		} else {
			// terms.Dark is 0 when the room holds no darkness (lighting
			// plan 6: a combine of nothing reads 0, never Absent).
			out = lightscale.Trim(cfg.DoublingStep, terms.Light, full, lightTarget+terms.Dark)
		}
		if out >= full {
			// No cut at all: the source runs at full strength, so it is not "trimmed".
			t.rec.LightTrim, t.rec.LightOutput = conditions.LightFull, 0
			continue
		}
		t.rec.SetLightOutput(out)
	}
}
