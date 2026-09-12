# GoMud Buffs System Context

## Overview

The GoMud buffs system provides comprehensive temporary status effects for characters with support for stat modifications, behavioral flags, round-based triggers, and duration management. It features a dual-layer architecture with immutable buff specifications and mutable buff instances, supporting complex timing mechanics, permanent buffs, and sophisticated flag-based behavior modification.

## Architecture

The buffs system is built around several key components:

### Core Components

**Buff Specifications (BuffSpec):**
- Immutable blueprint definitions for all buff types
- YAML-based storage with automatic loading and validation
- Time-based trigger rate calculations with game time integration
- Stat modification definitions and behavioral flags
- Config-driven tick behaviors with stat scaling

**Buff Instances (Buff):**
- Runtime instances with unique state tracking
- Round-based trigger counters and expiration management
- Source tracking for buff origin identification
- Permanent buff support for equipment and racial effects
- Start event queuing for delayed activation

**Buffs Collection (Buffs):**
- Efficient collection management with flag indexing
- Fast lookup maps for buff IDs and flags
- Automatic validation and rebuilding of internal indexes
- Batch operations for triggering and pruning

## Key Features

### 1. **Comprehensive Flag System**
- Behavioral modification flags (combat, movement, fleeing restrictions)
- Death prevention and revival mechanics
- Equipment interaction flags (permanent gear, curse removal)
- Status effect flags (poison, accuracy, stealth, vision enhancement)
- Environmental interaction flags (water cancellation, light emission)

### 2. **Advanced Timing System**
- Round-based trigger intervals with game time integration
- Flexible trigger rates using time string parsing
- Unlimited duration support for permanent effects
- Precise expiration tracking and automatic cleanup
- Trigger counting with configurable limits

### 3. **Stat Modification Integration**
- Dynamic stat bonuses and penalties
- Cumulative effects from multiple buffs
- Integration with character stat system
- Racial and equipment stat modifications
- Combat effectiveness modifiers

## Buff Structure

### Buff Specification Structure
```go
type BuffSpec struct {
    BuffId        int               // Unique identifier
    Name          string            // Display name
    Description   string            // Description text
    Secret        bool              // Hidden from player view
    TriggerNow    bool              // Immediate trigger on application
    TriggerRate   string            // Time-based trigger frequency
    RoundInterval int               // Calculated round interval
    TriggerCount  int               // Total trigger limit
    StatMods      statmods.StatMods // Stat modifications
    Flags         []Flag            // Behavioral flags
}
```

### Buff Instance Structure
```go
type Buff struct {
    BuffId         int    // Reference to BuffSpec
    Source         string // Origin identifier (spell, item, area)
    OnStartWaiting bool   // Pending start event
    PermaBuff      bool   // Permanent buff flag
    RoundCounter   int    // Elapsed rounds
    TriggersLeft   int    // Remaining triggers
}
```

### Buffs Collection Structure
```go
type Buffs struct {
    List      []*Buff           // Active buff instances
    buffFlags map[Flag][]int    // Flag to buff index mapping
    buffIds   map[int]int       // BuffId to index mapping
}
```

## Flag System

### Behavioral Flags
```go
const (
    // Combat and Movement Restrictions
    NoCombat       Flag = "no-combat"        // Prevents combat initiation
    NoMovement     Flag = "no-go"            // Prevents movement
    NoFlee         Flag = "no-flee"          // Prevents fleeing combat
    
    // Cancellation Conditions
    CancelIfCombat Flag = "cancel-on-combat" // Removes buff when combat starts
    CancelOnAction Flag = "cancel-on-action" // Removes buff on any action
    CancelOnWater  Flag = "cancel-on-water"  // Removes buff in water
    
    // Death and Revival
    ReviveOnDeath  Flag = "revive-on-death"  // Prevents death once
    
    // Equipment Interaction
    PermaGear      Flag = "perma-gear"       // Equipment cannot be removed
    RemoveCurse    Flag = "remove-curse"     // Allows cursed item removal
    
    // Status Effects
    Poison         Flag = "poison"           // Harmful poison effect
    Drunk          Flag = "drunk"            // Intoxication effects
    Hidden         Flag = "hidden"           // Stealth/invisibility
    Accuracy       Flag = "accuracy"         // Enhanced hit chance
    Blink          Flag = "blink"            // Dodge enhancement
    
    // Sensory Enhancement
    EmitsLight     Flag = "lightsource"      // Provides illumination
    SuperHearing   Flag = "superhearing"     // Enhanced hearing
    NightVision    Flag = "nightvision"      // See in darkness
    SeeHidden      Flag = "see-hidden"       // Detect hidden entities
    SeeNouns       Flag = "see-nouns"        // Enhanced object identification
    
    // Environmental Status
    Warmed         Flag = "warmed"           // Temperature regulation
    Hydrated       Flag = "hydrated"         // Hydration status
    Thirsty        Flag = "thirsty"          // Dehydration status
)
```

