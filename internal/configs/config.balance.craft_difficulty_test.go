package configs

import "testing"

// TestCraftDifficultyKnobDefaults pins the spec 5.1.1 anchors.
//
// CraftBaseDifficulty is 100 because 100 is the HUMAN STAT BASELINE. That is
// what makes the whole formula work without a special case: a baseline crafter
// holding exactly the recipe's minimum skill scores 100 + min*5 against a
// difficulty of 100 + min*5, which is 50% -- reproducing today's shipped
// CraftingBaseSuccessChance of 50 exactly.
func TestCraftDifficultyKnobDefaults(t *testing.T) {
	b := Balance{}
	b.Validate()

	if int(b.CraftBaseDifficulty) != 100 {
		t.Errorf("CraftBaseDifficulty = %v, want 100 (the human stat baseline, "+
			"which is what makes a baseline crafter exactly 50/50 at the "+
			"recipe minimum)", int(b.CraftBaseDifficulty))
	}
	if int(b.CraftSkillMinWeight) != 5 {
		t.Errorf("CraftSkillMinWeight = %v, want 5 (SkillWeight, as everywhere)",
			int(b.CraftSkillMinWeight))
	}
	if float64(b.CraftFloor) != 0.05 {
		t.Errorf("CraftFloor = %v, want 0.05 (reproduces the retired 5/95 clamp)",
			float64(b.CraftFloor))
	}
	if float64(b.SalvageFloor) != 0.15 {
		t.Errorf("SalvageFloor = %v, want 0.15 (reproduces the retired 15/85 clamp)",
			float64(b.SalvageFloor))
	}
}

// TestCraftFloorsAreNotZeroable pins spec 5.1.1.2: the floors are a MERCY BAND,
// and a 0 floor deletes it rather than disabling a nicety.
//
// 🔴 Uncapped salvage is the dangerous half. At salvage skill 50 the
// craft-then-salvage loop would retain about 99.9% of materials against 80.75%
// today -- roughly a 250x reduction in the crafting material sink, on the exact
// loop that farms crafting skill. An earlier spec draft retired these clamps
// "in favour of ContestFloor" and that was false: contest.AgainstDifficulty
// applies no floor at all, so deleting them would have replaced the band with
// nothing.
//
// The <=0 idiom is therefore correct here and a -1 sentinel would be WRONG: 0
// must not be a legal shipped value for either floor.
func TestCraftFloorsAreNotZeroable(t *testing.T) {
	b := Balance{}
	b.CraftFloor = 0
	b.SalvageFloor = 0
	b.Validate()

	if float64(b.CraftFloor) <= 0 {
		t.Error("an explicit CraftFloor of 0 must be corrected to the default, " +
			"not honoured -- a 0 floor deletes the mercy band")
	}
	if float64(b.SalvageFloor) <= 0 {
		t.Error("an explicit SalvageFloor of 0 must be corrected to the default, " +
			"not honoured -- uncapped salvage is a ~250x cut to the material sink")
	}
}
