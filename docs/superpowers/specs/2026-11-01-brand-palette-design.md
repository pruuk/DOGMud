# DOGMud brand palette, from three reference images

**Status:** draft for Malia's review, 2026-11-01. Nothing is applied yet.
**Owner:** Malia (look and feel). **Calabe's share:** none. Client CSS and JS only, no server work.
**Scope:** every colour token in `_datafiles/html/public/static/css/gomud.css` and the map palette `RoomGridSVG.LEATHER` in `static/js/gmcp.js`, **except the terminal's 16 colours** (`--term-*`), which stay as they are apart from the dim grey (see Decisions).
**Builds on:** PR #741 (colour tokens). Related: #729 (contrast).

## Why

The site and client use 125 colour values outside the terminal: 181 token entries, 140 distinct values. They grew one panel at a time, so the same gold appears under five names and a dozen dark browns differ by a shade. Some are GoMud leftovers that do not fit the theme at all (pure cyan mob names, magenta broadcasts, Material-design health bars, a grey legacy menu). This draft maps every one of them to a small brand palette drawn from three reference images Malia chose.

## Sources

| Reference | What it gives | Notes |
|---|---|---|
| Aelbert Cuyp, river landscape with horseman (`dogmudRedo/mood/thumb-1920-508256.jpg`) | **The base:** golds and ochres (45% of the image), olive greys (30%), warm umber darks, cream highlights | No pure white or black; the darkest is umber `#19140c`, the brightest cream `#efd897`. Old varnish yellows the whole image and there is no true blue. |
| Salvador Dalí, *The Elephants* (`orangebgSpiders.ai`) | **Warm meaning colours:** crimson, red-orange, orange, golden yellow | The sky is one continuous ramp. Its reds lean orange, so danger takes the crimson end to stay distinct from warnings. |
| *Pachinko Wizard* screenshot (`Pachinko_Wizard.png`) | **Cool meaning colours:** the hues of green, teal and sky blue | Its colours are neon (chroma 80 to 98). Only the hue is used; saturation is brought down to the paintings' level (chroma 24 to 34). |

Method: 32,400 pixels sampled per image, grouped in CIELAB, and every current colour compared with CIEDE2000 (under 5 is barely noticeable, over 15 is clearly a different colour). Contrast is the WCAG ratio against the dark panel `#201913` and the light vellum card `#d9c39e`; text needs 4.5.

## The palette

### Base (Cuyp)

| Role | Ramp, dark to light |
|---|---|
| Umber darks (surfaces) | `#19140c` `#252015` `#39301b` `#4a3d1e` `#5d502e` |
| Gold and ochre | `#473019` `#736133` `#8c7f52` `#a48d4e` `#aa8636` `#b5a26c` `#dfcc94` |
| Olive greys | `#5d5c4e` `#7e8073` `#969889` `#b4b4a3` `#cec6aa` |
| Highlight | cream `#efd897` |
| Secondary text (tuned to pass) | gold `#96885f` (4.95 on dark), olive grey `#8b897a` (4.92 on dark) |

### Warm meaning colours (Dalí)

| Role | Ramp, dark to light | Use |
|---|---|---|
| Crimson | `#700f1e` `#7e1f21` `#962a22` `#ad2c1f` `#b84f3e`, vivid `#bc2915` | danger, alerts, low health |
| Red-orange | `#983b1f` `#b4451f` `#c44f1b` `#d25c1e` `#dc6b34` | frames, locked doors, low stamina |
| Orange | `#a86331` `#db6f1d` `#e17d2b` `#e8892e` `#e99741` | warnings, stamina |
| Golden yellow | `#ac7c39` `#ea9c2b` `#e7a747` `#e6b058` `#e7b86c` | focus bar, player names, quest marker |

### Cool meaning colours (Pachinko hues, toned)

| Role | Steps (L30, 42, 55, 66, 76) | Use |
|---|---|---|
| Green, hue 131 | `#2f4f1c` `#4b6c37` `#6c8d56` `#88aa71` `#a3c68b` | online, full health |
| Teal, hue 177 | `#085044` `#2e6e60` `#508f81` `#6dac9d` `#87c8b8` | party, copper accent |
| Sky blue, hue 250 | `#004d6f` `#1f6a8e` `#498bb1` `#68a8cf` `#84c3eb` | water, mob names |

