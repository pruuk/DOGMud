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
flags:
  - key: NN-door
    values: [trainer]
    description: The player took this quest from the trainer, not Toke.

triggers:
  # A trainer start skips the arrival talk (owner ruling 2026-11-22).
  - event: quest_granted
    quest_token: NN-start
    conditions:
      has_flag: {"NN-door": "trainer"}
      missing: ["NN-<second step>"]
    actions:
      - grant: "NN-<second step>"
```

In the trainer's dialogue file, a new offer:

```yaml
- key: <trade>_offer
  kind: offer
  quest: NN
  label: <quest title>
  playerLine: <player's ask, in the player's voice>
  text: <the trainer's pitch, carrying the lesson the arrival topic gave>
  questRequired: [31-end]
  questExcluded: [NN-start, NN-end]
  grantsQuest: NN-start
  setsQuestFlag: {key: NN-door, value: trainer}
  givesItem: <10043 for Vorn, 10038 for Iden, 20097 for Gethin; none otherwise>
```

Toke's offer needs no change: its `questExcluded: [NN-start, NN-end]` already
hides it once the trainer's offer is taken (ruling 3).

## Tasks

Run subagent-driven (Sonnet for the YAML, controller reviews each diff).
Brief every subagent on the Edit/Write rule and on `dogmud-authoring-quests`
and `dogmud-player-copy`.

1. **Quest 32 and Vorn first, as the pattern.** Flag, `quest_granted`
   trigger, Vorn's `drill_offer` with the cudgel, topic key in
   `topic-keys.txt`. Opener: a new variant placed before the "find Toke"
   one, `questRequired: [31-end]`, `questExcluded: [32-start, 32-end]`,
   whose text and hint point at the offer. The "find Toke" variant keeps its
   gates, so it now shows only before Find Your Footing, where it is still
   true. Boot check, then the controller plays it locally before the other
   seven start.
2. **The other seven** (35, 38, 41, 44, 47, 50, 98), same shape, one subagent
   per two or three files. Iden gives the sling, Gethin the lantern.
3. **Bridge copy.** Quests 90 and 93: "Crier Toke, who sent you to ..."
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
| Skip uses a flag plus `quest_granted`, not a `topic_heard` on the offer | `topic_heard` fires before the queued grant lands, so it would never match; Take the Tower already uses this pattern | Only the mechanism changes; player effect is the same |
| The trainer's offer text carries the arrival lesson | The skipped arrival topic is where the lesson lives; without it the player gets only the next-step opener | Drop the lesson from the pitch and keep it short |
| No flag value for Toke's door | A stale `trainer` flag only matters on a resume at `start`, which the grant chain leaves no time for | Add `setsQuestFlag: {NN-door: toke}` to Toke's eight offers |
| `start` step text that says Toke sent the player stays | A trainer-started player skips `start` and never reads it | Reword the eight `start` descriptions too |
| Arrival topic labels ("Crier Toke sent me...") stay | They show only on the Toke path, where they are true | Reword them neutral |
