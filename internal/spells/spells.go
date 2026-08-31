package spells

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/casing"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/stats"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/pkg/errors"
)

type SpellType string

type SpellData struct {
	SpellId            string    `yaml:"spellid,omitempty"`
	Name               string    `yaml:"name,omitempty"`
	Aliases            []string  `yaml:"aliases,omitempty"` // short single-word invocation forms (primary first)
	Description        string    `yaml:"description,omitempty"`
	Type               SpellType `yaml:"type,omitempty"`
	Schools            []string  `yaml:"schools,omitempty"`    // Can have multiple school tags
	Categories         []string  `yaml:"categories,omitempty"` // AI categorization: self_defense, self_offense, etc. Free-form strings.
	Cost               int       `yaml:"cost,omitempty"`       // Conviction cost
	HealthCost         int       `yaml:"healthcost,omitempty"` // Optional Health cost for life-force magic
	WaitRounds         int       `yaml:"waitrounds,omitempty"`
	Difficulty         int       `yaml:"difficulty,omitempty"`           // Augments final success chance by this %
	PrimaryStat        string    `yaml:"primarystat,omitempty"`          // Stat used for spell rolls and progression
	BaseFolds          int       `yaml:"base_folds,omitempty"`           // 0 = default to 4
	TargetDefenseType  string    `yaml:"target_defense_type,omitempty"`  // "physical", "mental", "" = none
	ComponentTag       string    `yaml:"component_tag,omitempty"`        // Required item component tag (e.g. "stone")
	EffectType         string    `yaml:"effect_type,omitempty"`          // "damage"|"heal"|"buff"|"shield"|"dot"|"knockdown"|"charm"|"drain_area" (mob-cast only: area life-drain + self-heal, see resolveMobDrainArea)
	EffectMagnitude    int       `yaml:"effect_magnitude,omitempty"`     // Legacy: base damage/heal amount
	DamageMultiplier   float64   `yaml:"damage_multiplier,omitempty"`    // Spell damage multiplier for new pipeline (Stage 34)
	EffectDuration     int       `yaml:"effect_duration,omitempty"`      // DoT tick count (default 0 = use 3)
	BuffIds            []int     `yaml:"buff_ids,omitempty"`             // Buff IDs to apply (for "buff" effect type)
	QuestRequired      string    `yaml:"quest_required,omitempty"`       // Quest token required before spell can be discovered
	MobOnly            bool      `yaml:"mob_only,omitempty"`             // Boss/NPC signature ability: players may never DISCOVER it. See GetEligibleSpells.
	NoDamageInterrupt  bool      `yaml:"no_damage_interrupt,omitempty"`  // Telegraphed casts: skip damage/position concentration-break (still interrupted by the disruptor system)
	IgnoreMoveCooldown bool      `yaml:"ignore_move_cooldown,omitempty"` // Scripted boss abilities: bypass the shared special-move cast cooldown (btree controls cadence)

	// Companion summoning fields: replaces JS onMagic for summon spells
	SummonMobId int `yaml:"summon_mob_id,omitempty"`
	// SummonPetMultiplier is the single dial for this pet's tier. It scales the
	// caster's own power into the companion's stat pool (see
	// characters.CalcCompanionPool), and multiplies CompanionReserveDefault to
	// give the ongoing Conviction the companion reserves.
	//
	// It replaced summon_base_pool in U7b. Under the old shape the pet's base
	// pool MULTIPLIED the caster's power and the corpse was averaged in
	// afterwards, so the corpse's share grew until it swamped the pet choice: at
	// a 1000-pool corpse a skeleton fielded 587 and a golem 675, meaning five
	// times the price bought 15% more pet. Applying the multiplier AFTER the
	// average keeps every tier proportionally separated at every corpse size.
	SummonPetMultiplier  float64 `yaml:"summon_pet_multiplier,omitempty"`
	SummonComponentId    int     `yaml:"summon_component_id,omitempty"`
	SummonRequiresCorpse bool    `yaml:"summon_requires_corpse,omitempty"`
	SummonMinCorpsePool  int     `yaml:"summon_min_corpse_pool,omitempty"`

	// YAML text fields — flavor text sent by the engine (replaces JS messaging)
	CastUserText  string `yaml:"cast_user_text,omitempty"`
	CastRoomText  string `yaml:"cast_room_text,omitempty"`
	WaitUserText  string `yaml:"wait_user_text,omitempty"`
	WaitRoomText  string `yaml:"wait_room_text,omitempty"`
	MagicUserText string `yaml:"magic_user_text,omitempty"`
	MagicRoomText string `yaml:"magic_room_text,omitempty"`
}

