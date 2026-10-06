package conditions

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

const (
	TriggersLeftExpired   = 0 // When it hits this number it will be pruned ASAP
	TriggersLeftUnlimited = 1000000000
)

type Condition struct {
	ConditionId    int    `yaml:"conditionid"`              // Which condition template does it refer to? The tag states the save key explicitly.
	Source         string `yaml:"source,omitempty"`         // Optional source identifier for where this condition originated. Example: spell, item, area
	OnStartWaiting bool   `yaml:"onstartwaiting,omitempty"` // Is the onstart event waiting to trigger?
	Permanent      bool   `yaml:"permanent,omitempty"`      // Is this condition from a worn item or race?
	// Need to instance track the following:
	RoundCounter int `yaml:"roundcounter,omitempty"` // How many rounds have passed. Triggers on (RoundCounter%RoundInterval == 0)
	TriggersLeft int `yaml:"triggersleft,omitempty"` // How many times it triggers
	TickAmount   int `yaml:"tickamount,omitempty"`   // Snapshot: computed at application time, applied each trigger

	// Magnitude is the per-instance strength the applier set. A spec effect
	// whose value is the word "magnitude" reads it; a tick_from_magnitude
	// record snapshots it into TickAmount. Zero means "no effect" for a
	// multiplier and nothing for a flat. A non-zero magnitude that truncates
	// to zero (e.g. -0.5) snapshots as 1 in its sign instead, since a zero
	// tick would be unrecoverable; a magnitude of exactly zero snapshots as
	// zero.
	Magnitude float64 `yaml:"magnitude,omitempty"`

	// Stacks holds a stacking record's applications, each with its own
	// timer. Empty for every other record. See stacks.go.
	Stacks []Stack `yaml:"stacks,omitempty"`

	// Light-source state (lighting plan 5a), meaningful only on a record whose
	// spec declares light_strength. See light.go. The worn-item refresh
	// re-adds a held record without touching these, so a trim survives an
	// unrelated equipment change.
	LightTrim   LightTrim `yaml:"lighttrim,omitempty"`
	LightOutput float64   `yaml:"lightoutput,omitempty"`
	Hooded      bool      `yaml:"hooded,omitempty"`
}

func (b *Condition) StatMod(statName string) int {
	if b.Expired() {
		return 0
	}
	if conditionInfo := GetConditionSpec(b.ConditionId); conditionInfo != nil {
		return conditionInfo.StatMods.Get(statName)
	}
	return 0
}

func (b *Condition) Expired() bool {
	return b.TriggersLeft <= TriggersLeftExpired
}

// expire marks a held record expired for the prune pass and clears its
// stacks in the same step. Every internal path that expires a still-held
// record (RemoveCondition, HasFlag's expire branch, tickStacks) must go through
// this rather than setting TriggersLeft directly: AddCondition, AddConditionScaled and
// RefreshCondition can all revive an expired, unpruned record, and one that kept
// its old stacks would come back to life with them still live.
func (b *Condition) expire() {
	b.TriggersLeft = TriggersLeftExpired
	b.Stacks = nil
}

// A list of applied conditions
type Conditions struct {
	List           []*Condition
	conditionFlags map[Flag][]int // a map of condition flags to the index of the condition
	conditionIds   map[int]int    // a map of a conditionId to it position in conditionList
}

func New() Conditions {
	return Conditions{
		List:           []*Condition{},
		conditionFlags: make(map[Flag][]int),
		conditionIds:   make(map[int]int),
	}
}

func (bs *Conditions) Validate(forceRebuild ...bool) {
	if bs.conditionFlags == nil {
		bs.conditionFlags = make(map[Flag][]int)
	}
	if bs.conditionIds == nil {
		bs.conditionIds = make(map[int]int)
	}

	if (len(bs.List) != len(bs.conditionIds)) || (len(forceRebuild) > 0 && forceRebuild[0]) {
		// Rebuild
		bs.conditionIds = make(map[int]int)
		bs.conditionFlags = make(map[Flag][]int)

		for idx, b := range bs.List {
			bs.conditionIds[b.ConditionId] = idx
			bSpec := GetConditionSpec(b.ConditionId)
			if bSpec == nil {
				mudlog.Warn("conditions.Validate()", "conditionId", b.ConditionId, "error", "invalid character conditionId")
				continue
			}
			for _, flag := range bSpec.Flags {
				if _, ok := bs.conditionFlags[flag]; !ok {
					bs.conditionFlags[flag] = []int{}
				}
				bs.conditionFlags[flag] = append(bs.conditionFlags[flag], idx)
			}
		}
	}
}

