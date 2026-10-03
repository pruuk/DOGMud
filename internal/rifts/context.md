# rifts Context

## Purpose

Rifts are portal-entered, non-Euclidean dungeons built one room at a time.
A door's destination *pool* (A passage, B feature, C puzzle/trap, D monsters,
E boss, F exit) is rolled by weight when its room is built; the rooms behind
a room's doors are built when a player first enters it; rooms are torn down
behind the party, so there is no going back and no map. The only way out is
an exit (F) room, and every door into one is locked: it needs a generic,
consumable key (the profile's `key_item_id`) that only bosses and hard
puzzles give.

Design: `claude/rifts-design.md` in the project docs. Wiring (listeners,
commands, router registration, config) is `modules/rifts`.

Generated rooms (`gen.go`): as rooms are built, a model now and then
writes a new room for the bank on a player's own key (a lively feature,
installed by `modules/rifts`), and the pools grow. Not yet built: more
puzzle kinds, and per-profile loot beyond keys and ore.

## Files

- **data.go**: `Pool` (`PoolPassage`..`PoolExit`, `AllPools`), `IntRange`,
  `MobTiers`, `TierStatPools`, `Profile` (+ `Msg`, `Templates`), `LoreSpec`,
  `WritingSpec`, `Messages`, `RequiredMessages`, `DoorSpec`, `PuzzleSpec`,
  `TrapSpec`, `EncounterSpec`, `OreSpec`, `Template`; `GetProfile`,
  `ProfileIds`, `DataDir`, `LoadDataFiles`, `ValidateMutators`; unexported
  `loadFrom`, `readStrict`, `validate`, `wrap`, `anyChance`.
- **roll.go**: pure decisions, `rng(n)` injected: `RollPool`, `PlanDoors`,
  `RollRange`, `StatPool`, `ChooseDoors`.
- **runtime.go**: `Run`, `RiftRoom`, `Door`; `RunForRoom`, `RoomInfo`,
  `Runs`, `GetRun`, `IsRiftRoom`, `Sweep`, `(*Run).ForceEnd`; unexported
  `newRun`, `buildRoom`, `pickWriting`, `expand`, `removable`, `sweep`,
  `over`, `end`, `hasPlayers`, `hostilesIn`.
- **router.go**: `Router` (an `rooms.ExitRouter`) and `EntryGuard` (an
  `rooms.EntryGuard`).
- **portal.go**: portal sites. `Site`, `SiteRecord`, `SiteState`,
  `SiteSettings`; `Today`, `Sites`, `SiteAt`, `State`, `Restore`, `DailyTick`,
  `Rotate`, `AddSite`, `RemoveSite`, `OpenPortal`, `PortalNoun`,
  `(*Run).ClosePortal`, `(*Site).String`; errors `ErrNoProfile`, `ErrNoRoom`,
  `ErrRoomEphemeral`, `ErrPortalTaken`.
- **lore.go**: lore objects, writings and rubble. `OnLook`, `IsReader`,
  `LoreStudied`, `FragmentsRead`, `ResetLore` (admin), `ClaimRubble`.
- **events.go**: `OnRoomChange`, `OnMobDeath`, `OnPlayerDeath`,
  `OnPlayerDespawn`, `OnPlayerSpawn`.
- **puzzle.go**: `Answer`, `Touch` (riddle and sequence kinds, supported
  but unused by shipped rooms: their solutions never change); trap and
  solve/punish helpers.
- **instance.go**: who may share a run. `sameParty`; unexported
  `company`, `claimed`, `mayJoin`, `claim`, `partyRunAt`. A run belongs to
  its first entrant and their party; the entry guard commits a player to a
  run (`claims`) on the move through the portal.
- **lost.go**: losses and the lost-items record. `LossSpec` (profile
  `losses`), `LostItem`, `LostRecord` (walked by the bauble sweep's `rifts`
  source, `WalkLostItems`), `LostState`, `LostSnapshot`, `RestoreLost`,
  `SetLostChanged`, `LostCount`; unexported `forfeit`, `keepable`,
  `findLost`.
