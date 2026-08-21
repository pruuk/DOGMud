package configs

// validateSpells sets defaults for spell costs, spellcasting parameters,
// and enchantment fields.
func (b *Balance) validateSpells() {
	// ── SPELL COSTS ──────────────────────────────────────────────────────────
	if b.SpellConvictionCostMultiplier <= 0 {
		b.SpellConvictionCostMultiplier = 1.0
	}
	if b.SpellHealthCostMultiplier <= 0 {
		b.SpellHealthCostMultiplier = 1.0
	}
	if b.SelfCastProgressionMultiplier <= 0 {
		b.SelfCastProgressionMultiplier = 0.5
	}

	// ── SPELLCASTING ─────────────────────────────────────────────────────────
	if b.SpellConcentrationBase <= 0 {
		b.SpellConcentrationBase = 50
	}
	if b.SpellInitiationWillpowerDivisor < 1 {
		b.SpellInitiationWillpowerDivisor = 4
	}
	if b.SpellFoldsSkillFactor < 1 {
		b.SpellFoldsSkillFactor = 25
	}
	if b.SpellDamageScale <= 0 {
		b.SpellDamageScale = 1.0
	}
	if b.SpellDifficultyProgressionScale <= 0 {
		b.SpellDifficultyProgressionScale = 0.01
	}
	if b.SpellDiscoveryBaseChance <= 0 {
		b.SpellDiscoveryBaseChance = 5.0
	}
	if b.SpellDiscoveryDecayRate <= 0 {
		b.SpellDiscoveryDecayRate = 0.1
	}
	if b.SpellProficiencyCastsPerPoint < 1 {
		b.SpellProficiencyCastsPerPoint = 50
	}
	if b.ConcentrationFloor <= 0 || b.ConcentrationFloor > 0.5 {
		b.ConcentrationFloor = 0.02
	}
	if b.ConcentrationDamageThresholdPct < 1 {
		b.ConcentrationDamageThresholdPct = 10
	}

	// ── POOL RESERVATION ─────────────────────────────────────────────────────
	if b.PoolReservationCapPct <= 0 || b.PoolReservationCapPct > 1 {
		// 0.66 (U7b). Both guards matter: <= 0 is "absent from config.yaml", and
		// > 1 would make the cap unreachable and silently disable the ceiling,
		// which is the one failure mode nobody would notice until U8 shipped.
		b.PoolReservationCapPct = 0.66
	}

	// ── ENCHANTMENTS ─────────────────────────────────────────────────────────
	if b.EnchantTierUpBaseChance <= 0 {
		b.EnchantTierUpBaseChance = 0.02
	}
	if b.EnchantTierUsesBase < 1 {
		b.EnchantTierUsesBase = 25
	}
	if b.EnchantTierUsesScale <= 0 {
		b.EnchantTierUsesScale = 2.5
	}
	if b.EnchantRemovalPenaltyRounds < 1 {
		b.EnchantRemovalPenaltyRounds = 50
	}
	if b.EnchantMaxTier < 1 {
		b.EnchantMaxTier = 4
	}
}
