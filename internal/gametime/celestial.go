package gametime

import (
	"math"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const (
	// axialTiltDegrees is the planet's obliquity, which sets how far
	// declination swings across the year and therefore how long the longest
	// night is. Earth's value; world.md gives no other.
	axialTiltDegrees = 23.44

	// daysPerYear matches GameDate.ReCalculate, which divides the year into
	// 365 days. If that ever changes, this must change with it.
	daysPerYear = 365.0

	// moonReferenceIntensity is the floor the moon curve is measured from: the
	// light of a sky with every moon new, in the same arbitrary intensity units
	// the moon weights use. It shapes how fast moonlight rises off the
	// starlight anchor, and the two anchors themselves are config.
	//
	// It is a constant rather than a knob because nothing yet needs to turn it
	// independently of the anchors. Plan 6's balance pass may promote it.
	moonReferenceIntensity = 0.923

	// poleReferenceEpsilon is how close cos(latitude) may come to zero before
	// SunLight gives up and calls the sun absent.
	//
	// 🪤 A plain `<= 0` test does NOT catch the poles, which is the one case it
	// was written for. The config validator accepts a latitude of exactly 90,
	// and cos(90 degrees) in float64 is 6.12e-17: a tiny POSITIVE number, so
	// the guard is skipped, log2 of it is about -53.9, and the calibration
	// hands back a sun of roughly 490 on a scale that ends at 100. No NaN, no
	// panic, just a silently absurd number. Measured, not reasoned about:
	// reverting this to `<= 0` reddens TestPoleLatitudeGivesAnAbsentSunNotAnAbsurdOne
	// with "sun 490.22805498561 is off the -100..100 scale".
	//
	// The bound is generous on purpose. Within about a thousandth of a degree
	// of a pole the model has nothing useful to say anyway, so there is no cost
	// to giving up early and a real cost to giving up late.
	poleReferenceEpsilon = 1e-9
)

// declinationDegrees is the sun's declination on a given day of the year:
// zero at the equinoxes, plus or minus the axial tilt at the solstices.
//
// The plus-ten offset puts midwinter near day 356, matching the northern
// calendar the world's seasons are described in.
func declinationDegrees(dayOfYear int) float64 {
	return -axialTiltDegrees * math.Cos(2*math.Pi*(float64(dayOfYear)+10)/daysPerYear)
}

// halfDayHours is the time from local noon to sunset, in hours.
//
// Beyond the polar circles the arccosine has no solution, which is a real
// physical state and not an error: the sun either never sets or never rises. It
// clamps to 12 and 0 respectively rather than returning NaN, which would
// poison every light in the world.
func halfDayHours(latitudeDegrees float64, dayOfYear int) float64 {
	phi := latitudeDegrees * math.Pi / 180
	dec := declinationDegrees(dayOfYear) * math.Pi / 180
	c := -math.Tan(phi) * math.Tan(dec)
	switch {
	case c <= -1:
		return 12 // polar day: the sun never sets
	case c >= 1:
		return 0 // polar night: the sun never rises
	}
	return math.Acos(c) * 180 / math.Pi / 15
}

// NightHoursAt reports how many hours of night a given latitude sees on a given
// day of the year. At the equator it is twelve every day; at 46.5 degrees it
// runs from 8h23m at midsummer to 15h37m at midwinter.
func NightHoursAt(latitudeDegrees float64, dayOfYear int) float64 {
	return 24 - 2*halfDayHours(latitudeDegrees, dayOfYear)
}

// NightAt reports whether an hour of a day of the year is night at a latitude:
// the night is centred on midnight and NightHoursAt long. It is the one
// boundary GameDate.Night (and so IsNight) uses, exposed so a caller holding a
// day and an hour (a light golden, the street lamp's tests) can ask
// without moving the round counter.
func NightAt(latitudeDegrees float64, dayOfYear int, hour float64) bool {
	halfNight := NightHoursAt(latitudeDegrees, dayOfYear) / 2
	return hour >= 24-halfNight || hour < halfNight
}

// LampsLitAt reports whether the world's street lamps burn, given whether it
// is night, the clear-sky celestial light (CelestialLight: sun and moons
// before any sky fraction or weather) and the faces edge LightDimBelow. A
// lamp burns while it is night OR while that clear sky reads below the faces
// edge, the way a lamplighter works by eye (lighting plan 6, owner ruling
// O4 as amended): a midwinter morning still too dim to read a face keeps its
// lamps. The input is the clear sky, never the weathered one, so a storm
// lights no lamp and every lamp in the world changes at the same moment.
//
// An Absent celestial (-Inf) is below any edge; a NaN one, which only an
// arithmetic mistake produces, leaves the lamps to the night alone.
func LampsLitAt(night bool, celestial float64, dimBelow int) bool {
	return night || celestial < float64(dimBelow)
}

// LampsLit is LampsLitAt on the current round: IsNight, CelestialLight and
// the shipped LightDimBelow. The street lamps (rooms.BiomeInfo.StreetLamp)
// and the `time_of_day period: lamplit` behaviour condition both read it, so
// a biome lamp and a lantern fixture light and go out together.
func LampsLit() bool {
	return LampsLitAt(IsNight(), CelestialLight(), configs.GetLightingConfig().DimBelow)
}

// solarSinAltitude is the sine of the sun's altitude above the horizon, which
// is also the share of its light falling on level ground. Negative means below
// the horizon.
func solarSinAltitude(latitudeDegrees float64, dayOfYear int, hour float64) float64 {
	phi := latitudeDegrees * math.Pi / 180
	dec := declinationDegrees(dayOfYear) * math.Pi / 180
	return math.Sin(phi)*math.Sin(dec) +
		math.Cos(phi)*math.Cos(dec)*math.Cos(2*math.Pi*(hour-12)/24)
}

// SunLight is the sun's contribution to the sky at a given moment.
//
//	sun = sunFull + step * log2( sin altitude )
//
// 🔑 This has no "night" case. Below the horizon sin(altitude) is non-positive,
// the logarithm is undefined, and the term is simply Absent. The flat-topped
// middle of the day also comes for free, because sine is flat near its peak,
// which is why no flatness exponent exists.
//
// 🔑 The calibration anchor is exact rather than sampled. At an equinox the
// declination is zero, so sin(altitude) at noon is exactly cos(latitude); no
// magic day-of-year number is needed to find it.
//
// Deliberately omitted: atmospheric extinction, which would take a low winter
// sun further down than sin(altitude) alone, and the lore's 10-15% difference
// in seasonal lengths from orbital eccentricity (world.md:44). Neither changes
// a sight band at the shipped calibration.
func SunLight(cfg configs.Lighting, dayOfYear int, hour float64) float64 {
	s := solarSinAltitude(cfg.WorldLatitude, dayOfYear, hour)
	if s <= 0 {
		return lightscale.Absent()
	}
	reference := math.Cos(cfg.WorldLatitude * math.Pi / 180)
	if reference <= poleReferenceEpsilon {
		// A pole. There is no equinox noon to calibrate against, so the model
		// has nothing to say; treat the sun as absent rather than calibrating
		// against a logarithm that runs away. See poleReferenceEpsilon for why
		// this is not a comparison against zero.
		return lightscale.Absent()
	}
	step := cfg.DoublingStep
	if !(step > 0) {
		step = 1
	}
	sunFull := cfg.EquinoxNoon - step*math.Log2(reference)
	return sunFull + step*math.Log2(s)
}

// MoonLight is the three moons' combined contribution, given each moon's
// fullness in [0,1] as the existing phase functions report it.
//
// The weights are relative light at full, with albedo folded in: from the
// ground a large dull moon and a small bright one are indistinguishable, so
// only the product is observable.
//
// The result is interpolated between the two configured anchors on a
// logarithmic intensity axis, so the curve rises fast off starlight and
// flattens as the sky fills, which is how light actually reads.
func MoonLight(cfg configs.Lighting, swiftmoon, wanderer, eye float64) float64 {
	weightTotal := cfg.MoonWeightSwiftmoon + cfg.MoonWeightWanderer + cfg.MoonWeightEye
	if weightTotal <= 0 {
		return cfg.Starlight
	}
	intensity := moonReferenceIntensity +
		cfg.MoonWeightSwiftmoon*clamp01(swiftmoon) +
		cfg.MoonWeightWanderer*clamp01(wanderer) +
		cfg.MoonWeightEye*clamp01(eye)

	span := math.Log2((moonReferenceIntensity + weightTotal) / moonReferenceIntensity)
	if span <= 0 {
		return cfg.Starlight
	}
	position := math.Log2(intensity/moonReferenceIntensity) / span
	return cfg.Starlight + (cfg.MoonsFull-cfg.Starlight)*position
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}

// The celestial memo. One value for the WHOLE WORLD per round, not one per room.
//
// 🔑 This is NOT a performance measure and must not grow into a per-room cache.
// Measured, the uncached computation is 332 ns; at 500 LightLevel calls in a
// four-second round that is 0.005% of the round, and about 0.03% scaled for the
// single-CPU production droplet. The memo exists only because it is three lines.
// See the amendment spec's Performance section before "optimising" anything here.
//
// A config change mid-round is reflected on the next round, which is acceptable
// for a value that is constant within a round by construction.
var (
	celestialMu    sync.Mutex
	celestialRound uint64
	celestialValue float64
	celestialKnown bool
)

// CelestialLight is the sky's light at the current round: sun and moons
// combined, before any sky fraction, lamp or weather is applied.
func CelestialLight() float64 {
	round := util.GetRoundCount()

	celestialMu.Lock()
	defer celestialMu.Unlock()
	if celestialKnown && celestialRound == round {
		return celestialValue
	}

	cfg := configs.GetLightingConfig()
	gd := GetDate(round)
	hour := float64(gd.Hour24) + gd.MinuteFloat/60

	// 🔑 PhasesAtRound, not GetAllPhases. Both halves of this value must come
	// from the SAME round, or the memo files a sun from round N beside a moon
	// from round N+1 under key N. GetAllPhases reads the global counter itself,
	// so it cannot be pinned; this one takes the round we already captured.
	swift, wander, eye := PhasesAtRound(round)

	celestialValue = lightscale.Combine(cfg.DoublingStep,
		SunLight(cfg, gd.Day, hour),
		MoonLight(cfg, swift, wander, eye),
	)
	celestialRound = round
	celestialKnown = true
	return celestialValue
}

// ClearCelestialMemoForTest empties the celestial light memo.
//
// It exists because the memo above carries no config fingerprint either,
// only a round number: a test that changes lighting config and then calls
// CelestialLight() at a round another test already memoised under a
// different config silently gets that other test's answer. Named ForTest
// in the same spirit as gametime.ClearDateCacheForTest,
// configs.SetConfigForTest and util.SetRoundCountForTest.
//
// Production has no reason to call this: the round counter only advances,
// so the memo is never asked about a round it already answered under a
// different config.
func ClearCelestialMemoForTest() {
	celestialMu.Lock()
	defer celestialMu.Unlock()
	celestialKnown = false
	celestialRound = 0
	celestialValue = 0
}
