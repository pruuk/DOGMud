# DOGMud brand standards

**Owner:** Malia (look and feel). **Written:** 2026-11-02, from the decisions
of the 2026-11-01 and 2026-11-02 sessions. **Colour source of truth:**
[`../superpowers/specs/completed/2026-11-01-brand-palette-design.md`](../superpowers/specs/completed/2026-11-01-brand-palette-design.md).
Change a rule here only with Malia's agreement, and update the palette spec
in the same change when a colour moves.

## 1. The logo

Malia's logo (2026-11-15, #882): a planet with three moons on two orbits,
"DELUSIONS OF" above and "GRANDEUR" below. It replaced the crowned three
moons of 2026-11-02. Source files are hers (Illustrator); the site uses web
exports only. Each export comes in gold letters (`#efd897` / `#d9ac46`) for
dark surfaces and the teal bar, and browns (`#473019` / `#6f5418`) for light
surfaces. Malia names the gold set "_light" (light letters); the site names
files by the page they suit, so her "_light" ships as our dark-page file.

| Use | File (in `_datafiles/html/public/static/images/branding/`) | Where | Size on screen |
|---|---|---|---|
| Stacked logo, dark surfaces | `logo_stacked.png` (1024 x 1024) | home page hero, Dark mode | up to 560px wide, 86% of a phone's width, and no wider than 55% of the screen's height |
| Stacked logo, light surfaces | `logo_stacked_light.png` (1024 x 1024) | home page hero, Light mode | same |
| One-line logo | `logo_bar.png` (468 x 78) | the teal top bar on every page, both themes | 56px tall on wide screens, 48px on phones |

**Rules**
- **The stacked logo needs room.** It is detailed, so below about 300px wide
  the moons and "OF" stop reading. In a thin bar or a small space, use the
  one-line logo instead.
- **Match the surface.** The gold version goes on dark surfaces and the teal
  bar only; on cream or parchment use the brown version. The brown letters
  do not read on teal, so the bar keeps the gold logo in Light mode too.
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
| Logo cream | `#efd897` | "DELUSIONS OF" and the rules, gold version |
| Logo gold | `#d9ac46` | "GRANDEUR", gold version |
| Logo, brown version | `#473019` top words, `#6f5418` "GRANDEUR" | the logo on light surfaces |
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
| `favicon-planet.ico` | 16, 32, 48 | browser tabs; also served at `/favicon.ico` (`internal/web/web.go`) |
| `favicon-planet.png` | 32 | browser tabs |
| `apple-touch-icon-planet.png` | 180 | phone home screen |
| `icon-planet-192.png`, `icon-planet-512.png` | 192, 512 | Android, listed in `static/manifest.json` |

Malia's planet and moons (2026-11-15, #882). The tab icons come from her
see-through 100px mark; the phone icons from her 512 on the logo brown
`#473019`, with the 192 and 180 scaled from it. Phone icons need a solid
background: a phone fills see-through pixels with black or white. They
replaced the 2026-11-02 three-moons set (`favicon-moons`, `icon-moons-512`).
In-game icons are
simple line icons drawn in one style (`PANEL_TABS` in `static/js/dashboard.js`),
never emoji, which look different on every device.

## 6. Listing sites (legacy assets)

`banner_468x60.png`, `emblem_1024.png`, `logo_100.png` and `logo_256.png` in
the branding folder are for MUD listing sites, which hotlink them, so **never
rename or delete them**; new art replaces them in place. They sit on the
website teal `#085044`, since they send people to the website and a listing
site may show them on any page colour; a see-through listing image would
vanish on some sites. The banner is Malia's new logo on teal (2026-11-15,
#882). The three squares still show the 2026-11-07 art (#820: the full logo
at 1024 and 256, D.O.G. at 100) until teal versions of the new logo arrive.
Before that they showed the 2026-07-28 chrysalis emblem. See
[`ADVERTISING_LISTINGS.md`](ADVERTISING_LISTINGS.md).

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
