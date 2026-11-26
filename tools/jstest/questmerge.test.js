// questmerge.test.js: the quest editor's save merge (#958 builder fix). A save
// used to rebuild each object from the fields the editor has controls for and
// drop the rest. The gathers now overlay the controls on the loaded object.
// Each "no edit" case feeds the merge the values the controls hold when the
// user changes nothing, the way quests.js does, and expects the loaded object back.
// Run: node tools/jstest/questmerge.test.js
'use strict';
var path = require('path');
var QM = require(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'static', 'js', 'questmerge.js'));
var failures = 0;
function check(name, got, want) {
    var g = JSON.stringify(got), w = JSON.stringify(want);
    if (g === w) { console.log('ok   ' + name); return; }
    failures++; console.log('FAIL ' + name + '\n  got  ' + g + '\n  want ' + w);
}

// A step as the editor loads it: a destination, a branch, and a repeat-style
// field the editor has no control for.
var step = { id: 'fetch', description: 'd', hint: 'h', branch: 'windward',
    destination: { room: 3082, mob: [241, 242], area: 'x', zone: 'y' }, repeat_limit: 3 };
var noEditStep = { id: 'fetch', description: 'd', hint: 'h', branch: 'windward' };
check('step: nothing edited keeps destination, branch, repeat_limit',
    QM.step(step, noEditStep), step);
check('step: an edited hint lands and the rest survives',
    QM.step(step, { id: 'fetch', description: 'd', hint: 'new hint', branch: 'windward' }),
    { id: 'fetch', description: 'd', hint: 'new hint', branch: 'windward',
        destination: step.destination, repeat_limit: 3 });
check('step: a blank branch is omitted',
    QM.step(step, { id: 'fetch', description: 'd', hint: 'h', branch: '' }).branch, undefined);
check('step: a new step starts from {} and has no branch key',
    QM.step({}, { id: 'n', description: '', hint: '', branch: '' }),
    { id: 'n', description: '', hint: '' });
check('step: the loaded object is not altered',
    step.hint, 'h');

// A trigger as quest 76 has it: conditions.missing_items has no control.
var trig = { event: 'room_enter', room: 12, area: 'a', actions: [{ grant: '76-start' }],
    conditions: { has: ['76-x'], missing_items: [5, 6], missing_item: 9 } };
var trigNoEdit = { event: 'room_enter', actions: [{ grant: '76-start' }],
    filters: { room: 12, area: 'a', zone: '', count: 0, label: '', distinct: '', explored_line: '' },
    dropped: ['mob', 'item'],
    conditions: { has: ['76-x'], missing: undefined, in_room: undefined, has_item: undefined,
        missing_item: 9, has_flag: undefined, missing_flag: undefined, has_gold: undefined,
        has_masterwork: undefined } };
var gotTrig = QM.trigger(trig, trigNoEdit);
check('trigger: conditions.missing_items survives an unedited gather',
    gotTrig.conditions, trig.conditions);
check('trigger: event and actions survive', [gotTrig.event, gotTrig.actions], [trig.event, trig.actions]);
check('trigger: a cleared condition control clears its own field',
    QM.trigger(trig, { event: 'room_enter', actions: [], filters: {}, dropped: [],
        conditions: { has: ['76-x'], missing_item: undefined } }).conditions,
    { has: ['76-x'], missing_items: [5, 6] });
check('trigger: no condition left means no conditions key',
    QM.trigger({ event: 'room_enter', conditions: { has: ['a'] } },
        { event: 'room_enter', actions: [], filters: {}, dropped: [], conditions: { has: undefined } }).conditions,
    undefined);
check('trigger: a changed event takes the old event\'s filters with it',
    QM.trigger({ event: 'room_enter', room: 12, actions: [] },
        { event: 'skill_use', actions: [], filters: { skill: 'sword' }, dropped: ['room'], conditions: {} }),
    { event: 'skill_use', actions: [], skill: 'sword' });
check('trigger: a field with no control survives',
    QM.trigger({ event: 'room_enter', future_field: 7, actions: [] },
        { event: 'room_enter', actions: [], filters: {}, dropped: [], conditions: {} }).future_field, 7);

// Quest level: rewards.summary and rewards.surprise have no control.
var rewards = { gold: 10, summary: 'a line', surprise: true };
check('rewards: summary and surprise survive an unedited gather',
    QM.merge(rewards, { gold: 10 }), rewards);
check('rewards: an edited gold lands',
    QM.merge(rewards, { gold: 20 }), { gold: 20, summary: 'a line', surprise: true });
var quest = { questid: 1, name: 'q', repeat_limit: 2, rewards: rewards };
check('quest: repeat_limit survives an unedited gather',
    QM.merge(quest, { questid: 1, name: 'q', rewards: QM.merge(quest.rewards, { gold: 10 }) }), quest);

// The read-only lines and the picker.
check('destination: room', QM.destinationText({ room: 3082 }), 'Destination: room 3082');
check('destination: mob', QM.destinationText({ mob: [241] }), 'Destination: mob 241');
check('destination: mobs', QM.destinationText({ mob: [241, 242] }), 'Destination: mobs 241, 242');
check('destination: area and zone', QM.destinationText({ area: 'x', zone: 'y' }), 'Destination: area x, zone y');
check('destination: anywhere', QM.destinationText({ anywhere: true }), 'Destination: anywhere');
check('destination: empty is inferred', QM.destinationText({}), 'Destination: inferred from triggers');
check('destination: absent is inferred', QM.destinationText(undefined), 'Destination: inferred from triggers');
check('branch values: every flag value once, in order',
    QM.branchValues([{ key: 'side', values: ['windward', 'leeward'] }, { key: 'b', values: ['leeward', 'x'] }, { key: 'c' }]),
    ['windward', 'leeward', 'x']);

console.log(failures ? failures + ' FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
