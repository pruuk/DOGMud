package configs

import "testing"

func TestLightingDefaultsAreTheShippedCalibration(t *testing.T) {
	var b Balance
	b.Validate()
	if b.LightDoublingStep != 8 {
		t.Errorf("LightDoublingStep = %v, want 8", b.LightDoublingStep)
	}
	// 🔴 This assertion is the one that matters most in this test. A config
	// file that omits WorldLatitude (as config.yaml did until lighting plan 6)
	// arrives at a bare Balance, so this is not only a test fixture. If this
	// ever reads zero,
	// DOGMud is running with no latitude: a flat night, no seasons, and the
	// whole celestial model unreachable.
	if b.WorldLatitude != 46.5 {
		t.Errorf("WorldLatitude = %v, want 46.5", b.WorldLatitude)
	}
	if b.LightEquinoxNoon != 70 {
		t.Errorf("LightEquinoxNoon = %v, want 70", b.LightEquinoxNoon)
	}
	if b.LightMoonsFull != 35 {
		t.Errorf("LightMoonsFull = %v, want 35", b.LightMoonsFull)
	}
	if b.LightStarlight != 10 {
		t.Errorf("LightStarlight = %v, want 10", b.LightStarlight)
	}
	if b.LightMoonWeightSwiftmoon != 4 || b.LightMoonWeightWanderer != 1 || b.LightMoonWeightEye != 0.5 {
		t.Errorf("moon weights = %v/%v/%v, want 4/1/0.5",
			b.LightMoonWeightSwiftmoon, b.LightMoonWeightWanderer, b.LightMoonWeightEye)
	}
}

func TestDoublingStepRejectsNonPositive(t *testing.T) {
	for _, v := range []ConfigFloat{0, -3} {
		var b Balance
		b.LightDoublingStep = v
		b.Validate()
		if b.LightDoublingStep != 8 {
			t.Errorf("step %v survived validation as %v", v, b.LightDoublingStep)
		}
	}
}

// Latitude beyond the polar circles produces days with no sunrise or no sunset,
// which the model handles but which is almost never intended, so out of range
// reverts. Zero means UNSET and is coerced to the default.
//
// 🔴 This is the load-bearing assertion of the whole celestial model, not a
// boundary nicety. A config file that omits WorldLatitude (as config.yaml did
// until lighting plan 6) runs a bare Balance, whose WorldLatitude is zero. If zero
// were honoured as "no latitude", DOGMud would ship with a flat night, no
// seasons, and the entire model unreachable. Do not "fix" this test by making
// zero survive.
func TestWorldLatitudeCoercesZeroAndRejectsOutOfRange(t *testing.T) {
	var b Balance
	b.WorldLatitude = 0
	b.Validate()
	if b.WorldLatitude != 46.5 {
		t.Errorf("zero latitude survived as %v; zero means unset and must coerce to 46.5", b.WorldLatitude)
	}

	for _, v := range []ConfigFloat{-91, 91} {
		var b2 Balance
		b2.WorldLatitude = v
		b2.Validate()
		if b2.WorldLatitude != 46.5 {
			t.Errorf("latitude %v survived as %v", v, b2.WorldLatitude)
		}
	}

	// A southern or near-equatorial latitude an operator actually authored must
	// survive untouched, or the coercion above would be swallowing real values.
	for _, v := range []ConfigFloat{-46.5, 0.001, 66} {
		var b3 Balance
		b3.WorldLatitude = v
		b3.Validate()
		if b3.WorldLatitude != v {
			t.Errorf("authored latitude %v was changed to %v", v, b3.WorldLatitude)
		}
	}
}

// Starlight must sit below the all-moons-full value or the moon curve inverts
// and a full moon reads darker than a new one. Validated as a pair, the
// LightBlindBelow/LightDimBelow precedent.
func TestMoonRangeIsValidatedAsAPair(t *testing.T) {
	var b Balance
	b.LightStarlight = 40
	b.LightMoonsFull = 20
	b.Validate()
	if b.LightStarlight != 10 || b.LightMoonsFull != 35 {
		t.Errorf("inverted pair survived as %v/%v", b.LightStarlight, b.LightMoonsFull)
	}
}

func TestMoonWeightsRejectAllZero(t *testing.T) {
	var b Balance
	b.LightMoonWeightSwiftmoon = 0
	b.LightMoonWeightWanderer = 0
	b.LightMoonWeightEye = 0
	b.Validate()
	if b.LightMoonWeightSwiftmoon != 4 {
		t.Errorf("all-zero weights survived as %v", b.LightMoonWeightSwiftmoon)
	}
}

// Every field of Lighting is asserted, not a sample of them.
//
// 🔑 GetLightingConfig is a hand-written field-by-field mapping between two
// structs whose names deliberately differ (Balance.LightMoonsFull becomes
// Lighting.MoonsFull), which is exactly the shape where a copy-paste swap hides
// silently: reading Starlight into MoonsFull would compile, pass any sampled
// test, and inverts the moon curve at runtime. Each value below is distinct so
// a transposition cannot pass by coincidence.
func TestGetLightingConfigMirrorsBalance(t *testing.T) {
	c := GetConfig()
	c.Balance.LightBlindBelow = 30
	c.Balance.LightDimBelow = 31
	c.Balance.LightExitsAbove = 32
	c.Balance.LightDefaultVisionStrength = 13
	c.Balance.LightRealMinimum = 7
	c.Balance.LightDoublingStep = 11
	c.Balance.WorldLatitude = 12.5
	c.Balance.LightEquinoxNoon = 14.5
	c.Balance.LightStarlight = 15.5
	c.Balance.LightMoonsFull = 16.5
	c.Balance.LightMoonWeightSwiftmoon = 17.5
	c.Balance.LightMoonWeightWanderer = 18.5
	c.Balance.LightMoonWeightEye = 19.5
	SetConfigForTest(t, c)

	got := GetLightingConfig()

	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"BlindBelow", float64(got.BlindBelow), 30},
		{"DimBelow", float64(got.DimBelow), 31},
		{"ExitsAbove", float64(got.ExitsAbove), 32},
		{"DefaultVisionStrength", float64(got.DefaultVisionStrength), 13},
		{"RealMinimum", float64(got.RealMinimum), 7},
		{"DoublingStep", got.DoublingStep, 11},
		{"WorldLatitude", got.WorldLatitude, 12.5},
		{"EquinoxNoon", got.EquinoxNoon, 14.5},
		{"Starlight", got.Starlight, 15.5},
		{"MoonsFull", got.MoonsFull, 16.5},
		{"MoonWeightSwiftmoon", got.MoonWeightSwiftmoon, 17.5},
		{"MoonWeightWanderer", got.MoonWeightWanderer, 18.5},
		{"MoonWeightEye", got.MoonWeightEye, 19.5},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}
