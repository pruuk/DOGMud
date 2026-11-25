// commandchain.js: the limit on one command chain (a macro, a tick, a
// trigger's then or else commands), for the web client's automation editors.
//
// The server holds at most Network.MaxPendingInputs of a player's commands
// waiting, so a chain longer than that loses its last commands. The server
// refuses to save one (users.MaxCommandChain); the editors count as the
// player types and refuse to save past it, so the player learns why before
// pressing Save. The common long chain is a walking route.
//
// The limit comes from the server, in Char.Automation's maxCommands (pushed
// on login, before an editor can open). FALLBACK is used only when it is
// missing: the shipped value, which the server still enforces either way.
//
// Pure functions, loaded by the page and by node for the tests
// (tools/jstest/commandchain.test.js).

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;       // node, for the tests
    } else {
        root.CommandChain = api;    // browser
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var FALLBACK = 10;

    // count is how many commands a chain sends: it is split on ";" and each
    // piece trimmed, and a blank piece is not a command (the same rule as
    // the page's senders and the server's users.CommandChainLength).
    function count(chain) {
        var n = 0;
        String(chain == null ? '' : chain).split(';').forEach(function (c) {
            if (c.trim() !== '') n++;
        });
        return n;
    }

    // limit reads the server's limit from a Char.Automation payload.
    function limit(automation) {
        var m = automation && automation.maxCommands;
        return (typeof m === 'number' && m >= 1 && Math.floor(m) === m) ? m : FALLBACK;
    }

    // label is the live count shown under a commands field.
    function label(n, max) {
        return n + ' of ' + max + ' command' + (max === 1 ? '' : 's');
    }

    // warning is shown, in words, when a chain is over the limit.
    function warning(max) {
        return 'Too many commands: a chain can hold at most ' + max +
            '. Split a long route into two.';
    }

    function fits(chain, max) {
        return count(chain) <= max;
    }

    return {
        FALLBACK: FALLBACK,
        count: count,
        limit: limit,
        label: label,
        warning: warning,
        fits: fits
    };
}));
