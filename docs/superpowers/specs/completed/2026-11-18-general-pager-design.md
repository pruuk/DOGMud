# General pager for long replies (#842)

Status: design, awaiting owner review. Part of the touch arc (#783), raised
there by the owner on 2026-11-17.

## The problem

A reply longer than the reader's screen scrolls its start out of view.
Talking to Crier Toke (mob 9106) is the standing example: his opening words,
his hints and a menu of up to 35 topics arrive as one burst. On a phone the
burst pushes the prompt and the start of the reply off screen. A screen
reader user hears all of it before reaching the topic they want. Patch notes
as an in-game news feed (planned, owner 2026-11-18) would be longer still.

The pager is general: it sits on the one path every line takes to a player,
so any reply longer than a page is paged, with no change to the command that
printed it.

## Facts verified against source (master `180203fc5`, 2026-11-18)

| Fact | Where |
|---|---|
| Every line to one player goes out in `Message_SendMessage`: `templates.AnsiParse`, screen reader stripping, `connections.SendTo`, then `RedrawPrompt` at priority 100 | `internal/hooks/Message_SendMessages.go:19-45` |
| `UserRecord.SendText` renders to `WrapWidth()` and queues `events.Message{UserId, Text}`; `sendTextAt` takes a priority | `internal/users/userrecord.go:523-546` |
| 14 non-test sites build `events.Message{`: 8 in `rooms.go` (per-recipient room text), 2 in `userrecord.go`, `print.go`, `admin.bauble.go`, and the hook's own listener registration and comment | grep `events.Message{` |
| Every event goes through `events.AddToQueue(e, priority...)` under `qLock` | `internal/events/events.go:90` |
| `processInput` handles one input line; `typed := flags.Has(events.CmdTyped)`; `InputDone` is queued only when `typed && seq > 0` | `world.go:995-1012` |
| `Seq` is the web client's line number and is 0 for telnet | `world.go:41-42` |
| `InputDone` is queued at `InputDonePriority` 99: after every event that writes a command's text or opens a menu (highest 50), before the prompt redraw (100). A guard test fails if anything else queues at or above 99 | `internal/events/eventtypes.go:102-106`, `world_input_done_guard_test.go` |
| `InputDone`'s only listener sends `Char.Input.Done` and returns early for a connection that is not a web client with GMCP | `modules/gmcp/gmcp.InputDone.go:19-38` |
| A typed line goes to an open game menu first (`RouteMenuInput`), before prompts and commands | `world.go:1014-1016` |
| `talk` runs `conversation.Talk` synchronously. The NPC's words go through `npcSay` → `actions.Say` (room text, queued per recipient); hints go through `user.SendText`; the menu through `OpenMenu` → `SendText` or `sendTextAt(…, 50)` | `internal/conversation/talk.go:28-57,444-480`, `internal/users/userrecord.menu.go:105-135` |
| Crier Toke's dialogue has 35 topics: 9 always shown, 26 gated by quest state | `_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml` |
| Of 314 dialogue files, 282 have under 10 topics, 26 have 10 to 15, 6 have 16 or more (all in Pothole Coulee) | count of `topics:` entries |
| Telnet height is parsed from NAWS into `ClientSettings.Display.ScreenHeight`; 0 reads as `DefaultScreenHeight` 24 | `internal/inputhandlers/term_iac.go:210`, `term_ansi.go:205`, `internal/connections/clientsettings.go:7,52-56` |
| The web client reports `Client.Display {cols, screenreader}` only; the server stores cols through `users.StoreReportedCols` (clamped) on websocket connections only | `_datafiles/html/public/webclient-pure.html:562`, `modules/gmcp/gmcp.ClientDisplay.go:40-62`, `internal/users/wrapwidth.go:38` |
| `set linewidth <n>` saves `UserRecord.LineWidth`, range-checks, fires `UserSettingChanged` | `internal/usercommands/set.go:60,550-569` |
| `RedrawPrompt_SendRedraw` draws `user.GetCommandPrompt()`, deduplicated when a prompt question is open | `internal/hooks/RedrawPrompt_SendRedraw.go:12-45` |
| Screen reader web client: text answering the player's line goes to an assertive log until `Char.Input.Done` for that line; a line without a line break is a prompt, announced only when it changes | spec `2026-11-17-screen-reader-task0-design.md` sections 11, 12 |
| Added after the phone playtest (branch `feat/842-pager`, 2026-11-18): | |
| A web client's columns are stored clamped to 30..80 in `Display.ScreenWidth`; a telnet client's NAWS width lands in the same field and sets `ClientSettings.NAWSReported` | `internal/users/wrapwidth.go:39`, `internal/connections/clientsettings.go:32` |
| `util.VisibleWidth` measures with `go-runewidth` but strips `<ansi>` tags only; a held reply is already parsed to escape sequences | `internal/util/util.go:232`, `internal/hooks/Message_SendMessages.go` |
| The last login answer (the kick question included) is taken by `LoginPromptHandler`; `main.go` then calls `SendEnterWorld` and reads the next frame, so that answer never reaches `processInput` | `main.go:959-1011` (telnet), `main.go:1533-1585` (websocket) |
| `enterWorld` runs on MainWorker between event passes and queues `PlayerSpawn`; its `HandleJoin` queues the login commands (`motd` among them) as untyped `Input` at ready turn 0 and the first `look` (`CmdSecretly`) | `world.go:281`, `internal/hooks/PlayerSpawn_HandleJoin.go:305-340` |
| Event passes run every millisecond; a typed line is queued by `InputWorker` from another goroutine | `world.go:885`, `world.go:969-987` |

## Owner rulings (2026-11-18, this brainstorm)

1. **One general pager, not per command.** Any long reply pages, including
   future commands such as a news feed.
2. **Only the reply to the player's own line is held.** Combat, chat, room
   events and anything else that arrives on its own are never held behind a
   page stop, so a player reading never falls behind a fight.
3. **At a stop, Enter continues and anything else runs.** Enter shows the
   next page; `q` drops the rest; a menu number goes to the open menu even
   if its entry is on an unseen page; any other line drops the rest and runs
   as a command. Once the last page is shown, Enter has its menu meaning
   again.
4. **Page size is the reader's screen height, with a setting on top.**
   `set pagesize <n>` overrides it and `set pagesize off` turns paging off.
   A screen reader account defaults to a fixed size instead of the window.
5. **No paging in combat** (owner, on the written spec). While the player is
   in combat, every reply is sent whole.
6. **Pages count screen rows** (owner, after the phone playtest 2026-11-18):
   a page is measured in rows at the reader's width, so a line the client
   wraps counts for each row it fills, and the stop line's own rows are
   reserved. (The owner did not object to fixing this in the branch.)
7. **On a phone, pages follow the window with the keyboard down** (owner,
   option C of three): the web client reports its rows with the on-screen
   keyboard closed; the player closes the keyboard to read a long reply.
   Contextual buttons later in the touch arc will ease this.
8. **The login burst is never paged** (owner): the message of the day and
   the first room description come out whole.

## Design

### 1. Marking the reply (`internal/events`, `world.go`)

- `events.Message` gains `Reply bool`.
- `internal/events` keeps a reply window: the set of user ids whose typed
  line `processInput` is handling now. `events.OpenReply(userId)` and
  `events.CloseReply(userId)` change it under `qLock`.
- `AddToQueue` sets `Reply = true` on a `Message` whose `UserId` is in the
  window. This is the one place every message is queued, so the room text of
  an NPC's `say` (`rooms.go`), `SendText`, `print` and any future sender are
  all covered without touching them. A `Message{RoomId}` (shouts from next
  door, the AI companion's `ask`) is never marked: it is not addressed to
  one player.
- `processInput` opens the window for a typed line (`CmdTyped`, telnet and
  web alike), next to the existing `InputDone` defer. The window closes when
  that line's `InputDone` pops (the pager's listener, section 3). The window
  is a count per user, so two typed lines in one pass each open and close
  their own.
- `processInput` itself runs inside an event listener (`HandleInputEvents`,
  `world.go:143`), so the line, everything its events cascade into at
  priority below 99 (the `Quest` grant lines, `TalkMenuAgain`'s re-shown
  menu after an accepted offer), and its `InputDone` all happen in one
  event pass. Keeping the window open until `InputDone` covers that cascade.
- `InputDone` is now queued for every typed line, telnet included (`Seq` 0).
  The gmcp listener already ignores connections that are not web clients; it
  now also ignores `Seq` 0, because a web client's macro lines are re-queued
  as typed lines with no number (`world.go:1054-1059`) and must not send
  `Char.Input.Done {seq: 0}`. The pager listens for it as the end of the
  reply.

- **The login burst (ruling 8).** `enterWorld` calls
  `events.EnteringWorld(userId)`. Until the event pass that runs that
  player's `PlayerSpawn`, login commands and first look ends, `AddToQueue`
  marks nothing for them, even inside an open window. The path that paged it
  in the playtest: the last login answer never reaches `processInput`
  (facts table), so the burst is marked only when a typed line from the same
  player is handled in that pass ahead of the login commands. Such a line
  reaches the queue after `PlayerSpawn` but before `HandleJoin` queues the
  login commands (they are queued only when `PlayerSpawn` pops), so its
  window is open while they run and its `InputDone` (99) pops after them.
  `ProcessEvents` forgets every entering player as the pass ends.

Text a command causes in a later pass (a mob's queued `emote`, a delayed
re-run from `go`, a macro's later lines) is outside the window, so it is not
marked and goes out at once, as it does today. Section 7 says where each is
checked.

### 2. The pager (`internal/pager`, new package)

Pure, like `internal/menu`: no I/O, no locks.

- `Rows(line string, width int) int` (ruling 6): the screen rows a line
  fills at the reader's width: its visible width (escape sequences removed,
  a wide character two columns, measured with `go-runewidth`) divided by the
  width, rounded up, at least 1, so a blank line is a row.
- `Cut(lines []string, size, width int) [][]string`: pages of at most `size`
  screen rows, each line counting its `Rows`. A line taller than a page gets
  a page of its own. A trailing empty line is not carried to the top of the
  next page.
- `StopLine(page, of int) string`: "More (page 1 of 3): press Enter for the
  next page, or type q to stop." Plain words, no symbols, so a screen reader
  reads it as it is.
- `Size(reported, width, setting int, screenReader bool) (int, bool)`: the
  page size in rows and whether paging is on. Off when the setting says off.
  Otherwise the setting when set, else 20 for a screen reader account, else
  the reported height less the rows the stop line fills at that width, less
  1 for the prompt (16 rows at 36 columns: 16 - 2 - 1 = 13). Clamped to
  `MinSize` 10 .. `MaxSize` 200.
- `Pages(lines, width, reported, setting, screenReader) [][]string`: `Size`
  and `Cut` together. The stop line names the page count, so when a reply's
  stop line ("page 10 of 12") fills more rows than "page 1 of 1", the lines
  are cut again with those rows reserved.
- The reader's width (`UserRecord.PagerWidth`): a web client's reported
  columns; a telnet client's NAWS width when it sent one, else `WrapWidth`.

### 3. Holding and sending (`internal/hooks`, `internal/users`)

- `UserRecord` gains unsaved pager state: the lines collected for the reply
  being built and the pages still to show.
- `Message_SendMessage`: a `Reply` message for this user is parsed and
  stripped exactly as now, split into lines and appended to the collected
  reply instead of sent. Any other message is sent at once, as now.
- A new `InputDone` listener (`Pager_ReplyEnd`): if the collected reply fits
  on one page, it is sent as one write, unchanged from today's output. If
  not, the first page is sent and the rest kept; the prompt redraw that
  follows draws the stop line (below).
- `GetCommandPrompt` returns the stop line while pages remain, so every
  redraw, including the one after a combat line that arrives mid-read,
  puts the stop line back under the text.
- A new reply replaces any pages still waiting.
- A reply that fits on one page after blank lines at page edges are dropped is sent whole, so there is never a "page 1 of 1".
- **Combat (ruling 5).** "In combat" is `Character.IsInCombat()`, the test
  `menuGate` uses (`internal/users/menu_gate.go:23`), so menus and the pager
  cannot disagree. A reply that ends while the player is in combat is sent
  whole. If a fight starts while pages are waiting, the next message that
  reaches the player sends the rest at once, ahead of that message, and the
  normal prompt returns.

### 4. Input at a stop (`world.go`, `internal/users`)

`user.RoutePagerInput(line)` runs in `processInput` before
`RouteMenuInput`, only while pages remain:

- empty line: send the next page; consumed.
- `q`: drop the rest, redraw the normal prompt; consumed.
- anything else: drop the rest and fall through, so a number reaches the
  open menu and any other line runs as a command (whose own reply may page
  again).

The next page is sent directly (`connections.SendTo`, then `RedrawPrompt`),
not re-queued as messages, so it is never re-marked or re-collected.

### 5. Page size (`modules/gmcp`, web client, `internal/usercommands`)

- The web client adds `rows` (xterm's row count) to every `Client.Display`
  report. `clientDisplayHandler` stores it in `Display.ScreenHeight` for
  websocket connections, as it stores cols, through a new
  `users.StoreReportedRows` with a clamp (10 to 200).
- Telnet keeps its NAWS height. A telnet client that reports nothing gets
  24.
- Pages are counted in screen rows at the reader's width (ruling 6, section
  2): the height less the stop line's rows less the prompt's row. On a phone
  the web client reports its rows with the on-screen keyboard closed (ruling
  7), so a page fills the window the player reads with the keyboard down.
- `set pagesize <n>` counts rows too, so a setting and the screen agree on
  what a page holds; the help text's "lines" are the lines on screen.
- `set pagesize <n>` (10 to 200) and `set pagesize off`, saved on the
  account as `UserRecord.PageSize` (`yaml:"pagesize,omitempty"`; 0 = follow
  the screen, -1 = off), following `set linewidth`. `set pagesize auto`
  goes back to following the screen. `set pagesize` alone says the current
  size. Help file updated.

### 6. Screen readers

- A screen reader account pages at 20 rows unless `set pagesize` says
  otherwise. Twenty is long enough for a menu and its NPC's words, short
  enough to reach the choices without a long wait.
- The stop line is a prompt (no line break), so the web client's log reads
  it once and the answer region ends at `Char.Input.Done` as it does now.
  Telnet screen readers (Mudlet, MUSHclient) read it as any prompt.
- `Char.Input.Done` says `menu: true` while pages wait, so the phone
  keyboard stays up for Enter. The gmcp listener runs last among the
  `InputDone` listeners (`events.Last`), after `Pager_ReplyEnd` has cut the
  reply.

### 7. Testing

- `internal/pager`: page cuts at, below and above the size; a size of 1; an
  exact fit sends no stop line; `Size` for each source and the floor.
- `internal/events`: a `Message` queued inside the window is marked, one
  for another user or a `RoomId` message is not, and the window closes.
- Hook test: a marked reply longer than a page sends page one and holds the
  rest; an unmarked combat line during a stop goes out at once and the stop
  line is redrawn under it; a reply that fits is sent unchanged.
- Routing test: Enter, `q`, a menu number on an unseen page, and another
  command at a stop.
- Combat: a long reply ending in combat is sent whole; pages waiting when a
  fight starts go out in full before the first combat line.
- `talk` with a 20-topic NPC under test: every line of the reply and menu is
  marked (`conversation_pager_mark_test.go`). Accepting an offer (the `Quest`
  lines and the `TalkMenuAgain` menu) is checked in the playtest, through the
  real event chain.
- `TestProcessInput_DefersTheDoneMarker` (`world_input_done_test.go:64`),
  which pins the `typed && seq > 0` condition in source, is updated for the
  telnet queueing; the gmcp handler test gains a `Seq` 0 case. The priority
  guard (`world_input_done_guard_test.go`) is unaffected: nothing new queues
  at or above 99.
- Rows (ruling 6): a 36-column reader's ANSI-coloured 39-column lines fill 2
  rows each; the stop line's rows are reserved; a reply that fits in lines
  but not in rows pages; a telnet reader counts at its NAWS width
  (`internal/pager`, `userrecord_pager_test.go`).
- Login burst (ruling 8): nothing is marked for a player entering the world
  until the pass ends (`internal/events`), and `enterWorld` says so
  (`world_input_done_test.go`).
- Playtest: Toke at a small page size, on telnet (33333 inside the
  container) and in the web client, including a fight line arriving at a
  stop.

## Decisions taken without asking

- **Marking in `AddToQueue`, not in each sender.** One place covers the 14
  constructors and every future one. If wrong: a sender-side flag, which
  would have to be threaded through `rooms.go`'s fan-out.
- **The window stays open until `InputDone` pops** (changed while writing
  the plan: closing it when `processInput` returns would have let the
  re-shown menu after an accepted offer, and the quest-start lines, out
  ahead of the held reply). A listener in the same pass that sends this
  player unrelated text is marked too and pages with the reply. If wrong:
  close the window when `processInput` returns, and accept those lines
  arriving above the reply.
- **Lines a command causes in a later pass are not paged.** They go out at
  once. If wrong: nothing simple; they would need their own marker.
- **Screen reader default of 20 lines.** The original #842 note named 20 as
  xterm's announce limit; that limit is gone, and 20 is kept as a listening
  length. If wrong: one constant.
- **Floor of 10 lines, minus 2 for stop line and prompt** (superseded by
  ruling 6). A tiny window still shows a usable page. Now: the height less
  the rows the stop line fills at the reader's width, less 1 for the prompt;
  the floor stays 10 rows, which the math does not reach at the playtest's
  16 rows (13 at 36 columns, 12 at 30). A screen too short for 10 rows plus
  the stop line and the prompt still gets 10-row pages that overflow it. If
  wrong: a separate, lower floor for the
  screen-following size, with `MinSize` kept for `set pagesize`.
- **Rows counted after server wrapping** (superseded by ruling 6). A telnet
  client narrower than the wrap width wrapped again and showed more rows
  than counted, and a web client wrapped menu entries wider than its columns
  (the phone playtest). Now: every line counts its rows at the reader's
  width (`PagerWidth`), telnet at its NAWS width when it sent one.
- **The stop line is measured at its real page count** (`Pages` cuts again
  when "page 10 of 12" fills more rows than "page 1 of 1"). If wrong: measure
  "page 1 of 1" only and accept a page that can overflow by a row.
- **`set pagesize <n>` counts rows**, as the screen does, so the two agree;
  the help text keeps "lines", read as lines on screen. If wrong: count
  logical lines for a setting.
- **The login burst is spared for one event pass** (ruling 8), keyed to
  `enterWorld`, not to the login commands. Every login, reconnect and
  copyover reconnect passes there. A racing typed line's own text in that
  pass is sent at once too. In that race the first `look` waits to the next
  pass (the per-turn input tracker has the racing line). A requeued first
  look cannot run inside another line's window: a typed line sets the
  per-turn input tracker in the same handler that opens its window, so the
  look is requeued again. If wrong: a flag
  carried on the login commands' `Input` events instead.
- **A reply that fits is sent as one write.** Today each message is its own
  write; joining them changes nothing on screen. If wrong: send them one by
  one.
- **Stop wording** "More (page 1 of 3): press Enter for the next page, or
  type q to stop." If wrong: one string.
- **Pages waiting when a fight starts are sent, not dropped.** Ruling 5
  says no paging in combat; sending keeps the text and ends the stop. If
  wrong: drop them, as an open menu is closed by combat.
- **An open prompt question beats the pager** (found in code review). A reply that ends with a question open (a yes or no, a stake amount, a password) is sent whole. If a question opens while pages wait, the question is the prompt instead of the stop line, the typed line answers it, and the pages keep waiting while it is open; an answer that prints anything is a new reply and replaces them, as any new reply does. Without this the stop line hid the question and the next command was taken as its answer. If wrong: drop the waiting pages when a question opens.
- **Connections on the AI port never page** (found in final review). An agent's next command is rarely Enter, so a stop would silently drop the rest of a long reply (help text, room text) for the AI players the port is open to and for the playtest harness. If wrong: page them too and have agents send `set pagesize off`.
- **Screen reader pages are 20 rows, not 20 lines** (ruling 6 measures every page in rows). On the web nothing changes, because the server wraps to the client's columns; on a telnet client narrower than its line width a listener gets fewer lines per page. If wrong: count screen reader pages in lines. *Since #922 (2026-11-19, owner chose all text) a telnet session also wraps to a narrower reported width, so this telnet case matches the web.*
- **Only touch screens keep the tallest row count** (found in review of ruling 7). A desktop reports its real rows on every change, so dragging a window shorter or zooming shrinks pages at once. If wrong: desktops keep the tallest too, and pages overrun until a reconnect.
- **A stop line that wraps leaves its first row behind when another line arrives mid-read** (known, not fixed). The redraw erases only the prompt's last row, as it does for every prompt that wraps today. If wrong: shorten the stop line for narrow screens, or erase every row of a wrapped prompt. *Fixed 2026-11-19 by #922: the connection records the prompt it drew (`connections.SendPrompt`) and the text that replaces it clears every row the prompt fills; any other write, a typed Enter, or a width change falls back to one row.*
- **`q` stops the paging even though it is the short alias for `quests`**
  (`_datafiles/world/dogmud/keywords.yaml:345`). Only at a stop: there,
  `quests` in full still drops the rest and shows the list. If wrong: stop
  on a different word, or let `q` drop the rest and show the quest list
  (the "anything else" rule), with no stop word at all.
- **`set pagesize auto`** returns to following the screen (added while
  writing the plan; the spec had no way back after `set pagesize 30`). If
  wrong: another word.
- **No back or jump commands** (CircleMUD's `b`, `r`, page numbers): ruling
  3 chose the short set.

## Out of scope

- Paging text that was never a reply (a long broadcast, combat).
- A scrolling panel in the web client.
- The news feed command itself.
- `admin.locate`'s all-at-once `[Page 1/3]` tables; they now page through
  the pager like any long reply, and their own titles can go in a later
  cleanup.
