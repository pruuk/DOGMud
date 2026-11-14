# Web client base plan 6: phones (tab height, keyboard, X, sideways toast)

Status: design approved by the owner 2026-11-14 (brainstorm and mock); this file awaits his review.
Parent: `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md` (rulings 12 to 15, design section 6), under the touch epic #783.
Mock: https://claude.ai/artifact/Fb1BNLYYmRhbo3MdULYioW (five iPhone-sized screens: today versus plan 6 with the keyboard down and up, and the sideways toast).

## Facts verified against source

Read 2026-11-14 from master `95e61dddf`. `WP` is `_datafiles/html/public/webclient-pure.html`; `CSS` is `_datafiles/html/public/static/css/dashboard.css`; `JS/` is `_datafiles/html/public/static/js/`.

| Fact | Where |
|---|---|
| The open-tab cap is half the screen less the command box, tab bar, name strip and a 136px keep, never under 80px | `CSS:693` (vh), `:704` (dvh twin in `@supports`), keep at `:690` |
| With the keyboard up the cap is half the visible height, less the command box and 40px, never under 60px; the drawer sits on the command box | `CSS:791-799` |
| The cap rule at `:693`/`:704` is set on phone, column and slim alike | `CSS:698-705` |
| Column puts the drawer over the panel column beside the game; slim puts it over the game column | `CSS:525`, `:564`; column with keyboard `:785` |
| `--cmdbox-h` 64px, `--tabbar-h` 60px (64px on touch), `--tabname-h` 36px | `CSS:683`, `:678`, `:679`, `:769-774` |
| `html.keyboard-open` is set by the page's own visual viewport watch, or by the outer page when framed; the outer page resizes the frame to the visible height, the page on its own shrinks only `body` | `WP:2706-2716`; `_datafiles/html/public/webclient.html:26-38` |
| `ViewportFit.keyboardOpen`: the visible area is more than 120px short of the page | `JS/viewportfit.js:79-81` |
| `ViewportFit.isPhone` (short side under 600) and `isUprightPhone` (orientation API first, then the screen shape; touch only) exist; `markUprightPhone` already reads `window.orientation` for iOS before 16.4 | `JS/viewportfit.js:18-51` |
| After a send, `HotInput.shouldHideKeyboardAfterSend({touchOnly, phoneWidth, sent, inGame, menuOpen, password})` decides, and the send handler calls `textInput.blur()` | `JS/hotinput.js:149-154`; `WP:3227-3232` |
| Its tests | `tools/jstest/hotinput.test.js:275-287` |
| `phoneTouchOnly()` is `(hover: none) and (pointer: coarse)`; `phoneWidth()` is `ViewportFit.isPhone` of the screen | `WP:3299-3307` |
| `menuOpen` follows `Char.Menu` (`m.open === true`), which the server sends when a game menu or prompt opens or closes | `WP:1812`, `:1016`; `modules/gmcp/gmcp.Menu.go:68`, `:87` |
| `talk` opens the NPC topic menu through `conversation.Talk`; `ask` also routes to a talk conversation | `internal/usercommands/talk.go:66`; `ask.go:69-72` |
| Player prompts are also opened by `character`, `deletecharacter`, `guild`, `mail`, `password`, `picklock`, `renameself`, `start` | `StartPrompt` callers in `internal/usercommands/` |
| Chat commands: `say`, `whisper`, `reply`, `chat`, `newbie`, `trade`, `shout`, `emote`, `party`, `guild` | `internal/usercommands/usercommands.go:88-90`, `:122`, `:151`, `:174`, `:203`, `:212`, `:223`, `:261` |
| DOGMud aliases: `.` is say, the backtick is chat (broadcast in the default world), `yell`/`scream`/`holler` are shout, `/w` is whisper | `_datafiles/world/dogmud/keywords.yaml:322`, `:323`, `:346`, `:349` |
| `party` is mostly management; only `party chat` and `party say` talk | `internal/usercommands/party.go:22-113` |
| `guild chat`, `guild say` and `guild gc` talk | `internal/usercommands/guild.go:67` |
| GMCP from the server is parsed in `socket.onmessage` and handed to `handleGMCP` | `WP:2976-3015` |
| The server takes input every turn; turns are 50ms in the shipped config | `_datafiles/config.yaml:189` |
| Every connection line becomes `events.Input` with `CmdTyped`; `processInput` handles it, returning early after a menu answer | `world.go:961-967`, `:973-993` |
| The websocket loop reads every frame with `conn.ReadMessage()` and drops the frame type; GMCP from the client is binary (`SendGMCP`), typed lines are text (`SendData`), as is the copyover token | `main.go:1403`; `WP:3244`, `:2962`, `:3250-3263` |
| `WorldInput{FromId, InputText, ReadyTurn}` is built at three sites (telnet, resumed telnet, websocket) and sent through `World.SendInput` | `world.go:37-41`, `:249`; `main.go:1085`, `:1265`, `:1555` |
| `events.Input{UserId, MobInstanceId, InputText, ReadyTurn, Flags}`; `processInput` is called from two places | `internal/events/eventtypes.go:144-150`; `world.go:141`, `:184` |
| A typed line in a prompt becomes the answer and the prompt's command runs in its place; `password` asks every password through its prompt | `internal/users/userrecord.promptroute.go:12-24`; `internal/usercommands/password.go` |
| Event priority: a higher number is delivered after every lower-numbered line in flight | `internal/users/userrecord.go:504-506`; `internal/events/events.go:91` |
| `Char.Menu` is queued at priority -1, ahead of the menu's text | `modules/gmcp/gmcp.Menu.go:76-90` |
| `isWebClientConnection` and `isGMCPEnabled` gate GMCP by connection | `modules/gmcp/gmcp.go:79-93` |
| The command box is one input in `#panel-cmd .dash-panel-body`, a flex row | `WP:299-303`; `CSS:210` |
| The toast is `Dashboard._toast(msg)`, 2.6s, fixed 26px from the bottom | `JS/dashboard.js:272-282`; `CSS:1759-1768` |

