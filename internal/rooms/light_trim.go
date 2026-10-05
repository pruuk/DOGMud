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

// TrimLightFor trims every adjustable, unhooded light record c holds to c's own
// eyes: the least cut from full strength that keeps this room from dazzling
// them. The records trim one after another in held order, each seeing the
// room as the previous trims left it. A pre-pass sets the bearer's eligible
// sources off first, which already keeps each source out of its own "others";
// the exclude argument to composeLightExcluding is belt and braces, kept
// because it states the intent.
//
// It is plan 5a's only trim trigger. MoveToRoom and AddMob call it once the
// mover is in the room, so arrivals trim in entry order, and nobody already
// here re-trims when someone else walks in. Nothing calls it on a round tick:
// a room that changes around a standing bearer leaves their light as it was.
func (r *Room) TrimLightFor(c *characters.Character) {
	if r == nil || c == nil {
		return
	}
	type trimmable struct {
		rec  *conditions.Condition
		spec *conditions.ConditionSpec
	}
	var todo []trimmable
	for _, rec := range c.Conditions.LightSources() {
		spec := conditions.GetConditionSpec(rec.ConditionId)
		if spec == nil || rec.Hooded || !slices.Contains(spec.Flags, conditions.Adjustable) {
			continue
		}
		todo = append(todo, trimmable{rec, spec})
	}
	// A bearer with nothing adjustable (a plain torch, or no light at all)
	// pays nothing on a move.
	if len(todo) == 0 {
		return
	}

	cfg := configs.GetLightingConfig()
	celestial := gametime.CelestialLight()
	skyFilter := r.mutatorSkyFilter()
	target := messaging.LightTrimTarget(c.NightVisionStrength(), cfg.DazzleAbove)

	// Every source about to trim leaves the room first. Otherwise a source
	// would see the later ones still at full strength, trim to nothing, and
	// hand the room to whichever source comes last: held order inverted. With
	// them out, each source sees the room with the earlier trims and nothing
	// of its own that has not trimmed yet.
	for _, t := range todo {
		t.rec.SetLightOutput(lightscale.Absent())
	}
	for _, t := range todo {
		others := r.composeLightExcluding(cfg, celestial, skyFilter, t.rec).Raw
		full := t.rec.LightMax(t.spec)
		out := lightscale.Trim(cfg.DoublingStep, others, full, target)
		if out >= full {
			// No cut at all: the source runs at full strength, so it is not "trimmed".
			t.rec.LightTrim, t.rec.LightOutput = conditions.LightFull, 0
			continue
		}
		t.rec.SetLightOutput(out)
	}
}
