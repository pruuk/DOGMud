# #375 phase 4 slice 3: live destination examples and playtest, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Shipped content has a live step for every destination render (a pin, an NPC, an area
wash, a zone banner, anywhere), each kind is playtested by a seat, and each is captured from the
real web client into a screenshot.

**Architecture:** Content edits on three quest steps; one guard test that every destination
kind keeps a live step; a small reusable Node script that drives headless Edge over the DevTools
protocol to log into the real web client of a local server and save screenshots; a three-seat
playtest scenario. No engine change. Slices 1 and 2 shipped the resolver and the rendering.

**Tech stack:** quest YAML; Go test in `internal/quests`; Node 24 (built-in `WebSocket` and
`fetch`, no packages) with Microsoft Edge headless; the playtest harness.

**Spec:** `docs/superpowers/specs/2026-10-28-quest-arc-phase4-minimap-guidance-design.md`,
sections 6 and 7 and the slice list in section 8. Slice 3 of 3.

**Owner choices (2026-10-28, this session):**
- Area example: quest 45 `start` becomes `destination: {area: unquiet_grove}` (an explicit block,
  not inference: slice 1's conversion had given it `room: 5311`, and the spec's own section 1
  example is the explicit form).
- Several-copies example: quest 33 `start` becomes `destination: {mob: 9110}` (Bandit Scout,
  four spawn rooms, no declared fallback, so an unseen scout falls back to the spawn rooms).
