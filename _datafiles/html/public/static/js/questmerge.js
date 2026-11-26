// questmerge.js: the quest editor's save merge (#958 builder fix). The editor
// used to rebuild each object from the fields it has controls for, so a save
// dropped every other field the quest file carried (step destination, branch,
// repeat_limit, rewards.summary and surprise, a trigger's missing_items, and
// anything added later). Each gather now overlays its controls' values on a
// deep copy of the object the editor loaded. A new row starts from {}.
// Tested by tools/jstest/questmerge.test.js; used by quests.js.
(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.QuestMerge = api;
    }
}(typeof self !== 'undefined' ? self : this, function () {
    function clone(v) { return v === undefined ? undefined : JSON.parse(JSON.stringify(v)); }

    // merge overlays over on a deep copy of base. A key whose value in over is
    // undefined is deleted: an empty control clears its own field. Only the
    // keys named in over change; every other key of base survives.
    function merge(base, over) {
        var out = clone(base || {});
        Object.keys(over || {}).forEach(function (k) {
            if (over[k] === undefined) { delete out[k]; } else { out[k] = clone(over[k]); }
        });
        return out;
    }

    // step: v is {id, description, hint, branch}; a blank branch is omitted.
    function step(base, v) {
        return merge(base, {
            id: v.id, description: v.description, hint: v.hint,
            branch: v.branch ? v.branch : undefined
        });
    }

    // conditions: c holds the nine conditions the editor has controls for, an
    // empty one as undefined. The result is undefined when nothing is left.
    function conditions(base, c) {
        var out = merge(base, c);
        return Object.keys(out).length ? out : undefined;
    }

    // trigger: v is {event, actions, filters, dropped, conditions}. filters
    // are the offered filter values, dropped the filter keys another event
    // offers (a changed event takes its old filters with it), conditions the
    // control values (see conditions).
    function trigger(base, v) {
        var over = { event: v.event, actions: v.actions };
        (v.dropped || []).forEach(function (k) { over[k] = undefined; });
        Object.keys(v.filters || {}).forEach(function (k) { over[k] = v.filters[k]; });
        over.conditions = conditions((base || {}).conditions, v.conditions);
        return merge(base, over);
    }

    // destinationText is the read-only line under a step: its destination as
    // loaded (quests.StepDestination as json).
    function destinationText(d) {
        d = d || {};
        var parts = [];
        if (d.anywhere) { parts.push('anywhere'); }
        if (d.room) { parts.push('room ' + d.room); }
        if (d.mob && d.mob.length) { parts.push((d.mob.length > 1 ? 'mobs ' : 'mob ') + d.mob.join(', ')); }
        if (d.area) { parts.push('area ' + d.area); }
        if (d.zone) { parts.push('zone ' + d.zone); }
        return 'Destination: ' + (parts.length ? parts.join(', ') : 'inferred from triggers');
    }

    // branchValues lists the values of every flag in flags (rows with a values
    // array), in order, without repeats: what a step's Branch may name.
    function branchValues(flags) {
        var out = [];
        (flags || []).forEach(function (f) {
            (f.values || []).forEach(function (v) { if (v && out.indexOf(v) < 0) { out.push(v); } });
        });
        return out;
    }

    return { clone: clone, merge: merge, step: step, conditions: conditions, trigger: trigger,
        destinationText: destinationText, branchValues: branchValues };
}));