**Readability rule for all three cool ramps:** on the dark panels use L66 and lighter (6.6 and up); on the vellum cards use L30 (5.3 to 5.5).

## Decisions taken in this draft

- **Danger keeps a clear red** (Malia, 2026-11-01: a high tone of crimson). Low health becomes `#e35349`, the crimson hue at a lighter value: 4.6 on the dark panel, and well apart from mid stamina `#db6f1d` (ΔE 17.7) and the card frame (12.3). The photo's nearest red-orange sat on top of mid stamina.
- **Yellows keep their distinctions.** Player names, health mid, focus low and focus mid take different steps of the gold ramp instead of collapsing into one.
- **Foes and friends never share a hue.** Party stays teal; mob names move from pure cyan to sky blue.
- **The vellum status bar becomes readable.** "Connected", "Not connected" and "Reconnecting/Leaving" get dark steps: `#2f4f1c` (5.4), `#8a2828` (5.1), `#704714` (4.7). All three fail contrast today.
- **Six text colours that fail today are fixed** with the two secondary-text steps: secondary text, quest progress, "No active quests", toast subtitle, toast hint, whisper chat. One of them (`--text-secondary`) would otherwise have regressed from 5.1 to 4.4.
- **No pure white or black** (Malia, 2026-11-01: white tinted with yellow). `--white` becomes `#fef9ee`, almost white with a hint of the Cuyp cream (ΔE 5.3 from pure white). The black shadows become umber.
- **Broadcast chat uses the terminal's magenta** (Malia, 2026-11-01). The bright one, `#c890b0` (6.7 on dark), because the normal `#a8678f` fails (4.13).
- **One terminal colour changes: the dim grey** (Malia, 2026-11-01, after reading the login screen on her phone). `--term-bright-black` goes from `#6b5a48` (2.96 on the terminal background, fails) to `#e0d2b2`, the default game text colour (`--term-fg`, the "Welcome to" line). Tried first: `#9a8a74` (5.8) and `#d8c8a8`, still too hard to read at phone size. Dim text (hints, notes, dividers, about 570 uses in 500 game files) now looks the same as normal text; the other 15 terminal colours stay as they are.

## Open decisions for Malia

1. **The map biome tints** (city, forest, swamp…) are not in this table. They live in `gmcp.js` as a separate list. Measured 2026-11-01, they fail for many users: 1.0 to 1.6 against the map background and 1.0 to 1.3 between biomes, where shapes need 3:1 (WCAG 1.4.11). Hills/desert (ΔE 1.8), city/road (2.3) and forest/swamp (2.6) are near-identical even to normal vision. The room ink on top passes (5.3 to 7.0). Redesign rules: separate biomes by lightness as well as hue, add a pattern per biome, and keep one tint set per theme.
2. **Light theme.** Malia wants a dark/light switch for players' eye conditions, so every colour here needs a light-theme counterpart checked the same way. Not yet in this draft. Scope (Malia, 2026-11-01): the option lives in the site's settings and covers everything except the game text window, which always stays dark with its 16 terminal colours. No server or xterm.js work, so no work for Calabe.
3. **Once applied, check on screen:** the 7 uses of `--white`, and the low-health fill `#e35349` against its track on a phone.

## Full mapping

Generated from the measurements. "On dark panel" and "On vellum" are contrast ratios for the **new** value; 4.5 is the minimum for text.

### Visible changes (59)

Difference of 5 or more: you will see these change.

