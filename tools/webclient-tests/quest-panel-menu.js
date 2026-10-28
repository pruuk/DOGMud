#!/usr/bin/env node
/*
 * EXECUTES the real quest-panel right-click menu (Focus / Abandon, #375 p5).
 *
 *   node tools/webclient-tests/quest-panel-menu.js
 *
 * Zero dependencies. Renders quests through the page's real renderQuests,
 * builds fake row elements from the rendered markup, then runs the page's
 * real quest context-menu IIFE (cut out between its marker comments) against
 * a stub DOM and records every SendGMCP call.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const HTML = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html');
const html = fs.readFileSync(HTML, 'utf8');

function sliceBalanced(src, startIdx, open, close) {
  const s = src.indexOf(open, startIdx);
  let depth = 0;
  for (let i = s; i < src.length; i++) {
    if (src[i] === open) depth++;
    else if (src[i] === close) {
      depth--;
      if (depth === 0) return src.slice(s, i + 1);
    }
  }
  throw new Error('unbalanced ' + open + close);
}
function funcSrc(src, signature) {
  const i = src.indexOf(signature);
  if (i < 0) throw new Error('not found: ' + signature);
  return src.slice(i, i + sliceBalanced(src, i, '{', '}').length + (src.indexOf('{', i) - i));
}

const START = 'Quest right-click context menu';
const END = 'End quest context menu';
const si = html.indexOf(START);
const ei = html.indexOf(END);
const menuSrc = si >= 0 && ei > si ? html.slice(html.indexOf('(function', si), ei) : '';

// ---------------------------------------------------------------- DOM stub
function El(tag) {
  this.tag = tag; this.className = ''; this.dataset = {}; this.children = [];
  this._html = ''; this._text = ''; this.style = {}; this.title = '';
  this.listeners = {}; this.attrs = {};
  Object.defineProperty(this, 'innerHTML', {
    get() { return this._html; },
    set(v) { this._html = v; this.children = []; }
  });
  Object.defineProperty(this, 'firstChild', { get() { return this.children[0] || null; } });
}
El.prototype.appendChild = function (c) { c.parent = this; this.children.push(c); return c; };
El.prototype.removeChild = function (c) { this.children = this.children.filter((x) => x !== c); return c; };
El.prototype.contains = function (n) { for (; n; n = n.parent) if (n === this) return true; return false; };
El.prototype.addEventListener = function (ev, fn) { (this.listeners[ev] = this.listeners[ev] || []).push(fn); };
El.prototype.setAttribute = function (k, v) { this.attrs[k] = v; };
El.prototype.getAttribute = function (k) { return k in this.attrs ? this.attrs[k] : null; };
El.prototype.querySelector = function () { return null; };
El.prototype.querySelectorAll = function () { return []; };
// text of a rendered row or menu row
Object.defineProperty(El.prototype, 'textContent', {
  get() { return this._text; }, set(v) { this._text = v; }
});

const docListeners = {};
const body = new El('body');
global.document = {
  body,
  documentElement: { clientWidth: 800, clientHeight: 600 },
  createElement: (t) => new El(t),
  getElementById: () => new El('div'),
  querySelector: () => null,
  querySelectorAll: () => [],
  addEventListener: (ev, fn) => { (docListeners[ev] = docListeners[ev] || []).push(fn); },
  removeEventListener: (ev, fn) => { docListeners[ev] = (docListeners[ev] || []).filter((f) => f !== fn); },
};
global.window = { addEventListener: () => {}, innerWidth: 800, innerHeight: 600 };
global.GMCPStructs = {};
global.gr = { setQuestMarker: () => {}, rooms: new Map() };
global.companionGlue = null;

const sent = [];
global.SendGMCP = (mod, payload) => sent.push({ mod, payload });
const noop = () => {};
for (const name of ['appendGameText', 'updateMapPanelTitle']) global[name] = noop;

// A host that records written markup, to build fake rows from.
const host = new El('div');
host.querySelectorAll = () => [];
global.document.getElementById = (id) => (id === 'quests-list' ? host : new El('div'));

// eslint-disable-next-line no-eval
(0, eval)([
  funcSrc(html, 'function escapeHTML('),
  funcSrc(html, 'function renderQuests('),
  funcSrc(html, 'function writeQuestsHTML('),
].join('\n'));

let fails = 0;
function check(name, cond, detail) {
  if (cond) console.log('  PASS  ' + name);
  else { fails++; console.log('  FAIL  ' + name + '\n          ' + (detail || '')); }
}

// ---------------------------------------------------------------- render
host.innerHTML = '';
global.GMCPStructs = { Char: { Quests: [
  { id: 42, name: 'The Hunt', abandonable: true, focused: true },
  { id: 28, name: 'Waking to Gaius' },
  { id: 54, name: 'Back to the Forge', repeatable: true, status: 'ready' },
] } };
renderQuests();
const markup = host.innerHTML;
console.log('   markup: ' + JSON.stringify(markup.slice(0, 160)));

// Fake elements derived from the rendered markup.
function rowFor(cls, id) {
  const re = new RegExp('<div class="' + cls + '(?: [^"]*)?" data-quest-id="' + id + '"([^>]*)>');
  const m = re.exec(markup);
  if (!m) return null;
  const r = new El('div');
  r.className = cls;
  const attrs = {};
  m[1].replace(/data-([a-z-]+)="([^"]*)"/g, (_, k, v) => { attrs['data-' + k] = v; });
  attrs['data-quest-id'] = String(id);
  r.attrs = attrs;
  const nm = new RegExp('data-quest-id="' + id + '"[^>]*>(?:.*?)<span class="qname">([^<]*)</span>').exec(markup);
  const qn = new El('span');
  qn.textContent = nm ? nm[1] : '';
  r.querySelector = (sel) => (sel === '.qname' ? qn : null);
  return r;
}
function targetOf(row) {
  const t = new El('span');
  t.parent = row;
  t.closest = (sel) => {
    if (!row) return null;
    return sel === '.qrow[data-quest-id]' && row.className === 'qrow' && row.getAttribute('data-quest-id') !== null ? row : null;
  };
  return t;
}

// ---------------------------------------------------------------- run page IIFE
let loadErr = '';
try {
  if (!menuSrc) throw new Error('quest context menu block markers not found in webclient-pure.html');
  // eslint-disable-next-line no-eval
  (0, eval)(menuSrc);
} catch (err) { loadErr = String(err); }
check('the quest context menu block loads', !loadErr, loadErr);

function rightClick(row) {
  let prevented = false;
  const ev = { target: targetOf(row), clientX: 10, clientY: 10, preventDefault: () => { prevented = true; } };
  (docListeners.contextmenu || []).forEach((fn) => fn(ev));
  return prevented;
}
function menuEl() { return body.children.find((c) => c.className === 'quest-ctx') || null; }
function menuRow(label) {
  const m = menuEl();
  return m ? m.children.find((c) => c.textContent === label) : null;
}
function click(r) {
  const ev = { stopPropagation: noop, preventDefault: noop, target: r };
  (r.listeners.click || []).forEach((fn) => fn(ev));
}

const row42 = rowFor('qrow', 42);
const row28 = rowFor('qrow', 28);
check('the 42 row carries data-abandonable="1"', row42 && row42.getAttribute('data-abandonable') === '1', markup.slice(0, 200));
check('the 28 row carries data-abandonable="0"', row28 && row28.getAttribute('data-abandonable') === '0', markup.slice(0, 200));

console.log('SCENARIO 1: abandonable quest');
check('right click on the row is intercepted', rightClick(row42) === true, 'no preventDefault');
check('the menu has Focus', !!menuRow('Focus'), 'no Focus row');
const ab42 = menuRow('Abandon');
check('the menu has Abandon', !!ab42, 'no Abandon row');
check('Abandon is not disabled', ab42 && !/ctx-disabled/.test(ab42.className), ab42 && ab42.className);
check('the header names the quest', menuEl() && menuEl().children[0] && menuEl().children[0].textContent === 'The Hunt',
  menuEl() && menuEl().children[0] && menuEl().children[0].textContent);
sent.length = 0;
if (ab42) click(ab42);
check('clicking Abandon sends Char.Quests.Abandon {id:42}',
  sent.length === 1 && sent[0].mod === 'Char.Quests.Abandon' && sent[0].payload.id === 42, JSON.stringify(sent));
check('the menu closes after an action', menuEl() && menuEl().style.display === 'none', menuEl() && menuEl().style.display);

console.log('SCENARIO 2: quest that cannot be abandoned');
rightClick(row28);
const ab28 = menuRow('Abandon');
check('Abandon carries ctx-disabled', ab28 && /(^| )ctx-disabled( |$)/.test(ab28.className), ab28 && ab28.className);
check('Abandon keeps the tooltip', ab28 && ab28.title === 'Nobody would take this back from you. It has to be finished.', ab28 && ab28.title);
sent.length = 0;
if (ab28) click(ab28);
check('clicking the disabled Abandon sends nothing', sent.length === 0, JSON.stringify(sent));
rightClick(row28);
sent.length = 0;
const fo28 = menuRow('Focus');
if (fo28) click(fo28);
check('Focus sends Char.Quests.Focus {id:28}',
  sent.length === 1 && sent[0].mod === 'Char.Quests.Focus' && sent[0].payload.id === 28, JSON.stringify(sent));
check('a menu click does not reach the row click handler (menu lives outside the row)',
  menuEl() && !row28.contains(menuEl()), 'menu is inside the row');

console.log('SCENARIO 3: finished repeatable row');
const rep = rowFor('qrepeatrow', 54);
check('the finished repeatable row exists in the markup', !!rep, markup);
const before = JSON.stringify(menuEl() && menuEl().children.map((c) => c.textContent));
check('no menu is opened for it (selector does not match)', rightClick(rep) === false, 'preventDefault called');
check('and the menu is unchanged', JSON.stringify(menuEl() && menuEl().children.map((c) => c.textContent)) === before, 'menu rebuilt');

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails ? 1 : 0);
