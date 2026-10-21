# Quest Arc Phase 0: Menu Primitive (design)

> Epic #375, phase 0. Target: `docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md`. Route:
> `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`. Brainstormed with the owner 2026-10-20.

## Facts verified against source (master `a3b82e5ac`, 2026-10-20)

| Fact | Where |
|---|---|
| `prompt` takes every input line while a question is open, then re-runs the command that opened it | `world.go:977-990` |
| A choice question with options re-asks when nothing matches; matching is closest-prefix | `internal/prompt/prompt.go:127-183` |
| Tab completion offers an open question's options | `world.go:334` |
| One prompt per user, held on `UserRecord.activePrompt` under `sessionMu` | `internal/users/userrecord.go:62,76,784-815` |
| Typed input and engine-queued commands both arrive as `events.Input`; nothing marks which is which | `world.go:957` (input worker), `internal/users/userrecord.go:382-395` (`Command`), `internal/events/eventtypes.go:108` |
| Event flags available: `CmdSkipScripts`, `CmdSecretly`, `CmdIsRequeue`, `CmdBlockInput`, `CmdUnBlockInput`, `CmdFollowStep` | `internal/events/const.go:20-31` |
| Two follow paths move a follower by queuing a command: the follow module (`user.Command(exit, .25)`) and party follow (`CommandFlagged(rest, CmdFollowStep)`) | `modules/follow/follow.go:425`, `internal/usercommands/go.go:390` |
| Every room move fires `events.RoomChange{UserId, MobInstanceId, FromRoomId, ToRoomId, Unseen}` | `internal/events/eventtypes.go:210` |
| `numbered-list` renders: 19, all in admin builders (`admin.item.go` 4, `admin.room.go` 8, `admin.zone.go` 3, `admin.server.go` 2, `admin.spell.go` 1, `admin.room.exits.go` 1) | grep |
| Menu-index parses (render, `Atoi`, range check, index, name fallback): 6, at `admin.item.go:201,356`, `admin.room.go:423,592,779`, `admin.zone.go:149` | grep |
| Fixed-option prompts (`prompt.Ask` with a literal option list): 27, 13 in admin builders | grep |
| Other choice menus on `prompt`: `character` (new/view/change/delete/hire/quit, `character.go:64-87`), start's 1/2/3 tutorial choice written into the question text (`start.go:63`) | source |
| Templates: `tables/numbered-list` and `numbered-list-doubled` (two columns, fixed 33-char cells) over `templates.NameDescription{Id, Marked, Name, Description}` | `_datafiles/world/dogmud/templates/tables/`, `internal/templates/name_description.go:4` |
| State machines on `Character`: `CombatPhase`, `Awareness`, `Life`, `Activity`, `Position`, `Control`, `Presence`, `Perception` | `internal/characters/character.go:188-244` |
| Only `presence` and `perception` expose `RegisterObserver(name, func(from, to, reason))`; the others keep `inner *state.Machine` private | `presence.go:169`, `perception.go:104`, `activity.go:88`, `combatphase.go:121`, `life.go:59`, `position.go:115`, `control.go:77` |
| States: combatphase Idle/Engaging/Engaged/Disengaging; activity Free/Casting/Crafting/Salvaging; life Alive/Dead/Respawning; position Standing/Prone/Supine plus 11 grapple positions; control Neutral/Controlling/LosingControl/BecomingControlled/Controlled | `internal/state/*/*.go` |
| Sleeping is a condition flag, `HasConditionFlag(conditions.Sleeping)` | `internal/conditions/conditionspec.go:65`, `internal/characters/resources.go:395` |
| `quit` queues condition 0 with source `quitting` | `internal/usercommands/quit.go:16` |
| Conditions land in `hooks.ApplyConditions` | `internal/hooks/Condition_ApplyConditions.go:27` |
| Refusal lines exist: `WhileBusy`, `WhileFighting`, `WhileUnderAttack`, `TargetAsleepText` | `internal/refusal/busy.go`, `combat.go` |
| `talk` and `ask` are registered as allowed in combat | `internal/usercommands/usercommands.go:59,227` |
| Rounds are 4 s | `_datafiles/config.yaml:193` |
| No shared busy predicate exists; filed as #640 | #640 |
| Ticks and triggers run in the web client, not the server. Ticks are `setInterval` timers that call `SendData` directly | `_datafiles/html/public/webclient-pure.html:1393-1421` |
| Triggers fire immediately (`fireTriggerCommands`, lines 1723, 1809) or go to the action queue (`enqueueTriggerAction`, line 1721) | `webclient-pure.html:1500,1535` |
| The action queue already holds during the login burst (`queueLoginSettled`, released by the first `Commands.State` push or a 2.5 s fallback); ticks and immediate triggers have no such hold | `webclient-pure.html:1526,1561-1571,829-835` |
| `socket.onclose` never clears the tick timers, so after a reconnect they keep sending commands into the name and password prompts | `webclient-pure.html:2582` |
| Automation reaches the client as `Char.Automation`, pushed on `PlayerSpawn` (so only after login) and on `AutomationChanged` | `modules/gmcp/gmcp.Automation.go:32,44`, `internal/events/eventtypes.go:75` |