| Token | Current | New | Source | Change (ΔE) | On dark panel | On vellum | Note |
|---|---|---|---|---|---|---|---|
| `--auto-label` | `#c8b98a` | `#dfcc94` | Cuyp | 5.3 | 10.9 | 1.1 |  |
| `--bar-top` | `#241813` | `#281602` | Cuyp | 5.4 | 1.0 | 10.2 |  |
| `--brass-border` | `#5e431a` | `#50371d` | Cuyp | 6.0 | 1.6 | 6.4 |  |
| `--brass-dark` | `#8a6620` | `#736133` | Cuyp | 7.7 | 2.9 | 3.5 |  |
| `--brass-glint` | `#fff6d4` | `#efd897` | Cuyp | 10.3 | 12.3 | 1.2 |  |
| `--brass-glint-text` | `#fff4ce` | `#efd897` | Cuyp | 9.1 | 12.3 | 1.2 |  |
| `--brass-mid` | `#cb9f42` | `#aa8636` | Cuyp | 8.7 | 5.1 | 2.0 |  |
| `--gold-button-light` | `#f3e3b6` | `#efd897` | Cuyp | 5.3 | 12.3 | 1.2 |  |
| `--gold-rule` | `#c9a86a` | `#b5a26c` | Cuyp | 5.1 | 6.9 | 1.5 |  |
| `--ink-black` | `#0c0c0d` | `#19140c` | Cuyp | 5.4 | 1.1 | 10.7 |  |
| `--ink-gold` | `#c9a86a` | `#b5a26c` | Cuyp | 5.1 | 6.9 | 1.5 |  |
| `--menu-bg` | `#2e2e2e` | `#252015` | Cuyp | 8.2 | 1.1 | 9.5 |  |
| `--menu-border` | `#444444` | `#5d5c4e` | Cuyp | 11.0 | 2.6 | 3.9 |  |
| `--menu-button-active` | `#3a3a3a` | `#252015` | Cuyp | 10.8 | 1.1 | 9.5 |  |
| `--menu-button-border` | `#666666` | `#5d5c4e` | Cuyp | 8.4 | 2.6 | 3.9 |  |
| `--menu-button-hover` | `#5a5a5a` | `#5d5c4e` | Cuyp | 7.5 | 2.6 | 3.9 |  |
| `--menu-text` | `#cccccc` | `#b4b4a3` | Cuyp | 10.3 | 8.3 | 1.2 |  |
| `--quest-name` | `#c9b184` | `#b5a26c` | Cuyp | 5.7 | 6.9 | 1.5 |  |
| `--shadow` | `#000000` | `#19140c` | Cuyp | 6.0 | 1.1 | 10.7 |  |
| `--table-border-color` | `#000000` | `#19140c` | Cuyp | 6.0 | 1.1 | 10.7 |  |
| `--toast-desc` | `#a3937c` | `#9a977f` | Cuyp | 6.7 | 5.9 | 1.7 |  |
| `--toast-name` | `#f0e2c8` | `#cec6aa` | Cuyp | 7.5 | 10.2 | 1.0 |  |
| `--vellum` | `#d9c39e` | `#dfcc94` | Cuyp | 5.6 | 10.9 | 1.1 |  |
| `--vitals-cp-label` | `#333333` | `#252015` | Cuyp | 9.2 | 1.1 | 9.5 |  |
| `--vitals-drain` | `#222222` | `#19140c` | Cuyp | 6.2 | 1.1 | 10.7 |  |
| `--vitals-reserved` | `#16181c` | `#19140c` | Cuyp | 7.5 | 1.1 | 10.7 |  |
| `map.brassBorder` | `#5e431a` | `#50371d` | Cuyp | 6.0 | 1.6 | 6.4 |  |
| `map.crackLip` | `#caa86a` | `#b5a26c` | Cuyp | 5.4 | 6.9 | 1.5 |  |
| `map.ink` | `#c9a86a` | `#b5a26c` | Cuyp | 5.1 | 6.9 | 1.5 |  |
| `map.label` | `#e8d8b8` | `#cec6aa` | Cuyp | 5.7 | 10.2 | 1.0 |  |
| `map.pin` | `#cb9f42` | `#aa8636` | Cuyp | 8.7 | 5.1 | 2.0 |  |
| `map.road` | `#c9a86a` | `#b5a26c` | Cuyp | 5.1 | 6.9 | 1.5 |  |
| `map.sheen` | `#fff4d8` | `#cec6aa` | Cuyp | 10.6 | 10.2 | 1.0 |  |
| `map.vignette` | `#000000` | `#19140c` | Cuyp | 6.0 | 1.1 | 10.7 |  |
| `--cp-high` | `#ffffff` | `#efd897` | Cuyp cream | 21.2 | 12.3 | 1.2 | focus bar high (was pure white) |
| `--quest-progress` | `#8a7a58` | `#96885f` | Cuyp gold, secondary-text step | 5.5 | 4.9 | 2.0 | fixes a contrast fail (4.1 today) |
| `--toast-sub` | `#8d7c64` | `#96885f` | Cuyp gold, secondary-text step | 7.3 | 4.9 | 2.0 | fixes a contrast fail (4.0 today) |
| `--chat-whisper` | `#737670` | `#8b897a` | Cuyp olive grey, muted-text step | 8.7 | 4.9 | 2.1 | whisper chat; fixes a contrast fail (3.8 today) |
| `--quest-dim` | `#5a4a32` | `#8b897a` | Cuyp olive grey, muted-text step | 24.5 | 4.9 | 2.1 | "No active quests"; fixes a contrast fail (2.0 today) |
| `--toast-hint` | `#6e5f4b` | `#8b897a` | Cuyp olive grey, muted-text step | 17.2 | 4.9 | 2.1 | fixes a contrast fail (2.8 today) |
| `--charge-low` | `#b94040` | `#b84f3e` | Dali crimson | 5.3 | 3.5 | 2.9 | low-charge fill |
| `--conn-lost` | `#a06060` | `#8a2828` | Dali crimson (dark step) | 16.0 | 2.0 | 5.1 | "Not connected" on the vellum bar; today it fails contrast |
| `--cp-low` | `#ffd700` | `#ea9c2b` | Dali gold | 19.2 | 7.7 | 1.3 | focus bar low |
| `--cp-mid` | `#ffeb8a` | `#e6b058` | Dali gold | 16.1 | 8.9 | 1.1 | focus bar mid |
| `--hp-mid` | `#ffeb3b` | `#e7a747` | Dali gold | 21.4 | 8.3 | 1.2 | health bar mid |
| `--name-player` | `#fce94f` | `#e7b86c` | Dali gold | 16.9 | 9.5 | 1.1 | player names |
| `--conn-reconnecting` | `#b0a060` | `#704714` | Dali gold (dark step) | 34.2 | 2.1 | 4.7 | "Reconnecting"/"Leaving" on the vellum bar; today it fails contrast |
| `--sp-high` | `#daa520` | `#e8892e` | Dali orange | 14.8 | 6.7 | 1.5 | stamina bar high |
| `--frame-red` | `#d6694e` | `#dc6b34` | Dali red-orange | 7.2 | 5.1 | 2.0 | frame on the vellum cards |
| `--sp-low` | `#8b4513` | `#983b1f` | Dali red-orange | 7.8 | 2.5 | 4.1 | stamina bar low |
| `map.locked` | `#d0633f` | `#dc6b34` | Dali red-orange | 5.3 | 5.1 | 2.0 | locked doors on the map |
| `--hp-high` | `#4caf50` | `#88aa71` | Pachinko green, toned | 10.0 | 6.6 | 1.5 | health bar full |
| `--status-online` | `#356b3a` | `#2f4f1c` | Pachinko green, toned (dark step) | 9.8 | 1.9 | 5.4 | "Connected" on the vellum bar; today it fails contrast |
| `--name-mob` | `#00ffff` | `#84c3eb` | Pachinko sky blue, toned | 22.3 | 9.1 | 1.1 | mob names; blue, not teal, so foes never look like party |
| `map.water` | `#6f99c0` | `#68a8cf` | Pachinko sky blue, toned | 5.9 | 6.7 | 1.5 | water on the map |
| `--chip-copper-text` | `#8fb3a6` | `#6dac9d` | Pachinko teal, toned | 6.5 | 6.6 | 1.5 | text on copper chips |
| `--copper` | `#4a6b5d` | `#2e6e60` | Pachinko teal, toned | 5.8 | 2.9 | 3.5 | copper accent stays teal |
| `--chat-broadcast` | `#d700d7` | `#c890b0` | terminal bright magenta | 23.9 | 6.7 | 1.5 | the terminal's own magenta; the normal one fails contrast (4.13) |
| `--white` | `#ffffff` | `#fef9ee` | white tinted with cream | 5.3 | 16.5 | 1.6 | almost white, warmed toward the Cuyp cream |

