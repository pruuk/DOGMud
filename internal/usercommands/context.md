# User Commands Package Context

## Overview
The `internal/usercommands` package implements the complete command system for player interactions in GoMud. It defines all player-executable commands, from basic movement and communication to complex skills, combat actions, and administrative functions.

## Key Components

### Command Architecture (`usercommands.go`)
- **UserCommand function signature**: Standardized interface for all commands
  ```go
  type UserCommand func(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error)
  ```
- **CommandAccess structure**: Defines command permissions and restrictions
- **Command registry**: Central mapping of command names to implementations
- **Permission system**: Admin-only commands and downed-state restrictions

### Command Categories

#### **Basic Interaction Commands**
- **Movement**: `go`, `flee` - Navigation and escape mechanics
- **Communication**: `say`, `shout`, `whisper`, `emote`, `broadcast` - Player communication
- **Observation**: `look`, `inspect`, `consider`, `online` (`who` is its alias in `keywords.yaml`, #421) - Information gathering
- **Inventory**: `inventory`, `get`, `drop`, `give`, `put` - Item management
- **Drink** (`drink.go`, drink path unification 2026-09-28): `Drink` is a
  wrapper over `actions.Drink`, which holds every drink rule (toxicity, aging,
  crafter scaling, the special potions) for players and mobs alike. Do not add
  a rule here: the repo-root `drink_wrapper_guard_test.go` fails if this file
  or `internal/mobcommands/drink.go` applies drink rules itself.
- **Carried light (lighting plan 5a)**:
  - `hood` / `unhood` (`hood.go`: `Hood`, `Unhood`, helper `hoodedLight`)
    shut or open the hood of the `adjustable` light in the `Light` slot. A
    hooded record stays held and lit but sheds nothing (`Condition.Hooded`).
    `hoodedLight` sends its own refusal and tells an empty slot apart from a
    light with no hood. `Hood` takes `room.VisualSnapshot()` before setting
    `Hooded` and sends the room line with `SendTextVisualToSnapshot`, so it
    is judged by what each watcher could see just before the light went
    (#220): it reaches those who saw by the lantern and nobody who could not.
    `Unhood` takes the same snapshot before `ResetLight` and splits the room
    line by it (owner ruling 2026-10-06): a watcher who could see before
    (not `SightNone`) reads "throws back the hood", one who could not reads
    "A light flares to life, revealing <name> standing there, holding a
    lantern." Both are judged after the light is back (`SendTextVisual`,
    `SendTextVisualHidingNames`), so a watcher who still sees nothing gets
    neither and shapes read "a figure".
  - `cancel <spell>` (`cancel.go`: `Cancel`, `cancelCondition`,
    `cancelNameMatches`): an activity in progress ALWAYS wins, whatever the
    argument; only a free user reaches `cancelCondition`, which ends the first
    HELD record (held order, so deterministic) whose spec carries
    `conditions.Cancellable` and whose condition name, or a granting spell's
    id, alias or name, matches exactly or by a prefix of at least
    `cancelMinPrefix` (3) characters. A spell whose condition is not held can
    never be reached. Before a light or darkness record is removed,
    `rooms.KeepEndLineSnapshotsBeforeRemoval` keeps the room's snapshot, so
    its end line at the prune is judged by what each watcher could see just
    before it went out (#220).
  - `look` (`look.go`) tries `Character.FindItemNoun` (exact noun on a worn or
    carried item) BEFORE item matching, so `look hood` reaches the lantern's
    hood; looking at an item highlights its `ItemSpec.Nouns` before wrapping.

#### **Combat Commands**
- **Direct combat**: `attack`, `fire`, `throw` - Offensive actions
  (`fire` shoots a ranged weapon AND chambers the next round in the same
  action, consuming one round from a matching bundle; `shoot` is an alias.
  The player-facing `reload` is GONE -- `reload` is now admin-only, in
  `admin.reload.go`, and re-reads server data files)
- **Combat skills**: `disarm`, `tackle`, `backstab`, `recover` - Specialized combat techniques
- **Defensive**: `flee`, `aid` - Escape and assistance mechanics
- **Beast special moves (Phase 3)**: `rake`, `maul`, `pounce`, `gore`,
  `drain`, `throttle` — registered as player commands for full
  player↔mob parity, but gated at the action entry by species-identity
  predicates (`combat.SpeciesIsFanged`, `SpeciesIsClawed`,
  `SpeciesIsHorned`, `SpeciesHasLifeDrain`, `SpeciesIsQuadrupedPredator`).
  Baseline humanoid players cannot use them; they are intended for beast
  mobs and future beast-mutated players. The action returns a
  `Not<Identity>` result for ineligible callers. See
  `internal/combat/context.md` "Beast Moveset (Phase 3)" for full
  gate and mechanic details.

#### **Naming a creature in the dark (#454)**
Every command that names a creature in the room runs the typed name through
`actions.AimBySight` before it resolves: `attack` (a `*` wildcard needs some
sight), `target`, the melee specials (through `actions.StageMeleeTarget`),
`give` (the giver's own pet, named `pet` or by its own name, is exempt and
resolves before any room lookup; the item and recipient split admits only a
recipient `AimBySight` admits, and below full sight falls to the first split
whose item alone resolves, so a refusal never tells who is there), `show`, `consider`, `steal` and `plant`
(a refused noun may still be a container), `shadow`, `talk`, `ask`,
`party invite` and `rep`. A pet is named, never a shape: `ownPetNamed`
(`pet.go`) admits the player's own pet (`pet` or its name) at any sight, and
`petOwnerInSight` finds another player's pet only at full sight of a
perceived owner, so `pet <name>` (`AimNotHereLine` below full sight),
`get x from <pet>` and `give x <pet>` never confirm a pet in the dark.
`fire` refuses below full sight of the room it is
aimed into (`actions.ShotSight`) before any name resolves. Lines that follow
a shape name no one: `give`, `show`, `party invite` and `rep` hide each name
at its reader's sight with `messaging.HideNames` (an invitation or a report
to its recipient with `messaging.HideSpeakerNames`), and `consider`,
`steal`, `plant` and `shadow` hand their action `actions.UserActorAtSight`.
Party auto-assist skips a member who sees nothing. `share` pays the party
members in the room directly through give's gold path (`giveGoldToUser`,
which hides each name at its reader's sight), never by typing `give N gold
to @uid`, which the sight gate would refuse in the dark; membership is
already known, so no presence leaks.

#### **Skill-Based Commands**
- **Magic system**: `cast`, `enchant`, `unenchant`, `prepare` - Spellcasting mechanics
- **Stealth**: `sneak`, `picklock`, `pickpocket`, `peep` - Stealth and thievery
- **Utility skills**: `map`, `track`, `search`, `portal` - Exploration and navigation
- **Crafting**: Various skill-based creation and modification commands

#### **Economic Commands**
- **Trading**: `buy`, `sell`, `list`, `offer`, `appraise` - Commerce mechanics.
  `list` shows a living shop's secondhand shelf as a second table,
  "Secondhand goods" (`renderShelfListing` over `buildShelfRows`: the
  listed entries in shelf order, each in the lister's own view, a
  finder-view site in the root guard), trims the listed cap lazily and
  saves; "nothing to sell" fires only when both tables are empty.
- **Banking**: `bank` - Financial management
- **Services**: `train` - Character development

#### **Town Justice Commands** (5.1c)
- **`fine`** (`jail.go`): Shows a jailed player their current decaying fine
  and how to pay it. Uses `justice.JailInfo` + round math to compute the
  remaining gold owed.
- **`payfine`** (`jail.go`): Deducts the current fine from the player's
  on-person gold (bank as fallback) and calls `justice.ResolveDetention`
  to immediately release them. Blocks if the player cannot cover the fine.

#### **Social and Party Commands**
- **Party system**: `party` - Group management and coordination
- **Pets**: `pet`, `tame` - Animal companion system
- **Character management**: `character`, `set`, `alias` - Character customization.
  The `set` command supports a `set arrest <surrender|resist>` subcommand
  (`cmdSetArrest` in `set.go`) that stores `characters.ArrestPolicy` on the
  character. Default is `surrender`. The `resist` policy causes guards to
  attack immediately rather than wait through the arrest grace window.
  The `set combatverbosity <full|medium|light>` subcommand sets
  `UserRecord.CombatVerbosity` (persisted in the user YAML). `full` is
  the historical default; `medium` shows landed hits only (dodge/parry/block
  lines suppressed); `light` replaces per-swing lines with one compact
  summary per round. The viewer's floor rules (deaths, position-changing
  moves, and blows directed at the viewer) always pass regardless of
  setting; spectated fights render one step quieter than the viewer's own
  setting.

#### **Administrative Commands** (Admin-only)
- **World building**: `room`, `build`, `zone` - Environment creation and modification
- **Entity management**: `mob`, `item`, `spawn` - Game object manipulation
- **Server management**: `server`, `reload`, `teleport` - System administration
- **Player management**: `grant`, `modify`, `mute`, `deafen` - Player administration
- **Combat analytics**: `combatstats` - Combat analytics dashboard — view, filter, reset, export combat event data

### Command Processing Features

#### **Input Parsing and Validation**
- **Argument parsing**: Sophisticated parsing with quote respect for complex arguments
- **Target resolution**: Finding players, mobs, and objects by name or partial match
- **State validation**: Checking combat status, conditions, and other restrictions

#### **Permission and Security**
- **Role-based access**: Admin commands restricted by user permissions
- **State restrictions**: Commands blocked when downed, in combat, or affected by conditions
- **Cooldown management**: Time-based restrictions on command usage

#### **Event Integration**
- **Event flags**: Commands can be executed secretly or with special modifiers
- **Event emission**: Commands trigger events for logging and system integration
- **Combat integration**: Commands interact with combat state and aggro systems

### Skill Integration

#### **Skill-Based Commands** (`skill.*.go` files)
- **Cast system**: Magic spell casting with proficiency scaling
- **Brawling skills**: Physical combat techniques (disarm, tackle, throw)
- **Utility skills**: Map creation, portal magic, inspection abilities
- **Protection skills**: Aid and defensive capabilities
- **Search skill**: Discovery of hidden objects, containers, exits, and creatures

#### **Skill Validation**
- **Level requirements**: Commands check character skill levels
- **Proficiency effects**: Higher skill levels improve command effectiveness
- **Training integration**: Skills can be improved through use and training

### Administrative System

#### **World Management**
- **Room editing**: Comprehensive room modification capabilities
- **Zone management**: Creating and managing game world zones (note: `zone set autoscale` was removed in Phase 21; mob difficulty is now per-mob via `statpool`)
- **Spawn control**: Managing mob and item spawning

#### **Player Administration**
- **Character modification**: Changing player stats, levels, and properties
- **Punishment system**: Muting, deafening, and other disciplinary actions
- **Server monitoring**: System status and performance monitoring

#### **Bauble catalog** (`admin.bauble.go`, docs/baubles)
- `bauble status | stats | list | show | prompt | spawn | edit | regen |
  retire | restore | window`. Every subcommand that takes a record accepts a
  catalog id or the name of a bauble in the admin's pack or on the floor,
  matched as a player's `get` would match it (`resolveBaubleArg`).
- `edit` splits target from text at the first field word, so
  `bauble edit small doll name Rag Doll` works.
- Edits, retire and restore reach every copy of the bauble in the world at
  once, because items read their text from the catalog.
- `regen` and `spawn` run in the background through `internal/actions`
  (`RegenerateBauble`, `StartBaubleFind`); the admin is told when they finish.
- `bauble status` ends with a `Sweep:` line (`baubleSweepLine`) from
  `baubles.LastSweep()`: not run yet, failed with its error (nothing
  pruned), catalog empty, or records, still held, pruned, files read and
  parsed, disk time, mud-lock hold time, and the interval
  (`Balance.BaubleSweepHours`); a shard write that failed adds how many and
  that shard's records are unpruned (`SweepStatus.ShardErrors`).

#### **Looking at the floor** (`look.go`)
- `look <item>` falls back to items on the floor as its LAST branch, after
  exits, nouns, creatures and corpses, so it never shadows them. A bauble
  left lying (a household's, or its finder could not carry it) is looked at
  this way: "on the bookshelf" instead of "on the ground", and a household's
  adds "It belongs to this household. Taking it would be theft."
- The ground listing appends `BaubleSpotSuffix()`: "a Small Doll (on the
  bookshelf)".
- A finder-only bauble (unmoderated player-key text) reads as "Trinket" to
  everyone but its finder. `inventory`, `look` (a carried or floor item,
  and the room's floor and stash listing) and the bauble `appraise` ask for
  the reader's own view (`DisplayNameFor`, `NameFor`, `LongDescriptionFor`,
  `GetSpecFor`, `baubles.Record.MaterialFor`); every room line and every
  other command keeps the viewer-agnostic name. `bauble show` prints
  `player key`, `moderated` and `finder only`; `bauble list [n]
  [playerkey|unmoderated|finderonly]` filters.

#### **Household baubles** (`get.go`, `skill.skullduggery.steal.go`)
- `get` never takes a bauble that belongs to this room's household
  (`Item.BaubleBelongsTo`), so no pickup can start a crime by accident. An
  explicit `get <name>` is refused by the shared floor pickup
  (`actions.GetItemFromFloor` returns `actions.ErrHouseholdBauble` for every
  taker, mobs included), and `get` words it: "The X belongs to this household.
  To take it anyway, steal <word>.", before anything that would end the player's
  hiding; `get all`, `get all <name>` and `get all.<name>` skip
  it with "You leave the X: it belongs to this household. To take it anyway,
  steal <word>." (`leaveHouseholdBauble`), once each, and go on to take
  everything else the name matches (`takeableOnFloor`: the floor without
  this room's household baubles), so one protected trinket never stops the
  sweep. `stealWord` is the bauble's keyword, or `trinket`.
- Taking one is `steal <name>`: `parseStealArgs` falls back to
  `householdBaubleNamed` (a household bauble on this room's floor) when no
  creature or container matches, and hands it to `actions.Steal` as
  `StealOptions.HouseholdItem` (the steal checks; see
  `internal/actions/context.md`).

### Special Features

#### **Command Suggestions**
- **Fuzzy matching**: Suggesting similar commands for typos
- **Context-aware help**: Relevant command suggestions based on situation
- **Admin filtering**: Different suggestions for admin vs regular users

#### **Dialogue–Quest Integration** (`talk.go`, `ask.go`)
- **PlayerState construction**: `buildPlayerState(user)` creates a
  `dialogue.PlayerState` with callbacks for `HasQuest`, `HasItem`,
  `RemoveItem`, `GiveQuest`, `GiveItem`, etc. — passed to all dialogue
  engine calls so NPC dialogue can be gated on quest progress and
  inventory. `GiveItem` returns whether the item actually reached the
  player (false when `StoreItem` refuses over carry capacity, or on an
  invalid item id) — the dialogue engine then withholds the node's
  other effects, including `grantsQuest` (2026-08-03 soft-lock fix)
- **Quest context for LLM**: `buildQuestContext(user, mobId)` returns
  human-readable quest summaries injected into the LLM system prompt
  via `llm.ConversationContext.QuestContext`
- **Item consumption**: `requiresItem` on dialogue nodes removes the
  item from the player's backpack on activation (via `RemoveItem`)
- **Quest advancement**: `grantsQuest` fires `events.Quest` to
  advance quest state; the quest event handler processes rewards

#### **Scripting Integration**
- **JavaScript exposure**: Commands can be called from game scripts
- **Function export**: Command functions available to scripting system
- **Event-driven execution**: Commands can be triggered by game events

#### **Alias System**
- **Custom shortcuts**: Players can create command aliases
- **Macro support**: Complex command sequences through aliases
- **Personal customization**: Per-character alias storage
- **Web panel sync**: `cmdSetMacro` (`set.go`) and `Alias` (`alias.go`)
  emit `events.AutomationChanged{UserId}` after every macro or alias
  change so the `gmcp.Automation` module immediately re-pushes
  `Char.Automation` and the web automation panel stays in sync.

#### **Movement (`go.go`, movement parity 4b)**

`Go` is now a thin wrapper: it keeps the lock gate (pick, key ring, backpack
key), the narration, and the exit-message requeue, but the step's price and
the hidden-detection contests moved to `internal/actions/move.go` (see that
package's "Movement" section for the mechanism). The charge
(`actions.ChargeMove`) now runs AFTER the lock gate and after the
exit-message requeue, not before — before this slice it ran first, so
walking into a locked door or triggering an exit message cost a step;
**a locked door now costs nothing.** Detection (a sneaking mover spotted, or
the mover spotting a hidden occupant) runs through `actions.EntryDetection`
instead of `go.go`'s own inline rolls; the rare Search-training call
(`actions.TrainSearchOnMove`) still fires from here, inside the
`rooms.MoveToRoom` success branch. `move_wrapper_guard_test.go` (repo root)
fails if this file prices or detects a step itself again. The departure line
("X leaves to the north.") is judged by the room before the move: `Go` takes
`room.VisualSnapshot()` just before `rooms.MoveToRoom` and sends with
`SendTextVisualWithAudioToSnapshot`, so a mover is seen leaving by the light
they carry out (#456, owner ruling 2026-10-09). Arrival lines are judged
after the move.

## Dependencies
- `internal/users`: User management and character data
- `internal/rooms`: Room system for location-based commands
- `internal/events`: Event system for command effects and logging
- `internal/mobs`: NPC interaction and combat
- `internal/items`: Item manipulation and inventory management
- `internal/skills`: Skill system integration
- `internal/spells`: Magic system integration
- `internal/conditions`: Status effect checking and application
- `internal/scripting`: JavaScript runtime integration
- `internal/combat`: contest resolution. The movement contests (a sneaking
  mover against each occupant, and the mover spotting hidden players and
  mobs on arrival) moved out of `go.go` in movement parity 4b — they resolve
  in `actions.EntryDetection`, shared with mobs. The shadow contest moved to
  `actions.ShadowSenseRoll` in parity slice 6. The one in `throw.go` is
  still here and resolves through
  `combat.RunContest(attackScore, []contest.Entry{{Score: defenseScore}})`.
  U4 routed them to per-channel wrappers; U6 collapsed those into this single
  entry point. This package imports `internal/contest` for the `Entry` type
  only and must not call that package's roll functions directly.
- `shadow` (`skill.skullduggery.shadow.go`) only resolves the target and calls
  `actions.Shadow`; `shadow stop` reads `actions.ShadowTargetOf` and calls
  `actions.EndShadow`. Following, the spotted end and the sense roll live in
  `hooks.RoomChangeShadowFollow` (parity slice 6), not in `go.go`.

## Usage Patterns
- Commands follow consistent signature and return conventions
- State validation occurs before command execution
- Events are emitted for logging and system integration
- Permission checks prevent unauthorized access
- Error handling provides user feedback and system logging

## Testing
The package includes comprehensive testing for:
- Command parsing and argument handling
- Permission and access control
- State validation and restrictions
- Integration with other game systems
- Administrative functionality

## Architecture Benefits
- **Modular design**: Each command is self-contained and focused
- **Consistent interface**: All commands follow the same signature pattern
- **Extensible system**: New commands can be easily added to the registry
- **Permission control**: Granular access control for different user types
- **Event integration**: Commands seamlessly integrate with the game's event system

## Search Skill System

### Overview
The `search` command discovers hidden objects in rooms, including hidden containers, hidden nouns, secret exits, and hidden mobs. Uses Perception-based rolls with per-discovery granularity.

### Search Roll Formula
```
searchScore = dice.RollStat(Perception + SkillMultiplier(searchRank) * 25.0)
```

- **Perception**: Character's current Perception stat (~100 baseline)
- **SkillMultiplier**: Sqrt curve from current search rank to soft cap (rank 50)
- **dice.RollStat**: Applies global `RollSpread` factor for variance
- **searchScore**: Single roll covers all discoveries in one `search` command

### Tier Difficulty Targets
| Target | Hidden Type | Examples |
|--------|------------|----------|
| 125 | Secret exits, hidden containers | Doors behind tapestries, false walls |
| 135 | Stashed items, hidden creatures | Boxes under beds, camouflaged mobs |
| 175 | Hidden nouns | Faint carvings, ancient runes |

A player's search also takes a bauble roll after these tiers: no contest, a
chance set by the room's biome and raised by search skill. A find counts as a
won search; a roll that finds nothing awards nothing. See the Baubles section
of `internal/actions/context.md`.

`search <feature>` (`skill.search.go` passes `rest` as
`actions.SearchOptions.Feature`) searches one noun, discovered hidden noun or
visible container in the room: one bauble roll against that feature's own
window, never the room's, and no contested tier. The quest `command`
notification fires for every form of `search`, as before (quests 49 and 58
gate on it in room 5343). See "Targeted search" in
`internal/actions/context.md`.

### Per-Discovery Rolls
- Each hidden object in the room gets compared against `searchScore` individually
- Players with `searchScore ≥ target` discover that specific object
- Multiple discoveries possible in one `search` if roll is high enough
- Each discovery shows unique flavor text and adds to discovery tracking

### Anti-Botting Protection
- **Progression guard**: Search skill only gains progression if at least one undiscovered object was rolled against
- If all objects in the room are already discovered, skill use doesn't trigger progression
- Prevents skill grinding on discovered-only rooms

### Related Commands
- **`track`**: Uses search skill formula to find hidden tracks
- **`forage`**: Uses search skill formula to gather hidden resources
- All three commands use the same unified search score calculation

This package serves as the primary interface between players and the game world, providing a rich and comprehensive command system that supports all aspects of gameplay from basic interaction to advanced administrative functions.
## Files: one command per file

186 non-test files (recounted for lighting plan 5a, which added `hood.go`), and enumerating them would be noise — the filename **is**
the index. `command.go` implements `command`; `admin.<name>.go` is an admin
command; `skill.<name>.go` is a skill-gated one.

What matters is the shape, not the list:

- **Registration** is centralised. A command is not discovered from its
  filename; it is registered with its handler, its downed-allowed flag, and its
  admin-only flag. Modules register their own through
  `plugins.AddUserCommand` instead.
- **Handler signature:**
  ```text
  func <CommandName>(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error)
  ```
  `rest` is everything after the verb — parse it, do not re-split the whole
  line.
- **User/mob parity.** Most commands have a `internal/mobcommands` twin. The
  boot-time `CommandParity` check warns when a user command has no mob
  equivalent and is not on the user-only allowlist. Adding one usually means
  adding both, or explicitly allowlisting.
- **Shared logic belongs in `internal/actions`**, behind `actions.Actor`, so
  the user and mob paths cannot drift. A command file should be argument
  parsing plus a call into `actions`.
- **Voluntary-action admission** is mechanical shared-action work. A user
  wrapper will render a private, pool-aware refusal only when its returned
  `CostCommitResult` has `Status == characters.CostRefused`; it must not
  re-quote, re-commit, or charge a pool directly.
- **Flee is a thin wrapper (slice 4a).** `Flee` (`flee.go`) calls
  `actions.BeginFlee` and renders the returned `actions.FleeBegin`: a
  `fleeRefusalText[begin.Refusal]` line on refusal, the shortage line when
  `Short`, else "You attempt to flee...". The gates, the admission handoff,
  the `Disengaging` transition and the partial stamina cost all moved into
  `actions.BeginFlee`, shared with `mobcommands.Flee`. `TakeFleeAdmission` and
  `CancelFleeAdmission` no longer exist in this package — the handoff lives on
  `characters.Character` (`internal/characters/flee_admission.go`) and the
  round resolves through `actions.ResolveFlee`, called from
  `hooks.handlePlayerFlee`. A rejected transition or an out-of-combat command
  spends nothing and publishes no admission.
- **Casting interception preserves terminal semantics.** The generic command
  dispatcher (`usercommands.go`, unchanged by slice 4a) cancels a pending
  fold-cast only when `cmd == "flee"` and the character is actually in
  combat, before `Flee` itself runs; an out-of-combat rejection leaves the
  cast intact. The cancellation line never exposes the raw conviction already
  invested in the cast.
- **Player shortage lines are private and singular.** Autoattack, winning
  defence, flee, and grapple maintenance explain the skill-less resolution once
  at their owning round/action boundary. Losing defence candidates never emit a
  line. NPC wrappers use the same mechanics without private output.
- **Target resolution** uses the existing fuzzy matchers, which already handle
  multi-word input. Reach for `internal/parser` only when a command must
  *split* input into multiple slots (item vs. container, mob vs. player).

## Adding a command — the wiring checklist

Enumerate every step; a partially-wired admin command is the classic failure:
handler file, registration entry, help file, mob twin (or allowlist entry),
and — if it is player-facing — an entry in the relevant help category.

**If the command attacks a mob, it must seed aggression.** Admission-gated
melee specials and taunt route through `actions.StageMeleeTarget`; their shared
`Execute*` action commits engagement only after payment and cooldown. Anything
with its own targeting — `attack`,
`shoot`, `throw` — calls `actions.SeedAggression` itself. Skip it and the
attack is invisible to the revenge, opinion and justice systems: no assault
crime, no rep hit, no bounty, no witness memory. That was the real state of all
twelve special-move paths until 2026-08-14. See `internal/actions/context.md`
for the fresh-aggro contract, which is easy to get backwards.

When a taunt is defended, the coordinated Defy message is the complete result
for each audience. Do not also emit the ordinary taunt-hit narration; that
would describe the same resolution twice and can contradict the defence.

**`fire` narrates several FireResult flags, and one of them is an ordering
trap.** `FireResult.Revealed` and `AimedWhileEngaged` are set by
`actions.ExecuteFire`; `sendShootMessages` is the only thing that speaks them.
(`SurpriseOnCooldown` is GONE: the ambush is no longer refusable, so nothing
can set it.)

⚠️ **`FireResult.Chambered` must be narrated too, and its silent case is the
one that bites.** Chambering admits its own small cost and re-checks the weapon
and bundle by identity, so it can return with neither `Loaded` nor `NoAmmo`
set -- too spent, or the weapon changed underneath the shot. Saying nothing
there leaves the weapon empty and the NEXT shot reporting a cause it cannot
know, so the switch has a `default` arm on purpose. Do not "simplify" it away.

- `surpriseShotShooterLine(hit, triad, targetColored, tier, dealtDamage)` picks
  the shot-from-cover line. **The triad arm must stay ABOVE any damage-carrying
  arm.** `combat.SkillMoveResult.Hit` is `!Defended`, so a defended shot that
  still drew blood is `!hit && dealtDamage` — and the ORDINARY shot arms test
  that combination first, deliberately, to keep their damage-carrying line.
  Copying that order onto the ambush path shadowed the defence triad on every
  defended-partial ambush, so the shooter was never told what stopped it.
- The engaged-aim cue is latched once per engagement on
  `Character.RangedEngagedCueSpoken`. `shouldSpeakEngagedCue` returns both the
  speak decision and the value to store; storing `AimedWhileEngaged` verbatim is
  what re-arms the cue when the shooter gets clear, and it is also what heals
  the one case `EndAggro` cannot reach (a cross-room shooter who never held
  `Aggro` of their own).
- The ambush lines route through `messaging.CategorySurpriseAttack`, which no
  verbosity level suppresses. The engaged-aim cue does NOT: it fires on ordinary
  shots and is a mechanical explanation, so it uses `CategorySystem` like the
  cost refusal and defence shortage lines beside it.

None of these lines carries the melee `*[SURPRISE ATTACK]*` banner. A
20-column marker plus a target name plus a damage band does not fit in 80
columns, and these lines name the shot from cover in prose instead.
`shoot_narration_test.go` measures every composition at the p90 authored
mob-name length with the widest damage band.

**`attack` speaks the melee half of the same refusal, and the call ordering is
load-bearing.** `actions.EngageAggroType` returns `(aggroType,
surpriseOnCooldown)`; the second value is the melee twin of
`FireResult.SurpriseOnCooldown` and exists because `DefaultAttack` comes back
for two different reasons ("never hidden" and "hidden, but the shared
special-move timer refused the opener") and only the second may be spoken.
`sendMeleeAmbushDenial` is the only thing that speaks it, from both engagement
sites (player to mob, player to player), on `CategorySurpriseAttack` like the
ranged refusal.

- **Call it AFTER `SetAggro`.** The refusal is followed by a second line saying
  the attacker's cover is gone, and that line is gated on
  `Character.IsHidden()` read *after* the fact — the cascade that spends the
  cover (`internal/hooks/Awareness_Cascades.go`, on Idle to Engaging) runs
  inside `SetAggro`. Checking before it, or assuming the reveal happened, would
  lie on the `SetAggro` paths that return before the Combat Phase transition
  (the grace-period and taunt-hold guards) or have it vetoed.
- Losing cover for nothing is the real cost of a refused ambush, which is why
  there are two lines rather than one. Before this, the melee ambusher lost
  their cover, got an ordinary swing, and was told neither thing.
- The mob and behaviour-tree engagement paths discard the signal deliberately:
  it is feedback for the attacker, and their attacker is not a player.

**`throw` is the grenade verb and is untargeted by design** — it takes an item,
never a target, and resolves as a room AoE. Aimed thrown weapons (darts,
javelins) belong under `ranged-combat` and `ExecuteFire` instead. Settled
2026-08-14; the reasoning is in `internal/actions/context.md`.

### Sight and gates parity (slice 5a): shared bodies, this package only words them

`get`, `look`, `remove` and `craft` each open on a shared-body verdict from
`internal/actions` and translate it into player text; none of them re-implements
a gate.

- **`Equip`** (`equip.go`): the arm-suffix branch (`equip X armN`, or the
  legacy `armN` spelling) goes through `actions.EquipItemInArm(actor, rest,
  targetArmSlot)` instead of calling `Character.Wear` directly; the plain
  path still calls `actions.EquipItem`. Both return the same
  `actions.EquipItemResult`, so the two paths render identically
  (`result.ArmLabel` is empty for the plain path, set for the arm path, and
  picks the "You wield"/"You equip ... in your %s" wording). Routing the arm
  path through the shared body means it now meets `Wear`'s `MinStrength` and
  reservation gates like any other equip, and an item it knocks off a full
  pack lands on the floor instead of being lost, matching the plain path.
  Every equip room line (displaced items, the arm-slot "equips", "puts on",
  "wields") and the single `remove` line is judged against a
  `rooms.VisualSnapshot` taken before the change
  (`room.SendTextVisualToSnapshot`, #447): a light or darkness put on, taken
  off or displaced announces itself with the room as it was before (owner
  rule, 2026-10-05), so an observer blind before a torch was lit learns
  nothing and one the torch's removal leaves in the dark still sees it come
  off. For an item that touches no light the snapshot reads what the room
  reads after. One call per line keeps the viewpoint walk in the root
  `messaging_surface_guard_test.go` seeing each line's observer. `remove all`
  sends no room line. `hood` refuses a darkness in the light
  slot with its existing "has no hood." line (`hoodedLight` reads light
  sources only).
- **`busyRefusalText(verb string)`** (`busy_refuse.go`) is now the one string
  builder behind both `refuseWhileBusy` (the pre-dispatch gate most commands
  use) and a wrapper that renders `Busy` from a shared body's own result
  (`Remove`, below): the wording must not fork between "refused before the
  call" and "refused inside the call".
- **`Get`** (`get.go`): refuses up front on `actions.TooDarkToGet`
  ("You can't see anything to pick up!"), asked before the container, corpse
  and bag branches so all of them stay refused in the dark too.
- **`Look`** (`look.go`): every sight rule (no-sight refusal, a creature named
  only at clear sight and only if perceived, an exit's through-sight and
  lock, the pet at clear sight) lives in `actions.ResolveLook`, shared with
  the mob look; this function switches on `res.Kind` (`actions.LookBlind`,
  `LookTooDark`, `LookRoom`, `LookCreature`, ...) and only words the answer.
  `noSightRefusal` words the two no-sight kinds for `look`: a
  blinded looker reads "You can't see anything!", one in a room too dark
  is told light or other eyes would help (#364).
- **`Remove`** (`remove.go`): the `all` branch calls
  `actions.RemoveAllEquipment(actor)`, which owns the busy gate, the curse
  gate per item and one `EquipmentChange` event per removal; the wrapper
  renders `res.Busy` through `busyRefusalText`, a line per `res.Cursed` item,
  then the reservation-return disclosure. The single-item path calls
  `actions.RemoveEquipment(actor, rest)` and switches on `result.Busy`,
  `!result.Found`, `result.Cursed` and `result.CursedOverridden`, so a
  cursed item stays on unless the remover's enchant skill is high enough to
  lift it.
- **`Craft`** (`craft.go`): refuses up front on `actions.TooDarkToCraft`
  ("You can't see well enough to work on anything here."), before recipe
  lookup and the storage pull; `result.CannotSee` in the later
  `actions.InitiateCraft` switch is unreachable from this wrapper today (the
  early refusal always fires first) but is kept because the result's
  contract is shared with the mob and companion callers.

### Speech and emotes parity (slice 5b): shared bodies, this package only words them

`say` (`say.go`), `shout` (`shout.go`), `rally` (`rally.go`), `warcry`
(`warcry.go`) and `emote` (`emote.go`) are thin wrappers over
`actions.Say`/`Shout`/`ExecuteRally`+`SendHeard`/`ExecuteWarcry`+
`SendHeard`/`Emote`+`SendSeen`: the reveal, the room line with the speaker's
or actor's name hidden by each listener's sight, and the deafen split all
live in the shared body. Each wrapper keeps only its own concerns: mute,
drunk text (`say`, `shout`), uppercase (`shout`), escaping and the speaker's
own line. `speech_wrapper_guard_test.go` (repo root) fails if any of these
five, or their `internal/mobcommands` twins, re-forks that logic, and pins
each wrapper's call count on its shared body (one per room line it sends).

`Emote`'s free-form line (and its `@` form) is player chatter and stays
deafen-filtered (`actions.SendSeen(..., chatter: true)`, routed through
`Room.SendVisualCommunicationHidingNames`); the empty line and an alias line
are pre-written and reach everyone who can see regardless of Deafened
(`chatter: false`, `Room.SendTextVisualHidingNames`), and both bypass the
mute check too, since they carry no free text.

`internal/usercommands/offer.go`'s package-local `merchantSay` (shared by
`Offer` and `Appraise`, `appraise.go`) only calls
`actions.Say(&actions.MobActor{...}, line)`; it hand-rolls no room line of
its own, matching the canonical `merchantSay` in `internal/actions/sell.go`.

### Crafting: instant-complete narration (`craft.go`)

Two `messaging.SendTrio` helpers deliver crafting's text, and they are not
interchangeable:

- **`craftDeliver(user, cat, text)`** — the command's ordinary self-only
  responses (recipe refusal, station/skill gate, "you begin" notice). No
  second party, no room broadcast: `Actee` and `Observer` are always
  `messaging.NoLine` and `Room` is left unset. `SendTrio` only hides a name
  when it has a `Room` to judge sight by, so a self-only line with no `Room`
  passes through unchanged.
- **`craftDeliverInstant(user, room, roles)`** — the two instant-complete
  sites: `case result.ImmediateComplete` in `Craft()`, and `completeCraft`
  (the enchanting instant path). Both pair the crafter's own success line
  with a room observer line built from `recipe.Narrate`'s `{actor}`
  substitution, so it needs a real `Audience` with `Room` set. `craftDeliver`
  is deliberately NOT reused here for that reason.

Before **M4d PR 3**, both instant-complete sites sent the observer line on
`room.SendTextVisual` with no names, which never calls `messaging.HideNames`
— only tag-based `messaging.Anonymize` runs on that path. `{actor}` resolves
to `GetCharacterName(true)`, which wraps the name in an `<ansi fg="username">`
tag, so shipped content was incidentally safe (a `SightShapes` reader's tag
gets stripped regardless of delivery call); a future untagged name, or a
copy-paste that dropped the tag, would not have been. Both sites now go
through `craftDeliverInstant`, so a shapes-only room observer reads "a
figure" instead of the crafter's name on an instant complete, the same as
every other `SendTrio` room line in the codebase.

### Storage refuses hot stolen baubles (`storage.go`)

Every `storage add` path (one, several, all of a name, everything) asks
`storageRefusesStolen` (or, for a named add, `storageFindAddable`, which
passes over a hot match to a cool one of the same name and checks an
explicit `@handle` pick too) first: a bauble stolen within
`BaubleStolenHeatHours`, in this room's heat area, and not returned since
(`baubles.ItemIsHotIn(itm, room.Zone, now)`, read through the `storageNow`
clock) stays with the player and they are told
why (`storageSayStolen`); `add all` stores everything else. The Thornwall
Bank's item vault is storage, so this is "the bank will not take it". The
`add N` loop counts only deposits that happened. `give` hands a bauble
given to a mob to `actions.StolenBaubleGiven`, which treats one given back
to its owner as a return (docs/baubles Phase 6c), and marks any other
bauble given to a mob as a gift (`baubles.MarkGiven`).

### Crafting: storage is part of the answer (`craft.go`)

`user.ItemStorage` is known here and nowhere below. `actions.InitiateCraft`
is shared with mobs, which have no storage, so anything storage-aware is the
command layer's job. Three helpers hold that split:

- **`ensureComponentsFromStorage(user, room, recipe)`** pulls missing
  components out of the bank. ⚠️ **Must run before every dispatch in
  `Craft()`**, and every gate that would refuse the craft anyway belongs
  INSIDE its guard clause (recipe known, station satisfied, and
  `IsCrafting()`, which mirrors `InitiateCraft`'s `AlreadyCrafting`). An
  early return in `Craft()` instead would skip the pull for every path
  beneath it; `craft_storage_order_test.go` guards that.
- **`storageCompletable(user, r)`** asks whether the bank could complete
  this recipe right now. Used by `classifyRecipe` to bucket a row as ready.
- **`storageAwareMissingTag(user, r, fallback)`** gives the tag a refusal is
  allowed to print, from `crafting.HasIngredientsWithStorage`. 🐛 Prod
  defect 2026-09-21: the storage pull is all-or-nothing, so a shortfall the
  bank cannot fully cover moves nothing, and every refusal below was written
  from the carried-only `HasIngredients` answer. `craft setting` told a
  player "You are missing: copper-wire." with 39 in the bank, because
  copper-wire is the recipe's first ingredient; the real blocker was
  chrysalis-shard. Both refusal sites (the `InitiateCraft` result switch and
  `craftEnchanting`) now print through this helper, and `recipeStatus` asks
  `crafting.HasIngredientsWithStorage` directly for the `craft list` row.

Read that with the ordering rule above: running before the dispatch is
necessary and not sufficient. A path below the pull still has to ask a
storage-aware question, because all-or-nothing legitimately leaves a
shortfall in place.

## Fixtures (lighting 5e, ruling R9)

A fixture is part of the room. `look` leaves it out of "On the Ground" and
renders `descriptions/fixtures` right after the room description: one line
each, "The <Name> is lit." or "is unlit." from `itemlight.Lit`, coloured by
the description's own day/dark rule (`fixtureLines`). A darkness fixture
reads "The <Name> swallows the light." while it darkens and "The <Name> is
still." while it does not. `look <name>` still finds it ("You look at the
<Name> here:"). `get <fixture>` and `steal <fixture>` answer "The <Name> is
fixed in place." (`fixedInPlace`); `get all` passes fixtures without a word,
and `get all <name>` that only matches a fixture says it is fixed in place.
