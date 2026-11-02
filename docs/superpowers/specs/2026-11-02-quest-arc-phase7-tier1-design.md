# #375 phase 7, tier 1: past the arch

**Status:** design approved in brainstorm 2026-11-02; awaiting owner review of this file.
**Arc:** #375 quest and dialogue rework, phase 7. Roadmap:
`docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`; target page:
`docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`.
**Zones:** Ironwind Steppe, Thornwall City, Thornwall Outskirts, Watchers Crossing, Dustwalk Road,
Marches Spur Road. **Quests:** 4 to 15.

## Owner rulings this design rests on (2026-11-02)

1. **Walking order.** The remaining zones follow the way a player walks out of the Coulee. Veyra
   does not pull the Confluence forward, because no one but the owner has found her.
2. **Hand-holding ends at the Coulee.** Past Esk's arch there is no bridge trail and no hub. A zone
   pass fixes the quests and keeps the chains that exist. "They need to learn to dig after getting
   the basics anyways." This is how requirements rule 5 ("new areas hold the player's hand") reads
   from here on: the Coulee is the new area that does it.
3. **Esk's encouragement is subtle and generic.** He leans on the Chrysalis and what it does to
   people. He names no place, no mystery and no NPC.

## Facts verified against source (master `45ebaf305`, 2026-11-02)

| Fact | Source |
|---|---|
| Quests 4, 5, 6, 7, 9, 11, 12, 13, 14, 15 carry 39 backlog lines (2, 2, 2, 4, 4, 5, 2, 1, 13, 4); quests 8 and 10 carry none | `quests/loop-backlog.txt` |
| A backlog line with no matching finding fails CI, so fixing a finding forces deleting its line | header of `quests/loop-backlog.txt` |
| Hand-ins need the previous step's token (`one-action` `:prev`), and an item, flag or gold condition on a granting trigger is a finding (`:cond`) | `internal/questloop/rule_oneaction.go:37-46` |
| The hand-ins `evidence_handoff` (83), `tolvahandoff` (84), `dornhandoff` (89), `olenhandoff` (95), `quest_complete_dialogue` (250), `totem_return` (241) and `sample_return` (242) gate on `N-start`, the first step, and each has `requiresItem` | `dialogue/<zone>/<mob>.yaml` |
| `velkhandoff` (94) needs `14-evidence`, but the step before `end` is `report`, which nothing grants | `dialogue/thornwall_city/94.yaml:177-191`; backlog `one-action 14 step report:none` |
| Quests 7, 9, 15 grant their deliver step on `quest_granted` with `has_item` | `quests/7-*.yaml`, `9-*.yaml`, `15-*.yaml` |
| Hint limit is 200 characters. Over it: 6 `start` 222, 6 `report` 249, 11 `start` 239, 11 `sided` 227, 12 `start` 229 | `internal/questloop/rules_loop.go:60`; measured from the quest files |
| No destination and no place on a trigger: 11 `sided`, 12 `ritual`, 13 `extraction` | backlog `walk-destination` lines 190-192 |
| Quest 14 has six `room_interact` triggers on the typed noun `strongbox` in room 498; rule 14 forbids this | `quests/14-the_undertow.yaml` |
| Room 498 has `strongbox` as a noun only, no `containers:` block. Torvan (249) spawns there and carries the key (40034), `itemdropchance: 100` | `rooms/thornwall_city/498.yaml`; `mobs/thornwall_city/249-torvan_cresk.yaml:6,57-59` |
| Item 40034 is `type: key` with no `keylockid`. `unlock` finds a key by `KeyLockId` equal to `<roomid>-<container>` | `items/*/40034-*.yaml`; `internal/usercommands/unlock.go:44-50`; `internal/characters/inventory.go:111-123` |
| Prior art for a key and locked container: Halix's lockbox (room 3040, `keylockid: 3040-lockbox`). `unlock` moves the key to the key ring | `rooms/ironwind_steppe/3040.yaml`; `items/other-0/40-halixs_lockbox_key.yaml`; `unlock.go:70-71` |
| Prior art for an item in a container with a respawn: the tally stick (`container: crates`, `respawnrate: 5 real minutes`) | `rooms/thornwall_city/492.yaml:20-25` |
| Prior art for an item respawning in a locked container: room 503's chest (`difficulty: 20`, `relockinterval: 30 real minutes`, item 20061) | `rooms/thornwall_city/503.yaml` |
| Quests 5 and 6 do not gate on each other: Tolva's file names no `6-` token and Harn's no `5-` token | grep of `dialogue/watchers_crossing/84.yaml`, `86.yaml` |
| `picklock` raises a `command` quest event aimed at the container; `unlock` raises none | `internal/usercommands/picklock.go:405-416`; a grep of `unlock.go` for `questengine` finds nothing |
| Item 40036 (Bribe Ledger) has no room or mob source today; quest 14's triggers give it | grep of `rooms/` and `mobs/` |
| Old saves may hold `11-start` and the side flag without `11-sided`, which is why the hand-ins gate on `11-start` | comment in `quests/11-*.yaml`, `sided` step |
| `stand_with_sylara` / `stand_with_rhett` need `heardRequired` on the terms topic, and the terms topic needs the flag empty, so a flagged player who never heard the terms sees neither | `dialogue/ironwind_steppe/241.yaml:130-175`, `242.yaml:131-173` |
| Step ids leave content through `# retired` or `# renamed` in the manifest; new ones are added | header of `quests/step-ids.txt` |
| Topic keys are pinned; new keys are added to the manifest | `dialogue/topic-keys.txt:444-449` for Esk |
| Dashes in these quest files: 14 lines, all in YAML comments. Tier 1 dialogue: 0 | grep of `quests/`; `dialogue/` per folder |
| Esk's crossing line is the `passage` topic's `text` | `dialogue/pothole_coulee/9107.yaml:58-65` |
| A hand-in that finishes a quest offers the next offer on the same NPC, so 11 to 12 or 13 needs no breadcrumb | `internal/conversation/talk.go:267` (`queueNextOffer`) |

