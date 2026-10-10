package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func TestSkillMultiplier(t *testing.T) {
	// Rank 0 should return base (1.0)
	m0 := SkillMultiplier(0)
	if m0 != 1.0 {
		t.Errorf("SkillMultiplier(0) = %f, want 1.0", m0)
	}

	// Negative rank should return base
	mNeg := SkillMultiplier(-5)
	if mNeg != 1.0 {
		t.Errorf("SkillMultiplier(-5) = %f, want 1.0", mNeg)
	}

	// Rank 50 (soft cap) should return max (3.0)
	m50 := SkillMultiplier(50)
	if math.Abs(m50-3.0) > 0.01 {
		t.Errorf("SkillMultiplier(50) = %f, want ~3.0", m50)
	}

	// Rank 25 should be ~2.41 (1 + 2*sqrt(0.5))
	m25 := SkillMultiplier(25)
	expected25 := 1.0 + 2.0*math.Sqrt(0.5)
	if math.Abs(m25-expected25) > 0.01 {
		t.Errorf("SkillMultiplier(25) = %f, want ~%f", m25, expected25)
	}

	// Monotonically increasing
	prev := SkillMultiplier(0)
	for r := 1; r <= 50; r++ {
		cur := SkillMultiplier(r)
		if cur < prev {
			t.Errorf("SkillMultiplier(%d) = %f < SkillMultiplier(%d) = %f", r, cur, r-1, prev)
		}
		prev = cur
	}

	// Above soft cap should be clamped to max
	m100 := SkillMultiplier(100)
	if math.Abs(m100-3.0) > 0.01 {
		t.Errorf("SkillMultiplier(100) = %f, want ~3.0 (capped at soft cap)", m100)
	}
}

func TestCalcRawDamage_Physical(t *testing.T) {
	// Physical: stat * SkillMult * itemMult * MeleeDamageScale(0.30)
	// Stat 100, rank 0, mult 1.0 → 100 * 1.0 * 1.0 * 0.30 = 30
	raw := CalcRawDamage(100, 0, 1.0, ChannelPhysical)
	if math.Abs(raw-30.0) > 0.01 {
		t.Errorf("CalcRawDamage physical(100, 0, 1.0) = %f, want 30.0", raw)
	}

	// Stat 100, rank 50, mult 1.0 → 100 * 3.0 * 1.0 * 0.30 = 90
	raw2 := CalcRawDamage(100, 50, 1.0, ChannelPhysical)
	if math.Abs(raw2-90.0) > 0.01 {
		t.Errorf("CalcRawDamage physical(100, 50, 1.0) = %f, want 90.0", raw2)
	}

	// Zero item multiplier should fallback to 0.30
	// 100 * 1.0 * 0.30 * 0.30 = 9.0
	raw3 := CalcRawDamage(100, 0, 0, ChannelPhysical)
	if math.Abs(raw3-9.0) > 0.01 {
		t.Errorf("CalcRawDamage physical(100, 0, 0) = %f, want 9.0 (fallback)", raw3)
	}
}

func TestCalcRawDamage_Magical(t *testing.T) {
	// Magical: stat * SkillMult * itemMult * SpellDamageScale(1.0)
	// No stat normalization — flat multiplier
	// Stat 100, rank 0, mult 1.0 → 100 * 1.0 * 1.0 * 1.0 = 100
	raw := CalcRawDamage(100, 0, 1.0, ChannelMagical)
	if math.Abs(raw-100.0) > 0.01 {
		t.Errorf("CalcRawDamage magical(100, 0, 1.0) = %f, want 100.0", raw)
	}
}

func TestCalcRawDamage_Conviction(t *testing.T) {
	// Conviction: stat * SkillMult * itemMult * RhetoricDamageScale(1.0)
	// Stat 100, rank 0, mult 0.5 → 100 * 1.0 * 0.5 * 1.0 = 50
	raw := CalcRawDamage(100, 0, 0.5, ChannelConviction)
	if math.Abs(raw-50.0) > 0.01 {
		t.Errorf("CalcRawDamage conviction(100, 0, 0.5) = %f, want 50.0", raw)
	}
}