- **memory.go**: the lens table, puzzle kind `memory`. `MemorySpec` (the
  profile's `memory` block), `MemoryBoard`, `TurnLenses` (called by the
  module's command fallback for words like `1c`), `IsLensPlace`,
  `MemorySolution` (admin); unexported `newMemoryBoard`, `turnLens`,
  `render`, `lookTable`, `tellOthers`, `parseCoord`.
- **hunter.go**: `Hunt`, `(*Run).Hunted`, start/end/sweep of hunts,
  `fledFight`, `removeMob`.
- **gen.go**: generated rooms. `Generator` (`Chance`, `MaxPerPool`,
  `Reserve`, `Generate`), `SetGenerator`, `GenRequest`
  (`(*Profile).GenRequest`), `RoomReply` and its parts, `RoomSchemaName`
  (`rift_room`, listed in relay.js `LIVELY_SCHEMAS`), `RoomSchema`,
  `ParseRoomReply`, `BuildGenerated`, `ErrUnusableRoom`, `ForceGenerate`
  (admin), `ErrNotGenerating`, `ReadProfiles` (tools and tests); unexported
  `maybeGenerate` (called by `buildRoom`), `startGeneration`, `addGenerated`,
  `genRequest`, `exampleJSON`, `goodName`, `mentions`.
- **keys.go**: `PurgeKeys` (every rift-only item: keys and lights, carried,
  worn, or in a charmed companion's pack), `GiveKeyTo`.
- Tests: `data_test.go` (shipped data validates, broken data is named),
  `roll_test.go` (weights, min depth, no-soft-lock rules), `runtime_test.go`
  (portal, guard, router rules, teardown behind the player, logout, portal
  timeout, sequence puzzle, late joiner, a sweep between a move and its
  event, missed departures, ForceEnd, login back at the portal, lights
  purged), `sites_lore_test.go` (join window and fresh run, idle release,
  same-day restore, lore study and unlock, writings, placement of lore,
  writings and rubble, rubble claimed once), `hunter_test.go` (several of
  one kind in a room, flee through the seal starts a hunt, the hunter
  appears, holds, withdraws, dies; leaving ends it; it seals nothing),
  `gen_test.go` (the bank rules fit every shipped room, refusals, the
  request and schema, a run grows the pool and the file reloads, caps and
  one-at-a-time, bad generated files skipped at load),
  `test_main_test.go`.

## Data

`<DataFiles>/rifts/profiles/<id>.yaml` is a profile; its templates are
`<DataFiles>/rifts/rooms/<id>/<template id>.yaml`. File names must equal the
`id` field. Both are decoded with `yaml.UnmarshalStrict`: an unknown key is a
boot error. `LoadDataFiles` runs in `main.go` `loadAllDataFiles` after
`mobs.LoadDataFiles` (mob, item and condition ids are checked) and panics on
bad data like the other boot loaders. Checks include: every rollable pool has
a template; door ranges are sane (two or more doors except in an exit room)
and each template offers the pool's maximum; exit names are one lowercase
word, unique in the room; titles are canonical title case; puzzle rooms carry
a puzzle or trap, monster rooms an encounter; a puzzle names its
`sealed_door`, sequence nouns are room nouns, a memory puzzle has a profile
`memory` block and no room noun of its own named like the table (whose
symbols are 18+ distinct non-alphanumerics and `fails` 1+ each), only a
`hard` puzzle rewards a
key; every `RequiredMessages` key is authored; the key item is type `key`
with no `keylockid`; the light item is type `light`; lore has a noun, look
and mention; writings have distinct nouns; rubble has a noun, look, mention
and a valid bauble tier for every pool it can appear in; shared
`idlemessages` are required; `ambient_generated_chance` is 0-100 and needs
an `ambient_setting` when above 0.
A `generation` block (`GenerationSpec`) names the pools a model may write
for and needs a `setting`, a guide per pool, `effects` (word to condition
id) and `seeds`. A room file named `gen-*` (or `source: generated`) that
fails to read or validate is skipped with a warning, never fatal.
`ValidateMutators` (called in `main.go` right after `mutators.LoadDataFiles`,
which runs late in boot) checks `portal_mutator`.

## How a run works

1. **Portal sites.** Each real-world day (`Today`, server local date)
   `DailyTick` runs `Rotate`: every region in `portal_regions` gets
   `portal_sites_per_region` sites, chosen from room templates (not loaded
   rooms) whose biome is in `portal_biomes`, outside zones whose default biome
   is in `portal_skip_zone_biomes` (cities) or whose name contains a
   `portal_skip_zone_words` word (roads), outside instanced and
   non-Cartesian zones and the module's `ExcludeZones`. A site shows itself
   with the `portal_mutator` (a description line; decays in a day as a
   backstop) and a `portal_exit` noun. `modules/rifts` persists `State` and
   `Restore`s it at boot when it is from the same day.
   While a player stands at a site, `ensureRun` holds a pending run
   (`newRun`: an owned chunk and the entry room) and a temporary exit named
   `portal_exit` into it, so `enter crystal` (an alias of `go`), `go crystal`
   and `touch crystal` (the module hands it to `go`) all work and followers
   follow. The exit is routed (`routePortal`), so `look crystal` shows the
   noun and `picklock` refuses. The first player in starts a
   `join_window_seconds` window for the rest of the party; then the run
   closes to newcomers and the site makes a fresh run for the next player.
   Once the party has moved on from the entry room, a late joiner is routed
   to the room a member stands in (`frontierRoomId`), not the entry room. A
   pending run nobody entered is let go once nobody stands at the site.
   `keepPortal` re-adds the exit if the room was reloaded. Every run keeps its
   `OriginRoomId`, so its way out leads back to where it was entered even
   after the sites have moved.
2. **Building.** `buildRoom` picks a template (unused in this run first),
   rolls the door count and which of the template's doors appear, then rolls
   each door's pool with `PlanDoors`. Door descriptions become nouns
   (`look <door>`). D and E rooms get their mobs placed as soon as the room
   exists (`spawnMob`: never wander, never respawn, stat pool by tier plus
   `statpool_per_depth` per room of depth). Not through `SpawnInfo`: the
   orphan check in `Room.Prepare` folds two slots of one mob id into one
   mob, so a room of three Glint Stalkers would hold one. An ore roll adds the
   ore to the room's forage pool (`forager.RoomExtraYieldsKey`) and a seam
   noun. A `light_item_id` item may be left on the floor (`light_chance` by
   pool). A lore object (`lore`, chance by pool), a writing
   (`writing_chance`; a fragment not yet placed in this run first) and a
   rubble pile (`rubble.chance`) are added as nouns with a sentence appended
   to the description. None of these is a fixture: each is rolled when the
   room is built and goes when the room is torn down after it is left, and
   lights cannot be carried out (`PurgeKeys`). Every room gets
   `allow_recall=false`. Door exits point at the room itself until `expand`.
3. **Entering** (`OnRoomChange`): the player joins the run; if they came
   through a locked door, one key is spent and the door stays open for the
   rest of the party; the room `expand`s (the rooms behind its doors are
   built and its exits pointed at them); a trap may spring.
4. **Doors** (`Router`): mobs never pass; D and E rooms are sealed while a
   living, uncharmed, auto-aggro mob is present; a puzzle's sealed door waits
   for the solve; a locked door needs a key; an unbuilt destination is
   "unsettled". The exit room's `exit_name` exit routes to the portal's room
   (or the start room). Routed exits are also what keeps `picklock` off rift
   doors (`ExitRoute.PickRefusal`) and flee honest (`GetRandomExitFor`).
5. **Teardown** (`Sweep`, every round): a room is removed when nobody is in
   it and either someone has been in it or the room that led to it is gone.
   "Nobody" counts `RiftRoom.present` too: arrivals the `RoomChange` handler
   has seen and whose departure it has not. A sweep can run between a move
   and its queued event, and the handler needs the room it is told about.
   `checkMembers` is the backstop for departures never seen (keys purged).
   A run ends when nobody is inside and no portal is open; anyone still
   standing in its rooms is moved out first. Never from inside a
   `RoomChange` listener (see `rooms` context, `OwnedChunk.RemoveRoom`).
6. **Leaving.** Stepping out to a non-rift room leaves the run and purges
   keys. Death purges keys (`OnPlayerDeath`); the respawn move then leaves the
   run. Logging out leaves the run (`OnPlayerDespawn`). The portal room a
   player entered by is kept in MiscData `rift-origin` while they are inside,
   so at login (`OnPlayerSpawn`, which runs before core placement) a player
   who logged out inside a rift is moved back to it, even after a reboot or
   once the run is gone; `EntryGuard` redirects there too. `OnPlayerSpawn`
   also purges stray rift-only items from anyone arriving outside a rift.
   `end` and `ForceEnd` purge the items of the players they move out
   themselves, since the run is gone before their event is handled.

7. **Lore** (`OnLook`, from the `Looking` event, sneaking or not): studying
   a lore object not studied before says `lore_progress` and records its
   placement token in the character's MiscData `rift-lore-seen:<profile>`;
   the `lore.needed`-th says `lore_unlocked` and sets the permanent
   `rift-reader:<profile>` tag. Nothing else is said: no counts, no hints.
   Looking at a writing shows its description to anyone, then
   `writing_unreadable`, or for a reader its fragment;
   `rift-fragments:<profile>` records which they have read.
8. **Ambience.** A room's `IdleMessages` are its template's lines plus the
   profile's shared `idlemessages`, so they come on the world's ambient
   timing (`hooks.UserRoundTick`, 5% a round) through the world's senders.
   `AmbientPlace` (installed as `roomlife.SetPlaceHook` by the module) has
   such a line written fresh by a model `ambient_generated_chance` percent of
   the time (in place of `Modules.roomlife.Chance`), for `ambient_setting`,
   with no time of day: the same roomlife path, key rules and moderation as
   the world's. Nothing generated is kept.
9. **Rubble** (`ClaimRubble`, through `actions.SetFeatureCacheHook`, set by
   the module): the first `search rubble` in a room says `rubble_found` and
   the module delivers a bauble of `rubble.tier[pool]`
   (`actions.StartBaubleFind`); later searches say `rubble_empty`. Any other
   feature, or a room outside a rift, is left to the normal search.

## Gotchas

- `Router` and `EntryGuard` must spend nothing: look, picklock, unlock and
  flee ask the router; every placement asks the guard. Keys are spent after
  the move, in `OnRoomChange`. Their only effects are building a site's
  waiting run (`routePortal`) and recording a claim (`EntryGuard`).
- Runs live in memory only. A reboot or copyover drops them; players saved in
  a rift room land via the login fallback, and keys are purged on spawn.
  Sites are persisted by the module, runs are not.
- The site's mutator is saved with the overworld room. `Rotate` removes it
  from yesterday's rooms, and `Restore` clears it from the rooms of a saved
  day that is not today; if that state is ever lost, the mutator's own
  24-hour decay clears it. A room with an authored `crystal` noun is never a
  site, and `removeSite` only deletes the noun it put there.
- Mobs never cross a rift door or the portal: `mobcommands.Go` asks the exit
  routers on a mob's behalf (user id 0), and a forced `go <roomId>` into,
  out of or within an owned chunk is refused (`rooms.MobMayMove`). Rift mobs
  are `charm_immune`.
- Duplicate mob ids in one room's `SpawnInfo` collapse to a single mob (the
  orphan check in `Room.Prepare`). Rifts avoid it by placing mobs directly;
  authored rooms still have it.

## The Lenses (Obelisk mobs) and the hunter

Species 46 Lens: crystal security constructs, no body parts (no trip, kick,
bash or grapple; the legacy special-move AI finds nothing), charm- and
pack-flee-immune. Each kind's fight is its own behaviour tree in
`behaviors/rift_obelisk/`, with mob-only spells for its signature moves:

| Mob | Tier | Fight |
|---|---|---|
| 9835 Glint Stalker | trash | Sneaks when alone; ambushes; Splinter Rend (a bleed) once or twice; `vanish` (break off and sneak where it stands); lies low two idle rounds; again |
| 9836 Spine Lattice | trash | Shard Volley (`hits: 3`, three shards in one resolution) every 4 rounds; melee while it regrows |
| 9837 Splitlight | elite | Refraction (condition 133: return damage, dexterity); Prismatic Siphon (telegraphed area drain that heals it) every 4 rounds |
| 9838 The Watching Obelisk | boss | Two Stalkers at 75%, a Spine Lattice at 40%, overload at 30%; Convergence (4 folds, damage breaks it, spares its adds) every 11 rounds (7 overloaded); Facet Lance between |
| 9839 Facet Hunter | hunter | Wall Lance (2 folds, damage cannot break it) every 5 rounds, Lattice Burn (room-wide) every 11, melee between |

**Stealth** follows the ordinary rules: a sneaking player is rolled against
the room's watchers on entry (`actions.EntryDetection`, perception plus
search skill); undetected, nothing attacks them (`lookfortrouble`, the
random-player picker and `players_in_room` all skip hidden players, and a
wild mob's area harm and area drain miss them). A hidden player also slips
past a monster room's seal. Glint Stalkers have the lowest perception of the
Lenses (50); the Facet Hunter the highest (230, search 6).

**Lurkers** (`lurkers`): a passage holds a Glint Stalker 10% of the time. It
does not seal the passage.

**The hunt** (`hunter.go`). Monster and boss rooms are shut to walkers they
know of; a flee gets out (`rooms.FleeRouting`, set by `actions` flee).
Leaving a rift room while something there is fighting the player, or a
sealed room while they were seen (`fledFight`), hunts them: one hunt per
player. The hunter (`Mobs.Hunter`) is made once and is the same creature
from then on: it appears in the quarry's room `hunter_delay_seconds` (10 to
60) after they come into it, if they are still there; moving on restarts
the count, so a player who keeps moving stays ahead. Beside a hidden quarry
it does not attack: it rolls to spot them on arrival and every 2 rounds
(`actions.SpotHiddenPlayer`), then attacks. While it stands with a quarry it
can see, `Router` refuses that player every way out, the breach included;
others come and go, and a hidden quarry may slip away. When the quarry is
elsewhere it withdraws (`parkMob`: out of its room and the registry,
unchanged) and comes back (`mobs.RestoreInstance`) in their next room after
the delay, wounds and all. The hunt ends when the hunter dies
(`OnMobDeath`) or the quarry leaves the rift (exit, death, logout, `end`);
fleeing another fight later starts a new hunt with a new hunter. The hunter
never counts for `hostilesIn`. Admin `rift hunt` starts one on yourself.

