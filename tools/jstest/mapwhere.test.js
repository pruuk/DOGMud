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
    /~ Level |Somewhere in |_drawQuestBanner|data-wash-label|data-quest-banner/.test(src), false);
// Other ways to draw text: only the lTxt helper itself may create a text element.
check('exactly one lEl("text") site, the lTxt helper', (src.match(/lEl\(\s*["']text["']/g) || []).length, 1);
check('no createElementNS text element', /createElementNS\([^)]*["']text["']/.test(src), false);
check('the zone-exit label stays', /"→ " \+ e\.tozone/.test(src), true);

var page = fs.readFileSync(path.join(PUB, 'webclient-pure.html'), 'utf8');
check('the page loads mapwhere.js', /static\/js\/mapwhere\.js\?v=/.test(page), true);
check('the bar sits between the map buttons and the legend',
    page.indexOf('id="map-controls"') < page.indexOf('id="map-where"') && page.indexOf('id="map-where"') < page.indexOf('id="map-legend"'), true);
check('the title carries the level', /'Map \(' \+ obj\.Map\.zone \+ ', Level ' \+ \(obj\.Map\.cz \|\| 0\) \+ '\)'/.test(page), true);
check('the bar is refreshed on Room and Char.Quests', (page.match(/updateMapWhere\(\);/g) || []).length >= 2, true);
// #781: Room.Info comes again in the same room when its exits change; only a
// new room moves the map. The same room still updates its exits (D48).
check('recentre: the first Room.Info centres', MW.recentre(null, 5), true);
check('recentre: a new room centres', MW.recentre(5, 6), true);
check('recentre: the same room again does not', MW.recentre(5, 5), false);
check('the Room handler asks recentre before it moves the map',
    /var newRoom = MapWhere\.recentre\(allRooms\.lastRoomId, obj\.Info\.num\);\s*allRooms\.lastRoomId = obj\.Info\.num;/.test(page), true);
check('only a new room centres the map', /if \( newRoom \) \{\s*gr\.centerOnRoom\(r\.RoomId\);\s*\}/.test(page), true);
check('centerOnRoom is called once in the page', (page.match(/gr\.centerOnRoom\(/g) || []).length, 1);
var roomHandler = page.slice(page.indexOf('"Room": function() {'), page.indexOf('"Companion.Relay.Request"'));
check('the same room is not cut short: the Room handler returns only when Info is missing',
    (roomHandler.match(/return;/g) || []).length, 1);
check('a zone or floor change forgets the last room', /gr\.reset\(\);\s*allRooms\.lastRoomId = null;/.test(page), true);
check('a new connection forgets the last room', /socket\.onopen = function\(\) \{[^{}]*allRooms\.lastRoomId = null;/.test(page), true);
check('a temporary exit is not drawn as a map edge', /exitInfo\.kind !== "temporary"/.test(page), true);
console.log(failures ? failures + ' FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
