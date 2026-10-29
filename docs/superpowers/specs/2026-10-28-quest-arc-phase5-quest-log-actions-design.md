# #375 phase 5: quest log actions (abandon and focus)

Issue: #375 (quest and dialogue arc), roadmap `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`
"Phase 5: Quest log actions (engine half)". Goal: players control their log. Brainstormed with
the owner 2026-10-28. One spec, one slice.

## Facts verified against source (master `4342f3be8`, 2026-10-28)

| Fact | Where |
|---|---|
| `Character.QuestProgress map[int]string` (`questprogress`), `QuestCompletions`, `QuestFlags map[string]string` (`questflags`), `LastQuestId int` (`lastquestid`) | `internal/characters/character.go:320-325` |
| `GiveQuestToken(token) bool` advances only when the token is after the current one (`quests.IsTokenAfter`), clears that quest's step counts, sets `LastQuestId` for a non-secret quest; granting `N-start` to a player past it is a no-op returning false | `internal/characters/quests.go:104-131` |
| `HasQuest(token)`: false when the quest has no `QuestProgress` entry; true on that step or any later one | `internal/characters/quests.go:67-89` |
| `GetFocusQuestId()`: drops secret quests, keeps an explicit `LastQuestId` while it is active, else picks another active quest | `internal/characters/quests.go:359` |
| `Char.Quests.Focus {id}` sets `LastQuestId` for an active non-secret quest and re-sends `Char.Quests` | `modules/gmcp/gmcp.CharOp.go:135-160` |
| Quest panel: a left click on a `.qrow` sends `Char.Quests.Focus` | `_datafiles/html/public/webclient-pure.html:1076-1082` |
| Right-click menus to copy: the inventory tile menu (`openMenu`, close on outside click, Escape, scroll) and a second one | `webclient-pure.html:3679, 3837` |
| `GMCPCharModule_Payload_Quest` fields (id, name, repeatable, status, description, completion, hint, progress, focused, target fields, guide) | `modules/gmcp/gmcp.Char.go:1188-1214` |
| Offer eligibility: a topic's gates (`questRequired`, `questExcluded`, items, flags, gold) plus `QuestOpen` for a repeatable offer; the NPC's marker is the best marker over eligible topics | `internal/dialogue/topics.go:24-82`; `topic.go:29-33` |
| A topic's effects run in `applyQuestEffects`: `givesItem` first (failure applies nothing), then `requiresItem` taken, `grantsQuest`, `setsQuestFlag`, `bumpsRep`, `givesGold`; `chargesGold` is taken first in `Apply` and refunded on failure | `internal/dialogue/topics.go:174-190`; `engine.go:112-160` |
| `grantsQuest` queues `events.Quest`; the hook calls `GiveQuestToken`, refreshes markers, fires `quest_granted` triggers only on a fresh grant, and prints "You have been given a new quest" for a `start` step | `internal/conversation/playerstate.go:60-65`; `internal/hooks/Quest_HandleQuestUpdate.go:104-140` |
| Other grant paths: questengine's synchronous grant (`GameBridge.GrantQuest`, its own "given a new quest" banner), behaviour-tree quest action, playtest overlays | `internal/questengine/bridge.go:146-190`; `internal/behaviortree/actions_quest.go:40`; `internal/playtestprofiles/overlay.go:117` |
| The engine fires `quest_granted` for every token a trigger's actions grant (`executeActions` then a chain over `grantedTokens`); the hook fires it for a fresh dialogue grant | `internal/questengine/engine.go:225-248`; `Quest_HandleQuestUpdate.go:118-130` |
| `GameBridge{user, roomId}`, `GiveItem(itemId) error` (acquire, "You receive a ..."), `GiveGold(amount)` | `internal/questengine/bridge.go:32-40, 222-250` |
| 19 shipped `quest_granted` triggers: chains on 7, 9, 15, 29, 31, 88 (grant), 78 (say, grant), 50 (give Pouch of Shot 30064, value 40, `missing_item` only), 71-end (rep), 79 to 87 (recipes, secret) | `_datafiles/world/dogmud/quests/*.yaml` scan |
| Behaviour trees check `-start` tokens; the one start grant is the antechamber's `30-start` on finishing quest 28 | `_datafiles/world/dogmud/behaviors/rooms/newcomer_antechamber/6467.yaml:111` |
| `quests` command (aliases `q`, `quest`); `quests all` and `quests all+` (admin) | `internal/usercommands/quests.go:117, 207`; `_datafiles/world/dogmud/keywords.yaml:345` |
| Name matching helper `util.FindMatchIn(search, items...) (match, closeMatch)` | `internal/util/util.go:370` |
| `ItemSpec.Value int` | `internal/items/itemspec.go:269` |
| Help text `help quests` | `_datafiles/world/dogmud/templates/help/quests.template` |
| `GetFocusQuestId` fallback: the lowest-id active quest not at `end` | `internal/characters/quests.go:379-390` |
| Dialogue loads lazily per NPC (`dialogue.Load(mobId, zone)`, cached); `dialogue.LoadAllFiles()` reads every file, used by `questworld.Registry` for the boot quest checks; `SaveDialogueFile` writes a file and replaces its cache entry | `internal/dialogue/loader.go:46`; `internal/questworld/questworld.go:240`; `internal/dialogue/save.go:29-52` |

