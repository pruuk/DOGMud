// questtabs.js: splits the Char.Quests rows into the Quests panel's three
// tabs (web client base plan 7; layout spec rulings 18 and 19; owner P4).
// Active: no status. Repeat: a repeatable finished and offered again
// ("ready") or not yet ("waiting"), ready first, then by name. Completed:
// finished for good ("done"), by name. Tested by tools/jstest/questtabs.test.js.
(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.QuestTabs = api;
    }
}(typeof self !== 'undefined' ? self : this, function () {
    var TABS = ['active', 'repeat', 'done'];
    var EMPTY = { active: 'No active quests.', repeat: 'Nothing to repeat yet.', done: 'Nothing finished yet.' };

    function byName(a, b) { return String(a.name || '').localeCompare(String(b.name || '')); }

    function split(rows) {
        var out = { active: [], repeat: [], done: [], ready: 0 };
        (Array.isArray(rows) ? rows : []).forEach(function (q) {
            if (!q || typeof q !== 'object') { return; }
            if (!q.status) { out.active.push(q); }
            else if (q.status === 'ready' || q.status === 'waiting') { out.repeat.push(q); }
            else if (q.status === 'done') { out.done.push(q); }
        });
        out.repeat.sort(function (a, b) {
            var ra = a.status === 'ready' ? 0 : 1, rb = b.status === 'ready' ? 0 : 1;
            return ra !== rb ? ra - rb : byName(a, b);
        });
        out.done.sort(byName);
        out.ready = out.repeat.filter(function (q) { return q.status === 'ready'; }).length;
        return out;
    }

    function repeatLabel(ready) { return ready > 0 ? 'Repeat (' + ready + ')' : 'Repeat'; }
    function tabOrDefault(tab) { return TABS.indexOf(tab) >= 0 ? tab : 'active'; }

    return { TABS: TABS, EMPTY: EMPTY, split: split, repeatLabel: repeatLabel, tabOrDefault: tabOrDefault };
}));
