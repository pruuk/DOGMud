# Quest Arc Phase 1: One `talk` Command (Design)

> Epic #375, phase 1. Target: `docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`. Route:
> `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`. Builds on phase 0 (PR #642, `243b7fb39`).
> Owner rulings from the 2026-10-20 brainstorm are numbered R1 to R11 below and are not to be
> re-asked.

## 1. Facts verified against source (master `243b7fb39`, 2026-10-20)

Counts come from a read-only YAML sweep of `_datafiles/world/dogmud` run while writing this
spec; line numbers were read from the files at that commit.

| Fact | Value | Source |
|---|---|---|
| Dialogue files | 302; 286 have a `tree`, 16 do not | `_datafiles/world/dogmud/dialogue/**` |
| Files with ambient `greetings:` | 186 | same |
| Tree nodes | 1,086; 12 carry `requires`, 34 carry `unlocks` | same |
| Grant nodes (`grantsQuest`) | 167: 77 `-start`, 62 `-end`, 28 other (mid-quest) | same |
| Root variants | 249; 3 carry `grantsQuest`, 1 `givesItem` | `tree.root.variants` |
| Patterns | 1,192: 299 `keywords: [""]` catch-all, 314 greeting-only keywords, 326 sharing a keyword with a tree trigger, 253 other | rule in section 6.1 |
| Patterns with more than one response | 405 (65 among the 253 "other") | same |
| `chargesGold` entries | 9 | same |
| Dialogue `requiresItem` | 28 distinct items, 31 (mob, item) pairs | same |
| Quest `item_give` triggers | 33 in 24 quest files, 28 distinct items, 31 (mob, item) pairs; every one has a `grant` action | `_datafiles/world/dogmud/quests/*.yaml` |
| `item_give` action vocabulary | `grant` 34, `npc_say` 28, `consume_item` 10, `bump_rep` 9, `set_flag` 7, `give_item` 7, `give_gold` 5, `actor` 5 | same |
| Items handed in by both routes | 25 of 28; (mob, item) pairs in both: 28 of 31 | same |
| Give-only (mob, item) pairs | (9116, 10009), (9360, 40115), (9360, 40116) | same |
| Dialogue-only items | 52, 40032, 40033 | same |
| Dialogue types | `Pattern`, `TreeNode`, `QuestGreeting`, `TreeRoot`, `Tree`, `Greeting`, `DialogueFile`, `PlayerState`, `QuestFlagSet`, `RepBump` | `internal/dialogue/types.go` |
| Existing validator | `ValidateDialogueFile(df DialogueFile, v DialogueValidators) (errs, warns []string)`; `DialogueValidators{QuestExists, QuestEndToken, FlagDeclared, ItemExists}` | `internal/dialogue/validate.go:12,36` |
| Shipped-dialogue smoke sweep | `TestSmoke_ShippedDialoguePassesTheEditorValidator` | `boot_smoke_test.go:231` |
| Unknown-YAML-key drift gate loads | `_datafiles/world/dogmud` (config `DataFiles`) | `boot_smoke_test.go:382`, `_datafiles/config.yaml:240` |
| `talk` today | room line "approaches X for a conversation", LLM greet path, then `dialogue.Greet` or `Match(df, id, "", ps)` | `internal/usercommands/talk.go:62,70,128` |
| `ask` LLM path | `internal/usercommands/ask.go:205` | |
| `give` quest hook | `questengine.Notify("item_give", ...)` before the mob transfer; `ConsumeItem` returns early | `internal/usercommands/give.go:200` |
| Menu API | `users.(*UserRecord).OpenMenu(menu.Menu, menu.Session) (menu.CloseReason, bool)`; `menu.Confirm(question, yesLabel, noLabel) Menu`; `menu.LintLabels(Menu) []string`; markers `MarkerQuest`, `MarkerHandIn`, `MarkerInfo`; `Entry.Dim`; `CloseReplaced` | `internal/users/userrecord.menu.go:93`, `internal/menu/*.go` |
| Upstream quest star | `FormattedName.QuestAlert bool` renders `★` | `internal/characters/formattedname.go:46,97` |
| Star set in text roster | `internal/rooms/roomdetails.go:294` | |
| Star set in GMCP | `QuestFlag bool \`json:"quest_flag"\`` | `modules/gmcp/gmcp.Room.go:399,613` |
| Star stripped for unnamed viewers | `internal/actions/perception.go:46` | |
| Star source | `mobs.Mob.QuestFlags` (`mobs.go:122`), `rooms.SpawnInfo.QuestFlags` (`spawninfo.go:29`, copied at `rooms.go:1139`) | |
| DOGMud content setting mob/spawn `questflags` | 0 (8 files in the unloaded `world/default` sample do) | grep of `mobs/`, `rooms/` |
| Web client reading `quest_flag` | 0 | grep of `_datafiles/html` |
| Builder fields for mob quest flags | `static/js/mobs.js:696,753`, `public/build.html:923,966`, `admin/mobs/index.html:17`, `admin/mobs/mob.data.html:70` | |
| Saved quest progress | `Character.QuestProgress map[int]string \`yaml:"questprogress,omitempty"\`` | `internal/characters/character.go:319` |
| Character quest flags (kept, unrelated) | `Character.QuestFlags map[string]string` | `character.go:320` |
| Help templates | `_datafiles/world/dogmud/templates/help/{talk,ask,quests}.template` | |
| Dashes in existing dialogue text | 445 lines with an em or en dash; cleared in phase 7 step 4, not here | roadmap phase 7 |

## 2. Owner rulings (2026-10-20)

- **R1 Convert once, delete the old format.** A one-time converter rewrites all 302 files into
  `topics:`. The engine reads only topics. Patterns, tree, triggers, keywords and the volatile
  `requires`/`unlocks` memory are deleted. No load-time adapter, no fallback labels. This
  reverses the roadmap's "extend, don't replace" for dialogue.
- **R2 Labels written properly during conversion,** by subagents per zone under the menu language
  conventions and `LintLabels`, with owner samples per zone. Quest label = quest title. No
  crude-label warning list.
- **R3 `heardRequired: [key]`** replaces `requires`, read from the saved heard set. The heard set
  records every info topic heard; heard topics render Dim.
- **R4 LLM out of `talk` and `ask`.** `internal/llm`, `LLMProfile` and its builder editing stay.
  Requirements rule 17 is reworded (section 10). Future hook: #643.
- **R5 Admin dialogue editor off until phase 6.**
- **R6 Bare Accept / Leave ships in phase 1.** Grant only on Accept; phase 2 adds objective,
  reward and chain to the same card. Hand-ins complete on choice.
- **R7 Item hand-ins are topic-only.** Each `item_give` hand-in folds into its NPC topic and the
  trigger is deleted; the 3 give-only pairs become topics too.
- **R8 `give` resolves as choosing the topic, same output both ways.** An item some topic on that
  NPC wants, given while no such topic is eligible, is handed back before any transfer.
- **R9 Markers replace the dead upstream `★`** in the same name slot; the old fields are deleted.
- **R10 Data model** as in section 3.
- **R11 Talk flow** as in section 4; the menu is re-shown after every answer.
- **R12 Behavior-tree `ask` branches become topics** (raised while planning; section 4.1). A topic
  may name a `behavior` key; choosing it fires the tree with that key.
- **R14 Instance zone buy-ins are an amount prompt** (owner 2026-10-20, raised while
  executing). Sable 315 (arena, Planar Oasis) and the Threshold Keeper 9553 (crash site) open
  instance zones for a player-chosen buy-in. Choosing the zone's topic asks "How much gold
  will you stake?" with authored guidance on what the ranges mean for that zone (owner's
  example for the Elemental Oasis: 100 to 300 a midgame solo run, 300 to 600 endgame solo,
  600 and up for only the bravest solo players or for groups). These are instance zones;
  "rift" is FinalTwist's separate micro-instance system in #208. Ferries need no prompt: one
  topic per destination, the destination a fixed action parameter.
