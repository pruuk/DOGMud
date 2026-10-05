#!/usr/bin/env node
/*
 * Guards the Game-window sight border (lighting plan 5d, Char.Sight).
 *
 *   node tools/webclient-tests/sight-border.js
 *
 * Zero dependencies. Static analysis of webclient-pure.html and
 * dashboard.css; it does not execute the UI, so it needs no DOM.
 *
 * The server pushes Char.Sight {"band": "dark"|"shapes"|"faces"|"dazzled"}
 * when a player's light band changes, and inside every full Char payload.
 * The client must:
 *   1. handle "Char.Sight" and set one sight-<band> class on #panel-feed,
 *      with the tooltip the spec names for each band;
 *   2. delegate to it from the generic "Char" handler, which is the only
 *      handler a full Char push reaches (see char-handler-shadowing.js);
 *   3. style all four classes in dashboard.css, on #panel-feed itself, the
 *      element a pop-out moves into its window.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
const html = fs.readFileSync(path.join(root, 'webclient-pure.html'), 'utf8');
const css = fs.readFileSync(path.join(root, 'static', 'css', 'dashboard.css'), 'utf8');

const bands = {
  dark: 'Too dark to see.',
  shapes: 'Dim light: shapes, not faces.',
  faces: 'Good light.',
  dazzled: 'Too bright: the glare hurts.',
};

function handlerBody(src, key) {
  const start = src.indexOf('"' + key + '": function()');
  if (start < 0) return null;
  const open = src.indexOf('{', start);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  return null;
}

let fails = 0;
function check(ok, what) {
  console.log((ok ? '  OK        ' : '  FAIL      ') + what);
  if (!ok) fails++;
}

const sight = handlerBody(html, 'Char.Sight');
check(sight !== null, 'a "Char.Sight" handler exists');
if (sight) {
  check(sight.includes('panel-feed'), 'it targets #panel-feed');
  check(sight.includes('"sight-" + band') || sight.includes("'sight-' + band"), 'it sets a sight-<band> class');
  for (const [band, tip] of Object.entries(bands)) {
    check(sight.includes(tip), 'the ' + band + ' tooltip reads "' + tip + '"');
  }
}

const charStart = html.indexOf('"Char":function()');
const charBody = charStart < 0 ? '' : html.slice(charStart, html.indexOf('"Char.Conditions": function()', charStart));
check(charBody.includes("GMCPUpdateHandlers['Char.Sight']"), 'the generic "Char" handler delegates to Char.Sight');

for (const band of Object.keys(bands)) {
  check(new RegExp('#panel-feed\\.sight-' + band + '\\s*\\{[^}]*border-color').test(css),
    'dashboard.css colours #panel-feed.sight-' + band);
}

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails ? 1 : 0);
