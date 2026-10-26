# #375 Phase 3a Action Triggers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One acquire seam; quest gates on go and flee; look and search as the only interaction verbs; the search rework; and look and search taught in the newbie areas with copy that has passed an independent read and the owner's review.

**Architecture:** `acquire.AcquireItem` (leaf package `internal/acquire`) becomes the only way an item is acquired (guarded). The quest engine gains a side-effect-free `WouldFire` and a per-room list of search features; `go` and flee share `actions.QuestGateRefuses`. The command dispatcher raises `room_interact` only for the look family and search, matching template rooms. `actions.Search` splits into a whole-room search and a feature search, both on the shared special-move cooldown, with difficulties in config.yaml.

**Tech Stack:** Go, testify, world YAML, help templates.

**Spec:** `docs/superpowers/specs/2026-10-26-quest-arc-phase3a-action-triggers-design.md` (approved 2026-10-26).

---

## Rules for every task

- Edit files with the Edit and Write tools only (no `sed -i`, Python writes or `>` into tracked files).
- `git add <named paths>` only; never `-A`/`.`/`--all`, `commit -a` or `--no-verify`.
- Commit messages through a quoted heredoc ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- `gofmt -l .` over the whole repo prints nothing before every commit.
- Test binaries load Go defaults, not `config.yaml`: every new knob ships the same value in both.
- No em or en dashes in Go comments, player copy or docs.
- Any task that writes player-facing text loads `dogmud-player-copy` first; any task that edits world YAML loads `dogmud-authoring-content`; quest or dialogue edits load `dogmud-authoring-quests`.

## Facts verified against source (master `c90679ec7`, 2026-10-26)