## Rulings

From the layout spec, not reopened: 12 (an open tab is at most 30% of the visible height), 13 (the keyboard stays open after communication and closes after anything else; aliases and macros are out), 14 (a visible X of at least 44px closes the keyboard, shown only while it is open), 15 (a sideways phone gets the toast once per visit).

Owner, 2026-11-14 (memory `feedback-phone-keyboard-plan6-rulings`):

- R1. The keyboard stays up whenever the player's next act is to type again: a message to players and talking to NPCs. Party and guild count only when talking (`party chat|say`, `guild chat|say|gc`). The chat words are data under this principle, not a closed list.
- R2. Menus and prompts: the client waits for the game's answer. After any other command it waits briefly; if the game opens a menu or prompt, the keyboard stays up, otherwise it closes. Reported case: `talk Hadwen` closed the keyboard just before the topic menu arrived.
- R3. The 30% cap applies only where an open tab covers game text: the phone and slim layouts. Column keeps its current cap.
- R4. The mock is approved as drawn (2026-11-14).
- R5. The wait is timed from the game's reply, not from Enter, so a slow connection only delays the close (chosen over a fixed 350ms after Enter).
- R6. The server marks each command done, with whether a menu or prompt is open, so other players' lines cannot be mistaken for the reply (chosen over the client guessing from the first text after Enter). Plan 6 gains this small server piece.
- R7. The marker never carries typed text: lines are numbered at the socket on both ends and the marker carries the number (chosen over echoing ordinary commands only, after the owner flagged that an echo would send an in-game password back).

## What players see

On a phone, an open tab takes at most 30% of the screen, so about 21 lines of game text stay visible on an iPhone with the keyboard down (11 today) and 13 with it up (7 today). Chatting or answering an NPC's menu never drops the keyboard; looking, moving and fighting still close it so the result can be read. A brass X at the end of the command box closes the keyboard by hand. Turning the phone sideways shows one short note suggesting it be held upright.