**Shipped offers (310 dialogue files, 155 `grantsQuest` topics).** 76 non-secret quests are
offered by an NPC (some topic grants their `-start`). Four non-secret quests are not: 28 Waking to
Gaius (started by `room_enter`), 77 The Truth, 29 Two Roads (retired) and the test quest 1000000.
Accepting hands over an item for quests 6 (Maintenance Report), 10 (Protection Notice), 60 (Sealed
Road-Dispatch), 66 (Spent Wax Wrapper), 42 (Salt Pouch, value 1), 32 (Drillyard Cudgel, 12), 50
(Sling, 40), 39 (Firebomb, 20) and 14 (Oil Lantern, 8). Accepting charges gold only for the secret
pinnacle quests 79 to 87, which phase 7 turns into recipe sales. Quest 34's two offers set the
`34-boon` flag (`might` or `finesse`) on accept.

## Owner rulings (2026-10-28)

1. **An abandoned quest is shelved.** Its progress leaves the active log onto a shelf. While
   shelved, the quest is not held: no log row, no map target, no trigger counts toward it, and its
   giver's `[!]` returns. Re-accepting takes the saved step off the shelf.
2. **A second accept hands over the offer's item only when the player carries none and the item is
   worth nothing** (errand items come back, starter gear does not). An item a quest needs that only
   an NPC hands out should be authored worth nothing.
3. **Abandon needs a way back.** A quest no NPC offers cannot be abandoned.
4. Quests 79 to 87 are out of scope: they are secret (never in the log) and become recipe sales in
   phase 7.
5. **A pick made when accepting may change through abandon and re-accept.** Nothing has paid out
   yet, so a player may take the other offer on resume. Picks made mid-quest cannot change: the
   shelf keeps flags and the step reached, a choice topic is gated on holding the quest (hidden
   while shelved), and each choice moves the quest past itself. Picks at turn-in cannot change:
   a finished quest cannot be abandoned. Scan 2026-10-28 (310 dialogue files, 76 offered quests):
   the only accept-time pick is quest 34 (Garve's `tower_might` / `tower_finesse`, the `34-boon`
   flag paid at the boss kill); the mid-quest picks are quests 11, 18, 67 and 72.
6. **A quest whose `-start` is granted automatically cannot be abandoned.** Any quest trigger
   action or any behaviour tree that grants `N-start` would take a shelved quest N back up
   without the player choosing to, so N is not abandonable even when an NPC offers it.
   Instance: quest 30 (The Awakening), whose `30-start` is granted by `room_enter 5200`
   (`quests/30-the_awakening.yaml`) and by the antechamber room tree
   (`behaviors/rooms/newcomer_antechamber/6467.yaml`), though Hadwen (9100) offers it. Scan
   2026-10-28: the automatic `-start` grants are quests 28 and 30 only (28 has no offer, so the
   rule changes only quest 30).

## 1. The shelf

`Character.ShelvedQuests map[int]string` (`shelvedquests`, omitted when empty): quest id to the
step the player had reached.

- **Abandon** moves `QuestProgress[id]` to `ShelvedQuests[id]` and deletes it from
  `QuestProgress`. Quest flags and count progress ("Roots gathered: 2 of 5") stay where they are.
  Every reader that asks `HasQuest`, `GetQuestProgress` or the step therefore sees the quest as
  not held, with no change: triggers, dialogue gates, the log, the map, markers.