func TestApplyMitigation(t *testing.T) {
	c := shippedCurveForTest

	// No mitigation
	result := ApplyMitigation(100, 0, c)
	if math.Abs(result-100.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 0, curve) = %f, want 100.0", result)
	}

	// 50% mitigation, at the knee: counts in full
	result2 := ApplyMitigation(100, 0.50, c)
	if math.Abs(result2-50.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 0.50, curve) = %f, want 50.0", result2)
	}

	// 100% raw bends to 75% effective (#465): 100 -> 25
	result3 := ApplyMitigation(100, 1.00, c)
	if math.Abs(result3-25.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 1.00, curve) = %f, want 25.0 (bent)", result3)
	}

	// Negative mitigation treated as 0
	result4 := ApplyMitigation(100, -0.5, c)
	if math.Abs(result4-100.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, -0.5, curve) = %f, want 100.0", result4)
	}
}

func TestResourceMultiplier(t *testing.T) {
	// 100% resource → 1.0 (no penalty)
	m100 := ResourceMultiplier(100, 100, 0.28)
	if math.Abs(m100-1.0) > 0.001 {
		t.Errorf("ResourceMultiplier(100, 100, 0.28) = %f, want 1.0", m100)
	}

	// 0% resource → 1 - penaltyMax = 0.72
	m0 := ResourceMultiplier(0, 100, 0.28)
	if math.Abs(m0-0.72) > 0.01 {
		t.Errorf("ResourceMultiplier(0, 100, 0.28) = %f, want ~0.72", m0)
	}

	// 5% resource → ~0.747 (penalty ~25%)
	m5 := ResourceMultiplier(5, 100, 0.28)
	if m5 < 0.73 || m5 > 0.76 {
		t.Errorf("ResourceMultiplier(5, 100, 0.28) = %f, want ~0.747", m5)
	}

	// 50% resource → ~0.93 (barely noticeable)
	m50 := ResourceMultiplier(50, 100, 0.28)
	if m50 < 0.92 || m50 > 0.94 {
		t.Errorf("ResourceMultiplier(50, 100, 0.28) = %f, want ~0.93", m50)
	}

	// Monotonically decreasing as resource drops
	prev := ResourceMultiplier(100, 100, 0.28)
	for pct := 99; pct >= 0; pct-- {
		cur := ResourceMultiplier(pct, 100, 0.28)
		if cur > prev+0.001 { // small tolerance for float
			t.Errorf("ResourceMultiplier(%d, 100, 0.28) = %f > prev %f — not monotonic", pct, cur, prev)
		}
		prev = cur
	}

	// Edge case: max <= 0 → 1.0
	mZeroMax := ResourceMultiplier(50, 0, 0.28)
	if mZeroMax != 1.0 {
		t.Errorf("ResourceMultiplier(50, 0, 0.28) = %f, want 1.0", mZeroMax)
	}
}

func TestGetConvictionDamageDescription(t *testing.T) {
	// Should not panic on zero max
	desc := GetConvictionDamageDescription(10, 0)
	if desc == "" {
		t.Error("GetConvictionDamageDescription returned empty string")
	}

	// Various thresholds
	desc1 := GetConvictionDamageDescription(2, 100)
	if desc1 != "a feeble jab at their resolve" {
		t.Errorf("expected feeble jab, got: %s", desc1)
	}

	desc2 := GetConvictionDamageDescription(60, 100)
	if desc2 != "a devastating attack on their will" {
		t.Errorf("expected devastating attack, got: %s", desc2)
	}
}

// ─── Stage 40.2: DamageScale and MitigationCurveFor edge cases ─────────────

func TestDamageScale(t *testing.T) {
	tests := []struct {
		name    string
		channel DamageChannel
		want    float64
	}{
		{"Physical", ChannelPhysical, 0.30},
		{"Magical", ChannelMagical, 1.0},
		{"Conviction", ChannelConviction, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DamageScale(tt.channel)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("DamageScale(%v) = %f, want %f", tt.channel, got, tt.want)
			}
		})
	}
}

