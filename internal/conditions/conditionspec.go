package conditions

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/casing"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/pkg/errors"
)

// Something temporarily attached to a character
// That modifies some aspect of their status
/*
Examples:
Fast Healing - increased natural health recovery for 10 rounds
Poison - add -10 health every round for 5 rounds
*/

type Flag string

const (
	//
	// All Flags must be lowercase
	//
	All Flag = ``

	// Behavioral flags
	NoCombat       Flag = `no-combat`
	NoMovement     Flag = `no-go`
	NoFlee         Flag = `no-flee`
	NoAggroTarget  Flag = `no-aggro-target` // grace-period protection: mobs cannot acquire aggro on the bearer
	CancelIfCombat Flag = `cancel-on-combat`
	CancelOnAction Flag = `cancel-on-action`
	CancelOnDamage Flag = `cancel-on-damage` // chunk 3.3: cancels on any damage event
	CancelOnWater  Flag = `cancel-on-water`

	// Death preventing
	ReviveOnDeath Flag = `revive-on-death`

	// Gear related
	PermaGear   Flag = `perma-gear`
	RemoveCurse Flag = `remove-curse`

	// Harmful flags
	Poison Flag = `poison`
	Drunk  Flag = `drunk`

	// Protective flags. PoisonImmunity is the answer to Poison: while it is
	// held, poison-flagged conditions and the poisoned condition are refused.
	// Stone Stomach (condition 64) is the shipped holder.
	PoisonImmunity Flag = `poison-immunity`

	// Useful flags
	Hidden         Flag = `hidden`
	Sleeping       Flag = `sleeping` // chunk 3.3: bearer is asleep
	SuperHearing   Flag = `superhearing`
	NightVision    Flag = `nightvision`
	InfraredVision Flag = `infraredvision`
	Warmed         Flag = `warmed`
	Hydrated       Flag = `hydrated`
	Thirsty        Flag = `thirsty`

	// Phase 25 spell condition flags
	Haste         Flag = `haste`
	DamageBonus   Flag = `damage-bonus`
	Slow          Flag = `slow`
	SkillProgress Flag = `skill-progress`
	MutationRate  Flag = `mutation-rate`

	// Flags that reveal things
	SeeHidden Flag = `see-hidden`
	SeeNouns  Flag = `see-nouns`

	Dampened Flag = `dampened` // #22 crash-site: Chrysalis suppression — mutation/spell power scaled down

	// SilentStart marks a condition whose start is narrated by whatever applies it
	// (warcry, rally, the bloom detox drink), so it has no start notice of
	// its own and the guard does not require start_actee. The end notice
	// is unaffected.
	SilentStart Flag = `silent-start`

	// Bleeding marks a bleed record: death cause reads it, as it reads Poison.
	Bleeding Flag = `bleeding`
	// Quiet marks a record that is listed but sends no start or end line,
	// because it is reapplied every round it persists (prone recovery, the
	// grapple exposure) and any line would repeat each round.
	Quiet Flag = `quiet`

	// Stacking marks a tick record where every application is its own stack
	// with its own timer, instead of refreshing the one instance. The record
	// ticks the sum of its live stacks once a round and ends with its longest
	// stack. It requires tick_from_magnitude and a one-round triggerrate.
	// Meant for the bleed record (122).
	Stacking Flag = `stacking`

	// Adjustable marks a light source that trims itself to its bearer's eyes
	// each time the bearer enters a room (lighting plan 5a), or a darkness
	// source that trims itself to its bearer's usable range (5d). It requires
	// the light_strength or the darkness_strength effect.
	Adjustable Flag = `adjustable`
	// Cancellable marks a condition its holder may end early with
	// `cancel <spell>`. Opt-in: the Cat's Eye Draught is ruled uncancellable.
	Cancellable Flag = `cancellable`

	// Arbitrarily chosen round for calculating trigger round counts
	validationRound = 1000000
)