func (bs *Conditions) StatMod(statName string) int {
	conditionAmt := 0
	for _, b := range bs.List {
		conditionAmt += b.StatMod(statName)
	}
	return conditionAmt
}

func (bs *Condition) Name() string {
	if sp := GetConditionSpec(bs.ConditionId); sp != nil {
		return sp.Name
	}
	return ""
}

func (bs *Conditions) RemoveCondition(conditionId int) bool {
	if index, ok := bs.conditionIds[conditionId]; ok {
		bs.List[index].expire()
		return true
	}
	return false
}

func (bs *Conditions) TriggersLeft(conditionId int) int {
	if idx, ok := bs.conditionIds[conditionId]; ok {
		return bs.List[idx].TriggersLeft
	}
	return 0
}

func (bs *Conditions) GetConditionIdsWithFlag(action Flag) []int {
	conditionIds := []int{}
	for _, idx := range bs.conditionFlags[action] {
		conditionIds = append(conditionIds, bs.List[idx].ConditionId)
	}
	return conditionIds
}

func (bs *Conditions) HasFlag(action Flag, expire bool) bool {

	if action != All {
		if _, ok := bs.conditionFlags[action]; !ok {
			return false
		}
	}

	found := false
	for index, b := range bs.List {
		if b.Expired() {
			continue
		}

		// Determine whether this condition matches the requested flag.
		// action == All matches EVERY non-expired condition, including conditions
		// that declare no flags at all — e.g. pure regen potion conditions
		// like Healing Salve (condition 54). The old per-flag loop never ran
		// for a flagless condition, so CancelConditionsWithFlag(conditions.All) on death
		// silently left those conditions active.
		matches := action == All
		if !matches {
			// A save can carry a condition id whose spec is gone, and Validate indexes
			// it anyway, so the lookup can come back nil. ProgressMult guards the
			// same way.
			spec := GetConditionSpec(b.ConditionId)
			if spec == nil {
				continue
			}
			matches = slices.Contains(spec.Flags, action)
		}
		if !matches {
			continue
		}

		found = true

		// If not expiring, the first match is enough.
		if !expire {
			return found
		}

		// Expire mode: mark/remove this condition and keep scanning so every
		// matching condition is expired (required for action == All).
		// Condition zero is special, and if force cancelled, it is removed
		// from the list outright.
		if b.ConditionId == 0 {
			bs.List = append(bs.List[:index], bs.List[index+1:]...)
		} else {
			b.expire()
			bs.List[index] = b
		}
	}

	return found
}

// defaultProgressMult is what a condition carrying skill-progress or mutation-rate
// is worth when it declares no progress_mult of its own. It is the literal the
// two consuming call sites hardcoded before a condition could say otherwise, kept
// here so every existing condition behaves exactly as it did.
const defaultProgressMult = 2.0

// ProgressMult reports how much the held conditions carrying flag quicken the
// progression that flag gates: skill progression for SkillProgress, mutation
// progress for MutationRate. It returns 1.0 when no held condition carries the
// flag, so a caller can multiply unconditionally.
//
// A flagged condition with no progress_mult contributes defaultProgressMult. Held
// flagged conditions do not stack; the strongest value wins.
func (bs *Conditions) ProgressMult(flag Flag) float64 {

	// Same fast negative as HasFlag: the flag index is only ever a filter,
	// since it keeps entries for conditions that have since expired.
	if flag != All {
		if _, ok := bs.conditionFlags[flag]; !ok {
			return 1.0
		}
	}

	mult := 1.0
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		spec := GetConditionSpec(b.ConditionId)
		if spec == nil {
			continue
		}
		if flag != All && !slices.Contains(spec.Flags, flag) {
			continue
		}
		conditionMult := defaultProgressMult
		if spec.ProgressMult > 0 {
			conditionMult = spec.ProgressMult
		}
		if conditionMult > mult {
			mult = conditionMult
		}
	}

	return mult
}

func (bs *Conditions) HasCondition(conditionId int) bool {
	if _, ok := bs.conditionIds[conditionId]; ok {
		return true
	}
	return false
}

func (bs *Conditions) Started(conditionId int) {
	if idx, ok := bs.conditionIds[conditionId]; ok {
		bs.List[idx].OnStartWaiting = false
	}
}

