#!/usr/bin/env node
// live-shot.js: log characters into the real web client in headless Edge and
// save screenshots (#375 phase 4 slice 3). It needs a running local server;
// it never starts one. Plain Node 24 (built-in WebSocket and fetch).
//
//   node tools/webshot/live-shot.js <shots.json>
//
// shots.json:
//   { "url": "http://127.0.0.1:18090/webclient-pure.html",
//     "edge": "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
//     "profileDir": "<scratch>/edge-profile", "width": 1600, "height": 1000,
//     "debugPort": 9333,
//     "sessions": [ { "username": "u", "password": "p",
//         "steps": [ {"send": "look", "after": 1500}, {"wait": 1000},
//                    {"tail": 20}, {"eval": "document.title"},
//                    {"shot": "C:/.../a.png"} ] } ] }
//
// "tail" prints the last N lines of the game terminal. After the password, a
// "Kick them?" prompt (the character is still linked from an earlier session)
// is answered yes.
'use strict';
const { spawn } = require('node:child_process');
const fs = require('node:fs');

const cfg = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const port = cfg.debugPort || 9333;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// pageSocket waits for Edge's debug port and returns the page target's socket URL.
async function pageSocket() {
  for (let i = 0; i < 100; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
      const page = list.find((t) => t.type === 'page');
      if (page) return page.webSocketDebuggerUrl;
    } catch (_) { /* not up yet */ }
    await sleep(200);
  }
  throw new Error(`Edge debug port ${port} never answered`);
}

function devtools(wsUrl) {
  const ws = new WebSocket(wsUrl);
  let next = 0;
  const pending = new Map();
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data);
    if (!m.id || !pending.has(m.id)) return;
    const { res, rej } = pending.get(m.id);
    pending.delete(m.id);
    if (m.error) rej(new Error(m.error.message)); else res(m.result);
  };
  const ready = new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej; });
  const send = (method, params = {}) => new Promise((res, rej) => {
    const id = ++next;
    pending.set(id, { res, rej });
    ws.send(JSON.stringify({ id, method, params }));
  });
  return { ready, send, close: () => ws.close() };
}

async function evaluate(dt, expression) {
  const r = await dt.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
  if (r.exceptionDetails) throw new Error(`${r.exceptionDetails.text}: ${expression}`);
  return r.result.value;
}

// typeLine enters one line the way a player does: the text in the command
// input, then Enter (webclient-pure.html's keydown handler reads event.key and
// event.target.value).
function typeLine(dt, text) {
  return evaluate(dt, `(() => {
    const i = document.getElementById('command-input');
    i.value = ${JSON.stringify(text)};
    i.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    return true;
  })()`);
}

// screenTail is the last n lines of the game terminal, read from the page's
// top-level xterm `term` (the terminal draws to a canvas, so the DOM has no text).
function screenTail(dt, n) {
  return evaluate(dt, `(() => {
    const b = term.buffer.active, out = [];
    for (let y = Math.max(0, b.length - ${n}); y < b.length; y++) {
      const line = b.getLine(y);
      if (line) out.push(line.translateToString(true));
    }
    return out.join('\\n');
  })()`);
}

async function main() {
  const edge = spawn(cfg.edge, [
    '--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${cfg.profileDir}`,
    `--window-size=${cfg.width || 1600},${cfg.height || 1000}`, '--no-first-run', 'about:blank',
  ], { stdio: 'ignore', windowsHide: true });
  console.log(`edge pid ${edge.pid}`);
  try {
    const dt = devtools(await pageSocket());
    await dt.ready;
    await dt.send('Page.enable');
    for (const s of cfg.sessions) {
      await dt.send('Page.navigate', { url: cfg.url });
      await sleep(2500);
      await evaluate(dt, `document.getElementById('connect-button').click(), true`);
      await sleep(2000);
      await typeLine(dt, s.username);
      await sleep(1500);
      await typeLine(dt, s.password);
      await sleep(3000);
      // A character still linked from an earlier session is offered a kick.
      if ((await screenTail(dt, 3)).includes('Kick them?')) {
        await typeLine(dt, 'y');
        await sleep(3000);
      }
      await sleep(1000);
      for (const st of s.steps) {
        if (st.send !== undefined) {
          await typeLine(dt, st.send);
          await sleep(st.after || 1500);
        } else if (st.wait) {
          await sleep(st.wait);
        } else if (st.tail) {
          console.log(await screenTail(dt, st.tail));
        } else if (st.eval) {
          console.log(JSON.stringify(await evaluate(dt, st.eval)));
        } else if (st.shot) {
          const r = await dt.send('Page.captureScreenshot', { format: 'png' });
          fs.writeFileSync(st.shot, Buffer.from(r.data, 'base64'));
          console.log(`saved ${st.shot}`);
        }
      }
    }
    dt.close();
  } finally {
    edge.kill(); // the Edge this script started, by its own PID
  }
}

main().catch((e) => { console.error(e); process.exitCode = 1; });
