# Dialogue Schema Reference

Since quest arc #375 phase 1, every NPC conversation is `talk <npc>`: the NPC
says its opener, then the player picks from a numbered menu of topics. A
dialogue file is the greetings, the opener and the topics. The old keyword
`patterns:`, conversation `tree:` and `memory:` sections are deleted; a file
that still carries one fails to load and the boot refuses it.

Source of truth: `internal/dialogue/types.go`, `topic.go` (the fields) and
`world.go` (`ValidateWorld`, the rules). `internal/dialogue/context.md`
describes the package.

## 1. Filename & Location

**Path formula:**
```
_datafiles/world/dogmud/dialogue/{zone_folder}/{mobid}.yaml
```

- `{zone_folder}` = the mob's zone display name, lowercased with spaces as
  underscores (`Sanctum Basin` -> `sanctum_basin`), the folder the runtime reads.
- `{mobid}` = the integer mob ID. The filename IS the ID, and `mobid:` inside
  must equal it.

The boot refuses a file whose name is not a mob id, whose `mobid:` differs from
the name, that sits in a folder other than its mob's zone, or whose id no mob
template has.

## 2. Field Reference

### Top-level Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `mobid` | int | **yes** | Must match the filename and the mob YAML's `mobid`. |
| `zone` | string | **yes** | Display name of the zone. |
| `defaultMood` | string | no | Starting mood (see Mood Values). Default `neutral`. |
| `greetings` | list | no | Ambient lines offered when a player walks in. |
| `opener` | object | no | What the NPC says when `talk` opens, before the menu. |
| `topics` | list | no | The menu. |

Decoding is strict: an unknown key (a typo, or an old `patterns:`) fails the
file, and so does a bare scalar where a list is expected
(`questRequired: "5-start"` instead of `questRequired: [5-start]`).

### Greetings

```yaml
greetings:
  - text: "Welcome, traveler."
    moods: ["friendly", "neutral"]   # only offered in these moods
  - text: "Keep your distance."      # untagged: the fallback
```

An NPC greets each player once per mob instance. The first greeting whose
`moods` holds the NPC's mood wins, else the first untagged one, else nothing.

### Opener

```yaml
opener:
  text: "Welcome to the Crossing Inn. Sit anywhere."
  hints: "You could ask Tolva about the toll."
  variants:                          # first passing variant wins
    - questRequired: [5-end]
      text: "Thanks to you, the magistrate will have that ledger."
    - questRequired: [5-start]
      questExcluded: [5-end]
      text: "Any luck with that ledger?"
```

A variant takes the gates below and its own `text` and `hints`. It has no
effects. Put more specific variants (quest finished) before less specific ones
(quest started).

### Topics

```yaml
topics:
  - key: toll_problem               # ^[a-z0-9_]+$, unique in the file
    kind: info                      # info | offer | handin
    label: What is wrong with the toll?
    text: "Harn set up that toll two years ago."
    hints: "You could ask about Harn."   # narrator line after the reply
    marked: true                    # [*] on the menu and the NPC until heard
```

| Field | Kinds | Notes |
|-------|-------|-------|
| `key` | all | Saved on characters as heard (`<mobid>.<key>`), so treat it as permanent. |
| `kind` | all | `info` (a question), `offer` (a quest offer), `handin` (a quest hand-in). |
| `quest` | offer, handin | The quest id. |
| `label` | all | The menu line, at most 50 characters, no em or en dash. An offer or hand-in's label is the quest's title. Info labels are unique in the file and never equal a quest title. |
| `playerLine` | offer, handin | What the player says on choosing it, at most 70 characters. |
| `text` | all | The NPC's reply, first person. |
| `hints` | all | Narrator text addressed to the player. |
| `marked` | info | Marks an unheard info topic `[*]`. |
| `heardRequired` | all | Info topic keys in this file the player must have heard first. No cycles. |
| `breadcrumb` | offer | What the NPC says after the opener while this offer's gates hide it, until the quest is taken. |
| `behavior` | all | Fires the mob's behavior tree with `player_topic` and this key once the topic has played; the tree needs a matching `topic_is`. |

**Gates** (topics and opener variants):

| Field | Type | Effect |
|-------|------|--------|
| `questRequired` | `[]string` | Player must hold these quest tokens. |
| `questExcluded` | `[]string` | Hidden while the player holds any of these. |
| `questFlagRequired` | map | Every flag must equal its value. |
| `questFlagExcluded` | map | Hidden if any flag equals its value. |
| `requiresItem` | int | Player must carry it in the backpack; taken when the topic applies. |
| `carriesItem` | int | Player must carry it anywhere; never taken (a reusable key). |
| `itemExcluded` | int | Hidden while the player carries it anywhere (one handout per open quest). |
| `goldRequired` | int | Hidden from a player carrying less. Takes nothing. |
| `masterworkRequired` | int | Player must own a masterwork crafted at this skill or above. |

**Effects** (topics only), applied in a fixed order when the topic is chosen
(or given its item):