// TestMitigationCurveFor verifies each channel returns its OWN configured
// curve, all four knobs.
//
// The twelve knobs ship at the same four numbers on every channel, so
// comparing against config alone cannot detect cross-wiring. The overrides
// below give every knob a distinct value so a mix-up actually fails. Pinned
// through SetConfigForTest, which restores on cleanup (the cap test this
// replaced wrote a permanent overlay that leaked into every later test).
func TestMitigationCurveFor(t *testing.T) {
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.PhysicalMitigationKnee, b.PhysicalMitigationCap = 0.41, 0.61
	b.PhysicalMitigationAim, b.PhysicalMitigationBend = 0.011, 0.31
	b.MagicalMitigationKnee, b.MagicalMitigationCap = 0.42, 0.62
	b.MagicalMitigationAim, b.MagicalMitigationBend = 0.012, 0.32
	b.ConvictionMitigationKnee, b.ConvictionMitigationCap = 0.43, 0.63
	b.ConvictionMitigationAim, b.ConvictionMitigationBend = 0.013, 0.33
	configs.SetConfigForTest(t, cfg)

	tests := []struct {
		name    string
		channel DamageChannel
		want    MitigationCurve
	}{
		{"Physical", ChannelPhysical, MitigationCurve{Knee: 0.41, Cap: 0.61, Aim: 0.011, Bend: 0.31}},
		{"Magical", ChannelMagical, MitigationCurve{Knee: 0.42, Cap: 0.62, Aim: 0.012, Bend: 0.32}},
		{"Conviction", ChannelConviction, MitigationCurve{Knee: 0.43, Cap: 0.63, Aim: 0.013, Bend: 0.33}},
		// An unknown channel falls through to the physical curve.
		{"UnknownChannel", DamageChannel(99), MitigationCurve{Knee: 0.41, Cap: 0.61, Aim: 0.011, Bend: 0.31}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MitigationCurveFor(tt.channel)
			if math.Abs(got.Knee-tt.want.Knee) > 1e-9 || math.Abs(got.Cap-tt.want.Cap) > 1e-9 ||
				math.Abs(got.Aim-tt.want.Aim) > 1e-9 || math.Abs(got.Bend-tt.want.Bend) > 1e-9 {
				t.Errorf("MitigationCurveFor(%v) = %+v, want %+v (its own configured curve)", tt.channel, got, tt.want)
			}
		})
	}
}

// With no overrides a test binary runs the Go defaults, which equal the
// shipped curve (config_mitigation_curve_test.go pins the shipped file).
func TestMitigationCurveFor_DefaultsAreTheShippedCurve(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance = configs.Balance{}
	cfg.Balance.Validate()
	configs.SetConfigForTest(t, cfg)

	for _, ch := range []DamageChannel{ChannelPhysical, ChannelMagical, ChannelConviction} {
		got := MitigationCurveFor(ch)
		if math.Abs(got.Knee-0.50) > 1e-9 || math.Abs(got.Cap-0.90) > 1e-9 ||
			math.Abs(got.Aim-0.05) > 1e-9 || math.Abs(got.Bend-0.40) > 1e-9 {
			t.Errorf("MitigationCurveFor(%v) = %+v, want the shipped 0.50 / 0.90 / 0.05 / 0.40", ch, got)
		}
	}
}

func TestCalcRawDamage_EdgeCases(t *testing.T) {
	// Zero stat → zero damage
	got := CalcRawDamage(0, 0, 1.0, ChannelPhysical)
	if got != 0 {
		t.Errorf("CalcRawDamage(0, 0, 1.0, Physical) = %f, want 0", got)
	}

	// Zero itemMult → fallback to 0.30 (unarmed level), NOT zero
	got = CalcRawDamage(100, 10, 0.0, ChannelMagical)
	if got <= 0 {
		t.Errorf("CalcRawDamage(100, 10, 0.0, Magical) = %f, want > 0 (fallback)", got)
	}
}

func TestApplyMitigation_EdgeCases(t *testing.T) {
	// Far past any build, mitigation stops at the 90% cap: 100 -> 10
	got := ApplyMitigation(100.0, 10.0, shippedCurveForTest)
	expected := 10.0 // 100 * (1 - 0.90)
	if math.Abs(got-expected) > 0.01 {
		t.Errorf("ApplyMitigation(100, 10.0, curve) = %f, want %f", got, expected)
	}

	// Zero raw → zero result
	got = ApplyMitigation(0, 0.5, shippedCurveForTest)
	if got != 0 {
		t.Errorf("ApplyMitigation(0, 0.5, curve) = %f, want 0", got)
	}
}
