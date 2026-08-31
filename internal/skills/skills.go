package skills

import (
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/casing"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/stats"
)

type SkillTag string

func (s SkillTag) String(subtag ...string) string {
	result := string(s)
	if len(subtag) > 0 {
		result += `:` + strings.Join(subtag, `:`)
	}
	return result
}

func (s SkillTag) Sub(subtag string) SkillTag {
	return SkillTag(string(s) + subtag)
}

const (
	// DOG combat & magic skills
	WeaponCombat  SkillTag = `weapon-combat`  // Melee attack & defense with weapons
	UnarmedCombat SkillTag = `unarmed-combat` // Fist/body attacks & defense, grappling
	RangedCombat  SkillTag = `ranged-combat`  // Bows, crossbows, pistols — aimed shots (Perception)
	Spellcasting  SkillTag = `spellcasting`   // All magic — offense & defense
	Rhetoric      SkillTag = `rhetoric`       // Conviction attacks — taunt, demoralize (Stage 34)

	// DOG non-combat skills
	Skullduggery  SkillTag = `skullduggery`  // Sneaking, stealing, lockpicking, surprise attacks
	Search        SkillTag = `search`        // Finding hidden objects, creatures, and resources
	Bartering     SkillTag = `bartering`     // Trade prices, negotiation, appraisal
	Blacksmithing SkillTag = `blacksmithing` // Metal weapons, armor, tools
	Alchemy       SkillTag = `alchemy`       // Potions, salves, medicines
	Tailoring     SkillTag = `tailoring`     // Cloth and leather goods
	Cooking       SkillTag = `cooking`       // Food preparation, buffs from meals
	Jewelcrafting SkillTag = `jewelcrafting` // Rings, pendants, gemwork
	Enchanting    SkillTag = `enchanting`    // Imbuing items with magic (31.6)
	Salvage       SkillTag = `salvage`       // Breaking down items for materials
	Manifestation SkillTag = `manifestation` // Companion summoning, charming, necromancy
)

// skillBlurbs gives a short, player-facing "what it does" line for each skill,
// shown in the skills list so players know what a skill covers. Notably,
// foraging (the `forage` command) trains Search, so Search's blurb says so —
// players who forage were confused that there was no separate "foraging" skill.
var skillBlurbs = map[SkillTag]string{
	WeaponCombat:  "Melee with weapons -- attack and defense.",
	UnarmedCombat: "Fists, grappling, and unarmed defense.",
	RangedCombat:  "Bows, crossbows, and firearms (aimed with Perception).",
	Spellcasting:  "All magic, offense and defense.",
	Rhetoric:      "Conviction attacks -- taunt and demoralize.",
	Skullduggery:  "Sneaking, stealing, lockpicking, and ambush.",
	Search:        "Finding hidden things -- and foraging the wild for resources.",
	Bartering:     "Better prices and appraisal when you trade.",
	Blacksmithing: "Forging metal weapons, armor, and tools.",
	Alchemy:       "Brewing potions, salves, and medicines.",
	Tailoring:     "Crafting cloth and leather goods.",
	Cooking:       "Preparing food and the buffs good meals give.",
	Jewelcrafting: "Rings, pendants, and gemwork.",
	Enchanting:    "Imbuing items with magic.",
	Salvage:       "Breaking items down into materials.",
	Manifestation: "Summoning companions, charming, and necromancy.",
}

// SkillBlurb returns the short description for a skill name, or "" if unknown.
func SkillBlurb(name string) string {
	return skillBlurbs[SkillTag(name)]
}

var (
	allSkillNames = []SkillTag{}

	Professions = map[string][]SkillTag{
		"warrior": {
			WeaponCombat,
			UnarmedCombat,
		},
		"hunter": {
			RangedCombat,
			Search,
		},
		"ranger": {
			Search,
		},
		"mage": {
			Spellcasting,
		},
		"healer": {
			Spellcasting,
		},
		"rogue": {
			Skullduggery,
			WeaponCombat,
		},
		"merchant": {
			Bartering,
		},
		"survivalist": {
			Search,
		},
		"smith": {
			Blacksmithing,
		},
		"alchemist": {
			Alchemy,
		},
		"tailor": {
			Tailoring,
		},
		"cook": {
			Cooking,
		},
		"artificer": {
			Jewelcrafting,
			Enchanting,
		},
		"orator": {
			Rhetoric,
			Bartering,
		},
		"scavenger": {
			Search,
			Salvage,
		},
	}
)