### Progression Flags and `progress_mult`

Two flags are more than booleans the engine merely tests. `skill-progress` and
`mutation-rate` quicken skill progression and mutation progress while the buff
is held, and how much they quicken it is authored per buff through the optional
`progress_mult:` YAML key behind `BuffSpec.ProgressMult`. Read it with
`Buffs.ProgressMult(flag)` rather than by testing the flag: it returns 1.0 when
no held buff carries the flag, so a caller can multiply unconditionally, and a
held flagged buff that declares no `progress_mult` is worth 2.0, the default the
two consuming call sites (`internal/characters/progression.go` and
`internal/hooks/NewRound_UserRoundTick.go`) used to hardcode. Held flagged buffs
do not stack; the strongest value wins. So Savant's Infusion (buff 72) at 2.5
beats Essence of Growth (buff 71) at the default, and Chrysalis Catalyst (buff
74) at 3.0 beats Mutagen Brew (buff 73).

### Flags are validated at load (slice E, 2026-09-12)

`AllFlags` lists every declared `Flag`. `BuffSpec.ValidateFlags()` reports the
first flag a spec carries that is not in the list. `ValidateLoadedFlags()` walks
the loaded registry in id order and panics on the first offender, naming the
buff id, name and flag; `LoadDataFiles` calls it after the registry is live. It
is a named function precisely so a test can call it: no test loads world YAML,
so an inlined check had nothing red to prove it works. The root guard
`buff_flag_guard_test.go` walks the dogmud buff files and fails the build on
the same condition, and `TestAllFlagsNamesEveryDeclaredConstant` parses the
constants out of this package so the list cannot fall behind. Flags are
compared exactly, nothing is normalised: the Cat's Eye Draught shipped with
`night-vision` for `nightvision` and did nothing for weeks.
The call from `LoadDataFiles` itself is pinned by nothing (no test loads world
YAML, and a clean boot cannot detect a missing negative check); the function is
pinned by a direct panic test, and the root guard is the gate that blocks a
merge. The boot panic is defence in depth for a file edited by hand on prod.


`poison-immunity` (`PoisonImmunity`, Stone Stomach): while held, `AddBuff` and
`AddBuffScaled` refuse a spec carrying `poison`, and `Character.AddCondition`
refuses `ConditionPoisoned`. Refusal is silent. The three toxins (39 Venom, 40
Spore Toxin, 78 Toxic Cloud) and 75 Nausea carry `poison` since the same slice;
before it NO buff did, so `CancelBuffsWithFlag(Poison)` in Purge Affliction and
Cleansing Wave cancelled nothing and the `poisoned` adjective never showed.

A refusal is a refusal all the way out: `Character.AddBuff` returns an error,
`Buff_ApplyBuffs` returns on it before the start notice, `start_remove_buffs`,
`TrackBuffStarted` or `BuffsTriggered`, and `Character.AddCondition` returns a
`bool` the two spell dot sites test before narrating. `HasFlag` guards a nil
spec, since every add now asks it and a save can hold a dead buff id.

### Flag Usage Patterns

The sketch below is abridged and predates two fixes in the live body: `All`
matches a flagless buff, and the spec lookup is nil-guarded before `.Flags`.
Read `buffs.go` for the real thing.

```go
// Check for specific behavioral flags
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
        
        bSpec := GetBuffSpec(b.BuffId)
        for _, flag := range bSpec.Flags {
            if flag == action || action == All {
                found = true
                
                // Optionally expire the buff when checked
                if expire {
                    if b.BuffId == 0 { // Special buff 0 handling
                        bs.List = append(bs.List[:index], bs.List[index+1:]...)
                    } else {
                        b.TriggersLeft = TriggersLeftExpired
                        bs.List[index] = b
                    }
                    break
                }
                
                return found
            }
        }
    }
    
    return found
}

// Get all buff IDs with specific flag
func (bs *Buffs) GetBuffIdsWithFlag(action Flag) []int {
    buffIds := []int{}
    for _, idx := range bs.buffFlags[action] {
        buffIds = append(buffIds, bs.List[idx].BuffId)
    }
    return buffIds
}
```

## Timing and Trigger System

