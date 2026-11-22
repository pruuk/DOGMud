# Plan: Coulee trainers offer their own trade (#940)

Issue: #940. Owner rulings 2026-11-22 (recorded on the issue):

1. Toke keeps starting every trade chain. The first quest giver of each chain
   (the trainer) also offers it directly.
2. A player who takes the quest from the trainer skips the "go to the
   trainer" step.
3. Toke stops offering a trade once it is taken from the trainer, with no new
   line.
4. Find Your Footing (31) and Esk's Road (89) stay Toke only.
5. A trainer's offer is gated like Toke's: Find Your Footing done (`31-end`).

No spec: the rulings above are the design. This plan is the build.

## Facts verified against source (2026-11-22)

| Fact | Where |
|---|---|
| Toke's eight trade offers: `send_drill` 32, `send_forge` 35, `send_brew` 38, `send_steppe` 41, `send_folding` 44, `send_folk` 47, `send_range` 50, `send_light` 98 | `dialogue/pothole_coulee/9106.yaml` |
| Each requires `31-end`, excludes `NN-start` and `NN-end` | same |
| Toke gives an item on two: `send_drill` 10043 (cudgel), `send_range` 10038 (sling) | same |
| Trainer and second step per quest: 32 Vorn 9108 `met`; 35 Rusk 9116 `met`; 38 Birna 9128 `met`; 41 Tarn 9136 `met`; 44 Grieve 9144 `met`; 47 Wenna 9153 `met`; 50 Iden 9161 `met`; 98 Gethin 9125 `lantern` | each quest's `steps` |
| Every second step is granted by one `topic_heard` on the trainer's arrival topic, `has: NN-start`: `drill_arrive`, `heat_arrive`, `brew_arrive`, `sign_arrive`, `cast_lesson`, `words_lesson`, `shoot_lesson`, `first_light_lantern` | each quest's `triggers` |
| Gethin's arrival topic gives the lantern (`givesItem: 20097`); no other arrival topic gives an item | `9125.yaml` |
| Each trainer has an opener variant sending a not-yet-started player to Toke ("Crier Toke starts every trade in the coulee") | e.g. `9108.yaml:16-27` |
| Arrival topic labels say Toke sent the player ("Crier Toke sent me to learn to fight.") | e.g. `9108.yaml:126` |
| A dialogue `grantsQuest` is one token, and a player with no progress can only take `start` | `internal/quests/quests.go:385` (`IsTokenAfter`) |
| `topic_heard` fires synchronously after a topic's effects; the offer's grant is only queued (`events.Quest`), so a `topic_heard` trigger on the offer cannot see `NN-start` | `internal/conversation/talk.go:318`, `internal/conversation/playerstate.go:52` |
| `setsQuestFlag` is applied inside `Apply`, before the queued grant lands | `internal/dialogue/engine.go:144` |
| Precedent: an offer sets a flag, a `quest_granted` trigger reads it with `has_flag` | `quests/34-take_the_tower.yaml:87` (`34-boon`) |
| `HasQuest` counts earlier steps as held, so `missing: NN-met` is false for anyone past `met` (a resume past it never re-fires) | `internal/characters/quests.go:62-84` |
| Bridges that say Toke sent the player: quest 90 start ("Crier Toke, who sent you to Drillmaster Vorn") and quest 93 start | `quests/90-*.yaml:22`, `quests/93-*.yaml:20` |
| Each trade quest's `start` step text says Toke sent the player; a trainer-started player skips `start`, so never reads it | e.g. `quests/35-first_heat.yaml:31` |
| Dialogue topic keys are listed in a manifest | `dialogue/topic-keys.txt` (`internal/guards/topic_manifest_test.go`) |
| Quest flags must be declared in the quest YAML or boot panics | `dogmud-authoring-quests` skill, "Quest Flags System" |

## Mechanism (same for all eight)

In the quest file `quests/NN-*.yaml`:

```yaml
triggers:
  # A trainer start skips the arrival talk (owner ruling 2026-11-22).
  - event: quest_granted
    quest_token: NN-start
    conditions:
      in_room: <trainer's room>
      has: ["NN-start"]
      missing: ["NN-<second step>"]
    actions:
      - grant: "NN-<second step>"
```

(Amended in Task 1: the flag version is refused by the `one-action` loop
rule. See the decisions table.)

In the trainer's dialogue file, a new offer:

```yaml
- key: <trade>_start   # the <trade>_offer keys are retired in topic-keys.txt
  kind: offer
  quest: NN
  label: <quest title>
  playerLine: <player's ask, in the player's voice>
  text: <the trainer's pitch, carrying the lesson the arrival topic gave>
  questRequired: [31-end]
  questExcluded: [NN-start, NN-end]
  grantsQuest: NN-start
  givesItem: <10043 for Vorn, 10038 for Iden, 20097 for Gethin; none otherwise>
  objective: <the second step's task as it reads at the trainer; the card's first line>
```

Toke's offer needs no change: its `questExcluded: [NN-start, NN-end]` already
hides it once the trainer's offer is taken (ruling 3).

## Tasks

Run subagent-driven (Sonnet for the YAML, controller reviews each diff).
Brief every subagent on the Edit/Write rule and on `dogmud-authoring-quests`
and `dogmud-player-copy`.

1. **Quest 32 and Vorn first, as the pattern.** The `quest_granted`
   trigger (room gated, see Mechanism), Vorn's `drill_start` with the cudgel, topic key in
   `topic-keys.txt`. Opener: a new variant placed before the "find Toke"
   one, `questRequired: [31-end]`, `questExcluded: [32-start, 32-end]`,
   whose text and hint point at the offer. The "find Toke" variant keeps its
   gates, so it now shows only before Find Your Footing, where it is still
   true. Boot check, then the controller plays it locally before the other
   seven start.
2. **The other seven** (35, 38, 41, 44, 47, 50, 98), same shape, one subagent
   per two or three files. Iden gives the sling, Gethin the lantern.
3. **Bridge copy.** Quests 90, 91, 92 and 93 (corrected during the build: the
   first fact sweep found only 90 and 93): "Crier Toke, who sent you to ..."
   becomes wording true for both doors.
4. **Toke's own lines.** Audit `9106.yaml` for variants or topics gated on a
   trade's `NN-start` that assume he sent the player; reword any that would
   read false.
5. **Comments.** Replace the "Crier Toke starts every trade (#375 phase 7)"
   comments in the eight trainer files with the 2026-11-22 ruling.
6. **Gate.** `go run ./cmd/dialoguecheck -data _datafiles/world/dogmud
   -dialogue pothole_coulee`; `go test ./internal/guards/ ./internal/quests/
   ./internal/questengine/ ./internal/dialogue/`; boot check in a detached
   worktree; `bash tools/lint.sh`.
7. **Adversarial playtest** (content gate): one seat takes all eight trades
   from trainers, one from Toke, one mixes, and one tries to take a trade
   from the trainer before Find Your Footing, abandons and resumes, and loses
   the starter item. Findings go to GitHub issues.
8. **Patch notes**, then PR with "Part of #940".

## Owner test (ordinary player steps)

