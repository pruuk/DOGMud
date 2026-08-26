# GoMud Configuration Management System Context

## Overview

The GoMud configuration system provides comprehensive, type-safe configuration management with YAML-based storage, runtime overrides, environment variable support, and validation. It supports hierarchical configuration structures, dot-notation access, and secure handling of sensitive data through a sophisticated type system and validation framework.

## Architecture

The configuration system is built around a centralized `Config` struct with several key components:

### Core Components

**Configuration Structure:**
- Hierarchical configuration organized into logical subsections (Server, Network, GamePlay, etc.)
- Type-safe configuration values using custom types (`ConfigString`, `ConfigInt`, `ConfigBool`, etc.)
- Automatic validation and default value enforcement
- Thread-safe access with read-write mutex protection

**Override System:**
- Runtime configuration overrides stored separately from base configuration
- Dot-notation path support for nested configuration access
- Persistent override storage in YAML format
- Automatic path correction and fuzzy matching for configuration keys

**Type System:**
- Custom configuration types with string conversion and validation
- Special `ConfigSecret` type that redacts sensitive values in output
- Automatic type inference and conversion from string values
- Support for complex types including slices and nested structures

**Validation Framework:**
- Per-subsection validation with range checking and defaults
- Locked configuration properties that cannot be changed at runtime
- Banned name patterns for user input validation
- Environment variable integration with automatic assignment

## Key Features

### 1. **Type-Safe Configuration**
- Custom configuration types prevent type errors and provide consistent interfaces
- Automatic validation and range checking for all configuration values
- Support for integers, floats, booleans, strings, slices, and secret values
- Type inference from string input for dynamic configuration updates

### 2. **Hierarchical Structure**
- Logical organization into subsections (Server, Network, GamePlay, FilePaths, etc.)
- Dot-notation access for nested configuration properties
- Automatic path resolution and correction for typos or partial matches
- Flattening and unflattening of nested structures for override management

### 3. **Runtime Configuration Management**
- Live configuration updates without server restart
- Persistent override storage separate from base configuration
- Thread-safe configuration access with mutex protection
- Configuration change validation and rollback on errors

### 4. **Security Features**
- `ConfigSecret` type automatically redacts sensitive values in logs and output
- Environment variable support for secure credential injection
- Locked configuration properties to prevent unauthorized changes
- Validation of user input against banned patterns

### 5. **Override System**
- Dot-notation configuration overrides (e.g., `Server.MaxCPUCores`)
- Persistent storage of overrides in separate YAML file
- Automatic path correction and fuzzy matching
- Override precedence over base configuration values

## Configuration Subsections

### Server Configuration
```yaml
Server:
  MudName: "My MUD Server"
  CurrentVersion: "1.0.0"
  Seed: "random-seed-value"        # ConfigSecret type
  MaxCPUCores: 4
  OnLoginCommands: ["look", "who"]
  Motd: "Welcome to the game!"
  NextRoomId: 1000
  Locked: ["Seed"]                 # Locked properties
```

### Network Configuration
```yaml
Network:
  MaxTelnetConnections: 50
  TelnetPort: ["4000", "4001"]
  LocalPort: 4002
  HttpPort: 80
  HttpsPort: 443
  HttpsRedirect: true
  AfkSeconds: 300
  MaxIdleSeconds: 1800
  TimeoutMods: false
  ZombieSeconds: 60
  LogoutRounds: 10
```

### GamePlay Configuration
```yaml
GamePlay:
  AllowItemBuffRemoval: true
  Death:
    CorpseDecayRounds: 100
  ShopRestockRate: "1h"
  ContainerSizeMax: 50
  MaxAltCharacters: 5
  ConsistentAttackMessages: true
  PVP: "limited"                   # enabled, disabled, limited, off
  PVPMinimumLevel: 10
  XPScale: 100.0
  MobConverseChance: 25
```

### File Paths Configuration
```yaml
FilePaths:
  DataFiles: "_datafiles"
  PublicHtml: "_datafiles/html/public"
  AdminHtml: "_datafiles/html/admin"
  HttpsCertFile: "cert.pem"
  HttpsKeyFile: "key.pem"
  WebCDNLocation: "/static"
  CarefulSaveFiles: true
```

