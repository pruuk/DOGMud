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
  web alike) and closes it on return (`defer`), next to the existing
  `InputDone` defer.
- `InputDone` is now queued for every typed line, telnet included (`Seq` 0).
  The gmcp listener already ignores connections that are not web clients, so
  nothing new is sent on telnet; the pager listens for it as the end of the
  reply.

Text a command causes later, from another event's listener (a mob's
queued `emote`, a delayed re-run from `go`), is outside the window, so it is
not marked and goes out at once, as it does today. The test in section 7
checks that `talk` to Toke is fully marked.

### 2. The pager (`internal/pager`, new package)

Pure, like `internal/menu`: no I/O, no locks.

- `Cut(lines []string, size int) [][]string`: pages of at most `size` rows.
  A line counts as one row (text is already wrapped to the reader's width
  by `RenderForRecipient` or the room sender). A trailing empty line is not
  carried to the top of the next page.
- `StopLine(page, of int) string`: "More (page 1 of 3): press Enter for the
  next page, or type q to stop." Plain words, no symbols, so a screen reader
  reads it as it is.
- `Size(reported int, setting int, screenReader bool) (int, bool)`: the page
  size and whether paging is on. Off when the setting says off. Otherwise
  the setting when set, else 20 for a screen reader account, else the
  reported height minus 2 (stop line and prompt), never below a floor of 10.

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
- `set pagesize <n>` (10 to 200) and `set pagesize off`, saved on the
  account as `UserRecord.PageSize` (`yaml:"pagesize,omitempty"`; 0 = follow
  the screen, -1 = off), following `set linewidth`. `set pagesize` alone
  says the current size. Help file updated.

### 6. Screen readers

- A screen reader account pages at 20 lines unless `set pagesize` says
  otherwise. Twenty is long enough for a menu and its NPC's words, short
  enough to reach the choices without a long wait.
- The stop line is a prompt (no line break), so the web client's log reads
  it once and the answer region ends at `Char.Input.Done` as it does now.
  Telnet screen readers (Mudlet, MUSHclient) read it as any prompt.

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
- `talk` to Toke under test with a page size of 10: every line of his reply
  and menu is marked, and nothing of it arrives before page one.
- The `InputDone` guard test is updated for the telnet queueing; a guard
  keeps `AddToQueue` the only place `Reply` is set.
- Playtest: Toke at a small page size, on telnet (33333 inside the
  container) and in the web client, including a fight line arriving at a
  stop.

## Decisions taken without asking

- **Marking in `AddToQueue`, not in each sender.** One place covers the 14
  constructors and every future one. If wrong: a sender-side flag, which
  would have to be threaded through `rooms.go`'s fan-out.
- **Lines a command causes through a later event are not paged.** They go
  out at once, before the held reply, which can put them above it. If wrong:
  the window could stay open until `InputDone` pops instead of closing when
  `processInput` returns, at the risk of catching unrelated text queued in
  the same pass.
- **Screen reader default of 20 lines.** The original #842 note named 20 as
  xterm's announce limit; that limit is gone, and 20 is kept as a listening
  length. If wrong: one constant.
- **Floor of 10 lines, minus 2 for stop line and prompt.** A tiny window
  still shows a usable page. If wrong: constants.
- **Rows counted after server wrapping.** A telnet client narrower than the
  wrap width wraps again and shows more rows than counted. If wrong: count
  against the NAWS width as well.
- **A reply that fits is sent as one write.** Today each message is its own
  write; joining them changes nothing on screen. If wrong: send them one by
  one.
- **Stop wording** "More (page 1 of 3): press Enter for the next page, or
  type q to stop." If wrong: one string.
- **`q` stops the paging even though it is the short alias for `quests`**
  (`_datafiles/world/dogmud/keywords.yaml:345`). Only at a stop: there,
  `quests` in full still drops the rest and shows the list. If wrong: stop
  on a different word, or let `q` drop the rest and show the quest list
  (the "anything else" rule), with no stop word at all.
- **No back or jump commands** (CircleMUD's `b`, `r`, page numbers): ruling
  3 chose the short set.

## Out of scope

- Paging text that was never a reply (a long broadcast, combat).
- A scrolling panel in the web client.
- The news feed command itself.
- `admin.locate`'s all-at-once `[Page 1/3]` tables; they now page through
  the pager like any long reply, and their own titles can go in a later
  cleanup.