const (
	WaitRoundsDefault = 3

	Neutral    SpellType = "neutral"    // Neutral, no expected actor target, use on
	HarmSingle SpellType = "harmsingle" // Harmful, defaults to current aggro - magic missile etc
	HarmMulti  SpellType = "harmmulti"  // Harmful, defaults to all aggro mobs - chain lightning etc
	HelpSingle SpellType = "helpsingle" // Helpful, defaults on self - heal etc
	HelpMulti  SpellType = "helpmulti"  // Helpful, defaults on party - mass heal etc
	HarmArea   SpellType = "harmarea"   // Hits everyone in the room, even if hidden or friendly
	HelpArea   SpellType = "helparea"   // Hits everyone in the room, even if hidden

	// DOG Spell Schools
	SchoolElemental     = "elemental"     // Fire, ice, lightning, earth, wind - offensive elemental magic
	SchoolEnhancement   = "enhancement"   // Buffs, shields, enchantments - augmentation magic
	SchoolMental        = "mental"        // Illusions, charms, telepathy - mind-affecting magic (Psionics skill)
	SchoolVital         = "vital"         // Healing, curing, life/death manipulation - vital force magic
	SchoolManifestation = "manifestation" // Companion summoning, charming, binding - uses Charisma+manifestation
)

var (
	allSpells     = map[string]*SpellData{}
	spellsByAlias = map[string]*SpellData{}
)

func (s SpellType) HelpOrHarmString() string {
	switch s {
	case Neutral:
		return `Neutral`
	case HelpSingle, HelpMulti, HelpArea:
		return `Helpful`
	case HarmSingle, HarmMulti, HarmArea:
		return `Harmful`
	}
	return `Unknown`
}

func (s SpellType) TargetTypeString(short ...bool) string {
	// Return a short version
	if len(short) > 0 && short[0] {
		switch s {
		case Neutral:
			return `Self`
		case HelpSingle, HarmSingle:
			return `Single`
		case HelpMulti, HarmMulti:
			return `Group`
		case HelpArea, HarmArea:
			return `Area`
		}
		return `Unknown`
	}
	// Regular handling
	switch s {
	case Neutral:
		return `Self`
	case HelpSingle:
		return `Single Target`
	case HarmSingle:
		return `Single Target`
	case HelpMulti:
		return `Group Target`
	case HarmMulti:
		return `Group Target`
	case HelpArea, HarmArea:
		return `Area Target`
	}
	return `Unknown`
}

// Finds a match for a spell by name or id
func FindSpell(spellName string) string {
	if sd, ok := allSpells[spellName]; ok {
		return sd.SpellId
	}
	for _, spellInfo := range allSpells {
		if strings.ToLower(spellInfo.Name) == spellName {
			return spellInfo.SpellId
		}
	}
	return ``
}

func GetSpell(spellId string) *SpellData {
	if sd, ok := allSpells[spellId]; ok {
		return sd
	}
	return nil
}

func FindSpellByName(spellName string) *SpellData {

	var closestMatch *SpellData = nil

	spellName = strings.ToLower(spellName)
	for _, spellData := range allSpells {

		testName := strings.ToLower(spellData.Name)

		if testName == spellName {
			return spellData
		}

		if closestMatch == nil && strings.HasPrefix(strings.ToLower(spellData.Name), spellName) {
			closestMatch = spellData
		}

	}
	return closestMatch
}

