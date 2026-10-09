# Mob Commands Package Context

## Overview
The `internal/mobcommands` package implements the AI command system for non-player characters (NPCs/mobs) in GoMud. It provides intelligent behavior patterns, combat AI, social interactions, and autonomous decision-making capabilities that bring the game world to life through sophisticated NPC behaviors.

## Key Components

### Command Architecture (`mobcommands.go`)
- **MobCommand function signature**: Standardized interface for all mob AI commands
  ```go
  type MobCommand func(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error)
  ```
- **CommandAccess structure**: Defines command availability and restrictions for mobs
- **Command registry**: Central mapping of AI command names to implementations
- **Autonomous execution**: Commands executed by AI logic rather than player input

### AI Behavior Categories

#### **Combat Intelligence**
- **Threat assessment**: `lookfortrouble` - Advanced hostility detection and target selection
- **Combat actions**: `attack`, `backstab`, `fire`, `throw` - Offensive capabilities
  (`fire` shoots a ranged weapon AND chambers the next round in the same
  action, via `actions.ExecuteFire`; `shoot` remains an alias. The mob `reload`
  command is GONE, along with the btree `try_reload` node, because there is no
  longer a reload step to take. Archer mobs use the btree `try_fire` /
  `keep_distance` actions rather than calling these commands directly.)
- **Special moves**: `bash`, `trip`, `kick`, `grapple`, `hamstring` - selected
  by `combat.ChooseSpecialMove` and dispatched here via `mob.Command(name)`.
  Each is anatomy-gated by the actor's species `body_parts` (grapple needs
  `arms`, trip/kick need `legs`, hamstring needs `legs` + a `Bite`/`Claws`
  natural attack; bash needs a shield-or-`NaturalBash` plus arms-or-`NaturalBash`).
  The dedicated `bite` command was **retired** — biting is now the basic
  attack for fanged species (see `internal/combat` Natural-Attack Subtype
  Resolution). `toxic-bite` (mutation) is unaffected.
- **Beast moves (Phase 3)**: `rake` (clawed), `maul` (fanged), `pounce`
  (quadruped predator, not grappling), `gore` (horned), `drain` (lifedrain
  species flag), `throttle` (fanged) — registered as both mob and user
  commands for full parity. Selected by `combat.ChooseSpecialMove` via the
  new `predator`, `ambush_predator`, and `brute` AI profiles (also weighted
  in `default`/`aggressive`). Each is gated by the exported predicates
  `combat.SpeciesIsFanged` / `SpeciesIsClawed` / `SpeciesIsHorned` /
  `SpeciesHasLifeDrain` / `SpeciesIsQuadrupedPredator` at three sync points:
  `CanUse*` in `ai.go`, `CommandIsReady`, and the action entry. Drift rows
  in `command_readiness_drift_test.go` keep all three in sync.
- **Rhetoric**: `taunt` and its wolf-flavoured `howl` variant use the shared
  coordinated Defy renderer when defended. Its attacker, defender, and room
  lines replace the ordinary hit narration rather than following it. Every
  room line either sends, on a fumble, a hit or a miss, goes out through
  `rooms.Room.SendTextHidingNames` with `messaging.HideNames` (sight gates
  slice 5b), so a shapes-only observer reads "a figure" for the mob or its
  target rather than a name; the personal line to a player target is hidden
  the same way through `messaging.HideNames` directly. The old two-tier
  darkness helpers (`darkness.go`) that used to do this by hand are deleted.