## What the player sees

**Past the arch, the same freedom as today**, with three differences:

1. Esk says the game stops walking them, and says it in the world's own terms.
2. Quest steps in these zones each ask for one thing, and the map points somewhere on all of them.
3. Quest 14's strongbox opens with the same commands as any lockbox.

Nothing else changes: no new quests, NPCs or rewards, and no change to which quests exist or who
gives them.

## Esk's lines

Two new info topics on Warden Esk (9107) and one changed line. The wording below is approved.

**`out_there`**, `marked: true` (shows `[*]`), open to the Opened (`questRequired: [30-end]`).
Label: "What is waiting out there?"

> Nobody can tell you that. The Chrysalis has had a long time to work on the people out there, and
> no two of them have come out the same. You will see things you have no name for. Ask about them.
> Most folk will tell you more than they meant to.

Hint: "Past the arch, nobody sends you to your next quest. Talk to people and look closely at what
is around you."

**`what_it_does`**, `heardRequired: [out_there]`, same gate. Label: "What will it do to me?"

> These eyes came the winter I decided to keep this door. I never asked for them. I meant it, and I
> kept on meaning it, and that is as much as I understand. Whatever you hold to hardest, the
> Chrysalis seems to hear. Be careful what you are certain of.

**`passage`**, `text` becomes:

> So. The coulee has taught you what it can, for now. The rest is out there, for whoever goes
> looking.

Both keys go in `dialogue/topic-keys.txt`. Text is wrapped by the engine. Esk's existing `eyes`
topic stays: it says the eyes brighten when he pays attention, and this topic says when they came.

**Limits on future edits to these lines:** no mention of the crash, the ships or what the Chrysalis
actually is, and no place, directions or NPC names.

## Quest fixes

### Hand-ins follow the last step (4, 5, 7, 9, 15)

Each hand-in's `questRequired` becomes the step before `end` (`4-evidence`, `5-deliver`,
`7-deliver`, `9-deliver`, `15-return`) in place of `N-start`. Each step is then reached only by its
existing `item_gain`.

### An item already carried (4, 5, 7, 9, 15)

`item_gain` cannot fire for an item the player picked up before accepting. Today quests 7, 9 and 15
work around this with a hidden grant, and 4 and 5 by letting the hand-in skip the step. Both go. In
their place, a `quest_granted` trigger with `has_item` and no grant prints one line to the actor:

> You already carry the tithe ledger. Set it down and pick it up again to begin.

Quest 4 has a place step first, so its line reads "Set it down and pick it up again once you reach
the Bandit Hollow." A pickup after dropping raises `item_gain` like any other. The item is named in
each quest's own line. A save already on the start step while holding the item sees no line, because
it accepted before this shipped. Dropping and getting the item still works, and the patch note says
so.

### Quest 14: a real strongbox

Room 498 gains a real locked container, and every `room_interact` on `strongbox` goes.

```yaml
containers:
  strongbox:
    lock:
      difficulty: 20
      relockinterval: 5 real minutes
spawninfo:
  - mobid: 249
  - itemid: 40036
    container: strongbox
    respawnrate: 5 real minutes
```

- Item 40034 gains `keylockid: 498-strongbox`, and its description loses its `--`.
- The `strongbox` noun stays, as on Halix's lockbox, and its text keeps matching the room.
- `unlock` gains a `command` quest event on a successful open of a container or exit, mirroring
  `notifyPicklockQuest`. Both success paths raise it. A refused unlock raises nothing.

Steps, with one action each. `start`, `explore` and `confront` are unchanged.

