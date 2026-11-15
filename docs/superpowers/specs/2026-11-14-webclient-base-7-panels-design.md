# Web client base plan 7: panels (Map, Quests, Comms, Equipped, menus)

Status: design approved by the owner 2026-11-14 (brainstorm and mocks); this file awaits his review.
Parent: `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md` (rulings 16 to 21, design section 7), under the touch epic #783. Builds on plan 6 (`docs/superpowers/specs/2026-11-14-webclient-base-6-phones-design.md`, PR #869, branch `feature/webclient-base-6-phones`).
Mocks: https://claude.ai/artifact/VtaJsG4hfn4JRUi569z2SK (the Map panel three ways, the menus, the Quests tabs).
Real devices: plans 6 and 7 are tested together on phones after this plan is built (owner, 2026-11-14).

## Facts verified against source

Read 2026-11-14 from `feature/webclient-base-6-phones` at `5336b9125`. `WP` is `_datafiles/html/public/webclient-pure.html`; `CSS` is `_datafiles/html/public/static/css/dashboard.css`; `JS` is `_datafiles/html/public/static/js/`.

| Fact | Where |
|---|---|
| The Map title is set to `'Map (' + obj.Map.zone + ')'` in the Zone.Map handler; `obj.Map.cz` (the level) is passed to `setZoneSnapshot` just above | `WP:987`, `:990`; `modules/gmcp/gmcp.Zone.go:37` |
| The map art draws the zone title and `~ Level N ~` | `JS/gmcp.js:955-962` |
| It draws the current room's name under its token | `JS/gmcp.js:1013-1017` |
| It draws the focused quest's name under its target room | `JS/gmcp.js:1060-1066` |
| It draws the quest area label on a plate over the gold wash, and the "Somewhere in <label>" banner | `JS/gmcp.js:700-709`, `:721-736` |
| `Room.Info.name` (the room title) is read client side; the focused quest's `name` and `target_label` go to `setQuestMarker` | `WP:2323-2324`, `:1000`, `:1007`; `modules/gmcp/gmcp.Room.go:455`; `modules/gmcp/gmcp.Char.go:727-729` |
| Under the map: `#map-controls` (Fit, Center, zoom, the Legend toggle), then the folded `#map-legend`; no location bar | `WP:329-345` |
| `renderQuests` writes `#quests-list` through `writeQuestsHTML`; finished repeatables (`status` set) go under "Repeatable work", sorted by name, each with "(repeatable)" and a hint line | `WP:1143-1166` |
| Active rows carry "(repeatable)" when `q.repeatable`; only `.qrow` gets the focus click and the menu | `WP:1191`, `:1247-1251`, `:4461-4466` |
| `Char.Quests` skips quests whose progress step is `end` (finished) and secret quests, then appends `finishedRepeatQuests` (repeatables finished, not in progress, not shelved, not secret, not at their cap) with `status` `ready` or `waiting` | `modules/gmcp/gmcp.Char.go:28-60`, `:675-690`, `:745` |
| The payload row type, with `Repeatable`, `Status`, `Abandonable` | `modules/gmcp/gmcp.Char.go:1194-1211` |
| `quests all` lists finished quests: progress at `end` (via `activeQuestRecord`, completion 1) plus `finishedRepeatRows` (repeatables finished, including those at their cap), secret ones hidden unless an admin asks for `all+`, shelved ones after | `internal/usercommands/quests.go:89-121`, `:123`, `:168`, `:221-290`, `:323` |
| Subtabs pattern: `.subtabs` group, `button.subtab[data-tab]`, `aria-pressed`, `.active` | `WP:365-370`, `:1439-1448`; `CSS:997-1049` |
| Game window line height 1.2; quest list `.qname` 1.25, `.qhint` and `.qprogress` 1.4 | `WP:470`; `CSS:360`, `:366`, `:375` |
| The panel label "Chat": title, hint ("...more room to read Chat."), `aria-label="Chat channels"`, an HTML comment; `dashboard.js` `chat: { label: "Chat" }` feeds the tab's aria label, the name pill and "Close Chat" | `WP:376`, `:379`, `:382`, `:385`; `JS/dashboard.js:48`, `:76`, `:99`, `:555` |
| A jstest matches the hint string | `tools/jstest/viewportfit.test.js:258` |
| The Chat channel tab and the `#panel-chat` / `data-panel="chat"` ids | `WP:390`; `tools/jstest/chat-hints.test.js` |
| Equipped: entries with no item are skipped; tiles on `repeat(auto-fill, minmax(46px, 1fr))`; `.inv-slab` slot labels `display: none`; `.inv-tile.empty` CSS unused; a stale comment says clicks reach a tile menu | `WP:1315-1364`; `CSS:1134-1135`, `:1152-1166`, `:1186`, `:1190-1196`, `:1198-1209` |
| Three context menus, each built lazily on body and opened on `contextmenu` only: items `.inv-ctx`, Automation `.auto-ctx`, Quests `.quest-ctx`; header `.ctx-header` | `WP:4036-4041`, `:4104-4108`, `:4185-4190`, `:4266`, `:4308-4311`, `:4356-4361`, `:4375`, `:4413-4417`, `:4461-4466` |
| Their CSS is three copies | `CSS:1263-1300`, `:1465-1503`, `:1506-1550` |
| A left click or tap on an item tile does nothing; a click on an Automation entry runs it (a tick's toggle switches it); a click on a quest row focuses it | `WP:2068-2071`, `:1247-1251` |
| No long-press or pointer handler exists for menus; iOS Safari fires no `contextmenu` on long-press | `WP` (grep `touchstart`, `pointerdown`: only the plan 6 X) |

## Rulings

From the layout spec, not reopened: 16 (Chat becomes Comms; the Chat channel keeps its name), 17 (Map title `Map (Zone, Level N)`; a Location and Destination bar under the map buttons; names come off the art), 18 (Quests tabs replace "Repeatable work"; Repeat rows end "Ready now" or "Not yet", ready first; the tab shows the ready count), 19 (the quest list uses the game window's line spacing), 20 (item menu: name at the larger size, boxed, separate from the actions, actions indented; on desktop by right-click and on touch by a tap), 21 (empty equipment slots stay unshown). The layout spec's choice "All three right-click menus match" was approved with it.

Owner, 2026-11-14 (memory `feedback-webclient-plan7-rulings`):

- P1. No text on the map art at all: besides ruling 17's names, the quest area label and the "Somewhere in" banner come off too (option A of the mock).
- P2. On touch, a long-press opens the item, quest and Automation menus; a tap keeps its current job (an item tile's tap opens its menu, per ruling 20). Desktop keeps right-click.
- P3. The long-press is a stopgap: contextual buttons belong to the touchscreen arc (#779, #780; noted on #780).
- P4. Quests get a third tab, Completed.
- P5. The menu and Quests mocks are approved as drawn, with P4 added.
- P6. The zone-exit labels stay on the map art (`→ Greenford` at the edge, `JS/gmcp.js:1141-1148`): they are the only place the client says where a zone exit leads. Single-character marks stay too (compass N, shop and bank glyphs, `?`, stairs). Asked while planning, after the spec was approved.

## What players see

The Map title reads `Map (Greenford, Level 1)` and a bar under the map buttons reads `Location: Mill Lane` and `Destination: Eastfield`; the map itself shows only symbols. Quests has three tabs: Active, Repeat (with how many are ready) and Completed. The Chat panel is called Comms. Every right-click menu shows the thing's name large and boxed, with its actions indented below, and opens on a phone by a long-press, or for an item by a tap.

## Design

### 1. Map

- **Title:** `WP:990` becomes `'Map (' + zone + ', Level ' + cz + ')'`.
- **No text on the art (P1):** the six draws in `JS/gmcp.js` (zone title, level, current room name, quest name, area label plate, "Somewhere in" banner) are removed, with their positioning comments. The wash colour, its dashed outline, the quest pin and every symbol stay. `_drawQuestBanner` goes; its caller and the `data-quest-banner` / `data-wash-label` cleanup go with it.
- **The bar:** a `<div id="map-where">` between `#map-controls` and `#map-legend`, two spans: `Location: <room>` from `Room.Info.name`, updated on every `Room.Info`; `Destination: <label>` from the focused quest's `target_label`, falling back to its `name`, updated wherever `setQuestMarker` is called; the Destination span is hidden when no quest is focused. It wraps to two lines when narrow; each span ellipsises within the panel width. Labels in the ink gold, values in the title colour, Destination's value in the quest gold, as in the mock.

### 2. Quests

- **Server (P4):** `Char.Quests` gains rows with `status: "done"` for finished quests: progress step `end`, or a repeatable at its cap; not in progress, not shelved, not secret. One helper, `usercommands.CompletedQuestIds(c *characters.Character) []int` (sorted), defines "completed" for both `quests all` and the panel, so they cannot disagree; `quests all` keeps its own row text. `finishedRepeatQuests` keeps sending `ready` / `waiting` for repeatables that can be taken again.
- **Client:** three subtabs over the list, in the existing pattern: Active, Repeat, Completed. Repeat's label is `Repeat (n)` with n the ready count, plain `Repeat` when none is ready. Completed has no count.
  - Active: today's active rows, unchanged, tag included; "No active quests." when empty.
  - Repeat: ready first, then by name; each row is the name and, right aligned, "Ready now" (quest gold) or "Not yet" (the whole row dimmed). No "(repeatable)" tag, no hint line. "Nothing to repeat yet." when empty.
  - Completed: names only, by name, dimmed; no menu, no click. "Nothing finished yet." when empty.
  - The open tab is remembered across re-renders (each push), per page load.
  - Each tab's list is written through `writeQuestsHTML` to its own host, so the render guard (finding 19) covers all three.
- **Spacing (ruling 19):** the quest list's line height is 1.2, the game window's.

### 3. Comms

The panel title, the hint line, the `aria-label`, the HTML comment and `dashboard.js`'s `label` (and so the tab's aria label, the name pill and "Close Comms") read Comms. The Chat channel tab, `chat-hints.test.js`, and the `chat` ids stay. `viewportfit.test.js:258`'s string follows the hint.

### 4. Equipped

Already as ruled (46px columns, slot labels off, empty slots not drawn). The unused `.inv-tile.empty` CSS goes, and the stale comment at `CSS:1186` is corrected.

### 5. Menus

- **One stylesheet:** the three menus share one set of classes (`.ctx-menu` on each container beside its own class; `.ctx-header`, `.ctx-row`, `.ctx-sep`, `.ctx-disabled`), replacing the three copies.
- **Look (ruling 20, mock):** the header is the name at `--title` (20px, 18px on upright phones), regular weight, in a gold-edged box inset from the menu's edge; rows are indented under it (left padding 24px) and taller (8px vertical padding) for touch.
- **Opening:** right-click as today. A tap on an item tile opens its menu at the tile. A shared `LongPress` helper (`JS/longpress.js`, pure timing logic tested in node) fires after 500ms of a still touch (cancelled by a move over 10px, a lift or a scroll), opens the menu at the touch point, and swallows the click that follows so the tap action does not also run. It is attached to item tiles, quest rows and Automation entries.

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| Completed shows names only, no count on its tab, no menu | It only grows; a count says little and a menu has nothing to offer (Focus and Abandon do not apply) | A count or a "Look at quest" action can be added later |
| Empty-tab lines: "No active quests.", "Nothing to repeat yet.", "Nothing finished yet." | Each tab says what it is for when empty | One-line copy changes |
| `Repeat` shows no count when none is ready | "Repeat (0)" reads as an error | Show `(0)` |
| The open tab is kept per page load, not saved | A reload starting on Active is predictable | Save it in localStorage |
| Menu header regular weight | Rule 7 keeps bold for titles, quest names, your own name and room titles | Bold in one rule |
| Long-press 500ms, 10px move tolerance | Common platform values; Android's is 400 to 500ms | Constants in `longpress.js` |
| `CompletedQuestIds` lives in `usercommands` beside `quests all` | `modules/gmcp` already imports `usercommands` (`Abandonable`); one definition next to its first user | Move it to `internal/quests` or `characters` if a third user appears |

Amended 2026-11-14 (plan 7 plan): the three quest tabs share one list host rather than one host per tab, so the tab strip re-renders one container; `quests all` was not rewritten onto `CompletedQuestIds`, and `TestCompletedQuestIds_MatchesQuestsAll` (which reads `quests all`'s own row builders) holds the two together instead.

Amended 2026-11-14 (plan 7 plan): a desktop left-click on an item tile opens its menu too, through one handler for click and tap (a click did nothing before); right-click is unchanged.

Amended 2026-11-14 (plan 7 build): "Ready now" is drawn in `--quest-name-focus`, because brass fails 4.5:1 on the light panel. Quest rows that are not ready, and Completed rows, are quieted by regular weight as well as colour, because a dimmer colour alone fails contrast in light mode. The map bar's Destination uses `--cp-mid` (#e6b058), the map's own quest gold, in every theme. Dimmed menu rows use `--toast-desc` (5.22:1).

## Out of scope

Contextual buttons, the command bar and touch movement (#779, #780, #781). Menus by keyboard (#728). The xterm versus HTML renderer comparison (layout spec section 8).

## Testing

- Go: `CompletedQuestIds` for a finished one-off, a capped repeatable, an uncapped finished repeatable (excluded), a shelved quest (excluded), a secret quest (excluded), one in progress (excluded); `Char.Quests` carries `done` rows for exactly those; `quests all` lists the same set as before (its existing tests stay green).
- jstest: the quest split and sort (Active, Repeat ready first, Completed by name), the Repeat label count, empty lines, the tab kept across a re-render; the map bar text from `Room.Info` and a focused quest, Destination hidden without one; a source guard that `gmcp.js`'s map renderer draws no `text` element; the Comms labels and the untouched Chat channel; `LongPress` timing (fires at 500ms, cancelled by a move, a lift, a scroll; the following click swallowed); the shared menu classes on all three menus.
- Headless Playwright (plan 6's harness): desktop right-click and phone tap / long-press open each menu; the Map title and bar after login; the Quests tabs.
- Real devices: with plan 6, after this plan.
- Gates: gofmt, build, lint, `go test ./...`, jstests, boot.
