# Sight Gates Wrap-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the sight-gates epic (#382) in one PR on `fix/sight-gates-wrapup`: narration names without adjective tags (#453), typed names resolving only at full sight (#454), departure and quit lines judged before the move (#456), the sneak hide surviving a reload (#451), a mob fold that never ends silently (#242), a deterministic scout scan test (#251), and copy (#455, #449).

**Architecture:** Every fix reuses a mechanism the codebase already has: `FormattedName` and `messaging.StripNameAdjectives` (H1); `admitCastAim`'s sight switch and shape helpers, lifted into `actions/sight_aim.go`, plus `messaging.HideNames` / `HideSpeakerNames` / `ParticipantSight` as `attack.go` already uses them (H2); `Room.VisualSnapshot` / `SendTextVisualToSnapshot` with a new sound twin sharing `SendTextVisualWithAudio`'s per-reader body (H3); `hideForStealthRecord` beside `reconcileShroudHide` (H4); `sendMobSpellFailed` and `recordConcentrationFailure` behind one `fizzleMobFold` (H5); the existing `actTryScan` and `actions.Scan` gates, pinned by tests (H6); the normalize pipeline's end-punctuation stage, `messaging.UnseenFigure`, `rooms.visualDecision` and the wrapper's break rule (H7).

**Tech Stack:** Go 1.x, testify, GoMud engine packages under `internal/` and `modules/`, YAML content and templates under `_datafiles/`.

**Spec:** `docs/superpowers/specs/2026-10-09-sight-gates-wrapup-design.md` (APPROVED 2026-10-09; owner calls: arrivals keep, typed names in the dark are gated; ruling: a mover is seen by their own light on the way in and out).

**Worktree:** `C:/tmp/dogmud-sight-wrapup`, branch `fix/sight-gates-wrapup`. Branch head when this plan was written: `bd1220964` (master `b1c1829ec` plus three spec commits; no Go code differs from master). The PR body says "Refs #382" and names the issues it resolves in words, never with a closing keyword.

---

## How this plan was verified

Every task below was dry-run on `bd1220964` before this plan was written, in throwaway worktrees: the code in each step was applied, each new test was run failing against the old code and passing against the new, and the affected packages, the root guard tests (`go test . -count=1`), `go vet`, `gofmt -l` and `golangci-lint run --new-from-rev=HEAD` (0 new issues) were run after each fix group. The four fix groups (H1 with H3, H2, H4 to H6, H7) were each dry-run in task order on their own base, and then all 19 code tasks were applied together in plan order on one fresh `bd1220964` worktree and the whole tree was run (see "Integrated dry run" below). The worktrees were then removed. Line numbers below are from `bd1220964`; each edit is anchored by quoted text, and where an earlier task in this plan shifts a line, the anchor still finds it.

## Facts verified against source (`bd1220964`)

Built by grepping and reading for this plan, not copied from the spec. One table per fix group.

### H1 and H3

| Fact | Where (verified) |
|---|---|
| `GetCharacterName(ansi bool)` returns `c.getFormattedName(0, "username").String()` for `ansi` | `internal/characters/formattedname.go:164-169` |
| `FormattedName.String` appends the adjective span (`:81-94`), the quest star (`:96-98`) and `" and " + PetName` (`:100-102`) after the tag | `formattedname.go:72-103` |
| `getFormattedName` fills `Adjectives` from `GetAdjectives()` (`:195`), sets `Suffix` `dead` or `aggro`, and `PetName` (`:207`); it never sets `QuestAlert` (only `rooms/roomdetails.go:339` does) | read; grep `QuestAlert\s*=` |
| `GetAdjectives` adds `dead`, `shop`, `lit` (`EmitsLight`), `hidden` (`IsHidden`), `poisoned`, then the static `c.Adjectives` | `characters/description.go:144-174` |
| 29 production `GetCharacterName(true)` calls (plus the comment at `Condition_ApplyConditions.go:152` and `messaging/anonymize.go:11`) | grep, excluding `_test.go` |
| Mob holder names: `Condition_ApplyConditions.go:155-157` (`mobDisplayName(m, r, 0)`), `NewRound_MobRoundTick.go:290`, `NewTurn_PruneConditions.go:104-106`; `mobDisplayName` is defined at `NewRound_DoCombat_helpers.go:398-404` and renders `GetMobNameIndexed`, which carries the span | read |
| `messaging.StripNameAdjectives(text string) string` keeps the identity tag, drops the span after it | `messaging/hidenames_tagged.go:45` |
| Test fixture: `seedAllRegistries` (hooks) puts users 1 Aliceia and 2 Bobrick and mob 100 Skeleton in room 1; `seedNarrationConditions` seeds glow 7001 (start), shiver 7002 (trigger), fade 7003 (end), lantern 7006 (a light, `EffectLightStrength` 50) | `hooks/hooks_test.go:57`, `hooks/narration_testhelpers_test.go:37-108` |
| Quit line: `if _, ok := room.RemovePlayer(evt.UserId); ok { tplTxt, _ := templates.Process("player-despawn", ...); sendVisualRoomText(room, messaging.CategoryLogout, tplTxt) }` | `hooks/PlayerDespawn_HandleLeave.go:150-153` |
| `sendVisualRoomText(room, cat, msg, excl...)` is `room.SendTextVisual` | `hooks/NewRound_DoCombat_helpers.go:436-441` |
| `onPlayerDespawnForAwareness` (default priority) runs `Awareness.ForceVisible` BEFORE `HandleLeave` (`events.Last`); so at the quit line a hidden quitter is already Visible and every reader `Perceives` them | `hooks/Logout_AwarenessCleanup.go:22-41`; `hooks/hooks.go:101`; `state/awareness/awareness.go:245-297` |
| `Character.Perceives(other)`: true for self, a not-hidden `other`, or a viewer with see-hidden | `characters/character.go:965-973` |
| `UserRecord.SetTempData(key, value)` (nil deletes) / `GetTempData(key) any` | `users/userrecord.go:545-575` |
| hooks tests point `FilePaths.DataFiles` at an empty temp dir, so `templates.Process` cannot be exercised there | `hooks/hooks_test.go:35-55` |
| `Room.VisualSnapshot()` `rooms.go:400`; `SendTextVisualToSnapshot(snap, cat, txt, names, excl...)` `:419`; `SendTextVisualWithAudio(cat, visualTxt, audioTxt, excl...)` `:490-537`, whose per-reader decision switch equals `visualDecision` (`:440-450`) | read |
| Player departure: `rooms.MoveToRoom` at `usercommands/go.go:265`; the two departure lines (pet and no pet) `room.SendTextVisualWithAudio` at `:308` and `:316`; arrival lines at `:331` and later | read |
| Mob forced `go <roomId>`: `RemoveMob`/`AddMob` at `mobcommands/go.go:76-78`, exit line `sendMovementMessage(room, ...)` `:82-86`, entry `:89-93`; `sendMovementMessage` (`:37-39`) is `room.SendTextVisualWithAudio` and has only these two callers | read; grep |
| `RelocateMob`: `RemoveMob`/`AddMob` `actions/relocate_mob.go:98-100`, exit line `from.SendTextVisualWithAudio` `:105-109`, entry `:111-115` | read |
| Carried light reaches the room through `Room.carriedTerms`, which sums every mob's and player's light records | `rooms/lighting.go:242-270` |
| The narration guard's observer list names `SendTextVisualWithAudio` and `SendTextVisualToSnapshot` but only on a receiver named `room`; `bauble_finder_view_guard_test.go:98-99` `beyondReaderCalls` lists the same senders | `messaging_surface_guard_test.go:893-921` |


### H2

| Fact | Where |
|---|---|
| `admitCastAim(actor Actor, spellInfo *spells.SpellData, targetName string) (castAim, bool)`; the sight switch is `:59-78`; shape rewrite `:80-87` | `internal/actions/cast_admission.go:40-89` |
| Shape helpers: `shapeWord` `:18`, `castNamesAShape` `:108`, `castShapeIndex` `:117`, `castFigures` `:131` (players then mobs, `Perceives`-filtered, rewritten `@<uid>` / `#<mid>`); used only in `cast_admission.go` and one comment in `cast_sight_followups_test.go:57-58` | grep |
| Cast refusals: none + empty or shape: `You can't see anything to aim at.`; none + name: `You don't see them here.`; shapes + name: one-line hint `You can only make out shapes here. Try <cmd>cast X shape</cmd> or <cmd>cast X 2.shape</cmd>.` | `cast_admission.go:62`, `:64`, `:73-75` |
| `admitCastAim` does NOT check `IsPlayer`: a mob caster is gated too (slice F), pinned by `TestCastSight_MobCasterNeedsSightToo` | `cast_admission.go:40-89`; `cast_sight_test.go:162` |
| Called once, `InitiateCast` step 2b | `actions/cast.go:99` |
| `messaging.ParticipantSight(observer, room)` returns `SightNone` for Blinded, `SightFull` for a nil observer or nil room; no sleep gate | `messaging/predicates.go:56-87` |
| `FindByNameSeenBy` resolves `@<uid>` and `#<mid>` and applies `viewer.Perceives` to them | `rooms/rooms.go:2050-2057`, `:2084-2109` |
| `ResolveTargetActor(r, name, opts...)` = `FindByNameSeenBy(o.Viewer, ...)` + exclusions | `actions/target_resolution.go:79-118` |
| Player resolvers of a typed creature name (all room occupants, `Viewer: user.Character`): attack `usercommands/attack.go:103` (`FindAttackTarget`, `actions/combat_attack.go:119`); target `target.go:62`; melee specials `actions/melee_target.go:172` (11 callers: `bash.go:28`, `drain.go:22`, `gore.go:22`, `grapple.go:24`, `kick.go:22`, `maul.go:22`, `pounce.go:22`, `rake.go:22`, `taunt.go:21`, `throttle.go:34`, `trip.go:27`); give `give.go:73`; show `show.go:43`; consider `consider.go:28`; steal `skill.skullduggery.steal.go:84`; plant `skill.skullduggery.plant.go:94`; shadow `skill.skullduggery.shadow.go:54`; talk `talk.go:43`; ask `ask.go:95`; party invite `party.go:182`; rep `report.go:60`; fire `shoot.go:53` (`resolveShootTarget`, `:674-702`) and `actions/combat_fire.go:189`, `:194` | grep `ResolveTargetActor\|FindByNameSeenBy\|FindAttackTarget` |
| `party invite` and `rep` resolve ONLY room occupants (`ResolveTargetActor(room, ...)`), never world-wide | `party.go:182`; `report.go:60` |
| There is no player `follow` command: not in the `usercommands.go` registry, not in `_datafiles/world/dogmud/keywords.yaml`. Following a person is `shadow` | grep |
| `lookup_viewer_guard_test.go` keys lookups by `path\|function` with counts, not by line; `AimBySight` performs no lookup, so no entry moves | `lookup_viewer_guard_test.go:34-81` |
| Party auto-assist types a player command: `attack #<mid>` (`attack.go:221`, `hooks/NewRound_DoCombat_helpers.go:1373`) and `attack @<uid>` (`attack.go:339`) through `UserRecord.Command`, which queues `events.Input` | read; `users/userrecord.go:375-388` |
| Attack's target switch hands `Target` the creature's NAME for an empty or `*` rest | `attack.go:153-163` |
| `ExecuteFire` already refuses a shot below `CanSeeClearly(char, targetRoom)` (`TooDarkToAim`) and a Blinded shooter, but only AFTER resolving the name (`:189-194`), and `usercommands.Fire`'s pre-fire guards (self, party "X is in your party!", PvP) run on a resolved name before that | `actions/combat_fire.go:275-282`; `usercommands/shoot.go:53-83` |
| `scanReach(viewer, here, next) (SightDecision, bool)` | `actions/scan.go:199-210` |
| `SendTextVisual` room lines with tagged names are anonymized for a shapes reader by the pipeline; raw `SendText` personal lines are not sight-gated | `rooms/rooms.go:280-282`, `:451-466`; `users/userrecord.go:500-514` |
| Give personal lines: player item `give.go:90-95`, player gold `:129-134`, mob gold `:182-184`, quest-consumed `:206-208`, mob item `:233-235` | read |
| Show personal lines `show.go:61-68`, `:84-86`; party invite `party.go:201-202`; rep `report.go:76-81` | read |
| Quest return line: `user.SendText(messaging.CategoryMobEmote, fmt.Sprintf("%s hands back the %s.\n", ctx.MobName, item.Name()))`; `ctx.MobName` is the bare `mob.Character.Name` | `behaviortree/actions_quest.go:175`; `helpers.go:255` |
| `messaging.HideNames` (hides bare and tagged names, "a figure"/"something"), `HideSpeakerNames` (tagged only, "a figure"/"someone"), `Anonymize` (every identity tag to "a figure") | `messaging/hidenames.go:55`, `:94`; `messaging/anonymize.go` |
| Attack's own #214 idiom: wrap the `Sprintf` in `messaging.HideNames(..., ParticipantSight(user.Character, room))` in place; the viewpoint audit keys on the literal, so the literal stays inline | `usercommands/attack.go:186-203`; comment `target.go:181-186` |
| No `actions` function steal/plant/consider/shadow reach type-asserts `*UserActor` (the only assertions are `action_readiness.go:65`, `buy.go:328`, `drink.go:306`, `melee_target.go:238`) | grep `\.(\*UserActor)` |
| Usercommands test fixture: room 1 (city, `Lamp` 60, PvP) holds Aliceia (1), Bobrick (2) and Skeleton (mob 100); room 2 north of it; seeded "cave" biome has `SkyLight` 0 | `usercommands/usercommands_test.go:185-262` |


### H4, H5 and H6

| Fact | Where (verified) |
|---|---|
| `reconcileShroudHide()` re-enters a shroud hide for a Visible holder of a live 31 through `hideForStealthRecord(31)`; it has no sneak sibling | `internal/characters/shroud_hide.go:188-200` |
| `Validate` calls `c.Conditions.Validate()`, `c.reconcilePerception()`, `c.reconcileShroudHide()` in that order, then (only for `Validate(true)`) `c.reapplyPermanentConditions()` | `internal/characters/validate.go:694-696`, `:720-722` |
| A fresh Awareness machine is built when the field is nil (Awareness is `yaml:"-"`, so every load) | `validate.go:605-606` |
| A load runs `Validate(true)` then re-saves | `internal/users/users.go:515-519` (`loadUserFromPath`) |
| The rebuild sources a 9 only for `c.IsHidden() && !c.HiddenByShroud() && c.holdsLiveStealthRecord()`; an unsourced permanent record is `RemoveCondition`ed (expired, so the prune pass tells its end line) | `internal/characters/conditions.go:357-359`, `:363-369`, `:378-384` |
| `hideForStealthRecord(9)` on a Visible character: `TransitionToConcealing`, then `ResolveConcealmentAs(HideSneak)`; returns `refused` false for a 9 in every branch | `internal/characters/conditions.go:190-219` |
| `holdsLiveStealthRecord()` = a live (unexpired) record 9 | `internal/characters/conditions.go:230-232` |
| The Hidden cascade (`AddCondition(9, true)`) and the reveal cascade (`CancelConditionsWithFlag(conditions.Hidden)`, then `SetMiscData("sneaking", nil)`) are wired per character by `fireCharacterCreated` in `Validate`, before the reconcile calls; the `characters` package tests do not wire them | `internal/hooks/Awareness_Cascades.go:44-77`; `validate.go:647-650`; `shroud_hide_test.go:14-16` |
| `sneaking` lives in the saved `MiscData` (`yaml:"miscdata,omitempty"`); set by `actions.Sneak` | `internal/characters/character.go:324`; `internal/actions/sneak.go:177`, `:258` |
| Mob fold TargetGone is set only for a mob target missing or at Health < 1, or a user target missing (or downed, for a harm spell); a user target in another room is not gone | `internal/hooks/combat_shared_helpers.go:660-688` |
| `handleMobFoldCasting` TargetGone branch: `recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))` then `sendMobSpellFailed(mob, mobRoom, "fizzles")` | `internal/hooks/NewRound_DoCombat_helpers.go:847-849` |
| `sendMobSpellFailed` = `sendVisualElseAudible(room, CategorySpellDisruption, "<subject>'s spell <verb>.", messaging.SoundSpellSputtersOut)` | `NewRound_DoCombat_helpers.go:513-519`; `:480-486` |
| `clearCastingActivity(ch, trigger)` is a no-op unless `ch.Activity.IsCasting()` | `combat_shared_helpers.go:506-513` |
| `IdleMobs` releases a mob in combat whose user target is nil or in another room: `mob.Command("emote mumbles about losing their quarry.")`, `targeting.Release(&mob.Character, targeting.ReasonDisengage)`; `Release` is `c.EndAggro()` and touches no casting | `internal/hooks/NewRound_IdleMobs.go:64-71`; `internal/targeting/commit.go:84-89` |
| The mob fold step (`handleMobFoldCasting`) runs only for a mob `IsInCombat()`; nothing else advances or clears a non-combat mob's fold (the forager teleport `ForceFree`s its own) | `internal/hooks/NewRound_DoCombat.go:326-333`; `internal/behaviortree/actions_forager.go:505-528` |
| `DoCombat` is registered on `NewRound` before `IdleMobs`, so in one round the fold step runs first | `internal/hooks/hooks.go:59`, `:67` |
| `resolveMobSpell` resolves only targets in `room` (mob: alive and `RoomId == room.RoomId`; user: `RoomId == room.RoomId`); with none it returns false and sends nothing | `internal/hooks/spell_resolution.go:633-656` |
| `Character.CastingData() (activity.CastingData, bool)` nil-guards Activity | `internal/characters/character.go:792-797` |
| `actTryScan` promotes the first hostile sighting of `actions.Scan` to `ctx.SoftTarget`; `isHostileToMob` is true for every player | `internal/behaviortree/actions_scout.go:23-77`, `:263-275` |
| `actions.Scan` lists occupants only when `scanReach` is not `SightNone`, and only `listedOccupants` (`Perceives`) | `internal/actions/scan.go:91-106`, `:199-211`, `:224-256` |
| No test drives `actTryScan` | `grep -rn actTryScan internal --include=*_test.go`: no match (the grep matches the non-test file, so it can succeed) |
| Goblin species 5 carries condition 29; Night Vision 29 `nightvision_strength: 18`; HEAD `LightBlindBelow: 25` (line 952), `LightExitsAbove: 55` (line 959; Go default 65) | `species/5-goblin.yaml:5-6`; `conditions/29-night_vision.yaml`; `git show HEAD:_datafiles/config.yaml` |
| Goblin Scout 217: `behavior_archetype: scout`, `conditionids: [9]`, `speciesid: 5` | `mobs/ironwind_steppe/217-goblin_scout.yaml:3`, `:7`, `:40` |
| Scout tree: `try_search` branch (`:48`) before `try_scan` branch (`:58`) | `behaviors/archetypes/scout.yaml` |
| `behaviortree` test fixture `sightScene(t, biome)`: mob 8101 "Watcher" in room 8100, biome `city` lamp 90 or `cave` sky 0; seeds condition 29 with the `NightVision` flag. `hideForTest` hides a character | `internal/behaviortree/sight_test.go:21-60`, `:14`; `conditions_scout_sight_test.go:13-19` |
| `rooms.SeedRoomsForTest` replaces the whole room registry and returns its restore | `internal/rooms/test_helpers.go:8-24` |
| `hooks` test fixture `seedFallbackRoom(t, lamp, eyes)`: users 1 and 2 in cave room 2 at a pinned lamp, user 1 with the named eyes; mob 100 "Skeleton" starts in room 1 | `internal/hooks/dark_room_fallback_sight_test.go:24-47`; `hooks_test.go:98-113` |


### H7

| Fact | Where (verified) |
|---|---|
| Minimap and `map` build each symbol's tag with `strings.Replace` once per legend symbol over the whole line, `fg="map-%s" bg="mapbg-%s"` from `strings.ToLower(legend)` | `internal/usercommands/look.go:700-705`; `internal/usercommands/skill.map.go:176-179` |
| The legend template builds the same tag with `lowercase $name` | `_datafiles/world/dogmud/templates/maps/map.template:20` and the identical `_datafiles/world/default/templates/maps/map.template:20` |
| Legend names come from the room's `maplegend` or the biome `name` (`mapper.go:1041-1052`) plus `keywords.yaml` legend overrides | read |
| Multi-word legend names shipped: biomes "Deep Water", "Dense Forest", "City Backstreet", "City Thoroughfare" (dogmud), "Deep Water" (default); room maplegend "Training Yard"; override "Throne Room" (Frostfang) | `_datafiles/world/*/biomes/*.yaml`; grep `maplegend:`; `keywords.yaml:378-387` |
| Colour aliases for map tags live in `ansi-aliases.yaml` (`map-water: 12`, `map-forest: 2`, `map-city: 15`, ...); no multi-word alias exists | `_datafiles/world/dogmud/ansi-aliases.yaml:37-62`; `_datafiles/world/default/ansi-aliases.yaml:28-53` |
| `templates` already depends on `rooms`; `mapper` does not depend on `templates`, so `templates` may import `mapper` | `go list -deps` |
| `skipStages` skips every stage for `CategoryMobEmote` and for the player-typed `CategoryEmote` | `internal/messaging/normalize.go:26-46` |
| Mob emotes go out as `CategoryMobEmote` (`mobcommands/emote.go:25`, `:36`); player emotes as `CategoryEmote` (`usercommands/emote.go` via `actions.FormatEmoteText`) | read |
| About 30 other production senders use `CategoryMobEmote` (defuse, drink, forage, plant, search, sleep, steal, caravan, ferry, the admin spawn echoes, ...) | grep `CategoryMobEmote` |
| Single-line `- 'emote ...'` list items in `world/dogmud/mobs`: 715 end with no full stop (every one ends in a letter), 51 with one; 1262 more are folded over lines | grep |
| `claws.yaml` makes `{itemname}` the subject of a singular verb on 13 lines: 173, 230, 247, 305, 329, 353, 380, 385, 396, 402, 403, 424, 425; `bite.yaml` on line 173 | grep |
| Natural attacks render through the species' `unarmedname` ("claws", "fangs", "jaws", "teeth", "maw", ...) on its `natural_attack` pool | `combat/combat_helpers.go:373-400`; `species/*.yaml` |
| `bite.yaml` is otherwise authored plural ("clack", "find", "CRITICALLY MAUL") | grep |
| `internal/narration/testdata/stores/combat_messages.golden` snapshots the combat-message pools; `-update` regenerates | `internal/narration/snapshot_test.go:111`, `:188` |
| Description modifiers are applied in the mutator loop AFTER the wrap; the wrap is `util.SplitString` at `:110` (minimap) and `:139` | `internal/rooms/roomdetails.go:110`, `:139`, `:185-215` |
| `colorpatterns.ApplyColorPattern` tags every rune, so wrapping coloured text can break inside a word | `internal/colorpatterns/colorpatterns.go:54-104` |
| `Room.ActiveMutators` yields nothing for a room with no zone config | `internal/rooms/rooms.go:3074-3098` |
| Crit banner: `` `<ansi fg="crit-text">***</ansi> ` + line + ` <ansi fg="crit-text">***</ansi>` `` on four lines | `internal/combat/combat_helpers.go:1801-1808` |
| `WrapAnsi` breaks only at `' '` and `'\n'`; U+00A0 counts as one visible column | `internal/messaging/wrap.go:143-160` |
| Prompt `{target}` below full sight prints `<ansi fg="mobname">an unseen foe</ansi>`; the gate is a BOOL callback `SetCanSeeInRoomCheck` registered in `main.go:321-329` over `messaging.CanSeeClearly`, so the prompt cannot tell shapes from none | `internal/users/userrecord.prompt.go:36-64`, `:372-381`, `:538-540` |
| GMCP `Char.Enemies` sets `e.Name = `an unseen foe`` when `!messaging.CanSeeClearly` | `modules/gmcp/gmcp.Char.go:474`, `:506` |
| `messaging.UnseenNoun(d)` is "a figure" at `SightShapes`, else "something"; `UnseenFigure` wraps it in `combat-anon` | `internal/messaging/hidenames.go:30-41` |
| `rooms.visualDecision` = CanSeeClearly then CanSeeShapes then none (the sleep-gated decision every visual sender uses); unexported | `internal/rooms/rooms.go:438-446` |
| `users` may import `messaging` (`messaging` does not depend on `users`) | `go list -deps` |
| Admin echoes: user lines `admin.item.go:128`, `admin.mob.go:205`, `admin.spawn.go:51`, `:77`; room lines `:131`, `:208`, `:54`, `:80` | read |
| The four echo events are registry rows of the narration guard, keyed by file and literal | `messaging_surface_guard_test.go:1303`, `:1305`, `:1311`, `:1312` |
| `online` headers: Name, Title, Online, Role; the admin view prepends UserId and appends Zone, RoomId | `internal/usercommands/online.go:19-29`, `:71-91` |


## Spec facts that were wrong or imprecise

- **H1** "`GetCharacterName(true)` is ... `:72-103`, `:195`": `:195` adds the adjectives; the pet name is set at `formattedname.go:207`. No effect on the fix.
- **H2** "`attack <name>`, the 11 melee specials, `give` and `shoot` resolve through `FindByNameSeenBy`": `melee_target.go:172` and `give.go:73` call `ResolveTargetActor`, which calls `FindByNameSeenBy` at `target_resolution.go:90`. `give.go:387` is `giveTargetResolves`, the argument splitter; it tells the giver nothing, so it needs no gate.
- **H2** lists `follow`: there is no player `follow` command (not in the `usercommands.go` registry, not in `keywords.yaml`). Following a person is `shadow`, which is gated.
- **H2** omits `target`, attack's sibling: `attack` hands every target switch to `Target` (`attack.go:149-166`), and `target <name>` in pitch dark printed "You turn your attention to Bobrick!" in the dry run. Gated.
- **H2** shoot: `ExecuteFire` already refuses below full sight (`combat_fire.go:280`), but only after resolving the name, so the pre-fire guards ("X is in your party!") and `NoTarget` versus `TooDarkToAim` answered differently for a creature that is there. The fix judges sight before any name resolves.
- **H2** `party invite` and `rep` resolve ONLY room occupants (`party.go:182`, `report.go:60`), never world-wide, so gating them is safe.
- **H3** "Four departure room lines ... `usercommands/go.go:265`, `:308-321`": `:265` is the `rooms.MoveToRoom` call; the two player departure lines are at `:308` (with a pet) and `:316` (without). The count of four is right.
- **H3** "readers who did not perceive the quitter are excluded": at `HandleLeave` time no such reader exists, because logout's `ForceVisible` listener (`onPlayerDespawnForAwareness`, default priority) has already run before `HandleLeave` (`events.Last`). The exclusion list must be captured before `ForceVisible` (D-H3a).
- **H4** "the rebuild finds no source for record 9 and removes it: `conditions.go:356-358`, `:376-381`": the sourcing check is `:357-359`, the removal loop `:378-384`.
- **H5** "A mob fold that completes with every target gone ... `spell_resolution.go:646-655`": the target loops are `:633-654`, the return `:656`. And worse than the spec says: on master the released mob is left `IsCasting()` with its stale fold indefinitely, because nothing advances or clears a fold outside combat. Task 11's IdleMobs test fails on master on that assertion first.
- **H7** "`claws.yaml` puts a singular verb after `{itemname}` on 8 lines": 13 lines do (173, 230, 247, 305, 329, 353, 380, 385, 396, 402, 403, 424, 425; the spec missed the crit lines whose capitalised verb follows a colour tag). `bite.yaml:173` ("Your fangs grazes") is the same defect.
- **H7** "736 end without a full stop": 715 single-line `- 'emote ...'` items end without one at `bd1220964` (51 with one); the remedy is unchanged.
- **H7** `gmcp/gmcp.Char.go:506` is `modules/gmcp/gmcp.Char.go:506`.
- **H7** multi-word biomes: dogmud ships four (Deep Water, Dense Forest, City Backstreet, City Thoroughfare) and the default world one (Deep Water); plus maplegend "Training Yard" and the keywords override "Throne Room".
- **H6** "Night Vision ... blind under light 7": right; the playtest also needs `LightExitsAbove`, which ships at 55 (Go default 65): a scout sees out of its own room only at that light or more.

## Design points where the code forced a different choice than the spec (for the owner)

