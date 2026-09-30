# GoMud Event-Driven Architecture Context

## Overview

The GoMud event system provides a comprehensive event-driven architecture that enables decoupled communication between game systems through a priority-based event queue with listener registration, unique event handling, and sophisticated processing controls. It supports both synchronous and asynchronous event processing with thread-safe operations and performance optimization.

## Architecture

The event system is built around several key components:

### Core Components

**Event Queue System:**
- Priority-based event queue using heap data structure
- Unique event handling to prevent duplicate processing
- Thread-safe operations with mutex protection
- Requeue mechanism for deferred event processing

**Listener Management:**
- Dynamic listener registration and unregistration
- Priority-based listener ordering (First, Normal, Last)
- Wildcard listeners for debugging and monitoring
- Return value control for event flow management

**Event Types:**
- Strongly typed event system with interface-based design
- Built-in event types for all major game systems
- Generic events for plugin and module integration
- Unique events with automatic deduplication

**Processing Control:**
- Configurable event flags for behavior modification
- Event cancellation and requeue capabilities
- Performance monitoring and debugging support
- Automatic error handling and logging

## Key Features

### 1. **Priority-Based Event Processing**
- Events can be assigned priority levels (lower numbers = higher priority)
- FIFO ordering within same priority level
- Heap-based queue for efficient priority processing
- Automatic order preservation for fair processing

### 2. **Unique Event Handling**
- Events implementing `UniqueEvent` interface are automatically deduplicated
- Only one instance of a unique event can exist in queue at a time
- Useful for preventing redundant operations (e.g., map rebuilding, prompt redrawing)

### 3. **Flexible Listener System**
- Listeners can be registered for specific event types or all events (wildcard)
- Support for listener priorities: First, Normal, Last execution order
- Dynamic listener registration and removal during runtime
- Return value control for event processing flow

### 4. **Thread-Safe Operations**
- All event queue operations are protected by mutexes
- Safe concurrent access from multiple goroutines
- Optimized locking for high-performance event processing

### 5. **Performance Monitoring**
- Built-in performance tracking and debugging
- Event processing time measurement
- Statistics on events without listeners
- Configurable debug output for troubleshooting

## Event Types

### Core Game Events

**Player Lifecycle:**
```go
type PlayerSpawn struct {
    UserId        int
    ConnectionId  uint64
    RoomId        int
    Username      string
    CharacterName string
}

type PlayerDespawn struct {
    UserId        int
    RoomId        int
    Username      string
    CharacterName string
    TimeOnline    string
}
```

**Character Events:**
```go
type CharacterCreated struct {
    UserId        int
    CharacterName string
}

type CharacterVitalsChanged struct {
    UserId int
}

type CharacterStatsChanged struct {
    UserId int
}
```

There is no `LevelUp` event: level-up is disabled in DOGMud and the upstream
event type is gone.

**Combat and Death:**
```go
type PlayerDeath struct {
    UserId        int
    RoomId        int
    Username      string
    CharacterName string
    Permanent     bool
    KilledByUsers []int
}

type MobDeath struct {
    MobId         int
    InstanceId    int
    RoomId        int
    CharacterName string
    Level         int
    PlayerDamage  map[int]int
}
```

**Room and Movement:**
```go
type RoomChange struct {
    UserId        int
    MobInstanceId int
    FromRoomId    int
    ToRoomId      int
    Unseen        bool
}

type RoomAction struct {
    RoomId       int
    SourceUserId int
    SourceMobId  int
    Action       string
    Details      any
    ReadyTurn    uint64
}
```

**Communication:**
```go
type Communication struct {
    SourceUserId        int
    SourceMobInstanceId int
    TargetUserId        int
    CommType            string // say, party, broadcast, whisper, shout
    Name                string
    Message             string
}

type Broadcast struct {
    Text             string
    TextScreenReader string
    IsCommunication  bool
    SourceIsMod      bool
    SkipLineRefresh  bool
}
```

### System Events

**Timing Events:**
```go
type NewRound struct {
    RoundNumber uint64
    TimeNow     time.Time
}

type NewTurn struct {
    TurnNumber uint64
    TimeNow    time.Time
}

type DayNightCycle struct {
    IsSunrise bool
    Day       int
    Month     int
    Year      int
    Time      string
}
```

**Input Processing:**
```go
type Input struct {
    UserId        int
    MobInstanceId int
    InputText     string
    ReadyTurn     uint64
    Flags         EventFlag
}
```