// AddConditionScaled adds a condition with its duration multiplied by durationMult. A
// stacking record can only be added through AddConditionMagnitude, because a
// stack needs its own rounds and amount that this call has no room to carry;
// a stacking spec is refused rather than left to create a live record with
// no stacks.
func (bs *Conditions) AddConditionScaled(conditionId int, durationMult float64) bool {
	if spec := GetConditionSpec(conditionId); spec != nil && spec.IsStacking() {
		return false
	}
	return bs.addConditionScaled(conditionId, durationMult)
}

// addConditionScaled is the writer AddConditionScaled and the non-stacking branch of
// AddConditionMagnitude share. addStack also calls it, once per new stack, to
// create or touch the record before it appends that stack, which is why this
// unexported form does not itself refuse a stacking spec: AddConditionScaled's
// exported wrapper is where that refusal belongs.
func (bs *Conditions) addConditionScaled(conditionId int, durationMult float64) bool {
	if conditionInfo := GetConditionSpec(conditionId); conditionInfo != nil {

		// Poison immunity (Stone Stomach): a poison-flagged condition is refused
		// while the holder is immune. Checked here so every application path,
		// event or direct, honours it. Silent: the immunity's own start line
		// already told the player.
		if slices.Contains(conditionInfo.Flags, Poison) && bs.HasFlag(PoisonImmunity, false) {
			return false
		}

		triggers := int(float64(conditionInfo.TriggerCount) * durationMult)
		if triggers < 1 {
			triggers = 1
		}
		newCondition := Condition{
			ConditionId:  conditionInfo.ConditionId,
			RoundCounter: 0,
			Permanent:    false,
			TriggersLeft: triggers,
		}

		if idx, ok := bs.conditionIds[conditionId]; ok {
			bs.List[idx].TriggersLeft = newCondition.TriggersLeft
			bs.List[idx].RoundCounter = 0
			bs.List[idx].Permanent = newCondition.Permanent
			return true
		}

		bs.List = append(bs.List, &newCondition)
		listIndex := len(bs.List) - 1
		bs.conditionIds[conditionId] = listIndex
		for _, flag := range conditionInfo.Flags {
			if _, ok := bs.conditionFlags[flag]; !ok {
				bs.conditionFlags[flag] = []int{}
			}
			bs.conditionFlags[flag] = append(bs.conditionFlags[flag], listIndex)
		}
		return true
	}
	return false
}

// AddConditionMagnitude applies a record for an EXACT trigger count with a
// per-instance magnitude. It is the writer door for every record that used to
// be a combat condition. triggers 0 means the spec's own triggercount. A held
// record of the same id is refreshed and its magnitude, triggers and tick
// snapshot overwritten, which is what the old condition add did. Returns false when
// refused (poison immunity) or unknown.
//
// triggers is the exact trigger count, not a duration in rounds. Every record
// that goes through this door today ticks once a round, so the trigger count
// is the rounds. It is an int on
// purpose: AddConditionScaled truncates float64(count) * mult, and 3.3 * 10 is
// 32.999... in binary, so a multiplier would shorten some durations by a
// round. The former conditions all computed an integer.
//
// A stacking record (see the Stacking flag) appends a stack instead of
// overwriting; triggers is then that stack's rounds.
func (bs *Conditions) AddConditionMagnitude(conditionId int, triggers int, magnitude float64) bool {
	if spec := GetConditionSpec(conditionId); spec != nil && spec.IsStacking() {
		return bs.addStack(spec, triggers, magnitude)
	}
	if !bs.addConditionScaled(conditionId, 1.0) {
		return false
	}
	idx, ok := bs.conditionIds[conditionId]
	if !ok {
		return false
	}
	if triggers > 0 {
		bs.List[idx].TriggersLeft = triggers
	}
	bs.List[idx].Magnitude = magnitude
	if spec := GetConditionSpec(conditionId); spec != nil {
		if spec.TickFromMagnitude {
			// The magnitude IS the signed per-round amount; see tickAmountFor.
			bs.List[idx].TickAmount = tickAmountFor(magnitude)
		}
		if spec.IsLightSource() || spec.IsDarknessSource() {
			// A fresh magnitude is a fresh cast: full strength, hood open.
			bs.List[idx].ResetLight()
		}
	}
	return true
}