### Small changes (9)

Difference under 5: barely noticeable, but the value moves to the brand palette.

| Token | Current | New | Source | Change (ΔE) | On dark panel | On vellum | Note |
|---|---|---|---|---|---|---|---|
| `--text-secondary` | `#9a8a6a` | `#96885f` | Cuyp gold, secondary-text step | 3.3 | 4.9 | 2.0 | secondary text; the plain photo match would fail contrast (4.4) |
| `--madder` | `#8c3b30` | `#962a22` | Dali crimson | 4.3 | 2.2 | 4.6 | alerts and the right column rule |
| `--hp-low` | `#f44336` | `#e35349` | Dali crimson, high tone | 4.1 | 4.6 | 2.2 | lighter crimson; clearly red, distinct from the stamina oranges |
| `--toast-kicker` | `#d9b36a` | `#e7b86c` | Dali gold | 3.0 | 9.5 | 1.1 | toast kicker text |
| `map.questGold` | `#e8b84a` | `#e6b058` | Dali gold | 4.5 | 8.9 | 1.1 | quest marker |
| `--sp-mid` | `#d2691e` | `#db6f1d` | Dali orange | 2.5 | 5.2 | 1.9 | stamina bar mid |
| `--online-green` | `#7fae6a` | `#88aa71` | Pachinko green, toned | 3.0 | 6.6 | 1.5 | online indicator on dark |
| `--party-teal` | `#6bb0a0` | `#6dac9d` | Pachinko teal, toned | 1.3 | 6.6 | 1.5 | party members |
| `map.party` | `#6bb0a0` | `#6dac9d` | Pachinko teal, toned | 1.3 | 6.6 | 1.5 | party marker on the map |

