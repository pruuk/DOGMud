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
4. **A quest handles an item the player already has, as a general case.** The NPC notices it and
   says what to do with it, and the engine does the rest. Asking a player to drop an item and pick
   it up again is out.
5. **A quest has to make sense as a story, not only pass the checks.** A zone pass reads where each
   quest's items come from and fixes the quest when the answer is odd. A quest item with an owner
   standing beside it comes from that owner. One the giver would not have can be found first.

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
| Prior art for a check at step start: an explore step fires when its step starts, from `evaluate` on `quest_granted`, through `fire`, with a default line or `explored_line` | `internal/questengine/explore.go:98-133`; `engine.go:177-179` |
| Prior art for a second offer of one quest when an item is carried: Brennan's `disc_quest` (`carriesItem: 40167`) and `disc_tale` (`itemExcluded: 40167`) | `dialogue/greenford/9516.yaml:131-165` |
| Shipped `item_gain` triggers: 21 of 24 have no filter beyond `item`; the other 3 have `reason`, `count` or `room` | parse of `quests/*.yaml` |
| The offer card's first line is always the quest's first step description | `internal/conversation/card.go:39-41` |
| A granted token is queued, so the menu re-shown after Accept reads state before the quest's chain ran; `heldAlso` only adds the granted token | `internal/conversation/talk.go:169-182` |
| A queued re-show already exists: `TalkNextOffer` at priority 45 runs after a hand-in's banner and reward lines (priority 0) and before a menu shown last (priority 50). Whether an accepted offer's chain has run by then is what Hook 3's test proves | `internal/events/eventtypes.go:105-118`; `internal/conversation/nextoffer.go:41-45` |
| Quest 4's `investigate` is granted by room 407's tree and by `room_enter 408`; Tessara's opener already has a `carriesItem: 16` variant | `behaviors/rooms/dustwalk_road/407.yaml`; `quests/4-*.yaml`; `dialogue/dustwalk_road/83.yaml:30-48` |
| Items 21 (crossing ledger) and 29 (tithe ledger) spawn loose in rooms where their keeper stands (421 with Harn, 476 with Pell); item 16 (purse) spawns loose in 408. All three are `not_salable` with no value | `rooms/watchers_crossing/421.yaml:32-34`; `rooms/thornwall_city/476.yaml:29-31`; `rooms/dustwalk_road/408.yaml:26-28`; `items/other-0/` |
| Loot must not lie on the ground; containers are the way | `dogmud-authoring-quests` skill, "Items" |
| Pell's `tithe_ledger` topic already gates on `9-start`, no `9-end`, `itemExcluded: 29`, and says "Help yourself" | `dialogue/thornwall_city/99.yaml:46-60` |
| Harn's `show_ledger` says every coin is accounted for | `dialogue/watchers_crossing/86.yaml:105-112` |
| A topic's `givesItem` goes through `acquire.AcquireItem`, which raises `item_gain`, so a plain trigger advances on it | `internal/conversation/playerstate.go:66-82`; `internal/acquire/acquire.go:45-48` |
| "Type get ledger" and similar lines to rewrite: quest 9 hint, Olen 95 lines 31, 41, 58, Pell 99 line 53, quest 5 hint, Tolva 84 line 41. "Type get purse": quest 4 hint, Tessara 83 line 44 | grep |
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
4. The two ledgers come from the people who keep them, and an NPC who wants something the player
   already has says so.

No new quests, NPCs or rewards, and no change to which quests exist or who gives them.

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

### Where the items come from (ruling 5)

Audit of every item in quests 4 to 15. Only these change:

| Quest | Today | Change |
|---|---|---|
| 5 | Crossing ledger loose in the Tollhouse, with Harn at the counter | Harn lends it |
| 9 | Tithe ledger loose in the Records Office, and Pell says "help yourself" | Pell hands over the temple's copy |
| 4 | Purse loose in the Bandit Hollow | Moves into a `goods` container in the same room |

The freight crate (15), pest sample (7), totem and windstone (11), and everything in quest 14 already
make sense. Quest 6's report and quest 10's notice are already handed over by their NPCs.

**Quest 5.** Room 421 loses its item 21 spawn. Harn (86) gains an info topic, `lend_ledger`, marked,
gated `questRequired: [5-start]`, `questExcluded: [5-end]`, `itemExcluded: 21`, with `givesItem: 21`.
The gate also means a lost ledger can be asked for again. Label: "May I see your ledger?"

> Tolva sent you, I expect. Take it. Read every page, and show it to whoever you like. Every coin is
> in there.