### Round-Based Triggers
```go
// Trigger buffs based on round intervals
func (bs *Buffs) Trigger(buffId ...int) (triggeredBuffs []*Buff) {
    for idx, b := range bs.List {
        // Handle specific buff triggering if requested
        if len(buffId) > 0 {
            found := false
            for _, id := range buffId {
                if b.BuffId == id {
                    found = true
                    break
                }
            }
            if !found {
                continue
            }
        }
        
        if buffInfo := GetBuffSpec(b.BuffId); buffInfo != nil {
            if b.TriggersLeft > 0 {
                b.RoundCounter++
                
                // Check if it's time to trigger
                if b.RoundCounter%buffInfo.RoundInterval == 0 {
                    triggeredBuffs = append(triggeredBuffs, b)
                    
                    // Decrement triggers unless unlimited
                    if b.TriggersLeft != TriggersLeftUnlimited {
                        b.TriggersLeft--
                    } else {
                        // Reset counter to prevent overflow
                        b.RoundCounter = 0
                    }
                }
                
                bs.List[idx] = b
            }
        }
    }
    
    return triggeredBuffs
}
```

### Duration Calculations
```go
// Calculate remaining and total duration
func GetDurations(buff *Buff, spec *BuffSpec) (roundsLeft int, totalRounds int) {
    totalRounds = spec.TriggerCount * spec.RoundInterval
    roundsLeft = totalRounds - buff.RoundCounter
    return roundsLeft, totalRounds
}

// Check if buff has expired
func (b *Buff) Expired() bool {
    return b.TriggersLeft <= TriggersLeftExpired
}
```

### Time String Processing
```go
// Validate and convert time strings to round intervals
func (b *BuffSpec) Validate() error {
    // Special handling for logout/meditation buff
    if b.BuffId == 0 {
        b.TriggerCount = int(configs.GetNetworkConfig().LogoutRounds)
    }
    
    // Convert time string to round interval using game time system
    b.RoundInterval = int(validationCalculator.AddPeriod(b.TriggerRate) - validationRound)
    
    if b.TriggerCount < 1 {
        return fmt.Errorf("buffId %d (%s) has TriggerCount < 1", b.BuffId, b.Name)
    }
    
    if b.RoundInterval < 1 {
        return fmt.Errorf("buffId %d (%s) has RoundInterval < 1. Is %s valid?", 
                         b.BuffId, b.Name, b.TriggerRate)
    }
    
    return nil
}
```

## Stat Modification System

### Individual Buff Stat Modifications
```go
// Get stat modification from single buff
func (b *Buff) StatMod(statName string) int {
    if b.Expired() {
        return 0
    }
    
    if buffInfo := GetBuffSpec(b.BuffId); buffInfo != nil {
        return buffInfo.StatMods.Get(statName)
    }
    
    return 0
}
```

### Cumulative Stat Modifications
```go
// Calculate total stat modification from all active buffs
func (bs *Buffs) StatMod(statName string) int {
    buffAmt := 0
    for _, b := range bs.List {
        buffAmt += b.StatMod(statName)
    }
    return buffAmt
}
```

### Buff Value Calculation
```go
// Calculate relative power/value of a buff for balance
func (b *BuffSpec) GetValue() int {
    val := 0
    
    // Sum absolute values of all stat modifications
    for _, v := range b.StatMods {
        val += int(math.Abs(float64(v)))
    }
    
    // Add value for frequency (more frequent = more valuable)
    freqVal := max(5-b.RoundInterval, 0)
    val += freqVal
    
    // Add value for flags (5 points per flag)
    val += len(b.Flags) * 5
    
    // Multiply by trigger count for total effect
    if b.TriggerCount > 0 {
        val *= b.TriggerCount
    }
    
    return val
}
```

## Buff Management Operations

### Adding Buffs
```go
// Add new buff or refresh existing buff
func (bs *Buffs) AddBuff(buffId int, isPermanent bool) bool {
    if buffInfo := GetBuffSpec(buffId); buffInfo != nil {
        newBuff := Buff{
            BuffId:       buffInfo.BuffId,
            RoundCounter: 0,
            PermaBuff:    false,
            TriggersLeft: buffInfo.TriggerCount,
        }
        
        // Handle permanent buffs (from equipment/race)
        if isPermanent {
            newBuff.TriggersLeft = TriggersLeftUnlimited
            newBuff.PermaBuff = true
        }
        
        // Check if buff already exists
        if idx, ok := bs.buffIds[buffId]; ok {
            // Refresh existing buff
            bs.List[idx].TriggersLeft = newBuff.TriggersLeft
            bs.List[idx].PermaBuff = newBuff.PermaBuff
            return true
        }
        
        // Add new buff
        bs.List = append(bs.List, &newBuff)
        listIndex := len(bs.List) - 1
        bs.buffIds[buffId] = listIndex
        
        // Update flag indexes
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
```