### Unchanged (57)

Already within a just-noticeable difference of the painting, so they keep their value:

`--antique-gold` `#b89047`, `--auto-button-border` `#5a3a18`, `--bar-bottom` `#1a110b`, `--bar-deep` `#191009`, `--bg-base` `#191310`, `--brass-light` `#f4dd92`, `--brass-shade` `#4a3410`, `--brass-text` `#3b2a10`, `--button-disabled` `#2a2018`, `--chip-alert-bg` `#2a1d12`, `--chip-bg` `#1d1610`, `--chip-copper-bg` `#16201c`, `--desk` `#0e0a06`, `--gold-button-dark` `#9c7b34`, `--ink-deep` `#2b231d`, `--input-bg` `#150f0a`, `--menu-head` `#241a10`, `--nav-scrim` `#140e09`, `--nav-selected` `#2a2114`, `--page-edge` `#110b07`, `--page-glow` `#251a12`, `--panel-bg` `#201913`, `--panel-bg-alt` `#211a14`, `--panel-border` `#3a2a18`, `--panel-border-soft` `#2e251c`, `--quest-bar-track` `#160e08`, `--quest-name-focus` `#f0dcae`, `--relay-bg` `#111111`, `--relay-border` `#6b5a3a`, `--scrim` `#0a0705`, `--status-muted` `#6e5a38`, `--text-primary` `#d2c3a4`, `--text-tagline` `#b9a983`, `--tile-hover-bottom` `#1c1209`, `--tile-hover-top` `#332818`, `--tile-top` `#2a2016`, `--title-gold` `#e8d2a0`, `--toast-top` `#2b2118`, `--vitals-name` `#4a3a28`, `--vitals-name-self` `#1a1208`, `--well` `#150f08`, `--well-empty` `#140f08`, `map.brassText` `#3b2a10`, `map.crack` `#120a04`, `map.faint` `#8a7a55`, `map.glow` `#e8c98a`, `map.ink2` `#9c8048`, `map.legendBg` `#241810`, `map.partyDk` `#243f3a`, `map.partyFill` `#1c2e2a`, `map.plain` `#9c8048`, `map.ridge` `#a98a55`, `map.rim` `#0e0703`, `map.roomEdge` `#3a2a18`, `map.roomFill` `#2a1d12`, `map.title` `#e8d2a0`, `map.trail` `#a98a55`
