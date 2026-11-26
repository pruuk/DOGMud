# Moving by touch, plan 2b: the deck (implementation plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The touch deck walks: the inverted T of compass and stairs buttons, the other-exits level (Back, named and diagonal exits, More), locked, dark and combat states, read from the `Room.Info` that plan 2a ships and sent back as `Char.Button.Press`, switched on by default for phones and tablets.

**Look (owner ruling R1, below):** every deck button is the site's shiny brass (`.brass`) with dark text; kinds differ by shape and edge; the pitch-dark state is olive grey.

**Architecture:** `touchdeck.js` stays the one pure module of the deck: it gains `moves` (one `Room.Info` and the `Commands.State` mode turned into a model), `cells` (the model laid out in the deck's shape and level), `levelAfter` and `tap` (the level and what a tap sends) and `render` (the cells drawn as buttons). `dashboard.js` `applyDeck` draws those cells where it drew empty slots, and keeps the state; the page hands it `Room.Info`, the mode and a sender (`SendGMCP("Char.Button.Press", {id})`, `gameLine`). One server change: an instance portal's button reads its zone (owner ruling 18), so `Room.Info` exits gain an optional `label`, decided by a new `rooms.ForDeck` on the server, because only the server knows which portal is the viewer's.

**Tech Stack:** Plain ES5 browser JavaScript (UMD, no build step), Node 24 jstests with no test runner, CSS in `dashboard.css` with tokens in `gomud.css`; Go 1.25 (module `github.com/GoMudEngine/GoMud`) and testify for the one server task.

**Spec:** `docs/superpowers/specs/2026-11-22-moving-by-touch-design.md` (issue #781, epic #783), section "2b: deck movement", rulings 3, 4, 8, 10, 11 and 18, and the client half of "Tests and checks". Roadmap row 2 of `docs/roadmaps/TOUCH_ARC_ROADMAP.md`. Mockup `docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html`: frames 1 to 6 and 8 are this plan; frames 7 (the Map tab) and 9's Menu button are plan 2c. **Not here:** the Map tab's copy of the T, the sideways phone's folded bars and Menu (#938), Flee and the combat swap (plan 3d), the command slots on the right (spec 3).

**Before building.** Ruling 12 asked for Malia's look at the mockup before 2b is built: she reviewed it on 2026-11-26 and her one change is ruling 18 (shipped in PR #963). The `Room.Info` lag measurement the spec asked for before 2b ships was done in plan 2a Task 16: nil lag (fact F44).

**Branch.** Execute on `feature/781-moving-by-touch-2b`, branched from `master` at `ebbba8037` or later. Every commit below builds and passes its tests.

**Rules every task follows (brief every subagent on these):**
- Edit files with the Edit and Write tools only. No `sed -i`, no Python read-modify-write, no `>` into tracked files (`guard.py` H3).
- `git add` named paths only; never `-A`, `.`, `--all`, never `git commit -a` (H1). Never stash (H6). Never `--no-verify` (H2).
- No em or en dashes in code comments, docs or player text. Player text follows `dogmud-player-copy` (plain, ESL-clear, no raw numbers).
- A new test is trusted only after it was seen red: every task has a sabotage step. Restore the exact line afterwards.
- A Go task's test step also runs `go test . ./internal/guards/ -count=1`. A JS task's test step runs the file it touched, then the JS gate (Task 10 step 4) before its commit.
- Every Go package touched keeps its `context.md` accurate (Task 9 lists the lines). `tools/context_md_audit.py` runs in the final gate.
- The page's code reaches the dashboard only through `window.Dashboard && Dashboard.x` guards: `tools/jstest/webclient-connect.test.js` evaluates `socket.onclose` with `window = {}`.
- Commit messages through a quoted heredoc, ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

---

## Facts verified against source (master ebbba8037)

Read on 2026-11-26 with grep and Read at `ebbba8037`. Line numbers move; re-grep before an edit.

| # | Fact | Where |
|---|---|---|
| F1 | `Room.Info` top level: `num`, `name`, `description`, `area`, `environment`, `symbol`, `coords`, `exits` (name to room id), `exitsv2`, `details`, `dark` (bool), `moves` (map), `darkmarks` (map), `Contents` | `modules/gmcp/gmcp.Room.go:635-648` |
| F2 | Each `exitsv2` entry: `num`, `dx`, `dy`, `dz`, `details` ([]string), `kind` (string), `id` (string). No `label` field today | `gmcp.Room.go:651-659` |
| F3 | `kind` is one of `compass`, `vertical`, `diagonal`, `named`, `temporary` (`rooms.ExitKind`); `ExitKindOf` marks every `ExitsTemp` entry `temporary` whatever its name | `internal/rooms/known_exits.go:19-51` |
| F4 | `details` carries `secret`, `locked` (locked now, `ke.LockedNow`), `player_has_key`, `player_has_pick_combo` | `gmcp.Room.go:332-347` |
| F5 | `moves` is `buttons.FixedIds()`: `north` m1, `south` m2, `east` m3, `west` m4, `up` m5, `down` m6, sent in every room, dark or lit | `internal/buttons/buttons.go:23-25`, `:38-40`; `gmcp.Room.go:305` |
| F6 | `darkmarks`: T direction to `wall` (`buttons.MarkWall`) or `locked` (`buttons.MarkLocked`), only in the dark | `internal/buttons/darkmarks.go:9-10`; `gmcp.Room.go:307` |
| F7 | At no sight `dark` is true and `exits`, `exitsv2` are present and empty; `moves` still holds the six fixed ids | `known_exits.go:85-86`; `modules/gmcp/gmcp.RoomExits_test.go:49-66` |
| F8 | `buildRoomExitsInfo(user, room, sight)` builds the exit half; ids come from `buttons.MovementIds(user.UserId, room, known)`; a temporary exit sharing a permanent exit's name is listed once | `gmcp.Room.go:300-351`, `:309`, `:312-316` |
| F9 | `MovementIds` gives a T direction its fixed id by name, so a temporary exit named `north` (the Planar Oasis threshold, `cubegen.go:195`) carries `m1` with kind `temporary` | `buttons.go:65-85`; `internal/rooms/cubegen.go:195-201` |
| F10 | A per-room id is `x` plus 8 hex digits; `Resolve` turns it into `go <exit name>` only in the room it was issued for | `buttons.go:126-153` |
| F11 | The round watcher's signature covers room, dark, each `exitsv2` entry's name, kind, id, room and details, and the marks; not a label | `modules/gmcp/gmcp.RoomExitsWatch.go:43-69` |
| F12 | Wire freeze pins the `Room.Info` keys and the `exitsv2` keys `num dx dy dz details kind id` | `modules/gmcp/gmcp_wire_freeze_test.go:31-37` |
| F13 | The client sends a press as `Char.Button.Press {"id": "<string>"}`; the server routes it to MainWorker and `pressButton` | `modules/gmcp/gmcp.go:409-416`; `gmcp.CharOp.go:162-185`; `gmcp.ButtonPress.go:40-54` |
| F14 | A press is queued as typed input (`CmdTyped|CmdFromClient|CmdButton`); a stale id gets `That button was out of date. Try again.` and `Room.Info` again | `gmcp.ButtonPress.go:44-53`; `internal/refusal/button.go:7` |
| F15 | No client code sends `Char.Button.Press` today (grep of `webclient-pure.html` and `static/js` for `Button.Press`: none; the same grep finds the server's routing, so it can match) | grep |
| F16 | `SendGMCP(pkg, obj)` sends a binary GMCP frame and returns false when the socket is not open | `_datafiles/html/public/webclient-pure.html:3624-3634` |
| F17 | `gameLine(text)` writes one of the page's own lines to the game window and the screen reader log | `webclient-pure.html:2989-2993` |
| F18 | The `Room` handler runs for `Room.Info` and its `Contents` pushes; it returns early only when `obj.Info` is missing; the map code follows | `webclient-pure.html:884-1004` |
| F19 | `handleGMCP` replaces a dotted namespace wholesale, so each `Room.Info` replaces the last (no stale `exitsv2` keys linger) | `webclient-pure.html:2741-2758` |
| F20 | `Commands.State` carries `mode`: `exploration`, `combat` or `dead`; pushed to every GMCP player every round | `modules/gmcp/gmcp.Commands.go:198-204`, `:95-118`, `:250` |
| F21 | The page reads the mode in `checkDeathClearsQueue` with `(((GMCPStructs["Commands"] || {}).State) || {}).mode || ""`; the `Commands` and `Commands.State` handlers call it | `webclient-pure.html:2264-2268`, `:1207-1223` |
| F22 | `tools/jstest/webclient-action-queue.test.js` evaluates the page from `var actionQueue = [];` (`:2028`) to `function runTriggers(` (`:2270`), so a helper placed above `checkDeathClearsQueue` is inside it | `webclient-action-queue.test.js:37` |
| F23 | `socket.onclose` starts with `stopAutomation();` then `setInputQueueFull(false);`; `webclient-connect.test.js` evaluates it with `global.window = {}` | `webclient-pure.html:3430-3432`; `tools/jstest/webclient-connect.test.js:80`, `:102` |
| F24 | The page's `init()` runs on `<body onLoad>`, after `dashboard.js` ran `Dashboard.init` on `DOMContentLoaded`; its last line is `connectButton.click();` | `webclient-pure.html:247`, `:3940`, `:4004`; `static/js/dashboard.js:1146-1148` |
| F25 | The deck element is `<div id="touch-deck" aria-hidden="true"></div>` inside `#touch-slot`; `tools/webclient-tests/touch-frame.js:35` pins that exact tag | `webclient-pure.html:331-333` |
| F26 | The Touch controls note reads "Buttons above the command box for moving and acting. Automatic turns them on for phones and tablets once they have buttons. Always shows them now." | `webclient-pure.html:306` |
| F27 | `touchdeck.js`: `AUTOMATIC_SHOWS = false` (`:35`); `deckOn` (`:53-57`); `BTN = 48, GAP = 8, SMALL_BTN = 44, SMALL_GAP = 5, PAD = 8` (`:60`); `ROWS = 2, PHONE_COLS = 6, SIDE_ROWS = 4, SIDE_COLS = 3` (`:61`); a local `span(n, btn, gap)` (`:64`); `shape(boxW, boxH, phone, sideways)` returns `{rows, cols, button, gap, pad, side, width, height}` (`:66-88`); exports at `:134-140` | `_datafiles/html/public/static/js/touchdeck.js` |
| F28 | A sideways phone's shape is 4 rows of 3 (`side: true`), 48px when `span(4, 48, 8) <= boxH`, else 44px | `touchdeck.js:78-81` |
| F29 | `touchdeck.test.js:46-48` pin `AUTOMATIC_SHOWS` false and Automatic off on touch | `tools/jstest/touchdeck.test.js` |
| F30 | `applyDeck` decides the slot (`TD.slotShows`), sets `--deck-cols`, `--deck-btn`, `--deck-gap`, `--deck-pad`, `--deck-h`, then draws `s.rows * s.cols` empty `span.deck-slot` cells when the count changes | `static/js/dashboard.js:626-679` |
| F31 | `Dashboard.init` reads `#touch-slot` and `#touch-deck` (`:190-191`); the first `var self = this;` in `init` is at `:198`, after them | `dashboard.js` |
| F32 | `dashboard.js` builds line icons from stroke paths with `createElementNS(SVG_NS, "svg")` (`PANEL_TABS`, `makeTabBar`) | `dashboard.js:50-90` |
| F33 | `#touch-deck` CSS: a grid sized by `var(--deck-btn ...)` and `var(--deck-gap ...)`, `background: var(--ink-deep)`, `border-top: 2px solid var(--gold-rule)` (`:268-277`); `.deck-slot` is a dashed brass-border cell (`:280-284`); while the queue is full the deck greys and ignores taps (`:1935-1942`); the sideways side column (`:1336-1355`) | `_datafiles/html/public/static/css/dashboard.css` |
| F34 | `touch-targets.test.js` takes the first `#touch-deck {` rule and requires `var(--deck-btn` and `var(--deck-gap` in it, and requires `.deck-slot` to set no size | `tools/jstest/touch-targets.test.js:80-86` |
| F35 | Type guard: `font-size` must be `var(--text|title|cmd|nav-text|foot-text)` (`typeSizeOK`, `:68`), tokens defined only in `gomud.css` (`typeTokenDef`, `:73`), weight `400`, `normal` or `var(--bold)`; `dashboard.css` and `dashboard.js` are in scope, `touchdeck.js` is not; the ruled sizes are pinned in `TestTypeTokensHoldTheRuledSizes` | `internal/guards/client_type_guard_test.go:21-38`, `:66-77`, `:203-228` |
| F36 | Type tokens: `--text: 16px; --title: 20px;` (`gomud.css:166-167`); upright phones `--text: 14px; --title: 18px;` (`:199`); no 12px token exists | `_datafiles/html/public/static/css/gomud.css` |
| F37 | The web client is always dark: the Light theme rules are `:root[data-theme="light"]:not([data-client])`, and the client page is `<html lang="en" data-client>` | `gomud.css:215`; `webclient-pure.html:2` |
| F38 | Brand: "Two sizes"; "Every button is brass"; kinds by shape and edge (movement square, other exits dashed, Back and More heavy square edge); labels at the client's text size; red only for warnings; icons and edges at least 3:1, text 4.5:1 | `docs/guides/BRAND_STANDARDS.md` sections 2 to 4, `:119-127` |
| F39 | (Lit colours superseded by owner ruling R1; the dark colours stand.) The approved mockup draws movement tiles `#3a2c1d` with a `#c9a35a` edge and `#f1e4cc` text (arrow 18px, letter 10px), dim at opacity .22, dark tiles `#19140c` with a dashed `#5d5c4e` edge and `#8b897a` text, the wall mark `\2715` in `#e35349`, the Dark tile `#252015` / `#969889` / `#cec6aa`, Back `#2a2017` / `#8a6f3a` / `#e8d3a0`, other exits a 2px dashed edge, named labels 12px | `docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html:31-56` |
| F40 | Mockup frame 9 (sideways): row 1 up, north, down; row 2 west, south, east; row 3 the exits button in column 1 | `s21-moving-by-touch.html:280-285` |
| F41 | `exit.MaxNameLetters = 6`; the instance portal key `<zone>-<userId>` is exempt | `internal/exit/name.go:21`, `:16-20` |
| F42 | The portal key is built inline: `exitKey := zoneName + "-" + fmt.Sprintf("%d", user.UserId)`, added with `UserId: user.UserId`; the instance authorizes the opener and their party at that moment | `internal/behaviortree/actions_mob.go:569`, `:577-582`, `:548-555` |
| F43 | `KnownExits` lists every `ExitsTemp` entry, so a player sees other players' portals too; `MoveToRoom` refuses an ephemeral room whose instance does not authorize the player ("The portal's energy pushes you back. It wasn't opened for you.") | `known_exits.go:103-105`; `internal/rooms/roommanager.go:455-465`; `internal/rooms/instances.go:34-36` |
| F44 | `Room.Info` reaches the client 0.0 to 0.5 ms before its "You head" line (plan 2a Task 16, 10 moves): no lag to fix before 2b | `docs/superpowers/plans/2026-11-22-moving-by-touch-2a-server.md:4860` |
| F45 | The client keeps no user id: grep of `webclient-pure.html` for `userid`/`user_id` (any case) finds nothing | grep |
| F46 | No double-tap suppression exists in the client (grep for `double.?tap`, `DOUBLE_TAP`, any case, in `static/js` and the page: none) | grep |
| F47 | `HotInput.shouldFocusCommandBar` leaves a pointer-focused button's keys to the command bar, so deck buttons need no change there | `static/js/hotinput.js:123-136` |
| F48 | `LongPress` is not needed: no movement button has a long press in spec 2 | spec "2b: deck movement" |
| F49 | `webclient-colours.test.js` has `contrast(a, b)` and `token(css, name)` (reads `--name: #hex`) and reads `gomud.css` and `dashboard.css` | `tools/jstest/webclient-colours.test.js:18-36` |
| F50 | `live-shot.js` steps: `send`, `wait`, `tail`, `eval`, `shot`; per-session `emulate: {width, height, dpr, mobile, touch}` sets page and screen size and touch | `tools/webshot/live-shot.js:8-21`, `:160-172` |
| F51 | Size matrix: 375x548, 360x800, 412x915, 320x640, 800x360, 820x1180, 1180x820, 690x840, 1366x768 | `touch-targets.test.js:46-56` |
| F52 | CI's javascript job: `node --check` every tracked non-vendored `.js`, every `tools/jstest/*.test.js`, every `tools/webclient-tests/*.js` | `.github/workflows/validate.yml:80-130` |
| F53 | Rooms for checks: Tower Top 5243 (west, shrine, ledge); Junction Vault 6408 (northeast, southwest); Guard Barracks 473 (down locked, south, up); Old Watercourse 6418 (east, west, secret north; dark); Cracked Cistern 5235 (west, northeast) in Pothole Coulee; the start room is 5200, The Awakening Pool, Pothole Coulee | room YAML; `_datafiles/config.yaml:2702` |
| F54 | `glow` is an alias of the spell Chrysalis Glow, so "cast glow" in the Dark tile's line works as typed | `_datafiles/world/dogmud/spells/chrysalis-glow.yaml:2-4` |
| F55 | The site's brass component is the class `.brass`: `color: var(--brass-text)`, `background: var(--brass-grad)`, `border: 1px solid var(--brass-border)`, `border-radius: 6px`, a pressed `:active` state, `display: inline-block`; the client already puts it on its own buttons (`#cmd-close-kbd class="brass"`) | `_datafiles/html/public/static/css/gomud.css:307-329`; `webclient-pure.html:339` |
| F56 | Brass tokens: `--brass-text: #1a1206`, `--brass-border: #50371d` (`gomud.css:25-26`), `--brass-grad` a radial shine (`:24`); in the web client `:root[data-client] { --brass-grad: linear-gradient(#edd489, #c8a95b); }` ("label #1a1206 12.69 at the top, 8.19 at the bottom") and `.brass { text-shadow: none; }` | `dashboard.css:1384-1390` |
| F57 | The brand doc's known shortfall (3.08 where a label reaches the dark edge `#736133`) is the radial shine written out in `.subtab`'s own `background`, not `--brass-grad` in the client | `BRAND_STANDARDS.md:66-70`; `dashboard.css:1228` |
| F58 | `dashboard.css` loads after `gomud.css`, so a `dashboard.css` rule of equal specificity wins over `.brass` | `webclient-pure.html:210-211` |

**Could not verify:** JetBrains Mono's letter width (0.6 em) is its published advance width, not read from the woff2 here; Task 11 measures a six-letter label in the real browser. Whether a newly created character can walk to a pitch-dark room in ordinary play is not checked; Task 12 says what to do if not.

## Owner rulings on this plan (2026-11-26, owner and Malia, plan review)

- **R1 (was D5): every deck button is shiny brass with dark text**, the brand rule, as the round tabs and sub-tabs: the movement T, the exits button, named and diagonal exits, Back and More. They wear the site's `.brass` class (F55), so they take the client's own brass (`--brass-grad` with the client's linear override, `--brass-border`, `--brass-text`, F56) from its tokens; no colour is copied. Kinds still differ by shape and edge: movement square, other exits a dashed edge, Back and More a heavy square edge. The pitch-dark state keeps the ruled olive grey (ruling 11).
- **R2: every exit the player may legally enter gets a button**, including a party member's instance portal beside the player's own. A portal the player may not enter keeps none (it is still listed on the exits line).

## Decisions taken without asking

| # | Decision | Why | If wrong |
|---|---|---|---|
| D1 | The instance portal's label (ruling 18) is decided on the server: `exitsv2` entries gain `label` (omitted when it is the exit's own name), set by a new `rooms.ForDeck` | The client has no user id (F45), so it cannot tell the viewer's portal from another's; the sync rule keeps button building on the server | Strip `-<digits>` in the client and show every portal |
| D2 | "May legally enter" (R2) is `rooms.MayEnterInstance`, the rule `MoveToRoom` refuses on: the opener and the party the instance authorized when it opened (F42). A portal that fails it keeps no `id` but stays in `exits` and `exitsv2` | Ruling 6: listers agree, so the exits line and the maps keep it; R2 gives buttons only to exits the player may enter | Give every portal a button and let the server refuse it as typed |
| D3 | Two portals to one zone both get a button with the same label (the zone). The player's own sorts first, then the others A to Z by key; the server marks the own one with the `exitsv2` detail `own_portal` (the client has no user id, F45). Its screen reader label adds "(yours)"; the visible label does not change | R2; a detail string reuses the existing `details` list (no new wire field); the order puts the exit ruling 18 named, "their own instance exit", in the first slot | Open question 1: show something on the second button |
| D4 | `MoveToRoom`'s instance check becomes `rooms.MayEnterInstance`, read by `ForDeck` too | One rule for who may enter (unify rule) | Keep the check inline and copy it |
| D5 | (Now ruling R1, above.) | | |
| D6 | The pitch-dark tile's edge is olive grey `#7e8073` (4.55:1), not the mockup's `#5d5c4e` (2.71:1). Still needed: the dark state keeps olive grey under R1 | Brand rule: edges at least 3:1; the next step of the same olive ramp | Keep `#5d5c4e` |
| D7 | The letter under an arrow is 12px (mockup 10px); the arrow is `--title` (18px on an upright phone, as drawn) | 12px is the only small size ruling 18 allows; the type guard admits no other | Draw arrow and letter as one SVG |
| D8 | A third type token `--deck-label: 12px` in `gomud.css`, admitted by the type guard and pinned there, recorded in the brand doc | Ruling 18 sets named-button text at 12px; the guard's rule is "sizes come from tokens" | Per-line exemptions in `typeExemptLines` |
| D9 | At 44px buttons a name gets `letter-spacing: -0.5px` | Six letters at 12px are 43.2px, the inside of a 44px button with 1px edges is 42px; with the spacing 40.2px | Let the name touch the edge |
| D10 | A lit room's "no exit" arrow is disabled; in the dark all six stay tappable; in combat the dimmed buttons stay tappable | Ruling 4 for the dark; the mode comes once a round (F20), so a disabled deck would stay dead up to a round after a fight, and the server's refusal says why | Disable movement in combat |
| D11 | A tap on the exits button, Back or More, or on an exit that sends, holds every tap off for 0.5 s (`guardUntil` in the tap state): each of them redraws the deck or moves the player, so a different button lands under the finger and the second tap of a double tap must not act on it (double tap on exits sending the third exit, double tap on Back walking up, an exit then its `Room.Info` then a second walk north). A T tap sets no hold, so T after T sends twice | Roadmap fat-finger rule; ruling 8 for the T; review of Tasks 2 to 4 found the per-button guard let those three sequences act | Ignore quick repeats on the T too |
| D12 | The client sorts other exits again (diagonals clockwise from northeast, then A to Z by label), with a jstest that reads the server's `exitOrder` and fails if they part | `exitsv2` is a JSON object (Go marshals maps by key), so the server's order does not survive; a sort is cheaper than a new wire field | Add an order field to `exitsv2` |
| D13 | Back reads "‹ Back" and More "More ›"; More turns to the next page and wraps to the first | The mockup's Back; no room needs More today | Back names its target in words |
| D14 | Back and More get a 2px solid `--brass-border` edge with 2px corners, the exits button a 2px dashed one; movement keeps `.brass`'s 1px edge and 6px corners. The brass fill, not the edge, outlines a button on the deck (6.82 to 10.57 against `--ink-deep`; the edge alone is 1.40), and the edge reads on the brass (4.87 to 7.55) | Brand: kinds by shape and edge; under R1 the edge must read against brass, not against the deck | 1px as drawn |
| D15 | A sideways phone's side column carries the T and the exits button in 2b (frame 9 places); Menu stays in 2c | The side column exists since plan 1a and would otherwise show empty slots once the deck turns on | Hide the deck sideways until 2c |
| D16 | The Dark tile's line is written by the page (`gameLine`), not the server | It is not a command and needs no server state; `gameLine` is the page's one helper for its own lines | Make it a server button |
| D17 | `#touch-deck` loses `aria-hidden` and becomes `role="group" aria-label="Movement"`; each button has a plain label ("Go north (locked)") | Focusable buttons inside `aria-hidden` are unreachable for TalkBack and VoiceOver | Keep it hidden |
| D18 | Mockup frame 2 shows shrine before ledge; the plan lists ledge first | The spec rules A to Z | Order by the room file |
| D19 | A closed connection clears the deck to empty slots until the next `Room.Info` | A button pressed on a dead socket sends nothing | Keep the last buttons |
| D20 | The list stays open when `Room.Info` repeats in the same room, and closes on a move, in the dark, or when no other exit is left | A door unlocking should not throw the player out of the list | Close it on every `Room.Info` |
| D21 | Only `combat` dims; `dead` does not | The spec names combat only | Dim when dead too |
| D22 | Deck buttons carry `touch-action: manipulation` | A fast double tap must walk twice (ruling 8), not zoom | None |
| D23 | No adversarial playtest seat in 2b | 2b adds no game rule beyond the label; the server rules had their seat in plan 2a | Add a seat that presses forged and stale ids |
| D24 | Dim (no exit) and combat-dimmed buttons stay brass at opacity 0.22 over the deck. A lit button's darkest stop against a dim one's lightest is 3.83:1, so the state reads at the 3:1 icon bar; the dim tile against the deck is 1.54 to 1.78, which WCAG allows for an inactive control | The mockup's dim; one opacity for both dims | A higher opacity (0.45 gives dim tile 2.51 to 3.32 against the deck but lit against dim only 2.06) |
| D25 | The padlock draws in `currentColor`: the brass text on a lit button (8.19 to 12.69), the olive grey text on a dark tile (5.19) | One stroke rule; no padlock colour of its own | A padlock token |
| D26 | The brand doc's known shortfall does not reach the deck: deck buttons use `--brass-grad`, which the client overrides with the even linear brass (F56, F57), so a 12px label is 8.19 at its worst. A jstest reads the override and fails if a label drops below 4.5 on either stop | `.subtab`'s shortfall comes from its own written-out radial shine | Give the deck its own brass |

---

## File map

**New files**
- `tools/jstest/touchdeck-moves.test.js`: the movement model, cells, level, tap and render.
- `tools/jstest/dashboard-deck.test.js`: `dashboard.js` wiring (setMoves, setDeckMode, taps, sender) against the real `touchdeck.js`.
- `internal/rooms/known_exits_deck_test.go`: `ForDeck` and `MayEnterInstance`.

**Modified files** (task in brackets): `internal/exit/name.go`, `internal/exit/name_test.go`, `internal/rooms/known_exits.go`, `internal/rooms/roommanager.go`, `internal/behaviortree/actions_mob.go`, `modules/gmcp/gmcp.Room.go`, `modules/gmcp/gmcp.RoomExits_test.go`, `modules/gmcp/gmcp_wire_freeze_test.go` [1]; `_datafiles/html/public/static/js/touchdeck.js`, `internal/guards/exit_name_length_guard_test.go` [2, 3, 4]; `_datafiles/html/public/static/css/gomud.css`, `internal/guards/client_type_guard_test.go` [5]; `_datafiles/html/public/static/css/dashboard.css`, `tools/jstest/webclient-colours.test.js`, `tools/jstest/touch-targets.test.js` [6]; `_datafiles/html/public/static/js/dashboard.js`, `_datafiles/html/public/webclient-pure.html`, `tools/webclient-tests/touch-frame.js` [7]; `touchdeck.js`, `tools/jstest/touchdeck.test.js`, `webclient-pure.html` [8]; `docs/README.md`, `docs/PATCH_NOTES.md`, `docs/guides/BRAND_STANDARDS.md`, `internal/exit/context.md`, `internal/rooms/context.md`, `internal/behaviortree/context.md`, `modules/gmcp/context.md` [9].

## Task list

1. Server: an instance portal's button reads its zone (`exit.InstancePortalKey`, `rooms.MayEnterInstance`, `rooms.ForDeck`, `exitsv2` `label`)
2. `touchdeck.js`: the movement model (`moves`) and its order guard
3. `touchdeck.js`: cells, level and tap
4. `touchdeck.js`: drawing the buttons (`render`)
5. The `--deck-label` type token (ruling 18)
6. The deck's stylesheet, its contrast checks and the name-fit check
7. Wiring: `dashboard.js`, the page, the static checks
8. Turning it on: `AUTOMATIC_SHOWS` and the Settings note
9. Docs: README, PATCH_NOTES, brand doc, context.md
10. Full gate
11. HEADLESS: the size matrix screenshots and measures (controller)
12. OWNER'S CHECK: Malia's S21 (#834)
13. PR

---

### Task 1: Server: an instance portal's button reads its zone

**Files:**
- Modify: `internal/exit/name.go`, `internal/exit/name_test.go`
- Modify: `internal/rooms/roommanager.go` (`:455-465`), `internal/rooms/known_exits.go`
- Create: `internal/rooms/known_exits_deck_test.go`
- Modify: `internal/behaviortree/actions_mob.go:569`
- Modify: `modules/gmcp/gmcp.Room.go` (`:300-351`, `:651-659`), `modules/gmcp/gmcp.RoomExits_test.go`, `modules/gmcp/gmcp_wire_freeze_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/exit/name_test.go`:

```go
// Owner ruling 18: a player's instance portal is keyed "<zone>-<userId>",
// and its deck button reads the zone.
func TestInstancePortalKeyAndZone(t *testing.T) {
	key := InstancePortalKey("arena", 42)
	if key != "arena-42" {
		t.Fatalf("InstancePortalKey = %q, want arena-42", key)
	}
	if zone, ok := InstancePortalZone(key, TemporaryRoomExit{UserId: 42}); !ok || zone != "arena" {
		t.Fatalf("InstancePortalZone(%q) = %q, %v; want arena, true", key, zone, ok)
	}
	for _, c := range []struct {
		key string
		uid int
	}{
		{"arena-42", 0}, // no opener: the ferry plank, a mob rift, the instance return
		{"arena-42", 7}, // a number that is not the exit's opener
		{"return", 0},
		{"-42", 42}, // no zone
		{"plank", 42},
	} {
		if zone, ok := InstancePortalZone(c.key, TemporaryRoomExit{UserId: c.uid}); ok {
			t.Errorf("InstancePortalZone(%q, uid %d) = %q, true; want false", c.key, c.uid, zone)
		}
	}
}
```

Create `internal/rooms/known_exits_deck_test.go`:

```go
package rooms

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/exit"
)

func deckNames(l KnownExitList) []string {
	out := []string{}
	for _, ke := range l.Exits {
		out = append(out, ke.Name)
	}
	return out
}

// freshInstance swaps in an empty instance registry holding one instance
// whose entry room is entry, authorizing users.
func freshInstance(t *testing.T, entry int, users ...int) {
	t.Helper()
	orig := instanceRegistry
	t.Cleanup(func() { instanceRegistry = orig })
	instanceRegistry = NewInstanceRegistry()
	instanceRegistry.Add(&ZoneInstance{InstanceId: entry, EntryRoomId: entry, AuthorizedUsers: users})
}

func TestMayEnterInstance(t *testing.T) {
	entry := ephemeralRoomIdMinimum + 4343
	freshInstance(t, entry, 42)
	require.True(t, MayEnterInstance(99, 1), "a room outside any instance")
	require.True(t, MayEnterInstance(42, entry), "a player the instance authorized")
	require.False(t, MayEnterInstance(99, entry), "a stranger")
	require.True(t, MayEnterInstance(99, entry+1), "an ephemeral room no instance holds, as MoveToRoom allowed")
}

// Owner ruling 18: the deck labels a player's instance portal with its zone
// and offers it only to a player who may enter it; known itself is left
// whole, since the exits line lists every exit.
func TestForDeck_LabelsAnInstancePortalByItsZoneAndOffersOnlyEnterablePortals(t *testing.T) {
	entry := ephemeralRoomIdMinimum + 4242
	freshInstance(t, entry, 42, 43)
	known := KnownExitList{Exits: []KnownExit{
		{Name: "west", Kind: ExitCompass, RoomId: 1},
		{Name: "shrine", Kind: ExitNamed, RoomId: 2},
		{Name: "arena-42", Kind: ExitTemporary, RoomId: entry, Temp: exit.TemporaryRoomExit{RoomId: entry, UserId: 42}},
	}}

	deck, labels := ForDeck(known, 42)
	require.Equal(t, []string{"west", "shrine", "arena-42"}, deckNames(deck), "the opener")
	require.Equal(t, "arena", labels["arena-42"])
	require.NotContains(t, labels, "shrine", "an exit shown by its own name has no label")

	deck, _ = ForDeck(known, 43)
	require.Contains(t, deckNames(deck), "arena-42", "a party member the instance authorized")

	deck, labels = ForDeck(known, 99)
	require.NotContains(t, deckNames(deck), "arena-42", "a stranger: the portal would push them back")
	require.NotContains(t, labels, "arena-42")
	require.Len(t, known.Exits, 3, "known is left whole")
}

// R2: a party member's portal to the same zone gets a button beside the
// player's own; both read the zone; only the player's own is theirs.
func TestForDeck_EveryPortalAPlayerMayEnterGetsAButton(t *testing.T) {
	entry42, entry43 := ephemeralRoomIdMinimum+4242, ephemeralRoomIdMinimum+4343
	orig := instanceRegistry
	t.Cleanup(func() { instanceRegistry = orig })
	instanceRegistry = NewInstanceRegistry()
	instanceRegistry.Add(&ZoneInstance{InstanceId: entry42, EntryRoomId: entry42, AuthorizedUsers: []int{42, 43}})
	instanceRegistry.Add(&ZoneInstance{InstanceId: entry43, EntryRoomId: entry43, AuthorizedUsers: []int{43}})
	known := KnownExitList{Exits: []KnownExit{
		{Name: "arena-42", Kind: ExitTemporary, RoomId: entry42, Temp: exit.TemporaryRoomExit{RoomId: entry42, UserId: 42}},
		{Name: "arena-43", Kind: ExitTemporary, RoomId: entry43, Temp: exit.TemporaryRoomExit{RoomId: entry43, UserId: 43}},
	}}
	deck, labels := ForDeck(known, 43)
	require.Equal(t, []string{"arena-42", "arena-43"}, deckNames(deck), "43 may enter both")
	require.Equal(t, "arena", labels["arena-42"])
	require.Equal(t, "arena", labels["arena-43"])
	require.False(t, IsOwnPortal(known.Exits[0], 43))
	require.True(t, IsOwnPortal(known.Exits[1], 43))

	deck, _ = ForDeck(known, 42)
	require.Equal(t, []string{"arena-42"}, deckNames(deck), "42 was not authorized for 43's instance")
	require.False(t, IsOwnPortal(KnownExit{Name: "north", Kind: ExitCompass}, 42), "a room exit is nobody's portal")
}

func TestForDeck_ARoomWithNoPortalIsReturnedAsItIs(t *testing.T) {
	known := KnownExitList{Exits: []KnownExit{
		{Name: "north", Kind: ExitCompass, RoomId: 1},
		{Name: "plank", Kind: ExitTemporary, RoomId: 2, Temp: exit.TemporaryRoomExit{RoomId: 2}},
	}}
	deck, labels := ForDeck(known, 42)
	require.Equal(t, []string{"north", "plank"}, deckNames(deck))
	require.Nil(t, labels)
}
```

Append to `modules/gmcp/gmcp.RoomExits_test.go`:

```go
// Owner ruling 18 and R2: an instance portal's button reads its zone, not
// the number, and is wired to that portal's own key; a second portal the
// viewer may enter gets a button too, and only the viewer's own carries the
// own_portal detail the client sorts first by. (Neither destination is an
// instance room here, so the viewer may enter both; the refusal case is
// rooms.TestForDeck_*.)
func TestRoomInfo_InstancePortalsAreLabelledByZoneAndTheViewersOwnIsMarked(t *testing.T) {
	viewer, room := exitsFixture(t, 60)
	own := exit.InstancePortalKey("arena", viewer.UserId)
	theirs := exit.InstancePortalKey("arena", viewer.UserId+1)
	require.True(t, room.AddTemporaryExit(own, exit.TemporaryRoomExit{RoomId: 9721, UserId: viewer.UserId}))
	require.True(t, room.AddTemporaryExit(theirs, exit.TemporaryRoomExit{RoomId: 9721, UserId: viewer.UserId + 1}))

	p := roomInfoFor(t, viewer)
	for _, key := range []string{own, theirs} {
		require.Equal(t, "arena", p.ExitsV2[key].Label, key)
		line, ok := buttons.Resolve(viewer.UserId, 9720, p.ExitsV2[key].Id)
		require.True(t, ok, key)
		require.Equal(t, "go "+key, line)
	}
	require.Contains(t, p.ExitsV2[own].Details, "own_portal")
	require.NotContains(t, p.ExitsV2[theirs].Details, "own_portal")
	require.Empty(t, p.ExitsV2["out"].Label, "an exit shown by its own name sends no label")
	require.NotContains(t, p.ExitsV2["portal"].Details, "own_portal", "a plain temporary exit is nobody's portal")
}
```

In `modules/gmcp/gmcp_wire_freeze_test.go`, directly after the `for _, k := range []string{"num", "dx", ...}` loop, add:

```go
	// The deck button's text when it is not the exit's name (ruling 18);
	// omitted otherwise.
	assert.Contains(t, keysOf(GMCPRoomModule_Payload_Contents_ExitInfo{Label: "arena"}), "label")
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/exit/ ./internal/rooms/ ./modules/gmcp/ -run "InstancePortal|MayEnterInstance|ForDeck|WireFreeze" -count=1`
Expected: build failures: `undefined: InstancePortalKey`, `undefined: MayEnterInstance`, `undefined: ForDeck`, `undefined: IsOwnPortal`, `unknown field Label`.

- [ ] **Step 3: Implement**

`internal/exit/name.go`: change the import block to

```go
import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)
```

and append:

```go
// InstancePortalKey is the exit key of the portal a player opens into their
// own instance of zone (behaviortree actOpenInstancePortal): "<zone>-<userId>".
// It is exempt from MaxNameLetters (owner ruling 18): the touch deck labels
// it with InstancePortalZone instead.
func InstancePortalKey(zone string, userId int) string {
	return zone + "-" + strconv.Itoa(userId)
}

// InstancePortalZone reports the zone of a player's instance portal: a
// temporary exit whose key is InstancePortalKey of some zone and of the
// exit's own UserId. Any other exit: false. The touch deck's button reads
// the zone ("arena"), never the number (owner ruling 18).
func InstancePortalZone(key string, t TemporaryRoomExit) (string, bool) {
	if t.UserId <= 0 {
		return "", false
	}
	zone, ok := strings.CutSuffix(key, "-"+strconv.Itoa(t.UserId))
	if !ok || zone == "" {
		return "", false
	}
	return zone, true
}
```

`internal/behaviortree/actions_mob.go:569`: replace

```go
	exitKey := zoneName + "-" + fmt.Sprintf("%d", user.UserId)
```

with

```go
	exitKey := exit.InstancePortalKey(zoneName, user.UserId)
```

(`fmt` stays imported: `:323` uses it.)

`internal/rooms/roommanager.go`: directly above `func MoveToRoom(` add

```go
// MayEnterInstance reports whether userId may enter roomId: any room outside
// an instance, an ephemeral room no instance holds, or a room of an instance
// that authorized them (the opener and their party when it opened).
// MoveToRoom refuses on it, and the touch deck offers an instance portal only
// when it holds (rooms.ForDeck, owner ruling 18).
func MayEnterInstance(userId int, roomId int) bool {
	if !IsEphemeralRoomId(roomId) {
		return true
	}
	inst := instanceRegistry.FindByRoomId(roomId)
	return inst == nil || inst.IsAuthorized(userId)
}

```

and in `MoveToRoom` replace

```go
	// Instance access control: block unauthorized entry to instanced zones
	if IsEphemeralRoomId(toRoomId) {
		if inst := instanceRegistry.FindByRoomId(toRoomId); inst != nil {
			if !inst.IsAuthorized(userId) {
				if user != nil {
					user.SendText(messaging.CategoryError, `<ansi fg="red">The portal's energy pushes you back. It wasn't opened for you.</ansi>`)
				}
				return ErrInstanceAccessDenied
			}
		}
	}
```

with

```go
	// Instance access control: block unauthorized entry to instanced zones
	if !MayEnterInstance(userId, toRoomId) {
		if user != nil {
			user.SendText(messaging.CategoryError, `<ansi fg="red">The portal's energy pushes you back. It wasn't opened for you.</ansi>`)
		}
		return ErrInstanceAccessDenied
	}
```

`internal/rooms/known_exits.go`: append

```go
// ForDeck is the part of known the touch deck offers userId, and the label
// of each offered button whose text is not its exit's name, by exit name
// (#781, owner ruling 18 and the plan review's R2: every exit the player may
// legally enter gets a button). An exit is offered under its own name,
// except a player's instance portal (exit.InstancePortalZone): it is
// labelled with its zone, without the number, and offered only to a player
// MayEnterInstance lets through, so a party member's portal is offered
// beside the player's own. known is not changed: the exits line and
// Room.Info's exit maps still list every exit. A room with no portal (nearly
// every room) returns known itself and nil labels, since Room.Info's round
// watcher calls this for every GMCP player every round.
func ForDeck(known KnownExitList, userId int) (KnownExitList, map[string]string) {
	var labels map[string]string
	for _, ke := range known.Exits {
		if _, portal := deckPortalZone(ke); portal {
			labels = map[string]string{}
			break
		}
	}
	if labels == nil {
		return known, nil
	}
	deck := KnownExitList{Dark: known.Dark, Exits: make([]KnownExit, 0, len(known.Exits))}
	for _, ke := range known.Exits {
		zone, portal := deckPortalZone(ke)
		if !portal {
			deck.Exits = append(deck.Exits, ke)
			continue
		}
		if !MayEnterInstance(userId, ke.RoomId) {
			continue
		}
		deck.Exits = append(deck.Exits, ke)
		labels[ke.Name] = zone
	}
	return deck, labels
}

// IsOwnPortal reports whether ke is userId's own instance portal, the one
// the deck lists first among portals to one zone (plan 2b D3).
func IsOwnPortal(ke KnownExit, userId int) bool {
	_, portal := deckPortalZone(ke)
	return portal && ke.Temp.UserId == userId
}

// deckPortalZone is exit.InstancePortalZone for a known exit; only a
// temporary exit can be a player's instance portal.
func deckPortalZone(ke KnownExit) (string, bool) {
	if ke.Kind != ExitTemporary {
		return "", false
	}
	return exit.InstancePortalZone(ke.Name, ke.Temp)
}
```

`modules/gmcp/gmcp.Room.go`, in `GMCPRoomModule_Payload_Contents_ExitInfo` after the `Id` field add:

```go
	Label   string   `json:"label,omitempty"` // the deck button's text when it is not the exit's name: an instance portal's zone (ruling 18)
```

and in `buildRoomExitsInfo` replace

```go
	ids := buttons.MovementIds(user.UserId, room, known)
```

with

```go
	// The deck's buttons: every known exit the viewer may enter (rooms.ForDeck,
	// owner ruling 18 and R2). An instance portal they may not enter stays in
	// the exit maps, as on the exits line, with no id.
	deck, labels := rooms.ForDeck(known, user.UserId)
	ids := buttons.MovementIds(user.UserId, room, deck)
```

and directly after the `v2 := GMCPRoomModule_Payload_Contents_ExitInfo{ ... }` literal add:

```go
		if label, ok := labels[ke.Name]; ok {
			v2.Label = label
		}
		// The viewer's own instance portal: the deck lists it first among
		// portals to one zone (plan 2b D3; the client has no user id).
		if rooms.IsOwnPortal(ke, user.UserId) {
			v2.Details = append(v2.Details, `own_portal`)
		}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/exit/ ./internal/rooms/ ./internal/behaviortree/ ./modules/gmcp/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: every package `ok`. `internal/behaviortree` `portal_stake_test.go:169-170` (`arena-1`) still passes: the key is unchanged.

- [ ] **Step 5: Prove the tests can fail**

In `ForDeck` change `if !MayEnterInstance(userId, ke.RoomId) {` to `if false {`: `TestForDeck_LabelsAnInstancePortal...` fails at "a stranger" and `TestForDeck_EveryPortal...` at "42 was not authorized". Restore. In `buildRoomExitsInfo` delete the `own_portal` append: the gmcp test fails at `Contains ... "own_portal"`. Restore. All pass again.

- [ ] **Step 6: Commit**

```bash
git add internal/exit/name.go internal/exit/name_test.go internal/rooms/roommanager.go internal/rooms/known_exits.go internal/rooms/known_exits_deck_test.go internal/behaviortree/actions_mob.go modules/gmcp/gmcp.Room.go modules/gmcp/gmcp.RoomExits_test.go modules/gmcp/gmcp_wire_freeze_test.go
git commit -F - <<'EOF'
feat(gmcp): an instance portal's deck button reads its zone (#781)

Owner ruling 18: Room.Info exitsv2 gains an optional label, the button
text when it is not the exit's name. rooms.ForDeck labels a player's
instance portal with its zone and offers every portal the player may
enter (rooms.MayEnterInstance, now MoveToRoom's check too), a party
member's beside their own; the own one carries the detail own_portal so
the deck lists it first. exit.InstancePortalKey is the one place the key
is built.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: `touchdeck.js`: the movement model and its order guard

**Files:**
- Modify: `_datafiles/html/public/static/js/touchdeck.js`
- Create: `tools/jstest/touchdeck-moves.test.js`
- Modify: `internal/guards/exit_name_length_guard_test.go`

- [ ] **Step 1: Write the failing tests**

Create `tools/jstest/touchdeck-moves.test.js`:

```js
// touchdeck-moves.test.js: moving by touch on the deck (touch arc plan 2b,
// docs/superpowers/specs/2026-11-22-moving-by-touch-design.md, mockup
// docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html). The Room.Info
// shapes below are plan 2a's (modules/gmcp/gmcp.Room.go).
//
// Run:  node tools/jstest/touchdeck-moves.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var ROOT = path.join(__dirname, '..', '..');
var TD = require(path.join(ROOT, '_datafiles', 'html', 'public', 'static', 'js', 'touchdeck.js'));

var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + g + ', want ' + w);
}

var MOVES = { north: 'm1', south: 'm2', east: 'm3', west: 'm4', up: 'm5', down: 'm6' };
function ex(kind, id, details, extra) {
    var e = { num: 1, dx: 0, dy: 0, dz: 0, details: details || [], kind: kind, id: id };
    if (extra) Object.keys(extra).forEach(function (k) { e[k] = extra[k]; });
    return e;
}
function info(num, v2, extra) {
    var r = { num: num, dark: false, exitsv2: v2, moves: MOVES, darkmarks: {} };
    if (extra) Object.keys(extra).forEach(function (k) { r[k] = extra[k]; });
    return r;
}

// Mockup frames 1 and 2: Tower Top, west plus two named exits.
var tower = info(5243, {
    west: ex('compass', 'm4'),
    shrine: ex('named', 'xa1b2c3d4'),
    ledge: ex('named', 'x0f0f0f0f')
});
// Frame 3: diagonals only.
var junction = info(6408, { southwest: ex('diagonal', 'x22222222'), northeast: ex('diagonal', 'x11111111') });
// Frame 4: down locked now.
var barracks = info(473, { down: ex('vertical', 'm6', ['locked']), south: ex('compass', 'm2'), up: ex('vertical', 'm5') });
// Frame 5: pitch dark after a bump north and a locked door felt west.
var dark = info(6418, {}, { dark: true, darkmarks: { north: 'wall', west: 'locked' } });
// Ruling 18 and R2: the viewer (43) may enter a party member's portal
// (arena-42) and their own (arena-43, marked own_portal); both read
// "arena". A portal they may not enter (tower-7) comes with no id.
var portals = info(900, {
    'arena-42': ex('temporary', 'x42424242', [], { label: 'arena' }),
    'arena-43': ex('temporary', 'x43434343', ['own_portal'], { label: 'arena' }),
    'tower-7': ex('temporary', '', [], { label: 'tower' }),
    plank: ex('temporary', 'x99999999')
});
// The Planar Oasis threshold: a temporary exit named north carries m1.
var oasis = info(901, { north: ex('temporary', 'm1') });

// --- moves: the model ------------------------------------------------------

check('no Room.Info yet: not ready', TD.moves(null, '').ready, false);
var m = TD.moves(tower, 'exploration');
check('Tower Top: only west is lit on the T',
    TD.T_DIRS.filter(function (d) { return m.t[d].lit; }), ['west']);
check('every T direction carries its fixed id', TD.T_DIRS.map(function (d) { return m.t[d].id; }),
    ['m5', 'm1', 'm6', 'm4', 'm2', 'm3']);
check('other exits A to Z by label (spec: named and temporary A to Z)',
    m.others.map(function (o) { return o.label; }), ['ledge', 'shrine']);
check('an other exit keeps its id and the name the server runs', m.others[1], { name: 'shrine', id: 'xa1b2c3d4', label: 'shrine', arrow: '', locked: false, own: false });
check('out of combat', m.combat, false);
check('combat comes from Commands.State', TD.moves(tower, 'combat').combat, true);
check('dead does not dim', TD.moves(tower, 'dead').combat, false);

var j = TD.moves(junction, '');
check('diagonals clockwise from northeast, with an arrow and their letters',
    j.others.map(function (o) { return o.arrow + o.label; }), ['\u2197NE', '\u2199SW']);

var b = TD.moves(barracks, '');
check('a door locked now carries the padlock', b.t.down.locked, true);
check('an open way does not', b.t.up.locked, false);

var d = TD.moves(dark, '');
check('dark: nothing is lit', TD.T_DIRS.some(function (k) { return d.t[k].lit; }), false);
check('dark: the marks ride on their directions', [d.t.north.mark, d.t.west.mark, d.t.east.mark], ['wall', 'locked', '']);
check('dark: no other exits', d.others.length, 0);
check('dark: every T id is still there (all six tappable)', TD.T_DIRS.every(function (k) { return d.t[k].id !== ''; }), true);
check('lit: no marks even if a stale one came', TD.moves(info(1, {}, { darkmarks: { north: 'wall' } }), '').t.north.mark, '');

var p = TD.moves(portals, '');
check('portals show their zone, the player\'s own first; one with no id gets no button',
    p.others.map(function (o) { return o.label + '=' + o.name; }), ['arena=arena-43', 'arena=arena-42', 'plank=plank']);
check('a temporary exit named north sits on the T, not in the list',
    [TD.moves(oasis, '').t.north.lit, TD.moves(oasis, '').others.length], [true, 0]);

// The server sorts known exits the same way (rooms.exitOrder); Room.Info's
// exitsv2 is a JSON object and loses it, so the client sorts again. Fails
// if the two orders part.
var goSrc = fs.readFileSync(path.join(ROOT, 'internal', 'rooms', 'known_exits.go'), 'utf8');
var order = /var exitOrder = map\[string\]int\{([\s\S]*?)\}/.exec(goSrc);
var ranks = {};
(order ? order[1] : '').replace(/"(\w+)":\s*(\d+)/g, function (_, k, v) { ranks[k] = Number(v); });
var serverDiagonals = Object.keys(ranks).filter(function (k) { return TD.DIAGONALS.indexOf(k) !== -1 || /^(north|south)(east|west)$/.test(k); })
    .sort(function (a, b) { return ranks[a] - ranks[b]; });
check('the client\'s diagonal order is the server\'s exitOrder', TD.DIAGONALS, serverDiagonals);

process.exit(failures ? 1 : 0);
```

Append to `internal/guards/exit_name_length_guard_test.go`:

```go
// The web client's copy of the limit (touchdeck.js NAME_LETTERS, which
// touch-targets.test.js uses to check a name this long fits a deck button)
// is exit.MaxNameLetters.
func TestTouchDeckNameLettersIsMaxNameLetters(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "html", "public", "static", "js", "touchdeck.js"))
	require.NoError(t, err)
	want := fmt.Sprintf("var NAME_LETTERS = %d;", exit.MaxNameLetters)
	require.Contains(t, string(data), want, "touchdeck.js must hold %q", want)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node tools/jstest/touchdeck-moves.test.js` then `go test ./internal/guards/ -run TestTouchDeckNameLetters -count=1`
Expected: the node run throws `TD.moves is not a function`; the Go test fails, `touchdeck.js must hold "var NAME_LETTERS = 6;"`.

- [ ] **Step 3: Implement**

In `touchdeck.js`, directly before `    return {` (`:134`), add:

```js
    // ---- Moving by touch (plan 2b, #781) ---------------------------------
    // Spec docs/superpowers/specs/2026-11-22-moving-by-touch-design.md;
    // mockup docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html. The
    // server builds every button (Room.Info exitsv2, moves, darkmarks and
    // dark, plan 2a); the client lays them out and sends back only an id
    // (Char.Button.Press).

    function own(o, k) { return !!o && Object.prototype.hasOwnProperty.call(o, k); }

    // The inverted T: each direction's row and column in the T block.
    var T_DIRS = ['up', 'north', 'down', 'west', 'south', 'east'];
    var T_PLACE = { up: [0, 0], north: [0, 1], down: [0, 2], west: [1, 0], south: [1, 1], east: [1, 2] };

    // Ruling 10: an arrow with its letter for compass and diagonal exits;
    // up and down draw the stairs pictogram (ICONS).
    var ARROWS = {
        north: ['\u2191', 'N'], south: ['\u2193', 'S'], east: ['\u2192', 'E'], west: ['\u2190', 'W'],
        northeast: ['\u2197', 'NE'], southeast: ['\u2198', 'SE'],
        southwest: ['\u2199', 'SW'], northwest: ['\u2196', 'NW']
    };
    var STAIRS = { up: 'stairs-up', down: 'stairs-down' };

    // Other exits come in a fixed order: the diagonals clockwise from
    // northeast, then every other exit A to Z by its label. The server sorts
    // the same way (rooms.exitOrder, internal/rooms/known_exits.go), but
    // Room.Info sends exitsv2 as a JSON object, which keeps no order, so the
    // client sorts again; touchdeck-moves.test.js fails if the two part.
    var DIAGONALS = ['northeast', 'southeast', 'southwest', 'northwest'];

    // The longest exit name, exit.MaxNameLetters (owner ruling 18);
    // internal/guards checks the two agree, and touch-targets.test.js that a
    // name this long fits a button at both sizes.
    var NAME_LETTERS = 6;

    function hasDetail(e, d) { return !!(e && e.details && e.details.indexOf(d) !== -1); }

    function diagonalRank(name) {
        var i = DIAGONALS.indexOf(name);
        return i < 0 ? DIAGONALS.length : i;
    }
    function otherCompare(a, b) {
        var r = diagonalRank(a.name) - diagonalRank(b.name);
        if (r) return r;
        var la = a.label.toLowerCase(), lb = b.label.toLowerCase();
        if (la !== lb) return la < lb ? -1 : 1;
        // Two portals to one zone share a label: the player's own first
        // (plan 2b D3), then by key.
        if (a.own !== b.own) return a.own ? -1 : 1;
        return a.name < b.name ? -1 : (a.name > b.name ? 1 : 0);
    }

    // The deck's view of one Room.Info (null before the first). mode:
    // Commands.State's mode ('exploration', 'combat', 'dead').
    //   ready   a Room.Info has come
    //   dark    the player sees nothing here: all six T buttons tappable
    //   combat  movement dims (the server refuses it anyway)
    //   t       per T direction: id (its fixed id), lit (an exit you know
    //           that way), locked (locked now), mark ('wall' or 'locked',
    //           in the dark only)
    //   others  the other exits that have a button, in order: name (what
    //           the server runs), id, label, arrow (diagonals), locked,
    //           own (the player's own instance portal, detail own_portal)
    function moves(info, mode) {
        var m = { ready: !!info, dark: !!(info && info.dark), combat: mode === 'combat', t: {}, others: [] };
        var v2 = (info && info.exitsv2) || {}, ids = (info && info.moves) || {}, marks = (info && info.darkmarks) || {};
        T_DIRS.forEach(function (dir) {
            var e = own(v2, dir) ? v2[dir] : null;
            m.t[dir] = {
                id: own(ids, dir) ? ids[dir] : '',
                lit: !m.dark && !!e,
                locked: !m.dark && hasDetail(e, 'locked'),
                mark: m.dark && own(marks, dir) ? marks[dir] : ''
            };
        });
        if (m.dark) return m;
        Object.keys(v2).forEach(function (name) {
            var e = v2[name];
            if (own(T_PLACE, name) || !e || !e.id) return;
            var a = own(ARROWS, name) ? ARROWS[name] : null;
            m.others.push({
                name: name,
                id: e.id,
                label: a ? a[1] : (e.label || name),
                arrow: a ? a[0] : '',
                locked: hasDetail(e, 'locked'),
                own: hasDetail(e, 'own_portal')
            });
        });
        m.others.sort(otherCompare);
        return m;
    }

```

In the returned object add, after `rowFade: rowFade, scrollLeftToShow: scrollLeftToShow,`:

```js
        moves: moves, T_DIRS: T_DIRS, DIAGONALS: DIAGONALS, NAME_LETTERS: NAME_LETTERS,
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `node tools/jstest/touchdeck-moves.test.js && node tools/jstest/touchdeck.test.js && go test . ./internal/guards/ -count=1`
Expected: every line `ok`, exit 0; Go `ok`.

- [ ] **Step 5: Prove the tests can fail**

Swap `'southwest', 'northwest'` in `DIAGONALS`: the order guard fails (the junction check does not: its fixture has only northeast and southwest). Restore. Change `var NAME_LETTERS = 6;` to `= 7;`: the Go guard fails. Restore.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/html/public/static/js/touchdeck.js tools/jstest/touchdeck-moves.test.js internal/guards/exit_name_length_guard_test.go
git commit -F - <<'EOF'
feat(webclient): the deck's movement model from Room.Info (#781)

Touch arc plan 2b. TouchDeck.moves turns one Room.Info and the
Commands.State mode into the T (fixed ids, lit, locked now, dark marks)
and the other exits in their fixed order. A jstest pins the diagonal
order to the server's exitOrder; a guard pins NAME_LETTERS to
exit.MaxNameLetters.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: `touchdeck.js`: cells, level and tap

**Files:**
- Modify: `_datafiles/html/public/static/js/touchdeck.js`
- Modify: `tools/jstest/touchdeck-moves.test.js`

- [ ] **Step 1: Write the failing tests**

In `tools/jstest/touchdeck-moves.test.js`, directly before `process.exit(`, add:

```js
// --- cells: the deck laid out ---------------------------------------------

var PHONE = TD.shape(344, 600, true, false);      // 2 x 6 at 48px (S21)
var SIDE = TD.shape(0, 300, false, true);         // 4 x 3: a phone on its side
var TABLET = TD.shape(788, 1000, false, false);   // 2 x 13
function kinds(cs) { return cs.map(function (c) { return c.kind === 'move' ? c.dir : c.kind; }); }

var top = TD.cells(TD.moves(tower, ''), PHONE, 'top', 0);
check('frame 1: the T on the left, the exits button beside it, the rest empty', kinds(top),
    ['up', 'north', 'down', 'other', 'empty', 'empty', 'west', 'south', 'east', 'empty', 'empty', 'empty']);
check('frame 1: exits is live (two named exits)', top[3].lit, true);
check('up and down draw the stairs; compass shows an arrow and its letter',
    [top[0].icon, top[1].arrow + top[1].label, top[2].icon], ['stairs-up', '\u2191N', 'stairs-down']);
check('frame 4: exits dims when the room has none', TD.cells(TD.moves(barracks, ''), PHONE, 'top', 0)[3].lit, false);
check('frame 6: combat reaches every movement cell',
    TD.cells(TD.moves(tower, 'combat'), PHONE, 'top', 0).filter(function (c) { return c.combat; }).length, 7);
check('a wider deck keeps the T and the exits slot where they are',
    kinds(TD.cells(TD.moves(tower, ''), TABLET, 'top', 0)).slice(0, 4), ['up', 'north', 'down', 'other']);
check('frame 9: on its side, the exits button sits under the T',
    kinds(TD.cells(TD.moves(tower, ''), SIDE, 'top', 0)).slice(0, 7), ['up', 'north', 'down', 'west', 'south', 'east', 'other']);

var dk = TD.cells(TD.moves(dark, ''), PHONE, 'top', 0);
check('frame 5: dark: the exits slot is the Dark tile', dk[3].kind, 'dark');
check('frame 5: dark: every T cell is dark and keeps its id',
    dk.filter(function (c) { return c.kind === 'move' && c.dark && c.id; }).length, 6);
check('frame 5: the wall mark north, the padlock mark west', [dk[1].mark, dk[6].mark], ['wall', 'locked']);
check('dark: the list never opens', TD.cells(TD.moves(dark, ''), PHONE, 'exits', 0)[3].kind, 'dark');

var list = TD.cells(TD.moves(tower, ''), PHONE, 'exits', 0);
check('frame 2: the list replaces the deck: Back top-left, then the exits', kinds(list).slice(0, 4), ['back', 'exit', 'exit', 'empty']);
check('frame 2: labels in order', [list[1].label, list[2].label], ['ledge', 'shrine']);
check('frame 2: no More when they fit', list.some(function (c) { return c.kind === 'more'; }), false);
check('frame 3: diagonals in the list keep arrow and letters',
    TD.cells(TD.moves(junction, ''), PHONE, 'exits', 0).slice(1, 3).map(function (c) { return c.arrow + c.label; }), ['\u2197NE', '\u2199SW']);
check('no room has more than two today, so an empty list falls back to the T',
    TD.cells(TD.moves(barracks, ''), PHONE, 'exits', 0)[0].kind, 'move');

var many = {};
for (var i = 0; i < 15; i++) many['p' + (i < 10 ? '0' : '') + i] = ex('named', 'x' + (10000000 + i));
var big = TD.moves(info(2, many), '');
var page0 = TD.cells(big, PHONE, 'exits', 0);
check('More bottom-right only when the list does not fit', page0[11].kind, 'more');
check('page 1 holds ten (twelve slots less Back and More)', page0.filter(function (c) { return c.kind === 'exit'; }).length, 10);
var page1 = TD.cells(big, PHONE, 'exits', 1);
check('page 2 holds the other five', page1.filter(function (c) { return c.kind === 'exit'; }).map(function (c) { return c.label; }),
    ['p10', 'p11', 'p12', 'p13', 'p14']);
check('More after the last page goes back to the first', TD.cells(big, PHONE, 'exits', 2)[1].label, 'p00');

// --- levelAfter: the deck returns to the T after a move ---------------------

var mt = TD.moves(tower, '');
check('after a move the deck returns to the T', TD.levelAfter('exits', 5243, 5346, TD.moves(info(5346, {}), '')), 'top');
check('the same room again keeps the list open', TD.levelAfter('exits', 5243, 5243, mt), 'exits');
check('the room going dark closes the list', TD.levelAfter('exits', 5243, 5243, TD.moves(info(5243, {}, { dark: true }), '')), 'top');
check('no other exits left closes the list', TD.levelAfter('exits', 5243, 5243, TD.moves(info(5243, { west: ex('compass', 'm4') }), '')), 'top');
check('the T stays the T', TD.levelAfter('top', 5243, 5243, mt), 'top');

// --- tap ------------------------------------------------------------------

var st = { level: 'top', page: 0, lastId: '', lastAt: 0 };
check('a lit arrow sends its fixed id', TD.tap(st, top[6], 1000).send, 'm4');
check('a dim arrow sends nothing', TD.tap(st, top[1], 1000).send, '');
check('a dark arrow sends its fixed id (all six tappable)', TD.tap(st, dk[1], 1000).send, 'm1');
check('a combat-dimmed arrow still sends (the server says why)',
    TD.tap(st, TD.cells(TD.moves(tower, 'combat'), PHONE, 'top', 0)[6], 1000).send, 'm4');
var r1 = TD.tap(st, top[6], 1000), r2 = TD.tap(r1, top[6], 1100);
check('ruling 8: a fast double tap on the T sends twice', [r1.send, r2.send], ['m4', 'm4']);
var opened = TD.tap(st, top[3], 1000);
check('the exits button opens the list at its first page', [opened.level, opened.page], ['exits', 0]);
check('a dim exits button does nothing', TD.tap(st, TD.cells(TD.moves(barracks, ''), PHONE, 'top', 0)[3], 1000).level, 'top');
var e1 = TD.tap(opened, list[2], 2000);
check('an exit sends its per-room id', e1.send, 'xa1b2c3d4');
check('a second tap on it within half a second is ignored', TD.tap(e1, list[2], 2000 + TD.DOUBLE_TAP_MS - 1).send, '');
check('after half a second it counts again', TD.tap(e1, list[2], 2000 + TD.DOUBLE_TAP_MS).send, 'xa1b2c3d4');
check('Back returns to the T', TD.tap(opened, list[0], 3000).level, 'top');
check('More turns the page', TD.tap({ level: 'exits', page: 0, lastId: '', lastAt: 0 }, page0[11], 1000).page, 1);
check('the Dark tile says what to do', TD.tap(st, dk[3], 1000).say, 'It is too dark to see the way. Light a torch or cast glow.');
check('an empty slot does nothing', TD.tap(st, top[4], 1000), { level: 'top', page: 0, lastId: '', lastAt: 0, send: '', say: '' });

```

- [ ] **Step 2: Run the test to verify it fails**

Run: `node tools/jstest/touchdeck-moves.test.js`
Expected: throws `TD.cells is not a function`.

- [ ] **Step 3: Implement**

In `touchdeck.js`, directly after the `moves` function (before `    return {`), add:

```js
    // A second tap on the same other-exit button within this long is
    // ignored (roadmap fat-finger rules). The six T buttons never ignore a
    // tap: a fast double tap walks twice, like typing (owner ruling 8).
    var DOUBLE_TAP_MS = 500;

    // The Dark tile's line (spec 2b, States).
    var DARK_LINE = 'It is too dark to see the way. Light a torch or cast glow.';

    // The deck's cells for one model, in slot order (row by row), for a
    // shape from shape(). level: 'top' (the T) or 'exits' (the other-exits
    // list, which replaces the whole deck: Back top-left, More bottom-right
    // only when the list does not fit). page: which page of the list.
    // kind: 'empty', 'move', 'other', 'dark', 'back', 'exit' or 'more'.
    function cells(model, s, level, page) {
        var n = s.rows * s.cols, out = [], i;
        for (i = 0; i < n; i++) out.push({ kind: 'empty' });
        if (!model || !model.ready) return out;
        if (level === 'exits' && !model.dark && model.others.length) {
            var more = model.others.length > n - 1;
            var per = more ? n - 2 : n - 1;
            var pages = Math.ceil(model.others.length / per);
            var p = (((page || 0) % pages) + pages) % pages;
            out[0] = { kind: 'back' };
            model.others.slice(p * per, p * per + per).forEach(function (o, k) {
                out[1 + k] = {
                    kind: 'exit', id: o.id, name: o.name, label: o.label, arrow: o.arrow,
                    locked: o.locked, own: o.own, combat: model.combat
                };
            });
            if (more) out[n - 1] = { kind: 'more' };
            return out;
        }
        T_DIRS.forEach(function (dir) {
            var t = model.t[dir], at = T_PLACE[dir], a = own(ARROWS, dir) ? ARROWS[dir] : null;
            out[at[0] * s.cols + at[1]] = {
                kind: 'move', dir: dir, id: t.id,
                arrow: a ? a[0] : '', label: a ? a[1] : '', icon: a ? '' : STAIRS[dir],
                lit: t.lit, locked: t.locked, mark: t.mark, dark: model.dark, combat: model.combat
            };
        });
        // The other-exits slot: beside the T on a two-row deck, under it in
        // a sideways phone's three-column side column (mockup frame 9).
        var slot = s.cols >= 4 ? 3 : 2 * s.cols;
        out[slot] = model.dark ? { kind: 'dark' }
            : { kind: 'other', lit: model.others.length > 0, combat: model.combat };
        return out;
    }

    // The level after a Room.Info: the T after a move (another room), in
    // the dark, or once the room has no other exits; otherwise as it was,
    // so a door unlocking while the list is open keeps it open.
    function levelAfter(level, lastNum, num, model) {
        if (level !== 'exits' || lastNum !== num) return 'top';
        if (!model.ready || model.dark || !model.others.length) return 'top';
        return 'exits';
    }

    // What a tap on cell c does. st: { level, page, lastId, lastAt }; now in
    // ms. Returns the next state with send (an id for Char.Button.Press, or
    // '') and say (a line for the game window, or '').
    function tap(st, c, now) {
        var r = { level: st.level, page: st.page, lastId: st.lastId, lastAt: st.lastAt, send: '', say: '' };
        if (!c) return r;
        switch (c.kind) {
        case 'move':
            if ((c.lit || c.dark) && c.id) r.send = c.id;
            break;
        case 'exit':
            if (!c.id || (c.id === st.lastId && now - st.lastAt < DOUBLE_TAP_MS)) break;
            r.send = c.id;
            r.lastId = c.id;
            r.lastAt = now;
            break;
        case 'other':
            if (c.lit) { r.level = 'exits'; r.page = 0; }
            break;
        case 'back':
            r.level = 'top';
            r.page = 0;
            break;
        case 'more':
            r.page = (st.page || 0) + 1;
            break;
        case 'dark':
            r.say = DARK_LINE;
            break;
        }
        return r;
    }

```

In the returned object, extend the line added in Task 2 to:

```js
        moves: moves, T_DIRS: T_DIRS, DIAGONALS: DIAGONALS, NAME_LETTERS: NAME_LETTERS,
        cells: cells, levelAfter: levelAfter, tap: tap, DOUBLE_TAP_MS: DOUBLE_TAP_MS, DARK_LINE: DARK_LINE,
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `node tools/jstest/touchdeck-moves.test.js && node tools/jstest/touchdeck.test.js && node tools/jstest/touch-targets.test.js`
Expected: every line `ok`, exit 0.

- [ ] **Step 5: Prove the tests can fail**

In `tap`, change `if (!c.id || (c.id === st.lastId && now - st.lastAt < DOUBLE_TAP_MS)) break;` to `if (!c.id) break;`: "a second tap on it within half a second is ignored" fails. Restore. (The review fix after Task 4 replaced `lastId`/`lastAt` with the `guardUntil` hold of D11; `touchdeck.js` `tap` and its tests in `touchdeck-moves.test.js` are the current text.) In `cells` change `var slot = s.cols >= 4 ? 3 : 2 * s.cols;` to `var slot = 3;`: the sideways check fails. Restore.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/html/public/static/js/touchdeck.js tools/jstest/touchdeck-moves.test.js
git commit -F - <<'EOF'
feat(webclient): the deck's cells, levels and taps (#781)

Touch arc plan 2b. TouchDeck.cells lays the model out in the deck's
shape: the inverted T and the exits slot, the Dark tile, and the
other-exits level with Back top-left and More bottom-right only when it
does not fit. levelAfter returns to the T after a move; tap sends a
button's id, ignores a quick second tap on an other exit (the T never,
ruling 8) and gives the Dark tile's line.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: `touchdeck.js`: drawing the buttons

**Files:**
- Modify: `_datafiles/html/public/static/js/touchdeck.js`
- Modify: `tools/jstest/touchdeck-moves.test.js`

- [ ] **Step 1: Write the failing tests**

In `tools/jstest/touchdeck-moves.test.js`, directly before `process.exit(`, add:

```js
// --- render: the buttons drawn --------------------------------------------

function FakeEl(tag, ns) {
    this.tagName = tag; this.ns = ns || ''; this.children = []; this.attrs = {};
    this.className = ''; this.textContent = ''; this.disabled = false; this.type = '';
}
FakeEl.prototype.appendChild = function (c) { this.children.push(c); return c; };
FakeEl.prototype.removeChild = function (c) { this.children.splice(this.children.indexOf(c), 1); return c; };
Object.defineProperty(FakeEl.prototype, 'firstChild', { get: function () { return this.children[0] || null; } });
FakeEl.prototype.setAttribute = function (k, v) { this.attrs[k] = String(v); };
var doc = {
    createElement: function (t) { return new FakeEl(t); },
    createElementNS: function (ns, t) { return new FakeEl(t, ns); }
};
function words(el) { return el.textContent + el.children.map(words).join(''); }
function has(el, cls) { return (' ' + el.className + ' ').indexOf(' ' + cls + ' ') !== -1; }
function svgs(el) { return el.children.filter(function (c) { return c.tagName === 'svg'; }).map(function (c) { return c.attrs['class']; }); }

var deck = new FakeEl('div');
deck.appendChild(new FakeEl('span'));            // the old empty slots go
TD.render(deck, top, doc);
check('one child per cell', deck.children.length, 12);
check('an empty cell is a slot, not a button', [deck.children[4].tagName, deck.children[4].className], ['span', 'deck-slot']);
var west = deck.children[6], north = deck.children[1], up = deck.children[0], other = deck.children[3];
check('a lit arrow is a button with its cell index', [west.tagName, west.type, west.attrs['data-cell'], west.disabled], ['button', 'button', '6', false]);
check('it shows the arrow and its letter', words(west), '\u2190W');
check('it reads plainly to a screen reader', west.attrs['aria-label'], 'Go west');
check('a movement button is square-edged by class', has(west, 'deck-move'), true);
check('R1: lit, dim, exits buttons wear the site brass', [has(west, 'brass'), has(north, 'brass'), has(other, 'brass')], [true, true, true]);
check('a dim arrow is disabled and says so', [north.disabled, has(north, 'deck-dim'), north.attrs['aria-label']], [true, true, 'No way north']);
check('up draws the stairs pictogram, not a letter', [svgs(up), words(up)], [['deck-icon'], '']);
check('the exits button: dashed by class, live, its words', [has(other, 'deck-other'), other.disabled, words(other)], [true, false, '\u22efexits']);

TD.render(deck, TD.cells(TD.moves(barracks, ''), PHONE, 'top', 0), doc);
var down = deck.children[2];
check('frame 4: a locked door carries the padlock', svgs(down), ['deck-icon', 'deck-lock']);
check('frame 4: and says it is locked', down.attrs['aria-label'], 'Go down (locked)');
check('frame 4: no other exits: the exits button is dim and disabled', [deck.children[3].disabled, has(deck.children[3], 'deck-dim')], [true, true]);

TD.render(deck, dk, doc);
check('frame 5: dark arrows are tappable in the dark style',
    [deck.children[7].disabled, has(deck.children[7], 'deck-blind')], [false, true]);
check('frame 5: the wall mark is a class (drawn by CSS)', has(deck.children[1], 'deck-mark-wall'), true);
check('frame 5: a locked door felt in the dark carries the padlock', svgs(deck.children[6]), ['deck-lock']);
check('frame 5: the Dark tile: closed eye and its word', [svgs(deck.children[3]), words(deck.children[3])], [['deck-icon'], 'Dark']);
check('R1: the pitch-dark buttons are olive grey, not brass', [has(deck.children[7], 'brass'), has(deck.children[3], 'brass')], [false, false]);

TD.render(deck, TD.cells(TD.moves(tower, 'combat'), PHONE, 'top', 0), doc);
check('frame 6: combat dims by class and keeps the button live',
    [has(deck.children[6], 'deck-combat'), deck.children[6].disabled], [true, false]);

TD.render(deck, list, doc);
check('frame 2: Back and the names', [words(deck.children[0]), words(deck.children[1]), words(deck.children[2])], ['\u2039 Back', 'ledge', 'shrine']);
check('frame 2: a name is a deck-name span (12px in CSS)', deck.children[1].children[0].className, 'deck-name');
check('an exit name reaches the page as text, never markup', (function () {
    var evil = TD.cells(TD.moves(info(3, { '<b>x': ex('named', 'x12345678') }), ''), PHONE, 'exits', 0);
    TD.render(deck, evil, doc);
    return deck.children[1].children[0].textContent;
}()), '<b>x');
TD.render(deck, page0, doc);
check('More reads More', words(deck.children[11]), 'More \u203a');
check('Back and More wear the brass too', [has(deck.children[0], 'brass'), has(deck.children[11], 'brass')], [true, true]);
TD.render(deck, TD.cells(TD.moves(portals, ''), PHONE, 'exits', 0), doc);
check('two portals to one zone: the same label, the player\'s own first and named so for a screen reader',
    [words(deck.children[1]), deck.children[1].attrs['aria-label'], words(deck.children[2]), deck.children[2].attrs['aria-label']],
    ['arena', 'Go arena (yours)', 'arena', 'Go arena']);

```

- [ ] **Step 2: Run the test to verify it fails**

Run: `node tools/jstest/touchdeck-moves.test.js`
Expected: throws `TD.render is not a function`.

- [ ] **Step 3: Implement**

In `touchdeck.js`, directly after the `tap` function, add:

```js
    var SVG_NS = 'http://www.w3.org/2000/svg';

    // Line icons as stroke paths, like the tab bar's (dashboard.js
    // PANEL_TABS), never emoji (brand). Drawn from the approved mockup: the
    // ISO 7001 style stairs (ruling 10), the padlock (ruling 3) and the
    // closed eye of the Dark tile (ruling 11).
    var ICONS = {
        'stairs-up': { box: 26, paths: ['M3 22h5v-5h5v-5h5V7h5', 'M16 4h6v6'] },
        'stairs-down': { box: 26, paths: ['M3 7h5v5h5v5h5v5h5', 'M22 16v6h-6'] },
        lock: { box: 12, paths: ['M3 5.5h6a1 1 0 0 1 1 1v3.5a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V6.5a1 1 0 0 1 1-1z',
                                 'M4 5.5V4a2 2 0 0 1 4 0v1.5'] },
        eye: { box: 26, paths: ['M3 12c3 4 7 6 10 6s7-2 10-6', 'M6 16l-2 3M10 18l-1 3.5M16 18l1 3.5M20 16l2 3'] }
    };

    function svgIcon(doc, name, cls) {
        var d = ICONS[name], svg = doc.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('viewBox', '0 0 ' + d.box + ' ' + d.box);
        svg.setAttribute('class', cls);
        svg.setAttribute('aria-hidden', 'true');
        svg.setAttribute('focusable', 'false');
        d.paths.forEach(function (p) {
            var path = doc.createElementNS(SVG_NS, 'path');
            path.setAttribute('d', p);
            svg.appendChild(path);
        });
        return svg;
    }

    function textEl(doc, cls, text) {
        var e = doc.createElement('span');
        e.className = cls;
        e.textContent = text;
        return e;
    }

    // Kinds are told apart by shape, edge and marks (CSS), never by colour
    // alone. Every button but the pitch-dark ones wears the site's brass
    // (.brass, gomud.css; owner ruling R1 on plan 2b).
    function classOf(c) {
        var k = ['deck-btn'];
        if (!c.dark && c.kind !== 'dark') k.push('brass');
        if (c.kind === 'move') {
            k.push('deck-move');
            if (c.dark) k.push('deck-blind'); else if (!c.lit) k.push('deck-dim');
            if (c.mark === 'wall') k.push('deck-mark-wall');
            if (c.combat && !c.dark) k.push('deck-combat');
        } else if (c.kind === 'other') {
            k.push('deck-other');
            if (!c.lit) k.push('deck-dim');
            if (c.combat) k.push('deck-combat');
        } else if (c.kind === 'exit') {
            k.push('deck-exit');
            if (c.combat) k.push('deck-combat');
        } else if (c.kind === 'dark') {
            k.push('deck-darktile');
        } else if (c.kind === 'back') {
            k.push('deck-back');
        } else if (c.kind === 'more') {
            k.push('deck-more');
        }
        return k.join(' ');
    }

    // A screen reader's words for a button, plain (dogmud-player-copy).
    function ariaOf(c) {
        switch (c.kind) {
        case 'move':
            if (!c.dark && !c.lit) return 'No way ' + c.dir;
            if (c.mark === 'wall') return 'Go ' + c.dir + ' (you hit a wall there)';
            if (c.locked || c.mark === 'locked') return 'Go ' + c.dir + ' (locked)';
            return 'Go ' + c.dir;
        case 'other': return c.lit ? 'Other exits' : 'No other exits';
        case 'exit': return 'Go ' + (c.arrow ? c.name : c.label) + (c.own ? ' (yours)' : '') + (c.locked ? ' (locked)' : '');
        case 'dark': return 'Too dark to see the exits';
        case 'back': return 'Back to the directions';
        case 'more': return 'More exits';
        }
        return '';
    }

    // A lit room's arrow with no exit that way, and the exits button with
    // no other exits, take no tap. In the dark all six arrows do (ruling 4).
    function disabledOf(c) {
        return (c.kind === 'move' && !c.lit && !c.dark) || (c.kind === 'other' && !c.lit);
    }

    // Draws cells into the deck element, replacing what it held. Text goes
    // in as textContent, never as markup (an exit name is server data). Each
    // button carries its cell's index in data-cell for the click handler
    // (dashboard.js).
    function render(deck, list, doc) {
        while (deck.firstChild) deck.removeChild(deck.firstChild);
        list.forEach(function (c, i) {
            if (c.kind === 'empty') {
                deck.appendChild(textEl(doc, 'deck-slot', ''));
                return;
            }
            var b = doc.createElement('button');
            b.type = 'button';
            b.className = classOf(c);
            b.setAttribute('data-cell', String(i));
            b.setAttribute('aria-label', ariaOf(c));
            if (disabledOf(c)) b.disabled = true;
            if (c.icon) b.appendChild(svgIcon(doc, c.icon, 'deck-icon'));
            if (c.arrow) {
                b.appendChild(textEl(doc, 'deck-arrow', c.arrow));
                b.appendChild(textEl(doc, 'deck-letter', c.label));
            } else if (c.kind === 'exit') {
                b.appendChild(textEl(doc, 'deck-name', c.label));
            }
            if (c.kind === 'other') {
                b.appendChild(textEl(doc, 'deck-name', '\u22ef'));
                b.appendChild(textEl(doc, 'deck-name', 'exits'));
            } else if (c.kind === 'dark') {
                b.appendChild(svgIcon(doc, 'eye', 'deck-icon'));
                b.appendChild(textEl(doc, 'deck-name', 'Dark'));
            } else if (c.kind === 'back') {
                b.appendChild(textEl(doc, 'deck-name', '\u2039 Back'));
            } else if (c.kind === 'more') {
                b.appendChild(textEl(doc, 'deck-name', 'More \u203a'));
            }
            if (c.locked || c.mark === 'locked') b.appendChild(svgIcon(doc, 'lock', 'deck-lock'));
            deck.appendChild(b);
        });
    }

```

In the returned object, after the line `cells: cells, ...` add:

```js
        render: render,
```

Also update the file's header comment: replace

```js
// (dashboard.js). Buttons only ever come from the server (the roadmap's sync
// rule), so in plan 1a the deck draws empty slots.
```

with

```js
// (dashboard.js). Buttons only ever come from the server (the roadmap's sync
// rule): plan 2b draws the movement buttons from Room.Info and sends back
// only their ids (Char.Button.Press); the command slots stay empty until
// spec 3.
```

and after `//   rowFade / scrollLeftToShow  a phone's sideways-scrolling sub-tab row (plan 1c)` add:

```js
//   moves / cells / levelAfter / tap / render  moving by touch (plan 2b)
```

and change `// Tested by tools/jstest/touchdeck.test.js and touch-targets.test.js.` to `// Tested by tools/jstest/touchdeck.test.js, touchdeck-moves.test.js and touch-targets.test.js.`

- [ ] **Step 4: Run the tests to verify they pass**

Run: `node tools/jstest/touchdeck-moves.test.js && node tools/jstest/touchdeck.test.js`
Expected: every line `ok`, exit 0.

- [ ] **Step 5: Prove the tests can fail**

In `render` change `e.textContent = text;` in `textEl` to `e.textContent = text.replace('<', '');`: "an exit name reaches the page as text" fails. Restore. In `disabledOf` drop `&& !c.dark`: "frame 5: dark arrows are tappable" fails. Restore.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/html/public/static/js/touchdeck.js tools/jstest/touchdeck-moves.test.js
git commit -F - <<'EOF'
feat(webclient): the deck draws its movement buttons (#781)

Touch arc plan 2b. TouchDeck.render draws the cells as buttons: arrows
with their letters, the stairs pictogram, the padlock, the wall mark
class, the Dark tile's closed eye, Back and More, with plain screen
reader labels. Names go in as text, never markup.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: The `--deck-label` type token (ruling 18)

**Files:**
- Modify: `internal/guards/client_type_guard_test.go` (`:68`, `:73`, `:122-152`, `:203-228`)
- Modify: `_datafiles/html/public/static/css/gomud.css` (`:166-171`)

- [ ] **Step 1: Write the failing tests**

In `internal/guards/client_type_guard_test.go`:

- In `TestClientTypeRulesCanFail`, in the map of failing probes add `` `  font-size: var(--deck-labels);`: "font-size var(--deck-labels)", ``; in the list of passing probes add `` `font-size: var(--deck-label);`, ``; and extend the token probe line to

```go
	if !typeTokenDef.MatchString(`  --text: 12px;`) || !typeTokenDef.MatchString(`  --deck-label: 12px;`) || typeTokenDef.MatchString(`--text-primary: #fff;`) || typeTokenDef.MatchString(`--title-gold: #fff;`) {
```

- In `TestTypeTokensHoldTheRuledSizes`, after `"--foot-text: 16px;",` add `"--deck-label: 12px;",`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/guards/ -run "TestClientTypeRulesCanFail|TestTypeTokensHoldTheRuledSizes" -count=1`
Expected: FAIL: `probe "font-size: var(--deck-label);" should pass`, the token probe, and `gomud.css lost "--deck-label: 12px;"`.

- [ ] **Step 3: Implement**

`client_type_guard_test.go`:

```go
	typeSizeOK    = regexp.MustCompile(`^var\(--(text|title|cmd|nav-text|foot-text|deck-label)\)$`)
```

```go
	typeTokenDef  = regexp.MustCompile(`(^|[\s;{])--(text|title|cmd|nav-text|foot-text|deck-label|bold)\s*:`)
```

and in the file's opening comment replace `// The web client and the site use one font, two text sizes, no italics and one` / `// bold (spec 2026-11-09, rulings 4 to 8a).` with:

```go
// The web client and the site use one font, two text sizes, no italics and one
// bold (spec 2026-11-09, rulings 4 to 8a), plus 12px for the names on touch
// deck buttons (--deck-label, owner ruling 18, 2026-11-26).
```

`gomud.css`, after `  --foot-text: 16px;  /* the site's footer */` add:

```css
  --deck-label: 12px; /* names on touch deck buttons (owner ruling 18): six letters fit a 44px button */
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test . ./internal/guards/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Prove the tests can fail**

Change the token to `--deck-label: 13px;`: `TestTypeTokensHoldTheRuledSizes` fails. Restore.

- [ ] **Step 6: Commit**

```bash
git add internal/guards/client_type_guard_test.go _datafiles/html/public/static/css/gomud.css
git commit -F - <<'EOF'
feat(webclient): a 12px type token for deck button names (#781)

Owner ruling 18: an exit name fits a deck button at 12px. --deck-label
is the one token for it; the type guard admits and pins it.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: The deck's stylesheet, its contrast checks and the name-fit check

**Files:**
- Modify: `_datafiles/html/public/static/css/dashboard.css` (`:268-284`)
- Modify: `tools/jstest/webclient-colours.test.js`, `tools/jstest/touch-targets.test.js`

- [ ] **Step 1: Write the failing tests**

In `tools/jstest/webclient-colours.test.js`, directly before `console.log(failures ?`, add:

```js
// Moving by touch (plan 2b). Lit deck buttons wear the site brass (owner
// ruling R1): the client's even brass (--brass-grad's :root[data-client]
// override in dashboard.css), --brass-text and --brass-border, measured on
// both stops of the gradient (text and the padlock, drawn in currentColor,
// 4.5:1; the dashed and heavy edges 3:1, brand section 2). The pitch-dark
// colours are tokens on #touch-deck. Dimmed buttons are inactive controls,
// which WCAG exempts; the dim state still has to stand 3:1 apart from a lit
// button.
var clientBrass = /:root\[data-client\] \{ --brass-grad: linear-gradient\((#[0-9a-fA-F]{6}), (#[0-9a-fA-F]{6})\); \}/.exec(dash);
check('touch deck: the client brass override is read', !!clientBrass, 'no :root[data-client] --brass-grad override in dashboard.css');
var stops = clientBrass ? [clientBrass[1], clientBrass[2]] : [];
var deckBg = token(css, 'ink-deep');
stops.forEach(function (stop) {
    [['a label (and the padlock)', 'brass-text', 4.5], ['the dashed and heavy edges', 'brass-border', 3]].forEach(function (p) {
        var r = contrast(token(css, p[1]), stop);
        check('touch deck: ' + p[0] + ' on brass ' + stop + ' passes ' + p[2] + ':1', r >= p[2], token(css, p[1]) + ' = ' + r.toFixed(2));
    });
    check('touch deck: brass ' + stop + ' outlines a button on the deck', contrast(stop, deckBg) >= 3, contrast(stop, deckBg).toFixed(2));
});
// The dim state: opacity over the deck, read from the .deck-dim rule.
function over(fgc, bgc, a) {
    return '#' + [1, 3, 5].map(function (i) {
        var f = parseInt(fgc.slice(i, i + 2), 16), b = parseInt(bgc.slice(i, i + 2), 16);
        return ('0' + Math.round(f * a + b * (1 - a)).toString(16)).slice(-2);
    }).join('');
}
var dimA = Number((/\.deck-dim,\s*\.deck-combat \{ opacity: ([0-9.]+); \}/.exec(dash) || [])[1]);
check('touch deck: the dim opacity is read', dimA > 0 && dimA < 1, String(dimA));
if (stops.length === 2 && dimA > 0) {
    var litDark = stops[1], dimLight = over(stops[0], deckBg, dimA);
    check('touch deck: a lit button stands 3:1 apart from a dim one', contrast(litDark, dimLight) >= 3,
        litDark + ' against ' + dimLight + ' = ' + contrast(litDark, dimLight).toFixed(2));
}
[
    ['a dark label on its tile', 'deck-blind-text', 'deck-blind-bg', 4.5],
    ['a dark edge on its tile', 'deck-blind-edge', 'deck-blind-bg', 3],
    ['the wall mark on a dark tile', 'deck-bump', 'deck-blind-bg', 3],
    ['the Dark tile label', 'deck-darktile-text', 'deck-darktile-bg', 4.5],
    ['the Dark tile edge on the dark deck', 'deck-darktile-edge', 'deck-blind-bg', 3]
].forEach(function (p) {
    var fgc = token(dash, p[1]), bgc = token(dash, p[2]);
    var r = contrast(fgc, bgc);
    check('touch deck: ' + p[0] + ' passes ' + p[3] + ':1', r >= p[3], fgc + ' on ' + bgc + ' = ' + r.toFixed(2));
});
check('touch deck: the wall mark is the warning red, the only red', token(dash, 'deck-bump') === token(css, 'hp-low'), token(dash, 'deck-bump'));
```

In `tools/jstest/touch-targets.test.js`, directly before `process.exit(`, add:

```js
// Plan 2b: a deck button fills its grid cell and sets no size of its own.
var btnRule = (/\.deck-btn \{[^}]*\}/.exec(css) || [''])[0];
check('a deck button is styled', btnRule !== '');
check('a deck button sets no size of its own', btnRule !== '' && !/(^|[;{\s])(width|height|min-width|min-height)\s*:/.test(btnRule));

// Owner ruling 18: the longest exit name fits its button at both sizes.
// JetBrains Mono's letters are 0.6em wide; a named button has a 1px edge
// each side. Task 11 of plan 2b measures it in the browser too.
var gomud = fs.readFileSync(path.join(js, '..', 'css', 'gomud.css'), 'utf8');
var labelPx = Number((/--deck-label:\s*(\d+)px;/.exec(gomud) || [])[1]);
var tight = Number((/#touch-deck\.deck-small \.deck-name \{[^}]*letter-spacing:\s*(-?[\d.]+)px/.exec(css) || [])[1]);
var LETTER = 0.6 * labelPx, EDGES = 2;
check('the deck name size is read from --deck-label', labelPx > 0);
check('a ' + TD.NAME_LETTERS + '-letter name fits a ' + TD.BTN + 'px button', TD.NAME_LETTERS * LETTER <= TD.BTN - EDGES);
check('a ' + TD.NAME_LETTERS + '-letter name fits a ' + TD.SMALL_BTN + 'px button with the small spacing',
    !isNaN(tight) && TD.NAME_LETTERS * (LETTER + tight) <= TD.SMALL_BTN - EDGES);
check('the fit check can fail: seven letters do not fit at the small size', 7 * (LETTER + tight) > TD.SMALL_BTN - EDGES);
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node tools/jstest/webclient-colours.test.js; node tools/jstest/touch-targets.test.js`
Expected: the colours test prints `FAIL touch deck: the dim opacity is read` and throws `token not found: deck-blind-text`; touch-targets prints `FAIL a deck button is styled` and the small-size fit line, exits 1.

- [ ] **Step 3: Implement**

`dashboard.css`: replace the `#touch-deck` rule

```css
#touch-deck {
  display: grid;
  grid-template-columns: repeat(var(--deck-cols, 6), var(--deck-btn, 48px));
  grid-auto-rows: var(--deck-btn, 48px);
  gap: var(--deck-gap, 8px);
  padding: var(--deck-pad, 8px);
  justify-content: center;
  background: var(--ink-deep);
  border-top: 2px solid var(--gold-rule);
}
```

with

```css
#touch-deck {
  display: grid;
  grid-template-columns: repeat(var(--deck-cols, 6), var(--deck-btn, 48px));
  grid-auto-rows: var(--deck-btn, 48px);
  gap: var(--deck-gap, 8px);
  padding: var(--deck-pad, 8px);
  justify-content: center;
  background: var(--ink-deep);
  border-top: 2px solid var(--gold-rule);
  /* Moving by touch (plan 2b). Lit buttons are the site brass (.brass,
     owner ruling R1): --brass-text #1a1206 is 12.69 and 8.19 on the
     client's two brass stops, --brass-border 7.55 and 4.87, and the brass
     outlines a button on this deck at 10.57 and 6.82. The pitch-dark state
     keeps the ruled olive grey (spec ruling 11), its colours below, contrast
     measured with WCAG's formula; tools/jstest/webclient-colours.test.js
     checks every pair. */
  --deck-blind-bg: #19140c;      /* dark room: umber, the darkest brand colour */
  --deck-blind-edge: #7e8073;    /* olive grey: 4.55 on --deck-blind-bg (the mockup's #5d5c4e is 2.71) */
  --deck-blind-text: #8b897a;    /* olive grey: 5.19 on --deck-blind-bg */
  --deck-bump: #e35349;          /* the wall mark, warning red: 4.88 on --deck-blind-bg */
  --deck-darktile-bg: #252015;   /* the Dark tile */
  --deck-darktile-edge: #969889; /* 6.23 on --deck-blind-bg */
  --deck-darktile-text: #cec6aa; /* 9.48 on --deck-darktile-bg */
}
```

and replace the `.deck-slot` comment line `/* An empty slot until the server sends buttons (spec 2 onward). */` with `/* An empty slot: one the server sends no button for (the command slots until spec 3). */`. After the `.deck-slot { ... }` rule add:

```css
/* Moving by touch (touch arc plan 2b; spec 2026-11-22; mockup
   docs/roadmaps/touch-arc-mocks/s21-moving-by-touch.html). touchdeck.js
   render() draws the buttons; every lit one also wears the site's .brass
   (gomud.css: colour, brass, edge, pressed state; owner ruling R1). This
   file loads after gomud.css, so .deck-btn's display and the edges below
   win over .brass. A button fills its grid cell, so its size is the deck's
   (--deck-btn from touchdeck.js shape), never its own. Kinds are told apart
   by shape, edge and marks, never by colour alone: movement square with
   .brass's 1px edge, other exits a dashed edge, Back and More a heavy
   square edge. */
.deck-btn {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 0;
  border-radius: 6px;
  font-family: var(--font-mono);
  font-weight: 400;
  line-height: 1.1;
  cursor: pointer;
  touch-action: manipulation;  /* no double-tap zoom: two quick taps walk twice (ruling 8) */
}
.deck-btn:disabled { cursor: default; }
.deck-btn:focus-visible { outline: 2px solid var(--vellum); outline-offset: 1px; }  /* 9.70 on the deck */
/* The edges read on the brass: --brass-border 7.55 and 4.87 on its stops. */
.deck-other { border: 2px dashed var(--brass-border); }
/* No exit that way, or in a fight (refused anyway; Flee comes with plan
   3d). Inactive controls: WCAG asks no contrast of them; a lit button's
   darker stop stands 3.83 apart from a dim button's lighter one. */
.deck-dim,
.deck-combat { opacity: 0.22; }
.deck-blind {
  background: var(--deck-blind-bg);
  border: 1px dashed var(--deck-blind-edge);
  color: var(--deck-blind-text);
}
.deck-mark-wall::after {
  content: "\2715";
  position: absolute;
  top: 1px;
  right: 3px;
  font-size: var(--deck-label);
  line-height: 1;
  color: var(--deck-bump);
}
.deck-darktile {
  gap: 2px;
  background: var(--deck-darktile-bg);
  border: 1px solid var(--deck-darktile-edge);
  color: var(--deck-darktile-text);
}
.deck-back,
.deck-more {
  border: 2px solid var(--brass-border);
  border-radius: 2px;
}
.deck-arrow { font-size: var(--title); line-height: 1; }
.deck-letter,
.deck-name { font-size: var(--deck-label); line-height: 1.1; }
.deck-name { white-space: nowrap; }
/* At 44px six letters (43.2px at 12px) would touch the 1px edges. */
#touch-deck.deck-small .deck-name { letter-spacing: -0.5px; }
.deck-icon { width: 24px; height: 24px; fill: none; stroke: currentColor; stroke-width: 1.8; }
/* The padlock in the button's own text colour: 8.19 to 12.69 on brass,
   5.19 on a pitch-dark tile. */
.deck-lock { position: absolute; top: 2px; right: 2px; width: 12px; height: 12px; fill: none; stroke: currentColor; stroke-width: 1.3; }
#touch-deck.deck-dark { background: var(--deck-blind-bg); }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `node tools/jstest/webclient-colours.test.js && node tools/jstest/touch-targets.test.js && go test ./internal/guards/ -run TestClientAndSiteTypeUsesTheTokens -count=1`
Expected: every line `ok`; the type guard `ok` (every new `font-size` is a token, the weight is 400, the family `var(--font-mono)`).

- [ ] **Step 5: Prove the tests can fail**

Set `--deck-blind-edge: #5d5c4e;`: "a dark edge on its tile passes 3:1" fails with `= 2.71`. Restore. Delete the `letter-spacing` line: the small-size fit check fails. Restore.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/html/public/static/css/dashboard.css tools/jstest/webclient-colours.test.js tools/jstest/touch-targets.test.js
git commit -F - <<'EOF'
feat(webclient): the deck's movement button styles (#781)

Touch arc plan 2b. Every lit button is the site brass with dark text
(owner ruling R1): square movement buttons, the dashed exits button,
heavy-edged Back and More; the pitch-dark state keeps the ruled olive
grey with the wall mark and the closed-eye Dark tile. The brass comes
from its tokens; webclient-colours.test.js measures the label, padlock
and edges on both brass stops, the dim state against a lit button and
every dark colour. touch-targets.test.js checks a button sets no size of
its own and that a six-letter name fits at 48px and 44px.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Wiring: `dashboard.js`, the page, the static checks

**Files:**
- Modify: `_datafiles/html/public/static/js/dashboard.js` (`:169`, `:191`, `:671-679`, after `applyDeck`)
- Modify: `_datafiles/html/public/webclient-pure.html` (`:332`, `:884-888`, `:1207-1213`, `:2264-2265`, `:3430-3432`, `:3998-4004`)
- Modify: `tools/webclient-tests/touch-frame.js`
- Create: `tools/jstest/dashboard-deck.test.js`

- [ ] **Step 1: Write the failing tests**

Create `tools/jstest/dashboard-deck.test.js`:

```js
// dashboard-deck.test.js: dashboard.js keeps the deck's movement state and
// sends a tap's id (touch arc plan 2b, #781).
//
// Run:  node tools/jstest/dashboard-deck.test.js
//
// Loads the real static/js/dashboard.js and touchdeck.js. applyDeck is
// replaced by one that lays the cells out at the S21's shape, since drawing
// is touchdeck.js render's job (touchdeck-moves.test.js). Dependency free,
// exits non-zero on failure.
'use strict';

var fs = require('fs');
var path = require('path');
var vm = require('vm');

var JS = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js');
var TD = require(path.join(JS, 'touchdeck.js'));

var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + g + ', want ' + w);
}

var win = { TouchDeck: TD };
var sandbox = {
    window: win,
    document: {
        getElementById: function () { return null; },
        addEventListener: function () {},
        querySelectorAll: function () { return []; }
    },
    console: console
};
vm.createContext(sandbox);
vm.runInContext(fs.readFileSync(path.join(JS, 'dashboard.js'), 'utf8'), sandbox, { filename: 'dashboard.js' });
var D = win.Dashboard;

var SHAPE = TD.shape(344, 600, true, false);
D.applyDeck = function () {
    D._deckCells = TD.cells(TD.moves(D._moves, D._deckMode), SHAPE, D._deckState.level, D._deckState.page);
};
var pressed = [], said = [];
D.setDeckSender(function (id) { pressed.push(id); }, function (t) { said.push(t); });

var MOVES = { north: 'm1', south: 'm2', east: 'm3', west: 'm4', up: 'm5', down: 'm6' };
function room(num, v2, extra) {
    var r = { num: num, dark: false, exitsv2: v2, moves: MOVES, darkmarks: {} };
    if (extra) Object.keys(extra).forEach(function (k) { r[k] = extra[k]; });
    return r;
}
function e(kind, id) { return { num: 1, dx: 0, dy: 0, dz: 0, details: [], kind: kind, id: id }; }
function at(kind, pred) {
    return D._deckCells.findIndex(function (c) { return c.kind === kind && (!pred || pred(c)); });
}
var tower = room(5243, { west: e('compass', 'm4'), shrine: e('named', 'xaaaa0001'), ledge: e('named', 'xaaaa0002') });

D.setMoves(tower);
D._deckTap(at('move', function (c) { return c.dir === 'west'; }));
D._deckTap(at('move', function (c) { return c.dir === 'west'; }));
check('a lit arrow sends its fixed id, and a quick second tap sends again (ruling 8)', pressed, ['m4', 'm4']);
D._deckTap(at('other'));
check('the exits button opens the list', D._deckState.level, 'exits');
D._deckTap(at('exit', function (c) { return c.label === 'shrine'; }));
D._deckTap(at('exit', function (c) { return c.label === 'shrine'; }));
check('an exit sends its id once for a quick double tap', pressed.slice(2), ['xaaaa0001']);
D.setMoves(tower);
check('Room.Info again in the same room keeps the list open', D._deckState.level, 'exits');
D.setMoves(room(5346, { north: e('compass', 'm1') }));
check('after a move the deck returns to the T', D._deckState.level, 'top');
D.setDeckMode('combat');
check('combat reaches the cells', D._deckCells[1].combat, true);
D.setMoves(room(6418, {}, { dark: true, darkmarks: { north: 'wall' } }));
D._deckTap(at('dark'));
check('the Dark tile writes its line through the page', said, [TD.DARK_LINE]);
D.setMoves(null);
check('a closed connection leaves only empty slots', D._deckCells.every(function (c) { return c.kind === 'empty'; }), true);

process.exit(failures ? 1 : 0);
```

In `tools/webclient-tests/touch-frame.js` replace

```js
check(/<div id="touch-deck" aria-hidden="true"><\/div>/.test(html), 'the deck starts empty and hidden from screen readers');
```

with

```js
check(/<div id="touch-deck" role="group" aria-label="Movement"><\/div>/.test(html), 'the deck starts empty and is one labelled group of buttons (plan 2b)');
```

and directly before `console.log(fails === 0 ?` add:

```js
// Plan 2b: moving by touch.
const roomAt = html.indexOf('"Room": function() {');
const movesAt = html.indexOf('Dashboard.setMoves(obj.Info)');
const coordsAt = html.indexOf('var coords = obj.Info.coords.split(",");');
check(roomAt > 0 && movesAt > roomAt && movesAt < coordsAt, 'Room.Info reaches the deck before the map code runs');
check(/setDeckSender\(function \(id\) \{ return SendGMCP\("Char\.Button\.Press", \{ id: id \}\); \}, gameLine\)/.test(html), 'a deck button sends only its id, as Char.Button.Press');
check(/"Commands": function\(\) \{[^}]*syncDeckMode\(\);/.test(html) && /"Commands\.State": function\(\) \{\s*checkDeathClearsQueue\(\);\s*syncDeckMode\(\);/.test(html), 'Commands.State\'s mode reaches the deck');
check(/socket\.onclose = function\(event\) \{[\s\S]{0,300}if \(window\.Dashboard && Dashboard\.setMoves\) Dashboard\.setMoves\(null\);/.test(html), 'a closed connection clears the buttons');
check(/TD\.cells\(model, s, this\._deckState\.level, this\._deckState\.page\)/.test(dash) && /TD\.render\(deck, cells, document\)/.test(dash), 'applyDeck draws the cells touchdeck.js lays out');
check(/closest\("button\[data-cell\]"\)/.test(dash), 'one click handler serves every deck button');
check(!/cell\.className = "deck-slot"/.test(dash), 'dashboard.js draws no empty slots of its own');
check(/\.deck-btn \{[^}]*touch-action: manipulation;/.test(css), 'a quick second tap is a tap, not a zoom');
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node tools/jstest/dashboard-deck.test.js; node tools/webclient-tests/touch-frame.js`
Expected: the first throws `D.setDeckSender is not a function`; touch-frame prints FAIL for the deck tag and each plan 2b line except the CSS one, exits 1.

- [ ] **Step 3: Implement `dashboard.js`**

After `    _deck: null,        // #touch-deck, inside the slot` add:

```js
    // Moving by touch (plan 2b): the last Room.Info and Commands.State mode,
    // the deck's level and tap memory (touchdeck.js tap), the cells drawn,
    // and the page's senders (setDeckSender).
    _moves: null,
    _movesNum: null,
    _deckMode: "",
    _deckState: { level: "top", page: 0, guardUntil: 0 },
    _deckCells: [],
    _deckSig: "",
    _deckPress: null,
    _deckSay: null,
```

After `      this._deck = document.getElementById("touch-deck");` add:

```js
      // One click handler for every deck button (plan 2b). A disabled
      // button gets no click.
      if (this._deck) {
        var deckOwner = this;
        this._deck.addEventListener("click", function (e) {
          var b = e.target && e.target.closest ? e.target.closest("button[data-cell]") : null;
          if (b && !b.disabled) deckOwner._deckTap(Number(b.getAttribute("data-cell")));
        });
      }
```

In `applyDeck` replace

```js
      var want = s.rows * s.cols;
      if (deck.childElementCount !== want) {
        while (deck.firstChild) deck.removeChild(deck.firstChild);
        for (var i = 0; i < want; i++) {
          var cell = document.createElement("span");
          cell.className = "deck-slot";
          deck.appendChild(cell);
        }
      }
```

with

```js
      // The buttons (plan 2b): touchdeck.js lays the last Room.Info out in
      // this shape and draws it, again only when a cell changes.
      var model = TD.moves(this._moves, this._deckMode);
      var cells = TD.cells(model, s, this._deckState.level, this._deckState.page);
      deck.classList.toggle("deck-small", s.button < TD.BTN);
      deck.classList.toggle("deck-dark", model.dark);
      var sig = JSON.stringify(cells);
      if (sig !== this._deckSig || deck.childElementCount !== cells.length) {
        this._deckSig = sig;
        this._deckCells = cells;
        TD.render(deck, cells, document);
      }
```

After the closing `},` of `applyDeck` add:

```js
    // The page hands over how the deck sends (plan 2b): press(id) sends a
    // Char.Button.Press, say(text) writes one of the page's own lines.
    setDeckSender: function (press, say) {
      this._deckPress = press;
      this._deckSay = say;
    },

    // A Room.Info arrived, or null when the connection closed. After a move
    // the deck returns to the T (touchdeck.js levelAfter).
    setMoves: function (info) {
      var TD = window.TouchDeck;
      if (!TD) return;
      var num = info ? info.num : null;
      var level = TD.levelAfter(this._deckState.level, this._movesNum, num, TD.moves(info, this._deckMode));
      if (level !== this._deckState.level) {
        this._deckState = { level: level, page: 0, guardUntil: this._deckState.guardUntil };
      }
      this._moves = info;
      this._movesNum = num;
      this.applyDeck();
    },

    // Commands.State's mode: in a fight the movement buttons dim.
    setDeckMode: function (mode) {
      var m = mode || "";
      if (m === this._deckMode) return;
      this._deckMode = m;
      this.applyDeck();
    },

    // A tap on the deck's cell i; touchdeck.js tap decides what it does.
    _deckTap: function (i) {
      var TD = window.TouchDeck, c = this._deckCells[i];
      if (!TD || !c) return;
      var r = TD.tap(this._deckState, c, Date.now());
      this._deckState = { level: r.level, page: r.page, guardUntil: r.guardUntil };
      if (r.send && this._deckPress) this._deckPress(r.send);
      if (r.say && this._deckSay) this._deckSay(r.say);
      this.applyDeck();
    },
```

- [ ] **Step 4: Implement the page (`webclient-pure.html`)**

- Replace `          <div id="touch-deck" aria-hidden="true"></div>` with `          <div id="touch-deck" role="group" aria-label="Movement"></div>`, and in the comment above it replace `controls deck, or on a phone the open tab` with `controls deck (plan 2b: its movement buttons), or on a phone the open tab`.
- In the `Room` handler replace

```js
                var obj = GMCPStructs["Room"];
                if ( !obj.Info ) {
                    return;
                }
```

with

```js
                var obj = GMCPStructs["Room"];
                if ( !obj.Info ) {
                    return;
                }

                // The touch deck's movement buttons read the same Room.Info
                // (plan 2b). First, so nothing in the map code below can
                // keep them from updating.
                if (window.Dashboard && Dashboard.setMoves) Dashboard.setMoves(obj.Info);
```

- Replace

```js
            "Commands": function() {
                drainQueue();
                checkDeathClearsQueue();
            },
            "Commands.State": function() {
                checkDeathClearsQueue();
```

with

```js
            "Commands": function() {
                drainQueue();
                checkDeathClearsQueue();
                syncDeckMode();
            },
            "Commands.State": function() {
                checkDeathClearsQueue();
                syncDeckMode();
```

- Replace

```js
        function checkDeathClearsQueue() {
            var mode = (((GMCPStructs["Commands"] || {}).State) || {}).mode || "";
```

with

```js
        // Commands.State's mode: exploration, combat or dead.
        function commandsMode() {
            return (((GMCPStructs["Commands"] || {}).State) || {}).mode || "";
        }

        // The mode reaches the touch deck: in a fight its movement buttons
        // dim (plan 2b).
        function syncDeckMode() {
            if (window.Dashboard && Dashboard.setDeckMode) Dashboard.setDeckMode(commandsMode());
        }

        function checkDeathClearsQueue() {
            var mode = commandsMode();
```

- Replace

```js
                stopAutomation(); // nothing fires at the login prompts (#375)
                setInputQueueFull(false); // the next connection's queue starts empty
```

with

```js
                stopAutomation(); // nothing fires at the login prompts (#375)
                setInputQueueFull(false); // the next connection's queue starts empty
                if (window.Dashboard && Dashboard.setMoves) Dashboard.setMoves(null); // no buttons until the next Room.Info (plan 2b)
```

- In `init()` replace

```js
            // 6) Play in Browser already said yes, so join straight away. Join
```

with

```js
            // 5b) A touch deck button sends only its id as Char.Button.Press
            // (plan 2b; the client never builds a command), and the Dark
            // tile's line goes into the game window like the page's own lines.
            if (window.Dashboard && Dashboard.setDeckSender) {
                Dashboard.setDeckSender(function (id) { return SendGMCP("Char.Button.Press", { id: id }); }, gameLine);
            }

            // 6) Play in Browser already said yes, so join straight away. Join
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `node tools/jstest/dashboard-deck.test.js && node tools/webclient-tests/touch-frame.js && node tools/jstest/webclient-connect.test.js && node tools/jstest/webclient-action-queue.test.js && node tools/jstest/dashboard-popout-layout.test.js && node tools/jstest/dashboard-comms-newest.test.js && go test ./internal/guards/ -run TestClientAndSiteTypeUsesTheTokens -count=1`
Expected: all `ok`, exit 0.

- [ ] **Step 6: Prove the tests can fail**

In `setMoves` replace the `levelAfter` line's result with `"exits"` (`var level = "exits";`): "after a move the deck returns to the T" fails. Restore. Remove `syncDeckMode();` from the `Commands.State` handler: touch-frame.js fails "Commands.State's mode reaches the deck". Restore.

- [ ] **Step 7: Run the JS gate** (Task 10 step 4). Expected: `failed=0`.

- [ ] **Step 8: Commit**

```bash
git add _datafiles/html/public/static/js/dashboard.js _datafiles/html/public/webclient-pure.html tools/webclient-tests/touch-frame.js tools/jstest/dashboard-deck.test.js
git commit -F - <<'EOF'
feat(webclient): the deck walks: Room.Info in, Char.Button.Press out (#781)

Touch arc plan 2b. dashboard.js applyDeck draws the cells touchdeck.js
lays out from the last Room.Info and Commands.State mode, keeps the
level (the list, Back, the T after a move) and sends a tap's id through
the page's SendGMCP; the Dark tile's line goes through gameLine. A
closed connection clears the buttons. The deck is one labelled group
for screen readers.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: Turning it on: `AUTOMATIC_SHOWS` and the Settings note

**Files:**
- Modify: `_datafiles/html/public/static/js/touchdeck.js:33-35`
- Modify: `tools/jstest/touchdeck.test.js:46-48`
- Modify: `_datafiles/html/public/webclient-pure.html:306`
- Modify: `tools/webclient-tests/touch-frame.js`

- [ ] **Step 1: Write the failing tests**

`tools/jstest/touchdeck.test.js`: replace lines 46 to 48 with

```js
check('plan 2b turns Automatic on', TD.AUTOMATIC_SHOWS, true);
check('Automatic on a touch screen: on', TD.deckOn('auto', true), true);
check('Automatic with a mouse: off', TD.deckOn('auto', false), false);
```

`tools/webclient-tests/touch-frame.js`, directly before `console.log(fails === 0 ?`, add:

```js
check(html.indexOf('<p class="settings-note" id="touch-note">Buttons above the command box for walking. Automatic shows them on phones and tablets. Always shows them on every screen. Never hides them.</p>') > 0, 'the Touch controls note says what each choice does now (plan 2b)');
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node tools/jstest/touchdeck.test.js; node tools/webclient-tests/touch-frame.js`
Expected: `FAIL plan 2b turns Automatic on`, `FAIL Automatic on a touch screen: on`; touch-frame FAILs the note.

- [ ] **Step 3: Implement**

`touchdeck.js`: replace

```js
    // Automatic shows the deck on a touch screen only once the deck has
    // buttons (owner ruling 2026-11-21). Plan 2b sets this to true.
    var AUTOMATIC_SHOWS = false;
```

with

```js
    // Automatic shows the deck on a touch screen once the deck has buttons
    // (owner ruling 2026-11-21): plan 2b's movement buttons (spec 2,
    // "Turning it on").
    var AUTOMATIC_SHOWS = true;
```

`webclient-pure.html:306`: replace the note's text with `Buttons above the command box for walking. Automatic shows them on phones and tablets. Always shows them on every screen. Never hides them.`

- [ ] **Step 4: Run the tests to verify they pass**

Run: `node tools/jstest/touchdeck.test.js && node tools/webclient-tests/touch-frame.js && node tools/jstest/touch-targets.test.js`
Expected: all `ok`.

- [ ] **Step 5: Prove the tests can fail**

Set `AUTOMATIC_SHOWS = false` again: two FAIL lines. Restore.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/html/public/static/js/touchdeck.js tools/jstest/touchdeck.test.js _datafiles/html/public/webclient-pure.html tools/webclient-tests/touch-frame.js
git commit -F - <<'EOF'
feat(webclient): phones and tablets show the deck by default (#781)

Touch arc plan 2b: the deck has buttons now, so Automatic shows it on a
touch screen (pointer: coarse). Always and Never still override.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: Docs: README, PATCH_NOTES, brand doc, context.md

**Files:**
- Modify: `docs/README.md`, `docs/PATCH_NOTES.md`, `docs/guides/BRAND_STANDARDS.md`
- Modify: `internal/exit/context.md`, `internal/rooms/context.md`, `internal/behaviortree/context.md`, `modules/gmcp/context.md`

- [ ] **Step 1: `docs/README.md`**

- Beside the 2a plan row (`superpowers/plans/2026-11-22-moving-by-touch-2a-server.md`, about line 390) add:

```markdown
| [`superpowers/plans/2026-11-26-moving-by-touch-2b-deck.md`](superpowers/plans/2026-11-26-moving-by-touch-2b-deck.md) | Touch arc plan 2b (#781), the deck walks: `touchdeck.js` `moves`, `cells`, `levelAfter`, `tap`, `render` (the inverted T, the other-exits level with Back and More, locked, dark and combat states); `dashboard.js` draws them from `Room.Info` and sends `Char.Button.Press`; the `--deck-label` 12px token (ruling 18); an instance portal's button reads its zone (`rooms.ForDeck`, `exitsv2` `label`); Automatic on for touch screens; headless size matrix and Malia's S21 check |
```

- In the `tools/jstest/` row, at the end, add: `Added 2026-11-26 (touch arc plan 2b): \`touchdeck-moves.test.js\` (the deck's movement model from \`Room.Info\`, its cells at every shape, the other-exits level with Back and More, the T after a move, taps (a quick double tap on the T walks twice, on an other exit once), the drawn buttons and their screen reader words, and a check that the client's diagonal order is the server's \`exitOrder\`), \`dashboard-deck.test.js\` (loads the real \`dashboard.js\` and \`touchdeck.js\`: \`Room.Info\` and the mode in, a tap's id out, the Dark tile's line, the buttons cleared on a closed connection); \`touchdeck.test.js\` now checks Automatic is on for touch screens; \`touch-targets.test.js\` checks a deck button sets no size and a six-letter name fits at 48px and 44px; \`webclient-colours.test.js\` measures every deck colour.`
- In the `touchdeck.js` row (about line 412), after `with \`fieldOf\` naming the field being typed in)`, add `; plan 2b: the movement buttons (\`moves\`, \`cells\`, \`levelAfter\`, \`tap\`, \`render\`), Automatic on for touch screens`, and add `touchdeck-moves.test.js` to its "Tested by" list.

- [ ] **Step 2: `docs/PATCH_NOTES.md`**, at the top under `# DOGMud Patch Notes` (date the PR merges):

```markdown
## 2026-11-26: Walk by touch

- On a phone or tablet, buttons above the command box let you walk without typing. Tap an arrow to go that way; the stairs go up or down.
- Only the ways you know light up. A padlock marks a locked door: tapping it tries the door, the same as typing the direction.
- Tap "exits" for the other ways out, such as northeast or a named path. Back returns to the arrows, and so does every step you take.
- In the dark the arrows turn grey and you can still try every way. A cross marks a wall you walked into, and a padlock a locked door you found.
- In a fight the arrows fade: you cannot walk away from a fight.
- To show the buttons on any screen, or to hide them, open Settings and choose Touch controls.
```

- [ ] **Step 3: `docs/guides/BRAND_STANDARDS.md`**

- Section 3, after the `**Two sizes:**` bullet, add a bullet:

```markdown
- **Touch deck names** (owner and Malia, 2026-11-26, touch arc ruling 18):
  names and letters on the deck's buttons are 12px (`--deck-label`), so a
  six-letter exit name fits a 44px button. Nothing else uses this size.
```

- Section 4, in the `**The touch deck**` bullet, replace `Labels at the client's text size, JetBrains Mono, regular weight.` with `Arrows at the title size, names and letters at 12px (\`--deck-label\`), JetBrains Mono, regular weight. In a pitch-dark room the movement buttons turn olive grey with a dashed edge (touch arc spec 2, ruling 11).` The sentence "Every button is brass." stays as it is (owner ruling R1 on plan 2b confirmed it).

- [ ] **Step 4: context.md**

- `internal/exit/context.md`, after the `**name.go**` paragraph's last sentence (`... is exempt by the same ruling.`) add: ` \`InstancePortalKey(zone, userId)\` builds that key (the one place, read by \`actOpenInstancePortal\`), and \`InstancePortalZone(key, t)\` reads the zone back when the key ends in the exit's own \`UserId\`; the touch deck labels the portal's button with it (\`rooms.ForDeck\`).`
- `internal/rooms/context.md`, at the end of the `## Known exits (#781)` section's first paragraph add: `\`ForDeck(known, userId)\` is the part the touch deck offers, with each button's label (owner ruling 18): every exit under its own name, except a player's instance portal, labelled with its zone and offered only when \`MayEnterInstance\`, so a party member's portal is offered beside the player's own; \`IsOwnPortal(ke, userId)\` names the player's own. \`known\` is left whole for the exits line. \`MayEnterInstance(userId, roomId)\` is the one instance access rule; \`MoveToRoom\` refuses on it.`
- `internal/behaviortree/context.md`, in the sentence ending `wired to that player's exit and its label leaves the number off.` add after it: ` The key comes from \`exit.InstancePortalKey\`.`
- `modules/gmcp/context.md`, in the `Room.Info` exits paragraph after `\`id\` (the movement button id, \`internal/buttons\`)` add `, \`label\` (only when the button text is not the exit's name: an instance portal's zone, from \`rooms.ForDeck\`; a portal the viewer may not enter keeps no \`id\`)`, and to the `details` list add `\`own_portal\` (the viewer's own instance portal, \`rooms.IsOwnPortal\`; the deck lists it first among portals to one zone)`.

- [ ] **Step 5: Check**

Run: `python tools/context_md_audit.py`
Expected: no finding for `exit`, `rooms`, `behaviortree` or `gmcp`.

- [ ] **Step 6: Commit**

```bash
git add docs/README.md docs/PATCH_NOTES.md docs/guides/BRAND_STANDARDS.md internal/exit/context.md internal/rooms/context.md internal/behaviortree/context.md modules/gmcp/context.md
git commit -F - <<'EOF'
docs: touch arc plan 2b rows, patch note, brand doc, context (#781)

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: Full gate

- [ ] **Step 1: gofmt**

Run: `gofmt -l internal/ modules/ *.go`
Expected: no output. For any file listed, run `gofmt -d <file>` and apply the change with the Edit tool.

- [ ] **Step 2: Build and the whole suite**

Run: `go build ./... && go test ./... -count=1 2>&1 | tail -40`
Expected: every package `ok` (or `no test files`). Then `go test . ./internal/guards/ -count=1`: `ok`. (Known Windows failures `TestDeleteZone_RemovesEveryTree` and `TestRenameZone_MovesRewritesAndRekeys` in `internal/rooms` are not this plan's; if any `ziggurat_test_zone/` or `rename_probe_zone/` folder appears under `_datafiles/world/dogmud/rooms/`, remove it.)

- [ ] **Step 3: Lint at CI's pin**

Run: `bash tools/lint.sh`
Expected: `0 issues.`

- [ ] **Step 4: JS gate (CI's javascript job, `.github/workflows/validate.yml`)**

Run (Git Bash):

```bash
failed=0
while IFS= read -r f; do case "$f" in *.min.js) continue ;; esac; if [ -f .jshintignore ] && grep -Fxq "$f" .jshintignore; then continue; fi; node --check "$f" || failed=1; done < <(git ls-files '*.js')
for t in tools/jstest/*.test.js; do node "$t" >/dev/null || { echo "FAIL $t"; failed=1; }; done
for t in tools/webclient-tests/*.js; do node "$t" >/dev/null || { echo "FAIL $t"; failed=1; }; done
echo "failed=$failed"
```

Expected: `failed=0`. (`git ls-files` lists only tracked files: run it after the new test files are committed, or add them to the loop by hand.)

- [ ] **Step 5: Boot smoke**

Run: `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1`
Expected: `ok`.

- [ ] **Step 6: context.md audit**

Run: `python tools/context_md_audit.py`
Expected: no new finding.

---

### Task 11: HEADLESS: the size matrix screenshots and measures (controller, not a subagent)

The controller runs this; screenshots land in the repo root (owner rule), named `2b-<size>-<state>.png`, and are not committed.

- [ ] **Step 1: Boot this branch on private ports**

```bash
S="<session scratchpad directory, full path>"
git worktree add --detach C:/tmp/dogmud-2b-shots HEAD
cd C:/tmp/dogmud-2b-shots && go build -o shots.exe .
```

Wipe instance saves in the worktree as `dogmud-shipping` says. Write `"$S/shots-manifest.yaml"` with the Write tool:

```yaml
entries:
  - profile: mid
    start_room: 5243
    actor_id: tower
  - profile: mid
    start_room: 6408
    actor_id: junction
  - profile: mid
    start_room: 473
    actor_id: barracks
  - profile: mid
    start_room: 6418
    actor_id: dark
```

Write `"$S/shots-override.yaml"`:

```yaml
Network:
  TelnetPort: [43333]
  LocalPort: 19999
  HttpPort: 18090
  AIPort: 45555
Playtest:
  ProfilesManifest: "<S>/shots-manifest.yaml"
```

Start it in the background (`CONFIG_PATH="$S/shots-override.yaml" ./shots.exe > "$S/shots.log" 2>&1`), record its PID, wait for `Server Ready` in the log. Logins are in `creds.json` beside the manifest. Never point anything at the owner's server.

- [ ] **Step 2: The size matrix at Tower Top**

Write `"$S/matrix.json"`: one session per size of F51, the `tower` login, `"emulate": {"width": W, "height": H, "dpr": 3, "mobile": true, "touch": true}` (the laptop 1366x768 with `"dpr": 1, "mobile": false, "touch": false`), steps:

```json
[ {"wait": 3500},
  {"eval": "matchMedia('(pointer: coarse)').matches"},
  {"eval": "(() => { const d = document.getElementById('touch-deck'); const kids = [...d.children]; const r = kids.map(e => e.getBoundingClientRect()); return JSON.stringify({ shows: document.getElementById('touch-slot').dataset.shows, n: kids.length, sizes: [...new Set(r.map(x => Math.round(x.width) + 'x' + Math.round(x.height)))], gap: r.length > 1 ? Math.round(r[1].left - r[0].right) : null, deckRight: Math.round(d.getBoundingClientRect().right), vw: innerWidth, pageW: document.documentElement.scrollWidth, lit: [...d.querySelectorAll('.deck-move:not(.deck-dim)')].map(b => b.getAttribute('aria-label')), exits: !!d.querySelector('.deck-other:not(.deck-dim)') }); })()"},
  {"shot": "<repo>/2b-<W>x<H>-top.png"} ]
```

Run: `node tools/webshot/live-shot.js "$S/matrix.json" > "$S/matrix.out"`

Expected for every touch size: `shows` `deck`; every cell `48x48` with `gap` 8, except 320x640 at `44x44` and 5; `deckRight <= vw` and `pageW <= vw` (no sideways scroll); `lit` is `["Go west"]`; `exits` true. 800x360: 12 cells in a 3-wide column, the exits button under the T. 1366x768 (no touch): `shows` `empty`, as on master (desktop unchanged).

- [ ] **Step 3: States at the S21 (360x800) and at 320x640**

One session each for `tower`, `junction`, `barracks`, `dark`, at both sizes, steps (the `eval` lines click real buttons):

- tower, R1: `{"eval": "getComputedStyle(document.querySelector('[aria-label=\"Go west\"]')).backgroundImage + '|' + getComputedStyle(document.querySelector('[aria-label=\"Go west\"]')).color"}` expect `linear-gradient(rgb(237, 212, 137), rgb(200, 169, 91))|rgb(26, 18, 6)` (the client brass and `--brass-text`).
- tower: shot top; `{"eval": "document.querySelector('.deck-other').click(); [...document.querySelectorAll('.deck-exit')].map(b => b.textContent + '|' + (b.firstChild.scrollWidth <= b.clientWidth))"}` expect `["ledge|true", "shrine|true"]` (the six-letter `shrine` fits in the real font); shot `exits`; click the `shrine` button, `{"wait": 2500}`, `{"eval": "!!document.querySelector('.deck-back')"}` expect `false` (back at the T in the new room); shot `after-move`.
- junction: click exits; expect the two labels `↗NE`, `↙SW` in that order; shot.
- barracks: `{"eval": "!!document.querySelector('[aria-label=\"Go down (locked)\"] .deck-lock')"}` expect `true`; shot.
- dark: `{"eval": "document.getElementById('touch-deck').classList.contains('deck-dark') + '|' + document.querySelectorAll('.deck-blind:not([disabled])').length"}` expect `true|6`; click `[aria-label="Go north"]`, `{"wait": 2500}`, `{"eval": "!!document.querySelector('.deck-mark-wall')"}` expect `true`; click `.deck-darktile`, `{"tail": 3}` shows `It is too dark to see the way. Light a torch or cast glow.`; shot. (If the `mid` profile carries a light and the room is not dark, record that and check the dark state on the device instead.)

- [ ] **Step 4: Combat**

Headless has no reliable fight on these rooms; combat dim is covered by the jstests (Tasks 3, 4, 7) and Task 12's device check. Record that.

- [ ] **Step 5: Record and clean up**

Put the numbers (sizes, gaps, scroll widths) in the PR body. Stop the server by its recorded PID only (`Stop-Process -Id <pid>`). `git worktree remove --force C:/tmp/dogmud-2b-shots` (PowerShell `Remove-Item -Recurse -Force` if Windows holds the exe).

---

### Task 12: OWNER'S CHECK: Malia's S21 (#834)

After the PR is green, before merge. Malia's Galaxy S21 on the local test server over Wi-Fi (look up the LAN IP each time; the owner's Pixel 8 cannot reach it). A login made with ordinary character creation (`new`), easy name; ordinary player steps only, no admin commands. Settings, Touch controls on Automatic.

1. Enter the game. Buttons sit above the command box: arrows, stairs, and "exits". Only the ways out of the room are bright.
2. Tap a bright arrow: you walk. Tap a faded one: nothing happens.
3. Tap a bright arrow twice quickly: you walk two rooms.
4. In Pothole Coulee, find the Cracked Cistern (west and northeast). "exits" is bright; tap it: Back and a northeast button. Tap northeast: you walk, and the arrows come back.
5. Walk to the Tower Top at the old tower. Tap "exits": `ledge` and `shrine`, both fully readable. Back returns to the arrows.
6. Start a fight with a creature. The buttons fade. Tap an arrow: the game says you cannot leave while fighting.
7. If you can reach a pitch-dark place without a light (the game says it is too dark to see): every arrow is grey and tappable, "exits" is a closed eye with the word Dark; tap it and read the line; tap a way with no exit: a cross appears on it. If no dark place is in reach, say so on #834 (the headless check covered it).
8. Turn the phone on its side: the buttons sit in a column beside the game, the exits button under the arrows.
9. Settings: Never hides the buttons; Automatic brings them back.

Record each result on #834 (`gh issue comment 834 --repo pruuk/DOGMud`).

---

### Task 13: PR

- [ ] **Step 1: Re-run the gate if anything changed since Task 10** (steps 1 to 6).

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feature/781-moving-by-touch-2b
gh pr create --repo pruuk/DOGMud --base master --head feature/781-moving-by-touch-2b --title "Touch arc 2b: walking by touch on the deck (#781)" --body-file "$S/pr-body.md"
```

`$S/pr-body.md` (written with the Write tool): a summary of tasks 1 to 9, the headless numbers (Task 11), the open questions still open, the line `Part of #781`, never a closing keyword, and the attribution line `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Confirm the URL `gh` prints says `pruuk/DOGMud`.

- [ ] **Step 3: Watch the checks**

Run: `gh pr checks <n> --repo pruuk/DOGMud --watch`, then `gh run list --repo pruuk/DOGMud --branch feature/781-moving-by-touch-2b` to confirm the javascript job ran. On a red run, `gh run view <id> --repo pruuk/DOGMud --log-failed`.

- [ ] **Step 4: After Malia's check (Task 12) and the owner's merge**

Comment on #781 with what landed and what is left (2c: the Map tab's T and the sideways Menu, #938); do not close it. The owner deploys.

---

## Self-review

**Spec coverage.**

| Spec item (2b, rulings, tests) | Task |
|---|---|
| Layout: inverted T left (up, north, down; west, south, east), exits button beside it, slot kept when empty; command slots empty | 3 (`cells`), 6 |
| Symbols: arrow with letter, stairs pictogram for up and down (ruling 10) | 2, 3, 4 |
| Lit, dim, padlock (locked now, ruling 3) | 2, 3, 4, 6 |
| Dark: olive grey, all six tappable, wall mark and padlock marks, Dark tile and its line (rulings 4, 11) | 2, 3, 4, 6, 7 |
| In combat movement dims | 2, 3, 4, 6, 7 |
| Other exits: a level replacing the deck, Back top-left, fixed order, More only when needed, back to the T after a move | 2, 3, 7 |
| Fixed ids so a fast double tap works like typing (ruling 8); press sends only the id (ruling 7) | 3, 7 |
| Stale id: the server refuses and resends `Room.Info`; the client redraws from it and keeps the list in the same room | 7 (`setMoves`, `levelAfter`); server side shipped in 2a |
| Exit names at most 6 letters, named text 12px, instance portal labelled without the number and wired to that player's exit (ruling 18) | 1, 5, 6 |
| Plan rulings: every deck button brass, dark state olive grey (R1); every exit the player may enter gets a button, own portal first (R2) | 4, 6, 11 (R1); 1, 2, 4 (R2) |
| `AUTOMATIC_SHOWS` true; the setting still overrides | 8 |
| 320px: 44px slots with 5px gaps | existing guard 6 (`touch-targets.test.js`), 6 (name fit), 11 |
| jstests: every state, the level and Back, `AUTOMATIC_SHOWS` on | 2, 3, 4, 7, 8 |
| Guard 6 across the size matrix | existing, extended in 6; measured in 11 |
| Colour contrast written beside each new colour | 6 |
| Headless size matrix, Malia's S21 check | 11, 12 |

**Out of scope, by the spec and roadmap:** the Map tab's T and the sideways Menu (2c); Flee and the combat swap (3d); command slots (3).

**Placeholder scan:** none. Every code step shows the code; every command shows its expected result.

**Type consistency:** `TouchDeck.moves(info, mode)` returns `{ready, dark, combat, t: {dir: {id, lit, locked, mark}}, others: [{name, id, label, arrow, locked, own}]}`; `cells(model, shape, level, page)` returns cells `{kind, ...}` with kinds `empty move other dark back exit more`; `levelAfter(level, lastNum, num, model)`; `tap(state, cell, now)` returns `{level, page, guardUntil, send, say}`; `render(deck, cells, doc)`; exports `T_DIRS`, `DIAGONALS`, `NAME_LETTERS`, `DOUBLE_TAP_MS`, `DARK_LINE`. `Dashboard.setDeckSender(press, say)`, `setMoves(info)`, `setDeckMode(mode)`, `_deckTap(i)`, state `_moves`, `_movesNum`, `_deckMode`, `_deckState`, `_deckCells`, `_deckSig`. Server: `exit.InstancePortalKey(zone, userId) string`, `exit.InstancePortalZone(key, TemporaryRoomExit) (string, bool)`, `rooms.MayEnterInstance(userId, roomId) bool`, `rooms.ForDeck(KnownExitList, userId) (KnownExitList, map[string]string)`, `rooms.IsOwnPortal(KnownExit, userId) bool`, the `exitsv2` detail `own_portal`, `GMCPRoomModule_Payload_Contents_ExitInfo.Label` (`json:"label,omitempty"`). Used with these signatures in every task.

## Open questions for the owner

The first two questions (button colour, other players' portals) are answered: rulings R1 and R2 above.

1. **Two portals with one label.** A player in a party can stand beside their own arena portal and a party member's: two buttons both read "arena" (their own first, D3). A screen reader hears "Go arena (yours)" for the first; the screen shows nothing to tell them apart. Is that enough, or should the second button show something more (for example the opener's name, which would need a label longer than six letters)? The plan keeps it as is.