## Configuration Types

### Basic Types
```go
type ConfigInt int           // Integer values with validation
type ConfigUInt64 uint64     // Unsigned 64-bit integers
type ConfigString string     // String values
type ConfigSecret string     // Automatically redacted strings
type ConfigFloat float64     // Floating-point values
type ConfigBool bool         // Boolean values
type ConfigSliceString []string  // String arrays
```

### Type Interface
```go
type ConfigValue interface {
    String() string    // String representation
    Set(string) error  // Set value from string
}
```

### Secret Type Behavior
```go
// ConfigSecret automatically redacts in output
func (c ConfigSecret) String() string {
    return `*** REDACTED ***`
}

// Access actual value through helper function
func GetSecret(v ConfigSecret) string {
    return string(v)
}
```

## Configuration Access Patterns

### Reading Configuration
```go
// Get complete configuration
config := configs.GetConfig()

// Access subsections
serverConfig := configs.GetServerConfig()
networkConfig := configs.GetNetworkConfig()
gameplayConfig := configs.GetGamePlayConfig()

// Access specific values
mudName := config.Server.MudName.String()
maxConnections := int(config.Network.MaxTelnetConnections)
pvpEnabled := config.GamePlay.PVP.String() == "enabled"
```

### Setting Configuration Values
```go
// Set configuration value by dot path
err := configs.SetVal("Server.MudName", "New Server Name")
err := configs.SetVal("Network.HttpPort", "8080")
err := configs.SetVal("GamePlay.PVP", "enabled")

// Configuration is automatically validated and saved
if err != nil {
    log.Printf("Configuration error: %v", err)
}
```

### Environment Variable Integration
```go
// Configuration fields can be populated from environment variables
type Server struct {
    DatabaseURL ConfigSecret `yaml:"DatabaseURL" env:"DATABASE_URL"`
    APIKey      ConfigSecret `yaml:"APIKey" env:"API_KEY"`
}

// Values are automatically loaded from environment on startup
```

## Override System

### Override File Format
```yaml
# config-overrides.yaml
Server:
  MudName: "Development Server"
  MaxCPUCores: 8
Network:
  HttpPort: 8080
  HttpsPort: 8443
GamePlay:
  PVP: "enabled"
  XPScale: 150.0
```

### Dot-Notation Access
```go
// All configuration paths support dot notation
allConfig := config.AllConfigData()
// Returns map with keys like:
// "Server.MudName" -> "My MUD Server"
// "Network.HttpPort" -> 80
// "GamePlay.PVP" -> "limited"

// Set values using dot notation
configs.SetVal("Server.MudName", "New Name")
configs.SetVal("GamePlay.Death.EquipmentDropChance", "0.1")
```

### Path Resolution and Correction
```go
// Automatic path correction for typos
fullPath, typeName := configs.FindFullPath("mudname")
// Returns: "Server.MudName", "configs.ConfigString"

fullPath, typeName := configs.FindFullPath("httpport")
// Returns: "Network.HttpPort", "configs.ConfigInt"

// Supports partial matches and case-insensitive lookup
```

## Validation System

### Per-Subsection Validation
```go
func (s *Server) Validate() {
    if s.Seed == `` {
        s.Seed = `Mud` // default value
    }
    
    if s.MaxCPUCores < 0 {
        s.MaxCPUCores = 0 // enforce minimum
    }
}

func (n *Network) Validate() {
    if n.MaxTelnetConnections < 1 {
        n.MaxTelnetConnections = 50 // default
    }
    
    if n.HttpPort < 0 {
        n.HttpPort = 0 // disable if negative
    }
}
```

### Banned Name Validation
```go
// Check if a name matches banned patterns
bannedPattern, isBanned := config.IsBannedName("testname")
if isBanned {
    return fmt.Errorf("Name matches banned pattern: %s", bannedPattern)
}
```

### Locked Configuration Properties
```go
// Some properties cannot be changed at runtime
func isEditAllowed(configPath string) bool {
    serverConfig := configs.GetServerConfig()
    for _, lockedPath := range serverConfig.Locked {
        if configPath == lockedPath {
            return false
        }
    }
    return true
}
```