### Removing Buffs
```go
// Remove specific buff by ID
func (bs *Buffs) RemoveBuff(buffId int) bool {
    if index, ok := bs.buffIds[buffId]; ok {
        bs.List[index].TriggersLeft = TriggersLeftExpired
        return true
    }
    return false
}

// Mark buff as started (no longer waiting for start event)
func (bs *Buffs) Started(buffId int) {
    if idx, ok := bs.buffIds[buffId]; ok {
        bs.List[idx].OnStartWaiting = false
    }
}
```

### Pruning Expired Buffs
```go
// Remove all expired buffs and rebuild indexes
func (bs *Buffs) Prune() (prunedBuffs []*Buff) {
    if len(bs.List) == 0 {
        return prunedBuffs
    }
    
    didPrune := false
    
    // Iterate backwards to safely remove items
    for i := len(bs.List) - 1; i >= 0; i-- {
        b := bs.List[i]
        prune := false
        
        buffInfo := GetBuffSpec(b.BuffId)
        if buffInfo == nil || b.Expired() {
            prune = true
        }
        
        if prune {
            prunedBuffs = append(prunedBuffs, b)
            bs.List = append(bs.List[:i], bs.List[i+1:]...)
            didPrune = true
        }
    }
    
    // Rebuild lookup indexes if any buffs were pruned
    if didPrune {
        bs.Validate(true)
    }
    
    return prunedBuffs
}
```

## Collection Validation and Indexing

### Index Management
```go
// Validate and rebuild internal indexes
func (bs *Buffs) Validate(forceRebuild ...bool) {
    if bs.buffFlags == nil {
        bs.buffFlags = make(map[Flag][]int)
    }
    if bs.buffIds == nil {
        bs.buffIds = make(map[int]int)
    }
    
    // Rebuild if size mismatch or forced
    if (len(bs.List) != len(bs.buffIds)) || (len(forceRebuild) > 0 && forceRebuild[0]) {
        bs.buffIds = make(map[int]int)
        bs.buffFlags = make(map[Flag][]int)
        
        // Rebuild all indexes
        for idx, b := range bs.List {
            bs.buffIds[b.BuffId] = idx
            
            bSpec := GetBuffSpec(b.BuffId)
            if bSpec == nil {
                mudlog.Warn("buffs.Validate()", "buffId", b.BuffId, "error", "invalid buffId")
                continue
            }
            
            // Index all flags for this buff
            for _, flag := range bSpec.Flags {
                if _, ok := bs.buffFlags[flag]; !ok {
                    bs.buffFlags[flag] = []int{}
                }
                bs.buffFlags[flag] = append(bs.buffFlags[flag], idx)
            }
        }
    }
}
```

### Query Operations
```go
// Check if specific buff exists
func (bs *Buffs) HasBuff(buffId int) bool {
    if _, ok := bs.buffIds[buffId]; ok {
        return true
    }
    return false
}

// Get remaining triggers for buff
func (bs *Buffs) TriggersLeft(buffId int) int {
    if idx, ok := bs.buffIds[buffId]; ok {
        return bs.List[idx].TriggersLeft
    }
    return 0
}

// Get all active buffs (optionally filtered by ID)
func (bs *Buffs) GetBuffs(buffId ...int) []*Buff {
    retBuffs := []*Buff{}
    for _, b := range bs.List {
        if !b.Expired() {
            if len(buffId) > 0 {
                // Filter by specific buff IDs
                for _, id := range buffId {
                    if b.BuffId == id {
                        retBuffs = append(retBuffs, b)
                        break
                    }
                }
            } else {
                // Return all active buffs
                retBuffs = append(retBuffs, b)
            }
        }
    }
    return retBuffs
}
```

## Display and Visibility
```go
// Get visible name and description (handles secret buffs)
func (b *BuffSpec) VisibleNameDesc() (name, description string) {
    if b.Secret {
        return "Mysterious Affliction", "Unknown"
    }
    return b.Name, b.Description
}

// Get buff display name
func (bs *Buff) Name() string {
    if sp := GetBuffSpec(bs.BuffId); sp != nil {
        return sp.Name
    }
    return ""
}
```

### The player-side notice (slice C, `notice.go`)

- `StartUserNotice() string` / `EndUserNotice() string`: the ONE door for the
  line the holder reads when a buff lands or ends. Authored `start_user_text`
  / `end_user_text` first; otherwise the generic "<Name> takes effect." /
  "<Name> has expired."; an empty string for a `secret` buff or one with no
  name. `Buff_ApplyBuffs` and the player prune pass in `NewTurn_PruneBuffs`
  read these instead of the raw fields. Room text is untouched and stays
  authored-only.
- `SilentNoticeBuffs() []string`: every loaded non-secret buff relying on the
  generic line, as `"<id> <name> (start, end)"`. `WarnSilentNotices()` logs
  one warning per entry at boot (wired in `main.go` after the species guard).