// AllFlags is every flag the engine understands. LoadDataFiles rejects a condition
// whose flags include anything else, exactly as spelled: the Cat's Eye
// Draught shipped with `night-vision` for `nightvision` and did nothing for
// weeks. TestAllFlagsNamesEveryDeclaredConstant keeps this list honest.
//
// The All sentinel is deliberately absent: it is the empty string, a query
// wildcard, never something a condition file may carry.
var AllFlags = []Flag{
	NoCombat,
	NoMovement,
	NoFlee,
	NoAggroTarget,
	CancelIfCombat,
	CancelOnAction,
	CancelOnDamage,
	CancelOnWater,
	ReviveOnDeath,
	PermaGear,
	RemoveCurse,
	Poison,
	Drunk,
	PoisonImmunity,
	Hidden,
	Sleeping,
	SuperHearing,
	NightVision,
	InfraredVision,
	Warmed,
	Hydrated,
	Thirsty,
	Haste,
	DamageBonus,
	Slow,
	SkillProgress,
	MutationRate,
	SeeHidden,
	SeeNouns,
	Dampened,
	SilentStart,
	Bleeding,
	Quiet,
	Stacking,
	Adjustable,
	Cancellable,
}

var (
	conditions map[int]*ConditionSpec = make(map[int]*ConditionSpec)

	validationCalculator = gametime.GetDate(validationRound)
)

type ConditionSpec struct {
	ConditionId   int               `yaml:"conditionid"` // Unique identifier for this condition spec. The tag states the file key explicitly.
	Name          string            // The name of the condition
	Description   string            // A description of the condition
	Secret        bool              // Whether or not the condition is secret (not displayed to the user)
	TriggerNow    bool              `yaml:"triggernow,omitempty"`    // if true, condition triggers once right when it is applied
	TriggerRate   string            `yaml:"triggerrate,omitempty"`   // How often should it trigger? (time string)
	RoundInterval int               `yaml:"roundinterval,omitempty"` // triggers every x rounds
	TriggerCount  int               `yaml:"triggercount,omitempty"`  // How many times it triggers before it is removed
	StatMods      statmods.StatMods `yaml:"statmods,omitempty"`      // stat mods for the duration of the condition
	Flags         []Flag            `yaml:"flags,omitempty"`         // A list of actions and such that this condition prevents or enables

	// ProgressMult is the multiplier applied by the skill-progress and
	// mutation-rate flags while this condition is held. 0 means the default
	// (2.0, the historic literal). The strongest held value wins.
	ProgressMult float64 `yaml:"progress_mult,omitempty"`

	// YAML text fields: flavor text sent by the engine (replaces JS messaging).
	//
	// The authored keys name the PHASE and then the AUDIENCE, in the canonical
	// role vocabulary every narration store shares since M4b-1. The holder's
	// half is `actee`, not `actor`: a condition is something that happens TO
	// the character holding it, which is why this store's {actee} token means
	// the holder while a spell's means the spell's target.
	StartUserText   string `yaml:"start_actee,omitempty"`
	StartRoomText   string `yaml:"start_observer,omitempty"`
	TriggerUserText string `yaml:"trigger_actee,omitempty"`
	TriggerRoomText string `yaml:"trigger_observer,omitempty"`
	EndUserText     string `yaml:"end_actee,omitempty"`
	EndRoomText     string `yaml:"end_observer,omitempty"`

	// Config-driven tick fields — replaces JS onTrigger for heal/DoT conditions
	TickPool              string  `yaml:"tick_pool,omitempty"`               // "health", "stamina", "conviction"
	TickPercent           float64 `yaml:"tick_percent,omitempty"`            // Base % of max pool. Positive=heal, negative=damage
	TickVariance          float64 `yaml:"tick_variance,omitempty"`           // Random variance added to percent
	TickMin               int     `yaml:"tick_min,omitempty"`                // Minimum absolute tick amount (default 1)
	StartRemoveConditions []int   `yaml:"start_remove_conditions,omitempty"` // Condition IDs to remove when this condition starts

	// Effects is the closed mechanical vocabulary combat reads through
	// Conditions.Effect. See effects.go. A value is a number or the word
	// "magnitude".
	Effects map[EffectKind]EffectValue `yaml:"effects,omitempty"`
	// TickFromMagnitude marks a tick record whose per-round amount is the
	// applier's magnitude rather than tick_percent of a pool: the spell dot
	// and bleed records. Requires tick_pool; forbids tick_percent.
	TickFromMagnitude bool `yaml:"tick_from_magnitude,omitempty"`
}

