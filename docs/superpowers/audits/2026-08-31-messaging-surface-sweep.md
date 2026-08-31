# Messaging Surface Sweep (M0)

This is M0 of the messaging unification arc. The arc exists because curated
inventories rot: a "verified" claim that quell had no messages survived two
weeks past its own fix, and a hand-built store list missed `idlemessages`, the
largest narration surface in the game. So this sweep was produced mechanically
by four independent detectors, and this document is its record, not a summary
written from memory.

## 1. How this was produced

Date: 2026-08-31. Every number below comes from running these commands against
the repo at the tip of `feature/messaging-m0-sweep`:

```bash
python tools/messaging_surface_audit.py > /tmp/sweep.txt
python tools/messaging_surface_audit.py --json > /tmp/sweep.json
go test -run TestEveryTextSurfaceIsRegistered . -v
```

`tools/messaging_surface_audit.py` is a stdlib-only Python walker (no YAML
library, filename and line scanning, the same approach as
`tools/id_inventory.py`). It is read only: it makes no changes to the repo.
Re-running it should reproduce this table exactly, or the difference is a real
finding worth investigating, not drift in the tool.

`messaging_surface_guard_test.go` (repo root) holds a second, independent walk
of the same world data and a registry, `textSurfaceRegistry`, that classifies
every key spelling the walk finds in 2 or more files. The test
`TestEveryTextSurfaceIsRegistered` fails in both directions: a new
unregistered spelling fails the build, and a registered spelling that stops
appearing also fails. This is what keeps the inventory from rotting the way
the quell claim did. It currently passes.

The guard is a second, independently written walk of the same data, not a
reader of this tool's output, so the two are allowed to disagree. As of this
sweep they agree on the schema/content split for every key.

## 2. The four methods and what each is blind to

| Method | Finds | Blind to |
|---|---|---|
| A, key names | keys whose *name* looks textual (`*_text`, `*message*`, an audience word like `toattacker`); YAML only (`.yaml`, `.yml`) | a key with an unguessable name (`blurb`, `caption`, `flavor`) |
| B, ANSI markup | any string containing `<ansi ...>` markup, wherever it lives; reads `.yaml`, `.yml` and `.template` | plain, uncolored player text |
| C, prose shape | a value that reads as a sentence (4 or more words, terminal punctuation), attributed to the nearest key; YAML only (`.yaml`, `.yml`), deliberately does not read `.template` | short fragments, single words, token only strings |
| D, Go tree walk | raw event queue pushes, token entry points, send helpers, `Category` constants, config wrappers, and ANSI markup inside Go string literals, across `internal/` and `modules/` | text authored in a data file but never sent through one of these matched Go patterns; plain (uncolored) Go string literals |

Method A is the only one filtered to "looks like a schema key by name," which
is why its output can be split into schema and content buckets by file count
(section 4). Methods B and C have no such filter: their key space is
unbounded author text, which is exactly why they catch what A cannot, and
exactly why their "found in 2+ files" bucket needs a different reading
(section 5).

## 3. The key table

41 key spellings appear in 2 or more files in `_datafiles/world/dogmud` by
Method A and are therefore schema, per the rule in section 4. Every one of
these 41 has a matching entry in `textSurfaceRegistry` in
`messaging_surface_guard_test.go`, one row per registry entry, sorted by count
descending so a re-run can be diffed against this table directly.