// buildSpellAliasIndex (re)builds the alias→spell index from allSpells, panicking
// on a duplicate alias or an alias that collides with a spellid.
func buildSpellAliasIndex() {
	spellsByAlias = make(map[string]*SpellData, len(allSpells))
	for _, s := range allSpells {
		for _, a := range s.Aliases {
			a = strings.ToLower(strings.TrimSpace(a))
			if a == "" || a == s.SpellId {
				continue // blank, or a redundant self-alias (already resolves via the id)
			}
			if _, clash := allSpells[a]; clash {
				panic(fmt.Sprintf("spell alias %q (on %s) collides with a spellid", a, s.SpellId))
			}
			if other, dup := spellsByAlias[a]; dup {
				panic(fmt.Sprintf("duplicate spell alias %q on %s and %s", a, other.SpellId, s.SpellId))
			}
			spellsByAlias[a] = s
		}
	}
}

// ResolveSpell resolves a token to a spell by canonical id, then alias, then
// full display name (case-insensitive). Returns nil if none match.
func ResolveSpell(token string) *SpellData {
	token = strings.ToLower(strings.TrimSpace(token))
	if sd, ok := allSpells[token]; ok {
		return sd
	}
	if sd, ok := spellsByAlias[token]; ok {
		return sd
	}
	return FindSpellByName(token)
}

// ResolveSpellId returns the canonical spellid for a token (id/alias/name), or "".
func ResolveSpellId(token string) string {
	if sd := ResolveSpell(token); sd != nil {
		return sd.SpellId
	}
	return ""
}

func GetAllSpells() map[string]*SpellData {
	retSpellBook := make(map[string]*SpellData)
	for k, v := range allSpells {
		retSpellBook[k] = v
	}
	return retSpellBook
}

func (s *SpellData) Id() string {
	return s.SpellId
}

// SpellData implements the Filepath method from the Loadable interface.
func (s *SpellData) Filepath() string {
	return util.FilePath(fmt.Sprintf("%s.yaml", s.SpellId))
}

// validStats is the closed set a spell's primarystat may name.
var validStats = map[string]struct{}{
	"strength": {}, "dexterity": {}, "perception": {},
	"vitality": {}, "willpower": {}, "charisma": {},
}

// U9 made primarystat load-bearing: it drives both the caster-side stat in
// spell resolution and the stat the cast trains. A typo must therefore fail at
// boot rather than falling back to a default, which is how the field spent its
// whole life describing an intent nothing implemented.
func (s *SpellData) validatePrimaryStat() error {
	if s.PrimaryStat == "" {
		return fmt.Errorf("spell %q: primarystat is required", s.SpellId)
	}
	if _, ok := validStats[s.PrimaryStat]; !ok {
		return fmt.Errorf("spell %q: primarystat %q is not one of the six stats", s.SpellId, s.PrimaryStat)
	}
	return nil
}

// CasterStatValue returns the caster-side stat value this spell's rolls,
// duration and shield strength are built from.
//
// CASTER SIDE ONLY. The DEFENDER's stat is owned by the U6 defence set --
// spell_resolution.go's quell read and charm's target resist both stay on
// Willpower by design, and routing them through here would silently move quell
// off the stat U6 designed it around.
func (s *SpellData) CasterStatValue(stats stats.Statistics) int {
	switch s.PrimaryStat {
	case "strength":
		return stats.Strength.ValueAdj
	case "dexterity":
		return stats.Dexterity.ValueAdj
	case "perception":
		return stats.Perception.ValueAdj
	case "vitality":
		return stats.Vitality.ValueAdj
	case "charisma":
		return stats.Charisma.ValueAdj
	default:
		return stats.Willpower.ValueAdj
	}
}

