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
- `Config.DisplayConfigData` is the only view a person may read (see "Locks and the redacted view")
- Environment variable support for secure credential injection
- `SetVal` refuses locked keys: `Server.Locked` plus the Go hard list `hardLocked`
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
  Death:
    CorpseDecayRounds: 100
  ShopRestockRate: "1h"
  ContainerSizeMax: 50
  MaxAltCharacters: 5
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

### APIFramework Configuration (`config.apiframework.go`)
The server's model API key, endpoint, daily token budget and breaker, shared
by every feature that calls a model (`internal/apiframework`, used by the AI
companion and bauble naming). `GetAPIFrameworkConfig()` returns it;
`Validate()` only tidies the strings and sets no numeric defaults: an absent
key decodes as zero, which `apiframework.Server` reads as "not set here" and
fills from the companion's old `Modules.aicompanion` settings, then the old
defaults. A negative `DailyTokenBudget` is no cap. `AllowCustomEndpoint` is
a `ConfigString`, not a `ConfigBool`, so an explicit "false" can override
the companion's old true (a bool cannot tell false from absent); `Validate`
makes it "true", "false" or empty, and anything unreadable is "false".
```yaml
APIFramework:
  APIKeyEnv: "OPENAI_API_KEY"
  APIKey: ""
  BaseURL: "https://api.openai.com/v1"
  AllowCustomEndpoint: ""   # "true", "false", or empty to inherit
  DailyTokenBudget: 2000000
  BreakerErrors: 5
  BreakerSeconds: 60
```

Bauble loot's balance knobs are `Balance.Bauble*` (`config.balance.baubles.go`),
with `Balance.BaublesEnabled` false by default.

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
// All configuration paths support dot notation. AllConfigData is RAW (lookups
// only); anything a person reads uses DisplayConfigData.
allConfig := config.DisplayConfigData()
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

## Locks and the redacted view

**Locks (`config_locks.go`).** `IsLocked(path)` is true when the path is on
`hardLocked` (exact, lowercase), ends in `locked`, or starts (lowercase) with
an entry of `Server.Locked`. `hardLocked` holds the security keys no in-game
command may change whatever `Server.Locked` says: `APIFramework.APIKey`,
`.APIKeyEnv`, `.BaseURL`, `.AllowCustomEndpoint` (the section is absent on
master; the entries cost nothing), `Modules.aicompanion.APIKey`, `.APIKeyEnv`,
`.BaseURL`, `.AllowCustomEndpoint`, `.RelayOrigin`, `.PlayerKeys`, `.Model`,
`.FastModel`, `.DeepModel`, `.ModerateOutput`, `.ModerationModel`,
`Modules.baubles.Model`, `.MaxCompletionTokens`, `.MaxConcurrent`,
`.UsePlayerKeys`, `.ModerateOutput`, `.ModerationModel`,
`FilePaths.WebDomain`, `Server.Locked`, and `Integrations.Discord.WebhookUrl`
(where server data is sent). `APIFramework.APIKey` is a `ConfigSecret`.

- `SetVal` is the OPERATOR write (`server set`, the `server config` menu,
  `setmotd`, `plugins.PluginConfig.Set`). It resolves the key with
  `FindFullPath`, then refuses with `ErrLockedConfig` when `IsLocked` names the
  RESOLVED path, before the unknown-key check. A bare suffix key (`seed`)
  cannot slip past a full-path lock.
- `SetEngineVal` is the ENGINE write for values the server maintains itself
  that `Server.Locked` keeps from operators: `Server.CurrentVersion`
  (`internal/migration`) and `Server.NextRoomId` (`internal/rooms`). It skips
  `Server.Locked` and still refuses `hardLocked`. A new engine-owned key in
  the shipped lock list must use it, or its write is silently refused.
- Both refuse `RedactedValue` as a value (`ErrRedactedValue`).
- `usercommands.isEditAllowed` delegates to `IsLocked`.

