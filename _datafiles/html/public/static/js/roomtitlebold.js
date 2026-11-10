// roomtitlebold.js: draws room titles in the one bold in the game window.
//
// Room titles are bold (rule 7 of the web client base spec). The game's colour
// tags have no working bold (ansitags reads bold= and ignores it), so the web
// client finds the room title's colour and adds bold to it. Anything else
// drawn in that colour turns bold too: a few lines reuse the alias.
//
// A top-level tag closes with a full reset, which ends the bold. A room-title
// tag nested inside another colour would stay bold until the outer tag closes,
// so keep this colour at the top level. Telnet players are not affected.
// internal/guards/client_type_guard_test.go holds this number equal to
// room-title in ansi-aliases.yaml.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.RoomTitleBold = api;  // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var ROOM_TITLE_COLOUR = 170;
    var PLAIN = '\x1b[38;5;' + ROOM_TITLE_COLOUR + 'm';
    var BOLD = '\x1b[1;38;5;' + ROOM_TITLE_COLOUR + 'm';

    function apply(data) {
        if (typeof data !== 'string' || data.indexOf(PLAIN) === -1) return data;
        return data.split(PLAIN).join(BOLD);
    }

    return { apply: apply };
}));
