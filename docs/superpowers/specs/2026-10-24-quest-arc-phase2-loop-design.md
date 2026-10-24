# Quest Arc Phase 2: Offer Card, Hand-in and the Closed Loop (Design)

> Epic #375, phase 2. Target: `docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`. Route:
> `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`. Builds on phase 1 (PR #661, `feea56908`).
> Owner rulings from the 2026-10-24 brainstorm are numbered P1 to P7 below and are not to be
> re-asked. One spec, two plans (P5): plan 2a is sections 3 to 6, plan 2b is section 7.

## 1. Facts verified against source (master `fd18743db`, 2026-10-24)

Counts come from two read-only YAML sweeps of `_datafiles/world/dogmud` (quest census and
step census) run while writing this spec. Test quest 1000000 is left out of every count.
Line numbers were read from the files at that commit.

| Fact | Value | Source |
|---|---|---|
| Quest files | 81 including test quest 1000000 | `quests/*.yaml` |
| Final step token granted by | a topic only 48, a trigger only 27, both 3, nothing in YAML 3 | quest census |
| Quests with no offer topic | 28, 29, 77 (plus 21, the secret marker quest) | quest census |
| Quests with a hand-in topic | 50 | quest census |
| Topic kinds shipped | `offer` 87, `handin` 67 | `dialogue/**` |
| Repeatable quests | 53 to 59, all `cooldown_rounds: 300` | `quests/53-*.yaml` to `59-*.yaml` |
| Chains already expressed by gates | 29 quests have their final token in some offer's `questRequired`; fan-outs: 11 to 12, 13; 30 to ten offers; 31 to the seven trail quests 32, 35, 38, 41, 44, 47, 50; 78 to the pinnacle quests 79 to 87 | quest census |
| Non-start steps | 227 | step census |
| Ways into a step (triggers plus topics) | 0 ways: 10 steps; 1 way: 174; 2: 30; 3: 9; 4: 4 | step census |
| Steps with no way in found in YAML | 14-report, 28-end, 30-rite, 30-end, 32-consider, 63-report, 65-report, 66-escort, 66-detox, 67-report | step census |
| Progressing triggers | 177; 14 do not require the previous step's token; 3 grant two own steps | step census |
| Extra conditions on progressing triggers | `has_item` 8, `missing_item` 4, `has_flag` 3, `missing_flag` 1; `in_room` 0 | step census |
| Trigger sequences | one, in quest 69 | `quests/69-the_gallery_cipher.yaml` |
| First step id | `start` in 80 files (test quest included); `end` in 1, quest 77 (The Truth), a one-step quest with no offer | `quests/*.yaml`, `quests/77-the_truth.yaml` |
| Quest struct | `Repeatable bool`, `CooldownRounds int` (`cooldown_rounds`); no limit, no count | `internal/quests/quests.go:61` |
| Step struct | `Id`, `Description`, `Hint`, `MapTarget`, `MapTargetMob`; every quest has a `description` and a first-step `description` | `internal/quests/quests.go:72`, quest census |
| Reward struct | `QuestId`, `Gold`, `ItemId`, `ConditionId`, `SkillInfo`, `StatInfo`, `RecipeInfo`, `ItemInfo`, `SpellId`, `PlayerMessage` (`actor`), `RoomMessage` (`observer`), `RoomId`, `RepFaction`, `RepAmount` | `internal/quests/quests.go:36` |
| Reward keys shipped | `actor` 66, `gold` 63, `observer` 56, `itemid` 28, `skillinfo` 25, `rep_faction`/`rep_amount` 15, `stat_info` 7, `recipe_info` 6, `item_info` 5, `spellid` 3, `conditionid` 1; `questid` 0 | quest census |
| Gold already shown as a number at payout | `You receive N gold!` | `internal/hooks/Quest_HandleQuestUpdate.go:314` |
| Cooldown refusal comes after Accept | step `start` of a repeatable on cooldown is refused with "Rest a while" | `Quest_HandleQuestUpdate.go:206` |
| Completion of a repeatable | clears progress and stamps the cooldown | `Quest_HandleQuestUpdate.go:446`, `:451` |
| Cooldown storage | MiscData key `questcd-<id>`, absolute round | `internal/characters/quests.go` (`SetQuestCooldown`, `QuestCooldownActive`) |
| Round count | 4 s rounds (`RoundSeconds: 4`), 900 per game day; persisted across restarts, so offline time does not count | `_datafiles/config.yaml:193,215`; `main.go:435,590` |
| Quest progress save | `QuestProgress map[int]string` (`questprogress`) | `internal/characters/character.go:319` |
| Heard topics save | `HeardTopics []string` (`heardtopics`) | `internal/characters/character.go:321` |
| Topic-key manifest | `dialogue/topic-keys.txt`, sections `# retired`, `# renamed old -> new`; test `TestTopicKeyManifest` and `_GuardCanFail` | `topic_manifest_test.go:40,59` |
| Heard renames on load | `loadedUser.Character.RenameHeardTopics(dialogue.HeardRenames())` | `internal/users/users.go:568` |
| Offer card today | `offerCard` builds `menu.Confirm(t.Label, "Accept", "Leave")`; `accept` re-checks `Eligible` | `internal/conversation/talk.go:320,338` |
| Grants are queued | `PlayerState.GiveQuest` queues an `events.Quest`; `heldAlso` counts the just-granted token as held for the re-shown menu | `internal/conversation/playerstate.go:57`, `talk.go:172` |
| Menu shape | `Menu{Heading, Entries, Leave, Default, Footer, Columns, Prefix, AcceptText, LeaveAliases}`; no body text field; `Confirm` at `resolve.go:132` | `internal/menu/menu.go:45` |
| Eligibility | `DialogueFile.Eligible` = gates plus `heardRequired`; nil interrogative callbacks fail closed | `internal/dialogue/topics.go:29` |
| `PlayerState` callbacks | `HasQuest`, `HasItem`, `CarriesItem`, `RemoveItem`, `GiveQuest`, `GiveItem`, `GetQuestFlag`, `SetQuestFlag`, `BumpRep`, `GiveGold`, `HasOwnMasterwork`, `HasGold`, `ChargeGold`, `HasHeard`, `MarkHeard` | `internal/dialogue/types.go` |
| Quest reference validator | `ValidateQuestRefs(q Quest, v QuestValidators) (errs, warns []string)`; called only by the quest editor save | `internal/quests/validate_refs.go:219`; `modules/gmcp/gmcp.Quest.go:66,232` |
| Boot quest checks | `Quest.Validate` per file, `questengine.ValidateAllFlags` | `internal/quests/quests.go`, `internal/questengine/loader.go:41,106` |
| Dialogue three doors | `validateShippedDialogue`, `dialogueDoor` | `main_dialogue.go:25,39` |
| Import direction | `dialogue` imports `quests`; neither reaches `users`, `characters`, `questengine`, `conversation`, `hooks` or `modules/gmcp` (`go list -deps`); `users` and `modules/gmcp` import both. A probe package importing `quests` and `dialogue`, plus a caller importing it with `users` and `modules/gmcp`, built clean | `go list`, throwaway build 2026-10-24 |
| Headless engine harness | `Engine.Notify(eventType, details, player, ctx)`; test players `fullMockPlayer`, `linearPlayer` | `internal/questengine/engine.go:73`, `newbie_one_command_steps_test.go:30` |
| Item sources | mob `loot_pool`, character `items` and `equipment`; shop `ItemId`; room `SpawnInfo.ItemId`; quest `give_item`, topic `givesItem`, quest rewards | `internal/mobs/mobs.go:86`, `internal/characters/character.go:144,148`, `internal/characters/shop.go:20`, `internal/rooms/spawninfo.go:14` |
| Repeatable shown to players | nowhere: only the builder's `questListRow.Repeatable` | `modules/gmcp/gmcp.Quest.go:91` |
| Text quest list | `usercommands.Quests` builds `QuestRecord{Id, Name, Description, Completion, BarFull, BarEmpty}` from `GetQuestProgress()` only, so a quest with no saved progress is never listed; template `character/quests.template` | `internal/usercommands/quests.go:17`, `_datafiles/world/dogmud/templates/character/quests.template` |
| Web quest panel payload | `Char.Quests`, `GMCPCharModule_Payload_Quest{Id, Name, Description, Completion, Hint, Focused, TargetRoom, NextRoom, NextDir}`; no repeatable field | `modules/gmcp/gmcp.Char.go:1143` |
| Web quest panel renderer | reads `Char.Quests` | `_datafiles/html/public/webclient-pure.html`; tests `tools/webclient-tests/quest-panel-refresh.js` |
| Item sources not yet read | forage tables, crafting outputs | `internal/actions/forage.go`, `internal/crafting` (plan 2b reads them) |

## 2. Owner rulings (2026-10-24)

- **P1 Chains are derived, not declared.** Quest N's next quests are the offers whose
  `questRequired` names N's final token. There is no `next:` field. The gates already decide
  eligibility, and a second record would need a sync guard.
- **P2 The reward line is generated, with an optional authored override.** One function
  words every reward. An override replaces it where a surprise or better wording is wanted,
  and a guard keeps the override honest.
- **P3 Repeat limit is a lifetime cap, and it is optional.** Frequency stays
  `cooldown_rounds`. A per-window cap was rejected: to make a quest come round less often,
  lengthen the cooldown. Repeating a daily every day is not an exploit. With no limit set, a
  repeatable repeats forever on its cooldown. (The optional reading was stated in the
  brainstorm and not objected to.) Requirements line 13 changes accordingly (section 8).
- **P4 "One action" is a structural rule, and alternatives are allowed.** Section 5's
  `one-action` rule. Several ways into one step are fine. The rule refuses the shapes that hide
  a second action.
- **P5 One spec, two plans.** Plan 2a: card, hand-in, repeats, validators, manifest. Plan 2b:
  the completability walkthrough.
- **P6 Players can tell a quest is repeatable** (owner 2026-10-24, from playtest-bot
  feedback): the offer card says so, the text and web quest lists tag it `(repeatable)`, and
  `quests all` and the web quest panel show finished repeatables with whether they can be
  taken again; the panel leaves out those done for good (section 4.3).
- **P7 The admin quest editor is off until phase 6** (owner 2026-10-24): a create or edit there could save a quest that fails the loop rules and stops the next boot; the editor-save loop door (5.4) stays in place for phase 6.

## 3. Offer card

Choosing an offer still plays the player's line and the NPC's pitch (phase 1). The card that
follows becomes:

```
  Cull the Wash
  Garve has asked you to thin out the bandits in the wash again: a
  bandit scout first, then a bandit squatter.
  Reward: 6 gold and training in weapon combat.

  1. Accept
  2. Leave
```

- **Heading:** the topic label (the quest title), as now.
- **Objective:** the quest's first step `description`.
- **Body text:** `menu.Menu` gains `Body []string`, paragraphs rendered between the heading
  and the entries and wrapped at the 80-column width by `Render`. `menu.Confirm` keeps its
  signature; the card sets `Body` on the menu it returns. `menu.LintLabels` does not lint
  `Body`.
- **Reward line:** `"Reward: " + quests.RewardSummary(q)`, left out when the summary is empty.
- **Unchanged:** `give` to an offer (phase 1 R8) shows this same card. Accept re-checks
  eligibility, Enter is Leave, and a behavior-topic offer (R18) still closes the menu on
  Accept.

### 3.1 `quests.RewardSummary`

`func RewardSummary(q *Quest, names RewardNames) string` is the only place reward wording
lives. `RewardNames` is a struct of name lookups (item, recipe, spell, skill, stat, faction)
injected the way `QuestValidators` is, so `quests` gains no registry imports.

| Field | Words |
|---|---|
| `Gold` | `N gold` |
| `ItemId` | the item's name with its article |
| `ItemInfo` | each item by name, with a count when more than one (`Iron Ingot (x3)`, the payout line's form) |
| `RecipeInfo` | `the recipe for <name>` per recipe |
| `SpellId` | `the spell <name>` |
| `SkillInfo` | `training in <skill>` per skill |
| `StatInfo` | `more <Stat>` per stat |
| `RepFaction` with `RepAmount > 0` | `the goodwill of <faction>` |
| `ConditionId`, `RoomId`, `QuestId`, `actor`, `observer` | nothing |

Parts join as "A", "A and B", "A, B, and C". No amount other than gold and item counts is
shown (player-copy rule). An empty result leaves the reward line out.

### 3.2 Override

New optional reward fields:

```yaml
rewards:
  summary: Garve will make it worth your while.
  surprise: true
```

- `summary` replaces the generated text after "Reward: ".
- Guard (rule `reward`, section 5): a `summary` must contain the name of every item, recipe
  and spell the quest pays, unless `surprise: true`. `surprise` without `summary` is
  refused. Both obey the dash and player-copy guards that already cover quest text.

## 4. Hand-in, next offer and repeats

### 4.1 Next offer

Choosing a hand-in pays out exactly as today (`Quest_HandleQuestUpdate`). Then:

1. Compute the next offers on this NPC: topics of kind `offer` that are `Eligible` under
   `heldAlso(ps, <final token>)` and whose `questRequired` contains that final token.
2. With exactly one, play it as if chosen from the menu: the player's line, the pitch and
   the card (section 3). Accept grants it. Leave returns to the talk menu.
3. With none or several, re-show the talk menu as phase 1 does. Several next offers show as
   `[!]` lines (the Toke trail, the quest 11 branch).
4. Skip step 2 when the hand-in is a behavior topic that closes the menu (R18).
5. Skip step 2, saying nothing, when the card could not open when the pitch runs: a menu
   gate holds (`MenuBlockedReason`) or a prompt is open (`give` hands in over a prompt). The
   offer waits in the talk menu with its `[!]`.

The hand-in path calls the same `resolve` an offer choice uses, so the pitch, card and
grant have one code path. For a repeatable, the final token is held only until the
completion pass clears progress, and the quest's own offer excludes its final token, so the
quest is not offered back to the player in the same breath.

### 4.2 Repeat limit and completion count

- New quest field `repeat_limit int` (optional, `omitempty`). 0 or absent means no cap. Only
  valid on a repeatable (rule `repeat`).
- New saved character field `QuestCompletions map[int]int` (`questcompletions`, `omitempty`),
  next to `QuestProgress`. It is a typed field rather than MiscData so its round trip is
  tested like `QuestProgress`. The completion pass increments it for a repeatable quest, in
  the same block that clears progress and stamps the cooldown (`Quest_HandleQuestUpdate.go:446`).
- New method `Character.QuestOpen(q *quests.Quest) bool`: false while
  `QuestCooldownActive`, or when `repeat_limit > 0` and the count has reached it. True for
  every non-repeatable.
- New `PlayerState.QuestOpen func(questId int) bool`. `Eligible` checks it for a topic of
  kind `offer` with `Quest > 0`. Per the package rule, a nil callback fails closed. The
  production callback (`conversation.playerState`) sets it, and every test `PlayerState` that
  offers a quest sets it too.
- A hidden offer behaves like any ineligible offer: its breadcrumb speaks, or nothing does,
  and `[!]` clears. The cooldown refusal at `Quest_HandleQuestUpdate.go:206` stays as the
  backstop for a grant from any other path, and also refuses at the cap with the line
  "You have done all of this work there is to do."
- Players who completed a repeatable before this lands start at count 0 (accepted in the
  brainstorm).

### 4.3 Showing that a quest repeats (P6)

All of it is generated from `repeatable`, `repeat_limit` and `QuestCompletions`, so no
content is written.

- **Offer card:** a repeatable's card gets one more body line after the reward:
  "You can take this on again after a while." On the last allowed run (a `repeat_limit` set
  and the count one short of it) the line is instead "This is the last time this work will
  be offered to you." No round count is shown (player-copy rule). Authored pitches may add
  their own flavour; no validator asks for it.
- **Quest lists:** the `quests` command and the web quest panel tag a repeatable with the
  word `(repeatable)` after its name, never a colour or icon alone.
  - `QuestRecord` gains `Repeatable bool`, and `quests.template` prints the tag.
  - `GMCPCharModule_Payload_Quest` gains `Repeatable bool` (`json:"repeatable,omitempty"`),
    and the web panel prints the same word.
- **Finished repeatables in `quests all`:** a repeatable with no current progress and a
  count of at least 1 is listed at full completion with `(repeatable)` and one status line in
  place of the step description:

  | State | Line |
  |---|---|
  | `QuestOpen` true | Ready to take on again. |
  | on cooldown | Not offered again yet. |
  | at its `repeat_limit` | Done for good. |

  A repeatable in progress is listed as any quest is, with its tag. The plain `quests` view
  (in progress only) does not show finished repeatables, as it hides every finished quest
  today. Secret repeatables stay hidden, like other secret quests.
- **Finished repeatables in the web quest panel** (owner 2026-10-24):
  - `Char.Quests` also carries each finished, non-secret repeatable that is not at its cap,
    with `status` set to `ready` or `waiting` and no description, hint or map target.
  - The panel draws them below the active quests under a "Repeatable work" label, dimmed,
    with the status in words ("Ready to take on again." / "Not offered again yet."). They
    cannot be focused or marked on the map.
  - "Done for good" repeatables stay out of the panel (a live list, not history); `quests all`
    shows them.
  - No event fires when a cooldown ends. The panel already re-reads `Char.Quests` on every
    room change (`roomChangeHandler`, `modules/gmcp/gmcp.Char.go:145`), so "waiting" becomes
    "ready" on the player's next move. Accepted; no timer is added.

## 5. Loop validators

### 5.1 Package `internal/questloop`

A new pure package importing only `quests` and `dialogue` (import graph checked in section 1).
It ships a `context.md`.

```go
type World struct {
    Quests   []*quests.Quest
    Dialogue []*dialogue.DialogueFile
    Refs     *quests.QuestValidators // existence callbacks, reused; nil skips the refs rule
    Names    quests.RewardNames
}

type Finding struct {
    Rule    string // rule id, e.g. "one-action"
    QuestId int
    Where   string // "step firstkill", "mob 9112 topic cull_offer" (format in 5.3)
    Msg     string
}

func Validate(w World) []Finding
func (f Finding) Key() string // "rule questId where", the backlog line
```

The boot and CI worlds come from the loaded registries. The editor door overlays the quest
being saved on them. A second new package, `internal/questworld`, builds those inputs from
the live registries: the reward names, the quest reference validators (moved out of
`modules/gmcp`, which keeps calling them), `Registry()`, `WithQuest(w, q)` and the backlog
loader. The offer card, the boot door, the CI sweep and the editor all use it. Its imports
(mobs, rooms, conditions, items, spells, skills, crafting, factions, dialogue, quests) reach
neither `conversation` nor `questworld` (`go list -deps`, 2026-10-24).

### 5.2 Rules

| Rule | Check |
|---|---|
| `start` | The first step's token is granted by at least one `offer` topic, and by nothing else: no trigger action, no `rewards.questid`, no topic of another kind. |
| `end` | The final step's token is granted only by `handin` topics, and by at least one. |
| `hint` | Every step except the final one has a non-empty `hint`. |
| `one-action` | For every non-start step: at least one way in; each way is one trigger (its event plus filters) or one topic; each way requires the previous step's token (`conditions.has` or `questRequired`); no way carries `has_item`, `missing_item`, `has_flag`, `missing_flag` or `has_gold`; no trigger grants more than one step of its own quest; no trigger uses a `sequence`. Several ways into one step are alternatives and pass. |
| `chain` | Every offer whose `questRequired` names quest N's final token sits on a mob that has a `handin` topic for N. |
| `repeat` | A repeatable has `cooldown_rounds > 0`. `repeat_limit` is refused on a non-repeatable and when negative. |
| `reward` | Section 3.2's override guard. |
| `refs` | Every refusal from `ValidateQuestRefs`, now run at all three doors. Its warnings stay warnings. |

`start` and `end` read grants from topics and triggers in the proposed world, so a quest and
its givers' topics validate as one unit. A one-step quest (today only 77) has one token that
is both first and final, so it cannot pass both rules; that is intended, since a closed loop
needs an offer and a separate hand-in. Quest 77 goes on the backlog.

### 5.3 Backlog ratchet

`_datafiles/world/dogmud/quests/loop-backlog.txt` lists today's findings, one `Finding.Key()`
per line, with a header comment like the topic manifest's.

- A finding on the list is logged as a warning at boot.
- A finding not on the list refuses: boot fails, the CI sweep fails, the editor save is
  refused.
- A list line with no matching finding fails the CI sweep (stale line), so the list only
  shrinks.
- Phase 7 deletes lines as each zone pass clears them. An empty list means every rule is
  hard. This is the roadmap's "warning list, then hard failure", made mechanical.

A key is `rule questId where`. `where` holds no positions, so inserting a step or a trigger
moves no other line, and it names the defect class, so a listed defect cannot hide a new one
at the same place:

| `where` | Names |
|---|---|
| `step firstkill` | a step, by id |
| `quest 53 trigger mob_death mob=9110 grants=53-firstkill` | a trigger, by its event, its non-zero filters and the tokens it grants (sorted); triggers of one quest with the same signature take ` #2`, ` #3` in file order |
| `quest 53 rewards questid`, `quest 53 rewards summary` | the reward grant, the reward override |
| `mob 9112 topic cull_in` | a dialogue topic |
| `quest 53:cooldown`, `:negative`, `:notrepeatable` | the three `repeat` checks |
| `quest 53 refs: <refusal text>` | one `refs` refusal, on one line, with its leading trigger index and step index named by signature and id |

`one-action` adds the reason to the way: `:none`, `:reward`, `:prev`, `:cond`, `:seq` or
`:double`. A way broken two ways is two lines.

### 5.4 Doors

| Door | Where |
|---|---|
| Boot | `validateShippedQuests` in a new `main_quests.go`, called beside `validateShippedDialogue`, through the same door and reload semantics as `dialogueDoor` |
| CI | `TestSmoke_ShippedQuestsPassTheLoop` (repo root) |
| Editor save | `modules/gmcp/gmcp.Quest.go` runs `questloop.Validate` over the registry world before and after the edited quest is swapped in, after `ValidateQuestRefs`. It refuses only findings the edit adds (not in the world before, not on the list) and warns about the edited quest's other findings. The swapped-in world's step and flag checks read the edited quest, so a step rename that breaks another quest is refused at save. Dormant while the editor is off (P7): `refuseQuestBuild` refuses every `Build.Quest.Get/Update/Create/Delete` before dispatch, and phase 6 re-enables this door |

### 5.5 Step-id manifest

- `_datafiles/world/dogmud/quests/step-ids.txt`: one `<questId>-<stepId>` per shipped step,
  with `# retired` and `# renamed old -> new` sections, in the same format as
  `topic-keys.txt`.
- Parser and `StepRenames()` live in `internal/quests` (`quests/manifest.go`), mirroring
  `dialogue/manifest.go`.
- `step_manifest_test.go` (repo root) holds the manifest to content, with a `_GuardCanFail`
  twin.
- `users.LoadUser` calls `Character.RenameQuestSteps(quests.StepRenames())` next to
  `RenameHeardTopics`. A renamed step's saved progress follows the rename. A retired step is
  left as is and logged, because a retired current step needs a content decision.

## 6. Tests (plan 2a)

Every guard ships with a fixture that breaks it and a test that sees it break.

- **Card:** objective wrapping at 80 columns; `RewardSummary` table test over every reward
  field, joins, counts, empty; override shown; surprise shown; `give` to an offer shows the
  card.
- **Next offer:** one next offer plays its card at once; Leave returns to the menu; several
  re-show the menu with `[!]`; none re-shows the menu; a behavior hand-in skips it.
- **Repeats:** an offer is hidden on cooldown (no Accept then "Rest a while"); hidden at the
  cap; `[!]` clears in both and returns when the cooldown ends; a double Accept or a double
  `give` cannot pass the cap; `QuestCompletions` survives save, load and restart.
- **Repeat display:** the card's again line and last-time line; `(repeatable)` in the text
  list and in `Char.Quests` (and the web panel test in `tools/webclient-tests`); a finished
  repeatable in `quests all` moves from "Not offered again yet" to "Ready to take on again"
  when the cooldown ends, and to "Done for good" at the cap; it is absent from plain
  `quests`.
- **Validators:** one malformed fixture per rule, refused at boot, CI and editor save; the
  ratchet fails on a new finding and on a stale line; the editor checks the proposed world,
  not the cached one.
- **Manifest:** an unlisted new step id fails; a removed step without a `# retired` or
  `# renamed` line fails; `LoadUser` rewrites `QuestProgress` through a rename.

## 7. Completability walkthrough (plan 2b)

### 7.1 Driver

It lives in `internal/questengine`, which already imports `dialogue`, and drives the real
engine headlessly. The mock players (`fullMockPlayer`, `linearPlayer`) move from `_test.go`
files into one shared test-helper file, so the walkthrough and the existing step tests use
one copy.

For each shipped quest:

1. Build a mock player holding every token the quest's offer gates require.
2. Choose the offer through `DialogueFile.Apply`.
3. For each step, and for each way into it, starting from a fresh copy of the state at
   that step:
   - for a trigger, build the event from its own filters (`mob_death` with its `mob`,
     `room_enter` with its `room`, `item_give` with its `item`, and so on), meet its
     conditions, call `Notify`, and require the token to move forward by exactly one step;
   - for a topic, `Apply` it and require the same.
4. Finish through a hand-in topic, and require completion and the derived next offers
   (section 4.1).

An alternative that breaks is reported by name.

### 7.2 Source checks

- **Items:** every item a step needs (`has_item`, `item_give`, `requiresItem`, `carriesItem`)
  has a source: a mob's `items`, `equipment` or `loot_pool`, a shop's `ItemId`, a room
  `SpawnInfo.ItemId`, a quest `give_item`, a topic `givesItem` or a quest reward. Forage
  tables and crafting outputs count too; plan 2b reads their fields first.
- **Liveness:** a mob source must be spawned by some room, and a shop source must belong to
  a spawned mob.
- **NPCs:** every `mob_death` target and every topic's NPC is spawned by some room.

### 7.3 Findings

Same `questloop.Finding` shape and the same backlog ratchet (section 5.3), with rules
`walk-dead-end`, `walk-no-source` and `walk-unspawned`. One fixture per rule, each proven to
fail.

**Not covered:** whether a room can be reached on foot, and combat difficulty. Those belong
to the zone playtests.

## 8. Docs and rollout

- **`context.md`:** new in `internal/questloop`, `internal/questworld` and `internal/manifest`
  (the stable-key format the topic and step manifests now share); updated in `internal/quests`,
  `internal/questengine`, `internal/dialogue`, `internal/conversation`, `internal/menu`,
  `internal/characters`.
- **Help:** `_datafiles/world/dogmud/templates/help/talk.template` describes the card and the
  next offer; `quests.template` in the same folder describes repeats, the `(repeatable)` tag
  and the finished-repeatable lines in `quests all`.
- **Requirements:** rule 13 becomes "Repeatable quests have a frequency, and a repeat limit
  where wanted"; the done-when line becomes "Repeatable quests enforce their frequency, and
  their limit where set" (P3).
- **Roadmap:** phase 2 links this spec and names plans 2a and 2b; the "repeat frequency" and
  "one action" spec questions are marked answered by P3 and P4.
- **`docs/README.md`:** lists this spec, the two plans, `step-ids.txt` and `loop-backlog.txt`.
- **Rollout:**
  - The new save field starts empty, so there is no migration and no wipe.
  - Content is unchanged in 2a except the generated `step-ids.txt` and `loop-backlog.txt`.
  - The seven repeatables get no `repeat_limit`, so they stay unlimited.
  - Deploy note: none beyond phase 1's.