- Two flags declare a deliberate silence: `hidden` (no end notice ever, a
  hider must not learn when the cover lapsed; room text still goes out) and
  `silent-start` (the applying command narrates the start; Warcry, Rally,
  Throttled, Sleeping and Bloom Detox are applied through `Character.AddBuff`,
  which never queues the buff event). A `silent-start` buff still owes the
  holder a line, just not from the hook: `actions.Sleep` sends buff 15's line
  through `AuthoredStartLine`, which is what the flag means by "the applier
  narrates".
- **A player buff must be applied through `users.UserRecord.AddBuff` or
  `UserRecord.AddBuffScaled`, the event path, or it lands in silence:
  `Character.AddBuff` / `Character.AddBuffScaled` apply in place and queue
  nothing, so `Buff_ApplyBuffs` never runs and no notice reaches the holder.**
  The multiplier rides on `events.Buff.DurationMult`, so a scaled application
  takes the same door. The root guard `buff_apply_path_guard_test.go` fails the
  build on any direct character-level add under `internal/usercommands` or
  `internal/actions` that is not in its allowlist with a reason.
- **Every non-secret buff in the dogmud world must carry authored
  `start_user_text` (unless `silent-start`) AND `end_user_text` (unless
  `hidden`), and a secret buff must carry no player text.** The root guard `buff_notice_guard_test.go` fails the build
  otherwise; the generic line is a runtime net, never the shipped experience.
  `secret: true` also hides the buff from `conditions`.

### The narration door (M3 item 5b, `narration.go`)

- `Phase` (`PhaseStart`, `PhaseTrigger`, `PhaseEnd`) selects the moment.
- `Narration(p Phase) narration.Variants`: the holder's line is the **Actee**
  (the buff happens to them; owner ruling 2026-09-12) and the room's line the
  Observer. Actor is empty and reserved for the caster, which M6 authors once
  `events.Buff` carries one. Start and End go through `StartUserNotice` /
  `EndUserNotice`, so the notice rules stay in their one door.
- `Narrate(p Phase, ctx textutil.TokenContext) narration.Roles` renders it. This
  is what `Buff_ApplyBuffs`, both round ticks and `NewTurn_PruneBuffs` call;
  they deliver each role themselves on today's category and channel.
- `AuthoredStartLine(ctx) string` renders `start_user_text` as written, ignoring
  the notice rules: the door for a silent-start buff's applier (sleep 15,
  arrest 88, stun 84, broken limb 83). Throttled (89) is silent-start too but
  has no sender: its move narrates the choke itself.
- `Validate` runs `narration.ValidateVariants` over every authored phase, on the
  raw fields, so a whitespace-only line fails the load even where a notice
  would hide it.
- **No file outside this package reads the six text fields.** The root guard
  `store_text_fields_guard_test.go` fails the build on one.

## Data Management and Search

### Buff Discovery
```go
// Search buffs by name or description
func SearchBuffs(searchTerm string) []int {
    searchTerm = strings.TrimSpace(strings.ToLower(searchTerm))
    results := make([]int, 0, 2)
    
    for _, buff := range buffs {
        if strings.Contains(strings.ToLower(buff.Name), searchTerm) ||
           strings.Contains(strings.ToLower(buff.Description), searchTerm) {
            results = append(results, buff.BuffId)
        }
    }
    
    return results
}

// Get all available buff IDs
func GetAllBuffIds() []int {
    results := make([]int, 0, len(buffs))
    for _, buff := range buffs {
        results = append(results, buff.BuffId)
    }
    return results
}
```

### File Management
```go
// Generate filename for buff specification
func (b *BuffSpec) Filename() string {
    filename := util.ConvertForFilename(b.Name)
    return fmt.Sprintf("%d-%s.yaml", b.BuffId, filename)
}

// Load all buff specifications from files
func LoadDataFiles() {
    start := time.Now()
    
    tmpBuffs, err := fileloader.LoadAllFlatFiles[int, *BuffSpec](
        string(configs.GetFilePathsConfig().DataFiles) + "/buffs"
    )
    if err != nil {
        panic(err)
    }
    
    buffs = tmpBuffs
    
    mudlog.Info("buffSpec.LoadDataFiles()", 
        "loadedCount", len(buffs), 
        "Time Taken", time.Since(start))
}
```

## Integration Patterns

### Character System Integration
```go
// Buffs integrate with character stats and behavior
- character.Buffs.StatMod("strength")     // Stat modifications
- character.Buffs.HasFlag(buffs.NoCombat) // Behavioral restrictions
- character.Buffs.Trigger()               // Round-based processing
- character.Buffs.Prune()                 // Cleanup expired buffs
```

