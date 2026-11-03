# Web Client Phone Fit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Carry out the owner's 2026-11-03 rulings on the web client: dim grey split by device, a readable Temple map marker, readable brass, phones full screen by default with a way back, and the command box staying above the on-screen keyboard.

**Architecture:** One small tested module, `static/js/viewportfit.js`, holds the decisions: what counts as a phone, whether to open full screen, and how tall the game frame may be. `webclient.html` (the outer page) owns full screen and the frame height. `webclient-pure.html` (the game, inside its iframe) picks the grey and shows the toggle. The game stays in its iframe throughout, so going full screen and back never drops the connection.

**Tech Stack:** Plain browser JS, dependency-free Node tests in `tools/jstest/`, Go `text/template` pages, `tools/webshot` style headless Edge for the live check.

**Out of scope, per the rulings:** phone text size (waits for the touch arc), the MOTD box (waits on Malia's check), vitals label colours and the rest of report section 3.

---

## What the owner sees

| Screen | Before | After |
|---|---|---|
| Phone, game text | Dim text as bright as normal text | Same: bright stays on phones |
| Tablet, laptop, desktop, game text | Dim text as bright as normal text | Dim text is a readable mid grey again (5.83:1), so hints and dividers recede |
| Map, Temple marker | Blank pale block (1.21:1) | Dark marker on the pale block (10.49:1) |
| Brass buttons and tabs | Text about 4.05:1, inactive tabs about 2.6:1 | Text about 5.45:1, inactive tabs about 4.5:1 |
| Phone, opening the web client | Site header and footer take 131 to 150 px | Full screen, with a ⛶ button in the session bar to bring them back |
| Phone or tablet, keyboard open | Command box can sit behind the keyboard | Game text and command box shrink to fit above the keyboard |

## Decisions in this plan worth your eye

1. **A phone is a device whose screen's shorter side is under 600 CSS px.** It reads the screen, so rotating, opening the keyboard or sitting in the iframe cannot flip it. Tablets, including Malia's Wacom, are not phones. A narrow desktop window is not a phone.
2. **Full screen stays in place.** It hides the site header, nav and footer around the iframe. It does not navigate to `/webclient-pure`, which would drop the connection.
3. **The ⛶ toggle lives in the session bar on every screen size,** and the choice is remembered per browser. The desktop ⛶ link had no way back either, so it gets the same fix. A remembered choice beats the phone default.
4. **Brass is fixed by darkening its text** (`--brass-text` to `#1a1206`), with the accepted palette's metal colours unchanged. Map buttons move from the old brass to the new so they match.

## Facts verified against source (2026-11-03, master `73806cac4`)

| Fact | Where |
|---|---|
| `--term-bright-black: #e0d2b2` equals `--term-fg` | `static/css/gomud.css:137,140` |
| `--brass-text: #3b2a10`, `--brass-mid: #aa8636` | `static/css/gomud.css:22,26` |
| xterm theme is read once from `--term-*` at start-up | `webclient-pure.html` `rootStyle`, `termColor` |
| `map-temple: 8` foreground, `mapbg-temple: 15` background | `_datafiles/world/dogmud/ansi-aliases.yaml` |
| Inactive tab: `filter: brightness(0.72); opacity: 0.88` | `static/css/dashboard.css` `.inv-tab.brass` |
| Map buttons use their own old brass | `static/js/gmcp.js:1489`, used at 904 and 1391-1393 |
| ⛶ hides header, nav and footer in place, no way back | `_header.html:16-32` |
| That `_header.html` load script throws on every other page, where `link` is null | `_header.html:18-22` |
| `iframe` is at the top of `webclient.html`, its script after it | `webclient.html:3-4` |
| Frame height comes from `contentContainer.clientHeight` only | `webclient.html:25` |
| `webclient-pure.html` has no viewport meta tag | grep for `viewport` finds no meta |
| `dashboard.css` has no media queries | grep for `@media` is empty |
| `?v={{ .ASSET_VERSION }}` is required on every public asset link | `internal/web/asset_version_test.go` |
| Terminal refits on panel resize and keeps scrollback | `static/js/termfit.js`, `tools/jstest/termfit.test.js` |

Contrast, computed by WCAG formula from the hex values:

| Pair | Ratio |
|---|---|
| `#9a8a74` on terminal background `#0c0c0d` | 5.83 |
| `#9a8a74` against normal text `#e0d2b2` | 2.24 |
| Temple now, `#e0d2b2` on `#f2e6c8` | 1.21 |
| Temple with mid grey, `#9a8a74` on `#f2e6c8` (why the alias must change) | 2.70 |
| Temple with colour 0, `#3a2f25` on `#f2e6c8` | 10.49 |
| `#3b2a10` on brass mid `#aa8636` | 4.05 |
| `#1a1206` on brass mid `#aa8636` | 5.45 |
| `#1a1206` on inactive tab at `brightness(0.9)`, about `#99792f` | about 4.5 |
| `#1a1206` on brass dark `#736133` (the gradient's far corner, not behind text) | 3.08 |

Browser behaviour (web sources, not measured here): iOS Safari and current Chrome on Android shrink only the visual viewport when the keyboard opens, so `100dvh` does not follow it. `visualViewport.height` does. `interactive-widget` and the VirtualKeyboard API are missing on iOS Safari.

## File map

| File | Change |
|---|---|
| `_datafiles/html/public/static/js/viewportfit.js` | Create: `isPhone`, `wantFullScreen`, `frameHeight` |
| `tools/jstest/viewportfit.test.js` | Create: module tests and page wiring checks |
| `tools/jstest/webclient-colours.test.js` | Create: pins the colour values above |
| `_datafiles/html/public/static/css/gomud.css` | Grey split, brass text |
| `_datafiles/html/public/static/css/dashboard.css` | Inactive tab readability |
| `_datafiles/html/public/static/js/gmcp.js` | Map buttons use the new brass |
| `_datafiles/world/dogmud/ansi-aliases.yaml` | Temple marker colour |
| `_datafiles/html/public/webclient.html` | Full screen, keyboard fit |
| `_datafiles/html/public/_header.html` | ⛶ link uses the same full screen code, no error on other pages |
| `_datafiles/html/public/webclient-pure.html` | Phone class, viewport meta, toggle button, top-level keyboard fit |
| `docs/PATCH_NOTES.md`, `docs/README.md` | Entries |

---

### Task 1: Colours

**Files:**
- Create: `tools/jstest/webclient-colours.test.js`
- Modify: `_datafiles/html/public/static/css/gomud.css:26,140`
- Modify: `_datafiles/html/public/static/css/dashboard.css` (`.inv-tab.brass`, `.inv-tab.brass:hover`)
- Modify: `_datafiles/html/public/static/js/gmcp.js:1489`
- Modify: `_datafiles/world/dogmud/ansi-aliases.yaml` (`map-temple`)

- [ ] **Step 1: Write the failing test**

```js
// webclient-colours.test.js: colour rulings of 2026-11-03.
//
// Run:  node tools/jstest/webclient-colours.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var ROOT = path.join(__dirname, '..', '..', '_datafiles');
function read() { return fs.readFileSync(path.join.apply(path, [ROOT].concat([].slice.call(arguments))), 'utf8'); }

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

function luminance(hex) {
    var c = [1, 3, 5].map(function (i) {
        var v = parseInt(hex.slice(i, i + 2), 16) / 255;
        return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
    });
    return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}
function contrast(a, b) {
    var x = luminance(a), y = luminance(b);
    return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}
function token(css, name) {
    var m = new RegExp('--' + name + ':\\s*(#[0-9a-fA-F]{6})').exec(css);
    if (!m) throw new Error('token not found: ' + name);
    return m[1];
}

var css = read('html', 'public', 'static', 'css', 'gomud.css');
var dash = read('html', 'public', 'static', 'css', 'dashboard.css');
var gmcp = read('html', 'public', 'static', 'js', 'gmcp.js');
var aliases = read('world', 'dogmud', 'ansi-aliases.yaml');

var bg = token(css, 'term-bg'), fg = token(css, 'term-fg');
var dim = token(css, 'term-bright-black');
var mid = token(css, 'brass-mid'), text = token(css, 'brass-text');

check('dim grey passes contrast on the terminal', contrast(dim, bg) >= 4.5, dim);
check('dim grey is dimmer than normal text', contrast(dim, fg) > 1.5, dim);
var phone = /:root\.phone\s*\{\s*--term-bright-black:\s*(#[0-9a-fA-F]{6})/.exec(css);
check('phones have their own dim grey', !!phone, 'no :root.phone rule');
check('the phone dim grey is the normal text colour', phone && phone[1] === fg, phone && phone[1]);

check('brass text passes on brass', contrast(text, mid) >= 4.5, text + ' on ' + mid);
check('map buttons use the page brass', gmcp.indexOf('brass: ["' + token(css, 'brass-light') + '", "' + mid + '", "' + token(css, 'brass-dark') + '"], brassText: "' + text + '"') !== -1, 'gmcp.js L.brass');

var tab = /\.inv-tab\.brass \{[^}]*\}/.exec(dash)[0];
var dimmed = /brightness\(([0-9.]+)\)/.exec(tab);
check('an inactive tab keeps its text readable', dimmed && Number(dimmed[1]) >= 0.9, tab);
check('an inactive tab is not see-through', !/opacity:\s*0\./.test(tab), tab);

check('the Temple marker has a dark colour on its pale block', /^\s*map-temple:\s*0\b/m.test(aliases), 'map-temple');
check('the Temple block stays pale', /^\s*mapbg-temple:\s*15\b/m.test(aliases), 'mapbg-temple');

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
```

- [ ] **Step 2: Run it and see it fail**

Run: `node tools/jstest/webclient-colours.test.js`
Expected: FAIL on the dim grey, phone rule, brass text, map buttons, tab and Temple checks.

- [ ] **Step 3: Change `gomud.css`**

Line 26 becomes:

```css
  --brass-text: #1a1206;
```

Line 140 becomes:

```css
  --term-black: #3a2f25;   --term-bright-black: #9a8a74;
```

Directly after the `:root { ... }` block that holds these tokens, add:

```css
/* Dim grey on phones is the normal text colour: at phone text sizes a mid grey
   is too hard to read (Malia, 2026-11-01; owner, 2026-11-03). The class is set
   before the terminal reads its colours (webclient-pure.html). */
:root.phone { --term-bright-black: #e0d2b2; }
```

- [ ] **Step 4: Change `dashboard.css`**

In `.inv-tab.brass` replace `filter: brightness(0.72);` and `opacity: 0.88;` with:

```css
  filter: brightness(0.9);
```

In `.inv-tab.brass:hover` replace `filter: brightness(0.85);` and `opacity: 1;` with:

```css
  filter: brightness(0.96);
```

In `.inv-tab.brass.active` keep the box shadow and `filter: brightness(1);` and remove `opacity: 1;`. In the `.inv-tab.brass` transition, change to `transition: filter 0.08s;`.

- [ ] **Step 5: Change `gmcp.js:1489`**

```js
  brass: ["#f4dd92", "#aa8636", "#736133"], brassText: "#1a1206", brassBorder: "#50371d",
```

- [ ] **Step 6: Change `ansi-aliases.yaml`**

```yaml
  map-temple: 0 # Dark marker on its pale block (mapbg-temple)
```

- [ ] **Step 7: Run the tests**

Run: `node tools/jstest/webclient-colours.test.js`
Expected: `all checks passed`

- [ ] **Step 8: Commit**

```bash
git add tools/jstest/webclient-colours.test.js _datafiles/html/public/static/css/gomud.css _datafiles/html/public/static/css/dashboard.css _datafiles/html/public/static/js/gmcp.js _datafiles/world/dogmud/ansi-aliases.yaml
git commit -F - <<'EOF'
fix(webclient): dim grey by device, Temple marker, readable brass

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 2: The decisions module

**Files:**
- Create: `_datafiles/html/public/static/js/viewportfit.js`
- Create: `tools/jstest/viewportfit.test.js`

- [ ] **Step 1: Write the failing test**

```js
// viewportfit.test.js: what counts as a phone, when to open full screen, and
// how tall the game frame may be with the on-screen keyboard open.
//
// Run:  node tools/jstest/viewportfit.test.js
'use strict';

var fs = require('fs');
var path = require('path');
var PUBLIC = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var VF = require(path.join(PUBLIC, 'static', 'js', 'viewportfit.js'));

var failures = 0;
function check(name, got, want) {
    if (got === want) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  got ' + got + ', want ' + want);
}

check('a phone held upright is a phone', VF.isPhone(390, 844), true);
check('a phone on its side is a phone', VF.isPhone(844, 390), true);
check('a small tablet is not a phone', VF.isPhone(600, 960), false);
check('a laptop is not a phone', VF.isPhone(1920, 1080), false);

check('a phone opens full screen', VF.wantFullScreen(null, true), true);
check('a larger screen does not', VF.wantFullScreen(null, false), false);
check('a phone player who turned it off stays off', VF.wantFullScreen('0', true), false);
check('a desktop player who turned it on stays on', VF.wantFullScreen('1', false), true);

check('no keyboard: the frame fills its space', VF.frameHeight(700, 700, 0), 700);
check('keyboard open: the frame fits above it', VF.frameHeight(700, 400, 0), 400);
check('a header above the frame counts', VF.frameHeight(600, 400, 100), 300);
check('a fractional height rounds down, never under the keyboard', VF.frameHeight(700, 400.6, 0), 400);
check('no visual viewport: the frame fills its space', VF.frameHeight(700, undefined, 0), 700);
check('never negative', VF.frameHeight(700, 50, 100), 0);

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
```

- [ ] **Step 2: Run it and see it fail**

Run: `node tools/jstest/viewportfit.test.js`
Expected: FAIL with `Cannot find module`

- [ ] **Step 3: Write the module**

```js
// viewportfit.js: phone detection, the full screen default, and the height of
// the game frame with the on-screen keyboard open.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.ViewportFit = api;    // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    // Judged from the screen, so rotating, the keyboard and the iframe cannot
    // change the answer. A phone's shorter side is under 600 CSS px; a tablet's
    // is not.
    function isPhone(screenW, screenH) {
        return Math.min(screenW, screenH) < 600;
    }

    // pref is the remembered choice: '1', '0', or null for none yet.
    function wantFullScreen(pref, phone) {
        if (pref === '1') return true;
        if (pref === '0') return false;
        return phone;
    }

    // layoutH: the height the page gives the frame. visibleH: the visual
    // viewport's height, which excludes the keyboard; undefined where the
    // browser has none. top: the frame's top edge within the visible area.
    function frameHeight(layoutH, visibleH, top) {
        if (typeof visibleH !== 'number') return layoutH;
        return Math.max(0, Math.min(layoutH, Math.floor(visibleH) - top));
    }

    return { isPhone: isPhone, wantFullScreen: wantFullScreen, frameHeight: frameHeight };
}));
```

- [ ] **Step 4: Run the test**

Run: `node tools/jstest/viewportfit.test.js`
Expected: `all checks passed`

- [ ] **Step 5: Commit**

```bash
git add _datafiles/html/public/static/js/viewportfit.js tools/jstest/viewportfit.test.js
git commit -F - <<'EOF'
feat(webclient): phone, full screen and keyboard height decisions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 3: Outer page: full screen and keyboard fit

