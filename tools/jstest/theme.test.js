// theme.test.js: the site's Dark / Light / Match device choice (design
// 2026-11-01).
//
// Run:  node tools/jstest/theme.test.js
//
// Covers _datafiles/html/public/static/js/theme.js with a fake window: what an
// unknown or missing saved value falls back to, how "device" follows the
// device setting, that set() saves and draws, that storage failures fall back
// to dark instead of throwing, and that a change saved by another page (the web
// client iframe) redraws this one. Also checks both pages load theme.js before
// their stylesheets, so a light page does not flash dark first.
//
// Dependency-free and assertion-library-free, matching renderguard.test.js.
// Exits non-zero on failure.

var path = require('path');
var fs = require('fs');

var SiteTheme = require(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'theme.js'
));

var failures = 0;
var checks = 0;

function check(name, got, want) {
    checks++;
    if (got !== want) {
        console.log('FAIL ' + name + '\n  got:  ' + JSON.stringify(got) + '\n  want: ' + JSON.stringify(want));
        failures++;
        return;
    }
    console.log('ok   ' + name);
}

// fakeWindow builds just enough of a window for theme.js. storageThrows makes
// every localStorage call throw, as in a private window with site data blocked.
function fakeWindow(opts) {
    opts = opts || {};
    var store = {};
    var listeners = {};
    var schemeListeners = [];
    var attrs = {};
    var w = {
        deviceLight: !!opts.deviceLight,
        localStorage: {
            getItem: function (k) {
                if (opts.storageThrows) throw new Error('blocked');
                return Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null;
            },
            setItem: function (k, v) {
                if (opts.storageThrows) throw new Error('blocked');
                store[k] = String(v);
            }
        },
        matchMedia: function () {
            return {
                matches: w.deviceLight,
                addEventListener: function (type, fn) { schemeListeners.push(fn); }
            };
        },
        addEventListener: function (type, fn) { (listeners[type] = listeners[type] || []).push(fn); },
        document: {
            documentElement: {
                setAttribute: function (k, v) { attrs[k] = v; },
                getAttribute: function (k) { return attrs[k]; }
            }
        },
        theme: function () { return attrs['data-theme']; },
        store: store,
        fire: function (type, e) { (listeners[type] || []).forEach(function (fn) { fn(e); }); },
        deviceChanged: function (light) {
            w.deviceLight = light;
            schemeListeners.forEach(function (fn) { fn(); });
        }
    };
    return w;
}

// Pure helpers.
check('normalize keeps dark', SiteTheme.normalize('dark'), 'dark');
check('normalize keeps light', SiteTheme.normalize('light'), 'light');
check('normalize keeps device', SiteTheme.normalize('device'), 'device');
check('normalize: missing value is dark', SiteTheme.normalize(null), 'dark');
check('normalize: unknown value is dark', SiteTheme.normalize('sepia'), 'dark');
check('resolve: device, device dark', SiteTheme.resolve('device', false), 'dark');
check('resolve: device, device light', SiteTheme.resolve('device', true), 'light');
check('resolve: light ignores device', SiteTheme.resolve('light', false), 'light');
check('resolve: dark ignores device', SiteTheme.resolve('dark', true), 'dark');

// A new visitor gets dark, even on a light device.
var w = fakeWindow({ deviceLight: true });
SiteTheme.start(w);
check('new visitor: choice is dark', SiteTheme.get(), 'dark');
check('new visitor: page draws dark', w.theme(), 'dark');

// set() saves and draws.
check('set light returns light', SiteTheme.set('light'), 'light');
check('set light: saved', w.store.siteTheme, 'light');
check('set light: page draws light', w.theme(), 'light');
SiteTheme.set('nonsense');
check('set unknown: saved as dark', w.store.siteTheme, 'dark');
check('set unknown: page draws dark', w.theme(), 'dark');

// Match device follows the device, now and when it changes.
SiteTheme.set('device');
check('device on a light device draws light', w.theme(), 'light');
w.deviceChanged(false);
check('device switched to dark: page draws dark', w.theme(), 'dark');
SiteTheme.set('light');
w.deviceChanged(true);
w.deviceChanged(false);
check('a device change does not move a fixed choice', w.theme(), 'light');

// Another page saved a new choice (the web client iframe): redraw.
w.store.siteTheme = 'dark';
w.fire('storage', { key: 'siteTheme' });
check('storage event for siteTheme redraws', w.theme(), 'dark');
w.store.siteTheme = 'light';
w.fire('storage', { key: 'sliderValues' });
check('storage event for another key is ignored', w.theme(), 'dark');

// Storage blocked: dark, and no throw.
var blocked = fakeWindow({ storageThrows: true, deviceLight: true });
SiteTheme.start(blocked);
check('blocked storage: page draws dark', blocked.theme(), 'dark');
var threw = false;
try { SiteTheme.set('light'); } catch (e) { threw = true; }
check('blocked storage: set does not throw', threw, false);
// The choice holds for this page even though it could not be saved (#768
// review item 13: set() used to re-read the failed save and stay dark).
check('blocked storage: set light draws light', blocked.theme(), 'light');
check('blocked storage: get returns the choice made', SiteTheme.get(), 'light');
SiteTheme.set('device');
check('blocked storage: device on a light device draws light', blocked.theme(), 'light');
blocked.deviceChanged(false);
check('blocked storage: device follows the device', blocked.theme(), 'dark');

// A choice made here is replaced by one another page saves later.
var shared = fakeWindow();
SiteTheme.start(shared);
SiteTheme.set('light');
shared.store.siteTheme = 'dark';
shared.fire('storage', { key: 'siteTheme' });
check('another page saving dark replaces this page\'s light', shared.theme(), 'dark');
check('get follows the other page\'s choice', SiteTheme.get(), 'dark');

// Both pages load theme.js before their first stylesheet.
function loadsBeforeCss(file, cssName) {
    var page = fs.readFileSync(path.join(
        __dirname, '..', '..', '_datafiles', 'html', 'public', file), 'utf8');
    var js = page.indexOf('static/js/theme.js');
    var css = page.indexOf(cssName);
    return js !== -1 && css !== -1 && js < css;
}
check('_header.html loads theme.js before gomud.css', loadsBeforeCss('_header.html', 'static/css/gomud.css'), true);
check('webclient-pure.html loads theme.js before xterm.css', loadsBeforeCss('webclient-pure.html', 'static/css/xterm.css'), true);

console.log(failures ? failures + ' of ' + checks + ' checks FAILED' : 'all ' + checks + ' checks passed');
process.exit(failures ? 1 : 0);
