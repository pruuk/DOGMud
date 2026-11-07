# PR #768 review fixes, and the green on/off switch

Spec: [`../../specs/completed/2026-11-01-webclient-rework-design.md`](../../specs/completed/2026-11-01-webclient-rework-design.md).
Review: Calabe's Claude on #768, 2026-11-04
(`issuecomment-6637366904`). This plan grows one section per item as Malia
rules on it; the keyboard items stay with Calabe's side (his ruling).

## Item 2: Light mode Automation rows (done 2026-11-05, uncommitted)
- **Finding held:** in Light mode the Ticks and Triggers rows and the queue bar
  keep their dark tiles, but the panel's text tokens turn umber, so the text
  measured 1.81 to 2.24 on them.
- **Malia's ruling:** text on those dark tiles keeps the Dark mode light golds.
- **Built:** a Light mode rule in `static/css/dashboard.css` (after the
  `.auto-tog` Light rules) resets `--ink-gold`, `--text-secondary` and
  `--title-gold` on `.auto-row:not(.pending)` and `.auto-qbar`. Name 6.34 to
  7.56, summary 4.54 to 5.42 (4.11 hovered, the same as Dark mode), queue
  count 11.51, state 4.86. A queued row turns cream and keeps its umber text.
- **Checked:** computed colours in headless Chrome, both modes.
- **Open, Malia's call:** the summary at 4.11 on a hovered row, in both modes.

## Green on/off switch (Malia, 2026-11-05; not a review item)
**Asked:** an Automation toggle that is on shows red. On should be green, the
session lamp's green `#70e010` (her Pachinko reference). New entries start on.

**Scope: toggles only** (sliding on/off switches): `.auto-tog`, drawn on tick
rows and trigger rows and in the tick and trigger editors
(`webclient-pure.html` :1434, :1501, :4366, :4593). Check boxes, such as
Settings' "Mute all sound", are not toggles and are not touched.

**What the code shows**
- `.auto-tog` (`dashboard.css` ~960-990): off = `--panel-border` track,
  `--text-secondary` knob; `.on` = `--madder` red track, `--vellum` knob.
  The Light block sets both knobs (`dashboard.css` ~1397).
- New ticks and triggers already start on (`enabledState = (prefillEnabled
  !== false)`, :4365 and :4592; "+ New" passes `true`, :4799). No JS change.
- The lamp's green is a literal in `SESSION_STATES` (:2623) and its rim a
  literal in `#btn-session .sess-dot.sess-lamp` (`dashboard.css`).

**Measured** (graphics need 3:1)
- A toggle sits only on a dark surface, in both modes: the row tile (its
  `--tile-*` and `--well` are not overridden in Light) and the editor box
  (outside `.dash-panel`, so its `--panel-bg` stays `#201913` in Light).
  Queued rows show a badge instead of a toggle.
- Green track 9.39 to 11.2 on the row tile, 8.48 on a hovered row, 10.21 in
  the editor. The umber rim `#3b2a10` (1.16 to 1.26 there) is defensive
  only, and matches the lamp. Umber knob on the green 8.11 (vellum 1.07).
- The old red track was 2.01 on the row tile.
- Off, unchanged, already weak: in Dark the track `#3a2a18` is 1.16 to 1.38
  on the tile, 1.26 in the editor; in Light the row track `#736133` is 2.65
  to 3.16. The knob carries the state (3.93 Dark, 4.39 Light) by its colour
  and position. Recorded, not changed here.

**Steps**
1. `gomud.css` `:root`: `--lamp-green: #70e010; --lamp-rim: #3b2a10;` with the
   numbers; the lamp's `SESSION_STATES` entry and rim rule use them.
2. `dashboard.css`: `.auto-tog.on` background `var(--lamp-green)`, border
   `var(--lamp-rim)`; `.auto-tog.on::after` background `var(--lamp-rim)`.
   **Rewrite** (do not delete) the Light `.auto-tog.on::after` rule to
   `var(--lamp-rim)`: the Light `.auto-tog::after` `#e3dcc4` outranks the
   base `.on` rule and would be 1.24 on the green. Off unchanged.
3. `@media (forced-colors: active)`: `.auto-tog::after { forced-color-adjust:
   none; background: CanvasText; }`, so on and off stay apart in high
   contrast mode (the knob position shows the state).
4. `BRAND_STANDARDS.md` section 4: rewrite the brass bullet that says it
   covers "future toggles"; add: toggles, on = lamp green with umber rim and
   knob, off = dark track, red never means on. Palette addendum
   (`brand-palette-design.md` ~174-177, "used only as a lamp fill"): amend to
   the toggle's second role. Patch notes wait for item 8's rewrite.
5. Not here: the toggle's 28x13 touch size (item 12), and its keyboard and
   screen reader access (a bare `<span>`, no role or `aria-checked`): raise
   with the review reply.

**Done when:** every on toggle is green with an umber rim and knob in both
modes, every off toggle unchanged, the lamp looks the same; the only other
change is item 2's (a Light row's hover border is now the Dark mode gold).

**Verify:** headless Chrome at 1366 and 390, Dark and Light: computed track,
border and knob of a row toggle on and off and of the editor toggle; the
lamp's colour; screenshots. All `tools/jstest`. Restart the test server (the
cache tag is set at boot); Malia checks "Sip" on localhost and her phone and
approves; then commit and push to #768.

## Review
BAR (2026-11-05), all folded in above; none changed what Malia asked for:
- The Light editor number was wrong: the editor stays dark (`#201913`).
- The Light surfaces I measured host no toggle; the real ones are dark, so
  the green carries it, and the hovered row (8.48) was missing.
- The Light knob rule must be rewritten, not deleted.
- "Off unchanged" now records its weak track numbers.
- A forced-colors rule was missing.
- The brand bullet on brass needs rewriting, not appending; the palette
  addendum said "lamp only".
- Item 2 changes a Light row's hover border (1.47 on the body, matching Dark
  mode).
Confirmed correct: the four draw sites are the only toggles; no JS reads the
colour; `var()` works inline for the lamp; no test names the colours; item
2's selectors and numbers.
