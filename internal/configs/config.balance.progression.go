package configs

// validateProgression sets defaults for skill/stat progression curves,
// multipliers, regen progression, and mutation progression fields.
func (b *Balance) validateProgression() {
	// ── PROGRESSION ───────────────────────────────────────────────────────────
	if b.SkillSoftCap < 1 {
		b.SkillSoftCap = 50
	}
	if b.StatProgressionSoftCap < 1 {
		b.StatProgressionSoftCap = 50
	}
	if b.BaseProgressionChance <= 0 || b.BaseProgressionChance > 1.0 {
		b.BaseProgressionChance = 0.30
	}
	if b.ProgressionDecayBelowCap <= 0 {
		b.ProgressionDecayBelowCap = 3.0
	}
	if b.ProgressionDecayAboveCap <= 0 {
		b.ProgressionDecayAboveCap = 2.0
	}
	// `< 0`, NOT `<= 0`. Both knobs are documented off-switches and 0 is a
	// legal shipped value; the `<= 0` idiom used by the neighbouring knobs
	// would silently restore the default and make disabling them impossible.
	if b.CritProgressionBonus < 0 {
		b.CritProgressionBonus = 2.0
	}
	if b.ObservedCritProgressionBonus < 0 {
		b.ObservedCritProgressionBonus = 0.5
	}
	// `<= 0`, not `< 0`: this is a safety floor, not an off-switch. A config
	// that omits the key must get the floor, not lose it — the `< 0` idiom two
	// lines above is why ObservedCritProgressionBonus sits at 0 in production.
	if b.ProgressionChanceFloor <= 0 {
		b.ProgressionChanceFloor = 1e-5
	}
	if b.UsesPerRank < 1 {
		b.UsesPerRank = 25
	}
	if b.SkillWeight <= 0 {
		b.SkillWeight = 2.0
	}

	// ── COSTS: INVERSE-SKILL MULTIPLIER (U7) ────────────────────────────────
	if b.CostSkillMultAtZero <= 0 {
		b.CostSkillMultAtZero = 1.10
	}
	if b.CostSkillMultAtMid <= 0 {
		b.CostSkillMultAtMid = 1.00
	}
	if b.CostSkillMultAtCap <= 0 {
		b.CostSkillMultAtCap = 0.40
	}
	if b.CostSkillMidRank < 1 {
		b.CostSkillMidRank = 25
	}
	if b.CostSkillCapRank < 1 {
		b.CostSkillCapRank = 100
	}
	// Cross-field invariant: the cap rank must sit strictly above the mid
	// rank, or costs.SkillCostMultiplier's two segments invert (the "above
	// mid" segment would run backwards). Enforced here, once, at load, where
	// a bad config.yaml edit is visible; costs.SkillCostMultiplier also
	// carries a belt-and-braces guard against this same case in case a
	// caller ever constructs a Balance by hand and skips validation.
	if b.CostSkillCapRank <= b.CostSkillMidRank {
		b.CostSkillCapRank = b.CostSkillMidRank + 1
	}

	// ── COSTS: ENCUMBRANCE MULTIPLIER (U7) ──────────────────────────────────
	if b.CostEncumbranceKnee <= 0 || b.CostEncumbranceKnee >= 1.0 {
		b.CostEncumbranceKnee = 0.75
	}
	// Below 1.0 is not "a gentle curve", it is an INVERSION: the first segment
	// runs from 1.0 at empty down to the knee multiplier, so loading up would
	// make every physical action CHEAPER. Corrected to the default rather than
	// clamped to 1.0, because a knee of exactly 1.0 flattens the whole first
	// segment and is almost certainly not what the author meant either.
	if b.CostEncumbranceKneeMult < 1.0 {
		b.CostEncumbranceKneeMult = 1.5
	}
	if b.CostEncumbranceMax <= 0 {
		b.CostEncumbranceMax = 5.0
	}
	// Cross-field invariant: the maximum must sit strictly above the knee
	// multiplier, or costs.EncumbranceMultiplier's second segment runs
	// BACKWARDS -- a character past the knee would get cheaper as they loaded
	// further toward capacity. Enforced here, once, at load, where a bad
	// config.yaml edit is visible, and paired with the knee guard above so both
	// halves of the curve's monotonicity are settled before any caller reads it.
	if b.CostEncumbranceMax <= b.CostEncumbranceKneeMult {
		b.CostEncumbranceMax = b.CostEncumbranceKneeMult * 2
	}

	// ── COSTS: COMPOSED-TOTAL CEILING (U7) ──────────────────────────────────
	// The ceiling costs.Calc puts on the PRODUCT of its multipliers. 0 is not a
	// usable value here (it would price every action free), so it falls back to
	// the default rather than being honoured.
	if b.CostTotalMultiplierMax <= 0 {
		b.CostTotalMultiplierMax = 6.0
	}

	// ── PROGRESSION MULTIPLIERS ──────────────────────────────────────────────
	if b.StatProgressionMultipliers == nil {
		b.StatProgressionMultipliers = map[string]float64{}
	}
	for k, v := range b.StatProgressionMultipliers {
		if v <= 0 {
			delete(b.StatProgressionMultipliers, k)
		}
	}
	if b.SkillProgressionMultipliers == nil {
		b.SkillProgressionMultipliers = map[string]float64{}
	}
	for k, v := range b.SkillProgressionMultipliers {
		if v <= 0 {
			delete(b.SkillProgressionMultipliers, k)
		}
	}

	// ── REGEN-BASED STAT PROGRESSION ─────────────────────────────────────────
	if b.RegenProgressionBase <= 0 {
		b.RegenProgressionBase = 0.005
	}
	if b.RegenProgressionBase > 1.0 {
		b.RegenProgressionBase = 1.0
	}
	if b.RegenProgressionCurve <= 0 {
		b.RegenProgressionCurve = 3.0
	}

	// ── MUTATIONS ─────────────────────────────────────────────────────────────
	if b.MutationBaseProgress <= 0 {
		b.MutationBaseProgress = 50.0
	}
	if b.MutationProgressScale <= 0 {
		b.MutationProgressScale = 1.5
	}
	if b.MutationMaxCount < 1 {
		b.MutationMaxCount = 5
	}
	if b.MutationMaxLevel < 1 {
		b.MutationMaxLevel = 3
	}
	if b.MutationDeepenChance <= 0 || b.MutationDeepenChance > 1.0 {
		b.MutationDeepenChance = 0.70
	}
	if b.MutationProgressGainPerRound <= 0 {
		b.MutationProgressGainPerRound = 1.0
	}
	if b.MutationAffinityPerSkillUse <= 0 {
		b.MutationAffinityPerSkillUse = 0.5
	}
	if b.MutationAffinityPerCombatEvent <= 0 {
		b.MutationAffinityPerCombatEvent = 0.5
	}
	if b.StatProgressionRate <= 0 {
		b.StatProgressionRate = 1.0
	}
	if b.MutationAffinityPerRarity <= 0 {
		b.MutationAffinityPerRarity = 2.0
	}
	if b.MutationAffinityDecay <= 0 || b.MutationAffinityDecay > 1.0 {
		b.MutationAffinityDecay = 0.98
	}
	if b.MutationBodyConvictionDecayMax <= 0 || b.MutationBodyConvictionDecayMax > 1.0 {
		b.MutationBodyConvictionDecayMax = 0.9
	}
	if b.MutationBeliefGearDecayMax <= 0 || b.MutationBeliefGearDecayMax > 1.0 {
		b.MutationBeliefGearDecayMax = 0.9
	}
	if b.MutationPoleDecayRef <= 0 {
		b.MutationPoleDecayRef = 60.0
	}
	if b.MutationLevel2Multiplier <= 0 {
		b.MutationLevel2Multiplier = 1.5
	}
	if b.MutationLevel3Multiplier <= 0 {
		b.MutationLevel3Multiplier = 2.0
	}
	if b.MutationLevel4Multiplier <= 0 {
		b.MutationLevel4Multiplier = 2.5
	}
}