func SkillExists(sk string) bool {
	for _, skTag := range allSkillNames {
		if sk == string(skTag) {
			return true
		}
	}
	return false
}

func GetAllSkillNames() []SkillTag {
	return append([]SkillTag{}, allSkillNames...)
}

// GetMutationTier returns the mutation tier prefix based on total mutation load.
func GetMutationTier(owned map[string]int) string {
	load := mutations.GetMutationLoad(owned)
	switch {
	case load >= 50:
		return "exalted"
	case load >= 30:
		return "ascendant"
	case load >= 15:
		return "evolved"
	case load >= 1:
		return "awakened"
	default:
		return ""
	}
}

// GetSkillTier returns the skill tier based on aggregate completion across all skills.
func GetSkillTier(allRanks map[string]int) string {
	// totalSkills is a tuning denominator for the title-threshold curve, not
	// the actual skill count (currently 16 DOG skills). Raise it only when
	// the curve needs rebalancing, not every time a skill is added.
	const totalSkills = 17
	const softCap = 50.0
	const demigodRankTotal = 1200.0 // raw rank-sum for the demigod pinnacle
	maxTotal := totalSkills * softCap

	total := 0.0
	for _, rank := range allRanks {
		total += float64(rank)
	}

	// demigod: the raw-total pinnacle — an enormous sum of ranks (only reachable
	// by pushing skills well past the soft cap across the board).
	if total >= demigodRankTotal {
		return "demigod"
	}
	// grandmaster: every profession mastered (all canonical skills at the master
	// rank, 50 = soft cap). The all-professions-mastered capstone.
	if allProfessionsMastered(allRanks) {
		return "grandmaster"
	}

	pct := total / maxTotal
	switch {
	case pct >= 0.56:
		return "master"
	case pct >= 0.31:
		return "expert"
	case pct >= 0.16:
		return "journeyman"
	case pct >= 0.06:
		return "apprentice"
	case pct >= 0.01:
		return "novice"
	default:
		return "scrub"
	}
}

// allProfessionsMastered reports whether every canonical DOG skill (the keys of
// SkillPrimaryStats) is at the master rank (50 = the soft cap) or above. This is
// the "grandmaster" all-professions-mastered capstone condition for GetSkillTier.
func allProfessionsMastered(allRanks map[string]int) bool {
	const masterRank = 50
	for skill := range SkillPrimaryStats {
		if allRanks[skill] < masterRank {
			return false
		}
	}
	return true
}

// statEntry is used for sorting stats by value.
type statEntry struct {
	name  string
	value int
}

// GetStatArchetype determines the character's archetype based on stat distribution.
func GetStatArchetype(s stats.Statistics) string {
	entries := []statEntry{
		{"Strength", s.Strength.Value},
		{"Dexterity", s.Dexterity.Value},
		{"Perception", s.Perception.Value},
		{"Vitality", s.Vitality.Value},
		{"Willpower", s.Willpower.Value},
		{"Charisma", s.Charisma.Value},
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].value > entries[j].value
	})

	top := entries[0]
	second := entries[1]
	third := entries[2]

	// Check if top stat is >10% above second → pure archetype
	threshold := 0.10
	if top.value > 0 && float64(top.value-second.value)/float64(top.value) > threshold {
		return pureArchetype(top.name)
	}

	// Check if top 2 are within 10% of each other AND both >10% above third → hybrid
	if top.value > 0 && float64(top.value-second.value)/float64(top.value) <= threshold {
		if third.value == 0 || float64(second.value-third.value)/float64(second.value) > threshold {
			return hybridArchetype(top.name, second.name)
		}
	}

	return "generalist"
}

func pureArchetype(stat string) string {
	switch stat {
	case "Strength":
		return "warrior"
	case "Dexterity":
		return "rogue"
	case "Perception":
		return "scout"
	case "Vitality":
		return "guardian"
	case "Willpower":
		return "channeler"
	case "Charisma":
		return "orator"
	default:
		return "generalist"
	}
}

