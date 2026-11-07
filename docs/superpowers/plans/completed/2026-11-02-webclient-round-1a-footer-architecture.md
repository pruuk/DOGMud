# Web round 1a: teal footer and Architecture cards (retroactive record)

> **Written after the work** (2026-11-02), to put the skipped plan step back
> into the repo. No BAR was run before execution; a BAR of the shipped diff
> follows. Spec:
> [`../../specs/completed/2026-11-01-webclient-rework-design.md`](../../specs/completed/2026-11-01-webclient-rework-design.md).

**Commits** (`feat/game-settings`, PR #768): `8c9ab3655` teal footer,
`70de3adc5` Architecture cards.

## Asked
Malia: teal at the bottom too, so the page is framed; a copy of the menu
above the footer on website pages only; Architecture cards without outlines,
shown to her first (the card plan was reviewed in plan mode before building).

## What the code showed
- `footer` was a rounded gradient card with an outline; credit text in olive
  grey, which fails on teal (4.46).
- The global `nav` rule pushed every nav right (meant for the header).
- `.diagram-card` fill `#211a14` is the twin of the page card behind it
  (`#201913`, 1.01): removing the outline alone makes the cards vanish.

## What changed
- `_footer.html`: a `nav aria-label="Site, bottom"` with the menu, skipped on
  `/webclient`; `gomud.css`: full-width teal footer, `.footer-nav`, nav gold
  credit, 44px links on touch; `nav` scoped to `header nav`.
- `architecture.html`: cards `border: 0`, fill `#252015`, 12px radius, soft
  shadow, deeper shadow on hover, a focus ring; light theme keeps its card
  colour.

## Definition of done
Every page framed teal; footer menu marks the current page and wraps on
phones; the phone menu still opens; cards separate from the page card with
all text 4.5 or more.

## Verified
Screenshots at 390, 820 and 1366, Dark and Light; contrast on teal (5.89,
6.67, 5.12) and on the card fill (4.62 to 10.92); phone menu opened in a
screenshot. Not seen in a screenshot: the keyboard focus ring on the cards
(scripted focus does not trigger `:focus-visible`).

## Review
BAR of the shipped diff (2026-11-02): nothing found in this slice's code.