**Files:**
- Modify: `_datafiles/html/public/webclient.html`
- Modify: `_datafiles/html/public/_header.html:16-32`
- Modify: `tools/jstest/viewportfit.test.js`

- [ ] **Step 1: Add the failing wiring checks**

In `viewportfit.test.js`, before the final `console.log`, add:

```js
var outer = fs.readFileSync(path.join(PUBLIC, 'webclient.html'), 'utf8');
var header = fs.readFileSync(path.join(PUBLIC, '_header.html'), 'utf8');
check('the outer page loads the module',
    outer.indexOf('/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"') !== -1, true);
check('the full screen code exists before the game frame starts loading',
    outer.indexOf('window.WebclientFrame') !== -1 &&
    outer.indexOf('window.WebclientFrame') < outer.indexOf('<iframe'), true);
check('the frame height follows the visual viewport',
    outer.indexOf('ViewportFit.frameHeight(') !== -1, true);
check('the header link uses the same full screen code',
    header.indexOf('WebclientFrame.setFullScreen(true)') !== -1, true);
check('the header script checks the link exists', /if \(!link\) return;/.test(header), true);
```

- [ ] **Step 2: Run it and see it fail**

Run: `node tools/jstest/viewportfit.test.js`
Expected: FAIL on those five.

- [ ] **Step 3: Replace `webclient.html` in full**