### Automation Events

```go
// AutomationChanged fires when a user's macros/aliases (and, later,
// ticks/triggers) change, so the Char.Automation GMCP payload can be
// re-pushed to the web client.
type AutomationChanged struct {
    UserId int
}
```

Emitted by `internal/usercommands/set.go` (`cmdSetMacro`) and
`internal/usercommands/alias.go` (`Alias`) after any macro or alias
mutation. The `gmcp.Automation` module listens for this event and
calls `sendAutomation(userId)` to re-push `Char.Automation` over
GMCP. In Phases 2–3 the same event will be emitted by inbound
`Char.Automation.Set` / `Char.Automation.Remove` GMCP handlers when
ticks or triggers are added or removed.

### Unique Events

**Events with automatic deduplication:**
```go
type RebuildMap struct {
    MapRootRoomId int
    SkipIfExists  bool
}

func (r RebuildMap) UniqueID() string {
    return `RebuildMap-` + strconv.Itoa(r.MapRootRoomId) + `-` + strconv.FormatBool(r.SkipIfExists)
}

type RedrawPrompt struct {
    UserId        int
    OnlyIfChanged bool
}

func (l RedrawPrompt) UniqueID() string {
    return `RedrawPrompt-` + strconv.Itoa(l.UserId)
}
```

## Event Flags

### Command Behavior Flags
```go
const (
    CmdNone                    EventFlag = 0
    CmdSkipScripts             EventFlag = 0b00000001  // Skip script processing
    CmdSecretly                EventFlag = 0b00000010  // Don't alert room to action
    CmdIsRequeue               EventFlag = 0b00000100  // Prevent infinite requeue loops
    CmdBlockInput              EventFlag = 0b00001000  // Block user input
    CmdUnBlockInput            EventFlag = 0b00010000  // Unblock user input
    CmdBlockInputUntilComplete EventFlag = CmdBlockInput | CmdUnBlockInput
)
```

### Flag Usage
```go
// Check if flag is set
if flags.Has(CmdSkipScripts) {
    // Skip script processing
}

// Add flag
flags.Add(CmdSecretly)

// Remove flag
flags.Remove(CmdBlockInput)
```

## Listener System

### Listener Registration
```go
// Register listener for specific event type
listenerId := events.RegisterListener(PlayerSpawn{}, func(e events.Event) events.ListenerReturn {
    spawn := e.(events.PlayerSpawn)
    log.Printf("Player %s spawned in room %d", spawn.CharacterName, spawn.RoomId)
    return events.Continue
})

// Register high-priority listener (executes first)
events.RegisterListener(PlayerDeath{}, handlePlayerDeath, events.First)

// Register final listener (executes last)
events.RegisterListener(MobDeath{}, logMobDeath, events.Last)

// Register wildcard listener (receives all events)
events.RegisterListener(nil, debugAllEvents)
```

### Listener Return Values
```go
const (
    Continue         ListenerReturn = 0b00000001  // Allow next listener to process
    Cancel           ListenerReturn = 0b00000010  // Stop processing this event
    CancelAndRequeue ListenerReturn = 0b00000100  // Stop and requeue for next cycle
)
```

### Listener Examples
```go
// Character death handler
func handlePlayerDeath(e events.Event) events.ListenerReturn {
    death := e.(events.PlayerDeath)
    
    // death.Permanent is always false (permadeath sunset); field kept for
    // upstream parity only. All deaths are treated as normal deaths.
    // Normal death, allow other handlers
    return events.Continue
}
```

## Event Processing

### Adding Events to Queue
```go
// Add event with default priority (0)
events.AddToQueue(events.PlayerSpawn{
    UserId:        123,
    ConnectionId:  456,
    RoomId:        1,
    Username:      "player1",
    CharacterName: "Hero",
})

// Add event with high priority (lower number = higher priority)
events.AddToQueue(events.PlayerDeath{
    UserId:        123,
    RoomId:        1,
    Username:      "player1",
    CharacterName: "Hero",
    Permanent:     false,
}, -10)

// Add unique event (automatically deduplicated)
events.AddToQueue(events.RedrawPrompt{
    UserId:        123,
    OnlyIfChanged: true,
})
```

### Event Processing Loop (example)
```go
// Process all events in queue
events.ProcessEvents()

// This is typically called from the main game loop:
func gameLoop() {
    for {
        // Handle network input
        processNetworkInput()
        
        // Process all queued events
        events.ProcessEvents()
        
        // Sleep until next tick
        time.Sleep(tickDuration)
    }
}
```

