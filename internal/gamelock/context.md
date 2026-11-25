# Game Lock System Context

## Overview

The `internal/gamelock` package provides a sophisticated locking mechanism for the GoMud game engine. It supports difficulty-based locks, automatic relocking, trap systems, and time-based lock management integrated with the game's temporal system.

## Key Components

### Core Files
- **gamelock.go**: Lock data structure and functionality

### Key Structures

#### Lock
```go
type Lock struct {
    Difficulty       uint8  `yaml:"difficulty,omitempty"`
    UnlockedRound    uint64 `yaml:"-"`
    RelockInterval   string `yaml:"relockinterval,omitempty"`
    TrapConditionIds []int  `yaml:"trapconditionids,omitempty,flow"`
    RotationSeed     uint64 `yaml:"rotationseed,omitempty"`
}
```
Represents a game lock with the following features:
- **Difficulty**: Numeric difficulty level (0 = no lock, >0 = locked)
- **UnlockedRound**: Game round when lock was opened (not persisted)
- **RelockInterval**: Time specification for automatic relocking
- **TrapConditionIds**: Condition IDs applied when lockpicking fails
- **RotationSeed**: Bumped by every `SetLocked`; one input to `Rotation`

## Every relock gets a new combination (owner ruling 2026-11-25)

A combination a player learned stops working at the lock's next relock,
whatever caused it: `SetLocked` (the `lock` command, a script, a forager's
lockbox) or the relock timer running out. The rule lives in two methods, and
every reader of a combination goes through them:

- **Rotation() uint64**: the rotation the combination is built from now.
  `RotationSeed` while the lock has never been opened, is open, or was last
  closed by `SetLocked`. Once the timer has relocked an opened lock, a hash
  of `RotationSeed` and `UnlockedRound` with the top bit set, so each
  open-then-timer-relock cycle gets its own rotation and none equals a plain
  `RotationSeed`. Worked out from state, with no write: `IsLocked` decides a
  timer relock lazily and a `Lock` is copied by value in many places.
- **Sequence(lockId string) string**: the combination now,
  `util.LockSequence(lockId, Difficulty, Rotation())`. `characters.HasKey`,
  `picklock`, `keyring` and the admin views call it; nothing else calls
  `util.LockSequence` (guarded in `internal/guards/lock_seed_guard_test.go`).

The lock state behind it outlives a room unload through
`internal/rooms/lock_memory.go` (exits are template-owned and `UnlockedRound`
is never saved). A restart makes every combination new anyway, through the
boot-rerolled lock seed. Tests: `rotation_test.go`.

### Constants
- **DefaultRelockTime**: `"1 hour"` - Default relock interval when none specified

## Key Methods

### Lock State Management
- **IsLocked() bool**: Determines current lock state
  - Returns false if difficulty is 0 (no lock)
  - Returns true if never unlocked (UnlockedRound == 0)
  - Calculates time-based relocking using game time system
  - Uses RelockInterval or DefaultRelockTime for timing, measured from
    UnlockedRound (it once measured from the current round, so an opened
    lock never relocked until a restart; `TestLock_RelocksAfterInterval`
    pins the fix)

- **SetUnlocked()**: Marks lock as unlocked
  - Sets UnlockedRound to current game round
  - Only applies to locks with difficulty > 0
  - Starts the relock timer

- **SetLocked()**: Forces lock to locked state
  - Resets UnlockedRound to 0
  - Bumps RotationSeed, so the combination changes
  - Immediately locks regardless of timing

## Dependencies

### Internal Dependencies
- `internal/gametime`: For game time calculations and period management
- `internal/util`: For accessing current game round counter

### External Dependencies
- YAML serialization support through struct tags

## Lock Mechanics

### Difficulty System
- **No Lock**: Difficulty 0 means no locking mechanism
- **Scaled Difficulty**: Higher numbers indicate harder locks
- **Skill Integration**: Difficulty affects lockpicking success rates
- **Flexible Scaling**: Supports any difficulty level from 1-255

### Time-Based Relocking
- **Automatic Relocking**: Locks automatically relock after specified time
- **Game Time Integration**: Uses in-game time system for calculations
- **Configurable Intervals**: Custom relock times per lock
- **Default Behavior**: Falls back to 1-hour default if no interval specified

