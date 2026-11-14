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
| Game text reaches the screen at one `term.write`, after `socket.onmessage` has returned early for GMCP | `WP:2976-3015`, `:3101` |
| The server takes input every turn; turns are 50ms in the shipped config | `_datafiles/config.yaml:189` |
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

## What players see

On a phone, an open tab takes at most 30% of the screen, so about 21 lines of game text stay visible on an iPhone with the keyboard down (11 today) and 13 with it up (7 today). Chatting or answering an NPC's menu never drops the keyboard; looking, moving and fighting still close it so the result can be read. A brass X at the end of the command box closes the keyboard by hand. Turning the phone sideways shows one short note suggesting it be held upright.

## Design

### 1. Keyboard after a send (`JS/hotinput.js`, `WP` send handler)

`shouldHideKeyboardAfterSend` becomes `keyboardAfterSend(s)` and returns `'keep'`, `'close'` or `'wait'`. `s` gains `line`, the text exactly as typed.

- Not a touch phone (`touchOnly && phoneWidth`), nothing sent, not in game, a menu already open, or the password prompt: `'keep'`. This is today's behaviour, unchanged.
- The line is a chat line: `'keep'`.
- Anything else: `'wait'`.

`isChatLine(line)` is exported beside it. The first word, lowercased, is matched against `CHAT_WORDS`: `say`, `whisper`, `reply`, `chat`, `newbie`, `trade`, `shout`, `emote`, `yell`, `scream`, `holler`, `/w`. A line starting `.` or the backtick counts whether or not a space follows (`.hello` is say). `party` and `guild` count when their second word is in `PARTY_TALK` (`chat`, `say`) or `GUILD_TALK` (`chat`, `say`, `gc`). The constants carry a comment naming R1 as the principle. A `talk` or `ask` line is not in the list: it waits, and the menu it opens keeps the keyboard (R2).

The send handler acts on the answer. `'close'` blurs as today. `'wait'` waits for the game's reply, so a slow connection delays the close and never drops a menu (owner, 2026-11-14, R5):

1. The keyboard stays up until the first game text after the send reaches the screen (the `term.write` at `WP:3101`; GMCP messages, which arrive all the time, do not count).
2. From that reply, `MENU_GRACE_MS` (100) more, because `Char.Menu` can trail the menu's text by a tick.
3. Then the input is blurred only if all of these hold: `menuOpen` is false, the command box still has focus, and its value is empty (the player has not started the next command).
4. `MENU_WAIT_CEILING_MS` (2000) after the send, step 3 runs anyway, for a command that prints nothing or a reply that never comes.

A new send cancels the previous wait. The `Char.Menu` handler needs no change: step 3 reads `menuOpen` when it runs. The wait is one small state object (`sentAt`, `replyAt`, the timers) kept beside `menuOpen`.

Typing time never counts: the wait starts at Enter. What it covers is the round trip, the server's input tick (`TurnMs: 50`, `_datafiles/config.yaml:189`) and the `Char.Menu` event, about 150 to 250ms on Wi-Fi and more on mobile data.

Ruling 13's aliases-and-macros exclusion stands: the client sees only the typed line, so an alias for `say` is a plain command and waits.

### 2. Tab height (`CSS`)

- Phone and slim: `--drawer-cap: max(80px, 30vh)`, with the `30dvh` twin in the existing `@supports` block. The `--drawer-keep` sum and its comment go: 30% already leaves the top of the screen.
- Phone and slim with the keyboard up: `--drawer-cap: max(60px, calc(0.3 * var(--visible-h, 100dvh)))`. `vh` cannot be used here: inside `/webclient` the frame is resized above the keyboard (`webclient.html:26-38`), so `vh` is the visible height, but the page opened on its own keeps `vh` at the layout height while only `body` is shrunk (`WP:2711-2713`). So both keyboard watches, the outer page's `fitFrameHeight` and the page's own, also set `--visible-h` on the game document's root, in px, from the same `visibleHeight` they already compute.
- Column: the cap rules move under `html.layout-column` unchanged, so a laptop's tab beside the game keeps its size, keyboard up or down.

### 3. The X (`WP` `#panel-cmd`)

A `<button type="button" id="cmd-close-kbd" aria-label="Close keyboard">` after the input, holding an inline stroke X. 48x48px (the command box's height; ruling 14 asks at least 44), the existing brass button look, `display: none` except under `html.keyboard-open`, so the box is full width the rest of the time. `pointerdown` calls `preventDefault()` so the tap does not move focus first, then `click` blurs the command box. It is a real button, reachable by keyboard and named for screen readers.

### 4. The sideways toast (`JS/viewportfit.js`, `JS/dashboard.js`)

`ViewportFit.isSidewaysPhone(screenW, screenH, orientationType, coarse)` is `coarse && isPhone && landscape`, the mirror of `isUprightPhone`, sharing its orientation reading (including the iOS `window.orientation` fallback, moved into a small `orientationType(win)` helper both use). Because it reads the screen, the keyboard cannot trigger it.

The dashboard checks it at load and on `orientationchange` and `resize`. The first time it is true in a browser tab, it shows `_toast("This game is easier to play with your phone held upright.")` and sets `sessionStorage['dogmud.sidewaysToast'] = '1'`. A reload in the same tab does not repeat it; a new tab or visit does. If `sessionStorage` throws, a page variable stops a repeat for that page load.

The toast gets a `.dash-toast.lifted` variant that sits just above the command box (`bottom: calc(var(--cmdbox-h) + var(--tabbar-h) + 12px)`), so on a phone it does not cover the tab bar. It shows for 4s, longer than the reset toast's 2.6s, since it is a sentence; `_toast` gains an optional duration.

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| A 100ms grace after the reply and a 2s ceiling, two named constants | The grace covers `Char.Menu` landing a tick (50ms) after the menu text; the ceiling is long enough for a poor mobile link and short enough that a silent command still closes the keyboard | Another player's line arriving right after a send counts as the reply, so on a slow link the keyboard can close just before a menu, as today. Malia's #834 test on mobile data checks it |
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
- The wait itself as pure steps: `blurAfterWait({menuOpen, focused, value})` is `true` only for not open, focused, empty; and the wait's timing (a reply then the grace, the ceiling with no reply, a second send cancelling the first) runs against fake timers.
- `tools/jstest/viewportfit.test.js`: `isSidewaysPhone` for an upright and sideways phone, a sideways tablet, a desktop, and an iOS device with no orientation API.
- A dashboard test for the toast's once-per-tab rule with a stub `sessionStorage`, and with one that throws.
- Headless Playwright check (plan 5's script, Playwright from `workspace/metaprompter/node_modules`): on a 390x844 touch phone, the Map tab's height is 30% (253px) with the keyboard down and 30% of the visible height with a simulated keyboard; on a 1000x700 column layout it is unchanged from master.
- Malia's real-device pass on #834: the X, the keyboard after `say`, `party chat`, `talk <npc>` then a menu choice, `look`, and the toast, on iOS and Android, including once on mobile data.
- Gates: gofmt, build, lint, `go test ./...`, jstests, a boot.
