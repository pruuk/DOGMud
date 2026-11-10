# DOGMud brand standards

**Owner:** Malia (look and feel). **Written:** 2026-11-02, from the decisions
of the 2026-11-01 and 2026-11-02 sessions. **Colour source of truth:**
[`../superpowers/specs/completed/2026-11-01-brand-palette-design.md`](../superpowers/specs/completed/2026-11-01-brand-palette-design.md).
Change a rule here only with Malia's agreement, and update the palette spec
in the same change when a colour moves.

## 1. The logo

Malia's logo: three moons, the middle one crowned, over the name. Source
files are hers (Illustrator); the site uses web exports only.

| Use | File (in `_datafiles/html/public/static/images/branding/`) | Where | Size on screen |
|---|---|---|---|
| Full logo, dark surfaces | `logo_full.png` (1200 x 790) | home page hero, Dark mode | up to 560px wide; 86% of a phone's width |
| Full logo, light surfaces | `logo_full_light.png` (1200 x 790) | home page hero, Light mode | same |
| D.O.G. mark | `logo_dog.png` (677 x 229) | the teal top bar on every page | 56px tall on wide screens, 48px on phones |

**Rules**
- **The full logo needs room.** It is stacked and detailed, so below about
  300px wide the moons and "OF" stop reading. In a thin bar or a small space,
  use the D.O.G. mark instead.
- **Match the surface.** The gold version goes on dark surfaces and the teal
  bar only; on cream or parchment use the light version. The gold letters
  wash out on light (1.8).
- **Never recolour, stretch, crop or add effects** (shadows, outlines, glows)
  to the logo files. New colours or crops come from Malia's source files.
- **Exports:** PNG, trimmed to the artwork (Illustrator: Object > Artboards >
  Fit to Artwork Bounds), about 3x the largest display size. Do not ship the
  raw Illustrator SVGs: their moon textures are 2,000+ shapes and 400 KB+.
- **Alt text** is the game's name (`{{ .CONFIG.Server.MudName }}`), so a
  screen reader, or a page whose image fails, still says
  "Delusions of Grandeur".

## 2. Colours

All colours come from the palette spec. The roles that define the brand:

| Role | Colour | Used for |
|---|---|---|
| Logo gold | `#d9ac46` | "DELUSIONS", "OF", the rules; "D" and "O" of D.O.G. |
| Logo deep gold | `#c3932d` | "GRANDEUR"; "G" of D.O.G. |
| Logo, light version | `#6f5418` top word, `#473019` "GRANDEUR" | the full logo on light surfaces |
| Website teal | `#085044` (brand teal L30) | the top bar and the footer: "this is the website" |
| Game umber | `#19140c` to `#39301b` | game surfaces, the round tab bar: "this is the game" |
| Brass | `#f4dd92` / `#aa8636` / `#736133` | every control you can press in the game |
| Nav gold | `#dfcc94` | menu text on teal (5.89) |
| Panel brown | `#201913` | game panels and cards; the favicon background |
| Light mode | title bars `#b4b4a3`, bodies `#cec6aa`, text `#2b2116` | game panels in Light mode |

**Rules**
- **Teal means website, umber and gold mean game.** A player should tell the
  site's menus from the game's controls at a glance on any screen.
- **One pairing per role, everywhere:** in Light mode text is darker than its
  surface, in Dark mode lighter. A dark or pressed surface gets cream text
  (`#efd897`), never dark text.
- **Text contrast at least 4.5:1** against what it sits on; icons and focus
  rings at least 3:1. Measure, do not eyeball, and write the number in the
  CSS comment beside the colour.
  **One known shortfall:** the dark `#1a1206` label on the shiny brass of the
  Chat, Inventory and Automation tabs (`.subtab`) is 13.77 on the light spot
  and 5.45 mid, but falls to 3.08 where a long label reaches the brass's
  dark edge `#736133`. Fixing it means changing that brass, which is
  Malia's call.
- **Red is for warnings only** (low health, alerts, queued actions). Do not
  use it for decoration, frames or selection.
- **No pure white or black.** The lightest is cream `#fef9ee` / `#efd897`, the
  darkest umber `#19140c`.

## 3. Type

- **JetBrains Mono** for everything: game text, panels, menus, buttons,
  headings and the site pages (`--font-body`, `--font-heading` and
  `--font-mono` all name it). Ligatures off.