```html
{{template "header" .}}

<script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"></script>
<script type="text/javascript">

    function resizeClientIFrame() {
        const contentContainer = document.getElementById('content-container');
        contentContainer.style.padding = "0px";

        wcIframe = document.getElementById('webclient-frame');
        // For some reason safari acts weird if we don't reset this first.
        wcIframe.width = "0px"; 
        wcIframe.height = "0px";
        
        // Calculate scrollbar width
        const outer = document.createElement('div');
        outer.style.cssText = 'visibility:hidden;overflow:scroll';
        document.body.appendChild(outer);
        outer.appendChild(document.createElement('div'));
        const scrollbarWidth = outer.offsetWidth - outer.firstChild.offsetWidth;
        document.body.removeChild(outer);
        
        // Set proper height/width - subtract scrollbar width to prevent cropping
        wcIframe.width = contentContainer.clientWidth - scrollbarWidth;
        // The on-screen keyboard shrinks only the visual viewport on iOS Safari
        // and current Chrome on Android, so 100dvh does not follow it. Fit the
        // frame above the keyboard, which puts the command box right on top of it.
        const vv = window.visualViewport;
        wcIframe.height = ViewportFit.frameHeight(
            contentContainer.clientHeight,
            vv ? vv.height : undefined,
            contentContainer.getBoundingClientRect().top);
    }

    // Full screen hides the site around the frame and leaves the frame alone,
    // so the game stays connected. The session bar's button and the header's
    // link both come through here.
    window.WebclientFrame = (function () {
        const KEY = 'dogmud.fullscreen';
        let on = false;
        let listener = null;

        function readPref() {
            try { return window.localStorage.getItem(KEY); } catch (e) { return null; }
        }
        function writePref(value) {
            try { window.localStorage.setItem(KEY, value); } catch (e) {}
        }
        function apply(next) {
            on = next;
            const display = on ? 'none' : '';
            ['header', 'footer'].forEach(function (tag) {
                const el = document.getElementsByTagName(tag)[0];
                if (el) el.style.display = display;
            });
            resizeClientIFrame();
            if (listener) listener(on);
        }
        return {
            start: function () {
                apply(ViewportFit.wantFullScreen(readPref(),
                    ViewportFit.isPhone(window.screen.width, window.screen.height)));
            },
            setFullScreen: function (next) { writePref(next ? '1' : '0'); apply(next); },
            isFullScreen: function () { return on; },
            onChange: function (fn) { listener = fn; fn(on); }
        };
    })();

    // Size the frame as soon as this page's layout exists; "load" alone waits
    // for everything inside the frame, which showed it at the browser's default
    // 300x150 until then.
    document.addEventListener("DOMContentLoaded", function () { WebclientFrame.start(); });
    window.addEventListener("load", resizeClientIFrame);
    window.addEventListener('resize', resizeClientIFrame);
    // Phones: the visible area changes when the address bar hides or the
    // on-screen keyboard opens. iOS also scrolls the page to show the focused
    // box; the frame already fits, so hold the page at the top.
    if (window.visualViewport) {
        window.visualViewport.addEventListener('resize', function () {
            window.scrollTo(0, 0);
            resizeClientIFrame();
        });
        window.visualViewport.addEventListener('scroll', function () { window.scrollTo(0, 0); });
    }
    window.addEventListener('beforeunload', function(event) {
        event.preventDefault();
        event.returnValue = 'Leave/Disconnect from the MUD?';
    });
</script>
<iframe id="webclient-frame" border="0" src="/webclient-pure.html"></iframe>
{{template "footer" .}}
```