| Key | Count | Scope | Directories |
|---|---:|---|---|
| `description` | 3338 | content | dogmud, achievements, biomes, buffs, types, +140 more (145 total) |
| `text` | 2231 | content | archetypes, crash_site_interior, dustwalk_road, eastern_highlands, ferries, +51 more (56 total) |
| `idlemessages` | 1285 | narration | amber_valley, ashwick, cascade_pass_road, crash_site_interior, dustwalk_road, +40 more (45 total) |
| `hints` | 1266 | content | dogmud, amber_valley, ashwick, cascade_pass_road, dustwalk_road, +30 more (35 total) |
| `toattacker` | 209 | narration | combat-messages, defense-messages, taunt-messages |
| `todefender` | 209 | narration | combat-messages, defense-messages, taunt-messages |
| `hint` | 200 | content | quests |
| `toroom` | 192 | narration | combat-messages, defense-messages, taunt-messages |
| `together` | 188 | narration | combat-messages, defense-messages |
| `greetings` | 186 | content | amber_valley, cascade_pass_road, east_road_to_greenford, greenford, new_plymouth_common, +11 more (16 total) |
| `lines` | 159 | narration | pairs, types, itemvoices, quests, ironwind_steppe, pothole_coulee |
| `send_text` | 134 | narration | quests |
| `failure_message` | 126 | narration | alchemy, blacksmithing, cooking, enchanting, jewelcrafting, tailoring |
| `success_message` | 126 | narration | alchemy, blacksmithing, cooking, enchanting, jewelcrafting, tailoring |
| `description_suffix` | 95 | content | enchantments |
| `tier_up_message` | 95 | narration | enchantments |
| `message` | 76 | narration | dustwalk_road, endless_trashheap, labyrinth_of_low_tunnels, pothole_coulee, stillwater, +2 more (7 total) |
| `npc_say` | 69 | narration | quests |
| `playermessage` | 66 | narration | quests |
| `cast_user_text` | 58 | narration | spells |
| `cast_room_text` | 57 | narration | spells |
| `end_user_text` | 56 | narration | buffs |
| `roommessage` | 56 | narration | quests |
| `start_user_text` | 56 | narration | buffs |
| `optionid` | 30 | narration | combat-messages, defense-messages, taunt-messages |
| `options` | 30 | narration | combat-messages, defense-messages, taunt-messages |
| `start_room_text` | 30 | narration | buffs |
| `room_text` | 23 | narration | marches_spur_road, quests |
| `descriptionmodifier` | 21 | content | mutators |
| `wait_user_text` | 18 | narration | spells |
| `separate` | 17 | narration | combat-messages |
| `trigger_user_text` | 15 | narration | buffs |
| `hidden_description` | 12 | content | kingsbarrow_vale, new_plymouth_sewers, pothole_coulee, stillwater, the_fernway_south, the_foldweave |
| `trigger_room_text` | 7 | narration | buffs |
| `end_room_text` | 6 | narration | buffs |
| `washing lines` | 3 | content | new_plymouth_common, new_plymouth_docks, new_plymouth_old_quarter |
| `on_taunt` | 2 | narration | itemvoices |
| `user_text` | 2 | narration | thornwall_city |
| `voice_id` | 2 | config | items/materials-40000 |
| `voiceid` | 2 | config | itemvoices |
| `wait_room_text` | 2 | narration | spells |

Scope split: 30 narration, 9 content, 2 config, summing to the 41 schema keys.
This agrees with `textSurfaceRegistry`, which is the point: they are one
statement in two places, and if a row here looked wrong the fix would be to
change both together, not just the document.

## 4. The schema and content split, and why it exists

Method A finds 89 distinct key spellings in total. A spelling that appears in
2 or more files is schema: some loader reads it and it recurs across files by
construction. A spelling that appears in exactly one file is content: an
author invented it once and it is not the registry's job to hold it. 48
spellings fall into the content bucket at the Method A level (things like
`buff-text` and `expiremessage`, single files in `ansi-aliases.yaml` or a
single buff).

The reason this split has to exist at all is room `nouns:` children. A room's
`nouns:` block maps author chosen phrases (`dragonfly:`, `hunt pool:`) to the
prose shown by `look <noun>`. There are thousands of these across the world
files, one distinct spelling per phrase, almost always confined to a single
room file. A registry keyed on spelling cannot hold thousands of one-off noun
phrases and should not try. The 2-file threshold is what keeps
`textSurfaceRegistry` a few dozen lines instead of several thousand.

## 5. Reconciliation, what each method found that the others missed

This is the section the four-method design exists for. Running four
detectors is worthless if their disagreements are quietly averaged away.

**Found by name only, no ANSI and no prose (42 keys).** Expected: structural
selector keys (`optionid`, `together`, `separate`), plain-enum style strings
(`taunt-success`, `taunt-failure`), and a handful of `.yaml` keys and prose
snippets that happen to also match a Method A stem
(`descriptionmodifier`, `greetings`, `npc_say`, `voice_id`, `voiceid`) but
carry no color and are too short or not sentence shaped to trip Method C.
Nothing in this bucket needed a new registry entry; each is already accounted
for.