- **D-H1.** `GetCharacterName(true)` keeps the identity tag's suffix colour (`username-dead`) and drops the adjective span, the quest star and the pet. Intended by the spec and wide: every spell, craft, quest, sleep, arrest and disenchant line that rendered "Aliceia (Lit)" now renders "Aliceia". `look` and the rosters use `GetPlayerName` / `GetMobName` and keep their adjectives.
- **D-H2-1 Hint text.** Two lines: `You can only make out shapes here.` then `Try <verb> shape or <verb> 2.shape.` (commands in `command` colour). When that second line would pass 80 columns it becomes `Use shape or 2.shape in place of a name.` Cast's old one-line hint was 87 columns or more; it now reads the same two lines.
- **D-H2-2 Id forms pass at shapes.** `@N` and `#N` resolve when the player makes out shapes (still `Perceives`-filtered): a shape is rewritten to that form, and party auto-assist types `attack #N`. Behaviour change for `cast`: `cast X #N` at shapes used to get the hint and now resolves. With no sight an id form is refused like a name.
- **D-H2-3 Blind party members stop auto-assisting.** All three auto-assist sites (`attack.go:213-222`, `:335-341`, `hooks/NewRound_DoCombat_helpers.go:1363-1377`) skip a member who sees nothing; otherwise that member would read "You don't see them here." every round.
- **D-H2-4 Wildcards and empty names.** `attack *` is refused with no sight and allowed at shapes. Bare `attack` and bare melee specials are unchanged; only a typed name is judged.
- **D-H2-5 Names hidden after a shape, beyond give and the quest line.** Making `shape` resolvable for every gated verb means every follow-up line would name the creature. So show, party invite and rep hide each name at the reader's sight; attack, target and `StageMeleeTarget` hide the name in their refusal lines; consider, steal, plant and shadow hand their action a new `actions.UserActorAtSight`, which anonymizes the player's own lines at shapes. Without this, `consider 2.shape` read "You consider Skeleton...".
- **D-H2-6 Sender word.** The recipient of a party invite or a `rep` whisper reads "Someone" (`HideSpeakerNames`, as speech does); the recipient of a give or show reads "Something" (`HideNames`), as attack's lines do.
- **D-H2-7 steal and plant nouns.** A refused name is still tried as a container (and for steal a household bauble or fixture); the sight refusal is told only when nothing else matches. So in the dark `steal lantern` now reads "You don't see them here." instead of "Steal from whom?".
- **D-H2-8 give pet.** `give <item> pet` (the giver's own pet) is exempt.
- **D-H2-9 fire.** Below full sight of the room the shot is aimed into (own room, or `scanReach` through an exit), every `fire` with an argument is refused with the existing lines before any other check, including the no-weapon check. No shape aiming for shots: an aimed shot already needs full sight.
- **D-H2-10 Own name.** Typing your own name at shapes or no sight gets the hint or refusal (`rep me`, `rep self` and self-casts still work).
- **D-H2-11 Test layout.** The spec put the attack, kick, give and steal tests in `actions/sight_aim_test.go`; those commands live in `usercommands`, so they are in `usercommands/sight_aim_commands_test.go`, and `actions/sight_aim_test.go` pins the rule itself.
- **D-H2-12 Left ungated (out of scope, noted).** `look <creature>` (`actions/look.go:87`), `sell` to a named merchant (`sell.go:133`), hiring a merc (`actions/buy.go:325`), and every mob command (D8). What an NPC says back after `talk` or `ask` aimed at a shape is not traced here; the playtest checks it.
- **D-H3a.** The quit line's exclusions are captured by the awareness logout listener, which already runs first, and handed to `HandleLeave` through the user's temp data (`despawnUnseenByKey`, helper `playersNotPerceiving`). No event field or listener order changes. Behaviour change: before, a hidden quitter's line named them to everyone; now a reader who could not see them reads nothing, and a see-hidden reader still reads it.
- **D-H3b.** `SendTextVisualWithAudio` and the new `SendTextVisualWithAudioToSnapshot` share one per-reader body (`deliverVisualElseAudio`); the old sender's behaviour is unchanged. `mobcommands.sendMovementMessage` stays with one caller (the forced-move entry line).
- **D-H4.** `reconcileSneakHide` acts on any Visible holder of a live record 9, player or mob. In live play that arises only from a load, because every exit from Hidden cancels the 9.
- **D-H5a.** `fizzleMobFold(mob, room, cs)` is the one ending: the fold step's TargetGone branch calls it too. In `IdleMobs` the fizzle line comes before the "mumbles about losing their quarry" emote.
- **D-H5b.** "No target left" in `resolveMobSpell` counts targets still in the room (the caster counts for a self-cast help spell). An area spell that finds nobody at completion now fizzles; today it says nothing. `fold-anchor`, `fold-recall` and `drain_area` return before the count and are unchanged.
- **D-H6.** No code change: the three scout tests pass on master. A mutation probe (bypassing the `scanReach` gate and the `Perceives` filter in `actions/scan.go`) makes the two refusal tests fail, so they can fail.
- **D-H7a.** One slug function, `mapper.LegendSlug`, shared by the minimap, `map` and (as the template function `mapslug`) the legend template. Multi-word biomes gain aliases with their single-word sibling's colour (deep water = water, dense forest = forest, city backstreet and city thoroughfare = city). "Training Yard" and "Throne Room" get a parseable tag but no alias, like every other maplegend.
- **D-H7b.** `CategoryMobEmote` gains the end-punctuation stage for every sender of that category (about 30: mob emotes plus defuse, drink, forage and other system lines that ride it), not only mob emotes. The stage never touches a line ending in `. ! ? , ) ] " ' *`. Player-typed emotes (`CategoryEmote`) stay exempt. No YAML is edited.
- **D-H7c.** The claws and bite lines are reworded so the actor does the verb ("You sink your claws into X!"), not made plural, so a species with a singular natural weapon ("maw") reads right too.
- **D-H7d.** Description modifiers now join the description before the wrap and before noun highlighting, so a room noun inside a modifier is highlighted too. Each part is wrapped as plain text; nouns are highlighted on the plain line, then the line is coloured (the colour pattern leaves the noun tags alone), then the minimap joins it. Review fix: the first cut coloured before highlighting, so a recolour-only modifier (wildfire) hid every noun and a coloured modifier's nouns were never highlighted.
- **D-H7e.** Only the crit frame gets the U+00A0 (written `string(rune(0x00A0))` in Go). The fumble frames and the progression and achievement banners keep their plain space.
- **D-H7f.** The prompt's sight callback was a yes/no (`SetCanSeeInRoomCheck`), so it could not tell shapes from no sight. It becomes `users.SetPromptSightCheck(func(*characters.Character) messaging.SightDecision)`, fed by a new `messaging.ReaderSight`, which `rooms.visualDecision` now returns. The prompt's unseen target takes the combat lines' `combat-anon` colour instead of `mobname`.
- **D-H7g.** Only the admin view of `online` drops Title; players keep it. Echoes: "You wave your hands and X appears." / "Admin waves their hands and X appears."

## Guards and package tests the dry run tripped, and how each task handles them

- **Task 3 (H2b):** `usercommands/attack_sight_names_test.go` `TestAttack_PitchDark_DoesNotNameTheMobToTheAttacker` expected the fight to start in pitch dark; it now expects `AimNotHereLine` and no fight. `TestAttack_PitchDark_CompanionRefusalDoesNotNameTheMob` reached the companion refusal by name in the dark; renamed `TestAttack_Shapes_CompanionRefusalDoesNotNameTheMob`, it reaches it at shapes with `2.shape`.
- **Task 4 (H2c):** `usercommands/get_fixture_test.go:111` `TestStealAFixtureInTheDarkSaysNothingOfIt` expected "Steal from whom?"; it now expects `actions.AimNotHereLine` (D-H2-7).
- **Task 8 (H3a):** no guard fails, but two root guard lists are widened so they keep recognising the departure sends once they move to the new method: `messaging_surface_guard_test.go` (the observer method switch) and `bauble_finder_view_guard_test.go` (`beyondReaderCalls`). Without the additions those guards would silently stop seeing four room broadcasts.
- **Task 15 (H7c):** `internal/narration` `TestSnapshotStores/combat_messages` golden mismatch on the 14 reworded lines; regenerated with `-update` after checking the diff holds only those lines.
- **Task 18 (H7f):** `TestCharEnemies_BlindViewerGetsNoIdentityOrHp` (gmcp) and `TestPromptTargetVisibilityGating` (users) assert the old "an unseen foe" text; updated in the task.
- **Task 19 (H7g):** root `TestNarrationSitesMatchViewpointAudit` (`messaging_surface_guard_test.go`) finds 4 new events and 4 stale rows; the four registry keys (`:1303`, `:1305`, `:1311`, `:1312` at base; `:1306`, `:1308`, `:1314`, `:1315` after Task 8) are re-keyed to the new literals, verdicts and reasons unchanged, and `gofmt -w` realigns the map.
- **Checked and untouched:** `condition_apply_path_guard_test.go` keys `Condition_ApplyConditions.go|107,109,111`; Task 1 edits only after line 152, so no key shifts, and no other keyed file is edited. `lookup_viewer_guard_test.go` keys by `path|function` with counts; `AimBySight` performs no lookup. The viewpoint audit keys on literals; every new `HideNames` wraps its `Sprintf` in place, so the literals stay inline. `move_wrapper_guard_test.go`, `copy_no_dash_test.go`, `send_trio_only_guard_test.go` and `sight_gates_wrapper_guard_test.go` pass unchanged.

## Every gated command (H2) (base `bd1220964` line of the resolve it guards)

| Command | Gate before | Verb in the hint |
|---|---|---|
| `cast` (targeted) | `actions/cast_admission.go:59` (now `aimAtSight`) | `cast <spellId>` |
| `attack <name>` | `usercommands/attack.go:103` | `attack` |
| `target <name>` | `usercommands/target.go:62` | `target` |
| `bash`, `drain`, `gore`, `grapple`, `kick`, `maul`, `pounce`, `rake`, `taunt`, `throttle`, `trip` | `actions/melee_target.go:172` | `opts.Verb` |
| `give <item> <who>` | `usercommands/give.go:73` | `give <item>` |
| `show <item> <who>` | `usercommands/show.go:43` | `show <item>` |
| `consider <who>` | `usercommands/consider.go:28` | `consider` |
| `steal <who>` | `usercommands/skill.skullduggery.steal.go:84` | `steal` |
| `plant <item> <who>` | `usercommands/skill.skullduggery.plant.go:94` | `plant <item> on` |
| `shadow <who>` | `usercommands/skill.skullduggery.shadow.go:54` | `shadow` |
| `talk <npc>` | `usercommands/talk.go:43` | `talk` |
| `ask <npc> <topic>` | `usercommands/ask.go:95` | `ask` |
| `party invite <who>` | `usercommands/party.go:182` | `party invite` |
| `rep <who>` | `usercommands/report.go:60` | `rep` |
| `fire` / `shoot` | `usercommands/shoot.go:53` (`actions.ShotSight`, no hint) | none |

Player-facing texts: `You can't see anything to aim at.` (no sight, a shape or no name); `You don't see them here.` (no sight, a name; a shape number past the last figure); `You can only make out shapes here.` + newline + `Try <ansi fg="command">VERB shape</ansi> or <ansi fg="command">VERB 2.shape</ansi>.`, or `Use <ansi fg="command">shape</ansi> or <ansi fg="command">2.shape</ansi> in place of a name.` when the verb line would pass 80 columns. All are 80 columns or less, contain no dash, and are pinned by `TestAimShapesHint_FitsEightyColumns`.


## File map, order and parallel lanes

| Task | Fix | Files |
|---|---|---|
| 1 | H1 | `internal/characters/formattedname.go`, `internal/characters/narration_name_test.go` (new), `internal/hooks/Condition_ApplyConditions.go`, `internal/hooks/NewRound_MobRoundTick.go`, `internal/hooks/NewTurn_PruneConditions.go`, `internal/hooks/condition_narration_name_test.go` (new), `internal/characters/context.md`, `internal/messaging/context.md` |
| 2 | H2a | `internal/actions/sight_aim.go` (new), `internal/actions/cast_admission.go`, `internal/actions/sight_aim_test.go` (new), `internal/actions/cast_sight_followups_test.go` (comment), `internal/actions/context.md` |
| 3 | H2b | `internal/usercommands/attack.go`, `target.go`, `internal/actions/melee_target.go`, `internal/hooks/NewRound_DoCombat_helpers.go`, `internal/usercommands/sight_aim_commands_test.go` (new), `internal/usercommands/attack_sight_names_test.go`, `internal/hooks/party_autoassist_sight_test.go` (new) |
| 4 | H2c | `internal/actions/sight_aim.go`, `internal/actions/context.md`, `internal/usercommands/give.go`, `show.go`, `consider.go`, `skill.skullduggery.steal.go`, `skill.skullduggery.plant.go`, `skill.skullduggery.shadow.go`, `sight_aim_commands_test.go`, `get_fixture_test.go` |
| 5 | H2d | `internal/usercommands/talk.go`, `ask.go`, `party.go`, `report.go`, `sight_aim_commands_test.go` |
| 6 | H2e | `internal/actions/combat_fire.go`, `internal/actions/context.md`, `internal/usercommands/shoot.go`, `sight_aim_commands_test.go` |
| 7 | H2f | `internal/behaviortree/actions_quest.go`, `internal/behaviortree/actions_quest_return_item_sight_test.go` (new), `internal/behaviortree/context.md`, `internal/usercommands/context.md` |
| 8 | H3a | `internal/rooms/rooms.go`, `internal/rooms/visual_snapshot_test.go`, `internal/hooks/PlayerDespawn_HandleLeave.go`, `internal/hooks/Logout_AwarenessCleanup.go`, `internal/hooks/despawn_line_snapshot_test.go` (new), `messaging_surface_guard_test.go`, `bauble_finder_view_guard_test.go`, `internal/rooms/context.md`, `internal/hooks/context.md` |
| 9 | H3b | `internal/usercommands/go.go`, `go_departure_light_test.go` (new), `internal/actions/relocate_mob.go`, `relocate_mob_departure_light_test.go` (new), `internal/mobcommands/go.go`, `go_departure_light_test.go` (new), `context.md` in `usercommands`, `actions`, `mobcommands` |
| 10 | H4 | `internal/characters/shroud_hide.go`, `validate.go`, `shroud_hide_test.go`, `internal/characters/context.md` |
| 11 | H5 | `internal/hooks/NewRound_DoCombat_helpers.go`, `NewRound_IdleMobs.go`, `spell_resolution.go`, `spell_channel_sight_test.go`, `internal/hooks/context.md` |
| 12 | H6 | `internal/behaviortree/actions_scout_sight_test.go` (new) |
| 13 | H7a | `internal/mapper/maptag.go` (new), `maptag_test.go` (new), `internal/mapper/context.md`, `internal/usercommands/look.go`, `skill.map.go`, `internal/templates/templatesfunctions.go`, `internal/templates/mapslug_func_test.go` (new), `_datafiles/world/{dogmud,default}/templates/maps/map.template`, `_datafiles/world/{dogmud,default}/ansi-aliases.yaml` |
| 14 | H7b | `internal/messaging/normalize.go`, `internal/messaging/mob_emote_punct_test.go` (new), `internal/messaging/context.md` |
| 15 | H7c | `_datafiles/world/dogmud/combat-messages/claws.yaml`, `bite.yaml`, `internal/items/natural_weapon_agreement_test.go` (new), `internal/narration/testdata/stores/combat_messages.golden` |
| 16 | H7d | `internal/rooms/roomdetails.go`, `internal/rooms/roomdetails_modifier_wrap_test.go` (new), `internal/rooms/context.md` |
| 17 | H7e | `internal/combat/combat_helpers.go`, `internal/combat/crit_banner_wrap_test.go` (new), `internal/messaging/wrap_nbsp_test.go` (new), `internal/combat/context.md` |
| 18 | H7f | `internal/messaging/predicates.go`, `internal/rooms/rooms.go`, `internal/users/userrecord.prompt.go`, `internal/users/users_test.go`, `main.go`, `modules/gmcp/gmcp.Char.go`, `modules/gmcp/gmcp.CharEnemies_test.go`, `modules/gmcp/gmcp.CharEnemies_shapes_test.go` (new), `internal/messaging/context.md`, `modules/gmcp/context.md`, `docs/architecture/wiring-seams.md` |
| 19 | H7g | `internal/usercommands/admin.item.go`, `admin.mob.go`, `admin.spawn.go`, `online.go`, `admin_spawn_echo_test.go` (new), `online_columns_test.go` (new), `messaging_surface_guard_test.go` |
| 20 | | Whole-tree verification (no files) |
| 21 | | Playtest (controller) |

**Order.** Run the tasks in number order unless a lane below says otherwise. Shared files force these sequences: Tasks 2 to 7 (shared `actions/sight_aim.go`, `actions/context.md`, `usercommands/sight_aim_commands_test.go`); 3 before 11 (`hooks/NewRound_DoCombat_helpers.go`); 7 before 9 (`usercommands/context.md`); 2, 4, 6 before 9 (`actions/context.md`); 1 before 10 (`characters/context.md`); 8 before 11 (`hooks/context.md`); 8 before 9 (9 calls the method 8 adds); 8 before 16 and 18 (`rooms/rooms.go`, `rooms/context.md`); 1 before 14 before 18 (`messaging/context.md`); 8 before 19 (`messaging_surface_guard_test.go`); 7 before 12 (same package; run its tests after 7 lands).

**Parallel lanes.** The controller may run a task in parallel with another only when their file sets AND Go packages are disjoint, because a half-applied task in a shared package breaks the other task's compile. Safe pairs at their turn: Task 15 (YAML, `items`, `narration`) with any task; Task 17 (`combat`, plus a new `messaging` test file) with any task outside `messaging` and `combat`; Task 13 (`mapper`, `templates`, `usercommands` look and map) with any task outside `usercommands` and `templates`; Task 10 (`characters`) with any task outside `characters` once Task 1 has landed. Everything else runs in sequence.

New Go files are package files, not repo-root guards, so `docs/README.md` gets no row for them.

## Rules for every task

- Edit files with the Edit tool. Never a Python read-modify-write. Never `git add -A` or `git add .`; stage named paths only.
- `gofmt -l` every touched Go file must print nothing; run `gofmt -w <file>` if it does.
- Line numbers in a task are the file's numbers at `bd1220964`; an earlier task may have shifted them. Find each edit by its quoted anchor text, not by number alone.
- Run targeted tests: `go test ./internal/<pkg>/ -run '<Name>' -count=1`. The guard tests at the repo root run with `go test . -run '<Name>' -count=1`; run the whole root set (`go test . -count=1`, about a minute) at the end of every task.
- `grep -c` exits 1 on zero matches. Run an "expect zero" check on its own line, never inside an `&&` chain.
- No em dash or en dash in any Go string literal, comment you write, YAML text, or commit message. Two anchors below quote a line that holds a pre-existing dash; those steps tell you to find the line by its dash-free part and say the dash goes.
- Player-facing text: 80 columns or less, no raw numbers for damage or durations, ESL-clear (`dogmud-player-copy`).
- Do not touch `_datafiles/config.yaml` (skip-worktree). No task here needs it; balance numbers quoted below come from `git show HEAD:_datafiles/config.yaml`, never a Go default.
- Commit messages end with a blank line, then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Do not write "closes #N", "fixes #N" or "resolves #N" anywhere, commit or PR.
- A test fails first: Step 2 of each task is the failing run. If a new test passes before the fix, stop and find out why. Two exceptions by design: Task 12 (H6), whose tests pin behaviour that is already right and which proves they can fail with a temporary mutation; and Task 17's `messaging/wrap_nbsp_test.go`, a pin of the wrapper property the crit fix relies on (the task's own crit test fails first).

---

### Task 1: H1, Narration names carry no tags (#453)

**Files:**
- Modify: `internal/characters/formattedname.go:161-169`
- Modify: `internal/hooks/Condition_ApplyConditions.go:152-158`, `internal/hooks/NewRound_MobRoundTick.go:289-290`, `internal/hooks/NewTurn_PruneConditions.go:104-106`
- Test: `internal/characters/narration_name_test.go` (new), `internal/hooks/condition_narration_name_test.go` (new)
- Docs: `internal/characters/context.md:832-836`, `internal/messaging/context.md:416-421`

- [ ] **Step 1: Write the failing characters test.** Create `internal/characters/narration_name_test.go` (it reuses `perceivesChar` and `perceivesHide` from `perceives_test.go`, same package):

```go
package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/pets"
)

// #453: GetCharacterName(true) is the narration name. Every production caller
// feeds it into a line about what the holder did ("{actee} seems to shimmer
// and fade from view"), so it carries the identity tag alone. The adjective
// span, the quest star and " and <pet>" belong to look and the rosters,
// which render through GetPlayerName / GetMobName, not this.
func TestGetCharacterName_TaggedIsTheIdentityTagAlone(t *testing.T) {
	c := perceivesChar(t, "Sil Vantage")
	c.Health = 10
	c.Adjectives = []string{`lit`}
	c.Pet = pets.Pet{Type: `dog`, Name: `Rex`}
	perceivesHide(t, c)

	got := c.GetCharacterName(true)
	if got != `<ansi fg="username">Sil Vantage</ansi>` {
		t.Fatalf("narration name must be the identity tag alone, got %q", got)
	}

	// look keeps every adjective and the pet.
	full := c.GetPlayerName(0).String()
	for _, want := range []string{`hidden`, ` and `} {
		if !strings.Contains(full, want) {
			t.Fatalf("GetPlayerName lost %q: %q", want, full)
		}
	}
}

// The suffix is part of the tag (the colour), not the adjective list, so a
// dead holder keeps it.
func TestGetCharacterName_TaggedKeepsTheDeadSuffix(t *testing.T) {
	c := perceivesChar(t, "Grix")
	c.Health = 0
	if got := c.GetCharacterName(true); got != `<ansi fg="username-dead">Grix</ansi>` {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Write the failing hooks test.** Create `internal/hooks/condition_narration_name_test.go`:

```go
package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #453: a condition line names its holder by the identity tag alone. The
// playtest read "Sil Vantage (hidden) seems to shimmer and fade from view."
// and "Sil Vantage (☀️Lit) sits down and begins to meditate.": the adjective
// span look prints had leaked into narration. The holder here carries a lit
// lantern and a pet, so every one of those decorations is in play.

// narrationDecorations are the pieces look adds to a name that a narrated
// line must never carry: the adjective span and the " and <pet>" tail.
var narrationDecorations = []string{`fg="black-bold">(`, `fg="petname"`, ` and `}

func assertNoNameDecorations(t *testing.T, line string) {
	t.Helper()
	for _, bad := range narrationDecorations {
		assert.NotContains(t, line, bad, "narration carried a look decoration: %q", line)
	}
}

func decorateHolder(t *testing.T) *users.UserRecord {
	t.Helper()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
	require.True(t, holder.Character.EmitsLight(), "precondition: the holder is lit")
	holder.Character.Pet = pets.Pet{Type: `dog`, Name: `Rex`}
	return holder
}

func TestConditionNarration_PlayerHolderNameCarriesNoDecorations(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	holder := decorateHolder(t)
	defer func() { holder.Character.Pet = pets.Pet{} }()

	t.Run("start", func(t *testing.T) {
		events.DrainQueuedMessagesForTest(2)
		ApplyConditions(events.Condition{UserId: 1, ConditionId: glowConditionId})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "glows.")
		require.NotEmpty(t, line)
		assert.True(t, strings.HasPrefix(plainText(line), "Aliceia glows."), "got %q", plainText(line))
		assertNoNameDecorations(t, line)
	})

	t.Run("trigger", func(t *testing.T) {
		require.True(t, holder.Character.Conditions.AddCondition(shiverConditionId, false))
		events.DrainQueuedMessagesForTest(2)
		UserRoundTick(events.NewRound{RoundNumber: 1})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "shivers.")
		require.NotEmpty(t, line)
		assertNoNameDecorations(t, line)
	})

	t.Run("end", func(t *testing.T) {
		require.True(t, holder.Character.Conditions.AddCondition(fadeConditionId, false))
		expire(t, holder.Character.Conditions.List, fadeConditionId)
		events.DrainQueuedMessagesForTest(2)
		PruneConditions(events.NewTurn{TurnNumber: 1})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "fades.")
		require.NotEmpty(t, line)
		assertNoNameDecorations(t, line)
	})
}

func TestConditionNarration_MobHolderNameCarriesNoAdjectives(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Conditions.AddCondition(lanternConditionId, false))
	require.True(t, mob.Character.EmitsLight(), "precondition: the mob is lit")

	t.Run("start", func(t *testing.T) {
		events.DrainQueuedMessagesForTest(2)
		ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: glowConditionId})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "glows.")
		require.NotEmpty(t, line)
		assert.Contains(t, line, `fg="mobname`)
		assertNoNameDecorations(t, line)
	})

	t.Run("trigger", func(t *testing.T) {
		require.True(t, mob.Character.Conditions.AddCondition(shiverConditionId, false))
		events.DrainQueuedMessagesForTest(2)
		MobRoundTick(events.NewRound{RoundNumber: 1})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "shivers.")
		require.NotEmpty(t, line)
		assertNoNameDecorations(t, line)
	})

	t.Run("end", func(t *testing.T) {
		require.True(t, mob.Character.Conditions.AddCondition(fadeConditionId, false))
		expire(t, mob.Character.Conditions.List, fadeConditionId)
		events.DrainQueuedMessagesForTest(2)
		PruneConditions(events.NewTurn{TurnNumber: 1})
		line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "fades.")
		require.NotEmpty(t, line)
		assertNoNameDecorations(t, line)
	})
}
```

- [ ] **Step 3: Run both and see them fail.**

Run: `go test ./internal/characters/ -run TestGetCharacterName_Tagged -count=1`
Expected: FAIL, both tests: `got "<ansi fg=\"username\">Sil Vantage</ansi> <ansi fg=\"black-bold\">(hidden|lit)</ansi> and <ansi fg=\"petname\">Rex</ansi>"` and `got "<ansi fg=\"username-dead\">Grix</ansi> <ansi fg=\"black-bold\">(dead)</ansi>"`.

Run: `go test ./internal/hooks/ -run TestConditionNarration_ -count=1`
Expected: FAIL, all six subtests (start, trigger, end for each holder); the player start line reads `got "Aliceia (lit) and Rex glows."`.

- [ ] **Step 4: Implement `GetCharacterName`.** In `internal/characters/formattedname.go`, replace the doc comment and body of `GetCharacterName` (anchor: `// GetCharacterName returns the character's name as a plain string (ansi=false)`) with:

```go
// GetCharacterName returns the character's name as a plain string (ansi=false)
// or as an ANSI-tagged display string (ansi=true). Works for both player and
// mob characters; uses the username color tag for ANSI display.
//
// The tagged form is the NARRATION name: the identity tag alone, keeping its
// suffix colour (dead) but no adjective span, quest star or " and <pet>"
// (#453). Every production caller feeds it into a line about what the holder
// did, and "Sil Vantage (hidden) seems to shimmer" told the room a state tag.
// look and the rosters render through GetPlayerName / GetMobName instead and
// keep every decoration.
func (c *Character) GetCharacterName(ansi bool) string {
	if !ansi {
		return c.Name
	}
	f := c.getFormattedName(0, `username`)
	f.Adjectives = nil
	f.QuestAlert = false
	f.PetName = ``
	return f.String()
}
```

- [ ] **Step 5: Strip the span at the three mob-holder sites.** All three files already import `messaging`.

In `internal/hooks/Condition_ApplyConditions.go`, replace:

```go
				charName = m.Character.GetCharacterName(true)
				if r := rooms.LoadRoom(m.Character.RoomId); r != nil {
					charName = mobDisplayName(m, r, 0)
				}
```

with:

```go
				// StripNameAdjectives: mobDisplayName is look's form and
				// carries the adjective span, a state tag, not prose (#453).
				charName = m.Character.GetCharacterName(true)
				if r := rooms.LoadRoom(m.Character.RoomId); r != nil {
					charName = messaging.StripNameAdjectives(mobDisplayName(m, r, 0))
				}
```

In `internal/hooks/NewRound_MobRoundTick.go`, replace:

```go
					roles := trigSpec.Narrate(conditions.PhaseTrigger,
						mobDisplayName(mob, room, 0),
```

with:

```go
					roles := trigSpec.Narrate(conditions.PhaseTrigger,
						messaging.StripNameAdjectives(mobDisplayName(mob, room, 0)), // #453
```

In `internal/hooks/NewTurn_PruneConditions.go`, replace:

```go
						holderName = mobDisplayName(mob, r, 0)
```

with:

```go
						holderName = messaging.StripNameAdjectives(mobDisplayName(mob, r, 0)) // #453
```

- [ ] **Step 6: Run the new tests and see them pass.**

Run: `go test ./internal/characters/ -run TestGetCharacterName -count=1` then `go test ./internal/hooks/ -run TestConditionNarration_ -count=1`
Expected: `ok` for both.

- [ ] **Step 7: Run the affected packages and the root guards.** The name change reaches every narration caller, so run the whole tree once:

Run: `go test ./... -count=1`
Expected: every package `ok` (the dry run found no other test asserting an adjective in a `GetCharacterName(true)` line; `characters/formattedname_test.go:136` only asserts no `-aggro`). `condition_apply_path_guard_test.go` keys `Condition_ApplyConditions.go|107,109,111`; this task edits only after line 152, so no key moves.

- [ ] **Step 8: Update `context.md`.** In `internal/characters/context.md`, after the line `  because both sides of the comparison were 0.` (in "Character Presentation"), add:

```markdown
  `GetCharacterName(true)` is the narration name: the identity tag alone
  (suffix colour kept), with no adjective span, quest star or " and <pet>"
  (#453). Every production caller feeds a narrated line, where "(hidden)" or
  "(☀️Lit)" is a state tag, not prose. `GetPlayerName` and `GetMobName` keep
  every decoration for `look` and the rosters.
```

In `internal/messaging/context.md`, replace:

```markdown
  `combat.RenderChannelDefenceMessages` applies it to both identities; the
  melee and counter paths build their names without adjectives instead
  (`combat.meleeIdentityTag`).
```

with:

```markdown
  `combat.RenderChannelDefenceMessages` applies it to both identities; the
  melee and counter paths build their names without adjectives instead
  (`combat.meleeIdentityTag`). The condition start, trigger and end lines
  for a mob holder apply it to `mobDisplayName` (hooks, #453); a player
  holder's `GetCharacterName(true)` carries no span to begin with.
```

- [ ] **Step 9: Format, vet, commit.**

Run: `gofmt -l internal/characters internal/hooks` (expect no output), `go vet ./internal/characters ./internal/hooks` (expect no output).

```bash
git add internal/characters/formattedname.go internal/characters/narration_name_test.go internal/hooks/Condition_ApplyConditions.go internal/hooks/NewRound_MobRoundTick.go internal/hooks/NewTurn_PruneConditions.go internal/hooks/condition_narration_name_test.go internal/characters/context.md internal/messaging/context.md
git commit -m "fix(narration): condition lines name the holder without look's tags (#453)

GetCharacterName(true) is the narration name and now renders the identity
tag alone: no adjective span, quest star or pet. The three mob-holder
condition lines strip the span from mobDisplayName.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: H2a, the shared sight rule, and cast on it (#454)

**Files:**
- Create: `internal/actions/sight_aim.go`
- Modify: `internal/actions/cast_admission.go` (whole file rewritten; it shrinks from 147 lines to 75)
- Modify: `internal/actions/cast_sight_followups_test.go:57-58` (a comment naming the renamed helpers)
- Test: `internal/actions/sight_aim_test.go` (new); `internal/actions/cast_sight_test.go` and `cast_sight_followups_test.go` must still pass unchanged
- Docs: `internal/actions/context.md:658-663`

- [ ] **Step 1: Write the failing test.** Create `internal/actions/sight_aim_test.go` (it reuses `castSightScene` and `giveCasterInfrared` from `cast_sight_test.go`):

```go
package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// #454: a typed name resolves only at full sight. These pin AimBySight, the
// rule every command that names a creature shares with cast. The scene is
// castSightScene's: Caster 7811, Witness 7812 and Other 7813, in "city" (lit)
// or "cave" (pitch dark; infrared makes it shapes only).

func TestAimBySight_ClearSightAdmitsTheNameAsTyped(t *testing.T) {
	a, room := castSightScene(t, "city")
	name, refusal := AimBySight(a.GetCharacter(), 7811, room, "witness", "kick")
	assert.Equal(t, "", refusal)
	assert.Equal(t, "witness", name)
}

func TestAimBySight_ShapesHintsATypedNameAndNamesTheVerb(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	_, refusal := AimBySight(a.GetCharacter(), 7811, room, "witness", "kick")
	assert.Contains(t, refusal, "You can only make out shapes here.")
	assert.Contains(t, refusal, `<ansi fg="command">kick shape</ansi>`)
	assert.Contains(t, refusal, `<ansi fg="command">kick 2.shape</ansi>`)
	assert.NotContains(t, strings.ToLower(refusal), "witness")
}

func TestAimBySight_ShapesResolvesAShapeToItsFigure(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	name, refusal := AimBySight(a.GetCharacter(), 7811, room, "2.shape", "kick")
	assert.Equal(t, "", refusal)
	assert.Equal(t, "@7813", name, "figures are the other players in room order")

	name, refusal = AimBySight(a.GetCharacter(), 7811, room, "4.shape", "kick")
	assert.Equal(t, AimNotHereLine, refusal, "there are only two figures")
	assert.Equal(t, "4.shape", name)
}

// An id form is what a shape becomes, and what party auto-assist types.
func TestAimBySight_ShapesAdmitsAnIdForm(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	for _, id := range []string{"@7812", "#100"} {
		name, refusal := AimBySight(a.GetCharacter(), 7811, room, id, "attack")
		assert.Equal(t, "", refusal, "%q", id)
		assert.Equal(t, id, name)
	}
}

func TestAimBySight_NoSightResolvesNothing(t *testing.T) {
	a, room := castSightScene(t, "cave")
	for _, tc := range []struct{ name, want string }{
		{"witness", AimNotHereLine},
		{"@7812", AimNotHereLine},
		{"shape", AimNothingLine},
		{"2.shape", AimNothingLine},
		{"all.shape", AimNothingLine},
		{"", AimNothingLine},
	} {
		_, refusal := AimBySight(a.GetCharacter(), 7811, room, tc.name, "kick")
		assert.Equal(t, tc.want, refusal, "%q", tc.name)
	}
}

// The 80-column rule: every line of the hint fits, and a verb too long for
// the second line drops out of it.
func TestAimShapesHint_FitsEightyColumns(t *testing.T) {
	for _, verb := range []string{"kick", "give iron longsword", "cast empathic-shroud", strings.Repeat("x", 40), ""} {
		hint := AimShapesHint(verb)
		for _, line := range strings.Split(hint, "\n") {
			plain := aimTestTag.ReplaceAllString(line, "")
			assert.LessOrEqual(t, len([]rune(plain)), 80, "verb %q: %q", verb, plain)
		}
	}
	assert.Contains(t, AimShapesHint(strings.Repeat("x", 40)), "in place of a name")
}

var aimTestTag = regexp.MustCompile(`<[^>]*>`)
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/actions/ -run 'AimBySight|AimShapesHint' -count=1`
Expected: FAIL to build, `undefined: AimBySight`, `undefined: AimNotHereLine`, `undefined: AimShapesHint`.

- [ ] **Step 3: Create `internal/actions/sight_aim.go`.**

```go
package actions