New character with `new`. Finish the Awakening and Toke's town walk. Walk to
the Drill Yard and `talk vorn`: First Blood is offered with `[!]`. Accept it:
you hold the cudgel and Vorn tells you to wield it. Go back to Toke: First
Blood is no longer on his menu, the other trades still are.

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| Skip uses a `quest_granted` trigger gated `in_room: <trainer's room>`, not a `topic_heard` on the offer and not a flag (amended during Task 1) | `topic_heard` fires before the queued grant lands; the `one-action` loop rule refuses a flag condition on a step grant (`internal/questloop/rule_oneaction.go:45`). Toke's schedule keeps him in 5203 at every hour and each trainer has `maxwander: 0` in their own room, so the room tells the doors apart | If a trainer or Toke ever moves rooms, the trigger's room needs updating |
| `chain` loop rule accepts a second door (amended during Task 1; taken without asking, then **approved by the owner 2026-11-22**, option A) | The rule refused any offer gated on `31-end` off Toke, who takes 31's hand-in. It stands for requirement 65 ("Every chain offers its next quest at hand-in", `docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`), which still holds while Toke offers every trade. Now an offer also passes when another offer for the same quest, behind the same gate, sits on the hand-in NPC | Revert the rule change and give each trainer a bridge quest instead |
| The talk menu after any accepted offer, and after a hand-in, is redrawn from the queue (`TalkMenuAgain`, priority 45) once the grant and its `quest_granted` chain have landed; `heldAlso` is removed (scope widened during Task 2 review; taken without asking, then **approved by the owner 2026-11-22**) | Redrawing at once showed a trainer-door player the skipped arrival topic as the objective, labelled "Toke sent me". Item offers already waited this way; leaving hand-ins on the old path would keep two mechanisms for one job | Revert to the immediate redraw and give the door path its own exception |
| The trainer's offer text carries the arrival lesson | The skipped arrival topic is where the lesson lives; without it the player gets only the next-step opener | Drop the lesson from the pitch and keep it short |
| `start` step text that says Toke sent the player stays | A trainer-started player skips `start` and never reads it | Reword the eight `start` descriptions too |
| Arrival topic labels ("Crier Toke sent me...") stay | They show only on the Toke path, where they are true | Reword them neutral |

## Addendum (2026-11-22): quest-given items vanish when they leave you

**Found by the playtest** (breaker seat, raw log `Drillyard Cudgel (x2)`): drop
the starter item, choose the trainer's "I've lost my cudgel." line, pick the
old one up. Repeat for as many as you like. Live on master, on every
replacement line (Vorn, Iden, Gethin, Birna, Tarn). The items sell: cudgel 12,
sling 40, pouch of shot 40, lantern 20, firebomb 20.

**Owner ruling (2026-11-22, option B):** a copy a quest hands you vanishes
whenever it leaves your hands (drop, give, put in a container, store, mail,
and the rest below). Shops will not buy it. While you keep it, it is ordinary
gear, and you keep it after the quest ends. Fix it in this branch.

### Facts verified against source (2026-11-22)

| Fact | Where |
|---|---|
| A topic's `givesItem` makes `items.New(itemId)` and stores it through `acquire.AcquireItem(user, newItem, acquire.AcquireTopic)` | `internal/conversation/playerstate.go:58-74` |
| A quest trigger's `give_item` does the same with `acquire.AcquireQuestGive` | `internal/questengine/bridge.go:290-299` |
| Those two reasons (`topic_give`, `quest_give`) are used nowhere else | `internal/acquire/acquire.go:36,40`; grep of `internal/` |
| Quest rewards (`rewards.itemid`) use `acquire.AcquireReward`, a separate path | `internal/hooks/Quest_HandleQuestUpdate.go:224-231` |
| Five of the six starter items are ordinary shop and loot goods too (the hooded lantern has 11 other sources), so the mark must be on the copy, not the template | grep of `shops/`, `mobs/` |
| `items.Item` already carries per-copy fields such as `StashedBy int yaml:"stashedby,omitempty"`, and saves with the character by `yaml.Marshal`, so a new tagged field persists with no other change | `internal/items/items.go:42,65`; `internal/users/autosave_prepare.go:56` |
| No single function sees every way an item leaves: only drop and give go through `actions/transfer.go` (`TransferItemToFloor` :22, `TransferItemBetweenChars` :72) | read-only sweep of 56 files |
| Voluntary ways out that bypass it: put (`usercommands/put.go:117`), stash (`stash.go:30`), storage and stow (`storage.go:371` `storageAddQuiet`), mail (`mail.go:217`), guild donate (`guild.go:661-671`), auction (`modules/auctions/auctions.go:494`), plant x3 (`actions/plant.go:160,319,476`), give to a pet (`usercommands/give.go:403`), trade-in as payment (`actions/buy.go:249`) | same sweep |
| Every sell, offer and appraise prices through one function, which already refuses template quest items with a `Tell` line | `internal/actions/sell_quote.go:59,66-67` |
| `sell <name>` and `sell all` check before that function | `internal/actions/sell.go:179-183, 253` |
| Death moves nothing: the corpse keeps a copy of the character, and looting reads only `corpse.Loot`, which a player corpse never fills | `internal/hooks/Death_PlayerCorpse.go:60`; `internal/usercommands/get.go:790-812` |
| Using up, eating, drinking, throwing, crafting and key absorption destroy the item; they are not ways out | same sweep |

