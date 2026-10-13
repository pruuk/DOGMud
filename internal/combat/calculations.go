package combat

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// powerScoreTerms are PowerScore's components, kept apart so drift guard 2
// can hold the shared ones to the fight read's layer 1 (#301).
type powerScoreTerms struct {
	meleeOffense float64 // layer 1: every planned swing's mean against the neutral target
	spellOffense float64 // magical and conviction channel raw damage, half weight each
	mitigation   float64 // effective mitigation, averaged over the three channels
	avoidance    float64 // layer 1: mean of dodge, parry and block as the contest scores them
	naturalArmor float64
	durability   float64
	skills       float64
	mutations    float64
	kd           float64
}

func (p powerScoreTerms) total() float64 {
	return p.meleeOffense + p.spellOffense + p.mitigation*300 + p.avoidance*2 + p.naturalArmor +
		p.durability + p.skills + p.mutations + p.kd
}

// powerScoreParts computes PowerScore's terms. Melee offense and avoidance
// come from the fight read's shared helpers (fight_expect.go), so PowerScore,
// consider and combat cannot drift apart (#301).
func powerScoreParts(char characters.Character) powerScoreTerms {
	c := &char
	p := powerScoreTerms{}

	// PowerScore rates the build at full pools; the fight read (PredictFight)
	// reads live state. char is a by-value copy, so this leaves the caller's
	// character alone (ints only: the copy is shallow).
	c.Health = c.HealthMax.Value
	c.Stamina = c.StaminaMax.Value

	// ── OFFENSE ──────────────────────────────────────────────────────────
	p.meleeOffense = meleeOffenseRaw(c, sourceTypeOf(c))

	spellcastingRank := char.GetSkillLevel(skills.Spellcasting)
	spellMult := 1.0
	if char.Equipment.Weapon.ItemId > 0 {
		if sdm := char.Equipment.Weapon.GetSpec().SpellDamageMultiplier; sdm > 0 {
			spellMult = sdm * mutations.GearEffectivenessMultiplier(char.Mutations)
		}
	}
	magAtk := CalcRawDamage(char.Stats.Willpower.ValueAdj, spellcastingRank, spellMult, ChannelMagical)
	convAtk := CalcRawDamage(char.Stats.Charisma.ValueAdj, char.GetSkillLevel(skills.Rhetoric), 0.5, ChannelConviction)
	p.spellOffense = magAtk*0.5 + convAtk*0.5

	// ── DEFENSE ──────────────────────────────────────────────────────────
	// Effective, not raw (#465).
	p.mitigation = (EffectiveMitigationFor(c, ChannelPhysical) +
		EffectiveMitigationFor(c, ChannelMagical) +
		EffectiveMitigationFor(c, ChannelConviction)) / 3.0
	p.avoidance = meleeAvoidance(c)
	p.naturalArmor = float64(mutations.GetNaturalArmor(char.Mutations))

	// ── DURABILITY ───────────────────────────────────────────────────────
	p.durability = float64(char.HealthMax.Value) +
		float64(char.StaminaMax.Value)*0.5 +
		float64(char.ConvictionMax.Value)*0.5

	// ── SKILLS ───────────────────────────────────────────────────────────
	totalRanks := 0
	for _, rank := range char.GetAllSkillRanks() {
		totalRanks += rank
	}
	p.skills = math.Sqrt(float64(totalRanks)) * 25.0

	// ── MUTATIONS ────────────────────────────────────────────────────────
	totalMutLevels := 0
	for _, level := range char.Mutations {
		if level > 0 {
			totalMutLevels += level
		}
	}
	p.mutations = float64(totalMutLevels) * 20.0

	// ── KD RATIO ─────────────────────────────────────────────────────────
	deaths := char.KD.TotalDeaths
	if deaths < 1 {
		deaths = 1
	}
	p.kd = math.Min(float64(char.KD.TotalKills)/float64(deaths)*10.0, 50.0)

	return p
}

// PowerScore computes an absolute power score for a character: one number for
// the behavior trees, targeting and the leaderboard. Its melee offense and
// avoidance are the fight read's shared terms (powerScoreParts).
func PowerScore(char characters.Character) float64 {
	return powerScoreParts(char).total()
}