- [ ] **Step 4: Replace the `_header.html` script (lines 16-32)**

```html
  <script type="text/javascript">
    window.addEventListener("load", function(){
      const link = document.getElementById('hide-header-footer');
      if (!link) return;

      link.addEventListener('click', function(event) {
        // Stay on the page: following the link would drop the game connection.
        event.preventDefault();
        window.WebclientFrame.setFullScreen(true);
      });
    });
  </script>
```

- [ ] **Step 5: Run the tests**

Run: `node tools/jstest/viewportfit.test.js && go test -count=1 ./internal/web/`
Expected: `all checks passed`, then `ok`

- [ ] **Step 6: Commit**

```bash
git add _datafiles/html/public/webclient.html _datafiles/html/public/_header.html tools/jstest/viewportfit.test.js
git commit -F - <<'EOF'
feat(webclient): full screen on phones and the frame fits above the keyboard

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 4: Game page: phone class, toggle, top-level fit

**Files:**
- Modify: `_datafiles/html/public/webclient-pure.html` (head, session bar, `rootStyle`, end of the main script)
- Modify: `tools/jstest/viewportfit.test.js`

- [ ] **Step 1: Add the failing wiring checks**

In `viewportfit.test.js`, before the final `console.log`, add:

```js
var game = fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8');
check('the game page has a viewport tag',
    game.indexOf('<meta name="viewport" content="width=device-width, initial-scale=1.0"') !== -1, true);
