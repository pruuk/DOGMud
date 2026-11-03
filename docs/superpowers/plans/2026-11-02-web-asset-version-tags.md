# Asset version tags on static links (retroactive record)

> **Written after the work** (2026-11-02), to put the skipped plan step back
> into the repo. No BAR was run before execution; a BAR of the shipped diff
> follows. Spec:
> [`../specs/2026-11-01-webclient-rework-design.md`](../specs/2026-11-01-webclient-rework-design.md).

**Commit:** `6452ab436` on `feat/game-settings` (PR #768). Supersedes the
fixed favicon tag of `fbf3f8338`.

## Asked
Malia picked "automatic" over a manual date tag, knowing it means a small
server change for Calabe to review: browsers kept old CSS, JS and favicons
after changes (the game frame even past a hard reload).

## What the code showed
- `internal/web/web.go` builds `templateData` for every `.html` page
  (`REQUEST`, `PATH`, `CONFIG`, `STATS`, `NAV`, ...); non-HTML files go
  through `http.ServeFile`, which ignores a query string.
- Static links live in `_header.html` (all website pages),
  `webclient-pure.html` and `build.html`.
- Two JS tests (`feedscroll`, `companion-relay-glue`) match the exact
  script-link text.

## What changed
- `web.go`: package var `assetVersion` (server start time, base 36) and
  template value `ASSETV`.
- `?v={{ .ASSETV }}` on every `/static` CSS, JS and icon link in the three
  pages.
- `internal/web/assetversion_test.go`: `assetVersion` is set; no untagged
  static link in the three pages (proved against the previous header: 2
  hits, now 0).
- The two JS tests accept the tag. `internal/web/context.md` documents
  `ASSETV`.

## Definition of done
Each restart changes the tag; every static link carries it; a future untagged
link fails a test; nothing else in serving changes.

## Verified
`go build ./...`; `golangci-lint v2.12.2 --new-from-merge-base` on
`internal/web`: 0 issues; `go test ./internal/web/`; all JS tests; server
restarted, tagged links seen in the served HTML, `?v=` URLs return 200, the
game page renders.

## Review
BAR of the shipped diff (2026-11-02): the claims above overstated the test.
It checked only CSS and JS links in three named files, so the logos stayed
untagged, and it would pass if the `ASSETV` key left `templateData`
(text/template renders `?v=<no value>`). Fixed on the branch:
- The logos in `_header.html` and `index.html` carry the tag.
- `TestStaticLinksCarryAssetVersion` checks every top-level public page for
  untagged `/static` CSS, JS, PNG, ICO, JSON, SVG, WebP and JPG links
  (proved: the old `index.html` had 2 hits).
- `TestTemplateDataCarriesAssetVersion` fails if web.go stops setting
  `"ASSETV": assetVersion`.
Cannot carry a tag: the icon list inside `static/manifest.json` and the
`/favicon.ico` route. Documented in `internal/web/context.md` and the brand
standards: give such files new names if they change.

## Superseded (2026-11-03)
Master's PR #773 added `ASSET_VERSION` (a hash of `static/css` and
`static/js`, `internal/web/asset_version.go`) for the same job. In the merge
(plan `2026-11-03-webclient-merge-phone-fit.md`, Malia: master's wins on
overlap) `ASSETV`, `assetVersion` in `web.go` and `assetversion_test.go` were
removed; images carry no tag and get new names when they change.
