package characters

import "testing"

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{1, 2},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{8, 8},
		{9, 16},
	}

	for _, tt := range tests {
		got := NextPowerOfTwo(tt.input)
		if got != tt.expected {
			t.Errorf("NextPowerOfTwo(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestCalcFoldsPerRound(t *testing.T) {
	tests := []struct {
		perception      int
		spellcastingLvl int
		expected        int
	}{
		// max(1, round((perception + weightedSkill*25) / 100)), SkillWeight=2.0
		{0, 0, 1},   // round(0/100)=0 → clamped to 1
		{50, 2, 2},  // round((50+4*25)/100) = round(1.5) = 2
		{100, 4, 3}, // round((100+8*25)/100) = round(3.0) = 3
		{75, 3, 2},  // round((75+6*25)/100) = round(2.25) = 2
		{25, 1, 1},  // round((25+2*25)/100) = round(0.75) = 1
	}

	for _, tt := range tests {
		got := CalcFoldsPerRound(tt.perception, tt.spellcastingLvl)
		if got != tt.expected {
			t.Errorf("CalcFoldsPerRound(%d, %d) = %d, want %d",
				tt.perception, tt.spellcastingLvl, got, tt.expected)
		}
	}
}

// The spell-attack score test was deleted with its helper (U6b Task 4). It
// pinned the x15-per-rank spell hit-gate score (weighted skill x the deleted
// skill-factor knob) — pinning the defect the collapse removes. The caster's
// contest side is now combat.AttackSide (stat + rank x SkillWeight), pinned
// by TestAttackSide_Score in internal/combat.