func (s *SpellData) Validate() error {

	if err := s.validatePrimaryStat(); err != nil {
		return err
	}

	if s.Difficulty < 0 {
		s.Difficulty = 0
	} else if s.Difficulty > 100 {
		s.Difficulty = 100
	}

	// Validate YAML text tokens
	for _, text := range []string{
		s.CastUserText, s.CastRoomText,
		s.WaitUserText, s.WaitRoomText,
		s.MagicUserText, s.MagicRoomText,
	} {
		for _, w := range textutil.ValidateTokens(text) {
			mudlog.Warn("Spell.Validate", "spellId", s.SpellId, "warning", w)
		}
	}

	// Validate summon fields
	if s.SummonMobId > 0 && s.SummonPetMultiplier <= 0 {
		mudlog.Warn("Spell.Validate", "spellId", s.SpellId, "warning", "summon_mob_id set but summon_pet_multiplier is 0 or missing")
	}
	if s.SummonRequiresCorpse && s.SummonMinCorpsePool == 0 {
		mudlog.Warn("Spell.Validate", "spellId", s.SpellId, "warning", "summon_requires_corpse set but summon_min_corpse_pool is 0")
	}

	return nil
}

func (s *SpellData) GetDifficulty() int {
	return s.Difficulty
}

// HasSchool returns true if the spell belongs to the given school.
func (s *SpellData) HasSchool(school string) bool {
	for _, sc := range s.Schools {
		if sc == school {
			return true
		}
	}
	return false
}

// GetSchoolsString returns a comma-separated string of spell schools
func (s *SpellData) GetSchoolsString() string {
	if len(s.Schools) == 0 {
		return "Unknown"
	}
	return strings.Join(s.Schools, ", ")
}

// GetTotalConvictionCost returns the conviction cost, optionally scaled by a multiplier
func (s *SpellData) GetTotalConvictionCost(multiplier float64) int {
	if multiplier <= 0 {
		multiplier = 1.0
	}
	return int(float64(s.Cost) * multiplier)
}

// GetTotalHealthCost returns the health cost, optionally scaled by a multiplier
func (s *SpellData) GetTotalHealthCost(multiplier float64) int {
	if multiplier <= 0 {
		multiplier = 1.0
	}
	return int(float64(s.HealthCost) * multiplier)
}

// RequiredSkillFor is a spell's skill minimum for DISCOVERY, derived from its
// authored difficulty. It is the spell-side equivalent of a recipe's
// skill_minimum, which crafting has gated on all along
// (crafting.GetEligibleRecipes).
//
// U10b-3 REPLACED A FOLD GATE WITH THIS. Discovery used to compare a spell's
// base_folds against a MaxFoldsForSkill ladder, which conflated two unrelated
// things: folds measure how long and involved a cast is, not how hard the spell
// is to learn. The two only loosely correlate in the authored content, and
// where they diverged the gate did the wrong thing in both directions:
//
//	Core Discharge / Core Drain -- difficulty 45, base_folds 2, so the old
//	ladder let a spellcasting-0 NOVICE discover both.
//	Charm -- difficulty 60, base_folds 36, against a ladder that topped out at
//	32, so it was UNDISCOVERABLE at any skill by anyone. No quest grants it and
//	no item teaches it, which made it unobtainable outright.
//
// base_folds still does its real jobs elsewhere (cast duration via
// calcSpellDuration, AI spell scoring, telegraph/interruptibility). It just no
// longer decides what you are allowed to learn.
//
// The ratio is a config knob so this stays a tuning edit. At the shipped 1.0 a
// spell's difficulty IS its required skill, which is the whole point: the
// number authors already write is the number that gates it.
func RequiredSkillFor(difficulty int) int {
	if difficulty <= 0 {
		return 0
	}
	ratio := float64(configs.GetBalanceConfig().SpellDiscoverySkillPerDifficulty)
	if ratio <= 0 {
		return 0 // off-switch: difficulty gates nothing
	}
	return int(math.Ceil(float64(difficulty) * ratio))
}

