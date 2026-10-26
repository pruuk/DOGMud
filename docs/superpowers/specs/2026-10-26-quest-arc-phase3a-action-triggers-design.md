# #375 phase 3a: action triggers, the engine half that existing quests rely on

Issue: #375 (quest and dialogue arc), roadmap `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`
"Phase 3: Action triggers". Phase 3 is split (owner, 2026-10-26): **3a** (this spec) fixes what
existing quests depend on and settles the interaction verbs; **3b** adds new trigger kinds (talk,
use with a target, observe, explore by area). Closes #588. Brainstormed with the owner 2026-10-26.

## Facts verified against source (master `c90679ec7`, 2026-10-26)

| Fact | Where |
|---|---|
| Trigger events allowed by the loader: room_enter, room_gate, item_give, skill_use, mob_death, command, item_gain, quest_granted, room_interact, command_issued | `internal/quests/quests.go:115-120` |
| Trigger filters include `verb`; the engine already matches it (`t.Verb != "" && t.Verb != d.Verb`) | `internal/quests/triggers.go:20`; `internal/questengine/engine.go:200-229` |
| Shipped quest triggers by event: room_interact 84, command 50, mob_death 32, item_gain 22, quest_granted 19, command_issued 14, room_enter 13, room_gate 1; 81 quest files; no trigger sets `verb` | `_datafiles/world/dogmud/quests/*.yaml` |
| `room_enter` is already raised for every move, teleports included, through the RoomChange hook (#619) | `internal/hooks/RoomChange_QuestRoomEnter.go` |
| `room_interact` is raised in the command dispatcher before any command runs, for ANY verb typed with a noun, with `RoomId: room.RoomId` (the instance id, not the template) | `internal/usercommands/usercommands.go:383-394` |
| `command_issued` matches the template room via `rooms.OriginalRoomId` | `internal/usercommands/usercommands.go:537-539` |
| `Character.StoreItem` is the one inventory write; it raises no event; 62 production call sites | `internal/characters/inventory.go:178` |
| `item_gain` comes from `events.ItemOwnership{Gained:true}`, emitted by hand at each acquisition site; helpers `TransferItemToBackpack`, `TransferItemBetweenChars` bundle store and event | `internal/hooks/ItemOwnership_CheckItemQuests.go:40`; `internal/actions/transfer.go:51-109` |
| Acquisition routes that store with no `item_gain`: quest trigger `give_item` (`questengine/bridge.go:168`), quest reward item (`hooks/Quest_HandleQuestUpdate.go:211`) and item grants (`:236`), behaviour-tree `give_item` (`behaviortree/actions_quest.go:120`) and `give_item_multiple` (`:268`), salvage (`actions/salvage.go:423`), instant craft (`actions/craft.go:241`, `usercommands/craft.go:764`), mail (`usercommands/mail.go:30`), shield replacement (`hooks/combat_shared_helpers.go:225`) | grep |
| Two routes raise `item_gain` even when the store fails: the second buy path (`actions/buy.go:757` before `:795`) and steal (`actions/steal.go:335-342`) | audit |
| The quest world counts quest `give_item` and rewards as item sources | `internal/questworld/sources.go:95-105` |
| Cast notifies `command` with `Command: "cast"`, `Noun: spellInfo.SpellId` only after the cast is initiated (validated, conviction spent); a stale comment says progression fires in InitiateCast | `internal/usercommands/skill.cast.go:375-394` |
| `command: cast` is used only by quests 28, 44, 45, all Pothole Coulee teaching quests | quest files |
| The only quest gate check is `questGateRefuses`, called once, from `go` | `internal/usercommands/go.go:57, 220` |
| Flee picks an exit filtering only locked exits | `internal/actions/flee.go:210-219` |
| Search: `actions.Search(actor, SearchOptions{Feature})`; `search <feature>` is the whole room search plus the feature's bauble roll (owner rulings 2026-09-26/27) | `internal/actions/search.go:157`; `internal/actions/search_feature.go:15-36` |
| Search cooldown is a literal `TryCooldown("search", "2 rounds")` | `internal/actions/search.go:188` |
| Hidden nouns: `HiddenNoun{Description, HiddenDescription}`, found by a whole-room search at a literal difficulty 175 | `internal/rooms/rooms.go:69-74`; `internal/actions/search.go:375-389` |
| 11 rooms author `hidden_nouns` | `_datafiles/world/dogmud/rooms/**` |
| `command: search` quest triggers: quests 49 and 58, one each | quest files |
| Shared special-move cooldown: `actions.ClaimSpecialMove` / `SpecialMoveReady`, `Balance.SpecialMoveCooldown` shipped at 4 rounds (rounds are 4 s); used by bash, trip, kick, cast, consider | `internal/actions/special_move_cooldown.go`; `_datafiles/config.yaml:786-792` |
| `look` aliases: `l`, `examine` | `_datafiles/world/dogmud/keywords.yaml:327` |
| `read` is its own command for carried items | `internal/usercommands/usercommands.go:196`; `internal/usercommands/read.go` |
| Dewey (newcomer antechamber) | `_datafiles/world/dogmud/mobs/newcomer_antechamber/9491-dewey.yaml`, dialogue `.../dialogue/newcomer_antechamber/9491.yaml` |
| Help pages `look`, `search` exist | `_datafiles/world/dogmud/templates/help/look.template`, `search.template` |
| Validators read triggers through `triggerSignature` | `internal/questloop/grants.go:99` |

## Owner rulings (2026-10-26)

1. Phase 3 splits into 3a (this) and 3b (talk, use with a target, observe, explore by area).
2. Acquisition goes through one seam with a reason; the event fires only when the item lands.
3. Casting stays as it is: the trigger fires on a real cast attempt keyed by the canonical spell id. An effect-based step ("mend the scout") waits for 3b.
4. Quest gates apply to every move that walks through an exit (go, flee) through one shared check, and to nothing else (teleports, system moves, the ferry are exempt).
5. **Look and search are the only interaction verbs.** look, l, examine, exam and read are the look family; search is the other. Every other verb typed at a room feature triggers nothing.
6. **Targeted search finds only its own feature's secrets** (overturns the 2026-09-26 ruling that a targeted search is a whole-room search plus a feature roll). Plain `search` searches the whole room at a higher difficulty and can uncover the same secret, with text that reads as a thorough search.
7. **Every feature that hides something hints at it in its look text**, including each step of a chain. Features with nothing hidden do not hint.
8. **Search claims the shared special-move cooldown** (with spells, bash, trip, kick): a deliberate tradeoff.
9. Teaching look and search (Dewey's antechamber, Pothole Coulee) and the help cross-linking are part of 3a, not deferred to phase 7.

## Design

### 1. One acquire seam

`actions.AcquireItem(user *users.UserRecord, itm items.Item, reason AcquireReason) bool` stores the
item (`StoreItem`) and, only when the store succeeds, queues `events.ItemOwnership{UserId, Item,
Gained: true, Reason}`. Returns whether it landed. `AcquireReason` is a string type with constants
named for routes: get, buy, loot, give, reward, quest_give, tree_give, craft, salvage, forage,
harvest, steal, search_find, storage, vault, auction, mail, topic_give, shield_swap, and so on. The
reason travels on the event (a new field on `ItemOwnership`) and into `EventDetails` for later
filters; 3a adds no trigger filter on it.

- Every acquisition route calls the seam, including the ones that miss today. The transfer helpers
  route through it.
- The second buy path and steal stop raising the event when the store fails.
- **Guard** (`internal/guards`): production code may call `Character.StoreItem` only from the
  seam and from an allowlist of non-acquisition moves (unequip, displaced gear, moves between a
  player's own containers), each entry with its reason. A new acquisition route cannot silently
  skip the event.
- `questworld` item sources are checked against the routes that now raise the event, so the
  walkthrough never counts a source that cannot fire.

### 2. Casting

No behaviour change. The stale comment at `skill.cast.go:378` is corrected.

### 3. Walking gates (closes #588)

The gate check becomes one function used by `go` and by flee: "may this player walk this exit".
`fleeExit` skips exits it refuses, as it skips locked ones; with every exit refused, flee fails the
way it fails with every exit locked. Teleports, system moves and the ferry are untouched.

### 4. `room_interact`: two verb families

- The dispatcher raises `room_interact` only for the look family (look, l, examine, exam, read)
  and for search. `exam` becomes a look alias. `read` keeps its job for carried items and falls
  back to the room feature when the player holds nothing by that name.
- `EventDetails.Verb` carries `look` for the whole look family and `search` for search. A trigger's
  `verb` is empty (means look), `look`, or `search`; the loader rejects anything else.
- A `verb: search` trigger fires on `search <noun>` naming its feature, after the search runs, with
  no roll (a teaching step never stalls on dice; ruling 3's "attempting is enough").
- Plain `search` can uncover the same quest secret (ruling 6): for each `verb: search` trigger
  whose feature is in the room, it rolls at `SearchRoomHiddenDifficulty`, and on a success fires
  the trigger with that feature as the noun and prints the thorough-search line. The engine lists
  a room's search-trigger features so search need not read quest files.
- The event matches the template room (`rooms.OriginalRoomId`), like every other trigger, so it
  works in instanced rooms.
- Any other verb typed with a word that names a room feature, when no command handles it, gets a
  nudge: "Try look <noun> or search <noun>." Speech and meta commands (say, tell, whisper, emote,
  help and the like) run as normal and trigger nothing.

### 5. Search rework

- **Plain `search`**: searches the whole room as today (secret exits, hidden containers, stashes,
  hiders, hidden nouns). Hidden nouns roll at `SearchRoomHiddenDifficulty` (config.yaml, 175).
  A hidden noun found this way prints a thorough-search line naming where it was ("You work through
  the room inch by inch. Tucked behind the <feature>, you find ...").
- **`search <feature>`**: searches only that feature: the hidden nouns tucked into it roll at
  `SearchFeatureHiddenDifficulty` (config.yaml, 125), plus the feature's own bauble roll. It no
  longer finds secret exits, containers, stashes or hiders elsewhere in the room.
- `HiddenNoun` gains `on: <visible noun key>`, the feature it is tucked into. A guard requires every
  hidden noun to name an existing room noun (or a discovered hidden noun, for chains).
- Both searches claim the shared special-move cooldown (`ClaimSpecialMove`); the `"search", "2
  rounds"` literal is deleted. A search refused for cooldown sends no quest event, as today.
- The ruling 2026-09-26/27 text in `search_feature.go` is replaced by the new rule.

### 6. Content

- The 11 rooms with hidden nouns get `on:` and, per ruling 7, a hint in the parent feature's look
  text. Chains hint at each step.
- Quests 49 and 58 (`command: search`) move to `room_interact` with `verb: search` on the feature
  where their item hides, if that reads naturally; otherwise they keep `command: search`.
- The 84 look-style `room_interact` triggers need no change (empty verb means look). Any whose room
  prose taught a different verb ("pry", "lift") is reworded to look or search.
- **Teaching:** Dewey's antechamber teaches looking at a feature; a Pothole Coulee room teaches
  searching a feature that hides something (with a hinting look text), and the plain-search
  alternative. Copy follows `dogmud-player-copy` and assumes no MUD experience.
- **Help:** `help look` and `help search` carry the lesson (look closely at what you see; search a
  thing that looks disturbed, or the whole room at worse odds and the cost of a special move) and
  cross-link each other. Pages about room features, hidden things, quests, cooldowns and the
  new-player help link to both.

### 7. Validators and walkthrough

- The loader validates `verb` values. `triggerSignature` includes `verb`.
- The walkthrough's `fireTrigger` sends the trigger's verb (empty becomes look).
- A guard checks every `verb: search` trigger's noun names a feature (room noun or hidden noun) in
  its room.

## Testing

- Acquire: each formerly missing route raises `item_gain` exactly once; a refused store raises
  nothing; the `StoreItem` guard fails on a planted call outside the allowlist (proved red first).
- Gates: flee never picks a gated exit; all gated means flee fails; a teleport into a gated room is
  unaffected; `go` behaviour unchanged.
- `room_interact`: look, l, examine, exam, read fire a look trigger; `say <noun>` and other verbs
  do not; `search <noun>` fires only a search trigger; a trigger in an instanced room matches; the
  nudge appears for an unknown verb at a feature.
- Search: targeted search finds a tucked hidden noun at the lower difficulty and nothing elsewhere;
  plain search finds it at the higher difficulty with the thorough line; both claim the shared
  cooldown and are refused while a special move cools down.
- Data guards: hidden noun `on:` targets, search-trigger nouns, verb values.
- Walkthrough and questloop stay green over the shipped quests.
- Adversarial playtest: a fresh character in Dewey's antechamber and Pothole Coulee learns look and
  search from the content alone (naive-newbie method, no coaching).

## Out of scope (3b)

Talk (topic heard) triggers, use with a target, observe, explore by area, effect-based spell steps,
choice menus offered on look.
