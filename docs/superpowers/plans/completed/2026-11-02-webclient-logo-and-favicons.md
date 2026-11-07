# Logo, light logo and favicons (retroactive record)

> **Written after the work** (2026-11-02), to put the skipped plan step back
> into the repo. No BAR was run before execution; a BAR of the shipped diff
> follows. Spec:
> [`../../specs/completed/2026-11-01-webclient-rework-design.md`](../../specs/completed/2026-11-01-webclient-rework-design.md).

**Commits** (`feat/game-settings`, PR #768): `acaa0edf5` palette logo golds,
`3081f1482` logo in top bar and hero, `5b15be32c` favicons, `fbf3f8338`
favicon version tags (superseded by the asset version tags), `8043849c4`
light-mode home logo and the footer stripe fix.

## Asked
Malia: test her logo on the site; then use her exports; a light version for
Light mode; favicons of the moons on Calabe's dark brown `#1e1915`.

## What the code showed
- The header used a gold-gradient text wordmark (`.gomud-btn`); the hero an
  `h1` text title; the favicon was GoMud's crescent (`static/images/favicon.png`,
  also served at `/favicon.ico` by `internal/web/web.go`).
- Her Illustrator SVGs were 400 to 490 KB (2,000+ moon texture shapes): too
  heavy for every page. Her first logo was dark brown and orange, unreadable
  on the dark site (1.10 to 1.90).
- `#1e1915` is within ΔE 1.6 of the panel brown `#201913`.

## What changed
- `static/images/branding/logo_dog.png` (677 x 229) in `_header.html`;
  `logo_full.png` and `logo_full_light.png` (1200 x 790) in `index.html`,
  one shown per `data-theme`. `.gomud-btn` and the light wordmark tokens
  removed from `gomud.css`.
- Palette spec gains the logo golds (`#d9ac46`, `#c3932d`).
- Favicons from her `moonsofgaius.svg`: `favicon.ico` (16/32/48),
  `favicon.png`, `apple-touch-icon.png`, `icon-192.png`, `icon-512.png`,
  `static/manifest.json`; linked from `_header.html` and `webclient-pure.html`.
- The game page footer sat 7px below the game: in Light mode a cream stripe.
  Removed with `body:has(... "/webclient") footer { margin-top: 0 }`.

## Definition of done
The logo reads in both modes; the top bar mark is legible at 48 to 56px; the
favicon is recognisable at 16px; files are web-sized; alt text names the game.

## Verified
Screenshots at 390, 820, 1366 and 1920 in Dark and Light; letter colours
sampled from her files; contrast measured on page, teal and cream; every
icon served with the right type; JS tests pass.

## Review
BAR of the shipped diff (2026-11-02): nothing found in this slice's code.
The logo links were untagged, fixed with the asset version tags (see that
plan's review).