**Redacted view (`config_display.go`).** `AllConfigData` returns values raw
and is for the lookups only (`buildKeyLookups`). `DisplayConfigData` takes the
same exclusion patterns and replaces with `RedactedValue` every `ConfigSecret`
and every value `isSecretConfigValue` names: the check runs `secretNameRule`,
a case-insensitive suffix match of `apikey`, `api_key`, `secret`, `password`,
`token`, `webhookurl`, `secretkey` or `privatekey`, against EVERY element of
the dotted path, not only the leaf, and also deep-scans a stored slice or map
value (`containsSecret`) for a nested `ConfigSecret` or a map key matching the
same rule, since `buildDotPaths` stores a slice or map whole and never
recurses into it. The scan caps at depth 32 and fails closed past the cap,
reporting the value as secret rather than risk printing one. Module settings
are untyped (`Modules map[string]any`), so a module's key is a plain string
and only its name marks it. The boot log (`logBootConfig` in
`boot_config_log.go`), the `server set` listing, the `server config` menu and
`/viewconfig` all read `DisplayConfigData`; the root test
`config_display_guard_test.go` (`TestNoDisplayReadsRawConfig`) fails on any
`AllConfigData`, `DotPaths` or `GetOverrides` call outside this package, and
on any page template that reaches `Modules` through `.CONFIG` or `getconfig`.

**Tests.** `SetConfigWithLookupsForTest(t, c) string` installs `c` with real
lookups (a test binary never runs `ReloadConfig`, so without it every key is
"unknown" and a lock test passes for the wrong reason), snapshots `overrides`,
and points `CONFIG_PATH` at a scratch file whose path it returns.

