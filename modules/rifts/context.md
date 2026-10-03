# modules/rifts Context

## Purpose

Connects `internal/rifts` to the running game. All rules live in
`internal/rifts`; this module only wires them in. It also persists the
lost-items record (plugin storage `lost-items`, loaded at `onLoad`, saved
on every change through `rifts.SetLostChanged`; a record that fails to
load is never saved over).

## Files

- **rifts.go**: `init` registers the plugin `rifts`, the commands, the exit
  router and entry guard (`rooms.AddExitRouter(rifts.Router)`,
  `rooms.AddEntryGuard(rifts.EntryGuard)`), the search feature-cache hook
  (`actions.SetFeatureCacheHook(searchRubble)`: `search rubble` in a rift
  room claims the pile's one find via `rifts.ClaimRubble` and starts a bauble
  find of the returned tier with `actions.StartBaubleFind`), the roomlife
  place hook (`roomlife.SetPlaceHook(riftPlace)`: a rift room's generated
  ambient events use the profile's chance and setting, timeless), the event
  listeners and an
  `OnLoad` callback that restores today's portal sites from plugin storage
  (`portal-sites`, `rifts.SiteState`). Sites are saved whenever they change.
- **gen.go**: the room writer (`roomWriter`, a `rifts.Generator` and a
  lively feature: `apiframework.PurposeRifts`, `ConsumerRifts`,
  `DimRiftsKeyholder`, breaker `rifts-moderation`) and its config
  (`GenConfig`, `buildGenConfig`). `Reserve` refuses when the room could not
  be moderated (so no tokens are spent on a room that would be dropped);
  `Generate` asks through `lively.Ask`, builds the room with
  `rifts.BuildGenerated`, moderates every text including noun and exit
  names, and sets the provenance.
- **prompt.go**: `RoomPromptVersion` and the prompt (`buildRoomMessages`).
- **files/data-overlays/config.yaml**: `Modules.rifts` defaults.
- **files/data-overlays/keywords.yaml**: help categories for `answer`,
  `touch` and `lenses`.
- **files/datafiles/templates/help/**: `answer.template`, `touch.template`,
  `lenses.template`.

## Commands

- `1c`, `c1`, `1c 4e` (player, at a lens table): turn lenses. Not
  registered commands: a `usercommands.AddFallbackHandler` hook hands a
  word nothing else claimed to `rifts.TurnLenses`, which only claims a lens
  place in a room with a table.
- `answer <words>` (player): answers a riddle room (`rifts.Answer`).
- `touch <thing>` (player): touches a room noun; drives sequence puzzles
  (`rifts.Touch`), harmless anywhere else. Touching a portal site's crystal
  is `go crystal`.
- `enter crystal` needs no code here: `enter` is already an alias of `go`.
- `rift` (admin): `open [profile]` places a portal site in the current room
  for today and makes it joinable; `sites` lists today's sites; `rotate`
  moves them now; `unsite` removes the one here; `list` lists runs; `info`
  shows the current rift room's pool, template and doors (and a lens table's
  layout and stage); `lost` lists the lost-items record; `gen` shows the
  bank's room counts (authored + generated) and `gen <pool>` writes a new
  room of that pool now on your own key; `key` gives
  yourself a key; `hunt` sets the rift's hunter on you as if you had fled a
  fight; `lore` shows your lore progress (`lore read` makes you a reader, `lore reset` forgets it,
  for testing); `close [run id]`
  force-ends a run.

`rift` and `rifts` are also help aliases for the older instance system
(`keywords.yaml`, `instances` topic, the Riftkeeper NPC). Player-facing rift
wording comes from each profile (`portal_exit`, messages), so the overlap is
only in the admin command's name and the help alias.

## Listeners

`RoomChange`, `MobDeath`, `PlayerDeath`, `PlayerDespawn`, `PlayerSpawn` and
`Looking` call the matching `rifts.On*` handler. `NewRound` runs
`rifts.Sweep()` (teardown, portal upkeep) and, when enabled,
`rifts.DailyTick` (moves the sites when the day turns), saving after a move,
and re-reads the live `Generate*` settings.

## Config (`Modules.rifts`)

- `Enabled` (true): sites move on their own each day. Admin `rift open`
  works either way.
- `ExcludeZones`: zone names a site never appears in, on top of each
  profile's own rules (towns whose zones are not built from city biomes).
- `GenerateEnabled` (true), `GenerateChance` (25, live),
  `GenerateMinSecondsPerPlayer` (300, live), `GenerateTimeoutSeconds` (75),
  `GenerateMaxCompletionTokens` (4000), `GenerateDailyTokensPerUser`
  (40000, live), `GenerateMaxPerPool` (200, live), `GenerateModerateOutput`
  (true), `GenerateLogRequests` (false): generated rooms.

How many sites, which regions and biomes, and which city and road zones to
skip are per profile, in `rifts/profiles/<id>.yaml`.

## Dependencies

`internal/actions`, `internal/apiframework`, `internal/lively`, `internal/baubles`, `internal/events`, `internal/messaging`, `internal/roomlife`, `internal/mudlog`,
`internal/plugins`, `internal/rifts`, `internal/rooms`, `internal/users`,
`internal/util`.