### Mechanism

- `items.Item` gains `QuestGiven bool yaml:"questgiven,omitempty"`, plus one
  method that says so (e.g. `IsQuestGiven()`).
- Set at one place: `acquire.AcquireItemForChar` marks the copy when the
  reason is `AcquireTopic` or `AcquireQuestGive`.
- One helper, e.g. `actions.VanishQuestGiven(...)`, removes the copy and sends
  the line. Every voluntary way out above calls it instead of moving the item:
  drop, give to another player, put, stash, storage and stow, mail, guild
  donate, auction, plant. `sell` refuses in `QuoteSale` (and `sell all` skips
  it); a trade-in never takes a quest-given copy as payment.
- **Giving one to an NPC (owner ruling 2026-11-22, on review of this
  addendum):** the NPC considers it and hands it back; it never keeps it and
  it does not vanish. A companion (a creature that follows and fights for the
  player: a charmed mob or a pet) takes it as usual and may equip it. Today an
  NPC that did not ask for an item "considers the X for a moment" and then
  runs `gearup`, which equips it if better and otherwise KEEPS it
  (`usercommands/give.go:318-319`, `mobcommands/gearup.go`); only a
  behaviour-tree `player_give` handler hands items back. So for a quest-given
  copy and a non-companion NPC: the existing "considers" line, then a hand-back
  line, and the copy stays with the player. A topic that wants the item
  (`requiresItem` / `carriesItem`) and a `player_give` tree handler keep their
  current behaviour. Decision taken without asking: a pet counts as a
  companion here (if wrong, pets hand it back too).
- `look` / `examine` on the item adds one line saying it is quest gear and
  vanishes if it leaves your hands, so no one loses it by surprise.
- A guard test lists every `RemoveItem` call site outside the allowed ones
  (consumption, in-place replacement, the vanish helper), so a new way out
  added later fails CI until it is classified. This keeps the ruling enforced.

### Tasks

9. **Mark and vanish** (Opus: many touch points). Field, setter at the acquire
   seam, vanish helper, every voluntary way out above, the sell refusal, the
   trade-in filter, the look line, unit tests per way out (a quest-given copy
   vanishes; a bought copy of the same item moves as before), and the guard.
   Update `internal/items/context.md`, `internal/acquire/context.md`,
   `internal/actions/context.md`.
10. **Re-run the breaker seat** on the cudgel, sling and lantern: drop, give,
    put in a container, sell, then ask the trainer. One copy at a time, never
    two.
11. **Patch notes** line.

### Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| Quest rewards (`rewards.itemid`) are NOT marked | A reward is paid once and no NPC replaces it, so it cannot be farmed; players expect to keep and trade rewards | Mark `AcquireReward` too |
| Mail, guild donate and auction make the copy vanish rather than refuse | One rule for every way out is easier to learn and to enforce than a list of refusals | Refuse those three with a line instead |
| ~~Giving a quest-given copy to an NPC that does not want it makes it vanish~~ **Overruled by the owner 2026-11-22**: see the ruling below | | |
| No migration: copies handed out before this change are not marked | There are no live players; marking old copies would need guessing which were quest-given | Mark every carried copy of a quest starter item at login |
| Vanish line: "The Drillyard Cudgel fades away. Quest gear lasts only while you carry it." | Plain, says what happened and why | Reword |
