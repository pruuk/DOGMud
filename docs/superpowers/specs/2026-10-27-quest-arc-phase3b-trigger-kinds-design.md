# #375 phase 3b: new trigger kinds (talk, targets, effect spells, areas and counts)

Issue: #375 (quest and dialogue arc), roadmap `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`
"Phase 3: Action triggers". Phase 3a (PR #688) fixed what existing quests depend on; this spec adds
the trigger kinds new and ported quests need. One spec, four slices, each with its own plan, PR and
playtest (owner, 2026-10-27). Brainstormed with the owner 2026-10-27.

## Facts verified against source (master `906a97ae0`, 2026-10-27)

| Fact | Where |
|---|---|
| Trigger events allowed by the loader: room_enter, room_gate, item_give, skill_use, mob_death, command, item_gain, quest_granted, room_interact, command_issued | `internal/quests/quests.go:113-120` |
| `TriggerDef` filters: room, mob, item, skill, command, quest_token, noun, verb. No count, area or zone field | `internal/quests/triggers.go:11-23` |
| `EventDetails`: UserId, RoomId, MobId, ItemId, Skill, Command, Noun, RoomNoun, Verb, QuestToken | `internal/questengine/types.go:39-50` |
| Filters are matched in `matchTriggerFields`; triggers are indexed by event type (`triggerIndex[eventType]`) | `internal/questengine/engine.go:163-250` |
| `Engine.WouldFire(eventType, details, player)` is a dry run (phase 3a) | `internal/questengine/dryrun.go:14` |
| Topic effects: grantsQuest, givesItem, setsQuestFlag, bumpsRep, givesGold, chargesGold, moodChange | `internal/dialogue/topic.go:53-61` |
| 187 topic `grantsQuest`: 87 grant a `-start` token, 68 an `-end` token, **32 a mid-quest step**, all 32 on `kind: info` topics in 27 dialogue files | grep of `dialogue/**` |
| `DialogueFile.Apply` runs the charge and paid effects, then `MarkHeard(HeardKey(mobId, key))` for an info topic; it returns false when a charge or effect refuses | `internal/dialogue/topics.go:172-187` |
| `play` calls `applyWithHandover`, then the NPC line, hints and `after` (markers, mood, behavior topic) only when Apply succeeded | `internal/conversation/talk.go:277-322` |
| `[*]` shows on a topic only when `Marked` and not yet heard | `internal/dialogue/topics.go:48-62` |
| Heard topics save as `HeardTopics []string`; `MarkHeard`, `HasHeard` | `internal/characters/character.go:321`; `internal/characters/heard.go` |
| `questloop` `one-action` counts one topic as a way into a step | `internal/questloop/rule_oneaction.go:36`; `grants.go:60-66` |
| `internal/conversation` already depends on `internal/questengine` (transitively), so the reverse import is impossible and the forward one is safe | `go list -deps` |
| The `command` event is raised from **32** production sites (cast, throw, fire/shoot, trip, kick, taunt, steal, sneak, track, forage, search, buy, sell, drink, plant, shadow, equip, craft, consider, help, list, say, set, sleep, warcry, flee and others); `command_issued` from 1 (`usercommands.go:538`) | grep `Notify("command",` |
| Only two `command` sites pass anything beyond command and room: cast passes `Noun: spellInfo.SpellId` (`skill.cast.go:387-392`), equip passes `ItemId` (`equip.go:213-217`) | same grep |
| Shipped `command` triggers: forage 7, craft 7, cast 6, fire 4, list 3, equip 3, trip 2, track 2, drink 2, eleven others 1 each | grep of `quests/` |
| No `use` command exists | `internal/usercommands/usercommands.go` |
| Spell resolution: `resolveSpell` (player caster), per target `resolveAgainstMob` / `resolveAgainstPlayer` return `(fumbled, landed)`; mob casters go through `resolveMobSpell`, `resolveMobSpellAgainstMob`, `resolveMobSpellAgainstPlayer`; the Go-hook spells fold-anchor, fold-recall, purge-affliction return early from `resolveSpell` | `internal/hooks/spell_resolution.go:78, 357, 502, 574, 785, 822, 227-270` |
| A help spell (attack_type none) on a mob resolves uncontested through `resolveHelpSpell`; the heal travels as a queued condition (`applySpellHeal`) | `spell_resolution.go:364-372`; `internal/hooks/spell_help_effects.go:161-182` |
| `events.Healed` fires at the cast on a mob, not on the result; consumed only by `modules/aicompanion` | `internal/events/eventtypes.go:661-669`; `spell_help_effects.go:176` |
| Spell `EffectType` values in use include damage, heal, shield, dot, knockdown, purge, charm, identify, drain_area | `internal/spells/context.md`; `spell_resolution.go` |
| New characters start with `conviction-spike`, `chrysalis-glow`, `identify`; no heal | `internal/characters/character.go:377-381` |
| Heal spells: Mend Flesh (`heal`, difficulty 5), Mend Wounds, Mass Mend, Mend All, Communion of Flesh; Arc-Weld Repair is `mob_only` | `_datafiles/world/dogmud/spells/*.yaml` |
| Spells are discovered by chance on a cast from the four schools once `Spellcasting >= difficulty x SpellDiscoverySkillPerDifficulty` (shipped 1.0); quest 46 (The Unfolded) rewards `heal` | `internal/hooks/NewRound_DoCombat_helpers.go:745-762`; `_datafiles/config.yaml:1975`; `quests/46-the_unfolded.yaml:49` |
| `internal/spells/context.md` still says "Starting spells reduced to 1 (Conviction Spike only)": stale | `internal/spells/context.md` |
| Rooms carry `zone` and `biome`; no named room set exists anywhere in world data | `internal/rooms/rooms.go:86, 96`; grep of `_datafiles/world/dogmud` |
| Quest 45 repeats one `command: cast` trigger for rooms 5311, 5312, 5313 | `quests/45-*.yaml:83-115` |
| `room_enter` is raised for every move including teleports (RoomChange hook) and at login | `internal/hooks/RoomChange_QuestRoomEnter.go:43`; `PlayerSpawn_HandleJoin.go:327` |
| `VisitedRooms map[string][]int` (zone to template room ids) is the web map's fog of war; `MarkRoomVisited` and `HasVisitedRoom` scan the slice linearly | `internal/characters/character.go:329, 680-700` |
| A room is added only by `actions.MarkRoomMappedIfSeen`, only when the character can see something there; it returns true only when the room is newly mapped | `internal/actions/map_visit.go:24-40` |
| Kill steps today are one kill each (quest 53: one scout, then one squatter); no trigger counts | `quests/53-*.yaml` |
| Saved quest state: `QuestProgress map[int]string`, `QuestCompletions map[int]int`, `MiscData map[string]any` | `internal/characters/character.go:320-327` |
| Minimap target: `ResolveQuestTarget` resolves map_target -1, map_target_mob, map_target, an inferred room_enter room, else 0 | `internal/questengine/map_target.go:38-53` |
| Manifests: `dialogue/topic-keys.txt`, `quests/step-ids.txt`, both on `internal/manifest`; guards `topic_manifest_test.go`, `step_manifest_test.go` | `_datafiles/world/dogmud/`; `internal/guards/` |
| The 2026-10-24 prod lag was a full `Zone.Map` built on the game loop per move | memory `project-prod-lag-2026-10-24`; `map_visit.go:16-19` |

## Owner rulings (2026-10-27)

1. One spec covering every 3b slice; each slice gets its own plan.
2. **A quest step completes on something the player does.** Observe is dropped from 3b and from
   requirement 14, which now reads "cast, explore, acquire, talk, use". It comes back only if a
   phase 7 zone pass has a step that needs it; the starting design is in "Deferred: observe".
3. **Talk:** a quest-side `topic_heard` trigger owns every mid-quest talk step. The 32 mid-step
   `grantsQuest` topics move onto it. Offers and hand-ins stay on topics (phase 2).
4. **Targets:** every targeted `command` site passes its target through one helper; a guard holds
   every site to it.
5. **Effect spells:** a `spell_landed` event per target a spell lands on, filtered by effect or by
   spell. Every effect step must be reachable: the spell is taught earlier on the quest's path, or
   the quest's offer is gated on a quest that teaches it, with a breadcrumb pointing at the teacher
   ("Go and learn a healing spell from ..., then come back").
6. **Areas:** one file per area naming zones and/or rooms; a zone counts whole, so a two-zone area
   plus a few road rooms lists two zones and those rooms.
7. **Explore counts what the player has seen:** a room counts once it is on the player's map, which
   needs sight, so a room crossed in the dark counts for neither. Rooms mapped before the quest
   count. A step whose area is already fully mapped completes at once with its own line.
8. **Performance:** nothing added scans on the per-move path; budgets are measured, not assumed.

## Design

### 1. Slice 1: talk (`topic_heard`)

**Trigger.** New event `topic_heard`, new trigger field `topic` (a topic key), with the existing
`mob` filter (template id). Both are required on a `topic_heard` trigger; the loader refuses either
missing, and refuses `topic` on any other event.

**When it fires.** In `conversation.play`, after `applyWithHandover` returns true and before
`after`, for every topic kind:
`Notify("topic_heard", EventDetails{UserId, RoomId: template room, MobId: mob template id, Topic:
key})`. A topic whose charge or effect refused fires nothing. `EventDetails` gains `Topic string`.

**Heard before the step does not count.** The trigger fires on hearing the topic while the step is
current (its `conditions.has` names the previous step's token, as `one-action` already requires).
A player who heard the topic earlier picks it again. The reason: the line should land when the
quest makes it matter.

**The objective marker.** A topic that is the current step's objective shows `[*]` even when
heard. `dialogue.PlayerState` gains `Objective func(mobId int, key string) bool`;
`conversation.PlayerState` wires it to `questengine.GetEngine().WouldFire("topic_heard", ...)`.
`TopicMarker` returns `MarkerInfo` when `Objective` reports true, before its `Marked && !heard`
check. The NPC's name marker (`Marker`) follows, so the roster shows `[*]` on the NPC too. A nil
callback reports false.

**Migration of the 32.** Each mid-step `grantsQuest` on an info topic becomes:
- the topic loses `grantsQuest` and keeps its gates (`questRequired`, `questExcluded`) so it is
  still shown only during the step;
- the quest gains one `topic_heard` trigger with `mob`, `topic`, `conditions.has: [previous
  token]`, `missing: [this token]`, and `grant: this token`, plus any other effect the topic
  carried that belongs to the quest (none move unless the plan finds one).

The plan lists the 32 with their files from the grep above. The walkthrough
(`questengine/walkthrough.go`) learns `topic_heard` as a way into a step, so every migrated quest
still walks.

**Validators.** `questloop` gains rule `talk-on-quest`: a topic granting a token that is neither
its quest's first step nor its final step is a warning (in `loop-backlog.txt`), becoming an error
after phase 7 like the other loop rules. A `topic_heard` trigger whose `mob` has no dialogue file,
or whose file has no topic `topic`, is a boot error.

### 2. Slice 2: targets on `command` events

**One helper.** `questengine.CommandEvent(actor, room, command string, target Target)` builds the
`EventDetails` for every `command` site; the 32 sites call it instead of building the struct by hand.
`Target` holds `Kind` (`self`, `mob`, `player`, `item`, `none`), `MobId` (template), `ItemId`
(template) and the existing `Noun` (spell id for cast). `EventDetails` gains `TargetKind string`.
A trigger gains `target: self|mob|player|item` (optional), matched against `TargetKind`; the
existing `mob:` and `item:` filters now match on `command` events too.

**Which target.** The command's own resolved target, after `AimBySight` or the item match the
command already does. Never re-resolved. A command with several targets (an area grenade) raises
one event per target that was affected, kind `mob` or `player`; a grenade that hits nothing raises
one event with kind `none`. A command with no target (help, list, sleep) passes `none`.

**Guard.** `internal/guards` gains a test that every `Notify("command"` in production code goes
through `CommandEvent` (an AST walk, like the existing call-site guards), with a must-fail fixture
proving the guard can fail.

**Mobs share the rule.** A mob actor raising the same command (a mob that trips, throws, casts)
builds the same event through the helper; quests never listen for mob actors today, and the
engine's per-user `PlayerState` means a mob event matches no quest trigger, but the event shape is
one.

**Existing triggers unchanged.** A trigger with no `target`, `mob` or `item` matches as today, so
the 49 shipped `command` triggers keep working.

### 3. Slice 3: effect spells (`spell_landed`)

**Event.** `spell_landed`, raised once per target a spell lands on, from the resolver's own
`landed` verdict: `resolveAgainstMob`, `resolveAgainstPlayer`, the self-cast path, and the three
Go-hook spells (fold-anchor, fold-recall, purge-affliction, which land on their target or the
caster when not fumbled). Mob casters raise it through `resolveMobSpell*` with the same shape.
Details: `Noun` = spell id, `Skill` unused, new `Effect string` = the spell's `EffectType`, and the
target fields from slice 2. A miss, fizzle, fumble or defended help spell raises nothing.

**Trigger.** New field `effect` (matched against `Effect`), plus `noun` (spell id), `mob`, `item`,
`target`, `room`, and slice 4's `area`/`zone`. "Mend the scout" is
`event: spell_landed, effect: heal, mob: 9112`.

**`command: cast` stays** as the attempt event ("try casting at the mark" lessons). `events.Healed`
is left alone; it serves the AI companion and means "tried to tend".

**Reachability.** The walkthrough tracks the spells a walking player holds (starters from
`StarterSpells`, plus `teach_spell` actions and `spellid` rewards met earlier on the path,
including quests that gate the offer through `questRequired`). A `spell_landed` way into a step is
reachable when some held spell matches its `effect` or `noun`. An unreachable effect step is a
walkthrough failure (warning list until phase 7, like the walkthrough's other findings). The
writer fixes it by teaching the spell earlier, or by gating the offer on a quest that teaches it
and giving the hidden offer a breadcrumb that names the teacher.

**Effect names are vocabulary.** `EffectType` values used by a shipped `effect:` filter go into a
manifest (`quests/effect-names.txt`); renaming one strands content and the guard says so.

### 4. Slice 4: areas, counts and explore

**Area files.** `_datafiles/world/dogmud/areas/<key>.yaml` (flat: an area may span zones):

```yaml
key: grove_heart
name: the Grove            # player-facing, used by the minimap (phase 4) and the tracker
zones: [pothole_coulee]    # optional: every room in each zone
rooms: [5311, 5312, 5313]  # optional: individual rooms
```

A new leaf package `internal/areas` loads them, resolves each to a set of template room ids at boot
(zones through the loaded rooms' `zone`), and builds the reverse index `roomId -> []areaKey`. The
loader refuses an unknown zone, an unknown room, a key that is not the file name, and an area that
resolves to no rooms; it warns on a listed room already inside a listed zone. Area keys go into the
stable-id manifest (`areas/area-keys.txt`). Ships a `context.md`.

**`area` and `zone` filter every event.** Two new trigger fields. `matchTriggerFields` tests the
event's template room against the area's set (one map lookup) or the room's zone. Quest 45's three
triggers become one with `area:`.

**Counts.** New trigger field `count: N` (N >= 2; 1 is the default and is refused when written).
Each match whose conditions pass increments a saved counter for (quest, step); the trigger's
actions run only when the counter reaches N. The counter is saved on the character in a new map
`QuestStepCounts map[string]int` keyed `"<questId>-<stepId>"`, cleared when the quest's token
advances past that step, and kept when the quest is abandoned (phase 5), like the step token.
Tracker and quest log show "<label>: 2 of 5". A count trigger may carry `label:` for that line;
without one the line uses the step description. (Counts are not among the number classes the
player-copy rules forbid: damage, healing, armour, durations.)

**Explore (`distinct`).** New trigger field `distinct: all|N` on a `room_enter` trigger with an
`area`. It counts distinct rooms of the area on the player's map, so prior mapping counts and an
unseen room counts for nothing. Progress uses the same `QuestStepCounts` entry.
- **Seeded at step start.** When the previous step's token is granted, one pass over the area's
  rooms against `VisitedRooms` (through a set built for the pass) sets the count. If it already
  meets the target, the trigger fires at once and the player reads the step's `explored_line`, or
  by default "You already know every corner of <area name>."
- **Advanced only on a newly mapped room.** `MarkRoomMappedIfSeen` returning true raises a quest
  check for that room; if the room is in an area with an active `distinct` step, the count goes up
  by one. A room already on the map costs nothing.
- **Validator.** `distinct: all` on an area containing a room behind a quest gate or a locked exit
  is a boot error; an area over 40 rooms with `distinct: all` is a warning.

**Map targets are phase 4.** This slice ships the area data, the filters and the counters. Phase 4
adds `map_target_area`, the nearest-room arrow, the in-area outline and the "explored N of M"
fog markers. The seven quest shapes the owner named, for phase 4's reference:

| Quest shape | Resolves on | Map outside the area | Map inside, not done |
|---|---|---|---|
| Go to area | `room_enter` + `area` | Arrow to the nearest area room | Done on arrival |
| Gather mats there | `item_gain` + `item` + `area` + `count` | Arrow | Area outlined; tracker count |
| Kill mobs there | `mob_death` + `mob` + `area` + `count` | Arrow | Area outlined; tracker count |
| Find a specific room | `room_enter` + `room` | Arrow to the area, never the room | Area outlined; room not revealed |
| Explore it fully | `room_enter` + `area` + `distinct: all` | Arrow | Rooms not yet mapped marked; tracker count |
| Find a wandering NPC | the action on that NPC (topic, give, kill) | Arrow to the area | Area outlined; NPC not tracked |
| Rare drop anywhere there | `item_gain` + `item` + `area` | Arrow | Area outlined |

Outlines and fog markers use shape or pattern; green against blue never carries meaning.

### 5. Performance

| Hot path | Work added | Bound |
|---|---|---|
| Any event | `area`/`zone` filter: one map lookup, only on triggers already selected by event type and passing the cheaper filters first | O(1) per candidate trigger |
| `count` | Increment a saved int after conditions pass | O(1), only on a real match |
| Explore | Only when a room is newly mapped; reverse-index lookup | O(areas containing the room), zero on rooms already mapped |
| Explore seed | One pass over the area's rooms | Once per step start |
| `topic_heard` | One Notify per topic played | Once per menu choice |
| Objective marker | `WouldFire` per eligible info topic when a menu or roster marker is built | Per menu build, never per round |
| `spell_landed`, `command` targets | One Notify per affected target | Already per action |

**Measured, not assumed.** Slice 4's PR carries a Go benchmark of `Notify("room_enter")` with the
shipped quests plus 50 synthetic area triggers loaded, before and after, and fails review if the
per-call cost rises more than 10%. The slice 4 playtest records CPU the way the 10-24 lag fix did.

### 6. Docs and help

Each slice updates the `context.md` of every package it touches (`questengine`, `quests`,
`dialogue`, `conversation`, `questloop`, `hooks`, new `areas`), fixes the stale starting-spells
line in `internal/spells/context.md`, documents the new trigger fields in the quest authoring skill
(`dogmud-authoring-quests`) and the GMCP trigger table (`modules/gmcp/gmcp.Quest.go:157`), and adds
new files to `docs/README.md`. The requirements page's rule 14 and the roadmap's phase 3 entry are
amended with this spec.

## Slices and order

1. **Slice 2, targets** (first: slices 1, 3 and 4 reuse its target fields).
2. **Slice 3, effect spells.**
3. **Slice 1, talk** (the 32-topic migration).
4. **Slice 4, areas, counts and explore.**

Each slice: its own plan in `docs/superpowers/plans/`, a worktree, the pre-push gate, a PR, and an
adversarial playtest whose findings are filed before the slice closes.

## Testing

- Each new event or filter: a test that it fires on the real action and does not fire on a
  look-alike (a cast that misses; a topic whose charge refuses; a room entered in the dark; a kill
  outside the area; a command aimed at the wrong mob).
- Guards: `CommandEvent` call-site guard with a must-fail fixture; area-key and effect-name
  manifests with their `_GuardCanFail` twins.
- Walkthrough and `questloop` stay green over shipped quests after the 32-topic migration; a
  fixture quest with an unreachable effect step fails the walkthrough.
- Counts: survive logout, clear on step advance, stay on abandon (abandon lands in phase 5; the
  test pins the storage contract now).
- Explore: seeded count from prior mapping; the already-explored line; dark rooms do not count.
- The slice 4 benchmark above.

## Deferred: observe

If a phase 7 step needs one: named "happenings" raised in a room by content (a behaviour tree
action, a quest `sequence` step) and by a fixed engine set (dawn, dusk, moon phase, weather change,
patrol arrival), firing `observe` for each player there with sight other than none, judged before
the change, or by hearing when the happening says `sense: hearing`. The emitter checks an index of
listened-for keys before fanning out, so an unheard happening costs nothing.

## Out of scope

Minimap area targets, arrows, outlines and fog markers (phase 4); abandon and focus (phase 5);
editors (phase 6); porting content beyond the 32 topic grants and quest 45 (phase 7); observe.
