// theme.js: the site's Dark / Light / Match device choice.
//
// One choice covers the whole site: the pages around the web client and the
// web client itself (design 2026-11-01). The game text window always stays
// dark; its colours are the --term-* tokens, which the light theme leaves
// alone.
//
// The choice is saved in localStorage under "siteTheme" as "dark", "light" or
// "device". Dark is the default. apply() resolves it to "dark" or "light" and
// sets <html data-theme="...">, which gomud.css reads. "device" follows the
// phone or computer's own setting (prefers-color-scheme) and updates when that
// changes.
//
// The web client runs in an iframe of the same site, so a change made in its
// Settings menu reaches the page around it through the storage event.
//
// Load it in <head>, before the stylesheets paint, so a light-theme page does
// not flash dark first.

(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;      // node, for the tests
    } else {
        root.SiteTheme = api;
        api.start(root);
    }
}(this, function () {
    'use strict';

    var KEY = 'siteTheme';
    var CHOICES = ['dark', 'light', 'device'];
    var DEFAULT = 'dark';

    // normalize turns anything stored or passed in into a known choice.
    function normalize(choice) {
        return CHOICES.indexOf(choice) >= 0 ? choice : DEFAULT;
    }

    // resolve turns a choice into the theme to draw: "dark" or "light".
    // deviceIsLight says whether the device asks for a light scheme.
    function resolve(choice, deviceIsLight) {
        choice = normalize(choice);
        if (choice === 'device') return deviceIsLight ? 'light' : 'dark';
        return choice;
    }

    var win = null;

    // The choice made on this page, kept in memory so it holds even when it
    // could not be saved (private windows, blocked site data). null until
    // set() runs; a change saved by another page replaces it.
    var remembered = null;

    // Storage can throw (private windows, blocked site data); the default
    // theme is always safe to fall back to.
    function get() {
        if (remembered !== null) return remembered;
        try {
            return normalize(win.localStorage.getItem(KEY));
        } catch (e) {
            return DEFAULT;
        }
    }

    function deviceIsLight() {
        try {
            return win.matchMedia('(prefers-color-scheme: light)').matches;
        } catch (e) {
            return false;
        }
    }

    function apply() {
        win.document.documentElement.setAttribute('data-theme', resolve(get(), deviceIsLight()));
    }

    function set(choice) {
        choice = normalize(choice);
        remembered = choice;
        try {
            win.localStorage.setItem(KEY, choice);
        } catch (e) {
            // Not saved, but still shown for this page.
        }
        apply();
        return choice;
    }

    function start(w) {
        win = w;
        remembered = null;
        apply();
        // Another page saved a choice (the web client iframe, another tab):
        // it replaces the one remembered here, so read storage again.
        w.addEventListener('storage', function (e) {
            if (e.key !== KEY) return;
            remembered = null;
            apply();
        });
        try {
            w.matchMedia('(prefers-color-scheme: light)').addEventListener('change', apply);
        } catch (e) {
            // Older browsers: "Match device" picks up a change on the next load.
        }
    }

    return {
        CHOICES: CHOICES,
        normalize: normalize,
        resolve: resolve,
        get: get,
        set: set,
        apply: apply,
        start: start
    };
}));
