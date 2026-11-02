// motdrestyle.js — show the Message of the Day without its box in the web
// client.
//
// The server draws the MOTD (internal/usercommands/motd.go, motdBox) as a
// yellow double-line box 80 columns wide, padded on the right:
//
//    ╔═════ … ═════╗
//    ║  .:  M E S S A G E   O F   T H E   D A Y        ║
//    ╠═════ … ═════╣
//    ║ text line                                       ║
//    ╚═════ … ═════╝
//
// The web client drops the border rows and side bars, starts every line at
// the left edge, keeps the title in its own colour and draws the text in the
// default game text colour, the same as "Welcome to" on the login screen
// (design 2026-11-01). Telnet and Mudlet players still get the box.
//
// Client side only, so it matches the box by its shape. If motdBox changes
// shape or title, restyle() finds no box and returns the text unchanged: the
// old box shows again, nothing else breaks.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.MotdRestyle = api;    // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var TITLE = 'M E S S A G E   O F   T H E   D A Y';
    var ESC = /\x1b\[[0-9;]*[A-Za-z]/g;    // every escape sequence
    var CTRL = /\x1b\[[0-9;]*[A-LN-Za-ln-z]/g; // escapes that are not colours
    var BORDER = /^\s*[╔╠╚]═+[╗╣╝]\s*$/;  // a top, middle or bottom rule
    var TITLE_COLOR = '\x1b[1;36m';        // bright cyan, as the server sends
    var RESET = '\x1b[0m';                 // back to the default text colour

    // Restyle the MOTD box inside one websocket message, if there is one.
    function restyle(data) {
        if (typeof data !== 'string' || data.indexOf(TITLE) === -1) return data;

        var lines = data.split('\r\n');
        var start = -1, end = -1, i;
        for (i = 0; i < lines.length; i++) {
            var vis = lines[i].replace(ESC, '');
            if (!BORDER.test(vis)) continue;
            if (start === -1) {
                if (vis.indexOf('╔') !== -1) start = i;
            } else if (vis.indexOf('╚') !== -1) {
                end = i;
                break;
            }
        }
        if (start === -1 || end === -1) return data;

        var out = [], carry = '';
        for (i = start; i <= end; i++) {
            var line = lines[i];
            var plain = line.replace(ESC, '');
            // Escapes that are not colours (cursor moves, line clears) stay.
            var ctrl = (line.match(CTRL) || []).join('');

            if (BORDER.test(plain)) { carry += ctrl; continue; }

            var m = /^\s*║\s?(.*?)\s*║\s*$/.exec(plain);
            if (!m) return data; // not the shape we know; leave it alone
            var text = m[1];
            if (text.indexOf(TITLE) !== -1) {
                out.push(carry + ctrl + TITLE_COLOR + text.trim() + RESET);
            } else {
                out.push(carry + ctrl + RESET + text);
            }
            carry = '';
        }
        if (!out.length) return data;
        // The server closes its colour after the bottom rule; end on the
        // default colour too, with any escapes the bottom rule carried.
        out[out.length - 1] += carry + RESET;

        return lines.slice(0, start).concat(out, lines.slice(end + 1)).join('\r\n');
    }

    return { restyle: restyle };
}));