| Step | Reached by | Hint says |
|---|---|---|
| `loot` | `mob_death 249`, needs `14-confront` (the `missing_item` condition goes) | Type loot torvan. If you already carry the key, drop it and get it again. |
| `unlock` | `item_gain 40034`, needs `14-loot` | Type unlock strongbox. |
| `opened` (new) | `command: unlock` or `command: picklock`, `target: container`, `noun: strongbox`, `room: 498`, needs `14-unlock` | Type get ledger from strongbox. |
| `evidence` | `item_gain 40036`, needs `14-opened` | Unchanged: take it to Velk. |

- `report` moves to `# retired` in `step-ids.txt`: nothing ever granted it, so no save holds it.
  `velkhandoff` already needs `14-evidence`, now the step before `end`.
- `opened` goes in `step-ids.txt`. Its destination is room 498.
- **Picking is allowed.** A skilled player can pick the lock at any time, as with any lockbox.
  One who takes the ledger early uses the same drop-and-get as anyone holding an item too soon.
- **Shared container.** Another player can take the ledger from an open box. It respawns in five
  real minutes, so a player arriving just after waits rather than gets stuck. The tally stick
  already works this way.
- **Lost ledger.** The old false-bottom recovery goes, because the respawn replaces it.
- **Old saves.** A save on `14-confront`, `14-loot` or `14-unlock` continues: the key works once the
  item has its `keylockid`. A save on `14-evidence` holds the ledger, or can get another. A save on
  `14-evidence` whose key was consumed by the old trigger and whose ledger is lost kills Torvan
  again for a new key.

### Quest 11: hand-ins follow `sided`

`totem_return` and `sample_return` require `11-sided` in place of `11-start`.

A save holding the side flag without `11-sided` would lose its hand-in, so each side gains a small
recovery info topic, `reaffirm_sylara` and `reaffirm_rhett`: label "I stand by what I chose.",
gated on `11-start`, the matching `11-branch` value, and excluding `11-sided` and `11-end`. A
`topic_heard` trigger grants `11-sided` from it. A new player never sees it, because hearing the
stand topic grants `11-sided` at once. The existing regression test
`TestQuest11CanBeFinishedAfterTalkingToKaelFirst` is extended to cover the flagged save.

### Hints and map targets

Hints are cut to 200 characters without losing the first direction or the command to type: 6 `start`
and `report`, 11 `start` and `sided`, 12 `start`.

| Step | Destination |
|---|---|
| 11 `sided` | `zone: Ironwind Steppe` (the side decides the room) |
| 12 `ritual` | `room: 3115` |
| 13 `extraction` | `room: 3030` |

### Dashes

The 14 comment lines with dashes in quests 4 to 15 are reworded, as zone pass step 4 asks.

## Breadcrumbs and markers

The only tier 1 offers gated on another quest with no `breadcrumb` are 12 and 13, and both come up
at the hand-in of 11 through `queueNextOffer`. No breadcrumbs are added. No `[*]` decisions change
outside Esk's first new topic.

## Testing

- **The ratchet:** all 39 backlog lines deleted, with the sweep green. No line is added.
- **`unlock` event:** a test beside `picklock_quest_test.go` proves a key opening raises the event
  once, on both success paths, and a refusal raises nothing, each proven able to fail.
- **Quest 14 on shipped content:** the container exists and is locked, the key's `keylockid` matches,
  and no `room_interact` remains in quest 14.
- **Carrying line:** one test that accept with the item prints the line and grants nothing, and that
  a drop and pickup then advances.
- **Esk:** `what_it_does` is hidden until `out_there` is heard, and both are hidden before `30-end`.
  A copy guard is not added; the limits above live in this spec and in a comment beside the topics.
- **Gates before push:** `gofmt`, `go build`, full `go test ./...`, `DOGMUD_BOOT_SMOKE=1` sweep,
  `golangci-lint`, isolated boot.

## Playtest

One adversarial run after the plan's last task, on a fresh instance:

| Seat | Tries |
|---|---|
| Quest 14 | The whole line, then again with a picked lock, and with a second seat taking the ledger first |
| Carrying | Picks up Olen's ledger before talking to Olen, then accepts |
| Steppe | Kael first, then a side, then the hand-in; 12 or 13 offered at hand-in |
| Old save | Quest 11 with the flag set and no `11-sided` (admin fixture) |
| Crossing | A newcomer reads Esk and crosses: does the line make them want to look? |

Findings fixed in the PR when they belong to this pass; the rest are filed.

## Out of scope

- Any bridge quest, hub, or link between areas (ruling 2).
- Tiers 2 to 5, quest 21 and sample quest 1000000.
- #667 and #625, the disc line.
- Tolva and Harn's rivalry stays unlinked; neither quest locks out the other today, and this pass
  does not change that.
- Hard-failing the validators and deleting `room_interact`: phase 7's closing step.

## Done means

All 39 lines gone, quest 14 has no `room_interact`, Esk's lines ship as approved, the playtest ran,
and the roadmap's phase 7 status names tier 1 as done. One PR. `docs/PATCH_NOTES.md` carries a dated
entry, including the note for a player already holding a quest's item on its first step. The
`internal/questengine` `context.md` lists `unlock` beside `picklock`.
