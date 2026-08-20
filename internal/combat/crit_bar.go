package combat

import "github.com/GoMudEngine/GoMud/internal/configs"

// CritBarFor is THE attacker-side crit threshold for every channel, as a pure
// function of the CHANNEL's skill pair (owner decision, 2026-08-19):
//
//	bar = clamp(base − slope×(atkRank − defRank), floor, ceiling)
//
// atkRank is the attack's governing skill rank (AttackSide.SkillRank at the
// seam; the equipped combat skill for melee). defRank is the WINNING defence's
// governing skill rank — spellcasting behind quell, rhetoric behind defy, the
// weapon/unarmed skills behind the physical three. Out-skill your target and
// the bar falls toward the floor; get out-skilled and it rises to the CEILING,
// which is what lets a gold-scaled, skill-poor boss still buy crits against a
// veteran (uncapped, a 1000g boss crits a veteran essentially never — the
// pre-U6b melee behaviour; the shipped 3.0 puts it near half its saturated
// rate instead). Ceiling 0 means uncapped, and is legal.
//
// All three values are config (CritBarSkillSlope 0.05, CritBarFloor 1.5,
// CritBarCeiling 3.0) — they were balance literals inside internal/ before
// U6b, which standing rule 1 forbids.
//
// Melee's old Accuracy/Blink adjustments do not survive: no shipped content
// ever granted either flag and both were deleted as upstream stowaways.
//
// READ THIS BEFORE REASONING ABOUT CRIT RATES. What this function returns is
// only the BAR. Since chunk 5.11d the thing measured against the bar is the
// normalized opposed-roll MARGIN, not a self-relative z-score, so the
// opponent's stats, gear and position dominate the actual crit rate. See
// margin_crit.go. Note also the deliberate double count: skill already raises
// the margin through the attack score (SkillWeight) and lowers this bar as
// well; playtest 2026-08-14 says that feels good, so it stays.
func CritBarFor(atkRank, defRank int) float64 {
	b := configs.GetBalanceConfig()
	bar := ContestCritThreshold - float64(b.CritBarSkillSlope)*float64(atkRank-defRank)
	if bar < float64(b.CritBarFloor) {
		bar = float64(b.CritBarFloor)
	}
	if c := float64(b.CritBarCeiling); c > 0 && bar > c {
		bar = c
	}
	return bar
}

// DefenseCritBar is the defender-side threshold. Melee shipped this as a
// hardcoded 2.0 separate from its own dynamic attack bar; U6b keeps it a
// single constant for every channel, ON PURPOSE — a defensive crit unlocks
// the counter tier, and skill already reaches the defensive-crit rate through
// the margin (a skilled defender out-rolls more often and by more). Shifting
// this bar by the same skill pair would triple-count skill on the defence
// side. Documented here so nobody "unifies" it into CritBarFor without
// deciding that.
func DefenseCritBar() float64 { return ContestCritThreshold }