| # | Fact | Where |
|---|---|---|
| F1 | `Character.StoreItem(i items.Item) bool`; 62 production call sites | `internal/characters/inventory.go:178` |
| F2 | `events.ItemOwnership{UserId, MobInstanceId, Item, Gained}` | `internal/events/eventtypes.go:308-315` |
| F3 | Store+event helpers `TransferItemToBackpack`, `TransferItemBetweenChars` | `internal/actions/transfer.go:51-109` |
| F4 | Acquisition sites that emit today: get (`usercommands/get.go:462/469, 546/548, 813/819`), give (`actions/give.go:26`), tree `return_item` (`behaviortree/actions_quest.go:189`), buy (`actions/buy.go:663/678`, `757/795`), loot (`usercommands/loot.go:107`), forage (`actions/forage.go:153`), plant (`plant.go:299`), steal (`steal.go:335, 611, 917`), bauble search (`search_bauble.go:363`), storage (`storage.go:386`), guild vault (`guild.go:663`), auctions (`modules/auctions/auctions.go:531, 958`), topic give (`conversation/playerstate.go:79`), timed craft (`hooks/NewRound_UserRoundTick.go:629`) | audit 2026-10-26 |
| F5 | Sites that store with no event: `questengine/bridge.go:168`, `hooks/Quest_HandleQuestUpdate.go:211, 236`, `behaviortree/actions_quest.go:120, 268`, `actions/salvage.go:423`, `actions/craft.go:241`, `usercommands/craft.go:764`, `usercommands/mail.go:30`, `hooks/combat_shared_helpers.go:225` | grep (verified) |
| F6 | Event before failed store: `buy.go:757` (store `:795` unchecked), `steal.go:335-342` | audit |
| F7 | Quest item sources counted by the validator | `internal/questworld/sources.go:95-105` |
| F8 | `Engine.Notify` runs `evaluate`, which matches (`matchTriggerFields`, conditions) and executes actions in one pass; no dry-run exists | `internal/questengine/engine.go:73-185, 200-229` |
| F9 | `questGateRefuses(user, room, destRoom)` notifies `room_gate` with the template id; called once | `internal/usercommands/go.go:51-63, 220` |
| F10 | `fleeExit(room, preferred)` filters locked exits only, then `room.GetRandomExit()` | `internal/actions/flee.go:210-219` |
| F11 | Dispatcher raises `room_interact` for every command before dispatch, `RoomId: room.RoomId`, `Verb: alias`, `Noun`, `RoomNoun: room.NounNamedBy(rest)` | `internal/usercommands/usercommands.go:383-394` |
| F12 | `command_issued` uses `rooms.OriginalRoomId` | `internal/usercommands/usercommands.go:537-539` |
| F13 | Loader's valid events map | `internal/quests/quests.go:115-120` |
| F14 | `TriggerDef.Verb` exists; engine matches it when set | `internal/quests/triggers.go:20`; `engine.go:224` |
| F15 | `triggerSignature` builds the backlog key | `internal/questloop/grants.go:99` |
| F16 | Walkthrough builds `EventDetails` from trigger filters | `internal/questengine/walkthrough.go:671-712` |
| F17 | `actions.Search(actor, SearchOptions{Feature})`; feature resolution `FindSearchFeature`; cooldown literal `TryCooldown("search", "2 rounds")`; hidden nouns roll `AgainstDifficulty(searchScore, 175.0)` and print "You discover something: <noun>" plus `HiddenDescription`; "You find nothing of interest." is deliberately identical for nothing-there and failed | `internal/actions/search.go:157-470`; `search_feature.go:15-110` |
| F18 | `HiddenNoun{Description, HiddenDescription}`; `Room.HiddenNouns map[string]HiddenNoun` | `internal/rooms/rooms.go:69-74, 113` |
| F19 | Rooms with `hidden_nouns`: kingsbarrow_vale 5441, new_plymouth_sewers 6418, pothole_coulee 5343 (The Reliquary), stillwater 4107 4115 4124 4130 4140, the_fernway_south 4161, the_foldweave 6428 6435 | `_datafiles/world/dogmud/rooms` |
| F20 | `command: search` triggers: quest 49 (The Old Shrine, room 5343, grants `49-find`) and quest 58 (Reading the Stones, room 5343, grants `58-end`) | quest files |
| F21 | Shared cooldown: `actions.ClaimSpecialMove(char) bool`, `ReleaseSpecialMove`, `SpecialMoveReady`; `Balance.SpecialMoveCooldown` = 4 | `internal/actions/special_move_cooldown.go`; `config.yaml:792` |
| F22 | `look` aliases `l`, `examine` | `_datafiles/world/dogmud/keywords.yaml:327` |
| F23 | `Read` command reads carried items | `internal/usercommands/read.go:14` |
| F24 | Cast quest notify and its stale comment | `internal/usercommands/skill.cast.go:375-394` |
| F25 | Newbie areas: newcomer antechamber rooms 6258 The Threshold, 6259 Knowing Yourself, 6260 What You Carry, 6261 The World Speaks, 6262 The Proving, 6467 The Landing; Dewey mob 9491 and dialogue `dialogue/newcomer_antechamber/9491.yaml`; Pothole Coulee (176 room files) | world data |
| F26 | Help pages `look.template`, `search.template` | `_datafiles/world/dogmud/templates/help/` |

## File map

- Create `internal/acquire/acquire.go` (`AcquireReason`, `AcquireItem`, `AcquireItemForChar`) with its `context.md`, `internal/guards/acquire_seam_guard_test.go`.
- Create `internal/actions/quest_gate.go` (`QuestGateRefuses`, `WalkableExit`).
- Create `internal/questengine/dryrun.go` (`WouldFire`, `SearchFeatures`).
- Create `internal/guards/hidden_noun_on_guard_test.go`, `internal/guards/search_trigger_noun_guard_test.go`.
- Modify as each task lists.

---

### Task 1: The acquire seam

**Files:** Create `internal/acquire/acquire.go`, `internal/acquire/acquire_test.go`, `internal/acquire/context.md`; modify `internal/events/eventtypes.go:308-313`.

> **Amended 2026-10-26 (Task 2):** the seam was first written as `internal/actions/acquire.go`, then moved to its own leaf package `internal/acquire` because `internal/actions` imports `internal/questengine`, so the quest bridge's `give_item` (`questengine/bridge.go`) could not call it. Callers write `acquire.AcquireItem(..., acquire.AcquireGet)`. The transfer-helper tests stay in `internal/actions/transfer_gain_test.go`.

