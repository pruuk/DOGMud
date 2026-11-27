# Quest branches (#958): spec and plan

Owner ruling 2026-11-26 (option A on #958): quests learn to branch. Each step
can name the side it belongs to, the quest validator checks each side as its
own line, and the quest log counts only the player's own side's steps. Quest
11, The Windwarden's Dilemma, is the first user.

Branch `fix/quest-11-split-sided` already holds commit `725b8d9e9` (quest 11
split into `sylara`, `totem`, `rhett`, `samples`; load migration; dialogue
gates; tests). It passes build, lint and the unit suites, and fails boot on the
straight-line validator. This work builds on it.

## Facts verified against source (master `41a002523`, 2026-11-26)

| Fact | Where |
|---|---|
| A step is `Id`, `Description`, `Hint`, `Destination`; no branch field | `internal/quests/quests.go:80` |
| A quest declares flags as `QuestFlagDef{Key, Values, Description}`; key is local (`branch`), registered as `11-branch` | `quests.go:25`, `:74`, `save.go:24` `flagKey` |
| Step order is file order. `IsTokenAfter` walks `Steps` from the current step looking for the next | `quests.go:379` |
| Only two callers of `IsTokenAfter`: `Character.HasQuest` (a later step counts as holding earlier ones) and `GiveQuestToken` (a grant must move forward) | `internal/characters/quests.go:83`, `:116` |
| The dialogue bridge, behaviour trees and quest conditions all reach `Character.HasQuest` | `questengine/bridge.go:80`, `behaviortree/conditions_player.go:9` |
| The walkthrough player has its own copy: `HasQuest` and `grant` index into a per-quest step id list it is given | `questengine/walkplayer.go:32`, `:53`, `:66` |
| The walk builds that list from every step of every quest, then walks each quest step `c` to `c+1`, and reports a firing that passes a step as "skips to" | `questengine/walkthrough.go:38-56`, `:284`, `:531` `skipped` |
| The one-action rule wants every way into step `i` to require step `i-1`, and refuses any flag, item or gold condition on a trigger | `internal/questloop/rule_oneaction.go:20-47` |
| Start, end, hint, hint-length and talk-on-quest rules read only the first, last, or each step on its own; they need no change | `questloop/rules_loop.go:14`, `:36`, `:64`, `:77`; `rule_talk.go:11` |
| Quest log progress is `completed / len(Steps)`, computed twice: the `quests` command and the GMCP Char quest payload | `usercommands/quests.go:124-145`, `modules/gmcp/gmcp.Char.go:729-756` |
| The web quest builder's step editor writes `id`, `description`, `hint`, `map_target` and nothing else, so a builder save drops every step's `destination` (`map_target` left Go at #375 phase 4) | `_datafiles/html/public/static/js/quests.js:746-763`; `SaveQuest` writes the posted struct, `save.go:33` |
| Quest 11 is the only quest with step-level sides; quest 49 mentions "branch" only in prose | `grep -ln branch quests/*.yaml` |
| The walk's Ways are indexed by token over the whole world, so both hand-ins are Ways into `11-end` on every path | `questloop/ways.go:45-66`, `walkthrough.go:371-419` (BAR) |
| A trigger whose grant the character refuses still runs its other actions and marks the command handled | `questengine/engine.go:202`, `:236`, `:413-445`; `bridge.go:188` |
| `cmd/dialoguecheck` keeps its own step-order model | `cmd/dialoguecheck/walk.go:28-49` `grantTo`, `held` |
| The playtest overlay grants quest tokens before it sets quest flags | `playtestprofiles/overlay.go:116-130` |
| Repeating a quest clears its tokens, not its flags | `Quest_HandleQuestUpdate.go:347`, `characters/quests.go:182-205` |
| `DoQuestStepMigration` (run when the user index is missing) resets an unknown saved step to `start` | `users/migration.go:10-31`, `main.go:423-427` |
| A builder save also drops `repeat_limit`, `rewards.summary`, `rewards.surprise` and trigger `conditions.missing_items` (quest 76 uses it); the editor door never runs the destination check boot runs | `quests.js:632-650`, `:854-877`; `gmcp.Quest.go:234-257`; `main_quests.go` `destinationDoor` |

## Design

**Authoring.** A step may carry `branch: <value>`. The value must belong to
exactly one of the quest's declared flags, and every branched step in a quest
must use the same flag (one fork per quest). The first and last steps cannot
be branched. `Quest.Validate` refuses anything else.

```yaml
flags:
  - key: branch
    values: [sylara, rhett]
steps:
  - id: start
  - id: sylara
    branch: sylara
  - id: totem
    branch: sylara
  - id: rhett
    branch: rhett
  - id: samples
    branch: rhett
  - id: end
```

**A player's path** is the quest's steps in file order, keeping the unbranched
steps and those whose branch equals the player's side. The side is the branch
of the player's current step when that step is branched; only on an
unbranched step is it read from the flag. So a save resting on `rhett` stays on
Rhett's path whatever its flag says (a mismatch is logged), and can never be
left holding no step. With the flag unset on an unbranched step, the path is
the unbranched steps only. A quest with no branched steps has one path, all of
its steps, so every existing quest behaves exactly as today. A branched quest
may not be repeatable (a repeat clears tokens, not flags, so the second run
would be locked to the first run's side).

**One helper, every reader.** `quests` gains `(*Quest).Path(flag func(string)
string) []QuestStep`, `(*Quest).Paths() [][]QuestStep` (one per declared value
of the branch flag, or the single path), `IsTokenAfterOnPath(cur, next, flag)`
and `(*Quest).StepProgress(stepId, flag) (done, total int)`. Then:

- `Character.HasQuest` and `GiveQuestToken` use the path. A step off the
  player's path is never held and never granted. So a Rhett player no longer
  "holds" Sylara's steps. `Path` works on indexes, without building a slice,
  because `HasQuest` runs on every trigger check.
- **The trigger engine skips a trigger that grants a branched step off the
  player's path**, before it runs any action. Refusing only the grant (BAR 3)
  would still play the trigger's text, hand over its items and swallow the
  command. With this, triggers need no flag condition, and the one-action
  rule's ban on flag conditions stays as it is. The talk menu's "would this
  fire" check (`conversation/playerstate.go:157` `WouldFire`) reads the same
  gate.
- The quest log and the GMCP payload both call `StepProgress` (today's two
  copies become one). A Rhett player at `rhett` reads step 2 of 4, not 4 of 6.
  An unsided player at `start` reads 1 of 2; after choosing, 2 of 4. The bar
  holds at half across the choice, which is true. The admin `all+` row keeps
  its own count.
- **one-action** checks each path as its own line. A way into a step must
  require that step's previous step on some path that contains it, and every
  such previous step must be required by at least one way (so no side is left
  with no way on). Unbranched quests have one path: the rule is unchanged for
  them.
- **The walkthrough** walks each path in turn over the real quest. The walk
  player's `HasQuest` and `grant` call the same path helper with the walk
  player's own flags, so a side whose flag is set after its grant fails the
  walk as it would fail in play. A path walk tries only the Ways whose
  required steps of their own quest all lie on that path (the predicate
  one-action uses), so Sylara's hand-in is not reported as a dead end on
  Rhett's path (BAR 1). Each walk checks the flag ended on its path's value.
  Findings name the path in the message, never in the key; the quest is
  registered with the engine once, and the shared step map is never changed
  for other quests.
- **`cmd/dialoguecheck`** drops its own step-order copy (`qmodel.grantTo`,
  `held`) for the path helper.
- **Loose ends that grant before a flag exists:** the playtest overlay sets
  flags before tokens. `DoQuestStepMigration` (via `SearchOfflineUsers`)
  resets any step the loaded quest lacks to `start`, so it would catch
  `11-sided` if it runs before `MigrateQuest11SidedStep`. Not yet verified
  which runs first; either order is safe (the save ends on its side's step, or
  on `start` with its flag, where the stand reopens). Task 2 checks the order
  and tests the outcome.

**Quest 11** then drops its trigger flag conditions (the engine guards the
side now) and its hand-ins require the return step (`11-totem`, `11-samples`),
the step before `end` on each path. The two long hints are cut under 200
characters. The migration in `725b8d9e9` is unchanged.

**Builder.** The step editor gains a Branch picker (blank, or a value of the
quest's flags). The whole quest editor gathers onto a deep copy of the quest it
loaded, at every level, so a field it has no control for survives a save: step
`destination`, `repeat_limit`, `rewards.summary` and `rewards.surprise`,
trigger `conditions.missing_items`, and any field added later. The dead
`map_target` controls go, with a read-only line showing each step's
destination; a new step gets its destination from its triggers or from the
YAML file, and the help text says so. The editor door runs the same
destination check boot runs, so a save that would panic the next boot is
refused. This fixes the pre-existing loss of all of these on a builder save
(found while reading this editor; BAR 7 widened it).

## Decisions taken without asking

1. **`branch: <value>` with the flag found from the value**, instead of naming
   the flag on each step. Shorter to write, and the loader refuses a value two
   flags share. If wrong: the field becomes `branch: {flag: value}`, a loader
   change only.
2. **One fork per quest.** Nested or sequential forks are not needed by any
   shipped quest. If wrong: paths become a tree; the helper's callers do not
   change.
3. **Off-path steps are never held.** This changes `HasQuest` for branched
   quests only. Quest 11's gates are written for it. If wrong: a Rhett player
   again "holds" Sylara's steps, and every gate on them needs a flag check, as
   in `725b8d9e9`.
4. **The engine skips a trigger that would grant an off-path step**, whole,
   rather than the validator allowing flag conditions on triggers. If wrong:
   the one-action rule's flag ban gets a branch exception instead (`has_flag`
   equal to the step's branch, on triggers granting a branched step).
5. **Hand-ins require the return step**, not the fetch step as `725b8d9e9`
   has. That is what the per-path one-action rule asks; the item pickup
   trigger moves a player carrying the item to the return step, including one
   who already held it when the fetch step began. If wrong: a player whose
   pickup somehow did not fire cannot hand in until they drop and pick up the
   item.
6. **The builder fix rides along** because the Branch picker sits in the same
   editor, and adding a field there without fixing the fields it already loses
   would leave the editor inconsistent. If wrong: split it into its own PR
   under #377.
7. **The side comes from the current step when that step is branched**, not
   from the flag (BAR 4). A save whose flag disagrees with its step keeps
   moving on its step's side. If wrong: such a save is moved back to `start`
   at load instead.
8. **Branched quests cannot be repeatable** (BAR 6), rather than clearing the
   flag on a repeat. No repeatable quest branches today. If wrong: a repeat
   clears the branch flag, and the side is chosen again each run.
9. **The builder gets no destination editor.** It keeps and shows what the
   file has. If wrong: a destination editor is its own task under #377.

## Plan

Each task is test first, on `fix/quest-11-split-sided`.

1. `quests`: `Branch` field, `Validate` rules (one flag, not first or last
   step, not repeatable), `Path` (side from a branched current step, else the
   flag; no allocation), `Paths`, `IsTokenAfterOnPath`, `StepProgress`. Table
   tests over a branched and a plain quest: flag unset, flag disagreeing with
   the step, every refusal.
2. `characters`: `HasQuest` and `GiveQuestToken` on the path. Tests: a Rhett
   player does not hold `11-sylara`; a Sylara grant to a Rhett player is
   refused; plain quests unchanged (existing suite). Check whether
   `DoQuestStepMigration` or `MigrateQuest11SidedStep` meets a `11-sided` save
   first, and test the outcome for both orders.
3. Trigger engine: skip a trigger granting an off-path branched step before
   any action, and `WouldFire` the same. Test: a branch-entry trigger with an
   `actor` line and a `give_item` does nothing for the other side.
4. Quest log and GMCP: both call `StepProgress`; delete the copies; `all+`
   keeps its own. Test the Rhett count reads 2 of 4.
5. `questloop`: per-path one-action, with a failing test for a branch that has
   no way on, and the existing tests unchanged.
6. Walkthrough: walk player on the path helper with its own flags; walk each
   path trying only the Ways whose required steps lie on it; path named in
   messages. Tests: a branch that dead-ends is reported; a branch whose flag is
   set after its grant is reported; quest 11 reports nothing.
7. `cmd/dialoguecheck` on the path helper; the playtest overlay sets flags
   before tokens (with a test).
8. Quest 11 content: `branch:` on its four steps, drop trigger flag
   conditions, hand-ins on the return steps, hints under 200, comments in the
   quest and both dialogue files rewritten for the new rule. The dialogue test
   builds its states from a real `Character` with quest 11 loaded, at both
   fetch and return steps of each side, instead of hand-written held sets.
9. Builder: Branch picker, gather onto a deep copy of the loaded quest at every
   level, read-only destination line, drop `map_target`, help text, editor door
   runs the destination check. Tests: a save round-trips quest 11 and quest 76
   unchanged. Run the `validate / javascript` job's checks locally.
10. Docs: the `dogmud-authoring-quests` skill and its `.agents/` mirror (a
   Branches section), `docs/schemas/dialogue.md`, and the `context.md` of
   `quests`, `questloop`, `questengine` (walk player rule) and `characters`
   (`HasQuest`); patch notes line already written.
11. Gates: gofmt, build, touched suites, `tools/lint.sh` to 0, boot check in a
   detached worktree to `Server Ready`, then an adversarial playtest of quest
   11 on both sides (fresh characters, ordinary player steps) before the PR.

## BAR (2026-11-26)

One Opus reviewer, given the spec and the code. 13 findings; the controller
checked findings 3, 5, 8 and 13 against source and took the rest as reported.
All are folded in above:

- Blocker: the per-path walk would report the other side's hand-in as a dead
  end, because Ways are indexed over the whole world (walk only the Ways whose
  required steps lie on the path).
- The walk player kept its own step rule, so a flag set after its grant would
  pass the walk and fail in play (walk player on the path helper).
- Refusing only the grant still ran a trigger's text and items for the other
  side (the engine skips the whole trigger).
- A save whose flag disagrees with its step would have held no step (side from
  the current step).
- The playtest overlay grants before it sets flags (flags first).
- A repeatable branched quest would lock to the first run's side (refused).
- The builder loses more than destinations, and its door skips the
  destination check boot runs (deep-copy gather, check at the door).
- `cmd/dialoguecheck` has a second step-order copy (path helper).
- The dialogue test's hand-written held sets could not catch a `HasQuest`
  regression, and the planned update would have failed (real `Character`).
- Walk hazards: register once, never alter the shared step map, path in the
  message only.
- Docs and comments missed by the plan (task 10, task 8).
- The quest log bar holds at half across the choice (accepted as true) and
  `HasQuest` is hot (no allocation).
- Facts table corrections (quest 49, `DoQuestStepMigration`).

The reviewer traced both sides through Kael, Sylara and Rhett under the new
rule and found neither stuck, neither shown the other's topics, no re-offer,
and the flag set before the grant on every path a player takes.
