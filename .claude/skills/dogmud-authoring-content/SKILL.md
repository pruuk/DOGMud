---
name: dogmud-authoring-content
description: Use when creating or editing world YAML - rooms, mobs, items, zones. Covers the filename-must-match-name-field panic, the unquoted-colon gotcha, cardinal-exit and noun-key conventions, running tools/id_inventory.py before picking an ID, world coordinate consistency, and the mandatory adversarial playtest gate that every content plan ends with.
---

This skill covers authoring DOGMud world content: rooms, mobs, items, and
zones. It collects the pre-flight ID and coordinate collision checks, the
mandatory adversarial playtest gate every content plan ends with, the
filename and YAML traps that panic the server at boot, the room and zone
geometry conventions, the naming and room-script rules, and the slash
commands that generate the files themselves.

## Before you create a file

Two pre-flight checks belong here: ID collision and coordinate collision.

Run `python tools/id_inventory.py` before creating any new YAML. Lifted
verbatim from CLAUDE.md's "ID Inventory & Collision Prevention" section:

### ID Inventory & Collision Prevention
**Always run `python tools/id_inventory.py` before creating a new YAML.**
The script walks the world data tree and reports per-zone ID ranges,
gaps, and the next free ID per type (rooms / mobs / items / behaviors /
buffs / quests / dialogue). Filename-only parser, no YAML library
needed.

Common invocations:
- `python tools/id_inventory.py --zone stillwater` - focus one zone
- `python tools/id_inventory.py --type rooms` - focus one type
- `python tools/id_inventory.py --alloc rooms 20` - reserve a 20-ID
  block past the global max, for parallel subagent dispatch

**Parallel content-creation strategy.** When dispatching multiple
content-creation subagents in parallel (`/new-room`, `/new-mob`,
`/new-item`, etc.), they will otherwise scan the filesystem at the
same time, see the same "next free ID," and collide. Two options:

1. **Sequential dispatch (default).** Run content-creation subagents
   one at a time. Slower wall-clock but zero collision risk. Use this
   unless wall-time genuinely matters.

2. **Pre-allocated ID blocks.** When parallelism is worth the
   complexity:
   - For each parallel agent, run `id_inventory.py --alloc <type>
     <count>` to reserve a contiguous block.
   - Embed the assigned range in that agent's dispatch prompt
     verbatim ("use rooms IDs in 5101-5120").
   - Each agent picks IDs only from its assigned block. The blocks
     don't overlap by construction.
   - After merge, run the script once more as a detection pass.

Code-only subagents (no YAML creation) can always run in parallel,
this only matters for content tasks.

`id_inventory.py` is a filename-only pass. It will not catch every collision
by itself: [[feedback_verify_ids_before_creating]] documents three real
panics where scanning only the target zone folder, or only the target item
slot subfolder, missed a duplicate ID living elsewhere. Rooms and mobs are
globally unique across every zone folder (mobs including `summons/`), and
armor items are globally unique across every slot subfolder within a
category (`armor-20000/head/`, `armor-20000/body/`, `armor-20000/back/`, and
so on share one numbering pool). Scan the whole category, not just the part
you are adding to.

The second pre-flight is coordinate collision, needed whenever you place a
new room or a new zone. That check is fuller than a one-line summary here
can cover; see `## Room and zone conventions` below for the full-world
coordinate scan and the crawl-based placement rule. Do not skip it just
because `id_inventory.py` came back clean: ID collisions and coordinate
collisions are different failure modes and neither check catches the other.

## The playtest gate

Lifted verbatim from CLAUDE.md's "Content Playtest-Review Gate (SOP)"
section:

### Content Playtest-Review Gate (SOP)
Any plan or task that authors **player-facing content** (rooms, mobs, items,
quests, dialogue, tutorials, onboarding, room prose) MUST end with an **in-game
adversarial playtest-harness review before the work is handed to the user to
playtest**. This is a required final task on every content plan, not an
optional extra.

Boot-clean and "YAML parses" verify the *system*, never the *experience*.
Content defects (instructions buried in room `description:` prose, confusing
or double-rendered prompts, broken/mis-ordered lesson gates, dead-ends,
awkward pacing, wrong NPC voice) are invisible to a boot test and to code
reasoning. They only surface when something plays the content as a confused
human would.