**Known gap, not fixed here.** `ReloadConfig` builds the lookups from the
config BEFORE the load, so on a single boot module keys without a data
overlay (the aicompanion's) do not resolve and `SetVal` refuses them as
unknown.

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

See "Locks and the redacted view" above for the current lock rule
(`configs.IsLocked`, `configs.SetVal`, `configs.SetEngineVal`) and the
redacted display view (`configs.DisplayConfigData`). `usercommands.isEditAllowed`
now delegates to `IsLocked` rather than walking `Server.Locked` itself.

## Configuration Loading and Persistence

### Startup Process
1. **Load Base Configuration**: Read `_datafiles/config.yaml`
2. **Load Overrides**: Read `config-overrides.yaml` if it exists, from `$CONFIG_PATH` or else `<DataFiles>/config-overrides.yaml`, where DataFiles is the value step 1 just loaded (`overridePathFor`); `SetVal` writes to the same path
3. **Apply Environment Variables**: Set values from environment
4. **Validate Configuration**: Run all validation functions
5. **Build Lookup Tables**: Create path and type lookup maps

### Runtime Updates
```go
// Configuration changes are immediately persisted. SetVal refuses locked keys
// (ErrLockedConfig); engine-owned locked keys use SetEngineVal.
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
    allConfigData := configs.GetConfig().DisplayConfigData()
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
| `SleepRegenMultiplier` | 5.0 | HP/SP/CP per-round regen multiplier when bearer has the Sleeping condition. Chunk 3.3. |
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

**Third shape, and a deliberate exemption:**
`ProgressionFailureFraction` (`config.balance.progression.go:46`) is written
`< 0 || > 1.0` with a non-zero default of 0.35 and looks like a live instance
of exactly the trap above. It is not, and **it must not be "fixed" to `<= 0`.**
0 is a legal explicit off-switch there ("a lost resolved action teaches
nothing"), so a `<= 0` guard would make that setting unreachable. What saves
the absent-key case is that the two are separated *before* the unmarshal
instead of after it: `newUnloadedConfig` (`configs.go:411`) pre-seeds `-1`, and
`loadConfig` is the only path that decodes a config document, so a value still
negative at validation time means "absent" and nothing else. `newUnloadedConfig`
is also the initializer for the package-level `configData`, so a `GetConfig()`
read landing before `ReloadConfig()` finishes resolves to the default rather
than to a bare 0.
`TestProgressionFailureFraction_AbsentKeyLoadsTheDefault` goes red if that
wiring is removed.

**The general rule this establishes:** the guard shape alone is not enough
information to judge a knob. Any knob whose legitimate range **includes zero**
AND whose absent-key default is **non-zero** needs the pre-unmarshal sentinel,
because no predicate applied after the unmarshal can tell an explicit 0 from an
absent key. Before flagging a `< 0 || > 1.0` validator, check whether the field
is seeded in `newUnloadedConfig`; if it is, the shape is correct and load-bearing.

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

### Graded room lighting (plans 1, 2, 3a, 5a, 5b and 5c of the graded lighting arc)

Thirteen knobs, validated in their own file (`config.balance.lighting.go`)
rather than folded into `validateMisc`, because the arc kept adding more
here across plans: plan 1 shipped the three band thresholds, plan 2 added
the vision-strength fallback, plan 3a added the eight knobs that turn the
sky itself into a solar and lunar model, and plan 5b added the dazzle
edge. Twelve of the thirteen are absent from `_datafiles/config.yaml`, so
the shipped value is the Go default in every case; `LightDazzleAbove` is
the exception, shipped in the file at its default of 75.

| Knob | Type | Default | Effect |
|------|------|---------|--------|
| `LightBlindBelow` | ConfigInt | 25 | Below this, a normal observer is blind. |
| `LightDimBelow` | ConfigInt | 50 | Below this, a normal observer reads shapes only. |
| `LightExitsAbove` | ConfigInt | 65 | At or above this, exits into adjacent rooms are visible. |
| `LightDazzleAbove` | ConfigInt | 75 | Where the comfortable band ends and too-bright begins for a normal observer; a vision ability moves it down by its strength. Plan 5b. |
| `LightDefaultVisionStrength` | ConfigInt | 12 | Window shift (`internal/messaging.SightThroughWindow`'s `strength`) for a vision flag that declares no strength of its own. Plan 2. |
| `LightDoublingStep` | ConfigFloat | 8 | Scale points per doubling of physical light; the one constant `internal/lightscale.Combine`/`Attenuate` take as `step`. Plan 3a. |
| `WorldLatitude` | ConfigFloat | 46.5 | Degrees north; the world's ONLY seasonal input (declination, day length, sunrise, sunset, noon height all derive from it). Plan 3a. |
| `LightEquinoxNoon` | ConfigFloat | 70 | Calibration anchor: the sky's light at noon on an equinox. Plan 3a. |
| `LightStarlight` | ConfigFloat | 10 | Sky light with every moon new. Plan 3a. |
| `LightMoonsFull` | ConfigFloat | 35 | Sky light with every moon full. Plan 3a. |
| `LightMoonWeightSwiftmoon` | ConfigFloat | 4.0 | Swiftmoon's relative light at full. Plan 3a. |
| `LightMoonWeightWanderer` | ConfigFloat | 1.0 | The Wanderer's relative light at full (the baseline). Plan 3a. |
| `LightMoonWeightEye` | ConfigFloat | 0.5 | The Eye's relative light at full. Plan 3a. |

**Light-spell scaling (lighting plan 5a).** Six more `ConfigFloat` knobs,
validated in the same `validateLighting` and exposed on `configs.Lighting` as
`SpellStrengthBase`/`SpellStrengthStatDivisor`/`SpellStrengthSkillDivisor` and
`SpellDurationBase`/`SpellDurationStatDivisor`/`SpellDurationSkillDivisor`.
`internal/hooks.magnitudeSpellApplication` (renamed from
`lightSpellApplication` in lighting plan 5c, when it generalised past light;
see "Vision-spell scaling and infravision" below) casts a `light_strength:
magnitude` condition at `StrengthBase + stat/StrengthStatDivisor +
skill/StrengthSkillDivisor` for `DurationBase + stat/DurationStatDivisor +
skill/DurationSkillDivisor` triggers (rounded, at least 1). Unlike the twelve
above, these six DO ship in `_datafiles/config.yaml`, at their defaults. Any
value not above 0 is coerced to its default (a zero divisor would divide by
zero, and a test binary never loads `config.yaml`).

| Knob | Default |
|------|---------|
| `LightSpellStrengthBase` | 40 |
| `LightSpellStrengthStatDivisor` | 10 |
| `LightSpellStrengthSkillDivisor` | 2 |
| `LightSpellDurationBase` | 2 |
| `LightSpellDurationStatDivisor` | 50 |
| `LightSpellDurationSkillDivisor` | 20 |

**Vision-spell scaling and infravision (lighting plan 5c).** Eight more
knobs, validated in the same `validateLighting`. Six follow the light trio's
own idiom (base + stat/StatDivisor + skill/SkillDivisor), one pair per scaled
kind, exposed on `configs.Lighting` as `NightVisionSpellBase`/
`NightVisionSpellStatDivisor`/`NightVisionSpellSkillDivisor` and
`InfraSpellBase`/`InfraSpellStatDivisor`/`InfraSpellSkillDivisor`.
`magnitudeSpellApplication` (through `conditions.SpellScaledMagnitude`, which
the admin `setcondition` command also calls at a new character's stat 100 and
skill 0) picks the trio matching `ConditionSpec.ScaledKind()` and shares the light trio's `SpellDuration*` knobs for all three kinds'
duration. The other two are infravision's own: `LightInfraReachCap`
(`ConfigInt`, bounds every infra-reach source's combined total — spell,
potion, mutation, condition) and `LightInfraPenaltyFloor` (`ConfigFloat`, the
sight multiplier infravision gives at its first point of reach, rising
linearly to no penalty at the cap). All eight ship in `_datafiles/config.yaml`
at their defaults, in the "LIGHT: VISION SPELLS AND INFRAVISION" block after
the light trio's own.

| Knob | Type | Default |
|------|------|---------|
| `LightNightVisionSpellBase` | ConfigFloat | 4 |
| `LightNightVisionSpellStatDivisor` | ConfigFloat | 12.5 |
| `LightNightVisionSpellSkillDivisor` | ConfigFloat | 6.5 |
| `LightInfraSpellBase` | ConfigFloat | 5 |
| `LightInfraSpellStatDivisor` | ConfigFloat | 7 |
| `LightInfraSpellSkillDivisor` | ConfigFloat | 3 |
| `LightInfraReachCap` | ConfigInt | 50 |
| `LightInfraPenaltyFloor` | ConfigFloat | 0.90 |

`LightInfraReachCap` coerces to 50 outside `(0, 100]` (a cap of zero divides
by zero in the penalty ramp; above 100 reaches past the scale).
`LightInfraPenaltyFloor` coerces to 0.90 outside `(0, 1.0]` (it is a
multiplier). `configs.Lighting` also carries `DarkCap`, which is not its own
knob: it is `Balance.DarknessCombatPenalty` (the COMBAT: DARKNESS knob below)
copied onto `Lighting`, because `internal/messaging.infraDarkCap` expresses
the infravision penalty as a dark fraction against it and reads it off the
narrow `Lighting` struct rather than the 400-field `Balance` copy.

**Darkness-spell scaling (lighting plan 5d).** Six more knobs in
`validateLighting`, the light trio's idiom on their own names so darkness can
be retuned apart from light (owner ruling D9): the yaml keep the `Light`
family prefix, and `configs.Lighting` drops it (`DarknessSpellStrengthBase`,
`DarknessSpellStrengthStatDivisor`, `DarknessSpellStrengthSkillDivisor`,
`DarknessSpellDurationBase`, `DarknessSpellDurationStatDivisor`,
`DarknessSpellDurationSkillDivisor`). `conditions.SpellScaledMagnitude` reads
the strength trio and `conditions.SpellScaledTriggers` the duration trio for
a `darkness_strength: magnitude` condition (Chrysalis Pall, 131). Zero or
negative reverts to the default. All six ship in `_datafiles/config.yaml` at
glow's values, in the "LIGHT: DARKNESS SPELL (lighting plan 5d)" block after
the 5c block.

| Knob | Type | Default and shipped |
|------|------|---------|
| `LightDarknessSpellStrengthBase` | ConfigFloat | 40 |
| `LightDarknessSpellStrengthStatDivisor` | ConfigFloat | 10 |
| `LightDarknessSpellStrengthSkillDivisor` | ConfigFloat | 2 |
| `LightDarknessSpellDurationBase` | ConfigFloat | 2 |
| `LightDarknessSpellDurationStatDivisor` | ConfigFloat | 50 |
| `LightDarknessSpellDurationSkillDivisor` | ConfigFloat | 20 |

`LightBlindBelow` and `LightDimBelow` validate as a PAIR, the
`LightStarlight`/`LightMoonsFull` precedent below: an inverted or
out-of-range pair reverts both to their defaults rather than leaving one
knob correct and the other wrong. Zero is deliberately coerced rather than
honoured for these two, unlike `SneakFailCooldown`'s honoured zero.
`LightExitsAbove` validates on its own range plus one cross-axis rule, that
it must not sit below `LightBlindBelow`.

`LightDazzleAbove` (plan 5b) validates in `validateLighting` on its own
range and one cross-axis rule, that it must sit above `LightDimBelow` and
at most 100; an invalid value falls back to one above the current
`LightDimBelow`, or to its shipped default of 75 if that fallback would
itself be out of range. It is where the observer's comfortable band ends
and the dazzle ramp begins; a vision ability's strength shifts the whole
band, dazzle edge included, down by that much.

`DarknessCombatPenalty` and `DazzleCap` (both `ConfigFloat`, both COMBAT:
DARKNESS knobs validated in `config.balance.combat.go`, not this file) are
the ramp's two caps: `DarknessCombatPenalty` (default 0.80) at and below
the blind edge, `DazzleCap` (default 0.80) one ramp-width past the dazzle
edge and beyond. `internal/messaging.SightScoreMultiplier` reads both to
build the linear ramp between them. Plan 5b retired the two knobs these
replaced, `DarknessShapesCombatPenalty` and `DarknessScoreMultiplier`
(the flat three-verdict penalty); `DarknessCombatPenalty`'s yaml key name
is kept for config compatibility even though its meaning changed from a
flat penalty to a ramp cap. See `internal/combat/context.md`'s "Sight: the
verdict and the ramp" section for how the ramp is read.

⚠️ **`internal/rooms.LightDark`/`LightRoomOnly`/`LightFull` are GONE.** Plan
1 shipped them as a bridge from the old three-value visibility model onto
the graded scale; plan 3a deleted them along with `legacyVisibility()`
once `Room.LightLevel()` composed light from the sky, a lamp and carried
light directly. A stale copy of this file once called them "load-bearing
against these defaults"; they are not consulted anywhere any more.

`LightDefaultVisionStrength` (plan 2) clamps to `[0, 24]` first, then zero
(whether authored directly or reached by clamping a negative) defaults to
12, following the `ProgressMult` idiom where zero means "unset", not "shift
by nothing", so the effective authored range is `[1, 24]`. The upper bound
is the exported constant `LightWindowShiftCap` (24,
`config.balance.lighting.go`), which `windowShiftCap` in
`internal/messaging/window.go` is defined from, and which the spell and
potion magnitude caps (`conditions.SpellScaledMagnitude`,
`items.PotionMagnitudeApplication`) also read, so there is one number, not
a duplicated literal. A value above 24 clamps down to it rather than reverting to the
default, honouring the operator's intent (a strong shift) at the strongest
the window model can express, the same way `LightExitsAbove` clamps rather
than reverts for its own out-of-range case above.

`LightDoublingStep` coerces any non-positive value to 8 rather than
honouring it: zero divides by zero inside `internal/lightscale.Combine`.
`WorldLatitude` treats zero as UNSET and coerces it to 46.5, the same idiom
as `LightDefaultVisionStrength`, because Go cannot distinguish an unset
float from an authored zero and none of these knobs ship in
`_datafiles/config.yaml` today: a bare `Balance{}` is what production
actually runs on. Honouring zero would have shipped a world with no
latitude at all, no seasons, and the celestial model never reached. An
operator wanting equator-like twelve-hour nights all year authors a
latitude near zero (e.g. `0.001`), not exactly zero. `LightEquinoxNoon`
also coerces zero, on the same "unset, not a legitimate dark noon"
reasoning. `LightStarlight`/`LightMoonsFull` validate as a pair, the
`LightBlindBelow`/`LightDimBelow` precedent again: starlight at or above
the full-moon value would make a full moon read darker than a new one, so
an invalid pair reverts both. The three moon weights floor negative values
at zero, and if all three land at zero (no span at all for the moon curve)
the whole trio reverts to its defaults.

**`GetLightingConfig() Lighting`** (`config.lighting_accessor.go`) exists
because `GetBalanceConfig()` copies the entire 424-field `Balance` struct
under two read locks to answer a question about one int: measured at 99.75
ns against 8.23 ns for the much smaller `GetTimingConfig()`. Fifteen call
sites were paying that copy, and `Room.LightLevel()` is called from a
per-round loop, so the cost was not incidental. `GetLightingConfig` reads
only the twelve knobs above into a small `Lighting` struct; the knobs stay
declared on `Balance` (the yaml schema is unchanged), only the read path is
narrowed. `internal/gametime`'s `SunLight`, `MoonLight` and
`CelestialLight`, and `internal/rooms`'s `Room.LightLevel()`/`IsLit()`, all
read through this accessor rather than `GetBalanceConfig()`.

### Bleed stacks (conditions unification slice 1b)

Fifteen `ConfigInt` knobs, three per bleed move, in the Bleed stacks block of
`_datafiles/config.yaml` (after `RhetoricActionBaseConvictionCost`). Each
landed rake, maul, hamstring, drain or throttle hit adds one stack to the
target's 122 Bleeding record; `actions.bleedPerRound` turns the knobs into
that stack's amount. Shipped values equal the Go defaults.

| Knob | Default | Effect |
|------|---------|--------|
| `RakeBleedRounds` / `RakeBleedStrengthDivisor` / `RakeBleedMin` | 10 / 50 / 1 | Rake stack: rounds it lasts; per-round health loss is attacker Strength / divisor, never below min |
| `MaulBleedRounds` / `MaulBleedStrengthDivisor` / `MaulBleedMin` | 12 / 35 / 1 | Maul stack, same shape |
| `HamstringBleedRounds` / `HamstringBleedStrengthDivisor` / `HamstringBleedMin` | 12 / 50 / 1 | Hamstring stack, same shape |
| `DrainBleedRounds` / `DrainBleedStrengthDivisor` / `DrainBleedMin` | 10 / 50 / 1 | Drain stack, single and area drain alike |
| `ThrottleBleedRounds` / `ThrottleBleedStrengthDivisor` / `ThrottleBleedMin` | 8 / 33 / 1 | Throttle stack, same shape |

`validateCombat` (`config.balance.combat.go`) replaces any value below 1 with
its default, so an absent key (0) and a negative value both load the default
and the divisor is never zero. `config_bleed_stacks_test.go` pins the
defaults, the legal range, that the shipped file names every key, and
(`TestShippedBleedTuningMeetsTheSliceTargets`) that each stack lasts 2 to 3
`SpecialMoveCooldown`s and totals 1.5 to 3 times the single bleed hit it
replaced at Strength 100; retuning `SpecialMoveCooldown` is expected to trip
that test.

## Files

Config is split one file per section, all assembled in `configs.go`.

| File | Section |
|------|---------|
| `configs.go` | Assembly, load/save, `GetConfig`, overrides plumbing |
| `config_types.go` | Shared config value types |
| `config_locks.go` | `hardLocked`, `IsLocked`: which keys `SetVal` refuses |
| `config_display.go` | `DisplayConfigData`, `RedactedValue`: the redacted view |
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
| `config.balance.lighting.go` | Graded room lighting thresholds, and the plan 3a solar/lunar knobs |
| `config.lighting_accessor.go` | `Lighting` struct and `GetLightingConfig()`, a narrow read that avoids copying all of `Balance` |
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