## Dependencies

`internal/casing`, `internal/conditions`, `internal/configs`, `internal/dice`,
`internal/events`, `internal/exit`, `internal/forager`, `internal/items`,
`internal/messaging`, `internal/mobs`, `internal/mudlog`, `internal/mutators`,
`internal/rooms`, `internal/users`, `internal/util`. Imported by `main.go` (loader) and
`modules/rifts`.

## Generated rooms (gen.go)

1. **Trigger.** `buildRoom` calls `maybeGenerate(pool)` under the mud lock.
   With a generator installed, a pool in the profile's `generation.pools`,
   a member in the run, the generator's `Chance` rolled, the pool under
   `MaxPerPool` generated rooms and none of that profile and pool already
   being written, the first member (in a random order) whose key the
   generator will `Reserve` gets the job. The room being built never waits:
   it uses the bank as it is.
2. **Request** (`genRequest`, under the lock, a snapshot): the profile's
   canon (`setting`) and the pool's guide, the exact door count (the pool's
   `doors.max`), description word bounds, nouns the game places itself
   (lore, rubble, writings, ore, portal, exit name), the effect words, three
   example rooms in the reply's shape (authored first), every title and
   description opening in the pool, three seed words, and a frozen copy of
   the profile without its bank.
3. **Off the lock** the module calls the model (`modules/rifts` gen.go) and
   `BuildGenerated` holds the reply to the bank's rules: plain ASCII prose
   of the right lengths, exactly the door count, nouns and exits of 3 to 16
   letters that are no command or direction and not reserved, every noun
   mentioned, a new title and opening, a C room with a lens table
   (never a key reward) or trap and a "sealed" way out, a D room with a
   trash (2-3) or elite (1) encounter, an F room naming its way out; then
   the loader's own `validate`.