Procedure: run the playtest harness with an explicitly **critical,
adversarial** mandate, e.g. `/playtest local --checkout <abs>
bug-finder 2026-08-03-prepush-sweep.yaml` (or a route/feature-specific goals
file that already has `ephemeral:`). Spawn a fresh character, drive the real
player flow end to end, read every line of in-game output, and report every
usability problem bluntly. Fix what it finds, re-run if needed, and only then
turn it over to the user. Do NOT claim content work "done" on the strength of
a clean boot alone.

## Filenames

This section is the technical `ConvertForFilename()` file-naming derivation
(zone folder names, mob/item filenames), not in-fiction NPC name choice; see
`## Naming and room scripts` for that.

Lifted verbatim from CLAUDE.md's "Data File Naming Convention" section:

### Data File Naming Convention
Before creating any new data file, verify the expected filename from the loader's `Filepath()` method:
- **Zone folder names must use underscores, not hyphens.** The engine derives the expected path by calling `ConvertForFilename()` on the zone's display name (e.g., `"Sanctum Basin"` → folder `sanctum_basin/`). A mismatch causes a startup panic: `filesystem path "..." did not end in Filepath() "..."`. This applies to both `rooms/` and `mobs/` subdirectories.
- Buffs: `{buffid}-{ConvertForFilename(name)}.yaml` - e.g., `name: Stunned` → `2-stunned.yaml`
- `ConvertForFilename()`: lowercase, keep a-z/0-9, drop apostrophes, all other chars → underscore
- Spells: use the `spellid` field value directly as the filename base (no conversion needed)
- Items/mobs follow the same `ConvertForFilename` pattern
- Mismatch between filesystem path and `Filepath()` output causes a startup panic

