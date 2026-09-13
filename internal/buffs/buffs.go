package buffs

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

const (
	TriggersLeftExpired   = 0 // When it hits this number it will be pruned ASAP
	TriggersLeftUnlimited = 1000000000
)

type Buff struct {
	BuffId         int    // Which buff template does it refer to?
	Source         string `yaml:"source,omitempty"`         // Optional source identifier for where this buff originated. Example: spell, item, area
	OnStartWaiting bool   `yaml:"onstartwaiting,omitempty"` // Is the onstart event waiting to trigger?
	PermaBuff      bool   `yaml:"permabuff,omitempty"`      // Is this buff from a worn item or race?
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
}

func (b *Buff) StatMod(statName string) int {
	if b.Expired() {
		return 0
	}
	if buffInfo := GetBuffSpec(b.BuffId); buffInfo != nil {
		return buffInfo.StatMods.Get(statName)
	}
	return 0
}

func (b *Buff) Expired() bool {
	return b.TriggersLeft <= TriggersLeftExpired
}

// A list of applied buffs
type Buffs struct {
	List      []*Buff
	buffFlags map[Flag][]int // a map of buff flags to the index of the buff
	buffIds   map[int]int    // a map of a buffId to it position in buffList
}

func New() Buffs {
	return Buffs{
		List:      []*Buff{},
		buffFlags: make(map[Flag][]int),
		buffIds:   make(map[int]int),
	}
}

func (bs *Buffs) Validate(forceRebuild ...bool) {
	if bs.buffFlags == nil {
		bs.buffFlags = make(map[Flag][]int)
	}
	if bs.buffIds == nil {
		bs.buffIds = make(map[int]int)
	}

	if (len(bs.List) != len(bs.buffIds)) || (len(forceRebuild) > 0 && forceRebuild[0]) {
		// Rebuild
		bs.buffIds = make(map[int]int)
		bs.buffFlags = make(map[Flag][]int)

		for idx, b := range bs.List {
			bs.buffIds[b.BuffId] = idx
			bSpec := GetBuffSpec(b.BuffId)
			if bSpec == nil {
				mudlog.Warn("buffs.Validate()", "buffId", b.BuffId, "error", "invalid character buffId")
				continue
			}
			for _, flag := range bSpec.Flags {
				if _, ok := bs.buffFlags[flag]; !ok {
					bs.buffFlags[flag] = []int{}
				}
				bs.buffFlags[flag] = append(bs.buffFlags[flag], idx)
			}
		}
	}
}

func (bs *Buffs) StatMod(statName string) int {
	buffAmt := 0
	for _, b := range bs.List {
		buffAmt += b.StatMod(statName)
	}
	return buffAmt
}

func (bs *Buff) Name() string {
	if sp := GetBuffSpec(bs.BuffId); sp != nil {
		return sp.Name
	}
	return ""
}

func (bs *Buffs) RemoveBuff(buffId int) bool {
	if index, ok := bs.buffIds[buffId]; ok {
		bs.List[index].TriggersLeft = TriggersLeftExpired
		return true
	}
	return false
}

func (bs *Buffs) TriggersLeft(buffId int) int {
	if idx, ok := bs.buffIds[buffId]; ok {
		return bs.List[idx].TriggersLeft
	}
	return 0
}

func (bs *Buffs) GetBuffIdsWithFlag(action Flag) []int {
	buffIds := []int{}
	for _, idx := range bs.buffFlags[action] {
		buffIds = append(buffIds, bs.List[idx].BuffId)
	}
	return buffIds
}

