# /new-quest

Generate all files for a planned quest from an approved sketch. Reads a quest
plan markdown file and creates/modifies all listed files.

## Instructions

You are generating all game files for a DOGMud quest from an approved plan.
Follow these steps exactly.

### Step 1 — Validate argument

`$ARGUMENTS` must be a filename (with or without path). Look for it in:
1. `_datafiles/quest_plans/$ARGUMENTS`
2. `_datafiles/quest_plans/$ARGUMENTS.md` (append `.md` if missing)
3. The literal path if it looks like a full path

**If no argument is provided or the file is not found:**

> No quest plan file specified. Create a plan first with:
> `/sketch-quest "your quest concept here"`
>
> Then run `/new-quest <plan-filename>` to generate the files.

### Step 2 — Load and parse the plan

Read the plan markdown file. Extract:
- Quest ID, name, steps, rewards
- File list (creates and modifications)
- Step chain with trigger mechanisms
- Alternative paths
- Gotchas checklist items

### Step 3 — Load context

Read:
1. `docs/world.md` — lore, tone, world context
2. `docs/schemas/dialogue.md` — dialogue field reference
3. All existing files that will be modified (read each one before editing)
4. Relevant schema docs for any items/mobs being created:
   - `docs/schemas/item.md` if creating items
   - `docs/schemas/mob.md` if creating mobs
   - `docs/schemas/room.md` if modifying rooms

### Step 4 — Generate files in order

Work through the file list from the plan. For each file:

**4a. Quest YAML** — create `_datafiles/world/dogmud/quests/{id}-{name}.yaml`
with steps and rewards from the plan.

**4b. Items** — create any new quest items under the appropriate
`_datafiles/world/dogmud/items/` subdirectory.

**4c. Dialogue files** — create new or modify existing dialogue YAMLs, adding
quest topics with proper gating (`docs/schemas/dialogue.md`). For each
dialogue file:
- The quest offer is a `kind: offer` topic with `quest: <id>`, the quest's
  title as its `label`, a `playerLine`, and `grantsQuest` set to the quest's
  start token. Each hand-in is a `kind: handin` topic granting the end
  token. Other steps' conversation is `kind: info` topics.
- Use `questRequired` and `questExcluded` for proper step gating
  - **End-token exclusion:** every `grantsQuest` topic must exclude BOTH
    the granted token AND `{questid}-end` in `questExcluded`, or a
    completed quest is re-offered
- Item delivery steps are a topic with `requiresItem` (taken) or
  `carriesItem` (shown, kept); `give <item> <npc>` plays it. A quest
  `item_give` trigger may not grant a token (the boot refuses it).
- Use `heardRequired` for "ask about X before Y" within one NPC; heard
  topics are saved, so it never bricks a quest.
- **Narrative voice:** NPC `text` must use first person ("I", "my"). Hints
  must describe player options from the player's perspective. Never write
  3rd-person self-references like "Ask about why she left" — write "You
  could ask why she left" or "You could ask about the marriage."
- Labels: at most 50 characters, `playerLine` at most 70, no em or en dash,
  no semicolons in spoken text. The boot validators check all of it.

**4d. Room behavior trees** — create room behavior tree handlers for item
pickups / command intercepts if needed. Confirm the noun appears in the room's
`description` or `nouns` section so `get <noun>` feels natural.

**4e. Behavior trees.** Item hand-ins are dialogue topics (4c), never
behavior trees or quest triggers. `give <item> <npc>` asks the NPC's topics
first, BEFORE any transfer:
- An eligible topic that wants the item plays as if chosen from the menu.
- A topic wants it but none is eligible: the item stays with the player. A
  `player_give` branch with `item_matches` for that item, using only speech
  and `return_item`, can say why.
- No topic wants it: `give.go` transfers the item BEFORE the tree's
  `player_give` fires, so a handler can only react after the fact, typically
  with `return_item`.
- Quest givers who hand out items via `givesItem` need a recovery topic
  (open when the player has the quest but not the item) that gives a
  replacement. `itemExcluded` keeps it to one while the player carries one.

**4f. Mob YAML modifications** — group changes, spawninfo additions. Verify
quest NPCs are NOT in hostile mob groups.

**4g. Room YAML modifications** — spawninfo, nouns, exits. Check for matching
instance saves in `rooms.instances/` before modifying.

Before writing each file, run the relevant gotchas checklist items from the
plan to verify correctness.

### Step 5 — Post-generation checklist

After all files are written:

1. Run `go build ./...` to verify no compilation errors, then
   `DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_' -count=1`: the dialogue
   validators refuse every rule in 4c
2. **Check every step (but the final one) has a destination.** Each step
   either declares a `destination:` block or lets inference find one from the
   triggers gated on it. The shapes: `destination: {room: <id>}` (a place or
   fixture), `{area: <key>}`, `{zone: <Zone Name>}`, `{mob: <id>}` or
   `{mob: [<id>, <id>]}` (go see an NPC; any one of a list will do), a mob
   with one of `room`, `area` or `zone` as its fallback, or
   `destination: anywhere` (done wherever the player stands). With no block,
   the destination is inferred from every place field (`room`,
   `conditions.in_room`, `mob`, `area`, `zone`) of the step's triggers. Rules and rationale:
   `/sketch-quest`, "MAP DESTINATION".
   - The quest loop sweep (`DOGMUD_BOOT_SMOKE=1 go test . -run
     TestSmoke_ShippedQuestsPassTheLoop -count=1`) runs the destination rule:
     a step that resolves to nothing is a `walk-destination` finding, and an
     `anywhere` step whose triggers name a place is a
     `walk-destination-anywhere` finding. A new finding fails CI.
   - Boot refuses an unknown room, mob, area or zone in a block
     (`ValidateQuestRefs`), and `TestQuestFilesCarryNoMapTargetKey`
     (`internal/guards`) fails on any leftover `map_target` key.
3. List all instance saves that need deletion:
   ```
   _datafiles/world/dogmud/rooms.instances/{zone}/
   _datafiles/world/dogmud/mobs.instances/{zone}/
   ```
4. Delete stale instance saves (after confirmation from the user)

Remind the user:

> Quest files generated. Restart the server and test with the verification
> plan from the sketch document.

### Step 6 — Print verification plan

Reproduce the verification plan from the sketch so the user can test
immediately:

1. Restart server
2. Walk to quest giver → `talk <npc>` → choose the quest → confirm quest granted
3. For each step: perform the action → confirm quest advances
4. For each alternative path: test that too
5. Try to skip steps (go to completion NPC early) → confirm blocked
6. Complete quest → confirm rewards

---

## Usage

```
/new-quest 5-the_innkeepers_complaint.md
/new-quest 14-the_lost_caravan.md
```

`$ARGUMENTS` is the filename of a quest plan in `_datafiles/quest_plans/`.
Create a plan first with `/sketch-quest` if you don't have one.