import (
	"fmt"
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// shapeWord is what a player who makes out shapes aims at: `shape`, `2.shape`,
// `shape#2`. No dogmud mob, item, noun or alias uses the word.
const shapeWord = "shape"

// The refusals a player's sight gives a typed name (#454). Neither echoes the
// name, so a creature that is there and a name that matches nothing read the
// same.
const (
	// AimNothingLine answers a player who sees nothing and named a shape, or
	// named nothing at all.
	AimNothingLine = `You can't see anything to aim at.`
	// AimNotHereLine answers a name a player cannot see to pick out.
	AimNotHereLine = `You don't see them here.`
	// aimShapesLine opens the hint a player who makes out shapes only reads
	// when they typed a name.
	aimShapesLine = `You can only make out shapes here.`
	// aimHintWidth is the widest the hint's second line may be, in visible
	// columns (the 80-column rule, dogmud-player-copy).
	aimHintWidth = 80
)

// AimBySight judges a creature name a player typed against what the player
// makes out of room (#454, lifted from the cast's own rule, follow-up slice A):
//
//	clear sight:  every name, as typed
//	shapes only:  `shape`, `N.shape`, `shape#N`, or an id form (`@N`, `#N`);
//	              a typed name gets a hint naming verb
//	no sight:     nothing
//
// It returns the name to resolve, with a shape rewritten to "@<userId>" or
// "#<mobInstanceId>" (forms FindByName already resolves), and refusal: the
// line to tell the player, or "" when the name is admitted. verb is what the
// player types before the name, e.g. "kick" or "give torch"; the hint repeats
// it. An empty name is admitted at shapes (the caller decides what no name
// means) and refused with no sight.
//
// Player actors only: a mob acts on shapes and needs no hint (D8). An id form
// is admitted at shapes because it is what a shape becomes, and what the
// game's own party auto-assist types (`attack #N`).
func AimBySight(viewer *characters.Character, selfUserId int, room *rooms.Room, name, verb string) (string, string) {
	if room == nil {
		return name, ``
	}
	return aimAtSight(viewer, selfUserId, room, messaging.ParticipantSight(viewer, room), name, verb)
}

// aimAtSight is AimBySight at a sight the caller already judged.
func aimAtSight(viewer *characters.Character, selfUserId int, room *rooms.Room, sight messaging.SightDecision, name, verb string) (string, string) {
	shape := aimShapeIndex(name)
	switch sight {
	case messaging.SightNone:
		if name == `` || aimNamesAShape(name) {
			return name, AimNothingLine
		}
		return name, AimNotHereLine
	case messaging.SightShapes:
		if name == `` || aimIdForm(name) {
			return name, ``
		}
		if shape == 0 {
			return name, AimShapesHint(verb)
		}
	}

	if shape > 0 {
		figures := aimFigures(viewer, selfUserId, room)
		if shape > len(figures) {
			return name, AimNotHereLine
		}
		return figures[shape-1], ``
	}
	return name, ``
}

// AimShapesHint is the hint a player who makes out shapes only reads when
// they typed a name: two lines, the second naming verb with a shape after it.
// A verb too long for the second line to fit 80 columns drops out of it.
func AimShapesHint(verb string) string {
	try := fmt.Sprintf(`Try <ansi fg="command">%s shape</ansi> or <ansi fg="command">%s 2.shape</ansi>.`, verb, verb)
	if verb == `` || len(`Try  shape or  2.shape.`)+2*len([]rune(verb)) > aimHintWidth {
		try = `Use <ansi fg="command">shape</ansi> or <ansi fg="command">2.shape</ansi> in place of a name.`
	}
	return aimShapesLine + "\n" + try
}

// aimNamesAShape reports whether the player typed the shape word at all,
// including `all.shape`, which names no single figure. aimShapeIndex returns 0
// for that, and without this the refusal would call it a typed name.
func aimNamesAShape(name string) bool {
	if name == `` {
		return false
	}
	word, _ := util.GetMatchNumber(name)
	return word == shapeWord
}

// aimShapeIndex is N for `shape`, `N.shape` or `shape#N`, and 0 otherwise.
func aimShapeIndex(name string) int {
	if name == `` {
		return 0
	}
	word, n := util.GetMatchNumber(name)
	if word != shapeWord || n < 1 {
		return 0
	}
	return n
}

// aimIdForm reports whether name is "@<userId>" or "#<mobInstanceId>", the
// form a shape is rewritten to. FindByNameSeenBy still applies the viewer's
// Perceives to it, so it reaches exactly the figures a shape reaches.
func aimIdForm(name string) bool {
	if len(name) < 2 || (name[0] != '@' && name[0] != '#') {
		return false
	}
	n, err := strconv.Atoi(name[1:])
	return err == nil && n > 0
}

// aimFigures lists what the viewer makes out as shapes: the other players
// they perceive, in room order, then the mobs they perceive, in room order.
// Each is a name FindByName resolves: "@<userId>" or "#<mobInstanceId>".
func aimFigures(viewer *characters.Character, selfUserId int, room *rooms.Room) []string {
	figures := []string{}
	for _, uid := range room.GetPlayers() {
		if uid == selfUserId {
			continue
		}
		if u := users.GetByUserId(uid); u != nil && viewer.Perceives(u.Character) {
			figures = append(figures, fmt.Sprintf(`@%d`, uid))
		}
	}
	for _, mid := range room.GetMobs() {
		if m := mobs.GetInstance(mid); m != nil && viewer.Perceives(&m.Character) {
			figures = append(figures, fmt.Sprintf(`#%d`, mid))
		}
	}
	return figures
}
```

- [ ] **Step 4: Put cast on it.** Replace the whole of `internal/actions/cast_admission.go` with (the shape helpers moved to `sight_aim.go`; `castsAtSelf` stays):

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// castAim is what a player caster's sight lets a targeted cast aim at.
type castAim struct {
	// targetName is the name to resolve. A shape is rewritten to "@<userId>"
	// or "#<mobInstanceId>", forms FindByName already resolves.
	targetName string
	// ownFoeOnly is set when the caster makes out shapes only: a no-name
	// harmful cast may aim at the caster's own foe, not the party leader's.
	ownFoeOnly bool
}

// admitCastAim applies follow-up slice A's sight rules to a player's targeted
// cast (single-target harm or help, or multi-target harm). It returns refused=true after
// telling the caster why; nothing has been spent at that point.
//
//	clear sight:  names, your foe then your party leader's foe, shapes
//	shapes only:  your own foe, shapes; a typed name gets a hint
//	no sight:     every targeted cast refused
//
// The sight rule itself is AimBySight's (sight_aim.go, #454), shared with
// every other command that names a creature; this wrapper adds what is the
// cast's own: which spells it covers, the self-cast exemption, and what no
// name means at shapes.
//
// A self-cast (a help spell with no name, or the caster's own name) needs no
// sight. Area, help-multi and neutral casts are not affected.
func admitCastAim(actor Actor, spellInfo *spells.SpellData, targetName string) (castAim, bool) {
	aim := castAim{targetName: targetName}
	switch {
	case spellInfo.Targeting == combatvocab.TargetSingle:
	case spellInfo.IsHarm() && spellInfo.Targeting == combatvocab.TargetMulti:
	default:
		return aim, false
	}
	room := actor.GetRoom()
	if room == nil {
		return aim, false
	}
	if !spellInfo.IsHarm() && spellInfo.Targeting == combatvocab.TargetSingle && castsAtSelf(actor, targetName) {
		return aim, false
	}

	char := actor.GetCharacter()
	sight := messaging.ParticipantSight(char, room)
	name, refusal := aimAtSight(char, actor.GetUserId(), room, sight, targetName, `cast `+spellInfo.SpellId)
	if refusal != `` {
		actor.SendText(messaging.CategorySystem, refusal)
		return aim, true
	}
	aim.targetName = name
	aim.ownFoeOnly = sight == messaging.SightShapes && targetName == ``
	return aim, false
}

// castsAtSelf reports whether a help spell with this target name is aimed at
// the caster. Ruling 4: a self-cast needs no sight, so `cast heal caster` and
// `cast heal cast` must work in the dark, not just the exact spelling of the
// name. It matches the way the rest of the game matches names. Only the
// caster.s own name is a candidate, so a close match here means the typed noun
// matched nothing but themselves.
func castsAtSelf(actor Actor, targetName string) bool {
	if targetName == `` {
		return true
	}
	match, closeMatch := util.FindMatchIn(targetName, actor.GetName())
	return match != `` || closeMatch != ``
}
```

Then fix the two stale names in the comment at `internal/actions/cast_sight_followups_test.go:57-58`: replace `castShapeIndex reads 0 for it. Without` with `aimShapeIndex reads 0 for it. Without`, and `// castNamesAShape the refusal called it a typed name.` with `// aimNamesAShape the refusal called it a typed name.` (Add `internal/actions/cast_sight_followups_test.go` to this task's commit.)

- [ ] **Step 5: Run the new and the cast tests.**

Run: `go test ./internal/actions/ -run 'AimBySight|AimShapesHint|CastSight|CastViewer' -count=1`
Expected: `ok`. (All 16 existing `TestCastSight_*` / `TestCastViewer_*` pass unchanged, including the mob-caster ones: `admitCastAim` still gates mobs as slice F requires.)

- [ ] **Step 6: Docs.** In `internal/actions/context.md`, replace the bullet that begins `- **\`InitiateCast\`** runs \`admitCastAim\`` (6 lines, ends `` `RefusalExplained`, and spend nothing. ``) with:

```markdown
- **`AimBySight(viewer, selfUserId, room, name, verb) (string, string)`**
  (`sight_aim.go`, #454): the one rule for a creature name a player types.
  Clear sight admits the name as typed; shapes only admits `shape` /
  `N.shape` / `shape#N` (rewritten to the figure's `@<userId>` or
  `#<mobInstanceId>`; figures are perceived players then mobs, in room
  order), an id form (`@N`, `#N`, what party auto-assist types) and an empty
  name, and answers any other name with `AimShapesHint(verb)`; no sight
  admits nothing (`AimNothingLine` for a shape or no name, `AimNotHereLine`
  for a name). It returns the name to resolve and the refusal to tell, ""
  when admitted. Every player command that names a creature runs it (the
  list is in `internal/usercommands/context.md`). Mobs act on shapes.
- **`InitiateCast`** runs `admitCastAim` (`cast_admission.go`) for a player's
  single-target casts of either kind and harmful multi casts. The sight rule
  is `AimBySight`'s (verb `cast <spell>`); the cast adds the self-cast
  exemption and lets no name at shapes aim at the caster's own foe only.
  Refusals are narrated, set `RefusalExplained`, and spend nothing.
```

- [ ] **Step 7: Package and guards.**

Run: `gofmt -l internal/actions` (expect no output), `go vet ./internal/actions`, `go test ./internal/actions/ -count=1` (about a minute), `go test . -count=1`.
Expected: all `ok`. No root guard trips.

- [ ] **Step 8: Commit.**

```bash
git add internal/actions/sight_aim.go internal/actions/sight_aim_test.go internal/actions/cast_admission.go internal/actions/cast_sight_followups_test.go internal/actions/context.md
git commit -m "feat(actions): one sight rule for a typed creature name, cast on it (#454)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: H2b, attack, target and the melee specials judge a typed name by sight (#454)

**Files:**
- Modify: `internal/usercommands/attack.go:100-104`, `:151-163`, `:213-216`, `:326`, `:338`
- Modify: `internal/usercommands/target.go:61`, `:107`, `:110`, `:132`, `:166`, `:170`
- Modify: `internal/actions/melee_target.go:172`, `:194-195`, `:238-239`
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:1368-1370` and a new helper above `surpriseCandidate`
- Test: `internal/usercommands/sight_aim_commands_test.go` (new), `internal/usercommands/attack_sight_names_test.go`, `internal/hooks/party_autoassist_sight_test.go` (new)

- [ ] **Step 1: Write the failing tests.** Create `internal/usercommands/sight_aim_commands_test.go`:

```go
package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #454: a typed name resolves only at full sight. The scene is
// seedAllRegistries' room 1: Aliceia (1, the actor), Bobrick (2) and the
// Skeleton (mob 100). Aliceia's figures at shapes are Bobrick (shape 1) and
// the Skeleton (shape 2).

const aimInfraredConditionId = 9454

type aimBand int

const (
	aimFull aimBand = iota
	aimShapes
	aimDark
)

// aimScene seeds the registries and sets room 1 to the band asked for:
// lit (lamp 60), or pitch dark with Aliceia given infrared (shapes) or not.
func aimScene(t *testing.T, band aimBand) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		aimInfraredConditionId: {ConditionId: aimInfraredConditionId, Name: "Test Infrared",
			RoundInterval: 1, TriggerCount: 1, Flags: []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	user := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	if band != aimFull {
		room.Lamp = nil
		room.Biome = "cave"
		require.Equal(t, 0, room.LightLevel(), "the dark bands need a pitch-dark room")
	}
	if band == aimShapes {
		require.True(t, user.Character.Conditions.AddCondition(aimInfraredConditionId, true))
	}
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return user, room
}

var aimTagPattern = regexp.MustCompile(`<[^>]*>`)

// aimTold is everything userId was sent since the last drain, tags stripped.
func aimTold(userId int) string {
	return aimTagPattern.ReplaceAllString(strings.Join(events.DrainQueuedMessagesForTest(userId), ""), "")
}

func TestAttackSight_ClearSightResolvesAName(t *testing.T) {
	user, room := aimScene(t, aimFull)
	_, err := Attack("skeleton", user, room, events.EventFlag(0))
	require.NoError(t, err)
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId)
}

func TestAttackSight_ShapesHintsANameAndTakesAShape(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	_, _ = Attack("skeleton", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You can only make out shapes here.")
	assert.Contains(t, told, "attack shape")
	assert.Equal(t, 0, user.Character.CurrentCombatTarget().MobInstanceId, "a typed name engaged at shapes")

	_, _ = Attack("2.shape", user, room, events.EventFlag(0))
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId, "shape 2 is the Skeleton")
	assert.NotContains(t, aimTold(1), "Skeleton", "the attacker who aimed at a shape read its name")
}

func TestAttackSight_NoSightResolvesNothing(t *testing.T) {
	for _, rest := range []string{"skeleton", "shape", "*"} {
		user, room := aimScene(t, aimDark)
		_, _ = Attack(rest, user, room, events.EventFlag(0))
		assert.Equal(t, 0, user.Character.CurrentCombatTarget().MobInstanceId, "%q engaged with no sight", rest)
		told := aimTold(1)
		if rest == "skeleton" {
			assert.Contains(t, told, actions.AimNotHereLine)
		} else {
			assert.Contains(t, told, actions.AimNothingLine, "%q", rest)
		}
	}
}

// Every melee special stages its target through StageMeleeTarget.
func TestMeleeStageSight_KickAtEachBand(t *testing.T) {
	user, room := aimScene(t, aimFull)
	_, handled := actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.False(t, handled, "clear sight stages a named target")

	user, room = aimScene(t, aimShapes)
	_, handled = actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.True(t, handled)
	assert.Contains(t, aimTold(1), "kick 2.shape")
	_, handled = actions.StageMeleeTarget(user, room, "2.shape", actions.MeleeTargetOpts{Verb: "kick"})
	assert.False(t, handled, "a shape stages at shapes")

	user, room = aimScene(t, aimDark)
	_, handled = actions.StageMeleeTarget(user, room, "skeleton", actions.MeleeTargetOpts{Verb: "kick"})
	assert.True(t, handled, "no sight stages nothing")
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

// target is attack's sibling: attack hands a target switch to it.
func TestTargetSight_NoSightResolvesNothing(t *testing.T) {
	user, room := aimScene(t, aimDark)
	require.True(t, targeting.Commit(user.Character, state.ActorRef{MobInstanceId: 100}, targeting.ReasonAttack))
	_, _ = Target("bobrick", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId, "the target did not change")
}
```

Create `internal/hooks/party_autoassist_sight_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// #454: party auto-assist types `attack #<id>` for every member in the room.
// A member who sees nothing cannot pick the mob out, so assisting would only
// tell them "You don't see them here." every round. They are skipped; a member
// who can see still assists (the control).
func TestPartyAutoAssist_SkipsAMemberWhoSeesNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	p := parties.New(1)
	t.Cleanup(p.Disband)
	p.InvitePlayer(2)
	p.AcceptInvite(2)
	mob := mobs.GetInstance(100)
	defender := users.GetByUserId(1)

	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	events.DrainQueuedUserInputsForTest(2)
	handlePartyAutoAttack(mob, defender)
	assert.Equal(t, []string{"attack #100"}, events.DrainQueuedUserInputsForTest(2), "a member who can see assists")

	rooms.LoadRoom(1).Lamp = nil
	darken(t, 1)
	handlePartyAutoAttack(mob, defender)
	assert.Empty(t, events.DrainQueuedUserInputsForTest(2), "a member who sees nothing was sent to attack")
}
```

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./internal/usercommands/ -run 'AttackSight|MeleeStageSight|TargetSight' -count=1`
Expected: FAIL. Among the errors: `"You prepare to enter into mortal combat with a figure.\n" does not contain "You can only make out shapes here."`; `"skeleton" engaged with no sight`; `"You turn your attention to Bobrick!\n" does not contain "You don't see them here."` (a blind player told the name).

Run: `go test ./internal/hooks/ -run PartyAutoAssist -count=1`
Expected: FAIL, `Should be empty, but was [attack #100]`.

- [ ] **Step 3: Gate `attack`.** In `internal/usercommands/attack.go`, replace

```go
	} else {
		// Wildcard and named-target resolution delegated to shared helper.
		t := actions.FindAttackTarget(rest, room, user.UserId, 0, user.Character)
```

with

```go
	} else {
		// #454: a typed name resolves only at full sight. A shape becomes the
		// "@id" or "#id" it stands for. A wildcard picks among what the
		// attacker perceives, so it needs only some sight.
		if rest[0] != '*' {
			name, refusal := actions.AimBySight(user.Character, user.UserId, room, rest, `attack`)
			if refusal != `` {
				user.SendText(messaging.CategorySystem, refusal)
				return true, nil
			}
			rest = name
		} else if messaging.ParticipantSight(user.Character, room) == messaging.SightNone {
			user.SendText(messaging.CategorySystem, actions.AimNothingLine)
			return true, nil
		}
		// Wildcard and named-target resolution delegated to shared helper.
		t := actions.FindAttackTarget(rest, room, user.UserId, 0, user.Character)
```

Replace the target-switch name builder

```go
			targetName := rest
			if targetName == "" || targetName[0] == '*' {
				// For empty or random targets, find the actual name
				if attackMobInstanceId > 0 {
					if m := mobs.GetInstance(attackMobInstanceId); m != nil {
						targetName = m.Character.Name
					}
				} else if attackPlayerId > 0 {
					if p := users.GetByUserId(attackPlayerId); p != nil {
						targetName = p.Character.Name
					}
				}
			}
```

with

```go
			targetName := rest
			if targetName == "" || targetName[0] == '*' {
				// For empty or random targets, name the one resolved by its
				// id form: Target judges a typed name by sight (#454), and an
				// attacker who makes out shapes never typed one.
				if attackMobInstanceId > 0 {
					targetName = fmt.Sprintf(`#%d`, attackMobInstanceId)
				} else if attackPlayerId > 0 {
					targetName = fmt.Sprintf(`@%d`, attackPlayerId)
				}
			}
```

In the mob branch's party loop replace

```go
						if partyUser.Character.RoomId == user.Character.RoomId &&
							partyUser.Character.GetSetting("autoattack") != "off" &&
							!partyUser.Character.IsInCombat() {
```

with

```go
						// A member who sees nothing cannot pick the foe out
						// (#454), and would be told so on every assist.
						if partyUser.Character.RoomId == user.Character.RoomId &&
							partyUser.Character.GetSetting("autoattack") != "off" &&
							!partyUser.Character.IsInCombat() &&
							messaging.ParticipantSight(partyUser.Character, room) != messaging.SightNone {
```

In the PvP branch replace

```go
					user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> is in your party!`, p.Character.Name))
					return true, nil
```

with

```go
					user.SendText(messaging.CategorySystem, messaging.HideNames(
						fmt.Sprintf(`<ansi fg="username">%s</ansi> is in your party!`, p.Character.Name),
						[]string{p.Character.Name}, messaging.ParticipantSight(user.Character, room)))
					return true, nil
```

and replace

```go
							if partyUser.Character.RoomId == user.Character.RoomId {
								partyUser.Command(fmt.Sprintf(`attack @%d`, attackPlayerId)) // # denotes a specific mob instanceId
```

with

```go
							if partyUser.Character.RoomId == user.Character.RoomId &&
								messaging.ParticipantSight(partyUser.Character, room) != messaging.SightNone {
								partyUser.Command(fmt.Sprintf(`attack @%d`, attackPlayerId)) // # denotes a specific mob instanceId
```

- [ ] **Step 4: Gate `target`.** In `internal/usercommands/target.go`, replace

```go
	// Find the new target
	target, err := actions.ResolveTargetActor(room, rest, actions.ResolveTargetOptions{
```

with

```go
	// #454: a typed name resolves only at full sight.
	name, refusal := actions.AimBySight(user.Character, user.UserId, room, rest, `target`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
	rest = name
	sight := messaging.ParticipantSight(user.Character, room)

	// Find the new target
	target, err := actions.ResolveTargetActor(room, rest, actions.ResolveTargetOptions{
```

Then wrap the five lines that name the new target, keeping each literal inline (the viewpoint audit keys on it). Replace each `user.SendText(messaging.CategorySystem, fmt.Sprintf(LITERAL, NAME))` below with the `HideNames` form:

```go
			user.SendText(messaging.CategorySystem, messaging.HideNames(
				fmt.Sprintf("<ansi fg=\"mobname\">%s</ansi> is someone's companion!", m.Character.Name),
				[]string{m.Character.Name}, sight))
```

```go
			user.SendText(messaging.CategorySystem, messaging.HideNames(
				fmt.Sprintf("You can't attack <ansi fg=\"mobname\">%s</ansi>.", m.Character.Name),
				[]string{m.Character.Name}, sight))
```

```go
				user.SendText(messaging.CategorySystem, messaging.HideNames(
					fmt.Sprintf("<ansi fg=\"username\">%s</ansi> is in your party!", p.Character.Name),
					[]string{p.Character.Name}, sight))
```

```go
				user.SendText(messaging.CategorySystem, messaging.HideNames(
					fmt.Sprintf("You turn your attention to <ansi fg=\"mobname\">%s</ansi>!", m.Character.Name),
					[]string{m.Character.Name}, sight))
```

```go
				user.SendText(messaging.CategorySystem, messaging.HideNames(
					fmt.Sprintf("You turn your attention to <ansi fg=\"username\">%s</ansi>!", p.Character.Name),
					[]string{p.Character.Name}, sight))
```

- [ ] **Step 5: Gate the melee specials.** In `internal/actions/melee_target.go`, replace

```go
	target, err := ResolveTargetActor(room, rest, ResolveTargetOptions{ExcludeUserId: user.UserId, Viewer: user.Character})
```

with

```go
	// #454: a typed name resolves only at full sight. A shape becomes the
	// "@id" or "#id" it stands for.
	name, refusal := AimBySight(user.Character, user.UserId, room, rest, opts.Verb)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return nil, true
	}
	rest = name
	sight := messaging.ParticipantSight(user.Character, room)

	target, err := ResolveTargetActor(room, rest, ResolveTargetOptions{ExcludeUserId: user.UserId, Viewer: user.Character})
```

Replace

```go
			user.SendText(messaging.CategorySystem,
				fmt.Sprintf(`You can't attack <ansi fg="mobname">%s</ansi>.`, mob.Character.Name))
			return nil, true
```

with

```go
			user.SendText(messaging.CategorySystem, messaging.HideNames(
				fmt.Sprintf(`You can't attack <ansi fg="mobname">%s</ansi>.`, mob.Character.Name),
				[]string{mob.Character.Name}, sight))
			return nil, true
```

and

```go
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is in your party!`, p.Character.Name))
```

with

```go
		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is in your party!`, p.Character.Name),
			[]string{p.Character.Name}, sight))
```

- [ ] **Step 6: The round driver's auto-assist.** In `internal/hooks/NewRound_DoCombat_helpers.go` (`handlePartyAutoAttack`), replace

```go
				if memberUser.Character.RoomId == defUser.Character.RoomId &&
					memberUser.Character.GetSetting("autoattack") != "off" &&
					!memberUser.Character.IsInCombat() {
```

with

```go
				// A member who sees nothing cannot pick the mob out (#454),
				// and would be told so every round it swings.
				if memberUser.Character.RoomId == defUser.Character.RoomId &&
					memberUser.Character.GetSetting("autoattack") != "off" &&
					!memberUser.Character.IsInCombat() &&
					memberSeesSomething(memberUser) {
```

and insert directly above the line `// surpriseCandidate builds the skullduggery candidate a landed surprise attack`:

```go
// memberSeesSomething reports whether a party member makes out at least
// shapes in their room, which `attack #<id>` needs since #454.
func memberSeesSomething(u *users.UserRecord) bool {
	room := rooms.LoadRoom(u.Character.RoomId)
	if room == nil {
		return false
	}
	return messaging.ParticipantSight(u.Character, room) != messaging.SightNone
}

```

(The file already imports `messaging`, `rooms` and `users`.)

- [ ] **Step 7: Update the two #214 tests the gate changes.** In `internal/usercommands/attack_sight_names_test.go`, add `"github.com/GoMudEngine/GoMud/internal/actions"` to the imports (before `characters`). In `TestAttack_PitchDark_DoesNotNameTheMobToTheAttacker` replace

```go
	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "mortal combat", "the engagement line must still be sent: %q", out)
```

with

```go
	// #454: with no sight a typed name resolves to nothing, so the attacker is
	// refused, and the refusal names no one either.
	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, actions.AimNotHereLine, "the refusal must be sent: %q", out)
	require.Equal(t, 0, user.Character.CurrentCombatTarget().MobInstanceId, "no sight engaged a named target")
```

Replace the head of the companion test

```go
func TestAttack_PitchDark_CompanionRefusalDoesNotNameTheMob(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	darkenTestRoom1(t)
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
```

with

```go
// At shapes the attacker aims at a shape (#454), and the companion refusal
// must not hand over the name the shape stands for.
func TestAttack_Shapes_CompanionRefusalDoesNotNameTheMob(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	user.Character.EndAggro()
```

and in that test replace `handled, err := Attack("skeleton", user, room, 0)` with `handled, err := Attack("2.shape", user, room, 0)` and the message `"an attacker who sees nothing must not learn the companion's name: %q"` with `"an attacker who makes out shapes only must not learn the companion's name: %q"`. The `Attack("skeleton", ...)` line occurs three times in the file; anchor the Edit on the block from that line down to the companion message so it is unique.

- [ ] **Step 8: Run the tests.**

Run: `go test ./internal/usercommands/ -run 'AttackSight|MeleeStageSight|TargetSight|TestAttack_' -count=1` and `go test ./internal/hooks/ -run PartyAutoAssist -count=1`
Expected: `ok` both.

- [ ] **Step 9: Packages and guards.**

Run: `gofmt -l internal/usercommands internal/actions internal/hooks` (no output), `go vet ./internal/usercommands ./internal/actions ./internal/hooks`, `go test ./internal/usercommands/ ./internal/actions/ ./internal/hooks/ -count=1`, `go test . -count=1`.
Expected: all `ok`. No root guard trips (dry run).

- [ ] **Step 10: Commit.**

```bash
git add internal/usercommands/attack.go internal/usercommands/target.go internal/actions/melee_target.go internal/hooks/NewRound_DoCombat_helpers.go internal/usercommands/sight_aim_commands_test.go internal/usercommands/attack_sight_names_test.go internal/hooks/party_autoassist_sight_test.go
git commit -m "fix(combat): attack, target and melee specials name only what the attacker sees (#454)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: H2c, give, show, consider, steal, plant and shadow (#454)

**Files:**
- Modify: `internal/actions/sight_aim.go` (append `UserActorAtSight`)
- Modify: `internal/usercommands/give.go:73`, `:90-95`, `:129-134`, `:182-184`, `:206-208`, `:233-235`
- Modify: `internal/usercommands/show.go:43`, `:61-68`, `:84-86`
- Modify: `internal/usercommands/consider.go:28-29`, `:38`
- Modify: `internal/usercommands/skill.skullduggery.steal.go:53`, `:83-93`, `:120`
- Modify: `internal/usercommands/skill.skullduggery.plant.go:46`, `:93-103`, `:115`
- Modify: `internal/usercommands/skill.skullduggery.shadow.go:53`, `:75`
- Test: `internal/usercommands/sight_aim_commands_test.go` (append), `internal/usercommands/get_fixture_test.go:111`
- Docs: `internal/actions/context.md`

- [ ] **Step 1: Write the failing tests.** Add `"github.com/GoMudEngine/GoMud/internal/items"` to the imports of `internal/usercommands/sight_aim_commands_test.go` (after `events`) and append:

```go
func TestGiveSight_ClearSightNamesBothSides(t *testing.T) {
	user, room := aimScene(t, aimFull)
	user.Character.Gold = 50
	_, _ = Give("10 gold bobrick", user, room, events.EventFlag(0))
	assert.Equal(t, 40, user.Character.Gold)
	assert.Contains(t, aimTold(1), "to Bobrick.")
	assert.Contains(t, aimTold(2), "Aliceia gives you")
}

func TestGiveSight_ShapesHintsANameAndHidesNamesAfterAShape(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	user.Character.Gold = 50
	_, _ = Give("10 gold bobrick", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "give 10 gold shape")
	assert.Equal(t, 50, user.Character.Gold, "a typed name was given to at shapes")

	_, _ = Give("10 gold 1.shape", user, room, events.EventFlag(0))
	assert.Equal(t, 40, user.Character.Gold, "shape 1 is Bobrick")
	told = aimTold(1)
	assert.Contains(t, told, "to a figure.")
	assert.NotContains(t, told, "Bobrick", "the giver aimed at a shape and read its name")
	// Bobrick has no infrared: he sees nothing, and reads "Something".
	recipient := aimTold(2)
	assert.Contains(t, recipient, "Something gives you")
	assert.NotContains(t, recipient, "Aliceia", "a recipient who sees nothing read the giver's name")
}

func TestGiveSight_NoSightGivesNothing(t *testing.T) {
	user, room := aimScene(t, aimDark)
	user.Character.Gold = 50
	_, _ = Give("10 gold bobrick", user, room, events.EventFlag(0))
	assert.Equal(t, 50, user.Character.Gold)
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

func TestShowSight_ShapesHidesTheNameAfterAShape(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	user.Character.StoreItem(items.New(10001))
	_, _ = Show("sword 2.shape", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "to a figure.")
	assert.NotContains(t, told, "Skeleton")
}

func TestStealSight_EachBand(t *testing.T) {
	user, room := aimScene(t, aimFull)
	opts := parseStealArgs([]string{"skeleton"}, room, user)
	require.NotNil(t, opts)
	assert.Equal(t, 100, opts.TargetMobInstanceId)

	user, room = aimScene(t, aimShapes)
	assert.Nil(t, parseStealArgs([]string{"skeleton"}, room, user))
	assert.Contains(t, aimTold(1), "steal 2.shape")
	opts = parseStealArgs([]string{"2.shape"}, room, user)
	require.NotNil(t, opts, "a shape is stolen from at shapes")
	assert.Equal(t, 100, opts.TargetMobInstanceId)

	user, room = aimScene(t, aimDark)
	assert.Nil(t, parseStealArgs([]string{"skeleton"}, room, user))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

func TestConsiderSight_ShapesConsidersAFigure(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	_, _ = Consider("2.shape", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You consider a figure")
	assert.NotContains(t, told, "Skeleton")
}
```

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./internal/usercommands/ -run 'GiveSight|ShowSight|StealSight|ConsiderSight' -count=1`
Expected: FAIL (the tests build; nothing they call is new). Among the errors: `"You give 10 gold to Bobrick.\n" does not contain "give 10 gold shape"`; `"Aliceia gives you 10 gold.\n" should not contain "Aliceia"`; `Expected nil, but got: &actions.StealOptions{TargetMobInstanceId:100, ...}`; `"You don't see them here.\n" does not contain "You consider a figure"`.

- [ ] **Step 3: The actor for a gated action.** Append to `internal/actions/sight_aim.go`:

```go

// UserActorAtSight is the actor a player command that names a creature hands
// to its action (#454). At clear sight, or with no sight (where nothing
// resolves), it is &UserActor{User: user, Room: room}. A player who makes out
// shapes only aimed at a shape and never learned the name, so every line the
// action tells them anonymizes the identity tags it carries ("a figure"), as
// a room line already does for them.
func UserActorAtSight(user *users.UserRecord, room *rooms.Room) Actor {
	actor := &UserActor{User: user, Room: room}
	if room == nil || messaging.ParticipantSight(user.Character, room) != messaging.SightShapes {
		return actor
	}
	return &shapesUserActor{Actor: actor}
}

// shapesUserActor is a UserActor whose own lines name no one (see
// UserActorAtSight).
type shapesUserActor struct {
	Actor
}

func (a *shapesUserActor) SendText(cat messaging.Category, msg string) {
	a.Actor.SendText(cat, messaging.Anonymize(msg))
}
```

- [ ] **Step 4: give.** In `internal/usercommands/give.go` replace

```go
	target, err := actions.ResolveTargetActor(room, giveWho, actions.ResolveTargetOptions{Viewer: user.Character})
	if err == nil {
		if target.IsPlayer() {

			targetUser := target.(*actions.UserActor).User
```

with

```go
	// #454: a typed name resolves only at full sight. The word "pet" is the
	// giver's own pet, always at hand.
	if giveWho != `pet` {
		name, refusal := actions.AimBySight(user.Character, user.UserId, room, giveWho, `give `+giveWhat)
		if refusal != `` {
			user.SendText(messaging.CategorySystem, refusal)
			return true, nil
		}
		giveWho = name
	}
	// Each side's own line hides the other's name at that reader's sight, as
	// attack.go's do: a giver who aimed at a shape never learned the name.
	giverSight := messaging.ParticipantSight(user.Character, room)

	target, err := actions.ResolveTargetActor(room, giveWho, actions.ResolveTargetOptions{Viewer: user.Character})
	if err == nil {
		if target.IsPlayer() {

			targetUser := target.(*actions.UserActor).User
			recipientSight := messaging.ParticipantSight(targetUser.Character, room)
```

Replace the player item pair

```go
				user.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You give the <ansi fg="item">%s</ansi> to <ansi fg="username">%s</ansi>.`, result.Item.DisplayName(), targetUser.Character.Name),
				)
				targetUser.SendText(messaging.CategorySystem,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> gives you their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName()),
				)
```

with

```go
				user.SendText(messaging.CategorySystem, messaging.HideNames(
					fmt.Sprintf(`You give the <ansi fg="item">%s</ansi> to <ansi fg="username">%s</ansi>.`, result.Item.DisplayName(), targetUser.Character.Name),
					[]string{targetUser.Character.Name}, giverSight),
				)
				targetUser.SendText(messaging.CategorySystem, messaging.HideNames(
					fmt.Sprintf(`<ansi fg="username">%s</ansi> gives you their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName()),
					[]string{user.Character.Name}, recipientSight),
				)
```

the player gold pair

```go
					user.SendText(messaging.CategorySystem,
						fmt.Sprintf(`You give <ansi fg="gold">%d gold</ansi> to <ansi fg="username">%s</ansi>.`, giveGoldAmount, targetUser.Character.Name),
					)
					targetUser.SendText(messaging.CategorySystem,
						fmt.Sprintf(`<ansi fg="username">%s</ansi> gives you <ansi fg="gold">%d gold</ansi>.`, user.Character.Name, giveGoldAmount),
					)
```

with

```go
					user.SendText(messaging.CategorySystem, messaging.HideNames(
						fmt.Sprintf(`You give <ansi fg="gold">%d gold</ansi> to <ansi fg="username">%s</ansi>.`, giveGoldAmount, targetUser.Character.Name),
						[]string{targetUser.Character.Name}, giverSight),
					)
					targetUser.SendText(messaging.CategorySystem, messaging.HideNames(
						fmt.Sprintf(`<ansi fg="username">%s</ansi> gives you <ansi fg="gold">%d gold</ansi>.`, user.Character.Name, giveGoldAmount),
						[]string{user.Character.Name}, recipientSight),
					)
```

the mob gold line

```go
				user.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You give <ansi fg="gold">%d gold</ansi> to <ansi fg="username">%s</ansi>.`, giveGoldAmount, m.Character.Name),
				)
```

with

```go
				user.SendText(messaging.CategorySystem, messaging.HideNames(
					fmt.Sprintf(`You give <ansi fg="gold">%d gold</ansi> to <ansi fg="username">%s</ansi>.`, giveGoldAmount, m.Character.Name),
					[]string{m.Character.Name}, giverSight),
				)
```

and BOTH mob item lines (the quest-consumed one, indented five tabs, and the normal one, indented four tabs; edit each with its own indentation)

```go
user.SendText(messaging.CategorySystem,
	fmt.Sprintf(`You give the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`, giveItem.DisplayName(), m.Character.Name),
)
```

with

```go
user.SendText(messaging.CategorySystem, messaging.HideNames(
	fmt.Sprintf(`You give the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`, giveItem.DisplayName(), m.Character.Name),
	[]string{m.Character.Name}, giverSight),
)
```

The room lines (`room.SendTextVisual` with tagged names) are already anonymized for a shapes reader by the pipeline and stay as they are.

- [ ] **Step 5: show.** In `internal/usercommands/show.go` replace

```go
	target, err := actions.ResolveTargetActor(room, targetName, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		user.SendText(messaging.CategorySystem, "Who???")
		return true, nil
	}
```

with

```go
	// #454: a typed name resolves only at full sight.
	name, refusal := actions.AimBySight(user.Character, user.UserId, room, targetName, `show `+objectName)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
	targetName = name
	showerSight := messaging.ParticipantSight(user.Character, room)

	target, err := actions.ResolveTargetActor(room, targetName, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		user.SendText(messaging.CategorySystem, "Who???")
		return true, nil
	}
```

replace

```go
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`You show the <ansi fg="item">%s</ansi> to <ansi fg="username">%s</ansi>.`, showItem.DisplayName(), targetUser.Character.Name),
		)

		// Tell the Showee
		targetUser.SendText(messaging.CategorySystem,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> shows you their <ansi fg="item">%s</ansi>.`, user.Character.Name, showItem.DisplayName()),
		)
```

with

```go
		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`You show the <ansi fg="item">%s</ansi> to <ansi fg="username">%s</ansi>.`, showItem.DisplayName(), targetUser.Character.Name),
			[]string{targetUser.Character.Name}, showerSight),
		)

		// Tell the Showee, at the showee's own sight.
		targetUser.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`<ansi fg="username">%s</ansi> shows you their <ansi fg="item">%s</ansi>.`, user.Character.Name, showItem.DisplayName()),
			[]string{user.Character.Name}, messaging.ParticipantSight(targetUser.Character, room)),
		)
