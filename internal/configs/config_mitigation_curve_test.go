package configs

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The twelve mitigation curve knobs (#465), four per channel, in the order
// knee, cap, aim, bend for physical, magical, conviction.
var mitigationCurveKeys = []string{
	"PhysicalMitigationKnee", "PhysicalMitigationCap", "PhysicalMitigationAim", "PhysicalMitigationBend",
	"MagicalMitigationKnee", "MagicalMitigationCap", "MagicalMitigationAim", "MagicalMitigationBend",
	"ConvictionMitigationKnee", "ConvictionMitigationCap", "ConvictionMitigationAim", "ConvictionMitigationBend",
}

// The defaults equal the shipped values, so a test binary (which never loads
// config.yaml) runs the shipped curve.
var mitigationCurveDefaults = []ConfigFloat{0.50, 0.90, 0.05, 0.40}

func mitigationCurveValues(b Balance) []ConfigFloat {
	return []ConfigFloat{
		b.PhysicalMitigationKnee, b.PhysicalMitigationCap, b.PhysicalMitigationAim, b.PhysicalMitigationBend,
		b.MagicalMitigationKnee, b.MagicalMitigationCap, b.MagicalMitigationAim, b.MagicalMitigationBend,
		b.ConvictionMitigationKnee, b.ConvictionMitigationCap, b.ConvictionMitigationAim, b.ConvictionMitigationBend,
	}
}

func assertCurveDefaults(t *testing.T, b Balance, why string) {
	t.Helper()
	for i, v := range mitigationCurveValues(b) {
		assert.Equal(t, mitigationCurveDefaults[i%4], v, "%s: %s", why, mitigationCurveKeys[i])
	}
}

// An absent key reads 0 and takes the default on every channel.
func TestMitigationCurveKnobs_AbsentKeysTakeTheDefaults(t *testing.T) {
	b := Balance{}
	b.Validate()
	assertCurveDefaults(t, b, "empty Balance")
}

// Each bad value falls back to the shipped curve: a cap above 0.95 or not
// positive, a knee not below the cap, a negative aim or bend.
func TestMitigationCurveKnobs_InvalidValuesFallBack(t *testing.T) {
	cases := []struct {
		name                 string
		knee, cap, aim, bend ConfigFloat
		wantKnee, wantCap    ConfigFloat
		wantAim, wantBend    ConfigFloat
	}{
		{"cap above 0.95", 0.50, 0.99, 0.05, 0.40, 0.50, 0.90, 0.05, 0.40},
		{"negative cap", 0.50, -0.2, 0.05, 0.40, 0.50, 0.90, 0.05, 0.40},
		{"knee at the cap", 0.80, 0.80, 0.05, 0.40, 0.50, 0.80, 0.05, 0.40},
		{"knee above the cap", 0.85, 0.80, 0.05, 0.40, 0.50, 0.80, 0.05, 0.40},
		{"cap below the default knee", 0.60, 0.40, 0.05, 0.40, 0.50, 0.90, 0.05, 0.40},
		{"negative aim", 0.50, 0.90, -0.1, 0.40, 0.50, 0.90, 0.05, 0.40},
		{"negative bend", 0.50, 0.90, 0.05, -1, 0.50, 0.90, 0.05, 0.40},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := Balance{
				PhysicalMitigationKnee: c.knee, PhysicalMitigationCap: c.cap,
				PhysicalMitigationAim: c.aim, PhysicalMitigationBend: c.bend,
				MagicalMitigationKnee: c.knee, MagicalMitigationCap: c.cap,
				MagicalMitigationAim: c.aim, MagicalMitigationBend: c.bend,
				ConvictionMitigationKnee: c.knee, ConvictionMitigationCap: c.cap,
				ConvictionMitigationAim: c.aim, ConvictionMitigationBend: c.bend,
			}
			b.Validate()
			want := []ConfigFloat{c.wantKnee, c.wantCap, c.wantAim, c.wantBend}
			for i, v := range mitigationCurveValues(b) {
				assert.Equal(t, want[i%4], v, mitigationCurveKeys[i])
			}
		})
	}
}

// A NaN (YAML `.nan`) in any one knob takes that knob's default, never flows
// through to the curve. Each knob is poisoned alone, the other three legal.
func TestMitigationCurveKnobs_NaNFallsBack(t *testing.T) {
	nan := ConfigFloat(math.NaN())
	for poisoned, name := range []string{"knee", "cap", "aim", "bend"} {
		t.Run(name, func(t *testing.T) {
			vals := []ConfigFloat{0.50, 0.90, 0.05, 0.40}
			vals[poisoned] = nan
			b := Balance{
				PhysicalMitigationKnee: vals[0], PhysicalMitigationCap: vals[1],
				PhysicalMitigationAim: vals[2], PhysicalMitigationBend: vals[3],
				MagicalMitigationKnee: vals[0], MagicalMitigationCap: vals[1],
				MagicalMitigationAim: vals[2], MagicalMitigationBend: vals[3],
				ConvictionMitigationKnee: vals[0], ConvictionMitigationCap: vals[1],
				ConvictionMitigationAim: vals[2], ConvictionMitigationBend: vals[3],
			}
			b.Validate()
			assertCurveDefaults(t, b, "NaN "+name)
		})
	}
}

// A legal curve survives Validate untouched, including the 0.95 cap ceiling
// and a knee just under the cap.
func TestMitigationCurveKnobs_LegalValuesSurvive(t *testing.T) {
	b := Balance{
		PhysicalMitigationKnee: 0.40, PhysicalMitigationCap: 0.95,
		PhysicalMitigationAim: 0.10, PhysicalMitigationBend: 0.25,
		MagicalMitigationKnee: 0.60, MagicalMitigationCap: 0.61,
		MagicalMitigationAim: 0.01, MagicalMitigationBend: 2.0,
		ConvictionMitigationKnee: 0.30, ConvictionMitigationCap: 0.75,
		ConvictionMitigationAim: 0.05, ConvictionMitigationBend: 0.40,
	}
	b.Validate()
	assert.Equal(t, []ConfigFloat{0.40, 0.95, 0.10, 0.25, 0.60, 0.61, 0.01, 2.0, 0.30, 0.75, 0.05, 0.40},
		mitigationCurveValues(b))
}

// config.yaml names all twelve knobs at the owner's values (knee 0.50, cap
// 0.90, aim 0.05, bend 0.40 on every channel). Read from the file because a
// test binary never loads it; checked before Validate, which would otherwise
// paper over a missing or misplaced key with the same defaults.
func TestShippedConfigCarriesTheMitigationCurve(t *testing.T) {
	src := shippedConfigSource(t)
	for _, k := range mitigationCurveKeys {
		assert.True(t, bytesContainsKey(src, k+":"), "config.yaml must name %s", k)
	}
	cfg, err := loadConfig(src)
	require.NoError(t, err)
	assertCurveDefaults(t, cfg.Balance, "shipped config.yaml before Validate")
}