- **R15 to R21: conversion decisions** (owner 2026-10-20, from the converter's held cases):
  - **R15** Found the item before taking the quest (Malk 15, Delia 16): talk shows the offer,
    the player accepts, then hands in from the same menu; a `give` before accepting hands the
    item back. The chain triggers that completed 15 and 16 on accept (a live bug) go.
  - **R16** Hand-in narration lines that only the old give route printed (quests 35, 68, 69,
    74) are dropped. No narration field.
  - **R17** Hadwen grants no new quest from his tree. Quest 29 "Two Roads" is replaced by two
    quests Hadwen offers after the rite (Toke's road, Esk's road); the player takes either,
    both or neither. The rite still completes quest 30.
  - **R18** A behavior topic (a timed scene: Hadwen's rite, Esk's crossing) closes the talk
    menu instead of re-showing it.
  - **R19** The bandit leader's talk option is dropped; nothing in conversation stops his
    ambush.
  - **R20** Every tutorial that teaches `ask` is converted to a `talk` lesson or dropped
    (quest 28's lesson in room 6261 becomes a talk lesson). The 98 lines in 36 files that
    tell players to "type ask" are rewritten or removed in the zone pass.
  - **R21** No quest is handed out in a batch: Crier Toke offers each of the seven trail
    quests (32, 35, 38, 41, 44, 47, 50) as its own topic after Find Your Footing, accepted
    one at a time; the starter items move onto the matching offers. Bridges that chain one
    quest into the next giver ("go talk to Fred") belong to phase 2's chain mechanism and the
    Pothole Coulee content pass (roadmap).
  - **R22** (owner 2026-10-21) A crafting quest's made item is never a hostage to its
    turn-in. Selling the crafted item to the quest's own smith counts as the turn-in (Rusk,
    quest 35: "I love the entrepreneurial spirit. You'll be a smith in no time."); a player
    who still has it shows it and keeps it; a player who sold or lost it elsewhere can still
    report without crafting again.
  - **R23** (owner 2026-10-21) With no active quest, `hint` tells every player how to find one:
    look for someone marked `[!]` and talk to them. One line for every player everywhere; no
    per-quest fallback.
  - **R24** (owner 2026-10-21) New-player copy is written for people who have never played a
    MUD: the player asks about what is on their screen ("What is this h/sp/cp line?", not
    "What are my three pools?"), and a concept is taught before any line that relies on it
    (what a command is, before "What if I forget a command?"). The guide's own menu covers
    the first verbs a newcomer needs, combat included.
  - **R25** (owner 2026-10-21) The character-creation question "How much of Gaius do you
    already know?" has no default: Enter asks again, so nobody skips the tutorial by
    pressing Enter. Pinned by `TestConfirmMenu_StartEnterReasks`.
  - Settled without asking: `-report` give routes (14, 63, 65, 67) become direct `-end`
    topics; quest 66 becomes one hand-in topic (fixes the ask-route soft-lock); `requires` on
    an offer (mobs 95, 96) becomes a quest gate; the Threshold Keeper's disc and Rusk's dagger
    use the `carriesItem` gate (shown, never taken).
- **R13 AI companion** (section 4.2): `ask` on the player's own bonded companion still routes to
  the module; `companion-ask` picks one of the NPC's eligible info topics by key, never an offer
  or hand-in. (Planning simplified the mechanism: the owner's words pick the topic at command
  time, so the model needs no topic list; section 4.2.)

## 3. Data model

### 3.1 File shape

```yaml
mobid: 94
zone: thornwall_city
defaultMood: neutral
greetings: [...]            # ambient arrival lines, unchanged
opener:                     # spoken when talk opens; carries no effects
  text: "..."
  hints: "..."
  variants:                 # first whose gates pass wins; same gate fields as topics
    - { questRequired: [10-start], text: "...", hints: "..." }
topics:
  - key: protection_report  # a mid-quest step, so kind info (section 3.3)
    kind: info              # info | offer | handin
    label: "I have proof of the protection racket."
    text: "You have evidence of a protection racket? Show me. ..."
    hints: "..."
    marked: true
    heardRequired: []
    moodChange: friendly
    # gates
    questRequired: [10-start]
    questExcluded: [10-report, 10-end]
    requiresItem: 30
    # effects
    grantsQuest: 10-report
```

`memory:` (`MemoryConfig`) is deleted with the tree memory it configured.

### 3.2 Topic fields

| Field | Kinds | Meaning |
|---|---|---|
| `key` | all | Stable id, unique within the file, `^[a-z0-9_]+$`. Saved as `<mobId>.<key>`. |
| `kind` | all | `info`, `offer` or `handin`. Sets the marker and the flow. |
| `quest` | offer, handin | Quest id. |
| `label` | all | Menu text. Offer and hand-in: the quest title. Info: the player's line. |
| `playerLine` | offer, handin | What the player says aloud on choosing it. Shown as the menu `Detail`. |
| `text` | all | What the NPC says. Offer: the pitch, which must read correctly before acceptance. |
| `hints` | all | Private hint line, as today. |
| `marked` | info | Lights `[*]` until heard. |
| `heardRequired` | all | Info keys in the same file that must be heard first. |
| `breadcrumb` | offer | Spoken by the NPC after the opener while this offer is hidden by its gates. |
| Gates | all | `questRequired`, `questExcluded`, `questFlagRequired`, `questFlagExcluded`, `requiresItem`, `itemExcluded`, `goldRequired`, `masterworkRequired`. Semantics unchanged. |
| Effects | all | `grantsQuest`, `givesItem`, `setsQuestFlag`, `bumpsRep`, `givesGold`, `chargesGold`, `moodChange`; `requiresItem` is taken. Order and failure rules unchanged (`applyQuestEffects`, `tryCharge`). |

Topics are never gated by mood (R10). Menu order is authored order, with offers and hand-ins
floated above info topics.

### 3.3 Kinds

- **offer:** eligible when every gate passes and `heardRequired` is heard. Marker `[!]`. Its
  `grantsQuest` must be the quest's start token.
- **handin:** eligible likewise. Marker `[?]`. Its `grantsQuest` must be the quest's end token.
- **info:** marker `[*]` while `marked` and unheard; Dim once heard; plain otherwise. The 28
  mid-quest grant nodes become info topics that carry the step's effects and are `marked`.

### 3.4 Heard set

`Character.HeardTopics []string \`yaml:"heardtopics,omitempty"\``, kept sorted and unique,
holding `<mobId>.<key>`. Every info topic heard is added (R3). Offers and hand-ins are not
recorded: their state comes from quest tokens. Existing characters start empty, which loses
nothing, because today's `PlayerMemory` is in-process and resets on every restart. Accessors on
`Character`: `HasHeard(key string) bool`, `MarkHeard(key string)`. The field follows the
`dogmud-persistence` rules (character-scoped, no migration marker needed for an empty start).

### 3.5 Deleted

`Pattern`, `TreeNode`, `Tree`, `TreeRoot`, `QuestGreeting` (replaced by `Opener` and
`OpenerVariant`), `MemoryConfig`, `PlayerMemory` node state (`UnlockedNodes`,
`CurrentRootSeen`, `RecentTopics`, `VisitCount`, `LastVisitRound`), `Match`,
`MatchWithFallbackInfo`, `TreeAdvance`, `Greet` (replaced by opener selection), `UpdateMemory`, `IsExpired`, `ResetMemory`, response
variants (folded into one `text`), and every quest `item_give` trigger. Greet-once tracking for
ambient `greetings` (`HasGreeted`, `MarkGreeted`) and mood state stay. The plan confirms each
deletion's callers with `codegraph_callers` before removing it.

## 4. The talk flow

1. **`talk <npc>`** passes the phase 0 gate (`OpenMenu` refuses while an interrupt rule holds),
   and the NPC must be visible to the player by name. The room sees the existing line
   ("Mira approaches Warden Velk for a conversation."). The NPC says the opener (first eligible
   variant, else `opener.text`) as ordinary speech, then the `breadcrumb` of each hidden offer in
   authored order. Hints go to the player only. Then the menu opens, private to the player:
   heading "Talking with Warden Velk", entries as section 3.2, Leave on Enter. Talking to
   another NPC replaces the menu (`CloseReplaced`). An NPC whose file has no eligible topic
   still speaks its opener and no menu opens.
2. **Info chosen:** the player says the label, the NPC says the text, effects apply, the key is
   added to the heard set, hints go to the player, and **the menu is re-shown** with markers and
   Dim states recomputed (R11).
3. **Offer chosen:** the player says `playerLine`, the NPC says the pitch, then a private
   `menu.Confirm` card: `1. Accept` / `2. Leave`, Enter = Leave. Accept runs `StillValid` (the
   offer is still eligible and the NPC still present and named) and applies the effects; a
   failed re-check gives one line and grants nothing. Either answer returns to the talk menu,
   re-shown.
4. **Hand-in chosen**, and any topic of any kind that takes an item: the player line (the label
   for info), then the handover line, then the NPC reply. The handover line is printed only when
   the topic has `requiresItem` and the effects took it: the player sees "You give the bandit
   purse to Warden Velk." and the room sees the same act through the sight rules (the lines
   `give.go` prints today). The owner's own example: "Here is the evidence you were looking
   for, Warden", then the purse changes hands, then the Warden answers. A hand-in completes on
   choice with no card (R6). The menu is re-shown, counting the quest token the choice just
   granted as held (the grant is queued and lands on a later event pass).