- [ ] **Step 1: Failing tests** in `acquire_test.go`: `AcquireItem` on a user with room stores the item and queues exactly one `ItemOwnership{Gained:true, Reason:"get"}`; on a user whose `StoreItem` refuses (over twice carry capacity: build the fixture the way `bridge.go:155-168`'s comment describes), it returns false and queues nothing. Use `events.DrainQueuedEventsForTest` or the package's existing event-capture helper (grep `ItemOwnership` in existing `_test.go` files for the pattern).
- [ ] **Step 2: Run, watch it fail** (`undefined: AcquireItem`).
- [ ] **Step 3:** Add `Reason string // #375 3a: how the item arrived (acquire.AcquireReason); "" for losses` to `ItemOwnership`.
- [ ] **Step 4:** Create `acquire.go`:

```go
package acquire

// AcquireReason names how an item reached a player (#375 phase 3a). It rides
// on events.ItemOwnership so a later trigger filter can tell "crafted it" from
// "bought it" without another refactor.
type AcquireReason string

const (
	AcquireGet       AcquireReason = `get`
	AcquireGive      AcquireReason = `give`
	AcquireBuy       AcquireReason = `buy`
	AcquireLoot      AcquireReason = `loot`
	AcquireForage    AcquireReason = `forage`
	AcquireHarvest   AcquireReason = `harvest`
	AcquireSteal     AcquireReason = `steal`
	AcquireSearch    AcquireReason = `search_find`
	AcquireStorage   AcquireReason = `storage`
	AcquireVault     AcquireReason = `vault`
	AcquireAuction   AcquireReason = `auction`
	AcquireTopic     AcquireReason = `topic_give`
	AcquireCraft     AcquireReason = `craft`
	AcquireSalvage   AcquireReason = `salvage`
	AcquireReward    AcquireReason = `reward`
	AcquireQuestGive AcquireReason = `quest_give`
	AcquireTreeGive  AcquireReason = `tree_give`
	AcquireMail      AcquireReason = `mail`
	AcquireShieldSwap AcquireReason = `shield_swap`
)

// AcquireItem is the one way an item is acquired by a player: it stores the
// item and, only when it landed, raises ItemOwnership{Gained} so quest
// item_gain triggers and every other listener hear it. A guard
// (internal/guards/acquire_seam_guard_test.go) forbids StoreItem anywhere
// else except named non-acquisition moves.
func AcquireItem(user *users.UserRecord, itm items.Item, reason AcquireReason) bool {
	if user == nil || !user.Character.StoreItem(itm) {
		return false
	}
	events.AddToQueue(events.ItemOwnership{
		UserId: user.UserId,
		Item:   itm,
		Gained: true,
		Reason: string(reason),
	})
	return true
}
```

If a route acquires for a character with no `UserRecord` in reach, add `AcquireItemForChar(char *characters.Character, userId int, ...)` beside it rather than skipping the seam; record why in its comment.

- [ ] **Step 5:** Route `TransferItemToBackpack` and `TransferItemBetweenChars` (F3) through the seam for the gaining player, keeping their loss events as they are.
- [ ] **Step 6:** `go test ./internal/actions/ ./internal/events/` passes. Commit.

### Task 2: Every acquisition route through the seam

**Files:** every site in F4 and F5, plus F6.

- [ ] **Step 1: Failing tests**, one per formerly missing route (F5), each asserting exactly one `item_gain`-producing `ItemOwnership{Gained:true}` with its reason after the action: quest trigger `give_item` (questengine bridge), quest reward item and item grants (Quest_HandleQuestUpdate), behaviour-tree `give_item` and `give_item_multiple`, salvage, instant craft (both sites), mail, shield replacement. Plus: buy second path and steal with a refused store raise nothing.
- [ ] **Step 2: Run, watch them fail.**
- [ ] **Step 3:** Replace each F4 site's `StoreItem` + hand-placed `ItemOwnership` pair with one `acquire.AcquireItem(..., reason)` call (or `acquire.AcquireItemForChar` for an `Actor` or bare character), and each F5 `StoreItem` with the seam. Keep each site's existing refusal handling (a false return still does what a false `StoreItem` did). In `Quest_HandleQuestUpdate.go:211` remove the hand-fired item QuestToken notify only if `CheckItemQuests` now covers it (it does for `Gained` with a QuestToken item; verify by test that the token is granted exactly once).
- [ ] **Step 4:** `go test ./...` passes. Commit.

### Task 3: The StoreItem guard and the validator's sources

**Files:** Create `internal/guards/acquire_seam_guard_test.go`; modify `internal/questworld/sources.go` only if a source no longer matches a firing route.

- [ ] **Step 1:** Write the guard: walk production `.go` files under `internal/` and `modules/` (pattern of `walkGoCalls` in `internal/guards/blank_line_guard_test.go:105`), find calls to a method named `StoreItem`, and fail for any outside `internal/acquire/acquire.go` and an allowlist map `{"path/file.go": "reason"}` of non-acquisition moves (unequip, displaced gear, own-container moves, companion inventories, mob inventories). Every allowlist entry carries a one-line reason.
- [ ] **Step 2: Prove it can fail:** add a temporary `StoreItem` call in a scratch production file, run, see the failure naming it, remove it.
- [ ] **Step 3:** Add a test in `internal/questworld` asserting every source kind `sources.go:95-105` counts maps to a route that now raises the event (the reward and `give_item` kinds).
- [ ] **Step 4:** Run guards and questworld. Commit.

### Task 4: Engine dry-run and search features

**Files:** Create `internal/questengine/dryrun.go`, `dryrun_test.go`; refactor `engine.go` `evaluate` minimally.

- [ ] **Step 1: Failing tests:** `WouldFire("room_gate", details, player)` is true when a matching trigger's conditions hold, false otherwise, and executes no action (a trigger with `grant` leaves the player's tokens unchanged; a trigger with `actor` text sends nothing). `SearchFeatures(roomId)` returns the sorted distinct nouns of `room_interact` triggers with `verb: search` in that room.
- [ ] **Step 2:** Extract the match step of `evaluate` (field match at the chosen noun tier, then conditions) into `func (e *Engine) matching(eventType string, d EventDetails, player PlayerState) []*indexedTrigger`; `evaluate` uses it then executes; `WouldFire` returns `len(matching(...)) > 0`. `SearchFeatures` reads `e.triggerIndex["room_interact"]`.
- [ ] **Step 3:** `go test ./internal/questengine/` (including walkthrough tests) passes. Commit.