- **Resume.** Any grant of `N-start` while `ShelvedQuests[N]` exists restores the shelved step into
  `QuestProgress`, deletes the shelf entry, and sets `LastQuestId` (focus). This lives in one
  place, `Character.ResumeShelvedQuest(questId) (step string, ok bool)`, called by
  `GiveQuestToken` for a `start` token, so every grant path (dialogue, questengine, behaviour
  tree, overlays) resumes the same way.
- **On resume** the player reads "You take up <quest> again." in place of "You have been given a
  new quest" (both banners: the hook's and `GameBridge.GrantQuest`'s), and markers refresh.
  `quest_granted` triggers for `N-start` still fire: shipped ones are state-gated chains that a
  resume must keep (quests 7, 9, 15: holding the item advances the quest; 78: not yet finished).
  Mid-quest rewards cannot repeat: their triggers are gated on `missing` tokens the player still
  holds.
- **No handout with resale value on a resume, from any source** (ruling 2 applied everywhere).
  While a grant chain runs for a resumed quest, `GameBridge` is marked resumed: `give_item`
  gives only an item worth nothing that the player does not carry, and `give_gold` gives
  nothing. Instance: quest 50's `quest_granted` trigger gives a Pouch of Shot (value 40)
  whenever `50-start` is granted to a player without one, at any step; on a resume it does not.
  The offer's own effects follow the same rule (section 3).
- A shelved quest that is finished another way cannot happen: no trigger counts for it.

## 2. Abandoning

`quests abandon <name>` (and `quest abandon`, `q abandon`), the web panel's right-click Abandon,
and GMCP `Char.Quests.Abandon {id}`; all three call one function,
`usercommands.AbandonQuest(user, questId) (ok bool, line string)`.

Refused, with the line shown to the player:

| Case | Line |
|---|---|
| no active quest matches | `You have no quest called that.` |
| no full match, and the name fits more than one quest in part (typed command only) | `More than one quest matches that. Type more of its name.` |
| secret quest | (never matched: secret quests are not listed) |
| no NPC offers it (ruling 3) | `Nobody would take this back from you. It has to be finished.` |
| a trigger or behaviour tree grants its `-start` (ruling 6) | `Nobody would take this back from you. It has to be finished.` |

On success: `You set <quest> aside. Talk to <giver> to take it up again. You will pick up where
you left off.` No confirmation prompt: abandoning loses nothing. If it was the focused quest,
`LastQuestId` is cleared and `GetFocusQuestId` picks another. `Char.Quests` is re-sent and
markers refresh.

**Who offers it.** Dialogue files load lazily per NPC, so an offer index is built at boot from
`dialogue.LoadAllFiles()` (the scan the quest loop door already makes): quest id to the offering
NPCs (every topic whose `grantsQuest` is that quest's `-start`), with the first NPC's name for
the lines above. A quest is abandonable when the index has it. `dialogue.SaveDialogueFile`
updates the index for the file it writes, so a builder save keeps it current.

**The offer must be able to show again.** A quest is abandonable only when some offering topic's
`questExcluded` tokens and `questFlagExcluded` keys name no other quest (its own quest's are
fine: a shelved quest is not held). An offer hidden by another quest's token hides for good once
the player gains that token, and a quest shelved behind it would be lost. Instance: quest 88's
only offer (`pothole_coulee/9100.yaml` `road_toke`) excludes `31-start` and `29-end`, so 88 cannot
be abandoned. The boot door logs such quests as information (13, 88 and 89 on 2026-10-28).

**No automatic start (ruling 6).** The same boot door computes `questloop.AutoStarts` over the
quest loop world (`questworld.RegistryWith`, the index every loop rule reads): every quest whose
`-start` a quest trigger's `grant` (a sequence's `on_complete` included) or a behaviour tree's
`grant_quest` gives. Offers, `rewards.questid` and item questtokens do not count. It hands the
set to `usercommands.SetAutoStarted`, and `Abandonable` refuses those quests. The set lives in
`usercommands`, not the dialogue offer index, because it is abandon policy: `dialogue.Resumable`
keeps meaning "an offer can show again". The boot Info line `offered but not abandonable` names
both reasons: `an offer hides on another quest` and `a trigger or behaviour tree grants its
start` (30 on 2026-10-28).

Name matching: `util.FindMatchIn` over the sorted names of the player's active non-secret quests,
exact match first, then a close match only when it is the one name that fits in part. The web client sends the quest id over GMCP.

## 3. Re-accepting and the offer's effects

