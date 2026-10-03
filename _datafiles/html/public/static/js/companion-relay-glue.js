// companion-relay-glue.js: the game page's side of the companion key relay.
//
// A player may run their companion on their own model key. The key lives
// only in the relay page (modules/aicompanion/relayweb/relay.js), served on
// its own origin and loaded here in a hidden iframe. This glue passes the
// server's Companion.Relay.Request to that frame and the frame's answer back
// as Companion.Relay.Response. It never receives a key and never forwards
// one: every message it sends is built field by field from checked values,
// never by passing on what the frame sent.
//
//   server -> page  Companion.Relay.Request {id, body, deadlineMs?}
//   page -> frame   {type:'hello', account} {type:'setup'}
//                   {type:'request', id, body, deadlineMs?}
//   frame -> page   {type:'status', ready, model?, finds?, lively?, locked}
//                   {type:'response', id, status, body} {type:'hide'}
//   page -> server  Companion.Relay.Ready {model, finds, lively} | Companion.Relay.Gone {}
//                   Companion.Relay.Response {id, status, body}
//
// Tests: tools/jstest/companion-relay-glue.test.js.

(function (root, factory) {
  var api = factory();
  if (typeof module === 'object' && module.exports) {
    module.exports = api; // node, for the tests
  } else {
    root.CompanionRelayGlue = api;
  }
}(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  // The server closes a player's websocket on an inbound frame over 64 KiB
  // (internal/web wsMaxMessageBytes), so a reply frame is kept to 60 KiB.
  var MAX_FRAME_BYTES = 60 * 1024;
  // IAC SB GMCP before the text and IAC SE after it (SendGMCP).
  var IAC_BYTES = 5;
  var MAX_MODEL = 100;
  var MAX_ACCOUNT = 64;
  // More outstanding requests than this means something is wrong; the
  // server allows far fewer.
  var MAX_PENDING = 64;
  var RESPONSE = 'Companion.Relay.Response';

  function isValidId(id) {
    return typeof id === 'string' && /^[0-9a-f]{1,64}$/i.test(id);
  }

  function isValidModel(m) {
    return typeof m === 'string' && m.length > 0 && m.length <= MAX_MODEL && /^[\x21-\x7e]+$/.test(m);
  }

  // canonicalRelayOrigin returns the origin the browser will report for the
  // relay ("https://host[:port]", lowercased, default port dropped), or ""
  // for anything that is not a bare https origin.
  function canonicalRelayOrigin(s) {
    if (typeof s !== 'string' || s === '') { return ''; }
    var u;
    try { u = new URL(s); } catch (e) { return ''; }
    if (u.protocol !== 'https:' || u.hostname === '' || u.username !== '' || u.password !== '' ||
        u.pathname !== '/' || u.search !== '' || u.hash !== '' || /[?#]/.test(s)) {
      return '';
    }
    return u.origin;
  }

  // frameBytes is the size of the websocket frame SendGMCP would send.
  function frameBytes(pkg, obj) {
    return new TextEncoder().encode(pkg + ' ' + JSON.stringify(obj)).length + IAC_BYTES;
  }

  // createGlue is the glue's rules apart from the DOM. env:
  //   relayOrigin     the relay's origin (checked here again)
  //   sendGMCP(pkg, obj)        to the server
  //   postToFrame(msg, origin)  to the relay frame
  //   frameWindow()   the relay frame's window, to check who is speaking
  //   onState({visible, open, ready, locked}) and onHide(), for the page
  function createGlue(env) {
    var relayOrigin = canonicalRelayOrigin(env.relayOrigin);
    var frameReady = false;
    var account = '';        // the logged-in account, '' when logged out
    var announced = '';      // the account the frame was last told
    var relayUp = false;     // the frame answered the current hello
    var ready = false;
    var locked = false;
    var open = false;
    var pending = [];        // request ids forwarded and not yet answered

    function state() {
      return { visible: account !== '' && relayUp, open: open, ready: ready, locked: locked };
    }
    function changed() { if (env.onState) { env.onState(state()); } }

    function send(pkg, obj) { return env.sendGMCP(pkg, obj); }
    function post(msg) {
      if (relayOrigin === '' || !frameReady) { return false; }
      env.postToFrame(msg, relayOrigin);
      return true;
    }

    function fail(id) { send(RESPONSE, { id: id, status: 0, body: '' }); }

    function announce() {
      if (account === '' || announced === account) { return; }
      if (post({ type: 'hello', account: account })) {
        announced = account;
        relayUp = false;
      }
    }

    function hide() {
      open = false;
      if (env.onHide) { env.onHide(); }
      changed();
    }

    // hello records the logged-in account and tells the frame. The frame
    // binds a stored key to it and drops the key when it changes.
    function hello(a) {
      if (typeof a !== 'string') { return; }
      a = a.trim();
      if (a === '' || a.length > MAX_ACCOUNT) { return; }
      account = a;
      announce();
      changed();
    }

    // frameLoaded runs on every load of the frame. A reloaded frame has
    // lost its state, so it is told the account again.
    function frameLoaded() {
      frameReady = true;
      announced = '';
      relayUp = false;
      announce();
      changed();
    }

    // disconnected forgets the login. The frame keeps any key it holds; the
    // next hello tells it who logged in, and a different account drops it.
    function disconnected() {
      account = '';
      announced = '';
      relayUp = false;
      ready = false;
      pending = [];
      hide();
    }

    function onGMCPRequest(obj) {
      if (!obj || typeof obj !== 'object' || !isValidId(obj.id)) { return; }
      var id = obj.id;
      var body = obj.body;
      if (body === undefined || body === null) { fail(id); return; }
      var msg = { type: 'request', id: id, body: body };
      // How long the server waits for this call; the frame stops its fetch
      // then. Passed on only as a positive whole number of milliseconds.
      var d = obj.deadlineMs;
      if (typeof d === 'number' && Number.isInteger(d) && d > 0) { msg.deadlineMs = d; }
      if (account === '' || pending.indexOf(id) !== -1 || !post(msg)) {
        fail(id);
        return;
      }
      pending.push(id);
      if (pending.length > MAX_PENDING) { pending.shift(); }
    }

    function onResponse(d) {
      var at = isValidId(d.id) ? pending.indexOf(d.id) : -1;
      if (at === -1) { return; }
      pending.splice(at, 1);
      var status = typeof d.status === 'number' && Number.isInteger(d.status) ? d.status : 0;
      var body = typeof d.body === 'string' ? d.body : '';
      if (typeof d.body !== 'string' || status === 0) { status = 0; body = ''; }
      var out = { id: d.id, status: status, body: body };
      if (frameBytes(RESPONSE, out) > MAX_FRAME_BYTES) {
        out = { id: d.id, status: 0, body: '' };
      }
      send(RESPONSE, out);
    }

    function onStatus(d) {
      if (announced === '') { return; } // not an answer to any hello
      relayUp = true;
      locked = d.locked === true;
      if (d.ready === true && isValidModel(d.model)) {
        ready = true;
        // finds: the player allowed their key to name what they find.
        // lively: the player allowed their key to make the world livelier
        // around them (townsfolk idle moments and every such feature).
        send('Companion.Relay.Ready', { model: d.model, finds: d.finds === true, lively: d.lively === true });
      } else {
        ready = false;
        send('Companion.Relay.Gone', {});
      }
      changed();
    }

    // onFrameMessage admits only the relay frame itself, on the relay
    // origin, sending an object.
    function onFrameMessage(ev) {
      if (relayOrigin === '' || !ev || ev.origin !== relayOrigin || !env.frameWindow ||
          ev.source !== env.frameWindow() || ev.source === null || ev.source === undefined) {
        return;
      }
      var d = ev.data;
      if (!d || typeof d !== 'object' || Array.isArray(d)) { return; }
      switch (d.type) {
        case 'response': onResponse(d); break;
        case 'status': onStatus(d); break;
        case 'hide': hide(); break;
      }
    }

    // openSetup shows the relay's key form. Only for a logged-in player
    // whose relay has answered.
    function openSetup() {
      if (account === '' || announced !== account || !relayUp) { return false; }
      if (!post({ type: 'setup' })) { return false; }
      open = true;
      changed();
      return true;
    }

    return {
      hello: hello, frameLoaded: frameLoaded, disconnected: disconnected,
      onGMCPRequest: onGMCPRequest, onFrameMessage: onFrameMessage,
      openSetup: openSetup, state: state
    };
  }

  // boot builds the frame and wires the glue into the page. It does nothing
  // unless the server named a relay origin. opts: sendGMCP(pkg, obj), and
  // the page's key button (shown only while the glue allows setup).
  function boot(win, doc, opts) {
    var relayOrigin = canonicalRelayOrigin(win.COMPANION_RELAY_ORIGIN);
    if (relayOrigin === '' || !doc || !doc.body) { return null; }
    opts = opts || {};
    var button = opts.button || null;

    var overlay = doc.createElement('div');
    overlay.id = 'companion-relay-overlay';
    overlay.style.cssText = 'display:none;position:fixed;inset:0;z-index:10000;' +
      'background:rgba(0,0,0,0.6);align-items:center;justify-content:center';

    var frame = doc.createElement('iframe');
    // allow-scripts: the relay is a script. allow-same-origin: it keeps its
    // OWN origin (the relay host) for its localStorage and WebCrypto; it
    // gains nothing over this page, whose origin differs. allow-popups and
    // allow-popups-to-escape-sandbox: from a click INSIDE the frame it opens
    // the key window, a top-level page on its own origin whose address bar
    // the player can check; the window is the frame's, never this page's.
    // No allow-forms: the relay has no form, so no password manager is ever
    // offered the key. Nothing else: it may not navigate this page or show
    // dialogs. No allow attribute, so it gets no browser features; no name,
    // so nothing can target it.
    frame.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox');
    frame.setAttribute('referrerpolicy', 'no-referrer');
    frame.title = 'Companion key';
    frame.style.cssText = 'width:min(440px,95vw);height:min(560px,90vh);border:1px solid #6b5a3a;' +
      'border-radius:6px;background:#111';

    var glue = createGlue({
      relayOrigin: relayOrigin,
      sendGMCP: opts.sendGMCP,
      postToFrame: function (msg, origin) {
        if (frame.contentWindow) { frame.contentWindow.postMessage(msg, origin); }
      },
      frameWindow: function () { return frame.contentWindow; },
      onHide: function () { overlay.style.display = 'none'; },
      onState: function (s) {
        overlay.style.display = s.open ? 'flex' : 'none';
        if (button) {
          button.style.display = s.visible ? '' : 'none';
          button.textContent = s.locked ? 'Unlock companion key' : 'Companion key';
        }
      }
    });

    frame.addEventListener('load', function () { glue.frameLoaded(); });
    win.addEventListener('message', function (ev) { glue.onFrameMessage(ev); });
    if (button) {
      button.addEventListener('click', function () { glue.openSetup(); });
    }

    overlay.appendChild(frame);
    doc.body.appendChild(overlay);
    frame.src = relayOrigin + '/companion-relay.html';
    return glue;
  }

  return {
    MAX_FRAME_BYTES: MAX_FRAME_BYTES,
    canonicalRelayOrigin: canonicalRelayOrigin, frameBytes: frameBytes,
    createGlue: createGlue, boot: boot
  };
}));
