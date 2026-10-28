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
//     "sessions": [ { "username": "u", "password": "p",
//         "steps": [ {"send": "look", "after": 1500}, {"wait": 1000},
//                    {"tail": 20}, {"eval": "document.title"},
//                    {"shot": "C:/.../a.png"} ] } ] }
//
// "tail" prints the last N lines of the game terminal. After the password, a
// "Kick them?" prompt (the character is still linked from an earlier session)
// is answered yes. Edge picks its own debug port (written to
// <profileDir>/DevToolsActivePort), so a browser already on a fixed port is
// never driven by mistake.
'use strict';
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const cfg = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const CALL_TIMEOUT_MS = 30000;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// On Windows the msedge.exe this script spawns relaunches itself and exits,
// so its PID is not the browser's. The browser's own PID comes from DevTools
// (SystemInfo.getProcessInfo) and is what the exit paths kill.
let edge = null;
let browserPid = 0;
function killEdge() {
  for (const pid of [browserPid, edge && edge.pid]) {
    try { if (pid) process.kill(pid); } catch (_) { /* already gone */ }
  }
}
process.on('exit', killEdge);
process.on('SIGINT', () => { killEdge(); process.exit(130); });

// targets waits for the Edge this script started to write its debug port,
// then returns the browser and page targets' socket URLs.
async function targets(portFile) {
  for (let i = 0; i < 150; i++) {
    try {
      const port = fs.readFileSync(portFile, 'utf8').split('\n')[0].trim();
      const version = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json();
      const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
      const page = list.find((t) => t.type === 'page');
      if (page) return { browser: version.webSocketDebuggerUrl, page: page.webSocketDebuggerUrl };
    } catch (_) { /* not up yet */ }
    await sleep(200);
  }
  throw new Error(`Edge never wrote a usable ${portFile}`);
}

function devtools(wsUrl) {
  const ws = new WebSocket(wsUrl);
  let next = 0;
  const pending = new Map();
  const failAll = (err) => {
    for (const { rej, timer } of pending.values()) { clearTimeout(timer); rej(err); }
    pending.clear();
  };
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data);
    if (!m.id || !pending.has(m.id)) return;
    const { res, rej, timer } = pending.get(m.id);
    clearTimeout(timer);
    pending.delete(m.id);
    if (m.error) rej(new Error(m.error.message)); else res(m.result);
  };
  ws.onclose = () => failAll(new Error('DevTools socket closed'));
  const ready = new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej; });
  const send = (method, params = {}) => new Promise((res, rej) => {
    const id = ++next;
    const timer = setTimeout(() => {
      pending.delete(id);
      rej(new Error(`DevTools ${method} timed out`));
    }, CALL_TIMEOUT_MS);
    pending.set(id, { res, rej, timer });
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

// screenTail is the last n lines of the game terminal up to the cursor, read
// from the page's top-level xterm `term` (the terminal draws to a canvas, so
// the DOM has no text; rows below the cursor are blank viewport).
function screenTail(dt, n) {
  return evaluate(dt, `(() => {
    const b = term.buffer.active, out = [];
    const end = b.baseY + b.cursorY + 1;
    for (let y = Math.max(0, end - ${n}); y < end; y++) {
      const line = b.getLine(y);
      if (line) out.push(line.translateToString(true));
    }
    return out.join('\\n');
  })()`);
}

// connect clicks Connect and waits for the page's game socket (the top-level
// `socket`) to open, so the login lines are never typed into a closed client.
async function connect(dt) {
  await evaluate(dt, `document.getElementById('connect-button').click(), true`);
  for (let i = 0; i < 75; i++) {
    if (await evaluate(dt, `typeof socket !== 'undefined' && !!socket && socket.readyState === 1`)) return;
    await sleep(200);
  }
  throw new Error(`the web client never connected to ${cfg.url}`);
}

async function main() {
  const portFile = path.join(cfg.profileDir, 'DevToolsActivePort');
  fs.rmSync(portFile, { force: true });
  edge = spawn(cfg.edge, [
    '--headless=new', '--remote-debugging-port=0', `--user-data-dir=${cfg.profileDir}`,
    `--window-size=${cfg.width || 1600},${cfg.height || 1000}`, '--no-first-run', 'about:blank',
  ], { stdio: 'ignore', windowsHide: true });
  const spawnFailed = new Promise((_, rej) => edge.on('error', rej));
  let browser = null;
  try {
    const t = await Promise.race([targets(portFile), spawnFailed]);
    browser = devtools(t.browser);
    await browser.ready;
    const info = await browser.send('SystemInfo.getProcessInfo');
    browserPid = (info.processInfo.find((p) => p.type === 'browser') || {}).id || 0;
    console.log(`edge browser pid ${browserPid}`);
    const dt = devtools(t.page);
    await dt.ready;
    await dt.send('Page.enable');
    for (const s of cfg.sessions) {
      await dt.send('Page.navigate', { url: cfg.url });
      await sleep(2500);
      await connect(dt);
      await sleep(1500);
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
    if (browser) await browser.send('Browser.close').catch(() => {});
    await sleep(1000);
    killEdge();
  }
}

main().catch((e) => { console.error(e); process.exitCode = 1; });
