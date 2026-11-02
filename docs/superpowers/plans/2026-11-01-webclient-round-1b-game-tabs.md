# Web client round 1b: game tabs, brass and panels (retroactive record)

> **Written after the work** (2026-11-02), to put the skipped plan step back
> into the repo. No BAR was run before execution; a BAR of the shipped diff
> follows (see "Review" at the end). Spec:
> [`../specs/2026-11-01-webclient-rework-design.md`](../specs/2026-11-01-webclient-rework-design.md).

**Commit:** `67981533b` on `feat/game-settings` (PR #768). It also carries the
2026-11-01 Settings menu, website light theme and glass menus
(`static/js/theme.js`, `tools/jstest/theme.test.js`).

## Asked
Malia: game tabs players can tell from the website menu, readable on any
device; round icon tabs (Version B from her mockup) with one sliding name
pill; friendlier names; then, while reviewing on localhost, brass and panel
fixes.

## What the code showed
- `static/js/dashboard.js` built two different tab strips: `.rail-tabs` text
  buttons (rail) and emoji `.tabbar-btn` divs (phone), labels from panel ids
  ("Vitals", "Art", "Trig").
- Three sub-tab styles: chat `.tab-button` (inline CSS in
  `webclient-pure.html`), `.inv-tab.brass` (10px, dimmed), `.auto-tab`
  spans (no keyboard access).
- `webclient-pure.html` had no viewport meta: phones drew `/webclient-pure`
  at 980px.
- A saved layout could restore a popped-out Game window on a phone; the
  window title repeated the panel header.
- The closed phone drawer peeked above the tab bar over the command box.

## What changed
- `dashboard.js`: `PANEL_TABS` (labels + line icons) and `makeTabBar()`
  shared by rail and phone; one pill; `aria-pressed`; `_relayoutTabs`;
  pop-outs docked when narrow (`_dockPoppedForNarrow`, `_repopAfterNarrow`,
  `_suspendSave`); drawer close as a square icon button.
- `dashboard.css`: `.gtabs` / `.gtab` / `.gtab-pill`; phone bar sizes
  (`--tabbar-h`, `--tabname-h`); closed drawer fully hidden; `.subtab`
  shared style; even game brass (`:root[data-client] --brass-grad`); gold
  card outlines; one teal title line; winbox header hidden; pop-out button
  hidden when narrow; rail min 280px.
- `webclient-pure.html`: viewport meta; panel titles renamed; sub-tab markup
  (`subtabs` / `subtab`, Automation spans became buttons, `aria-pressed`).
- `gomud.css`: site bar teal; capsule tokens retoned.
- `modules/gmcp/gmcp.Char.go`: comment only (panel renamed).

## Definition of done
Every tab reachable by mouse, touch and keyboard with its state announced;
no control over the command box; text 4.5 or more; phone, rail and wide all
correct after a mode change and with a saved popped-out layout.

## Verified
Headless Chrome at 360, 390, 820, 1366 and 1920; saved-layout reproduction
of Malia's report; Tab and Enter path on the rail tabs; contrast numbers in
the CSS comments; all `tools/jstest` pass.

## Review
BAR of the shipped diff (2026-11-02), findings for this slice, all fixed on
the branch:
- **Game window docked under the command box.** Only side panels recorded a
  home column, so `redock("feed")` did nothing and WinBox re-attached the
  Game panel after `#panel-cmd` (also breaking the `#panel-feed + #panel-cmd`
  sight borders). This slice made it automatic on narrowing. Fix: init
  records `homeCol` for every `.dash-panel`.
- **Layout saving could stay off.** `_dockPoppedForNarrow` set `_suspendSave`
  without `try/finally`. Fixed.
- **Preview pill over the command box** on the phone bar with no panel open
  (mouse or keyboard). Fixed: no preview there while nothing is open.
- **Settings note was stale** after round 2 ("the game screen stays dark").
  Reworded.

The "definition of done" above was not met for the Game pop-out until this
fix. Still open (existed before): the panel-head ▾ / ⧉ and Automation
"+ New" are non-focusable spans.