## Configuration Loading and Persistence

### Startup Process
1. **Load Base Configuration**: Read `_datafiles/config.yaml`
2. **Load Overrides**: Read `config-overrides.yaml` if it exists
3. **Apply Environment Variables**: Set values from environment
4. **Validate Configuration**: Run all validation functions
5. **Build Lookup Tables**: Create path and type lookup maps

### Runtime Updates
```go
// Configuration changes are immediately persisted
err := configs.SetVal("Server.MudName", "New Name")
// This automatically:
// 1. Validates the new value
// 2. Updates the override file
// 3. Reloads the configuration
// 4. Validates the complete configuration
```

### Configuration Reload
```go
// Reload configuration from files
err := configs.ReloadConfig()
if err != nil {
    log.Printf("Failed to reload config: %v", err)
}
```

## Integration Examples

### User Command Integration
```go
// Server configuration command
func server_Config(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
    args := util.SplitButRespectQuotes(rest)
    
    if len(args) >= 2 {
        configName := strings.ToLower(args[0])
        configValue := strings.Join(args[1:], ` `)
        
        if err := configs.SetVal(configName, configValue); err != nil {
            user.SendText(fmt.Sprintf("Config error: %s=%s (%s)", configName, configValue, err))
            return true, nil
        }
        
        user.SendText(fmt.Sprintf("Configuration updated: %s=%s", configName, configValue))
        return true, nil
    }
    
    // Show current configuration
    allConfigData := configs.GetConfig().AllConfigData()
    // Display configuration options...
}
```

### Web Interface Integration
```go
// Template access to configuration
templateData := map[string]any{
    "CONFIG": configs.GetConfig(),
    "STATS":  GetStats(),
}

// In templates:
// {{.CONFIG.Server.MudName}}
// {{.CONFIG.Network.HttpPort}}
// {{.CONFIG.GamePlay.PVP}}
```

### Plugin Configuration
```go
// Plugin-specific configuration
func (p *PluginConfig) Set(name string, val any) {
    configs.SetVal(fmt.Sprintf(`Modules.%s.%s`, p.pluginName, name), fmt.Sprintf(`%v`, val))
}

func (p *PluginConfig) Get(name string) any {
    m := configs.Flatten(configs.GetModulesConfig())
    return m[fmt.Sprintf(`%s.%s`, p.pluginName, name)]
}
```

## Performance Considerations

### Configuration Caching
- Configuration is cached in memory with read-write mutex protection
- Changes trigger validation and persistence but don't require full reload
- Lookup tables provide O(1) access to configuration paths and types

### Thread Safety
```go
var configDataLock sync.RWMutex

func GetConfig() Config {
    configDataLock.RLock()
    defer configDataLock.RUnlock()
    
    if !configData.validated {
        configData.Validate()
    }
    return configData
}
```

### Validation Optimization
- Validation only runs when configuration changes
- Cached validation state prevents redundant validation calls
- Subsection validation allows targeted updates

## Security Features

### Secret Management
```go
// Secrets are automatically redacted in logs and output
type Server struct {
    DatabasePassword ConfigSecret `yaml:"DatabasePassword" env:"DB_PASSWORD"`
    APIKey          ConfigSecret `yaml:"APIKey" env:"API_KEY"`
}

// Access actual values securely
dbPassword := configs.GetSecret(config.Server.DatabasePassword)
```

### Configuration Locking
```go
// Prevent runtime changes to sensitive configuration
Server:
  Locked: ["Seed", "DatabasePassword", "APIKey"]
```

### Input Validation
```go
// Validate user input against banned patterns
Validation:
  BannedNames: ["admin*", "root", "system", "*test*"]
```

## Error Handling and Logging

### Configuration Errors
- Type conversion errors with detailed messages
- Path resolution errors with suggestions for correct paths
- Validation errors with specific constraint information
- File I/O errors with full context

### Logging Integration
```go
// Configuration changes are logged
mudlog.Info("SetVal", "path", propertyPath, "value", newVal, "success", true)
mudlog.Error("SetVal", "path", propertyPath, "error", err)
```