- Zone example, option 1: quest 61 `bandit` becomes `destination: {mob: 9178, zone: The Empty
  Reach}` (ruling 11's per-step zone fallback). The Reach-Touched is a hidden ambusher, so a
  player without see-hidden always gets the zone: an arrow toward the Reach from outside, the
  "Somewhere in The Empty Reach" banner inside. Inside, the panel and the `quest` log read
  "You are in The Empty Reach." (`GuideLine` checks a label before it checks for a fallback,
  as spec section 4 orders the rows.)

---

## Ground rules

- Edit files with the Edit and Write tools only (no `sed -i`, no `>` into tracked files, no Python
  rewrites). `git add` named paths only; never `-A`, `.`, `--all`, `commit -a`, `--no-verify`.
- Commits via quoted heredoc ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Every new check is seen failing before it passes.
- No em or en dashes in comments, copy or docs. Player copy follows `dogmud-player-copy`.
- Green against blue never carries meaning (owner is green/blue colourblind).
- Every `gh` command carries `--repo pruuk/DOGMud`.
- Servers: private ports only, kill by PID only, never by name or port sweep. The owner's own
  server may be running on port 8090.
- Scratch files (override config, manifest, Edge profile, logs) go in the session scratchpad,
  by its full path, never `$TMP` or `/tmp`.

## Facts verified against source (master `550bffa46`, 2026-10-28)

| Fact | Where |
|---|---|
| `quests.StepDestination{Anywhere bool; Room int; Mobs []int; Area string; Zone string}` | `internal/quests/destination.go:19-25` |
| `Quest.QuestId int`, `Quest.Secret bool`, `Quest.Steps []QuestStep` | `internal/quests/quests.go:67-71` |
| `chdirRepoRootForTest(t)` (package `quests`, test-only) chdirs to the repo root and reloads config; the round-trip test globs `configs.GetFilePathsConfig().DataFiles + "/quests/*.yaml"` | `internal/quests/unification_equivalence_test.go:272`; `roundtrip_test.go:20-27` |
| `mobSet`: perceived copies first (`Perceives` plus `SightFull` in the copy's room), else the declared room, area or zone fallback, else the union of spawn rooms as Kind area with no label; `npc` and `fallback` are set on every fallback | `internal/questengine/destination.go:190-218` |
| `zoneSet` has Kind zone and `label` = the zone name; no `zoneRooms`, so `target_rooms` is empty for a zone | `destination.go:106-108, 397-404` |
| `GuideLine` inside: a Label gives "You are in <label>."; else a fallback with an NPC gives "Usually found around here: <npc>."; else "You are where you need to be." | `internal/questengine/guide.go` |
| `GuideLine`'s doc comment and `internal/questengine/context.md:264-265` still say the web client does not read `guide` yet; the panel has read it since slice 2 | `guide.go:5-8`; `webclient-pure.html:1040-1043` |
| Client banner: drawn only when `inside`, a `label`, and kind `area` or `zone`; the wash needs kind `area` or at least two `target_rooms` | `static/js/gmcp.js:674, 722-724` |
| Existing unit tests already cover a mob with a zone fallback, several copies, the spawn-room fallback, and each `GuideLine` row | `destination_test.go:165, 310, 331`; `guide_test.go:17-21` |
| Quest 45 `start` today: `destination: {room: 5311}`; its `cast` trigger is `area: unquiet_grove`; area rooms 5311, 5312, 5313 (`the unquiet grove`); Orrin's Whispering Grove 5309 is outside the area, `west` to 5311 | `quests/45-the_unquiet_grove.yaml:37-40, 89-93`; `areas/unquiet_grove.yaml`; `rooms/pothole_coulee/5309.yaml` |
| Quest 33 `start` today: `destination: {room: 5232}`; trigger `mob_death` 9110 gated `has: [33-start]` | `quests/33-hold_the_wash.yaml:33-44, 84-90` |
| Bandit Scout 9110: Pothole Coulee, hostile, maxwander 1, spawns in 5232 Lower Wash, 5233 Gravel Bend, 5236 Upper Wash, 5238 Broken Gate; Squatter's Hollow is 5234 | `mobs/pothole_coulee/9110-bandit_scout.yaml`; `rooms/pothole_coulee/*.yaml` |
| Quest 61 `bandit` today: `destination: {room: 5395}`; triggers `mob_death` 9178 gated `has: [61-bandit]` | `quests/61-hedgerow_toll.yaml:32-41, 87-95` |
| The Reach-Touched 9178: zone `The Empty Reach`, `behavior_archetype: ambusher` (hidden surprise strike), maxwander 2, sole spawn 5395 The Abandoned Camp; the zone has 12 rooms (5390 to 5401, 5390 "Where the Road Gives Out"); Pemberton's Lake & Ladle 5378 is in `North Road North` | `mobs/the_empty_reach/9178-the_reach_touched.yaml`; `rooms/the_empty_reach/` |
| No shipped step declares `area:` or `zone:` today (the one `area:` hit is quest 45's trigger) | grep `^\s+(area\|zone): ` over `quests/*.yaml` |
| None of the three steps is in `loop-backlog.txt` (17 `walk-destination` lines, lines 280-296) | `_datafiles/world/dogmud/quests/loop-backlog.txt` |
| CI sweep `TestSmoke_ShippedQuestsPassTheLoop` (gated on `DOGMUD_BOOT_SMOKE=1`) runs `Destinations` and ratchets against the backlog | `quest_loop_smoke_test.go:26-62` |
| The playtest container publishes only the AI port (55555), so the browser cannot reach it | `internal/playtestenv/compose.playtest.yml:7`; `readiness.go:14` |
| A server materializes playtest profiles at boot when `Playtest.ProfilesManifest` is set (`ProfilesDir` defaults to `tools/playtest/profiles`) and writes `creds.json` beside the manifest | `main.go:434`; `internal/playtestprofiles/materialize.go:94-111`; `internal/configs/config.playtest.go` |
| Manifest: `entries: [{profile, start_room, actor_id, overlays}]`; overlays include `set_quest_tokens`, `grant_spells`; creds rows `{profile, actor_id, username, password}` | `internal/playtestprofiles/types.go:63-99` |
| Profiles carry no quest progress, so a seat's only active quest is the one its tokens set; `GetFocusQuestId` then picks it | `tools/playtest/profiles/*.yaml`; `internal/characters/quests.go:359` |
| `server set day` / `server set night` (admin) | `internal/usercommands/admin.server.go:99-108` |
| Web client: served at `/webclient-pure.html` on `Network.HttpPort`; `#connect-button`; `#command-input` keydown handler acts on `event.key === 'Enter'` with `event.target.value` | `webclient-pure.html:269, 327, 2486, 2810-2840` |
| Node v24.13.0; Edge at `C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe` | `node --version`; `ls` |
| CI runs every `tools/webclient-tests/*.js`, so the screenshot script must live elsewhere | `.github/workflows/validate.yml:102` |
| Boot recipe: detached worktree, `go build -o boot-check.exe .`, `CONFIG_PATH=<scratch override>` with ports 43333 / 19999 / 18090 / 45555 | `.claude/skills/dogmud-shipping/SKILL.md:149-162` |
| Spells: `chrysalis-glow`, `conviction-spike` | `_datafiles/world/dogmud/spells/` |

## File map

| File | Change |
|---|---|
| `internal/quests/live_destination_kinds_test.go` (new) | every destination kind has a live step |
| `_datafiles/world/dogmud/quests/45-the_unquiet_grove.yaml` | `start`: area destination |
| `_datafiles/world/dogmud/quests/33-hold_the_wash.yaml` | `start`: mob 9110 |
| `_datafiles/world/dogmud/quests/61-hedgerow_toll.yaml` | `bandit`: mob 9178, zone fallback |
| `internal/questengine/guide.go`, `internal/questengine/context.md` | stale "web client does not read `guide`" lines |
| `tools/webshot/live-shot.js` (new), `tools/webshot/README.md` (new) | screenshot driver |
| `tools/playtest/scenarios/375-p4s3-live-kinds.yaml` (new) and `tools/playtest/goals/scenarios/375-p4s3-live-kinds/{grove,wash,reach}.yaml` (new) | playtest |
| `.claude/skills/dogmud-authoring-quests/SKILL.md` | choosing a mob fallback; the live examples |
| `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, `docs/PATCH_NOTES.md`, `docs/README.md` | status, notes, index |

---

### Task 0: Worktree

- [ ] **Step 1:** From the main checkout on master:

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin && git status --short   # want clean
git worktree add -b feat/375-p4-content C:/tmp/dogmud-375-p4s3 origin/master
```

- [ ] **Step 2:** Write memory `project-375-phase4-slice3-worktree.md` naming `C:/tmp/dogmud-375-p4s3`
  and branch `feat/375-p4-content`, add its line to `MEMORY.md`. Every later task works in that
  worktree.

### Task 1: Guard test, every destination kind keeps a live step

**Files:** Create `internal/quests/live_destination_kinds_test.go`

- [ ] **Step 1: Write the test.** The principle is the render kind, not a list of quests: any
  non-secret step of the kind counts.

```go
package quests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"gopkg.in/yaml.v2"
)

// Every way the map can show a destination has at least one live step in
// shipped content (#375 phase 4 slice 3), so the playtests and screenshots
// that prove each render keep a step to stand on. A kind counts whether it
// is the destination or a mob's fallback: a zone fallback draws the same
// banner as a zone. Any non-secret step of the kind will do.
func TestShippedContent_EveryDestinationKindHasALiveStep(t *testing.T) {
	chdirRepoRootForTest(t)

	dir := configs.GetFilePathsConfig().DataFiles.String() + `/quests`
	files, err := filepath.Glob(dir + `/*.yaml`)
	if err != nil || len(files) == 0 {
		t.Fatalf("no quest files found under %s: %v", dir, err)
	}

	seen := map[string]string{}
	note := func(kind, where string) {
		if _, ok := seen[kind]; !ok {
			seen[kind] = where
		}
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var q Quest
		if err := yaml.Unmarshal(data, &q); err != nil {
			t.Fatalf("%s: parse: %v", f, err)
		}
		if q.Secret {
			continue
		}
		for _, s := range q.Steps {
			d := s.Destination
			where := fmt.Sprintf("quest %d step %s", q.QuestId, s.Id)
			if d.Anywhere {
				note("anywhere", where)
			}
			if len(d.Mobs) > 0 {
				note("mob", where)
			}
			if d.Room > 0 {
				note("room", where)
			}
			if d.Area != "" {
				note("area", where)
			}
			if d.Zone != "" {
				note("zone", where)
			}
		}
	}
	for _, kind := range []string{"anywhere", "room", "mob", "area", "zone"} {
		if where, ok := seen[kind]; ok {
			t.Logf("%s: %s", kind, where)
		} else {
			t.Errorf("no live non-secret step has destination kind %q", kind)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/quests/ -run TestShippedContent_EveryDestinationKindHasALiveStep -v`
Expected: FAIL with `no live non-secret step has destination kind "area"` and the same for
`"zone"`; `anywhere`, `room` and `mob` logged with a quest each. If `area` or `zone` passes,
stop: a declared one exists that the facts table missed, so re-grep before going on. Do not
commit yet: Task 2 makes it pass.

### Task 2: The three live steps

**Files:** Modify the three quest files below.

- [ ] **Step 1: Quest 45 `start`.** In `_datafiles/world/dogmud/quests/45-the_unquiet_grove.yaml`
  replace

```yaml
  - id: start
    destination:
      room: 5311  # Veil Hollow, west of Orrin; a grove revenant spawns
```

with

```yaml
  - id: start
    destination:
      area: unquiet_grove  # the cast counts in any grove room (the cast trigger's area)
```

  The `cast` step keeps `room: 5311` (the kill can fall outside the area: drift-wisps also spawn
  at 5308 and 5310, so the area would be wrong there).

- [ ] **Step 2: Quest 33 `start`.** In `_datafiles/world/dogmud/quests/33-hold_the_wash.yaml` replace

```yaml
  - id: start
    destination:
      room: 5232  # Lower Wash, north of Squatter's Hollow: Bandit Scout (mob 9110)
```

with

```yaml
  - id: start
    destination:
      mob: 9110  # the nearest Bandit Scout the player can see; else the four rooms that spawn one
```

  The description's "The nearest scout is in the Lower Wash" and the hint stay: the Lower Wash
  is the nearest spawn to Squatter's Hollow, so the text and the arrow agree.

- [ ] **Step 3: Quest 61 `bandit`.** In `_datafiles/world/dogmud/quests/61-hedgerow_toll.yaml`
  replace

```yaml
  - id: bandit
    destination:
      room: 5395
```

with

```yaml
  - id: bandit
    destination:
      mob: 9178  # the Reach-Touched, a hidden ambusher: tracked only by a player who sees hidden
      zone: The Empty Reach  # otherwise the zone it haunts (owner 2026-10-28, ruling 11)
```

- [ ] **Step 4: Run the guard test and see it pass.**

Run: `go test ./internal/quests/ -run TestShippedContent_EveryDestinationKindHasALiveStep -v`
Expected: PASS, logging `area: quest 45 step start` and `zone: quest 61 step bandit`.

- [ ] **Step 5: Round trip, validators, loop sweep, boot.**

```bash
go test ./internal/quests/ ./internal/questengine/
DOGMUD_BOOT_SMOKE=1 go test . -run 'TestSmoke_ShippedQuestsPassTheLoop|TestSmoke_ServerBootsCleanWithRealData' -v 2>&1 | tail -30
```

Expected: all PASS; the loop sweep reports no new finding and no stale backlog line (none of the
three steps was in the backlog). If `walk-destination` appears for any of the three, stop and
report: a bare `mob: 9110` with no spawn rooms would be one, and the facts say it has four.

- [ ] **Step 6: Commit.**

```bash
git add internal/quests/live_destination_kinds_test.go \
  _datafiles/world/dogmud/quests/45-the_unquiet_grove.yaml \
  _datafiles/world/dogmud/quests/33-hold_the_wash.yaml \
  _datafiles/world/dogmud/quests/61-hedgerow_toll.yaml
git commit -F - <<'EOF'
content(quests): live area, several-copies and zone destinations (#375 p4)

Quest 45 start points at the unquiet grove area, quest 33 start at the
nearest Bandit Scout the player can see, quest 61 bandit at the
Reach-Touched with The Empty Reach as its fallback. A guard test keeps a
live step of every destination kind.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

### Task 3: Stale `guide` comments

**Files:** `internal/questengine/guide.go:5-8`, `internal/questengine/context.md:264-265`

- [ ] **Step 1:** In `guide.go` replace

```go
// `quest` command prints it, and the GMCP quest payload carries it as `guide`
// so the web quest panel can say the same words. The web client does not read
// `guide` yet; it shows the line from phase 4 slice 2. Plain text: the panel
// will escape markup.
```

  with

```go
// `quest` command prints it, and the GMCP quest payload carries it as `guide`,
// which the web quest panel shows, so both say the same words. Plain text: the
// panel escapes markup.
```

- [ ] **Step 2:** In `context.md` replace the sentence "The web client does not read `guide`
  yet: the quest panel shows it from phase 4 slice 2." with "The web quest panel shows it when
  the pin is not on the drawn map."
- [ ] **Step 3:** `go build ./internal/questengine/` then commit both files (`docs(questengine): guide is read by the web panel since slice 2 (#375 p4)`).

### Task 4: Screenshot driver

**Files:** Create `tools/webshot/live-shot.js`, `tools/webshot/README.md`

- [ ] **Step 1: Write the script.**

```js
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
//                    {"eval": "document.title"}, {"shot": "C:/.../a.png"} ] } ] }
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
      await sleep(4000);
      for (const st of s.steps) {
        if (st.send !== undefined) {
          await typeLine(dt, st.send);
          await sleep(st.after || 1500);
        } else if (st.wait) {
          await sleep(st.wait);
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
```

- [ ] **Step 2: Write `tools/webshot/README.md`.**

```markdown
# webshot

`live-shot.js` logs characters into the real web client of a running local
server, in headless Edge, and saves screenshots. It drives the page over the
DevTools protocol with Node's built-in WebSocket; no packages.

It never starts a server. Boot one from a detached worktree on private ports
(`dogmud-shipping`, boot check), with `Playtest.ProfilesManifest` in the
override config so the server materializes seeded characters at boot and
writes their logins to `creds.json` beside the manifest
(`internal/playtestprofiles`). The owner's own server may be on this machine:
never point the script at it.

    node tools/webshot/live-shot.js <shots.json>

The comment at the top of the script gives the `shots.json` shape. Put
`shots.json`, the Edge profile folder and the logins in the session
scratchpad; they hold passwords and are throwaway.

It lives outside `tools/webclient-tests/` because CI runs every script there.
```

- [ ] **Step 3:** `node --check tools/webshot/live-shot.js` (want no output). Commit both files
  (`tools(webshot): drive the real web client in headless Edge for screenshots (#375 p4)`). Add
  the folder to `docs/README.md` in Task 7.

### Task 5: Screenshots from the real web client

No repo change except the PNGs, which go in the main checkout's root, untracked (owner rule:
screenshots in the working folder).

- [ ] **Step 1: Manifest.** Set `S` to the session scratchpad's full path. Write `$S/shots/manifest.yaml`
  with the Write tool:

```yaml
entries:
  - {profile: admin, start_room: 5309, actor_id: admin}
  - profile: early
    start_room: 5309          # Whispering Grove, Orrin; the grove is west
    actor_id: grove
    overlays:
      set_quest_tokens: ["45-start"]
      grant_spells: {chrysalis-glow: 1, conviction-spike: 1}
  - profile: early
    start_room: 5234          # Squatter's Hollow; the Lower Wash is north
    actor_id: wash
    overlays:
      set_quest_tokens: ["33-start"]
  - profile: early
    start_room: 5378          # the Lake & Ladle, North Road North
    actor_id: reach
    overlays:
      set_quest_tokens: ["61-start", "61-bandit"]
  - profile: early
    start_room: 5390          # Where the Road Gives Out, inside The Empty Reach
    actor_id: reachin
    overlays:
      set_quest_tokens: ["61-start", "61-bandit"]
```

- [ ] **Step 2: Override config.** Write `$S/shots/override.yaml`:

```yaml
Network:
  TelnetPort: [43333]
  LocalPort: 19999
  HttpPort: 18090
  AIPort: 45555
Playtest:
  ProfilesManifest: <full path of $S>/shots/manifest.yaml
```

- [ ] **Step 3: Boot.** Detached worktree at the branch head, built to a fixed path, started
  hidden (no console window), PID recorded:

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git worktree add --detach C:/tmp/dogmud-boot-check feat/375-p4-content
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
```

```powershell
$S = '<full scratchpad path>'
$env:CONFIG_PATH = "$S\shots\override.yaml"
$p = Start-Process -FilePath C:\tmp\dogmud-boot-check\boot-check.exe -WorkingDirectory C:\tmp\dogmud-boot-check `
  -RedirectStandardOutput "$S\shots\server.log" -RedirectStandardError "$S\shots\server.err" -WindowStyle Hidden -PassThru
$p.Id   # record it
```

  Wait for `Server Ready` in `server.log` (poll with a check command, not a sleep), then read
  `$S/shots/creds.json` for the four logins.

- [ ] **Step 4: Shots file.** Write `$S/shots/shots.json` (url `http://127.0.0.1:18090/webclient-pure.html`,
  `profileDir` `$S/shots/edge-profile`, the Edge path from the facts). Sessions in this order,
  `R` = the main checkout root `C:/Users/Calabe Davis/workspace/DOGMud`:

| Session | Steps | Shot | Expect |
|---|---|---|---|
| admin | `server set day` | (none) | "Time set to ..." |
| grove | `look`, `quests` | `R/p4s3-area-outside.png` | brass wash and plate "the unquiet grove" over 5311 to 5313; pin on Veil Hollow; panel "◆ marked on map" |
| grove | `west`, wait 1500 | `R/p4s3-area-inside.png` | banner "Somewhere in the unquiet grove"; no pin; `quests` reads "You are in the unquiet grove." (a revenant may attack: shoot at once) |
| wash | `look`, `quests` | `R/p4s3-mob-seen.png` | pin on the nearest scout's room; `quests` line "Head north." |
| admin | `server set night` | (none) | |
| wash | `look`, `quests` | `R/p4s3-mob-fallback.png` | if the scout is too dark to see: a wash without a plate over the spawn rooms on the map, line "Head north." toward the nearest; if still seen, record that and keep the shot |
| reach | `look`, `quests` | `R/p4s3-zone-outside.png` | off-map arrow stub; panel line "Head <dir>." |
| reachin | `look`, `quests` | `R/p4s3-zone-inside.png` | banner "Somewhere in The Empty Reach"; no pin; no wash; `quests` reads "You are in The Empty Reach." |

  Each character is one login; the two admin rows are two short admin logins. The walk from
  outside the Reach to inside it is the playtest's job (Task 6, the `reach` seat), so the
  inside shot uses its own character seeded at 5390, four sessions in the table order.

- [ ] **Step 5: Run and look.** `node tools/webshot/live-shot.js "$S/shots/shots.json"` from the
  worktree. The first run, add a `shot` right after login to confirm the login waits are long
  enough (the screen should show the room, not the password prompt); lengthen the waits if not.
  Open every PNG with the Read tool and check it against the Expect column. A screenshot that
  does not match is a finding: stop and report it with the image, do not change the engine or
  the client in this slice without the owner.

- [ ] **Step 6: Stop the server by its PID** (`Stop-Process -Id <pid>`; never by name), confirm
  with `Get-Process -Id <pid>` that it is gone, then `Remove-Item -Recurse -Force C:\tmp\dogmud-boot-check`
  from outside that folder and `git worktree prune` in the main checkout. Confirm no Edge from the
  script is left: the script prints its PID; `Get-Process -Id <that pid>` should fail.

### Task 6: Playtest, three seats

**Files:** Create `tools/playtest/scenarios/375-p4s3-live-kinds.yaml` and three goals files.

- [ ] **Step 1: Scenario.**

```yaml
# #375 phase 4 slice 3: one live step of each new destination kind. A seat on
# quest 45 casts inside the unquiet grove (an area), a seat on quest 33 hunts
# the nearest Bandit Scout (several copies), a seat on quest 61 hunts the
# hidden Reach-Touched across The Empty Reach (a zone fallback).
name: 375-p4s3-live-kinds
mode: party
summary: >-
  The quest log's destination line leads each seat to its step's place: the
  grove area, the nearest bandit scout, and The Empty Reach.
on_actor_stop: continue
budgets:
  wall_clock: 40m
requires:
  max_connections: 20
roster:
  - id: grove
    personality: bug-finder
    goals: goals/scenarios/375-p4s3-live-kinds/grove.yaml
  - id: wash
    personality: bug-finder
    goals: goals/scenarios/375-p4s3-live-kinds/wash.yaml
  - id: reach
    personality: bug-finder
    goals: goals/scenarios/375-p4s3-live-kinds/reach.yaml

group_goals:
  - id: destination-line-guides
    do: Each seat reads the quest log's destination line after every move and follows it.
    verify: The line always names a real exit of the current room or the right "you are in" message, and following it reaches the step's place.
```

- [ ] **Step 2: `grove.yaml`.**

```yaml
# GROVE seat: quest 45, The Unquiet Grove, first step: cast a spell with a
# fight on you anywhere in the unquiet grove. You start in the Whispering
# Grove with Keeper Orrin. Quote every game line VERBATIM.

ephemeral:
  profile: early
  start_room: 5309
  overlays:
    set_quest_tokens: ["45-start"]
    grant_spells: {chrysalis-glow: 1, conviction-spike: 1}
  budgets:
    wall_clock: 30m

goals:
  - >-
    Type `quests` and quote the whole entry for The Unquiet Grove, including
    the last line under it (the line that says where to head).
  - >-
    Follow that line: if it says "Head <direction>.", type that direction,
    then `quests` again and quote the new line. Repeat until it changes. Note
    any direction that is not an exit of your room (type `look` to check).
  - >-
    When the line says you are in the unquiet grove, quote it. Walk to one
    other grove room by its exits and quote the line there too: it should
    still say you are in the grove. Then fight a creature there and type
    `cast spike <creature>`; quote what the game says and the quest line
    after it.
```

- [ ] **Step 3: `wash.yaml`.**

```yaml
# WASH seat: quest 33, Hold the Wash, first step: kill a bandit scout. Several
# scouts spawn along the wash. You start in Squatter's Hollow. Quote every
# game line VERBATIM.

ephemeral:
  profile: early
  start_room: 5234
  budgets:
    wall_clock: 30m
  overlays:
    set_quest_tokens: ["33-start"]

goals:
  - >-
    Type `quests` and quote the whole entry for Hold the Wash, including the
    last line under it.
  - >-
    Follow that line one move at a time, typing `quests` after each move and
    quoting the line, until a bandit scout is in your room. Say how many moves
    it took and whether the scout was in the first room the line led you to.
    If it is dark, type `cast glow` first, and say whether the line changed
    when the room lit.
  - >-
    Kill the scout (`attack scout`). After it falls, quote the quest line for
    the next step.
```

  (If `cast glow` is refused because the early profile lacks the spell, add
  `grant_spells: {chrysalis-glow: 1}` to the overlays and rerun; lesson from 10-27.)

- [ ] **Step 4: `reach.yaml`.**

```yaml
# REACH seat: quest 61, Hedgerow Toll, second step: the hedgerow bandit is
# dead; put down the Reach-Touched in The Empty Reach to the north. It hides
# and strikes from ambush. You start at the Lake & Ladle. Quote every game
# line VERBATIM.

ephemeral:
  profile: early
  start_room: 5378
  overlays:
    set_quest_tokens: ["61-start", "61-bandit"]
  budgets:
    wall_clock: 30m

goals:
  - >-
    Type `quests` and quote the whole entry for Hedgerow Toll, including the
    last line under it.
  - >-
    Follow that line one move at a time, typing `quests` after each move and
    quoting the line, until it says you are in The Empty Reach. Note any
    direction that is not an exit of your room.
  - >-
    Inside The Empty Reach, use the step's hint to reach the Abandoned Camp.
    Quote the quest line in each room on the way: it should keep saying you
    are in The Empty Reach and never point at the Reach-Touched itself. If the
    Reach-Touched attacks, fight it and quote the quest line after it falls.
```

- [ ] **Step 5: Run it.** `/playtest-scenario --checkout C:/tmp/dogmud-375-p4s3 375-p4s3-live-kinds.yaml`
  per `dogmud-playtesting`. After it ends, `docker ps -a | grep <run_id>` and `docker rm -f` any
  container left (the stop leaves it; 10-28 lesson).
- [ ] **Step 6: Findings.** For each defect: check it on master, search open issues
  (`gh issue list --repo pruuk/DOGMud --search "<words>"`), file new ones with labels `bug`,
  `from-playtest` and an `area:*` label. A defect in this slice's three steps is fixed in the
  branch, with the guard and loop sweep rerun. Write the harness lessons memory
  (`project-375-phase4-slice3-playtest-harness-lessons-2026-10-28.md` or the run date) and its
  `MEMORY.md` line.
- [ ] **Step 7:** Commit the scenario and goals files (`test(playtest): slice 3 live destination kinds scenario (#375 p4)`).

### Task 7: Docs

- [ ] **Step 1: Authoring skill.** In `.claude/skills/dogmud-authoring-quests/SKILL.md`, after
  the `mob` takes one id or a list bullet, add:

```markdown
- **Choosing a mob's fallback.** An NPC that stays put needs none: its spawn
  room is the fallback. A beast with several spawns takes a bare `mob`, so an
  unseen one points at every room that spawns it (quest 33 `start`, the Bandit
  Scout). A hidden or wandering NPC takes the place it haunts, `zone:` or
  `area:`, because the player will rarely see it and its spawn room would
  give away where it hides (quest 61 `bandit`, the Reach-Touched in The Empty
  Reach). Inside a zone or area fallback the log reads "You are in <name>.".
- **Live examples of each kind** are held by
  `TestShippedContent_EveryDestinationKindHasALiveStep`
  (`internal/quests/live_destination_kinds_test.go`): area, quest 45 `start`;
  zone, quest 61 `bandit`. Moving the last step of a kind fails it.
```

- [ ] **Step 2: Roadmap.** In `docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`, phase 4 **Status**:
  slices 1 and 2 "merged (#709, #710)"; slice 3 "(live area, several-copies and zone-fallback
  steps, the playtest, screenshots from the real web client) is in PR, plan
  `docs/superpowers/plans/2026-10-28-quest-arc-phase4-slice3-content.md`".
- [ ] **Step 3: Patch notes.** Add to the top 2026-10-28 entry of `docs/PATCH_NOTES.md`
  (player copy, no numbers):

```markdown
- The Unquiet Grove's first step now shades the whole grove on the
  map, since a spell cast anywhere in it counts.
- Hold the Wash points you to the nearest bandit scout you can see.
- In Hedgerow Toll, the Reach-Touched hides, so the map sends you to
  The Empty Reach and leaves the finding to you.
```

- [ ] **Step 4: Index.** In `docs/README.md` (this plan's row is already there) add a row for
  `../tools/webshot/` right after the `../tools/webclient-tests/` row (line 98 on master),
  describing `live-shot.js` in one sentence as the README in Task 4 does.
- [ ] **Step 5:** Commit the four docs files.

### Task 8: Gate and PR

- [ ] **Step 1: Gate** per `dogmud-shipping`: `go build ./...`, `go vet ./...`, `go test ./...`,
  `golangci-lint run --new-from-merge-base=origin/master`,
  `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke`, every `node tools/webclient-tests/*.js`, the
  detached-worktree boot check (Task 5's boot counts if the branch has not changed since; else
  rerun). Paste each tail into the PR.
- [ ] **Step 2: Push and PR.** `git push -u origin feat/375-p4-content`, then
  `gh pr create --repo pruuk/DOGMud --base master` with: summary; the three step changes and why
  each; the guard test; the screenshots listed by file name in the repo root (untracked, the
  owner opens them there); playtest results and issues filed; gate output; deploy notes (quest
  files only; no save, config or engine change). End with the generated-with line. Do not write
  "closes #375" (phases 5 to 7 remain).
- [ ] **Step 3:** Watch CI (`gh pr checks --repo pruuk/DOGMud`), read failures with
  `gh run view <id> --repo pruuk/DOGMud --log-failed`. Merge only on the owner's go.

---

## Self-review

- Spec section 6 "one live step of each new kind": area (Task 2 step 1), zone as a fallback
  (step 3, owner option 1), several copies (step 2). Section 7 "playtest walks one quest of each
  kind and captures a screenshot of each in the browser into the repo root": Tasks 5 and 6.
  Section 8 slice 3 "the backlog lines": none of the three steps is in the backlog; the
  remaining 17 `walk-destination` lines are phase 7 work (ruling 13), unchanged here.
- Spec section 6 said quest 45 resolves "by inference"; slice 1 gave it a room, and this plan
  uses the explicit `area:` block (owner told, 2026-10-28).
- No engine change: the resolver, guide line and rendering for all three are covered by slice 1
  and 2 unit tests (facts table). The screenshots and seats are what prove them on live data.
- The outside-to-inside walk for the zone is the playtest seat's; the screenshot of the inside
  uses a second character seeded in the zone, so the script needs no route logic.