// GetEligibleSpells returns spell IDs the player could discover (not in spellBook, within fold threshold).
// When schools are provided, only spells belonging to at least one of those schools are returned.
// When schools is empty, all non-manifestation spells are returned (backward compat).
// Spells with QuestRequired set are never returned by discovery.
// GetEligibleSpells returns what a PLAYER may organically discover.
//
// ⚠️ MOB-ONLY SPELLS ARE EXCLUDED HERE, and the default is deliberately the
// safe one: a mob caller must opt in via GetEligibleSpellsForMob. Owner found
// Meirok had learned `core-drain` in play — the Core Guardian's signature
// recharge, whose own description reads "Mob-castable only ... not a spell any
// living caster could learn". That was prose; nothing enforced it.
//
// The three self-declared mob-only spells carry three DIFFERENT effect types
// (core-discharge damage, core-drain drain_area, repair-pulse heal), so there
// was no incidental property to filter on — hence the explicit MobOnly flag.
// repair-pulse is difficulty 10, so it was reachable by a near-novice.
//
// ⚠️ This is NOT a U10b-3 regression. RequiredSkillFor's own comment already
// names Core Discharge and Core Drain as spells the OLD fold ladder let a
// spellcasting-0 novice discover. U10b-3 raised the bar from 0 to 45 and
// narrowed the exposure; it never closed it. Meirok at spellcasting 51 walked
// straight over the new bar.
func GetEligibleSpells(spellBook map[string]int, skillLevel int, schools ...string) []string {
	return eligibleSpells(spellBook, skillLevel, false, schools)
}

// GetEligibleSpellsForMob is the same pool with mob-only spells ALLOWED. Mobs
// learn organically through the same seam (NewRound_DoCombat_helpers.go), and
// excluding these would stop a Core Guardian ever learning its own ability.
func GetEligibleSpellsForMob(spellBook map[string]int, skillLevel int, schools ...string) []string {
	return eligibleSpells(spellBook, skillLevel, true, schools)
}

func eligibleSpells(spellBook map[string]int, skillLevel int, allowMobOnly bool, schools []string) []string {
	var eligible []string
	for id, sp := range allSpells {
		if _, known := spellBook[id]; known {
			continue
		}
		// Boss/NPC signature abilities are never discovered by players.
		if sp.MobOnly && !allowMobOnly {
			continue
		}
		// Quest-gated spells are never discovered organically.
		if sp.QuestRequired != "" {
			continue
		}
		// School filtering.
		if len(schools) > 0 {
			matched := false
			for _, school := range schools {
				if sp.HasSchool(school) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		} else {
			// Backward compat: no school filter means all non-manifestation spells.
			if sp.HasSchool(SchoolManifestation) {
				continue
			}
		}
		// The skill gate, mirroring crafting's skill_minimum check. base_folds
		// used to decide this; see RequiredSkillFor for why it no longer does.
		if skillLevel >= RequiredSkillFor(sp.Difficulty) {
			eligible = append(eligible, id)
		}
	}
	return eligible
}

func LoadSpellFiles() {

	start := time.Now()

	dataPath := string(configs.GetFilePathsConfig().DataFiles) + `/spells`
	tmpAllSpells, err := fileloader.LoadAllFlatFiles[string, *SpellData](dataPath)
	if err != nil {
		panic(errors.Wrap(err, `filepath: `+dataPath))
	}

	for _, s := range tmpAllSpells {
		if s.Name != "" {
			casing.AssertCanonical(s.Name, "spell", s.SpellId)
		}
	}

	allSpells = tmpAllSpells

	buildSpellAliasIndex()

	mudlog.Info("spells.loadAllSpells()", "loadedCount", len(allSpells), "Time Taken", time.Since(start))

}

// DifficultiesFor returns the authored difficulty of each spell id, parallel to
// the input slice, for configs.WeightedDiscoveryPick. An unknown id contributes
// 0 (treated as easiest) rather than dropping out, so the returned slice always
// lines up with the caller's candidate slice -- a length mismatch there would
// silently pick the wrong spell.
func DifficultiesFor(spellIds []string) []int {
	out := make([]int, len(spellIds))
	for i, id := range spellIds {
		if sp := GetSpell(id); sp != nil {
			out[i] = sp.Difficulty
		}
	}
	return out
}