func hybridArchetype(stat1, stat2 string) string {
	// Create a pair key that works regardless of order
	pair := stat1 + "+" + stat2
	// Also check reversed
	pairRev := stat2 + "+" + stat1

	hybrids := map[string]string{
		"Strength+Willpower":   "paladin",
		"Strength+Vitality":    "juggernaut",
		"Strength+Dexterity":   "duelist",
		"Dexterity+Perception": "ranger",
		"Willpower+Charisma":   "sage",
		"Perception+Willpower": "seer",
		"Vitality+Willpower":   "stoic",
	}

	if name, ok := hybrids[pair]; ok {
		return name
	}
	if name, ok := hybrids[pairRev]; ok {
		return name
	}
	// Unmapped hybrid pair → use pure archetype of the top stat
	return pureArchetype(stat1)
}

// GetTitle returns the three-part title: "<MutationTier> <SkillTier> <StatArchetype>".
// E.g., "Awakened expert paladin" or "scrub warrior".
func GetTitle(owned map[string]int, allRanks map[string]int, s stats.Statistics) string {
	mutTier := GetMutationTier(owned)
	skillTier := GetSkillTier(allRanks)
	archetype := GetStatArchetype(s)

	parts := []string{}
	if mutTier != "" {
		parts = append(parts, mutTier)
	}
	parts = append(parts, skillTier)
	parts = append(parts, archetype)

	return casing.Title(strings.Join(parts, " "))
}

// SkillPrimaryStats maps each DOG skill to its primary governing stat.
// This stat is auto-tracked and progressed whenever the skill is used.
var SkillPrimaryStats = map[string]string{
	"weapon-combat":  "dexterity",
	"unarmed-combat": "dexterity",
	"ranged-combat":  "perception",
	"spellcasting":   "willpower",
	"skullduggery":   "dexterity",
	"search":         "perception",
	"rhetoric":       "charisma",
	"bartering":      "charisma",
	"blacksmithing":  "strength",
	"alchemy":        "perception",
	"tailoring":      "dexterity",
	"cooking":        "perception",
	"jewelcrafting":  "dexterity",
	"enchanting":     "perception",
	"salvage":        "perception",
	"manifestation":  "charisma",
}

// GetSkillPrimaryStat returns the primary governing stat for a skill,
// or an empty string if none is defined.
func GetSkillPrimaryStat(skillName string) string {
	return SkillPrimaryStats[skillName]
}

// SkillProgressionMultipliers controls how fast each skill progresses.
// Combat skills fire many times per round, so they get a low multiplier.
// Utility skills are used less often, so they get a high multiplier.
// Solved on measured play-time rates, U10b-1 Task 23. Regenerate with
// `python tools/balance/u10b1_solve_v4.py`; do not hand-edit these to taste.
//
// v3 is superseded on two counts: it read the HIT rate (0.5752) as the
// CLEAN-HIT rate, which is 0.3856, and it predates the Best-of firing
// convention, under which one resolved action pays ONE event rather than one
// per weapon entry and one per defence type.
//
// MUST stay in sync with Balance.SkillProgressionMultipliers in
// _datafiles/config.yaml. GetSkillProgressionMultiplier returns (0, false) on a
// config miss, meaning "use the hardcoded default", so THIS map is what every
// test binary sees -- tests never load config.yaml.
var SkillProgressionMultipliers = map[SkillTag]float64{
	// Melee. unarmed sits BELOW weapon deliberately, and the gap WIDENED under
	// the firing convention. Only ONE skill is awarded per round now, so the
	// question stopped being "how many entries cleanly hit" and became "which
	// skill rolled highest".
	//
	// Each is solved at its OWN concentrating build. For weapon-combat that is
	// 1H+shield. For unarmed-combat it is BARE HANDS, and that build was absent
	// from the solver until 2026-08-27: unarmed was being solved at 1H+fist, a
	// build nobody grinding unarmed would choose.
	//
	// Bare hands are a different SHAPE, not a faster version of 1H+fist. Two
	// empty hands put TWO fist entries in the plan, but attackerCandidates keys
	// on SKILL, so they fold into ONE unarmed-combat candidate with nothing to
	// out-roll it -- it wins the attacker Best-of every round rather than two in
	// three. Its clean hit is the OR across all 8 swings (0.980, against 0.858
	// for a lone fist over 4 and 0.623 for a longsword over 2). And
	// equipmentGatedMeleeDefences gives a bare-handed defender dodge and nothing
	// else: no parry, and with two EMPTY hands no block either. Dodge maps to
	// unarmed-combat, so it takes the WHOLE defence award instead of dodge's
	// 83.6% share.
	//
	// ⚠️ The clause "no block even holding a shield" used to appear here and is
	// no longer true. On 2026-08-30 that gate became per-slot, so a shield in
	// ANY hand now blocks. THIS SOLVE IS UNAFFECTED because the build it prices
	// holds nothing at all — but do not re-derive it from the old sentence.
	//
	// That totals 1.74 events per engaged round against 0.93 for a shield build.
	// It is paid for in damage (UnarmedDamageMultiplier 0.30) and in having no
	// parry or block at all, which is a trade the multiplier is now solved
	// against rather than one it was silently ignoring.
	//
	// The offhand fix (each weapon rolls its OWN skill) contributed to the move
	// but was the smaller half of it; the missing build was the larger.
	WeaponCombat:  1.34,
	UnarmedCombat: 0.72,
	// These three share ONE 4-round "special-move" key with 15 other verbs, so a
	// concerted grinder gets only ~22.5 uses/hour between them, not each.
	// ranged-combat joins indirectly: firing is ungated but reload is, and a
	// weapon must be loaded to fire.
	RangedCombat: 6.88,
	// spellcasting is the one combat skill whose multiplier FELL. It gained a
	// second faucet: the concentration contest was success-only, so a caster who
	// lost the hold trained nothing, and it now pays on a loss too. Damage under
	// ConcentrationDamageThresholdPct still never rolls, which is what keeps
	// that faucet from swamping deliberate casting.
	Spellcasting: 2.99,
	Rhetoric:     6.88,
	// Assumes the Phase D conjure cooldown is in place. Without it manifestation
	// runs at 225/hr standing still and this over-rewards it ~9x.
	Manifestation: 5.13,
	// Utility. bartering assumes the Phase D per-transaction fix; awarding per
	// unit made it unbounded in time and unfittable. search is anchored on
	// forage's 6-round cooldown at 100% engagement, NOT on the `search` command,
	// which is anti-botting-gated and cannot be ground.
	//
	// bartering is the control row: buy and sell award with won=true, so the
	// firing convention changed its rate by exactly nothing and its multiplier
	// is unmoved. Any future solve that shifts it has a bug in it.
	Search:       1.02,
	Bartering:    2.07,
	Skullduggery: 1.23,
	Salvage:      2.80,
	// Crafts, at the owner's 40% engagement: an hour of crafting is gather THEN
	// craft, so only part of it is spent at the station.
	Blacksmithing: 1.56,
	Alchemy:       1.56,
	Tailoring:     1.56,
	Cooking:       1.56,
	Jewelcrafting: 1.56,
	Enchanting:    1.56,
}