### Task 5: Walking gates for go and flee (closes #588)

**Files:** Create `internal/actions/quest_gate.go`; modify `internal/usercommands/go.go:51-63, 220`, `internal/actions/flee.go:210-219` and its caller; tests in `internal/actions/flee_gate_test.go`.

- [ ] **Step 1: Failing tests:** with a `room_gate` trigger guarding room B from A, a fleeing player in A with exits to B and C always flees to C; with only B, flee fails the way it fails with every exit locked; `go` into B is still refused with the trigger's narration; a teleport into B is unaffected.
- [ ] **Step 2:** `quest_gate.go`: move `questGateRefuses` to `actions.QuestGateRefuses(user, room, destRoom) bool` (unchanged behaviour: it narrates), and add `actions.QuestGateBlocks(user, room, destRoom) bool` using `questengine.GetEngine().WouldFire("room_gate", ...)` (no narration). `go.go` calls `QuestGateRefuses`.
- [ ] **Step 3:** `fleeExit` takes the fleeing user and skips exits where `QuestGateBlocks` is true, as it skips locked ones (`GetRandomExit` gains a filter or `fleeExit` builds the candidate list itself). Mob flee is unchanged (no user).
- [ ] **Step 4:** `go test ./internal/actions/ ./internal/usercommands/ ./internal/hooks/` (incl. `hooks/flee_quest_notify_test.go`). Commit with `Closes #588` in the body.

### Task 6: Two verb families for room_interact

**Files:** `internal/usercommands/usercommands.go:383-394`; `internal/usercommands/read.go`; `_datafiles/world/dogmud/keywords.yaml:327`; `internal/quests/quests.go` (verb validation); `internal/questloop/grants.go:99`; `internal/questengine/walkthrough.go:708`; `internal/usercommands/skill.cast.go:378` (comment).