Hint: "Take the crossing ledger back to Innkeeper Tolva at the Crossing Inn."

Harn gives it because he is sure it clears him. Tolva's hand-in then finds no charter in it, so his
pride is what sinks him. The existing `item_gain 21` trigger moves the step. Quest 5's hint, and
Tolva's opener line that the ledger "sits on the counter", now send the player to talk to Harn.

**Quest 9.** Room 476 loses its item 29 spawn. Pell's `tithe_ledger` topic gains `givesItem: 29`, and
its text becomes:

> The tithe ledger? That is a temple document, not a municipal one. But yes, I have a copy. The temple
> files duplicates here as a matter of public record. Here. Whatever you find in there, remember that
> I only file the documents. I do not judge the contents.

Hint: "Take the tithe ledger back to Temple Priest Olen and type talk olen." Quest 9's hint and Olen's
three lines say to talk to Clerk Pell in place of "Type get ledger". The existing `item_gain 29`
trigger moves the step. Olen does not hand it out himself, because he is the one who wants to read it.

**Quest 4.** Room 408 gains `containers: goods: {}`, the purse spawns in it, its spawn `message` goes,
and the room's description marks `goods` as a container the way the barn marks `crates`. Hints become
"Type get purse from goods."

### Hand-ins follow the last step (4, 5, 7, 9, 15)

Each hand-in's `questRequired` becomes the step before `end` (`4-evidence`, `5-deliver`,
`7-deliver`, `9-deliver`, `15-return`) in place of `N-start`. Each step is then reached only by its
existing `item_gain`.

### An item already carried

`item_gain` cannot fire for an item the player picked up before the step began. Today quests 7, 9
and 15 work around this with a hidden grant, and 4 and 5 by letting the hand-in skip the step. Both
go, for three general hooks that any quest in the game can use. After the ledger fixes, only the
purse, the pest sample and the freight crate can be held before their quests in this tier.

**Hook 1: the engine counts an item already held (general, no quest YAML).** A new check,
`heldAtStepStart`, sits beside `exploreAtStepStart` and runs at the same moment. When a step starts,
each `item_gain` trigger of that quest whose conditions pass and whose item the player holds fires at
once, through `fire`, so it is guarded and chained like any trigger.

- It covers only a trigger with no `reason`, `count`, `room`, `area` or `zone`. Those ask how or
  where the item arrived, which a held item cannot answer. That leaves the 21 plain triggers.
- It tells the player why: `held_line` on the trigger, else "You already have the <item name>."
- It works on any step, so the key and the ledger in quest 14 need nothing extra.
- It runs only when a step starts, so a save already resting on a step is handled under "Old saves".

**Hook 2: the NPC notices.** A quest whose first step is getting an item can carry a second offer,
the same pattern as Brennan's two disc offers: one with `itemExcluded: <item>`, one with
`carriesItem: <item>`. A new optional topic field, `objective:`, replaces the first line of the offer
card, because the first step's description ("Find the missing freight crate") is wrong for someone
holding it.
The loader refuses `objective` on anything but an offer.

**Hook 3: the menu comes back with the hand-in on it.** After Accept on an offer with `carriesItem`,
the menu re-shows through a queued event at `TalkNextOfferPriority`, after the quest's chain has run,
so the hand-in shows `[?]` at once. Other offers re-show as they do today.

The player's view, with Peddler Malk:

```
> talk malk
  1. [!] The Peddler's Overdue Freight
You say, "I found a freight crate in a ruined barn."
Peddler Malk says, "That is my freight. I know that forced lock..."
  The Peddler's Overdue Freight: Give the freight crate to Peddler Malk.
  Reward: ...
  1. Accept  2. Leave
You already have the freight crate.
  1. [?] The Peddler's Overdue Freight
```

The three offers in this tier, each with key `<offer key>_carrying` in `topic-keys.txt`:

| NPC, quest | The player says | The NPC says | `objective` |
|---|---|---|---|
| Tessara, 4 | I took this purse off some bandits. | A purse with marks on it. You found their stash already, then. That is the very proof I was about to ask someone to go and get. Hand it over and it goes in my next report. | Give the bandit's purse to Road Warden Tessara. |
| Dorn, 7 | I killed something in your fields and kept a piece. | That is a pest from the south fields, and a big one. I have been meaning to ask someone to bring me a sample, so I can see what the fallow field is feeding. Let me look at it. | Give the pest sample to Farmer Dorn. |
| Malk, 15 | I found a freight crate in a ruined barn. | That is my freight. I know that forced lock. I have waited on it for days and feared the bandits had it for good. Set it down here and let me count what is left. | Give the freight crate to Peddler Malk. |

