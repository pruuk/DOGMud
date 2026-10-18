// webclient-vitals-self.test.js: the viewer appears exactly once in the web
// client's Vitals window, by name, whether or not they are in a party.
//
// Run:  node tools/jstest/webclient-vitals-self.test.js
//
// Playtest 2026-10-17: in a party, Malia's own character showed twice, once
// as "You" at the top and once under her name at the bottom. The panel read
// the viewer's name from GMCPStructs.Char.Name, which the server never sends
// (the name lives at Char.Info.name), so it always fell back to "You" and
// never recognised the viewer's own Party.Vitals row. The duplicate shows
// whenever Party.Vitals has arrived without a Party.Members list, which is
// what a reconnect inside the zombie window used to leave behind.
//
// Parses the real functions out of webclient-pure.html rather than copying
// them, so it cannot drift from the shipping page. Dependency-free and exits
// non-zero on failure, like the other tests in this folder.

'use strict';

var fs = require('fs');
var path = require('path');

var HTML = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html');
var html = fs.readFileSync(HTML, 'utf8');

function extract(startMarker, endMarker) {
    var s = html.indexOf(startMarker);
    if (s < 0) throw new Error('marker not found in webclient-pure.html: ' + startMarker);
    var e = html.indexOf(endMarker, s + 1);
    if (e < 0) throw new Error('end marker not found in webclient-pure.html: ' + endMarker);
    return html.slice(s, e);
}

var code =
    extract('function availablePct(', 'function evalTriggerCondition(') + '\n' +
    extract('function buildVitalsSeg(', 'function handleGMCP(');

// --- DOM stub --------------------------------------------------------------
function El(tag) {
    this.tag = tag; this.className = ''; this.dataset = {}; this.children = [];
    this.textContent = ''; this.style = {}; this.parent = null;
}
El.prototype.appendChild = function (c) { c.parent = this; this.children.push(c); return c; };
El.prototype.insertBefore = function (n, ref) {
    var cur = this.children.indexOf(n); if (cur >= 0) this.children.splice(cur, 1);
    var i = this.children.indexOf(ref); if (i < 0) i = this.children.length;
    this.children.splice(i, 0, n); n.parent = this; return n;
};
El.prototype.remove = function () {
    if (!this.parent) return;
    var i = this.parent.children.indexOf(this);
    if (i >= 0) this.parent.children.splice(i, 1);
};
El.prototype._all = function (out) {
    out = out || [];
    this.children.forEach(function (c) { out.push(c); c._all(out); });
    return out;
};
function hasClass(el, c) { return (' ' + el.className + ' ').indexOf(' ' + c + ' ') >= 0; }
El.prototype.querySelectorAll = function (sel) {
    var m = sel.match(/^\.([a-z-]+)(?:\[data-name="(.*)"\])?$/);
    if (!m) return [];
    return this._all().filter(function (e) {
        if (!hasClass(e, m[1])) return false;
        if (m[2] === undefined) return true;
        return e.dataset.name === m[2].split('\\"').join('"').split('\\\\').join('\\');
    });
};
El.prototype.querySelector = function (sel) { return this.querySelectorAll(sel)[0] || null; };

var container = new El('div');
var sessName = new El('span');
global.document = {
    createElement: function (t) { return new El(t); },
    getElementById: function (id) {
        if (id === 'vitals-bars') return container;
        if (id === 'sess-name') return sessName;
        return null;
    }
};
global.GMCPStructs = {};
global.window = {};
(0, eval)(code); // indirect eval: the function declarations become globals

// --- helpers ---------------------------------------------------------------
var failures = 0, checks = 0;
function check(name, ok, detail) {
    checks++;
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}
function render() {
    updateVitalsWindow(); // eslint-disable-line no-undef
    return container.querySelectorAll('.vitals-row').map(function (r) {
        var n = r.querySelector('.vitals-name');
        return { key: r.dataset.name, label: n ? n.textContent : '', self: !!(n && hasClass(n, 'self')) };
    });
}
function viewer() {
    container.children = [];
    // The Char package exactly as the server shapes it: no top-level Name.
    GMCPStructs.Char = {
        Info: { account: 'malia', name: 'Malia', race: 'Human', 'class': 'Warrior' },
        Vitals: { hp: 80, hp_max: 100, stamina: 40, stamina_max: 50,
                  conviction: 10, conviction_max: 10,
                  hp_reserved: 0, stamina_reserved: 0, conviction_reserved: 0 }
    };
    GMCPStructs.Room = { Info: { name: 'A Room' } };
}
function member(name, pct) {
    return { name: name, health: pct, stamina: pct, conviction: pct, location: 'A Room' };
}
var show = function (rows) { return JSON.stringify(rows); };

// --- scenarios -------------------------------------------------------------

// 1. In a party with the full Party package: the viewer is one row, named,
//    and marked as self (so it is drawn from exact Char.Vitals data).
viewer();
GMCPStructs.Party = {
    Leader: 'Calabe',
    Members: [{ name: 'Calabe' }, { name: 'Malia' }],
    Vitals: { Calabe: member('Calabe', 90), Malia: member('Malia', 80) }
};
var r1 = render();
check('party: two rows', r1.length === 2, show(r1));
check('party: no "You" row', !r1.some(function (r) { return r.label === 'You'; }), show(r1));
check('party: the viewer is one row', r1.filter(function (r) { return r.label === 'Malia'; }).length === 1, show(r1));
check('party: the viewer row is marked self', r1.some(function (r) { return r.label === 'Malia' && r.self; }), show(r1));
check('party: session strip shows the name', sessName.textContent === 'Malia', 'got ' + sessName.textContent);

// 2. Party.Vitals with no Members list (what a reconnect left behind): the
//    panel falls back to the solo layout, and the viewer must still appear once.
viewer();
GMCPStructs.Party = { Vitals: { Calabe: member('Calabe', 90), Malia: member('Malia', 80) } };
var r2 = render();
check('vitals only: the viewer is one row', r2.filter(function (r) {
    return r.label === 'Malia' || r.label === 'You';
}).length === 1, show(r2));
check('vitals only: the viewer row is named, not "You"', r2.length > 0 && r2[0].label === 'Malia' && r2[0].self, show(r2));
check('vitals only: the other member still shows', r2.some(function (r) { return r.label === 'Calabe'; }), show(r2));

// 3. Solo with no Party at all: one self row, named.
viewer();
GMCPStructs.Party = undefined;
var r3 = render();
check('solo: one self row, named', r3.length === 1 && r3[0].label === 'Malia' && r3[0].self, show(r3));

// 4. Going from the stale solo layout to the full Party leaves no duplicate.
viewer();
GMCPStructs.Party = { Vitals: { Calabe: member('Calabe', 90), Malia: member('Malia', 80) } };
render();
GMCPStructs.Party = {
    Leader: 'Calabe',
    Members: [{ name: 'Calabe' }, { name: 'Malia' }],
    Vitals: { Calabe: member('Calabe', 90), Malia: member('Malia', 80) }
};
var r4 = render();
check('transition: still two rows, leader first', r4.length === 2 && r4[0].label === 'Calabe', show(r4));

console.log((failures ? failures + ' of ' : 'all ') + checks + ' checks ' + (failures ? 'FAILED' : 'passed'));
process.exit(failures ? 1 : 0);