- Self-hosted (`static/fonts/`): the official v2.304 web files, unmodified.
  The Google Fonts version has no box-drawing characters, which misaligns
  every box in the game.
- **Two sizes:** text 16px and titles 20px (`--text`, `--title`), 14px and
  18px on upright phones. The command box and every field a player types in
  are always 16px (`--cmd`): iOS zooms the page on anything smaller.
- **Site bars:** top bar 20px and footer 16px (`--nav-text`, `--foot-text`),
  16px and 14px on phones (under 768px wide or 500px tall).
- **No italics.** Regular weight for body text and every button. One bold
  (`--bold`, 600) for titles, quest names, your own character's name and room
  titles. No glow or text shadow on brass labels.
- Lettering in the map art scales with the map. The builder, the admin pages
  and the architecture diagrams keep their own styling.
- `internal/guards/client_type_guard_test.go` holds the font, sizes, weights
  and italics.

## 4. Controls in the game

- **Brass** marks anything pressable. Closed or idle: shiny brass with dark
  text. **Open or selected: pressed into dark brass with cream text.** This
  one rule covers the round tabs and the Chat, Inventory and Automation tabs.
- **Toggles** (sliding on/off switches, such as the Automation ones): on is
  the lit lamp, green `#70e010` with an umber `#3b2a10` rim and knob; off is
  the dark track with a lighter knob. Red never means on. New entries start
  on. Check boxes are a different control and keep the brass accent.
- **Round tabs** (Map, Quests, Health, Inventory, Chat, Conditions, Automation):
  brass circles on a dark umber bar with a teal rim, one sliding name pill.
- **Panels:** gold outline on every card, one teal line under every panel
  title. Pop-out windows are a wide-screen feature only.
- **Touch:** at least 44px tall targets on touch screens. One exception
  (Calabe and Malia, 2026-11-07, tested on her phone): on a phone (touch,
  under 600px wide or under 500px tall, either way up) 44px sub-tabs took
  most of an open Chat tab, so there the sub-tabs and the panel title
  buttons are 30px, still above the 24px minimum target (WCAG 2.5.8). Held
  sideways, the tab name pill is also hidden.

## 5. Icons

| File (in `static/images/`) | Size | Use |
|---|---|---|
| `favicon-moons.ico` | 16, 32, 48 | browser tabs; also served at `/favicon.ico` (`internal/web/web.go`) |
| `favicon-moons.png` | 32 | browser tabs |
| `apple-touch-icon.png` | 180 | phone home screen |
| `icon-192.png`, `icon-moons-512.png` | 192, 512 | Android, listed in `static/manifest.json` (the 512 was `icon-512.png` until 2026-11-07, when its 43px see-through strip at the bottom was filled with the panel brown) |

All from Malia's `moonsofgaius.svg` (three moons, no crown) on the panel
brown `#201913`, moons pulled in from the rounded corners. In-game icons are
simple line icons drawn in one style (`PANEL_TABS` in `static/js/dashboard.js`),
never emoji, which look different on every device.

## 6. Listing sites (legacy assets)

`banner_468x60.png`, `emblem_1024.png`, `logo_100.png` and `logo_256.png` in
the branding folder are for MUD listing sites, which hotlink them, so **never
rename or delete them**; new art replaces them in place. Redrawn 2026-11-07
(#820, Malia) on the website teal `#085044`, since they send people to the
website: the full logo at 1024 and 256, D.O.G. at 100, and a banner with the
crowned moons, the wordmark and dogmud.org in nav gold `#dfcc94` (logo gold
is 4.43 on teal, under 4.5 for words). Before that they showed the 2026-07-28
chrysalis emblem. See [`ADVERTISING_LISTINGS.md`](ADVERTISING_LISTINGS.md).

## 7. Shipping a brand change

- Every stylesheet and script link carries `?v={{ .ASSET_VERSION }}`, a hash
  of the CSS and JS files (`internal/web/asset_version.go`), so a deploy that
  changes them reaches players' browsers. `internal/web/asset_version_test.go`
  fails on an untagged one.
- **Images are not hashed and carry no tag.** A changed logo, icon or favicon
  gets a **new file name** (and its links change), or browsers keep showing
  the old picture.
- New image files: state the full path and add them to `docs/README.md`.
- Check Dark and Light mode, and phone (390), tablet (820) and desktop
  (1366) widths, before review.