- [ ] **Step 1: Failing tests** (usercommands): with a `room_interact` trigger on noun `floorboard` in room R, `look floorboard`, `l floorboard`, `examine floorboard`, `exam floorboard` and `read floorboard` (holding nothing so named) fire it; `say floorboard`, `whisper floorboard`, `help floorboard`, `get floorboard` do not; `search floorboard` fires only a `verb: search` trigger; the same trigger fires inside an instance of R; `pry floorboard` (no such command) prints `Try <look floorboard> or <search floorboard>.` and fires nothing. Loader rejects `verb: lift`.
- [ ] **Step 2:** Dispatcher: compute `family := interactionFamily(alias)` returning `look` for look/examine (after alias expansion `l` and `exam` resolve to look), `search` for search, `read` handled in step 3, else "". Raise `room_interact` only when family != "", with `Verb: family` and `RoomId` from `rooms.OriginalRoomId(room.RoomId)` (bridge keeps the real room). Remove the any-verb raise.
- [ ] **Step 3:** `read <x>`: when the player holds nothing matching, `Read` raises the look-family `room_interact` (same details) before its own refusal; if a trigger handled it, stop; else fall back to `look <x>`'s room-noun description when the room has that noun.
- [ ] **Step 4:** `keywords.yaml`: `look: ['l', 'examine', 'exam']`.
- [ ] **Step 5:** Nudge: in the dispatcher's unknown-command path, when `room.NounNamedBy(rest)` names a room noun (or a discovered hidden noun) and the command is not a known command, send `Try <ansi fg="command">look X</ansi> or <ansi fg="command">search X</ansi>.` (CategoryUsage, per M7 refusal conventions: read `internal/refusal` first) instead of the unknown-command line.
- [ ] **Step 6:** Loader: `verb` must be "", `look` or `search`; `triggerSignature` includes verb; walkthrough sends `Verb: "look"` when empty.
- [ ] **Step 7:** Fix the stale cast comment (F24) to say progression fires at resolution.
- [ ] **Step 8:** `go test ./internal/usercommands/ ./internal/quests/ ./internal/questloop/ ./internal/questengine/ ./internal/guards/ .` Commit.

### Task 7: Search rework

**Files:** `internal/actions/search.go`, `search_feature.go`; `internal/rooms/rooms.go:69-74`; `internal/configs/config.balance.go` + defaulter + `_datafiles/config.yaml`; `internal/usercommands/skill.search.go`; tests `internal/actions/search_split_test.go`; guard `internal/guards/hidden_noun_on_guard_test.go`.

- [ ] **Step 1: Knobs** (config.balance.go beside other search knobs, defaulter, config.yaml with a comment): `SearchRoomHiddenDifficulty: 175`, `SearchFeatureHiddenDifficulty: 125`. Test the defaults like Task 1 of the #680 plan did.
- [ ] **Step 2:** `HiddenNoun` gains `On string yaml:"on"` (the room noun or discovered hidden noun it is tucked into).
- [ ] **Step 3: Failing tests:**
  - `search <feature>` rolls only hidden nouns with `On == feature` at 125, plus the feature bauble; never finds secret exits, containers, stashes or hiders.
  - plain `search` rolls every tier as today, hidden nouns at 175; a found hidden noun prints the thorough line `You work through the room inch by inch. Tucked into the <on>, you find <noun>.` followed by its `HiddenDescription`.
  - both claim the shared special-move cooldown: after `bash` (or a direct `ClaimSpecialMove`), `search` is refused with the existing cooldown refusal and sends no quest event; after `search`, `ClaimSpecialMove` is false.
  - quest search triggers: `search <feature>` with a `verb: search` trigger on that feature fires it with no roll; plain `search` rolls once per `questengine.GetEngine().SearchFeatures(templateRoom)` feature at 175 and on success fires `room_interact` (`Verb: search`, `Noun: feature`) and prints the thorough line naming the feature.
  - "You find nothing of interest." stays identical whether nothing is there or a roll failed.
