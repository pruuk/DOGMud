package rooms

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// LightLevel reports the room's light on the graded -100 to 100 scale.
//
// Three kinds of term compose it, all on one operator, the sum of linear
// brightness (lightscale.Combine, lighting plan 6):
//
//  1. The sky, which is the celestial term attenuated by this room's sky
//     fraction. A room with no sky receives a term of 0, which adds nothing.
//  2. The room's own lamp, if it has one, joining the combine rather than
//     acting as a floor, so a lantern-lit tavern plus a carried torch does not
//     double-count.
//  3. Everything anyone in the room carries, one term per light.
//
// Any real light then reads at least LightRealMinimum (plan 6, ruling O3).
// Every carried darkness is combined on the same operator and taken away
// from the result in points, which is the only way a room reads below 0
// (lighting plan 5d; plan 6, ruling O1).
//
// Weather attenuates the SKY only, through each active mutator's skylight
// fraction: a blizzard does not dim a lantern.
func (r *Room) LightLevel() int {
	return r.lightLevel(configs.GetLightingConfig(), gametime.CelestialLight(), gametime.LampsLit())
}

// IsLit reports whether a normal observer can see anything at all here.
//
// 🔑 This is the predicate plan 1's design promised and never built. Fifteen
// call sites were hand-rolling `LightLevel() >= GetBalanceConfig().LightBlindBelow`,
// each copying a 424-field struct (99.75 ns measured) to read one int. This
// reads the narrow lighting config once.
func (r *Room) IsLit() bool {
	cfg := configs.GetLightingConfig()
	return r.lightLevel(cfg, gametime.CelestialLight(), gametime.LampsLit()) >= cfg.BlindBelow
}

// lightLevel is LightLevel with its clock reads (the celestial light and
// whether the street lamps are lit, gametime.LampsLit) injected, so it is
// testable without global state and so a caller holding both can avoid
// reading twice.
func (r *Room) lightLevel(cfg configs.Lighting, celestial float64, lampsLit bool) int {
	return r.lightLevelWithSkyFilter(cfg, celestial, lampsLit, r.mutatorSkyFilter())
}

// lightLevelWithSkyFilter is the composition itself, with the active mutators'
// sky filter already multiplied out, so tests can drive it directly.
func (r *Room) lightLevelWithSkyFilter(cfg configs.Lighting, celestial float64, lampsLit bool, skyFilter float64) int {
	return r.composeLight(cfg, celestial, lampsLit, skyFilter).Level
}

// LightTerms is the room's light broken into the terms LightLevel combines,
// for a caller that needs to know WHY the light is what it is.
// internal/lightnotice names the cause of a band change from them.
type LightTerms struct {
	// Level is exactly LightLevel(): both come from composeLight.
	Level int
	// Raw is the net light Level rounds and clamps: Light minus Dark. An
	// unlit room with no darkness is 0 (lighting plan 5d, ruling D3).
	Raw float64
	// Light is the combined light of the sky, the lamp and every carried
	// light, raised to LightRealMinimum when it is above 0 but below it
	// (lighting plan 6, ruling O3); 0 when nothing lights the room, and never
	// below 0. A light's or a darkness's trim solves against it.
	Light float64
	// Dark is the combined darkness every carried darkness takes away, on
	// the same combine; 0 when nobody carries one (lighting plan 5d).
	Dark float64
	// Sky is the sky term after the sky fraction and the weather filter, in
	// light-scale units; 0 when the room has no sky.
	Sky float64
	// SkyFilter is the fraction of the sky active weather lets through: the
	// product of the active mutators' skylight values, 1 when clear.
	SkyFilter float64
	// Lamp is the room's own lamp; 0 when HasLamp is false.
	Lamp    int
	HasLamp bool
	// Carried reports that someone in the room carries a light.
	Carried bool
	// Darkened reports that someone in the room carries a darkness.
	Darkened bool
	// Fixture is the combined light of the room's lit light fixtures (items
	// with `fixture: light`, internal/itemlight); 0 when
	// none is lit. A fixture is part of the room, never a carried light, so
	// it never sets Carried (lighting 5e, X5).
	Fixture float64
	// CarriedLight is the combine of carried light alone; 0 when nobody
	// here carries a lit light. A lantern
	// dimming while still lit, or a second light arriving, moves it where
	// Carried does not (lighting 5e, X5).
	CarriedLight float64
}