### Combat System Integration
```go
// Combat checks buff flags for behavior modification
if sourceChar.HasBuffFlag(buffs.Accuracy) {
    critChance *= 2 // Double crit chance
}

if targetChar.HasBuffFlag(buffs.Blink) {
    critChance /= 2 // Half crit chance against blink
}

if !sourceChar.HasBuffFlag(buffs.Hidden) {
    // Send visible combat messages
}
```

### Event System Integration
```go
// Buffs trigger events for start, effect, and end
events.AddToQueue(events.Buff{
    MobInstanceId: mobInstanceId,
    BuffId:        buffId,
    Source:        source,
})
```

## Usage Examples

### Basic Buff Management
```go
// Create new buff collection
buffs := buffs.New()

// Add temporary buff
buffs.AddBuff(poisonBuffId, false)

// Add permanent buff (from equipment)
buffs.AddBuff(strengthBuffId, true)

// Check for specific behavior
if buffs.HasFlag(buffs.NoCombat, false) {
    user.SendText("You cannot engage in combat right now.")
    return
}

// Process round-based triggers
triggeredBuffs := buffs.Trigger()
for _, buff := range triggeredBuffs {
    // Handle buff effects
    processBuff(buff)
}

// Clean up expired buffs
prunedBuffs := buffs.Prune()
for _, buff := range prunedBuffs {
    // Send buff expiration messages
    notifyBuffExpired(buff)
}
```

### Stat Modification Usage
```go
// Calculate total stat bonuses from all buffs
strengthBonus := character.Buffs.StatMod("strength")
dexBonus := character.Buffs.StatMod("dexterity")
healthBonus := character.Buffs.StatMod("health")

// Apply to character stats
character.Stats.Strength.ValueAdj += strengthBonus
character.Stats.Dexterity.ValueAdj += dexBonus
character.HealthMax.Value += healthBonus
```

### Flag-Based Behavior Control
```go
// Check movement restrictions
if character.Buffs.HasFlag(buffs.NoMovement, false) {
    user.SendText("You are unable to move.")
    return
}

// Check combat restrictions with expiration
if character.Buffs.HasFlag(buffs.CancelOnAction, true) {
    user.SendText("Your concentration is broken!")
    // Buff automatically expired by HasFlag call
}

// Environmental interactions
if character.Buffs.HasFlag(buffs.EmitsLight, false) {
    room.LightLevel += 1 // Provide illumination
}
```

## Dependencies

- `internal/statmods` - Stat modification system integration
- `internal/configs` - Configuration management for file paths and timing
- `internal/gametime` - Game time system for trigger rate calculations
- `internal/fileloader` - YAML file loading and validation system
- `internal/util` - Utility functions for file operations and validation
- `internal/mudlog` - Logging system for debugging and monitoring

This comprehensive buffs system provides sophisticated temporary status
effects with precise timing control, behavioral modification, stat
integration, and config-driven effect behaviors.

---

## DOGMud chunk-4d buffs

Two new buffs added in chunk 4d (T9 + T10). Neither is a regen potion or
combat potion — they are combat consequence buffs applied by the submission
outcome resolver (`internal/combat/submission_outcome.go`).

| ID | Name | Duration | Source | Effect |
|----|------|----------|--------|--------|
| 83 | Broken Limb | ~3600 rounds (~1 hr play) | Cripple submission outcome via `applyBrokenLimbBuff` | Reduces combat effectiveness for the afflicted limb's weapon role; persists across respawn; cannot be dispelled early by normal means |
| 84 | Submission Stunned | 1 round | Crit submission tier (mercy policy only) via `applyStunnedBuff` | Brief combat stagger; auto-clears at the end of the following round |

**Buff 83 (Broken Limb)** is the first persistent, non-dispellable
combat debuff players commonly encounter. Triggered by a cripple-policy
submission where the sub type targets a joint (armbar, kimura). Choke-class
subs (RNC, Triangle, Anaconda, Guillotine) do NOT trigger buff 83 because
they have no body-part target — the policy degrades to subdue instead.

**Buff 84 (Submission Stunned)** is a 1-round stagger applied to the
recipient when a mercy-policy submission lands a crit roll
(`SubTierCrit`). Only fires on mercy policy because subdue/cripple/lethal
send the defender through the death cascade and the buff would be a no-op.

See `internal/combat/context.md` "Submission System" for the full
context in which these buffs are applied.

---

## DOGMud chunk-3.3 buffs (Sleeping)

### New flags

| Flag | String value | Purpose |
|------|-------------|---------|
| `Sleeping` | `"sleeping"` | Bearer is asleep — gates regen boost, first-hit-crit, room rendering. Chunk 3.3. |
| `CancelOnDamage` | `"cancel-on-damage"` | Buff cancels when any damage is applied to bearer. Wired in damage pipeline. Chunk 3.3. |

