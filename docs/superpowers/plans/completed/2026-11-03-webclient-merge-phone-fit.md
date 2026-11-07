# Merge master's phone fit (#773, #774) into the web rework (#768)

> Spec: [`../../specs/completed/2026-11-01-webclient-rework-design.md`](../../specs/completed/2026-11-01-webclient-rework-design.md).
> Master's plan: [`2026-11-03-webclient-phone-fit.md`](2026-11-03-webclient-phone-fit.md) (Calabe).
> Status: BAR done (findings at the end, folded in); approved by Malia and
> merged 2026-11-03. **Rule (Malia, 2026-11-03):
> where both did the same job, Calabe's wins; ours fits on top.**

**Branch:** `feat/game-settings` (PR #768), merging `origin/master` at
`307042aa8`. A trial merge (aborted) conflicts in 10 files; all other files
merge cleanly.

## The overlaps and how each resolves

| Area | Ours (#768) | Master (#773/#774) | Resolution |
|---|---|---|---|
| Cache tags | `ASSETV` = server start time (`internal/web/web.go` `assetVersion`), `assetversion_test.go` | `ASSET_VERSION` = hash of `static/css` + `static/js` (`internal/web/asset_version.go`, `asset_version_test.go`); same Go name `assetVersion` | **Master's.** Drop our var, `ASSETV` key and our test; every link uses `{{ .ASSET_VERSION }}`. Our extra coverage (logos, favicons, manifest links) moves into one added test in master's file style, with the template key check. Note: the hash ignores images, so a changed image needs a new name (already the brand-standards rule). |
| Brass | even game brass, `.subtab` replaces `.inv-tab.brass`, no text glow, 15px regular session buttons | darker `--brass-text #1a1206`, `.inv-tab.brass` brightness tweaks | **Both:** keep master's darker `--brass-text` (all brass reads better); keep our `.subtab`; drop master's `.inv-tab.brass` block (the markup no longer uses it). Re-measure label contrast and update our CSS comments. |
| gomud.css | site Light theme block | `:root.phone { --term-bright-black }` (dim grey on phones) | **Both blocks.** |
| Session bar | Settings menu (replaced the 🔊 `#menu-icon`), status circle inside Enter Game, `SESSION_STATES` | `#menu-icon` still, new `#btn-fullscreen` (⛶, round 26px), a `connecting` state in `setSessionState` | **Ours plus master's two additions:** keep Settings and `SESSION_STATES`; add master's `connecting` state (amber ●, disabled, "Enter Game, connecting"); add `#btn-fullscreen` styled as our brass square like Reset layout (icon on phones, icon + "Full screen" label on wide screens, like Settings). Master's `#menu-icon` styles are dropped (no such button on our side). Malia reviews the ⛶ look. |
| Phone keyboard | blur after every send on touch-only screens under 700px (`HotInput.shouldHideKeyboardAfterSend`); xterm textarea `inputmode="none"` on phones | frame and command box fit above the keyboard (`viewportfit.js`); "phone" = screen's shorter side under 600px | **Master's definition of phone** for our rule too (one meaning of phone): `phoneWidth()` becomes `ViewportFit.isPhone(screen.width, screen.height)`; keep `touchOnly`. Our blur stays (it is the behaviour Malia chose); master's fit handles the keyboard while it is open. Test both together on Malia's phone. |
| Viewport meta | added (`initial-scale=1`) | added (`initial-scale=1.0`) | **Master's line**, one tag. |
| Header | favicons, logo, `ASSETV` tags | `ASSET_VERSION`, its load-script fix | **Master's tags and script; our favicons and logo lines** retagged. |
| build.html, two jstests | `ASSETV` patterns | `ASSET_VERSION` | **Master's.** |
| docs/README.md, PATCH_NOTES.md | our entries | Calabe's entries | **Both**, ours first in PATCH_NOTES as before. |

Files that merge cleanly but interact: `gmcp.js` (master changes the map
buttons' brass; our `controlsHost` move must keep his colours),
`termfit.js` (master's terminal refit), `webclient.html` (master's full
screen and frame height), `motdrestyle.js`.

## Steps
1. `git merge --no-ff origin/master`; resolve each file per the table.
2. Go: delete our `assetVersion` / `ASSETV` from `web.go`; delete
   `assetversion_test.go`; add `TestPublicPagesVersionTheirImageLinks` (logos,
   favicons, manifest carry `?v={{ .ASSET_VERSION }}`) and a key check beside
   master's tests; update `internal/web/context.md` (one cache-tag story).
3. Retag every `ASSETV` in `_datafiles/html/public/*.html`; grep for `ASSETV`
   until none remain.
4. Session bar: add `connecting` to `SESSION_STATES`; add `#btn-fullscreen` as
   a `.brass` button with `.btn-icon` / `.btn-label` like Settings; check
   master's toggle code finds it (it shows the button with
   `style.display`).
5. Phone rule: `phoneWidth()` → `ViewportFit.isPhone(...)`; update
   `hotinput.test.js` static checks and the keyboard plan.
6. Docs: update our plans' facts that changed (asset tags plan, keyboard
   plan, session status plan), brand standards §7.

## Definition of done
- No `ASSETV` anywhere; one cache-tag mechanism (master's) with tests.
- All of master's behaviours still work: phone full screen by default with ⛶
  to leave, frame above the keyboard, dim grey on phones, Temple marker,
  darker brass text, connecting state.
- All of ours still work: round tabs, sub-tabs, map strip, Light panels,
  status circle, keyboard closing after each send, logo, favicons, footer.
- `go build ./...`, `go test ./internal/web/`, lint 0 issues, every
  `tools/jstest`, `tools/webclient-tests`, `tools/test_rgsvg_controls.mjs`;
  server boots to Server Ready.

## Verification
Headless Chrome (fresh profile) at 390, 820, 1366 in Dark and Light: session
bar in each state including connecting, ⛶ shown and working, brass contrast
re-measured. Malia on her phone: full screen by default, ⛶ back, keyboard
closes after a send, command box above the keyboard while typing. Then push
to #768 and tell Calabe what was kept from each side.

## BAR (2026-11-03), folded in
Corrections to the table and steps above:
1. **Master's `tools/jstest/webclient-colours.test.js`** reads the
   `.inv-tab.brass` block, which our `.subtab` replaced: retarget its two
   inactive-tab checks at `.subtab` (same intent: inactive tabs readable and
   not see-through) and tell Calabe.
2. **Master's `tools/jstest/webclient-connect.test.js`** extracts
   `setSessionState` and fakes a text-only button: rewrite its fake to our
   button (`.sess-dot`, `.sess-label`, `.sess-status`, `SESSION_STATES` in the
   extracted window) and assert the status words and `disabled`; keep every
   "one socket at a time" check as is.
3. **Head scripts:** keep our `theme.js` **and** all of master's scripts
   (`viewportfit.js`, `termfit.js`), all tagged `ASSET_VERSION`. Delete our
   viewport meta by hand (it is outside the conflict markers); keep master's
   exact line, which `viewportfit.test.js` checks.
4. **⛶ button:** on wide screens it shows its label only, like Settings and
   Reset layout ("Full screen" / "Exit full screen"); on phones its ⛶ icon.
   Master's `onChange` also sets the label, and the spoken name starts with
   the visible words (WCAG 2.5.3). Drop master's whole round-button block for
   `#menu-icon, #btn-fullscreen` (ID rules that would beat `.brass`); keep a
   focus ring; add `#btn-fullscreen` to our phone icon-square selectors.
5. **Phone definition:** using `ViewportFit.isPhone(screen)` for the keyboard
   rule means phones held sideways now close the keyboard too, and touch
   tablets in a narrow split view no longer do. Matches "phones" in player
   terms; the resize re-check comment is updated; landscape joins the phone
   test. `data-mode="phone"` (dashboard under 700px) stays for layout.
6. **Reset layout is hidden on phones by master** (merges cleanly): our phone
   Reset icon square goes; remove its dead phone rules.
7. **Image tags dropped:** master's hash covers only CSS and JS, so a
   `?v=` on logos, favicons and the manifest would never change when they do.
   Remove those tags and do not add the image-link test; the rule becomes
   "a changed image gets a new file name" (BRAND_STANDARDS §7 and
   `internal/web/context.md` rewritten; this deploy refreshes the new
   favicons anyway, because CSS and JS changed too).
8. **Map buttons keep master's brass** (radial gradient, light text shadow)
   rather than the even game brass: master's colour test pins them, and the
   rule says master's wins. Listed for Malia.
9. **Connecting** gets its own glyph, an amber dotted circle `◌`, so it
   differs from leaving (amber ●).
10. Nits: move master's keyboard-open CSS out from under our drawer comment;
    "no ASSETV" applies to code, tests and living docs (history stays in the
    shipped plans); drop unused imports in `web.go`; list this plan in
    `docs/README.md`.

## What Malia will see change from yesterday (for her review)
- Phones open the game **full screen** (no site header or footer) with a ⛶
  button in the session bar to bring them back (Calabe's).
- **Reset layout is hidden on phones** (Calabe's).
- **Darker brass text** on every brass button, easier to read (Calabe's).
- **Map buttons** keep Calabe's shinier brass, a little different from the
  other game buttons.
- A new amber **◌ Connecting** state on the Enter Game button.
- The keyboard closing after each send now also happens with the **phone
  held sideways**.
- While typing on a phone, the game and command box **fit above the keyboard**
  (Calabe's).