// RefreshCondition tops a held condition's remaining triggers back up to the spec's
// TriggerCount and touches nothing else: RoundCounter keeps its cadence
// (AddCondition resets it, which would starve any condition whose RoundInterval is
// above one), Permanent and TickAmount are left alone. A condition already
// permanent (TriggersLeftUnlimited) is left as-is rather than clamped down
// to a finite TriggerCount. Returns false when the condition is not held or has
// no live spec (a save can carry a dead id: Validate indexes it into
// conditionIds before checking whether GetConditionSpec finds anything, so "held"
// does not imply a spec exists). Room mutators use this to keep a condition
// alive for the whole visit without re-narrating it.
//
// A stacking record can only be added through AddConditionMagnitude, because a
// stack needs its own rounds and amount that this call has no room to carry;
// a stacking spec is refused rather than topped up to the spec's single
// TriggerCount, which would misreport a live record's duration or, on an
// expired-but-unpruned one with no stacks, revive it to tick for nothing.
func (bs *Conditions) RefreshCondition(conditionId int) bool {
	idx, ok := bs.conditionIds[conditionId]
	if !ok {
		return false
	}

	conditionInfo := GetConditionSpec(conditionId)
	if conditionInfo == nil {
		return false
	}

	if conditionInfo.IsStacking() {
		return false
	}

	if bs.List[idx].Permanent {
		return true
	}

	bs.List[idx].TriggersLeft = conditionInfo.TriggerCount
	return true
}

// AddCondition applies a record for the spec's own trigger count, or unlimited
// when isPermanent. A stacking record can only be added through
// AddConditionMagnitude, because a stack needs its own rounds and amount that this
// call has no room to carry; a stacking spec is refused rather than left to
// create a live record with no stacks.
func (bs *Conditions) AddCondition(conditionId int, isPermanent bool) bool {
	if conditionInfo := GetConditionSpec(conditionId); conditionInfo != nil {

		if conditionInfo.IsStacking() {
			return false
		}

		// Poison immunity (Stone Stomach): a poison-flagged condition is refused
		// while the holder is immune. Checked here so every application path,
		// event or direct, honours it. Silent: the immunity's own start line
		// already told the player.
		if slices.Contains(conditionInfo.Flags, Poison) && bs.HasFlag(PoisonImmunity, false) {
			return false
		}

		newCondition := Condition{
			ConditionId:  conditionInfo.ConditionId,
			RoundCounter: 0,
			Permanent:    false,
			TriggersLeft: conditionInfo.TriggerCount,
		}

		if isPermanent {
			newCondition.TriggersLeft = TriggersLeftUnlimited
			newCondition.Permanent = true
		}

		if idx, ok := bs.conditionIds[conditionId]; ok {
			bs.List[idx].TriggersLeft = newCondition.TriggersLeft
			bs.List[idx].RoundCounter = 0
			bs.List[idx].Permanent = newCondition.Permanent
			return true
		}

		bs.List = append(bs.List, &newCondition)
		listIndex := len(bs.List) - 1
		bs.conditionIds[conditionId] = listIndex
		for _, flag := range conditionInfo.Flags {
			if _, ok := bs.conditionFlags[flag]; !ok {
				bs.conditionFlags[flag] = []int{}
			}
			bs.conditionFlags[flag] = append(bs.conditionFlags[flag], listIndex)
		}

		return true
	}

	return false
}

// Returns what conditions were triggered
func (bs *Conditions) Trigger(conditionId ...int) (triggeredConditions []*Condition) {

	for idx, b := range bs.List {

		// Special case where 1 or more specific conditionId's were expectred to trigger (ONLY!)
		// This might happen if a condition needs to trigger before a round begins
		if len(conditionId) > 0 {
			for _, id := range conditionId {
				if b.ConditionId != id {
					continue
				}
			}
		}

		if conditionInfo := GetConditionSpec(b.ConditionId); conditionInfo != nil {

			// Pure flag conditions (no triggerrate set in YAML) have
			// RoundInterval==0 and are never meant to tick — they're
			// just stat-mod + flag markers (e.g., Hidden #9). Skip
			// them in the trigger loop so the modulo below doesn't
			// divide by zero. They're removed by explicit
			// CancelCondition* paths (e.g., cancel-on-combat flag) or by
			// time-based duration if one is later authored.
			if conditionInfo.RoundInterval < 1 {
				continue
			}

			// If there's no more life left to it, prune it
			// We do this first so that it's the first thing that happens AFTER a full round has already passed.
			if b.TriggersLeft > 0 {
				b.RoundCounter++
				if b.RoundCounter%conditionInfo.RoundInterval == 0 {
					if conditionInfo.IsStacking() {
						// A stacking record ticks its stacks and derives
						// TriggersLeft from them; see tickStacks.
						if b.tickStacks() {
							triggeredConditions = append(triggeredConditions, b)
						}
					} else {
						// It cannot be pruned unless it is triggered
						triggeredConditions = append(triggeredConditions, b)
						if b.TriggersLeft != TriggersLeftUnlimited {
							b.TriggersLeft--
						} else {
							// If unimited, reset the counter to prevent some future overflow
							b.RoundCounter = 0
						}
					}
				}
				bs.List[idx] = b
			}

		}

	}

	return triggeredConditions
}