```

and replace

```go
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`You show the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`, showItem.DisplayName(), targetMob.Character.Name),
		)
```

with

```go
		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`You show the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`, showItem.DisplayName(), targetMob.Character.Name),
			[]string{targetMob.Character.Name}, showerSight),
		)
```

- [ ] **Step 6: consider.** In `internal/usercommands/consider.go` replace

```go
	target, err := actions.ResolveTargetActor(room, strings.Join(args, " "),
		actions.ResolveTargetOptions{ExcludeUserId: user.UserId, Viewer: user.Character})
```

with

```go
	// #454: a typed name resolves only at full sight.
	name, refusal := actions.AimBySight(user.Character, user.UserId, room, strings.Join(args, " "), `consider`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
	target, err := actions.ResolveTargetActor(room, name,
		actions.ResolveTargetOptions{ExcludeUserId: user.UserId, Viewer: user.Character})
```

and replace `actor := &actions.UserActor{User: user, Room: room}` (line 38) with `actor := actions.UserActorAtSight(user, room)`.

- [ ] **Step 7: steal.** In `internal/usercommands/skill.skullduggery.steal.go` replace `actor := &actions.UserActor{User: user, Room: room}` at line 53 (the one before `actions.Steal(actor, *opts)`; leave the `TooDarkToGet` one at line 113 alone) with `actor := actions.UserActorAtSight(user, room)`. Replace

```go
	// Try mob/player resolution first.
	target, err := actions.ResolveTargetActor(room, targetNoun, actions.ResolveTargetOptions{Viewer: user.Character})
	if err == nil {
		if target.IsPlayer() {
			user.SendText(messaging.CategorySystem, "You can't steal from other players.")
			return nil
		}
		return &actions.StealOptions{
			TargetMobInstanceId: target.(*actions.MobActor).Mob.InstanceId,
		}
	}
```

with

```go
	// Try mob/player resolution first, if the thief's sight lets the noun name
	// a creature (#454). A refused noun may still be a container or a bauble,
	// which keep their own rules; the refusal is told only if it is neither.
	creatureNoun, aimRefusal := actions.AimBySight(user.Character, user.UserId, room, targetNoun, `steal`)
	if aimRefusal == `` {
		target, err := actions.ResolveTargetActor(room, creatureNoun, actions.ResolveTargetOptions{Viewer: user.Character})
		if err == nil {
			if target.IsPlayer() {
				user.SendText(messaging.CategorySystem, "You can't steal from other players.")
				return nil
			}
			return &actions.StealOptions{
				TargetMobInstanceId: target.(*actions.MobActor).Mob.InstanceId,
			}
		}
	}
```

and replace the function's last lines

```go
	user.SendText(messaging.CategorySystem, "Steal from whom?")
	return nil
}
```

with

```go
	if aimRefusal != `` {
		user.SendText(messaging.CategorySystem, aimRefusal)
		return nil
	}
	user.SendText(messaging.CategorySystem, "Steal from whom?")
	return nil
}
```

- [ ] **Step 8: plant.** In `internal/usercommands/skill.skullduggery.plant.go` replace `actor := &actions.UserActor{User: user, Room: room}` (line 46) with `actor := actions.UserActorAtSight(user, room)`. Replace

```go
	// Try mob/player resolution first.
	target, err := actions.ResolveTargetActor(room, targetNoun, actions.ResolveTargetOptions{Viewer: user.Character})
	if err == nil {
		if target.IsPlayer() {
			user.SendText(messaging.CategorySystem, "You can't plant items on other players.")
			return actions.PlantOptions{}, false
		}
		return actions.PlantOptions{
			ItemNoun:            itemNoun,
			TargetMobInstanceId: target.(*actions.MobActor).Mob.InstanceId,
		}, true
	}
```

with

```go
	// Try mob/player resolution first, if the planter's sight lets the noun
	// name a creature (#454). A refused noun may still be a container; the
	// refusal is told only if it is not.
	creatureNoun, aimRefusal := actions.AimBySight(user.Character, user.UserId, room, targetNoun, `plant `+itemNoun+` on`)
	if aimRefusal == `` {
		target, err := actions.ResolveTargetActor(room, creatureNoun, actions.ResolveTargetOptions{Viewer: user.Character})
		if err == nil {
			if target.IsPlayer() {
				user.SendText(messaging.CategorySystem, "You can't plant items on other players.")
				return actions.PlantOptions{}, false
			}
			return actions.PlantOptions{
				ItemNoun:            itemNoun,
				TargetMobInstanceId: target.(*actions.MobActor).Mob.InstanceId,
			}, true
		}
	}
```

and replace

```go
			ContainerNoun: containerName,
		}, true
	}

	user.SendText(messaging.CategorySystem, "Plant on whom?")
```

with

```go
			ContainerNoun: containerName,
		}, true
	}

	if aimRefusal != `` {
		user.SendText(messaging.CategorySystem, aimRefusal)
		return actions.PlantOptions{}, false
	}
	user.SendText(messaging.CategorySystem, "Plant on whom?")
```

- [ ] **Step 9: shadow.** In `internal/usercommands/skill.skullduggery.shadow.go` replace

```go
	// Resolve target in the current room, excluding the player themselves.
	target, err := actions.ResolveTargetActor(room, strings.ToLower(rest), actions.ResolveTargetOptions{
```

with

```go
	// #454: a typed name resolves only at full sight.
	name, refusal := actions.AimBySight(user.Character, user.UserId, room, strings.ToLower(rest), `shadow`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
	rest = name

	// Resolve target in the current room, excluding the player themselves.
	target, err := actions.ResolveTargetActor(room, strings.ToLower(rest), actions.ResolveTargetOptions{
```

and replace `actor := &actions.UserActor{User: user, Room: room}` (line 75) with `actor := actions.UserActorAtSight(user, room)`.

- [ ] **Step 10: The fixture test the steal gate changes.** In `internal/usercommands/get_fixture_test.go`, `TestStealAFixtureInTheDarkSaysNothingOfIt`, replace

```go
	assert.Contains(t, out, "Steal from whom?")
}
```

with

```go
	// #454: with no sight the noun names no creature either, so the thief
	// reads the sight refusal, which names nothing.
	assert.Contains(t, out, actions.AimNotHereLine)
}
```

(The file already imports `actions`.)

- [ ] **Step 11: Docs.** In `internal/actions/context.md`, directly after the `AimBySight` bullet added in Task H2a, insert:

```markdown
- **`UserActorAtSight(user, room) Actor`**: the actor a gated command hands
  its action. At shapes only it anonymizes every identity tag in the lines
  the action tells the player ("a figure"), since they aimed at a shape and
  never learned the name; otherwise it is a plain `*UserActor`.
```

- [ ] **Step 12: Run the tests, packages and guards.**

Run: `go test ./internal/usercommands/ -run 'GiveSight|ShowSight|StealSight|ConsiderSight|TestStealAFixture' -count=1`
Expected: `ok`.

Run: `gofmt -l internal/usercommands internal/actions` (no output), `go vet ./internal/usercommands ./internal/actions`, `go test ./internal/usercommands/ ./internal/actions/ -count=1`, `go test . -count=1`.
Expected: all `ok`. Without Step 10, `TestStealAFixtureInTheDarkSaysNothingOfIt` fails with `"You don't see them here." does not contain "Steal from whom?"` (the only package test this task trips).

- [ ] **Step 13: Commit.**

```bash
git add internal/actions/sight_aim.go internal/actions/context.md internal/usercommands/give.go internal/usercommands/show.go internal/usercommands/consider.go internal/usercommands/skill.skullduggery.steal.go internal/usercommands/skill.skullduggery.plant.go internal/usercommands/skill.skullduggery.shadow.go internal/usercommands/sight_aim_commands_test.go internal/usercommands/get_fixture_test.go
git commit -m "fix(sight): give, show, consider, steal, plant and shadow name only what is seen (#454)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: H2d, talk, ask, party invite and rep (#454, owner call 2)

**Files:**
- Modify: `internal/usercommands/talk.go:41-43`, `ask.go:92`, `party.go:182`, `:201-202`, `report.go:59-60`, `:76-81`
- Test: `internal/usercommands/sight_aim_commands_test.go` (append)

- [ ] **Step 1: Write the failing tests.** Add `"github.com/GoMudEngine/GoMud/internal/parties"` to the imports of `internal/usercommands/sight_aim_commands_test.go` (after `items`) and append:

```go
// Owner call 2: talk, ask, party invite and rep find their target among the
// room's occupants, so a typed name in the dark would confirm who is there.
func TestTalkAskSight_NoSightResolvesNothing(t *testing.T) {
	user, room := aimScene(t, aimDark)
	_, _ = Talk("skeleton", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	_, _ = Ask("skeleton quest", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
}

func TestTalkSight_ShapesHintsAName(t *testing.T) {
	user, room := aimScene(t, aimShapes)
	_, _ = Talk("skeleton", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), "talk 2.shape")
}

func TestPartyInviteSight_EachBand(t *testing.T) {
	user, room := aimScene(t, aimDark)
	t.Cleanup(func() {
		if p := parties.Get(1); p != nil {
			p.Disband()
		}
	})
	_, _ = Party("invite bobrick", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	assert.Empty(t, aimTold(2), "a player nobody could see was invited")

	user, room = aimScene(t, aimShapes)
	_, _ = Party("invite 1.shape", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You invited a figure to your party.")
	assert.NotContains(t, told, "Bobrick")
	invitee := aimTold(2)
	assert.Contains(t, invitee, "Someone invited you", "Bobrick sees nothing, so his inviter is someone")
}

func TestRepSight_EachBand(t *testing.T) {
	user, room := aimScene(t, aimDark)
	_, _ = Report("bobrick", user, room, events.EventFlag(0))
	assert.Contains(t, aimTold(1), actions.AimNotHereLine)
	assert.Empty(t, aimTold(2), "a whisper reached a player nobody could see")

	user, room = aimScene(t, aimShapes)
	_, _ = Report("1.shape", user, room, events.EventFlag(0))
	told := aimTold(1)
	assert.Contains(t, told, "You report to a figure:")
	assert.NotContains(t, told, "Bobrick")
	assert.Contains(t, aimTold(2), "Someone reports to you:")
}
```

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./internal/usercommands/ -run 'TalkAskSight|TalkSight|PartyInviteSight|RepSight' -count=1`
Expected: FAIL. Among the errors: `"You invited Bobrick to your party.\n" does not contain "You don't see them here."` and `Should be empty, but was Aliceia invited you to their party...` (a player in pitch dark invited, by name, someone they could not see, and he was told who); `"You report to Bobrick: ..." does not contain "You don't see them here."`.

- [ ] **Step 3: talk.** In `internal/usercommands/talk.go` replace

```go
	searchName := args[0]

	target, err := actions.ResolveTargetActor(room, searchName, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil || target.IsPlayer() {
```

with

```go
	// #454, owner call 2: a typed name resolves only at full sight, or
	// talking would confirm who is there in the dark.
	searchName, refusal := actions.AimBySight(user.Character, user.UserId, room, args[0], `talk`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}

	target, err := actions.ResolveTargetActor(room, searchName, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil || target.IsPlayer() {
```

- [ ] **Step 4: ask.** In `internal/usercommands/ask.go` replace the line `	searchName := args[0]` with

```go
	// #454, owner call 2: a typed name resolves only at full sight, or
	// asking would confirm who is there in the dark.
	searchName, refusal := actions.AimBySight(user.Character, user.UserId, room, args[0], `ask`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
```

- [ ] **Step 5: party invite.** In `internal/usercommands/party.go` (`cmdPartyInvite`) replace

```go
	target, err := actions.ResolveTargetActor(room, rest, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`%s not found.`, rest))
		return true, nil
	}
```

with

```go
	// #454, owner call 2: the invitee is found among the room's occupants, so
	// a typed name resolves only at full sight, or inviting would confirm who
	// is there in the dark.
	name, refusal := actions.AimBySight(user.Character, user.UserId, room, rest, `party invite`)
	if refusal != `` {
		user.SendText(messaging.CategorySystem, refusal)
		return true, nil
	}
	inviterSight := messaging.ParticipantSight(user.Character, room)

	target, err := actions.ResolveTargetActor(room, name, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`%s not found.`, rest))
		return true, nil
	}
```

and replace

```go
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You invited <ansi fg="username">%s</ansi> to your party.`, invitedUser.Character.Name))
		invitedUser.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="username">%s</ansi> invited you to their party. Type <ansi fg="command">party accept</ansi> or <ansi fg="command">party decline</ansi> to respond.`, user.Character.Name))
```

with

```go
		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`You invited <ansi fg="username">%s</ansi> to your party.`, invitedUser.Character.Name),
			[]string{invitedUser.Character.Name}, inviterSight))
		// The invitation is words from a person: "Someone" to a reader who
		// cannot see who, as speech reads (HideSpeakerNames).
		invitedUser.SendText(messaging.CategorySystem, messaging.HideSpeakerNames(
			fmt.Sprintf(`<ansi fg="username">%s</ansi> invited you to their party. Type <ansi fg="command">party accept</ansi> or <ansi fg="command">party decline</ansi> to respond.`, user.Character.Name),
			[]string{user.Character.Name}, messaging.ParticipantSight(invitedUser.Character, room)))
```

- [ ] **Step 6: rep.** In `internal/usercommands/report.go` replace

```go
		// Find target player in room
		target, err := actions.ResolveTargetActor(room, rest, actions.ResolveTargetOptions{Viewer: user.Character})
```

with

```go
		// Find target player in room. #454, owner call 2: a typed name
		// resolves only at full sight, or a whisper would confirm who is there
		// in the dark.
		name, refusal := actions.AimBySight(user.Character, user.UserId, room, rest, `rep`)
		if refusal != `` {
			user.SendText(messaging.CategorySystem, refusal)
			return true, nil
		}
		target, err := actions.ResolveTargetActor(room, name, actions.ResolveTargetOptions{Viewer: user.Character})
```

and replace

```go
		targetUser.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="whisper"><ansi fg="username">%s</ansi> reports to you: %s</ansi>`,
			c.Name, barText))
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`<ansi fg="whisper">You report to <ansi fg="username">%s</ansi>: %s</ansi>`,
			targetUser.Character.Name, barText))
```

with

```go
		// Each side's line hides the other's name at that reader's sight. The
		// report is a whisper, so its sender is "Someone" (HideSpeakerNames).
		targetUser.SendText(messaging.CategorySystem, messaging.HideSpeakerNames(fmt.Sprintf(
			`<ansi fg="whisper"><ansi fg="username">%s</ansi> reports to you: %s</ansi>`,
			c.Name, barText),
			[]string{c.Name}, messaging.ParticipantSight(targetUser.Character, room)))
		user.SendText(messaging.CategorySystem, messaging.HideNames(fmt.Sprintf(
			`<ansi fg="whisper">You report to <ansi fg="username">%s</ansi>: %s</ansi>`,
			targetUser.Character.Name, barText),
			[]string{targetUser.Character.Name}, messaging.ParticipantSight(c, room)))
```

- [ ] **Step 7: Run the tests, package and guards.**

Run: `go test ./internal/usercommands/ -run 'TalkAskSight|TalkSight|PartyInviteSight|RepSight' -count=1`
Expected: `ok`.

Run: `gofmt -l internal/usercommands` (no output), `go vet ./internal/usercommands`, `go test ./internal/usercommands/ -count=1`, `go test . -count=1`.
Expected: all `ok`. No guard trips (dry run).

- [ ] **Step 8: Commit.**

```bash
git add internal/usercommands/talk.go internal/usercommands/ask.go internal/usercommands/party.go internal/usercommands/report.go internal/usercommands/sight_aim_commands_test.go
git commit -m "fix(sight): talk, ask, party invite and rep name only what is seen (#454)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: H2e, fire judges sight before any name resolves (#454)

**Files:**
- Modify: `internal/actions/combat_fire.go:90` (new `ShotSight` above `ExecuteFire`)
- Modify: `internal/usercommands/shoot.go:20` (import), `:53`
- Test: `internal/usercommands/sight_aim_commands_test.go` (append)
- Docs: `internal/actions/context.md`

- [ ] **Step 1: Write the failing tests.** Add `"github.com/GoMudEngine/GoMud/internal/messaging"` to the imports of `internal/usercommands/sight_aim_commands_test.go` (after `items`) and append:

```go
// An aimed shot needs full sight of the room it is aimed into, judged before
// any name resolves. Resolving first answered "Bobrick is in your party!" to a
// shooter in pitch dark, which told them Bobrick was there.
func TestFireSight_NoSightRefusesBeforeANameResolves(t *testing.T) {
	user, room := aimScene(t, aimDark)
	p := parties.New(1)
	t.Cleanup(p.Disband)
	p.InvitePlayer(2)
	p.AcceptInvite(2)
	for _, rest := range []string{"bobrick", "nobody"} {
		_, _ = Fire(rest, user, room, events.EventFlag(0))
		told := aimTold(1)
		assert.Contains(t, told, "It is too dark to aim.", "%q", rest)
		assert.NotContains(t, told, "Bobrick", "%q", rest)
	}
}

// A shot through an exit is judged by what the shooter makes out of the room
// beyond (scanReach), not of their own lit room.
func TestShotSight_ThroughAnExitReadsTheRoomBeyond(t *testing.T) {
	user, room := aimScene(t, aimFull)
	rooms.LoadRoom(2).Biome = "cave"
	require.Equal(t, 0, rooms.LoadRoom(2).LightLevel())
	assert.Equal(t, messaging.SightFull, actions.ShotSight(user.Character, room, "skeleton"))
	assert.Equal(t, messaging.SightNone, actions.ShotSight(user.Character, room, "skeleton north"))
}
```

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./internal/usercommands/ -run 'FireSight|ShotSight' -count=1`
Expected: FAIL to build, `undefined: actions.ShotSight`. (With `ShotSight` in place but `shoot.go` unchanged, the first test fails with `"Bobrick is in your party!\n" does not contain "It is too dark to aim."` and `"You don't have a ranged weapon equipped.\n" does not contain "It is too dark to aim."`.)

- [ ] **Step 3: `ShotSight`.** In `internal/actions/combat_fire.go`, insert directly above `// ExecuteFire resolves a ranged shot immediately. rest is either "<target>"`:

```go
// ShotSight is what a player shooter makes out of the room a shot in rest is
// aimed into (#454): their sight of their own room for "<target>", or, for
// "<target words...> <direction>" through an exit, scanReach's sight into the
// room beyond (SightNone when it does not reach). It parses rest as
// ExecuteFire does and resolves no name, so a caller can refuse a shooter who
// cannot see before any name is looked up.
func ShotSight(viewer *characters.Character, room *rooms.Room, rest string) messaging.SightDecision {
	if room == nil {
		return messaging.SightNone
	}
	args := strings.Fields(rest)
	if len(args) >= 2 {
		if name, roomId := room.FindExitByName(args[len(args)-1]); name != "" {
			if adj := rooms.LoadRoom(roomId); adj != nil {
				sight, reaches := scanReach(viewer, room, adj)
				if !reaches {
					return messaging.SightNone
				}
				return sight
			}
		}
	}
	return messaging.ParticipantSight(viewer, room)
}

```

- [ ] **Step 4: The gate in `Fire`.** In `internal/usercommands/shoot.go`, add `"github.com/GoMudEngine/GoMud/internal/state/perception"` to the imports (after `internal/state`), and replace

```go
	// the deliberate price of safe pre-fire gating.
	tUserId, _, tRoom, _ := resolveShootTarget(room, rest, user.Character)
```

with

```go
	// the deliberate price of safe pre-fire gating.
	//
	// #454: an aimed shot needs full sight of the room it is aimed into, and
	// that is judged BEFORE any name resolves. Resolving first let the guards
	// below ("X is in your party!", "You can't shoot yourself.") and the
	// later too-dark refusal answer differently for a creature that is there,
	// which told a shooter in the dark who was present. Every name, a shape
	// included, is refused below full sight with the same lines ExecuteFire
	// would give.
	if strings.TrimSpace(rest) != `` {
		if actions.ShotSight(user.Character, room, rest) != messaging.SightFull {
			if user.Character.Perception != nil && user.Character.Perception.State() == perception.Blinded {
				user.SendText(messaging.CategorySystem, `You can't see well enough to aim.`)
			} else {
				user.SendText(messaging.CategorySystem,
					`It is too dark to aim. You need light, or eyes that do not need it.`)
			}
			return true, nil
		}
	}
	tUserId, _, tRoom, _ := resolveShootTarget(room, rest, user.Character)
```

- [ ] **Step 5: Docs.** In `internal/actions/context.md`, directly after the `UserActorAtSight` bullet (Task H2c), insert:

```markdown
- **`ShotSight(viewer, room, rest) messaging.SightDecision`**
  (`combat_fire.go`): what a shooter makes out of the room a shot is aimed
  into, parsing `rest` as `ExecuteFire` does: their own room, or `scanReach`
  through an exit. `usercommands.Fire` refuses below full sight before any
  name resolves.
```

- [ ] **Step 6: Run the tests, packages and guards.**

Run: `go test ./internal/usercommands/ -run 'FireSight|ShotSight' -count=1`
Expected: `ok`.

Run: `gofmt -l internal/usercommands internal/actions` (no output), `go vet ./internal/usercommands ./internal/actions`, `go test ./internal/usercommands/ ./internal/actions/ -count=1`, `go test . -count=1`.
Expected: all `ok`. No guard trips (dry run; the existing shoot tests in `shoot_test.go`, `shoot_narration_test.go`, `combat_fire_test.go` pass unchanged).

- [ ] **Step 7: Commit.**

```bash
git add internal/actions/combat_fire.go internal/actions/context.md internal/usercommands/shoot.go internal/usercommands/sight_aim_commands_test.go
git commit -m "fix(ranged): a shot is refused in the dark before any name resolves (#454)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: H2f, the quest NPC's return line, and the command list (#454)

**Files:**
- Modify: `internal/behaviortree/actions_quest.go:17` (import), `:175`
- Test: `internal/behaviortree/actions_quest_return_item_sight_test.go` (new)
- Docs: `internal/behaviortree/context.md:393`, `internal/usercommands/context.md` (new section above `#### **Skill-Based Commands**`)

- [ ] **Step 1: Write the failing test.** Create `internal/behaviortree/actions_quest_return_item_sight_test.go` (it reuses `seedReturnItemFixture` from `actions_quest_return_item_test.go`):

```go
package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #454: the quest NPC's "hands back" line printed its name raw. A player who
// gave to a shape in the dark never learned that name, so the line hides it at
// the player's sight.
func TestReturnItem_HandsBackLineHidesTheNameInTheDark(t *testing.T) {
	cleanup := seedReturnItemFixture(t)
	defer cleanup()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	room := rooms.LoadRoom(riRoomId)
	room.Biome = "cave"
	room.Lamp = nil
	if room.LightLevel() != 0 {
		t.Fatalf("fixture room must be pitch dark, got light %d", room.LightLevel())
	}

	mob := mobs.GetInstance(riInstanceId)
	given := items.New(riItemId)
	if !mob.Character.StoreItem(given) {
		t.Fatal("setup: mob could not store the given item")
	}
	user := users.GetByUserId(riUserId)
	events.DrainQueuedMessagesForTest(user.UserId)

	ctx := &EvalContext{
		InstanceId: riInstanceId, MobId: riTemplateId, RoomId: riRoomId, MobName: "test guard",
		Event: EventContext{EventType: "player_give", UserId: riUserId, ItemId: riItemId, ItemUUID: given.UUID, RoomId: riRoomId},
	}
	if res := actReturnItem(nil, ctx); res != Success {
		t.Fatalf("actReturnItem: want Success, got %v", res)
	}
	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
	if strings.Contains(out, "test guard") {
		t.Fatalf("a player who sees nothing read the NPC's name: %q", out)
	}
	if !strings.Contains(out, "hands back the brass token") {
		t.Fatalf("the hands-back line must still be sent: %q", out)
	}
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/behaviortree/ -run ReturnItem_HandsBack -count=1`
Expected: FAIL, `a player who sees nothing read the NPC's name: "<ansi fg=\"mob-emote\">test guard hands back the brass token.</ansi>..."`.

- [ ] **Step 3: Implement.** In `internal/behaviortree/actions_quest.go` add `"github.com/GoMudEngine/GoMud/internal/rooms"` to the imports (after `mutations`), and replace

```go
	user.SendText(messaging.CategoryMobEmote, fmt.Sprintf("%s hands back the %s.\n", ctx.MobName, item.Name()))
	return Success
```

with

```go
	// The NPC's name is hidden at the player's sight (#454): a player who gave
	// to a shape never learned it.
	sight := messaging.SightFull
	if room := rooms.LoadRoom(user.Character.RoomId); room != nil {
		sight = messaging.ParticipantSight(user.Character, room)
	}
	user.SendText(messaging.CategoryMobEmote, messaging.HideNames(
		fmt.Sprintf("%s hands back the %s.\n", ctx.MobName, item.Name()),
		[]string{ctx.MobName}, sight))
	return Success
```