4. **Bank** (`addGenerated`, back under the lock): the title is checked
   again, the id made unique (`gen-<pool>-<slug>`), the template saved with
   `util.Save` to `rooms/<profile>/<id>.yaml` (`source: generated`, `model`,
   `prompt_version`, `created`) and appended to the pool. `pickTemplate`
   gives authored rooms half the picks when both kinds are on offer.

## The lens table (memory.go)

- **Board:** built with the room (`buildRoom`): 18 symbols drawn from
  `memory.symbols`, each on two of 36 cells, shuffled. A new board every
  build; the layout never changes between stages. The table's noun and a
  mention sentence are added to the room.
- **Turning:** `TurnLenses` takes one or two places (`1c`, `c1`, `1c 4e`),
  checked whole before any turns. Readers only (`IsReader`); not in combat;
  nothing on a dark or solved table. One lens waits for its partner at a
  time (`Open`, `OpenBy`); one left by a player no longer in the room sinks
  back for free.
- **Misses:** a mismatch costs one of the stage's `fails` (8, 4, 2); both
  are shown (red) and turn back. A stage running out turns every lens face
  down, locked ones too; after the last the table is `Dark` for good and
  the router refuses its door with `dark_look` (readers) or `sealed_puzzle`.
- **Solve:** all 18 pairs locked → `solve` (door unsealed, reward; an ore
  reward now tells the solver what they took).
