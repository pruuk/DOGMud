package configs

// WeightedSkill is a skill rank's contribution to any additive score
// (attack, defence, swing count, stealth, search, crafting difficulty, the
// progression roll): rank x SkillWeight up to SkillWeightKneeRank, then
// SkillWeightAboveKnee per rank above it (owner 2026-10-31, the endgame
// power leak). A knee of 0 is today's straight line. Every reader of
// SkillWeight goes through here; internal/guards keeps it that way.
func WeightedSkill(rank float64) float64 {
	if rank <= 0 {
		return 0
	}
	return WeightedSkillFor(GetBalanceConfig(), rank)
}

// WeightedSkillFor is WeightedSkill against a balance snapshot the caller
// already holds, for functions that take a Balance as a parameter (their
// tests inject one) and must not read the global behind its back.
func WeightedSkillFor(b Balance, rank float64) float64 {
	if rank <= 0 {
		return 0
	}
	sw := float64(b.SkillWeight)
	knee := float64(b.SkillWeightKneeRank)
	if knee <= 0 || rank <= knee {
		return rank * sw
	}
	return knee*sw + (rank-knee)*float64(b.SkillWeightAboveKnee)
}