- [ ] **Step 4: Docs.** In `internal/behaviortree/context.md:393` (the `return_item` row), replace the row's tail, from `` `ctx.Event.ItemId` match) `` through `` holding it. |`` (it holds a pre-existing dash between `match)` and `never`; copy the exact old text from the file into the Edit tool), with `` `ctx.Event.ItemId` match), never a fresh copy. Fails if the mob isn't holding it. The "hands back" line hides the NPC's name at the player's sight (#454). |``. This also removes the dash.

In `internal/usercommands/context.md`, insert directly above the FIRST `#### **Skill-Based Commands**` heading (the one followed by `- **Magic system**`, right after the Combat Commands list; a second heading, `#### **Skill-Based Commands** (`skill.*.go` files)`, sits further down, so anchor the Edit on the heading plus its `- **Magic system**` line):

```markdown
#### **Naming a creature in the dark (#454)**
Every command that names a creature in the room runs the typed name through
`actions.AimBySight` before it resolves: `attack` (a `*` wildcard needs some
sight), `target`, the melee specials (through `actions.StageMeleeTarget`),
`give` (the word `pet` is exempt), `show`, `consider`, `steal` and `plant`
(a refused noun may still be a container), `shadow`, `talk`, `ask`,
`party invite` and `rep`. `fire` refuses below full sight of the room it is
aimed into (`actions.ShotSight`) before any name resolves. Lines that follow
a shape name no one: `give`, `show`, `party invite` and `rep` hide each name
at its reader's sight with `messaging.HideNames` (an invitation or a report
to its recipient with `messaging.HideSpeakerNames`), and `consider`,
`steal`, `plant` and `shadow` hand their action `actions.UserActorAtSight`.
Party auto-assist skips a member who sees nothing.

```

- [ ] **Step 5: Run the tests, packages and guards.**

Run: `go test ./internal/behaviortree/ -run ReturnItem -count=1`
Expected: `ok` (all five `TestReturnItem_*`).

Run: `gofmt -l internal/behaviortree` (no output), `go vet ./internal/behaviortree`, `go test ./internal/behaviortree/ -count=1`, `go test . -count=1`, `python tools/context_md_audit.py` (none of `actions`, `usercommands`, `behaviortree` listed).
Expected: all `ok`.

- [ ] **Step 6: Commit.**

```bash
git add internal/behaviortree/actions_quest.go internal/behaviortree/actions_quest_return_item_sight_test.go internal/behaviortree/context.md internal/usercommands/context.md
git commit -m "fix(quest): the hands-back line names the NPC only at full sight (#454)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: H3a, The quit line is judged before the quitter leaves (#456)

**Files:**
- Modify: `internal/rooms/rooms.go:476-537` (`SendTextVisualWithAudio`), add `SendTextVisualWithAudioToSnapshot` and `deliverVisualElseAudio`
- Modify: `internal/hooks/PlayerDespawn_HandleLeave.go:150-153`, add `removeAndAnnounceDespawn` at the end of the file
- Modify: `internal/hooks/Logout_AwarenessCleanup.go:28-41`, add `despawnUnseenByKey` and `playersNotPerceiving`
- Modify: `messaging_surface_guard_test.go:898`, `bauble_finder_view_guard_test.go:99`
- Test: `internal/rooms/visual_snapshot_test.go` (append), `internal/hooks/despawn_line_snapshot_test.go` (new)
- Docs: `internal/rooms/context.md:22-28`, `internal/hooks/context.md:1148-1152`

- [ ] **Step 1: Write the failing rooms test.** In `internal/rooms/visual_snapshot_test.go`, replace the import block with:

```go
import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)
```

and append:

```go
// #456: SendTextVisualWithAudioToSnapshot is the sound twin. A departure is
// judged by the room before the mover left with their light: a reader who
// saw them then reads the line, one who saw nothing then hears the sound,
// and one who was not there then reads nothing.
func TestSendTextVisualWithAudioToSnapshot_SightThenSoundAtTheMoment(t *testing.T) {
	r := sightTestRoom(t, "cave")
	r.Lamp = LampPtr(60) // the mover's light, still here
	ordel := users.GetByUserId(7413).Character
	ordel.Perception = characters.New().Perception
	require.NoError(t, ordel.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	r.RemovePlayer(7412) // not here when the snapshot is taken

	snap := r.VisualSnapshot()
	require.Equal(t, VisualSnapshot{7411: messaging.SightFull, 7413: messaging.SightNone}, snap)

	// The change: the light leaves with its bearer; Bobrick arrives.
	r.Lamp = nil
	require.Equal(t, messaging.SightNone, r.ParticipantSight(7411), "fixture: the room must now be dark")
	r.AddPlayer(7412)
	for _, id := range []int{7411, 7412, 7413} {
		events.DrainQueuedMessagesForTest(id)
	}

	r.SendTextVisualWithAudioToSnapshot(snap, messaging.CategoryRoomExit,
		`<ansi fg="username">Kesh</ansi> leaves to the north.`, `You hear someone leave the room.`)

	saw := events.DrainQueuedMessageEventsForTest(7411)
	require.Len(t, saw, 1, "a reader who saw the mover then reads the line")
	require.Equal(t, "Kesh leaves to the north.", hidingSenderText(saw[0]))

	heard := events.DrainQueuedMessageEventsForTest(7413)
	require.Len(t, heard, 1, "a reader who saw nothing then hears it")
	require.Equal(t, "You hear someone leave the room.", hidingSenderText(heard[0]))

	require.Empty(t, events.DrainQueuedMessageEventsForTest(7412), "a reader who arrived after reads nothing")
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/rooms/ -run TestSendTextVisualWithAudioToSnapshot -count=1`
Expected: FAIL, build error `r.SendTextVisualWithAudioToSnapshot undefined`.

- [ ] **Step 3: Implement the sound twin.** In `internal/rooms/rooms.go`, replace the whole body of `SendTextVisualWithAudio` (from `func (r *Room) SendTextVisualWithAudio(` to its closing brace; keep its doc comment) with:

```go
func (r *Room) SendTextVisualWithAudio(cat messaging.Category, visualTxt string, audioTxt string, excludeUserIds ...int) {
	for _, uid := range r.GetPlayers() {
		if excluded(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		deliverVisualElseAudio(u, visualDecision(u.Character, r), cat, visualTxt, audioTxt)
	}
}

// SendTextVisualWithAudioToSnapshot is SendTextVisualWithAudio judged against
// snap, the sound twin of SendTextVisualToSnapshot: it reaches exactly the
// players who are in the room now AND were in snap, each at the decision snap
// recorded for them. A reader recorded at SightNone hears audioTxt.
//
// A departure is the case that prompted it (#456, owner ruling 2026-10-09): a
// mover is seen by the light they carry on their own way out, so the line is
// judged by the room before they left with it. Take the snapshot before the
// move, move, then send.
func (r *Room) SendTextVisualWithAudioToSnapshot(snap VisualSnapshot, cat messaging.Category, visualTxt string, audioTxt string, excludeUserIds ...int) {
	for _, uid := range r.GetPlayers() {
		if excluded(uid, excludeUserIds) {
			continue
		}
		decision, ok := snap[uid]
		if !ok {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		deliverVisualElseAudio(u, decision, cat, visualTxt, audioTxt)
	}
}

// deliverVisualElseAudio is the one per-recipient body of the two
// visual-with-audio senders: visualTxt on the visual channel at decision, or
// audioTxt on the audio channel for a reader at SightNone (nothing at all when
// audioTxt is empty).
func deliverVisualElseAudio(u *users.UserRecord, decision messaging.SightDecision, cat messaging.Category, visualTxt string, audioTxt string) {
	in := messaging.RenderInput{
		Category:      cat,
		Text:          visualTxt,
		Channel:       messaging.ChannelVisual,
		SightDecision: decision,
		LineWidth:     u.GetLineWidth(),
	}
	if decision == messaging.SightNone {
		if audioTxt == "" {
			return
		}
		// The audio channel deliberately skips the sight gate and the
		// anonymizer, which is correct here: audioTxt is already written
		// to name nobody.
		in = messaging.RenderInput{
			Category:  cat,
			Text:      audioTxt,
			Channel:   messaging.ChannelAudio,
			LineWidth: u.GetLineWidth(),
		}
	}

	rendered := messaging.RenderForRecipient(in)
	if rendered == "" {
		return
	}
	events.AddToQueue(events.Message{
		UserId: u.UserId,
		Text:   rendered + "\n",
	})
}
```

`visualDecision` (same file) is exactly the switch the old body inlined (`CanSeeClearly` to `SightFull`, `CanSeeShapes` to `SightShapes`, else `SightNone`), so `SendTextVisualWithAudio` is unchanged.

- [ ] **Step 4: Run the rooms package.**

Run: `go test ./internal/rooms/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Write the failing quit-line tests.** Create `internal/hooks/despawn_line_snapshot_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #456: the quit line is judged by the room before the quitter left. The
// playtest's quitter wore a lit torch at night; the watcher, who had just
// read their name, read "drag a figure away", because the line went out
// after RemovePlayer had taken the torch's light with them.

const despawnTestLine = `Suddenly and without warning spirits and demons reach up through the ground and drag <ansi fg="username">Aliceia</ansi> away!`

func TestDespawnLine_JudgedByTheQuittersOwnLight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	quitter := users.GetByUserId(1)
	require.True(t, quitter.Character.Conditions.AddCondition(lanternConditionId, false))
	require.Equal(t, messaging.SightFull, room.ParticipantSight(2), "fixture: the quitter's light lights the room")
	drainPlain(2)

	removeAndAnnounceDespawn(room, quitter, despawnTestLine)

	require.Equal(t, messaging.SightNone, room.ParticipantSight(2), "fixture: the light left with the quitter")
	assert.Equal(t, 1, countContaining(drainPlain(2), "drag Aliceia away!"),
		"the watcher saw the quitter by their own light as they went")
}

// A hidden quitter is forced visible by logout before HandleLeave runs
// (Logout_AwarenessCleanup.go). The readers who could not make them out
// before that are excluded from the line: it would otherwise name, to a
// room that never saw them, someone it did not know was there.
func TestDespawnLine_ReadersWhoDidNotPerceiveTheQuitterReadNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90)
	quitter := users.GetByUserId(1)
	hidePlayer(t, quitter)
	drainPlain(2)

	onPlayerDespawnForAwareness(events.PlayerDespawn{UserId: 1, RoomId: 1})
	require.False(t, quitter.Character.IsHidden(), "fixture: logout forces the quitter visible")
	removeAndAnnounceDespawn(room, quitter, despawnTestLine)

	assert.Equal(t, 0, countContaining(drainPlain(2), "drag"),
		"a reader who could not see the hidden quitter read their quit line")
}
```

(`hidePlayer` is in `hidden_player_disruption_test.go`; `seedNarrationConditions`, `darken`, `drainPlain` in `narration_testhelpers_test.go`.) The hooks tests point `FilePaths.DataFiles` at an empty temp dir, so `templates.Process` cannot run here: the helper takes the processed line as an argument.

- [ ] **Step 6: Extract the helper with today's order, so the tests compile and fail on behaviour.** In `internal/hooks/PlayerDespawn_HandleLeave.go`, replace:

```go
	if _, ok := room.RemovePlayer(evt.UserId); ok {
		tplTxt, _ := templates.Process("player-despawn", user.Character.Name)
		sendVisualRoomText(room, messaging.CategoryLogout, tplTxt)
	}
```

with:

```go
	despawnTxt, _ := templates.Process("player-despawn", user.Character.Name)
	removeAndAnnounceDespawn(room, user, despawnTxt)
```

and append at the end of the file:

```go
func removeAndAnnounceDespawn(room *rooms.Room, user *users.UserRecord, line string) {
	if _, ok := room.RemovePlayer(user.UserId); ok {
		sendVisualRoomText(room, messaging.CategoryLogout, line)
	}
}
```

(The local is `despawnTxt` because `tplTxt` is declared with `:=` two lines further down for the `goodbye` template.)

- [ ] **Step 7: Run them and see them fail.**

Run: `go test ./internal/hooks/ -run TestDespawnLine_ -count=1`
Expected: FAIL, both: "the watcher saw the quitter by their own light as they went" and "a reader who could not see the hidden quitter read their quit line".

- [ ] **Step 8: Judge the line before the move, and exclude the unseeing.** Replace the helper appended in Step 6 with:

```go
// removeAndAnnounceDespawn takes the quitter out of room and tells the room,
// judged by the room BEFORE they left (#456, owner ruling 2026-10-09: a mover
// is seen by the light they carry on their own way out). Readers listed under
// despawnUnseenByKey, who could not make out a hidden quitter before logout
// forced them visible, read nothing.
func removeAndAnnounceDespawn(room *rooms.Room, user *users.UserRecord, line string) {
	snap := room.VisualSnapshot()
	if _, ok := room.RemovePlayer(user.UserId); ok {
		unseenBy, _ := user.GetTempData(despawnUnseenByKey).([]int)
		room.SendTextVisualToSnapshot(snap, messaging.CategoryLogout, line, nil, unseenBy...)
	}
}
```

In `internal/hooks/Logout_AwarenessCleanup.go`, replace the import block with:

```go
import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)
```

replace:

```go
	u := users.GetByUserId(evt.UserId)
	if u == nil {
		return events.Continue
	}
	u.Character.Awareness.ForceVisible(state.TransitionReason{
```

with:

```go
	u := users.GetByUserId(evt.UserId)
	if u == nil {
		return events.Continue
	}
	// #456: HandleLeave's quit line runs after this, when the quitter is
	// visible to everyone. Record now who could NOT make them out, so the
	// line skips those readers. nil (no one) deletes the key.
	var unseenBy []int
	if room := rooms.LoadRoom(u.Character.RoomId); room != nil {
		unseenBy = playersNotPerceiving(room, u.Character)
	}
	if len(unseenBy) == 0 {
		u.SetTempData(despawnUnseenByKey, nil)
	} else {
		u.SetTempData(despawnUnseenByKey, unseenBy)
	}
	u.Character.Awareness.ForceVisible(state.TransitionReason{
```

and append at the end of the file:

```go
// despawnUnseenByKey is the user temp-data key onPlayerDespawnForAwareness
// sets for HandleLeave: the ids of the players in the quitter's room who did
// not perceive them before logout forced them visible.
const despawnUnseenByKey = `despawnUnseenBy`

// playersNotPerceiving lists the players in room who cannot make out c
// (characters.Character.Perceives: c is hidden and they have no see-hidden).
func playersNotPerceiving(room *rooms.Room, c *characters.Character) []int {
	var ids []int
	for _, uid := range room.GetPlayers() {
		if v := users.GetByUserId(uid); v != nil && !v.Character.Perceives(c) {
			ids = append(ids, uid)
		}
	}
	return ids
}
```

- [ ] **Step 9: Run them and see them pass.**

Run: `go test ./internal/hooks/ -run TestDespawnLine_ -count=1`
Expected: `ok`.

- [ ] **Step 10: Teach the two root guards the new sender.** Neither fails without this (dry run), but both match senders by exact name and would silently stop seeing the departure lines Task H3b moves onto it.

In `messaging_surface_guard_test.go`, replace:

```go
			"SendTextVisualToSnapshot",
			// Sight gates slice 5b
```

with:

```go
			"SendTextVisualToSnapshot",
			// Its sound twin (#456): a departure judged by the room before
			// the mover left with their own light.
			"SendTextVisualWithAudioToSnapshot",
			// Sight gates slice 5b
```

In `bauble_finder_view_guard_test.go`, replace:

```go
	"SendTextVisualWithAudio": true, "SendTextVisualToSnapshot": true,
```

with:

```go
	"SendTextVisualWithAudio": true, "SendTextVisualToSnapshot": true, "SendTextVisualWithAudioToSnapshot": true,
```

- [ ] **Step 11: Run the packages and the root guards.**

Run: `go test ./internal/rooms/ ./internal/hooks/ -count=1` then `go test . -count=1`
Expected: `ok` for each. The quit line's observer send is `room.SendTextVisualToSnapshot` (receiver `room`), which the narration guard already recognises, as it recognised `sendVisualRoomText` before.

- [ ] **Step 12: Update `context.md`.** In `internal/rooms/context.md`, replace:

```markdown
  first, make the change, then send. Lighting plan 5d uses it for a
```

with:

```markdown
  first, make the change, then send. Its sound twin,
  `SendTextVisualWithAudioToSnapshot(snap, cat, visualTxt, audioTxt,
  excludeUserIds...)`, is `SendTextVisualWithAudio` judged against a
  snapshot: a reader recorded at `SightNone` hears `audioTxt`. The departure
  lines use it (#456, owner ruling 2026-10-09: a mover is seen by the light
  they carry on their own way out; arrivals stay judged after the move). Both
  visual-with-audio senders share one per-reader body,
  `deliverVisualElseAudio`. Lighting plan 5d uses `SendTextVisualToSnapshot` for a
```

In `internal/hooks/context.md`, under `### Logout_AwarenessCleanup.go`, after the paragraph ending `state or leaks if a character is reused or respawned.`, add:

```markdown

Before it forces the quitter visible, `onPlayerDespawnForAwareness` stores
under the user temp-data key `despawnUnseenByKey` the players in the room
who could not make them out (`playersNotPerceiving`, by
`Character.Perceives`). `HandleLeave` runs after it (`events.Last`) and its
`removeAndAnnounceDespawn` takes `Room.VisualSnapshot()` before
`RemovePlayer`, then sends the `player-despawn` line with
`SendTextVisualToSnapshot`, excluding those players (#456): the quitter is
seen going by their own light, and a hidden quitter is not named to a room
that never saw them.
```

- [ ] **Step 13: Format, vet, commit.**

Run: `gofmt -l internal/rooms internal/hooks *.go` (expect no output), `go vet ./internal/rooms ./internal/hooks` (expect no output).

```bash
git add internal/rooms/rooms.go internal/rooms/visual_snapshot_test.go internal/hooks/PlayerDespawn_HandleLeave.go internal/hooks/Logout_AwarenessCleanup.go internal/hooks/despawn_line_snapshot_test.go messaging_surface_guard_test.go bauble_finder_view_guard_test.go internal/rooms/context.md internal/hooks/context.md
git commit -m "fix(lighting): the quit line is judged before the quitter leaves (#456)

Room.SendTextVisualWithAudioToSnapshot is the sound twin of
SendTextVisualToSnapshot. The quit line snapshots the room before
RemovePlayer, and skips readers who could not see a hidden quitter
before logout forced them visible.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: H3b, Departure lines are judged before the move (#456)

Runs after Task H3a (it calls `Room.SendTextVisualWithAudioToSnapshot`).

**Files:**
- Modify: `internal/usercommands/go.go:265`, `:308`, `:316`
- Modify: `internal/actions/relocate_mob.go:98-105`
- Modify: `internal/mobcommands/go.go:72-86`
- Test: `internal/usercommands/go_departure_light_test.go` (new), `internal/actions/relocate_mob_departure_light_test.go` (new), `internal/mobcommands/go_departure_light_test.go` (new)
- Docs: `internal/usercommands/context.md:275-276`, `internal/actions/context.md:311-312`, `internal/mobcommands/context.md:294-296`

- [ ] **Step 1: Write the three failing tests.**

Create `internal/usercommands/go_departure_light_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const goDepartureLightCond = 9781 // a carried light, literal strength 60

// #456, owner ruling 2026-10-09: a mover is seen by the light they carry on
// their own way out. Aliceia carries the only light in the dark cave (room 2)
// and walks south; Bobrick, left behind in the dark, saw her go by her own
// light, so he reads her name, not the sound.
func TestGo_DepartureIsJudgedByTheMoversOwnLight(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		goDepartureLightCond: {ConditionId: goDepartureLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	room1, cave := rooms.LoadRoom(1), rooms.LoadRoom(2)
	cave.Biome = "cave"
	mover, watcher := users.GetByUserId(1), users.GetByUserId(2)
	for _, u := range []*users.UserRecord{mover, watcher} {
		room1.RemovePlayer(u.UserId)
		u.Character.RoomId = 2
		cave.AddPlayer(u.UserId)
	}
	mover.Character.ActionPoints = 100
	require.True(t, mover.Character.Conditions.AddCondition(goDepartureLightCond, false))
	require.Equal(t, messaging.SightFull, cave.ParticipantSight(watcher.UserId), "fixture: the mover's light lights the cave")
	events.DrainQueuedMessagesForTest(watcher.UserId)

	handled, err := Go("south", mover, cave, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, mover.Character.RoomId, "fixture: the mover left")
	require.Equal(t, messaging.SightNone, cave.ParticipantSight(watcher.UserId), "fixture: the light left with the mover")

	got := hoodTestText(watcher.UserId)
	require.Contains(t, got, "Aliceia", "the watcher saw the mover go by her own light")
	require.NotContains(t, got, "You hear someone leave the room.")
}
```

(Literal 60, not 40: in this fixture a 40 light puts the watcher at shapes, and the precondition needs faces. `hoodTestText` is in `hood_test.go`; the fixture's room 2 is "Dark Cave" with a `south` exit to room 1, and `seedAllRegistries` seeds the `cave` biome with no sky light.)

Create `internal/actions/relocate_mob_departure_light_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const relocateLightCond = 9782 // a carried light, literal strength 60

// #456, owner ruling 2026-10-09: a mover is seen by the light they carry on
// their own way out. The mob carries the only light in a cave and walks
// north; the watcher left in the dark saw it go by that light, so reads its
// name rather than the footsteps.
func TestRelocateMob_DepartureIsJudgedByTheMoversOwnLight(t *testing.T) {
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const from, to, instId, watcherId = 99441, 99442, 98441, 99451
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		from: {RoomId: from, Zone: "test", Biome: "cave", Exits: map[string]exit.RoomExit{"north": {RoomId: to}}},
		to:   {RoomId: to, Zone: "test", Biome: "cave", Exits: map[string]exit.RoomExit{"south": {RoomId: from}}},
	}, map[string]*rooms.ZoneConfig{}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		relocateLightCond: {ConditionId: relocateLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	w := users.NewTestUser(watcherId, "watcher", "Watcher", 0)
	w.Character.RoomId = from
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{watcherId: w}))

	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Lamplighter"
	m.Character.RoomId = from
	if !m.Character.Conditions.AddCondition(relocateLightCond, false) {
		t.Fatal("fixture: the mob must carry the light")
	}
	mobs.SetInstanceForTest(instId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })

	fromRoom, toRoom := rooms.LoadRoom(from), rooms.LoadRoom(to)
	fromRoom.AddPlayer(watcherId)
	fromRoom.AddMob(instId)
	if got := fromRoom.ParticipantSight(watcherId); got != messaging.SightFull {
		t.Fatalf("fixture: the mob's light must light the room, sight %d", got)
	}
	events.DrainQueuedMessagesForTest(watcherId)

	RelocateMob(m, fromRoom, "north", toRoom, false)

	if got := fromRoom.ParticipantSight(watcherId); got != messaging.SightNone {
		t.Fatalf("fixture: the light must leave with the mob, sight %d", got)
	}
	got := strings.Join(events.DrainQueuedMessagesForTest(watcherId), "\n")
	if !strings.Contains(got, "Lamplighter") || strings.Contains(got, "footsteps") {
		t.Fatalf("the watcher must see the mob leave by its own light, got %q", got)
	}
}
```

Create `internal/mobcommands/go_departure_light_test.go`:

```go
package mobcommands

import (
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const mobGoLightCond = 9783 // a carried light, literal strength 60

// #456, owner ruling 2026-10-09: a mover is seen by the light they carry on
// their own way out. A forced `go <roomId>` (callforhelp) by a mob carrying
// the only light in a cave: the watcher left in the dark saw it run off by
// that light, so reads its name rather than the footsteps.
func TestMobGo_ForcedMove_DepartureIsJudgedByTheMoversOwnLight(t *testing.T) {
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const fromRoom, toRoom, instId, watcherId = 9620, 9621, 98620, 9630
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		fromRoom: {RoomId: fromRoom, Zone: "test", Biome: "cave"},
		toRoom:   {RoomId: toRoom, Zone: "test", Biome: "cave"},
	}, map[string]*rooms.ZoneConfig{}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		mobGoLightCond: {ConditionId: mobGoLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	w := users.NewTestUser(watcherId, "watcher", "Watcher", 0)
	w.Character.RoomId = fromRoom
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{watcherId: w}))

	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Caller"
	m.Character.RoomId = fromRoom
	if !m.Character.Conditions.AddCondition(mobGoLightCond, false) {
		t.Fatal("fixture: the mob must carry the light")
	}
	mobs.SetInstanceForTest(instId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })

	from := rooms.LoadRoom(fromRoom)
	from.AddPlayer(watcherId)
	from.AddMob(instId)
	if got := from.ParticipantSight(watcherId); got != messaging.SightFull {
		t.Fatalf("fixture: the mob's light must light the room, sight %d", got)
	}
	events.DrainQueuedMessagesForTest(watcherId)

	if handled, err := Go(strconv.Itoa(toRoom), m, from); err != nil || !handled {
		t.Fatalf("Go: handled %v, err %v", handled, err)
	}

	if got := from.ParticipantSight(watcherId); got != messaging.SightNone {
		t.Fatalf("fixture: the light must leave with the mob, sight %d", got)
	}
	got := strings.Join(events.DrainQueuedMessagesForTest(watcherId), "\n")
	if !strings.Contains(got, "runs off suddenly") || strings.Contains(got, "footsteps") {
		t.Fatalf("the watcher must see the mob run off by its own light, got %q", got)
	}
}
```

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./internal/usercommands/ -run TestGo_DepartureIsJudgedByTheMoversOwnLight -count=1`
Expected: FAIL, `"<ansi fg=\"room-exit\">You hear someone leave the room.</ansi>..." does not contain "Aliceia"`.

Run: `go test ./internal/actions/ -run TestRelocateMob_DepartureIsJudgedByTheMoversOwnLight -count=1`
Expected: FAIL, `the watcher must see the mob leave by its own light, got "<ansi fg=\"room-exit\">You hear footsteps moving away.</ansi>\n"`.

Run: `go test ./internal/mobcommands/ -run TestMobGo_ForcedMove_DepartureIsJudged -count=1`
Expected: FAIL, `the watcher must see the mob run off by its own light, got "<ansi fg=\"room-exit\">You hear hurried footsteps receding.</ansi>\n"`.

- [ ] **Step 3: Player `go`.** In `internal/usercommands/go.go`, replace:

```go
		if err := rooms.MoveToRoom(user.UserId, destRoom.RoomId); err != nil {
```

with:

```go
		// #456, owner ruling 2026-10-09: a mover is seen by the light they
		// carry on their own way out, so the departure line below is judged
		// by the room before the move. Arrivals keep being judged after it.
		departSnap := room.VisualSnapshot()

		if err := rooms.MoveToRoom(user.UserId, destRoom.RoomId); err != nil {
```

Then change both departure sends (the two identical `room.SendTextVisualWithAudio(messaging.CategoryRoomExit,` lines, two lines above `` fmt.Sprintf(`<ansi fg="username">%s</ansi> and %s leave %s.` `` and `` fmt.Sprintf(`<ansi fg="username">%s</ansi> leaves %s.` ``; use the Edit tool's `replace_all`, since the line is the same text twice and nowhere else in the file) from:

```go
					room.SendTextVisualWithAudio(messaging.CategoryRoomExit,
```

to:

```go
					room.SendTextVisualWithAudioToSnapshot(departSnap, messaging.CategoryRoomExit,
```

Leave every `destRoom.SendTextVisualWithAudio(messaging.CategoryRoomEntry, ...)` arrival line as it is (owner call 1).

- [ ] **Step 4: `RelocateMob`.** In `internal/actions/relocate_mob.go`, replace:

```go
	from.RemoveMob(mob.InstanceId)
	ClearRoomAggroOnDeparture(from, mob.InstanceId)
	dest.AddMob(mob.InstanceId)

	c := configs.GetTextFormatsConfig()

	if !sneaking {
		from.SendTextVisualWithAudio(messaging.CategoryRoomExit,
```

with:

```go
	// #456, owner ruling 2026-10-09: a mover is seen by the light they carry
	// on their own way out, so the exit line is judged by the room before
	// the move. The entry line keeps being judged after it.
	departSnap := from.VisualSnapshot()

	from.RemoveMob(mob.InstanceId)
	ClearRoomAggroOnDeparture(from, mob.InstanceId)
	dest.AddMob(mob.InstanceId)

	c := configs.GetTextFormatsConfig()

	if !sneaking {
		from.SendTextVisualWithAudioToSnapshot(departSnap, messaging.CategoryRoomExit,
```

- [ ] **Step 5: Mob forced `go`.** In `internal/mobcommands/go.go`, replace:

```go
			sneaking := actions.MobIsSneaking(mob)

			room.RemoveMob(mob.InstanceId)
			actions.ClearRoomAggroOnDeparture(room, mob.InstanceId)
			destRoom.AddMob(mob.InstanceId)

			if !sneaking {
				// Tell the old room they are leaving
				sendMovementMessage(room, messaging.CategoryRoomExit,
```

with:

```go
			sneaking := actions.MobIsSneaking(mob)

			// #456, owner ruling 2026-10-09: a mover is seen by the light
			// they carry on their own way out, so the exit line is judged
			// by the room before the move; the entry line, after it.
			departSnap := room.VisualSnapshot()

			room.RemoveMob(mob.InstanceId)
			actions.ClearRoomAggroOnDeparture(room, mob.InstanceId)
			destRoom.AddMob(mob.InstanceId)

			if !sneaking {
				// Tell the old room they are leaving
				room.SendTextVisualWithAudioToSnapshot(departSnap, messaging.CategoryRoomExit,
```

`sendMovementMessage` keeps its one remaining caller, the entry line.

- [ ] **Step 6: Run them and see them pass.**

Run: `go test ./internal/usercommands/ -run TestGo_ -count=1`, `go test ./internal/actions/ -run TestRelocateMob -count=1`, `go test ./internal/mobcommands/ -count=1`
Expected: `ok` for each (the existing `TestRelocateMob_*` sneak and exit-line tests and `TestMobGo_ForcedMove_SneakGatesTheAnnounceLines` still pass).

- [ ] **Step 7: Run the affected packages and the root guards.**

Run: `go test ./internal/usercommands/ ./internal/actions/ ./internal/mobcommands/ -count=1` then `go test . -count=1`
Expected: `ok` for each. In the dry run no root guard failed: `move_wrapper_guard_test.go` reads both `go.go` files for pricing and detection only, `copy_no_dash_test.go` lists `usercommands/go.go` and the new comment has no dash, and the narration guard sees the departure sends through the name Task H3a added.

- [ ] **Step 8: Update `context.md`.** In `internal/usercommands/context.md`, replace:

```markdown
`rooms.MoveToRoom` success branch. `move_wrapper_guard_test.go` (repo root)
fails if this file prices or detects a step itself again.
```

with:

```markdown
`rooms.MoveToRoom` success branch. `move_wrapper_guard_test.go` (repo root)
fails if this file prices or detects a step itself again. The departure line
("X leaves to the north.") is judged by the room before the move: `Go` takes
`room.VisualSnapshot()` just before `rooms.MoveToRoom` and sends with
`SendTextVisualWithAudioToSnapshot`, so a mover is seen leaving by the light
they carry out (#456, owner ruling 2026-10-09). Arrival lines are judged
after the move.
```

In `internal/actions/context.md`, replace:

```markdown
  through the same exit. A sneaking mob sends no exit, entry or next-room
  line, as a sneaking player never has (parity slice 6, ruling D1).
```

with:

```markdown
  through the same exit. A sneaking mob sends no exit, entry or next-room
  line, as a sneaking player never has (parity slice 6, ruling D1). The exit
  line is judged by `from` before the move (`Room.VisualSnapshot`, then
  `SendTextVisualWithAudioToSnapshot`), so a mob is seen leaving by the light
  it carries out with it (#456); the entry line is judged after the move.
```

In `internal/mobcommands/context.md`, replace:

```markdown
  move and sends neither line when sneaking, matching the ordinary exit
  path.
```

with:

```markdown
  move and sends neither line when sneaking, matching the ordinary exit
  path. Its exit line is judged by the room before the move
  (`Room.VisualSnapshot`, then `SendTextVisualWithAudioToSnapshot`, #456);
  the entry line still goes through `sendMovementMessage`, judged after it.
```

- [ ] **Step 9: Format, vet, commit.**

Run: `gofmt -l internal/usercommands internal/actions internal/mobcommands` (expect no output), `go vet ./internal/usercommands ./internal/actions ./internal/mobcommands` (expect no output).

```bash
git add internal/usercommands/go.go internal/usercommands/go_departure_light_test.go internal/actions/relocate_mob.go internal/actions/relocate_mob_departure_light_test.go internal/mobcommands/go.go internal/mobcommands/go_departure_light_test.go internal/usercommands/context.md internal/actions/context.md internal/mobcommands/context.md
git commit -m "fix(lighting): departure lines are judged before the move (#456)

Owner ruling 2026-10-09: a mover is seen by the light they carry on
their own way out. Player go, RelocateMob and the mob forced go
snapshot the room before the move and send the exit line with
SendTextVisualWithAudioToSnapshot. Arrivals are unchanged.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: H4, The sneak hide survives a reload (#451)

**Files:**
- Modify: `internal/characters/shroud_hide.go` (append after `reconcileShroudHide`, which ends the file at `:200`)
- Modify: `internal/characters/validate.go:696`
- Test: `internal/characters/shroud_hide_test.go` (imports, append)
- Docs: `internal/characters/context.md:1458-1463`

- [ ] **Step 1: Write the failing test.** In `internal/characters/shroud_hide_test.go`, add `"gopkg.in/yaml.v2"` to the import block, after `"github.com/stretchr/testify/require"` (the same yaml package `users.loadUserFromPath` decodes a save with):

```go
import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)
```

Then append to the end of the file:

```go

// #451: a player saved while hidden by a sneak (autosave, copyover, a
// graceful shutdown) came back Visible, and the load's permanent rebuild
// (Validate(true), users.loadUserFromPath) found no source for the saved
// permanent record 9, so it expired and the prune pass told the room
// "emerges from the shadows". A live 9 on a reloaded holder re-enters the
// sneak hide the way a live 31 re-enters the shroud hide.
func TestSneakHide_SurvivesReload(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	sneakHide(t, c)
	c.SetMiscData(`sneaking`, true)

	saved, err := yaml.Marshal(c)
	require.NoError(t, err)
	loaded := &Character{}
	require.NoError(t, yaml.Unmarshal(saved, loaded))
	require.Nil(t, loaded.Awareness, "fixture: Awareness is not saved")

	require.NoError(t, loaded.Validate(true)) // what users.loadUserFromPath runs
	require.True(t, loaded.IsHidden(), "the sneak hide must survive the reload")
	require.False(t, loaded.HiddenByShroud(), "it comes back as a sneak")
	require.True(t, loaded.holdsLiveStealthRecord(), "record 9 must stay live")
	for _, rec := range loaded.Conditions.List {
		if rec.ConditionId == conditionIdHidden {
			require.False(t, rec.Expired(), "an expired 9 is pruned with its end line")
		}
	}
	require.Equal(t, true, loaded.GetMiscData(`sneaking`), "the saved flag matches the hide")

	require.NoError(t, loaded.Validate(true))
	require.True(t, loaded.IsHidden(), "a second Validate changes nothing")
}

// The sibling: a 9 that a reveal already cancelled stays cancelled.
func TestSneakHide_CancelledRecordStaysVisibleOnReload(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	sneakHide(t, c)
	c.Conditions.RemoveCondition(conditionIdHidden) // what the reveal cascade's cancel leaves
	c.Awareness = awareness.NewMachine()
	require.NoError(t, c.Validate(true))
	require.False(t, c.IsHidden())
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/characters/ -run 'TestSneakHide_' -count=1`
Expected: FAIL in `TestSneakHide_SurvivesReload`, "the sneak hide must survive the reload". `TestSneakHide_CancelledRecordStaysVisibleOnReload` passes on master and after the fix: it is the guard that the fix does not revive a cancelled hide.

- [ ] **Step 3: Implement.** Append to `internal/characters/shroud_hide.go`, after the closing brace of `reconcileShroudHide`:

```go

// reconcileSneakHide is reconcileShroudHide's sibling for a sneak hide
// (#451). Validate calls it after reconcileShroudHide.
//
// Reload: a loaded character gets a fresh, Visible Awareness machine while
// its saved permanent record 9 comes back live, and the permanent rebuild
// (Validate(true)) found no source for a 9 on a Visible holder, so it
// expired and the prune pass told the room "emerges from the shadows". A
// Visible holder of a live 9 re-enters the sneak hide through
// hideForStealthRecord, the door record 9 lands through, so the rebuild
// that follows keeps the 9 (reapplyPermanentConditions sources a live 9 on
// a sneak-hidden holder) and the saved `sneaking` flag matches the hide.
// Every exit from Hidden cancels 9 with the other hidden-flag records
// (Awareness_Cascades.go), so a revealed hide does not come back here.
func (c *Character) reconcileSneakHide() {
	if c.Awareness == nil {
		return
	}
	if c.Awareness.State() == awareness.Visible && c.holdsLiveStealthRecord() {
		c.hideForStealthRecord(conditionIdHidden)
	}
}
```

In `internal/characters/validate.go`, replace

```go
	c.reconcileShroudHide()
```

with

```go
	c.reconcileShroudHide()
	c.reconcileSneakHide()
```

- [ ] **Step 4: Run it and see it pass, then the package.**

Run: `go test ./internal/characters/ -run 'TestSneakHide_|TestShroud|TestStealthRecord' -count=1`
Expected: PASS.

Run: `go test ./internal/characters/ ./internal/hooks/ ./internal/actions/ ./internal/users/ ./internal/mobs/ -count=1`
Expected: every package `ok`.

- [ ] **Step 5: Docs.** In `internal/characters/context.md`, replace

```
through `hideForStealthRecord` at the record's score. Breaking: an observer
```

with

```
through `hideForStealthRecord` at the record's score. `reconcileSneakHide()`
runs right after it and does the same for a sneak hide (#451): a Visible
holder of a live record 9 (a reload) re-enters the sneak hide through
`hideForStealthRecord(9)`, so the `Validate(true)` rebuild keeps the 9, no end
line is told, and the saved `sneaking` misc key matches. Breaking: an observer
```

- [ ] **Step 6: Format and commit.**

Run: `gofmt -l internal/characters` (expect no output).

```bash
git add internal/characters/shroud_hide.go internal/characters/validate.go internal/characters/shroud_hide_test.go internal/characters/context.md
git commit -m "fix(stealth): a sneak hide survives a reload (#451)

A saved sneak-hidden character reloaded Visible and the load's permanent
rebuild expired its record 9, so the room read the end line. A Visible
holder of a live 9 now re-enters the sneak hide, as the shroud hide does.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: H5, A mob fold is never dropped silently (#242)

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:513-525` (new `fizzleMobFold` after `sendMobSpellFailed`), `:847-849` (TargetGone branch)
- Modify: `internal/hooks/NewRound_IdleMobs.go:68-69`
- Modify: `internal/hooks/spell_resolution.go:633-656` (`resolveMobSpell`)
- Test: `internal/hooks/spell_channel_sight_test.go` (imports, append)
- Docs: `internal/hooks/context.md:1756`

Runs after any other task that edits these three hooks files (see the plan's ordering notes); anchors are quoted text.

- [ ] **Step 1: Write the failing tests.** In `internal/hooks/spell_channel_sight_test.go`, add `"github.com/GoMudEngine/GoMud/internal/characters"` to the import block, directly before `"github.com/GoMudEngine/GoMud/internal/combatvocab"`. Then append to the end of the file:

```go

// foldingMobInRoom puts the test mob in room, mid-fold on a harm spell aimed
// at targetUserId, and in combat with that player, as a mob caster stands
// when its target walks out.
func foldingMobInRoom(t *testing.T, room *rooms.Room, targetUserId int) *mobs.Mob {
	t.Helper()
	m := spellChannelMob(t)
	rooms.LoadRoom(m.Character.RoomId).RemoveMob(m.InstanceId)
	room.AddMob(m.InstanceId)
	m.Character.Validate()
	m.Character.SetAggro(targetUserId, 0, characters.DefaultAttack)
	require.True(t, m.Character.IsInCombat(), "fixture: the mob fights its target")
	require.NoError(t, m.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-bolt", FoldsNeeded: 3, TargetUserIds: []int{targetUserId}},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	return m
}

// walkOut moves a player from room to room 1, as a target leaving mid-fold.
func walkOut(t *testing.T, room *rooms.Room, userId int) {
	t.Helper()
	room.RemovePlayer(userId)
	users.GetByUserId(userId).Character.RoomId = 1
	rooms.LoadRoom(1).AddPlayer(userId)
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
}

// #242: a mob's target who walked out is not "gone" to the fold step (only a
// dead or logged-out one is), so IdleMobs released the mob mid-fold: it left
// combat, the fold step never ran again, and the spell ended with no line at
// all. The fold now ends first, told as the TargetGone fizzle is: by sight to
// a reader who sees (a figure at shapes), by sound to one who sees nothing.
func TestMobFold_TargetWalksOut_FizzlesAtEachReadersSight(t *testing.T) {
	cases := []struct {
		name  string
		lamp  int
		eyes  int
		sight messaging.SightDecision
		want  string
	}{
		{"faces read the caster", 60, heatEyesConditionId, messaging.SightFull, "Skeleton's spell fizzles."},
		{"shapes read a figure", 10, heatEyesConditionId, messaging.SightShapes, "spell fizzles."},
		{"sees nothing, hears it", 0, nightEyesConditionId, messaging.SightNone, messaging.SoundSpellSputtersOut},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, tc.lamp, tc.eyes)
			m := foldingMobInRoom(t, room, 2)
			walkOut(t, room, 2)
			require.Equal(t, tc.sight, messaging.ParticipantSight(users.GetByUserId(1).Character, room))

			IdleMobs(events.NewRound{RoundNumber: 1})

			require.False(t, m.Character.IsCasting(), "the fold ends with the release")
			require.False(t, m.Character.IsInCombat(), "and the mob is released")
			got := drainPlain(1)
			require.Equal(t, 1, countContaining(got, tc.want), "%v", got)
			if tc.sight != messaging.SightFull {
				require.Zero(t, countContaining(got, "Skeleton"), "%v", got)
			}
			require.Empty(t, drainPlain(2), "the player who left reads nothing of the room")
		})
	}
}

// #242: the fold step can also complete the cast in the round the target
// walked out, before IdleMobs releases the mob. resolveMobSpell then found
// no target in the room, resolved nothing and said nothing.
func TestMobFold_CompletesWithNoTargetLeft_Fizzles(t *testing.T) {
	cases := []struct {
		name  string
		lamp  int
		eyes  int
		sight messaging.SightDecision
		want  string
	}{
		{"faces read the caster", 60, heatEyesConditionId, messaging.SightFull, "Skeleton's spell fizzles."},
		{"shapes read a figure", 10, heatEyesConditionId, messaging.SightShapes, "spell fizzles."},
		{"sees nothing, hears it", 0, nightEyesConditionId, messaging.SightNone, messaging.SoundSpellSputtersOut},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, tc.lamp, tc.eyes)
			m := spellChannelMob(t)
			walkOut(t, room, 2)
			require.Equal(t, tc.sight, messaging.ParticipantSight(users.GetByUserId(1).Character, room))

			landed := resolveMobSpell(m, activity.CastingData{SpellId: "test-bolt", TargetUserIds: []int{2}}, testHarmSpell(), room)

			require.False(t, landed)
			got := drainPlain(1)
			require.Equal(t, 1, countContaining(got, tc.want), "%v", got)
			if tc.sight != messaging.SightFull {
				require.Zero(t, countContaining(got, "Skeleton"), "%v", got)
			}
		})
	}
}
```

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./internal/hooks/ -run 'TestMobFold_' -count=1`
Expected: FAIL. Every `TargetWalksOut` subtest fails on "the fold ends with the release" (on master the released mob is left casting); every `CompletesWithNoTargetLeft` subtest fails on the line count, `expected: 1, actual: 0`.

- [ ] **Step 3: Implement the shared ending.** In `internal/hooks/NewRound_DoCombat_helpers.go`, insert before `// sendMobWeaving narrates a mob still holding its fold. Sight only: a quiet`:

```go
// fizzleMobFold ends a mob's fold whose target is gone and tells the room
// (#242): the casting is cleared (a no-op when the fold step already
// cleared it), the concentration failure is recorded against the fold's
// target, and the room reads "<caster>'s spell fizzles." by sight or hears
// the sputter. It is the one ending for every way a mob's target goes: the
// fold step's TargetGone (dead or logged out), IdleMobs releasing a mob whose
// target walked out mid-fold, and a fold that completes with no target left
// in the room (resolveMobSpell).
func fizzleMobFold(mob *mobs.Mob, room *rooms.Room, cs activity.CastingData) {
	clearCastingActivity(&mob.Character, activity.TriggerConcentrationBreak)
	recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(cs))
	sendMobSpellFailed(mob, room, "fizzles")
}

```

In the same file, in `handleMobFoldCasting`, replace

```go
	case result.TargetGone:
		recordConcentrationFailure(combat.Mob, combat.User, &mob.Character, castingTargetChar(csBeforeProcess))
		sendMobSpellFailed(mob, mobRoom, "fizzles")
```

with

```go
	case result.TargetGone:
		fizzleMobFold(mob, mobRoom, csBeforeProcess)
```

(The player branch at `:633` reads `case result.TargetGone:` too; the replacement above is the one followed by `combat.Mob, combat.User`.)

- [ ] **Step 4: End the fold before the release.** In `internal/hooks/NewRound_IdleMobs.go`, replace

```go
				if user == nil || user.Character.RoomId != mob.Character.RoomId {
					mob.Command(`emote mumbles about losing their quarry.`)
```

with

```go
				if user == nil || user.Character.RoomId != mob.Character.RoomId {
					// A fold in progress ends first (#242): released, the mob
					// leaves combat, the fold step never runs for it again,
					// and the spell would hang unspoken.
					if cs, ok := mob.Character.CastingData(); ok {
						if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
							fizzleMobFold(mob, room, cs)
						}
					}
					mob.Command(`emote mumbles about losing their quarry.`)
```

(`rooms` is already imported there.)

- [ ] **Step 5: A completed fold with no target left fizzles.** In `internal/hooks/spell_resolution.go`, in `resolveMobSpell`, replace the two target loops and the return:

```go
	for _, mobInstId := range cs.TargetMobInstanceIds {
		if mobInstId == mob.InstanceId {
			// MS: the caster is its own target. Only a help spell puts a mob
			// in its own list (a HelpSingle with no target, or its own place
			// in an area help), and it takes the same uncontested step and
			// appliers as every other pairing. A mob never harms itself.
			if !spellData.IsHarm() {
				self := actions.NewMobActorInRoom(mob, room)
				anyLanded = resolveHelpSpell(newSpellEffectCtx(&mob.Character, self, self, room, spellData,
					magnitude, uncontestedSpellResult())) || anyLanded
			}
			continue
		}
		if target := mobs.GetInstance(mobInstId); target != nil && target.Character.Health > 0 && target.Character.RoomId == room.RoomId {
			anyLanded = resolveMobSpellAgainstMob(mob, target, room, spellData, side, magnitude) || anyLanded
		}
	}
	for _, userId := range cs.TargetUserIds {
		if target := users.GetByUserId(userId); target != nil && target.Character.RoomId == room.RoomId {
			anyLanded = resolveMobSpellAgainstPlayer(mob, target, room, spellData, side, magnitude) || anyLanded
		}
	}

	return anyLanded
```

with

```go
	// found counts the targets still here to resolve against. With none, the
	// fold completed on a room its every target had left (#242): it fizzles
	// aloud, as the fold step's TargetGone does, instead of resolving nothing
	// and saying nothing.
	found := 0
	for _, mobInstId := range cs.TargetMobInstanceIds {
		if mobInstId == mob.InstanceId {
			// MS: the caster is its own target. Only a help spell puts a mob
			// in its own list (a HelpSingle with no target, or its own place
			// in an area help), and it takes the same uncontested step and
			// appliers as every other pairing. A mob never harms itself.
			found++
			if !spellData.IsHarm() {
				self := actions.NewMobActorInRoom(mob, room)
				anyLanded = resolveHelpSpell(newSpellEffectCtx(&mob.Character, self, self, room, spellData,
					magnitude, uncontestedSpellResult())) || anyLanded
			}
			continue
		}
		if target := mobs.GetInstance(mobInstId); target != nil && target.Character.Health > 0 && target.Character.RoomId == room.RoomId {
			found++
			anyLanded = resolveMobSpellAgainstMob(mob, target, room, spellData, side, magnitude) || anyLanded
		}
	}
	for _, userId := range cs.TargetUserIds {
		if target := users.GetByUserId(userId); target != nil && target.Character.RoomId == room.RoomId {
			found++
			anyLanded = resolveMobSpellAgainstPlayer(mob, target, room, spellData, side, magnitude) || anyLanded
		}
	}
	if found == 0 {
		fizzleMobFold(mob, room, cs)
	}

	return anyLanded
```

- [ ] **Step 6: Run them and see them pass, then the package and the guards.**

Run: `go test ./internal/hooks/ -run 'TestMobFold_|TestMobSpell|TestPlayerSpell|TestMobDrainArea' -count=1`
Expected: PASS.

Run: `go test ./internal/hooks/ ./internal/mobcommands/ ./internal/behaviortree/ -count=1`
Expected: every package `ok`.

Run: `go test . -count=1`
Expected: `ok`. (Dry run: no guard tripped; no root guard keys a line in these three files.)

- [ ] **Step 7: Docs.** In `internal/hooks/context.md`, insert before the line `Cross-references:` (the one directly after the "Layered disruption" paragraph of "Chunk 4f"):

```
**Target gone, mob caster (#242):** `fizzleMobFold(mob, room, cs)`
(`NewRound_DoCombat_helpers.go`) is the one ending for a mob fold whose target
is gone: it clears the cast, records the concentration failure and calls
`sendMobSpellFailed(mob, room, "fizzles")` (seen, or heard as
`messaging.SoundSpellSputtersOut`). Three paths reach it: the fold step's
`TargetGone` (a dead or logged-out target), `IdleMobs` releasing a mob whose
target walked out mid-fold (the fold ends before the release; outside combat
no fold step would run again), and `resolveMobSpell` completing with no target
left in the room.

```

- [ ] **Step 8: Format and commit.**

Run: `gofmt -l internal/hooks` (expect no output).

```bash
git add internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/NewRound_IdleMobs.go internal/hooks/spell_resolution.go internal/hooks/spell_channel_sight_test.go internal/hooks/context.md
git commit -m "fix(spells): a mob fold whose target walks out fizzles aloud (#242)

IdleMobs released a mob mid-fold when its target left the room, leaving the
cast hanging with no line; a fold that completed on an empty room said
nothing. Both now end through fizzleMobFold, shared with TargetGone.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: H6, The scout's scan verified (#251)

**Files:**
- Create: `internal/behaviortree/actions_scout_sight_test.go`

No production change: the dry run found these pass on master. A test that passes first is expected here, so Step 2 proves the tests can fail by a temporary mutation instead.

- [ ] **Step 1: Write the tests.** Create `internal/behaviortree/actions_scout_sight_test.go`:

```go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #251: a scout tree's try_scan (actTryScan) promotes the first hostile
// sighting of actions.Scan to its SoftTarget, and every player is hostile
// (isHostileToMob). Scan's structured result follows the scanner's sight
// (scanReach, then listedOccupants); these tests drive the btree action
// itself, so a scout cannot pick a target it cannot make out.
const (
	scoutNextRoomId = 8102
	scoutPlayerId   = 8160
)

// scoutScanScene is sightScene's scout, standing in the lit room 8100 (city,
// lamp 90) with Night Vision as a goblin scout has, beside room 8102 (cave,
// lamp nextLamp) to the north, which holds one player and nobody else.
func scoutScanScene(t *testing.T, nextLamp int) (*EvalContext, *users.UserRecord) {
	t.Helper()
	m, here := sightScene(t, "city")
	require.NoError(t, m.Character.AddCondition(sightNightVisionConditionId, true))
	here.Exits = map[string]exit.RoomExit{"north": {RoomId: scoutNextRoomId}}
	next := &rooms.Room{RoomId: scoutNextRoomId, Biome: "cave", Lamp: rooms.LampPtr(nextLamp),
		Exits: map[string]exit.RoomExit{"south": {RoomId: here.RoomId}}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{here.RoomId: here, scoutNextRoomId: next},
		map[string]*rooms.ZoneConfig{}))
	require.Equal(t, nextLamp, next.LightLevel(), "fixture: the next room's light is pinned")

	u := users.NewTestUser(scoutPlayerId, "kesh", "Kesh", 98160)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{scoutPlayerId: u}))
	u.Character.RoomId = scoutNextRoomId
	next.AddPlayer(scoutPlayerId)

	return &EvalContext{InstanceId: m.InstanceId, RoomId: here.RoomId}, u
}