**Found by ANSI but not by name (16 keys).** This is the bucket that proves
Method B's reason for existing: `<no-key>`, `aliases`, `beginner`, `commands`,
`counter`, `defender`, `example`, `examples`, `expert`, `master`, `materials`,
`motd`, `note`, `station`, and two prose fragments from the help system
(`as a shortcut to quickly issue to a command as follows`,
`this skill improves through use`). These are almost entirely `help.yaml`
structural fields (skill tier labels, command aliases, worked examples) whose
key names give no hint they carry colored player text. A name based detector
alone would never have found this surface.

**Found by prose but not by name: 394 schema-level spellings, grouped by
directory.** Method C's schema bucket (values that read as prose, key
spelling repeated in 2 or more files) totals 422 keys; 394 of them are new
relative to Method A. This is not 394 separate findings. Grouped by source
directory, the picture is unmistakable:

| Keys found (this directory) | Directory | Example nouns |
|---:|---|---|
| 266 | `_datafiles/world/dogmud/rooms/pothole_coulee` | air, altar, anvil, arch, barricade |
| 151 | `_datafiles/world/dogmud/rooms/stillwater` | air, alcove, altar, anvil, arch |
| 137 | `_datafiles/world/dogmud/rooms/ironwind_steppe` | alcoves, altar, archway, banks, bed |
| 119 | `_datafiles/world/dogmud/rooms/the_confluence` | alchemy bench, cooking fire, cut, dark, enchanting circle |
| 118 | `_datafiles/world/dogmud/rooms/greenford` | inn sign, the anvil, the barge, the benches, the books |
| 93 | `_datafiles/world/dogmud/rooms/thornwall_city` | alcove, altar, anvil, bar, barrels |
| 92 | `_datafiles/world/dogmud/rooms/amber_valley` | hush, light, the counter, the fig tree, the gate |
| 92 | `_datafiles/world/dogmud/rooms/stillwater_marsh` | bank, board trail, bone pile, cattails, kettle |
| 73 | `_datafiles/world/dogmud/rooms/the_fernway_south` | bone pile, droppings, foxglove, lean-to, lockbox |
| 52 | `_datafiles/world/dogmud/rooms/hartcharn` | alchemy bench, anvil, bar, barrel, board |

...and 152 more directories, almost every one of them a `rooms/<zone>`
directory. Verified by spot check: `air:`, `altar:` and `anvil:` in
`_datafiles/world/dogmud/rooms/pothole_coulee/5200.yaml` all sit under a
`nouns:` block. **This is one surface, the `nouns:` surface, with thousands of
author-chosen children, showing up as hundreds of sibling key spellings.**
Counting it as 394 findings would be as wrong as counting it as zero, which is
why `textSurfaceRegistry` registers `nouns:`-shaped drift only once, if at
all, rather than per phrase. Summed across all 162 directories this grouping
touches (schema and single-file content combined, minus anything Method A
already found), it accounts for 2,506 key-in-directory occurrences.

A number worth flagging rather than quoting from an earlier draft: the design
spec's working notes, taken mid-build, once measured Method C's schema bucket
at 433 keys against 1,556 content keys. This sweep's own run measures 422
schema and 1,441 content. The tool changed between that note and this run (a
scope fix that stopped Method C from reading `.template` files, per the plan's
Task 3), so the two numbers are not the same measurement and the difference is
not a regression. Trust the number in this document; it is what the current
tool, run today, actually reports.

**Go files with ANSI text and no data file behind them: 275 files, 2,257
matching lines.** These are invisible to every data-side method (A, B and C
all walk `_datafiles/`, not `internal/` or `modules/`). A sample: the
`internal/actions/` package alone contributes `buy.go`, `consider.go`,
`defuse.go`, `emote.go`, `forage.go`, `melee_target.go`,
`mutation_cocoon.go`, `mutation_venom_coat.go`, `plant.go`, `salvage.go`,
`say.go`, `scan.go`, `search.go`, `sell.go`, `shadow.go`, `sleep.go`,
`sleeping_target.go`, `sneak.go`, `steal.go` and `track.go`, nineteen files
in one package, and that is before counting `internal/combat/`,
`internal/behaviortree/`, `internal/hooks/` and the rest.

