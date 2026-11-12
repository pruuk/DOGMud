# Web client base: text width, type and layout

Status: approved by the owner, 2026-11-09, including the choices listed below. One plan per slice.
Parent: the touch epic #783. This is the base every child builds on. It covers #775, #776, #778 and #829, and part of #730 and #780.

Reference, the agreed look: https://claude.ai/artifact/299yMwQcZ6kWdxQhnMb3zg (source `_archive/notes/reports/full-client-mock/`, every visual change is in `static/css/mock-overrides.css`). Options research: `_archive/notes/reports/MUD web client rendering options.md`.

## Facts verified against source

Read 2026-11-09 from master at `be17c3cbe`. `origin/master` is `572348425`, which adds only #828 (images and two guides) and #830 (a `lang` attribute and an iframe `title`). `WP` is `_datafiles/html/public/webclient-pure.html`.

| Fact | Where |
|---|---|
| Game text is sized by `TARGET = 80, MAX_FS = 20`: the font shrinks until 80 columns fit, then CSS `scale()` | `static/js/termfit.js:20`, `:73-124` |
| xterm 4.19.0, built with `cols: 80`, `fontSize: 20` | `WP:431-435` |
| One write of server text | `WP:3043` |
| 39 non-test calls to `SplitStringNL` or `WrapAnsi`; 27 pass a literal 80 (23 on one line, 4 split across lines in `drink.go` twice, `talk.go`, `aicompanion/commands.go`) | grep, reconciled with the research note |
| `setScreenWidth = 80` | `internal/usercommands/set.go:77` |
| Room description wraps at 80, and `80 - 7` beside the tinymap | `internal/rooms/roomdetails.go:136`, `:117` |
| Menu `lineWidth = 80` | `internal/menu/menu.go:24`, `:84`, `:144` |
| `shouldWrap` allows 48 of 69 categories; Speech, Whisper, Shout and Emote are excluded because they are pre-wrapped at 80 | `internal/messaging/pipeline.go:127-170` |
| Pipeline width comes from `u.GetLineWidth()` | `internal/users/userrecord.go:512`; `internal/rooms/rooms.go:271`, `:346`, `:464`, `:538`, `:551`, `:587`, `:621` |
| `LineWidth` is per user, saved, default 80; `set linewidth` takes 40 to 240 | `internal/users/userrecord.go:56`, `:543-548`; `internal/usercommands/set.go:522` |
| `Display.ScreenWidth` is per connection, default 80; only telnet paths set it | `internal/connections/clientsettings.go:22-32`; `internal/inputhandlers/term_iac.go:153-155` |
| Writing it goes through `connections.OverwriteClientSettings` | `internal/connections/connections.go:353` |
| Client messages to the server are dispatched by a `switch` on the GMCP name; `Zone.Map.Resync` is the closest example | `modules/gmcp/gmcp.go:297-300`, `:422` |
| `IsWebsocket` already branches behaviour in four files | Defined at `internal/connections/connections.go:82`; used in `look.go:716`, `MSP_PlaySound.go`, `WebClientCommand_SendWebClientCommand.go`, `login_prompt_handler.go` |
| Templates: 608 files, of which 58 are hard layouts (51 outside help) and 182 aligned columns (170 in help). A heuristic with 11 files read by eye | research note section 5 |
| Template variants exist: `fname.screenreader.template` is tried first; 9 exist | `internal/templates/templates.go:186-193` |
| The template config is cached per user and cleared by `ClearTemplateConfigCache` | `internal/templates/templates.go:119-142` |
| `GetTable` lays out tables | `internal/templates/templates.go:382` |
| 100 lines mention `font-size` (`WP` 28, `dashboard.css` 56, `gomud.css` 16); 20 lines set italics | grep |
| Fonts live in `static/fonts/`, each with its license file beside it | directory listing |
| `BRAND_STANDARDS.md` section 3 names Source Sans 3 and Source Code Pro | `docs/guides/BRAND_STANDARDS.md:72-80` |
| Layout modes are `wide` (1100px and up), `rail` (700 to 1100), `phone` | `static/js/dashboard.js:243-247` |
| Saved layouts use the key `dogmud.dashboard.layout.v1` | `static/js/dashboard.js:636` |
| The tab height cap is half the screen, and an open tab already sits above the command box when the keyboard is up | `dashboard.css:592`, `:659-667` |
| `html.keyboard-open` hides the tab bar | `dashboard.css:652` |
| After a command the keyboard closes through one function | `WP:3169`; `static/js/hotinput.js:149-154`; tested in `tools/jstest/hotinput.test.js` |
| The command box is 16px because iOS zooms on anything smaller | `WP:169-173` |
| Item, Automation and Quest menus open on `contextmenu` only, with a `ctx-header` line | `WP:3918`, `:3986`, `:4065`, `:4146`, `:4236`, `:4255`, `:4341` |
| Empty equipment slots are deliberately not drawn | `WP:1264-1268` |
| Map title is set to `Map (zone)`; the map floor is `obj.Map.cz` | `WP:930-933` |
| `Room.Info` carries the room `name`; the focused quest carries `name`, `target_label` | `modules/gmcp/gmcp.Room.go:603`, `:455`; `WP:948-951` |
| Map art draws zone, level, room name and quest name | `static/js/gmcp.js:955-962`, `:1013-1017`, `:1061-1065` |
| Finished repeatable quests arrive with status `ready` or `waiting` | `modules/gmcp/gmcp.Char.go:28-33`; `WP:1090-1105` |
| A toast component exists | `static/js/dashboard.js:229`; `dashboard.css:1634` |

