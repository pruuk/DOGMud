package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// toxChar builds a character with a known alchemy rank and vitality.
// Stat setup is `.Base` then Recalculate(), which is what makes ValueAdj real.
func toxChar(alchemy, vitality int) *Character {
	c := &Character{Skills: map[string]int{string(skills.Alchemy): alchemy}}
	c.Stats.Vitality.Base = vitality
	c.Stats.Vitality.Recalculate()
	return c
}

// pinToxicityScales pins the SHIPPED scales so this test measures the tuning the
// game actually runs, not whatever the Go defaults drift to later.
// SetConfigForTest assigns without validating and self-registers the restore.
func pinToxicityScales(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.ToxicityBaseMax = 0
	c.Balance.ToxicityAlchemyScale = 2.5
	c.Balance.ToxicityVitalityScale = 3
	configs.SetConfigForTest(t, c)
}

// TestToxicityCalibrationTable pins the whole design target: which potion in a
// row is the one that first pushes you into a band that actually costs you
// something. A later edit to either scale that moves any of these numbers is a
// balance change and must be a deliberate one -- this test is the tripwire.
//
// Low tier costs 8 (healing salve, stamina tonic, conviction draught).
// Mid tier costs 11 (warrior's brew, preacher's tincture, windrunner draught).
// "Bites" means reaching 50%, the first band carrying a penalty.
func TestToxicityCalibrationTable(t *testing.T) {
	pinToxicityScales(t)

	// firstBiting returns the 1-based index of the first potion of cost `each`
	// that reaches the 50% penalty line, or 0 if the drink gate rejects one first.
	firstBiting := func(c *Character, each float64) int {
		max := c.GetToxicityMax()
		for i := 1; i <= 12; i++ {
			total := float64(i) * each
			if total > max { // drink.go rejects a potion that would exceed max
				return 0
			}
			if total/max >= 0.50 {
				return i
			}
		}
		return 0
	}

	cases := []struct {
		name             string
		alchemy, vit     int
		wantMax          float64
		wantLow, wantMid int
	}{
		{"noob", 0, 100, 33.33, 3, 2},
		{"veteran, no alchemy", 0, 150, 50.00, 4, 3},
		{"mid alchemist", 25, 125, 51.67, 4, 3},
		{"meirok, real prod save", 58, 150, 73.20, 5, 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := toxChar(tc.alchemy, tc.vit)

			if got := c.GetToxicityMax(); math.Abs(got-tc.wantMax) > 0.01 {
				t.Errorf("GetToxicityMax() = %.2f, want %.2f", got, tc.wantMax)
			}
			if got := firstBiting(c, 8); got != tc.wantLow {
				t.Errorf("low tier (8): potion %d bites, want %d", got, tc.wantLow)
			}
			if got := firstBiting(c, 11); got != tc.wantMid {
				t.Errorf("mid tier (11): potion %d bites, want %d", got, tc.wantMid)
			}
		})
	}
}

// TestToleranceIsEarnedNotInherited is the flavour claim as an assertion: a
// veteran who never brewed is LESS tolerant than a practised brewer. Toughness
// is not tolerance. That inversion is the entire point of the change, and it is
// the first thing a "simplification" back to a vitality-only formula would break.
func TestToleranceIsEarnedNotInherited(t *testing.T) {
	pinToxicityScales(t)

	toughVeteran := toxChar(0, 150) // brawny, never touched a still
	// The crossover against a vitality-150 veteran sits at alchemy ~42
	// (42/2.5 + 100/3 = 50.1 vs 50.0), so this fixture uses 50 to clear it with
	// headroom. Note 50 is the shipped SkillSoftCap: an accomplished brewer, NOT
	// a casual one. The inversion asserted here is real but NARROW -- it does not
	// hold for a genuinely middling alchemist against a high-vitality character,
	// so player-facing copy must not claim that it does.
	practisedBrewer := toxChar(50, 100) // ordinary body, serious alchemy practice

	if practisedBrewer.GetToxicityMax() <= toughVeteran.GetToxicityMax() {
		t.Errorf("brewer max %.1f must EXCEED tough-but-untrained veteran max %.1f",
			practisedBrewer.GetToxicityMax(), toughVeteran.GetToxicityMax())
	}
}