**Quest 4 loses its `investigate` step**, because a place step before the item would send a player
holding the purse down to the gully for nothing. `start` now reaches `evidence` directly on
`item_gain 16`, which is "go here, then do one action". `4-investigate` moves to `# renamed` as
`4-investigate -> 4-start`. Room 407's tree keeps its text and no longer grants, and it now gates on
`4-start` and missing `4-evidence`. Tessara's `4-investigate` opener variant goes. Her existing
`carriesItem: 16` opener variant stays.

**Old saves.** A save resting on a first step with the item in hand has no step start coming. It
keeps working through the existing `carriesItem` route: quests 4, 5, 7, 9 and 15 each get a recovery
info topic on the giver, shown only to a player holding the start token and the item but not the
deliver token. A `topic_heard` trigger grants the deliver step. Its label is "I have it here." A new
player never sees it, because Hook 1 moves them past that state first.

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
  `notifyPicklockQuest`. Both success paths raise it. "You do not have the key" raises nothing.
- `unlock` on a container that is already open also raises it. The player typed the step's command
  and the box is open, and without this a box another player just opened would hold the step for up
  to five minutes.

Steps, with one action each. `start`, `explore` and `confront` are unchanged.

| Step | Reached by | Hint says |
|---|---|---|
| `loot` | `mob_death 249`, needs `14-confront` (the `missing_item` condition goes) | Type loot torvan. |
| `unlock` | `item_gain 40034`, needs `14-loot` | Type unlock strongbox. |
| `opened` (new) | `command: unlock` or `command: picklock`, `target: container`, `noun: strongbox`, `room: 498`, needs `14-unlock` | Type get ledger from strongbox. |
| `evidence` | `item_gain 40036`, needs `14-opened` | Unchanged: take it to Velk. |

- `report` moves to `# retired` in `step-ids.txt`: nothing ever granted it, so no save holds it.
  `velkhandoff` already needs `14-evidence`, now the step before `end`.
- `opened` goes in `step-ids.txt`. Its destination is room 498.
- **Picking is allowed.** A skilled player can pick the lock at any time, as with any lockbox.
  One who took the key or the ledger early passes those steps through Hook 1. If Torvan was already
  dead when `confront` began, `loot` waits on his respawn, and a player holding the key meets him
  again with it in hand.
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
- **Hook 1:** a held item fires a plain trigger at step start with its line; a trigger with `reason`,
  `count`, `room`, `area` or `zone` does not fire; an item not held does not fire; the visit guard
  stops a repeat. Each is proven able to fail. A repeatable quest with a plain `item_gain` is checked
  for a loop the hook could open.
- **Hook 2:** the loader refuses `objective` off an offer; the card shows `objective` in place of
  the first step description; each pair of offers shows exactly one at a time.
- **Hook 3:** accept on a `carriesItem` offer shows the hand-in with `[?]` in the re-shown menu.
- **Items:** no room spawns items 21 or 29, and item 16 spawns in a container, on shipped content.
  Harn's and Pell's topics give the item and move the step, and can be chosen again after it is lost.
- **Old saves:** the recovery topic shows only in its state, and grants the deliver step.
- **Esk:** `what_it_does` is hidden until `out_there` is heard, and both are hidden before `30-end`.
  A copy guard is not added; the limits above live in this spec and in a comment beside the topics.
- **Gates before push:** `gofmt`, `go build`, full `go test ./...`, `DOGMUD_BOOT_SMOKE=1` sweep,
  `golangci-lint`, isolated boot.

## Playtest

One adversarial run after the plan's last task, on a fresh instance:

| Seat | Tries |
|---|---|
| Quest 14 | The whole line, then again with a picked lock, and with a second seat taking the ledger first |
| Carrying | Takes Malk's crate before talking to Malk: the NPC's line, the card, Accept, the `[?]`, the hand-in. Then with Dorn's sample, drops the item while the card is open, and accepts |
| Ledgers | Quest 5 from Harn and quest 9 from Pell, in the evening when Harn is at the inn; loses a ledger and asks again; does quests 5 and 6 on one character |
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
entry. `internal/questengine/context.md` lists `unlock` beside `picklock` and describes Hook 1,
`internal/dialogue/context.md` describes `objective`, and the `dogmud-authoring-quests` skill gains
the pattern for an item already held, so later tiers write it the same way.
