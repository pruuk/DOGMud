// mapwhere.js: the words of the Map's Location / Destination bar (web client
// base plan 7, ruling 17). roomName: Room.Info.name. quest: the focused
// Char.Quests row, or null. Destination is its target label, else its name;
// empty when no quest is focused. Tested by tools/jstest/mapwhere.test.js.
(function (root, factory) {
    var api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.MapWhere = api;
    }
}(typeof self !== 'undefined' ? self : this, function () {
    function clean(s) { return String(s == null ? '' : s).trim(); }
    function text(roomName, quest) {
        var dest = '';
        if (quest) { dest = clean(quest.target_label) || clean(quest.name); }
        return { location: clean(roomName), destination: dest };
    }
    return { text: text };
}));