### New buff

| ID | Name | Duration | Source | Effect |
|----|------|----------|--------|--------|
| 15 | Sleeping | Unlimited (until woken) | `actions.Sleep`, sleep user command, sleep mob command, schedule executor | Applies `Sleeping` + `CancelOnDamage` + `NoCombat` + `NoMovement` flags; triggers `SleepRegenMultiplier` (5×) regen; forces first-hit-crit on all attackers for the round the buff is active. Cancelled by damage, failed steal, shout-in-room, light source entering room, `stand`, or schedule segment end. |

### Usage pattern

```go
// Check if character is asleep
if c.HasBuffFlag(buffs.Sleeping) { ... }

// Wake a sleeper (central hook)
mobs.OnSleeperWoken(c)

// Cancel all sleeping buffs (schedule exit path)
c.CancelBuffsWithFlag(buffs.Sleeping)
```

---

## DOGMud chunk-5.1c buffs (Jailed)

### New buff

| ID | Name | Duration | Source | Effect |
|----|------|----------|--------|--------|
| 88 | Jailed | Scaled to sentence rounds via `AddBuffScaled` | `internal/justice.ExecuteArrest` | Carries two flags: `no-go` (`NoMovement`) prevents all movement, and `no-aggro-target` makes the bearer invisible to mob aggro targeting. `TriggersLeft` is set to the sentence length in rounds so the buff expires naturally at sentence end. Removed explicitly by `internal/justice.ResolveDetention` on timer expiry or fine payment. |

**`NoMovement` flag** (`no-go`, `NoMovement Flag = "no-go"`) is checked by
`go.go` (room-exit commands), `flee.go` (flee), and `spell_foldrecall.go`
(recall) to keep a jailed player locked in by every egress path.

**`NoAggroTarget` flag** (`no-aggro-target`) — the same flag respawn-grace
uses — makes the jailed player un-targetable by all mob aggro paths
(LookForTrouble, retarget, etc.). The combat round
(`hooks/NewRound_DoCombat.go`) additionally drops a mob's *stale* aggro on
a `no-aggro-target` player, so a guard that was already fighting the player
before arrest stops pursuing them into the cell.

**`AddBuffScaled(buffId int, scale float64)`** is the mechanism: passing
`float64(rounds)` as scale sets `TriggersLeft = buff.TriggerCount * scale`.
For buff 88 (TriggerCount=1, TriggerRate="1 round"), this yields exactly
`rounds` triggers remaining — one per round of the sentence.

The buff's `start_user_text` and `end_user_text` fire automatically via
the buff system at cell entry and at removal. Because `RemoveBuff` fires
`end_user_text` ("The cell door swings open. You are free to go."), that is
the single release line for BOTH the timer-expiry and pay-fine paths —
`ResolveDetention` deliberately sends no release flavor of its own (avoids
the duplicate-message bug). `ExecuteArrest` sends an additional
arrest-context line at cell entry; `payfine` sends a payment line that does
not mention the door.

---

## DOGMud Shield/Ward Spell Scaling

Two live spells use `effect_type: shield`: `conviction-ward`
(`effect_magnitude: 75`) and `chrysalis-cocoon` (`effect_magnitude: 125`), both
defined under `_datafiles/world/dogmud/spells/`. The `case "shield"` branch of
`resolveSpell`/`resolveMobSpell` in `internal/hooks/spell_resolution.go` (the
player path near line 1093, the mob path near line 1414) is the single handler
for every shield spell, present or future.

```go
shieldBonus := (spellData.CasterStatValue(user.Character.Stats) + weightedSkill) / 3
if magnitude > 0 {
    shieldBonus = int(math.Round(float64(shieldBonus) * float64(magnitude) / 100.0))
}
if out.AttackerCrit {
    shieldBonus = int(float64(shieldBonus) * 1.5)
}
target.Character.AddCondition(characters.ConditionShield, duration, float64(shieldBonus), "spell")
```