// TestZeroToleranceCharacterIsSafe guards the divide-by-zero path. With
// ToxicityBaseMax at 0, a character with no alchemy and no vitality has a max of
// 0 -- which every consumer must treat as "no toxicity system", never as a
// divide. All three readers already early-return on max <= 0; this pins that.
func TestZeroToleranceCharacterIsSafe(t *testing.T) {
	pinToxicityScales(t)

	c := toxChar(0, 0)
	if got := c.GetToxicityMax(); got != 0 {
		t.Fatalf("GetToxicityMax() = %v, want 0", got)
	}
	c.Toxicity = 50 // nonsense state, but must not panic or divide

	if got := c.ToxicityBand(); got != 0 {
		t.Errorf("ToxicityBand() = %d, want 0 at zero max", got)
	}
	if got := c.ToxicitySicknessDamage(); got != 0 {
		t.Errorf("ToxicitySicknessDamage() = %d, want 0 at zero max", got)
	}
	if r, p, d := c.GetToxicityPenalties(); r != 1.0 || p != 1.0 || d != 1.0 {
		t.Errorf("GetToxicityPenalties() = %v/%v/%v, want all 1.0 at zero max", r, p, d)
	}
}

// TestToxicityBandsAreFinerThanPenalties pins the deliberate asymmetry this
// change introduces. ToxicityBand and GetToxicityPenalties used to mirror each
// other exactly and the code said so. They no longer do: bands 1 and 2 are pure
// feedback and carry NO penalty, so a player can watch pressure build before it
// costs anything. Restoring the symmetry would silently delete both warning
// tiers, which is why this test exists.
func TestToxicityBandsAreFinerThanPenalties(t *testing.T) {
	pinToxicityScales(t)

	c := toxChar(0, 300) // max exactly 100, so ratios read as percentages
	if got := c.GetToxicityMax(); got != 100 {
		t.Fatalf("fixture max = %v, want 100", got)
	}

	cases := []struct {
		pct         float64
		wantBand    int
		wantName    string
		wantPenalty bool
	}{
		{0.00, 0, "clear", false},
		{0.14, 0, "clear", false},
		{0.15, 1, "sour", false},      // NEW -- visible, costs nothing
		{0.29, 1, "sour", false},      // NEW
		{0.30, 2, "unsettled", false}, // NEW -- visible, costs nothing
		{0.49, 2, "unsettled", false}, // NEW
		{0.50, 3, "queasy", true},     // unchanged penalty threshold
		{0.74, 3, "queasy", true},
		{0.75, 4, "sick", true}, // unchanged penalty threshold
		{0.89, 4, "sick", true},
		{0.90, 5, "critical", true}, // unchanged penalty threshold
		{1.00, 5, "critical", true},
	}

	for _, tc := range cases {
		c.Toxicity = 100 * tc.pct

		if got := c.ToxicityBand(); got != tc.wantBand {
			t.Errorf("at %.0f%%: ToxicityBand() = %d, want %d", tc.pct*100, got, tc.wantBand)
		}
		if got := c.ToxicityBandName(); got != tc.wantName {
			t.Errorf("at %.0f%%: ToxicityBandName() = %q, want %q", tc.pct*100, got, tc.wantName)
		}

		regen, per, dex := c.GetToxicityPenalties()
		penalised := regen != 1.0 || per != 1.0 || dex != 1.0
		if penalised != tc.wantPenalty {
			t.Errorf("at %.0f%%: penalised = %v, want %v (regen %.2f per %.2f dex %.2f)",
				tc.pct*100, penalised, tc.wantPenalty, regen, per, dex)
		}
	}
}

// TestBandNamesFitTheStatusColumn guards a layout constraint that is invisible
// in Go: status.template pads the band name to 13 visual characters, so a
// longer name would push the box border off and break the whole status screen.
func TestBandNamesFitTheStatusColumn(t *testing.T) {
	pinToxicityScales(t)

	c := toxChar(0, 300)
	for pct := 0.0; pct <= 1.0; pct += 0.05 {
		c.Toxicity = 100 * pct
		if n := c.ToxicityBandName(); len(n) > 13 {
			t.Errorf("band name %q is %d chars, exceeds the 13-char status column", n, len(n))
		}
	}
}