func (bs *Buffs) HasFlag(action Flag, expire bool) bool {

	if action != All {
		if _, ok := bs.buffFlags[action]; !ok {
			return false
		}
	}

	found := false
	for index, b := range bs.List {
		if b.Expired() {
			continue
		}

		// Determine whether this buff matches the requested flag.
		// action == All matches EVERY non-expired buff, including buffs
		// that declare no flags at all — e.g. pure regen potion buffs
		// like Healing Salve (buff 54). The old per-flag loop never ran
		// for a flagless buff, so CancelBuffsWithFlag(buffs.All) on death
		// silently left those buffs active.
		matches := action == All
		if !matches {
			// A save can carry a buff id whose spec is gone, and Validate indexes
			// it anyway, so the lookup can come back nil. ProgressMult guards the
			// same way.
			spec := GetBuffSpec(b.BuffId)
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

		// Expire mode: mark/remove this buff and keep scanning so every
		// matching buff is expired (required for action == All).
		// Buff zero is special, and if force cancelled, it is removed
		// from the list outright.
		if b.BuffId == 0 {
			bs.List = append(bs.List[:index], bs.List[index+1:]...)
		} else {
			b.TriggersLeft = TriggersLeftExpired
			bs.List[index] = b
		}
	}

	return found
}

// defaultProgressMult is what a buff carrying skill-progress or mutation-rate
// is worth when it declares no progress_mult of its own. It is the literal the
// two consuming call sites hardcoded before a buff could say otherwise, kept
// here so every existing buff behaves exactly as it did.
const defaultProgressMult = 2.0

// ProgressMult reports how much the held buffs carrying flag quicken the
// progression that flag gates: skill progression for SkillProgress, mutation
// progress for MutationRate. It returns 1.0 when no held buff carries the
// flag, so a caller can multiply unconditionally.
//
// A flagged buff with no progress_mult contributes defaultProgressMult. Held
// flagged buffs do not stack; the strongest value wins.
func (bs *Buffs) ProgressMult(flag Flag) float64 {

	// Same fast negative as HasFlag: the flag index is only ever a filter,
	// since it keeps entries for buffs that have since expired.
	if flag != All {
		if _, ok := bs.buffFlags[flag]; !ok {
			return 1.0
		}
	}

	mult := 1.0
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		spec := GetBuffSpec(b.BuffId)
		if spec == nil {
			continue
		}
		if flag != All && !slices.Contains(spec.Flags, flag) {
			continue
		}
		buffMult := defaultProgressMult
		if spec.ProgressMult > 0 {
			buffMult = spec.ProgressMult
		}
		if buffMult > mult {
			mult = buffMult
		}
	}

	return mult
}

func (bs *Buffs) HasBuff(buffId int) bool {
	if _, ok := bs.buffIds[buffId]; ok {
		return true
	}
	return false
}

func (bs *Buffs) Started(buffId int) {
	if idx, ok := bs.buffIds[buffId]; ok {
		bs.List[idx].OnStartWaiting = false
	}
}

// AddBuffScaled adds a buff with its duration multiplied by durationMult.
func (bs *Buffs) AddBuffScaled(buffId int, durationMult float64) bool {
	if buffInfo := GetBuffSpec(buffId); buffInfo != nil {

		// Poison immunity (Stone Stomach): a poison-flagged buff is refused
		// while the holder is immune. Checked here so every application path,
		// event or direct, honours it. Silent: the immunity's own start line
		// already told the player.
		if slices.Contains(buffInfo.Flags, Poison) && bs.HasFlag(PoisonImmunity, false) {
			return false
		}

		triggers := int(float64(buffInfo.TriggerCount) * durationMult)
		if triggers < 1 {
			triggers = 1
		}
		newBuff := Buff{
			BuffId:       buffInfo.BuffId,
			RoundCounter: 0,
			PermaBuff:    false,
			TriggersLeft: triggers,
		}

		if idx, ok := bs.buffIds[buffId]; ok {
			bs.List[idx].TriggersLeft = newBuff.TriggersLeft
			bs.List[idx].RoundCounter = 0
			bs.List[idx].PermaBuff = newBuff.PermaBuff
			return true
		}

		bs.List = append(bs.List, &newBuff)
		listIndex := len(bs.List) - 1
		bs.buffIds[buffId] = listIndex
		for _, flag := range buffInfo.Flags {
			if _, ok := bs.buffFlags[flag]; !ok {
				bs.buffFlags[flag] = []int{}
			}
			bs.buffFlags[flag] = append(bs.buffFlags[flag], listIndex)
		}
		return true
	}
	return false
}