// LightTerms reports the terms behind LightLevel, from the same single
// computation.
func (r *Room) LightTerms() LightTerms {
	return r.composeLight(configs.GetLightingConfig(), gametime.CelestialLight(), gametime.LampsLit(), r.mutatorSkyFilter())
}

// composeLight is the one computation behind LightLevel and LightTerms.
func (r *Room) composeLight(cfg configs.Lighting, celestial float64, lampsLit bool, skyFilter float64) LightTerms {
	return r.composeLightExcluding(cfg, celestial, lampsLit, skyFilter, nil)
}

// composeLightExcluding is composeLight with one carried record left out of
// whichever combine it belongs to, which is the room a trimming source sees:
// everything except itself.
func (r *Room) composeLightExcluding(cfg configs.Lighting, celestial float64, lampsLit bool, skyFilter float64, exclude *conditions.Condition) LightTerms {
	carried, dark := r.carriedTerms(exclude)
	fxLight, fxDark := itemlight.Terms(r.RoomId)
	return r.composeWithFixtures(cfg, celestial, lampsLit, skyFilter, carried, dark, fxLight, fxDark)
}

// composeWith is the composition with the carried light and darkness terms
// supplied and no fixtures, so a test needs no users, mobs or items. It
// composes with every street lamp lit, which is what every test written
// before the street lamp's hours (lighting plan 6) assumed; a test of the
// lamp's hours calls composeWithFixtures with lampsLit set.
func (r *Room) composeWith(cfg configs.Lighting, celestial, skyFilter float64, carried, dark []float64) LightTerms {
	return r.composeWithFixtures(cfg, celestial, true, skyFilter, carried, dark, nil, nil)
}

// composeWithFixtures is the composition itself, with every term supplied.
//
// Lights combine on the linear sum; darknesses combine among themselves the
// same way; the net light is the combined light (0 when none, at least
// LightRealMinimum when any) minus the combined darkness (0 when none),
// clamped to [-100, 100] (lighting plan 5d, owner decision 1; plan 6). A lit light fixture is one term in
// the light combine and a darkness fixture one in the darkness combine,
// beside what people carry (lighting 5e, Rule 10). Fixtures never trim.
func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial float64, lampsLit bool, skyFilter float64, carried, dark, fixtureLight, fixtureDark []float64) LightTerms {
	step := cfg.DoublingStep
	if !(step > 0) {
		step = 1
	}

	out := LightTerms{SkyFilter: skyFilter}
	terms := make([]float64, 0, 2+len(fixtureLight)+len(carried))

	// 1. The sky, attenuated by this room's fraction and then by any weather
	// filtering it. A filter multiplies the fraction, and the fraction
	// multiplies the sky's brightness: 0.5 removes very nearly one doubling
	// step by day and less at night, where there is less light to take, so a
	// storm is merely gloomy at noon and blinding at midnight. A fraction of
	// zero reads 0, which adds nothing, so a cave needs no special case.
	out.Sky = lightscale.Attenuate(step, celestial, r.skyLightFraction()*skyFilter)
	terms = append(terms, out.Sky)

	// 2. The room's own lamp.
	if lamp, ok := r.lampValue(lampsLit); ok {
		out.Lamp, out.HasLamp = lamp, true
		terms = append(terms, float64(lamp))
	}

	// 3. Every lit light fixture, each its own term (lighting 5e): part of
	// the room, like its lamp, never a carried light.
	out.Fixture = lightscale.Combine(step, fixtureLight...)
	terms = append(terms, fixtureLight...)

	// 4. Every light anyone here carries, each its own term (lighting plan 5a):
	// a candle and a torch are different sources, and two torches are one
	// doubling step brighter than one.
	out.CarriedLight = lightscale.Combine(step, carried...)
	if len(carried) > 0 {
		out.Carried = true
		terms = append(terms, carried...)
	}

	// The combined light. It cannot read below 0 (lighting plan 6): no light
	// at all reads exactly 0, an unlit cave, and only magical darkness goes
	// below it.
	//
	// The floor (lighting plan 6, owner ruling O3): any real light reads at
	// least LightRealMinimum before darkness is subtracted, so a sliver of
	// starlight through a crack never rounds to the same 0 as a sealed cave.
	// Light carries the floored value, because it is the light darkness
	// subtracts from, and so the light a darkness trim must solve against.
	out.Light = lightscale.Combine(step, terms...)
	if floor := float64(cfg.RealMinimum); out.Light > 0 && out.Light < floor {
		out.Light = floor
	}
	v := out.Light

	// 5. Every darkness anyone here carries (lighting plan 5d), and every
	// darkness fixture (lighting 5e), combined among themselves on the same
	// linear sum and taken away from the light in points. Two darknesses of 50 take
	// 58, not 100. Darkened still means a CARRIED darkness.
	darkTerms := make([]float64, 0, len(dark)+len(fixtureDark))
	darkTerms = append(darkTerms, dark...)
	darkTerms = append(darkTerms, fixtureDark...)
	out.Dark = lightscale.Combine(step, darkTerms...)
	if len(dark) > 0 {
		out.Darkened = true
	}
	v -= out.Dark
	out.Raw = v
	out.Level = levelOfRaw(v)
	return out
}