- [ ] **Step 4:** Implement: replace the `TryCooldown("search", "2 rounds")` with `ClaimSpecialMove` (players and mobs alike); split the tier loop on `hasFeature`; read the knobs; add the quest-feature rolls for plain search; move the targeted-search quest notify out of `skill.search.go`'s generic `command: search` notify (keep that notify for plain search only, until Task 9 migrates quests 49 and 58).
- [ ] **Step 5:** Replace the 2026-09-26/27 ruling text at `search_feature.go:15-36` with the new rule (owner, 2026-10-26: a targeted search finds only its feature's secrets; plain search the whole room at worse odds; both on the special-move cooldown).
- [ ] **Step 6: Guard** `hidden_noun_on_guard_test.go`: every shipped hidden noun has a non-empty `on` naming a room noun or another hidden noun in the same room; prove it fails on a planted fixture first. (It fails on shipped data until Task 9; commit it with Task 9 if that is cleaner, but write and prove it here.)
- [ ] **Step 7:** `go test ./internal/actions/ ./internal/usercommands/ ./internal/configs/ ./internal/rooms/`. Commit.

### Task 8: Search-trigger noun guard

- [ ] Create `internal/guards/search_trigger_noun_guard_test.go`: every `verb: search` trigger's noun names a room noun or hidden noun of its room (template). Prove it fails on a planted trigger. Commit.

### Task 9: World content: hidden nouns, quests 49 and 58, verb sweep

**Content writer:** load `dogmud-player-copy`, `dogmud-authoring-content`, `dogmud-authoring-quests`, and `feedback-write-for-players-new-to-muds` (memory). Write for someone who has never played a MUD.

- [ ] **Step 1:** For each F19 room: give every hidden noun `on:` (the visible feature it is tucked into, adding a room noun if none fits), and per ruling 7 add a subtle hint to that feature's look text (and to a hidden noun's own description when it hides a further step). Hints are observations, never instructions ("the flagstones near the plinth sit unevenly", not "try searching the flagstones").
- [ ] **Step 2:** Quests 49 and 58 (room 5343, The Reliquary): move `command: search` to `event: room_interact`, `verb: search`, `noun: <the feature the disc lies under>`; keep conditions, grants and actor text; check the step hints and offer text in the givers' dialogue still read right (search the feature, or the whole room).
- [ ] **Step 3:** Sweep the rooms of the 84 look-style `room_interact` triggers (quests 14, 18, 20, 63, 65, 68-77) and their quest hint and dialogue text for any verb other than look or search taught to the player ("pry", "lift", "dig", "move", "open the cache"): reword to look or search.
- [ ] **Step 4:** Collect every changed line (file, before, after) into `3a-content-diff.md` in the controller's session scratchpad (the controller passes its full path in the brief) for Task 12.
- [ ] **Step 5:** Guards from Tasks 7 and 8 pass; `go test ./internal/guards/ ./internal/questengine/ ./internal/questloop/ .` (walkthrough green). Commit.

### Task 10: Teaching look and search in the newbie areas

**Content writer:** same skills as Task 9, plus read every room in F25's antechamber and Dewey's whole dialogue first, so the new beats fit the existing voice and order.