// Calculates the value of this condition
func (b *ConditionSpec) GetValue() int {
	val := 0

	for _, v := range b.StatMods {
		val += int(math.Abs(float64(v)))
	}

	freqVal := max(5-b.RoundInterval, 0)
	val += freqVal
	val += len(b.Flags) * 5

	if b.TriggerCount > 0 {
		val *= b.TriggerCount
	}

	return val
}

// Listed reports whether a held record appears in the player's condition
// lists: the in-game `conditions` command and the Char.Conditions GMCP
// payload. Both call this, so the two can never disagree. A hidden record is
// left out because it would tell you that you are hidden; a secret record is
// engine bookkeeping or a state the player is not meant to know about (owner
// ruling 2026-09-14: shown in neither list, not as "Mysterious Affliction").
func (b *ConditionSpec) Listed() bool {
	return !b.Secret && !slices.Contains(b.Flags, Hidden)
}

// IsLightSource reports whether a record of this spec sheds light. A darkness
// source is not a light (lighting plan 5d, ruling D1): this stays light-only,
// so every light reader excludes darkness by construction.
func (b *ConditionSpec) IsLightSource() bool {
	_, ok := b.Effects[EffectLightStrength]
	return ok
}

// IsDarknessSource reports whether a record of this spec takes light away
// from its room (lighting plan 5d).
func (b *ConditionSpec) IsDarknessSource() bool {
	_, ok := b.Effects[EffectDarknessStrength]
	return ok
}

// AnyDarknessSource reports whether any of the condition ids names a
// darkness source: an item whose worn conditions darken its room announces
// itself as lit (lighting plan 5d, ruling D6). Unknown ids are skipped.
func AnyDarknessSource(conditionIds []int) bool {
	for _, id := range conditionIds {
		if spec := GetConditionSpec(id); spec != nil && spec.IsDarknessSource() {
			return true
		}
	}
	return false
}

type ConditionMessage struct {
	User string
	Room string
}

type ConditionMessages struct {
	Start  ConditionMessage
	Effect ConditionMessage
	End    ConditionMessage
}

func GetConditionSpec(conditionId int) *ConditionSpec {
	if conditionId < 0 {
		conditionId *= -1
	}

	if condition, ok := conditions[conditionId]; ok {
		return condition
	}

	return nil
}

func GetAllConditionIds() []int {

	var results []int = make([]int, 0, len(conditions))
	for _, condition := range conditions {
		results = append(results, condition.ConditionId)
	}

	return results
}

// Searches for conditions whose name contains text and returns their Ids
func SearchConditions(searchTerm string) []int {

	searchTerm = strings.TrimSpace(strings.ToLower(searchTerm))

	var results []int = make([]int, 0, 2)

	for _, condition := range conditions {
		if strings.Contains(strings.ToLower(condition.Name), searchTerm) {
			results = append(results, condition.ConditionId)
		} else if strings.Contains(strings.ToLower(condition.Description), searchTerm) {
			results = append(results, condition.ConditionId)
		}
	}

	return results
}

// Presumably to ensure the datafile hasn't messed something up.
func (b *ConditionSpec) Id() int {
	return b.ConditionId
}

