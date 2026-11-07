// Chat channel hints (#808, Calabe 2026-11-07): each channel window starts
// with a hint the first message removes, and each button carries the same
// words as its tooltip.
'use strict';
var fs = require('fs');
var path = require('path');
var page = fs.readFileSync(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'), 'utf8');
var failures = 0;
function check(name, got, want) {
    if (got === want) { console.log('ok   ' + name); return; }
    failures++; console.log('FAIL ' + name + '\n  got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want));
}
var hints = {
    say: 'What people say in the room you are in. To talk, type <code>say hello</code>.',
    whisper: 'Private messages between you and one player, wherever they are. Type <code>whisper &lt;name&gt; &lt;message&gt;</code>. <code>reply &lt;message&gt;</code> answers the last one.',
    party: 'Only your party hears this. Type <code>party say &lt;message&gt;</code>. No party yet? <code>party create</code>, then <code>party invite &lt;name&gt;</code>.',
    broadcast: 'Game tips and announcements. No one can post here. <code>set tips</code> turns tips off or on.',
    chat: 'Everyone online, for general talk. Type <code>chat &lt;message&gt;</code>. Shouts from nearby show here too. <code>channels</code> turns a channel off.',
    newbie: 'Everyone online. Ask anything about the game, and other players will help. No question is too small. Type <code>newbie &lt;message&gt;</code>.',
    trade: 'Everyone online, for buying and selling. Type <code>trade &lt;message&gt;</code>.'
};
function plain(html) { return html.replace(/<\/?code>/g, '').replace(/&lt;/g, '<').replace(/&gt;/g, '>'); }
Object.keys(hints).forEach(function (ch) {
    var win = new RegExp('<div id="comm-' + ch + '" class="chat-window[^"]*"><div class="chat-hint">([\\s\\S]*?)</div></div>').exec(page);
    check(ch + ' window starts with its hint', win ? win[1] : null, hints[ch]);
    var btn = new RegExp('<button[^>]*id="comm-tab-' + ch + '"[^>]*title="([^"]*)"').exec(page);
    check(ch + ' button tooltip', btn ? btn[1].replace(/&lt;/g, '<').replace(/&gt;/g, '>') : null, plain(hints[ch]));
});
var handlerAt = page.indexOf('"Comm": function()');
var handler = page.slice(handlerAt, page.indexOf('"Room": function()', handlerAt));
check('the handler removes a window\'s hint before the first line',
    /var hint = panel\.querySelector\('\.chat-hint'\);\s*if \(hint\) panel\.removeChild\(hint\);/.test(handler) &&
    handler.indexOf("querySelector('.chat-hint')") < handler.indexOf('panel.appendChild(p)'), true);
check('shouts go into the Chat window',
    /var windowName = channelName === 'shout' \? 'chat' : channelName;/.test(handler), true);
check('a shout reads "<name> shouts: <words>"',
    /\(channelName === 'shout' \? ' shouts' : ''\)/.test(handler), true);
console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