## Dependencies

- `gopkg.in/yaml.v2` - YAML parsing and generation
- `internal/mudlog` - Logging and monitoring
- `internal/util` - File operations and utilities
- `sync` - Thread-safe access control
- `reflect` - Dynamic configuration introspection
- `os` - Environment variable access and file operations

## Usage Examples

### Dynamic Configuration Updates
```go
// Update server settings at runtime
configs.SetVal("Server.MudName", "Production Server")
configs.SetVal("Network.MaxTelnetConnections", "100")
configs.SetVal("GamePlay.XPScale", "75.0")

// Changes are immediately validated and persisted
```

### Configuration Validation
```go
// Custom validation in subsections
func (g *GamePlay) Validate() {
    if g.XPScale <= 0 {
        g.XPScale = 100.0 // default
    }
    
    if g.PVP != "enabled" && g.PVP != "disabled" && g.PVP != "limited" {
        g.PVP = "limited" // default
    }
    
    if g.MaxAltCharacters < 0 {
        g.MaxAltCharacters = 5 // default
    }
}
```

### Environment Variable Integration
```go
// Automatic environment variable loading
os.Setenv("DATABASE_URL", "postgres://localhost/muddb")
os.Setenv("API_KEY", "secret-api-key")

// Configuration automatically loads these values
config := configs.GetConfig()
dbURL := configs.GetSecret(config.Server.DatabaseURL)
apiKey := configs.GetSecret(config.Server.APIKey)
```

This configuration system provides a robust foundation for managing all aspects of GoMud server configuration with type safety, validation, security, and runtime flexibility.

---

## DOGMud Balance Configuration Knobs

The `Balance` subsection of the config holds all gameplay-tuning constants.
Additions by chunk are documented below.

### Weapon Reach (chunk 4c)

Three knobs control how harshly long weapons are penalised in grapples.
All live under `Balance` in `_datafiles/config.yaml`.

| Knob | Default | Effect |
|------|---------|--------|
| `ReachStandingGrappleRadius` | 0.5 | Effective radius (m) for Clinch / BackStanding. Weapons longer than this are penalised. |
| `ReachGroundGrappleRadius` | 0.3 | Effective radius (m) for ground grapple states (Mount, Guard, etc.). Tighter than standing. |
| `ReachUtilityFloor` | 0.15 | Minimum damage multiplier from the reach curve. Prevents long weapons from doing literal zero damage. |

See `internal/combat/context.md` "Weapon Reach Utility" for the full
formula and exempt subtype list.

### Submission System (chunk 4d)

Four knobs control when submission windows open and how tiers are
resolved. All live under `Balance` in `_datafiles/config.yaml`. (U6b
Task 13 deleted the sub-only skill weight knob — both sides of the sub
roll now use the global `SkillWeight` — and the sub crit z-threshold:
the stun-crit tier is a margin crit vs `CritBarFor` and has no z knob.)

| Knob | Default | Effect |
|------|---------|--------|
| `SubmissionAttemptAlpha` | 1.0 | Min drift-margin z-score (absolute) for a sub window to open on either side of the grapple. |
| `SubmissionAttemptCritZ` | 2.0 | Defender drift z >= this opens a bottom-sub window regardless of margin (the crit shortcut). |
| `SubBadZThreshold` | -1.0 | Sub-roll z-score below which the bad tier fires (attempter falls Prone, grapple breaks). |
| `SubGoldLossFraction` | 0.20 | Fraction of defender's carried gold transferred to attacker on subdue/cripple outcomes. |

See `internal/combat/context.md` "Submission System (chunk 4d)" for the
full resolution flow and policy matrix.

### NPC Schedules (chunk 3.2)

