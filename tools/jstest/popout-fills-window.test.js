// popout-fills-window.test.js: a popped-out panel fills its window (#942,
// group playtest 2026-11-21: "a small Comms panel shows a channel's oldest
// message, not its newest").
//
// Run:  node tools/jstest/popout-fills-window.test.js
//
// The bug: Dashboard.popout mounts the panel itself in a WinBox. The
// window's body clips what it holds (webclient-pure.html, .wb-body
// overflow: hidden) and gives it no height, so the panel grew as tall as
// everything in it. A Comms channel window never became shorter than its
// messages, so it could not scroll: a small Comms window showed the oldest
// lines and cut off the newest. The Game window hid its newest lines and the
// Map its buttons the same way. Measured in headless Chrome 2026-11-25: a
// 300px Comms window held a 1414px panel; with the rule, 263px, and the
// channel scrolls to its newest line.
//
// This checks the stylesheet, the page and dashboard.js still line up:
// the panel is what the WinBox mounts, the window body clips, and a
// top-level dashboard.css rule (not one inside a media query) gives every
// popped-out panel the window's full height. Dependency free, exits
// non-zero on failure.
'use strict';

var fs = require('fs');
var path = require('path');

var PUBLIC = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
var css = fs.readFileSync(path.join(PUBLIC, 'static', 'css', 'dashboard.css'), 'utf8');
var page = fs.readFileSync(path.join(PUBLIC, 'webclient-pure.html'), 'utf8');
var dash = fs.readFileSync(path.join(PUBLIC, 'static', 'js', 'dashboard.js'), 'utf8');

var failures = 0, checks = 0;
function check(name, ok, detail) {
    checks++;
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + (detail ? '\n  ' + detail : ''));
}

// Top-level rules only: { selectors: [...], body: '...' }. A rule inside an
// @media block would not reach every popped-out window.
function topLevelRules(text) {
    text = text.replace(/\/\*[\s\S]*?\*\//g, '');
    var rules = [], depth = 0, start = 0, sel = '';
    for (var i = 0; i < text.length; i++) {
        var c = text[i];
        if (c === '{') {
            if (depth === 0) { sel = text.slice(start, i).trim(); start = i + 1; }
            depth++;
        } else if (c === '}') {
            depth--;
            if (depth === 0) {
                if (sel.charAt(0) !== '@') {
                    rules.push({
                        selectors: sel.split(',').map(function (s) { return s.trim().replace(/\s+/g, ' '); }),
                        body: text.slice(start, i)
                    });
                }
                start = i + 1;
            }
        }
    }
    return rules;
}

// --- the WinBox mounts the panel itself, with the dash-winbox class ---------
var popout = (/popout: function \(name\) \{[\s\S]*?new WinBox\(opts\)/.exec(dash) || [''])[0];
check('dashboard.js popout mounts the panel in the WinBox', /mount:\s*panel\b/.test(popout));
check('dashboard.js popout gives the WinBox the dash-winbox class', /class:\s*"dash-winbox"/.test(popout));

// --- the window body clips, so the panel must fit it --------------------------
check('the page clips the WinBox body (.wb-body overflow: hidden)',
      /\.wb-body\s*\{[^}]*overflow\s*:\s*hidden/.test(page));

// --- a popped-out panel takes the window's full height ------------------------
var fills = topLevelRules(css).filter(function (r) {
    return r.selectors.indexOf('.winbox.dash-winbox .dash-panel') >= 0 &&
           /(^|[;\s])height\s*:\s*100%/.test(r.body);
});
check('dashboard.css gives .winbox.dash-winbox .dash-panel height: 100% at top level',
      fills.length === 1, 'found ' + fills.length + ' such rule(s)');

// The fill depends on the panel being a flex column whose body shrinks and
// scrolls; check those rules are still there.
var panelRule = topLevelRules(css).filter(function (r) { return r.selectors.indexOf('.dash-panel') >= 0; })[0];
check('.dash-panel is a flex column',
      !!panelRule && /display\s*:\s*flex/.test(panelRule.body) && /flex-direction\s*:\s*column/.test(panelRule.body));
var bodyRule = topLevelRules(css).filter(function (r) { return r.selectors.indexOf('.dash-panel-body') >= 0; })[0];
check('.dash-panel-body may shrink (min-height: 0)', !!bodyRule && /min-height\s*:\s*0/.test(bodyRule.body));

console.log('\n' + (checks - failures) + ' of ' + checks + ' checks passed');
process.exit(failures ? 1 : 0);