// AddBuffMagnitude applies a record for an EXACT trigger count with a
// per-instance magnitude. It is the writer door for every record that used to
// be a combat condition. triggers 0 means the spec's own triggercount. A held
// record of the same id is refreshed and its magnitude, triggers and tick
// snapshot overwritten, which is what AddCondition did. Returns false when
// refused (poison immunity) or unknown.
//
// triggers is the exact trigger count, not a duration in rounds: for a
// one-round-interval record the two coincide, but the three-round-interval
// dot and bleed records need buffs.TickTriggers to convert a rounds-literal
// duration into the trigger count this parameter expects. It is an int on
// purpose: AddBuffScaled truncates float64(count) * mult, and 3.3 * 10 is
// 32.999... in binary, so a multiplier would shorten some durations by a
// round. The former conditions all computed an integer.
func (bs *Buffs) AddBuffMagnitude(buffId int, triggers int, magnitude float64) bool {
	if !bs.AddBuffScaled(buffId, 1.0) {
		return false
	}
	idx, ok := bs.buffIds[buffId]
	if !ok {
		return false
	}
	if triggers > 0 {
		bs.List[idx].TriggersLeft = triggers
	}
	bs.List[idx].Magnitude = magnitude
	if spec := GetBuffSpec(buffId); spec != nil && spec.TickFromMagnitude {
		// The magnitude IS the signed per-round amount: negative harms.
		// A non-zero magnitude that truncates to zero (e.g. -0.5) is floored
		// to 1 in its sign instead: a zero snapshot is unrecoverable, since
		// the round tick's fallback recomputes from TickPercent, which
		// validateEffects forces to 0 on a tick_from_magnitude record, so
		// ComputeTickAmount would return 0 and the record would tick for
		// nothing forever. Mirrors the old poison/bleed hook's clamp.
		amt := int(magnitude)
		if amt == 0 && magnitude != 0 {
			if magnitude < 0 {
				amt = -1
			} else {
				amt = 1
			}
		}
		bs.List[idx].TickAmount = amt
	}
	return true
}

// RefreshBuff tops a held buff's remaining triggers back up to the spec's
// TriggerCount and touches nothing else: RoundCounter keeps its cadence
// (AddBuff resets it, which would starve any buff whose RoundInterval is
// above one), PermaBuff and TickAmount are left alone. A buff already
// permanent (TriggersLeftUnlimited) is left as-is rather than clamped down
// to a finite TriggerCount. Returns false when the buff is not held or has
// no live spec (a save can carry a dead id: Validate indexes it into
// buffIds before checking whether GetBuffSpec finds anything, so "held"
// does not imply a spec exists). Room mutators use this to keep a buff
// alive for the whole visit without re-narrating it.
func (bs *Buffs) RefreshBuff(buffId int) bool {
	idx, ok := bs.buffIds[buffId]
	if !ok {
		return false
	}

	buffInfo := GetBuffSpec(buffId)
	if buffInfo == nil {
		return false
	}

	if bs.List[idx].PermaBuff {
		return true
	}

	bs.List[idx].TriggersLeft = buffInfo.TriggerCount
	return true
}

func (bs *Buffs) AddBuff(buffId int, isPermanent bool) bool {
	if buffInfo := GetBuffSpec(buffId); buffInfo != nil {

		// Poison immunity (Stone Stomach): a poison-flagged buff is refused
		// while the holder is immune. Checked here so every application path,
		// event or direct, honours it. Silent: the immunity's own start line
		// already told the player.
		if slices.Contains(buffInfo.Flags, Poison) && bs.HasFlag(PoisonImmunity, false) {
			return false
		}

		newBuff := Buff{
			BuffId:       buffInfo.BuffId,
			RoundCounter: 0,
			PermaBuff:    false,
			TriggersLeft: buffInfo.TriggerCount,
		}

		if isPermanent {
			newBuff.TriggersLeft = TriggersLeftUnlimited
			newBuff.PermaBuff = true
		}

		if idx, ok := bs.buffIds[buffId]; ok {
			bs.List[idx].TriggersLeft = newBuff.TriggersLeft
			bs.List[idx].RoundCounter = 0
			bs.List[idx].PermaBuff = newBuff.PermaBuff
			return true
		}

		bs.List = append(bs.List, &newBuff)
		listIndex := len(bs.List) - 1
		bs.buffIds[buffId] = listIndex
		for _, flag := range buffInfo.Flags {
			if _, ok := bs.buffFlags[flag]; !ok {
				bs.buffFlags[flag] = []int{}
			}
			bs.buffFlags[flag] = append(bs.buffFlags[flag], listIndex)
		}

		return true
	}

	return false
}