## 6. The delivery layer

**Send helpers: 4 distinct names, 8 call sites, 6 files.** `SendText` is
implemented on four different types (`*MobActor`, `*UserActor`,
`*GameBridge`, `*Room`, `*UserRecord`, five sites total), plus `Room` also
implements `SendTextVisual` and `SendTextVisualToUser`, and
`internal/textutil/spelltext.go` has the standalone `SendPhaseText`. Verified
by reading every site the tool reported:

```
internal/actions/actor_mob.go:43       func (a *MobActor) SendText(...)
internal/actions/actor_user.go:42      func (a *UserActor) SendText(...)
internal/questengine/bridge.go:200     func (b *GameBridge) SendText(...)
internal/rooms/rooms.go:279            func (r *Room) SendText(...)
internal/rooms/rooms.go:307            func (r *Room) SendTextVisual(...)
internal/rooms/rooms.go:346            func (r *Room) SendTextVisualToUser(...)
internal/textutil/spelltext.go:21      func SendPhaseText(...)
internal/users/userrecord.go:435       func (u *UserRecord) SendText(...)
```

**Token substitution: 2 engines.** `internal/textutil/tokens.go:18` declares
`SubstituteTokens` as a plain function. `internal/items/attack_messages.go:55`
declares `SetTokenValue` as a method with a receiver,
`func (am ItemMessage) SetTokenValue(...)`, not a plain function. Both
engines share vocabulary (`{source}`, `{target}`) with different meanings
depending on which one renders.

**Raw pipeline bypass: 13 sites.** `events.AddToQueue(events.Message{...})`
called directly, skipping `Category`, color, normalization, the sight gate
and verbosity entirely. 6 are in `internal/characters/progression.go` (the
known bypass), 5 are the pipeline's own plumbing in
`internal/rooms/rooms.go`, and 1 each are in `internal/usercommands/print.go`
and `internal/users/userrecord.go`.

**Config-driven wrappers: 21 sites** matching `(Enter|Exit)RoomMessageWrapper`.
10 are in `internal/configs/config.textformats.go` (the wrapper definitions
themselves), 4 are in `internal/mobcommands/go.go`, and 7 are in
`internal/usercommands/go.go`.

**Message categories: a name-collision correction.** The tool's mechanical
detector for `Category` constants (a line starting with a tab, then
`Category` and an uppercase letter) reports 83 matching lines across
`internal/` and `modules/`. A direct count of actual `Category` constant
declarations in `internal/messaging/messaging.go` is 60, confirmed by
`grep -c`. The remaining 23 lines are name collisions the pattern cannot
distinguish from a real declaration: `internal/achievements/achievements.go`
and `internal/items/affixgen.go` each declare their own, unrelated
`Category`-prefixed constants, and `internal/messaging/verbosity.go` contains
`case Category...:` lines that reference the already-declared values rather
than declaring new ones. **60 is the correct count of message categories.**
This is recorded here rather than silently fixed in the tool, because it is
itself a finding about the limits of a name-pattern detector (see section
10).

## 7. Corrections to earlier claims

The messaging unification design spec was written from a first pass of
grepping and hand counting before this mechanical sweep existed. Several of
its numbers were already wrong the day it was written, and re-verifying
against this sweep's tool output confirms (rather than merely repeats) the
corrections the spec itself later recorded:

- **The store count grew.** The design's curated table lists 14 narration
  stores. This sweep's mechanical key enumeration finds narration-shaped
  surfaces that table does not mention at all: `idlemessages`
  (1,285 occurrences across 45 directories, room and zone ambient flavor),
  the room `SpawnInfo.Message` field (`message`, 76 occurrences across 7
  directories), and behavior-tree `user_text` / `room_text` action params
  (2 occurrences in `thornwall_city`, and a further 1 in
  `marches_spur_road/275-old_edrin.yaml` for `room_text`, distinct from the
  22 quest-engine `room_text` occurrences that share the same spelling). None
  of these five surfaces are rows in the 14-store table. A curated list
  cannot grow on its own; only a mechanical re-count catches this, which is
  the whole argument for M0 existing as a slice rather than a preamble.