check('the game page loads the module',
    game.indexOf('/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"') !== -1, true);
check('the phone class is set before the terminal reads its colours',
    game.indexOf("classList.add('phone')") !== -1 &&
    game.indexOf("classList.add('phone')") < game.indexOf('const rootStyle ='), true);
check('the session bar has the full screen toggle', game.indexOf('id="btn-fullscreen"') !== -1, true);
check('the toggle talks to the outer page', game.indexOf('window.parent.WebclientFrame') !== -1, true);
```

- [ ] **Step 2: Run it and see it fail**

Run: `node tools/jstest/viewportfit.test.js`
Expected: FAIL on those five.

- [ ] **Step 3: Head**

After `<title>...</title>` at the top of `webclient-pure.html`, add:

```html
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
```

After the `motdrestyle.js` script tag, add:

```html
    <script src="{{ .CONFIG.FilePaths.WebCDNLocation }}/static/js/viewportfit.js?v={{ .ASSET_VERSION }}"></script>
```

- [ ] **Step 4: Phone class**

Directly above `const rootStyle = getComputedStyle(document.documentElement);`, add:

```js
        // Phones keep the bright dim grey (gomud.css :root.phone).
        if (window.ViewportFit.isPhone(window.screen.width, window.screen.height)) {
            document.documentElement.classList.add('phone');
        }