## Goal

One menu primitive that every fixed choice in the game uses: one look, one way to resolve
input, one set of interrupt rules. Phase 1 builds `talk` on it.

## Owner rulings for this phase (2026-10-20)

1. A standalone menu primitive, not a `prompt` feature.
2. No clicking in the terminal. Menus are typed in every client.
3. Game menus are non-modal: a number, a name or Enter goes to the menu; anything else closes
   it and runs as a command.
4. A game menu cannot open, and an open one closes, while the player is in combat, casting,
   crafting or salvaging, dead or respawning, asleep, controlled, or
   quitting. Any room change closes it. (Owner 2026-10-20: off their feet is not a gate for
   audio interactions.)
5. A leader pulling a follower away closes the follower's menu with three lines (follower,
   leader, room left behind), through the sight rules.
6. Every fixed choice moves onto the primitive, including yes/no confirms.
7. Confirms have two entries; No is the way out and Enter picks No. Confirms for actions that
   cannot be undone name the action ("Yes, disband it").
8. Labels are natural language with no `Ask:` / `Task:` prefix. Markers carry the type.
9. Menu language conventions (below), including speaking in the NPC's manner where it fits.
10. A shared busy check is future work (#640); this phase builds its gate from the same
    sources and moves onto #640 when it lands.
11. Ticks and triggers pause while a menu is open. Anything that would have fired is queued,
    and the queue does not start releasing until the menu closes.
12. Ticks and triggers never fire while the player is logging in.

## Non-goals

- Talk topics, markers' meaning, quest offers: phases 1 and 2. Phase 0 only renders a
  marker column the caller fills.
- Replacing `prompt` for free-text questions (mail body, passwords, names).
- The shared busy check itself (#640).

## Design

### 1. `internal/menu`: the pure core

No session state, no I/O, no imports of `users` or `characters`.

```go
type Entry struct {
    Id     string // caller's key, returned on choice
    Marker string // "", "[!]", "[?]", "[*]"; rendered in a 4-wide column
    Label  string // natural language, one line
    Detail string // optional second line, indented under the label
    Dim    bool   // rendered dim (heard info, small talk)
}

type Menu struct {
    Heading string  // caller's line above the list (an NPC line, a question)
    Entries []Entry
    Leave   string  // label of the final numbered leave entry; "" for none (confirms)
    Footer  string  // defaults to "Type a number, or press Enter to leave."
}

type Result struct {
    Kind  ResultKind // Chosen, Leave, NoSuchNumber, Ambiguous, NotAnAnswer
    Entry *Entry     // set when Kind == Chosen
    Matches []Entry  // set when Kind == Ambiguous
}

func Render(m Menu) string
func Resolve(m Menu, input string) Result
func Confirm(question, yesLabel, noLabel string) Menu
```

**Rendering.** One column at the 80-character wrap:

```
Kael sets down his whittling. "Ask what you like, I'll answer plain."

  1. [!] The Feud at the Well
         Do you want help settling the feud at the well?
  2. [*] How do I reach the high pass?
  3.     What do you carve?
  4.     Leave

Type a number, or press Enter to leave.
```

- The marker column is always 4 wide, blank when there is no marker, so labels align.
- `Detail` wraps under the label at the label's indent.
- `Dim` uses grey brightness, never a green/blue distinction. Colors use existing palette
  aliases; the plan picks them.
- A confirm renders as a two-entry block with no Leave entry:
  ```
  Disband "Iron Wolves"? This cannot be undone.
    1. Yes, disband it
    2. No, keep it
  ```

**Resolving input.**
- Empty input: `Leave` (for a confirm, the No entry, returned as `Chosen`).
- A number in range: `Chosen`. Out of range: `NoSuchNumber`.
- Text: case-insensitive exact label match, else a unique label prefix: `Chosen`. Several
  prefix matches: `Ambiguous`. For confirms, `y`/`yes` and `n`/`no` also match their entry.
- Anything else: `NotAnAnswer`.

### 2. Game menu session (`users` layer)

```go
func (u *UserRecord) OpenMenu(m menu.Menu, s menu.Session) bool // false if gated
func (u *UserRecord) CloseMenu(reason menu.CloseReason)
func (u *UserRecord) ActiveMenu() *menu.Open
```

`menu.Session` carries `OnChoose(entry)`, `OnClose(reason)`, the interrupt rules to apply
(the standard game set by default), and an optional context check (phase 1 uses it for "the
NPC is still here and visible by name").

- One open game menu per player, held beside `activePrompt` under the same `sessionMu`.
  Opening a menu closes any open one (reason: replaced) and refuses while a builder prompt is
  open.
- `OpenMenu` refuses, with the matching `refusal` line, when any interrupt rule holds.
- **Input routing.** In `processInput`, before the prompt check, a line of **typed** input
  goes to the open menu. `Chosen` re-checks every rule and the context check, then closes the
  menu and calls `OnChoose`. `Leave` closes it. `NoSuchNumber` and `Ambiguous` reply and keep
  it open. `NotAnAnswer` closes it (reason: other command) and the line runs as a normal
  command.
- **Typed versus queued input.** A new event flag, set only by the input worker
  (`world.go:957`), marks a line the player typed. Engine-queued commands (`user.Command`,
  follow steps, scripts) never answer a menu or a prompt; they run as commands. This also
  closes an existing hole where a queued follow step could be consumed as a builder prompt's
  answer. The plan verifies every producer of `events.Input` before relying on the flag.

### 3. Interrupts

The standard game rule set, checked at open, on every choice, and pushed by events:

| Rule | Source | Push signal |
|---|---|---|
| In combat | `CombatPhase` not Idle | new `RegisterObserver` on combatphase |
| Casting, crafting, salvaging | `Activity` not Free | new `RegisterObserver` on activity |
| Dead or respawning | `Life` not Alive | new `RegisterObserver` on life |
| Controlled | `Control` Controlled or BecomingControlled | new `RegisterObserver` on control |
| Asleep | `HasConditionFlag(conditions.Sleeping)` | `ApplyConditions` notifies the session when a condition lands |
| Quitting | the `quitting` condition | same |
| Room change | `events.RoomChange` for the user | event listener |

- Off their feet (`Position` not Standing) is not a rule (owner 2026-10-20: off their feet is
  not a gate for audio interactions).
- The five new `RegisterObserver` methods copy `presence.go:169` exactly.
- Observers are registered when a menu opens and are inert when no menu is open, so an idle
  player pays nothing; the plan settles whether to register once per character instead.
- Each close sends one line naming the cause, from `refusal` where a line exists ("You break
  off as the fight begins."). Phase 1 supplies NPC-specific wording.
- Admin builder menus are modal and ignore these rules unless the caller opts in.
- The gate is one internal function in the menu session built from the sources above, marked
  to move onto #640.

### 4. Follow pull

Both follow paths (`modules/follow/follow.go:425`, `internal/usercommands/go.go:390`) call
`CloseMenu` with a follow reason naming the leader **before** queuing the follower's step. The
follow reason sends:

| Reader | Line |
|---|---|
| Follower | `You break off your talk with Kael to follow Brannoc.` |
| Leader | `Mira breaks off a talk with Kael to follow you.` |
| Room left behind | `Mira breaks off a talk with Kael and follows Brannoc.` |

- Names go through the existing sight and anonymize rules for each reader (a follower in the
  dark reads as "someone"; the NPC's name only when the reader sees the NPC by name).
- In phase 0 the "talk with Kael" part comes from the session's subject string; a non-talk
  game menu uses the generic form ("You stop what you were doing to follow Brannoc.").

### 5. Builder session

- A `prompt.Question` gains an optional `*menu.Menu`. When set, the question renders through
  `menu.Render` and its answer resolves through `menu.Resolve`; `Response` holds the chosen
  entry's `Id`.
- Builder flows stay modal: the prompt still takes every typed line, re-asks on
  `NoSuchNumber` / `Ambiguous`, and re-runs its command as today.
- `prompt` keeps free-text questions unchanged.

### 6. Migration (all in this phase)

- The 19 `numbered-list` renders and their 6 menu-index parses move onto `menu` via the
  builder session. The two `numbered-list` templates and the parse blocks are deleted when
  nothing uses them (`NameDescription` stays if other templates use it; the plan checks).
- The 27 fixed-option prompts become menus: player-facing ones (character, guild, mail,
  rename, auctions, start, picklock) on the game session where they are not part of a modal
  flow, on the builder session where they are; the 13 builder ones on the builder session.
- `character`'s option menu and start's 1/2/3 tutorial choice become real menus.
- Confirms adopt rule 7 wording where the action cannot be undone (guild disband, character
  delete, account delete, leadership transfer).

### 7. Automation pause (web client ticks and triggers)

Why it matters beyond comfort: game menus are non-modal, so a tick's command arriving while a
menu is open would close the menu as "another command".

**Server signal.** A new `events.MenuChanged{UserId, Open bool}` fires whenever a game menu or
a builder prompt opens or closes (the last one closing sends `Open: false`). The gmcp module
pushes it as `Char.Menu {"open": true|false}`, following the `AutomationChanged` pattern in
`modules/gmcp/gmcp.Automation.go`. Telnet clients with GMCP can use the same signal in their
own scripts.

**While a menu or prompt is open:**
- A tick that comes due goes to the action queue instead of `SendData` (one entry per tick,
  deduplicated by id like triggers, within the existing cap of 10).
- A trigger that matches, immediate or queued, goes to the action queue with its resolved
  commands.
- `drainQueue` releases nothing.

**When it closes:** the queue resumes at its normal pace (one action per round, released by
`Commands.State`), so a backlog never bursts out at once.

**Queue audit first.** The owner suspects the action queue misbehaves in some cases,
"front" mode especially. Before the pause is built on it, the plan writes JS tests for the
existing queue: front entries ahead of back entries, front-after-front ordering, dedup, the
cap, deferral and staleness, the in-flight timeout, death clearing, and the login hold. Every
defect those tests find is fixed in this phase. The pause then gets its own tests on top.

**Login:**
- Automation is dropped, not paused, during login (owner 2026-10-20: paused would be nicer
  only if it works reliably). There is nothing correct to queue: the client does not know the
  arriving character's ticks and triggers until `Char.Automation` arrives, and after a
  reconnect it still holds the previous session's set.
- `socket.onclose` clears every tick timer, resets `tickTimersSig`, empties the action
  queue and sets `queueLoginSettled = false`.
- Ticks start only after the first `Char.Automation` push of the session (sent on
  `PlayerSpawn`, so only once the character is in the world), and triggers do not fire before
  it either. Nothing fires at the name or password prompt, on first connect or on reconnect.

### 8. Menu language conventions

Recorded in `dogmud-player-copy` and `internal/menu/context.md`:

1. An info label is what the player says, in the player's voice, usually a question. A quest
   entry's label is the quest's title, and its `Detail` line is what the player says: a new
   quest offers help ("Do you want help settling the feud at the well?"), a hand-in reports
   the result ("I've settled the feud at the well.").
2. Never name the NPC in the third person and never use narrator voice in a label or a
   player line.
3. One line each, at most 50 characters for a label and 70 for a `Detail` line
   (a `Detail` starts at column 9 or 10, so 70 always fits in 80).
4. ESL-clear: plain words, no idioms, no em or en dashes, no raw numbers for stats or rewards.
5. Specific, not generic. No two labels in one menu read alike.
6. Leave lines may be in character ("Goodbye.") and default to "Leave".
7. Confirms are two entries; for actions that cannot be undone, the entries name the action.
8. Speak in the NPC's manner where it fits: gruff and plain, educated and pretentious. Labels
   may match the register of whom the player is addressing.
9. Validators enforce the checkable rules: length, no dashes, no duplicate labels in one menu,
   no `Ask:` / `Task:` prefix. (Phase 0 ships the check as a `menu` function; phase 1 wires it
   into the dialogue validator.)

## Tests and guards

Every guard ships with a fixture proving it can fail.

- **Core unit tests:** `Render` (alignment with and without markers, `Detail` wrap at 80, dim,
  confirm layout, empty menu), `Resolve` (number, out of range, exact name, unique prefix,
  ambiguous prefix, Enter, confirm y/n, not an answer).
- **Session tests:** each interrupt rule refuses `OpenMenu`; each push signal closes an open
  menu with its line; a choice re-checks rules and the context check; a closed menu never
  calls `OnChoose`; opening replaces; a builder prompt blocks opening.
- **Typed versus queued:** a queued command never answers a menu or a prompt; a typed one does.
- **Follow pull:** both follow paths close the menu before the step, and the three lines read
  correctly for each reader, lit and dark.
- **Migration:** each migrated builder flow and confirm has a test driving it by number and by
  name, and each confirm that defaults to No still does.
- **Guard test:** fails on any `numbered-list` template render, or any menu-index `Atoi`
  parse, outside `internal/menu`.
- **Label lint:** the conventions check rejects a fixture per checkable rule.
- **Queue audit (JS):** the existing queue's behaviors listed in section 7, each with a test,
  front mode in every ordering case; a red test is a defect fixed in this phase.
- **Automation pause (JS tests in `tools/jstest/`, beside `webclient-action-queue.test.js`):**
  a tick due while `Char.Menu` is open is queued, not sent; a trigger match while open is
  queued; nothing drains until the close push; after close the queue releases one action per
  round; a socket close clears every timer and the queue; no tick or trigger sends before the
  first `Char.Automation` of a session.
- **Server signal:** opening and closing a game menu or builder prompt fires `MenuChanged`
  with the right `Open` value, including closes caused by interrupts and follow pulls.

## Docs

- New `internal/menu/context.md`.
- Update `modules/gmcp/context.md` (`Char.Menu`),
  `internal/prompt/context.md`, `internal/users/context.md` (menu session),
  `internal/state/context.md` (the five observer methods), `modules/follow/context.md`.
- Menu-language section in `.claude/skills/dogmud-player-copy`.
- `docs/README.md` lists this spec.

## Exit criteria

- Every fixed choice in the game renders and resolves through `internal/menu`.
- No `numbered-list` render or menu-index `Atoi` parse remains outside it; the guard proves it.
- Game menus close on every interrupt within the event that causes it, and refuse to open
  while one holds.
- Queued commands never answer menus or prompts.
- Web client ticks and triggers queue while a menu or prompt is open, resume paced after it
  closes, and never fire during login.
- Builders behave as before, driven by number or name.
- All tests above pass on the full suite.

## For the plan to settle

- Which existing palette aliases render numbers, markers and dim entries.
- Whether interrupt observers register per open menu or once per character.
- The messaging category for menu output (an existing one unless `internal/messaging`'s
  context requires a new one).
- Every producer of `events.Input`, before the typed flag is relied on.
- Whether `templates.NameDescription` has users beyond the numbered lists.
