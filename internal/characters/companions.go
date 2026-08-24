package characters

import (
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// ── Pet → Companion migration (DEFERRED) ─────────────────────────────────────
//
// The legacy Pet system (Character.Pet of type pets.Pet) is fundamentally
// different from the Companions system and cannot be cleanly migrated today:
//
//   - Pets have no MobId — they are identified by a Type string and loaded
//     from pet YAML definitions, not mob templates. CompanionInfo requires
//     a MobId to respawn the companion on login.
//
//   - Pets provide stat mods and buffs directly to the player character
//     (GetStatMod, GetBuffs). The Companions system does not have this concept.
//
//   - Pets have their own inventory (Pet.Items) which has no counterpart in
//     CompanionInfo.
//
// Migration plan (future work):
//  1. Create pet-backed mob templates that mirror each pet type's stats/buffs.
//  2. Add a PetMobId field to each pet YAML definition.
//  3. In Validate(), create a CompanionInfo{SourceType: CompanionPet, MobId: …}
//     for each existing Pet and clear Character.Pet.
//  4. Remove the pets package once all players have migrated.
//
// Until then, Character.Pet and Character.Companions coexist. CompanionPet
// source type is reserved for when this migration is implemented.
// ─────────────────────────────────────────────────────────────────────────────

// CompanionSourceType describes how a companion came to follow the player.
type CompanionSourceType string

const (
	CompanionSummoned CompanionSourceType = "summoned"
	CompanionConjured CompanionSourceType = "conjured"
	CompanionCharmed  CompanionSourceType = "charmed"
	CompanionRaised   CompanionSourceType = "raised"
	CompanionPet      CompanionSourceType = "pet"
)

// CompanionInfo holds the persistent state of a single companion.
// InstanceId is runtime-only and is not saved to disk.
type CompanionInfo struct {
	MobId      int                 `yaml:"mobid"`
	InstanceId int                 `yaml:"-"` // runtime only
	SourceType CompanionSourceType `yaml:"source_type"`
	Name       string              `yaml:"name"`
	BaseName   string              `yaml:"base_name,omitempty"` // mob template name, e.g. "Spirit Wolf"
	Nickname   string              `yaml:"nickname,omitempty"`  // player-given name, e.g. "Fred"
	AutoAssist bool                `yaml:"auto_assist"`
	// SchemaVersion is absent (0) on any companion saved before U10b-0 Phase C.
	// StatTraining in those files fused the template's authored stats, the
	// spawn pool and earned gains into one number; the spawn path separates
	// them on restore. A companion at the current version is gains-only.
	SchemaVersion    int            `yaml:"schema_version,omitempty"`
	StatTraining     map[string]int `yaml:"stat_training,omitempty"`
	Skills           map[string]int `yaml:"skills,omitempty"`
	SkillUseCount    map[string]int `yaml:"skill_use_count,omitempty"`
	Mutations        map[string]int `yaml:"mutations,omitempty"`
	SpellBook        map[string]int `yaml:"spellbook,omitempty"`
	MutationProgress float64        `yaml:"mutation_progress,omitempty"`
	// Gear persistence — snapshotted at logout, restored on respawn.
	Items     []items.Item `yaml:"items,omitempty"`     // Carried (backpack)
	Equipment Worn         `yaml:"equipment,omitempty"` // Equipped/worn slots
	// Conviction reserved to keep this companion fielded, snapshotted at summon
	// time so it doesn't drift when the summoner's skill/mutation changes mid-life.
	ConvictionReserve int `yaml:"conviction_reserve,omitempty"`
}

// GetCompanion finds a companion by name (case-insensitive partial match).
// Supports N.name / name#N disambiguation (via util.GetMatchNumber).
// Returns nil if no match is found.
func (c *Character) GetCompanion(name string) *CompanionInfo {
	search, matchNum := util.GetMatchNumber(name)
	lower := strings.ToLower(search)
	count := 0
	for i := range c.Companions {
		if strings.Contains(strings.ToLower(c.Companions[i].Name), lower) {
			count++
			if count == matchNum {
				return &c.Companions[i]
			}
		}
	}
	return nil
}

// GetCompanionByInstanceId finds a companion by its runtime mob instance ID.
// Returns nil if not found.
func (c *Character) GetCompanionByInstanceId(instanceId int) *CompanionInfo {
	for i := range c.Companions {
		if c.Companions[i].InstanceId == instanceId {
			return &c.Companions[i]
		}
	}
	return nil
}

// AddCompanion adds a companion to the character's companion list.
// Returns false if the character is already at max companion capacity.
func (c *Character) AddCompanion(info CompanionInfo) bool {
	if len(c.Companions) >= c.GetMaxCompanions() {
		return false
	}
	c.Companions = append(c.Companions, info)
	return true
}

// RemoveCompanion removes a companion by runtime instance ID.
// Returns a pointer to the removed CompanionInfo, or nil if not found.
func (c *Character) RemoveCompanion(instanceId int) *CompanionInfo {
	for i := range c.Companions {
		if c.Companions[i].InstanceId == instanceId {
			removed := c.Companions[i]
			c.Companions = append(c.Companions[:i], c.Companions[i+1:]...)
			return &removed
		}
	}
	return nil
}

// GetMaxCompanions returns the SOFT count backstop on simultaneous companions.
// It is a safety limit only — the real constraint is the Conviction reservation
// ceiling (see WouldBreachReservationCap). The Manifester apex
// ("companion-cap-raise" flag) raises the backstop.
func (c *Character) GetMaxCompanions() int {
	cfg := configs.GetBalanceConfig()
	cap := int(cfg.CompanionSoftCap)
	if cap < 1 {
		cap = 5
	}
	if mutations.HasMutationFlag(c.Mutations, "companion-cap-raise") {
		if apex := int(cfg.CompanionSoftCapApex); apex > cap {
			cap = apex
		}
	}
	return cap
}

// CalcCompanionPool computes the stat pool for a player-summoned companion.
//
//	B    = charisma + manifestationSkill * ManifestationPoolCoefficient
//	pool = round(B * petMultiplier)                      (conjured, no corpse)
//	pool = round(((B + corpsePool) / 2) * petMultiplier) (raised, from a corpse)
//
// The multiplier is applied AFTER the corpse average, and that ordering is the
// whole point. Under the old shape the pet's base pool multiplied the caster's
// power and the corpse was averaged in afterwards, so the corpse's share grew
// until it swamped the pet choice: at a 1000-pool corpse a skeleton fielded 587
// and a golem 675, meaning five times the price bought 15% more pet. Applying
// the multiplier last keeps every tier proportionally separated at every corpse
// size, which is pinned by
// TestCalcCompanionPool_TiersStaySeparatedAtEveryCorpseSize. Do not "simplify"
// this by folding the multiplier into B.
//
// Known consequence, accepted: mid-level summoners lose roughly 20%, because
// manifestation x 5 is flat where the old term multiplied a base pool.
// High-skill summoners gain slightly. If newer summoners feel too weak in
// playtest, the lever is ManifestationPoolCoefficient or a flat constant added
// to B, NOT the per-pet multipliers -- moving those would flatten the tiers this
// function exists to separate.
//
// This is NOT CalcSpawnPoolFromBase, which is the behaviour-tree add scaler and
// keeps the old curve for authored boss encounters.
func CalcCompanionPool(charisma int, manifestationSkill int, petMultiplier float64, corpsePool int) int {
	base := float64(charisma + manifestationSkill*ManifestationPoolCoefficient)
	if corpsePool > 0 {
		base = (base + float64(corpsePool)) / 2.0
	}
	pool := int(math.Round(base * petMultiplier))
	if pool < 1 {
		// Never field a pool-zero companion: NewMobByIdFresh divides by the pool
		// when distributing stats, and every downstream ratio reads a zero pool
		// as "no penalty at all".
		return 1
	}
	return pool
}

// ManifestationPoolCoefficient is how much one rank of manifestation adds to a
// companion's power base. Deliberately a constant and not a config knob: it is
// the shape of the formula rather than a tuning dial, and the spec names it as
// the FIRST lever to reach for if the playtest says newer summoners are weak, at
// which point it earns a knob.
const ManifestationPoolCoefficient = 5

// CalcSpawnPoolFromBase scales an AUTHORED base pool by the spawner's Charisma
// and manifestation skill.
//
//	scale  = 1.0 + charisma/chaFactor + manifestationSkill*skillFactor
//	result = round(baseStatPool * scale)
//
// This is the BEHAVIOUR-TREE add scaler and its only caller is
// behaviortree.actSummonCompanion. It is NOT the player companion formula --
// that is CalcCompanionPool, which U7b reshaped.
//
// It deliberately keeps the old curve. Its callers are authored boss encounters
// whose base_pool values were tuned against exactly this shape: the Core
// Guardian and Warden Prime summon repair frames at base_pool 50, Old Edrin at
// 60, and the Sentinel at 300. Putting them on the companion formula would nerf
// the Sentinel's adds roughly fivefold and buff the Core Guardian's by about a
// fifth, neither of which U7b intends.
//
// Config knobs: ManifestStatScaleChaFactor (default 150, NOT 200 -- the pre-U7b
// doc comment said 200, which was wrong; the config defaulter already floors the
// knob at 150, so the fallback below is unreachable in practice and matches it
// only so the two can never disagree), ManifestStatScaleSkillFactor
// (default 0.02).
func CalcSpawnPoolFromBase(baseStatPool int, charisma int, manifestationSkill int) int {
	cfg := configs.GetBalanceConfig()
	chaFactor := float64(cfg.ManifestStatScaleChaFactor)
	skillFactor := float64(cfg.ManifestStatScaleSkillFactor)
	if chaFactor <= 0 {
		chaFactor = 150
	}
	scale := 1.0 + float64(charisma)/chaFactor + float64(manifestationSkill)*skillFactor
	return int(math.Round(float64(baseStatPool) * scale))
}

// CompanionReserveBase returns the PRE-reduction Conviction a companion of the
// given pet multiplier reserves: CompanionReserveDefault scaled by the
// multiplier (D9).
//
// This replaced two flat tiers (280 and 352) shared across both families. The
// ongoing budget now tracks pet POWER, which is what makes the reservation
// ceiling a real choice rather than a disguised companion count. Cast cost is a
// one-time toll on a companion that persists across logout and reboot with full
// state, so it is the wrong place to carry differentiation; reservation is.
//
// A multiplier of 0 (charm, the brood floor, the homunculus -- none of which is
// an authored summon with a pet tier) means "unscaled", so those paths keep
// their own bases untouched.
func CompanionReserveBase(petMultiplier float64) int {
	base := int(configs.GetBalanceConfig().CompanionReserveDefault)
	if petMultiplier <= 0 {
		return base
	}
	if r := int(math.Round(float64(base) * petMultiplier)); r > 0 {
		return r
	}
	return 1
}

// CalcCompanionReserve returns the Conviction a companion of the given base cost
// reserves for THIS summoner, after the manifestation-skill and Manifester-
// mutation reductions and the U7 inverse-skill rider.
//
//	reservation = round(base * (1 - reduction) * costs.SkillCostMultiplier(manif))
//
// The rider COMPOSES onto the existing reduction; it does not replace it
// (D10 §4.1). Replacing would be strictly worse at every rank: the U7 curve
// bottoms at 0.40 while the existing reduction already reaches 0.45 at
// manifestation 55 and 0.21 with the Manifester mutation, so a replacement would
// make companions dearer for everyone, the opposite of intent.
//
// Known consequence, accepted: composed, the curve double-counts manifestation
// below rank 55 and is a 10% PENALTY at rank 0, only becoming a discount past
// rank 25. That matches the settled decision on the item side and is deliberate.
func (c *Character) CalcCompanionReserve(baseCost int) int {
	cfg := configs.GetBalanceConfig()
	manif := c.GetSkillLevel(skills.Manifestation)
	manifRed := math.Min(float64(cfg.CompanionReserveSkillCap), float64(manif)*float64(cfg.CompanionReserveSkillPct))
	mutRank := mutations.GetCompanionReserveRank(c.Mutations)
	mutRed := math.Min(float64(cfg.CompanionReserveMutCap), float64(mutRank)*float64(cfg.CompanionReserveMutPctPerRank))
	reduction := math.Min(float64(cfg.CompanionReserveTotalCap), manifRed+mutRed)

	reserve := float64(baseCost) * (1.0 - reduction) * costs.SkillCostMultiplier(manif)
	if r := int(math.Round(reserve)); r > 0 {
		return r
	}
	// A companion that reserves nothing is an unbounded one. (Until U7b the
	// login pass also read 0 as its "legacy record" marker; D11 replaced that
	// backfill with a recompute of every companion, so 0 marks nothing now and
	// the floor stands on its own.)
	return 1
}