```

- [ ] **Step 5: Session bar button**

Directly after the `menu-icon` button line in `#dash-session`, add:

```html
        <button type="button" class="brass" id="btn-fullscreen" style="display:none">⛶</button>
```

- [ ] **Step 6: Toggle and top-level fit**

Directly above `// Session bar: one button that reads Leave Game` add:

```js
        // Full screen lives on the outer page (webclient.html). Opened on its
        // own, this page has no site around it, so the button stays hidden.
        (function () {
            var outer = window.parent !== window ? window.parent.WebclientFrame : null;
            var button = document.getElementById('btn-fullscreen');
            if (outer) {
                button.style.display = '';
                outer.onChange(function (on) {
                    var label = on ? 'Show the site header and menu' : 'Full screen';
                    button.title = label;
                    button.setAttribute('aria-label', label);
                });
                button.addEventListener('click', function () {
                    outer.setFullScreen(!outer.isFullScreen());
                });
                return;
            }
            // On its own, this page fits itself above the on-screen keyboard.
            var vv = window.visualViewport;
            if (!vv) return;
            vv.addEventListener('resize', function () {
                window.scrollTo(0, 0);
                document.body.style.height = Math.floor(vv.height) + 'px';
            });
        })();
```

- [ ] **Step 7: Run all client tests**

Run: `for t in tools/jstest/*.test.js; do node "$t" > /dev/null || echo "FAILED $t"; done; for t in tools/webclient-tests/*.js; do node "$t" > /dev/null || echo "FAILED $t"; done; go test -count=1 ./internal/web/`
Expected: no `FAILED` lines, then `ok`

- [ ] **Step 8: Commit**

```bash
git add _datafiles/html/public/webclient-pure.html tools/jstest/viewportfit.test.js
git commit -F - <<'EOF'
feat(webclient): phone grey, full screen toggle, keyboard fit on the bare page

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 5: Docs and live check

**Files:**
- Modify: `docs/PATCH_NOTES.md`, `docs/README.md`
- Scratch only: a headless Edge script in the session scratchpad

- [ ] **Step 1: Patch notes**

Add at the top of `docs/PATCH_NOTES.md`, under the title:

```markdown
## 2026-11-03: Playing on phones

- On a phone, the web client now opens full screen. The ⛶ button in the session
  bar brings the site header and menu back, and it remembers your choice.
- When the on-screen keyboard opens, the game text and command box shrink to fit
  above it.
- On tablets and larger screens, dim text is dim again, so hints and dividers sit
  back behind the game text. Phones keep it bright.