- **`SetTokenValue` is a method, not a plain function.** Verified above in
  section 6 by reading the declaration directly, in
  `internal/items/attack_messages.go`:
  `func (am ItemMessage) SetTokenValue(tokenName TokenName, ...) ItemMessage`.
  A detector that only matches `func Name(` is structurally blind to it,
  which is exactly what an early draft of this tool did.
- **Send helpers number more than three.** Verified above in section 6: 4
  distinct names, 8 call sites, 6 files, not the "three send helpers" an
  early design pass counted.
- **"54 Go files with in-code text pools" was far too low.** That figure
  counted only `[]string{` slice literals. The Go tree walk in this sweep
  (Method D) instead matches any `<ansi ...>` markup inside a `.go` string
  literal, and finds 275 files across 2,257 lines, five times the earlier
  estimate. The earlier figure was not wrong arithmetic, it was measuring a
  narrower thing (in-code string-slice pools) and labeling it as if it
  covered all in-code player text.

## 8. Drift and consolidation targets found

- **`voiceid` versus `voice_id`.** The same concept, spelled two different
  ways in two schemas that must agree for sentient-item chatter to resolve.
  `internal/items/itemspec.go:275` tags `ItemSpec.VoiceId` as
  `yaml:"voice_id,omitempty"` (underscore); the field of the same name at
  `internal/itemvoices/itemvoices.go:38` tags `VoiceSpec.VoiceId` as
  `yaml:"voiceid"` (no underscore). Neither value is player prose; both are
  foreign-key identifiers, so both are filed as `config` scope in the table
  above. This drift is a consolidation target for a later stage of the arc,
  not something this sweep fixes.
- **`text` and `hints` each carry two scopes at once.** `text` is
  overwhelmingly dialogue content (286 dialogue files, NPC speech read via
  `talk`/`ask`), but the identical spelling is also genuine narration
  elsewhere: `internal/behaviortree/actions_dialogue.go`'s `text` action
  param (say and emote actions, 44 behavior files), `ConversationLine.Text`
  in `internal/conversations/conversation.go` (18 files), and
  `SayLineDef.Text` inside quest `npc_say` triggers. `hints` is dominated
  (286 of 287 files) by `internal/dialogue/types.go`'s `Hints` field, narrator
  perspective option text read on request, but the single top-level
  `_datafiles/world/dogmud/hints.yaml` file (verified above: a plain
  `hints:` list, no per-NPC nesting) reuses the identical spelling for
  periodic gameplay tips broadcast every few minutes, which is narration
  shaped, not content read on request. **This is a real limitation of a
  registry keyed on spelling: scope depends on the store a key lives in, not
  the key's name.** Both are filed as `content` in the table above because
  the majority use dominates, with the minority use noted as a known gap.
  This is an input to the arc's eventual schema design, not something a
  spelling-keyed registry can resolve on its own.
- **`washing lines` reached the schema bucket by coincidence, not by
  construction.** It is a room `nouns:` child (author-chosen phrase, not a
  loader-owned field) that happens to appear verbatim in three unrelated room
  files: `rooms/new_plymouth_common/5613.yaml`,
  `rooms/new_plymouth_docks/5519.yaml` and
  `rooms/new_plymouth_old_quarter/6033.yaml`. It crossed the 2-file threshold
  purely because three authors independently chose the same noun phrase, not
  because any loader recurs on it. Filed as `content` in the registry with
  this noted explicitly, rather than treated as a real recurring schema
  surface.

## 9. What a curated inventory missed

This is the evidence for why M0 had to be mechanical rather than reviewed by
eye:

- **`idlemessages`, 1,285 occurrences across 45 directories.** The single
  largest narration surface in the game, and it does not appear anywhere in
  the design spec's original hand-built store table.
