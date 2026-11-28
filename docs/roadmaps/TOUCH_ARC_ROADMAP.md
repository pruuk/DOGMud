# Touch arc roadmap: playing by touch on phones and tablets

Epic #783. Written 2026-11-19 from a brainstorm with the owner and Malia.
This is a **roadmap, not a spec**: it sets the design direction for every
device, orientation and panel, and the work breakdown (WBS) of the specs and
plans that build it. Each spec in the WBS settles its own details, starting
from the open questions listed here.

Mockups (open in a browser; drawn 1:1 in CSS pixels unless they say
otherwise): `docs/roadmaps/touch-arc-mocks/`.

| Mock | Shows |
|---|---|
| `s21-layouts-v2.html` | Galaxy S21: the Map tab with the map pushed left and a compass; the two deck shapes tried (the inverted T won) |
| `s21-deck-v3.html` | The out-of-combat deck: inverted T, other-exits button, fixed and contextual commands (colours superseded by the brass rule, see Look and feel) |
| `s21-combat.html` | In combat: X, the deck swaps in place; Y, the Health tab carries the combat buttons |
| `s21-comms.html` | Comms: tap a message to reply (screen 2 was chosen) |
| `s21-submenus.html` | Three-level contextual menus, Cast and Craft, in the brass look |
| `tablets-v2.html` | Upright tablet, open foldable, sideways tablet, sideways phone (half size) |
| `s21-moving-by-touch.html` | Spec 2 (#781): the T with its states (lit, dim, padlock, dark, ✕), the other-exits level, the Map tab copy, 320px, and the sideways phone's Menu (owner approved 2026-11-22) |

Reference screenshots of production on real phones (kept locally, not in
git: `_archive/` is ignored):
`_archive/screenshots/phones-2026-11-19/Samsung_Galaxy_S21_5G/` (Malia) and
`_archive/screenshots/phones-2026-11-19/Pixel8/` (owner).

## Facts verified against source and screenshots

| Fact | Where |
|---|---|
| Movement is refused while in combat; only flee leaves a fight | `internal/usercommands/go.go:67-79` |
| Flee takes no direction: the game picks the exit | `internal/usercommands/flee.go:32-33` (`BeginFlee(..., "")`) |
| The existing `target` command switches the **combat** target and records aggression and an assault crime against the new target | `internal/usercommands/target.go` (`bumpOpinionOnTargetSwitch`, `RecordAssaultCrime`) |
| Enchant crafting takes an optional slot (`craft <recipe> ring#2`); with none it auto-picks when exactly one equipped item fits and otherwise asks which | `internal/usercommands/enchant_slot.go:23-73`, `craft.go:craftEnchanting` |
| 64 spells, every one in at least one of 5 schools: mental 17, vital 14, manifestation 14, elemental 10, enhancement 9 | `_datafiles/world/dogmud/spells/*.yaml` (`schools:`) |
| 6 crafting disciplines: alchemy 37 recipes, enchanting 24, tailoring 20, blacksmithing 19, jewelcrafting 18, cooking 9 | `_datafiles/world/dogmud/recipes/*/` |
| 1,355 rooms. Exits: north 701, south 692, east 627, west 627, down 88, up 87, named 13, diagonal 22 (in 18 rooms) | `_datafiles/world/dogmud/rooms/*/*.yaml` |
| `say` and the channel commands do not parse `@name`; it would print as literal text | `internal/usercommands/say.go` |
| Seven round tabs on a phone: Map, Quests, Health, Inventory, Comms, Conditions, Automation | screenshots; `static/js/dashboard.js` `PANEL_TABS` |
| Phone drawer today: capped at 30% of the visible height (the Map at 45%); command box area 64px, tab bar 60px, tab name strip 36px | `static/css/dashboard.css:752-760`; `docs/guides/BRAND_STANDARDS.md` section 4 |
| Galaxy S21: 360 x 800 CSS px at 3x. Text rows about 22.4px. Session bar 58, "Game" title bar 32, command box 62, name pill 30, tab bar 56. Keyboard about 45% of the screen. Map art about 264 x 244, centred, about 48px empty each side | Malia's screenshots, measured |
| Pixel 8: 412 x 915 CSS px. Full screen, no tab open: about 28 rows. Gboard about 41% | owner's screenshots, measured |
| Chrome's address bar and Android's navigation bar cost about 130px on the S21 when not in full screen | screenshots |
| In Chrome's Desktop site mode the game frame shrinks to about a third of the screen (zoomed out); Android tablets default to Desktop site | #927; `static/js/viewportfit.js:108` |
| Brand: brass marks anything pressable; pressed dark brass with cream text means open or selected; red is for warnings only; touch targets at least 44px, with a 30px exception for phone sub-tabs and panel title buttons; line icons, never emoji | `docs/guides/BRAND_STANDARDS.md` sections 2 to 5 |
| Existing client tests and guards: `tools/jstest/*.test.js` (including `longpress`, `viewportfit`, `layoutpick`, `termfit`), `tools/webclient-tests/*.js` (run by CI), `internal/guards/client_type_guard_test.go` (fonts, sizes, weights) | `.github/workflows/validate.yml:107-128` |
| Headless screenshots of the real client against a local server: `tools/webshot/live-shot.js` (headless Edge, DevTools protocol) | `tools/webshot/README.md` |
| #730's only open item is the Automation on/off switch size (Malia, 2026-11-19) | #730 |

## The frame (every touch device)

**When the touch controls show.** By main input: devices driven mainly by a
finger (`pointer: coarse`: phones and tablets) show the controls deck;
touchscreen laptops and narrow desktop windows do not. A player setting
overrides it: automatic, always, never.

**The deck.** A block of buttons directly above the input bar. On a phone it
is two rows of six 48px buttons (328 of the S21's 344px).
- **Out of combat:** movement as an inverted T on the left (north above
  south, up and down either side of north), an **other exits** button beside
  it (shown only when the room has diagonal or named exits; its slot is kept
  when empty so nothing moves), and commands on the right.
- **In combat (default X):** the movement buttons and the other-exits slot
  collapse into one **Flee**, an ordinary button in one slot in the top row
  (owner and Malia, 2026-11-19: consistency over making Flee prominent),
  and the freed slots hold combat commands. Movement is refused in combat
  anyway.
- **In combat with Health open (Y):** if the player opens the Health tab
  during a fight, the tab carries Flee and the combat buttons beside the
  bars, and that becomes the player's combat default; closing Health during
  a fight returns to X. Y is not the default because many companions or
  party members make the Health tab too tall. The choice is stored on the
  device (browser storage, keyed by account name).

**The input bar** stays visible under the deck, with a wider gap and a
divider between them. Tapping it opens the keyboard.

**The keyboard.** It covers the deck and the tab bar (nobody types and taps
buttons at once). Real keyboards take 40 to 45% of the screen. Where typing
goes decides what stays: typing in the command box makes an open tab **step
aside** until the keyboard closes; typing in a tab's own field (Automation's
Advanced view) keeps the tab and the game text gives up its space.

**Tabs (panels) on a phone.** An open tab covers the deck. The Map tab
carries the movement buttons beside the map (the map pushed left, keeping
its size as far as the buttons allow). Closing a tab brings back the deck,
or the player's combat default during a fight.

**Contextual menus.** At most three levels (More pages do not count as a
level). A level replaces the whole deck. **Back is always top-left** and
names where it goes; **More is always bottom-right** and appears only when
the list does not fit. Each level keeps its items in fixed slots as far as
possible. Flee lives only on the top level: to flee from a submenu, go Back.
- **Game menus on the deck** (owner, 2026-11-21; plan 3h): while a game
  menu is open (a talk menu, a quest offer, a yes or no question, anything
  drawn by the one menu primitive `internal/menu` and announced to the web
  client as `Char.Menu`), the deck shows its choices instead of the usual
  buttons: Back top-left, then 1, 2, 3 and so on, and More bottom-right when
  they do not fit. The server builds them like every other button (3a). To
  settle in its spec: whether Back is the menu's own Leave entry, or steps
  out of a sub-level (a quest offer back to the talk list) while Leave stays
  a numbered choice. Needs a mockup Malia approves, like every menu spec.
- **The phone's top level out of combat** (owner, 2026-11-19): movement
  and other exits take seven slots; the other five are Target, Interact,
  Attack, Cast and More, in that order. More holds Craft, status, help and
  other common commands; anything more esoteric is typed. Wider screens add
  contextual slots.
- **Cast** is always on the top level. A short tap repeats the last cast
  (spell and target). A long press opens the five schools and **Rhetoric**
  (treated as one more school: taunt, rally and the other conviction
  moves), then what the player knows in it (too costly ones dimmed).
- **Interact:** talk, look, consider, search, open, lockpick, assess and the
  like, as a submenu of the things here and what can be done to them.
- **Craft** lives under More on a phone (on the top level where there is
  room). A short tap repeats the last craft; a long press opens the
  disciplines the player knows a recipe in, then the recipes (dimmed when
  materials or the station are missing).
- **Target** is a top-level button: kind (Item, Slot, Room, Creature), then
  what can be targeted of that kind, visible things only. The target is
  saved until changed; Cast and Craft use it by default; area effects
  ignore it; anything that makes no sense against it is refused as if
  typed. It is a **new, harmless server-side selection**, never the
  existing combat `target` command. The Slot kind lists only what an enchant
  can need: rings, weapons and wrists the player wears, and only when there
  is more than one (`ring#2`, `weapon#1`, `wrist#2`).
- **Look** (inside Interact on a phone): tap looks at the room; long press
  lists what is here to look at.
- **Attack moves:** tap attacks; long press offers the other moves against
  that target (trip, bash and the rest), cooldowns dimmed.
- **Buff renew:** tap renews it now; long press makes it renew itself
  whenever it drops (a visible trigger in Automation).

**Fat-finger rules.** 48px targets with 8px gaps everywhere except the 30px
sub-tab and title-button exception. A wider gap and a divider between the
deck and the input bar. Fixed slots: the first four or five top-level
buttons keep their place and job, the rest are contextual. Consequential
buttons (Flee, Attack, drop, sell, abandon) sit farthest from the input bar.
The client names the button it pressed and the server refuses a stale tap
(target gone or changed). A second tap on the same button within 0.5s is
ignored.

**Phones in general.** Strongly encourage full screen (it gives back about
130px). Drop the "Game" title bar on phones (32px, does nothing there). Tab
sub-tabs (Inventory, Comms) sit in one row that scrolls sideways instead of
wrapping to two (about 30px each).

## Devices and orientations

| Device | Direction |
|---|---|
| Upright phone (S21, Pixel 8) | The frame above. About 20 game rows on the S21 in full screen with the deck; about 9 with the Map open |
| Sideways phone | Nearly unusable once the keyboard opens, but typing is sometimes needed, so it is supported minimally: session and tab bars fold away, the deck becomes a side column. With the Map open, the upright treatment: movement inside the Map panel, other commands hidden until it closes |
| Upright tablet, open foldable | The phone deck, wider: two rows of 12 to 14 columns, so more contextual slots. Panels behind the tab bar as today. A folded foldable's cover screen is a phone |
| Sideways tablet | Map, Health and Conditions stay open beside the game as today, and **the deck stays above the input bar** like every other orientation (not on the Map), so the Map stays large |
| Touchscreen laptop, desktop | No deck by default (main input is a mouse or trackpad); the setting can force it on |

## Panels

| Panel | Direction |
|---|---|
| Map | Movement buttons beside the map, map pushed left. No Fit, Center, zoom row on touch (as today); Legend in the title bar |
| Health | The combat Y variant (bars plus Flee and combat buttons) when opened during a fight |
| Conditions | Each condition is a button; a long press offers contextual trigger, tick and alias options (for example, renew this buff whenever it drops) |
| Automation | Button-forward: contextual buttons with tap and long-press options. An **Advanced** button opens today's Ticks, Triggers, Macros and Aliases screens. This replaces the small on/off switches, #730's last open item |
| Inventory and Equipment | Tapping an item shows its actions as buttons inside the panel (Wield, Look, Drop, Sell; Drink, Give); a long press shows the full list and details. Sub-tabs in one scrolling row |
| Comms | Tapping a message replies to it: the command box is prefilled and the keyboard opens (Comms steps aside, as every tab does). Whisper: `whisper <name> `; every public channel the player is not muted on (say, party, shout, chat, newbie, trade): `<channel command> @<name> `. Broadcasts cannot be replied to. Channels in one scrolling row |
| Quests | Tapping a quest shows Focus (points the Map's Destination at it), Hint and Abandon (far from the input bar, confirms) |

## Look and feel

Conform to `docs/guides/BRAND_STANDARDS.md`:
- **Every button is brass** (shiny brass, dark text), the brand's mark for
  anything pressable. Pressed dark brass with cream text stays reserved for
  open or selected (and a button's brief pressed state).
- **Kinds are told apart by shape and edge, never by colour alone:**
  movement square; contextual commands rounded; other exits dashed edge;
  Back and More heavy square edge. Flee looks like any other command (no
  special edge or size). A small ◢ in the
  corner marks a button that opens a deeper level or does more on a long
  press.
- **Sizes:** 48px targets with 8px gaps; the 30px phone sub-tab and title
  button exception stays (Malia, 2026-11-19). Labels at the client's text
  size, JetBrains Mono, regular weight, no italics.
- **Icons:** simple line icons in one style, never emoji (the mocks use
  emoji as placeholders). **Red only for warnings**: cooldowns are dimmed,
  not red. Green and blue never carry meaning.
- **Brand doc changes** (shipped with the first touch plan): touch targets
  48px with 8px gaps (from 44px); the button kinds and their shapes; the ◢
  mark; the deck; the Map's movement buttons.

## How we test

**Headless (Claude).** `tools/webshot/live-shot.js` against a local server
(never the owner's), at a fixed size matrix: **small iPhone in Safari
375 x 548** (the shortest; see below), S21 360 x 800 (the narrowest),
Pixel 8 412 x 915, sideways phone 800 x 360, iPad 820 x 1180 both ways,
open foldable about 690 x 840, laptop 1366 x 768. **The minimum phone the
design must fit is pinned at 360 wide by about 550 tall** (the narrowest
and the shortest together, owner 2026-11-19). iPhones in Safari cannot use
full screen (Safari has no full-screen mode for pages on iPhone), so they
always keep Safari's bars: an iPhone SE's 375 x 667 screen leaves about
375 x 548 for the page (the commonly reported figure, not measured here).
Chrome's built-in "iPhone SE" device is the whole screen, so this size is
set as a custom device. Nobody has a small iPhone to test by hand, so it is
checked in device mode only. Touch emulation must be turned
on explicitly (headless Chrome is not `pointer: coarse` without it). There
is no on-screen keyboard headless, so keyboard states are approximated by a
shorter viewport and always confirmed on a real phone.

**By hand (owner and Malia).** On the Galaxy S21 and the Pixel 8 for every
plan before it merges (tracked on #834). **Contingent:** a refurbished iPad
and/or Android tablet if one is bought; until then tablet checks are
headless only and say so.

**Acceptance criteria (every touch plan).**
- Every touch target at least 48px with 8px gaps (30px sub-tab exception).
- On the S21 in full screen: about 20 game rows with the deck, about 9 with
  the Map open. At the pinned minimum (360 x about 550): the deck and input
  bar fit with no scrolling and the game keeps at least about 9 rows with
  the deck (tight; the less-text work matters most here). Nothing the
  player needs is covered with the keyboard up.
- No sideways page scroll; text contrast at least 4.5:1, icons and focus
  rings at least 3:1; green against blue never carries meaning.
- Tap and long press do what this roadmap says; a stale tap is refused; a
  double tap does not fire twice.
- Telnet and Mudlet are unchanged; desktop layout D is unchanged.
- Server pushes only on change (prod lag history).

## Tests and guards

**The rule that keeps client and server in step: the client never builds
commands.** The server builds every button at every level and sends it with
an id; the client sends back only the id; the server turns the id back into
a command through the same code that parses typed commands, and refuses it
if it is stale. (Comms' reply prefix is sent by the server with each
message, for the same reason.)

Guards (approved by the owner, 2026-11-19), each with a can-fail case:
1. **Round trip:** fixed scenarios (room with an NPC, a fight, a forge, a
   caster, a shop) generate every button at every level; each command runs
   through the real parser and must be recognised. Renaming or dropping a
   command breaks the build.
2. **Coverage:** every player command in the registry has a button provider
   or sits on a "typed only" list with a reason. A new command fails CI
   until someone decides.
3. **Data-driven lists:** menus come from game data (spells by school,
   recipes by discipline), so new spells and recipes appear on their own; a
   guard requires every spell to have a school and every recipe a
   discipline.
4. **Menu shape:** at most three levels, Back top-left, More bottom-right,
   the same item in the same slot.
5. **Sight:** a hidden creature or one in the dark yields no button.
6. **Touch targets:** a jstest measures every button at the size matrix
   against 48px and 8px gaps.
7. **Client behaviour:** jstests for tap, long press, stale refusal, double
   tap, the keyboard rule, and the combat default.

Existing guards carry on: `client_type_guard_test.go`, the wrap-width
guards, `tools/jstest`, `tools/webclient-tests`.

## Open questions for the specs

1. **Settled (owner, 2026-11-19): the phone's top level out of combat.**
   Movement and other exits take seven slots, leaving five, in this order:
   **Target**; **Interact** (talk, look, consider, search, open, lockpick,
   assess and the like); **Attack** (tap attacks, long press opens special
   moves); **Cast** (spell schools plus **Rhetoric** treated as one more
   school: taunt, rally and the other conviction moves); **More** (Craft,
   status, help and other common commands; anything more esoteric is
   typed). In combat, Flee takes one ordinary slot and the rest of the
   combat top level is for the command bar spec to settle (Target, Interact, Attack,
   Cast and More keep their places where they can, plus contextual combat
   slots such as a buff to renew).
2. **Settled (owner, 2026-11-22, spec 2): the inverted T.** Was: **The Map's movement buttons:** the inverted T (same as the deck, about
   160px wide, map shrinks) or a two-column compass (about 104px, map loses
   about 12%). Consistency favours the T.
3. **How the server ranks contextual commands** for the flex slots, and how
   a slot keeps its place as the moment changes.
4. **Auto-renew triggers:** triggers match text lines today; renewing a buff
   when it drops needs a trigger on the condition ending, or a reliable
   line.
5. **The full-screen nudge:** a one-time prompt on phones (like the
   sideways toast), and what it says.
6. **Desktop site mode** (#927): fix the frame, and decide whether tablets
   in Desktop site need anything more.
7. **What a tap on Talk does when several NPCs are present**, and similar
   one-of-many choices inside the three-level limit.
8. **Narrower than 360px.** WCAG's reflow check is 320px wide (the
   templates already go down to 30 columns). Two rows of six 48px buttons
   need 328px plus the deck's padding, so they do not fit at 320. Options:
   44px buttons with 5px gaps there (289px, still above the WCAG 24px
   minimum), or fewer columns and a third row.

## Work breakdown

Order follows the owner's arc order (2026-11-19, on #783): layout first,
then movement, then the command bar, then panels, then less text. Every
plan ends with the headless size matrix and Malia's device check (#834).

| # | Spec | Plans (each one PR) | Needs | Main side |
|---|---|---|---|---|
| 0 | none (bug) | #927: Desktop site frame height | none | client |
| 1 | **Touch frame** (#783) | 1a deck container and input bar per device and orientation (empty slots, sizes, setting, main-input detection); 1b keyboard rule and tabs stepping aside; 1c phone polish: no Game title bar, scrolling sub-tabs, full-screen nudge, brand doc update | 0 | client |
| 2 | **Moving by touch** (#781) | 2a server: exits for the player's room, sight-gated, pushed on change; 2b deck movement (inverted T, other-exits list); 2c the Map tab's movement buttons (2a, 2b and 2c built, 2026-11-27; plan 2c: `docs/superpowers/plans/completed/2026-11-27-moving-by-touch-2c-map-and-menu.md`, PR #967 merged 2026-11-27) | 1 | both |
| 3 | **Command bar and contextual menus** (#779) | 3a button model and protocol: ids, levels, Back/More, stale refusal, double tap, and guards 1, 2, 4, 5 skeleton; 3b client renderer: slots, long press, levels; 3c first providers: Look, Talk, Score, Consider, attack moves; 3d combat swap X (Flee) and Y (Health), combat default on the device; 3e Target selection; 3f Cast and Craft menus (guard 3); 3g buff renew and auto-renew; 3h game menus on the deck (an open talk menu, quest offer or question shows its choices as buttons) | 1, 2 | both |
| 4 | **Panels by touch** (#780), one short spec per panel | 4a Comms reply (server-sent prefix); 4b Inventory and Equipment item actions; 4c Quests; 4d Conditions buttons; 4e Automation button-forward with Advanced (closes #730) | 3 (3g for 4d and 4e) | both |
| 5 | **Less text** (#777, then #782) | 5a measure a phone session; 5b brief room text, combat verbosity, and a shorter prompt on small screens (owner 2026-11-21: shorter bars and labels, so the prompt fits one line on a phone instead of wrapping to two or three); 5c the rest, ranked by the measurement | 1 | server |

Each spec is written, reviewed and approved before its plan; each plan opens
in VS Code for review before it is built (project rule).

**Any spec that has a menu needs a detailed mockup that Malia approves**
(owner, 2026-11-19), before the spec is approved: every level, Back and
More, at the pinned minimum phone and the S21, in the brass look, with real
labels from the game data. This covers the command bar (3), Target (3e),
Cast and Craft (3f), and every panel spec (4).

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| A short tap on Craft repeats the last craft; with no previous cast or craft, a tap opens the list | By analogy with the owner's Cast ruling | Craft taps always open the list |
| The combat default is keyed by account name in browser storage | Two accounts on one phone should not share it | Key by device only |
| A contextual level replaces the whole deck, movement included | More slots per level; Back returns | Keep movement visible and shrink levels to the command side |
| The other-exits slot stays reserved when empty | Fixed slots: nothing moves between rooms | Give the slot to a command when empty |
| Cooldowns shown by dimming, not red | Brand: red only for warnings | A different cooldown mark |
| Broadcasts cannot be replied to | They are system messages | Reply to the sender where there is one |
| An open foldable is treated as an upright tablet | Similar size and shape | Its own layout |
| Headless keyboard states approximated by a shorter viewport | No on-screen keyboard headless | Device check only |
| The size matrix, and the row counts (S21 measured from screenshots; about 9 at 360 x 550 estimated) as acceptance numbers | The S21 is measured; the small iPhone in Safari (375 x 548) is the commonly reported figure and its rows are estimated from the S21's strip heights | Measure on a real small iPhone if one becomes available; re-measure per plan |
| Mocks before the brass rule (`s21-layouts-v2`, `s21-deck-v3`, `s21-combat`) show dark movement tiles, and `s21-combat` shows Flee in two slots | Drawn before the brass and one-slot Flee rulings; the layouts still hold | Redraw them; the menu specs' approved mockups replace them anyway |