`PlayerState` gains `Shelved func(questId int) bool`. In `DialogueFile.Apply`, when the topic's
`grantsQuest` is `N-start` and `Shelved(N)`:

- `chargesGold` is not taken, and `givesGold` is not paid (no handout with resale value).
- `givesItem` is given only when `CarriesItem(item)` is false and the item's `Value` is 0
  (ruling 2). Otherwise it is skipped and counts as delivered.
- `setsQuestFlag`, `bumpsRep`, `moodChange` apply as usual. A player who resumes quest 34
  through the other offer changes their boon before earning it (ruling 5).

## 4. Focus

`quests focus <name>`, the right-click Focus, the existing left click and `Char.Quests.Focus`;
all call `usercommands.FocusQuest(user, questId) bool`, which holds the checks the GMCP handler
has today (active, not secret), sets `LastQuestId`, and re-sends `Char.Quests`. Refusal line: `You
have no quest called that.`

## 5. The quest log

- `quests` (active): unchanged; shelved quests do not show.
- `quests all`: shelved quests appear after the active ones with the line `Set aside. Talk to
  <giver> to take it up again.` in place of the step description, and no bar.
- `help quests` gains `quests focus <name>` and `quests abandon <name>`.

## 6. GMCP and the web client

- `GMCPCharModule_Payload_Quest` gains `abandonable bool` (`abandonable`, omitted when false):
  true for an active, non-secret quest an NPC offers.
- New handler `Char.Quests.Abandon {id}` next to `Char.Quests.Focus`.
- Quest panel: right-click on a `.qrow` opens a menu built like the inventory menu: **Focus** and
  **Abandon**. Abandon is disabled when `abandonable` is false, with the tooltip `Nobody would
  take this back from you. It has to be finished.` Finished repeatable rows (status ready or
  waiting) get no menu, as they cannot be focused today.
- Colour never carries meaning alone (owner is green/blue colourblind): the disabled item is
  dimmed and keeps its tooltip.

## 7. Testing

- `ShelvedQuests`: abandon moves the step; `HasQuest` false while shelved; resume restores it and
  clears the shelf; a grant of `N-start` resumes through every path (dialogue, questengine
  `bridge`, behaviour tree, overlay).
- A trigger gated on a shelved quest does not fire; after resume, a trigger gated `missing` on a
  token the player already holds does not fire again (no repeat reward).
- Offer effects on resume: no charge; worthless item given only when missing; valued item not
  given; flag still set. On a first accept, unchanged.
- Trigger handouts on resume: a `quest_granted` `give_item` of a valued item and a `give_gold`
  give nothing; a worthless missing item is given; a chain `grant` still advances (quest 7's
  shape: resumed at `start` while holding the item).
- The `[!]` returns after abandon and leaves after resume.
- Refusals: unknown name, no offering NPC (quest 28 on shipped data), an automatic start
  (ruling 6: a fixture trigger granting `N-start`; quest 30 refused and quest 42 allowed on
  shipped data).
- Focus by name and by click; abandoning the focused quest moves focus.
- `quests all` row for a shelved quest; `help quests` lines.
- GMCP: `abandonable` true and false; `Char.Quests.Abandon` with an id that cannot be abandoned
  changes nothing.
- Web: stub-DOM checks in `tools/webclient-tests/` for the menu items, the disabled state and the
  GMCP sent; a headless Edge screenshot (`tools/webshot/live-shot.js`) of the open menu.
- Save round trip: a character with `shelvedquests` saves and loads; an old save without it loads
  unchanged.
- Every probe shown able to fail before its green counts.
- Playtest: one seat on quest 42 (The Hunt) at `track`: abandon, see the `[!]` on Hunter Delk,
  kill a pronghorn (no progress), re-accept (resumes at `track`, "You take up The Hunt again.", no
  second Salt Pouch while carrying one), kill a pronghorn (advances). A second seat tries to
  abandon quest 28 and reads the refusal, and uses `quests focus`.

## 8. Docs

`context.md` for `characters`, `dialogue`, `conversation`, `hooks`, `usercommands`,
`questengine`, `modules/gmcp`; the roadmap (phase 5 status); the `dogmud-authoring-quests` skill
(ruling 2's authoring rule: an item only an NPC hands out for a quest is worth nothing; abandon
needs an offer; a pick made in the offer can change through abandon, so put picks that must
stick mid-quest, ruling 5); `help quests`; PATCH_NOTES; `docs/README.md` for this spec and the plan.