// Presumably to ensure the datafile hasn't messed something up.
func (b *ConditionSpec) Validate() error {

	// Validate YAML text tokens. An unknown token fails the load rather than
	// warning (messaging arc M4a): the loader turns this error into a boot
	// panic, so a typo cannot reach a player as raw text. Ambient stores
	// (weather, gossip, tips) keep warning until M4b sets the two-tier policy.
	for _, text := range []string{
		b.StartUserText, b.StartRoomText,
		b.TriggerUserText, b.TriggerRoomText,
		b.EndUserText, b.EndRoomText,
	} {
		if w := textutil.ValidateTokens(text); len(w) > 0 {
			return fmt.Errorf("conditionId %d (%s): %s", b.ConditionId, b.Name, strings.Join(w, "; "))
		}
	}

	// A whitespace-only line would be sent as-is; refuse it at load.
	if err := b.validateNarration(); err != nil {
		return err
	}

	if err := b.validateEffects(); err != nil {
		return err
	}

	// Validate tick fields
	if b.TickPool != "" {
		switch b.TickPool {
		case "health", "stamina", "conviction":
			// valid
		default:
			return fmt.Errorf("conditionId %d (%s) has invalid tick_pool %q (must be health/stamina/conviction)", b.ConditionId, b.Name, b.TickPool)
		}
		if !b.TickFromMagnitude {
			if b.TickPercent == 0 {
				mudlog.Warn("ConditionSpec.Validate", "conditionId", b.ConditionId, "warning", "tick_pool set but tick_percent is 0")
			}
		}
	}

	// If this is the quit/meditating condition, override the trigger count
	if b.ConditionId == 0 {
		b.TriggerCount = int(configs.GetNetworkConfig().LogoutRounds)
	}

	// If TriggerRate is set, validate RoundInterval and TriggerCount
	if b.TriggerRate != "" {
		b.RoundInterval = int(validationCalculator.AddPeriod(b.TriggerRate) - validationRound)

		if b.TriggerCount < 1 {
			return fmt.Errorf("conditionId %d (%s) has a TriggersCount of < 1, must be at least 1", b.ConditionId, b.Name)
		}
		if b.RoundInterval < 1 {
			return fmt.Errorf("conditionId %d (%s) has a RoundInterval of < 1, must be at least 1. Is %s a valid time string?", b.ConditionId, b.Name, b.TriggerRate)
		}
	}

	// A stack's amount IS the applier's magnitude and a stack counts rounds,
	// so a stacking record must be tick_from_magnitude and tick every round.
	// Checked after RoundInterval is derived above; an empty triggerrate
	// leaves it 0 and is refused too.
	if b.IsStacking() {
		if !b.TickFromMagnitude {
			return fmt.Errorf("conditionId %d (%s) is stacking without tick_from_magnitude; a stack's amount is the applier's magnitude", b.ConditionId, b.Name)
		}
		if b.RoundInterval != 1 {
			return fmt.Errorf("conditionId %d (%s) is stacking with triggerrate %q; a stack counts rounds, so the record must tick every round", b.ConditionId, b.Name, b.TriggerRate)
		}
	}

	return nil
}

// ValidateLoadedFlags panics on the first loaded condition carrying a flag the
// engine does not declare, naming the id, name and flag. LoadDataFiles calls
// it, so a typo in a condition file fails the boot rather than loading silently and
// doing nothing; species.ValidateSpeciesConditionIds guards its data the same way.
// Ids are walked in order so the panic names the same condition every time.
func ValidateLoadedFlags() {
	ids := make([]int, 0, len(conditions))
	for id := range conditions {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if err := conditions[id].ValidateFlags(); err != nil {
			panic(err)
		}
	}
}

// ValidateFlags reports the first flag this spec carries that the engine does
// not declare. Compared exactly; nothing is normalised.
func (b *ConditionSpec) ValidateFlags() error {
	for _, f := range b.Flags {
		if !slices.Contains(AllFlags, f) {
			return fmt.Errorf("conditionId %d (%s) carries unknown flag %q; see conditions.AllFlags", b.ConditionId, b.Name, f)
		}
	}
	return nil
}

func (b *ConditionSpec) Filename() string {
	filename := util.ConvertForFilename(b.Name)
	return fmt.Sprintf("%d-%s.yaml", b.ConditionId, filename)
}

func (b *ConditionSpec) Filepath() string {
	return b.Filename()
}

// file self loads due to init()
func LoadDataFiles() {

	start := time.Now()

	dataPath := string(configs.GetFilePathsConfig().DataFiles) + `/conditions`
	tmpConditions, err := fileloader.LoadAllFlatFiles[int, *ConditionSpec](dataPath)
	if err != nil {
		panic(errors.Wrap(err, `filepath: `+dataPath))
	}

	for id, b := range tmpConditions {
		if b.Name != "" {
			casing.AssertCanonical(b.Name, "condition", fmt.Sprintf("%d", id))
		}
	}

	conditions = tmpConditions

	// A flag the engine does not declare is a typo that would load silently and
	// do nothing, the way the Cat's Eye Draught did. Fail the boot instead.
	ValidateLoadedFlags()

	mudlog.Info("conditionSpec.LoadDataFiles()", "loadedCount", len(conditions), "Time Taken", time.Since(start))
}

// HasSpec reports whether a condition id is defined. Mirrors mutations.HasSpec so
// cross-package validators can take it as an injected checker, which is how
// species.ValidateSpeciesConditionIds consumes it.
func HasSpec(conditionId int) bool {
	return GetConditionSpec(conditionId) != nil
}