func ChanceToTame(s *users.UserRecord, t *mobs.Mob) int {

	var MOD_SKILL_MIN int = 1   // Minimum base tame ability
	var MOD_SKILL_MAX int = 100 // Maximum base tame ability

	var MOD_SIZE_SMALL int = 0    // Modifier for small creatures
	var MOD_SIZE_MEDIUM int = -10 // Modifier for medium creatures
	var MOD_SIZE_LARGE int = -25  // Modifier for large creatures

	var MOD_SKILLDIFF_MIN int = -4 // Lowest skill delta modifier
	var MOD_SKILLDIFF_MAX int = 4  // Highest skill delta modifier
	var SKILL_SCALE int = 6        // Scale factor to preserve impact range (4*6=24, similar to old ±25)

	var MOD_HEALTHPERCENT_MAX float64 = 50 // Highest possible bonus for target HP being reduced

	var FACTOR_IS_AGGRO float64 = .50 // Overall reduction of chance if target is aggro

	proficiencyModifier := s.Character.MobMastery.GetTame(int(t.MobId))

	if proficiencyModifier < MOD_SKILL_MIN {
		proficiencyModifier = MOD_SKILL_MIN
	} else if proficiencyModifier > MOD_SKILL_MAX {
		proficiencyModifier = MOD_SKILL_MAX
	}

	speciesInfo := species.GetSpecies(s.Character.SpeciesId)

	sizeModifier := 0
	switch speciesInfo.Size {
	case species.Large:
		sizeModifier = MOD_SIZE_LARGE
	case species.Small:
		sizeModifier = MOD_SIZE_SMALL
	case species.Medium:
	default:
		sizeModifier = MOD_SIZE_MEDIUM
	}

	// Taming is now handled via spellcasting; use spellcasting skill vs mob combat skill
	tamerSkill := s.Character.GetSkillLevel(skills.Spellcasting)
	mobSkill := t.Character.GetCombatSkillLevel()
	skillDiff := tamerSkill - mobSkill
	if skillDiff > MOD_SKILLDIFF_MAX {
		skillDiff = MOD_SKILLDIFF_MAX
	} else if skillDiff < MOD_SKILLDIFF_MIN {
		skillDiff = MOD_SKILLDIFF_MIN
	}
	scaledSkillDiff := skillDiff * SKILL_SCALE

	// Percentage-of-max read, so it measures the pool the tamer can actually
	// reach (U7 Task 11). Against the raw max a reserved tamer never reaches the
	// full-health end of this scale even at a full pool.
	healthPoolMax := s.Character.EffectivePoolMax(characters.PoolHealth)
	healthModifier := 0.0
	if healthPoolMax > 0 {
		healthModifier = MOD_HEALTHPERCENT_MAX - math.Ceil(float64(s.Character.Health)/float64(healthPoolMax)*MOD_HEALTHPERCENT_MAX)
	}

	var aggroModifier float64 = 1
	if t.Character.IsAggro(s.UserId, 0) {
		aggroModifier = FACTOR_IS_AGGRO
	}

	return int(math.Ceil((float64(proficiencyModifier) + float64(scaledSkillDiff) + healthModifier + float64(sizeModifier)) * aggroModifier))
}

// ChanceToSwitchTarget calculates the percentage chance (0-100) that a character
// can successfully switch targets mid-combat.
// Success is based on:
// - Combat skill (60% weight) - Higher skill = smoother transitions
// - Dexterity (40% weight) - Agility helps reposition quickly
//
// Base chance: 50%
// Skill bonus: +0.3% per combat skill level (max +30% at skill 100)
// Dexterity bonus: +0.2% per dexterity point (max +20% at dex 100)
//
// Returns: chance out of 100 (e.g., 75 = 75% chance)
func ChanceToSwitchTarget(c *characters.Character) int {
	const baseChance float64 = 50.0
	const skillWeight float64 = 0.3 // 0.3% per skill level
	const dexWeight float64 = 0.2   // 0.2% per dex point

	combatSkill := c.GetCombatSkillLevel()
	dexterity := c.Stats.Dexterity.ValueAdj

	skillBonus := float64(combatSkill) * skillWeight
	dexBonus := float64(dexterity) * dexWeight

	totalChance := baseChance + skillBonus + dexBonus

	// Cap at 95% (always a small chance of failure)
	if totalChance > 95.0 {
		totalChance = 95.0
	}

	// Floor at 25% (even unskilled fighters have some chance)
	if totalChance < 25.0 {
		totalChance = 25.0
	}

	return int(totalChance)
}