[[feedback_filename_must_match_name_field]] restates why this matters: the
mismatch is not a build error and not a soft failure. `go build` passes
cleanly either way. The failure is a runtime panic at server boot, so a
file that looks fine and compiles fine can still take the server down the
next time it restarts. Pick the `name:` field first (it is what players
type), compute `ConvertForFilename(name)` from it, and only then name the
file. Do not use a longer filename "for human readability" when the
`name:` field is deliberately short (a mob whose `name:` is "lars" must be
filed as `356-lars.yaml`, not `356-lars_ketilson.yaml`, even if the
character's full name is Lars Ketilson). If the display needs a fuller
form, put that in description or dialogue text, not in `name:`.

## YAML traps

[[feedback_yaml_colon_gotcha]]: a colon followed by a space inside YAML
scalar text (noun descriptions, idle messages, dialogue lines) parses as a
mapping key and panics the server at boot, not at build time. This has hit
production repeatedly during zone builds, including on this agent's own
writes, not only subagent output. Scan every generated YAML file for
unquoted colons in text content before committing: fix them by hand
(replace with a comma or rephrase), never with a bulk find-and-replace,
since a blind sweep will also mangle real YAML keys like `firepit:`. The
block-scalar `description:` and `hidden_description:` fields (using `>` or
`|`) are safe since YAML treats their colons as literal text; the danger is
indented continuation lines under `nouns:` and `hidden_nouns:` keys.

[[feedback_ansi_plural_inside_tag]]: when a room description highlights a
plural noun with `<ansi fg="itemname">...</ansi>`, the trailing "s" (or
"es") must be inside the closing tag, not after it. An orphaned plural "s"
outside the tag renders as highlighted text the player can see but cannot
`look` at, because the noun lookup key is the unsuffixed singular. Either
move the "s" inside the tag so the rendered word matches the noun key
exactly, or add the plural form as its own `nouns:` alias pointing at the
same prose (both fixes can be combined).

## Room and zone conventions

[[feedback_cardinal_exits_only]]: prefer cardinal direction exits (north,
south, east, west, up, down) over enter/leave. Cardinal exits are what
players expect; enter/leave exits work mechanically but confuse players
trying to get into a building. Use up/down for stacked interior rooms that
share the exterior's x/y coordinate, and down into a negative z for
basements and cellars. Reserve enter/leave for cases with no sensible
cardinal direction, such as a portal or a fall.

[[feedback_noun_keys_space_separated]]: multi-word noun keys must be
space-separated (`notice board`), not hyphenated (`notice-board`). The
engine auto-aliases each word in a space-separated key so `look notice`,
`look board`, and `look notice board` all resolve; a hyphenated key only
matches the literal hyphenated string. The ANSI highlight tag in the
description body must use the same space form the noun key uses, so what
the player sees matches what they can type. Hyphens are still fine inside
description prose for adjective compounds (`lake-stone`, `lake-wind`);
the rule is about noun target keys, not prose.

Room and zone geometry must stay internally consistent:

- [[feedback-room-cartesian-consistency]]: no two rooms in a zone may share
  x/y/z coordinates, and exits must be spatially reciprocal (a room reached
  by going north from (x,y) should sit at (x, y+1) or similar, and its
  return exit should go the opposite direction) unless the exit is
  deliberately one-way.
- [[reference_world_coordinate_frame_crawl]]: the whole overworld is one
  connected coordinate frame (roughly 1183 rooms across roughly 37 zones as
  last measured), so a new room's position must be free globally across
  that component, not just within its own zone folder. The consistency
  validator and the web mapper both determine a room's actual position by
  crawling exit deltas from a zone root, not by reading the authored
  `coord:` field: an authored coordinate duplicate is harmless, a crawled
  position duplicate panics the boot. What actually places a new room is
  the exit direction you attach it by. The only authoritative check is a
  boot with `GamePlay.MapConsistencyEnforce: panic`; subagents cannot run
  the server, so precompute coordinates and have the controller boot-verify.
- [[reference_room_coordinate_and_reciprocity_gotchas]] adds two more traps
  discovered while triaging room geometry. First, rooms can carry two
  different coordinate representations: a nested `coord: {x, y, z}` block
  that is not a field on the `Room` struct at all and is silently ignored
  by the loader, and flat top-level `x:`, `y:`, `z:` keys that are the real
  fields the loader reads (compare all three of x, y, and z, and only the
  flat ones, or a triage can file a phantom collision). Second, a one-way
  exit whose destination room has no return exit isolates that destination
  from the crawl entirely, so the reciprocity check on it is silently
  skipped rather than flagged; the tell in a boot log is two separate `New
  Mapper` lines for what should be one zone, one of them suffixed with a
  room id.

[[feedback_zone_coord_planning]]: before placing a new zone, run a
full-world coordinate collision scan, not a spot check of one or two
zones assumed to be adjacent. A prior zone plan checked only its two
expected neighbors, missed two more zones that also bordered the new
coordinate space, and needed a 71-room shift plus a new interlude zone to
untangle after the fact.

## Naming and room scripts

This section is about in-fiction NPC name uniqueness and JS scripting
policy, not the technical filename derivation; see `## Filenames` for that.

[[feedback_no_name_recycling_no_js]] carries two rules from a newbie-area
rework:

1. Check a candidate NPC name against both the existing mob roster and the
   project's novel manuscript before using it. A prior mob was named after
   the novel's protagonist by accident, and a second name collided with an
   existing mob. Near-collisions (Tess, Tessa, Tessara) are also worth
   avoiding.
2. Avoid JS room scripts. The project deliberately removed as much JS
   scripting as it could because debugging it cost more than it was worth.
   When a content beat needs custom behavior, prefer existing data
   affordances (dialogue trees, quest engine, mutators) or existing (or
   new, TDD'd) Go behavior-tree actions before reaching for a JS script,
   and flag the need to the user first if JS still looks necessary.

## Slash commands

Lifted verbatim from CLAUDE.md's "Content Generation Commands" section:

### Content Generation Commands
Use slash commands to generate new data files. Claude automatically loads docs/world.md,
the relevant schema, and existing examples before generating.

- `/new-mob "description"` - generate a mob YAML (+ optional JS stub)
- `/new-room "description"` - generate a room YAML
- `/new-item "description"` - generate an item YAML
- `/zone-sketch "concept"` - plan a new zone (room list + adjacency) before generating rooms
- `/sketch-quest "concept"` - plan a new quest (step chain, gating, files needed) for review
- `/new-quest <plan-file>` - generate all files from an approved `/sketch-quest` plan

Schema reference: `docs/schemas/` (room, mob, item, spell, buff, dialogue)
Full workflow: `docs/guides/CONTENT_GENERATION_GUIDE.md`

After generating any file: restart server. If editing an existing zone, check
`_datafiles/world/dogmud/rooms.instances/` for stale instance saves.

## Sources

- [[feedback_filename_must_match_name_field]]
- [[feedback_yaml_colon_gotcha]]
- [[feedback_cardinal_exits_only]]
- [[feedback_noun_keys_space_separated]]
- [[feedback_verify_ids_before_creating]]
- [[feedback_zone_coord_planning]]
- [[feedback_no_name_recycling_no_js]]
- [[feedback-room-cartesian-consistency]]
- [[feedback_ansi_plural_inside_tag]]
- [[reference_world_coordinate_frame_crawl]]
- [[reference_room_coordinate_and_reciprocity_gotchas]]
