package configs

// validateGathering sets defaults for the wilderness-trades gathering knobs:
// the gathering roll, tool tier multipliers and material grade sell values.
//
// Most knobs use the <=0 idiom: a zero there is meaningless (a zero tool
// multiplier, a zero durability) and is replaced by the default.
//
// The knobs below have a meaningful zero and are defaulted only when
// NEGATIVE, as StaminaPerStrength is: 0 turns the feature off. Like every
// legal-zero knob, leaving one out of config.yaml reads as 0.
//
//	GatherCarcassEase, GatherSizeDifficultyMedium/Large, GatherTargetedDifficulty,
//	TimberEase, MiningEase                 0 = no ease / no extra difficulty
//	GatherRareBaseChance, MiningGemChance  0 = no rare parts / no gems
//	ShopWalkInDevaluePerUnit               0 = walk-in goods never slide
//	GearCritWearChance, GearArmorCritWearChance, BowShotWearChance
//	                                       0 = that gear never wears
//	RepairCostRatio                        0 = merchants mend for free
//	WornSellPenalty                        0 = worn gear sells at full price
func (b *Balance) validateGathering() {
	if b.GatherSkillWeight <= 0 {
		b.GatherSkillWeight = 1.5
	}
	if b.GatherBaseDifficulty <= 0 {
		b.GatherBaseDifficulty = 100
	}
	if b.GatherGradeStepSigma <= 0 {
		b.GatherGradeStepSigma = 1.0
	}

	if b.ToolMultCrude <= 0 {
		b.ToolMultCrude = 0.8
	}
	if b.ToolMultIron <= 0 {
		b.ToolMultIron = 1.0
	}
	if b.ToolMultSteel <= 0 {
		b.ToolMultSteel = 1.15
	}
	if b.ToolMultMasterwork <= 0 {
		b.ToolMultMasterwork = 1.3
	}

	if b.QualityValueCrude <= 0 {
		b.QualityValueCrude = 0.4
	}
	if b.QualityValueStandard <= 0 {
		b.QualityValueStandard = 1.0
	}
	if b.QualityValueFine <= 0 {
		b.QualityValueFine = 1.6
	}
	if b.QualityValueSuperb <= 0 {
		b.QualityValueSuperb = 2.5
	}
	if b.QualityValuePristine <= 0 {
		b.QualityValuePristine = 4.0
	}

	if b.GatherStatPoolDifficulty <= 0 {
		b.GatherStatPoolDifficulty = 0.2
	}
	if b.GatherCarcassEase < 0 {
		b.GatherCarcassEase = 20
	}
	if b.GatherSizeDifficultyMedium < 0 {
		b.GatherSizeDifficultyMedium = 5
	}
	if b.GatherSizeDifficultyLarge < 0 {
		b.GatherSizeDifficultyLarge = 15
	}
	if b.GatherTargetedDifficulty < 0 {
		b.GatherTargetedDifficulty = 15
	}
	if b.GatherJobRoundsSmall <= 0 {
		b.GatherJobRoundsSmall = 2
	}
	if b.GatherJobRoundsMedium <= 0 {
		b.GatherJobRoundsMedium = 4
	}
	if b.GatherJobRoundsLarge <= 0 {
		b.GatherJobRoundsLarge = 6
	}
	if b.GatherRareBaseChance < 0 {
		b.GatherRareBaseChance = 0.15
	}
	if b.GatherStatPerBonusUnit <= 0 {
		b.GatherStatPerBonusUnit = 50
	}
	if b.CorpseStaleGradeAt <= 0 {
		b.CorpseStaleGradeAt = 0.5
	}
	if b.CorpseMeatLostAt <= 0 {
		b.CorpseMeatLostAt = 0.75
	}
	if b.ShopWalkInDevaluePerUnit < 0 {
		b.ShopWalkInDevaluePerUnit = 0.02
	}

	if b.ShopScrapDecayRounds <= 0 {
		b.ShopScrapDecayRounds = 900
	}

	if b.TimberEase < 0 {
		b.TimberEase = 10
	}
	if b.TimberTierDifficulty <= 0 {
		b.TimberTierDifficulty = 15
	}
	if b.TimberStandMin <= 0 {
		b.TimberStandMin = 6
	}
	if b.TimberStandMax < b.TimberStandMin {
		b.TimberStandMax = 10
		if b.TimberStandMax < b.TimberStandMin {
			b.TimberStandMax = b.TimberStandMin
		}
	}
	if b.TimberRegrowRounds <= 0 {
		b.TimberRegrowRounds = 900
	}
	if b.TimberChopRoundsBase <= 0 {
		b.TimberChopRoundsBase = 4
	}
	if b.TimberMaxLogs <= 0 {
		b.TimberMaxLogs = 4
	}

	if b.ToolDurabilityCrude <= 0 {
		b.ToolDurabilityCrude = 30
	}
	if b.ToolDurabilityIron <= 0 {
		b.ToolDurabilityIron = 80
	}
	if b.ToolDurabilitySteel <= 0 {
		b.ToolDurabilitySteel = 160
	}
	if b.ToolDurabilityMasterwork <= 0 {
		b.ToolDurabilityMasterwork = 320
	}

	if b.RareToolMultCrude <= 0 {
		b.RareToolMultCrude = 0.5
	}
	if b.RareToolMultIron <= 0 {
		b.RareToolMultIron = 1.0
	}
	if b.RareToolMultSteel <= 0 {
		b.RareToolMultSteel = 1.5
	}
	if b.RareToolMultMasterwork <= 0 {
		b.RareToolMultMasterwork = 2.0
	}

	if b.GradeDamageCrude <= 0 {
		b.GradeDamageCrude = 0.85
	}
	if b.GradeDamageFine <= 0 {
		b.GradeDamageFine = 1.08
	}
	if b.GradeDamageSuperb <= 0 {
		b.GradeDamageSuperb = 1.16
	}
	if b.GradeDamagePristine <= 0 {
		b.GradeDamagePristine = 1.25
	}
	if b.GradeSpeedCrude <= 0 {
		b.GradeSpeedCrude = 0.85
	}
	if b.GradeSpeedFine <= 0 {
		b.GradeSpeedFine = 1.05
	}
	if b.GradeSpeedSuperb <= 0 {
		b.GradeSpeedSuperb = 1.1
	}
	if b.GradeSpeedPristine <= 0 {
		b.GradeSpeedPristine = 1.15
	}
	if b.GradeWeightCrude <= 0 {
		b.GradeWeightCrude = 1.05
	}
	if b.GradeWeightFine <= 0 {
		b.GradeWeightFine = 0.97
	}
	if b.GradeWeightSuperb <= 0 {
		b.GradeWeightSuperb = 0.94
	}
	if b.GradeWeightPristine <= 0 {
		b.GradeWeightPristine = 0.9
	}
	if b.GradeArmorCrude <= 0 {
		b.GradeArmorCrude = 0.85
	}
	if b.GradeArmorFine <= 0 {
		b.GradeArmorFine = 1.08
	}
	if b.GradeArmorSuperb <= 0 {
		b.GradeArmorSuperb = 1.16
	}
	if b.GradeArmorPristine <= 0 {
		b.GradeArmorPristine = 1.25
	}

	if b.MiningEase < 0 {
		b.MiningEase = 5
	}
	if b.MiningTierDifficulty <= 0 {
		b.MiningTierDifficulty = 15
	}
	if b.MiningVeinMin <= 0 {
		b.MiningVeinMin = 4
	}
	if b.MiningVeinMax < b.MiningVeinMin {
		b.MiningVeinMax = 8
		if b.MiningVeinMax < b.MiningVeinMin {
			b.MiningVeinMax = b.MiningVeinMin
		}
	}
	if b.MiningRegrowRounds <= 0 {
		b.MiningRegrowRounds = 1800
	}
	if b.MiningRoundsBase <= 0 {
		b.MiningRoundsBase = 4
	}
	if b.MiningMaxOre <= 0 {
		b.MiningMaxOre = 3
	}
	if b.MiningGemChance < 0 {
		b.MiningGemChance = 0.04
	}

	if b.GearDurabilityWeapon <= 0 {
		b.GearDurabilityWeapon = 60
	}
	if b.GearDurabilityArmor <= 0 {
		b.GearDurabilityArmor = 30
	}
	if b.GearCritWearChance < 0 {
		b.GearCritWearChance = 0.5
	}
	if b.GearArmorCritWearChance < 0 {
		b.GearArmorCritWearChance = 0.5
	}
	if b.BowShotWearChance < 0 {
		b.BowShotWearChance = 0.04
	}
	if b.GearWornMult <= 0 {
		b.GearWornMult = 0.95
	}
	if b.GearBadlyWornMult <= 0 {
		b.GearBadlyWornMult = 0.85
	}
	if b.GearBrokenMult <= 0 {
		b.GearBrokenMult = 0.25
	}
	if b.RepairCostRatio < 0 {
		b.RepairCostRatio = 0.5
	}
	if b.WornSellPenalty < 0 {
		b.WornSellPenalty = 0.6
	}
}