// Returns what buffs were triggered
func (bs *Buffs) Trigger(buffId ...int) (triggeredBuffs []*Buff) {

	for idx, b := range bs.List {

		// Special case where 1 or more specific buffId's were expectred to trigger (ONLY!)
		// This might happen if a buff needs to trigger before a round begins
		if len(buffId) > 0 {
			for _, id := range buffId {
				if b.BuffId != id {
					continue
				}
			}
		}

		if buffInfo := GetBuffSpec(b.BuffId); buffInfo != nil {

			// Pure flag buffs (no triggerrate set in YAML) have
			// RoundInterval==0 and are never meant to tick — they're
			// just stat-mod + flag markers (e.g., Hidden #9). Skip
			// them in the trigger loop so the modulo below doesn't
			// divide by zero. They're removed by explicit
			// CancelBuff* paths (e.g., cancel-on-combat flag) or by
			// time-based duration if one is later authored.
			if buffInfo.RoundInterval < 1 {
				continue
			}

			// If there's no more life left to it, prune it
			// We do this first so that it's the first thing that happens AFTER a full round has already passed.
			if b.TriggersLeft > 0 {
				b.RoundCounter++
				if b.RoundCounter%buffInfo.RoundInterval == 0 {
					// It cannot be pruned unless it is triggered
					triggeredBuffs = append(triggeredBuffs, b)
					if b.TriggersLeft != TriggersLeftUnlimited {
						b.TriggersLeft--
					} else {
						// If unimited, reset the counter to prevent some future overflow
						b.RoundCounter = 0
					}
				}
				bs.List[idx] = b
			}

		}

	}

	return triggeredBuffs
}

func (bs *Buffs) GetBuffs(buffId ...int) []*Buff {
	retBuffs := []*Buff{}
	for _, b := range bs.List {
		if !b.Expired() {

			if len(buffId) > 0 {
				for _, id := range buffId {
					if b.BuffId != id {
						continue
					}
					retBuffs = append(retBuffs, b)
				}
			} else {
				retBuffs = append(retBuffs, b)
			}

		}
	}
	return retBuffs
}

func (bs *Buffs) Prune() (prunedBuffs []*Buff) {

	if len(bs.List) == 0 {
		return prunedBuffs
	}

	var prune bool = false
	var didPrune bool = false
	for i := len(bs.List) - 1; i >= 0; i-- {

		prune = false

		b := bs.List[i] // Get a ptr to the data within the slice

		buffInfo := GetBuffSpec(b.BuffId)

		if buffInfo == nil {
			prune = true
		} else {
			// If there's no more life left to it, prune it
			// We do this first so that it's the first thing that happens AFTER a full round has already passed.
			if b.Expired() {
				prune = true
			}
		}

		if prune {
			prunedBuffs = append(prunedBuffs, b)
			// remove the buff
			bs.List = append(bs.List[:i], bs.List[i+1:]...)
			didPrune = true
		}
	}

	// Since pruning occured, rebuild the lookups
	if didPrune {
		bs.Validate(true)
	}

	return prunedBuffs
}

// SetTickAmount sets the TickAmount on the most recently added buff with
// the given buffId. Called right after AddBuff to set the snapshot.
func (bs *Buffs) SetTickAmount(buffId int, amount int) {
	if idx, ok := bs.buffIds[buffId]; ok {
		bs.List[idx].TickAmount = amount
	}
}

func GetDurations(buff *Buff, spec *BuffSpec) (roundsLeft int, totalRounds int) {

	totalRounds = spec.TriggerCount * spec.RoundInterval

	return totalRounds - buff.RoundCounter, totalRounds
}