### Trap System
- **Failure Consequences**: Failed lockpicking attempts trigger traps
- **Condition Application**: Traps apply negative effects via condition system
- **Multiple Traps**: Supports multiple condition effects per trap
- **Configurable Effects**: Trap effects defined through condition IDs

## Usage Patterns

### Creating Locks
```go
// Simple lock with difficulty
simpleLock := gamelock.Lock{
    Difficulty: 5,
}

// Advanced lock with custom relock time and traps
advancedLock := gamelock.Lock{
    Difficulty:     10,
    RelockInterval: "30 minutes",
    TrapConditionIds:    []int{123, 456}, // Poison and paralysis conditions
}
```

### Lock Interaction
```go
// Check if locked
if lock.IsLocked() {
    // Attempt lockpicking or require key
    if lockpickingSuccess {
        lock.SetUnlocked()
    } else {
        // Apply trap effects from TrapConditionIds
    }
}

// Force lock closed
lock.SetLocked()
```

### Time-Based Management
```go
// Lock will automatically relock after specified interval
// No manual intervention needed - IsLocked() handles timing
if lock.IsLocked() {
    // Lock is currently locked (either never opened or relocked)
} else {
    // Lock is currently unlocked (within relock interval)
}
```

## Integration Points

### Game Time System
- **Round-Based Timing**: Uses game rounds for precise timing
- **Period Calculations**: Leverages gametime.AddPeriod() for intervals
- **Temporal Consistency**: Maintains consistency with game world time

### Condition System
- **Trap Integration**: TrapConditionIds reference condition system effects
- **Failure Consequences**: Failed lockpicking applies negative conditions
- **Effect Stacking**: Multiple trap conditions can be applied simultaneously

### Skill System
- **Lockpicking Skills**: Lock difficulty affects skill check success rates
- **Skill Progression**: Harder locks provide better skill advancement
- **Failure Learning**: Trap consequences teach caution

### Item System
- **Key Integration**: Locks can be opened with appropriate keys
- **Lockpicking Tools**: Tools may modify difficulty or success rates
- **Magical Items**: Special items might bypass or modify lock behavior

## Data Persistence

### YAML Serialization
- **Persistent Fields**: Difficulty, RelockInterval, TrapConditionIds and
  RotationSeed saved (an exit's lock is template-owned, so its RotationSeed is
  kept in memory across an unload instead, `rooms/lock_memory.go`)
- **Transient State**: UnlockedRound not persisted (resets on server restart;
  kept in memory across a room unload)
- **Configuration Integration**: Locks defined in room/container YAML files

### State Management
- **Server Restart Behavior**: All locks reset to locked state on restart
- **Clean State**: No persistent unlocked state prevents exploitation
- **Configuration Driven**: Lock properties defined in data files

## Time Interval Formats

### Supported Formats
- **Hours**: "1 hour", "2 hours"
- **Minutes**: "30 minutes", "45 minutes"
- **Days**: "1 day", "3 days"
- **Flexible Parsing**: Uses gametime system's period parsing

### Default Behavior
- **Fallback**: Uses DefaultRelockTime when RelockInterval is empty
- **Consistency**: Ensures all locks have defined relock behavior
- **Predictability**: Players can learn lock timing patterns

## Security Features

### Anti-Exploitation
- **Time-Based Relocking**: Prevents permanent unlocking
- **Trap Consequences**: Discourages brute force attempts
- **Difficulty Scaling**: Higher security for important areas
- **State Reset**: Server restarts reset all lock states

### Balanced Gameplay
- **Temporary Access**: Unlocked state is temporary
- **Risk/Reward**: Traps balance lockpicking attempts
- **Skill Progression**: Difficulty provides advancement opportunities

## Performance Considerations

### Efficient State Checking
- **Lazy Evaluation**: IsLocked() calculates state on demand
- **No Background Processing**: No continuous lock monitoring needed
- **Minimal Memory**: Lightweight structure with minimal state

### Time Calculations
- **Game Time Integration**: Leverages existing time system
- **Cached Calculations**: Game time system handles optimization
- **Round-Based Precision**: Uses game rounds for accurate timing