// ExpiresOnNextTrigger reports whether the next Trigger will leave this record
// expired. It is Trigger's own arithmetic asked one round early, and must
// change whenever Trigger does: a record with no spec or no interval is never
// ticked, the round counter has to land on the interval, a non-stacking
// record runs out on its last trigger, and a stacking one when tickStacks
// drops its last stack. The round ticks use it to snapshot the room just
// before a light or darkness runs out (#220). An unlimited record never
// expires here.
func (b *Condition) ExpiresOnNextTrigger(spec *ConditionSpec) bool {
	if spec == nil || spec.RoundInterval < 1 || b.TriggersLeft <= 0 {
		return false
	}
	if (b.RoundCounter+1)%spec.RoundInterval != 0 {
		return false
	}
	if spec.IsStacking() {
		for _, s := range b.Stacks {
			if s.RoundsLeft > 1 {
				return false
			}
		}
		return true
	}
	return b.TriggersLeft == 1
}

func (bs *Conditions) GetConditions(conditionId ...int) []*Condition {
	retConditions := []*Condition{}
	for _, b := range bs.List {
		if !b.Expired() {

			if len(conditionId) > 0 {
				for _, id := range conditionId {
					if b.ConditionId != id {
						continue
					}
					retConditions = append(retConditions, b)
				}
			} else {
				retConditions = append(retConditions, b)
			}

		}
	}
	return retConditions
}

func (bs *Conditions) Prune() (prunedConditions []*Condition) {

	if len(bs.List) == 0 {
		return prunedConditions
	}

	var prune bool
	var didPrune bool = false
	for i := len(bs.List) - 1; i >= 0; i-- {

		prune = false

		b := bs.List[i] // Get a ptr to the data within the slice

		conditionInfo := GetConditionSpec(b.ConditionId)

		if conditionInfo == nil {
			prune = true
		} else {
			// If there's no more life left to it, prune it
			// We do this first so that it's the first thing that happens AFTER a full round has already passed.
			if b.Expired() {
				prune = true
			}
		}

		if prune {
			prunedConditions = append(prunedConditions, b)
			// remove the condition
			bs.List = append(bs.List[:i], bs.List[i+1:]...)
			didPrune = true
		}
	}

	// Since pruning occured, rebuild the lookups
	if didPrune {
		bs.Validate(true)
	}

	return prunedConditions
}

// SetTickAmount sets the TickAmount on the most recently added condition with
// the given conditionId. Called right after AddCondition to set the snapshot.
func (bs *Conditions) SetTickAmount(conditionId int, amount int) {
	if idx, ok := bs.conditionIds[conditionId]; ok {
		bs.List[idx].TickAmount = amount
	}
}

// GetDurations reports how many rounds this INSTANCE has left and how many it
// had in total, both in rounds rather than triggers.
//
// roundsLeft reads the instance, not the spec. It used to be
// spec.TriggerCount*spec.RoundInterval - condition.RoundCounter, which is only
// right for a record added at its spec default: every AddConditionMagnitude and
// AddConditionScaled producer sets an exact trigger count, and a record added with
// 2 of a spec's 10 triggers displayed 30 rounds remaining instead of 6.
// TriggersLeft*RoundInterval is the whole rounds still owed; RoundCounter
// modulo RoundInterval is how far into the current interval the record
// already is.
//
// totalRounds keeps the spec figure so a bar has a stable scale, but never
// less than roundsLeft — a record refreshed above its spec default would
// otherwise report a remainder larger than its own total.
//
// An unlimited record (a permanent condition) keeps the old answer rather than a
// nine-digit one: its callers gate on Condition.Permanent and render "sustained".
func GetDurations(condition *Condition, spec *ConditionSpec) (roundsLeft int, totalRounds int) {

	if spec.RoundInterval < 1 {
		// A pure flag record never ticks, so it has no duration to report.
		return 0, 0
	}

	totalRounds = spec.TriggerCount * spec.RoundInterval

	if condition.TriggersLeft == TriggersLeftUnlimited {
		return totalRounds - condition.RoundCounter, totalRounds
	}

	roundsLeft = condition.TriggersLeft*spec.RoundInterval - condition.RoundCounter%spec.RoundInterval

	return roundsLeft, max(totalRounds, roundsLeft)
}