## Design

### 1. Keyboard after a send (`JS/hotinput.js`, `WP` send handler)

`shouldHideKeyboardAfterSend` becomes `keyboardAfterSend(s)` and returns `'keep'` or `'wait'`. `s` gains `line`, the text exactly as typed. (No command closes the keyboard at once any more: every in-game command that is not chat waits for its marker.)

- Not a touch phone (`touchOnly && phoneWidth`), nothing sent, not in game, a menu already open, or the password prompt: `'keep'`. This is today's behaviour, unchanged.
- The line is a chat line: `'keep'`.
- Anything else: `'wait'`.

`isChatLine(line)` is exported beside it. The first word, lowercased, is matched against `CHAT_WORDS`: `say`, `whisper`, `reply`, `chat`, `newbie`, `trade`, `shout`, `emote`, `yell`, `scream`, `holler`, `/w`, `gc`, `pchat`, `psay`. A line starting `.` or the backtick counts whether or not a space follows (`.hello` is say). `party` and `guild` count when their second word is in `PARTY_TALK` (`chat`, `say`) or `GUILD_TALK` (`chat`, `say`, `gc`). The constants carry a comment naming R1 as the principle. A `talk` or `ask` line is not in the list: it waits, and the menu it opens keeps the keyboard (R2).

The send handler acts on the answer. `'wait'` waits for the server to say it has finished the command (R5, R6, section 1a), so a slow connection only delays the close and other players' lines cannot end the wait:

1. The client numbers its text frames: `linesSent` counts every string it sends on the socket (`SendData`, and the copyover token at `WP:2962`), starting at 0 when a socket opens. A `'wait'` remembers its line's number.
2. Each `Char.Input.Done` carries the number of the line it answers (`seq`), never the line's text (R7). A marker with the waiting number ends the wait with its `menu`. A marker with a higher number means the waiting line never reached the world (an admin system command, a dropped line), so the wait ends at once with `menuOpen`. A lower number (an earlier command's marker) is ignored.
3. The input is then blurred only if all of these hold: the message's `menu` is false, the command box still has focus, and its value is empty (the player has not started the next command).
4. `MENU_WAIT_CEILING_MS` (2000) after the send, step 3 runs anyway with `menuOpen` in place of `menu`, for a line the server never answers (a dropped connection, a line the server discards).

A new `'wait'` replaces the previous one. Typing time never counts: the wait starts at Enter and covers the round trip, the server's input tick (`TurnMs: 50`, `_datafiles/config.yaml:189`) and the command itself.

### 1a. The server marks each command done (`world.go`, `modules/gmcp`)