### Custom Event Creation (example)
```go
// Define custom event type
type CustomGameEvent struct {
    PlayerId int
    Action   string
    Data     map[string]any
}

func (c CustomGameEvent) Type() string {
    return "CustomGameEvent"
}

// Register listener for custom event
events.RegisterListener(CustomGameEvent{}, func(e events.Event) events.ListenerReturn {
    custom := e.(CustomGameEvent)
    // Handle custom event
    return events.Continue
})

// Fire custom event
events.AddToQueue(CustomGameEvent{
    PlayerId: 123,
    Action:   "special_action",
    Data:     map[string]any{"value": 42},
})
```

## Integration Patterns

### Hook System Integration (example)
```go
// Event hooks are registered as listeners
func init() {
    events.RegisterListener(events.NewRound{}, handleNewRound)
    events.RegisterListener(events.PlayerSpawn{}, handlePlayerJoin)
    events.RegisterListener(events.RoomChange{}, handleRoomChange)
}

// Hook implementations
func handleNewRound(e events.Event) events.ListenerReturn {
    round := e.(events.NewRound)
    
    // Process combat
    handleCombat(round)
    
    // Process mob AI
    processMobAI(round)
    
    // Auto-healing
    processAutoHeal(round)
    
    return events.Continue
}
```

### Module Integration (example)
```go
// Modules can register for events they care about
type AuctionModule struct {
    // module fields
}

func (m *AuctionModule) Initialize() {
    events.RegisterListener(events.PlayerSpawn{}, m.onPlayerJoin)
    events.RegisterListener(events.NewTurn{}, m.processAuctions)
}

func (m *AuctionModule) onPlayerJoin(e events.Event) events.ListenerReturn {
    spawn := e.(events.PlayerSpawn)
    // Send auction notifications to new player
    return events.Continue
}
```

### Scripting Integration (example)
```go
// JavaScript can raise custom events
func RaiseEvent(name string, data map[string]any) {
    events.AddToQueue(events.ScriptedEvent{
        Name: name,
        Data: data,
    })
}

// Event listener handles scripted events
func handleScriptedEvent(e events.Event) events.ListenerReturn {
    scripted := e.(events.ScriptedEvent)
    
    switch scripted.Name {
    case "custom-quest-complete":
        handleQuestComplete(scripted.Data)
    case "special-room-effect":
        handleRoomEffect(scripted.Data)
    }
    
    return events.Continue
}
```

## Performance Considerations

### Event Queue Optimization
- Heap-based priority queue provides O(log n) insertion and removal
- Unique event deduplication prevents redundant processing
- Mutex locking optimized for high-frequency operations
- Requeue mechanism prevents blocking on temporary failures

### Memory Management
```go
// Events are processed and removed from queue immediately
// No long-term event storage or memory leaks
// Automatic cleanup of unique event tracking

// Performance monitoring
func ProcessEvents() {
    start := time.Now()
    defer func() {
        if time.Since(start) > threshold {
            log.Printf("Event processing took %v", time.Since(start))
        }
    }()
    
    // Process events...
}
```

### Debug and Monitoring
```go
// Enable event debugging
events.SetDebug(true)

// Monitor events without listeners
// Automatically logs when events have no registered listeners
// Helps identify missing event handlers

// Performance tracking
// Automatic timing of event processing loops
// Configurable sampling to reduce overhead
```

## Error Handling

### Event Processing Errors
- Listener panics are recovered and logged
- Invalid events are skipped with error logging
- Missing listeners are tracked and reported
- Performance issues are automatically detected and logged

### Listener Management (example)
```go
// Safe listener removal
func cleanup() {
    // Unregister listeners when no longer needed
    events.UnregisterListener(PlayerSpawn{}, listenerId)
    
    // Clear all listeners (typically for testing)
    events.ClearListeners()
}
```

## Dependencies

- `container/heap` - Priority queue implementation
- `sync` - Thread-safe operations and mutex protection
- `internal/mudlog` - Logging and error reporting
- `internal/util` - Performance tracking and utilities
- `time` - Event timing and performance measurement

## Usage Examples

