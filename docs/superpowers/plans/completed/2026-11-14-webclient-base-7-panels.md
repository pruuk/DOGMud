# Web client base, plan 7: panels (Map, Quests, Comms, Equipped, menus)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Map shows its level in the title and a Location / Destination bar instead of words on the art; Quests has Active, Repeat and Completed tabs; the Chat panel is called Comms; all three right-click menus share one look with the name boxed at the title size, and open on touch by a long-press (an item tile also by a tap).

**Architecture:** One server change: `Char.Quests` gains `done` rows from a new `usercommands.CompletedQuestIds`, held to `quests all` by a parity test. Three small pure client modules, each tested in node: `questtabs.js` (split and sort the quest rows into tabs), `mapwhere.js` (the bar's text), `longpress.js` (press timing and touch wiring). The page wires them in; the three menus' CSS folds into one `.ctx-menu` set.

**Tech Stack:** Go (`internal/usercommands`, `modules/gmcp`), plain browser JavaScript, node tests in `tools/jstest`, CSS, Playwright for the browser check.

Spec: `docs/superpowers/specs/2026-11-14-webclient-base-7-panels-design.md` (approved 2026-11-14, rulings P1 to P6). Parent: `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md` rulings 16 to 21. Mocks: https://claude.ai/artifact/VtaJsG4hfn4JRUi569z2SK. Builds on plan 6 (PR #869, branch `feature/webclient-base-6-phones`). Real-device test: plans 6 and 7 together, after this plan.

Every agent: edit files with the Edit and Write tools only, never a shell write into a tracked file; `git add` named paths only; never stash; commit through a quoted heredoc ending with a blank line and `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; no servers started except where a task says so, killed by PID.

---

## Facts verified against source

The spec's table (read at `5336b9125`) holds; these were read for the plan on the same commit. `WP` is `_datafiles/html/public/webclient-pure.html`; `CSS` is `_datafiles/html/public/static/css/dashboard.css`; `JS` is `_datafiles/html/public/static/js/`.

| Fact | Where |
|---|---|
| Scripts load in the page head with `{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/<name>.js?v={{ .ASSET_VERSION }}` | `WP:212`, `:217` |
| The Map panel markup: `#map-controls` then `#map-legend` | `WP:329`, `:336` |
| The Quests panel body is `<div class="dash-panel-body"><div id="quests-list">...` on one line | `WP:349-352` |
| The `"Room"` GMCP handler processes `Room.Info` | `WP:757` |
| The `"Zone.Map"` handler sets the title; the `"Char.Quests"` handler calls `renderQuests()` then finds the focused quest for `setQuestMarker` | `WP:978-993`, `:997-1015` |
| `renderQuests` and `writeQuestsHTML` (a `RenderGuard.setHTML` on one host) | `WP:1143-1270`, `:1271-1278` |
| The item menu: `buildMenu` sets `className = "inv-ctx"`; `openMenu(e, tile)` reads `e.clientX/Y`; the `contextmenu` delegation | `WP:4030-4041`, `:4185-4190` |
| The Automation menu `auto-ctx` and its delegation; the Quest menu `quest-ctx` and its delegation | `WP:4261-4269`, `:4356-4361`; `:4375-4381`, `:4461-4466` |
| Map renderer text draws: `lTxt`/`embText` defined at 34 and 54; call sites 707 (area label), 734 (banner), 955 and 959 (title, level), 971 (compass N), 1013 (room name), 1021 (service glyph), 1061 (quest name), 1144 (zone-exit label), 1182 (`?`), 1300 (stairs) | `JS/gmcp.js` |
| `_drawQuestBanner()` | `JS/gmcp.js:721-737` |
| Quest CSS: `.quests-empty`, `.qrow`, `.qname` line-height 1.25, `.qhint` and `.qprogress` 1.4, then the `.qrepeat` / `.qgroup` / `.qrepeatrow` rules | `CSS:350-395` |
| `hotinput.test.js` checks the three repeat rules and their colour | `tools/jstest/hotinput.test.js:473-477` |
| `webclient-html-escape.test.js` evaluates `writeQuestsHTML` and `renderQuests` from the page with a fake DOM | `tools/jstest/webclient-html-escape.test.js:55-125` |
| `finishedRepeatQuests(c, lookup)` and its test pattern (a `lookup` func, a bare `characters.Character`) | `modules/gmcp/gmcp.Char.go:37-60`; `modules/gmcp/gmcp.Char_repeat_test.go` |
| `Char.Quests` appends the repeat rows | `modules/gmcp/gmcp.Char.go:745` |
| `finishedRepeatRows` labels a capped repeatable `repeatDoneLine` | `internal/usercommands/quests.go:89-121` |
| `quests.SeedQuestsForTest(qs ...*Quest) func()` | `internal/quests/test_helpers.go:9` |
| Character fields `QuestProgress`, `QuestCompletions`, `ShelvedQuests`; `RepeatCapReached(q)`, `IsShelved(id)` | `internal/characters/character.go:325`; `internal/characters/quests.go:237`, `:351` |

## Owner rulings for this plan

The spec's P1 to P6: no text on the map art except the zone-exit labels and single-character marks (P1, P6); long-press opens the three menus on touch, a tap keeps its job, an item tile's tap opens its menu (P2); long-press is a stopgap for the touchscreen arc (P3); a Completed tab (P4); the mocks approved (P5).

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| One list host (`#quests-list`) shows the selected tab, not one host per tab (the spec said each tab its own host) | One `RenderGuard` host, and `webclient-html-escape.test.js` keeps working; switching tabs re-renders | Three hosts later if a tab needs to keep its scroll |
| `quests all` is not rewritten onto `CompletedQuestIds` (the spec said the helper defines it for both); a parity test holds the two to the same set | `quests all` builds display rows from progress and repeat rows with their own text; a parity test enforces agreement without touching that output | Rewrite `quests all` onto the helper |
| A left click on an item tile opens its menu on desktop too | One handler for click and tap; on desktop a click did nothing before | Gate it on touch input |
| The bar's Destination falls back to the quest name when the target has no label (spec wording) | As specified | Hide the Destination instead |
| The Quests tab bar scrolls with the list inside the 220px panel body | Least change to the panel's sizing rules (`CSS:350-351`) | Make the tab bar sticky |

## File structure

- Create `internal/usercommands/quests_completed.go`, `internal/usercommands/quests_completed_test.go`.
- Modify `modules/gmcp/gmcp.Char.go` (a `done` status, `completedQuestRows`, the append); create `modules/gmcp/gmcp.Char_completed_test.go`.
- Create `JS/questtabs.js`, `JS/mapwhere.js`, `JS/longpress.js` and `tools/jstest/questtabs.test.js`, `tools/jstest/mapwhere.test.js`, `tools/jstest/longpress.test.js`.
- Modify `WP` (script tags, Quests tabs, map bar, title, Comms labels, menu classes, tap and long-press wiring), `JS/gmcp.js` (text draws out), `JS/dashboard.js` (Comms label), `CSS` (quests, map bar, menus, Equipped cleanup).
- Modify `tools/jstest/hotinput.test.js`, `tools/jstest/viewportfit.test.js`, `tools/jstest/webclient-html-escape.test.js` where they name what changed.
- Docs: `internal/usercommands/context.md`, `modules/gmcp/context.md`, `docs/PATCH_NOTES.md`, `docs/README.md`, the spec (decision amendments).

---

### Task 0: Worktree

- [ ] **Step 1:** The main checkout has `feature/webclient-base-7-panels` checked out with the spec. Free it and make the worktree:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git status --short        # expect only this plan file untracked
git checkout master
git worktree add C:/tmp/dogmud-p7 feature/webclient-base-7-panels
cp docs/superpowers/plans/2026-11-14-webclient-base-7-panels.md C:/tmp/dogmud-p7/docs/superpowers/plans/
rm docs/superpowers/plans/2026-11-14-webclient-base-7-panels.md
cd C:/tmp/dogmud-p7
git add docs/superpowers/plans/2026-11-14-webclient-base-7-panels.md
git commit -q -F - <<'EOF'
docs(plan): web client base plan 7, panels

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Record `C:/tmp/dogmud-p7` in memory `project-touch-arc-base-first`.

---

### Task 1: Completed quests on the server

**Files:** Create `internal/usercommands/quests_completed.go`, `internal/usercommands/quests_completed_test.go`, `modules/gmcp/gmcp.Char_completed_test.go`; modify `modules/gmcp/gmcp.Char.go:28-31`, `:745`, `:1201`.

- [ ] **Step 1: Failing tests.** `internal/usercommands/quests_completed_test.go`:

```go
package usercommands

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// Plan 7, owner P4: the Quests panel's Completed tab.
func completedFixture() (map[int]*quests.Quest, *characters.Character) {
	qs := map[int]*quests.Quest{
		10: {QuestId: 10, Name: "One Off"},
		11: {QuestId: 11, Name: "Secret One Off", Secret: true},
		12: {QuestId: 12, Name: "Capped", Repeatable: true, RepeatLimit: 1},
		13: {QuestId: 13, Name: "Again", Repeatable: true},
		14: {QuestId: 14, Name: "Working"},
		15: {QuestId: 15, Name: "Shelved Capped", Repeatable: true, RepeatLimit: 1},
		16: {QuestId: 16, Name: "Secret Capped", Repeatable: true, RepeatLimit: 1, Secret: true},
	}
	c := &characters.Character{
		QuestProgress:    map[int]string{10: "end", 11: "end", 14: "start"},
		QuestCompletions: map[int]int{12: 1, 13: 1, 15: 1, 16: 1},
		ShelvedQuests:    map[int]string{15: "start"},
	}
	return qs, c
}

func TestCompletedQuestIds(t *testing.T) {
	qs, c := completedFixture()
	lookup := func(id int) *quests.Quest { return qs[id] }
	require.Equal(t, []int{10, 12}, CompletedQuestIds(c, lookup),
		"a finished one-off and a capped repeatable; not secret, not shelved, not one that can be taken again, not one in progress")
}

// quests all and the Completed tab agree on what is finished for good: the
// one-offs at their end step plus the repeatables quests all marks done.
func TestCompletedQuestIds_MatchesQuestsAll(t *testing.T) {
	qs, c := completedFixture()
	var list []*quests.Quest
	for _, q := range qs {
		list = append(list, q)
	}
	t.Cleanup(quests.SeedQuestsForTest(list...))

	var fromQuestsAll []int
	for id, step := range c.GetQuestProgress() {
		if q := quests.GetQuestById(id); step == "end" && q != nil && !q.Secret && !c.IsShelved(id) {
			fromQuestsAll = append(fromQuestsAll, id)
		}
	}
	for _, row := range finishedRepeatRows(c, c.GetQuestProgress(), false) {
		if row.Description == repeatDoneLine {
			fromQuestsAll = append(fromQuestsAll, row.Id)
		}
	}
	sort.Ints(fromQuestsAll)
	require.Equal(t, fromQuestsAll, CompletedQuestIds(c, quests.GetQuestById))
}
```

`modules/gmcp/gmcp.Char_completed_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func TestCompletedQuestRows(t *testing.T) {
	qs := map[int]*quests.Quest{
		10: {QuestId: 10, Name: "One Off"},
		12: {QuestId: 12, Name: "Capped", Repeatable: true, RepeatLimit: 1},
	}
	lookup := func(id int) *quests.Quest { return qs[id] }
	c := &characters.Character{
		QuestProgress:    map[int]string{10: "end"},
		QuestCompletions: map[int]int{12: 1},
	}
	require.Equal(t, []GMCPCharModule_Payload_Quest{
		{Id: 10, Name: "One Off", Status: questStatusDone},
		{Id: 12, Name: "Capped", Repeatable: true, Status: questStatusDone},
	}, completedQuestRows(c, lookup))
}
```

- [ ] **Step 2: Run, see them fail.** `go test -count=1 -run 'TestCompletedQuest' ./internal/usercommands/ ./modules/gmcp/` fails to compile (`CompletedQuestIds`, `completedQuestRows`, `questStatusDone` undefined). If a field or helper name in the fixture does not match the source (for example the `Quest` struct's repeat limit field), read `internal/quests` and `internal/characters` and adapt the fixture, reporting it.

- [ ] **Step 3: Implement.** `internal/usercommands/quests_completed.go`:

```go
package usercommands

import (
	"sort"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// CompletedQuestIds are the quests c has finished for good, sorted, for the
// web client's Completed tab (plan 7, owner P4): a quest whose progress is at
// its end step, and a repeatable at its repeat cap, neither shelved. Secret
// quests are left out, as plain `quests all` leaves them out. A repeatable
// that can be taken again belongs to the Repeat tab (gmcp
// finishedRepeatQuests). TestCompletedQuestIds_MatchesQuestsAll holds this to
// `quests all`.
func CompletedQuestIds(c *characters.Character, lookup func(int) *quests.Quest) []int {
	seen := map[int]bool{}
	progress := c.GetQuestProgress()
	for id, step := range progress {
		if step != `end` || c.IsShelved(id) {
			continue
		}
		if q := lookup(id); q != nil && !q.Secret {
			seen[id] = true
		}
	}
	for id, n := range c.QuestCompletions {
		if n < 1 || c.IsShelved(id) {
			continue
		}
		if _, inProgress := progress[id]; inProgress {
			continue
		}
		q := lookup(id)
		if q == nil || q.Secret || !q.Repeatable || !c.RepeatCapReached(q) {
			continue
		}
		seen[id] = true
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}
```

In `modules/gmcp/gmcp.Char.go`, the status consts gain `questStatusDone = "done"` with the comment "a quest finished for good: the Completed tab (plan 7)"; add beside `finishedRepeatQuests`:

```go
// completedQuestRows are the Char.Quests rows for quests c has finished for
// good, for the web client's Completed tab (plan 7, owner P4), in
// usercommands.CompletedQuestIds order.
func completedQuestRows(c *characters.Character, lookup func(int) *quests.Quest) []GMCPCharModule_Payload_Quest {
	var rows []GMCPCharModule_Payload_Quest
	for _, id := range usercommands.CompletedQuestIds(c, lookup) {
		if q := lookup(id); q != nil {
			rows = append(rows, GMCPCharModule_Payload_Quest{Id: id, Name: q.Name, Repeatable: q.Repeatable, Status: questStatusDone})
		}
	}
	return rows
}
```

After line 745 (`payload.Quests = append(payload.Quests, finishedRepeatQuests(...)...)`):

```go
		payload.Quests = append(payload.Quests, completedQuestRows(user.Character, quests.GetQuestById)...)
```

Update the `Status` field comment at `:1201` to name all three values: `ready`, `waiting` (a repeatable finished and offered again or not yet), `done` (finished for good).

- [ ] **Step 4: Pass.** `go test -count=1 ./internal/usercommands/ ./modules/gmcp/` all `ok`; `gofmt -l internal/ modules/` empty.

- [ ] **Step 5: Commit** `feat(quests): Char.Quests carries finished quests for a Completed tab (plan 7)` with the four Go files.

---

### Task 2: `questtabs.js`

**Files:** Create `JS/questtabs.js`, `tools/jstest/questtabs.test.js`.

- [ ] **Step 1: Failing test** `tools/jstest/questtabs.test.js`:

```js
// questtabs.test.js: the Quests panel's three tabs (web client base plan 7,
// layout spec rulings 18 and 19, owner P4).
// Run: node tools/jstest/questtabs.test.js
'use strict';
var path = require('path');
var QT = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'questtabs.js'));
var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++; console.log('FAIL ' + name + '\n  got  ' + g + '\n  want ' + w);
}
var rows = [
    { id: 1, name: 'The Truth', focused: true },
    { id: 2, name: 'Rope for Hadwen', repeatable: true },
    { id: 3, name: 'Night Watch', repeatable: true, status: 'ready' },
    { id: 4, name: 'Mill Wheel Grease', repeatable: true, status: 'waiting' },
    { id: 5, name: 'Rat Patrol', repeatable: true, status: 'ready' },
    { id: 6, name: 'Zebra Hunt', status: 'done' },
    { id: 7, name: 'Apple Picking', repeatable: true, status: 'done' }
];
var s = QT.split(rows);
check('active: rows with no status', s.active.map(function (q) { return q.id; }), [1, 2]);
check('repeat: ready first, then by name', s.repeat.map(function (q) { return q.id; }), [3, 5, 4]);
check('completed: by name', s.done.map(function (q) { return q.id; }), [7, 6]);
check('ready count', s.ready, 2);
check('label with ready', QT.repeatLabel(2), 'Repeat (2)');
check('label with none ready', QT.repeatLabel(0), 'Repeat');
check('empty lines', [QT.EMPTY.active, QT.EMPTY.repeat, QT.EMPTY.done],
    ['No active quests.', 'Nothing to repeat yet.', 'Nothing finished yet.']);
check('an unknown tab falls back to Active', QT.tabOrDefault('bogus'), 'active');
check('a known tab is kept', QT.tabOrDefault('done'), 'done');
check('no data: three empty tabs', QT.split(null), { active: [], repeat: [], done: [], ready: 0 });
check('an unknown status is not shown anywhere', QT.split([{ id: 9, name: 'x', status: 'odd' }]),
    { active: [], repeat: [], done: [], ready: 0 });
check('the input is not reordered', rows.map(function (q) { return q.id; }), [1, 2, 3, 4, 5, 6, 7]);
console.log(failures ? failures + ' FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
```

- [ ] **Step 2:** `node tools/jstest/questtabs.test.js` fails (`Cannot find module`).

- [ ] **Step 3:** `JS/questtabs.js`:

```js
// questtabs.js: splits the Char.Quests rows into the Quests panel's three
// tabs (web client base plan 7; layout spec rulings 18 and 19; owner P4).
// Active: no status. Repeat: a repeatable finished and offered again
// ("ready") or not yet ("waiting"), ready first, then by name. Completed:
// finished for good ("done"), by name. Tested by tools/jstest/questtabs.test.js.
(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.QuestTabs = api;
    }
}(typeof self !== 'undefined' ? self : this, function () {
    var TABS = ['active', 'repeat', 'done'];
    var EMPTY = { active: 'No active quests.', repeat: 'Nothing to repeat yet.', done: 'Nothing finished yet.' };

    function byName(a, b) { return String(a.name || '').localeCompare(String(b.name || '')); }

    function split(rows) {
        var out = { active: [], repeat: [], done: [], ready: 0 };
        (Array.isArray(rows) ? rows : []).forEach(function (q) {
            if (!q || typeof q !== 'object') { return; }
            if (!q.status) { out.active.push(q); }
            else if (q.status === 'ready' || q.status === 'waiting') { out.repeat.push(q); }
            else if (q.status === 'done') { out.done.push(q); }
        });
        out.repeat.sort(function (a, b) {
            var ra = a.status === 'ready' ? 0 : 1, rb = b.status === 'ready' ? 0 : 1;
            return ra !== rb ? ra - rb : byName(a, b);
        });
        out.done.sort(byName);
        out.ready = out.repeat.filter(function (q) { return q.status === 'ready'; }).length;
        return out;
    }

    function repeatLabel(ready) { return ready > 0 ? 'Repeat (' + ready + ')' : 'Repeat'; }
    function tabOrDefault(tab) { return TABS.indexOf(tab) >= 0 ? tab : 'active'; }

    return { TABS: TABS, EMPTY: EMPTY, split: split, repeatLabel: repeatLabel, tabOrDefault: tabOrDefault };
}));
```

- [ ] **Step 4:** the test passes. **Step 5: Commit** `feat(webclient): the quest list splits into Active, Repeat and Completed (plan 7)`.

---

### Task 3: The Quests panel's tabs

**Files:** Modify `WP` (head script tag, `:352`, `renderQuests`, a tab click handler), `CSS:350-395`, `tools/jstest/hotinput.test.js:473-477`, `tools/jstest/webclient-html-escape.test.js`.

- [ ] **Step 1: Failing page checks.** Append to `tools/jstest/hotinput.test.js` (it already reads `page` and `css`), and replace its three repeat-rule checks (`:473-477`) with these:

```js
// Plan 7, rulings 18 and 19, P4: Quests has Active, Repeat and Completed tabs.
check('the page loads questtabs.js', /static\/js\/questtabs\.js\?v=/.test(page), true);
check('the Quests panel has its three tabs in the existing subtab pattern',
    /<div id="quest-tabs" class="subtabs" role="group" aria-label="Quests">\s*<button type="button" class="subtab active" aria-pressed="true" data-qtab="active">Active<\/button>\s*<button type="button" class="subtab" aria-pressed="false" data-qtab="repeat">Repeat<\/button>\s*<button type="button" class="subtab" aria-pressed="false" data-qtab="done">Completed<\/button>\s*<\/div>/.test(page), true);
check('the old "Repeatable work" heading is gone', /Repeatable work/.test(page), false);
check('Repeat rows end "Ready now" or "Not yet"', /'Ready now' : 'Not yet'/.test(page), true);
check('the quest list uses the game window line spacing',
    /\.qrow \.qname \{[^}]*line-height:\s*1\.2;/.test(css) && /\.qhint \{[^}]*line-height:\s*1\.2;/.test(css) && /\.qprogress \{[^}]*line-height:\s*1\.2;/.test(css), true);
check('a not-ready Repeat row and a Completed row are dimmed by colour, never see-through',
    /\.qrepeatrow:not\(\.ready\)[^{]*\{[^}]*color: var\(--quest-dim\)/.test(css) && /\.qdonerow[^{]*\{[^}]*color: var\(--quest-dim\)/.test(css)
    && !/\.(qrepeatrow|qdonerow)[^{]*\{[^}]*opacity/.test(css), true);
```

In `tools/jstest/webclient-html-escape.test.js`, load the real `questtabs.js` into the evaluated environment so `window.QuestTabs` exists (read how the file builds its fake `window`; add `QuestTabs: require(<path to questtabs.js>)` to it), and make its quest fixtures include one `ready`, one `waiting` and one `done` row whose names carry `<script>`; render with each of the three tabs selected (`renderQuests.tab = 'repeat'` and so on, then `renderQuests()`) and check the markup holds the escaped names and no raw `<script>`.

- [ ] **Step 2:** `node tools/jstest/hotinput.test.js` and `node tools/jstest/webclient-html-escape.test.js` fail on the new checks.

- [ ] **Step 3: Implement.**
  - Head: `<script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/questtabs.js?v={{ .ASSET_VERSION }}"></script>` beside the `hotinput.js` tag.
  - `WP:352` becomes (the tabs inside the panel body, above the list):

```html
          <div class="dash-panel-body"><div id="quest-tabs" class="subtabs" role="group" aria-label="Quests">
            <button type="button" class="subtab active" aria-pressed="true" data-qtab="active">Active</button>
            <button type="button" class="subtab" aria-pressed="false" data-qtab="repeat">Repeat</button>
            <button type="button" class="subtab" aria-pressed="false" data-qtab="done">Completed</button>
          </div><div id="quests-list"><div class="quests-empty">No active quests.</div></div></div>
```

  - In `renderQuests`, replace from `var repeats = quests.filter(` through the `if (!quests.length) { ... return; }` block with:

```js
            // Plan 7: Active, Repeat and Completed tabs (QuestTabs). The list
            // host shows the selected tab; the choice lives on the function so
            // it survives every push, per page load.
            var QT = window.QuestTabs;
            var tabs = QT.split(quests);
            var tab = renderQuests.tab = QT.tabOrDefault(renderQuests.tab);
            updateQuestTabs(tab, tabs.ready);
            if (tab === "repeat" || tab === "done") {
                var rows = tab === "repeat" ? tabs.repeat : tabs.done;
                var listHTML = rows.length ? "" : '<div class="quests-empty">' + QT.EMPTY[tab] + '</div>';
                rows.forEach(function (q) {
                    if (tab === "repeat") {
                        var ready = q.status === "ready";
                        listHTML += '<div class="qrepeatrow' + (ready ? ' ready' : '') + '" data-quest-id="' + Number(q.id) + '">'
                            + '<span class="qname">' + escapeHTML(q.name || "") + '</span>'
                            + '<span class="qstate">' + (ready ? 'Ready now' : 'Not yet') + '</span></div>';
                    } else {
                        listHTML += '<div class="qdonerow" data-quest-id="' + Number(q.id) + '">'
                            + '<span class="qname">' + escapeHTML(q.name || "") + '</span></div>';
                    }
                });
                writeQuestsHTML(host, listHTML);
                return;
            }
            quests = tabs.active;
            if (!quests.length) {
                writeQuestsHTML(host, '<div class="quests-empty">' + QT.EMPTY.active + '</div>');
                return;
            }
```

    and remove the later `html += repeatsHTML;`. Keep the "(repeatable)" tag on active rows. Add after `writeQuestsHTML`:

```js
        // The Quests tab bar: which tab is pressed, and the Repeat count.
        function updateQuestTabs(tab, ready) {
            var bar = document.getElementById("quest-tabs");
            if (!bar) { return; }
            Array.prototype.forEach.call(bar.querySelectorAll("button[data-qtab]"), function (b) {
                var on = b.getAttribute("data-qtab") === tab;
                b.classList.toggle("active", on);
                b.setAttribute("aria-pressed", on ? "true" : "false");
                if (b.getAttribute("data-qtab") === "repeat") { b.textContent = window.QuestTabs.repeatLabel(ready); }
            });
        }
        (function () {
            var bar = document.getElementById("quest-tabs");
            if (!bar) { return; }
            bar.addEventListener("click", function (e) {
                var b = e.target.closest("button[data-qtab]");
                if (!b) { return; }
                renderQuests.tab = b.getAttribute("data-qtab");
                renderQuests();
            });
        })();
```

  - CSS: `.qrow .qname`, `.qhint`, `.qprogress` line-height `1.2` (ruling 19, the game window's, `WP:470`). Replace the `.qgroup` and `.qrepeatrow` rules (keep `.qrepeat`, the active rows' tag) with:

```css
/* Repeat tab (plan 7, ruling 18): name, then "Ready now" or "Not yet" at the
   right. A row not ready yet is quieter by colour, never see-through (#768
   review item 3). Completed rows are quiet the same way. */
.qrepeatrow, .qdonerow { display: flex; align-items: baseline; gap: 8px; padding: 5px 5px 4px; font-family: var(--font-body); font-size: var(--text); line-height: 1.2; }
.qrepeatrow .qname, .qdonerow .qname { flex: 1; min-width: 0; overflow-wrap: anywhere; font-weight: var(--bold); color: var(--quest-name); }
.qrepeatrow .qstate { flex: 0 0 auto; color: var(--quest-progress); }
.qrepeatrow.ready .qstate { color: var(--brass-mid); }
.qrepeatrow:not(.ready), .qrepeatrow:not(.ready) .qname, .qrepeatrow:not(.ready) .qstate { color: var(--quest-dim); }
.qdonerow, .qdonerow .qname { color: var(--quest-dim); }
#quest-tabs { margin: 0 0 4px; }
```

    Check `--quest-dim` and `--brass-mid` meet contrast on the panel in dark and light mode (the comment block at `CSS:385-395` records the measured ratios for the old rules; measure the new ones the same way, and if `--quest-dim` text falls under 4.5:1 use `--quest-progress` for the dimmed state instead and say so).

- [ ] **Step 4:** all jstests pass. **Step 5: Commit** `feat(webclient): Quests gets Active, Repeat and Completed tabs (plan 7)`.

---

### Task 4: The Map title, the bar, no text on the art

**Files:** Create `JS/mapwhere.js`, `tools/jstest/mapwhere.test.js`; modify `WP` (head tag, `:329-336` markup, the `Zone.Map`, `Room` and `Char.Quests` handlers), `JS/gmcp.js`, `CSS` (map bar).

- [ ] **Step 1: Failing tests.** `tools/jstest/mapwhere.test.js`:

```js
// mapwhere.test.js: the Map's Location / Destination bar and the art's text
// (web client base plan 7, ruling 17, owner P1 and P6).
// Run: node tools/jstest/mapwhere.test.js
'use strict';
var fs = require('fs');
var path = require('path');
var PUB = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var MW = require(path.join(PUB, 'static', 'js', 'mapwhere.js'));
var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++; console.log('FAIL ' + name + '\n  got  ' + g + '\n  want ' + w);
}
check('room and a focused quest with a target label',
    MW.text('Mill Lane', { name: 'Water for the Mill', target_label: 'Eastfield' }), { location: 'Mill Lane', destination: 'Eastfield' });
check('no target label: the quest name', MW.text('Mill Lane', { name: 'Water for the Mill' }), { location: 'Mill Lane', destination: 'Water for the Mill' });
check('no focused quest: no destination', MW.text('Mill Lane', null), { location: 'Mill Lane', destination: '' });
check('nothing yet', MW.text(undefined, undefined), { location: '', destination: '' });
check('blank names trim to nothing', MW.text('  ', { name: ' ', target_label: ' ' }), { location: '', destination: '' });

// The art keeps only the zone-exit labels and one-character marks (P1, P6).
var src = fs.readFileSync(path.join(PUB, 'static', 'js', 'gmcp.js'), 'utf8');
var calls = src.split('\n').filter(function (l) {
    return /\b(lTxt|embText)\(/.test(l) && !/^\s*function (lTxt|embText)\(/.test(l) && !/g\.appendChild\(lTxt\(a, s\)\)/.test(l);
}).length;
check('the map renderer draws five text kinds: compass N, service glyph, zone-exit label, ?, stairs', calls, 5);
check('no title, level, room name, quest name, area label or banner text',
    /~ Level |Somewhere in |_drawQuestBanner|data-wash-label/.test(src), false);
check('the zone-exit label stays', /"→ " \+ e\.tozone/.test(src), true);

var page = fs.readFileSync(path.join(PUB, 'webclient-pure.html'), 'utf8');
check('the page loads mapwhere.js', /static\/js\/mapwhere\.js\?v=/.test(page), true);
check('the bar sits between the map buttons and the legend',
    page.indexOf('id="map-controls"') < page.indexOf('id="map-where"') && page.indexOf('id="map-where"') < page.indexOf('id="map-legend"'), true);
check('the title carries the level', /'Map \(' \+ obj\.Map\.zone \+ ', Level ' \+ \(obj\.Map\.cz \|\| 0\) \+ '\)'/.test(page), true);
check('the bar is refreshed on Room and Char.Quests', (page.match(/updateMapWhere\(\);/g) || []).length >= 2, true);
console.log(failures ? failures + ' FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
```

- [ ] **Step 2:** it fails (`Cannot find module`).

- [ ] **Step 3: Implement.** `JS/mapwhere.js`:

```js
// mapwhere.js: the words of the Map's Location / Destination bar (web client
// base plan 7, ruling 17). roomName: Room.Info.name. quest: the focused
// Char.Quests row, or null. Destination is its target label, else its name;
// empty when no quest is focused. Tested by tools/jstest/mapwhere.test.js.
(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.MapWhere = api;
    }
}(typeof self !== 'undefined' ? self : this, function () {
    function clean(s) { return String(s == null ? '' : s).trim(); }
    function text(roomName, quest) {
        var dest = '';
        if (quest) { dest = clean(quest.target_label) || clean(quest.name); }
        return { location: clean(roomName), destination: dest };
    }
    return { text: text };
}));
```

  - Head: the `mapwhere.js` script tag beside `questtabs.js`.
  - Markup: right after the closing `</div>` of `#map-controls` and before `#map-legend`:

```html
            <!-- Where you are and where your focused quest leads (plan 7,
                 ruling 17); the map art carries no words (P1). -->
            <div id="map-where" class="map-where">
              <span class="mw-loc" hidden><span class="mw-k">Location:</span> <span class="mw-v"></span></span>
              <span class="mw-dest" hidden><span class="mw-k">Destination:</span> <span class="mw-v"></span></span>
            </div>
```

  - Title (`WP:990`): `if (mapTitle) mapTitle.textContent = 'Map (' + obj.Map.zone + ', Level ' + (obj.Map.cz || 0) + ')';`
  - Near `renderQuests`, add:

```js
        // The Map's Location / Destination bar (plan 7, ruling 17).
        function updateMapWhere() {
            if (!window.MapWhere) { return; }
            var room = (GMCPStructs.Room && GMCPStructs.Room.Info) ? GMCPStructs.Room.Info.name : "";
            var data = (GMCPStructs.Char && GMCPStructs.Char.Quests) ? GMCPStructs.Char.Quests : [];
            var f = Array.isArray(data) ? data.filter(function (q) { return q && q.focused && !q.status; })[0] : null;
            var t = MapWhere.text(room, f);
            [[".mw-loc", t.location], [".mw-dest", t.destination]].forEach(function (p) {
                var el = document.querySelector("#map-where " + p[0]);
                if (!el) { return; }
                el.querySelector(".mw-v").textContent = p[1];
                el.hidden = !p[1];
            });
        }
```

    and call `updateMapWhere();` at the end of the `"Room"` handler's `Room.Info` branch and in the `"Char.Quests"` handler after `renderQuests();`.
  - `JS/gmcp.js`: delete the area-label plate and text (the `if (qm.label) { ... }` block at `:700-709`), the `_drawQuestBanner` method (`:712-737`, with its doc comment) and every call to it (grep `_drawQuestBanner`), the title and level draws (`:954-962`, keep the comment header only if it still describes something), the current room's name (`:1012-1017`, the `"Italic name label beneath"` comment with it), and the focused quest's name (`:1060-1066`). Keep the wash, its outline, the pin, the compass N, service glyphs, the zone-exit label, `?` and stairs. Grep the file for `data-quest-banner` and `data-wash-label` and remove now-dead cleanup.
  - CSS, after the `.map-legend` rules:

```css
/* The Map's Location / Destination bar (plan 7, ruling 17). Two spans that
   wrap to two lines when narrow; each ellipsises inside the panel. */
.map-where { display: flex; flex-wrap: wrap; gap: 2px 16px; margin: 6px 4px 0; padding: 6px 4px 0; border-top: 1px solid color-mix(in srgb, var(--panel-border) 70%, transparent); font-family: var(--font-body); font-size: var(--text); line-height: 1.2; }
.map-where > span { min-width: 0; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.map-where .mw-k { color: var(--ink-gold); }
.map-where .mw-v { color: var(--title-gold); }
.map-where .mw-dest .mw-v { color: var(--quest-name-focus); }
```

  - Check the map's own jstests and any test naming the removed draws (grep `tools/jstest` for `Level `, `Somewhere in`, `_drawQuestBanner`) and update them to the new truth.

- [ ] **Step 4:** all jstests pass. **Step 5: Commit** `feat(webclient): the Map title shows the level and a bar names where you are (plan 7)`.

---

### Task 5: Comms

**Files:** Modify `WP:376`, `:379`, `:382`, `:385`; `JS/dashboard.js:48`; `tools/jstest/viewportfit.test.js:258`; `tools/jstest/hotinput.test.js`.

- [ ] **Step 1: Failing check** (append to `hotinput.test.js`):

```js
// Plan 7, ruling 16: the panel is Comms; the Chat channel keeps its name.
check('the Comms panel title', /<span class="ph-title">Comms<\/span>/.test(page), true);
check('no panel says Chat any more', /<span class="ph-title">Chat<\/span>|aria-label="Chat channels"|read Chat\./.test(page), false);
check('the Comms channels are named for screen readers', /aria-label="Comms channels"/.test(page), true);
check('the Chat channel keeps its name', /id="comm-tab-chat"[^>]*data-label="Chat"/.test(page), true);
check('the phone tab is Comms', /chat:\s*\{ label: "Comms",/.test(dash), true);
```

(`dash` is the `dashboard.js` source the file already reads; if it reads it later in the file, move the check below that.)

- [ ] **Step 2:** fails. **Step 3:** change the title, the hint (`...more room to read Comms.`), `aria-label="Comms channels"`, the HTML comment, and `dashboard.js:48` `label: "Comms"` (which also renames the tab's aria label, its name pill and "Close Comms"). Leave `#panel-chat`, `data-panel="chat"`, `comm-tab-chat` and the channel's `data-label="Chat"`. Update `viewportfit.test.js:258` to the new hint. Grep `tools/jstest` for `"Chat"` and `Close Chat` and update only the panel ones.

- [ ] **Step 4:** all jstests pass. **Step 5: Commit** `feat(webclient): the Chat panel is called Comms (plan 7)`.

---

### Task 6: One menu look; Equipped tidy

**Files:** Modify `CSS:1263-1300`, `:1465-1503`, `:1506-1550` (and every other `.inv-ctx`, `.auto-ctx`, `.quest-ctx` rule: grep, including light mode), `CSS:1186-1196`; `WP:4036-4041`, `:4261-4269`, `:4375-4381` (each `buildMenu`).

- [ ] **Step 1: Failing checks** (append to `hotinput.test.js`):

```js
// Plan 7, ruling 20: one menu look for items, Automation and Quests.
['inv-ctx', 'auto-ctx', 'quest-ctx'].forEach(function (cls) {
    check(cls + ' is built as a shared ctx-menu', new RegExp('el\\.className = "' + cls + ' ctx-menu";').test(page), true);
});
check('the menu header is the name at the title size, boxed, regular weight',
    /\.ctx-menu \.ctx-header \{[^}]*font-size: var\(--title\);[^}]*font-weight: 400;[^}]*border: 1px solid var\(--gold-rule\);/.test(css), true);
check('menu rows are indented under the name and tall enough to tap',
    /\.ctx-menu \.ctx-row \{[^}]*padding: 8px 10px 8px 24px;/.test(css), true);
check('no per-menu copies of the menu rules are left',
    /\.(inv|auto|quest)-ctx \.ctx-(header|row|sep)\b/.test(css), false);
check('the unused empty-tile rule is gone', /\.inv-tile\.empty\b/.test(css), false);
```

- [ ] **Step 2:** fails. **Step 3: Implement.**
  - Each `buildMenu`: `el.className = "inv-ctx ctx-menu";` (and `auto-ctx ctx-menu`, `quest-ctx ctx-menu`).
  - Replace the three copies with one set. Read all three first and keep any rule only one of them has (for example `.ctx-disabled`, `.ctx-sep` margins, light-mode overrides), folded under `.ctx-menu`:

```css
/* Right-click and long-press menus: items, Automation and Quests share one
   look (plan 7, ruling 20). The name sits boxed at the title size, apart from
   the actions, which are indented under it and tall enough to tap. */
.ctx-menu {
  position: fixed;
  min-width: 160px;
  padding-bottom: 4px;
  background: var(--ink-deep);
  border: 1px solid var(--gold-rule);
  border-radius: 5px;
  box-shadow: 0 6px 18px color-mix(in srgb, var(--shadow) 65%, transparent);
  overflow: hidden;
  z-index: 9000;
  font-family: var(--font-body);
  font-size: var(--text);
}
.ctx-menu .ctx-header {
  margin: 6px;
  padding: 6px 10px;
  font-size: var(--title);
  font-weight: 400;
  color: var(--title-gold);
  background: var(--panel-bg);
  border: 1px solid var(--gold-rule);
  border-radius: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: min(28ch, 80vw);
  pointer-events: none;
  user-select: none;
}
.ctx-menu .ctx-row {
  padding: 8px 10px 8px 24px;
  color: var(--vellum);
  cursor: pointer;
  white-space: nowrap;
}
.ctx-menu .ctx-row:hover,
.ctx-menu .ctx-row.ctx-default {
  background: var(--panel-border);
  color: var(--ink-gold);
}
.ctx-menu .ctx-sep {
  margin: 0 6px;
  border-top: 1px solid color-mix(in srgb, var(--panel-border) 70%, transparent);
}
```

  - Equipped: delete `.inv-tile.empty` rules (`CSS:1190-1196`) after grepping `WP` to confirm nothing sets the class; fix the comment at `CSS:1186` to say a tap or click opens the tile's menu (true after Task 8).

- [ ] **Step 4:** all jstests pass. **Step 5: Commit** `feat(webclient): one menu look, the name boxed at the title size (plan 7)`.

---

### Task 7: `longpress.js`

**Files:** Create `JS/longpress.js`, `tools/jstest/longpress.test.js`.

- [ ] **Step 1: Failing test** `tools/jstest/longpress.test.js`:

```js
// longpress.test.js: a long touch opens a menu on phones (web client base
// plan 7, owner P2; a stopgap until the touchscreen arc's contextual buttons).
// Run: node tools/jstest/longpress.test.js
'use strict';
var path = require('path');
var LP = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'longpress.js'));
var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++; console.log('FAIL ' + name + '\n  got  ' + g + '\n  want ' + w);
}
var now = 0, timers = [];
function setT(f, ms) { var t = { at: now + ms, f: f, live: true }; timers.push(t); return t; }
function clearT(t) { if (t) { t.live = false; } }
function advance(ms) { now += ms; timers.forEach(function (t) { if (t.live && t.at <= now) { t.live = false; t.f(); } }); }
var fired = [];
var p = LP.createPress({ setTimeout: setT, clearTimeout: clearT, fire: function (x, y) { fired.push([x, y]); } });

check('half a second', LP.DELAY_MS, 500);
check('ten pixels of give', LP.MOVE_PX, 10);
p.down(100, 200); advance(499);
check('not yet at 499ms', fired.length, 0);
advance(1);
check('fires at 500ms where the finger is', fired, [[100, 200]]);
check('the lift after it reports a long press (swallow the click)', p.up(), true);

p.down(10, 10); advance(200);
check('a short tap is not a long press', p.up(), false);
advance(1000);
check('and never fires', fired.length, 1);

p.down(10, 10); p.move(15, 18); advance(600);
check('a small wobble still fires', fired.length, 2);
p.up();

p.down(10, 10); p.move(25, 10); advance(600);
check('a move past 10px cancels', fired.length, 2);
check('and its lift is a plain lift', p.up(), false);

p.down(10, 10); p.cancel(); advance(600);
check('a cancel (scroll, second finger) stops it', fired.length, 2);
console.log(failures ? failures + ' FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
```

- [ ] **Step 2:** fails. **Step 3:** `JS/longpress.js`:

```js
// longpress.js: a long touch opens a context menu on phones and tablets (web
// client base plan 7, owner P2). A stopgap: the touchscreen arc replaces it
// with contextual buttons (P3, #780). Mouse users keep right-click.
// createPress is the timing, pure and tested (tools/jstest/longpress.test.js);
// attach wires it to touch events for elements matching a selector.
(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.LongPress = api;
    }
}(typeof self !== 'undefined' ? self : this, function () {
    var DELAY_MS = 500;   // common platform value (Android 400 to 500ms)
    var MOVE_PX = 10;     // a finger that strays further is scrolling

    // opts: { setTimeout, clearTimeout, fire(x, y) }
    function createPress(opts) {
        var timer = null, sx = 0, sy = 0, did = false;
        function clear() { if (timer) { opts.clearTimeout(timer); timer = null; } }
        return {
            down: function (x, y) {
                clear(); did = false; sx = x; sy = y;
                timer = opts.setTimeout(function () { timer = null; did = true; opts.fire(sx, sy); }, DELAY_MS);
            },
            move: function (x, y) {
                if (timer && (Math.abs(x - sx) > MOVE_PX || Math.abs(y - sy) > MOVE_PX)) { clear(); }
            },
            // true when this press fired: the caller cancels the click that follows.
            up: function () { clear(); var d = did; did = false; return d; },
            cancel: function () { clear(); did = false; }
        };
    }

    // A long touch on an element matching selector calls open(x, y, el).
    function attach(doc, selector, open) {
        var el = null;
        var press = createPress({
            setTimeout: function (f, ms) { return setTimeout(f, ms); },
            clearTimeout: function (t) { clearTimeout(t); },
            fire: function (x, y) { if (el) { open(x, y, el); } }
        });
        doc.addEventListener('touchstart', function (e) {
            if (!e.touches || e.touches.length !== 1) { press.cancel(); el = null; return; }
            el = e.target && e.target.closest ? e.target.closest(selector) : null;
            if (el) { press.down(e.touches[0].clientX, e.touches[0].clientY); }
        }, { passive: true });
        doc.addEventListener('touchmove', function (e) {
            if (e.touches && e.touches[0]) { press.move(e.touches[0].clientX, e.touches[0].clientY); }
        }, { passive: true });
        doc.addEventListener('touchend', function (e) {
            if (press.up() && el) { e.preventDefault(); }   // no click after a long press
            el = null;
        }, { passive: false });
        doc.addEventListener('touchcancel', function () { press.cancel(); el = null; });
        doc.addEventListener('scroll', function () { press.cancel(); }, true);
    }

    return { DELAY_MS: DELAY_MS, MOVE_PX: MOVE_PX, createPress: createPress, attach: attach };
}));
```

- [ ] **Step 4:** passes. **Step 5: Commit** `feat(webclient): a long-press helper for touch menus (plan 7)`.

---

### Task 8: Menus open by tap and long-press

**Files:** Modify `WP` (head tag; the three menu IIFEs' delegation at `:4185-4190`, `:4356-4361`, `:4461-4466`), `CSS` (touch callout); `tools/jstest/hotinput.test.js`.

- [ ] **Step 1: Failing checks** (append to `hotinput.test.js`):

```js
// Plan 7, P2: a long-press opens each menu on touch; a tap on an item tile
// opens its menu; other taps keep their jobs.
check('the page loads longpress.js', /static\/js\/longpress\.js\?v=/.test(page), true);
['.inv-tile[data-handle]', '.auto-chip[data-auto-type], .auto-row[data-auto-type]', '.qrow[data-quest-id]'].forEach(function (sel) {
    check('long-press opens the menu on ' + sel,
        page.indexOf('LongPress.attach(document, "' + sel + '"') >= 0, true);
});
check('a tap or click on an item tile opens its menu at the tile',
    /addEventListener\("click", function\(e\) \{\s*var tile = e\.target\.closest\("\.inv-tile\[data-handle\]"\);/.test(page), true);
check('a long touch on these does not select text or raise the iOS callout',
    /\.inv-tile, \.qrow, \.auto-chip, \.auto-row \{[^}]*-webkit-touch-callout: none;[^}]*user-select: none;/.test(css), true);
```

- [ ] **Step 2:** fails. **Step 3: Implement.**
  - Head: the `longpress.js` script tag beside `mapwhere.js`.
  - Item menu IIFE, after its `contextmenu` listener:

```js
            // A tap (or a click) on a tile opens its menu at the tile
            // (ruling 20); a long touch opens it where the finger is (P2).
            document.addEventListener("click", function(e) {
                var tile = e.target.closest(".inv-tile[data-handle]");
                if (!tile || !tile.dataset.handle) return;
                var r = tile.getBoundingClientRect();
                openMenu({ clientX: r.left + r.width / 2, clientY: r.bottom, preventDefault: function () {} }, tile);
            });
            if (window.LongPress) LongPress.attach(document, ".inv-tile[data-handle]", function (x, y, el) {
                openMenu({ clientX: x, clientY: y, preventDefault: function () {} }, el);
            });
```

  - Automation and Quest IIFEs, each after its `contextmenu` listener, the same `LongPress.attach` with its own selector string exactly as the check names. A tap on those keeps its job (run the automation, focus the quest).
  - Read each `openMenu` first and confirm it reads only `clientX`, `clientY` and `preventDefault` from the event; if it reads more, pass it in the fake event and say so.
  - The menu opened by the tile's click must not be closed by that same click: `openMenu` adds its `closeOnOutside` capture listener on `document` during the click's bubble phase, so it does not run for this click; confirm by reading, and check it in the browser (Task 9).
  - CSS:

```css
/* A long touch opens a menu on these (plan 7): no text selection and no
   iOS callout fighting it. */
.inv-tile, .qrow, .auto-chip, .auto-row { -webkit-touch-callout: none; -webkit-user-select: none; user-select: none; }
```

- [ ] **Step 4:** all jstests pass. **Step 5: Commit** `feat(webclient): menus open by a tap on an item and a long-press on touch (plan 7)`.

---

### Task 9: Check it in a browser

No code unless a check fails. Boot the worktree on private ports (plan 6's recipe: `CONFIG_PATH` override with `HttpPort: 18090` etc. in the session scratchpad, `go build -o p7.exe .`), instance saves wiped, and with Playwright from `C:/Users/Calabe Davis/workspace/metaprompter/node_modules/playwright`, on `/webclient-pure.html`, create a character through the real client (`new`, name, password twice, Enter, `n`, `y`), then:

- [ ] Desktop 1280x900: the Map title reads `Map (<zone>, Level <n>)`; `#map-where .mw-loc` shows the room title and is not hidden; with no focused quest `.mw-dest` is hidden. The map's SVG holds no `text` element other than the compass N, service glyphs, zone-exit labels, `?` and stairs (collect every `text` element's content and list them).
- [ ] Quests: the three tab buttons exist; clicking Repeat and Completed shows their empty lines; Active shows the starting quest if any.
- [ ] Right-click an item tile, a quest row and an Automation entry (if one exists; create a macro with the Automation panel's add button if none): each menu has class `ctx-menu`, its header's computed `font-size` is 20px, its first row's `padding-left` 24px. A left click on an item tile opens its menu and the menu is still open 300ms later.
- [ ] Phone 390x844 touch: the item tile tap opens the menu; a long-press on a quest row opens its menu (dispatch `touchstart` at the row, wait 600ms, dispatch `touchend`, built with `new Touch({identifier: 1, target: row, clientX, clientY})` inside `page.evaluate`), and a short touch (100ms) on the quest row does not open it.
- [ ] The Comms tab: on the phone the tab bar's chat button's `aria-label` is `Comms`.

Record the results for the PR; stop the server by its task, then confirm no `p7.exe` is left.

---

### Task 10: Docs

- [ ] `internal/usercommands/context.md`: `CompletedQuestIds` (what it counts, the parity test).
- [ ] `modules/gmcp/context.md`: `Char.Quests` rows now carry `status` `done` for quests finished for good (the Completed tab), beside `ready` / `waiting`.
- [ ] The spec: under "Decisions taken without asking" add the plan's two departures (one list host; `quests all` held by a parity test rather than rewritten), each as an "Amended 2026-11-14 (plan 7 plan)" line.
- [ ] `docs/PATCH_NOTES.md`, a new top entry dated the day it ships, player framing, no raw numbers:

```markdown
## 2026-11-14: Clearer panels

- The Map's title names the level you are on, and a line under the map says where you are and where your quest is taking you. The map itself is easier to read with the labels off it.
- Quests has three tabs: Active, Repeat (with how many are ready again) and Completed, a list of everything you have finished.
- The Chat panel is now called Comms. Its Chat channel is unchanged.
- Menus show the item, quest or automation name large at the top, with the actions below it. On a phone, press and hold an item, quest or automation to open its menu; tap an item to open its menu too.
```

- [ ] `docs/README.md`: rows for `static/js/questtabs.js`, `static/js/mapwhere.js`, `static/js/longpress.js` (beside `displayreport.js`), the three new jstests in the `tools/jstest/` row ("Added 2026-11-14 (web client base plan 7): ..."), and `internal/usercommands/quests_completed.go` where `internal/usercommands` files are listed (follow the file's pattern; if it lists none, skip).
- [ ] Verify every symbol named exists (grep). Commit `docs: plan 7, panels`.

---

### Task 11: Gates and PR

- [ ] Gates: `gofmt -l internal/ modules/ *.go` empty; `go build ./...`; `golangci-lint run --new-from-merge-base=origin/master` 0 issues; `go test ./...`; every `tools/jstest/*.test.js`; boot check `Server Ready` 1, 0 panics; `LogToFile: false`.
- [ ] Final whole-branch review (Opus) of `feature/webclient-base-6-phones..HEAD`.
- [ ] Push and open the PR **against `feature/webclient-base-6-phones`** (stacked on #869), `--repo pruuk/DOGMud`, body with the rulings P1 to P6, the plan's decisions, the browser results, the deploy note (no migration, no config; `Char.Quests` gains `done` rows; players see it on their next page load), no closing keywords, ending with the Claude Code attribution line. The real-device test of plans 6 and 7 together follows; both merge after it, #869 first, then this PR retargeted to master.
