// webclient-html-escape.test.js: the web client builds two panels as HTML
// strings and assigns them with innerHTML: the Quests panel (renderQuests,
// written by writeQuestsHTML) and the Status panel (updateStatusPanel). Every
// value either comes from the server's GMCP (quest names and hints, condition
// names and descriptions, the character's name, race and class) or is a
// number or a constant. These tests pull the real functions out of
// webclient-pure.html, feed them hostile GMCP, and check nothing of it
// survives as markup.
//
// Run:  node tools/jstest/webclient-html-escape.test.js
//
// Dependency-free and assertion-library-free, matching hotinput.test.js.
// Exits non-zero on failure.

var path = require('path');
var fs = require('fs');

var PAGE = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'
), 'utf8');

var failures = 0;
var checks = 0;

function check(name, got, want) {
    checks++;
    if (got !== want) {
        console.log('FAIL ' + name + '\n  got:  ' + String(got).slice(0, 400) + '\n  want: ' + String(want).slice(0, 400));
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

// funcSrc returns the source of `function name(` up to its matching brace.
// The functions extracted here hold no brace inside a string or regex.
function funcSrc(name) {
    var start = PAGE.indexOf('function ' + name + '(');
    if (start < 0) { throw new Error('not found in the page: function ' + name); }
    var depth = 0;
    for (var i = PAGE.indexOf('{', start); i < PAGE.length; i++) {
        if (PAGE[i] === '{') { depth++; }
        else if (PAGE[i] === '}' && --depth === 0) { return PAGE.slice(start, i + 1); }
    }
    throw new Error('unbalanced: function ' + name);
}

// A minimal DOM: elements by id that record what innerHTML was given.
function makeEnv(gmcp) {
    var els = {};
    var document = {
        getElementById: function (id) {
            if (!els[id]) {
                els[id] = { id: id, innerHTML: '', querySelectorAll: function () { return []; } };
            }
            return els[id];
        }
    };
    var factory = new Function('document', 'window', 'GMCPStructs', 'gr', 'SendGMCP', [
        funcSrc('escapeHTML'),
        funcSrc('writeQuestsHTML'),
        funcSrc('renderQuests'),
        funcSrc('charDisplayName'),
        funcSrc('updateStatusPanel'),
        'return { escapeHTML: escapeHTML, renderQuests: renderQuests, updateStatusPanel: updateStatusPanel };'
    ].join('\n'));
    var fns = factory(document, {}, gmcp, { rooms: new Map() }, function () {});
    return { fns: fns, html: function (id) { return document.getElementById(id).innerHTML; } };
}

// Payloads for every context the panels use: element text, and a
// double-quoted attribute (a condition's title). A value that broke out of
// either would add an element or an attribute.
var TAG = '<img src=x onerror=alert(1)>';
var ATTR = '" onmouseover="alert(1)" x="';
var BOTH = TAG + ATTR + "'&";

function noMarkupFrom(html) {
    return html.indexOf('<img') === -1 && html.indexOf('" onmouseover=') === -1;
}

// --- escapeHTML -----------------------------------------------------------------

(function () {
    var e = makeEnv({}).fns.escapeHTML;
    check('escapeHTML neutralizes a tag', e(TAG), '&lt;img src=x onerror=alert(1)&gt;');
    check('escapeHTML escapes both quote styles and ampersands', e('a"b\'c&d'), 'a&quot;b&#39;c&amp;d');
    check('escapeHTML renders a non-string as empty', e({ toString: function () { return TAG; } }), '');
    check('escapeHTML renders undefined as empty', e(undefined), '');
}());

// --- the Quests panel (webclient-pure.html renderQuests) --------------------------

(function () {
    var env = makeEnv({
        Char: {
            Quests: [
                { id: '1" onclick="alert(1)', name: BOTH, completion: '50%"><img src=x>', focused: true, hint: BOTH, next_dir: BOTH },
                { id: 2, name: BOTH, completion: 10, focused: false, hint: BOTH }
            ]
        }
    });
    env.fns.renderQuests();
    var html = env.html('quests-list');
    check('the Quests panel was rendered', html.indexOf('qrow') !== -1, true);
    check('no quest field becomes markup', noMarkupFrom(html), true);
    check('a quest name is shown escaped', html.indexOf('&lt;img src=x onerror=alert(1)&gt;&quot; onmouseover=&quot;') !== -1, true);
    check('a non-number quest id cannot leave its attribute', html.indexOf('onclick') === -1, true);
    check('a non-number completion cannot leave its style', html.indexOf('50%"') === -1, true);
}());

// A hint over the 200-character cap renders the clamp and its "more" toggle
// (#375 P8); both carry the quest id and the hint text, so both stay escaped.
(function () {
    var env = makeEnv({
        Char: {
            Quests: [
                { id: '9" onclick="alert(1)', name: 'n', completion: 1, focused: true,
                  hint: new Array(12).join(BOTH), next_dir: BOTH }
            ]
        }
    });
    env.fns.renderQuests();
    var html = env.html('quests-list');
    check('a long hint renders its toggle', html.indexOf('qhint-more') !== -1, true);
    check('no long-hint field becomes markup', noMarkupFrom(html), true);
    check('a non-number quest id cannot leave the toggle attribute', html.indexOf('onclick') === -1, true);
}());

// --- the Status panel (webclient-pure.html updateStatusPanel) ---------------------

(function () {
    var cond = {};
    cond[BOTH] = { name: BOTH, description: BOTH, duration: BOTH };
    cond['plain'] = { description: ATTR };
    var env = makeEnv({
        Char: {
            Name: BOTH,
            // The name the panel shows comes from Info.name (charDisplayName).
            Info: { name: BOTH, race: BOTH, 'class': BOTH },
            Conditions: cond
        }
    });
    env.fns.updateStatusPanel();
    var html = env.html('status-conditions');
    check('the Status panel was rendered', html.indexOf('cond-chip') !== -1, true);
    check('no status field becomes markup', noMarkupFrom(html), true);
    check('a condition description stays inside its title attribute',
        html.indexOf('title="&lt;img src=x onerror=alert(1)&gt;&quot; onmouseover=&quot;alert(1)&quot; x=&quot;&#39;&amp;"') !== -1, true);
    check('a condition named only by its key is escaped', html.indexOf('>plain<') !== -1, true);
}());

console.log('\n' + (checks - failures) + '/' + checks + ' checks passed');
if (failures > 0) { process.exit(1); }