// levelOfRaw rounds a net light reading to a level and clamps it to the
// scale's [-100, 100]. The clamp happens in floating point BEFORE the int
// conversion, because int() of a float beyond the int range (or of +Inf) is
// implementation-defined in Go and on amd64 yields the most negative int,
// which would turn a blinding room into maximal magical darkness. +Inf reads
// 100 and -Inf -100. NaN (an arithmetic mistake upstream, which
// lightscale already refuses to produce) reads 0, the darkest natural dark,
// rather than either extreme.
func levelOfRaw(v float64) int {
	switch {
	case math.IsNaN(v):
		return 0
	case v >= 100:
		return 100
	case v <= -100:
		return -100
	}
	return int(math.Round(v))
}

// carriedTerms is every carried light term and every carried darkness term in
// the room, leaving out one record (the source being trimmed) when exclude is
// non-nil. One pass over the room's occupants: each bearer's records are read
// exactly once.
func (r *Room) carriedTerms(exclude *conditions.Condition) (light, dark []float64) {
	add := func(c *characters.Character) {
		for _, rec := range c.Conditions.LightAndDarknessSources() {
			if rec == exclude {
				continue
			}
			spec := conditions.GetConditionSpec(rec.ConditionId)
			v, ok := rec.LightNow(spec)
			if !ok {
				continue
			}
			if spec.IsDarknessSource() {
				dark = append(dark, v)
			} else {
				light = append(light, v)
			}
		}
	}
	for _, id := range r.mobs {
		if m := mobs.GetInstance(id); m != nil {
			add(&m.Character)
		}
	}
	for _, id := range r.players {
		if u := users.GetByUserId(id); u != nil && u.Character != nil {
			add(u.Character)
		}
	}
	return light, dark
}

// mutatorSkyFilter multiplies the skylight fraction of every active mutator
// that declares one; 1 means nothing filters the sky. Outdoor-only mutators
// never reach an indoor biome (ActiveMutators skips them), so weather stays
// out of roofed rooms.
func (r *Room) mutatorSkyFilter() float64 {
	filter := 1.0
	for mut := range r.ActiveMutators {
		if spec := mut.GetSpec(); spec != nil && spec.SkyLight != nil {
			filter *= *spec.SkyLight
		}
	}
	return filter
}

// skyLightFraction is this room's sky fraction: its own override if it has one,
// otherwise its biome's.
//
// ⚠️ GetBiome can return nil when the biome registry has not been loaded, which
// is the normal state in a unit test that does not read _datafiles. A nil check
// here is not defensive padding: without it every table-driven lighting test
// must load the whole world first, and a nil dereference in LightLevel would
// take down a live room read.
func (r *Room) skyLightFraction() float64 {
	if r.SkyLight != nil {
		return *r.SkyLight
	}
	if b := r.GetBiome(); b != nil {
		return b.SkyLightFraction()
	}
	return 1.0
}

// lampValue is this room's own light source as it burns when the street lamps
// are (or are not) lit, and whether it burns at all. A room's own `lamp:`
// override burns at all hours; a biome lamp marked streetlamp only while
// gametime.LampsLit (lighting plan 6, owner ruling O4 as amended). Nil-safe
// for the same reason as skyLightFraction.
func (r *Room) lampValue(lampsLit bool) (int, bool) {
	if r.Lamp != nil {
		return *r.Lamp, true
	}
	if b := r.GetBiome(); b != nil {
		return b.LampAt(lampsLit)
	}
	return 0, false
}