- **The room `nouns:` surface, thousands of author-chosen children.** Method
  C alone finds 394 schema-level spellings and 1,441 single-file spellings
  that Method A's name-based filter cannot see at all, because neither the
  container key `nouns` nor its children contain any of Method A's stems.
  Grouped by directory this is 2,506 key-in-directory occurrences across 162
  directories, almost entirely `rooms/<zone>` (see section 5's table).
- **Behavior-tree `user_text` / `room_text`.** 2 occurrences of `user_text`
  (both in `thornwall_city`) and 1 behavior-tree occurrence of `room_text`
  (`marches_spur_road/275-old_edrin.yaml`), driving a mob's scripted speech
  or emote when a behavior-tree event fires. Nothing in a store-name-based
  inventory would think to look inside `behaviors/`.
- **The dash-prefixed quest keys `npc_say` and `send_text`, zero plain-form
  occurrences.** Both are list items (`- npc_say:`, `- send_text:`) inside
  `internal/quests/triggers.go` trigger definitions. Verified directly: running
  a line-start-anchored key regex (`^\s*([a-z_][a-z0-9_]*):`), the same shape
  the very first version of this tool used, against every file under
  `_datafiles/world/dogmud/quests` returns zero matches for `npc_say`,
  `send_text` or `room_text`. The dash prefix (`- npc_say:`) put the key past
  a line-start anchor, so these two surfaces (69 and 134 occurrences
  respectively once found) were **completely invisible**, not merely
  undercounted, until the key regex was widened to match after a sequence
  dash.

## 10. Known limitations of this sweep

- **The 2-file schema threshold mis-sorts coincidentally-repeated room
  nouns.** `washing lines` (section 8) is the concrete example: a one-off
  author phrase that happened to be chosen independently in three room files
  reads as a recurring schema key even though no loader treats it specially.
  The threshold is a heuristic proxy for "a loader owns this," not a proof of
  it.
- **Multi-line double-quoted YAML scalars can still produce a spurious key.**
  The tool's block-scalar handling covers the `key: |` and `key: >` literal
  and folded scalar styles, but a double-quoted scalar that wraps onto a
  following physical line (`send_text: "…` continuing unquoted on the next
  line) is a narrower variant of the same problem that was deliberately left
  unhandled, per the plan's Task 3 notes, unless a future run's reconciliation
  bucket gets noisy enough to justify it.
- **Method C does not read `.template` files, by design.** Help, splash and
  login prose live in `.template` files with no YAML key structure, so
  "attribute this prose to the nearest key" is meaningless there; it produced
  junk keys like a prose line that happens to end in a colon before a bullet
  list. Method B already covers `.template` files via ANSI markup, so they
  are not silently skipped, only excluded from prose-shape detection
  specifically.
- **The guard test and the Python tool are two independent walks that
  happen to agree today, not two views of one walk.** The Go test
  `messaging_surface_guard_test.go` deliberately re-implements its own walk
  of `_datafiles/world/dogmud` rather than reading this tool's JSON output,
  so that a single implementation bug cannot silently pass both the human
  report and CI at once. They are not required to agree on every future run;
  if they diverge, that divergence is itself the finding to investigate, not
  a bug to reconcile away by making one match the other.
- **The mechanical `Category` constant detector overcounts by name
  collision.** See section 6: 83 matched lines, 60 real declarations, 23
  false positives from unrelated `Category`-prefixed constants in other
  packages and switch-case references. Any Go-side detector built on a bare
  name pattern rather than an AST parse inherits this class of error.
- **`_datafiles/world/default/` is not walked at all.** All three data-side
  methods (A, B, C) are hardcoded to `_datafiles/world/dogmud`. A parallel
  `_datafiles/world/default/` tree exists alongside it (confirmed present,
  containing its own `ansi-aliases.yaml`, `buffs/`, `combat-messages/` and
  more) and is completely outside this sweep's scope. Any drift between the
  two trees is invisible to every method in this document.

## References

- Tool: [`tools/messaging_surface_audit.py`][tool]
- Guard: [`messaging_surface_guard_test.go`][guard] (repo root)
- Design: [`2026-08-31-messaging-unification-design.md`][design]
- Plan: [`2026-08-31-messaging-m0-sweep.md`][plan]

[tool]: ../../../tools/messaging_surface_audit.py
[guard]: ../../../messaging_surface_guard_test.go
[design]: ../specs/2026-08-31-messaging-unification-design.md
[plan]: ../plans/2026-08-31-messaging-m0-sweep.md