| Knob | Default | Effect |
|------|---------|--------|
| `ScheduleMaxPathRetries` | 20 | After N consecutive failed `pathto` attempts, a scheduled mob falls back to `pathto home`. Chunk 3.2. Also governs patrol path retries (chunk 3.4 reuses the same threshold; no separate knob). |
| `SleepRegenMultiplier` | 5.0 | HP/SP/CP per-round regen multiplier when bearer has the Sleeping buff. Chunk 3.3. |
| `ScheduleWakeGraceRounds` | 50 | After a forced wake during a sleep segment, suppress re-sleep for N rounds (~200 sec real-time at default tick rate). Chunk 3.3. |
| `ConversationBaseChancePct` | 1.0 | Per-tick % chance a fully-idle NPC attempts to start an idle conversation. Chunk 3.6. |
| `ConversationPlayerArrivalBoostPct` | 25 | On player arrival in a room with relateable, idle NPCs, % chance to start one. Chunk 3.6. |
| `ConversationCooldownRounds` | 50 | Cooldown applied to both NPCs after a conversation completes. Chunk 3.6. |

### Ranged Combat (ranged-weapons feature)

| Knob | Default | Effect |
|------|---------|--------|
| `RangedShotScale` | 1.0 | Global multiplier on all ranged shot damage. Tune up/down to adjust ranged vs melee balance without touching individual weapon stats. |

The flat ranged shield-bonus knob was deleted by U6b Task 8: a shielded
defender now answers a shot with a real block CONTEST entry via the channel
defence seam (`combat.DefenceEntriesFor`), not a score addend.

### Surprise-attack and ranged damage multipliers (U10d)

Three `Balance` knobs, added by the U10d surprise-attack redesign
(`config.balance.go`; defaults enforced in `config.balance.combat.go`).
The redesign also **deleted** five now-nonexistent knobs that may still
turn up in stale notes or old branches:
`SurpriseAttackOffhandPenalty` and `SurpriseAttackExtraArm1Penalty`
through `...ExtraArm4Penalty` — none of the five exist in the config
struct any more.

| Knob | Go default | Shipped in `config.yaml` | Effect |
|------|-----------|---------------------------|--------|
| `SurpriseOpeningStrikeMultiplier` | 1.0 | 1.0 | Extra multiplier on a MELEE surprise opening strike. |
| `SurpriseRangedStrikeMultiplier` | 1.0 | 0.5 | The ranged counterpart — deliberately lower, since the ranged opener also inherits `RangedUnengagedDamageMultiplier`. |
| `RangedUnengagedDamageMultiplier` | 1.0 | 2.75 | Ranged damage multiplier applied when nothing in the room currently targets the shooter. |

All three validate to their Go default (1.0) when shipped at or below
zero (`b.SurpriseOpeningStrikeMultiplier <= 0`, etc., in
`config.balance.combat.go`). The `<= 0` shape is deliberate — see the
validator-shape gotcha below.

### Gotcha: a `< 0 || > 1.0` validator can never apply its own default

Balance validators are written in two shapes, and only one of them works
for a knob whose legitimate range includes zero:

```go
if b.X <= 0        { b.X = <default> }   // safe
if b.X < 0 || b.X > 1.0 { b.X = <default> }   // TRAP
```

`yaml.Unmarshal` is non-strict, so a key **absent** from
`_datafiles/config.yaml` leaves the field at Go's zero value. Zero is
neither negative nor above 1.0, so the second shape's branch never runs
and the knob stays **inert at 0.0** — not at the default its own comment
advertises. This is not hypothetical: it is exactly how the five
`SurpriseAttack*Penalty` knobs deleted by U10d came to be 0.0, which made
every limb of the old surprise volley an unconditional auto-hit for the
whole life of the feature. The dangerous combination to grep for is
**non-zero advertised default + `< 0 || > 1.0` validator + no key in
`config.yaml`.**

A sweep on 2026-08-25 found no live instance remaining. Both surviving
users of that shape are safe for reasons specific to them:
`MinAttackCritChance` / `MinDefenseCritChance`
(`config.balance.combat.go:330,333`) treat 0 as a deliberate off-switch
and are written explicitly in `config.yaml`; `EquipmentDropChance`
(`config.gameplay.go:65`) has a coded default of 0.0. **Correction on
record:** a U10d review reported `SubGoldLossFraction` as a live instance.
It is not — `sub_gold_loss_fraction: 0.20` is present at
`_datafiles/config.yaml:925` under `Balance:`, matching the field's yaml
tag, so the subdue/cripple gold transfer
(`internal/combat/submission_outcome.go:201`) runs at the intended value.
Do not "fix" it.