- **Tactical support**: `callforhelp` - Coordinated group combat behaviors
- **Self-preservation**: `flee` (flee parity, slice 4a) is a wrapper over
  `actions.BeginFlee` (`flee.go`): it only begins the escape (the gates, the
  admission, the `Disengaging` transition, the stamina cost) and renders
  nothing but the grapple line, because every other refusal is silent for a
  mob. The escape itself resolves a round later, in
  `hooks.handleMobFlee` -> `actions.ResolveFlee`, the same shared body the
  player's round uses. Before 4a a mob's flee was free, instant, and ignored
  roots, standing and whether the mob was even fighting. Like the player's
  fold-casting intercept in `usercommands.go`, `Flee` first drops a fold-cast
  (`activity.TriggerCastCancel`, no refund, the room sees "breaks their
  concentration.") whenever the mob is casting and in combat, before the
  gates; otherwise `hooks.handleMobFoldCasting`, which runs ahead of
  `handleMobFlee`, would finish the spell first. An out-of-combat flee keeps
  the cast.

#### **Social and Communication AI**
- **Conversation system**: `converse` - Dynamic NPC-to-NPC dialogue
- **Player interaction**: `sayto`, `say`, `shout` - Contextual communication.
  `say` (`say.go`), `shout` (`shout.go`), `rally` (`rally.go`), `warcry`
  (`warcry.go`) and `emote` (`emote.go`) are thin wrappers over
  `actions.Say`/`Shout`/`ExecuteRally`+`SendHeard`/`ExecuteWarcry`+
  `SendHeard`/`Emote`+`SendSeen` (sight gates slice 5b): the reveal, the
  room line with the name hidden by each listener's sight, the deafen split
  (unfiltered for a mob) and, for `shout`, the adjacent-room line and waking
  sleepers, all live in the shared body. `speech_wrapper_guard_test.go`
  (repo root) fails if any of these five re-forks that logic.
- **Emotional expression**: `emote` - Rich behavioral expressions
- **Quest integration**: `givequest` - Dynamic quest assignment

#### **Autonomous Movement**
- **Wandering behavior**: `wander` - Intelligent exploration with constraints
- **Pathfinding**: `pathto` - Goal-directed navigation
- **Zone awareness**: Movement restricted by zone boundaries
- **Home behavior**: Return-to-home mechanics for territorial mobs

#### **Item and Resource Management**
- **Inventory control**: `get`, `drop`, `put`, `give` - Intelligent item
  handling. A mob's `get` never takes a household's bauble:
  `actions.GetItemFromFloor` refuses it (`ErrHouseholdBauble`) and the mob says
  nothing. `get` is a thin wrapper over `actions.GetItemFromFloor` and
  `actions.GetGoldFromFloor` (sight gates parity, slice 5a): the dark refusal,
  the exploding-item refusal and the pickup itself all live in `internal/
  actions/get.go`; a mob reveals itself by cancelling `conditions.Hidden` on a
  successful pickup and is silent on every refusal.
- **Equipment management**: `equip`, `remove`, `gearup` - Automated gear
  optimization. `remove` (`remove.go`) is a thin wrapper over
  `actions.RemoveEquipment` (single item) and `actions.RemoveAllEquipment`
  (`remove all`); the busy and cursed-item gates live in those shared bodies
  in `internal/actions/remove_equip.go`, and a mob is silent on every refusal
  (a cursed item simply stays on, with no line). `equip` and `remove` judge every room line (displaced items, "puts on",
  "wields", "removes") against a `rooms.VisualSnapshot` of the room taken
  before the change, sent with `Room.SendTextVisualToSnapshot` (lighting
  plan 5d ruling D6 as amended by the owner on 2026-10-05; #447), the
  siblings of the player's commands; so does the idle floor pickup,
  `hooks.EquipBestFloorItem`.
- **Resource consumption**: `eat`, `drink` - Survival behaviors. `Drink`
  (`drink.go`) is a wrapper over `actions.Drink`, the same body a player drinks
  through (drink path unification 2026-09-28), so a mob pays toxicity, reads
  potion freshness and crafter skill, and gets every special potion's effect.
  The repo-root `drink_wrapper_guard_test.go` fails if the wrapper grows drink
  rules again.
- **Alchemy and crafting**: `alchemy` - Automated production behaviors.
  `craft` (`craft.go`) is a thin wrapper over `actions.InitiateCraft`
  (sight gates parity, slice 5a): the dark refusal (`CraftResult.CannotSee`)
  lives in `internal/actions/craft.go`, and every refusal, including a mob
  that cannot see to work, is a silent no-op.

#### **Support and Utility Behaviors**
- **Healing assistance**: `aid`, `lookforaid` - Medical support AI
- **Environmental interaction**: `look`, `show` - Awareness and demonstration.
  `look` (`look.go`) resolves through `actions.ResolveLook` (sight gates
  parity, slice 5a), the same body `usercommands.Look` uses, so a mob checks
  the dark, a locked exit and a hidden target by the player's own rules. Every
  room line it sends goes through `Room.SendTextVisualHidingNames`, so an
  observer who only sees shapes reads "a figure" instead of the mob's name.
- **Magic usage**: `cast`, `portal` - Spellcasting AI with tactical considerations
- **Stealth operations**: `sneak` - Covert movement capabilities

### Advanced AI Features

#### **Hostility and Threat Management** (`lookfortrouble.go`)
- **Multi-factor threat assessment**: Race hatred, alignment conflicts, group hostilities
- **Player party awareness**: Sophisticated targeting that considers party dynamics
- **Charmed mob handling**: Different behaviors for player-controlled NPCs
- **Escalation prevention**: Boredom counters to prevent endless aggression

#### **Coordinated Group Behaviors** (`callforhelp.go`)
- **Range-based assistance**: Configurable help radius for tactical support
- **Selective recruitment**: Target-specific ally summoning
- **Communication integration**: Emotive calls for help with custom messages
- **Strategic positioning**: Intelligent movement for optimal combat support

#### **Intelligent Navigation** (`wander.go`)
- **Goal-oriented wandering**: Seeking loot, players, or specific objectives
- **Territorial constraints**: Respecting home zones and wander limits
- **Return-home logic**: Automatic navigation back to spawn points
- **Environmental awareness**: Zone-restricted movement patterns
- **Quote before issuing** (movement parity 4b): `Wander` calls
  `actions.QuoteMobStep(mob, exitName)` before committing to a step; an
  unaffordable step is neither taken nor counted against `WanderCount`, and
  pack followers moved through the same exit are each gated the same way
  (`mobs.MovePackFollowers`'s `canStep` argument — see
  `internal/mobs/context.md`).

#### **Movement (`go.go`, movement parity 4b)**
- Before this slice a mob's walk was entirely free (no action points, no
  stamina) and rolled no hidden-detection contest at all. `mobcommands.Go`
  now calls the same shared bodies the player path uses
  (`internal/actions/move.go`, see that package's "Movement" section): it
  pays `actions.ChargeMove` AFTER its lock gates (silently refusing and
  taking no step on a refusal — a mob has no one to tell), then
  `actions.RelocateMob`, then rolls `actions.EntryDetection` on arrival and
  the rare `actions.TrainSearchOnMove`, both ways, same as a player.

#### **Dynamic Conversations** (`converse.go`)
- **Context-aware dialogue**: Conversations based on mob types and situations
- **Multi-participant support**: Complex NPC-to-NPC interaction chains
- **State management**: Conversation tracking to prevent conflicts
- **Scripted flexibility**: Support for forced conversation scenarios

### AI Decision Making

#### **Behavioral Prioritization**
- **Combat override**: Combat takes precedence over other activities
- **State-based decisions**: Different behaviors based on health, conditions, and state
- **Environmental factors**: Room conditions influence behavior choices
- **Social awareness**: Presence of players and other mobs affects decisions

#### **Intelligent Targeting**
- **Threat assessment algorithms**: Complex scoring for target selection
- **Party dynamics**: Understanding player group relationships
- **Alignment considerations**: Moral and ethical targeting preferences
- **Race-based hostilities**: Cultural and species-based conflicts

#### **Resource Management**
- **Inventory optimization**: Automatic equipment and item management
- **Survival priorities**: Food, drink, and healing behaviors
- **Economic behaviors**: Trading and resource acquisition patterns
- **Crafting automation**: Intelligent use of alchemy and creation skills

### Integration with Game Systems

#### **Character System Integration**
- **Condition awareness**: Behaviors modified by active status effects
- **Skill utilization**: AI uses mob skills and abilities appropriately
- **Health monitoring**: Behavior changes based on health status
- **Charm handling**: Different behaviors for player-controlled mobs

#### **Combat System Integration**
- **Aggro management**: Sophisticated threat and targeting systems
- **Tactical awareness**: Understanding of combat mechanics and timing
- **Group coordination**: Multi-mob combat strategies
- **Defensive behaviors**: Retreat and evasion capabilities

#### **Quest System Integration**
- **Dynamic quest giving**: NPCs can assign quests based on conditions
- **Progress awareness**: Mobs understand player quest states
- **Reward distribution**: Intelligent quest completion handling
- **Story integration**: Behaviors that support narrative elements

### Performance and Efficiency

#### **Optimized Execution**
- **Conditional processing**: Commands only execute when relevant
- **State caching**: Efficient tracking of mob states and conditions
- **Range limitations**: Bounded search areas for performance
- **Selective activation**: Behaviors triggered only when needed

#### **Memory Management**
- **Lightweight state tracking**: Minimal memory footprint for AI state
- **Efficient pathfinding**: Optimized navigation algorithms
- **Conversation management**: Proper cleanup of dialogue states
- **Resource pooling**: Shared resources for common AI operations

## Dependencies
- `internal/mobs`: Core mob management and state
- `internal/rooms`: Room system for spatial awareness
- `internal/characters`: Character system for mob properties
- `internal/users`: Player interaction and targeting
- `internal/conditions`: Status effect awareness
- `internal/conversations`: Dynamic dialogue system
- `internal/mapper`: Pathfinding and navigation
- `internal/parties`: Player group dynamics understanding

## Usage Patterns
- Commands executed autonomously by mob AI systems
- State validation ensures appropriate behavior selection
- Environmental awareness drives decision making
- Social dynamics influence interaction patterns
- Combat priorities override other behaviors

## Architecture Benefits
- **Autonomous intelligence**: Mobs behave independently without constant scripting
- **Scalable AI**: System supports complex behaviors across many NPCs simultaneously
- **Flexible behaviors**: Easy to add new AI patterns and capabilities
- **Performance optimized**: Efficient execution suitable for large-scale deployment
- **Integrated design**: Seamless interaction with all game systems

This package transforms static NPCs into dynamic, intelligent entities that create a living, breathing game world through sophisticated AI behaviors and decision-making systems.
## Files: one command per file

75 non-test files, one per mob command; the filename is the command. This
package is the mob-side twin of `internal/usercommands`.

Handler signature:

```text
func <CommandName>(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error)
```

Note it takes a `*mobs.Mob` and has no `flags` parameter — that difference is
why a command available to both players and mobs must be registered twice
(`AddUserCommand` and `AddMobCommand` take different handler types).

- **Parity is checked at boot.** `CommandParity` warns when a user command has
  no mob equivalent and is not on the user-only allowlist. Adding a player
  command usually means adding the twin here too.
- **Shared logic belongs in `internal/actions`**, behind `actions.Actor`, so
  the two paths cannot drift. A command file should be argument parsing plus a
  call into `actions`.
- **Voluntary-action admission** stays in that shared action path. Mob wrappers
  consume the returned `CostCommitResult` for control flow but never emit a
  player-private refusal line and never charge a pool independently.
- **Partial shortages are silent too.** Mob autoattack, winning defence, flee,
  and grapple maintenance use the same short-funded, skill-less mechanics as
  players without an invisible private warning.
- **`Go` relocates mobs through `actions.RelocateMob`** (`go.go`), after its
  own far-side lock check; a successful `flee` (`hooks.handleMobFlee`) ends in
  the same call, uncharged. There is no `clearRoomAggroOnDeparture` in this
  package any more — aggro cleanup on departure moved to
  `actions.ClearRoomAggroOnDeparture` alongside `RelocateMob`, so both callers
  share it. `Go` passes its `actions.MobIsSneaking(mob)` state, so a sneaking
  mob's step is not announced (parity slice 6, ruling D1).
- **`Go`'s forced path** (`rest` is a room id with no matching exit, used by
  `callforhelp`) moves the mob directly with `room.RemoveMob`/
  `destRoom.AddMob` instead of `actions.RelocateMob`, so it renders its own
  exit and entry lines. It reads `actions.MobIsSneaking(mob)` before the
  move and sends neither line when sneaking, matching the ordinary exit
  path.

## Fixtures (lighting 5e)

A mob's `get all` skips fixtures; a single `get` of one is refused by
`actions.TakeFloorItem` (`ErrFixture`). `hasLootItems` (`wander.go`) does not
count a fixture as loot, so a floor holding only a fixture does not draw a
scavenger.