| Field | Type | Effect |
|-------|------|--------|
| `chargesGold` | int | Taken first; all or nothing. Needs a `goldRequired` of at least the same price. |
| `givesItem` | int | Delivered before anything else; if the player cannot carry it, nothing applies and any charge is refunded. |
| `grantsQuest` | string | Quest token granted. An offer grants its quest's start token, a hand-in its end token. |
| `setsQuestFlag` | `{key, value}` | Flag written. |
| `bumpsRep` | list of `{faction, delta}` | Reputation changes. |
| `givesGold` | int | Gold given. |
| `moodChange` | string | The NPC's mood afterwards. |

An info topic is recorded as heard once its effects land. Offers and hand-ins
are not: quest state lives in tokens.

### Mood Values

`friendly`, `neutral`, `hostile`, `afraid`, `grateful` (`dialogue.Mood`). Mood
picks greetings and changes through `moodChange`; it never gates a topic. It
lives per mob instance and resets on respawn.

## 3. Annotated Example

Abbreviated from `_datafiles/world/dogmud/dialogue/watchers_crossing/84.yaml`
(Tolva, quest 5):

```yaml
mobid: 84
zone: Watchers Crossing
defaultMood: neutral
opener:
  text: "Welcome to the Crossing Inn. ..."
  variants:
  - questRequired: [5-start]
    questExcluded: [5-deliver, 5-end]
    text: "Any luck with that ledger? ..."
topics:
- key: help_quest
  kind: offer
  quest: 5
  label: The Innkeeper's Complaint
  playerLine: Do you want help with that toll?
  text: "Harn keeps a ledger at the tollhouse. ..."
  questExcluded: [5-start, 5-end]
  grantsQuest: 5-start
  moodChange: friendly
- key: tolvahandoff
  kind: handin
  quest: 5
  label: The Innkeeper's Complaint
  playerLine: I brought you Harn's crossing ledger.
  text: "The crossing ledger. Let me see those numbers. ..."
  questRequired: [5-start]
  questExcluded: [5-end]
  requiresItem: 21
  grantsQuest: 5-end
  moodChange: grateful
- key: toll_problem
  kind: info
  label: What is wrong with the toll?
  text: "Harn set up that toll two years ago. ..."
  marked: true
```

`give ledger tolva` plays `tolvahandoff` exactly as choosing it from the menu
does: a topic that wants an item (`requiresItem` or `carriesItem`) is the
hand-in for it.

## 4. Gotchas

**Every `grantsQuest` must exclude the granted token and the quest's end
token.** Example: `grantsQuest: 10-start` needs
`questExcluded: [10-start, 10-end]`, or the offer repeats or a finished quest
is offered again. The boot refuses either omission.

**Quest tokens use the format `{questId}-{stepId}`** and must name a real
step. `HasQuest()` is true on that step or any later one, so use
`questExcluded` to hide a topic once a quest moves past a point.

**`grantsQuest` fires the quest event handler,** which pays every reward in
the quest YAML. No separate reward mechanism is needed.

**Quest `item_give` triggers may not grant a token.** A hand-in is a
`handin` topic with `requiresItem`.

**Voice.** `text` is the NPC speaking, first person. `hints` is narrator text
for the player: "You could ask why she left", never "Ask about why she left"
when "she" is the speaker.

**No semicolons in spoken text** (`text`, labels, `playerLine`, `breadcrumb`,
greetings, the opener and its variants). `;` is the command separator and cuts
a mob's line short. Hints are exempt.

**`requiresItem` searches the backpack only.** Quest items must not be
`is_component: true`, which routes them to the component pouch.

**Every flag a topic names must be declared** in its quest's `flags:` block,
or the boot panics (`questengine.ValidateAllFlags`).

## 5. Branching Quests with Flags

A branching quest records the player's choice in a flag and gates later
topics on it. Declare the flag first:

```yaml
# _datafiles/world/dogmud/quests/11-the_windwardens_dilemma.yaml
flags:
  - key: branch
    values: [sylara, rhett]
```

Content writes the full key, `11-branch`, everywhere else.

Set it on the topic that commits the player:

```yaml
- key: side_with_rhett
  kind: info
  label: I will stand with Rhett.
  text: "Then we are agreed."
  questRequired: [11-start]
  questExcluded: [11-middle, 11-end]
  grantsQuest: 11-middle
  setsQuestFlag: {key: 11-branch, value: rhett}
```

Gate each branch's later topics and opener variants with
`questFlagRequired: {11-branch: rhett}` or
`questFlagExcluded: {11-branch: sylara}`. The menu shows only eligible topics,
so a wrong-path player never sees the other branch; no ordering or dismissal
topic is needed. Give each NPC an opener variant per branch during the active
phase, so a wrong-path player hears a closing line instead of the default
opener, which often hints at the quest.

### Checklist for Branching Quests

1. Flag declared in the quest YAML with every value.
2. `setsQuestFlag` on each branch's committing topic.
3. `questFlagRequired` or `questFlagExcluded` on every branch topic.
4. Opener variants per branch for the active and finished phases.