- **Who sees what:** the turner gets the board; others in the room get
  sight-gated lines, the readable ones only if they can read
  (`seen`/`match`/`reset`/`dark`, else the `*_unread` versions); a hidden
  turner is "Someone".
- **No strand:** a puzzle room keeps a door besides its sealed one that
  needs no key (`keepAWayOpen`), since a table can be lost.

## Instances, keys and losses

- **One run per party.** `routePortal` names a room and has no side effect
  beyond building the site's waiting run: the run of the player's party if
  it is open to them (`partyRunAt`), else the waiting run. `EntryGuard`
  admits a non-member only from the run's origin room, into its entry or
  frontier room, when `mayJoin` (a fresh run, or in a party with everyone it
  belongs to), and then `claim`s it; a claimed waiting run stops being the
  site's, so the next stranger gets a new one. Nobody flees into a rift.
- **Keys** come only from bosses and lens tables: a memory puzzle must
  reward a key (hard), and no other kind may. Generated lens tables give a
  key too.
- **No free way out:** `rooms.NoRecall` (temp data `allow_recall=false`)
  refuses the `tutorial` command, fold anchors, quest reward and quest
  engine teleports and behaviour-tree `move_player` out of a rift room;
  fold-recall already honoured it. Shooting through a routed exit is
  refused. Every rift mob must be `submission_policy: lethal` (checked at
  load): a subduing mob would send its victim home free.
- **Losses** (`losses`): death takes everything carried loose (pack,
  component bag, bandolier), all carried gold, and each worn piece at
  `worn_chance` (25%); logging out inside takes what is carried loose, told
  at the next login; a linkdead player killed while waiting pays only the
  logout cost. Never taken: keys, tickets, quest-token items, bound items
  and house keys, rift-only items. A reboot costs nothing.
- **The record:** taken items (not gold) go into `lostItems` (oldest drop
  off past `keep`), persisted by the module. The first search of a rubble
  pile has `find_chance` (5%) to give 1 to `find_max` (2) of them, chosen at
  random, removed from the record for good (an item that does not fit stays
  in it); stolen-goods marks are cleared.
