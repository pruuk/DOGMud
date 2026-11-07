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

// The Chat, Inventory and Automation tabs share one style, .subtab (PR #768,
// which replaced .inv-tab.brass). Each tab's label must pass 4.5:1 on the
// middle of its own brass, read from the rules themselves: the inactive tab's
// text on the shiny brass, and the open (.subtab.active) tab's cream text on
// the pressed-in darker brass (#768 review item 15).
function rule(sel) {
    var m = new RegExp('^' + sel.replace(/\./g, '\\.') + ' \\{[^}]*\\}', 'm').exec(dash);
    return m ? m[0] : '';
}
// A colour in a rule: a #rrggbb, or a var(--token) looked up in gomud.css.
function colourOf(value) {
    if (!value) return null;
    if (/^#[0-9a-fA-F]{6}$/.test(value)) return value;
    var v = /^var\(--([\w-]+)\)$/.exec(value);
    try { return v ? token(css, v[1]) : null; } catch (e) { return null; }
}
function textColour(body) { var m = /(?:^|[;{\s])color:\s*([^;]+);/.exec(body); return m ? colourOf(m[1].trim()) : null; }
// The middle stop of the brass gradient, where the label sits.
function brassMiddle(body) { var m = /background:[^;]*?,\s*(var\(--[\w-]+\)|#[0-9a-fA-F]{6}) 46%/.exec(body); return m ? colourOf(m[1]) : null; }

var tab = rule('.subtab'), open = rule('.subtab.active');
check('the shared tab style exists', tab !== '', 'no .subtab rule at the start of a line in dashboard.css');
check('the open tab style exists', open !== '', 'no .subtab.active rule at the start of a line in dashboard.css');
var tabText = textColour(tab), tabBrass = brassMiddle(tab);
check('an inactive tab names its text and brass colours', !!(tabText && tabBrass), 'text ' + tabText + ', brass ' + tabBrass);
check('an inactive tab label passes 4.5:1 on its brass',
    !!(tabText && tabBrass) && contrast(tabText, tabBrass) >= 4.5,
    tabText + ' on ' + tabBrass + ' = ' + (tabText && tabBrass ? contrast(tabText, tabBrass).toFixed(2) : 'n/a'));
var openText = textColour(open), openBrass = brassMiddle(open);
check('the open tab names its text and brass colours', !!(openText && openBrass), 'text ' + openText + ', brass ' + openBrass);
check('the open tab label passes 4.5:1 on its darker brass',
    !!(openText && openBrass) && contrast(openText, openBrass) >= 4.5,
    openText + ' on ' + openBrass + ' = ' + (openText && openBrass ? contrast(openText, openBrass).toFixed(2) : 'n/a'));
// An inactive tab is not dimmed at all; if a brightness filter ever comes
// back it must stay readable.
var dimmed = /brightness\(([0-9.]+)\)/.exec(tab);
check('an inactive tab is not dimmed', !dimmed || Number(dimmed[1]) >= 0.9, tab);
check('an inactive tab is not see-through', tab !== '' && !/opacity:\s*0\./.test(tab), tab);

check('the Temple marker has a dark colour on its pale block', /^\s*map-temple:\s*0\b/m.test(aliases), 'map-temple');
check('the Temple block stays pale', /^\s*mapbg-temple:\s*15\b/m.test(aliases), 'mapbg-temple');

// A marker colour is only readable on its own block, so every place that draws
// a map marker from its slug passes the block too, as the minimap does.
var title = read('world', 'dogmud', 'templates', 'descriptions', 'room-title.template');
check('the room title draws a marker on its block',
    title.indexOf('<ansi fg="map-%s" bg="mapbg-%s">%s</ansi>') !== -1, 'room-title.template');
var roominfo = read('world', 'dogmud', 'templates', 'admincommands', 'ingame', 'roominfo.template');
check('the admin room info draws a marker on its block',
    /fg="map-[^"]*" bg="mapbg-/.test(roominfo) && !/fg="map-\{\{[^"]*"\s*>/.test(roominfo), 'roominfo.template');

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