// The control: a visible player in a lit next room is promoted. Without it
// the two refusals below could pass on a scene the scan never reaches.
func TestActTryScan_PromotesAVisiblePlayerInALitRoom(t *testing.T) {
	ctx, u := scoutScanScene(t, 90)
	require.Equal(t, Success, actTryScan(map[string]any{}, ctx))
	require.Equal(t, state.ActorRef{UserId: u.UserId}, ctx.SoftTarget)
}

// A visible, unlit player in a pitch-dark next room is out of the scout's
// sight: Night Vision shifts the window but cannot lift light 0.
func TestActTryScan_IgnoresAPlayerInAPitchDarkRoom(t *testing.T) {
	ctx, _ := scoutScanScene(t, 0)
	require.Equal(t, Failure, actTryScan(map[string]any{}, ctx))
	require.True(t, ctx.SoftTarget.IsZero(), "no SoftTarget for a player the scout cannot see")
}

// A sneak-hidden player in a lit next room is never a sighting: the scout
// has no see-hidden, so it does not perceive them.
func TestActTryScan_IgnoresAHiddenPlayerInALitRoom(t *testing.T) {
	ctx, u := scoutScanScene(t, 90)
	hideForTest(t, u.Character)
	require.Equal(t, Failure, actTryScan(map[string]any{}, ctx))
	require.True(t, ctx.SoftTarget.IsZero(), "no SoftTarget for a player the scout does not perceive")
}
```

- [ ] **Step 2: Run them, then prove they can fail.**

Run: `go test ./internal/behaviortree/ -run 'TestActTryScan_' -count=1 -v`
Expected: all three PASS.

Prove the two refusals can fail (temporary; reverted in the same step). In `internal/actions/scan.go`, change `adjRoom); sight != messaging.SightNone {` to `adjRoom); sight != messaging.SightNone || true {`, and in `listedOccupants` change `return viewer.Perceives(c)` to `return viewer.Perceives(c) || true`. Run the same command.
Expected: `TestActTryScan_IgnoresAPlayerInAPitchDarkRoom` and `TestActTryScan_IgnoresAHiddenPlayerInALitRoom` FAIL; the control passes.

Revert: `git checkout -- internal/actions/scan.go`, then `git status --short internal/actions/scan.go` prints nothing.

If any test fails WITHOUT the mutation, that is a real #251 leak: stop and report it; the fix becomes its own task.

- [ ] **Step 3: Package and guards.**

Run: `go test ./internal/behaviortree/ -count=1` and `go test . -count=1`
Expected: `ok` for both.

- [ ] **Step 4: Format and commit.**

Run: `gofmt -l internal/behaviortree` (expect no output).

```bash
git add internal/behaviortree/actions_scout_sight_test.go
git commit -m "test(scout): try_scan ignores the dark and the hidden (#251)

Drives actTryScan: a visible player in a lit next room is promoted (the
control); an unlit player in a pitch-dark room and a sneak-hidden player in a
lit one are not.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: H7a, Map legend tags slugged once, in one pass (#455)

**Files:**
- Create: `internal/mapper/maptag.go`, `internal/mapper/maptag_test.go`, `internal/templates/mapslug_func_test.go`
- Modify: `internal/usercommands/look.go:700-705`, `internal/usercommands/skill.map.go:3-6`, `:176-179`, `internal/templates/templatesfunctions.go:18`, `:80-82`
- Modify: `_datafiles/world/dogmud/templates/maps/map.template:20`, `_datafiles/world/default/templates/maps/map.template:20`, `_datafiles/world/dogmud/ansi-aliases.yaml:43-45`, `:57`, `_datafiles/world/default/ansi-aliases.yaml:34`
- Docs: `internal/mapper/context.md:19`, `:90`

- [ ] **Step 1: Write the failing tests.** Create `internal/mapper/maptag_test.go`:

```go
package mapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// #455: "Deep Water" put a space inside fg="map-deep water", the tag parser
// gave up, and the raw tag printed as text on every look.
func TestLegendSlug_SpacesBecomeHyphens(t *testing.T) {
	assert.Equal(t, "deep-water", LegendSlug("Deep Water"))
	assert.Equal(t, "city-thoroughfare", LegendSlug("City Thoroughfare"))
	assert.Equal(t, "cave", LegendSlug("Cave"))
}

func TestColorizeLegendLine_DeepWaterTagHasNoSpace(t *testing.T) {
	got := ColorizeLegendLine("║≈─@║", map[rune]string{'≈': "Deep Water", '@': "You"})
	assert.Equal(t,
		`║<ansi fg="map-room"><ansi fg="map-deep-water" bg="mapbg-deep-water">≈</ansi></ansi>─`+
			`<ansi fg="map-room"><ansi fg="map-you" bg="mapbg-you">@</ansi></ansi>║`, got)
}

// One pass, rune by rune: a legend symbol that also occurs inside a tag
// already written ('m' is in "map-room") is never rewritten. The old
// per-symbol strings.Replace loop rewrote it whenever map order put the
// letter's pass after the other symbol's.
func TestColorizeLegendLine_NeverRewritesInsideATag(t *testing.T) {
	legend := map[rune]string{'≈': "Deep Water", 'm': "Mine"}
	for i := 0; i < 20; i++ {
		got := ColorizeLegendLine("≈m", legend)
		assert.Equal(t,
			`<ansi fg="map-room"><ansi fg="map-deep-water" bg="mapbg-deep-water">≈</ansi></ansi>`+
				`<ansi fg="map-room"><ansi fg="map-mine" bg="mapbg-mine">m</ansi></ansi>`, got)
	}
}

// Every shipped multi-word biome has a colour alias under its slug, so the
// hyphenated tag still colours the symbol.
func TestMultiWordBiomesHaveSlugAliases(t *testing.T) {
	for _, world := range []string{"default", "dogmud"} {
		root := filepath.Join("..", "..", "_datafiles", "world", world)
		raw, err := os.ReadFile(filepath.Join(root, "ansi-aliases.yaml"))
		require.NoError(t, err)
		var aliases struct {
			Colors map[string]any `yaml:"colors"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &aliases))
		require.NotEmpty(t, aliases.Colors, world)

		files, err := filepath.Glob(filepath.Join(root, "biomes", "*.yaml"))
		require.NoError(t, err)
		require.NotEmpty(t, files, world)
		for _, f := range files {
			b, err := os.ReadFile(f)
			require.NoError(t, err)
			var biome struct {
				Name string `yaml:"name"`
			}
			require.NoError(t, yaml.Unmarshal(b, &biome))
			if !strings.Contains(biome.Name, " ") {
				continue
			}
			_, ok := aliases.Colors["map-"+LegendSlug(biome.Name)]
			assert.True(t, ok, "%s: biome %q has no map-%s alias", world, biome.Name, LegendSlug(biome.Name))
		}
	}
}
```

Create `internal/templates/mapslug_func_test.go`:

```go
package templates

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #455: maps/map.template builds the legend's colour tag with mapslug, the
// same slug the minimap and the map command use, so "Deep Water" is
// map-deep-water there too and not the unparseable "map-deep water".
func TestMapSlugFuncHyphenatesTheLegendName(t *testing.T) {
	tpl, err := template.New("legend").Funcs(funcMap).Parse(`fg="map-{{mapslug .}}"`)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, "Deep Water"))
	assert.Equal(t, `fg="map-deep-water"`, buf.String())
}
```

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./internal/mapper/ -run 'LegendSlug|ColorizeLegendLine|MultiWordBiomes' -count=1`
Expected: build failure, `undefined: ColorizeLegendLine` and `undefined: LegendSlug`.

Run: `go test ./internal/templates/ -run MapSlug -count=1`
Expected: FAIL, `function "mapslug" not defined`.

- [ ] **Step 3: Implement the builder.** Create `internal/mapper/maptag.go`:

```go
package mapper

import (
	"fmt"
	"strings"
)

// LegendSlug turns a map legend name into the slug its colour aliases are
// keyed by: lower case, every space a hyphen ("Deep Water" is "deep-water").
// A space inside fg="map-..." stops the tag parser, and the raw tag then
// prints as text (#455). The room minimap (look), the map command and the
// map legend template (templates funcMap "mapslug") all use this one slug.
func LegendSlug(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

// ColorizeLegendLine wraps every legend symbol on one rendered map line in
// its colour tags. It walks the line once, rune by rune, so a symbol that
// also occurs inside a tag already written is never rewritten (a
// strings.Replace per symbol rewrote it whenever map order put that symbol's
// pass second).
func ColorizeLegendLine(line string, legend map[rune]string) string {
	var b strings.Builder
	for _, r := range line {
		name, ok := legend[r]
		if !ok {
			b.WriteRune(r)
			continue
		}
		slug := LegendSlug(name)
		fmt.Fprintf(&b, `<ansi fg="map-room"><ansi fg="map-%s" bg="mapbg-%s">%c</ansi></ansi>`, slug, slug, r)
	}
	return b.String()
}
```

- [ ] **Step 4: Use it in `look` and `map`.** In `internal/usercommands/look.go`, replace

```go
			for i := 1; i <= c.Height; i++ {
				for sym, txtLegend := range legend {
					txtLc := strings.ToLower(txtLegend)
					tinyMap[i] = strings.Replace(tinyMap[i], string(sym), fmt.Sprintf(`<ansi fg="map-room"><ansi fg="map-%s" bg="mapbg-%s">%c</ansi></ansi>`, txtLc, txtLc, sym), -1)
				}
			}
```

with

```go
			for i := 1; i <= c.Height; i++ {
				tinyMap[i] = mapper.ColorizeLegendLine(tinyMap[i], legend)
			}
```

(`look.go` still uses `fmt` and `strings` elsewhere; leave its imports.) In `internal/usercommands/skill.map.go`, replace

```go
		for sym, txtLegend := range legend {
			txtLc := strings.ToLower(txtLegend)
			displayLines[i] = strings.Replace(displayLines[i], string(sym), fmt.Sprintf(`<ansi fg="map-room"><ansi fg="map-%s" bg="mapbg-%s">%c</ansi></ansi>`, txtLc, txtLc, sym), -1)
		}
```

with

```go
		displayLines[i] = mapper.ColorizeLegendLine(displayLines[i], legend)
```

and delete the now unused `"strings"` line from its import block (`fmt` stays: line 43 uses it).

- [ ] **Step 5: The template function and the templates.** In `internal/templates/templatesfunctions.go`, add `"github.com/GoMudEngine/GoMud/internal/mapper"` to the import block after the `language` import, and after the `"lowercase"` entry of `funcMap` add:

```go
		// The map legend's colour slug, shared with the minimap and the
		// map command so the legend's tag matches theirs (#455).
		"mapslug": mapper.LegendSlug,
```

In BOTH `_datafiles/world/dogmud/templates/maps/map.template` and `_datafiles/world/default/templates/maps/map.template`, line 20, replace

```
<ansi fg="map-{{lowercase $name}}" bg="mapbg-{{lowercase $name}}">
```

with

```
<ansi fg="map-{{mapslug $name}}" bg="mapbg-{{mapslug $name}}">
```

- [ ] **Step 6: The aliases.** In `_datafiles/world/dogmud/ansi-aliases.yaml` replace

```yaml
  map-water: 12 # Bright blue
  map-shore: 12 # Bright blue
  map-forest: 2
```

with

```yaml
  map-water: 12 # Bright blue
  map-deep-water: 12 # Bright blue; legend "Deep Water" (mapper.LegendSlug)
  map-shore: 12 # Bright blue
  map-forest: 2
  map-dense-forest: 2 # legend "Dense Forest"
```

and after `  map-city: 15 # Bright white` add

```yaml
  map-city-backstreet: 15 # Bright white; legend "City Backstreet"
  map-city-thoroughfare: 15 # Bright white; legend "City Thoroughfare"
```

In `_datafiles/world/default/ansi-aliases.yaml`, after `  map-water: 12 # Bright blue` add

```yaml
  map-deep-water: 12 # Bright blue; legend "Deep Water" (mapper.LegendSlug)
```