`weightedSkill` is the caster's spellcasting skill level times `SkillWeight`
(ships 5.0 against a Go default of 2.0). `magnitude` is
`spellData.EffectMagnitude`, and 100 is the 1.0x baseline: a spell carrying
`effect_magnitude: 75` applies 0.75 of the base roll, one carrying 125 applies
1.25x. There is an `if out.AttackerCrit { shieldBonus *= 1.5 }` bump on the
player cast path, but it is unreachable for both shipped shield spells, not
just the mob one. `conviction-ward` and `chrysalis-cocoon` are both
`type: helpsingle` with no `target_defense_type`, so shielding yourself never
runs an opposed contest for either caster type: `resolveSpell`
(`internal/hooks/spell_resolution.go` near line 172) takes the
`spellData.TargetDefenseType == ""` branch and calls `applyPlayerEffect` with
a synthetic `combat.ChannelDefenceResult{DamageMultiplier: 1}` instead of a
real roll, so `AttackerCrit` is false by construction. `applyPlayerEffect`
only carries an `out` parameter at all because it is shared with
`resolveAgainstPlayer`, the contested-attack path used by unwilling-target
spells (`TargetDefenseType != ""`); a self-buff never reaches that path. So
`applyMobSelfEffect`'s missing crit check is a consequence of that function's
narrower scope (mobs only ever self-buff, so nothing forced it to share the
contested-attack signature), not evidence that mobs are treated differently
from players; neither self-buff is contested. A future fix would need a real
roll to crit against: the static-difficulty seam
`contest.AgainstDifficulty(score, difficulty)` (`internal/contest/contest.go`)
already exists and is used by search, track, and forage checks; no spell path
calls it. Duration is computed
by the unexported `calcSpellDuration(baseFolds, spellcastingSkill, willpower)`
in the same file, not by anything in this package:
`duration = baseFolds * (10 + willpower/20 + spellcastingSkill/2)`.

**Where the magnitude actually lands.** `AddCondition(characters.ConditionShield,
...)` does not touch `magical_mitigation` or `conviction_mitigation` at all.
The condition's magnitude is read only by `Character.GetPhysicalMitigation()`
(`internal/characters/combat.go`), which sums it with gear
`physical_mitigation`, mutation natural armor, and species natural armor, then
clamps the total at `PhysicalMitigationCap`. So both "magical" ward spells buy
physical mitigation through the shield condition, not magical or conviction
mitigation.

A shield spell can separately carry `buff_ids`, and those buffs use the
ordinary statmod path described above instead: `chrysalis-cocoon` grants
`buff_ids: [52]` (Chrysalis Shell, in
`_datafiles/world/dogmud/buffs/52-chrysalis_shell.yaml`), whose
`statmods: {magical_mitigation: 15, conviction_mitigation: 15}` are summed by
`Buffs.StatMod()` and read by `Character.GetMagicalMitigation()` /
`GetConvictionMitigation()` through `c.StatMod("magical_mitigation")` /
`c.StatMod("conviction_mitigation")`. `conviction-ward` sets no `buff_ids`, so
it grants no magical or conviction mitigation at all despite its name.

### Mitigation caps

| Cap | Shipped (`config.yaml`) | Go default |
|-----|--------------------------|------------|
| `PhysicalMitigationCap` | 0.75 | 0.75 |
| `MagicalMitigationCap` | 0.75 | 0.75 |
| `ConvictionMitigationCap` | 0.75 | 0.75 |

All three are read by `combat.ApplyMitigation`
(`internal/combat/damage_pipeline.go`) and validated positive and at most 1.0
by `internal/configs/smoke_test.go`. Nothing in this package enforces them;
they live downstream, in the damage pipeline.

## Gotchas

- **Shield magnitude converges on the physical mitigation cap, not on a
  per-spell ceiling.** `conviction-ward` (magnitude 75) and `chrysalis-cocoon`
  (magnitude 125) both feed `GetPhysicalMitigation()`, and once a caster's
  shield roll alone clears `PhysicalMitigationCap` (0.75), every shield spell
  reads identically at the player: same condition, same message, same
  effective mitigation, regardless of magnitude. The full mechanism, the
  thresholds it saturates at, and the design options under discussion are in
  `docs/audits/2026-08-30-shield-spells-converge-at-the-cap.md`; read that
  before re-deriving or retuning this.
- The one durable difference between the two spells today is
  `chrysalis-cocoon`'s `buff_ids: [52]` grant (adds magical and conviction
  mitigation through the statmod path above, invisible unless the target is
  actually taking magical or conviction damage) and its longer `base_folds`
  (8 against 4), which changes duration but is never surfaced to the player.

## Files

| File | Purpose |
|------|---------|
| `buffspec.go` | The authored `BuffSpec` and its loader |
| `notice.go` | The player-side start/end notice resolver and the silent-buff listing |
| `narration.go` | The narration door: `Phase`, `Narration`, `Narrate`, `AuthoredStartLine`, `validateNarration` |
| `buffs.go` | Applied-buff instances, flags, stat mods |
| `tick.go` | Per-round buff processing and expiry |
| `test_helpers.go` | Test fixtures |

Buff files are named `{buffid}-{ConvertForFilename(name)}.yaml` — `name:
Stunned` must be `2-stunned.yaml`, or loading panics at startup.