## What players get

Game text is one readable size on every device, and text fits the screen instead of the screen shrinking to fit the text. An upright phone reads 14px text at about 42 columns where it read 7 to 8px at 80. The client looks like one design: one font, two text sizes, the game on the left on large screens and the panels organised around a square map.

## Rulings

These are the owner's and Malia's decisions of 2026-11-09. They are recorded in `project-touch-arc-base-first` and are not reopened by the plan.

**Text width**
1. All wrapping goes through one place and follows the device width.
2. The automatic width is at most 80 columns.
3. Templates must work down to 30 columns, to meet WCAG 2.2 AA reflow at 320px. Maps and wide tables are exempt and may scroll sideways.

**Type**
4. One font in the client: JetBrains Mono, with ligatures off. Atkinson Hyperlegible Mono and Fira Mono were second and third. All are SIL OFL 1.1.
5. Two sizes: 16px text and 20px titles everywhere, 14px and 18px on upright phones.
6. The command box is always 16px.
7. No italics. Regular weight for body text and every button. One bold, for titles, quest names, your own character's name and room titles.
8. The site top bar and footer use the same font at 20/16 on desktop, laptop and tablet, and 16/14 on phones.
8a. The other site pages match, for consistency: Home, Who's Online, Architecture, Config and 404 use the same font, the same two sizes, and rule 7. Source Sans 3 and Source Serif 4 retire from the site. The Home logo stays an image. The admin builder pages, the admin dashboard pages and the architecture diagrams keep their own styling (owner, 2026-11-09).

**Layout**
9. Large screens: the game on the left, up to 80 columns, command box under it. To its right, a square Map as the largest panel with Health above Status beside it. Below, 60/40 columns: Comms over Quests on the wide side, Inventory over Automation on the narrow side.
10. Sideways tablets: the Map, Health and Status stay open beside the game. Comms, Quests, Inventory and Automation sit behind the round icon bar and open over the lower right, covering the bottom of the Map when they do. Accepted.
11. Upright phones, sideways phones and upright tablets: the game is full width and all panels sit behind the icon bar.
12. An open tab is at most 30% of the visible height, keyboard up or down.