(Both alias files are CRLF in the working tree and LF in the index; the Edit tool keeps the file's endings.)

- [ ] **Step 7: Run the tests and see them pass.**

Run: `go test ./internal/mapper/ ./internal/templates/ ./internal/usercommands/ -count=1`
Expected: `ok` for all three.

Run: `go test . -count=1`
Expected: `ok`.

- [ ] **Step 8: context.md.** In `internal/mapper/context.md`, in the "Files" list, after the `mapper.map.go` bullet, add:

```markdown
- **maptag.go**: `LegendSlug` and `ColorizeLegendLine`, the one colour-tag
  builder for a rendered map line (minimap, `map`, legend template).
```

and in the Rendering code block after `func (c *Config) OverrideSymbol(roomId int, symbol rune, legend string)` add the two signatures, then this paragraph directly after the block's closing fence (before the existing `GetLimitedMap` paragraph):

```go
func LegendSlug(name string) string
func ColorizeLegendLine(line string, legend map[rune]string) string
```

```markdown
A legend name becomes a colour tag through `LegendSlug` only: lower case,
spaces as hyphens (`map-deep-water`). A space inside `fg="..."` stops the tag
parser and prints the tag as text (#455). `ColorizeLegendLine` walks a line
once, rune by rune, so a symbol inside a tag it already wrote is never
rewritten. `usercommands/look.go` (minimap), `usercommands/skill.map.go` and
the `mapslug` template function in `maps/map.template` all use it. A new
multi-word biome needs a `map-<slug>` alias in `ansi-aliases.yaml`
(`TestMultiWordBiomesHaveSlugAliases`).
```

- [ ] **Step 9: Format and commit.**

Run: `gofmt -l internal/mapper internal/usercommands internal/templates` (expect no output)

```bash
git add internal/mapper/maptag.go internal/mapper/maptag_test.go internal/mapper/context.md internal/usercommands/look.go internal/usercommands/skill.map.go internal/templates/templatesfunctions.go internal/templates/mapslug_func_test.go _datafiles/world/dogmud/templates/maps/map.template _datafiles/world/default/templates/maps/map.template _datafiles/world/dogmud/ansi-aliases.yaml _datafiles/world/default/ansi-aliases.yaml
git commit -m "fix(map): one slugged legend tag builder for look, map and the legend (#455)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: H7b, Mob emotes end with a full stop (#455)

**Files:**
- Modify: `internal/messaging/normalize.go:26-31`
- Create: `internal/messaging/mob_emote_punct_test.go`
- Docs: `internal/messaging/context.md:17` (after the a/an paragraph of stage 2)

- [ ] **Step 1: Write the failing test.** Create `internal/messaging/mob_emote_punct_test.go`:

```go
package messaging

import "testing"

// #455: shipped idle emotes ("emote stares at the ground between his
// feet") print with no full stop. CategoryMobEmote keeps its own casing,
// articles and words, and takes only the end-punctuation stage.
func TestNormalizeMobEmoteGetsOnlyEndPunctuation(t *testing.T) {
	in := `<ansi fg="mobname">Beggar Oswin</ansi> <ansi fg="137">stares at the the ground between his feet</ansi>`
	want := `<ansi fg="mobname">Beggar Oswin</ansi> <ansi fg="137">stares at the the ground between his feet.</ansi>`
	if got := Normalize(CategoryMobEmote, in); got != want {
		t.Errorf("mob emote: got %q, want %q", got, want)
	}
	// Lower-case start and "a" before a vowel stay as authored.
	if got := Normalize(CategoryMobEmote, "a eel twitches"); got != "a eel twitches." {
		t.Errorf("mob emote must skip every stage but end punctuation, got %q", got)
	}
	// Idempotent, and an authored stop is not doubled.
	if got := Normalize(CategoryMobEmote, "It nods."); got != "It nods." {
		t.Errorf("mob emote double-punctuated: %q", got)
	}
}

// A player types their own emote: the server never rewrites it.
func TestNormalizePlayerEmoteStaysExempt(t *testing.T) {
	in := `<ansi fg="username">Ordel</ansi> <ansi fg="137">waves</ansi>`
	if got := Normalize(CategoryEmote, in); got != in {
		t.Errorf("player emote rewritten: %q", got)
	}
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/messaging/ -run 'MobEmoteGetsOnly|PlayerEmoteStays' -count=1`
Expected: FAIL, `mob emote must skip every stage but end punctuation, got "a eel twitches"` (and the first case, missing stop).

- [ ] **Step 3: Implement.** In `internal/messaging/normalize.go`, replace

```go
	switch cat {
	case CategoryRoomDescription, CategoryRoomEntry, CategoryRoomExit,
		CategoryWeather, CategoryTimeOfDay, CategoryLight, CategorySplash,
		CategoryNPCDialogue, CategoryDialogueHint,
		CategoryMobIdle, CategoryMobEmote,
```

with

```go
	switch cat {
	case CategoryMobEmote:
		// Authored mob prose keeps its casing, articles and words, but a
		// sentence still ends: 700+ shipped idle emotes were written with
		// no full stop and printed that way (#455). Player-typed emotes
		// ride CategoryEmote, which stays fully exempt below.
		return stageCapitalize | stageAAnAgreement | stageDupWordCollapse |
			stageNameCanon
	case CategoryRoomDescription, CategoryRoomEntry, CategoryRoomExit,
		CategoryWeather, CategoryTimeOfDay, CategoryLight, CategorySplash,
		CategoryNPCDialogue, CategoryDialogueHint,
		CategoryMobIdle,
```

- [ ] **Step 4: Run the tests and see them pass.**

Run: `go test ./internal/messaging/ ./internal/actions/ ./internal/mobcommands/ ./internal/behaviortree/ ./internal/hooks/ ./internal/caravan/ ./internal/ferry/ -count=1`
Expected: all `ok` (`actions` takes about a minute).

Run: `go test . -count=1`
Expected: `ok` (`wrap_live_render_golden_test.go` renders `CategoryMobEmote` lines and still passes).

- [ ] **Step 5: context.md.** In `internal/messaging/context.md`, stage 2 of "Pipeline Stages", after ``not the sound, so "a useful" becomes "an useful"; a known limitation.`` add:

```markdown
   `CategoryMobEmote` skips every stage except sentence-end punctuation,
   so authored mob emotes keep their own words but end with a stop
   (#455); the player-typed `CategoryEmote` skips every stage.
```

- [ ] **Step 6: Commit.**

```bash
git add internal/messaging/normalize.go internal/messaging/mob_emote_punct_test.go internal/messaging/context.md
git commit -m "fix(messaging): mob emotes take the end-punctuation stage (#455)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 15: H7c, Natural-weapon lines never make the weapon a singular subject (#455)

**Files:**
- Modify: `_datafiles/world/dogmud/combat-messages/claws.yaml:173, 230, 247, 305, 329, 353, 380, 385, 396, 402, 403, 424, 425`, `_datafiles/world/dogmud/combat-messages/bite.yaml:173`
- Create: `internal/items/natural_weapon_agreement_test.go`
- Regenerate: `internal/narration/testdata/stores/combat_messages.golden`

- [ ] **Step 1: Write the failing test.** Create `internal/items/natural_weapon_agreement_test.go`:

```go
package items

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// naturalWeaponSubjectVerb finds {itemname} used as the subject of a
// singular verb: the next word, through one colour tag and any capitalised
// adverb, ends in "s" ("bites", "CRITICALLY EVISCERATES").
var naturalWeaponSubjectVerb = regexp.MustCompile(`\{itemname\}</ansi> (?:<ansi fg="[^"]*">)?(?:[A-Z]+ )*[A-Za-z]+[sS]\b`)

// #455: a mob's natural weapon name is the species' unarmed name, mostly
// plural ("claws", "fangs", "jaws"), so "A figure's claws bites into you!"
// The claws and bite pools never make {itemname} the subject of a singular
// verb; the attacker does the verb, "with their {itemname}".
func TestNaturalWeaponPoolsNeverPutASingularVerbAfterTheWeapon(t *testing.T) {
	for _, name := range []string{"claws.yaml", "bite.yaml"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "combat-messages", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if m := naturalWeaponSubjectVerb.FindString(line); m != "" {
				t.Errorf("%s:%d: %q makes the weapon the subject of a singular verb", name, i+1, m)
			}
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/items/ -run NaturalWeaponPools -count=1`
Expected: FAIL with 14 errors: `claws.yaml` lines 173, 230, 247, 305, 329, 353, 380, 385, 396, 402, 403, 424, 425 and `bite.yaml:173`.

- [ ] **Step 3: Reword the lines.** Each is one Edit, old line to new line (the leading `        - ` indentation is unchanged). `claws.yaml`:

173 old: `'Your <ansi fg="item">{itemname}</ansi> grazes <ansi fg="{acteetype}">{actee}</ansi>.'`
173 new: `'You graze <ansi fg="{acteetype}">{actee}</ansi> with your <ansi fg="item">{itemname}</ansi>.'`

230 old: `'Your <ansi fg="item">{itemname}</ansi> rends <ansi fg="{acteetype}">{actee}</ansi>!'`
230 new: `'You rend <ansi fg="{acteetype}">{actee}</ansi> with your <ansi fg="item">{itemname}</ansi>!'`

247 old: `'Your <ansi fg="item">{itemname}</ansi> flashes as you maintain the pressure with masterful control!'`
247 new: `'You flash your <ansi fg="item">{itemname}</ansi> and keep up the pressure with masterful control!'`

305 old: `'Your <ansi fg="item">{itemname}</ansi> bites into <ansi fg="{acteetype}">{actee}</ansi>!'`
305 new: `'You sink your <ansi fg="item">{itemname}</ansi> into <ansi fg="{acteetype}">{actee}</ansi>!'`

329 old: `'<ansi fg="{actortype}">{actor}''s</ansi> <ansi fg="item">{itemname}</ansi> bites into you!'`
329 new: `'<ansi fg="{actortype}">{actor}</ansi> sinks their <ansi fg="item">{itemname}</ansi> into you!'`

353 old: `'<ansi fg="{actortype}">{actor}''s</ansi> <ansi fg="item">{itemname}</ansi> bites into <ansi fg="{acteetype}">{actee}</ansi>!'`
353 new: `'<ansi fg="{actortype}">{actor}</ansi> sinks their <ansi fg="item">{itemname}</ansi> into <ansi fg="{acteetype}">{actee}</ansi>!'`

380 old: `'Your <ansi fg="item">{itemname}</ansi> <ansi fg="cyan-bold">CRITICALLY EVISCERATES</ansi> <ansi fg="{acteetype}">{actee}</ansi>!'`
380 new: `'You <ansi fg="cyan-bold">CRITICALLY EVISCERATE</ansi> <ansi fg="{acteetype}">{actee}</ansi> with your <ansi fg="item">{itemname}</ansi>!'`

385 old: `'You whirl and your <ansi fg="item">{itemname}</ansi> <ansi fg="cyan-bold">SHREDS</ansi> <ansi fg="{acteetype}">{actee}</ansi>!'`
385 new: `'You whirl and <ansi fg="cyan-bold">SHRED</ansi> <ansi fg="{acteetype}">{actee}</ansi> with your <ansi fg="item">{itemname}</ansi>!'`

396 old: `'Your positioning is flawless - your <ansi fg="item">{itemname}</ansi> <ansi fg="cyan-bold">OBLITERATES</ansi> <ansi fg="{acteetype}">{actee}</ansi>!'`
396 new: `'Your positioning is flawless, and you <ansi fg="cyan-bold">OBLITERATE</ansi> <ansi fg="{acteetype}">{actee}</ansi> with your <ansi fg="item">{itemname}</ansi>!'`

402 old: `'<ansi fg="{actortype}">{actor}''s</ansi> <ansi fg="item">{itemname}</ansi> <ansi fg="cyan-bold">CRITICALLY EVISCERATES</ansi> you!'`
402 new: `'<ansi fg="{actortype}">{actor}</ansi> <ansi fg="cyan-bold">CRITICALLY EVISCERATES</ansi> you with their <ansi fg="item">{itemname}</ansi>!'`

403 old: `'<ansi fg="{actortype}">{actor}''s</ansi> <ansi fg="item">{itemname}</ansi> delivers a <ansi fg="cyan-bold">CRITICAL RAKE</ansi>!'`
403 new: `'<ansi fg="{actortype}">{actor}</ansi> delivers a <ansi fg="cyan-bold">CRITICAL RAKE</ansi> with their <ansi fg="item">{itemname}</ansi>!'`

424 old: `'<ansi fg="{actortype}">{actor}''s</ansi> <ansi fg="item">{itemname}</ansi> <ansi fg="cyan-bold">CRITICALLY EVISCERATES</ansi> <ansi fg="{acteetype}">{actee}</ansi>!'`
424 new: `'<ansi fg="{actortype}">{actor}</ansi> <ansi fg="cyan-bold">CRITICALLY EVISCERATES</ansi> <ansi fg="{acteetype}">{actee}</ansi> with their <ansi fg="item">{itemname}</ansi>!'`

425 old: `'<ansi fg="{actortype}">{actor}''s</ansi> <ansi fg="item">{itemname}</ansi> delivers a <ansi fg="cyan-bold">CRITICAL RAKE</ansi> to <ansi fg="{acteetype}">{actee}</ansi>!'`
425 new: `'<ansi fg="{actortype}">{actor}</ansi> delivers a <ansi fg="cyan-bold">CRITICAL RAKE</ansi> to <ansi fg="{acteetype}">{actee}</ansi> with their <ansi fg="item">{itemname}</ansi>!'`

`bite.yaml`:

173 old: `'Your <ansi fg="item">{itemname}</ansi> grazes <ansi fg="{acteetype}">{actee}</ansi>.'`
173 new: `'You graze <ansi fg="{acteetype}">{actee}</ansi> with your <ansi fg="item">{itemname}</ansi>.'`

(The old 329, 353, 402, 424 and 380 strings each occur once in their file; 173's old text occurs once in each file. If an Edit reports a non-unique match, include the line above as context.)

- [ ] **Step 4: Run the test and see it pass; regenerate the golden.**

Run: `go test ./internal/items/ -count=1`
Expected: `ok`.

Run: `go test ./internal/narration/ -count=1`
Expected: FAIL, `TestSnapshotStores/combat_messages` golden mismatch at `bite|weak|together|beginner|0` (this guard trips by design on a content change).

Run: `go test ./internal/narration/ -run TestSnapshotStores -update -count=1`, then `git diff --stat internal/narration`
Expected: `combat_messages.golden | 24 +++++-----`, and `git diff internal/narration` shows only `bite|weak|...`, `claws|weak|...`, `claws|normal|...`, `claws|heavy|...`, `claws|critical|...` and the three `derived|bite|weak|actor` rows, each moving from the old text to the new. Anything else in the diff is a finding: stop.

Run: `go test ./internal/narration/ ./internal/combat/ ./internal/messaging/ -count=1` and `go test . -count=1`
Expected: all `ok`.

- [ ] **Step 5: Commit.**

```bash
git add _datafiles/world/dogmud/combat-messages/claws.yaml _datafiles/world/dogmud/combat-messages/bite.yaml internal/items/natural_weapon_agreement_test.go internal/narration/testdata/stores/combat_messages.golden
git commit -m "content(combat): claws and bite lines never put a singular verb after the weapon (#455)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 16: H7d, Description modifiers join the description before the wrap (#455)

**Files:**
- Modify: `internal/rooms/roomdetails.go:17` (import), `:105-110`, `:139`, `:185-215`, end of file
- Create: `internal/rooms/roomdetails_modifier_wrap_test.go`
- Docs: `internal/rooms/context.md:125`

- [ ] **Step 1: Write the failing test.** Create `internal/rooms/roomdetails_modifier_wrap_test.go`:

```go
package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The shipped sanctuary text (mutators/sanctuary.yaml), 117 columns on one
// line.
const modifierWrapSanctuaryText = "A peace older than the stones themselves settles over you here. Wounds close more easily and breath comes more deeply."

// modifierWrapRoom is room 1 of seedRegistry under an appended description
// modifier, with a viewer standing in it.
func modifierWrapRoom(t *testing.T) (*Room, *users.UserRecord) {
	t.Helper()
	t.Cleanup(seedRegistry())
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId: "test-wrap-sanctuary",
		DescriptionModifier: &mutators.TextModifier{
			Behavior: mutators.TextAppend,
			Text:     modifierWrapSanctuaryText,
		},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7491: users.NewTestUser(7491, "wrapper", "Wrapper", 97491),
	}))
	GetZoneConfig("TestZone").Mutators.Add("test-wrap-sanctuary")
	return roomManager.rooms[1], users.GetByUserId(7491)
}

// #455: a mutator's description modifier was appended after the
// description had been wrapped, so the Mending Hut printed its sanctuary
// line as one 117-column line. It now joins the description before the wrap.
func TestGetDetails_DescriptionModifierIsWrapped(t *testing.T) {
	r, viewer := modifierWrapRoom(t)

	desc := GetDetails(r, viewer).Description
	if !strings.Contains(desc, "A peace older than the stones") || !strings.Contains(desc, "comes more deeply.") {
		t.Fatalf("the modifier text is missing from the description: %q", desc)
	}
	for _, line := range strings.Split(desc, "\n") {
		if w := util.VisibleWidth(strings.TrimRight(line, "\r")); w > 80 {
			t.Errorf("description line is %d columns, want <= 80: %q", w, line)
		}
	}
}

// With the minimap beside it, the modifier wraps to the description column
// like the rest, so no line runs past the map.
func TestGetDetails_DescriptionModifierWrapsBesideTheMinimap(t *testing.T) {
	r, viewer := modifierWrapRoom(t)
	tinymap := []string{"╔═════╗", "║.....║", "║.....║", "║..@..║", "║.....║", "║.....║", "╚═════╝"}

	desc := GetDetails(r, viewer, tinymap).Description
	if !strings.Contains(desc, "A peace older than the stones") {
		t.Fatalf("the modifier text is missing from the description: %q", desc)
	}
	for _, line := range strings.Split(desc, "\n") {
		if w := util.VisibleWidth(strings.TrimRight(line, "\r")); w > 80 {
			t.Errorf("description line is %d columns, want <= 80: %q", w, line)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/rooms/ -run DescriptionModifier -count=1`
Expected: FAIL in both tests, `description line is 118 columns, want <= 80: "A peace older than the stones ..."`.

- [ ] **Step 3: Implement.** In `internal/rooms/roomdetails.go`, replace

```go
	renderNouns := true

	if len(tinymap) > 0 {
		desclineWidth := 80 - 7 // 7 is the width of the tinymap
		padding := 1
		description := util.SplitString(details.Description, desclineWidth-padding)
```

with

```go
	renderNouns := true

	// A mutator's description modifier joins the description BEFORE the
	// wrap, so a long modifier wraps with the rest (#455).
	descParts := descriptionWithModifiers(r, details.Description)

	if len(tinymap) > 0 {
		desclineWidth := 80 - 7 // 7 is the width of the tinymap
		padding := 1
		description := wrapDescriptionParts(descParts, desclineWidth-padding)
```

Replace `		roomDesc := util.SplitString(details.Description, 80)` with

```go
		roomDesc := wrapDescriptionParts(descParts, 80)
```

In the mutator loop, replace the whole `if mutSpec.DescriptionModifier != nil { ... }` block (from `		if mutSpec.DescriptionModifier != nil {` through the `}` before `		// Alert modifiers can only add to the list.`) with

```go
		// Description modifiers were applied before the wrap; see
		// descriptionWithModifiers.

```

Delete the import line `	"github.com/GoMudEngine/GoMud/internal/term"` (its only uses were in the deleted block). Append to the end of the file:

```go

// descriptionPart is one block of a room description: the room's own text
// or a mutator's description modifier, with the colour pattern to apply
// after it is wrapped.
type descriptionPart struct {
	text         string
	colorPattern string
}

// descriptionWithModifiers applies every active mutator's description
// modifier (prepend, append or replace) to the room's description, as
// parts still to be wrapped. GetDetails used to add them after the wrap,
// so a long modifier printed as one unwrapped line (#455). A replace with
// no text recolours every part that has no pattern of its own yet, as the
// old in-place ApplyColorPattern left already coloured text alone.
func descriptionWithModifiers(r *Room, description string) []descriptionPart {
	parts := []descriptionPart{{text: description}}
	for mut := range r.ActiveMutators {
		mutSpec := mut.GetSpec()
		if mutSpec == nil || mutSpec.DescriptionModifier == nil {
			continue
		}
		mod := mutSpec.DescriptionModifier
		switch mod.Behavior {
		case mutators.TextPrepend:
			if mod.Text != `` {
				parts = append([]descriptionPart{{text: mod.Text, colorPattern: mod.ColorPattern}}, parts...)
			}
		case mutators.TextReplace:
			if mod.Text != `` {
				parts = []descriptionPart{{text: mod.Text, colorPattern: mod.ColorPattern}}
				continue
			}
			for i := range parts {
				if parts[i].colorPattern == `` {
					parts[i].colorPattern = mod.ColorPattern
				}
			}
		case mutators.TextAppend:
			if mod.Text != `` {
				parts = append(parts, descriptionPart{text: mod.Text, colorPattern: mod.ColorPattern})
			}
		}
	}
	return parts
}

// wrapDescriptionParts wraps each part to width as plain text, then colours
// each wrapped line. A colour pattern tags every rune, so wrapping the
// tagged text could break a word in two; colouring after the wrap cannot.
func wrapDescriptionParts(parts []descriptionPart, width int) []string {
	var lines []string
	for _, p := range parts {
		for _, line := range util.SplitString(p.text, width) {
			lines = append(lines, colorpatterns.ApplyColorPattern(line, p.colorPattern))
		}
	}
	return lines
}
```

- [ ] **Step 4: Run the tests and see them pass.**

Run: `gofmt -l internal/rooms` (expect no output), then `go test ./internal/rooms/ ./internal/usercommands/ -count=1` and `go test . -count=1`
Expected: all `ok`.

- [ ] **Step 5: context.md.** In `internal/rooms/context.md`, section "Room Details and Presentation (`roomdetails.go`)", after the "Room alerts" bullet add:

```markdown
- **Description modifiers**: `descriptionWithModifiers` applies each active
  mutator's prepend, append or replace text BEFORE the wrap, and
  `wrapDescriptionParts` wraps each part as plain text then colours each
  line, so a long modifier (the sanctuary line) wraps with the description
  and beside the minimap (#455).
```

- [ ] **Step 6: Commit.**

```bash
git add internal/rooms/roomdetails.go internal/rooms/roomdetails_modifier_wrap_test.go internal/rooms/context.md
git commit -m "fix(rooms): description modifiers wrap with the description (#455)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 17: H7e, The crit banner's closing *** stays with the last word (#455)

**Files:**
- Modify: `internal/combat/combat_helpers.go:1581` (before `buildAttackMessages`), `:1801-1808`
- Create: `internal/combat/crit_banner_wrap_test.go`, `internal/messaging/wrap_nbsp_test.go`
- Docs: `internal/combat/context.md:1670`

A note on the character: write U+00A0 as `string(rune(0x00A0))` in Go, never as a literal no-break space and never as a `\u` escape typed through a tool that may unescape it (the dry run's Write tool turned the six-character escape into the raw character). After each step, `grep -c $'\xc2\xa0' <file>` on a touched Go file must print `0` (it exits 1 on zero matches; run it on its own line).

- [ ] **Step 1: Write the failing test.** Create `internal/combat/crit_banner_wrap_test.go`:

```go
package combat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"gopkg.in/yaml.v2"
)

// seedShippedGenericForBannerTest installs the shipped generic.yaml as the
// only attack message pool, so a crit swing builds real lines.
func seedShippedGenericForBannerTest(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRootForTest(t),
		"_datafiles", "world", "dogmud", "combat-messages", "generic.yaml"))
	if err != nil {
		t.Fatalf("read generic.yaml: %v", err)
	}
	var g items.WeaponAttackMessageGroup
	if err := yaml.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse generic.yaml: %v", err)
	}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: &g,
	}))
}

// #455: the wrapper breaks at any ASCII space, so a crit line one word too
// long left its closing *** alone on the next line ("... Early Strider!" /
// "***"). The close is now joined to the last word by a no-break space
// (U+00A0), which the wrapper never breaks at.
func TestBuildAttackMessages_CritBannerCloseStaysWithTheLastWord(t *testing.T) {
	seedShippedGenericForBannerTest(t)
	src, tgt := defenceFixture(1000)
	src.Name = "Rurik"
	tgt.Name = "Selka"
	tgt.HealthMax.Base = 100
	tgt.HealthMax.Recalculate()

	result := &AttackResult{Crit: true}
	ws := weaponSetup{weaponName: "Iron Longsword", weaponSubType: items.Generic}
	sdp := swingDamageParams{dmgMean: 20}
	buildAttackMessages(result, src, tgt, ws, sdp,
		30, 0, 0, 0, User, User, "", false, false)

	const closeBanner = `<ansi fg="crit-text">***</ansi>`
	lines := append(append(append([]TaggedMessage{}, result.MessagesToSource...), result.MessagesToTarget...), result.MessagesToSourceRoom...)
	if len(lines) < 3 {
		t.Fatalf("a crit swing built %d lines, want the attacker, defender and room lines", len(lines))
	}
	for _, m := range lines {
		if !strings.Contains(m.Text, "!") && !strings.Contains(m.Text, ".") {
			t.Fatalf("fixture built an empty crit line: %q", m.Text)
		}
		if !strings.HasSuffix(m.Text, string(rune(0x00A0))+closeBanner) {
			t.Errorf("the closing banner is not joined to the last word: %q", m.Text)
		}
		// Wrapped at every width from 20 to 80, no line is the banner alone.
		for width := 20; width <= 80; width++ {
			for _, line := range strings.Split(messaging.WrapAnsi(m.Text, width), "\n") {
				if strings.TrimSpace(remoteRoomHidingTag.ReplaceAllString(line, "")) == "***" {
					t.Errorf("at width %d the closing *** wrapped onto a line of its own: %q", width, m.Text)
				}
			}
		}
	}
}
```

(`defenceFixture` and `repoRootForTest` are existing package test helpers; `remoteRoomHidingTag` is the `<[^>]*>` pattern in `wait_remote_room_hiding_test.go`.)

Create `internal/messaging/wrap_nbsp_test.go` (a pin of the property the fix relies on; it passes before and after):

```go
package messaging

import (
	"strings"
	"testing"
)