- The Temple marker on the map is visible again.
- Brass buttons and inventory tabs are easier to read.
```

- [ ] **Step 2: README**

In `docs/README.md`, in the `tools/jstest` row, append before the closing ` |`:

```markdown
. Added 2026-11-03 (phone fit): `viewportfit.test.js` (covers [`../_datafiles/html/public/static/js/viewportfit.js`](../_datafiles/html/public/static/js/viewportfit.js): what counts as a phone, the full screen default, the game frame's height above the on-screen keyboard), `webclient-colours.test.js` (the dim grey, brass and Temple marker colour rulings)
```

- [ ] **Step 3: Live check on a private server**

Boot from a detached worktree on private ports (`dogmud-shipping`), never the owner's server. Drive `/webclient`, not `/webclient-pure`, in headless Edge over DevTools, with `Emulation.setDeviceMetricsOverride` `{width: 360, height: 740, screenWidth: 360, screenHeight: 740, deviceScaleFactor: 3, mobile: true}`, then again at 1280x800 without emulation. Record each of these and take a screenshot of each state:

| Check | Phone, expected | 1280x800, expected |
|---|---|---|
| `getComputedStyle(document.querySelector('header')).display` | `none` | not `none` |
| Frame document: `documentElement.classList.contains('phone')` | `true` | `false` |
| Frame document: `term.options.theme.brightBlack` or `--term-bright-black` | `#e0d2b2` | `#9a8a74` |
| ⛶ clicked: header display, then `socket.readyState` | shown, `1` | hidden, `1` |
| Reload after ⛶: full screen state | remembered | remembered |
| Set the viewport height to 400 by override, run the page's `resizeClientIFrame()` | frame 400 tall, command box bottom edge within 400 | |
| `socket` after all of the above; buffer length before and after the height change | same socket, no lines lost | same socket |
| Temple marker: `help map-examples` or a map with a temple, screenshot | dark mark on a pale block | dark mark on a pale block |

Stop the server by PID. Headless Edge has no on-screen keyboard, so this proves the height logic and not a real keyboard.

- [ ] **Step 4: Gates**

Run: `gofmt -l internal/ modules/ main.go; go build ./...; ~/go/bin/golangci-lint run --new-from-merge-base=origin/master; go test -count=1 ./internal/web/ ./internal/guards/ .`
Expected: `gofmt` prints nothing, `0 issues.`, all `ok`

- [ ] **Step 5: Commit, PR, merge when green**

```bash
git add docs/PATCH_NOTES.md docs/README.md
git commit -F - <<'EOF'
docs: phone fit patch notes and test index

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git push -u origin feat/webclient-phone-fit
gh pr create --repo pruuk/DOGMud --base master --head feat/webclient-phone-fit --title "feat(webclient): phone fit, dim grey split, readable brass and Temple marker" --body-file <scratchpad>/pr-body.md
gh pr checks <n> --repo pruuk/DOGMud --watch
gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch
```

The PR body lists the owner's rulings, the live check table, and "Needs a real device: Malia on her phone and tablet, keyboard open, before the deploy." It ends with the Claude Code attribution line.

- [ ] **Step 6: Ask Malia to test before the deploy**

Give the owner this list for Malia: on her phone and on her tablet, open the web client, open the keyboard, and check the command box sits right above it and the newest text is visible; rotate; use ⛶ both ways; check the game stays connected. The list also covers the iPhone question from report section 4, whether the inventory and automation menus open at all.

## Risks only a real device can show

- Some keyboards change height as suggestions appear. The `resize` handler follows them, but a jump is possible.
- iOS may still nudge the page on focus. `scrollTo(0, 0)` should hold it; if it fights the browser, that handler is the first thing to change.
- Resetting the frame to 0 on each resize is old behaviour for Safari. During a keyboard animation it runs many times; if it flickers, skip the reset when the size is unchanged.
- The dashboard's phone layout is what this plan fits above the keyboard, as it stands. Rearranging it belongs to the touch arc.

## Corrections made while building

- **The dashboard does have a phone layout.** `dashboard.css` has no media queries, but `dashboard.js` sets `data-mode="phone"` under 700 px, with a bottom tab bar. The tab bar sat between the command box and the keyboard, so it now steps aside while the keyboard is open (`html.keyboard-open`).
- **Zoom:** pinch zoom also shrinks the visual viewport, so `ViewportFit.visibleHeight` ignores it. Otherwise zooming to read would squash the game.
- **Keyboard path:** it changes only the frame's height. The old reset to 0 could close the keyboard on iOS by taking the focused box out of view.
- **Framing:** the toggle code tolerates another site framing the page, where reading the parent throws.
- **Font:** a height-only refit keeps the font, which also moved by half a pixel when the keyboard opened.
- **⛶ button:** it uses the sound button's round shape. The `brass` class squashed it to a sliver in the session bar.