**Phones**
13. The keyboard stays open after a communication command and closes after anything else. Communication means what was literally typed: `say`, `whisper`, `reply`, `chat`, `newbie`, `trade`, `shout`, `emote`, `party`, and the built-in shortcuts `.`, `/w`, the backtick, `yell`, `scream`, `holler`. Aliases and macros are out.
14. A visible X in the command box closes the keyboard. It shows only while the keyboard is open and is at least 44px wide.
15. A phone turned sideways gets a toast once per visit: "This game is easier to play with your phone held upright."

**Panels**
16. The panel titled Chat becomes Comms. Its Chat channel keeps its name.
17. Map title: `Map (Zone, Level N)`. Under the map buttons, a bar: `Location: <room>` and `Destination: <room, area or zone>`. It may take two rows on a laptop. Zone, level, room and quest names come off the map art.
18. Quests get brass tabs, Active and Repeat, replacing the "Repeatable work" heading. Repeat rows end in "Ready now" or "Not yet", ready first. The tab shows how many are ready, for example "Repeat (2)".
19. The quest list uses the game window's line spacing.
20. Item menu, for right-click on desktop and laptop and for a touch on tablets and phones: the item name is at the larger size, boxed in and separate from the actions, and the action text is indented relative to it.
21. Empty equipment slots stay unshown.

**Template forms**
22. The game picks the widest form that fits. Forms are the ones in design section 2, accepted against the real-device table.
23. Splash art is the full art or words only. There is no small art.
24. No screen moves information behind a separate command.
25. The 55 admin command screens are out of scope for the 30-column rule for now (owner, 2026-11-09), and the template guard exempts `admincommands/`.

## Choices made while writing, for your review

Nothing below was ruled on. Each is my default, and any of them can be changed before the plan.

- **Web width versus `set linewidth`.** A web session wraps at the smallest of its reported columns, 80, and the saved `LineWidth` when one is set. So on the web `set linewidth` can only narrow. Telnet behaves as today. The alternative lets a saved 100 widen a web session, which the 80-column window cannot show.
- **Equipped panel.** Built as mocked: the real 46px columns, with slot labels off. You saw it and moved on, but never said yes.
- **All three right-click menus match** (items, Automation, Quests), because they are near-copies. Rule 20 named only the item menu.
- **Upright tablet strip.** The empty strip right of the 80 columns stays. Filling it would mean changing a locked size for one device.
- **Form picking is automatic.** You chose the forms; measuring each form's longest line against the screen is my way of applying "widest that fits", so nobody maintains a list of widths.
- **Column dragging and panel swapping retire** on large screens, because layout D is a fixed arrangement. Fold and pop-out stay, and Reset layout stays to dock pop-outs. The saved layout key moves to `v2`, so old saved arrangements do not fight the new one.
- **The `rail` mode retires.** Windows that are too small and not touch use the phone layout, as the mock does. A mouse window at least 900px wide and wider than tall gets the full layout.
- **Tablet versus laptop is decided by touch** (`pointer: coarse`), because a Surface Pro is as wide as a laptop. A Surface with a trackpad attached may get the laptop layout.

## Out of scope