- [ ] **Step 1: Look (Dewey's antechamber).** In the antechamber room that already teaches looking, or the one Dewey points at for it, make one room feature reward `look <feature>` with something worth finding (a line of lore or a small useful item through a `room_interact` look trigger), and have Dewey's existing teaching line name the idea in plain words: look closely at the things described around you.
- [ ] **Step 2: Search (Pothole Coulee).** The Reliquary (5343) already hosts quest 49's find: make its look text hint at the feature (Task 9), and make the givers' step hint teach both routes in plain words: search the thing that looks disturbed, or search the whole room (slower, less certain). If the Coulee has an earlier, gentler spot on the newbie path for a first search, add one there instead and leave the Reliquary as the second use.
- [ ] **Step 3:** Every taught command appears once, formatted as a command (`<ansi fg="command">look ledger</ansi>`), with no other verbs mentioned.
- [ ] **Step 4:** Append every changed line to the Task 9 diff file. Walkthrough and guards green. Commit.

### Task 11: Help pages and cross-links

- [ ] **Step 1:** Rewrite `help look` and `help search` (F26) for new players: look at what a room describes to learn more and sometimes find things; search a thing that looks disturbed, or the whole room at worse odds; searching uses the same cooldown as special moves. Each links the other.
- [ ] **Step 2:** Add `look` and `search` to the "See also" of the pages on room features, hidden things, quests, cooldowns and the new-player help (find them with grep in `templates/help/`; list them in the commit).
- [ ] **Step 3:** Help-template guards (`go test ./internal/usercommands/ ./internal/guards/`). Append changed help text to the diff file. Commit.

### Task 12: Content review gate (newbie areas)

The owner asked for extra review here: nothing goofy or confusing in the intro areas.

- [ ] **Step 1: Independent cold read.** A fresh Opus subagent with NO access to the spec or plan reads, in play order, every room, Dewey line, quest hint and help page touched by Tasks 9 to 11 (from the diff file plus the full surrounding room and dialogue text), as a player who has never played a MUD. It reports, per line: confusing, jargon, tone break, instruction buried in prose, verb other than look/search, raw numbers, dashes, a hint that gives the answer away, or a hint too faint to notice. Read-only; it writes its findings to the scratchpad.
- [ ] **Step 2:** Fix every finding the controller agrees with (record any it rejects, with the reason).
- [ ] **Step 3: Owner review page.** Build an HTML page (artifact-design skill) listing every changed player-facing line in the newbie areas (antechamber, Pothole Coulee, Dewey, the two help pages): room, before, after, why. Publish it and give the owner the link. **Stop and wait for the owner's approval; apply requested changes and republish until approved.**
- [ ] **Step 4:** Commit any fixes.

### Task 13: Docs

- [ ] `context.md` for `acquire` (shipped in Task 2; refresh if later tasks touch it), `actions` (QuestGateRefuses/Blocks, search split), `questengine` (WouldFire, SearchFeatures), `usercommands` (verb families, nudge, read fallback), `rooms` (HiddenNoun.On), `quests` (verb values); `docs/PATCH_NOTES.md` player-facing entry (look and search are how you find things; flee no longer slips past quest gates); roadmap phase 3 status; run `python tools/context_md_audit.py`. Commit.

### Task 14: Gate and PR

- [ ] Load `dogmud-shipping`; full pre-push gate including `GOOS=linux GOARCH=arm GOARM=5 go build .` and the detached-worktree boot check; push; `gh pr create --repo pruuk/DOGMud`, body with player effect first, rulings, deploy notes (no migration), the owner-approved content page link, `Closes #588` only. Do not merge before Task 15.

### Task 15: Playtest

Load `dogmud-playtesting`; brief seats with `project-680-playtest-harness-lessons-2026-10-26` and `project-375-phase2a-playtest-harness-lessons-2026-10-24`; one subagent per seat.

| Seat | Setup | Pass when |
|---|---|---|
| N1 naive newbie | fresh character, `creation_flow`, no coaching, no admin | reaches the Coulee having used `look <feature>` because the content suggested it; finds the Reliquary secret by `search <feature>` or plain `search` from the hints alone; reports any confusing line verbatim |
| N2 naive newbie | as N1, different seed | same; a second opinion on the copy |
| V verbs | admin profile, room with a look trigger and a search trigger | look/l/examine/exam/read fire; say/whisper/help/get do not; `pry X` gets the nudge; instanced room trigger fires |
| G gates | quest 14 at the cellar step; hostile mob in 485 | flee never enters 486 while gated; flee succeeds through another exit or fails like all-locked |
| S search | admin profile, Reliquary and a stillwater hidden-noun room | targeted vs plain odds feel right; thorough line reads well; search and bash share the cooldown both ways |
| A acquire | admin profile, quest with a reward item and an item_gain step | reward, salvage and mail items fire item_gain once; a full pack raises nothing |

- [ ] File findings (pre-existing ones as issues; this PR's as fixes on the branch with tests), post a summary comment on the PR, then merge after the owner-approved content review and green CI.
