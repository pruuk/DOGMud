# Quest arc (#375) closing step: hard validators, `look_at` and `search_at`, dash guards

**Status:** approved in brainstorm by the owner, 2026-11-06. One PR.
**Roadmap:** `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, "Closing step".

## Facts verified against source (master `a130651d9`, 2026-11-06)

| Fact | Source |
|------|--------|
| Boot already refuses a loop finding the backlog does not list (panic at boot, error on reload); listed findings only warn | `main_quests.go:107-136` (`questDoor`, `questDoorFrom`) |
| `loop-backlog.txt` holds comment lines only | `_datafiles/world/dogmud/quests/loop-backlog.txt` |
| The backlog API: `Backlog`, `ParseBacklog`, `FormatBacklog`, `Ratchet` | `internal/questloop/backlog.go:14,25,42,57` |
| `questworld.BacklogPath`, `questworld.LoadBacklog` | `internal/questworld/questworld.go:322,327` |
| The destination rule only logs at boot; CI ratchets it | `main_quests.go:64,77-84`; `internal/questengine/destinations_rule.go:13-16` |
| Backlog consumers beyond the above: six guards, the CI sweep, the quest editor's save door, delete refs, a key test | `internal/guards/{coulee,tier1..tier5}_backlog_guard_test.go`; `quest_loop_smoke_test.go:22,64-83,179-227`; `modules/gmcp/gmcp.Quest.go:45,61`; `modules/gmcp/gmcp.Delete_refs.go`; `internal/questloop/keys_test.go` |
| Phase 3a redefined `room_interact`: it fires only for the look family (look, l, examine, exam, read) and for search; `verb` picks the family | spec `completed/2026-10-26-quest-arc-phase3a-action-triggers-design.md` section 4 |
| `Trigger.Verb`, `VerbLook`, `VerbSearch`, `InteractVerb` | `internal/quests/triggers.go:25,92,93,98` |
| Loader: `room_interact` in the event set; `verb` refused on any other event; `count` refused on it | `internal/quests/quests.go:96,141,219,235-238` |
| Engine and tools that name it | `internal/questengine/engine.go:379`, `dryrun.go:31-37`, `walkthrough.go:684,847`; `internal/quests/samecommand.go:59-61` |
| Where it is raised | `internal/usercommands/usercommands.go:384-391,586-604` (`notifyRoomInteract`); `read.go:62`; `skill.search.go:72-103`; `internal/actions/search.go:100,133,599` |
| Quest editor's event lists (editor off since 2026-10-28) | `modules/gmcp/gmcp.Quest.go:150,164`; `_datafiles/html/public/static/js/quests.js:519,533` |
| Shipped triggers: 8 | quest 28 (`:281`, `:297`), 72 (`:81`, `:104`), 75 (`:62`, `:81`), 76 (`:156`, `verb: search`), 77 (`:109`) |
| Test files naming it: 25 | `grep -rln "room_interact\|RoomInteract" --include=*_test.go .` |
| Saves hold step ids (`"<quest id>-<step id>"`), not event names | `_datafiles/world/dogmud/quests/step-ids.txt` header; to be re-checked in the plan against `questprogress` |
| The prose dash guard leaves out quests, dialogue and conversations, and skips whole comment lines only | `internal/guards/prose_dash_guard_test.go:14-28,64` |
| Dashes left: dialogue 0; quests 11 lines, all comments (3 are trailing comments on a data line, `31-find_your_footing.yaml:28,49`, `75-the_surveyors_report.yaml:23`); conversations 6 (`pairs/116_117.yaml:12`, `pairs/259_262.yaml:28`, four `types/*.yaml` descriptions) | `grep -rn "[—–]"` over the three folders; spaced hyphens outside comments: 0 |
| The Coulee dash guard exists | `internal/guards/coulee_dash_guard_test.go` |

## What players see

Nothing changes in play, except two conversation lines that lose their dashes.

## Part 1: validators refuse boot; the backlog is retired

- Delete `loop-backlog.txt`, `internal/questloop/backlog.go` (and its test), and
  `questworld.BacklogPath` and `LoadBacklog`.
- `questDoor` refuses on any `questloop.Validate` finding: a panic at boot, an error on a
  reload, the same refusal an unlisted finding gets today. The "known findings" warning goes.
- `destinationDoor` refuses the same way on any destination finding, instead of logging it.
  The inferred-destinations count stays an info line.
- The walkthrough (`questengine.Walk`) stays CI-only, for boot time. `quest_loop_smoke_test.go`
  fails on any finding from `Validate` plus the walk; `DOGMUD_LOOP_BACKLOG_WRITE` goes.
- The quest editor's save door (`gmcp.Quest.go`) refuses any finding; it loses its `backlog`
  field. `gmcp.Delete_refs.go` and `keys_test.go` follow.
- The six backlog guards are deleted. One guard replaces them: no `loop-backlog.txt` exists,
  and no Go file defines or calls `Ratchet`, `ParseBacklog` or `LoadBacklog`. It ships with a
  fixture proving it can fail.

## Part 2: `room_interact` becomes `look_at` and `search_at`

- `look_at` fires for the look family (look, l, examine, exam, read). `search_at` fires on
  `search <noun>` naming the feature, after the search runs and with no roll, and on a plain
  search that uncovers the feature at `SearchRoomHiddenDifficulty`. Behaviour is unchanged.
- `Trigger.Verb`, `VerbLook`, `VerbSearch` and `InteractVerb` are deleted; the event name
  carries the family.
- Both events keep `room_interact`'s rules: `count` is refused, a handled event swallows the
  command (`countSwallowsCommand`), and `samecommand.go` compares event names.
- The loader refuses `event: room_interact` and any `verb:` key with a message that names
  `look_at` and `search_at`.
- The 8 shipped triggers move: quest 76's `verb: search` trigger becomes `search_at`, the
  other 7 become `look_at`. Comments that name `room_interact` are reworded.
- Every raise site, engine path, dry run (`SearchFeatures` indexes `search_at`), walkthrough
  path and the editor's event lists move. The editor stays off; its lists are kept correct.
- The authoring skill (`.claude/skills/dogmud-authoring-quests/SKILL.md`) and
  `.claude/commands/sketch-quest.md` describe the new events.
- A code guard fails if `room_interact` appears in any Go file or world YAML, with a fixture
  proving it can fail. Completed specs and plans are history and are not touched.

## Part 3: dash guards

- Fix all 17 lines, comments included, so the folders hold no em or en dash at all.
- Add `quests`, `dialogue` and `conversations` to `proseDashDirs`; drop the exemption comment.
- Delete `coulee_dash_guard_test.go`, now covered.

## Docs

- Roadmap: the closing step is done. `QUEST_DIALOGUE_REQUIREMENTS.md` boxes are the owner's
  to tick and are not touched.
- `context.md` for `internal/questloop`, `internal/quests`, `internal/questengine` and
  `internal/questworld`; `tools/context_md_audit.py` passes.
- `docs/README.md` lists this spec and its plan. A patch note only if the two conversation
  lines count as visible change (they are ambient copy; one line at most).

## Tests

- Each new guard ships a fixture that breaks it and a test that sees it break.
- Loader tests: `room_interact` and `verb:` are refused with the naming message; `look_at` and
  `search_at` load; `count` is refused on both.
- The existing look and search tests (25 files) move to the new names and stay green.
- Full `go test ./...`, two boots from a detached worktree, and a short playtest: Dewey's
  threshold stone (`look_at`, quest 28), the disc's `search stone` (`search_at`, quest 76),
  and a plain `search` uncovering a spot.

## Out of scope

Ticking the requirement boxes; the web builder (phase 6, postponed); #667 and #625.