// PIN, not a red-first test: the crit banner (combat.critBannerClose, #455)
// relies on WrapAnsi breaking only at an ASCII space, so a no-break space
// (U+00A0) keeps the closing *** with the last word at every width.
func TestWrapAnsi_NeverBreaksAtANoBreakSpace(t *testing.T) {
	nbsp := string(rune(0x00A0))
	in := `<ansi fg="crit-text">***</ansi> You strike Early Strider!` + nbsp + `<ansi fg="crit-text">***</ansi>`
	for width := 10; width <= 60; width++ {
		lines := strings.Split(WrapAnsi(in, width), "\n")
		last := lines[len(lines)-1]
		if !strings.Contains(last, "Strider!"+nbsp) {
			t.Errorf("width %d: the closing *** left its last word: %q", width, lines)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/combat/ -run CritBannerClose -count=1`
Expected: FAIL, `the closing banner is not joined to the last word: "<ansi fg=\"crit-text\">***</ansi> Your <ansi fg=\"item\">Iron Longsword</ansi> ..."` and `at width N the closing *** wrapped onto a line of its own` for some widths. The pool line is picked at random, so the quoted text and the widths vary run to run (the integrated run read "lands a lucky CRITICAL HIT on Selka!" at width 30); what must hold is that both kinds of error appear.

- [ ] **Step 3: Implement.** In `internal/combat/combat_helpers.go`, directly above `func buildAttackMessages(result *AttackResult, sourceChar *characters.Character, targetChar *characters.Character,` add:

```go
// critBannerOpen and critBannerClose frame every line of a critical hit. The
// close is joined to the line's last word by a no-break space (U+00A0, built
// from its code point so the source holds no invisible character): the
// wrapper (messaging.WrapAnsi) breaks only at an ASCII space, so the closing
// *** can no longer wrap onto a line of its own (#455).
const critBannerOpen = `<ansi fg="crit-text">***</ansi> `

var critBannerClose = string(rune(0x00A0)) + `<ansi fg="crit-text">***</ansi>`

```

and replace the crit block

```go
	if result.Crit {
		toAttackerMsg = items.ItemMessage(`<ansi fg="crit-text">***</ansi> ` + string(toAttackerMsg) + ` <ansi fg="crit-text">***</ansi>`)
		toDefenderMsg = items.ItemMessage(`<ansi fg="crit-text">***</ansi> ` + string(toDefenderMsg) + ` <ansi fg="crit-text">***</ansi>`)
		toAttackerRoomMsg = items.ItemMessage(`<ansi fg="crit-text">***</ansi> ` + string(toAttackerRoomMsg) + ` <ansi fg="crit-text">***</ansi>`)
		if len(string(toDefenderRoomMsg)) > 0 {
			toDefenderRoomMsg = items.ItemMessage(`<ansi fg="crit-text">***</ansi> ` + string(toDefenderRoomMsg) + ` <ansi fg="crit-text">***</ansi>`)
		}
	}
```

with

```go
	if result.Crit {
		toAttackerMsg = items.ItemMessage(critBannerOpen + string(toAttackerMsg) + critBannerClose)
		toDefenderMsg = items.ItemMessage(critBannerOpen + string(toDefenderMsg) + critBannerClose)
		toAttackerRoomMsg = items.ItemMessage(critBannerOpen + string(toAttackerRoomMsg) + critBannerClose)
		if len(string(toDefenderRoomMsg)) > 0 {
			toDefenderRoomMsg = items.ItemMessage(critBannerOpen + string(toDefenderRoomMsg) + critBannerClose)
		}
	}
```

- [ ] **Step 4: Run the tests and see them pass.**

Run: `gofmt -l internal/combat internal/messaging` (expect no output); `go test ./internal/combat/ -run CritBannerClose -count=1`; `go test ./internal/messaging/ -run NoBreakSpace -count=1`
Expected: `ok`, `ok`.

Run: `go test ./internal/combat/ ./internal/messaging/ ./internal/hooks/ -count=1` and `go test . -count=1`
Expected: all `ok` (`normalize_test.go:67-75` still passes: the line ends in `*`, which the end-punctuation stage skips).

- [ ] **Step 5: context.md.** In `internal/combat/context.md`, after the "vii. Build Messages" code block (which ends `Send to attacker, defender, room observers.` and a closing fence) add:

```markdown

The crit frame is `critBannerOpen` / `critBannerClose`. The close joins the
last word with a no-break space (U+00A0) so the wrapper, which breaks only at
an ASCII space, never strands the closing `***` on a line alone (#455).
```

- [ ] **Step 6: Commit.**

```bash
git add internal/combat/combat_helpers.go internal/combat/crit_banner_wrap_test.go internal/messaging/wrap_nbsp_test.go internal/combat/context.md
git commit -m "fix(combat): the crit banner's closing *** never wraps alone (#455)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 18: H7f, Prompt and GMCP name an unseen foe as the combat lines do (#455)

**Files:**
- Modify: `internal/messaging/predicates.go:208-214` (add after), `internal/rooms/rooms.go:438-446`
- Modify: `internal/users/userrecord.prompt.go:13` (import), `:36-64`, `:372-381`, `:535-540`; `main.go:321-329`
- Modify: `modules/gmcp/gmcp.Char.go:469`, `:474`, `:490-491`, `:506`
- Test: `internal/users/users_test.go:1574-1616`, `modules/gmcp/gmcp.CharEnemies_test.go:17`, `:92`; create `modules/gmcp/gmcp.CharEnemies_shapes_test.go`
- Docs: `internal/messaging/context.md:359-361`, `modules/gmcp/context.md:348`, `:354-356`, `:366`, `docs/architecture/wiring-seams.md:35`

- [ ] **Step 1: Write the failing tests.** Create `modules/gmcp/gmcp.CharEnemies_shapes_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const charEnemiesHeatEyesConditionId = 97031

// #455: in a shapes-only fight the enemy row said "an unseen foe" while every
// combat line said "a figure". The row now uses UnseenNoun of the viewer's
// sight. The viewer here carries heat sight in a pitch-dark room, so they
// make out shapes and nothing more.
func TestCharEnemies_ShapesViewerReadsAFigure(t *testing.T) {
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		charEnemiesHeatEyesConditionId: {
			ConditionId: charEnemiesHeatEyesConditionId,
			Name:        "Test Heat Eyes",
			Flags:       []conditions.Flag{conditions.InfraredVision},
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}},
		},
	}))
	room := &rooms.Room{RoomId: 9710, Zone: "TestZone", Biome: "cave"}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9710: room},
		map[string]*rooms.ZoneConfig{
			"TestZone": {Name: "TestZone", RoomId: 9710, RoomIds: map[int]struct{}{9710: {}}},
		},
	))

	enemy := &mobs.Mob{MobId: 1, InstanceId: 511, Zone: "TestZone"}
	enemy.Character.Name = "Grave Wight"
	enemy.Character.Health = 30
	enemy.Character.HealthMax.Value = 50
	enemy.Character.CombatPhase = combatphase.NewMachine()
	enemy.Character.SetAggro(9711, 0, characters.DefaultAttack)
	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Zone: "TestZone"}},
		map[int]*mobs.Mob{511: enemy},
	))
	room.AddMob(511)

	viewer := users.NewTestUser(9711, "viewer", "Viewer", 97711)
	viewer.Character.RoomId = 9710
	require.True(t, viewer.Character.Conditions.AddCondition(charEnemiesHeatEyesConditionId, true))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9711: viewer}))
	room.AddPlayer(9711)

	g := &GMCPCharModule{}
	data, _ := g.GetCharNode(viewer, `Char.Enemies`)
	enemies, ok := data.([]GMCPCharModule_Enemy)
	require.True(t, ok)
	require.Len(t, enemies, 1)
	require.Equal(t, "a figure", enemies[0].Name, "a shapes viewer reads what the combat lines say")
	require.Equal(t, 0, enemies[0].Hp, "a shapes viewer must carry no readable hp")
	require.Equal(t, 0, enemies[0].MaxHp)
}
```

In `modules/gmcp/gmcp.CharEnemies_test.go`, replace

```go
	require.Equal(t, "an unseen foe", e.Name, "must match the prompt's wording exactly")
```

with

```go
	require.Equal(t, "something", e.Name, "must match the prompt's and the combat lines' wording exactly (#455)")
```

and in its header comment replace `// token (userrecord.prompt.go canSeeTargetForPrompt / messaging.CanSeeClearly),` with `// token (userrecord.prompt.go promptTargetSight / messaging.ReaderSight),`.

In `internal/users/users_test.go`, `TestPromptTargetVisibilityGating`, replace the body from `	// Restore the global check after the test so we don't leak state.` through the function's closing `}` with:

```go
	// Restore the global check after the test so we don't leak state.
	defer SetPromptSightCheck(nil)

	u := &UserRecord{
		UserId: 7001,
		Character: &characters.Character{
			Name:        "seer",
			CombatPhase: combatphase.NewMachine(),
		},
	}
	// U12c-2: the combat target lives on the machine, so the fixture commits
	// through the seam rather than assigning the field.
	u.Character.SetAggro(0, 999999, characters.DefaultAttack)
	at := func(d messaging.SightDecision) func(*characters.Character) messaging.SightDecision {
		return func(*characters.Character) messaging.SightDecision { return d }
	}

	// #455: the {target} token names an unseen foe as the combat lines do,
	// "a figure" at shapes and "something" with no sight, never a name.
	SetPromptSightCheck(at(messaging.SightNone))
	if out := u.ProcessPromptString(`{target}`); !strings.Contains(out, ">something<") {
		t.Errorf("a prompt that sees nothing should read %q, got %q", "something", out)
	}
	SetPromptSightCheck(at(messaging.SightShapes))
	if out := u.ProcessPromptString(`{target}`); !strings.Contains(out, ">a figure<") {
		t.Errorf("a shapes prompt should read %q, got %q", "a figure", out)
	}

	// Sighted: with no such mob instance registered, the lookup yields
	// nothing, but crucially no placeholder is used (the sighted branch was
	// taken). This proves the gate flips on visibility.
	SetPromptSightCheck(at(messaging.SightFull))
	seeOut := u.ProcessPromptString(`{target}`)
	if strings.Contains(seeOut, "something") || strings.Contains(seeOut, "a figure") {
		t.Errorf("sighted prompt must not use the unseen placeholder, got %q", seeOut)
	}

	// targethealth / targetpos must be suppressed entirely below full sight.
	for _, d := range []messaging.SightDecision{messaging.SightShapes, messaging.SightNone} {
		SetPromptSightCheck(at(d))
		if out := u.ProcessPromptString(`{targethealth}{targetpos}`); out != "" {
			t.Errorf("sight %v prompt should suppress targethealth/targetpos, got %q", d, out)
		}
	}

	// Default (no check registered) defaults to full sight.
	SetPromptSightCheck(nil)
	if got := u.promptTargetSight(); got != messaging.SightFull {
		t.Errorf("promptTargetSight should default to SightFull when no check is registered, got %v", got)
	}
}
```

(`users_test.go` already imports `messaging`, `strings`, `characters` and `combatphase`.)

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./modules/gmcp/ -run CharEnemies -count=1`
Expected: FAIL, `TestCharEnemies_ShapesViewerReadsAFigure`: expected `a figure`, actual `an unseen foe`; `TestCharEnemies_BlindViewerGetsNoIdentityOrHp`: expected `something`, actual `an unseen foe`.

Run: `go test ./internal/users/ -run TestPromptTargetVisibilityGating -count=1`
Expected: build failure, `undefined: SetPromptSightCheck`.

- [ ] **Step 3: `messaging.ReaderSight`, and `rooms.visualDecision` returns it.** In `internal/messaging/predicates.go`, after the closing `}` of `CanSeeShapes` add:

```go

// ReaderSight is the sight decision a reader's visual line is judged at:
// SightFull when CanSeeClearly, SightShapes when CanSeeShapes, else
// SightNone. Unlike ParticipantSight it carries the sleep gate, as every
// visual sender does (rooms.visualDecision is this function). The fight
// prompt's {target} and GMCP Char.Enemies name an unseen foe with
// UnseenNoun of it, so they say what the combat lines say (#455).
func ReaderSight(observer *characters.Character, room RoomVisibility) SightDecision {
	switch {
	case CanSeeClearly(observer, room):
		return SightFull
	case CanSeeShapes(observer, room):
		return SightShapes
	}
	return SightNone
}
```

In `internal/rooms/rooms.go`, replace the body of `visualDecision`

```go
	switch {
	case messaging.CanSeeClearly(c, lighting):
		return messaging.SightFull
	case messaging.CanSeeShapes(c, lighting):
		return messaging.SightShapes
	}
	return messaging.SightNone
```

with

```go
	return messaging.ReaderSight(c, lighting)
```

- [ ] **Step 4: The prompt.** In `internal/users/userrecord.prompt.go`, add `"github.com/GoMudEngine/GoMud/internal/messaging"` to the import block after the `gametime` import. Replace everything from `// canSeeInRoomFn reports whether a character can clearly see in the` through the closing `}` of `canSeeTargetForPrompt` with:

```go
// promptSightFn reports the sight decision a character reads the room they
// occupy at (messaging.ReaderSight: blindness, sleep, room light, night
// sight and heat sight). Registered at boot from main.go, which can import
// rooms/ + messaging/ (users/ cannot import rooms/, an import cycle). nil
// means full sight (the boot- and test-safe default). Follows the same
// callback pattern as characters.SetUserUntargetableCheck.
//
// The fight prompt uses this to hide the combat target's identity, health
// and position below full sight, matching the combat-text darkness gating
// in NewRound_DoCombat, and to name the target as those lines do: "a
// figure" at shapes, "something" with no sight (#455).
var promptSightFn func(c *characters.Character) messaging.SightDecision

// SetPromptSightCheck registers the prompt sight check. Repeated
// registrations overwrite; pass nil to disable (tests).
func SetPromptSightCheck(fn func(c *characters.Character) messaging.SightDecision) {
	promptSightFn = fn
}

// promptTargetSight returns the sight decision the fight prompt renders the
// combat target at. SightFull when no check is registered (boot, tests).
func (u *UserRecord) promptTargetSight() messaging.SightDecision {
	if promptSightFn == nil {
		return messaging.SightFull
	}
	return promptSightFn(u.Character)
}
```

Replace

```go
	canSeeChecked := false
	canSeeTarget := true
	sees := func() bool {
		if !canSeeChecked {
			canSeeTarget = u.canSeeTargetForPrompt()
			canSeeChecked = true
		}
		return canSeeTarget
	}
```

with

```go
	sightChecked := false
	targetSight := messaging.SightFull
	sight := func() messaging.SightDecision {
		if !sightChecked {
			targetSight = u.promptTargetSight()
			sightChecked = true
		}
		return targetSight
	}
	sees := func() bool { return sight() == messaging.SightFull }
```

and in the `{target}` case replace the three comment lines that open with `// Hide the target's identity when the player can't see it` (the middle one, `// (blind / dark room without special vision) ... the name is`, holds a pre-existing dash, elided here: copy the exact old text from the file into the Edit tool) and the code after them:

```go
				// Hide the target's identity when the player can't see it
				// (blind / dark room without special vision) ... the name is
				// info they don't have. Matches combat-text darkness gating.
				if !sees() {
					promptOut.WriteString(`<ansi fg="mobname">an unseen foe</ansi>`)
				} else {
```

with

```go
				// Hide the target's identity when the player can't see it
				// clearly: the name is info they don't have. The target is
				// what the combat lines call it at this sight, "a figure" or
				// "something", in the same colour (#455).
				if !sees() {
					promptOut.WriteString(messaging.UnseenFigure(sight()))
				} else {
```

In `main.go`, replace

```go
	users.SetCanSeeInRoomCheck(func(c *characters.Character) bool {
		if c == nil {
			return true
		}
		room := rooms.LoadRoom(c.RoomId)
		if room == nil {
			return true
		}
		return messaging.CanSeeClearly(c, room)
	})
```

with

```go
	users.SetPromptSightCheck(func(c *characters.Character) messaging.SightDecision {
		if c == nil {
			return messaging.SightFull
		}
		room := rooms.LoadRoom(c.RoomId)
		if room == nil {
			return messaging.SightFull
		}
		return messaging.ReaderSight(c, room)
	})
```

- [ ] **Step 5: GMCP.** In `modules/gmcp/gmcp.Char.go`: replace `			// (userrecord.prompt.go canSeeTargetForPrompt / messaging.CanSeeClearly).` with `			// (userrecord.prompt.go promptTargetSight / messaging.ReaderSight).`; replace

```go
			canSee := messaging.CanSeeClearly(user.Character, roomInfo)
```

with

```go
			sight := messaging.ReaderSight(user.Character, roomInfo)
			canSee := sight == messaging.SightFull
```

replace

```go
					// same two things the prompt withholds. Stay binary
					// like the prompt -- no "a figure" tier here.
```

with

```go
					// same two things the prompt withholds. The name is
					// what the combat lines call it at this sight, "a
					// figure" at shapes and "something" with none (#455).
```

and replace ``					e.Name = `an unseen foe` `` with

```go
					e.Name = messaging.UnseenNoun(sight)
```

- [ ] **Step 6: Run the tests and see them pass.**

Run: `go build ./... && go vet ./internal/users ./modules/gmcp .`; `gofmt -l internal/users modules/gmcp main.go internal/messaging internal/rooms` (expect no output)
Run: `go test ./internal/users/ ./modules/gmcp/ ./internal/messaging/ ./internal/rooms/ -count=1` and `go test . -count=1`
Expected: all `ok`.

Run (on its own line): `grep -rn "unseen foe" --include=*.go internal modules main.go`
Expected: exactly three comment hits, `internal/messaging/predicates.go` (the `ReaderSight` doc), `internal/users/users_test.go` (the #455 comment) and `modules/gmcp/gmcp.CharEnemies_shapes_test.go` (its header). No string literal says "an unseen foe" any more.

- [ ] **Step 7: Docs.** `internal/messaging/context.md`, in the predicates list after the `CanSeeShapes(observer, room) bool` bullet (which ends `shapes either.`) add:

```markdown
  - `ReaderSight(observer, room) SightDecision` = the two above as one
    decision (full, shapes, none). `rooms.visualDecision` is this
    function; the fight prompt's `{target}` and GMCP `Char.Enemies` name an
    unseen foe with `UnseenNoun` of it, as the combat lines do (#455).
```

`modules/gmcp/context.md`: replace ``| `name` | `Name` | the mob's name, or `an unseen foe` (see below) |`` with ``| `name` | `Name` | the mob's name, or `a figure` / `something` (see below) |``; replace

```markdown
`messaging.CanSeeClearly(user.Character, roomInfo)`, the same predicate
`userrecord.prompt.go`'s `canSeeTargetForPrompt` reads.
```

with

```markdown
`messaging.ReaderSight(user.Character, roomInfo)`, the same decision
`userrecord.prompt.go`'s `promptTargetSight` reads.
```

and replace ``- `Name` becomes the literal `an unseen foe`.`` with

```markdown
- `Name` becomes `messaging.UnseenNoun(sight)`: `a figure` at shapes,
  `something` with no sight, the words the combat lines use (#455).
```

`docs/architecture/wiring-seams.md:35`: replace `SetCanSeeInRoomCheck` with `SetPromptSightCheck` in the `users` row.

- [ ] **Step 8: Commit.**

```bash
git add internal/messaging/predicates.go internal/messaging/context.md internal/rooms/rooms.go internal/users/userrecord.prompt.go internal/users/users_test.go main.go modules/gmcp/gmcp.Char.go modules/gmcp/gmcp.CharEnemies_test.go modules/gmcp/gmcp.CharEnemies_shapes_test.go modules/gmcp/context.md docs/architecture/wiring-seams.md
git commit -m "fix(prompt,gmcp): an unseen foe reads a figure or something, as combat lines do (#455)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 19: H7g, Admin spawn echoes and admin `online` fit 80 columns (#449)

**Files:**
- Modify: `internal/usercommands/admin.item.go:128`, `:131`; `internal/usercommands/admin.mob.go:205`, `:208`; `internal/usercommands/admin.spawn.go:51`, `:54`, `:77`, `:80`; `internal/usercommands/online.go:15-29`, `:71-91`
- Modify: `messaging_surface_guard_test.go:1303`, `:1305`, `:1311`, `:1312` (registry keys; after Task 8 adds three lines above them they sit at `:1306`, `:1308`, `:1314`, `:1315`, so find them by their text)
- Create: `internal/usercommands/admin_spawn_echo_test.go`, `internal/usercommands/online_columns_test.go`

- [ ] **Step 1: Write the failing tests.** Create `internal/usercommands/admin_spawn_echo_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// #449: the admin spawn echoes ran 84 to 93 columns ("You wave your hands
// around and Goblin Scout appears in the air and falls to the ground.").
// They now read "You wave your hands and X appears.", and the room's line
// matches.
func TestSpawn_EchoesAreShortAndMatchTheRoomLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	admin := users.GetByUserId(1)
	admin.Role = users.RoleAdmin
	room := rooms.LoadRoom(1)

	for _, tc := range []struct {
		rest, self, watcher string
	}{
		{"gold 7", "You wave your hands and 7 gold appears.", "Aliceia waves their hands and 7 gold appears."},
		{"container crate", "You wave your hands and crate appears.", "Aliceia waves their hands and crate appears."},
	} {
		craftPlainLines(1)
		craftPlainLines(2)
		_, err := Spawn(tc.rest, admin, room, 0)
		require.NoError(t, err)

		self := craftPlainLines(1)
		require.Contains(t, self, tc.self, "spawn %q", tc.rest)
		for _, line := range self {
			require.LessOrEqual(t, util.VisibleWidth(line), 80, "spawn %q echo over 80 columns: %q", tc.rest, line)
		}
		require.Contains(t, craftPlainLines(2), tc.watcher, "spawn %q room line", tc.rest)
	}
}
```

Create `internal/usercommands/online_columns_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #449: the admin view of `online` adds UserId, Zone and RoomId to the
// player columns and ran to 102 columns. It drops Title; players keep it.
func TestOnlineColumns_AdminViewDropsTitle(t *testing.T) {
	require.Equal(t, []string{`UserId`, `User.Name`, `Online`, `Role`, `Zone`, `RoomId`}, onlineColumns(true))
	require.Equal(t, []string{`User.Name`, `Title`, `Online`, `Role`}, onlineColumns(false))
}
```

(`seedAllRegistries` seeds Aliceia (1) and Bobrick (2) in lit room 1; `craftPlainLines` drains a user's queue as tag-stripped lines. The `item` and `mob` spawn paths need loaded item and mob data, so they are covered by the playtest, not here; their literals change exactly as these two do.)

- [ ] **Step 2: Run them and see them fail.**

Run: `go test ./internal/usercommands/ -run 'Spawn_Echoes|OnlineColumns' -count=1`
Expected: build failure, `undefined: onlineColumns`. With the online test file set aside, `TestSpawn_EchoesAreShortAndMatchTheRoomLine` fails: `[...] "You wave your hands around and 7 gold appears from thin air and falls to the ground." does not contain "You wave your hands and 7 gold appears."`.

- [ ] **Step 3: The echoes.** Eight literal edits, user line then room line:

`internal/usercommands/admin.item.go`:
- `` `You wave your hands around and <ansi fg="item">%s</ansi> appears from thin air and falls to the ground.` `` becomes `` `You wave your hands and <ansi fg="item">%s</ansi> appears.` ``
- `` `<ansi fg="username">%s</ansi> waves their hands around and <ansi fg="item">%s</ansi> appears from thin air and falls to the ground.` `` becomes `` `<ansi fg="username">%s</ansi> waves their hands and <ansi fg="item">%s</ansi> appears.` ``

`internal/usercommands/admin.mob.go`:
- `` `You wave your hands around and <ansi fg="mobname">%s</ansi> appears in the air and falls to the ground.` `` becomes `` `You wave your hands and <ansi fg="mobname">%s</ansi> appears.` ``
- `` `<ansi fg="username">%s</ansi> waves their hands around and <ansi fg="mobname">%s</ansi> appears in the air and falls to the ground.` `` becomes `` `<ansi fg="username">%s</ansi> waves their hands and <ansi fg="mobname">%s</ansi> appears.` ``

`internal/usercommands/admin.spawn.go`:
- `` `You wave your hands around and <ansi fg="container">%s</ansi> appears from thin air and falls to the ground.` `` becomes `` `You wave your hands and <ansi fg="container">%s</ansi> appears.` ``
- `` `<ansi fg="username">%s</ansi> waves their hands around and <ansi fg="container">%s</ansi> appears from thin air and falls to the ground.` `` becomes `` `<ansi fg="username">%s</ansi> waves their hands and <ansi fg="container">%s</ansi> appears.` ``
- `` `You wave your hands around and <ansi fg="gold">%d gold</ansi> appears from thin air and falls to the ground.` `` becomes `` `You wave your hands and <ansi fg="gold">%d gold</ansi> appears.` ``
- `` `<ansi fg="username">%s</ansi> waves their hands around and <ansi fg="gold">%d gold</ansi> appears from thin air and falls to the ground.` `` becomes `` `<ansi fg="username">%s</ansi> waves their hands and <ansi fg="gold">%d gold</ansi> appears.` ``

Leave "You wave your hands around pathetically." (`admin.spawn.go:90`) as it is.

- [ ] **Step 4: `online`.** In `internal/usercommands/online.go`, replace

```go
func Online(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	isAdmin := user.Role != users.RoleUser

	headers := []string{
		language.T(`User.Name`),
		language.T(`Title`),
		language.T(`Online`),
		language.T(`Role`),
	}

	if isAdmin {
		headers = append([]string{language.T(`UserId`)}, headers...)
		headers = append(headers, []string{language.T(`Zone`), language.T(`RoomId`)}...)
	}
```

with

```go
// onlineColumns lists the online table's columns by translation key, in
// order. The admin view adds UserId, Zone and RoomId and drops Title, which
// had pushed it to 102 columns (#449); players keep Title.
func onlineColumns(isAdmin bool) []string {
	if isAdmin {
		return []string{`UserId`, `User.Name`, `Online`, `Role`, `Zone`, `RoomId`}
	}
	return []string{`User.Name`, `Title`, `Online`, `Role`}
}

func Online(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	isAdmin := user.Role != users.RoleUser

	columns := onlineColumns(isAdmin)
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = language.T(column)
	}
```

and replace

```go
			row := []string{
				charName,
				onlineInfo.Title,
				onlineTime,
				onlineInfo.Role,
			}

			formatting := []string{
				`<ansi fg="username">%s</ansi>`,
				`<ansi fg="white-bold">%s</ansi>`,
				`<ansi fg="magenta">%s</ansi>`,
				`<ansi fg="role-` + permClass + `-bold">%s</ansi>`,
			}

			if isAdmin {
				row = append([]string{strconv.Itoa(u.UserId)}, row...)
				row = append(row, []string{u.Character.Zone, strconv.Itoa(u.Character.RoomId)}...)

				formatting = append([]string{`<ansi fg="userid">%s</ansi>`}, formatting...)
				formatting = append(formatting, []string{`<ansi fg="zone">%s</ansi>`, `<ansi fg="1">%s</ansi>`}...)
			}
```

with

```go
			// Each column's value and its formatting, keyed as onlineColumns
			// names them.
			cells := map[string][2]string{
				`UserId`:    {strconv.Itoa(u.UserId), `<ansi fg="userid">%s</ansi>`},
				`User.Name`: {charName, `<ansi fg="username">%s</ansi>`},
				`Title`:     {onlineInfo.Title, `<ansi fg="white-bold">%s</ansi>`},
				`Online`:    {onlineTime, `<ansi fg="magenta">%s</ansi>`},
				`Role`:      {onlineInfo.Role, `<ansi fg="role-` + permClass + `-bold">%s</ansi>`},
				`Zone`:      {u.Character.Zone, `<ansi fg="zone">%s</ansi>`},
				`RoomId`:    {strconv.Itoa(u.Character.RoomId), `<ansi fg="1">%s</ansi>`},
			}
			row := make([]string, len(columns))
			formatting := make([]string, len(columns))
			for i, column := range columns {
				row[i], formatting[i] = cells[column][0], cells[column][1]
			}
```

- [ ] **Step 5: Run the package tests, then the root guard (it trips).**

Run: `gofmt -l internal/usercommands` (expect no output); `go vet ./internal/usercommands/`; `go test ./internal/usercommands/ -count=1`
Expected: `ok`.

Run: `go test . -run TestNarrationSitesMatchViewpointAudit -count=1`
Expected: FAIL: 4 candidate events unregistered (`usercommands/admin.item.go|You wave your hands and <ansi fg="item">%s</ansi> appears.` and the mob, container and gold twins) and 4 stale registry entries (the old `You wave your hands around and ...` keys). The verdict does not change (actor+observer, no person as actee), only the literal.

- [ ] **Step 6: Re-key the four registry rows.** In `messaging_surface_guard_test.go`, change only the map keys (leave each value, verdict and reason as it is):

- `"usercommands/admin.item.go|You wave your hands around and <ansi fg=\"item\">%s</ansi> appears from thin air a"` becomes `"usercommands/admin.item.go|You wave your hands and <ansi fg=\"item\">%s</ansi> appears."`
- `"usercommands/admin.mob.go|You wave your hands around and <ansi fg=\"mobname\">%s</ansi> appears in the air a"` becomes `"usercommands/admin.mob.go|You wave your hands and <ansi fg=\"mobname\">%s</ansi> appears."`
- `"usercommands/admin.spawn.go|You wave your hands around and <ansi fg=\"container\">%s</ansi> appears from thin "` becomes `"usercommands/admin.spawn.go|You wave your hands and <ansi fg=\"container\">%s</ansi> appears."`
- `"usercommands/admin.spawn.go|You wave your hands around and <ansi fg=\"gold\">%d gold</ansi> appears from thin "` becomes `"usercommands/admin.spawn.go|You wave your hands and <ansi fg=\"gold\">%d gold</ansi> appears."`

Run: `gofmt -l messaging_surface_guard_test.go`. It prints the file name: the shorter keys change the map's column alignment. Run `gofmt -w messaging_surface_guard_test.go` (it only realigns the values of those rows), then `go test . -count=1`
Expected: `ok`.

- [ ] **Step 7: Commit.**

```bash
git add internal/usercommands/admin.item.go internal/usercommands/admin.mob.go internal/usercommands/admin.spawn.go internal/usercommands/online.go internal/usercommands/admin_spawn_echo_test.go internal/usercommands/online_columns_test.go messaging_surface_guard_test.go
git commit -m "fix(admin): spawn echoes and admin online fit 80 columns (#449)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 20: Whole-tree verification

- [ ] **Step 1: Format.**

Run: `gofmt -l ./internal ./modules *.go`
Expected: no output.

- [ ] **Step 2: Vet.**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: Full suite.**

Run: `go test ./... -count=1` (a few minutes)
Expected: every package `ok`. A failure is a finding: fix it in the task it belongs to, do not skip it.

- [ ] **Step 4: Lint, new issues only (as CI's `only-new-issues`).**

Run: `golangci-lint run --new-from-rev=master ./...`
Expected: `0 issues.` (errcheck is on: assign an unused error return with `_ =`.)

- [ ] **Step 5: context.md audit.**

Run: `python tools/context_md_audit.py`
Expected: none of `actions`, `usercommands`, `hooks`, `characters`, `rooms`, `messaging`, `behaviortree`, `mobcommands`, `mapper`, `combat`, `users` or `modules/gmcp` listed.

- [ ] **Step 6: No dashes in what this branch added.**

Run (on its own line; zero matches expected): `git diff master -- '*.go' '*.yaml' '*.template' '*.md' | grep '^+' | grep -nP '[\x{2013}\x{2014}]'`
Expected: no output.

- [ ] **Step 7: The config bit.**

Run: `git diff master --stat -- _datafiles/config.yaml`
Expected: no output (no task touches it). In this worktree `git ls-files -v _datafiles/config.yaml` reads `H` (a fresh worktree index); the main checkout reads `S`. Leave both as they are.

---

### Task 21: Playtest (executed by the controller, not a subagent)

Run against a local build of this branch with the harness, following `dogmud-playtesting` (kill only your own server, by PID). Harness lessons from the earlier sight-gates runs are in memory `project-sight-closeout-playtest-harness-lessons-2026-10-08`. Goals, not mechanics:

- **#453 (Task 1):** carry a lit torch and sneak into hiding, own a pet, and let a condition start, tick and end (a potion, a bleed). The room's lines read the bare name: no "(Lit)", no "(hidden)", no "and <pet>". `look` still shows the adjectives.
- **#454 (Tasks 2 to 7):** in a room where you make out shapes only, `attack <name>`, `kick <name>`, `give <item> <name>`, `steal <name>`, `talk <npc>`, `ask <npc> about <topic>`, `party invite <name>` and `rep <name>` each read the two-line hint; `kick shape`, `give <item> 2.shape` work and the lines that follow do not name the creature. In pitch dark every one reads "You don't see them here." whether or not anyone is there. `fire` into a dim room is refused before any name is checked. A quest NPC handing an item back in the dark is "Something". Check what an NPC says after `talk shape` or `ask shape about <topic>` (D-H2-12).
- **#456 (Tasks 8 and 9):** at night a player carrying a lit torch quits, walks out with `go`, and a mob carrying a light is moved with `RelocateMob` (or forced `go`): a watcher left in the dark reads the name as they leave. A sneak-hidden player quitting is not named to a watcher who could not see them.
- **#451 (Task 10):** sneak into hiding and let the character save while hidden (an autosave), then stop your own server by its PID before any logout runs and start it again; log back in: still hidden, no "emerges from the shadows" line. (A normal `quit` runs `ForceVisible` first, which ends the hide by design.)
- **#242 (Task 11):** a mob caster mid-fold whose player target walks out: the room reads the fizzle by sight (full and shapes) or by sound (blinded), before "mumbles about losing their quarry". Repeat with an area fold that completes on an empty room.
- **#251 (Task 12):** spawn a Goblin Scout (mob 217) in a room lit at 55 or more (shipped `LightExitsAbove` 55; below it the scout cannot see out and the control fails for the wrong reason) with no player in it, beside a pitch-dark room holding an unlit player (Night Vision cannot lift light 0) and a lit room holding a sneak-hidden player. Watch for five minutes: the scout must not move toward either. Then the dark-room player lights a torch, and the scout must track them within five minutes. That is the control.
- **#455 (Tasks 13 to 18):** the minimap and `map` colour Deep Water, Dense Forest and the city streets; a mob emote ends with a full stop; a clawed mob's crit and hit lines read "sinks their claws"; a room description with a mutator modifier wraps at 80; a long crit line never leaves `***` alone on a line; the prompt and the GMCP enemy list read "a figure" at shapes and "something" in the dark.
- **#449 (Task 19):** admin `spawn` echoes fit 80 columns; admin `online` has no Title column.

File every finding as a GitHub issue on `pruuk/DOGMud` (`--repo pruuk/DOGMud`; exploit-class leaks stay out of public issues). If no leak is found, the PR body lists #242, #251, #449, #451, #453, #454, #455 and #456 as resolved, in words, without a closing keyword, and #382 gets its closing comment.

---

## Integrated dry run

All 19 code tasks were applied together, in plan order, on one fresh `bd1220964` worktree, from this document as written: new files copied from the plan's code blocks, every edit made with the Edit tool on the plan's quoted anchors after the earlier tasks had already changed the files.

- Every new test that is meant to fail first failed first, for the reason its step gives, and passed after its implementation step. Task 12's two refusal tests failed under the temporary `scan.go` mutation and passed once it was reverted.
- `go test . -count=1` passed at the end of every task.
- On the combined tree (Task 20): `gofmt -l ./internal ./modules *.go` printed nothing; `go vet ./...` was clean; `go test ./... -count=1` passed with 131 packages `ok` and none failing; `golangci-lint run --new-from-rev=HEAD ./...` reported `0 issues.`; `python tools/context_md_audit.py` listed only packages this plan does not touch, all of them already listed on master; the dash scan of the added lines and the U+00A0 scan of the added Go lines both found nothing.
- The union of the 19 tasks' `git add` lists matched exactly the 98 paths the run changed or created.
- No cross-task conflict surfaced. H1's tag change did not move any H2 or H5 expectation. Task 14's end-punctuation stage leaves Task 7's "hands back" line and empty lines alone, because `appendEndPunct` returns blank text unchanged. Tasks 8 and 18 both edit `rooms.go`, and Tasks 1, 14 and 18 all edit `messaging/context.md`, but their anchors do not overlap.

The integration forced these corrections to the plan text (no design change):

- **Task 3, Step 7:** `Attack("skeleton", user, room, 0)` occurs three times in `attack_sight_names_test.go`. The step now says to anchor the Edit on the block that runs down to the companion message.
- **Task 7, Step 4:** `internal/usercommands/context.md` has two `#### **Skill-Based Commands**` headings. The step now names the first one and anchors on the heading together with its `- **Magic system**` line.
- **Task 9, Step 3:** the two player departure sends are identical text, two lines above their `Sprintf` rather than one. The step now says to use the Edit tool's `replace_all`.
- **Task 13, Step 8:** the mapper paragraph's position is now stated: directly after the Rendering block's closing fence.
- **Task 17, Step 2:** the crit pool line is chosen at random, so the failing text and widths vary between runs. The expected output now describes the two kinds of error instead of quoting one line.
- **Task 18, Step 6:** the `unseen foe` grep finds three comment hits (`predicates.go`, `users_test.go` and the new GMCP test), not one.
- **Task 19:** after Task 8 the four registry keys sit three lines lower (`:1306`, `:1308`, `:1314`, `:1315`). Also, `gofmt -w` on `messaging_surface_guard_test.go` is required, not optional, because the shorter keys change the map's alignment.

## Self-review against the spec

- H1 Task 1; H2 Tasks 2 to 7 (cast on the shared rule, attack and target and the melee specials, give and show and consider and steal and plant and shadow, talk and ask and party invite and rep, fire, the give lines and the quest return line); H3 Tasks 8 and 9 (arrivals unchanged, per owner call 1); H4 Task 10; H5 Task 11; H6 Task 12; H7 map tags Task 13, mob emote stop Task 14, claws Task 15, modifiers before the wrap Task 16, crit banner Task 17, prompt and GMCP Task 18, admin echoes and `online` Task 19; testing and close Tasks 20 and 21.
- Spec items the code changed: `follow` is `shadow`; `target` added; the quit-line exclusion is captured before `ForceVisible`; 13 claws lines plus one bite line, not 8; see "Spec facts that were wrong" and the design points.
- Out of scope, per spec: room prose dashes (#248), crit disarm, arrival lines.
