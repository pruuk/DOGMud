# Session bar: the status circle moves into the Enter Game button

> Spec: [`../../specs/completed/2026-11-01-webclient-rework-design.md`](../../specs/completed/2026-11-01-webclient-rework-design.md)
> ("Session bar status"). Status: approved by Malia; BAR done (below);
> built and checked by her on localhost 2026-11-02.

## Context
Malia (2026-11-02): the session bar's status text ("○ Not connected",
"● Connected", "↻ Reconnecting…") reads smaller than the 15px buttons next to
it. Her simpler fix: put the circle inside the Enter Game button. She chose
**circle only**: the button reads "○ Enter Game" / "● Leave Game"; the
circle's colour shows the status; screen readers and the tooltip still say
the words.

## What the code shows
- Markup (`_datafiles/html/public/webclient-pure.html` ~236-241): `#dash-session`
  holds `span#sess-name` (character name, "—" until login), `span#sess-conn`
  (status text, inline colour), then `button#btn-session` (the action).
- One function sets both: `setSessionState(state)` (~2615) for
  `connected` / `leaving` / `reconnecting` / `offline`, writing
  `sessConn.textContent` + colour and `sessionButton.textContent` +
  `disabled`.
- Nothing else reads `#sess-conn`: only two CSS rules (`dashboard.css` 58,
  142) and the markup; no tests.
- Status colours on the button's even brass (`#edd489` → `#c8a95b`): connected
  `#2f4f1c` 6.37 to 4.11, lost `#8a2828` 5.95 to 3.84, reconnecting `#704714`
  5.53 to 3.57. Fine for a circle (graphics need 3:1), not for coloured words,
  so the label stays the brass text colour (9.43 to 6.09).

## Steps
1. **Process first** (Calabe's rule): add a "Session bar status" note to the
   spec (`docs/superpowers/specs/2026-11-01-webclient-rework-design.md`) and
   write this as `docs/superpowers/plans/2026-11-02-webclient-session-status.md`;
   run a BAR subagent on it; fold in defect fixes, and ask Malia before any
   change to the behaviour she chose ((the player-view feedback rule)).
2. **Markup:** remove `span#sess-conn`; the button becomes
   `<button id="btn-session"><span class="sess-dot" aria-hidden="true">○</span> <span class="sess-label">Enter Game</span></button>`.
3. **`setSessionState`:** sets the dot glyph and colour (● connected and
   leaving, ↻ reconnecting, ○ offline; the existing colour tokens), the label
   text, `disabled`, and an `aria-label` / `title` that says both, for example
   "Enter Game, not connected", "Leave Game, connected", "Reconnecting".
4. **CSS** (`dashboard.css`): drop the two `.sess-conn` rules; `.sess-dot`
   gets its colour from an inline style or a state class, a small right gap;
   phone layout unchanged (the button already has room).
5. **Docs:** a patch-notes line is not needed (look change only); the spec
   note records Malia's choice.

## Verification
- Headless Chrome at 390, 820, 1366: offline, then connected (at the login
  prompt), then in the game (fed `handleGMCP('Char', …)`), then leaving;
  screenshot each; the circle colour and label match the state; nothing else
  on the bar moved or wrapped.
- Accessibility name of the button per state (`aria-label`).
- All `tools/jstest`, `tools/webclient-tests`, `tools/test_rgsvg_controls.mjs`.
- Malia checks on localhost (and phone), then commit and push to PR #768.

## States (after the BAR)

| State | Circle | Colour | Label | Disabled | Screen reader / tooltip |
|---|---|---|---|---|---|
| offline | ○ | `--conn-lost` | Enter Game | no | "Enter Game, not connected" |
| connected, at login | ● | `--status-online` | Enter Game | no | "Enter Game, connected" |
| connected, in game | ● | `--status-online` | Leave Game | no | "Leave Game, connected" |
| reconnecting | ↻ | `--conn-reconnecting` | Enter Game | yes | "Enter Game, reconnecting" |
| leaving | ● | `--conn-reconnecting` | Leaving… | yes | "Leaving…, leaving the game" |

## BAR (2026-11-02), folded in
- **Name must contain the visible label** (WCAG 2.5.3): no `aria-label`;
  a visually hidden `span.sess-status` after `.sess-label` holds ", not
  connected" etc., so the accessible name is always label + status. Adds a
  `.visually-hidden` utility (none existed).
- **Connected's green came from a deleted rule:** every state sets its
  colour explicitly (table above).
- **`setSessionState` must drop its `getElementById('sess-conn')`** line, or
  every state change throws once the span is gone.
- **Leaving keeps its label** "Leaving…" (connected and leaving share ●).
- **Accepted:** while the Yes / Stay question shows, the button (and its
  circle) is hidden; the state is always connected-in-game then, and a drop
  restores it. Status changes are not announced (no live region), as today.
  Forced-colours mode drops the colour; the shapes and labels still differ.
- **Verification adds:** reconnecting and leaving at all widths, Light mode,
  forced-colours, a click on the inner spans, the name per state.

## Change after Malia's look (2026-11-02)
The connected green should be the brighter yellow green of the Pachinko
reference: sampled `#70e010` (its green bin). On the brass it is only 1.2,
so connected is drawn as a small lamp: the green fill inside a 1.5px dark
umber rim `#3b2a10` (6.09 on the brass, 8.11 on the green). Not connected
(red ○) and reconnecting / leaving (amber) are unchanged.

## Also (Malia, 2026-11-02)
The "Leave the game?" question and its Yes / Stay buttons get the bar's own
gaps (12px, 8px on phones) instead of a text space: `#sess-confirm` is an
inline flex row.

## After the merge with master's phone fit (2026-11-03)
Master added a `connecting` state (#773): it joins `SESSION_STATES` as an
amber dotted circle `◌` ("Enter Game, connecting", disabled). Master's ⛶
full screen button sits in the bar as a brass button like Settings (label on
wide screens, icon on phones). Master hides Reset layout on phones.
`tools/jstest/webclient-connect.test.js` reads the status words from the
button's hidden `.sess-status` span.