### Gotcha: the special-move cooldown comments are stale in two places

`SpecialMoveCooldown` (`config.balance.go:246`) is commented
`// Shared cooldown rounds for bash/trip/kick (default 5)`, and
`_datafiles/config.yaml:660-664` says bash, trip, kick and cast. Neither
is accurate: **44 non-test `.go` files under `internal/` and `modules/`
claim the `special-move` key**, and U10d added the melee and ranged
ambush openers to that population. The `config.yaml` comment is the worse
of the two, because it is what a tuner reads. Both are deliberately left
as-is for now — `config.yaml` carries `skip-worktree` and needs the
commit-from-`git show HEAD:` procedure, and correcting only the Go half
would leave the pair still disagreeing. Handed to **U11**'s config audit;
see the "U11 inbox from U10d" section of
`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`.

### Unified action-cost admission (U8)

These are raw config inputs before `costs.Calc`; they are not final charges.
Validation rejects missing, non-positive, NaN, and infinite values. A
non-positive action modifier remains the calculator's neutral `1.0` behavior.

| Knob | Actions |
|------|---------|
| `ShootBaseStaminaCost` | Shoot |
| `ReloadBaseStaminaCost` | Reload |
| `SpecialMoveBaseStaminaCost` | Bash, trip, kick, grapple initiation, hamstring, rake, maul, pounce, gore, drain, throttle, throw |
| `SneakBaseStaminaCost` | Sneak |
| `RhetoricActionBaseConvictionCost` | Taunt, rally, warcry |
| `GrappleStaminaCostPerRound` | Ongoing grapple maintenance before controller/controlled role multiplier |

`GrappleStaminaCostPerRound` is a `ConfigFloat`; grapple initiation uses the
special-move base instead. Physical rows add encumbrance, every row applies the
inverse governing-skill term, and callers may supply a documented modifier.
See the live config and validation code for tuning values.

## Files

Config is split one file per section, all assembled in `configs.go`.

| File | Section |
|------|---------|
| `configs.go` | Assembly, load/save, `GetConfig`, overrides plumbing |
| `config_types.go` | Shared config value types |
| `overrides.go` | `CONFIG_PATH` override-file layering |
| `discovery.go` | Reflection-based knob discovery |
| `testing_support.go` | Test helpers |
| `config.server.go` | Server identity and version |
| `config.network.go` | Ports, timeouts, connection limits |
| `config.filepaths.go` | Data file locations |
| `config.logging.go` | Log level, file, rotation |
| `config.timing.go` | Round length, rounds per day |
| `config.gameplay.go` | Gameplay knobs (map enforcement, petitions, …) |
| `config.balance.go` | Balance root |
| `config.balance.combat.go` | Combat maths, mitigation caps, defence floor |
| `config.balance.progression.go` | Stat/skill progression |
| `config.balance.spells.go` | Spell scaling |
| `config.balance.shops.go` | Shop pricing and restock |
| `config.balance.mobs.go` | Mob scaling |
| `config.balance.discovery.go` | Discovery/offset mechanics |
| `config.balance.misc.go` | Everything else in Balance |
| `config.roles.go` | Role definitions |
| `config.modules.go` | Per-module config bags |
| `config.integrations.go` | Discord and other integrations |
| `config.llm.go` | Ollama/LLM settings |
| `config.analytics.go` | Analytics |
| `config.memory.go` | Memory reporting |
| `config.textformats.go` | Text formatting |
| `config.translation.go` | Translation |
| `config.specialrooms.go` | Special room ids |
| `config.lootgoblin.go` | Loot goblin |
| `config.validation.go` | Cross-section validation |

**`_datafiles/config.yaml` has `skip-worktree` set** in this repository. `git
add` will refuse it with a misleading "sparse-checkout" message — unset the
bit, stage your hunks, then set it again.

**Override files are flat dot-separated keys** (`Network.TelnetPort: [33334]`)
layered over `config.yaml` via `CONFIG_PATH`. That is how the pre-push boot
test runs on alternate ports without touching the tracked config.