5. **`give <item> <npc>`** (R8), checked before any transfer and before the quest engine:
   - an eligible topic on that NPC has `requiresItem` OR `carriesItem` equal to the item:
     resolve exactly as choosing it (step 4): player line, handover line, NPC reply, first
     such topic in authored order. A `carriesItem` topic never takes the item and prints no
     handover line. No menu needs to be open. (corrected 2026-10-21: a shown item was falling
     through to an ordinary give and was lost)
   - some topic on that NPC wants the item but none is eligible: the NPC hands it back and the
     item never leaves the player ("Warden Velk looks at the bandit purse and hands it back.").
   - otherwise: today's give, unchanged.
6. **Markers** (R9): one function, `dialogue.Marker(df *DialogueFile, ps *PlayerState) string`,
   returns `[?]`, `[!]`, `[*]` or `""` by the requirements' priority. It feeds:
   - the text roster, replacing `FormattedName.QuestAlert bool` with `Marker string` in the same
     slot. A viewer at Shapes or None sight already gets anonymous figures in place of the whole
     roster (`rooms/roomdetails.go:336`), and the sight-line name helpers clear it
     (`actions/perception.go:46`, `characters/formattedname.go:178`,
     `hooks/spell_resolution.go:474`);
   - GMCP `Room.Info.Contents.Npcs`: `marker` string replaces `quest_flag` bool. No web client
     script lists NPC names from GMCP today (`webclient-pure.html:629` reads only `adjectives`
     for the map's shop icon), so players see the marker in the text roster; the field is there
     for the client arc;
   - each menu line's `Entry.Marker`.
   Combat lines, `look <npc>` and other name uses stay unmarked. Deleted with it: `mob.QuestFlags`,
   `SpawnInfo.QuestFlags` and its copy in `rooms.go`, the GMCP bool, and the builder fields listed
   in section 1.
7. **`ask <npc> ...`** prints one redirect line ("Conversations happen through talk now. Try
   talk velk.") and does nothing else, with the one exception in 4.2. The quest engine's
   `dialogue` event (`ask.go`, one content use: `quests/28-waking_to_gaius.yaml:422`, granting
   `28-asked` on mob 9491) is folded into a topic effect by the converter and the trigger is
   deleted.
8. **LLM** (R4): the `llm.AskAsync` paths in `talk.go` and `ask.go` are removed.
9. **Editor off** (R5): the dialogue panel in `static/js/dialogue.js` is hidden and
   `modules/gmcp/gmcp.Dialogue.go` refuses saves with one line naming phase 6.
10. **Mood:** `moodChange` applies as a topic effect, and ambient `greetings` keep their `moods`
    filter. Nothing gates a topic or the opener on mood.
11. **Player state** comes through `PlayerState`, extended with `HasHeard func(key string) bool`
    and `MarkHeard func(key string)`; `dialogue` keeps its rule of not importing `characters`.

### 4.1 Behavior-tree topics (R12)

Today `askNpcChain` (`internal/usercommands/ask.go:177`) offers every question to the mob's
behavior tree as `player_ask` before dialogue. Shipped content: 26 `player_ask` branches in 12
behavior files (Threshold Keeper 9553, the three ferry hands 9574 to 9576, quay agent 9572,
ferry agent 9571, passage agent 9573, Cleric Hadwen 9100, Warden Esk 9107, Barmaid Dal 117,
Sable 315, bandit leader 254), whose actions are `say` 47, `send_user_text` 6, `respond` 5,
`set_state` 3, `board_ferry` 3, `open_instance_portal` 2, `grant_quest` 2, `move_player` 1,
`emote` 1. The `noncombat_questgiver` archetype mentions `player_ask` only in a comment.

- A topic gains `behavior: true`. Choosing the topic (after its line, text and effects) fires
  the mob's tree with a new event `player_topic` carrying the topic's own key (planning: the
  key is always the topic's own, so the field is a flag). The `player_ask` event
  is removed from `behaviortree/events.go`.
- A new tree condition `check: topic_is` with `topic: <key>` replaces `keyword_match` inside
  those branches. Other
  conditions in the branch (`player_has_quest`, `player_has_item`) stay as tree conditions, and
  the topic carries matching gates so the menu shows it only when the branch can act.
- Branches whose only actions speak (`say`, `respond`, `send_user_text`, `emote`) become plain
  info topics with no `behavior`, and the branch is deleted.
- The 2 `grant_quest` branches become offer topics with Accept / Leave and their tree branch is
  deleted (requirements: no quest is granted except by accepting from an NPC menu).
- Branches with `board_ferry`, `open_instance_portal`, `move_player` or `set_state` keep their
  tree action and become topics with `behavior`.
- Validator: a `behavior: true` topic's key must appear in a `topic_is` condition in that
  mob's tree, and every `topic_is` key must name a topic on that mob.

### 4.2 AI companion (R13)

`modules/aicompanion` is off by default (`Modules.aicompanion.Enabled: false`).

- `ask <own bonded companion> ...` still routes through `companionai.RouteAsk`
  (`ask.go:142`): that is chat with the companion, not NPC dialogue. Every other `ask` redirects.
- `companion-ask <who> about <what>` resolves `<what>` against that NPC's eligible **info**
  topics when the owner types it (key, or a label containing every word typed, case-insensitive,
  exactly one match). No match or several: the owner gets one line listing the NPC's info topic
  labels and no authority is granted. A match stores the topic key in the companion's
  `askAuthority`. When the model's authorized `sayto` fires
  (`modules/aicompanion/actions.go:475`), `companionai.AskNpc` receives that key, not the
  model's words, and plays the topic exactly as the owner choosing it. Offers and hand-ins are
  never reachable this way. No prompt or model schema change is needed.

## 5. Validators

### 5.1 Proposed world

```go
type World struct {
    Files  map[int]*DialogueFile // by mob id
    Quests QuestLookup           // exists, start token, end token, title, declared flags
    Items  func(id int) bool
}
func ValidateWorld(w World) []Problem // Problem{MobId int; Where, Msg string; Warn bool}
```

`RegistryWorld()` builds it from the loaded registries and replaces `RegistryValidators()`;
`DialogueValidators` folds into `World`. Phase 6 overlays pending edits on a `World` before
calling the same function. Three doors call it: boot, the CI sweep (the renamed
`TestSmoke_ShippedDialogue...`), and the builder save (refused outright until phase 6, but wired
to the same call).

### 5.2 Rules (hard failures from phase 1)

The conversion clears the backlog in one pass, so no warning list is needed (R1, R2).

1. `key` present, slug-shaped, unique within the file.
2. `kind` is one of the three; `quest`, `playerLine` present exactly on offer and hand-in;
   `marked` only on info; `breadcrumb` only on offer.
3. Offer: `quest` exists and `grantsQuest` is its start token. Hand-in: its end token.
4. `label`, `playerLine` lint clean under `menu.LintLabels` (built as the menu the topic would
   appear in), and an offer or hand-in label equals the quest title.
5. `heardRequired` names info keys in the same file; no cycles.
6. Opener and its variants carry no effects.
7. `chargesGold: N` requires `goldRequired >= N` on the same topic.
8. No `;` in any spoken field (existing rule). No em or en dash in `label`, `playerLine`,
   `breadcrumb` (new text); old `text` waits for phase 7.
9. Every referenced quest token, flag and item exists (existing rules, kept).
10. Quest side: no `item_give` trigger may carry a `grant` action.

Each rule ships a malformed fixture and a test that sees the validator refuse it.

## 6. Conversion

### 6.1 Mechanical pass: `cmd/dialogueconvert`

A one-time Go command. It keeps private copies of the old types, reads the shipped files, and
writes the new files and a report to an output folder outside the repo; the result is copied into
place. It is deleted in the commit after the conversion lands (history keeps it).

- Tree nodes become topics: `key` from the node id; `kind` from `grantsQuest` (`-start` offer,
  `-end` hand-in, other or none info; mid-quest grants get `marked: true`); `quest` from the
  token's quest id; gates, effects, `text`, `hints`, `moodChange` copied; `requires` becomes
  `heardRequired`; `triggers` and `unlocks` dropped (a node reachable only through `unlocks` gets
  a `heardRequired` on the unlocking node, and the report lists each case for review).
- Root becomes the opener. The 3 root variants with effects become offer topics and leave the
  opener; the rest stay as opener variants.
- Patterns, classified in this order: `keywords: [""]` catch-all (299) is dropped, except that
  in the 16 tree-less files it becomes the opener when no greeting pattern exists; greeting-only
  keywords (`hello`, `hi`, `greet`, `hey`, `hail`, `greetings`, `good day`, and similar, 314)
  become the opener in tree-less files and are dropped elsewhere; a pattern sharing a keyword
  with a tree trigger (326) is dropped as covered; every other pattern (253) becomes an unmarked
  info topic, multiple responses folded into one `text` by the zone subagent.
- `chargesGold` topics (9) get `goldRequired` raised to at least the price.
- Each `item_give` trigger (33) is matched to the topic on the same mob wanting the same item.
  Its gates (`has`/`missing`), actions and `npc_say` line are reconciled into that topic; for the
  3 give-only pairs a new hand-in or info topic is created. The trigger is deleted from its quest
  file. Actions with no topic effect (`actor` 5, and any other) are listed in the report and
  decided case by case with the owner.
- `label`, `playerLine` and `breadcrumb` are written empty, so the validator fails until the
  zone pass fills them.

### 6.2 Zone pass: subagents

One subagent per zone writes labels, player lines and breadcrumbs, folds multi-response text,
rewrites each offer's pitch so it reads before acceptance, reconciles each merged hand-in line,
and decides `marked` for info topics (writer-flagged, small talk never marks). Briefing: the
nine menu language conventions in `internal/menu/context.md`, `dogmud-player-copy`, the Edit and
Write rule, and a "read N of N files" line. Each zone returns samples for the owner before the
next zone starts.

### 6.3 Reconciliation report

Every pre-conversion grant maps to exactly one topic: 77 start to 77 offers, 62 end to hand-ins,
28 mid to marked info topics, 3 variant grants to offers, and 33 `item_give` grants to topics.
The report checks the components and the total independently (two counts that must agree), and
lists every dropped pattern with its reason. Any mismatch blocks the conversion commit.

## 7. Topic-key manifest

`_datafiles/world/dogmud/dialogue/topic-keys.txt`, one `<mobId>.<key>` per line, sorted, generated
at conversion. Two optional sections, `# retired` (key removed on purpose) and
`# renamed` (`old -> new`). A guard test fails when a shipped key is missing from content without
a retired or renamed line, and when a content key is missing from the manifest (with a message
saying to append it). Renames are applied to `HeardTopics` on character load. The guard ships a
fixture that breaks it.

## 8. Tests

- Markers: `[!]` appears and clears on accept; `[?]` appears at the hand-in state and clears on
  hand-in; `[*]` clears on hearing and stays cleared across save, load and restart; no marker for
  a viewer who cannot name the NPC; priority order on a name with several.
- Heard set persistence round trip; manifest rename applied on load.
- `talk`: opener, breadcrumb, menu order, re-show after info, `CloseReplaced` on a second NPC.
- Offer: Leave grants nothing; Accept after eligibility is lost grants nothing; Accept grants once.
- `give`: an eligible hand-in prints the same lines as the menu choice; an ineligible wanted item
  stays on the player; giving twice cannot complete twice; an unwanted item is an ordinary give.
- `ask` redirect; code guards that `ask` has no topic handling and that `talk` and `ask` make no
  `llm` call.
- Validator: one malformed fixture per rule in 5.2, each refused.
- Shipped-content sweep over every converted file.
- GMCP `marker` field present and `quest_flag` gone.
- Adversarial playtest (`dogmud-playtesting`) of Thornwall quest 10 (offer, `give` hand-in,
  wrong-time `give`) and Pothole Coulee; findings filed as issues before the phase closes.

## 9. Docs

`templates/help/talk`, `ask`, `quests` rewritten. `context.md` updated for `internal/dialogue`,
`internal/menu`, `internal/usercommands`, `internal/characters`, `internal/rooms`, `internal/mobs`,
`modules/gmcp`. The `dogmud-authoring-quests` skill's dialogue and give.go sections rewritten for
topics. New files listed in `docs/README.md`.

## 10. Roadmap and requirements changes (land with this spec)

- Requirements rule 17 becomes: "LLM dialogue is out of this arc. A later hook may generate
  topics, then small quests, from an NPC's profile; they use the same topic structure and pass
  the same validators."
- Roadmap: "Extend, don't replace" is replaced for dialogue by R1; phase 1 approach rewritten to
  match this spec; its spec questions answered (LLM storage moot under R4; `UnlockedNodes` maps
  to `heardRequired`); phase 2's offer card noted as extending phase 1's Accept / Leave.

## 11. Plan shape

One plan in three stages, on one branch:

- **(A) Engine on fixtures:** new types, flow, `give`, markers, validators, persistence, guards,
  tested against fixture files. The old engine still serves the shipped files on the branch
  until (B) lands; nothing reads both formats for one file.
- **(B) Converter and zone passes:** the mechanical pass writes into the branch, the
  reconciliation report runs, then the zone subagents fill every label and owner samples are
  approved zone by zone. Until every zone is filled the validator fails, so the branch is not
  mergeable in between.
- **(C) Landing:** the converted files, the deletion of the old engine and types, the removal of
  the `item_give` triggers and the manifest go in together, so the game never runs half
  converted. Then docs, the playtest, and the converter's deletion.

## 12. Out of scope

Objective and reward lines on the offer card, chains, repeat limits (phase 2). Action triggers
beyond hand-ins (phase 3). Dashes in existing NPC text (phase 7). The admin dialogue editor
(phase 6). LLM topics (#643).
