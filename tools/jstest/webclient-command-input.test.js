// webclient-command-input.test.js: the command box keeps browser and phone
// helpers off (#833).
//
// Run:  node tools/jstest/webclient-command-input.test.js
//
// The command box is a plain text input every command passes through. The
// browser's form history offered earlier entries ("1") in a white pop-up over
// the game text, and a phone keyboard would capitalise or autocorrect what a
// player types, a password included (the same input becomes type=password at
// the login steps). Read from the real markup.
'use strict';

var fs = require('fs');
var path = require('path');
var html = fs.readFileSync(path.join(
    __dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'), 'utf8');

var failures = 0;
function check(name, ok, detail) {
    if (ok) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + '\n  ' + detail);
}

var tag = /<input[^>]*id="command-input"[^>]*>/.exec(html);
if (!tag) throw new Error('command-input markup not found');
function attr(name) {
    var m = new RegExp('\\s' + name + '="([^"]*)"').exec(tag[0]);
    return m ? m[1] : null;
}
check('the command box turns off browser form history', attr('autocomplete') === 'off',
    'autocomplete=' + attr('autocomplete'));
check('the command box turns off phone autocapitalize', attr('autocapitalize') === 'off',
    'autocapitalize=' + attr('autocapitalize'));
check('the command box turns off phone autocorrect', attr('autocorrect') === 'off',
    'autocorrect=' + attr('autocorrect'));
check('the command box turns off spellcheck', attr('spellcheck') === 'false',
    'spellcheck=' + attr('spellcheck'));

console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