The game window renderer, which is deferred (see design section 8). Tracked separately: the bar labels Health, Stamina and Conviction (#831), gold in the Inventory panel (#832), the suggestion popup (#833), fewer comms channels (not filed, the owner's "at some point"). The rest of the touch arc: the command bar (#779), movement by touch (#781), brief rooms and less text (#777, #782).

## Design

### 1. One width (server)

`func (u *UserRecord) WrapWidth() int` is the only source of a wrap width:

- A websocket session: the smallest of its columns (80 until it reports), 80, and `LineWidth` when set, never below 30.
- Everything else: `GetLineWidth()`, which is today's behaviour.

The web client reports columns with a new GMCP message, `Client.Display` `{ "cols": n }`, handled in the `gmcp.go` switch beside `Zone.Map.Resync`. The value comes from the client, so it is treated as untrusted. The handler ignores anything that is not a websocket connection, ignores a payload that is not a whole number, clamps it to 30 to 80 before storing, and acts only when the value changed. It defers to MainWorker by event, like `Zone.Map.Resync`, and writes `Display.ScreenWidth` through `OverwriteClientSettings`, exactly as NAWS does. A client can only ever change its own connection's width. It is per connection, so a phone never changes the player's desktop or saved setting.

The client sends it only when the column count changes, after the size has been stable for 250ms. A height-only change, such as the keyboard, never sends. Mudlet needed this same settling in 2026 because mid-resize widths reached servers.

Sweep, so there is one place:
- The 7 `GetLineWidth` pipeline sites and `userrecord.go:512` read `WrapWidth()`.
- The 27 literal 80s, `set.go`, `roomdetails.go` and `menu.go` take the width from the recipient. Speech, Shout and Emote stop pre-wrapping and join `shouldWrap`, because text sent to a room must wrap per recipient. The comment at `pipeline.go:127-130` explains why they were excluded, and that reason goes away with the pre-wrap. Text sent to one known player (whispers, room descriptions, system lines) keeps its wrap call and passes that player's `WrapWidth()`.
- Authoring limits that reject lines over 80 (`lightnotice/store.go:63`, `darkcombat/store.go:57`) are content rules and stay.

Text already on screen keeps its line breaks. New text uses the new width.

### 2. Templates to 30 columns

A desktop does not change: at 80 columns the widest form is used. Narrower forms appear only on screens that cannot fit it, and on a desktop only for a table already wider than 80 columns, which wraps mid-row today (owner, 2026-11-10). A template may have several forms, and **the game uses the widest form whose longest line fits the session's width.** No per-device rules or width thresholds are written down, so a shop list with short item names stays bordered on a narrower screen than one with long names.

Variants sit beside the base file and are named for the width they are drawn to, such as `character/status.w42.template` and `.w30.template`, tried the way `name.screenreader.template` is. The width joins `templateConfig`, and a width report clears the cache with `ClearTemplateConfigCache`. Forms are only rendered and compared when the base is wider than the session, and only for a template that has forms (owner, 2026-11-10).

Forms by use case, as chosen against real devices (the columns are the usual ones for those devices):

| Use case | Forms, widest first | Who gets what |
|---|---|---|
| Status and other boxed sheets | Boxes (80), two columns with section headings (42), one list with everything shown (30) | Boxes on laptops, desktops and large tablets. Two columns on iPhone 16 and Pixel 8 (42 to 44). One list below that |
| Generated tables (`GetTable`) | Bordered, plain columns (40), two lines per item (30) | Bordered where it fits, usually 62 or more |
| Aligned help pages (170 of the 182) | One form everywhere: label, then its text wrapped with a hanging indent | Every screen. Desktop values no longer line up in a column |
| Splash art | Full art, or words only. There is no small art | Full art where it fits (about 75 columns), words only below that. `Splash_Deliver` renders the art once and already sends screen-reader players the caption, so narrow sessions take that same caption path and no extra render happens |

Every form shows all of the information: no screen moves content behind a separate command.

Measuring limits. `Process` reads and parses a template on every call (`internal/templates/templates.go:212-214`), so each extra form tried costs one more read, parse and execute. Forms are tried only when the base is wider than the session and only for a template that has variants, widest first, stopping at the first that fits, so a phone pays at most three renders for a player-requested screen. Measuring never runs on a per-round or per-broadcast path. Plan 3a measures the cost on the production-sized machine limit before merging, and adds a parsed-template cache if it shows up. Measuring counts columns with `util.VisibleWidth` and executes nothing new: player text is passed as template data, as it is today, and `text/template` does not evaluate data as a template.

Rendered at 30, 42 and 80 columns, no line may exceed the width and no column may break, apart from the exemption in ruling 3.

Three kinds of wrapping sit inside templates, outside plan 1's sweep, and belong to plans 3 and 4:
- **Fixed widths in templates.** 11 lines in 10 template files call `splitstring` with a literal 59 to 73 (plan 3b; `screenwidth` from plan 3a replaces the literal), and `setdesc.go` echoes at 72 to match them.
- **Hard line breaks in authored text.** Of 468 help files, 434 have prose broken by hand into lines of 45 columns or more, and help rides `CategoryScreen`, which the pipeline does not wrap. So plan 4 covers every help page's prose as well as the 170 aligned ones: on a session under 80 columns, the lines of a paragraph are joined and wrapped to the reader's width.
Amended 2026-11-12 (plan 4, owner ruling): help files state their layout with ::rows, ::list and ::keep markers instead of the game guessing paragraph bounds; see docs/superpowers/plans/2026-11-12-webclient-base-4-help-pages.md.
- **Rendering without a reader.** `help.go:222` calls `Process` with user id 0, so plan 4 passes the reader through.

`motdBox` needs a narrow form (plan 3a): seen at 42 columns on 2026-11-09, its spaced-out title is cut off, and URLs too long for the box burst through its sides. `motdrestyle.js` matches the box by its shape and did not remove it at that width, so plan 5 checks it at narrow widths.

A 320px screen at 16px fits only 29 columns because of the game window's padding, so that padding is trimmed at that width to reach 30.

Through the wrap sweep at 80, telnet and Mudlet players get the same words and colors, and nearly the same line breaks. Measured on 2026-11-09, `WrapAnsi` breaks differently from `SplitStringNL` on 745 of 10,925 shipped prose lines (an upper bound, as the sample split some YAML at physical lines), always in the last word or two of a line. The plan's golden test names the accepted differences. `WrapAnsi` also closes and reopens color tags at each break. After the template rewrite, output is correct at 80. The telnet reader of an aligned help page gets the same wording in the new form.

### 3. Type

Design tokens in `gomud.css`, and nothing else sets a size in the client: `--text` (16px, 14px on upright phones), `--title` (20px, 18px on upright phones), `--cmd` (16px), and `--bar-top` and `--bar-foot` (20 and 16, 16 and 14 on phones). `--font-body`, `--font-heading` and `--font-mono` all become JetBrains Mono at `:root`, so the client and the site pages that share `gomud.css` change together. The same tokens replace the page sizes in `gomud.css` and the three in `architecture.html`.

JetBrains Mono is self-hosted in `static/fonts/` with its license text beside it, unmodified, and must be the complete font. The Google Fonts version omits box-drawing characters, which misaligned boxes in a mock. `BRAND_STANDARDS.md` section 3 is rewritten, including its 15px button rule.

Exceptions, on purpose: map art that scales with the map (the compass N and the shop and bank symbols), and icon glyphs.

### 4. Game window

`termfit.js` becomes: read the font size from the token, set it once, and set columns to what fits, at most 80. The shrink loop, the CSS `scale()` and the character-measure monkeypatch (`WP:461-483`) go. The size no longer depends on panel width, which also fixes the 200% zoom item on #730.

The fixed-at-80 construction option at `WP:432` goes with it.

### 5. Layout

`_computeMode` in `dashboard.js` returns:

| Mode | Condition |
|---|---|
| `wide` | Window at least 900px wide and wider than tall, screen orientation landscape, shorter screen side at least 600px, not touch |
| `tablet` | The same, on touch |
| `phone` | Everything else |

Both orientation checks are required. A device preset reported landscape while upright in testing, and the window shape alone is wrong with a keyboard up.

The layout is CSS grid as built in the mock. Map width is the smaller of 58% of the panel area and 52% of the height the panels have.

### 6. Phones and tablets

- `--drawer-cap` becomes 30% of the visible height.
- `shouldHideKeyboardAfterSend` gains the typed command and applies ruling 13.
- The X is a button in `#panel-cmd`, shown by `html.keyboard-open`. It calls `blur()`.
- The sideways toast uses the existing `dash-toast`, once per visit, and judges orientation from `screen.orientation` so the keyboard cannot trigger it.

### 7. Panels

- **Map:** extend `WP:933` with `obj.Map.cz`. Remove the four text draws from `gmcp.js`. The bar reads `Room.Info`'s `name` and the focused quest's `target_label`, falling back to `name`. With no focused quest, the Destination half is left out.
- **Quests:** split on the existing `status` field. The "(repeatable)" tag comes off the finished rows and stays on active ones.
- **Item menus:** a tap on a tile opens the same menu as right-click, which is the part of #780 this work needs. Rule 20's styling goes on the shared menu CSS.
- **Comms:** the title changes at `WP:352` and in the phone tab label.

### 8. Renderer comparison (after the base)

With width, type and layout in place, xterm at a fixed size and a throwaway HTML feed are compared on Malia's phones with the same combat log, at 1,000 and 3,000 lines: scrolling, selection, and input delay. That result is a separate decision and a separate spec. #733 (screen readers) is a requirement on whichever wins.

## Plans

One spec, one plan per slice. Each ships on its own and leaves the game working. Width reporting turns on only in plan 5, after the templates are ready, because a narrow width sent earlier would break boxes and tables on phones.

| Plan | Slice | What players see | Needs |
|---|---|---|---|
| 1 | Server wrap width: `WrapWidth`, the `Client.Display` handler, the sweep, telnet golden test | Almost nothing: a few line breaks at the end of lines move, and a saved `set linewidth` now also applies to speech, whispers, room descriptions, menus and `set` | None |
| 2 | Type system: JetBrains Mono, size tokens, no italics, button weights, top and bottom bars, the site pages, brand standard | The client and site look like one design. Game text still changes size with the window | None |
| 3a | Screen machinery: the form mechanism, `screenwidth`, generated tables, splash to words only, the MOTD box, the template guard (split from plan 3, owner 2026-11-10) | Nothing yet, since no client reports a narrow width | 1 |
| 3b | The hand-laid screens: narrow forms for status, inventory, skills and the other character and description screens, mail, the skill and stat banners, the auction broadcasts, the Go-built screens, and every table now wider than 80 columns redesigned to fit (owner, 2026-11-10), shown as a mock first | Nothing yet | 3a |
| 4 | Help pages to 30 columns, in batches: the aligned details, and paragraph rewrapping for hand-broken prose | Help details change form at every width | 3a |
| 5 | Game window goes live: fixed text size, columns follow the window, the client reports its width | Readable phone text, and 200% zoom works | 1, 2, 3a, 3b, 4 |
| 6 | Layout and phones: large-screen layout, sideways tablets, retiring rail and column dragging, 30% tabs, keyboard rules, X, toast | The new arrangement | 2, 5 |
| 7 | Panels: Map title and bar, Quests tabs, Comms rename, compressed Equipped, item menus by tap and right-click | The panel changes | 2, 6 |

The renderer comparison (section 8) follows these, under its own spec.

## Testing and guards

Rulings are enforced where they can be, not just remembered.

- **Telnet:** a golden test pins the visible text and line breaks at 80, with the accepted differences named one by one.
- **Width:** a guard fails any new hardcoded wrap width in Go.
- **Templates:** a guard renders every template at 30, 42 and 80 and fails on an over-long line, with an explicit exempt list for maps and wide tables.
- **Type:** a guard fails any `font-size` outside the tokens and any italic in client or site CSS.
- **Client:** `tools/jstest` covers the column fit, the mode table, and the keyboard list.
- **WCAG 2.2 AA checks:** 200% zoom, 320px reflow, and contrast at the new sizes. #729's color list is not changed by this work.
- **Real devices:** Malia tests on every device we have available before each client phase merges, including iOS, which has not been tested on the real keyboard. The work is assigned to her and tracked in GitHub.
- `set linewidth` above 80 still works on telnet, and narrows on the web.