### Complete Event Lifecycle (example)
```go
// 1. Define event type
type PlayerLoginEvent struct {
    UserId   int
    Username string
    LoginTime time.Time
}

func (p PlayerLoginEvent) Type() string { return "PlayerLogin" }

// 2. Register listeners
events.RegisterListener(PlayerLoginEvent{}, func(e events.Event) events.ListenerReturn {
    login := e.(PlayerLoginEvent)
    log.Printf("Player %s logged in at %v", login.Username, login.LoginTime)
    
    // Send welcome message
    events.AddToQueue(events.Message{
        UserId: login.UserId,
        Text:   "Welcome back!",
    })
    
    return events.Continue
})

// 3. Fire event
events.AddToQueue(PlayerLoginEvent{
    UserId:    123,
    Username:  "player1",
    LoginTime: time.Now(),
})

// 4. Process events
events.ProcessEvents()
```

This event system provides the foundation for GoMud's decoupled architecture, enabling flexible and efficient communication between all game systems while maintaining performance and reliability.
## Files

| File | Purpose |
|------|---------|
| `events.go` | The `Event` interface and dispatch |
| `eventtypes.go` | Every concrete event type |
| `listeners.go` | Listener registry and `ListenerReturn` |
| `queue.go` | The heap-based priority queue |
| `eventlogger.go` | Event logging |
| `const.go` | Constants and flags |

Test-only queue drain seams in `events.go` include
`DrainQueuedPlayerAttackedMobsForTest(userId)`. Passing zero drains all queued
`PlayerAttackedMob` events; a nonzero ID drains only that player's events.
`DrainQueuedGoldGivenForTest(userId)` does the same for `GoldGiven`, which
`give` fires when a player hands gold to a mob (`GiftAccepted` is items
only); `modules/aicompanion` reads it to know who gave a companion coin.
`DrainQueuedConditionsForTest(userId)` drains queued `Condition` events for a
player, and `DrainQueuedMobConditionsForTest(mobInstanceId)` is its mob twin
(drink path unification), matching on `MobInstanceId`. Zero drains every
queued `Condition` event in either.
`DrainQueuedHealedForTest(healerUserId)` drains queued `Healed` events the
same way (spell effects slice 3b); zero drains them all.
`DrainQueuedCharacterDiedForTest()` and `DrainQueuedMobDeathsForTest()` drain
every queued `CharacterDied` and `MobDeath` event; the second lets a test
assert on the damaging-players list the `MobDeath_*` hooks read.
`DrainQueuedMessageEventsForTest(userId)` (sight gates slice 5b) is
`DrainQueuedMessagesForTest` returning the whole `Message`, not just its
text, so a test can also read `IsCommunication`. It matches on
`msg.UserId == userId`, so a nonzero id drains only that user's queued
`Message` events, but 0 also matches every room-keyed `Message` (a
`RoomId`-keyed send leaves `UserId` at its zero value), draining those too.
`DrainQueuedRoomMessagesForTest(roomId)` drains every `RoomId`-keyed
`Message` queued for `roomId` (`Room.SendTextCommunication`,
`Room.SendTextToExits`) rather than a per-user one.

`DrainQueuedUserInputsForTest(userId)` (parity slice 6) drains the `Input`
events `UserRecord.Command` queued for a player (`MobInstanceId` 0, matched on
`UserId`); `DrainQueuedInputsForTest` and `InspectQueuedInputForTest` match
mobs only.

`Message.HiddenFromDeafened(deafened bool) bool` (sight gates slice 5b) is
the one statement of the Deafened moderation rule: true when `m` is player
chatter (`IsCommunication`) and the reader is deafened.
`internal/hooks/Message_SendMessages.go` applies it on both of its delivery
branches, and tests read it rather than copy the rule.

`Condition` (`eventtypes.go`) carries `TickScale float64` (tick amount at
apply, 2026-09-28) alongside `Magnitude`/`Triggers` and `DurationMult`: it
scales a `tick_pool` condition's per-round amount, computed where the
condition lands rather than snapshotted by its producer. Zero means 1.0.
`users.UserRecord.AddConditionTickScaled` and `mobs.Mob.AddConditionTickScaled`
are the only producers that set it, queuing it for `hooks.ApplyConditions`
to read; `hooks.spellTickScale(caster)` is the one caster formula a spell
passes through them, so a heal- or damage-over-time spell scales with the
caster's skill and gear on the very first cast, not only on a recast.
Potions and hazards queue `Condition` with no `TickScale`, so their tick
amount computes at the fallback scale of 1.0.

This is the synchronous engine bus. `internal/worldevents` is a separate,
passive record of notable happenings — do not confuse the two.