// GetSkillRankDescription converts a numeric skill level to a qualitative
// tier name. Skills soft-cap at 50 (master); grandmaster tier rewards the
// slow progression above the cap.
func GetSkillRankDescription(level int) string {
	switch {
	case level <= 0:
		return "unknown"
	case level <= 1:
		return "novice"
	case level <= 9:
		return "apprentice"
	case level <= 19:
		return "journeyman"
	case level <= 34:
		return "adept"
	case level <= 49:
		return "expert"
	case level <= 64:
		return "master"
	default:
		return "grandmaster"
	}
}

// GetProgressionMultiplier returns the progression speed multiplier for a skill.
// Config overrides take priority; falls back to the hardcoded SkillProgressionMultipliers map.
// Returns 1.0 for any skill not in either source.
func GetProgressionMultiplier(skillName string) float64 {
	b := configs.GetBalanceConfig()
	if mult, ok := b.GetSkillProgressionMultiplier(skillName); ok {
		return mult
	}
	if mult, ok := SkillProgressionMultipliers[SkillTag(skillName)]; ok {
		return mult
	}
	return 1.0
}

func init() {

	skillNameSet := map[SkillTag]struct{}{}

	for _, skills := range Professions {
		for _, skillName := range skills {

			if _, ok := skillNameSet[skillName]; ok {
				continue
			}

			skillNameSet[skillName] = struct{}{}
			allSkillNames = append(allSkillNames, skillName)
		}
	}

	// Register all DOG skills directly (ensures any not in professions are included)
	for _, sk := range []SkillTag{
		WeaponCombat, UnarmedCombat, RangedCombat, Spellcasting, Rhetoric,
		Skullduggery, Search, Bartering,
		Blacksmithing, Alchemy, Tailoring, Cooking, Jewelcrafting, Enchanting, Salvage,
		Manifestation,
	} {
		if _, ok := skillNameSet[sk]; !ok {
			skillNameSet[sk] = struct{}{}
			allSkillNames = append(allSkillNames, sk)
		}
	}

}