**Numbering.** The websocket read loop (`HandleWebSocketConnection`, `main.go:1403`) counts text frames from the moment the connection opens, the same frames the client counts; binary frames (the client's GMCP) are not counted. The count rides with the line: `WorldInput.Seq` (`world.go:37`, set at `main.go:1555`), copied by the input worker onto a new `events.Input.Seq` (`world.go:961-967`), and passed to `processInput` by both of its callers (`world.go:141`, `:184`). Telnet lines carry 0, as do lines the game queues itself (macro expansions at `world.go:1024-1029`, `user.Command`).

**The marker.** `processInput` (`world.go:973`) handles every line a player sends. For a typed line with a number, a deferred step at its top queues a new `events.InputDone{UserId, Seq}` at `events.InputDonePriority` (99), so it is delivered after every event that writes the command's text or opens a menu in the same pass (the highest today is 50) and before the prompt redraw (100). A higher priority pops after every lower one in flight (`internal/users/userrecord.go:504-506`). `TestInputDonePriority_NothingElseQueuesAtOrAbove` fails if any other `AddToQueue` or `sendTextAt` call reaches 99. The defer covers every return, including the menu-answer early return at `world.go:985-993`.

`modules/gmcp` gets a listener beside `menuChangedHandler` (`gmcp.Menu.go:76-90`). For a web client connection only (`isWebClientConnection`, `gmcp.go:92`) with GMCP on (`isGMCPEnabled`), it sends `Char.Input.Done` with `{"seq": <n>, "menu": <user.MenuOrPromptOpen()>}`, the menu state read when the event is handled, so it reports the state the command left. No typed text is ever sent back (R7): a password typed into the in-game `password` prompt travels to the server once and nowhere else. Telnet clients get nothing new.

`Char.Menu` itself is unchanged and still sent ahead of the menu's text (priority -1, `gmcp.Menu.go:80-89`); the client keeps using it to queue automations.

Limit: a command whose menu opens later through a queued event (not in the same pass) reports `menu: false`, and the keyboard closes as today. `talk` opens its menu in the same pass (`conversation.Talk`), so the reported case is covered. A server macro (a two-character macro the server expands) is marked when it is expanded, before its commands run in later passes, so a macro that opens a menu closes the keyboard as today.

Ruling 13's aliases-and-macros exclusion stands: the client sees only the typed line, so an alias for `say` is a plain command and waits.

### 2. Tab height (`CSS`)

- Phone and slim: `--drawer-cap: max(80px, 30vh)`, with the `30dvh` twin in the existing `@supports` block. The `--drawer-keep` sum and its comment go: 30% already leaves the top of the screen.
- Phone and slim with the keyboard up: `--drawer-cap: max(60px, calc(0.3 * var(--visible-h, 100dvh)))`. `vh` cannot be used here: inside `/webclient` the frame is resized above the keyboard (`webclient.html:26-38`), so `vh` is the visible height, but the page opened on its own keeps `vh` at the layout height while only `body` is shrunk (`WP:2711-2713`). So both keyboard watches, the outer page's `fitFrameHeight` and the page's own, also set `--visible-h` on the game document's root, in px, from the same `visibleHeight` they already compute.
- Column: the cap rules move under `html.layout-column` unchanged, so a laptop's tab beside the game keeps its size, keyboard up or down.

Amended 2026-11-14 (plan 6 browser check): the keyboard counts as open only while a text field has focus (ViewportFit.keyboardOpen, editableFocused); the Reset layout button no longer widens a phone's page before a layout is applied; the command box input may shrink beside the X.

### 3. The X (`WP` `#panel-cmd`)

A `<button type="button" id="cmd-close-kbd" aria-label="Close keyboard">` after the input, holding an inline stroke X. 48x48px (the command box's height; ruling 14 asks at least 44), the existing brass button look, `display: none` except under `html.keyboard-open`, so the box is full width the rest of the time. `pointerdown` calls `preventDefault()` so the tap does not move focus first, then `click` blurs the command box. It is a real button, reachable by keyboard and named for screen readers.

### 4. The sideways toast (`JS/viewportfit.js`, `JS/dashboard.js`)

`ViewportFit.isSidewaysPhone(screenW, screenH, orientationType, coarse)` is `coarse && isPhone && landscape`, the mirror of `isUprightPhone`, sharing its orientation reading (including the iOS `window.orientation` fallback, moved into a small `orientationType(win)` helper both use). Because it reads the screen, the keyboard cannot trigger it.

The dashboard checks it at load and on `orientationchange` and `resize`. The first time it is true in a browser tab, it shows `_toast("This game is easier to play with your phone held upright.")` and sets `sessionStorage['dogmud.sidewaysToast'] = '1'`. A reload in the same tab does not repeat it; a new tab or visit does. If `sessionStorage` throws, a page variable stops a repeat for that page load.

The toast gets a `.dash-toast.lifted` variant that sits just above the command box (`bottom: calc(var(--cmdbox-h) + var(--tabbar-h) + 12px)`), so on a phone it does not cover the tab bar. It shows for 4s, longer than the reset toast's 2.6s, since it is a sentence; `_toast` gains an optional duration.

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| A 2s ceiling, one named constant | Long enough for a poor mobile link, short enough that a lost answer still closes the keyboard | Raised or lowered in one line after Malia's #834 test on mobile data |
| The message is `Char.Input.Done` `{"seq", "menu"}`, web client only, one per typed in-game line | Named beside `Char.Menu`; telnet GMCP clients have no use for it yet | Sending it to telnet GMCP clients is one condition |
| Both ends count every text frame from the socket's open, login and copyover token included | Counting at the socket, not at the world, means login lines and lines the server swallows cannot make the two counts drift: each end counts the same frames | A future client text frame that is not a line (none today: GMCP is binary) must be counted on both ends or neither |
| A server macro (`=1` style, `world.go:1014-1031`) marks only the line typed, not its expansions | The expansions are queued with no number, so the wait ends once the macro has been expanded, before its commands run | A macro whose command opens a menu closes the keyboard, as today |
| The wait blurs only if the box is focused and empty | A player who already started typing the next command should not lose the keyboard mid-word | None known |
| `ask` waits rather than keeps | Under R2 the game decides: `ask` with a menu keeps the keyboard, `ask` answered in one line closes it | If `ask` should always keep, it joins `CHAT_WORDS` |
| "Once per visit" is once per browser tab (`sessionStorage`) | A reload is not a new visit; a new tab is | `localStorage` would show it once ever |
| The toast is lifted above the command box on phones, and lasts 4s | The existing spot covers the tab bar on a phone; a sentence needs more than 2.6s | The original spot and time are one class and one argument away |
| The X is 48px square | Matches the command box's 48px height; ruling 14's floor is 44 | None |
| Slim's keyboard-up cap is 30% too | R3: slim's tab covers game text | Moves under the column rule |

## Out of scope

The command bar (#779), movement by touch (#781), the panel changes of plan 7 (Map title and bar, Quests tabs, Comms rename, item menus by tap). Tablets keep today's keyboard behaviour (never blurred), as the layout spec and the 11-02 phone keyboard plan set.

## Testing

- `tools/jstest/hotinput.test.js`: `isChatLine` for every chat word and shortcut, `.hello` and the backtick without a space, `party chat hi` and `guild gc hi` keep, `party invite bob` and `guild deposit 5` do not, mixed case. `keyboardAfterSend` returns `'keep'` for each of today's keep cases, `'keep'` for chat, `'wait'` for `look` and `talk hadwen`. The existing cases are kept, renamed to the new function.
- The wait itself as pure steps: `blurAfterWait({menu, focused, value})` is `true` only for no menu, focused, empty; and the wait's matching (its own number's marker ends it with that marker's `menu`, a lower number does not, a higher number ends it with `menuOpen`, the ceiling with no marker, a new wait replacing the old, a new socket clearing it) runs against fake timers.
- Go: only a text frame advances the websocket count; `events.Input` carries `WorldInput.Seq`; a typed line with a number queues exactly one `InputDone` after the command's text, and a line with no number or not typed queues none; `processInput` defers it at its top (source guard, since `processInput` needs a running world). The gmcp listener sends `Char.Input.Done` `{seq, menu}` with `menu: true` while a prompt is open and `false` otherwise, sends nothing to a telnet connection, and its payload has no field holding typed text.
- `tools/jstest/viewportfit.test.js`: `isSidewaysPhone` for an upright and sideways phone, a sideways tablet, a desktop, and an iOS device with no orientation API.
- A dashboard test for the toast's once-per-tab rule with a stub `sessionStorage`, and with one that throws.
- Headless Playwright check (plan 5's script, Playwright from `workspace/metaprompter/node_modules`): on a 390x844 touch phone, the Map tab's height is 30% (253px) with the keyboard down and 30% of the visible height with a simulated keyboard; on a 1000x700 column layout it is unchanged from master.
- Malia's real-device pass on #834: the X, the keyboard after `say`, `party chat`, `talk <npc>` then a menu choice, `look`, and the toast, on iOS and Android, including once on mobile data.
- Gates: gofmt, build, lint, `go test ./...`, jstests, a boot.
